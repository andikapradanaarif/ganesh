//! Evidence-sufficiency acceptance: given the selected passages, decide whether
//! they can support an answer or the system must abstain. This module is the
//! measurement surface for issue #163. The decision is traceable: every rule
//! records the sentence span it relied on, so a reviewer can inspect exactly
//! why Ganesh said "ok" or "insufficient_context".
//!
//! Two experimental rules live behind config flags, default off (the off state
//! reproduces the revised v2 reference behavior exactly):
//!   quantity_binding  — experiment A: quantities must be attached to the
//!                       measured property in the same sentence; digits and
//!                       number words count; years and version tokens do not.
//!   transition_check  — experiment B: state/transition questions need
//!                       cue-class, polarity and environment compatible evidence.

/// A quantity mention found in a passage sentence, with its binding analysis.
#[derive(Debug, Clone)]
pub struct QuantityMention {
    pub passage_id: String,
    pub sentence: String,
    pub token: String,
    pub bound_terms: Vec<String>,
    /// Some(reason) when this mention cannot serve as measurement evidence.
    pub exclusion: Option<String>,
}

/// A cue sentence found for a state/transition question.
#[derive(Debug, Clone)]
pub struct StateSpan {
    pub passage_id: String,
    pub sentence: String,
    pub cue_class: String,
    pub negated: bool,
    pub environment: Option<String>,
    pub object_terms: Vec<String>,
    /// Proposition verdict (experiment C): supports / contradicts / unresolved.
    pub verdict: String,
}

/// Action groups for positive support. Positive evidence must establish the
/// requested action or a justified equivalent — never a merely class-related
/// event. Directional inference: completion entails the action started, never
/// the reverse. Counterexample where it must abstain: "the export finished"
/// cannot answer "did the export start on May 4" for a *different* export or
/// date, and "the export started" can never answer "did it finish".
fn action_group(stem: &str) -> Option<&'static str> {
    Some(match stem {
        "complet" | "finish" | "succeed" => "finish",
        "start" | "begin" | "began" => "start",
        "deploy" => "deploy",
        "run" => "run",
        "perform" | "occur" | "take" => "activity",
        "cancel" | "skip" => "cancel",
        "continu" | "underway" => "continue",
        "pass" => "pass",
        "fail" => "pass",
        "verifi" | "confirm" | "unverifi" | "unconfirm" => "verify",
        "propos" | "plan" | "intend" | "suggest" | "consider" => "propose",
        "attempt" | "tri" | "effort" => "attempt",
        "becam" | "becom" | "option" | "mandatori" | "deprecat" | "drop" | "adopt" => "transition",
        "live" => "run",
        _ => return None,
    })
}

/// Split a sentence into clauses on outcome-relevant boundaries. "While" and
/// semicolons separate activities; a comma alone does not (it is often part
/// of a single activity description).
pub fn clauses(sentence: &str) -> Vec<String> {
    let mut out = Vec::new();
    let lowered = format!(" {}", sentence.replace("While ", "while "));
    let has_while = lowered.contains("while");
    for part in lowered.split(';') {
        for sub in part.split(" while ") {
            // The comma is an activity boundary only in while-constructions
            // (the inverted 'While X, Y' form); elsewhere it is an
            // introductory phrase or clause-internal punctuation.
            let pieces: Vec<&str> = if has_while { sub.split(", ").collect() } else { vec![sub] };
            for piece in pieces {
                let t = piece.trim();
                if !t.is_empty() { out.push(t.to_string()); }
            }
        }
    }
    if out.is_empty() { out.push(sentence.to_string()); }
    out
}

/// Positive support: same action, or the directional completion-implies-start
/// inference. Everything else — including two events in the same epistemic
/// class — is not evidence for the asked action.
pub fn action_supports(q_pred: &str, evidence_stem: &str) -> bool {
    match (action_group(q_pred), action_group(evidence_stem)) {
        (Some(q), Some(e)) if q == e => true,
        (Some("start"), Some("finish")) => true,
        _ => false,
    }
}

/// The dominant predicate stem of a question: the first cue-class term in
/// the question's stemmed terms that belongs to an action group.
pub fn question_predicate(terms: &[String]) -> Option<String> {
    terms.iter().find(|t| action_group(t).is_some()).cloned()
}

#[derive(Debug, Clone, Default)]
pub struct AcceptanceTrace {
    pub decision: String,
    pub reasons: Vec<String>,
    pub quantity_mentions: Vec<QuantityMention>,
    pub state_spans: Vec<StateSpan>,
    pub qa_margin: Option<f32>,
    pub qa_span: String,
}

const NUMBER_WORDS: &[&str] = &[
    "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
    "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen",
    "eighteen", "nineteen", "twenty", "thirty", "forty", "fifty", "sixty",
    "seventy", "eighty", "ninety", "hundred", "thousand",
];

/// Split verbatim text into sentences on terminal punctuation followed by
/// whitespace. Abbreviation-agnostic and deliberately simple: sufficient for
/// proximity analysis, not a linguistic sentence model.
pub fn sentences(text: &str) -> Vec<String> {
    let mut out = Vec::new();
    let mut cur = String::new();
    let mut chars = text.chars().peekable();
    while let Some(ch) = chars.next() {
        cur.push(ch);
        if matches!(ch, '.' | '!' | '?') {
            match chars.peek() {
                Some(' ') | Some('\n') | None => {
                    let s = cur.trim().to_string();
                    if !s.is_empty() {
                        out.push(s);
                    }
                    cur.clear();
                }
                _ => {}
            }
        }
    }
    let s = cur.trim().to_string();
    if !s.is_empty() {
        out.push(s);
    }
    out
}

/// Whitespace-delimited surface tokens of a sentence, punctuation trimmed at
/// the edges but interior structure (hyphens, dots, case) preserved for
/// version/shape analysis.
fn surface_tokens(sentence: &str) -> Vec<String> {
    sentence
        .split_whitespace()
        .map(|t| t.trim_matches(|c: char| matches!(c, ',' | ';' | ':' | '(' | ')' | '"' | '\'')).to_string())
        .filter(|t| !t.is_empty())
        .collect()
}

/// A digit literal that stands alone (not embedded in an alphanumeric token
/// such as `i32` or `C23`). Returns the literal text.
fn standalone_digits(token: &str) -> Option<String> {
    if !token.is_empty() && token.chars().all(|c| c.is_ascii_digit()) {
        Some(token.to_ascii_lowercase())
    } else {
        None
    }
}

/// Version-like numeric shapes: dotted sequences (`2.0`, `1.18.3`) or digit
/// runs glued to letters inside one surface token (`C23`, `i32`, `R9`).
fn version_like(token: &str) -> bool {
    let digit_dot_digit = token.split('.').count() > 1
        && token.split('.').all(|part| !part.is_empty() && part.chars().all(|c| c.is_ascii_digit()));
    let mixed_alnum = token.chars().any(|c| c.is_ascii_digit())
        && token.chars().any(|c| c.is_alphabetic())
        && token.chars().all(|c| c.is_alphanumeric());
    digit_dot_digit || mixed_alnum
}

fn year_like(literal: &str) -> bool {
    if literal.len() != 4 {
        return false;
    }
    literal.parse::<u32>().map(|y| (1900..=2100).contains(&y)).unwrap_or(false)
}

/// Quantity candidate tokens in one sentence: standalone digit literals and
/// number words, each classified. Embedded digits inside version/identifier
/// tokens never surface here.
pub fn quantity_candidates(sentence: &str) -> Vec<(String, Option<String>)> {
    let mut out = Vec::new();
    for tok in surface_tokens(sentence) {
        let low = tok.to_ascii_lowercase();
        let comma_grouped = low.len() > 4
            && low.chars().all(|c| c.is_ascii_digit() || c == ',')
            && low.chars().any(|c| c == ',')
            && low.split(',').enumerate().all(|(i, part)| {
                (!part.is_empty() && part.chars().all(|c| c.is_ascii_digit()))
                    && (i == 0 && part.len() <= 3 || i > 0 && part.len() == 3)
            });
        if comma_grouped {
            out.push((low.replace(',', ""), None));
        } else if let Some(lit) = standalone_digits(&low) {
            if year_like(&lit) {
                out.push((lit, Some("year-like 4-digit literal".to_string())));
            } else {
                out.push((lit, None));
            }
        } else if version_like(&low) {
            // Digits embedded in versions/types are not measurement quantities.
            if low.chars().any(|c| c.is_ascii_digit()) {
                out.push((low, Some("version or identifier token".to_string())));
            }
        } else if NUMBER_WORDS.contains(&low.as_str()) {
            out.push((low, None));
        } else {
            // Leading-digit hyphenated forms such as `32-bit`: the digits are a
            // quantity glued directly to the measured unit.
            let digit_prefix: String = low.chars().take_while(|c| c.is_ascii_digit()).collect();
            if !digit_prefix.is_empty()
                && low[digit_prefix.len()..].starts_with('-')
                && low.chars().any(|c| c.is_alphabetic())
            {
                out.push((digit_prefix, None));
            }
        }
    }
    out
}

const EXTRACTION_FUNCTION_WORDS: &[&str] = &[
    "the", "a", "an", "does", "do", "did", "is", "are", "was", "were", "have", "has", "had",
    "in", "on", "of", "to", "and", "or", "for", "with", "by", "at", "from", "as", "it", "its",
    "there", "their", "that", "this", "be", "been", "being", "can", "could", "will", "would",
];

/// Extract the measured property of a quantity question from its surface text:
/// the content words following "how many", "how much" or "number/amount of",
/// up to the first function word. For "how many members does the committee
/// have" this yields [member]; the committee itself is not the measured thing.
pub fn measured_property_terms(query: &str) -> Vec<String> {
    let toks: Vec<String> = query
        .split_whitespace()
        .map(|t| t.trim_matches(|c: char| !c.is_alphanumeric()).to_ascii_lowercase())
        .filter(|t| !t.is_empty())
        .collect();
    let mut start = None;
    for (i, t) in toks.iter().enumerate() {
        if t == "many" || t == "much" {
            start = Some(i + 1);
            break;
        }
        if (t == "number" || t == "amount") && toks.get(i + 1).map(|n| n == "of").unwrap_or(false) {
            start = Some(i + 2);
            break;
        }
    }
    let mut out = Vec::new();
    if let Some(mut i) = start {
        while i < toks.len() && EXTRACTION_FUNCTION_WORDS.contains(&toks[i].as_str()) {
            i += 1;
        }
        while i < toks.len() && !EXTRACTION_FUNCTION_WORDS.contains(&toks[i].as_str()) {
            let stemmed = crate::search::stemmed_tokens(&toks[i]);
            out.extend(stemmed);
            i += 1;
        }
    }
    if out.is_empty() {
        // Measurement questions ("what is the diameter of X in meters"):
        // the measured property is the measurement noun itself, plus any
        // explicit unit. Predeclared linguistic lists, no fixture vocabulary.
        const UNITS: &[&str] = &[
            "meter", "meters", "metre", "metres", "kilometer", "kilometers", "kilometre",
            "kilometres", "mile", "miles", "feet", "foot", "inch", "inches", "kg",
            "kilogram", "kilograms", "ton", "tons", "tonne", "tonnes", "gram", "grams",
            "cm", "mm", "km", "watt", "watts", "byte", "bytes",
        ];
        for t in &toks {
            if crate::search::MEASURE_PROPS.contains(&t.as_str()) || UNITS.contains(&t.as_str()) {
                out.extend(crate::search::stemmed_tokens(t));
            }
        }
    }
    out.sort();
    out.dedup();
    out
}

/// Version-like surface tokens of a text, normalized lowercase: dotted numeric
/// versions (`3.0`) and letter-digit identifiers (`c89`, `r9`). Used for
/// version-consistency binding.
fn version_tokens(text: &str) -> std::collections::BTreeSet<String> {
    text.split_whitespace()
        .map(|t| t.trim_matches(|c: char| !c.is_alphanumeric() && c != '.' && c != '+' && c != '#').to_ascii_lowercase())
        .filter(|t| version_like(t))
        .collect()
}

// ---------------------------------------------------------------------------
// Experiment B lexicons: epistemic cue classes, in stemmed form.
// ---------------------------------------------------------------------------

pub const VERIFY_CUES: &[&str] = &["verifi", "confirm", "deploy", "run", "pass", "succeed", "complet", "live", "start", "unverifi", "unconfirm", "finish", "fail", "begin", "began", "reach", "perform", "cancel", "skip", "take", "occur", "continu", "underway"];
pub const PROPOSE_CUES: &[&str] = &["propos", "plan", "intend", "suggest", "consider"];
pub const ATTEMPT_CUES: &[&str] = &["attempt", "tri", "effort"];
pub const TRANSITION_CUES: &[&str] = &["becam", "becom", "option", "mandatori", "deprecat", "drop", "adopt"];

const NEGATION_MARKERS: &[&str] = &[
    " not ", "n't", " never ", "unverified", "unconfirmed", "without", "failed", "no evidence",
];

const ENV_STEMS: &[&str] = &["product", "stag", "test", "dev"];

pub fn cue_class_of(terms: &[String]) -> Option<&'static str> {
    let has = |lex: &[&str]| terms.iter().any(|t| lex.contains(&t.as_str()));
    if has(PROPOSE_CUES) {
        Some("propose")
    } else if has(ATTEMPT_CUES) {
        Some("attempt")
    } else if has(TRANSITION_CUES) {
        Some("transition")
    } else if has(VERIFY_CUES) {
        Some("verify")
    } else {
        None
    }
}

fn cue_terms_in(stemmed_sentence: &[String]) -> Vec<&'static str> {
    let mut out = Vec::new();
    for (class, lex) in [
        ("verify", VERIFY_CUES),
        ("propose", PROPOSE_CUES),
        ("attempt", ATTEMPT_CUES),
        ("transition", TRANSITION_CUES),
    ] {
        if stemmed_sentence.iter().any(|t| lex.contains(&t.as_str())) {
            out.push(class);
        }
    }
    // Embedded-cue suppression: when a weaker governing cue (propose/attempt)
    // is present, stronger-class cues in the same sentence are typically its
    // embedded object — "proposed deploying X" or "attempted to deploy X" do
    // not verify anything. Transition cues are orthogonal and unaffected.
    let governs = out.contains(&"propose") || out.contains(&"attempt");
    if governs {
        out.retain(|c| *c != "verify");
    }
    out
}

/// Class strength: evidence of a stronger fact also establishes weaker ones
/// along the same chain. Verification implies the attempt and, typically, that
/// deployment was at some point proposed; an attempt implies nothing about
/// verification.
pub fn class_satisfies(required: &str, found: &str) -> bool {
    let strength = |c: &str| match c {
        "propose" => 1,
        "attempt" => 2,
        "transition" => 3,
        "verify" => 4,
        _ => 0,
    };
    if required == "transition" || found == "transition" {
        // Transitions are orthogonal: only transition evidence satisfies a
        // transition question, and transition evidence does not by itself
        // verify a deployment state.
        return required == found;
    }
    strength(found) >= strength(required)
}

fn negated(sentence: &str) -> bool {
    let padded = format!(" {} ", sentence.to_ascii_lowercase());
    NEGATION_MARKERS.iter().any(|m| padded.contains(m))
}

fn env_of(stemmed: &[String]) -> Option<String> {
    for t in stemmed {
        if ENV_STEMS.contains(&t.as_str()) {
            return Some(t.clone());
        }
    }
    None
}

pub struct DecideInput<'a> {
    pub query: &'a str,
    /// Distinct stemmed content terms of the query.
    pub terms: &'a [String],
    /// Stemmed subject terms (article titles) mentioned in the query — excluded
    /// from measured-property binding because they name the subject, not the
    /// measured thing.
    pub subject_terms: &'a std::collections::BTreeSet<String>,
    /// Selected passages: id, verbatim text, section heading, stemmed index tokens.
    pub passages: &'a [(String, String, String, Vec<String>)],
    pub idf_union_coverage: f64,
    pub union_coverage: f64,
    pub top_coverage: f64,
    pub min_idf_union_coverage: f64,
    pub min_union_coverage: f64,
    pub min_top_coverage: f64,
    pub quantity_question: bool,
    pub quantity_unit_missing: bool,
    pub any_numeric_token: bool,
    pub quantity_binding: bool,
    /// M2: at least one selected passage entered the pool through dense
    /// embedding recall — enables the semantic coverage override.
    pub dense_support: bool,
    pub transition_check: bool,
    /// Experiment C: proposition-level verdicts. Negated compatible evidence
    /// contradicts and can settle a yes/no question with an explicit negative.
    pub proposition_check: bool,
    /// Experiment D: dated-entry context chain — headings plus unambiguous
    /// antecedents augment continuation sentences.
    pub context_chain: bool,
    /// Experiment E (issue #169): positive support must be action-exact or a
    /// justified equivalent for the same subject and scope.
    pub action_check: bool,
}

fn stemmed_query_like(text: &str) -> Vec<String> {
    crate::search::stemmed_tokens(text)
}


/// True when the digit token at this byte position in the raw sentence sits
/// inside a code-like operator context (==, !=, =, backticks, true/false
/// adjacency). Such literals are expressions, not measurements.
pub fn code_literal_context(sentence: &str, token: &str) -> bool {
    if token != "0" && token != "1" {
        return false;
    }
    let mut found = false;
    let bytes = sentence.as_bytes();
    let mut start = 0usize;
    while let Some(pos) = sentence[start..].find(token) {
        let abs = start + pos;
        let before_ok = abs == 0 || !bytes[abs - 1].is_ascii_digit();
        let after = abs + token.len();
        let after_ok = after >= bytes.len() || !bytes[after].is_ascii_digit();
        if before_ok && after_ok {
            let lo = abs.saturating_sub(4);
            let hi = (after + 4).min(bytes.len());
            let window = &sentence[lo..hi];
            if window.contains('=') || window.contains('!') || window.contains('<') || window.contains('>') || window.contains('`') {
                return true;
            }
            found = true;
        }
        start = abs + token.len();
        if start >= sentence.len() {
            break;
        }
    }
    let _ = found;
    false
}

/// True when the surface word "one" is used pronominally in this sentence:
/// followed by "of", or preceded by a small closed class of pronoun-triggering
/// words. Quantifying uses ("one million", "one window") survive.
pub fn pronominal_one(sentence: &str) -> bool {
    let toks: Vec<String> = sentence
        .split_whitespace()
        .map(|t| t.trim_matches(|c: char| !c.is_alphanumeric()).to_ascii_lowercase())
        .collect();
    const TRIGGERS: &[&str] = &["of", "instead", "use", "if", "when", "as", "like", "in", "that", "which", "than"];
    for (i, t) in toks.iter().enumerate() {
        if t == "one" {
            if toks.get(i + 1).map(|n| n == "of").unwrap_or(false) {
                return true;
            }
            if i > 0 && TRIGGERS.contains(&toks[i - 1].as_str()) {
                return true;
            }
        }
    }
    false
}

/// True when the literal occurs within `window` whitespace tokens of any of
/// the target stems (measured property or unit) in the sentence.
pub fn quantifying_window(sentence: &str, literal: &str, targets: &[String], window: usize) -> bool {
    if targets.is_empty() {
        return true;
    }
    let toks: Vec<String> = sentence
        .split_whitespace()
        .map(|t| t.trim_matches(|c: char| !c.is_alphanumeric()).to_ascii_lowercase())
        .collect();
    let stemmed: Vec<Vec<String>> = toks.iter().map(|t| crate::search::stemmed_tokens(t)).collect();
    for (i, t) in toks.iter().enumerate() {
        let bare = t.trim_end_matches(|c: char| !c.is_ascii_digit() && c != '.').trim_start_matches(|c: char| !c.is_ascii_digit());
        let norm = t.replace(',', "");
        let matches_lit = *t == *literal || bare == literal || norm == *literal || t.starts_with(literal);
        if !matches_lit {
            continue;
        }
        let lo = i.saturating_sub(window);
        let hi = (i + window + 1).min(toks.len());
        for j in lo..hi {
            if j == i {
                continue;
            }
            if stemmed[j].iter().any(|st| targets.contains(st)) {
                return true;
            }
        }
    }
    false
}

/// Environment stems functioning as deployment environments in the QUERY:
/// the environment word must follow a preposition ("in production", "to
/// staging", "on test"). Activity nouns like "germination test" do not, so
/// the queried activity is never stripped from attribution analysis.
pub fn query_env_stems(query: &str) -> std::collections::BTreeSet<String> {
    const PREPS: &[&str] = &["in", "to", "on", "at", "into", "onto"];
    const ENV_SURFACE: &[&str] = &["production", "prod", "staging", "stage", "test", "dev", "development"];
    let toks: Vec<String> = query
        .to_lowercase()
        .split(|c: char| !c.is_alphanumeric())
        .filter(|w| !w.is_empty())
        .map(|w| w.to_string())
        .collect();
    let mut out = std::collections::BTreeSet::new();
    for i in 1..toks.len() {
        if PREPS.contains(&toks[i - 1].as_str()) && ENV_SURFACE.contains(&toks[i].as_str()) {
            for st in crate::search::stemmed_tokens(&toks[i]) {
                if ENV_STEMS.contains(&st.as_str()) {
                    out.insert(st);
                }
            }
        }
    }
    out
}

/// Split a compound question into clauses on coordinating boundaries, so
/// per-clause precision checks apply only where their trigger lives.
pub fn question_clauses(query: &str) -> Vec<String> {
    let mut out = vec![String::new()];
    let q = query.to_lowercase();
    let mut rest = q.as_str();
    const BOUNDARIES: &[&str] = &[
        ", and which ", ", and who ", ", and what ", ", and where ",
        ", and when ", ", and how ", ", and ", "; ",
    ];
    while !rest.is_empty() {
        let hit = BOUNDARIES
            .iter()
            .filter_map(|b| rest.find(b).map(|pos| (pos, b.len())))
            .min_by_key(|(pos, _)| *pos);
        match hit {
            Some((pos, blen)) => {
                out.last_mut().unwrap().push_str(&rest[..pos]);
                out.push(String::new());
                rest = &rest[pos + blen..];
            }
            None => {
                out.last_mut().unwrap().push_str(rest);
                break;
            }
        }
    }
    out.into_iter().filter(|c| !c.trim().is_empty()).collect()
}

/// Future-tense date/year question: asks when something WILL happen.
pub fn future_tense_date_query(query: &str) -> bool {
    let q = query.to_lowercase();
    q.contains("in what year will")
        || q.contains("in which year will")
        || q.contains("what year will")
        || q.starts_with("when will")
        || q.contains("on what date will")
        || q.contains("on what exact date will")
        || q.contains("when will")
}

/// Canonical unit aliases: surface form -> canonical unit.
pub fn canonical_unit(token: &str) -> Option<&'static str> {
    Some(match token {
        "m" | "meter" | "meters" | "metre" | "metres" => "m",
        "in" | "inch" | "inches" => "in",
        "ft" | "foot" | "feet" => "ft",
        "km" | "kilometer" | "kilometers" | "kilometre" | "kilometres" => "km",
        "mi" | "mile" | "miles" => "mi",
        "nm" | "nanometer" | "nanometers" | "nanometre" | "nanometres" => "nm",
        "cm" | "centimeter" | "centimeters" | "centimetre" | "centimetres" => "cm",
        "mm" | "millimeter" | "millimeters" | "millimetre" | "millimetres" => "mm",
        "yd" | "yard" | "yards" => "yd",
        "kg" | "kilogram" | "kilograms" => "kg",
        "g" | "gram" | "grams" => "g",
        "mg" | "milligram" | "milligrams" => "mg",
        "t" | "ton" | "tons" | "tonne" | "tonnes" => "t",
        "lb" | "lbs" | "pound" | "pounds" => "lb",
        "oz" | "ounce" | "ounces" => "oz",
        "l" | "liter" | "liters" | "litre" | "litres" => "l",
        "ml" | "milliliter" | "milliliters" | "millilitre" | "millilitres" => "ml",
        "s" | "sec" | "second" | "seconds" => "s",
        "min" | "minute" | "minutes" => "min",
        "hr" | "hrs" | "hour" | "hours" => "hr",
        "w" | "watt" | "watts" => "w",
        "kw" | "kilowatt" | "kilowatts" => "kw",
        "mw" | "megawatt" | "megawatts" => "mw",
        "hz" | "hertz" => "hz",
        "khz" => "khz",
        "mhz" => "mhz",
        "ghz" => "ghz",
        "km/h" | "kmh" | "kph" => "km/h",
        "m/s" => "m/s",
        "mph" => "mph",
        "deg" | "degree" | "degrees" => "deg",
        "byte" | "bytes" => "byte",
        "bit" | "bits" => "bit",
        _ => return None,
    })
}

/// The unit explicitly requested by an "in <unit>" measurement question, if any.
pub fn requested_unit(query: &str) -> Option<&'static str> {
    let toks: Vec<String> = query
        .split_whitespace()
        .map(|t| t.trim_matches(|c: char| !c.is_alphanumeric() && c != '/').to_ascii_lowercase())
        .collect();
    for (i, t) in toks.iter().enumerate() {
        if t == "in" {
            if let Some(u) = toks.get(i + 1).and_then(|n| canonical_unit(n)) {
                return Some(u);
            }
        }
    }
    None
}

/// The canonical unit token adjacent (within 2 surface tokens) after a digit
/// literal in a sentence, if present.
pub fn adjacent_unit(sentence: &str, token: &str) -> Option<&'static str> {
    let toks: Vec<String> = sentence
        .split_whitespace()
        .map(|t| t.trim_matches(|c: char| !c.is_alphanumeric() && c != '/' && c != '.' && c != ',').to_ascii_lowercase())
        .collect();
    for (i, t) in toks.iter().enumerate() {
        let bare = t.replace(',', "").trim_matches('.').to_string();
        if bare != token {
            continue;
        }
        for j in 1..=2usize {
            if let Some(next) = toks.get(i + j) {
                // Sentence-final units carry their period ("meters.") — the
                // dot-preserving trim above must keep dots for digit tokens
                // but no canonical unit contains one.
                let unit_surface = next.trim_end_matches('.');
                if let Some(u) = canonical_unit(unit_surface) {
                    return Some(u);
                }
                if next.chars().any(|c| c.is_ascii_digit()) {
                    break;
                }
            }
        }
    }
    None
}

/// Exact-date trigger: the question explicitly demands calendar-day precision.
pub fn exact_date_query(query: &str) -> bool {
    let q = query.to_lowercase();
    const TRIGGERS: &[&str] = &[
        "exact date", "exact calendar date", "exact day", "on what date", "on what day",
        "what exact date", "what exact day", "which exact date", "which exact day",
    ];
    TRIGGERS.iter().any(|t| q.contains(t))
}


/// Strict decimal-measurement test: the pure digits.digits token must be
/// immediately followed (within 1–2 surface tokens) by an explicit measurement
/// unit, and not immediately preceded by version/release/v/in or a capitalized
/// software/standard name token.
pub fn unit_adjacent_decimal(sentence: &str, token: &str) -> bool {
    if !token.chars().all(|c| c.is_ascii_digit() || c == '.') || !token.contains('.') {
        return false;
    }
    const UNITS: &[&str] = &[
        "m", "meter", "meters", "metre", "metres", "km", "kilometer", "kilometers",
        "kilometre", "kilometres", "cm", "mm", "mile", "miles", "ft", "feet", "foot",
        "in", "inch", "inches", "kg", "kilogram", "kilograms", "g", "gram", "grams",
        "lb", "lbs", "ton", "tons", "tonne", "tonnes", "w", "watt", "watts", "kw",
        "mw", "hz", "ghz", "mhz", "mb", "gb", "kb", "byte", "bytes", "%", "percent",
        "million", "billion", "trillion", "celsius", "fahrenheit", "m/s", "km/s", "km/h", "mph",
    ];
    const PRE_BLOCKS: &[&str] = &["version", "v", "in", "release", "edition"];
    let toks: Vec<String> = sentence
        .split_whitespace()
        .map(|t| t.trim_matches(|c: char| !c.is_alphanumeric() && c != '.' && c != '%').to_ascii_lowercase())
        .collect();
    for (i, t) in toks.iter().enumerate() {
        let bare = t.trim_matches(|c: char| !c.is_ascii_digit() && c != '.');
        if bare != token {
            continue;
        }
        // preceded by blocking token or capitalized name directly glued?
        if i > 0 && PRE_BLOCKS.contains(&toks[i - 1].as_str()) {
            continue;
        }
        if i > 0 {
            let prev = &toks[i - 1];
            let raw_prev_capitalized = sentence
                .split_whitespace()
                .nth(i - 1)
                .map(|w| w.chars().next().map(|c| c.is_ascii_uppercase()).unwrap_or(false))
                .unwrap_or(false);
            if raw_prev_capitalized && !prev.chars().any(|c| c.is_ascii_digit()) && prev.len() > 2 {
                continue;
            }
        }
        // followed within 1–2 tokens by a unit
        for j in i + 1..=(i + 2).min(toks.len().saturating_sub(1)) {
            if j < toks.len() {
                let uj = toks[j].trim_end_matches(|c: char| !c.is_alphanumeric());
                if UNITS.contains(&uj) {
                    return true;
                }
            }
        }
    }
    false
}

/// Canonical irregular-verb forms: map base/past/participle variants of common
/// English irregular verbs to one shared key so event terms match across
/// morphology. General English, zero fixture vocabulary.
pub fn irregular_canon(form: &str) -> &str {
    const GROUPS: &[&[&str]] = &[
        &["sink", "sank", "sunk"],
        &["begin", "began", "begun"],
        &["fly", "flew", "flown"],
        &["build", "built"],
        &["write", "wrote", "written"],
        &["break", "broke", "broken"],
        &["fall", "fell", "fallen"],
        &["win", "won"],
        &["make", "made"],
        &["take", "took", "taken"],
        &["find", "found"],
        &["leave", "left"],
        &["lose", "lost"],
        &["draw", "drew", "drawn"],
        &["strike", "struck"],
        &["burn", "burnt", "burned"],
        &["rise", "rose", "risen"],
        &["hold", "held"],
        &["lead", "led"],
        &["send", "sent"],
        &["spend", "spent"],
        &["run", "ran"],
        &["go", "went", "gone"],
        &["come", "came"],
        &["see", "saw", "seen"],
        &["give", "gave", "given"],
        &["know", "knew", "known"],
        &["grow", "grew", "grown"],
        &["throw", "threw", "thrown"],
        &["speak", "spoke", "spoken"],
        &["choose", "chose", "chosen"],
        &["drive", "drove", "driven"],
        &["wear", "wore", "worn"],
        &["tear", "tore", "torn"],
        &["bear", "bore", "born", "borne"],
    ];
    for g in GROUPS {
        if g.contains(&form) {
            return g[0];
        }
    }
    form
}

/// True when the stemmed token matches another through irregular-verb canon:
/// compare canonical forms of both surface-level stems.
pub fn stem_matches_eventually(query_stem: &str, sent_stem: &str) -> bool {
    query_stem == sent_stem || irregular_canon(query_stem) == irregular_canon(sent_stem)
}

/// Future/planning cues that prevent a dated sentence from settling a
/// past-tense "did" question.
pub const FUTURE_CUES: &[&str] = &[
    "planned", "plan", "plans", "planning", "scheduled", "expected", "projected",
    "slated", "intended", "proposed", "proposal", "targeted", "aiming", "will",
    "would", "tentatively", "future", "contemplated", "suggested",
];

pub fn sentence_is_planned(sentence: &str) -> bool {
    let low = sentence.to_lowercase();
    let toks: Vec<&str> = low.split(|c: char| !c.is_alphabetic()).filter(|w| !w.is_empty()).collect();
    toks.iter().any(|w| FUTURE_CUES.contains(w))
        || low.contains("set to")
        || low.contains("due to")
        || low.contains("to develop")
        || low.contains("contract to")
        || low.contains("selected")
        || low.contains("awarded")
}


/// Roman numerals and small Arabic mission ordinals usable as distinguishing
/// mission qualifiers (Artemis III -> iii, Voyager 2 -> 2).
pub fn mission_qualifiers(query: &str) -> Vec<String> {
    const ROMAN: &[&str] = &["i", "ii", "iii", "iv", "v", "vi", "vii", "viii", "ix", "x"];
    let toks: Vec<String> = query
        .split(|c: char| !c.is_alphanumeric())
        .filter(|w| !w.is_empty())
        .map(|w| w.to_string())
        .collect();
    let mut out = Vec::new();
    for (i, t) in toks.iter().enumerate() {
        let low = t.to_lowercase();
        // roman numeral or arabic number adjacent to a capitalized proper noun
        let near_proper = (i > 0 && toks[i - 1].chars().next().map(|c| c.is_ascii_uppercase()).unwrap_or(false))
            || (i + 1 < toks.len() && toks[i + 1].chars().next().map(|c| c.is_ascii_uppercase()).unwrap_or(false));
        if near_proper && (ROMAN.contains(&low.as_str()) && low.len() <= 3
            || t.chars().all(|c| c.is_ascii_digit()) && t.len() <= 2)
        {
            out.push(low);
        }
    }
    out.sort();
    out.dedup();
    out
}

/// Normalize hyphenated compounds so de-orbit/deorbit and re-entry/reentry
/// share token forms.
pub fn dehyphen(tok: &str) -> String {
    tok.replace('-', "")
}

/// Does this sentence text mention the exact mission qualifier in a
/// proper-noun position (e.g. "Artemis III" / "Voyager 2")? Matches numeral
/// adjacent to a capitalized token, matching the query's own shape.
pub fn sentence_mentions_qualifier(sentence: &str, qualifier: &str) -> bool {
    let toks: Vec<String> = sentence.split_whitespace().map(|w| w.to_string()).collect();
    for (i, t) in toks.iter().enumerate() {
        let bare = t.trim_matches(|c: char| !c.is_alphanumeric()).to_lowercase();
        if bare != qualifier {
            continue;
        }
        let near_proper = (i > 0 && toks[i - 1].trim_matches(|c: char| !c.is_alphanumeric()).chars().next().map(|c| c.is_ascii_uppercase()).unwrap_or(false))
            || (i + 1 < toks.len() && toks[i + 1].trim_matches(|c: char| !c.is_alphanumeric()).chars().next().map(|c| c.is_ascii_uppercase()).unwrap_or(false));
        if near_proper {
            return true;
        }
    }
    false
}


/// Extractive wh-/factual question activation for the SQuAD 2.0 verifier:
/// yes/no state questions (did/was/were/is/are/does/do/has/have at start) are
/// excluded — the deterministic transition/proposition/action checks carry
/// those. Everything extractive activates.
pub fn qa_extractive_question(query: &str) -> bool {
    let q = query.to_lowercase();
    const YESNO: &[&str] = &["did ", "was ", "were ", "is ", "are ", "does ", "do ", "has ", "have ", "had "];
    for p in YESNO {
        if q.starts_with(p) {
            return false;
        }
    }
    const WH: &[&str] = &[
        "who", "what", "when", "where", "which", "how many", "how much", "how long",
        "how far", "how high", "how tall", "how deep", "how fast", "on what", "in what",
        "in which", "at what", "for what", "by what", "from what",
    ];
    WH.iter().any(|w| q.starts_with(w) || q.contains(&format!(" {w}")))
}

/// Past-tense occurrence question: did-shaped date/year question.
pub fn past_tense_date_query(query: &str) -> bool {
    let q = query.to_lowercase();
    (q.starts_with("when did") || q.contains("in what year did") || q.contains("in which year did")
        || q.contains("on what date did") || q.contains("on what exact date did")
        || q.contains("on what day did") || q.contains("what year did"))
        || exact_date_query(query) && q.contains(" did ")
}

pub fn decide(input: &DecideInput) -> AcceptanceTrace {
    let mut trace = AcceptanceTrace::default();
    let qenv_stems = query_env_stems(input.query);

    // ---- coverage gates, unchanged from the revised v2 reference ------------
    if input.idf_union_coverage < input.min_idf_union_coverage {
        trace.reasons.push(format!(
            "idf-union-coverage {:.3} below threshold {:.3}",
            input.idf_union_coverage, input.min_idf_union_coverage
        ));
    }
    if input.union_coverage < input.min_union_coverage {
        trace.reasons.push(format!(
            "union-coverage {:.3} below threshold {:.3}",
            input.union_coverage, input.min_union_coverage
        ));
    }
    if input.top_coverage < input.min_top_coverage {
        trace.reasons.push(format!(
            "top-coverage {:.3} below threshold {:.3}",
            input.top_coverage, input.min_top_coverage
        ));
    }

    // ---- quantity evidence ---------------------------------------------------
    if input.quantity_question {
        if input.quantity_unit_missing {
            trace.reasons.push("quantity unit term absent from selected passages".to_string());
        }
        if input.quantity_binding {
            // Experiment A: a quantity counts only when a digit literal or
            // number word shares a sentence with a measured-property term, and
            // the sentence is version-consistent with the question. When the
            // measured property cannot be extracted from the question surface,
            // fall back to the crude digit-presence rule rather than inventing
            // a stricter one.
            let measured_terms = measured_property_terms(input.query);
            let q_versions = version_tokens(input.query);
            let mut bound_found = false;
            for (pid, verbatim, _, _) in input.passages {
                for sentence in &sentences(verbatim) {
                    let cands = quantity_candidates(sentence);
                    if cands.is_empty() {
                        continue;
                    }
                    let sent_stemmed = stemmed_query_like(sentence);
                    let measured: Vec<String> = if measured_terms.is_empty() {
                        input
                            .terms
                            .iter()
                            .filter(|t| {
                                !input.subject_terms.contains(*t)
                                    && !t.chars().all(|c| c.is_ascii_digit())
                                    && sent_stemmed.contains(*t)
                            })
                            .cloned()
                            .collect()
                    } else {
                        measured_terms.iter().filter(|t| sent_stemmed.contains(*t)).cloned().collect()
                    };
                    // Version consistency: when the question names a version,
                    // a binding sentence must mention it; a sentence naming a
                    // different version measures that version instead.
                    let s_versions = version_tokens(sentence);
                    let version_ok = q_versions.is_empty()
                        || q_versions.iter().any(|v| s_versions.contains(v));
                    for (token, exclusion) in cands {
                        // Tightened quantifying evidence (predeclared): a
                        // pronominal "one" or a code-literal 0/1 is never a
                        // measurement, and the literal must sit within an
                        // 8-token window of the measured property or unit.
                        let version_override = exclusion.as_deref() == Some("version or identifier token")
                            && !measured_terms.is_empty()
                            && unit_adjacent_decimal(sentence, &token)
                            && !q_versions.contains(&token);
                        let tightened_exclusion = if exclusion.is_some() && !version_override {
                            exclusion.clone()
                        } else if token == "one" && pronominal_one(sentence) {
                            Some("pronominal one, not a quantity".to_string())
                        } else if code_literal_context(sentence, &token) {
                            Some("code-literal digit in operator expression".to_string())
                        } else if !measured_terms.is_empty()
                            && !quantifying_window(sentence, &token, &measured_terms, 8)
                        {
                            Some("numeric token outside quantifying window of measured property".to_string())
                        } else {
                            None
                        };
                        let mention_exclusion = match (&tightened_exclusion, version_ok) {
                            (Some(e), _) => Some(e.clone()),
                            (None, false) => Some("version mismatch with question".to_string()),
                            (None, true) => None,
                        };
                        trace.quantity_mentions.push(QuantityMention {
                            passage_id: pid.clone(),
                            sentence: sentence.clone(),
                            token: token.clone(),
                            bound_terms: measured.clone(),
                            exclusion: mention_exclusion.clone(),
                        });
                        if mention_exclusion.is_none() && !measured.is_empty() {
                            bound_found = true;
                        }
                    }
                }
            }
            if !bound_found {
                trace.reasons.push(
                    "quantity-binding: no digit literal or number word shares a sentence with the measured property in a version-consistent way"
                        .to_string(),
                );
            }
        } else if !input.any_numeric_token {
            // Reference behavior: crude presence of any digit token.
            trace.reasons.push("no numeric token in selected passages".to_string());
        }
    }

    // ---- requested-unit verification -----------------------------------------
    // An explicit "in <unit>" measurement question requires a bound digit
    // literal carrying the canonically matching unit within the quantifying
    // window of the measured property. Year-like literals stay excluded; a
    // unit mismatch (inches when the dimension exists in cm, tonnes when no
    // tonne measurement binds the mass) abstains.
    if input.quantity_question {
        if let Some(want) = requested_unit(input.query) {
            // Circular-binding guard: unit terms extracted into the measured
            // property set (e.g. "tonn" from "in tonnes") must not witness the
            // quantifying window — otherwise any digit adjacent to its own
            // unit token self-satisfies without binding the measured property.
            const UNIT_SURFACE: &[&str] = &[
                "m", "meter", "meters", "metre", "metres", "in", "inch", "inches",
                "ft", "foot", "feet", "km", "kilometer", "kilometers", "kilometre",
                "kilometres", "mi", "mile", "miles", "nm", "nanometer", "nanometers",
                "cm", "mm", "yd", "yard", "yards", "kg", "kilogram", "kilograms",
                "g", "gram", "grams", "mg", "t", "ton", "tons", "tonne", "tonnes",
                "lb", "lbs", "pound", "pounds", "oz", "ounce", "ounces", "l", "liter",
                "liters", "litre", "litres", "ml", "s", "sec", "second", "seconds",
                "min", "minute", "minutes", "hr", "hour", "hours", "w", "watt", "watts",
                "kw", "mw", "hz", "khz", "mhz", "ghz", "kmh", "kph", "mph", "deg",
                "degree", "degrees", "byte", "bytes", "bit", "bits",
            ];
            let unit_stems: std::collections::BTreeSet<String> = UNIT_SURFACE
                .iter()
                .flat_map(|u| crate::search::stemmed_tokens(u))
                .collect();
            let measured_terms: Vec<String> = measured_property_terms(input.query)
                .into_iter()
                .filter(|t| !unit_stems.contains(t))
                .collect();
            let mut matched = false;
            'unit: for (_pid, verbatim, _, _) in input.passages {
                for sentence in &sentences(verbatim) {
                    for (token, exclusion) in quantity_candidates(sentence) {
                        // Decimal measurements ("172.5") are shape-identical to
                        // dotted versions; when the token carries the requested
                        // measurement unit it is a measurement, not a version.
                        let decimal_unit = exclusion.as_deref() == Some("version or identifier token")
                            && token.contains('.')
                            && adjacent_unit(sentence, &token) == Some(want);
                        if exclusion.is_some() && !decimal_unit {
                            continue;
                        }
                        if !token.chars().any(|c| c.is_ascii_digit()) {
                            continue;
                        }
                        if adjacent_unit(sentence, &token) != Some(want) {
                            continue;
                        }
                        let window_ok = measured_terms.is_empty()
                            || quantifying_window(sentence, &token, &measured_terms, 8);
                        if window_ok {
                            matched = true;
                            break 'unit;
                        }
                    }
                }
            }
            if !matched {
                trace.reasons.push(format!(
                    "requested-unit-verification: no measurement in the requested unit '{want}' bound to the measured property in the selected passages"
                ));
            }
        }
    }

    // ---- exact-date precision verification -----------------------------------
    // When the question demands calendar-day precision, at least one selected
    // passage must tie a real adjacent month-day date to the queried event in
    // the same sentence — or, for dated logs, in the section heading while the
    // event appears in the passage. Open "when" and year-only questions are
    // not constrained by this rule.
    if exact_date_query(input.query) {
        let framing = crate::search::framing_stems();
        let base_filter = |t: &&String| {
            !input.subject_terms.contains(*t)
                && !framing.contains(*t)
                && !VERIFY_CUES.contains(&t.as_str())
                && !PROPOSE_CUES.contains(&t.as_str())
                && !ATTEMPT_CUES.contains(&t.as_str())
                && !TRANSITION_CUES.contains(&t.as_str())
        };
        // Clause scoping: in a compound question, the exact-date requirement
        // binds only the event vocabulary of the clause(s) carrying the
        // exact-date trigger — sibling-clause terms ("which architect designed
        // it") must not inflate the overlap requirement.
        let clauses = question_clauses(input.query);
        let triggered: Vec<&String> = clauses
            .iter()
            .filter(|c| exact_date_query(c))
            .collect();
        let scoped: std::collections::BTreeSet<String> = if triggered.is_empty() {
            std::collections::BTreeSet::new()
        } else {
            let mut acc = std::collections::BTreeSet::new();
            for c in triggered {
                for t in stemmed_query_like(c) {
                    acc.insert(t);
                }
            }
            acc
        };
        let all_event: Vec<&String> = input.terms.iter().filter(base_filter).collect();
        let event_terms: Vec<&String> = if scoped.is_empty() {
            all_event.clone()
        } else {
            let in_scope: Vec<&String> = all_event
                .iter()
                .filter(|t| scoped.contains(t.as_str()) || scoped.iter().any(|st| stem_matches_eventually(t, st)))
                .cloned()
                .collect();
            if in_scope.is_empty() { all_event.clone() } else { in_scope }
        };
        let substantive_hit = |ss: &[String]| {
            let hits = event_terms
                .iter()
                .filter(|t| {
                    ss.iter().any(|st| {
                        stem_matches_eventually(t, st) || stem_matches_eventually(&dehyphen(t), &dehyphen(st))
                    })
                })
                .count();
            if event_terms.len() >= 2 {
                hits * 2 >= event_terms.len()
            } else {
                hits >= 1
            }
        };
        let mut tied = false;
        'outer: for (_pid, verbatim, section, _) in input.passages {
            for sentence in &sentences(verbatim) {
                let ss = stemmed_query_like(sentence);
                if crate::search::date_tier(sentence) >= 2.5 && substantive_hit(&ss) && !sentence_is_planned(sentence) {
                    tied = true;
                    break 'outer;
                }
            }
            if crate::search::date_tier(section) >= 2.5 {
                for sentence in &sentences(verbatim) {
                    let ss = stemmed_query_like(sentence);
                    if substantive_hit(&ss) && !sentence_is_planned(sentence) {
                        tied = true;
                        break 'outer;
                    }
                }
            }
        }
        if !tied {
            trace.reasons.push(
                "exact-date-precision: no adjacent month-day calendar date tied to the queried event in the selected passages"
                    .to_string(),
            );
        }
    }

    // ---- future-tense date guard ---------------------------------------------
    // "In what year will X ..." is not settled by bare historical past-event
    // dates: the matching sentence must carry explicit future/planned/
    // predicted cues alongside its date or year mention.
    if future_tense_date_query(input.query) {
        const FUTURE_CUES: &[&str] = &[
            "will", "scheduled", "schedule", "planned", "plan", "predicted", "predict",
            "expected", "expect", "projected", "project", "forecast", "proposed", "propose",
            "intend", "intended", "due", "set to", "slated", "anticipat",
        ];
        let framing = crate::search::framing_stems();
        let event_terms: Vec<&String> = input
            .terms
            .iter()
            .filter(|t| {
                !input.subject_terms.contains(*t)
                    && !framing.contains(*t)
                    && !VERIFY_CUES.contains(&t.as_str())
                    && !PROPOSE_CUES.contains(&t.as_str())
                    && !ATTEMPT_CUES.contains(&t.as_str())
                    && !TRANSITION_CUES.contains(&t.as_str())
            })
            .collect();
        let mut future_dated = false;
        'fut: for (_pid, verbatim, _, _) in input.passages {
            for sentence in &sentences(verbatim) {
                let low = sentence.to_ascii_lowercase();
                let has_future_cue = FUTURE_CUES.iter().any(|c| low.contains(c));
                if !has_future_cue {
                    continue;
                }
                let ss = stemmed_query_like(sentence);
                let hits = event_terms
                    .iter()
                    .filter(|t| {
                        ss.iter().any(|st| {
                            stem_matches_eventually(t, st) || stem_matches_eventually(&dehyphen(t), &dehyphen(st))
                        })
                    })
                    .count();
                let event_hit = if event_terms.len() >= 2 { hits * 2 >= event_terms.len() } else { hits >= 1 };
                if !event_hit {
                    continue;
                }
                let has_date_or_year = crate::search::date_tier(sentence) >= 1.0
                    || sentence.split_whitespace().any(|w| {
                        let b = w.trim_matches(|c: char| !c.is_ascii_digit());
                        b.len() == 4 && b.chars().all(|c| c.is_ascii_digit())
                    });
                if has_date_or_year {
                    future_dated = true;
                    break 'fut;
                }
            }
        }
        if !future_dated {
            trace.reasons.push(
                "future-event-date: no explicit future/planned date tied to the queried event in the selected passages"
                    .to_string(),
            );
        }
    }

    // ---- past-tense occurrence guard -----------------------------------------
    // A past-tense "did" date/year question is not settled by planned or
    // future-modal date mentions. Scan for any realized (non-planned) dated
    // sentence tied to the event; if every dated event mention is planned or
    // future-modal, abstain with a typed reason.
    if past_tense_date_query(input.query) {
        let framing = crate::search::framing_stems();
        let qualifiers = mission_qualifiers(input.query);
        let event_terms: Vec<&String> = input
            .terms
            .iter()
            .filter(|t| {
                !input.subject_terms.contains(*t)
                    && !framing.contains(*t)
                    && !VERIFY_CUES.contains(&t.as_str())
                    && !PROPOSE_CUES.contains(&t.as_str())
                    && !ATTEMPT_CUES.contains(&t.as_str())
                    && !TRANSITION_CUES.contains(&t.as_str())
            })
            .collect();
        let mut realized = false;
        let mut planned_only = false;
        for (_pid, verbatim, section, _) in input.passages {
            for sentence in &sentences(verbatim) {
                let ss = stemmed_query_like(sentence);
                // Substantive event overlap: at least one action/event stem,
                // and when the query carries 2+ event terms a single
                // background noun never suffices — require the verb-class or
                // majority overlap. Hyphen-insensitive matching.
                let hits = event_terms
                    .iter()
                    .filter(|t| {
                        ss.iter().any(|st| {
                            stem_matches_eventually(t, st) || stem_matches_eventually(&dehyphen(t), &dehyphen(st))
                        })
                    })
                    .count();
                let event_hit = if event_terms.len() >= 2 {
                    hits * 2 >= event_terms.len()
                } else {
                    hits >= 1
                };
                if !event_hit {
                    continue;
                }
                let has_year = sentence.split_whitespace().any(|w| {
                    let b = w.trim_matches(|c: char| !c.is_ascii_digit());
                    b.len() == 4 && b.chars().all(|c| c.is_ascii_digit())
                });
                let dated = has_year || crate::search::date_tier(sentence) >= 1.0;
                if !dated {
                    continue;
                }
                // Subject binding: when the query names a mission/ordinal
                // qualifier, only a sentence (or its own single-mission
                // section heading) mentioning that exact qualifier can count.
                // Sibling missions never transfer realization.
                if !qualifiers.is_empty() {
                    let qhit = qualifiers.iter().any(|qv| {
                        sentence_mentions_qualifier(sentence, qv)
                            || sentence_mentions_qualifier(section, qv)
                    });
                    if !qhit {
                        continue;
                    }
                } else {
                    // Sub-component distractor guard: a sentence whose section
                    // names a specific component and which never mentions the
                    // queried subject itself cannot realize the main subject's
                    // event.
                    let subj_hit = input.subject_terms.iter().any(|st| ss.contains(st));
                    let sec = stemmed_query_like(section);
                    let section_specific = !section.is_empty()
                        && input.subject_terms.iter().all(|st| !sec.contains(st));
                    if !subj_hit && section_specific {
                        continue;
                    }
                }
                if sentence_is_planned(sentence) {
                    planned_only = true;
                } else {
                    realized = true;
                }
            }
        }
        if planned_only && !realized {
            trace.reasons.push(
                "date-occurrence: only planned/future-modal date mentions found for past-tense event query"
                    .to_string(),
            );
        }
    }

    // ---- SQuAD 2.0 extractive unanswerability verification -------------------
    // Activation: extractive wh-/factual questions only (yes/no state questions
    // are carried by the deterministic checks). Frozen threshold tau = 0.0 —
    // the natural SQuAD 2.0 boundary where the best extracted span must
    // strictly beat the [CLS] no-answer score on at least one selected passage.
    // Exposed calibration (acc-xfer-v4): answerable hits +4.6..+17.2, undated
    // sibling trap -4.47, missing-unit borderline positive — zero false
    // abstention margin at tau=0 for every answerable exposed case measured.
    if qa_extractive_question(input.query) {
        let mut max_margin = f32::NEG_INFINITY;
        let mut best_span = String::new();
        for (pid, verbatim, section, _) in input.passages {
            let article = pid.split('-').next().unwrap_or("");
            let text = format!("{article} — {section}\n{verbatim}");
            if let Some(v) = crate::qa_verify::verify_pair(input.query, &text) {
                if v.margin > max_margin {
                    max_margin = v.margin;
                    best_span = v.span.clone();
                }
            }
        }
        trace.qa_margin = Some(max_margin);
        trace.qa_span = best_span.clone();
        // Inert when the verifier is absent (no verdict obtained on any
        // passage): negative-infinity is the no-data sentinel, not evidence.
        if max_margin.is_finite() && max_margin < 0.0 {
            trace.reasons.push(format!(
                "qa-unanswerable: max span-vs-null margin {max_margin:.2} below threshold 0.00 (best span `{best_span}`)"
            ));
        }

    }

    // ---- state/transition evidence --------------------------------------------
    if input.transition_check {
        // Generic occurrence verbs (occur/happen) are not verification cues:
        // "on what date did the eruption occur" is an extractive date question,
        // not an operational-state question. Genuine state cues (verified,
        // complete, deployed, ...) still activate, including on wh-questions.
        // Activation lexicon: generic occurrence verbs (occur/happen) never
        // activate. On extractive wh-questions (on what date did X begin),
        // only strict operational-state cues activate — weak activity verbs
        // (begin/start/open/perform/take/reach/continu) are ordinary event
        // vocabulary there. Yes/no state questions keep the full lexicon.
        const STRICT_STATE_CUES: &[&str] = &[
            "verifi", "confirm", "unverifi", "unconfirm", "deploy", "complet",
            "finish", "pass", "fail", "cancel", "skip", "live",
        ];
        let wh_extractive = qa_extractive_question(input.query);
        let activation_terms: Vec<String> = input
            .terms
            .to_vec()
            .into_iter()
            .filter(|t| t != "occur" && t != "happen")
            .filter(|t| {
                if !wh_extractive {
                    return true;
                }
                // On wh-extractive questions a term activates only when it is
                // a strict state cue or not a cue at all; weak cues are
                // dropped so they cannot manufacture a verify requirement.
                !matches!(t.as_str(), "begin" | "began" | "start" | "open" | "perform" | "take" | "reach" | "continu" | "underway" | "run" | "succeed")
                    || STRICT_STATE_CUES.contains(&t.as_str())
            })
            .collect();
        if let Some(required) = cue_class_of(&activation_terms) {
            let q_env = {
                let qt = crate::search::query_terms(input.query);
                env_of(&qt)
            };
            let mut supportive = false;
            for (pid, verbatim, section, _) in input.passages {
                let passage_idents: std::collections::BTreeSet<String> = crate::search::stemmed_tokens(verbatim)
                    .into_iter()
                    .filter(|t| t.chars().any(|c| c.is_ascii_digit()) && t.chars().any(|c| c.is_alphabetic()))
                    .collect();
                let inherited: Option<String> = if passage_idents.len() == 1 {
                    passage_idents.iter().next().cloned()
                } else {
                    None
                };
                // Experiment D: the heading is the entry's date/context —
                // retrieval already indexes it, acceptance now sees it too.
                let heading_tokens: Vec<String> = if input.context_chain {
                    stemmed_query_like(section)
                } else {
                    Vec::new()
                };
                // Bounded chain: when the passage has exactly one subject
                // identifier, continuation sentences inherit the entry's
                // antecedent context. A second identifier (two subjects,
                // subject switch) disables chaining entirely; a cue sentence
                // carrying its own identifier is never augmented.
                let chainable = input.context_chain && passage_idents.len() == 1;
                let passage_sentences = sentences(verbatim);
                let mut antecedent_tokens: Vec<String> = Vec::new();
                for sentence in &passage_sentences {
                    let own_stemmed = stemmed_query_like(sentence);
                    let mut sent_stemmed = own_stemmed.clone();
                    let own_ident = sent_stemmed.iter().any(|t| {
                        t.chars().any(|c| c.is_ascii_digit()) && t.chars().any(|c| c.is_alphabetic())
                    });
                    if chainable && !own_ident {
                        for t in &antecedent_tokens {
                            if !sent_stemmed.contains(t) {
                                sent_stemmed.push(t.clone());
                            }
                        }
                    }
                    for t in stemmed_query_like(sentence) {
                        if !antecedent_tokens.contains(&t) {
                            antecedent_tokens.push(t);
                        }
                    }
                    for t in &heading_tokens {
                        if !sent_stemmed.contains(t) {
                            sent_stemmed.push(t.clone());
                        }
                    }
                    if let Some(idt) = &inherited {
                        if !sent_stemmed.contains(idt) {
                            sent_stemmed.push(idt.clone());
                        }
                    }
                    let object_terms: Vec<String> = input
                        .terms
                        .iter()
                        .filter(|t| {
                            !input.subject_terms.contains(*t)
                                && !VERIFY_CUES.contains(&t.as_str())
                                && !PROPOSE_CUES.contains(&t.as_str())
                                && !ATTEMPT_CUES.contains(&t.as_str())
                                && !TRANSITION_CUES.contains(&t.as_str())
                                && !qenv_stems.contains(*t)
                                && sent_stemmed.contains(*t)
                        })
                        .cloned()
                        .collect();
                    // The sentence must mention every object term: evidence
                    // about a different revision or a different aspect of the
                    // subject is not evidence for this question.
                    let object_all: Vec<&String> = input
                        .terms
                        .iter()
                        .filter(|t| {
                            !input.subject_terms.contains(*t)
                                && !VERIFY_CUES.contains(&t.as_str())
                                && !PROPOSE_CUES.contains(&t.as_str())
                                && !ATTEMPT_CUES.contains(&t.as_str())
                                && !TRANSITION_CUES.contains(&t.as_str())
                                && !qenv_stems.contains(*t)
                        })
                        .collect();
                    let ident_ok = object_all.iter()
                        .filter(|t| t.chars().any(|c| c.is_ascii_digit()))
                        .all(|t| sent_stemmed.contains(*t));
                    let ordinary: Vec<&&String> = object_all.iter()
                        .filter(|t| !t.chars().any(|c| c.is_ascii_digit()))
                        .collect();
                    let hits = ordinary.iter().filter(|t| sent_stemmed.contains(**t)).count();
                    let ordinary_ok = ordinary.is_empty() || hits * 2 >= ordinary.len();
                    if object_terms.is_empty() || !ident_ok || !ordinary_ok {
                        continue;
                    }
                    let classes = cue_terms_in(&sent_stemmed);
                    if classes.is_empty() {
                        continue;
                    }
                    let mut span = StateSpan {
                        passage_id: pid.clone(),
                        sentence: sentence.clone(),
                        cue_class: classes.join("+"),
                        negated: negated(sentence),
                        environment: env_of(&sent_stemmed),
                        object_terms,
                        verdict: "unresolved".to_string(),
                    };
                    let env_ok = match (&q_env, &span.environment) {
                        (Some(qe), Some(se)) => qe == se,
                        (Some(_), None) => false,
                        (None, _) => true,
                    };
                    let class_ok = classes.iter().any(|c| class_satisfies(required, c));
                    let mut verdict = "unresolved".to_string();
                    // Experiment E: positive support is action-aware. The
                    // evidence sentence must contain a cue stem that establishes
                    // the asked action or a justified equivalent — class
                    // membership alone never suffices.
                    let action_ok = if input.action_check {
                        question_predicate(&input.terms.to_vec()).map(|qp| {
                            // Clause-level event identity: when the sentence
                            // separates activities into clauses, the queried
                            // activity's object terms must share the clause
                            // carrying the outcome cue. A competing clause
                            // cannot donate its outcome. Single-clause and
                            // chained continuation sentences are unaffected.
                            let cls = clauses(sentence);
                            let qobj: Vec<&String> = input.terms.iter()
                                .filter(|t| !input.subject_terms.contains(*t)
                                    && !t.chars().any(|c| c.is_ascii_digit())
                                    && action_group(t).is_none()
                                    && !qenv_stems.contains(*t))
                                .collect();
                            let cue_in_own = own_stemmed.iter().any(|t| action_supports(&qp, t));
                            if cls.len() <= 1 {
                                cue_in_own
                            } else {
                                // Invariant: a claimed outcome must belong to the
                                // asked activity — explicitly in its clause, or
                                // through inheritance when the sentence never
                                // names the activity elsewhere. If the sentence's
                                // own text places the queried activity (subject OR
                                // object terms) in a clause that carries no outcome
                                // cue, the outcome belongs to a different activity
                                // and inherited context cannot override that
                                // attribution. Pure continuations — the activity
                                // absent from every clause — still inherit.
                                let stem_cls: Vec<Vec<String>> = cls.iter().map(|c| stemmed_query_like(c)).collect();
                                let act_terms: Vec<&String> = input.terms.iter()
                                    .filter(|t| !t.chars().any(|c| c.is_ascii_digit())
                                        && action_group(t).is_none()
                                        && !qenv_stems.contains(*t))
                                    .collect();
                                let act_only_elsewhere = act_terms.iter().any(|o| {
                                    stem_cls.iter().any(|sc| sc.contains(*o))
                                        && !stem_cls.iter().any(|sc2| sc2.iter().any(|t| action_supports(&qp, t)) && sc2.contains(*o))
                                });
                                let _ = &qobj;
                                cue_in_own && !act_only_elsewhere
                            }
                        }).unwrap_or(false)
                    } else {
                        true
                    };
                    if class_ok && env_ok && action_ok {
                        if !span.negated {
                            verdict = "supports".to_string();
                        } else if input.proposition_check {
                            // Contradiction requires the negated predicate to
                            // match the asked predicate group: non-confirmation
                            // does not contradict deployment, only confirmation.
                            // Contradiction is predicate-exact: a negation only
                            // contradicts the predicate it actually states.
                            // "The transfer did not complete" contradicts
                            // completion, never the integrity check passing.
                            // The un-prefixed forms pair with their positive
                            // stem (unverified/verified); nothing broader.
                            let q_pred = question_predicate(&input.terms.to_vec());
                            let pred_match = q_pred.as_ref().map(|qp| {
                                own_stemmed.iter().any(|t| {
                                    t == qp
                                        || (qp == "verifi" && t == "unverifi")
                                        || (qp == "confirm" && t == "unconfirm")
                                        || (qp == "pass" && t == "fail")
                                })
                            }).unwrap_or(false);
                            if pred_match {
                                verdict = "contradicts".to_string();
                            }
                        }
                    }
                    if verdict == "supports" {
                        supportive = true;
                    }
                    if input.proposition_check && verdict == "contradicts" {
                        supportive = true; // an explicit negative settles yes/no
                    }
                    span.verdict = verdict;
                    trace.state_spans.push(span);
                }
            }
            if !supportive {
                let mode = if input.proposition_check { "supports or explicitly contradicts" } else { "establishes" };
                trace.reasons.push(format!(
                    "transition-check: no sentence {mode} the required '{required}' state with compatible predicate and environment"
                ));
            }
        }
    }

    // ---- M2 semantic-aware coverage calibration -------------------------------
    // The coverage gates are lexical by construction: a paraphrased query
    // ("jogging" for a "running" note, "abandon" for "gave up") shares few or
    // no stems with its evidence and false-abstains even when dense retrieval
    // surfaced the right passage and the QA verifier extracts a confident
    // span. When (a) a dense-recalled candidate is among the selected
    // passages and (b) the QA span-vs-null margin is strongly positive, the
    // coverage reasons — and ONLY the coverage reasons — are waived. Every
    // typed deterministic check (quantity binding, unit verification,
    // transition/proposition cues) and the QA-unanswerability guard itself
    // are untouched, so abstention integrity is preserved exactly.
    let override_margin: f32 = std::env::var("GANESH_SEMANTIC_OVERRIDE_MARGIN")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(1.0);
    // Measurement/quantity questions are excluded: a missing measurement is
    // exact-evidence territory, and dense retrieval must never conjure one.
    if input.dense_support && !input.quantity_question {
        if let Some(m) = trace.qa_margin {
            if m.is_finite() && m >= override_margin {
                trace.reasons.retain(|r| {
                    !r.starts_with("idf-union-coverage")
                        && !r.starts_with("union-coverage")
                        && !r.starts_with("top-coverage")
                });
            }
        }
    }

    trace.decision = if trace.reasons.is_empty() {
        "ok".to_string()
    } else {
        "insufficient_context".to_string()
    };
    trace
}

#[cfg(test)]
mod sufficiency_tests {
    use super::*;

    fn mkpass(text: &str) -> Vec<(String, String, String, Vec<String>)> {
        vec![("p1".to_string(), text.to_string(), String::new(), Vec::new())]
    }

    fn nosubj() -> std::collections::BTreeSet<String> {
        std::collections::BTreeSet::new()
    }

    fn decide_one(query: &str, terms: Vec<String>, text: &str) -> AcceptanceTrace {
        let passages = mkpass(text);
        let input = DecideInput {
            query,
            terms: &terms,
            subject_terms: &nosubj(),
            passages: &passages,
            idf_union_coverage: 1.0,
            union_coverage: 1.0,
            top_coverage: 1.0,
            min_idf_union_coverage: 0.75,
            min_union_coverage: 0.0,
            min_top_coverage: 0.0,
            quantity_question: true,
            quantity_unit_missing: false,
            any_numeric_token: true,
            quantity_binding: true,
            dense_support: false,
            transition_check: false,
            proposition_check: false,
            context_chain: true,
            action_check: false,
        };
        decide(&input)
    }

    #[test]
    fn pronominal_one_and_code_literals_do_not_satisfy_operator_count() {
        // v30 class: "use of one in place of" and code booleans are not counts.
        let terms: Vec<String> = vec!["oper".to_string(), "c".to_string(), "defin".to_string()];
        let t1 = decide_one(
            "How many operators does C define?",
            terms.clone(),
            "C makes accidental use of one in place of another. The expression == 1 and != 0 are boolean tests of operators.",
        );
        assert!(t1.decision != "ok", "pronominal one and code literals must not bind");
        let t2 = decide_one("How many operators does C define?", terms, "C defines 45 operators in total.");
        assert_eq!(t2.decision, "ok", "a real count near the property must bind");
    }

    #[test]
    fn real_counts_pass_quantifying_window() {
        let terms: Vec<String> = vec!["window".to_string(), "build".to_string()];
        let t = decide_one(
            "How many windows does the Empire State Building contain?",
            terms,
            "The building also contains 6,514 windows.",
        );
        assert_eq!(t.decision, "ok");
        let t2 = decide_one(
            "How many watts does the array draw?",
            vec!["watt".to_string(), "array".to_string()],
            "The array draws 110 watts at peak.",
        );
        assert_eq!(t2.decision, "ok");
    }

    #[test]
    fn measurement_query_requires_unit_evidence() {
        // i05 class: measurement verification fires, abstains when absent,
        // passes when the unit measurement is present.
        let prop = measured_property_terms("What is the diameter of Hubble's main mirror in meters?");
        assert!(prop.iter().any(|t| t.starts_with("diamet")), "diameter must be extracted: {:?}", prop);
        assert!(prop.iter().any(|t| t.starts_with("meter") || t == "m"), "unit must be extracted: {:?}", prop);
        let terms: Vec<String> = vec!["diamet".to_string(), "meter".to_string(), "hubble".to_string()];
        let miss = decide_one(
            "What is the diameter of Hubble's main mirror in meters?",
            terms.clone(),
            "The main mirror of the telescope was carefully polished over several years.",
        );
        assert!(miss.decision != "ok", "no measurement present must abstain");
        let hit = decide_one(
            "What is the diameter of Hubble's main mirror in meters?",
            terms,
            "The main mirror has a diameter of 2.4 meters.",
        );
        assert_eq!(hit.decision, "ok", "present measurement must pass");
    }

    #[test]
    fn exact_date_requires_tier3_tied_to_event() {
        // i04 class: year-only evidence must not satisfy exact-day questions.
        assert!(exact_date_query("On what exact calendar date did Congress approve funding?"));
        assert!(!exact_date_query("When was the rover launched?"));
        assert!(!exact_date_query("In what year did construction begin?"));
    }

    #[test]
    fn exact_date_passes_when_tier3_tied() {
        let terms: Vec<String> = vec!["approv".to_string(), "fund".to_string()];
        let passages = mkpass("Congress approved the funding on 3 May 1978 for the program.");
        let input = DecideInput {
            query: "On what exact date did Congress approve the funding?",
            terms: &terms,
            subject_terms: &nosubj(),
            passages: &passages,
            idf_union_coverage: 1.0,
            union_coverage: 1.0,
            top_coverage: 1.0,
            min_idf_union_coverage: 0.75,
            min_union_coverage: 0.0,
            min_top_coverage: 0.0,
            quantity_question: false,
            quantity_unit_missing: false,
            any_numeric_token: false,
            quantity_binding: false,
            dense_support: false,
            transition_check: false,
            proposition_check: false,
            context_chain: true,
            action_check: false,
        };
        let tr = decide(&input);
        assert!(!tr.reasons.iter().any(|r| r.contains("exact-date-precision")), "tied tier-3 must satisfy: {:?}", tr.reasons);
    }

    #[test]
    fn exact_date_abstains_on_year_only() {
        let terms: Vec<String> = vec!["approv".to_string(), "fund".to_string()];
        let passages = mkpass("Congress approved the funding in 1978 for the program.");
        let input = DecideInput {
            query: "On what exact calendar date did Congress approve funding?",
            terms: &terms,
            subject_terms: &nosubj(),
            passages: &passages,
            idf_union_coverage: 1.0,
            union_coverage: 1.0,
            top_coverage: 1.0,
            min_idf_union_coverage: 0.75,
            min_union_coverage: 0.0,
            min_top_coverage: 0.0,
            quantity_question: false,
            quantity_unit_missing: false,
            any_numeric_token: false,
            quantity_binding: false,
            dense_support: false,
            transition_check: false,
            proposition_check: false,
            context_chain: true,
            action_check: false,
        };
        let tr = decide(&input);
        assert!(tr.reasons.iter().any(|r| r.contains("exact-date-precision")), "year-only must fail precision: {:?}", tr.reasons);
    }

    #[test]
    fn activity_noun_test_survives_attribution() {
        // i01 class: "test" as activity noun is not a deployment environment;
        // prepositional environments still are.
        assert!(query_env_stems("Did the germination test of seed lot G12 finish on January 7?").is_empty());
        assert!(!query_env_stems("Was the service deployed to test?").is_empty());
        assert!(!query_env_stems("Was the fix verified in production?").is_empty());
    }

    #[test]
    fn framing_words_do_not_penalize_coverage() {
        // v03/v23 class: framing stems are recognized as answer-shape words.
        let fs = crate::search::framing_stems();
        for w in ["exact", "calendar", "kind", "type", "country"] {
            let stemmed: Vec<String> = crate::search::stemmed_tokens(w);
            assert!(stemmed.iter().any(|t| fs.contains(t)), "{w} must be a framing stem");
        }
    }
}

#[cfg(test)]
mod surgical_fix_tests {
    use super::*;

    #[test]
    fn versions_stay_excluded_decimals_with_units_pass() {
        // qA06/v28 class: version-adjacent decimals never un-excluded.
        assert!(!unit_adjacent_decimal("In 2021, the Novolang 3.0 keyword list was revised.", "3.0"));
        assert!(!unit_adjacent_decimal("New features in 2.0 included static member functions.", "2.0"));
        // supported measurement class: unit-adjacent decimals un-excluded.
        assert!(unit_adjacent_decimal("with the proposed mirror diameter reduced from 3 m to 2.4 m", "2.4"));
        assert!(unit_adjacent_decimal("a height of 5.895 km above sea level", "5.895"));
        assert!(unit_adjacent_decimal("averaging 21.3 million km from Jupiter", "21.3"));
    }

    #[test]
    fn irregular_verbs_canonicalize() {
        assert_eq!(irregular_canon("sank"), "sink");
        assert_eq!(irregular_canon("sunk"), "sink");
        assert_eq!(irregular_canon("flew"), "fly");
        assert_eq!(irregular_canon("began"), "begin");
        assert_eq!(irregular_canon("built"), "build");
        assert!(stem_matches_eventually("sink", "sank"));
        assert!(stem_matches_eventually("fly", "flew"));
        assert!(!stem_matches_eventually("sink", "swam"));
    }

    #[test]
    fn planned_future_guard() {
        assert!(sentence_is_planned("NASA plans to deorbit the station in 2030."));
        assert!(sentence_is_planned("Artemis III, scheduled for late 2027, is planned to test a lander."));
        assert!(!sentence_is_planned("The station deorbited in March 2001."));
        assert!(past_tense_date_query("In what year did the station deorbit?"));
        assert!(past_tense_date_query("In what year did Artemis III land astronauts on the Moon?"));
        assert!(!past_tense_date_query("What kind of planet is Jupiter?"));
    }
}

#[cfg(test)]
mod occurrence_guard_tests {
    use super::*;

    #[test]
    fn mission_qualifier_extraction() {
        assert_eq!(mission_qualifiers("In what year did Artemis III land astronauts on the Moon?"), vec!["iii"]);
        assert!(mission_qualifiers("When was Voyager 2 launched?").contains(&"2".to_string()));
        assert!(mission_qualifiers("What kind of planet is Jupiter?").is_empty());
    }

    #[test]
    fn qualifier_sentence_binding() {
        assert!(sentence_mentions_qualifier("Artemis III is planned for late 2027.", "iii"));
        assert!(!sentence_mentions_qualifier("Artemis I flew on November 16, 2022.", "iii"));
        assert!(sentence_mentions_qualifier("Voyager 2 launched on August 20, 1977.", "2"));
        assert!(!sentence_mentions_qualifier("Voyager 1 launched in September 1977.", "2"));
    }

    #[test]
    fn hyphen_insensitive_event_match() {
        assert!(stem_matches_eventually(&dehyphen("de-orbit"), &dehyphen("deorbit")));
        assert_eq!(dehyphen("de-orbit"), "deorbit");
    }

    #[test]
    fn planned_development_cues() {
        assert!(sentence_is_planned("In June 2024, NASA selected SpaceX to develop the U.S. Deorbit Vehicle."));
        assert!(sentence_is_planned("NASA announced a planned date of January 2031 to de-orbit the ISS."));
        assert!(!sentence_is_planned("Pirs was deorbited on 26 July 2021."));
    }
}

