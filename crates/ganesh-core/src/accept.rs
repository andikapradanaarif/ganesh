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
    for part in lowered.split(';') {
        for sub in part.split(" while ") {
            for piece in sub.split(", ") {
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

/// Predicate groups for contradiction matching. A negated statement only
/// contradicts the asked proposition when the negated predicate is the same
/// one — "did not confirm" contradicts confirmed(R6) but says nothing about
/// deployed(R6). Support may use the looser class hierarchy; contradiction
/// may not.
const PREDICATE_GROUPS: &[&[&str]] = &[
    &["verifi", "confirm", "unverifi", "unconfirm"],
    &["deploy", "run", "live"],
    &["start"],
    &["pass", "succeed", "complet"],
    &["propos", "plan", "intend", "suggest", "consider"],
    &["attempt", "tri", "effort"],
    &["becam", "becom", "option", "mandatori", "deprecat", "drop", "adopt"],
];

fn predicate_group(stem: &str) -> Option<usize> {
    PREDICATE_GROUPS.iter().position(|g| g.contains(&stem))
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
        if let Some(lit) = standalone_digits(&low) {
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
    let Some(mut i) = start else { return Vec::new() };
    while i < toks.len() && EXTRACTION_FUNCTION_WORDS.contains(&toks[i].as_str()) {
        i += 1;
    }
    let mut out = Vec::new();
    while i < toks.len() && !EXTRACTION_FUNCTION_WORDS.contains(&toks[i].as_str()) {
        let stemmed = crate::search::stemmed_tokens(&toks[i]);
        out.extend(stemmed);
        i += 1;
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

pub fn decide(input: &DecideInput) -> AcceptanceTrace {
    let mut trace = AcceptanceTrace::default();

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
                        let mention_exclusion = match (&exclusion, version_ok) {
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

    // ---- state/transition evidence --------------------------------------------
    if input.transition_check {
        if let Some(required) = cue_class_of(&input.terms.to_vec()) {
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
                                && !ENV_STEMS.contains(&t.as_str())
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
                                && !ENV_STEMS.contains(&t.as_str())
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
                                    && !ENV_STEMS.contains(&t.as_str()))
                                .collect();
                            let cue_in_own = own_stemmed.iter().any(|t| action_supports(&qp, t));
                            if cls.len() <= 1 {
                                cue_in_own
                            } else {
                                let stem_cls: Vec<Vec<String>> = cls.iter().map(|c| stemmed_query_like(c)).collect();
                                let cue_clause_has_obj = stem_cls.iter().any(|sc| {
                                    sc.iter().any(|t| action_supports(&qp, t))
                                        && qobj.iter().any(|o| sc.contains(*o))
                                });
                                let obj_only_elsewhere = qobj.iter().any(|o| {
                                    stem_cls.iter().any(|sc| sc.contains(*o))
                                        && !stem_cls.iter().any(|sc2| sc2.iter().any(|t| action_supports(&qp, t)) && sc2.contains(*o))
                                });
                                cue_in_own && (cue_clause_has_obj || !obj_only_elsewhere)
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

    trace.decision = if trace.reasons.is_empty() {
        "ok".to_string()
    } else {
        "insufficient_context".to_string()
    };
    trace
}
