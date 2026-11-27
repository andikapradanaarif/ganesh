//! Local fact-checking encoder — INT8 ONNX, no generative LLM.
//!
//! Frozen configuration (see EVALUATION-acc-xfer-v6.md for provenance):
//! - Model: DeBERTa-v3-small fine-tuned on FEVER+DocNLI+LingNLI (binary),
//!   dynamically quantized INT8 ONNX, sha256 logged per report.
//! - Runtime: ONNX Runtime CPU execution provider via `ort`, zero network.
//! - Input: [CLS] premise [SEP] declarative_claim [SEP], capped at 512.
//! - Invoked ONLY on top-3 passages when qa_margin >= 0.0.

use ort::session::{builder::GraphOptimizationLevel, Session};
use ort::value::Tensor;
use std::sync::{Mutex, OnceLock};
use tokenizers::Tokenizer;

struct FcModel {
    session: Mutex<Session>,
    tokenizer: Tokenizer,
    pub entailment_idx: usize,
}

static MODEL: OnceLock<Option<FcModel>> = OnceLock::new();

fn model_dir() -> String {
    std::env::var("GANESH_FACTCHECK_MODEL_DIR")
        .unwrap_or_else(|_| "fixtures/factcheck-model".to_string())
}

fn load() -> Option<FcModel> {
    let dir = model_dir();
    let tokenizer = Tokenizer::from_file(format!("{dir}/tokenizer.json")).ok()?;
    let weights = format!("{dir}/model-int8.onnx");
    if !std::path::Path::new(&weights).exists() {
        return None;
    }
    // id2label: read entailment index from config.json when present.
    let entailment_idx = std::fs::read_to_string(format!("{dir}/config.json"))
        .ok()
        .and_then(|c| serde_json::from_str::<serde_json::Value>(&c).ok())
        .and_then(|v| {
            v.get("id2label").and_then(|m| {
                m.as_object().and_then(|obj| {
                    obj.iter()
                        .find(|(_, name)| name.as_str().unwrap_or("").to_lowercase().contains("entail"))
                        .and_then(|(k, _)| k.parse::<usize>().ok())
                })
            })
        })
        .unwrap_or(0);
    let session = Session::builder().ok()?
        .with_optimization_level(GraphOptimizationLevel::Level3).ok()?
        .with_intra_threads(2).ok()?
        .commit_from_file(weights).ok()?;
    Some(FcModel { session: Mutex::new(session), tokenizer, entailment_idx })
}

pub struct FcVerdict {
    pub entailment: f32,
    pub other: f32,
}

/// Verify one (premise, hypothesis) pair. None when the model is unavailable.
pub fn verify_pair(premise: &str, hypothesis: &str) -> Option<FcVerdict> {
    let m = MODEL.get_or_init(load).as_ref()?;
    let p_trunc: String = premise.split_whitespace().take(384).collect::<Vec<_>>().join(" ");
    let h_trunc: String = hypothesis.split_whitespace().take(96).collect::<Vec<_>>().join(" ");
    let encoding = m.tokenizer.encode((p_trunc.as_str(), h_trunc.as_str()), true).ok()?;
    let mut ids: Vec<i64> = encoding.get_ids().iter().map(|&x| x as i64).collect();
    let mut mask: Vec<i64> = encoding.get_attention_mask().iter().map(|&x| x as i64).collect();
    ids.truncate(512);
    mask.truncate(512);
    let len = ids.len();
    let shape = vec![1usize, len];
    let ids_t = Tensor::from_array((shape.clone(), ids)).ok()?;
    let mask_t = Tensor::from_array((shape, mask)).ok()?;
    let probs = {
        let mut session = m.session.lock().ok()?;
        // DeBERTa-v3 ONNX exports omit token_type_ids.
        let inputs = ort::inputs![ids_t, mask_t];
        let outputs = session.run(inputs).ok()?;
        let (_shape, data) = outputs[0].try_extract_tensor::<f32>().ok()?;
        let max = data.iter().cloned().fold(f32::NEG_INFINITY, f32::max);
        let exps: Vec<f32> = data.iter().map(|x| (x - max).exp()).collect();
        let sum: f32 = exps.iter().sum();
        exps.iter().map(|x| x / sum).collect::<Vec<_>>()
    };
    let entailment = probs.get(m.entailment_idx).cloned().unwrap_or(0.0);
    let other = 1.0 - entailment;
    Some(FcVerdict { entailment, other })
}

pub fn available() -> bool {
    MODEL.get_or_init(load).is_some()
}

pub fn status_summary() -> String {
    let dir = model_dir();
    let weights = format!("{dir}/model-int8.onnx");
    if std::path::Path::new(&weights).exists() && available() {
        let digest = sha256_prefix(&weights);
        format!("factcheck: active (model {dir}/model-int8.onnx int8-onnx, sha256 {digest})")
    } else {
        "factcheck: absent".to_string()
    }
}

fn sha256_prefix(path: &str) -> String {
    use std::io::Read;
    let mut f = match std::fs::File::open(path) {
        Ok(f) => f,
        Err(_) => return "unknown".to_string(),
    };
    let mut buf = Vec::new();
    if f.read_to_end(&mut buf).is_err() {
        return "unknown".to_string();
    }
    crate::citation::sha256_hex(&buf).chars().take(12).collect()
}

pub fn enforce_requirement(caller: &str) {
    if std::env::var("GANESH_REQUIRE_FACTCHECK").ok().as_deref() == Some("1") && !available() {
        eprintln!(
            "FATAL: GANESH_REQUIRE_FACTCHECK=1 but the factcheck model failed to load from `{}` ({caller}). Refusing silent skip.",
            model_dir()
        );
        std::process::exit(2);
    }
}

// ---------------------------------------------------------------------------
// Deterministic question+span -> declarative claim conversion.
// ---------------------------------------------------------------------------

const IRREGULAR_PAST: &[(&str, &str)] = &[
    ("win", "won"), ("begin", "began"), ("take", "took"), ("break", "broke"),
    ("choose", "chose"), ("become", "became"), ("fall", "fell"), ("fly", "flew"),
    ("grow", "grew"), ("know", "knew"), ("leave", "left"), ("make", "made"),
    ("rise", "rose"), ("see", "saw"), ("sing", "sang"), ("sink", "sank"),
    ("speak", "spoke"), ("steal", "stole"), ("swim", "swam"), ("write", "wrote"),
    ("go", "went"), ("come", "came"), ("run", "ran"), ("give", "gave"),
    ("build", "built"), ("buy", "bought"), ("bring", "brought"), ("catch", "caught"),
    ("fight", "fought"), ("find", "found"), ("hold", "held"), ("lead", "led"),
    ("lose", "lost"), ("meet", "met"), ("pay", "paid"), ("sell", "sold"),
    ("send", "sent"), ("spend", "spent"), ("stand", "stood"), ("teach", "taught"),
    ("win", "won"), ("hit", "hit"), ("cost", "cost"), ("put", "put"), ("shut", "shut"),
    ("split", "split"), ("spread", "spread"), ("burst", "burst"), ("bet", "bet"),
];

fn past_tense(verb: &str) -> String {
    let low = verb.to_lowercase();
    for (base, past) in IRREGULAR_PAST {
        if low == *base {
            return past.to_string();
        }
    }
    if low.ends_with('e') {
        return format!("{low}d");
    }
    if low.ends_with('y') && low.len() > 2 {
        let c = low.chars().nth(low.len() - 2).unwrap_or('a');
        if !"aeiou".contains(c) {
            return format!("{}ied", &low[..low.len() - 1]);
        }
    }
    format!("{low}ed")
}

const VERB_SKIP: &[&str] = &[
    "the", "a", "an", "of", "to", "in", "on", "at", "by", "for", "with", "from",
    "first", "second", "third", "last", "many", "much", "more", "most", "new",
];

/// Convert a wh-question plus extracted answer span into a declarative claim
/// the fact-checker can verify. Deterministic, rule-based, no LLM.
///
/// Examples:
/// - "In what year did Pierre Curie win the Nobel Prize in Chemistry?" + "1911"
///   -> "Pierre Curie won the Nobel Prize in Chemistry in 1911."
/// - "In what year did the first attempt of the Vancouver Expedition to summit
///   Mauna Loa succeed?" + "1793"
///   -> "The first attempt of the Vancouver Expedition to summit Mauna Loa succeeded in 1793."
pub fn to_declarative_claim(question: &str, span: &str) -> String {
    let q = question.trim().trim_end_matches('?').trim().to_string();
    let low = q.to_lowercase();
    let yearish = span.trim().len() == 4 && span.trim().chars().all(|c| c.is_ascii_digit());
    let prep = if yearish { "in" } else { "on" };

    // did-scaffold patterns: find the base-form main verb and conjugate.
    const DID_PREFIXES: &[&str] = &[
        "in what year did ", "in which year did ", "what year did ",
        "on what exact date did ", "on what date did ", "on what day did ",
        "when did ",
    ];
    for p in DID_PREFIXES {
        if let Some(body) = low.strip_prefix(p).map(|_| q[p.len()..].to_string()) {
            // Locate the main verb: first lowercase alphabetic token (len > 2)
            // not in the skip list and not part of a "to <verb>" infinitive.
            let toks: Vec<(usize, String)> = body
                .split_whitespace()
                .enumerate()
                .map(|(i, t)| (i, t.trim_matches(|c: char| !c.is_alphanumeric()).to_string()))
                .collect();
            let mut verb_idx = None;
            for (i, t) in &toks {
                let tl = t.to_lowercase();
                if tl.len() <= 2 || VERB_SKIP.contains(&tl.as_str()) {
                    continue;
                }
                if t.chars().next().map(|c| c.is_ascii_lowercase()).unwrap_or(false) {
                    if *i > 0 && toks[*i - 1].1.to_lowercase() == "to" {
                        continue;
                    }
                    verb_idx = Some(*i);
                    break;
                }
            }
            let mut words: Vec<String> = body.split_whitespace().map(|w| w.to_string()).collect();
            if let Some(vi) = verb_idx {
                // recompute the word index in whitespace terms
                let mut count = 0usize;
                for w in words.iter_mut() {
                    if count == vi {
                        let bare: String = w.chars().filter(|c| c.is_alphanumeric()).collect();
                        let past = past_tense(&bare);
                        *w = w.replacen(&bare, &past, 1);
                        break;
                    }
                    count += 1;
                }
            }
            let mut stmt = words.join(" ");
            if let Some(first) = stmt.get(..1) {
                stmt = format!("{}{}", first.to_uppercase(), &stmt[1..]);
            }
            return format!("{stmt} {prep} {}.", span.trim());
        }
    }

    // will-scaffold future patterns.
    const WILL_PREFIXES: &[&str] = &[
        "in what year will ", "in which year will ", "when will ",
        "on what date will ", "on what exact date will ",
    ];
    for p in WILL_PREFIXES {
        if low.starts_with(p) {
            let body = &q[p.len()..];
            let mut stmt = body.to_string();
            if let Some(first) = stmt.get(..1) {
                stmt = format!("{}{}", first.to_uppercase(), &stmt[1..]);
            }
            return format!("{stmt} {prep} {}.", span.trim());
        }
    }

    // Fallback: strip the wh-scaffold and assert the span as the predicate value.
    let stmt = q.clone();
    format!("{stmt} — the answer is {}.", span.trim())
}

#[cfg(test)]
mod tests {
    #[test]
    fn claim_examples() {
        let c = super::to_declarative_claim(
            "In what year did Pierre Curie win the Nobel Prize in Chemistry?", "1911");
        assert!(c.contains("won"), "{c}");
        assert!(c.ends_with("in 1911."), "{c}");
        assert!(c.starts_with("Pierre Curie"), "{c}");
        let c2 = super::to_declarative_claim(
            "In what year did the first attempt of the Vancouver Expedition to summit Mauna Loa succeed?", "1793");
        assert!(c2.contains("succeeded"), "{c2}");
        assert!(!c2.contains("summited"), "{c2}");
    }
}
