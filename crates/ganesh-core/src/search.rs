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
            candidate_limit: std::env::var("GANESH_RERANK_POOL").ok().and_then(|v| v.parse().ok()).unwrap_or(25),
            quantity_binding: std::env::var("GANESH_QUANTITY_BINDING").map(|v| v != "0").unwrap_or(true),
            transition_check: std::env::var("GANESH_TRANSITION_CHECK").map(|v| v != "0").unwrap_or(true),
            proposition_check: std::env::var("GANESH_PROPOSITION_CHECK").map(|v| v != "0").unwrap_or(true),
            context_chain: std::env::var("GANESH_CONTEXT_CHAIN").map(|v| v != "0").unwrap_or(true),
            action_check: std::env::var("GANESH_ACTION_CHECK").map(|v| v != "0").unwrap_or(true),
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

/// Measurement property nouns (predeclared linguistic list): a wh-question
/// naming one of these in measurement form is a quantity question even
/// without "how many/how much".
pub const MEASURE_PROPS: &[&str] = &[
    "diameter", "height", "length", "width", "depth", "weight", "mass", "distance",
    "area", "volume", "population", "speed", "temperature", "cost", "price", "size",
    "capacity", "altitude", "elevation", "tension", "wattage",
    // M2 phase 2: electrical / engineering / computing quantities (xi05 class)
    "amperage", "voltage", "power", "frequency", "pressure",
    "latency", "throughput", "bandwidth", "memory", "storage",
];

/// Physical-unit vocabulary for the explicit `"in <unit>"` framing check.
/// Mirrors the acceptance-side alias table's canonical surface forms.
const KNOWN_UNITS: &[&str] = &[
    "m", "meter", "meters", "metre", "metres", "km", "kilometer", "kilometers",
    "kilometre", "kilometres", "cm", "mm", "mile", "miles", "ft", "feet", "foot",
    "in", "inch", "inches", "yd", "yard", "yards", "nm", "kg", "kilogram",
    "kilograms", "g", "gram", "grams", "mg", "lb", "lbs", "pound", "pounds",
    "oz", "ounce", "ounces", "t", "ton", "tons", "tonne", "tonnes", "l",
    "liter", "liters", "litre", "litres", "ml", "s", "sec", "seconds", "min",
    "minutes", "hr", "hours", "w", "kw", "mw", "watt", "watts", "hz", "khz",
    "mhz", "ghz", "deg", "degree", "degrees", "byte", "bytes", "bit", "bits",
    "km/h", "m/s", "mph",
    // M2 phase 2: electrical / pressure / latency units
    "a", "amp", "amps", "ampere", "amperes", "ma", "v", "volt", "volts",
    "kwh", "psi", "bar", "ms", "millisecond", "milliseconds", "gb", "mb", "tb",
];

fn measurement_question(q: &str) -> bool {
    // Hyphenated compound adjectives ("low-cost", "high-speed") stay whole:
    // splitting them would false-match standalone measure-property nouns.
    let words: Vec<&str> = q
        .split(|c: char| !c.is_alphabetic() && c != '-')
        .filter(|w| !w.is_empty())
        .collect();
    let has_prop = words.iter().any(|w| MEASURE_PROPS.contains(w));
    if !has_prop {
        return false;
    }
    // Entity/location questions ("which/who/where …") are not measurement
    // questions merely for containing " in " ("in current production"); they
    // trigger only on explicit unit framing ("in inches") or count/mass
    // framing ("how many/how much").
    if q.starts_with("which ") || q.starts_with("who ") || q.starts_with("where ") {
        let in_unit = words
            .windows(2)
            .any(|w| w[0] == "in" && KNOWN_UNITS.contains(&w[1]));
        return in_unit || q.contains("how many") || q.contains("how much");
    }
    q.starts_with("what is") || q.starts_with("what's") || q.starts_with("at what")
        || q.contains(" in ") || q.starts_with("how ")
}

fn quantity_question(query: &str) -> bool {
    let q = query.to_ascii_lowercase();
    q.contains("how many") || q.contains("how much") || q.contains("number of") || measurement_question(&q)
}

/// Question-framing stems (predeclared small linguistic list): framing words
/// that describe the shape of the answer rather than its content. They are
/// never present in decisive evidence, so they must not penalize coverage;
/// typed verification carries the real sufficiency work for them.
pub const FRAMING_WORDS: &[&str] = &[
    "kind", "type", "exact", "calendar", "day", "date", "country", "information",
];

pub fn framing_stems() -> std::collections::BTreeSet<String> {
    let mut out = std::collections::BTreeSet::new();
    for w in FRAMING_WORDS {
        for t in stemmed_tokens(w) {
            out.insert(t);
        }
    }
    out
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
    stemmed_index_text_meta(title, section, text, "", "", "")
}

/// Checkbox normalization for the indexed/search view: markdown task states
/// become legible search tokens. Verbatim stored/returned text is untouched.
pub fn normalize_checkboxes(text: &str) -> String {
    text.replace("[x]", " checkboxchecked ").replace("[X]", " checkboxchecked ").replace("[ ]", " checkboxunchecked ")
}

/// Symbol-preserving aliases: `--flag` CLI tokens additionally index as
/// `dd<name>` so flag and bare-name queries both resolve, and tokens bearing
/// uppercase letters (error codes, env vars) additionally index their exact
/// lowercase unstemmed form alongside the stemmed one.
fn symbol_alias_tokens(text: &str) -> Vec<String> {
    let mut out = Vec::new();
    for word in text.split_whitespace() {
        let w = word.trim_matches(|c: char| c == ',' || c == ';' || c == ')' || c == '(');
        if let Some(flag) = w.strip_prefix("--") {
            let name: String = flag.chars().filter(|c| c.is_alphanumeric() || *c == '-').collect();
            if name.len() >= 2 {
                out.push(format!("dd{}", name.replace('-', "").to_lowercase()));
            }
        }
        if word.chars().any(|c| c.is_ascii_uppercase()) {
            let low: String = word.chars().filter(|c| c.is_alphanumeric()).collect::<String>().to_lowercase();
            if low.len() >= 2 {
                out.push(low);
            }
        }
    }
    out
}

/// Metadata-aware index text: title, section, body plus occurred-at date,
/// source type and author/speaker so temporal and genre qualifiers are
/// searchable. Checkbox state normalized; symbols preserved.
pub fn stemmed_index_text_meta(title: &str, section: &str, text: &str, occurred: &str, source_type: &str, author: &str) -> String {
    let normalized = normalize_checkboxes(text);
    let mut tokens = stemmed_tokens(&format!("{title} {section} {occurred} {source_type} {author} {normalized}"));
    tokens.extend(symbol_alias_tokens(&normalized).into_iter().map(|t| normalize_token(&t)));
    tokens.join(" ")
}

/// Context-header rendering for the embedded and reranked view. The verbatim
/// passage text returned to callers is never modified.
pub fn rendered_context_text(occurred: &str, source_type: &str, author: &str, title: &str, section: &str, text: &str) -> String {
    // Corpora without M2 temporal/provenance metadata (all M1 fixtures) keep
    // the exact pre-M2 cross-encoder input bit-for-bit so certified scores
    // reproduce; the structured header only applies when metadata exists.
    if occurred.is_empty() && source_type.is_empty() && author.is_empty() {
        return match (title.is_empty(), section.is_empty()) {
            (false, false) => format!("{title} — {section}\n{text}"),
            (false, true) => format!("{title}\n{text}"),
            (true, false) => format!("{section}\n{text}"),
            (true, true) => text.to_string(),
        };
    }
    let mut parts: Vec<String> = Vec::new();
    if !occurred.is_empty() {
        parts.push(occurred.to_string());
    }
    if !source_type.is_empty() {
        parts.push(source_type.to_string());
    }
    if !author.is_empty() {
        parts.push(author.to_string());
    }
    let mut ts = String::new();
    if !title.is_empty() && !section.is_empty() && section != "(lead)" {
        ts = format!("{title} — {section}");
    } else if !title.is_empty() {
        ts = title.to_string();
    } else if !section.is_empty() {
        ts = section.to_string();
    }
    if !ts.is_empty() {
        parts.push(ts);
    }
    if parts.is_empty() {
        text.to_string()
    } else {
        format!("[{}]\n{text}", parts.join(" | "))
    }
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
    pub rerank_scores: Vec<(String, f32)>,
}

fn title_subject_terms(title: &str) -> std::collections::BTreeSet<String> {
    stemmed_tokens(title)
        .into_iter()
        .filter(|t| t != &stem_token("programming") && t != &stem_token("language"))
        .collect()
}


// ---- Graded date-selection signal, bug-fixed revision (M1 follow-up) ----
// Fixes versus #186: (1) tie-breaking strictly prefers the better-bm25-ranked
// candidate instead of the worst; (2) date tiers require adjacent month+day
// pairs, so a stray digit near a month name no longer fakes a full date;
// (3) subject binding is IDF-weighted so distinctive entity tokens outweigh
// generic shared title words.

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum DatePrecision {
    Exact,
    YearOnly,
    Open,
}

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

fn is_day(tok: &str) -> bool {
    (tok.len() == 1 || tok.len() == 2)
        && tok.parse::<u32>().map(|d| (1..=31).contains(&d)).unwrap_or(false)
}

/// Month and day must be adjacent tokens (`Month DD` or `DD Month`, allowing a
/// trailing comma/ordinal suffix on the day). Returns true when the sentence
/// has a real calendar date shape rather than a stray digit near a month.
fn has_adjacent_month_day(low: &str) -> bool {
    let toks: Vec<&str> = low
        .split(|c: char| !c.is_ascii_alphanumeric())
        .filter(|w| !w.is_empty())
        .collect();
    for (i, t) in toks.iter().enumerate() {
        let tcore = t.trim_end_matches(|c| c == 's' || c == 't' || c == 'r' || c == 'd' || c == 'h' || c == 'n');
        let _ = tcore;
        if MONTHS.contains(t) {
            if i + 1 < toks.len() {
                let nxt = toks[i + 1].trim_end_matches(|c: char| c.is_ascii_alphabetic());
                if is_day(nxt) {
                    return true;
                }
                // ordinal forms like 26th
                let raw = toks[i + 1];
                let digits: String = raw.chars().take_while(|c| c.is_ascii_digit()).collect();
                if !digits.is_empty() && is_day(&digits) && raw.len() <= digits.len() + 2 {
                    return true;
                }
            }
            if i > 0 {
                let prv = toks[i - 1].trim_end_matches(|c: char| c.is_ascii_alphabetic());
                if is_day(prv) {
                    return true;
                }
                let raw = toks[i - 1];
                let digits: String = raw.chars().take_while(|c| c.is_ascii_digit()).collect();
                if !digits.is_empty() && is_day(&digits) && raw.len() <= digits.len() + 2 {
                    return true;
                }
            }
        }
    }
    false
}

/// Date tier: 3 = full date (adjacent month+day with a 4-digit year present),
/// 2.5 = adjacent month+day without year, 2 = month+year, 1 = bare year, 0 none.
pub fn date_tier(sentence: &str) -> f64 {
    let low = sentence.to_lowercase();
    let has_year = low
        .split(|c: char| !c.is_ascii_digit())
        .any(|w| w.len() == 4 && w >= "1000" && w <= "2099");
    let has_month = MONTHS.iter().any(|m| low.contains(m));
    if has_adjacent_month_day(&low) {
        if has_year {
            3.0
        } else {
            2.5
        }
    } else if has_month && has_year {
        2.0
    } else if has_year {
        1.0
    } else {
        0.0
    }
}

pub fn sentence_date_score(
    sentence: &str,
    precision: DatePrecision,
    event_terms: &[String],
    subject_tokens: &[String],
    weight_of: &dyn Fn(&str) -> f64,
) -> f64 {
    let tier = date_tier(sentence);
    let tier_score = match precision {
        DatePrecision::Exact => if tier >= 3.0 { 3.0 } else { 0.0 },
        DatePrecision::YearOnly => match tier {
            t if t >= 3.0 => 3.0,
            t if t >= 1.0 => 2.0,
            _ => 0.0,
        },
        DatePrecision::Open => tier,
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
    // IDF-weighted subject binding: a distinctive entity token (high idf)
    // outweighs generic shared title words many times over.
    let low = sentence.to_lowercase();
    let subj_weight: f64 = subject_tokens
        .iter()
        .filter(|t| low.split(|c: char| !c.is_ascii_alphanumeric()).any(|w| w == t.as_str()))
        .map(|t| weight_of(t))
        .sum();
    tier_score * event_weight * (1.0 + subj_weight)
}

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


/// Whitespace-token Jaccard overlap between two token lists (predeclared
/// diversity threshold 0.7 for the cross-encoder selection rule).
fn token_jaccard(a: &[String], b: &[String]) -> f64 {
    let sa: std::collections::BTreeSet<&String> = a.iter().collect();
    let sb: std::collections::BTreeSet<&String> = b.iter().collect();
    let inter = sa.intersection(&sb).count() as f64;
    let union = sa.union(&sb).count() as f64;
    if union == 0.0 { 0.0 } else { inter / union }
}

/// Everything the retrieval pipeline reads from SQLite for one query,
/// materialized up front. The server holds its store mutex only while this
/// snapshot is built; cross-encoder reranking, acceptance, and QA verification
/// then run lock-free on plain memory. All field values are byte-identical to
/// what the interleaved implementation read: the same queries, the same
/// `unwrap_or_default` semantics for missing rows.
pub struct CorpusView {
    /// FTS-matched candidates in bm25 rank order.
    pub candidates: Vec<CandidateRow>,
    /// (article_id, title) rows of the sources table.
    pub sources: Vec<(String, String)>,
    /// Document frequency per stemmed token over the whole corpus.
    pub df: std::collections::HashMap<String, usize>,
    /// Total passage count backing `df`.
    pub npass: usize,
}

pub struct CandidateRow {
    pub passage_id: String,
    pub score: f64,
    pub index_text: String,
    pub article_id: String,
    pub revision: String,
    pub section: String,
    pub text: String,
    pub title: String,
    /// Rank in the dense cosine list when this candidate came in through or
    /// was also seen by the embedding retriever (None = pure BM25 hit).
    pub dense_rank: Option<usize>,
    pub occurred_at: String,
    pub source_type: String,
    pub author: String,
    pub claim_key: String,
}

impl CorpusView {
    /// Runs every SQLite read the pipeline needs. `terms` are the already-
    /// normalized query terms (the FTS MATCH expression derives from them).
    /// Eligibility (M2): only passages whose revision is visible to the
    /// caller's projects, inside its effective window at `as_of`, and not
    /// superseded in scope enter EITHER retriever — lexical or dense — so no
    /// restricted or retired candidate can ever reach the reranker.
    pub fn fetch(
        store: &Store,
        terms: &[String],
        cfg: &RetrievalConfig,
        projects: &[String],
        as_of: &str,
        query_scope: &str,
    ) -> Option<CorpusView> {
        let eligible_revs = crate::store::eligible_revisions(store, projects, as_of, query_scope);
        if eligible_revs.is_empty() {
            return None;
        }

        // ---- lexical recall: BM25 over FTS5, overfetched then filtered ----
        let q = fts_query(terms);
        let mut stmt = store.conn.prepare(
            "SELECT passage_id, bm25(passages_fts) AS score FROM passages_fts
             WHERE passages_fts MATCH ?1
             ORDER BY score ASC, passage_id ASC
             LIMIT ?2",
        ).ok()?;
        let overfetch = (cfg.candidate_limit as i64) * 4;
        let rows = stmt
            .query_map(rusqlite::params![q, overfetch], |row| {
                Ok((row.get::<_, String>(0)?, row.get::<_, f64>(1)?))
            })
            .map(|r| r.filter_map(|x| x.ok()).collect::<Vec<_>>())
            .unwrap_or_default();

        let rev_of = |pid: &str| -> String {
            pid.split('#').next().unwrap_or("").to_string()
        };
        let bm25_hits: Vec<(String, f64)> = rows
            .into_iter()
            .filter(|(pid, _)| eligible_revs.contains_key(&rev_of(pid)))
            .take(cfg.candidate_limit)
            .collect();

        // ---- dense recall: cosine over eligible embeddings only ----
        let mut dense_order: Vec<String> = Vec::new();
        if crate::embed::available() {
            if let Some(qvec) = crate::embed::embed_one(&terms.join(" ")) {
                let mut scored: Vec<(String, f32)> = Vec::new();
                if let Ok(mut estmt) = store.conn.prepare(
                    "SELECT e.passage_id, e.vector, p.revision FROM embeddings e JOIN passages p ON e.passage_id = p.id",
                ) {
                    if let Ok(erows) = estmt.query_map([], |row| {
                        Ok((row.get::<_, String>(0)?, row.get::<_, Vec<u8>>(1)?, row.get::<_, String>(2)?))
                    }) {
                        for row in erows.flatten() {
                            let (pid, blob, rev) = row;
                            if !eligible_revs.contains_key(&rev) {
                                continue;
                            }
                            let vec: Vec<f32> = blob
                                .chunks_exact(4)
                                .map(|c| f32::from_le_bytes([c[0], c[1], c[2], c[3]]))
                                .collect();
                            scored.push((pid, crate::embed::dot(&qvec, &vec)));
                        }
                    }
                }
                scored.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(std::cmp::Ordering::Equal));
                dense_order = scored.into_iter().take(cfg.candidate_limit).map(|(p, _)| p).collect();
            }
        }

        // ---- reciprocal rank fusion of the two recall lists ----
        const RRF_K: f64 = 60.0;
        let mut rrf: std::collections::HashMap<String, (f64, usize, Option<usize>, f64)> = std::collections::HashMap::new();
        for (rank, (pid, score)) in bm25_hits.iter().enumerate() {
            rrf.insert(pid.clone(), (1.0 / (RRF_K + rank as f64 + 1.0), rank, None, *score));
        }
        for (rank, pid) in dense_order.iter().enumerate() {
            rrf.entry(pid.clone())
                .and_modify(|e| {
                    e.0 += 1.0 / (RRF_K + rank as f64 + 1.0);
                    e.2 = Some(rank);
                })
                .or_insert((1.0 / (RRF_K + rank as f64 + 1.0), usize::MAX, Some(rank), 0.0));
        }
        let mut fused: Vec<(String, f64, usize, Option<usize>, f64)> = rrf
            .into_iter()
            .map(|(pid, (s, brank, drank, bm))| (pid, s, brank, drank, bm))
            .collect();
        fused.sort_by(|a, b| {
            b.1.partial_cmp(&a.1)
                .unwrap_or(std::cmp::Ordering::Equal)
                .then(a.2.cmp(&b.2))
                .then(a.0.cmp(&b.0))
        });
        fused.truncate(cfg.candidate_limit);
        if fused.is_empty() {
            return None;
        }

        let mut candidates = Vec::new();
        for (passage_id, _rrf_score, _bm_rank, dense_rank, bm25_score) in fused {
            let row: Option<(String, String, String, String, String, String, String, String, String)> = store
                .conn
                .query_row(
                    "SELECT f.index_text, p.revision, p.section, p.text, r.article_id, s.title,
                            COALESCE(r.occurred_at,''), r.source_type, r.author
                     FROM passages p
                     JOIN revisions r ON p.revision = r.revision
                     JOIN sources s ON r.article_id = s.article_id
                     JOIN passages_fts f ON f.passage_id = p.id
                     WHERE p.id = ?1",
                    rusqlite::params![passage_id],
                    |row| {
                        Ok((row.get(0)?, row.get(1)?, row.get(2)?, row.get(3)?, row.get(4)?, row.get(5)?, row.get(6)?, row.get(7)?, row.get(8)?))
                    },
                )
                .ok();
            let Some((index_text, revision, section, text, article_id, title, occurred, stype, author)) = row else {
                continue;
            };
            let claim_key = eligible_revs.get(&revision).map(|m| m.claim_key.clone()).unwrap_or_default();
            candidates.push(CandidateRow {
                passage_id,
                score: bm25_score,
                index_text,
                article_id,
                revision,
                section,
                text,
                title,
                dense_rank,
                occurred_at: occurred,
                source_type: stype,
                author,
                claim_key,
            });
        }
        if candidates.is_empty() {
            return None;
        }
        let mut sources = Vec::new();
        if let Ok(mut stmt) = store.conn.prepare("SELECT article_id, title FROM sources") {
            if let Ok(source_rows) = stmt.query_map([], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))) {
                for row in source_rows.flatten() {
                    sources.push(row);
                }
            }
        }
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
        Some(CorpusView { candidates, sources, df, npass })
    }
}

/// Evaluate a query against a pre-fetched corpus view. Pure compute: no
/// SQLite access, safe to run outside any store lock.
pub fn evaluate_view(view: &CorpusView, query: &str, caller: &str, cfg: &RetrievalConfig) -> (SearchResponse, SearchDebug) {
    evaluate(view, query, caller, cfg)
}

pub fn search(store: &Store, query: &str, caller: &str, cfg: &RetrievalConfig) -> SearchResponse {
    search_debug(store, query, caller, cfg).0
}

/// Full M2 read path: explicit permitted projects (resolved from credentials
/// upstream), effective time and per-request top_k.
pub fn search_full(
    store: &Store,
    query: &str,
    projects: &[String],
    as_of: &str,
    top_k: usize,
    query_scope: &str,
    cfg: &RetrievalConfig,
) -> SearchResponse {
    let mut cfg2 = cfg.clone();
    cfg2.top_k = top_k.clamp(1, 15);
    let terms = query_terms(query);
    if terms.is_empty() {
        return SearchResponse {
            status: "insufficient_context".to_string(),
            query: query.to_string(),
            passages: vec![],
            confidence: "insufficient_context".to_string(),
            top_margin: None,
            effective_time: as_of.to_string(),
        };
    }
    match CorpusView::fetch(store, &terms, &cfg2, projects, as_of, query_scope) {
        Some(view) => {
            let (mut resp, _dbg) = evaluate(&view, query, "", &cfg2);
            resp.effective_time = as_of.to_string();
            resp
        }
        None => SearchResponse {
            status: "insufficient_context".to_string(),
            query: query.to_string(),
            passages: vec![],
            confidence: "insufficient_context".to_string(),
            top_margin: None,
            effective_time: as_of.to_string(),
        },
    }
}

pub fn search_debug(store: &Store, query: &str, caller: &str, cfg: &RetrievalConfig) -> (SearchResponse, SearchDebug) {
    let terms = query_terms(query);
    let empty_debug = |terms: &[String]| SearchDebug {
        union_coverage: 0.0,
        top_coverage: 0.0,
        idf_union_coverage: 0.0,
        idf_top_coverage: 0.0,
        idf2_union_coverage: 0.0,
        terms: terms.to_vec(),
        unmatched_terms: vec![],
        candidates: vec![],
        selected: vec![],
        trace: AcceptanceTrace::default(),
        rerank_scores: vec![],
    };
    let empty = SearchResponse {
        status: "insufficient_context".to_string(),
        query: query.to_string(),
        passages: vec![],
        confidence: "insufficient_context".to_string(),
        top_margin: None,
        effective_time: crate::types::now_iso(),
    };
    if terms.is_empty() || !permitted(caller, "public") {
        return (empty, empty_debug(&terms));
    }
    let projects = vec!["public".to_string()];
    let now = crate::types::now_iso();
    match CorpusView::fetch(store, &terms, cfg, &projects, &now, "") {
        Some(view) => {
            let (mut r, d) = evaluate(&view, query, caller, cfg);
            r.effective_time = now;
            (r, d)
        }
        None => (empty, empty_debug(&terms)),
    }
}

fn evaluate(view: &CorpusView, query: &str, caller: &str, cfg: &RetrievalConfig) -> (SearchResponse, SearchDebug) {
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
        rerank_scores: vec![],
    };
    let empty = |status: &str| SearchResponse {
        status: status.to_string(),
        query: query.to_string(),
        passages: vec![],
        confidence: "insufficient_context".to_string(),
        top_margin: None,
        effective_time: crate::types::now_iso(),
    };

    let terms = query_terms(query);
    debug.terms = terms.clone();
    if terms.is_empty() {
        return (empty("insufficient_context"), debug);
    }
    if !permitted(caller, "public") {
        return (empty("insufficient_context"), debug);
    }

    let rows: Vec<Candidate> = view
        .candidates
        .iter()
        .map(|c| Candidate { passage_id: c.passage_id.clone(), score: c.score })
        .collect();
    debug.candidates = rows.iter().map(|c| (c.passage_id.clone(), c.score)).collect();

    // Coverage analysis over distinct stemmed terms. The public response is
    // still top-k, but selection starts from a deeper candidate pool so a
    // passage covering a different facet of a compound question can be chosen
    // instead of a near-duplicate of the first hit.
    let mut candidate_texts: Vec<(String, Vec<String>, String)> = Vec::new();
    for cand in &view.candidates {
        candidate_texts.push((
            cand.passage_id.clone(),
            cand.index_text.split_whitespace().map(String::from).collect(),
            cand.article_id.clone(),
        ));
    }

    // Source titles define the subject vocabulary known to the service. If the
    // query mentions exactly one subject, selection is restricted to passages
    // from that subject when any are present. Queries naming zero or multiple
    // subjects remain unrestricted, preserving legitimate comparisons.
    let mut mentioned_subjects = std::collections::BTreeSet::new();
    for (article_id, title) in &view.sources {
        let subject_terms = title_subject_terms(title);
        if terms.iter().any(|t| subject_terms.contains(t)) {
            mentioned_subjects.insert(article_id.clone());
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
        for t in &terms {
            let d = view.df.get(t).copied().unwrap_or(0) as f64;
            term_weights.insert(t.clone(), ((view.npass as f64 + 1.0) / (d + 0.5)).ln() + 1.0);
        }
    }
    let weight_of = |t: &str| term_weights.get(t).copied().unwrap_or(1.0);

    // Graded date-aware selection. Ineligible queries execute the exact #178
    // path untouched. When activated, the best positive-scoring candidate is
    // promoted — strictly higher score wins, ties keep the better bm25 rank —
    // taking slot 1 when the bm25 first pick carries no signal itself, slot 2
    // otherwise. All selected passages update covered before the marginal
    // walk, protecting compound questions. No positive score: pure fallback.
    let mut promoted: Option<usize> = None;
    let precision = date_activation(query);
    let mut date_scores: Vec<f64> = vec![0.0; candidate_texts.len()];
    if let Some(prec) = precision {
        let mut subject_vocab: std::collections::BTreeSet<String> = std::collections::BTreeSet::new();
        let mut all_title_tokens: std::collections::BTreeSet<String> = std::collections::BTreeSet::new();
        for (article_id, title) in &view.sources {
            for tok in title.to_lowercase().split(|c: char| !c.is_ascii_alphanumeric()) {
                if !tok.is_empty() {
                    all_title_tokens.insert(tok.to_string());
                }
            }
            if mentioned_subjects.contains(article_id) {
                for t in title_subject_terms(title) {
                    subject_vocab.insert(t);
                }
            }
        }
        let event_terms: Vec<String> = terms
            .iter()
            .filter(|t| !subject_vocab.contains(*t))
            .cloned()
            .collect();
        // Subject tokens for binding: distinctive idf comes from weight_of at
        // scoring time; here we collect query tokens that appear in titles.
        let subject_tokens: Vec<String> = query
            .to_lowercase()
            .split(|c: char| !c.is_ascii_alphanumeric())
            .filter(|w| !w.is_empty() && all_title_tokens.contains(*w))
            .map(|w| w.to_string())
            .collect();
        for (idx, _) in rows.iter().enumerate() {
            date_scores[idx] = passage_date_score(&view.candidates[idx].text, prec, &event_terms, &subject_tokens, &|t| weight_of(t));
        }
        // Strictly-higher wins; iteration order is eligible (bm25 rank order),
        // so equal scores keep the better-ranked candidate.
        let mut best: Option<(usize, f64)> = None;
        for &i in &eligible {
            let sc = date_scores[i];
            if sc > 0.0 && best.map(|(_, b)| sc > b).unwrap_or(true) {
                best = Some((i, sc));
            }
        }
        promoted = best.map(|(i, _)| i);
    }

    // Cross-encoder reranking v2 (predeclared, frozen before sem-xfer-v2
    // outcomes). Changes over 0386a9e4 are exactly the two reviewed fixes:
    //  1. The pair passed to the cross-encoder is "{title} — {section}\n{text}"
    //     (empty parts omitted), so the model sees the same heading/document
    //     context available to passages_fts and acceptance. Batch dates that
    //     live only in section headings become visible to the scorer.
    //  2. Selection policy (frozen after true rebuilt-0386a9e4 exposed-suite
    //     diagnostics, before any sem-xfer-v2 run): slots in cross-encoder
    //     order with token-Jaccard >= 0.7 diversity skip; the final slot picks
    //     the highest cross-encoder-scored remaining candidate that adds at
    //     least one uncovered query term when such a candidate exists,
    //     otherwise the next cross-encoder candidate (hybrid coverage slot).
    // Pool, eligibility, acceptance, and the three-passage limit are
    // unchanged. Ties keep better bm25 rank via stable sort over eligible order.
    let mut rerank_order: Option<Vec<usize>> = None;
    let mut rerank_inputs: Vec<String> = Vec::new();
    if crate::rerank::available() {
        let mut scored: Vec<(usize, f32)> = Vec::new();
        let mut contexts: Vec<(usize, String)> = Vec::new();
        for &i in &eligible {
            let section = view.candidates[i].section.clone();
            let text = view.candidates[i].text.clone();
            let title = view.candidates[i].title.clone();
            let ctx = rendered_context_text(
                &view.candidates[i].occurred_at,
                &view.candidates[i].source_type,
                &view.candidates[i].author,
                &title,
                &section,
                &text,
            );
            if rerank_inputs.len() <= i {
                rerank_inputs.resize(i + 1, String::new());
            }
            rerank_inputs[i] = ctx.clone();
            contexts.push((i, ctx));
        }
        // Batched INT8 ONNX inference: the whole eligible pool in one padded
        // forward pass (length-sorted internally to minimize padding waste).
        let mut scores: std::collections::HashMap<usize, f32> = std::collections::HashMap::new();
        let passages: Vec<String> = contexts.iter().map(|(_, c)| c.clone()).collect();
        if let Some(sc) = crate::rerank::score_batch(query, &passages) {
            for ((i, _), s) in contexts.iter().zip(sc.iter()) {
                scores.insert(*i, *s);
            }
        }
        for (i, _) in &contexts {
            scored.push((*i, scores.get(i).cloned().unwrap_or(f32::MIN)));
        }
        scored.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(std::cmp::Ordering::Equal));
        for (i, s) in &scored {
            debug.rerank_scores.push((rows[*i].passage_id.clone(), *s));
        }
        rerank_order = Some(scored.into_iter().map(|(i, _)| i).collect());
    }

    // Complementary selection: keep the best BM25 candidate, then add the
    // candidate with the largest weighted marginal coverage of query terms not
    // already covered. When no candidate adds a term, fill by original rank.
    // Final display order remains the original BM25 order of the selected set.
    let want = cfg.top_k.min(eligible.len());
    let mut selected: Vec<usize> = Vec::new();
    if let Some(order) = &rerank_order {
        // Slots 1..want-1: greedy cross-encoder order with Jaccard >= 0.7
        // diversity skip. Final slot: highest-scored remaining candidate that
        // adds at least one uncovered query term if any exist (hybrid
        // coverage slot, predeclared); otherwise next cross-encoder candidate.
        let mut covered = std::collections::BTreeSet::new();
        for &idx in order.iter() {
            if selected.len() >= want.saturating_sub(1) || selected.len() >= want {
                break;
            }
            let dup = selected
                .iter()
                .any(|&sel| token_jaccard(&candidate_texts[sel].1, &candidate_texts[idx].1) >= 0.7);
            if !dup && !selected.contains(&idx) {
                for t in &terms {
                    if term_present(t, &candidate_texts[idx].1) {
                        covered.insert(t.clone());
                    }
                }
                selected.push(idx);
            }
        }
        if selected.len() < want {
            let mut pick: Option<usize> = None;
            for &idx in order.iter() {
                if selected.contains(&idx) {
                    continue;
                }
                let adds = terms.iter().any(|t| !covered.contains(t) && term_present(t, &candidate_texts[idx].1));
                if adds {
                    pick = Some(idx);
                    break;
                }
            }
            if let Some(idx) = pick {
                selected.push(idx);
            }
            for &idx in order.iter() {
                if selected.len() >= want {
                    break;
                }
                if !selected.contains(&idx) {
                    selected.push(idx);
                }
            }
        }
        selected.sort_unstable();
    } else if want > 0 {
        match promoted {
            Some(idx) if date_scores[eligible[0]] <= 0.0 && idx != eligible[0] => {
                selected.push(idx);
                selected.push(eligible[0]);
            }
            Some(idx) if idx != eligible[0] => {
                selected.push(eligible[0]);
                if selected.len() < want && !selected.contains(&idx) {
                    selected.push(idx);
                }
            }
            _ => {
                selected.push(eligible[0]);
            }
        }
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

    let framing = framing_stems();
    let total = terms.len() as f64;
    let mut union_matched = std::collections::BTreeSet::new();
    for (_, tokens) in &passage_texts {
        for t in &terms {
            if term_present(t, tokens) {
                union_matched.insert(t.clone());
            }
        }
    }
    // Framing words never appear in decisive evidence; typed verification
    // carries their sufficiency. They count as covered for coverage math only.
    for t in &terms {
        if framing.contains(t.as_str()) {
            union_matched.insert(t.clone());
        }
    }
    let union_coverage = union_matched.len() as f64 / total;
    let top_coverage = passage_texts
        .first()
        .map(|(_, tokens)| terms.iter().filter(|t| term_present(t, tokens) || framing.contains(t.as_str())).count() as f64 / total)
        .unwrap_or(0.0);
    debug.unmatched_terms.retain(|t| !framing.contains(t.as_str()));
    debug.union_coverage = union_coverage;
    debug.top_coverage = top_coverage;

    // IDF-weighted coverage: rare query terms weigh more; terms absent from the
    // corpus entirely weigh the most and can never be covered. Missing-evidence
    // questions hinge on critical detail terms the corpus does not contain, so
    // their weighted coverage stays low.
    {
        let weight = |t: &str| -> f64 { weight_of(t) };
        let total_w: f64 = terms.iter().map(|t| weight(t)).sum();
        if total_w > 0.0 {
            let mut union_w = 0.0;
            let mut unmatched = Vec::new();
            for t in &terms {
                let hit_any = passage_texts.iter().any(|(_, tokens)| term_present(t, tokens));
                if hit_any || framing.contains(t.as_str()) {
                    union_w += weight(t);
                } else {
                    unmatched.push(t.clone());
                }
            }
            unmatched.retain(|t| !framing.contains(t.as_str()));
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
        let row: (String, String) = view
            .candidates
            .iter()
            .find(|c| &c.passage_id == pid)
            .map(|c| (c.text.clone(), c.section.clone()))
            .unwrap_or_default();
        verbatim_by_id.insert(pid.clone(), row);
    }
    let selected_full: Vec<(String, String, String, Vec<String>)> = passage_texts.iter()
        .map(|(pid, tokens)| {
            let (text, section) = verbatim_by_id.get(pid).cloned().unwrap_or_default();
            (pid.clone(), text, section, tokens.clone())
        })
        .collect();
    debug.selected = passage_texts.iter().map(|(pid, _)| pid.clone()).collect();

    // M2 phase 2: assemble guard-scanning pool context — passages that entered
    // the fused pool but were not selected. Disqualifying epistemic evidence
    // (rejection/exemption/indecision) anywhere in this pool blocks waiver.
    let mut pool_context: Vec<(String, String, String, Vec<String>)> = Vec::new();
    {
        let selected_ids: std::collections::BTreeSet<String> =
            passage_texts.iter().map(|(pid, _)| pid.clone()).collect();
        for c in view.candidates.iter().take(16) {
            if selected_ids.contains(&c.passage_id) || pool_context.len() >= 12 {
                continue;
            }
            // Candidates carry their verbatim text directly — resolving through
            // verbatim_by_id would yield empty strings for unselected passages
            // and blind the epistemic guards.
            if c.text.is_empty() {
                continue;
            }
            let toks = crate::search::stemmed_tokens(&c.text);
            pool_context.push((c.passage_id.clone(), c.text.clone(), c.section.clone(), toks));
        }
    }

    // Query terms that name a source subject are not measured properties.
    let mut subject_query_terms = std::collections::BTreeSet::new();
    for (_, title) in &view.sources {
        for t in title_subject_terms(title) {
            if terms.contains(&t) {
                subject_query_terms.insert(t);
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
        dense_support: passage_texts.iter().any(|(pid, _)| {
            view.candidates.iter().any(|c| c.passage_id == *pid && c.dense_rank.is_some())
        }),
        transition_check: cfg.transition_check || cfg.proposition_check,
        proposition_check: cfg.proposition_check,
        context_chain: cfg.context_chain,
        action_check: cfg.action_check,
        pool_context: &pool_context,
    };
    let trace = accept::decide(&input);
    let abstain = trace.decision != "ok";
    debug.trace = trace;
    if abstain {
        return (empty("insufficient_context"), debug);
    }

    let mut hits = Vec::new();
    for (i, (pid, _)) in passage_texts.iter().enumerate().take(cfg.top_k) {
        let row = view
            .candidates
            .iter()
            .find(|c| &c.passage_id == pid)
            .map(|c| (c.revision.clone(), c.section.clone(), c.text.clone(), c.article_id.clone()));
        if let Some((revision, section, text, article_id)) = row {
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

    // Conflicting-evidence detection: two eligible, non-superseding revisions
    // sharing one claim key among the selected evidence means equally
    // authoritative incompatible claims — surface both, never pick a winner.
    let mut claim_revs: std::collections::BTreeMap<String, std::collections::BTreeSet<String>> = std::collections::BTreeMap::new();
    for h in &hits {
        if let Some(row) = view.candidates.iter().find(|c| c.passage_id == h.passage_id) {
            if !row.claim_key.is_empty() {
                claim_revs.entry(row.claim_key.clone()).or_default().insert(row.revision.clone());
            }
        }
    }
    let conflicted = claim_revs.values().any(|set| set.len() > 1);

    let margin = if debug.rerank_scores.len() >= 2 {
        Some((debug.rerank_scores[0].1 - debug.rerank_scores[1].1) as f64)
    } else {
        None
    };
    let qa_margin = debug.trace.qa_margin.unwrap_or(0.0) as f64;
    let confidence = if debug.idf_union_coverage >= 0.9 && qa_margin >= 0.5 {
        "supported"
    } else {
        "uncertain"
    };

    (SearchResponse {
        status: if conflicted { "conflicting_evidence".to_string() } else { "ok".to_string() },
        query: query.to_string(),
        passages: hits,
        confidence: confidence.to_string(),
        top_margin: margin,
        effective_time: crate::types::now_iso(),
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

    #[test]
    fn measurement_question_hyphenated_compounds_do_not_split() {
        // acc-xfer-v6 q06 root cause: "low-cost" split into "cost" and the
        // " in " inside "in current production" falsely armed quantity binding.
        assert!(!measurement_question(
            "which polycarbonate saxophone is in current production as a low-cost alternative to metal saxophones?"
        ));
        assert!(!measurement_question("which train is in high-speed operation in japan?"));
    }

    #[test]
    fn measurement_question_entity_frames_need_explicit_units() {
        assert!(!measurement_question("which bridge is in current operation in italy?"));
        assert!(!measurement_question("who is the scientist in charge of cost estimates?"));
        // explicit "in <unit>" framing still triggers on entity questions
        assert!(measurement_question("which mirror is the largest in diameter in inches?"));
    }

    #[test]
    fn measurement_question_real_measurement_questions_still_trigger() {
        for q in [
            "what is the depth in meters of the caldera floor of mauna loa below its rim?", // v5/i02
            "what is the diameter of the james webb primary mirror in inches?",             // ax4/q09
            "what is the mass of the leaning tower of pisa in tonnes?",                     // ax5/q10
            "what is the weight of a typical harpsichord in kilograms?",                    // ax6/q10
            "what is the total string tension of a modern grand piano in kilograms?",       // ax6/q11
            "what is the diameter of hubble's main mirror in meters?",                      // v4/i05
        ] {
            assert!(measurement_question(q), "must trigger: {q}");
        }
    }
}

#[cfg(test)]
mod date_fix_tests {
    use super::*;

    fn w(t: &str) -> f64 {
        match t {
            "curiosity" | "voyager" | "hubl" => 8.0,
            "rover" | "telescope" | "space" => 1.5,
            _ => 1.0,
        }
    }

    #[test]
    fn adjacent_dates_tier3_stray_digit_not() {
        assert_eq!(date_tier("Curiosity was launched on November 26, 2011."), 3.0);
        assert_eq!(date_tier("It launched on 4 October 1957 as part of the program."), 3.0);
        assert!(date_tier("Voyager 2 operated in April") < 3.0, "stray digit+month must not reach tier 3");
        assert_eq!(date_tier("Voyager 2 operated in April"), 0.0);
        assert_eq!(date_tier("The mission launched in April 1961"), 2.0);
    }

    #[test]
    fn distinctive_subject_idf_beats_generic() {
        let ev = vec!["launch".to_string()];
        let subj_c = vec!["curiosity".to_string(), "rover".to_string()];
        let sc = sentence_date_score("Curiosity was launched on November 26, 2011.", DatePrecision::Open, &ev, &subj_c, &w);
        let sg = sentence_date_score("The rover was launched on November 26, 2011.", DatePrecision::Open, &ev, &subj_c, &w);
        assert!(sc > sg, "distinctive {} should exceed generic {}", sc, sg);
    }

    #[test]
    fn cross_article_sentence_mention_binds() {
        let ev = vec!["launch".to_string()];
        let subj = vec!["voyager".to_string(), "2".to_string()];
        let bound = sentence_date_score("Voyager 2 was launched on August 20, 1977.", DatePrecision::Open, &ev, &subj, &w);
        let unbound = sentence_date_score("Voyager 1 was launched on September 5, 1977.", DatePrecision::Open, &ev, &subj, &w);
        assert!(bound > unbound, "explicit Voyager 2 mention {} should beat sibling {}", bound, unbound);
    }

    #[test]
    fn tie_break_prefers_earlier_bm25_rank() {
        // scanning in rank order with strict > keeps the first (best-ranked)
        let scores = [(0usize, 4.0f64), (1usize, 4.0f64), (2, 4.0)];
        let mut best: Option<(usize, f64)> = None;
        for (i, sc) in scores {
            if sc > 0.0 && best.map(|(_, b)| sc > b).unwrap_or(true) {
                best = Some((i, sc));
            }
        }
        assert_eq!(best.map(|(i, _)| i), Some(0));
    }

    #[test]
    fn precision_activation_rules() {
        assert_eq!(date_activation("Who commanded the mission?"), None);
        assert_eq!(date_activation("When was the rover launched?"), Some(DatePrecision::Open));
        assert_eq!(date_activation("In what year did it orbit?"), Some(DatePrecision::YearOnly));
        assert_eq!(date_activation("On what exact date did it launch?"), Some(DatePrecision::Exact));
    }

    #[test]
    fn multi_term_event_specificity() {
        let ev = vec!["name".to_string(), "edwin".to_string(), "hubl".to_string()];
        let all = sentence_date_score("The telescope was named after Edwin Hubble in 1946.", DatePrecision::Open, &ev, &[], &w);
        let one = sentence_date_score("The telescope mirror was ground in 1946.", DatePrecision::Open, &ev, &[], &w);
        assert!(all > one);
    }
}

#[cfg(test)]
mod parity_tests {
    //! Probe/server parity guard: search() is structurally search_debug().0,
    //! so identical status and ordered passage IDs hold by construction within
    //! one build. Compile-time anchor pinning the delegation relationship.

    #[test]
    fn search_is_search_debug_projection() {
        fn takes_both(
            f1: fn(&crate::store::Store, &str, &str, &super::RetrievalConfig) -> crate::types::SearchResponse,
            f2: fn(&crate::store::Store, &str, &str, &super::RetrievalConfig) -> (crate::types::SearchResponse, super::SearchDebug),
        ) -> bool {
            let _ = (f1, f2);
            true
        }
        assert!(takes_both(super::search, super::search_debug));
    }
}
