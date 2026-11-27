//! Local SQuAD 2.0 extractive-QA unanswerability verifier — INT8 ONNX edition.
//!
//! Frozen configuration:
//! - Model: deepset/minilm-uncased-squad2, dynamically quantized INT8 ONNX
//!   (quantized locally with onnxruntime dynamic quantization; sha256 logged).
//! - Runtime: ONNX Runtime CPU execution provider via `ort`, zero network.
//! - Input: [CLS] question [SEP] "{title} — {section}\n{text}" [SEP], question
//!   capped at 64 wordpiece tokens, passage at 384, total capped at 512.
//! - Scoring per selected passage: null_score = start[0]+end[0] over the CLS
//!   position; best_span = max start[i]+end[j] over passage positions
//!   (token_type==1, excluding special tokens), i<=j, j-i<30.
//!   margin = best_span - null_score.
//! - Runs ONLY on the final top-3 selected passages inside acceptance.

use ort::session::{builder::GraphOptimizationLevel, Session};
use ort::value::Tensor;
use std::sync::{Mutex, OnceLock};
use tokenizers::Tokenizer;

struct QaModel {
    session: Mutex<Session>,
    tokenizer: Tokenizer,
}

static MODEL: OnceLock<Option<QaModel>> = OnceLock::new();

fn model_dir() -> String {
    std::env::var("GANESH_QA_VERIFIER_MODEL_DIR")
        .unwrap_or_else(|_| "fixtures/qa-verifier-model".to_string())
}

pub(crate) fn load() -> Option<QaModel> {
    let dir = model_dir();
    let tokenizer = Tokenizer::from_file(format!("{dir}/tokenizer.json")).ok()?;
    let weights = format!("{dir}/model-int8.onnx");
    if !std::path::Path::new(&weights).exists() {
        return None;
    }
    let session = Session::builder().ok()?
        .with_optimization_level(GraphOptimizationLevel::Level3).ok()?
        .with_intra_threads(2).ok()?
        .commit_from_file(weights).ok()?;
    Some(QaModel { session: Mutex::new(session), tokenizer })
}

pub struct QaVerdict {
    pub margin: f32,
    pub span: String,
    pub null_score: f32,
    pub best_span_score: f32,
}

/// Verify one (question, passage) pair. None when the model is unavailable.
pub fn verify_pair(question: &str, passage: &str) -> Option<QaVerdict> {
    let m = MODEL.get_or_init(load).as_ref()?;
    score_with(m, question, passage).ok()
}

fn score_with(m: &QaModel, question: &str, passage: &str) -> ort::Result<QaVerdict> {
    let q_trunc: String = question.split_whitespace().take(64).collect::<Vec<_>>().join(" ");
    let p_trunc: String = passage.split_whitespace().take(384).collect::<Vec<_>>().join(" ");
    let encoding = m
        .tokenizer
        .encode((q_trunc.as_str(), p_trunc.as_str()), true)
        .map_err(|e| ort::Error::new(format!("tokenize: {e}")))?;
    let mut ids: Vec<i64> = encoding.get_ids().iter().map(|&x| x as i64).collect();
    let mut types: Vec<i64> = encoding.get_type_ids().iter().map(|&x| x as i64).collect();
    let mut mask: Vec<i64> = encoding.get_attention_mask().iter().map(|&x| x as i64).collect();
    ids.truncate(512);
    types.truncate(512);
    mask.truncate(512);
    let len = ids.len();
    let shape = vec![1usize, len];
    let ids_t = Tensor::from_array((shape.clone(), ids.clone()))?;
    let types_t = Tensor::from_array((shape.clone(), types.clone()))?;
    let mask_t = Tensor::from_array((shape, mask))?;
    let (start, end) = {
        let mut session = m.session.lock().map_err(|e| ort::Error::new(format!("lock: {e}")))?;
        let outputs = session.run(ort::inputs![ids_t, mask_t, types_t])?;
        let (s_shape, s_data) = outputs[0].try_extract_tensor::<f32>()?;
        let (_e_shape, e_data) = outputs[1].try_extract_tensor::<f32>()?;
        let seq_len = s_shape[1] as usize;
        (s_data[..seq_len].to_vec(), e_data[..seq_len].to_vec())
    };
    let null_score = start[0] + end[0];
    // passage positions: token_type==1, excluding [SEP] (id 102) and padding
    let sep_id = 102i64;
    let cls_positions: Vec<usize> = (0..len)
        .filter(|&p| types[p] == 1 && ids[p] != sep_id)
        .collect();
    let mut best = f32::NEG_INFINITY;
    let mut best_ij = (0usize, 0usize);
    for (ai, &i) in cls_positions.iter().enumerate() {
        for &j in cls_positions.iter().skip(ai) {
            if j - i >= 30 {
                break;
            }
            let sc = start[i] + end[j];
            if sc > best {
                best = sc;
                best_ij = (i, j);
            }
        }
    }
    if best == f32::NEG_INFINITY {
        best = null_score;
    }
    let vocab_ids: Vec<u32> = ids[best_ij.0..=best_ij.1.min(len - 1)].iter().map(|&x| x as u32).collect();
    let span = m.tokenizer.decode(&vocab_ids[..], true).unwrap_or_default();
    Ok(QaVerdict { margin: best - null_score, span, null_score, best_span_score: best })
}

pub fn available() -> bool {
    MODEL.get_or_init(load).is_some()
}

/// Status line for report certification, reusing the OnceLock.
pub fn status_summary() -> String {
    let dir = model_dir();
    let weights = format!("{dir}/model-int8.onnx");
    if std::path::Path::new(&weights).exists() && available() {
        let digest = sha256_prefix(&weights);
        format!("qa-verifier: active (model {dir}/model-int8.onnx int8-onnx, sha256 {digest})")
    } else {
        "qa-verifier: absent".to_string()
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

/// Fail-loud guard mirroring the reranker guard, reusing the OnceLock.
pub fn enforce_requirement(caller: &str) {
    if std::env::var("GANESH_REQUIRE_QA_VERIFIER").ok().as_deref() == Some("1") && !available() {
        eprintln!(
            "FATAL: GANESH_REQUIRE_QA_VERIFIER=1 but the QA verifier model failed to load from `{}` ({caller}). Refusing silent skip.",
            model_dir()
        );
        std::process::exit(2);
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn activation_rule() {
        use crate::accept::qa_extractive_question as act;
        assert!(act("In what year did Apollo 11 land?"));
        assert!(act("What is the diameter in meters?"));
        assert!(act("On what exact date did construction begin?"));
        assert!(act("How many workers built it?"));
        assert!(!act("Did the bank collapse in 1929?"));
        assert!(!act("Was the bridge opened in 1937?"));
        assert!(!act("Is the river navigable?"));
    }
    #[test]
    fn fail_loud_missing_model() {
        std::env::set_var("GANESH_QA_VERIFIER_MODEL_DIR", "/nonexistent-qa-dir");
        assert!(crate::qa_verify::load().is_none());
    }
    #[test]
    fn smoke_margins() {
        if !crate::qa_verify::available() { return; }
        let sup = crate::qa_verify::verify_pair(
            "When was Curiosity launched?",
            "Mars — Launch
Curiosity was launched from Cape Canaveral on November 26, 2011.").unwrap();
        assert!(sup.margin > 0.0, "supported margin {}", sup.margin);
        assert!(sup.span.contains("november 26"), "span {}", sup.span);
        let unsup = crate::qa_verify::verify_pair(
            "What is the diameter of the James Webb primary mirror in inches?",
            "JWST — Mirror
Webb's primary mirror consists of 18 hexagonal mirror segments made of gold-plated beryllium.").unwrap();
        assert!(unsup.margin < 0.0, "missing-unit margin {}", unsup.margin);
    }
}
