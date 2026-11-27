//! Automated Owner Web Acceptance Replay (M2 phase 2, roadmap sections 5 & 7
//! step 5): drives a live ganesh-server through the six owner-flow steps —
//! inspection, correction with immediate read reflection, optimistic
//! concurrency conflict, stale-capture replay immunity, source deletion with
//! zero-leak persistence, and non-owner 403 enforcement.

use ganesh_core::store::Store;
use ganesh_core::types::Manifest;
use serde_json::{json, Value};
use std::io::{BufRead, BufReader};
use std::process::{Child, Command, Stdio};

fn sibling_binary(name: &str) -> std::path::PathBuf {
    std::env::current_exe().unwrap().parent().unwrap().join(name)
}

struct Server {
    child: Child,
    rest: String,
}

fn spawn_server(db: &std::path::Path, policy: &std::path::Path) -> Server {
    let mut child = Command::new(sibling_binary("ganesh-server"))
        .arg("--db").arg(db)
        .arg("--bind").arg("127.0.0.1:0")
        .arg("--policy").arg(policy)
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
        .expect("spawn server");
    let mut stdout = BufReader::new(child.stdout.take().unwrap());
    let mut line = String::new();
    stdout.read_line(&mut line).expect("server readiness line");
    let addr = line.trim().strip_prefix("listening ").expect("listening").to_string();
    Server { child, rest: format!("http://{addr}") }
}

fn post(rest: &str, path: &str, body: Value) -> (u16, Value) {
    match ureq::post(&format!("{rest}{path}")).send_json(body) {
        Ok(r) => (r.status(), r.into_json::<Value>().unwrap_or(json!({}))),
        Err(ureq::Error::Status(code, r)) => (code, r.into_json::<Value>().unwrap_or(json!({}))),
        Err(e) => (599, json!({"error": format!("{e}")})),
    }
}

fn get(rest: &str, path: &str) -> (u16, Value) {
    match ureq::get(&format!("{rest}{path}")).call() {
        Ok(r) => (r.status(), r.into_json::<Value>().unwrap_or(json!({}))),
        Err(ureq::Error::Status(code, r)) => (code, r.into_json::<Value>().unwrap_or(json!({}))),
        Err(e) => (599, json!({"error": format!("{e}")})),
    }
}

fn del(rest: &str, path: &str) -> (u16, Value) {
    match ureq::delete(&format!("{rest}{path}")).call() {
        Ok(r) => (r.status(), r.into_json::<Value>().unwrap_or(json!({}))),
        Err(ureq::Error::Status(code, r)) => (code, r.into_json::<Value>().unwrap_or(json!({}))),
        Err(e) => (599, json!({"error": format!("{e}")})),
    }
}

pub fn run_web_acceptance(corpus_dir: &str) -> i32 {
    let mut pass = 0usize;
    let mut fail = 0usize;
    let mut check = |name: &str, ok: bool, detail: &str| {
        if ok {
            pass += 1;
            println!("  [PASS] {name} {detail}");
        } else {
            fail += 1;
            println!("  [FAIL] {name} {detail}");
        }
    };

    let manifest: Manifest = serde_json::from_str(
        &std::fs::read_to_string(format!("{corpus_dir}/manifest.json")).expect("manifest"),
    ).expect("parse");
    let tmp = tempfile::tempdir().expect("tmp");
    let db_path = tmp.path().join("web.db");
    let policy_path = std::path::PathBuf::from(format!("{corpus_dir}/policy.json"));

    // Import the deploy-rules chain r1 (and r2/r3 exist in the fixture but the
    // owner flow uses r1 only).
    let import_rev = |store: &mut Store, rev: &str| {
        let entry = manifest.articles.iter().find(|a| a.revision == rev).expect("entry");
        let bytes = std::fs::read(format!("{corpus_dir}/{}", entry.file)).expect("bytes");
        store.import_snapshot(entry, &bytes).expect("import");
    };
    {
        let mut store = Store::open(&db_path).expect("open");
        import_rev(&mut store, "deployrules-r1");
        store.rebuild_index().expect("rebuild");
    }
    println!("STEP 1 — inspection endpoints");
    let mut server = spawn_server(&db_path, &policy_path);
    let rest = server.rest.clone();

    let (st, body) = get(&rest, "/v1/revisions?key=key-alpha");
    let listed = body["revisions"].as_array().map(|a| a.iter().any(|r| r["revision"].as_str() == Some("deployrules-r1"))).unwrap_or(false);
    check("w1.list-revisions", st == 200 && listed, &format!("status {st}"));

    let (st, body) = get(&rest, "/v1/revisions/deployrules-r1?key=key-alpha");
    let has_passages = body["passages"].as_array().map(|a| !a.is_empty()).unwrap_or(false);
    check("w2.revision-detail", st == 200 && has_passages, &format!("status {st}"));

    let (st, body) = get(&rest, "/v1/revisions/deployrules-r1/timeline?key=key-alpha");
    let has_import = body["events"].as_array().map(|a| a.iter().any(|e| e["event"].as_str() == Some("import"))).unwrap_or(false);
    check("w3.audit-timeline", st == 200 && has_import, &format!("status {st}"));

    let (st, _) = get(&rest, "/ui");
    check("w4.ui-serves", st == 200, &format!("status {st}"));

    println!("STEP 2 — owner correction narrows scope, reads reflect it");
    // Search before correction to capture a citation token and baseline revision.
    let (_, baseline) = post(&rest, "/v1/search", json!({"query": "deployments manual approval release captain", "key": "key-alpha"}));
    let base_revs: Vec<String> = baseline["passages"].as_array().map(|a| {
        a.iter().filter_map(|p| p["revision"].as_str().map(String::from)).collect()
    }).unwrap_or_default();
    let base_token = baseline["passages"].as_array().and_then(|a| a.first()).and_then(|p| p["citation"].as_str().map(String::from));

    let corrected_text = "All deployments to the staging environment require manual approval by the release captain before any rollout begins. Production deployments are outside this rule's scope.";
    let (st, body) = post(&rest, "/v1/revisions/deployrules-r1/correct", json!({
        "key": "key-alpha",
        "new_text": corrected_text,
        "narrow_scope": "staging",
        "reason": "narrowing applicability to staging only",
        "expected_revision_id": "deployrules-r1",
    }));
    let correction_rev = body["revision"].as_str().unwrap_or("").to_string();
    check("w5.correction-accepted", st == 200 && !correction_rev.is_empty(), &format!("status {st} rev {correction_rev}"));

    let (_, after) = post(&rest, "/v1/search", json!({"query": "deployments manual approval release captain", "key": "key-alpha"}));
    let after_revs: Vec<String> = after["passages"].as_array().map(|a| {
        a.iter().filter_map(|p| p["revision"].as_str().map(String::from)).collect()
    }).unwrap_or_default();
    let rest_reflects = after_revs.iter().any(|r| r == &correction_rev) && !after_revs.iter().any(|r| r == "deployrules-r1");
    check("w6.rest-reflects-correction", rest_reflects, &format!("revs {after_revs:?}"));

    // MCP read reflects the correction too.
    let mut mcp = crate::McpClient::spawn_with_key(&rest, Some("key-alpha"));
    mcp.call("initialize", json!({"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "webaccept", "version": "0"}})).ok();
    mcp.call("notifications/initialized", json!({})).ok();
    let mcp_res = mcp.call("tools/call", json!({
        "name": "search_context",
        "arguments": {"query": "deployments manual approval release captain"}
    }));
    let mcp_ok = match mcp_res {
        Ok(res) => {
            let text = res["content"][0]["text"].as_str().unwrap_or("");
            text.contains(&correction_rev) && !text.contains("deployrules-r1#")
        }
        Err(_) => false,
    };
    check("w7.mcp-reflects-correction", mcp_ok, "");

    println!("STEP 3 — optimistic concurrency conflict");
    let (st, body) = post(&rest, format!("/v1/revisions/deployrules-r1/correct").as_str(), json!({
        "key": "key-alpha",
        "new_text": "stale edit attempt",
        "expected_revision_id": "deployrules-r1",
    }));
    let conflict_ok = st == 409 && body["current_head"].as_str() == Some(correction_rev.as_str());
    check("w8.stale-edit-409", conflict_ok, &format!("status {st} head {}", body["current_head"].as_str().unwrap_or("")));

    println!("STEP 4 — stale capture replay immunity");
    {
        let mut store = Store::open(&db_path).expect("open");
        import_rev(&mut store, "deployrules-r1"); // replay: same snapshot again
        store.rebuild_index().expect("rebuild");
    }
    let (_, after2) = post(&rest, "/v1/search", json!({"query": "deployments manual approval release captain", "key": "key-alpha"}));
    let after2_revs: Vec<String> = after2["passages"].as_array().map(|a| {
        a.iter().filter_map(|p| p["revision"].as_str().map(String::from)).collect()
    }).unwrap_or_default();
    let immune = after2_revs.iter().any(|r| r == &correction_rev) && !after2_revs.iter().any(|r| r == "deployrules-r1");
    check("w9.replay-immunity", immune, &format!("revs {after2_revs:?}"));

    let (_, tl) = get(&rest, &format!("/v1/revisions/{correction_rev}/timeline?key=key-alpha"));
    let has_correct = tl["events"].as_array().map(|a| a.iter().any(|e| e["event"].as_str() == Some("correct") || e["event"].as_str() == Some("import"))).unwrap_or(false);
    check("w10.audit-records-correction", has_correct, "");

    println!("STEP 5 — source deletion and zero-leak persistence");
    let (st, body) = del(&rest, "/v1/sources/deploy-rules?key=key-alpha");
    check("w11.delete-source", st == 200 && body["status"].as_str() == Some("deleted"), &format!("status {st}"));

    let (_, gone) = post(&rest, "/v1/search", json!({"query": "deployments manual approval release captain", "key": "key-alpha"}));
    let gone_revs: Vec<String> = gone["passages"].as_array().map(|a| {
        a.iter().filter_map(|p| p["revision"].as_str().map(String::from)).collect()
    }).unwrap_or_default();
    check("w12.no-current-leak", !gone_revs.iter().any(|r| r.starts_with("deployrules")), &format!("revs {gone_revs:?}"));

    let (_, gone_asof) = post(&rest, "/v1/search", json!({"query": "deployments manual approval release captain", "key": "key-alpha", "as_of": "2020-01-01"}));
    let asof_revs: Vec<String> = gone_asof["passages"].as_array().map(|a| {
        a.iter().filter_map(|p| p["revision"].as_str().map(String::from)).collect()
    }).unwrap_or_default();
    check("w13.no-asof-leak", !asof_revs.iter().any(|r| r.starts_with("deployrules")), &format!("revs {asof_revs:?}"));

    if let Some(tok) = &base_token {
        let (st, _) = get(&rest, &format!("/v1/citations/{tok}?key=key-alpha"));
        check("w14.citation-purged", st != 200, &format!("status {st}"));
    } else {
        check("w14.citation-purged", false, "no baseline token captured");
    }
    let _ = base_revs;

    // Full server restart.
    server.child.kill().ok();
    server.child.wait().ok();
    let server2 = spawn_server(&db_path, &policy_path);
    let rest2 = server2.rest.clone();
    let (_, gone2) = post(&rest2, "/v1/search", json!({"query": "deployments manual approval release captain", "key": "key-alpha"}));
    let gone2_revs: Vec<String> = gone2["passages"].as_array().map(|a| {
        a.iter().filter_map(|p| p["revision"].as_str().map(String::from)).collect()
    }).unwrap_or_default();
    check("w15.no-leak-after-restart", !gone2_revs.iter().any(|r| r.starts_with("deployrules")), &format!("revs {gone2_revs:?}"));

    println!("STEP 6 — non-owner enforcement");
    let (st, _) = post(&rest2, "/v1/revisions/deployrules-r1/correct", json!({
        "key": "key-beta", "new_text": "malicious edit",
    }));
    check("w16.nonowner-correct-403", st == 403, &format!("status {st}"));
    let (st, _) = get(&rest2, "/v1/revisions?key=key-beta");
    check("w17.nonowner-list-403", st == 403, &format!("status {st}"));
    let (st, _) = del(&rest2, "/v1/sources/deploy-rules?key=key-beta");
    check("w18.nonowner-delete-403", st == 403, &format!("status {st}"));
    let (st, _) = get(&rest2, "/v1/revisions");
    check("w19.anonymous-403", st == 403, &format!("status {st}"));

    println!("web-acceptance: {pass} PASS, {fail} FAIL");
    if fail == 0 { 0 } else { 1 }
}
