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

/// QA verifier runs single-pass scoring on at most 3 passages per query;
/// small batches favor fewer threads. Overridable for calibration.
const DEFAULT_THREADS: usize = 6;

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
        .with_intra_threads(std::env::var("GANESH_ORT_THREADS").ok().and_then(|v| v.parse().ok()).unwrap_or(DEFAULT_THREADS)).ok()?
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

/// SQuAD 2.0 sliding-window scoring: passages of at most 300 whitespace words
/// score in a single pass. Longer passages are scored over 300-word windows
/// with a 150-word stride, taking the maximum margin (and its span) across
/// windows. To bound worst-case CPU latency on huge passages, at most 4
/// windows run: window 0 always, then the windows with the largest query-term
/// overlap. This replaces blind front-truncation, which silently discarded
/// evidence past word 384 (the acc-xfer-v6 q06 root cause).
/// Compute the scoring windows for one passage: a single full-passage window
/// when short, otherwise up to MAX_WINDOWS 300-word windows (stride 150) with
/// window 0 first and the rest ordered by descending query-term overlap.
fn windows_for(question: &str, passage: &str) -> Vec<String> {
    const WIN: usize = 300;
    const STRIDE: usize = 150;
    const MAX_WINDOWS: usize = 4;
    let words: Vec<&str> = passage.split_whitespace().collect();
    if words.len() <= WIN {
        return vec![passage.to_string()];
    }
    let mut starts: Vec<usize> = Vec::new();
    let mut s = 0usize;
    loop {
        starts.push(s);
        if s + WIN >= words.len() {
            break;
        }
        s += STRIDE;
        if starts.len() > 64 {
            break;
        }
    }
    let qterms: std::collections::BTreeSet<String> = question
        .to_lowercase()
        .split(|c: char| !c.is_alphanumeric())
        .filter(|w| !w.is_empty())
        .map(|w| w.to_string())
        .collect();
    let overlap = |start: usize| -> usize {
        let end = (start + WIN).min(words.len());
        let mut seen = std::collections::BTreeSet::new();
        let mut n = 0;
        for w in &words[start..end] {
            let lw = w.to_lowercase();
            if qterms.contains(&lw) && seen.insert(lw) {
                n += 1;
            }
        }
        n
    };
    let mut rest: Vec<usize> = starts.iter().skip(1).copied().collect();
    rest.sort_by_key(|&st| std::cmp::Reverse(overlap(st)));
    let mut order: Vec<usize> = vec![starts[0]];
    order.extend(rest);
    order.truncate(MAX_WINDOWS);
    order.iter()
        .map(|&start| {
            let end = (start + WIN).min(words.len());
            words[start..end].join(" ")
        })
        .collect()
}

fn score_with(m: &QaModel, question: &str, passage: &str) -> ort::Result<QaVerdict> {
    let mut best: Option<QaVerdict> = None;
    for window_text in windows_for(question, passage) {
        let v = score_window(m, question, &window_text)?;
        let better = best.as_ref().map(|b| v.margin > b.margin).unwrap_or(true);
        if better {
            best = Some(v);
        }
    }
    Ok(best.expect("at least one window"))
}

/// Batched verification: every sliding window of every passage scored in ONE
/// padded ONNX session call, then grouped back per passage with the max-margin
/// window winning — identical selection semantics to sequential `verify_pair`
/// calls, but the neural cost is one batch instead of up to 3x4 sequential
/// forward passes. Padding is attention-masked, so scores are exact modulo
/// floating-point batching noise (margins here are 4..17, never knife-edge).
/// Returns one verdict slot per input passage (None when unscoreable).
pub fn verify_batch(question: &str, passages: &[String]) -> Vec<Option<QaVerdict>> {
    let m = match MODEL.get_or_init(load).as_ref() {
        Some(m) => m,
        None => return passages.iter().map(|_| None).collect(),
    };
    // Flatten (passage_idx, window) pairs.
    let mut flat: Vec<(usize, String)> = Vec::new();
    for (pi, p) in passages.iter().enumerate() {
        for w in windows_for(question, p) {
            flat.push((pi, w));
        }
    }
    if flat.is_empty() {
        return passages.iter().map(|_| None).collect();
    }
    // Tokenize all pairs; pad to the batch max length.
    let q_trunc: String = question.split_whitespace().take(64).collect::<Vec<_>>().join(" ");
    let mut items: Vec<(usize, Vec<i64>, Vec<i64>, Vec<i64>)> = Vec::new();
    for (pi, w) in &flat {
        let p_trunc: String = w.split_whitespace().take(384).collect::<Vec<_>>().join(" ");
        let encoding = match m.tokenizer.encode((q_trunc.as_str(), p_trunc.as_str()), true) {
            Ok(e) => e,
            Err(_) => continue,
        };
        let mut ids: Vec<i64> = encoding.get_ids().iter().map(|&x| x as i64).collect();
        let mut types: Vec<i64> = encoding.get_type_ids().iter().map(|&x| x as i64).collect();
        let mut mask: Vec<i64> = encoding.get_attention_mask().iter().map(|&x| x as i64).collect();
        ids.truncate(512);
        types.truncate(512);
        mask.truncate(512);
        items.push((*pi, ids, types, mask));
    }
    if items.is_empty() {
        return passages.iter().map(|_| None).collect();
    }
    let maxlen = items.iter().map(|it| it.1.len()).max().unwrap();
    let batch = items.len();
    let mut ids_flat = vec![0i64; batch * maxlen];
    let mut types_flat = vec![0i64; batch * maxlen];
    let mut mask_flat = vec![0i64; batch * maxlen];
    for (b, (_, ids, types, mask)) in items.iter().enumerate() {
        ids_flat[b * maxlen..b * maxlen + ids.len()].copy_from_slice(ids);
        types_flat[b * maxlen..b * maxlen + types.len()].copy_from_slice(types);
        mask_flat[b * maxlen..b * maxlen + mask.len()].copy_from_slice(mask);
    }
    let shape = vec![batch, maxlen];
    let run = (|| -> ort::Result<Vec<QaVerdict>> {
        let ids_t = Tensor::from_array((shape.clone(), ids_flat))?;
        let types_t = Tensor::from_array((shape.clone(), types_flat))?;
        let mask_t = Tensor::from_array((shape, mask_flat))?;
        let mut session = m.session.lock().map_err(|e| ort::Error::new(format!("lock: {e}")))?;
        let outputs = session.run(ort::inputs![ids_t, mask_t, types_t])?;
        let (s_shape, s_data) = outputs[0].try_extract_tensor::<f32>()?;
        let (_e_shape, e_data) = outputs[1].try_extract_tensor::<f32>()?;
        let seq_len = s_shape[1] as usize;
        let mut out = Vec::new();
        for (b, (_, ids, types, _mask)) in items.iter().enumerate() {
            let len = ids.len();
            let start: Vec<f32> = s_data[b * seq_len..b * seq_len + seq_len].to_vec();
            let end: Vec<f32> = e_data[b * seq_len..b * seq_len + seq_len].to_vec();
            let null_score = start[0] + end[0];
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
            out.push(QaVerdict { margin: best - null_score, span, null_score, best_span_score: best });
        }
        Ok(out)
    })();
    let verdicts = match run {
        Ok(v) => v,
        Err(_) => return passages.iter().map(|_| None).collect(),
    };
    // Group back per passage: max-margin window wins.
    let mut result: Vec<Option<QaVerdict>> = passages.iter().map(|_| None).collect();
    for (idx, v) in verdicts.into_iter().enumerate() {
        let pi = items[idx].0;
        let better = result[pi].as_ref().map(|b| v.margin > b.margin).unwrap_or(true);
        if better {
            result[pi] = Some(v);
        }
    }
    result
}

fn score_window(m: &QaModel, question: &str, passage: &str) -> ort::Result<QaVerdict> {
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
    #[test]
    fn sliding_window_finds_answer_past_word_400() {
        if !crate::qa_verify::available() { return; }
        // 420 filler words push the answer sentence beyond the old 384-word
        // blind truncation boundary; the sliding window must still extract it.
        let filler = "the instrument workshop produced many parts over the years ".repeat(60);
        let passage = format!(
            "Saxophone — Unusual variants\n{filler}The Vibratosax is in current production as a low-cost alternative to metal saxophones."
        );
        assert!(passage.split_whitespace().count() > 400, "test passage must exceed 400 words");
        let v = crate::qa_verify::verify_pair(
            "Which polycarbonate saxophone is in current production as a low-cost alternative to metal saxophones?",
            &passage,
        ).unwrap();
        assert!(v.margin > 0.0, "past-word-400 margin {}", v.margin);
        assert!(!v.span.is_empty(), "past-word-400 span empty");
    }
}
