//! Owner-correction supersession: a scope-narrowing owner correction using
//! the default full supersession mode must retire its overbroad predecessor
//! on both scoped and unscoped reads (web-acceptance w6/w7/w9 regression).

use ganesh_core::citation::sha256_hex;
use ganesh_core::store::{eligible_revisions, Store};
use ganesh_core::types::ManifestEntry;

fn entry(revision: &str, scope: &str, supersedes: Option<&str>, mode: &str, text: &str) -> (ManifestEntry, Vec<u8>) {
    let bytes = text.as_bytes().to_vec();
    (
        ManifestEntry {
            article_id: "deployrules".to_string(),
            title: "Deploy rules".to_string(),
            pageid: 0,
            revision: revision.to_string(),
            revision_url: String::new(),
            publisher: "test".to_string(),
            url: String::new(),
            license: String::new(),
            license_url: String::new(),
            permission: String::new(),
            retrieved: String::new(),
            bytes: bytes.len() as u64,
            sha256: sha256_hex(&bytes),
            file: String::new(),
            family: String::new(),
            effective_from: None,
            effective_until: None,
            occurred_at: Some("2026-01-01T00:00:00Z".to_string()),
            author: "owner".to_string(),
            source_type: "document".to_string(),
            project: "alpha".to_string(),
            scope: scope.to_string(),
            authority: 1,
            supersedes: supersedes.map(|s| s.to_string()),
            supersede_mode: mode.to_string(),
            claim_key: "deploy-rules".to_string(),
            audience: String::new(),
            supports: vec![],
        },
        bytes,
    )
}

#[test]
fn full_supersession_retires_predecessor_on_scoped_and_unscoped_reads() {
    let dir = tempfile::tempdir().unwrap();
    let mut store = Store::open(&dir.path().join("t.db")).unwrap();

    let (e1, b1) = entry("deployrules-r1", "", None, "", "All deployments are automatic.\n\nDetails here.");
    store.import_snapshot(&e1, &b1).unwrap();
    // Owner narrows "all deployments" -> "staging only" with default full mode.
    let (e2, b2) = entry(
        "deployrules-r1-oc1",
        "staging",
        Some("deployrules-r1"),
        "full",
        "Deployments are automatic in staging only.\n\nDetails here.",
    );
    store.import_snapshot(&e2, &b2).unwrap();

    let projects = vec!["alpha".to_string()];
    let as_of = "2026-06-01T00:00:00Z";

    // Unscoped read: predecessor must be retired, correction eligible.
    let unscoped = eligible_revisions(&store, &projects, as_of, "");
    assert!(
        !unscoped.contains_key("deployrules-r1"),
        "full supersession must retire the predecessor on unscoped reads"
    );
    assert!(unscoped.contains_key("deployrules-r1-oc1"));

    // Scoped read matching the narrowed scope: same retirement.
    let scoped = eligible_revisions(&store, &projects, as_of, "staging");
    assert!(
        !scoped.contains_key("deployrules-r1"),
        "full supersession must retire the predecessor on scoped reads"
    );
    assert!(scoped.contains_key("deployrules-r1-oc1"));

    // Explicit partial mode preserves the predecessor on unscoped reads
    // (phase-1 semantics) — the opt-in escape hatch still works.
    let dir2 = tempfile::tempdir().unwrap();
    let mut store2 = Store::open(&dir2.path().join("t.db")).unwrap();
    store2.import_snapshot(&e1, &b1).unwrap();
    let (e3, b3) = entry(
        "deployrules-r1-oc1",
        "staging",
        Some("deployrules-r1"),
        "partial",
        "Deployments are automatic in staging only.\n\nDetails here.",
    );
    store2.import_snapshot(&e3, &b3).unwrap();
    let unscoped2 = eligible_revisions(&store2, &projects, as_of, "");
    assert!(
        unscoped2.contains_key("deployrules-r1"),
        "explicit partial mode keeps the predecessor on unscoped reads"
    );
}
