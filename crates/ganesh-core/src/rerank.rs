//! Local cross-encoder passage reranker — INT8 ONNX Runtime edition.
//!
//! Frozen configuration:
//! - Model: cross-encoder/ms-marco-MiniLM-L-6-v2, dynamic INT8 ONNX
//!   (temsa/ms-marco-MiniLM-L-6-v2-onnx-cpu-qint8, sha256 logged per report).
//! - Runtime: ONNX Runtime CPU execution provider via `ort`, zero network at
//!   runtime, no provider calls.
//! - Input pair: [CLS] question [SEP] passage [SEP]. Question truncated to 64
//!   whitespace tokens; passage to 384; total sequence capped at 512.
//! - Batched inference: the whole candidate pool is scored in as few forward
//!   passes as possible (padding to the batch maximum length).
//! - Selection policy is unchanged: greedy slots in cross-encoder order with
//!   token-Jaccard >= 0.7 diversity skip; hybrid coverage final slot.

use ort::session::{builder::GraphOptimizationLevel, Session};
use ort::value::Tensor;
use std::sync::{Mutex, OnceLock};
use tokenizers::Tokenizer;

struct CrossEncoder {
    session: Mutex<Session>,
    tokenizer: Tokenizer,
}

static MODEL: OnceLock<Option<CrossEncoder>> = OnceLock::new();

fn model_dir() -> String {
    std::env::var("GANESH_RERANKER_MODEL_DIR")
        .unwrap_or_else(|_| "fixtures/reranker-model".to_string())
}

fn load() -> Option<CrossEncoder> {
    let dir = model_dir();
    let tokenizer = Tokenizer::from_file(format!("{dir}/tokenizer.json")).ok()?;
    let weights = format!("{dir}/model-int8.onnx");
    if !std::path::Path::new(&weights).exists() {
        return None;
    }
    let session = Session::builder().ok()?
        .with_optimization_level(GraphOptimizationLevel::Level3).ok()?
        .with_intra_threads(4).ok()?
        .commit_from_file(weights).ok()?;
    Some(CrossEncoder { session: Mutex::new(session), tokenizer })
}

fn encode_pair(tok: &Tokenizer, question: &str, passage: &str) -> (Vec<i64>, Vec<i64>, Vec<i64>) {
    let q_trunc: String = question.split_whitespace().take(64).collect::<Vec<_>>().join(" ");
    let p_trunc: String = passage.split_whitespace().take(384).collect::<Vec<_>>().join(" ");
    let encoding = match tok.encode((q_trunc.as_str(), p_trunc.as_str()), true) {
        Ok(e) => e,
        Err(_) => return (vec![], vec![], vec![]),
    };
    let mut ids: Vec<i64> = encoding.get_ids().iter().map(|&x| x as i64).collect();
    let mut types: Vec<i64> = encoding.get_type_ids().iter().map(|&x| x as i64).collect();
    let mut mask: Vec<i64> = encoding.get_attention_mask().iter().map(|&x| x as i64).collect();
    ids.truncate(512);
    types.truncate(512);
    mask.truncate(512);
    (ids, types, mask)
}

/// Score a batch of (question, passage) pairs sharing one question in a
/// single padded forward pass. None when the model is unavailable.
pub fn score_batch(question: &str, passages: &[String]) -> Option<Vec<f32>> {
    let enc = MODEL.get_or_init(load).as_ref()?;
    if passages.is_empty() {
        return Some(Vec::new());
    }
    let mut items: Vec<(Vec<i64>, Vec<i64>, Vec<i64>)> = passages
        .iter()
        .map(|p| encode_pair(&enc.tokenizer, question, p))
        .collect();
    if items.iter().any(|(ids, _, _)| ids.is_empty()) {
        return None;
    }
    // Length-sorted batching: score in ascending-length order so padding waste
    // stays minimal, then restore the caller's original ordering. Attention
    // masks make padded positions exact no-ops, so scores are unchanged.
    let mut order: Vec<usize> = (0..items.len()).collect();
    order.sort_by_key(|&i| items[i].0.len());
    let mut sorted: Vec<(Vec<i64>, Vec<i64>, Vec<i64>)> = Vec::with_capacity(items.len());
    for &i in &order {
        sorted.push(std::mem::take(&mut items[i]));
    }
    let mut items = sorted;
    let batch = items.len();
    let maxlen = items.iter().map(|(ids, _, _)| ids.len()).max().unwrap_or(1);
    let mut ids_flat: Vec<i64> = Vec::with_capacity(batch * maxlen);
    let mut types_flat: Vec<i64> = Vec::with_capacity(batch * maxlen);
    let mut mask_flat: Vec<i64> = Vec::with_capacity(batch * maxlen);
    for (ids, types, mask) in items.drain(..) {
        let pad = maxlen - ids.len();
        ids_flat.extend(ids.iter().cloned().chain(std::iter::repeat(0).take(pad)));
        types_flat.extend(types.iter().cloned().chain(std::iter::repeat(0).take(pad)));
        mask_flat.extend(mask.iter().cloned().chain(std::iter::repeat(0).take(pad)));
    }
    let shape = vec![batch, maxlen];
    let ids_t = Tensor::from_array((shape.clone(), ids_flat)).ok()?;
    let types_t = Tensor::from_array((shape.clone(), types_flat)).ok()?;
    let mask_t = Tensor::from_array((shape, mask_flat)).ok()?;
    let mut session = enc.session.lock().ok()?;
    let outputs = session
        .run(ort::inputs![ids_t, mask_t, types_t])
        .ok()?;
    let (_shape, data) = outputs[0].try_extract_tensor::<f32>().ok()?;
    let mut out = vec![0f32; batch];
    for (pos, &orig) in order.iter().enumerate() {
        out[orig] = data.get(pos).cloned().unwrap_or(0.0);
    }
    Some(out)
}

/// Score one (question, passage) pair. Returns None if the model is
/// unavailable or inference fails; callers treat None as "reranker absent".
pub fn score_pair(question: &str, passage: &str) -> Option<f32> {
    score_batch(question, &[passage.to_string()]).and_then(|v| v.first().cloned())
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
        format!("reranker: active (model {dir}/model-int8.onnx int8-onnx, sha256 {digest})")
    } else {
        "reranker: absent".to_string()
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

/// Fail-loud guard: refuse to run silently degraded when the operator
/// explicitly requires the neural reranker.
pub fn enforce_requirement(caller: &str) {
    if std::env::var("GANESH_REQUIRE_RERANKER").ok().as_deref() == Some("1") && !available() {
        eprintln!(
            "FATAL: GANESH_REQUIRE_RERANKER=1 but the reranker model failed to load from `{}` ({caller}). Refusing silent skip.",
            model_dir()
        );
        std::process::exit(2);
    }
}
