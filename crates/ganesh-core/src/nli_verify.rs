//! Local NLI entailment verifier (extract-then-entail), exposed-calibrated.
//!
//! Predeclared configuration:
//! - Model: typeform/distilbert-base-uncased-mnli, DistilBertForSequence-
//!   Classification, 6-layer/768-dim fp32 safetensors, pure Rust candle, CPU,
//!   zero network at runtime. id2label: 0=ENTAILMENT, 1=NEUTRAL,
//!   2=CONTRADICTION (verified from upstream config.json).
//! - Input: premise = "{title} — {section}\n{text}" of a selected passage,
//!   hypothesis = "Question: {question} Answer: {best_span}" using the SQuAD
//!   2.0 extracted span; evaluated ONLY on top-3 passages where qa margin >= 0.
//! - Head math: hidden[CLS] -> pre_classifier (linear+relu) -> classifier
//!   (linear) -> softmax over 3 labels.

use candle_core::{Device, Tensor};
use candle_nn::VarBuilder;
use candle_transformers::models::distilbert::{Config, DistilBertModel};
use std::sync::OnceLock;
use tokenizers::Tokenizer;

struct NliModel {
    model: DistilBertModel,
    pre_w: Tensor,
    pre_b: Tensor,
    cls_w: Tensor,
    cls_b: Tensor,
    tokenizer: Tokenizer,
}

static MODEL: OnceLock<Option<NliModel>> = OnceLock::new();

fn model_dir() -> String {
    std::env::var("GANESH_NLI_VERIFIER_MODEL_DIR")
        .unwrap_or_else(|_| "fixtures/nli-verifier-model".to_string())
}

pub fn load() -> Option<NliModel> {
    let dir = model_dir();
    let config: Config =
        serde_json::from_str(&std::fs::read_to_string(format!("{dir}/config.json")).ok()?).ok()?;
    let tokenizer = Tokenizer::from_file(format!("{dir}/tokenizer.json")).ok()?;
    let weights = format!("{dir}/model.safetensors");
    let device = Device::Cpu;
    let vb = unsafe {
        VarBuilder::from_mmaped_safetensors(&[weights], candle_core::DType::F32, &device).ok()?
    };
    let model = DistilBertModel::load(vb.pp("distilbert"), &config).ok()?;
    let pre_w = vb.pp("pre_classifier").get((768, 768), "weight").ok()?;
    let pre_b = vb.pp("pre_classifier").get(768, "bias").ok()?;
    let cls_w = vb.pp("classifier").get((3, 768), "weight").ok()?;
    let cls_b = vb.pp("classifier").get(3, "bias").ok()?;
    Some(NliModel { model, pre_w, pre_b, cls_w, cls_b, tokenizer })
}

pub struct NliVerdict {
    pub entailment: f32,
    pub neutral: f32,
    pub contradiction: f32,
}

/// Score one (premise, hypothesis) pair. None when the model is unavailable.
pub fn verify_pair(premise: &str, hypothesis: &str) -> Option<NliVerdict> {
    let m = MODEL.get_or_init(load).as_ref()?;
    let enc = m
        .tokenizer
        .encode((premise, hypothesis), true)
        .ok()?;
    let mut ids: Vec<u32> = enc.get_ids().to_vec();
    ids.truncate(512);
    let len = ids.len();
    let device = Device::Cpu;
    let input = Tensor::new(ids, &device).ok()?.reshape((1, len)).ok()?;
    let mask = Tensor::zeros((1, len), candle_core::DType::U8, &device).ok()?;
    let hidden = m.model.forward(&input, &mask).ok()?; // [1, seq, dim]
    let cls = hidden.narrow(1, 0, 1).ok()?.squeeze(1).ok()?; // [1, dim]
    let pre = cls.matmul(&m.pre_w.t().ok()?).ok()?.broadcast_add(&m.pre_b).ok()?.relu().ok()?;
    let logits = pre.matmul(&m.cls_w.t().ok()?).ok()?.broadcast_add(&m.cls_b).ok()?;
    let probs = candle_nn::ops::softmax_last_dim(&logits).ok()?;
    let v: Vec<f32> = probs.flatten_all().ok()?.to_vec1().ok()?;
    if v.len() != 3 {
        return None;
    }
    Some(NliVerdict { entailment: v[0], neutral: v[1], contradiction: v[2] })
}

pub fn available() -> bool {
    MODEL.get_or_init(load).is_some()
}

pub fn status_summary() -> String {
    let dir = model_dir();
    let weights = format!("{dir}/model.safetensors");
    if std::path::Path::new(&weights).exists() && available() {
        let digest = sha256_prefix(&weights);
        format!("nli-verifier: active (model {dir}, sha256 {digest})")
    } else {
        "nli-verifier: absent".to_string()
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
    if std::env::var("GANESH_REQUIRE_NLI_VERIFIER").ok().as_deref() == Some("1") && !available() {
        eprintln!(
            "FATAL: GANESH_REQUIRE_NLI_VERIFIER=1 but the NLI verifier model failed to load from `{}` ({caller}). Refusing silent skip.",
            model_dir()
        );
        std::process::exit(2);
    }
}
