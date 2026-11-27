use sha2::{Digest, Sha256};

/// Deterministic opaque citation token for a passage. Stable across reimports
/// and index rebuilds because it derives only from the passage identity.
pub fn token_for(passage_id: &str) -> String {
    let mut h = Sha256::new();
    h.update(b"ganesh-m1-citation-v1|");
    h.update(passage_id.as_bytes());
    let digest = h.finalize();
    hex(&digest[..12])
}

pub fn hex(bytes: &[u8]) -> String {
    let mut s = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        s.push_str(&format!("{b:02x}"));
    }
    s
}

pub fn sha256_hex(bytes: &[u8]) -> String {
    let mut h = Sha256::new();
    h.update(bytes);
    hex(&h.finalize())
}
