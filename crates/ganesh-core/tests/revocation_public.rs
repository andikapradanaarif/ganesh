//! Empirical regression test for the review-confirmed access-control hole:
//! revoking a PUBLIC revision must block every credentialed caller, and
//! time-bounded revisions' citations must open inside their window.

use ganesh_core::citation::sha256_hex;
use ganesh_core::store::{all_revision_meta, eligible_revisions, revoke, visible_to, Store};
use ganesh_core::types::ManifestEntry;

fn entry(revision: &str, project: &str, until: Option<&str>) -> ManifestEntry {
    let text = format!("body of {revision}");
    ManifestEntry {
        article_id: format!("art-{revision}"),
        title: format!("t-{revision}"),
        pageid: 0,
        revision: revision.to_string(),
        revision_url: String::new(),
        publisher: "test".into(),
        url: String::new(),
        license: String::new(),
        license_url: String::new(),
        permission: String::new(),
        retrieved: String::new(),
        bytes: text.len() as u64,
        sha256: sha256_hex(text.as_bytes()),
        file: String::new(),
        family: String::new(),
        effective_from: None,
        effective_until: until.map(|u| u.to_string()),
        occurred_at: None,
        author: String::new(),
        source_type: "wiki".into(),
        project: project.to_string(),
        scope: String::new(),
        authority: 0,
        supersedes: None,
        supersede_mode: String::new(),
        claim_key: String::new(),
        audience: String::new(),
        supports: vec![],
    }
}

fn store_with(rev: &str, project: &str, until: Option<&str>) -> (tempfile::TempDir, Store) {
    let dir = tempfile::tempdir().unwrap();
    let db = dir.path().join("t.db");
    let mut store = Store::open(&db).unwrap();
    let e = entry(rev, project, until);
    let text = format!("body of {rev}");
    store.import_snapshot(&e, text.as_bytes()).unwrap();
    (dir, store)
}

#[test]
fn revoking_public_revision_blocks_credentialed_callers() {
    let (_d, store) = store_with("pubdoc-r1", "public", None);
    let callers = vec!["public".to_string(), "alpha".to_string()];
    let metas = all_revision_meta(&store);
    let meta = metas.iter().find(|m| m.revision == "pubdoc-r1").unwrap();
    assert!(visible_to(&store, meta, &callers), "visible before revocation");
    revoke(&store, "pubdoc-r1", "public").unwrap();
    assert!(
        !visible_to(&store, meta, &callers),
        "revoke(rev, public) must block a caller carrying extra projects — the hole"
    );
}

#[test]
fn revoking_project_revision_still_blocks_that_project() {
    let (_d, store) = store_with("alphadoc-r1", "alpha", None);
    let callers = vec!["public".to_string(), "alpha".to_string()];
    let metas = all_revision_meta(&store);
    let meta = metas.iter().find(|m| m.revision == "alphadoc-r1").unwrap();
    assert!(visible_to(&store, meta, &callers));
    revoke(&store, "alphadoc-r1", "alpha").unwrap();
    assert!(!visible_to(&store, meta, &callers));
}

#[test]
fn timebounded_revision_citation_opens_inside_window() {
    // effective_until in the future: visible in search AND citation openable.
    let (_d, store) = store_with("timebox-r1", "public", Some("2999-01-01"));
    let projects = vec!["public".to_string()];
    let now = ganesh_core::types::now_iso();
    let elig = eligible_revisions(&store, &projects, &now, "");
    assert!(elig.contains_key("timebox-r1"), "in-window revision eligible");
    // citation path: the citation token comes from passages' citation rows —
    // import created them; fetch any token bound to the revision's passage.
    let mut stmt = store.conn.prepare(
        "SELECT c.token FROM citations c JOIN passages p ON c.passage_id = p.id WHERE p.revision = 'timebox-r1' LIMIT 1",
    ).unwrap();
    let token: String = stmt
        .query_row([], |r| r.get(0))
        .expect("import must create a citation token for the passage");
    let opened = store.open_citation(&token, "public").unwrap();
    assert!(opened.is_some(), "citation must open inside the effective window");
}
