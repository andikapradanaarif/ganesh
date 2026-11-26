use crate::accept::{self, AcceptanceTrace};
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
    /// Experiment A (issue #163): quantity evidence must bind a digit literal
    /// or number word to a measured-property term in the same sentence.
    /// Default off reproduces the revised v2 reference behavior.
    pub quantity_binding: bool,
    /// Experiment B (issue #163): state/transition questions need cue-class,
    /// polarity and environment compatible evidence in one sentence.
    pub transition_check: bool,
    /// Experiment C (issue #165): proposition verdicts — explicit contradiction
    /// of the asked predicate also settles yes/no questions.
    pub proposition_check: bool,
    /// Experiment D (issue #167): dated-entry context — section headings and
    /// unambiguous single-subject antecedents augment continuation sentences.
    pub context_chain: bool,
}

impl Default for RetrievalConfig {
    fn default() -> Self {
        RetrievalConfig {
            top_k: 3,
            min_idf_union_coverage: 0.75,
            min_union_coverage: 0.0,
            min_top_coverage: 0.0,
            candidate_limit: 50,
            quantity_binding: std::env::var("GANESH_QUANTITY_BINDING").map(|v| v == "1").unwrap_or(false),
            transition_check: std::env::var("GANESH_TRANSITION_CHECK").map(|v| v == "1").unwrap_or(false),
            proposition_check: std::env::var("GANESH_PROPOSITION_CHECK").map(|v| v == "1").unwrap_or(false),
            context_chain: std::env::var("GANESH_CONTEXT_CHAIN").map(|v| v == "1").unwrap_or(false),
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

fn is_connector(ch: char) -> bool {
    matches!(ch, '.' | '#' | '+' | '_' | '-')
}

/// Additional canonical tokens for symbolic or punctuation-bearing
/// identifiers. The component tokens remain indexed as well; these aliases
/// make forms such as `C#`, `C Sharp`, `c++`, `Node.js`, and `.NET`
/// retrievable without depending on one exact surface punctuation style.
fn identifier_alias_tokens(text: &str) -> Vec<String> {
    let chars: Vec<char> = text.chars().collect();
    let mut aliases = Vec::new();
    let mut i = 0usize;
    while i < chars.len() {
        if !(chars[i].is_alphanumeric() || is_connector(chars[i])) {
            i += 1;
            continue;
        }
        let start = i;
        while i < chars.len() && (chars[i].is_alphanumeric() || is_connector(chars[i])) {
            i += 1;
        }
        let mut span: String = chars[start..i].iter().collect();
        while span.ends_with(['.', '_', '-']) {
            span.pop();
        }
        while span.starts_with(['.', '_', '-']) {
            span.remove(0);
        }
        if !span.chars().any(|c| c.is_alphabetic()) || !span.chars().any(is_connector) {
            continue;
        }
        let mut canonical = String::new();
        for ch in span.chars() {
            if ch.is_alphanumeric() {
                canonical.push(ch.to_ascii_lowercase());
            } else if ch == '#' {
                canonical.push_str("sharp");
            } else if ch == '+' {
                canonical.push_str("plus");
            }
        }
        if canonical.len() >= 2 {
            aliases.push(canonical);
        }
    }

    // Written-out forms separated by spaces: a single letter followed by
    // "sharp" or by "plus plus" denotes the same symbolic identifier.
    let toks = raw_tokens(text);
    for w in toks.windows(2) {
        if w[0].len() == 1 && w[0].chars().next().is_some_and(|c| c.is_alphabetic()) && w[1] == "sharp" {
            aliases.push(format!("{}sharp", w[0]));
        }
    }
    for w in toks.windows(3) {
        if w[0].len() == 1 && w[0].chars().next().is_some_and(|c| c.is_alphabetic()) && w[1] == "plus" && w[2] == "plus" {
            aliases.push(format!("{}plusplus", w[0]));
        }
    }
    aliases
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

/// Quantity units whose singular and plural forms carry different meanings in
/// this corpus (`byte` the type versus `bytes` the unit, for example). These
/// keep their exact surface number instead of being conflated by stemming.
fn exact_number_unit(token: &str) -> bool {
    matches!(token, "byte" | "bytes" | "bit" | "bits" | "kilobyte" | "kilobytes" | "megabyte" | "megabytes")
}

fn quantity_question(query: &str) -> bool {
    let q = query.to_ascii_lowercase();
    q.contains("how many") || q.contains("how much") || q.contains("number of")
}

fn numeric_token(token: &str) -> bool {
    !token.is_empty() && token.chars().all(|c| c.is_ascii_digit())
}

fn normalize_token(token: &str) -> String {
    if exact_number_unit(token) {
        token.to_string()
    } else {
        stem_token(token)
    }
}

/// Stemmed content tokens of a text, English Porter, stopwords removed.
/// The search index stores passage text in this normalized form; queries are
/// normalized the same way, so matching is morphologically tolerant while the
/// stored passages keep their verbatim bytes.
pub fn stemmed_tokens(text: &str) -> Vec<String> {
    raw_tokens(text)
        .into_iter()
        .chain(identifier_alias_tokens(text))
        .filter(|t| keep_token(t))
        .map(|t| normalize_token(&t))
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
    pub selected: Vec<String>,
    pub trace: AcceptanceTrace,
}

fn title_subject_terms(title: &str) -> std::collections::BTreeSet<String> {
    stemmed_tokens(title)
        .into_iter()
        .filter(|t| t != &stem_token("programming") && t != &stem_token("language"))
        .collect()
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
        selected: vec![],
        trace: AcceptanceTrace::default(),
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

    // Coverage analysis over distinct stemmed terms. The public response is
    // still top-k, but selection starts from a deeper candidate pool so a
    // passage covering a different facet of a compound question can be chosen
    // instead of a near-duplicate of the first hit.
    let mut candidate_texts: Vec<(String, Vec<String>, String)> = Vec::new();
    for cand in &rows {
        let stemmed: String = store
            .conn
            .query_row(
                "SELECT index_text FROM passages_fts WHERE passage_id = ?1",
                rusqlite::params![cand.passage_id],
                |row| row.get(0),
            )
            .unwrap_or_default();
        let article_id: String = store
            .conn
            .query_row(
                "SELECT r.article_id FROM passages p JOIN revisions r ON p.revision = r.revision WHERE p.id = ?1",
                rusqlite::params![cand.passage_id],
                |row| row.get(0),
            )
            .unwrap_or_default();
        candidate_texts.push((cand.passage_id.clone(), stemmed.split_whitespace().map(String::from).collect(), article_id));
    }

    // Source titles define the subject vocabulary known to the service. If the
    // query mentions exactly one subject, selection is restricted to passages
    // from that subject when any are present. Queries naming zero or multiple
    // subjects remain unrestricted, preserving legitimate comparisons.
    let mut mentioned_subjects = std::collections::BTreeSet::new();
    if let Ok(mut stmt) = store.conn.prepare("SELECT article_id, title FROM sources") {
        if let Ok(source_rows) = stmt.query_map([], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))) {
            for row in source_rows.flatten() {
                let (article_id, title) = row;
                let subject_terms = title_subject_terms(&title);
                if terms.iter().any(|t| subject_terms.contains(t)) {
                    mentioned_subjects.insert(article_id);
                }
            }
        }
    }
    let restricted_article = if mentioned_subjects.len() == 1 {
        mentioned_subjects.iter().next().cloned()
    } else {
        None
    };
    let eligible: Vec<usize> = match &restricted_article {
        Some(article) if candidate_texts.iter().any(|(_, _, a)| a == article) => {
            (0..candidate_texts.len()).filter(|idx| candidate_texts[*idx].2 == *article).collect()
        }
        _ => (0..candidate_texts.len()).collect(),
    };

    let term_present = |term: &str, tokens: &[String]| tokens.iter().any(|h| h == term);

    // Document-frequency weights for query terms, computed once per query.
    let mut term_weights: std::collections::HashMap<String, f64> = std::collections::HashMap::new();
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
        for t in &terms {
            let d = df.get(t).copied().unwrap_or(0) as f64;
            term_weights.insert(t.clone(), ((npass as f64 + 1.0) / (d + 0.5)).ln() + 1.0);
        }
    }
    let weight_of = |t: &str| term_weights.get(t).copied().unwrap_or(1.0);

    // Complementary selection: keep the best BM25 candidate, then add the
    // candidate with the largest weighted marginal coverage of query terms not
    // already covered. When no candidate adds a term, fill by original rank.
    // Final display order remains the original BM25 order of the selected set.
    let want = cfg.top_k.min(eligible.len());
    let mut selected: Vec<usize> = Vec::new();
    if want > 0 {
        selected.push(eligible[0]);
        let mut covered = std::collections::BTreeSet::new();
        for t in &terms {
            if term_present(t, &candidate_texts[eligible[0]].1) {
                covered.insert(t.clone());
            }
        }
        while selected.len() < want {
            let mut best: Option<(usize, f64)> = None;
            for &idx in eligible.iter().skip(1) {
                if selected.contains(&idx) {
                    continue;
                }
                let marginal: f64 = terms.iter()
                    .filter(|t| !covered.contains(*t) && term_present(t, &candidate_texts[idx].1))
                    .map(|t| weight_of(t))
                    .sum();
                if marginal > 0.0 && best.map(|(_, b)| marginal > b).unwrap_or(true) {
                    best = Some((idx, marginal));
                }
            }
            let next = match best {
                Some((idx, _)) => idx,
                None => *eligible.iter().find(|idx| !selected.contains(idx)).unwrap(),
            };
            for t in &terms {
                if term_present(t, &candidate_texts[next].1) {
                    covered.insert(t.clone());
                }
            }
            selected.push(next);
        }
        selected.sort_unstable();
    }
    let passage_texts: Vec<(String, Vec<String>)> = selected.iter()
        .map(|idx| (candidate_texts[*idx].0.clone(), candidate_texts[*idx].1.clone()))
        .collect();

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
        let weight = |t: &str| -> f64 { weight_of(t) };
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

    // Acceptance decision (issue #163): extracted into accept::decide so every
    // rule records the sentence span it relied on. Verbatim text of the
    // selected passages is loaded here for sentence-level evidence analysis;
    // ranking and selection above are unaffected.
    let mut verbatim_by_id: std::collections::HashMap<String, (String, String)> = std::collections::HashMap::new();
    for (pid, _) in &passage_texts {
        let row: (String, String) = store.conn.query_row(
            "SELECT text, section FROM passages WHERE id = ?1",
            rusqlite::params![pid],
            |row| Ok((row.get(0)?, row.get(1)?)),
        ).unwrap_or_default();
        verbatim_by_id.insert(pid.clone(), row);
    }
    let selected_full: Vec<(String, String, String, Vec<String>)> = passage_texts.iter()
        .map(|(pid, tokens)| {
            let (text, section) = verbatim_by_id.get(pid).cloned().unwrap_or_default();
            (pid.clone(), text, section, tokens.clone())
        })
        .collect();
    debug.selected = passage_texts.iter().map(|(pid, _)| pid.clone()).collect();

    // Query terms that name a source subject are not measured properties.
    let mut subject_query_terms = std::collections::BTreeSet::new();
    if let Ok(mut stmt) = store.conn.prepare("SELECT title FROM sources") {
        if let Ok(rows) = stmt.query_map([], |row| row.get::<_, String>(0)) {
            for title in rows.flatten() {
                for t in title_subject_terms(&title) {
                    if terms.contains(&t) {
                        subject_query_terms.insert(t);
                    }
                }
            }
        }
    }

    let quantity = quantity_question(query);
    let quantity_unit_missing = quantity && terms.iter().any(|t| exact_number_unit(t))
        && terms.iter().filter(|t| exact_number_unit(t)).any(|t| !passage_texts.iter().any(|(_, tokens)| term_present(t, tokens)));
    let any_numeric = passage_texts.iter().any(|(_, tokens)| tokens.iter().any(|t| numeric_token(t)));
    // Cue-aware coverage (experiment B): state/transition questions often use
    // cue vocabulary the evidence expresses with different words ("verified"
    // vs "deployed ... passed"). A question cue term counts as covered when a
    // class-compatible cue appears in the selected passages.
    if cfg.transition_check || cfg.proposition_check {
        if let Some(required) = accept::cue_class_of(&terms) {
            let all_tokens: Vec<String> = passage_texts.iter().flat_map(|(_, t)| t.clone()).collect();
            let mut adjusted = debug.unmatched_terms.clone();
            adjusted.retain(|t| {
                let is_cue = accept::VERIFY_CUES.contains(&t.as_str())
                    || accept::PROPOSE_CUES.contains(&t.as_str())
                    || accept::ATTEMPT_CUES.contains(&t.as_str())
                    || accept::TRANSITION_CUES.contains(&t.as_str());
                if !is_cue {
                    return true;
                }
                let class_of_stem = |stem: &str| -> Option<&'static str> {
                    if accept::VERIFY_CUES.contains(&stem) { Some("verify") }
                    else if accept::PROPOSE_CUES.contains(&stem) { Some("propose") }
                    else if accept::ATTEMPT_CUES.contains(&stem) { Some("attempt") }
                    else if accept::TRANSITION_CUES.contains(&stem) { Some("transition") }
                    else { None }
                };
                let compat = accept::VERIFY_CUES.iter().chain(accept::PROPOSE_CUES.iter()).chain(accept::ATTEMPT_CUES.iter()).chain(accept::TRANSITION_CUES.iter())
                    .any(|cue| all_tokens.iter().any(|tok| tok == cue) && class_of_stem(cue).map(|c| accept::class_satisfies(required, c)).unwrap_or(false));
                !compat
            });
            if adjusted.len() < debug.unmatched_terms.len() {
                let total_w: f64 = terms.iter().map(|t| weight_of(t)).sum();
                if total_w > 0.0 {
                    let union_w: f64 = terms.iter()
                        .filter(|t| !adjusted.contains(*t) && passage_texts.iter().any(|(_, tokens)| term_present(t, tokens)) || !adjusted.contains(*t) && accept::VERIFY_CUES.iter().chain(accept::PROPOSE_CUES.iter()).chain(accept::ATTEMPT_CUES.iter()).chain(accept::TRANSITION_CUES.iter()).any(|c| c == t))
                        .map(|t| weight_of(t))
                        .sum();
                    debug.idf_union_coverage = union_w / total_w;
                    debug.unmatched_terms = adjusted;
                }
            }
        }
    }

    let input = accept::DecideInput {
        query,
        terms: &terms,
        subject_terms: &subject_query_terms,
        passages: &selected_full,
        idf_union_coverage: debug.idf_union_coverage,
        union_coverage,
        top_coverage,
        min_idf_union_coverage: cfg.min_idf_union_coverage,
        min_union_coverage: cfg.min_union_coverage,
        min_top_coverage: cfg.min_top_coverage,
        quantity_question: quantity,
        quantity_unit_missing,
        any_numeric_token: any_numeric,
        quantity_binding: cfg.quantity_binding,
        transition_check: cfg.transition_check || cfg.proposition_check,
        proposition_check: cfg.proposition_check,
        context_chain: cfg.context_chain,
    };
    let trace = accept::decide(&input);
    let abstain = trace.decision != "ok";
    debug.trace = trace;
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn symbolic_identifier_aliases_are_generated_consistently() {
        for text in ["C#", "c#", "C Sharp", "c sharp"] {
            assert!(stemmed_tokens(text).contains(&stem_token("csharp")), "{text}");
        }
        for text in ["C++", "c++", "C plus plus"] {
            assert!(stemmed_tokens(text).contains(&stem_token("cplusplus")), "{text}");
        }
        assert!(stemmed_tokens("Node.js").contains(&stem_token("nodejs")));
        assert!(stemmed_tokens(".NET").contains(&stem_token("net")));
    }

    #[test]
    fn ordinary_words_and_versions_survive_identifier_aliasing() {
        let toks = stemmed_tokens("end-user version 1.18");
        assert!(toks.contains(&stem_token("end")));
        assert!(toks.contains(&stem_token("user")));
        assert!(toks.iter().any(|t| t == "1"));
        assert!(toks.iter().any(|t| t == "18"));
    }
}

#[cfg(test)]
mod quantity_unit_tests {
    use super::*;

    #[test]
    fn quantity_units_preserve_surface_number() {
        assert!(stemmed_tokens("bytes").contains(&"bytes".to_string()));
        assert!(stemmed_tokens("byte").contains(&"byte".to_string()));
        assert!(!stemmed_tokens("bytes").contains(&"byte".to_string()));
        assert!(!stemmed_tokens("byte").contains(&"bytes".to_string()));
    }
}
