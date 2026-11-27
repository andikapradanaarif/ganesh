use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ManifestEntry {
    pub article_id: String,
    pub title: String,
    pub pageid: u64,
    pub revision: String,
    pub revision_url: String,
    pub publisher: String,
    pub url: String,
    pub license: String,
    pub license_url: String,
    pub permission: String,
    pub retrieved: String,
    pub bytes: u64,
    pub sha256: String,
    pub file: String,
    pub family: String,
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
    /// "ok" or "insufficient_context"
    pub status: String,
    pub query: String,
    pub passages: Vec<PassageHit>,
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
