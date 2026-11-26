//! Local cross-encoder passage reranker (issue: semantic reranker experiment).
//!
//! Predeclared configuration, frozen before any outcome inspection:
//! - Model: cross-encoder/ms-marco-MiniLM-L-6-v2 (Apache-2.0), safetensors,
//!   BERT architecture, 6 layers, hidden 384, single-logit sequence classifier.
//! - Runtime: candle (pure Rust), CPU, fp32, no provider calls, no network.
//! - Input pair: [CLS] question [SEP] passage [SEP]. Question truncated to at
//!   most 64 wordpiece tokens; passage to the first 384 tokens of its body;
//!   total sequence capped at 512 tokens per model maximum.
//! - The reranker scores every eligible candidate for every question,
//!   including date and quantity questions. It replaces bm25 ordering and all
//!   prior promotion logic inside selection; candidate generation, the top-50
//!   pool, eligibility, acceptance, and the REST/MCP contract are unchanged.
//! - Compound preservation: greedy selection in cross-encoder score order,
//!   skipping a candidate whose whitespace-token Jaccard overlap with any
//!   already-selected passage is >= 0.7, until three passages are selected.

use candle_core::{Device, Tensor};
use candle_nn::VarBuilder;
use candle_transformers::models::bert::{BertModel, Config};
use std::sync::OnceLock;
use tokenizers::Tokenizer;

struct CrossEncoder {
    model: BertModel,
    pooler_w: Tensor,
    pooler_b: Tensor,
    classifier_w: Tensor,
    classifier_b: Tensor,
    tokenizer: Tokenizer,
}

static MODEL: OnceLock<Option<CrossEncoder>> = OnceLock::new();

fn model_dir() -> String {
    std::env::var("GANESH_RERANKER_MODEL_DIR")
        .unwrap_or_else(|_| "fixtures/reranker-model".to_string())
}

fn load() -> Option<CrossEncoder> {
    let dir = model_dir();
    let config: Config =
        serde_json::from_str(&std::fs::read_to_string(format!("{dir}/config.json")).ok()?).ok()?;
    let tokenizer =
        Tokenizer::from_file(format!("{dir}/tokenizer.json")).ok()?;
    let weights = format!("{dir}/model.safetensors");
    let device = Device::Cpu;
    let vb = unsafe {
        VarBuilder::from_mmaped_safetensors(&[weights], candle_core::DType::F32, &device).ok()?
    };
    let model = BertModel::load(vb.pp("bert"), &config).ok()?;
    let pooler_w = vb.pp("bert.pooler.dense").get((config.hidden_size, config.hidden_size), "weight").ok()?;
    let pooler_b = vb.pp("bert.pooler.dense").get(config.hidden_size, "bias").ok()?;
    let classifier_w = vb.pp("classifier").get((1, config.hidden_size), "weight").ok()?;
    let classifier_b = vb.pp("classifier").get(1, "bias").ok()?;
    Some(CrossEncoder { model, pooler_w, pooler_b, classifier_w, classifier_b, tokenizer })
}

/// Score one (question, passage) pair. Returns None if the model is
/// unavailable or inference fails; callers treat None as "reranker absent".
pub fn score_pair(question: &str, passage: &str) -> Option<f32> {
    let enc = MODEL.get_or_init(load).as_ref()?;
    score_with(enc, question, passage).ok()
}

fn score_with(enc: &CrossEncoder, question: &str, passage: &str) -> candle_core::Result<f32> {
    // Truncation policy: question first 64 tokens, passage first 384 tokens.
    let q_trunc: String = question.split_whitespace().take(64).collect::<Vec<_>>().join(" ");
    let p_trunc: String = passage.split_whitespace().take(384).collect::<Vec<_>>().join(" ");
    let encoding = enc
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
    let ids_t = Tensor::from_vec(ids, (1, len), &device)?;
    let types_t = Tensor::from_vec(types, (1, len), &device)?;
    let mask_t = Tensor::from_vec(mask, (1, len), &device)?;
    let hidden = enc.model.forward(&ids_t, &types_t, Some(&mask_t))?;
    // CLS embedding: first token position.
    let cls = hidden.get(0)?.get(0)?; // [hidden]
    // Standard BertForSequenceClassification path: pooler dense + tanh, then
    // the classification head.
    let pooled = enc
        .pooler_w
        .matmul(&cls.unsqueeze(1)?)?
        .squeeze(1)?
        .add(&enc.pooler_b)?
        .tanh()?;
    let logits = pooled
        .unsqueeze(0)?
        .matmul(&enc.classifier_w.t()?)?
        .squeeze(0)?
        .add(&enc.classifier_b)?; // [1]
    logits.get(0)?.to_scalar::<f32>()
}

pub fn available() -> bool {
    MODEL.get_or_init(load).is_some()
}
