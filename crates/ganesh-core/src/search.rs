use crate::citation;
use crate::store::{permitted, Store};
use crate::types::{PassageHit, SearchResponse};

/// Retrieval and abstention parameters. Calibrated on the development
/// question set only; see fixtures/questions/FREEZE.md section 7.
#[derive(Debug, Clone)]
pub struct RetrievalConfig {
    /// Maximum passages returned per the evidence contract.
    pub top_k: usize,
    /// Minimum IDF-weighted fraction of distinct content query terms that must
    /// appear in at least one of the top-k passages (union coverage) for the
    /// system not to abstain. Rare terms weigh more; terms absent from the
    /// corpus weigh the most and can never be covered.
    pub min_idf_union_coverage: f64,
    /// Minimum unweighted union coverage, as a backstop.
    pub min_union_coverage: f64,
    /// Minimum fraction of distinct content query terms that must appear in
    /// the single best passage for the system not to abstain.
    pub min_top_coverage: f64,
    /// Number of rows pulled from the FTS index before abstention analysis.
    pub candidate_limit: usize,
}

impl Default for RetrievalConfig {
    fn default() -> Self {
        RetrievalConfig {
            top_k: 3,
            min_idf_union_coverage: 0.75,
            min_union_coverage: 0.0,
            min_top_coverage: 0.0,
            candidate_limit: 3,
        }
    }
}

const STOPWORDS: &[&str] = &[
    "a", "an", "the", "is", "are", "was", "were", "in", "on", "of", "to", "and", "or", "for",
    "with", "by", "at", "from", "as", "it", "its", "do", "does", "did", "can", "could", "what",
    "which", "who", "whom", "when", "where", "why", "how", "many", "much", "if", "be", "been",
    "has", "have", "had", "not", "no", "than", "that", "this", "these", "those", "under", "over",
    "per", "vs", "into", "their", "there", "between", "within", "during", "originally", "first",
];

/// Raw lowercase alphanumeric tokens, no filtering.
fn raw_tokens(text: &str) -> Vec<String> {
    let mut out = Vec::new();
    let mut cur = String::new();
    for ch in text.chars() {
        if ch.is_alphanumeric() {
            cur.push(ch.to_ascii_lowercase());
        } else if !cur.is_empty() {
            out.push(std::mem::take(&mut cur));
        }
    }
    if !cur.is_empty() {
        out.push(cur);
    }
    out
}

fn keep_token(t: &str) -> bool {
    (t.len() >= 2 || t.bytes().any(|b| b.is_ascii_digit())) && !STOPWORDS.contains(&t)
}

thread_local! {
    static STEMMER: rust_stemmers::Stemmer = rust_stemmers::Stemmer::create(rust_stemmers::Algorithm::English);
}

fn stem_token(token: &str) -> String {
    STEMMER.with(|s| s.stem(token).into_owned())
}

/// Stemmed content tokens of a text, English Porter, stopwords removed.
/// The search index stores passage text in this normalized form; queries are
/// normalized the same way, so matching is morphologically tolerant while the
/// stored passages keep their verbatim bytes.
pub fn stemmed_tokens(text: &str) -> Vec<String> {
    raw_tokens(text)
        .into_iter()
        .filter(|t| keep_token(t))
        .map(|t| stem_token(&t))
        .collect()
}

/// Stemmed index text for one passage: article title, heading context and body.
pub fn stemmed_index_text(title: &str, section: &str, text: &str) -> String {
    stemmed_tokens(&format!("{title} {section} {text}")).join(" ")
}

/// Distinct stemmed content terms of a query.
pub fn query_terms(query: &str) -> Vec<String> {
    let mut seen = std::collections::BTreeSet::new();
    for t in stemmed_tokens(query) {
        seen.insert(t);
    }
    seen.into_iter().collect()
}

/// Build an FTS5 OR query from stemmed terms. Terms are double-quoted so
/// punctuation in terms can never become query syntax.
fn fts_query(terms: &[String]) -> String {
    terms
        .iter()
        .map(|t| format!("\"{}\"", t.replace('"', "")))
        .collect::<Vec<_>>()
        .join(" OR ")
}

struct Candidate {
    passage_id: String,
    score: f64,
}

#[derive(Debug, Clone)]
pub struct SearchDebug {
    pub union_coverage: f64,
    pub top_coverage: f64,
    pub idf_union_coverage: f64,
    pub idf_top_coverage: f64,
    pub idf2_union_coverage: f64,
    pub terms: Vec<String>,
    pub unmatched_terms: Vec<String>,
    pub candidates: Vec<(String, f64)>,
}

pub fn search(store: &Store, query: &str, caller: &str, cfg: &RetrievalConfig) -> SearchResponse {
    search_debug(store, query, caller, cfg).0
}

pub fn search_debug(store: &Store, query: &str, caller: &str, cfg: &RetrievalConfig) -> (SearchResponse, SearchDebug) {
    let mut debug = SearchDebug {
        union_coverage: 0.0,
        top_coverage: 0.0,
        idf_union_coverage: 0.0,
        idf_top_coverage: 0.0,
        idf2_union_coverage: 0.0,
        terms: vec![],
        unmatched_terms: vec![],
        candidates: vec![],
    };
    let empty = |status: &str| SearchResponse {
        status: status.to_string(),
        query: query.to_string(),
        passages: vec![],
    };

    let terms = query_terms(query);
    debug.terms = terms.clone();
    if terms.is_empty() {
        return (empty("insufficient_context"), debug);
    }
    if !permitted(caller, "public") {
        return (empty("insufficient_context"), debug);
    }

    let q = fts_query(&terms);
    let mut stmt = match store.conn.prepare(
        "SELECT passage_id, bm25(passages_fts) AS score FROM passages_fts
         WHERE passages_fts MATCH ?1
         ORDER BY score ASC, passage_id ASC
         LIMIT ?2",
    ) {
        Ok(s) => s,
        Err(_) => return (empty("insufficient_context"), debug),
    };
    let rows = stmt
        .query_map(rusqlite::params![q, cfg.candidate_limit as i64], |row| {
            Ok(Candidate {
                passage_id: row.get(0)?,
                score: row.get(1)?,
            })
        })
        .map(|r| r.filter_map(|x| x.ok()).collect::<Vec<_>>())
        .unwrap_or_default();

    if rows.is_empty() {
        return (empty("insufficient_context"), debug);
    }
    debug.candidates = rows.iter().map(|c| (c.passage_id.clone(), c.score)).collect();

    // Coverage analysis over distinct stemmed terms.
    let mut passage_texts: Vec<(String, Vec<String>)> = Vec::new();
    for cand in rows.iter().take(cfg.top_k) {
        let stemmed: String = store
            .conn
            .query_row(
                "SELECT index_text FROM passages_fts WHERE passage_id = ?1",
                rusqlite::params![cand.passage_id],
                |row| row.get(0),
            )
            .unwrap_or_default();
        passage_texts.push((cand.passage_id.clone(), stemmed.split_whitespace().map(String::from).collect()));
    }

    let term_present = |term: &str, tokens: &[String]| tokens.iter().any(|h| h == term);

    let total = terms.len() as f64;
    let mut union_matched = std::collections::BTreeSet::new();
    for (_, tokens) in &passage_texts {
        for t in &terms {
            if term_present(t, tokens) {
                union_matched.insert(t.clone());
            }
        }
    }
    let union_coverage = union_matched.len() as f64 / total;
    let top_coverage = passage_texts
        .first()
        .map(|(_, tokens)| terms.iter().filter(|t| term_present(t, tokens)).count() as f64 / total)
        .unwrap_or(0.0);
    debug.union_coverage = union_coverage;
    debug.top_coverage = top_coverage;

    // IDF-weighted coverage: rare query terms weigh more; terms absent from the
    // corpus entirely weigh the most and can never be covered. Missing-evidence
    // questions hinge on critical detail terms the corpus does not contain, so
    // their weighted coverage stays low.
    {
        let mut df: std::collections::HashMap<String, usize> = std::collections::HashMap::new();
        let mut npass = 0usize;
        if let Ok(mut stmt) = store.conn.prepare("SELECT index_text FROM passages_fts") {
            if let Ok(rows) = stmt.query_map([], |row| row.get::<_, String>(0)) {
                for row in rows.flatten() {
                    npass += 1;
                    let mut seen = std::collections::BTreeSet::new();
                    for t in row.split_whitespace() {
                        if seen.insert(t.to_string()) {
                            *df.entry(t.to_string()).or_insert(0) += 1;
                        }
                    }
                }
            }
        }
        let weight = |t: &str| -> f64 {
            let d = df.get(t).copied().unwrap_or(0) as f64;
            ((npass as f64 + 1.0) / (d + 0.5)).ln() + 1.0
        };
        let total_w: f64 = terms.iter().map(|t| weight(t)).sum();
        if total_w > 0.0 {
            let mut union_w = 0.0;
            let mut unmatched = Vec::new();
            for t in &terms {
                let hit_any = passage_texts.iter().any(|(_, tokens)| term_present(t, tokens));
                if hit_any {
                    union_w += weight(t);
                } else {
                    unmatched.push(t.clone());
                }
            }
            debug.unmatched_terms = unmatched;
            debug.idf_union_coverage = union_w / total_w;
            // Squared-weight variant: emphasizes rare critical terms further.
            let total_w2: f64 = terms.iter().map(|t| weight(t).powi(2)).sum();
            if total_w2 > 0.0 {
                let union_w2: f64 = terms.iter()
                    .filter(|t| passage_texts.iter().any(|(_, tokens)| term_present(t, tokens)))
                    .map(|t| weight(t).powi(2)).sum();
                debug.idf2_union_coverage = union_w2 / total_w2;
            }
            if let Some((_, tokens0)) = passage_texts.first() {
                let top_w: f64 = terms.iter().filter(|t| term_present(t, tokens0)).map(|t| weight(t)).sum();
                debug.idf_top_coverage = top_w / total_w;
            }
        }
    }

    let abstain = debug.idf_union_coverage < cfg.min_idf_union_coverage
        || union_coverage < cfg.min_union_coverage
        || top_coverage < cfg.min_top_coverage;
    if abstain {
        return (empty("insufficient_context"), debug);
    }

    let mut hits = Vec::new();
    for (i, (pid, _)) in passage_texts.iter().enumerate().take(cfg.top_k) {
        let row = store.conn.query_row(
            "SELECT p.revision, p.section, p.text, r.article_id
             FROM passages p JOIN revisions r ON p.revision = r.revision WHERE p.id = ?1",
            rusqlite::params![pid],
            |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, String>(1)?,
                    row.get::<_, String>(2)?,
                    row.get::<_, String>(3)?,
                ))
            },
        );
        if let Ok((revision, section, text, article_id)) = row {
            hits.push(PassageHit {
                rank: i as u32 + 1,
                passage_id: pid.clone(),
                revision,
                article_id,
                section,
                text,
                citation: citation::token_for(pid),
            });
        }
    }

    (SearchResponse {
        status: "ok".to_string(),
        query: query.to_string(),
        passages: hits,
    }, debug)
}
