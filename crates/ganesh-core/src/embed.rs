//! Local dense bi-encoder embedder — INT8 ONNX Runtime edition (M2).
//!
//! Frozen configuration:
//! - Model: BAAI/bge-small-en-v1.5 (384-dim), dynamic INT8 ONNX
//!   (Xenova/bge-small-en-v1.5 onnx/model_quantized.onnx, sha256 logged per report).
//! - Runtime: ONNX Runtime CPU execution provider via `ort`, zero network at
//!   runtime, no provider calls.
//! - Input: single text, truncated to 384 whitespace tokens, sequence capped
//!   at 512. CLS pooling + L2 normalization (BGE convention). Queries use the
//!   BGE short query directly (no instruction prefix needed for short
//!   query-to-passage retrieval of this class; kept symmetric for simplicity).
//! - Batched inference: all texts embedded in one padded forward pass.

use ort::session::{builder::GraphOptimizationLevel, Session};
use ort::value::Tensor;
use std::sync::{Mutex, OnceLock};
use tokenizers::Tokenizer;

struct BiEncoder {
    session: Mutex<Session>,
    tokenizer: Tokenizer,
}

static MODEL: OnceLock<Option<BiEncoder>> = OnceLock::new();

const DEFAULT_THREADS: usize = 6;
pub const DIM: usize = 384;

fn model_dir() -> String {
    std::env::var("GANESH_EMBEDDER_MODEL_DIR")
        .unwrap_or_else(|_| "fixtures/embedder-model".to_string())
}

fn load() -> Option<BiEncoder> {
    let dir = model_dir();
    let tokenizer = Tokenizer::from_file(format!("{dir}/tokenizer.json")).ok()?;
    let weights = format!("{dir}/model-int8.onnx");
    if !std::path::Path::new(&weights).exists() {
        return None;
    }
    let session = Session::builder().ok()?
        .with_optimization_level(GraphOptimizationLevel::Level3).ok()?
        .with_intra_threads(std::env::var("GANESH_ORT_THREADS").ok().and_then(|v| v.parse().ok()).unwrap_or(DEFAULT_THREADS)).ok()?
        .commit_from_file(weights).ok()?;
    Some(BiEncoder { session: Mutex::new(session), tokenizer })
}

fn encode_text(tok: &Tokenizer, text: &str) -> (Vec<i64>, Vec<i64>, Vec<i64>) {
    let t_trunc: String = text.split_whitespace().take(384).collect::<Vec<_>>().join(" ");
    let encoding = match tok.encode(t_trunc.as_str(), true) {
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

/// Embed a batch of texts, internally chunked into padded forward passes of
/// at most 32 texts so large imports stay within the memory envelope.
/// Returns one L2-normalized 384-dim vector per input (CLS pooling).
/// None when the model is unavailable.
pub fn embed_batch(texts: &[String]) -> Option<Vec<Vec<f32>>> {
    if texts.len() > 32 {
        let mut out = Vec::with_capacity(texts.len());
        for chunk in texts.chunks(32) {
            out.extend(embed_batch(chunk)?);
        }
        return Some(out);
    }
    let enc = MODEL.get_or_init(load).as_ref()?;
    if texts.is_empty() {
        return Some(vec![]);
    }
    let encoded: Vec<(Vec<i64>, Vec<i64>, Vec<i64>)> =
        texts.iter().map(|t| encode_text(&enc.tokenizer, t)).collect();
    let maxlen = encoded.iter().map(|(i, _, _)| i.len()).max().unwrap_or(0);
    if maxlen == 0 {
        return None;
    }
    let n = encoded.len();
    let mut ids: Vec<i64> = vec![0; n * maxlen];
    let mut types: Vec<i64> = vec![0; n * maxlen];
    let mut mask: Vec<i64> = vec![0; n * maxlen];
    for (row, (i, t, m)) in encoded.iter().enumerate() {
        for (col, v) in i.iter().enumerate() {
            ids[row * maxlen + col] = *v;
        }
        for (col, v) in t.iter().enumerate() {
            types[row * maxlen + col] = *v;
        }
        for (col, v) in m.iter().enumerate() {
            mask[row * maxlen + col] = *v;
        }
    }
    let ids_t = Tensor::from_array(([n, maxlen], ids)).ok()?;
    let mask_t = Tensor::from_array(([n, maxlen], mask)).ok()?;
    let types_t = Tensor::from_array(([n, maxlen], types)).ok()?;
    let mut session = enc.session.lock().ok()?;
    let outputs = session
        .run(ort::inputs![ids_t, mask_t, types_t])
        .ok()?;
    // Output 0: last_hidden_state [n, maxlen, 384]; CLS pooling = index 0.
    let (_, data) = outputs[0].try_extract_tensor::<f32>().ok()?;
    let mut out: Vec<Vec<f32>> = Vec::with_capacity(n);
    for row in 0..n {
        let base = row * maxlen * DIM;
        // Panic-free slicing: an unexpected tensor shape must yield no
        // embedding rather than panic inside the locked inference region
        // (a poisoned mutex would abort every subsequent search/open).
        let mut v: Vec<f32> = match data.get(base..base + DIM) {
            Some(slice) => slice.to_vec(),
            None => return None,
        };
        let norm: f32 = v.iter().map(|x| x * x).sum::<f32>().sqrt();
        if norm > 0.0 {
            for x in v.iter_mut() {
                *x /= norm;
            }
        }
        out.push(v);
    }
    Some(out)
}

pub fn embed_one(text: &str) -> Option<Vec<f32>> {
    embed_batch(&[text.to_string()]).and_then(|v| v.into_iter().next())
}

/// Cosine similarity of L2-normalized vectors = dot product.
pub fn dot(a: &[f32], b: &[f32]) -> f32 {
    a.iter().zip(b.iter()).map(|(x, y)| x * y).sum()
}

pub fn available() -> bool {
    MODEL.get_or_init(load).is_some()
}

pub fn status_summary() -> String {
    let dir = model_dir();
    let weights = format!("{dir}/model-int8.onnx");
    let digest = sha256_prefix(&weights);
    match available() {
        true => format!("embedder: active (model {weights} int8-onnx, sha256 {digest})"),
        false => format!("embedder: unavailable (model {weights})"),
    }
}

fn sha256_prefix(path: &str) -> String {
    use std::io::Read;
    let mut f = match std::fs::File::open(path) {
        Ok(f) => f,
        Err(_) => return "missing".to_string(),
    };
    let mut h = sha2::Sha256::new();
    use sha2::Digest;
    let mut buf = [0u8; 65536];
    loop {
        match f.read(&mut buf) {
            Ok(0) => break,
            Ok(n) => h.update(&buf[..n]),
            Err(_) => return "error".to_string(),
        }
    }
    format!("{:x}", h.finalize())[..12].to_string()
}

pub fn enforce_requirement(caller: &str) {
    if std::env::var("GANESH_REQUIRE_EMBEDDER").ok().as_deref() == Some("1") && !available() {
        eprintln!("{caller}: GANESH_REQUIRE_EMBEDDER=1 but embedder model unavailable");
        std::process::exit(2);
    }
}
