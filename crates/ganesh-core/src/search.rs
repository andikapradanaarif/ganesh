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
    /// Experiment E (issue #169): action-aware positive support.
    pub action_check: bool,
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
            action_check: std::env::var("GANESH_ACTION_CHECK").map(|v| v == "1").unwrap_or(false),
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


// ---- Graded date-selection signal (M1 follow-up to #181/#184) ----
// All logic is pure and unit-tested below. Activation derives from the
// question text alone; the score is graded so a full date tied to every event
// term strictly outranks a bare year tied to one.

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum DatePrecision {
    Exact,
    YearOnly,
    Open,
}

/// Activation + precision from the question text alone.
pub fn date_activation(query: &str) -> Option<DatePrecision> {
    let q = query.to_lowercase();
    let activated = q.contains("when")
        || q.contains("what year")
        || q.contains("which year")
        || q.contains("what date")
        || q.contains("which date")
        || q.contains("what day")
        || q.contains("which day")
        || q.contains("on what");
    if !activated {
        return None;
    }
    if q.contains("exact") || q.contains("what date") || q.contains("which date") || q.contains("what day") || q.contains("which day") {
        Some(DatePrecision::Exact)
    } else if q.contains("what year") || q.contains("which year") {
        Some(DatePrecision::YearOnly)
    } else {
        Some(DatePrecision::Open)
    }
}

const MONTHS: [&str; 12] = [
    "january", "february", "march", "april", "may", "june", "july", "august", "september",
    "october", "november", "december",
];

/// Date tier of one sentence: 3 = full date (month+day+year or month+day),
/// 2 = month+year, 1 = bare 4-digit year, 0 = none.
pub fn date_tier(sentence: &str) -> u32 {
    let low = sentence.to_lowercase();
    let has_year = low
        .split(|c: char| !c.is_ascii_digit())
        .any(|w| w.len() == 4 && w >= "1000" && w <= "2099");
    let has_month = MONTHS.iter().any(|m| low.contains(m));
    let has_day = low
        .split(|c: char| !c.is_ascii_digit())
        .any(|w| {
            (w.len() == 1 || w.len() == 2)
                && w.parse::<u32>().map(|d| (1..=31).contains(&d)).unwrap_or(false)
        });
    if has_month && has_day {
        3
    } else if has_month && has_year {
        2
    } else if has_year {
        1
    } else {
        0
    }
}

/// Graded sentence score: date precision tier matched to the question,
/// weighted by the IDF sum of distinct non-subject event terms in the same
/// sentence, plus explicit subject-binding credit for distinguishing subject
/// tokens (numbers like 1/2/8/11 included). Returns 0.0 when the sentence
/// carries no usable signal.
pub fn sentence_date_score(
    sentence: &str,
    precision: DatePrecision,
    event_terms: &[String],
    subject_tokens: &[String],
    weight_of: &dyn Fn(&str) -> f64,
) -> f64 {
    let tier = date_tier(sentence);
    let tier_score = match precision {
        DatePrecision::Exact => match tier {
            3 => 3.0,
            _ => 0.0,
        },
        DatePrecision::YearOnly => match tier {
            3 => 3.0,
            2 => 2.0,
            1 => 2.0,
            _ => 0.0,
        },
        DatePrecision::Open => match tier {
            3 => 3.0,
            2 => 2.0,
            1 => 1.0,
            _ => 0.0,
        },
    };
    if tier_score == 0.0 {
        return 0.0;
    }
    let toks = stemmed_tokens(sentence);
    let event_weight: f64 = event_terms
        .iter()
        .filter(|t| toks.iter().any(|x| x == *t))
        .map(|t| weight_of(t))
        .sum();
    if event_weight <= 0.0 {
        return 0.0;
    }
    let subject_hits = subject_tokens
        .iter()
        .filter(|t| {
            let low = sentence.to_lowercase();
            low.split(|c: char| !c.is_ascii_alphanumeric()).any(|w| w == t.as_str())
        })
        .count() as f64;
    tier_score * event_weight * (1.0 + subject_hits)
}

/// Best sentence score of a passage body.
pub fn passage_date_score(
    body: &str,
    precision: DatePrecision,
    event_terms: &[String],
    subject_tokens: &[String],
    weight_of: &dyn Fn(&str) -> f64,
) -> f64 {
    body.split(|c| c == '.' || c == '\n')
        .map(|sent| sentence_date_score(sent, precision, event_terms, subject_tokens, weight_of))
        .fold(0.0, f64::max)
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

    // Graded date-aware selection (M1 follow-up). Ineligible queries execute
    // the exact #178 path below untouched. When activated, eligible candidates
    // receive a graded sentence-level score tying date precision, event-term
    // coverage and subject binding; the best positive-scoring candidate is
    // promoted — to slot 1 when the bm25 first pick itself carries no date
    // signal, otherwise to slot 2 — and every selected passage updates the
    // covered set before the marginal walk fills the remainder, protecting
    // compound questions. No positive score anywhere: pure #178 fallback.
    let mut promoted: Option<usize> = None;
    let precision = date_activation(query);
    let mut date_scores: Vec<f64> = vec![0.0; candidate_texts.len()];
    if let Some(prec) = precision {
        let mut subject_vocab: std::collections::BTreeSet<String> = std::collections::BTreeSet::new();
        let mut all_title_tokens: std::collections::BTreeSet<String> = std::collections::BTreeSet::new();
        if let Ok(mut stmt) = store.conn.prepare("SELECT article_id, title FROM sources") {
            if let Ok(source_rows) = stmt.query_map([], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))) {
                for row in source_rows.flatten() {
                    let (article_id, title) = row;
                    for tok in title.to_lowercase().split(|c: char| !c.is_ascii_alphanumeric()) {
                        if !tok.is_empty() {
                            all_title_tokens.insert(tok.to_string());
                        }
                    }
                    if mentioned_subjects.contains(&article_id) {
                        for t in title_subject_terms(&title) {
                            subject_vocab.insert(t);
                        }
                    }
                }
            }
        }
        let event_terms: Vec<String> = terms
            .iter()
            .filter(|t| !subject_vocab.contains(*t))
            .cloned()
            .collect();
        let subject_tokens: Vec<String> = query
            .to_lowercase()
            .split(|c: char| !c.is_ascii_alphanumeric())
            .filter(|w| !w.is_empty() && all_title_tokens.contains(*w))
            .map(|w| w.to_string())
            .collect();
        for (idx, cand) in rows.iter().enumerate() {
            let body: String = store
                .conn
                .query_row(
                    "SELECT text FROM passages WHERE id = ?1",
                    rusqlite::params![cand.passage_id],
                    |row| row.get(0),
                )
                .unwrap_or_default();
            date_scores[idx] = passage_date_score(&body, prec, &event_terms, &subject_tokens, &|t| weight_of(t));
        }
        if let Some((best_idx, best_score)) = eligible
            .iter()
            .map(|&i| (i, date_scores[i]))
            .filter(|(_, sc)| *sc > 0.0)
            .max_by(|a, b| a.1.partial_cmp(&b.1).unwrap_or(std::cmp::Ordering::Equal))
        {
            let _ = best_score;
            promoted = Some(best_idx);
        }
    }

    // Complementary selection: keep the best BM25 candidate, then add the
    // candidate with the largest weighted marginal coverage of query terms not
    // already covered. When no candidate adds a term, fill by original rank.
    // Final display order remains the original BM25 order of the selected set.
    let want = cfg.top_k.min(eligible.len());
    let mut selected: Vec<usize> = Vec::new();
    if want > 0 {
        let first = match promoted {
            // The promoted date-evidence passage takes slot 1 only when the
            // bm25 first pick carries no date signal of its own; otherwise it
            // inserts at slot 2 and the bm25 order is undisturbed.
            Some(idx) if date_scores[eligible[0]] <= 0.0 && idx != eligible[0] => {
                selected.push(idx);
                selected.push(eligible[0]);
                idx
            }
            Some(idx) if idx != eligible[0] => {
                selected.push(eligible[0]);
                if selected.len() < want && !selected.contains(&idx) {
                    selected.push(idx);
                }
                eligible[0]
            }
            _ => {
                selected.push(eligible[0]);
                eligible[0]
            }
        };
        let _ = first;
        let mut covered = std::collections::BTreeSet::new();
        for &sel_idx in &selected {
            for t in &terms {
                if term_present(t, &candidate_texts[sel_idx].1) {
                    covered.insert(t.clone());
                }
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
        action_check: cfg.action_check,
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

#[cfg(test)]
mod date_selection_tests {
    use super::*;

    fn w(_t: &str) -> f64 {
        1.0
    }

    #[test]
    fn open_when_prefers_full_date_over_bare_year() {
        let ev = vec!["launch".to_string()];
        let full = sentence_date_score("Curiosity was launched on November 26, 2011.", DatePrecision::Open, &ev, &[], &w);
        let bare = sentence_date_score("The rover launched in 2011.", DatePrecision::Open, &ev, &[], &w);
        assert!(full > bare, "full {} should exceed bare {}", full, bare);
        assert!(bare > 0.0);
    }

    #[test]
    fn multi_event_sentence_prefers_queried_event() {
        let ev = vec!["launch".to_string()];
        let launch = sentence_date_score("The launch occurred on July 16, 1969.", DatePrecision::Open, &ev, &[], &w);
        let landing = sentence_date_score("The landing occurred on July 20, 1969.", DatePrecision::Open, &ev, &[], &w);
        assert!(launch > 0.0);
        assert_eq!(landing, 0.0);
    }

    #[test]
    fn multi_term_event_specificity() {
        let ev = vec!["name".to_string(), "edwin".to_string(), "hubl".to_string()];
        let all = sentence_date_score("The telescope was named after Edwin Hubble in 1946.", DatePrecision::Open, &ev, &[], &w);
        let one = sentence_date_score("The telescope mirror was ground in 1946.", DatePrecision::Open, &ev, &[], &w);
        assert!(all > one, "all-terms {} should exceed generic {}", all, one);
    }

    #[test]
    fn numeric_subject_binding() {
        let ev = vec!["launch".to_string()];
        let subj = vec!["2".to_string()];
        let bound = sentence_date_score("Voyager 2 launch took place on August 20, 1977.", DatePrecision::Open, &ev, &subj, &w);
        let unbound = sentence_date_score("The launch took place on August 20, 1977.", DatePrecision::Open, &ev, &subj, &w);
        assert!(bound > unbound, "subject-bound {} should exceed unbound {}", bound, unbound);
    }

    #[test]
    fn ineligible_queries_do_not_activate() {
        assert_eq!(date_activation("Who commanded the mission?"), None);
        assert_eq!(date_activation("How much power does the rover produce?"), None);
        assert!(date_activation("When was the rover launched?").is_some());
        assert_eq!(date_activation("In what year did it orbit?"), Some(DatePrecision::YearOnly));
        assert_eq!(date_activation("On what exact date did it launch?"), Some(DatePrecision::Exact));
    }
}
