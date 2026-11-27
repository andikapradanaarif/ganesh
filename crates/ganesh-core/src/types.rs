use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ManifestEntry {
    pub article_id: String,
    pub title: String,
    #[serde(default)]
    pub pageid: u64,
    pub revision: String,
    #[serde(default)]
    pub revision_url: String,
    #[serde(default)]
    pub publisher: String,
    #[serde(default)]
    pub url: String,
    #[serde(default)]
    pub license: String,
    #[serde(default)]
    pub license_url: String,
    #[serde(default)]
    pub permission: String,
    #[serde(default)]
    pub retrieved: String,
    pub bytes: u64,
    pub sha256: String,
    pub file: String,
    #[serde(default)]
    pub family: String,
    // ---- M2 lifecycle metadata (optional; M1 fixtures default cleanly) ----
    #[serde(default)]
    pub effective_from: Option<String>,
    #[serde(default)]
    pub effective_until: Option<String>,
    #[serde(default)]
    pub occurred_at: Option<String>,
    #[serde(default)]
    pub author: String,
    #[serde(default = "default_source_type")]
    pub source_type: String,
    #[serde(default = "default_project")]
    pub project: String,
    #[serde(default)]
    pub scope: String,
    #[serde(default)]
    pub authority: i64,
    #[serde(default)]
    pub supersedes: Option<String>,
    /// "full" retires the predecessor in every scope; anything else is a
    /// partial scoped correction.
    #[serde(default)]
    pub supersede_mode: String,
    #[serde(default)]
    pub claim_key: String,
    /// Comma-separated extra projects allowed to read this revision. Empty
    /// means visible to its own project only (plus public-project rules).
    #[serde(default)]
    pub audience: String,
    /// Revisions supporting this one (multi-source claims).
    #[serde(default)]
    pub supports: Vec<String>,
}

fn default_source_type() -> String {
    "wiki".to_string()
}
fn default_project() -> String {
    "public".to_string()
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Manifest {
    pub version: String,
    pub created: String,
    pub frame: String,
    pub seed: u64,
    pub exclusions: Vec<String>,
    pub sampling: String,
    pub snapshot_format: String,
    pub articles: Vec<ManifestEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SearchRequest {
    pub query: String,
    #[serde(default)]
    pub caller: Option<String>,
    /// Credential key resolved against the server policy. Never self-asserted
    /// project parameters — access derives only from this key.
    #[serde(default)]
    pub key: Option<String>,
    /// Historical read: ISO date (YYYY-MM-DD). Default = current time.
    #[serde(default)]
    pub as_of: Option<String>,
    /// Passages to return (default 3, up to 15 for multi-hop synthesis).
    #[serde(default)]
    pub top_k: Option<usize>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct PassageHit {
    pub rank: u32,
    pub passage_id: String,
    pub revision: String,
    pub article_id: String,
    pub section: String,
    pub text: String,
    pub citation: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct SearchResponse {
    /// "ok", "insufficient_context" or "conflicting_evidence"
    pub status: String,
    pub query: String,
    pub passages: Vec<PassageHit>,
    /// Calibrated confidence: "supported" | "uncertain" | "insufficient_context".
    #[serde(default)]
    pub confidence: String,
    /// Margin between the top-1 and top-2 cross-encoder scores (None when
    /// fewer than two candidates were ranked).
    #[serde(default)]
    pub top_margin: Option<f64>,
    /// The effective time this read answered for (ISO date).
    #[serde(default)]
    pub effective_time: String,
}

/// Current UTC date in ISO YYYY-MM-DD form, computed without external crates.
pub fn now_iso() -> String {
    let secs = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0);
    iso_from_unix(secs)
}

pub fn iso_from_unix(secs: u64) -> String {
    let days = (secs / 86400) as i64;
    // civil-from-days algorithm (Howard Hinnant)
    let z = days + 719468;
    let era = if z >= 0 { z } else { z - 146096 } / 146097;
    let doe = (z - era * 146097) as u64;
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    let y = yoe as i64 + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = doy - (153 * mp + 2) / 5 + 1;
    let m = if mp < 10 { mp + 3 } else { mp - 9 };
    let y = if m <= 2 { y + 1 } else { y };
    format!("{:04}-{:02}-{:02}", y, m, d)
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SourceInfo {
    pub article_id: String,
    pub title: String,
    pub publisher: String,
    pub url: String,
    pub revision_url: String,
    pub license: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CitationOpen {
    pub citation: String,
    pub passage_id: String,
    pub revision: String,
    pub article_id: String,
    pub section: String,
    pub text: String,
    pub source: SourceInfo,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ErrorBody {
    pub error: String,
}
