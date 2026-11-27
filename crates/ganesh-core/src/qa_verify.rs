//! Local SQuAD 2.0 extractive-QA unanswerability verifier.
//!
//! Predeclared configuration, frozen before any transfer-packet evaluation:
//! - Model: deepset/minilm-uncased-squad2 (MiniLM, BERT architecture,
//!   BertForQuestionAnswering head), safetensors, CPU fp32, pure Rust candle,
//!   zero network at runtime.
//! - Input: [CLS] question [SEP] "{title} — {section}\n{text}" [SEP], question
//!   capped at 64 wordpiece tokens, passage at 384, total capped at 512.
//! - Scoring per selected passage: null_score = start[0]+end[0] over the CLS
//!   position; best_span = max start[i]+end[j] over passage positions
//!   (token_type==1, excluding special tokens), i<=j, j-i<30.
//!   margin = best_span - null_score. Positive: an answer span beats no-answer.
//! - The verifier runs ONLY on the final top-3 selected passages inside
//!   acceptance (~3 forward passes per query).

use candle_core::{Device, Tensor};
use candle_nn::VarBuilder;
use candle_transformers::models::bert::{BertModel, Config};
use std::sync::OnceLock;
use tokenizers::Tokenizer;

struct QaModel {
    model: BertModel,
    qa_w: Tensor, // [2, hidden]
    qa_b: Tensor, // [2]
    tokenizer: Tokenizer,
}

static MODEL: OnceLock<Option<QaModel>> = OnceLock::new();

fn model_dir() -> String {
    std::env::var("GANESH_QA_VERIFIER_MODEL_DIR")
        .unwrap_or_else(|_| "fixtures/qa-verifier-model".to_string())
}

fn load() -> Option<QaModel> {
    let dir = model_dir();
    let config: Config =
        serde_json::from_str(&std::fs::read_to_string(format!("{dir}/config.json")).ok()?).ok()?;
    let tokenizer = Tokenizer::from_file(format!("{dir}/tokenizer.json")).ok()?;
    let weights = format!("{dir}/model.safetensors");
    let device = Device::Cpu;
    let vb = unsafe {
        VarBuilder::from_mmaped_safetensors(&[weights], candle_core::DType::F32, &device).ok()?
    };
    let model = BertModel::load(vb.pp("bert"), &config).ok()?;
    let qa_w = vb.pp("qa_outputs").get((2, config.hidden_size), "weight").ok()?;
    let qa_b = vb.pp("qa_outputs").get(2, "bias").ok()?;
    Some(QaModel { model, qa_w, qa_b, tokenizer })
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

fn score_with(m: &QaModel, question: &str, passage: &str) -> candle_core::Result<QaVerdict> {
    let q_trunc: String = question.split_whitespace().take(64).collect::<Vec<_>>().join(" ");
    let p_trunc: String = passage.split_whitespace().take(384).collect::<Vec<_>>().join(" ");
    let encoding = m
        .tokenizer
        .encode((q_trunc.as_str(), p_trunc.as_str()), true)
        .map_err(|e| candle_core::Error::Msg(format!("tokenize: {e}")))?;
    let mut ids: Vec<u32> = encoding.get_ids().to_vec();
    let mut types: Vec<u32> = encoding.get_type_ids().to_vec();
    let mut mask: Vec<u32> = encoding.get_attention_mask().to_vec();
    ids.truncate(512);
    types.truncate(512);
    mask.truncate(512);
    let device = Device::Cpu;
    let len = ids.len();
    let ids_t = Tensor::from_vec(ids.clone(), (1, len), &device)?;
    let types_t = Tensor::from_vec(types.clone(), (1, len), &device)?;
    let mask_t = Tensor::from_vec(mask, (1, len), &device)?;
    let hidden = m.model.forward(&ids_t, &types_t, Some(&mask_t))?; // [1, seq, hidden]
    // qa_outputs projection: hidden @ qa_w.T + qa_b -> [seq, 2]
    let flat = hidden.squeeze(0)?; // [seq, hidden]
    let logits = flat.matmul(&m.qa_w.t()?)?.broadcast_add(&m.qa_b)?; // [seq, 2]
    let start: Vec<f32> = logits.narrow(1, 0, 1)?.squeeze(1)?.to_vec1::<f32>()?;
    let end: Vec<f32> = logits.narrow(1, 1, 1)?.squeeze(1)?.to_vec1::<f32>()?;
    let null_score = start[0] + end[0];
    // passage positions: token_type==1, excluding [SEP] (id 102) and padding
    let sep_id = 102u32;
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
    // decode span tokens
    let vocab_ids = &ids[best_ij.0..=best_ij.1.min(len - 1)];
    let span = m
        .tokenizer
        .decode(&vocab_ids[..], true)
        .unwrap_or_default();
    Ok(QaVerdict { margin: best - null_score, span, null_score, best_span_score: best })
}

pub fn available() -> bool {
    MODEL.get_or_init(load).is_some()
}

/// Status line for report certification.
pub fn status_summary() -> String {
    let dir = model_dir();
    let weights = format!("{dir}/model.safetensors");
    if std::path::Path::new(&weights).exists() && load().is_some() {
        let digest = sha256_prefix(&weights);
        format!("qa-verifier: active (model {dir}, sha256 {digest})")
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

/// Fail-loud guard mirroring the reranker guard.
pub fn enforce_requirement(caller: &str) {
    if std::env::var("GANESH_REQUIRE_QA_VERIFIER").ok().as_deref() == Some("1") && load().is_none() {
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
