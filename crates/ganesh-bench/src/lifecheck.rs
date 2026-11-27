//! Generic data-driven lifecycle scenario runner. A scenario JSON describes
//! imports, revocations, restarts and expectations over a live two-project
//! server; used by frozen transfer packets to exercise lifecycle behavior
//! without hardcoded revision IDs.

use ganesh_core::store::{revoke, Store};
use ganesh_core::types::Manifest;
use serde_json::{json, Value};
use std::io::{BufRead, BufReader};
use std::process::{Child, Command, Stdio};
use std::time::Duration;

fn sibling_binary(name: &str) -> std::path::PathBuf {
    let exe = std::env::current_exe().unwrap();
    exe.parent().unwrap().join(name)
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
    let addr = line.trim().strip_prefix("listening ").expect("listening line").to_string();
    Server { child, rest: format!("http://{addr}") }
}

fn rest_search(rest: &str, key: &str, query: &str, as_of: Option<&str>) -> Value {
    let mut body = json!({ "query": query, "key": key });
    if let Some(d) = as_of {
        body["as_of"] = json!(d);
    }
    match ureq::post(&format!("{rest}/v1/search")).send_json(body) {
        Ok(resp) => resp.into_json::<Value>().unwrap_or(json!({"status":"decode_error"})),
        Err(e) => json!({"status": format!("http_error: {e}")}),
    }
}

fn rest_open(rest: &str, key: &str, token: &str) -> bool {
    matches!(ureq::get(&format!("{rest}/v1/citations/{token}?key={key}")).call(), Ok(_))
}

fn first_passage_token(db: &std::path::Path, revision: &str) -> String {
    let store = Store::open(db).expect("open");
    let pid: String = store
        .conn
        .query_row(
            "SELECT id FROM passages WHERE revision = ?1 ORDER BY seq ASC LIMIT 1",
            rusqlite::params![revision],
            |row| row.get(0),
        )
        .unwrap_or_else(|_| format!("{revision}#p1"));
    ganesh_core::citation::token_for(&pid)
}

pub fn run_scenario(corpus_dir: &str, scenario_path: &str) -> i32 {
    let manifest: Manifest = serde_json::from_str(
        &std::fs::read_to_string(format!("{corpus_dir}/manifest.json")).expect("manifest"),
    ).expect("parse manifest");
    let scenario: Value = serde_json::from_str(
        &std::fs::read_to_string(scenario_path).expect("scenario"),
    ).expect("parse scenario");
    let tmp = tempfile::tempdir().expect("tmpdir");
    let db_path = tmp.path().join("xfer.db");
    let policy_path = std::path::PathBuf::from(format!("{corpus_dir}/policy.json"));

    let mut passed = 0usize;
    let mut total = 0usize;
    let mut server: Option<Server> = None;

    let import_one = |store: &mut Store, rev: &str| {
        let entry = manifest.articles.iter().find(|a| a.revision == rev).unwrap_or_else(|| panic!("entry {rev}"));
        let bytes = std::fs::read(format!("{corpus_dir}/{}", entry.file)).expect("bytes");
        store.import_snapshot(entry, &bytes).expect("import");
    };

    for step in scenario["steps"].as_array().expect("steps") {
        if let Some(revs) = step.get("import").and_then(Value::as_array) {
            let mut store = Store::open(&db_path).expect("open");
            for rev in revs {
                import_one(&mut store, rev.as_str().unwrap());
            }
            store.rebuild_index().expect("rebuild");
            println!("  step: import {} revisions", revs.len());
        }
        if step.get("start_server").and_then(Value::as_bool).unwrap_or(false) {
            server = Some(spawn_server(&db_path, &policy_path));
            println!("  step: server start");
        }
        if step.get("restart").and_then(Value::as_bool).unwrap_or(false) {
            if let Some(mut s) = server.take() {
                let _ = s.child.kill();
                let _ = s.child.wait();
            }
            std::thread::sleep(Duration::from_millis(300));
            server = Some(spawn_server(&db_path, &policy_path));
            println!("  step: server restart");
        }
        if let Some(rv) = step.get("revoke") {
            let store = Store::open(&db_path).expect("open");
            revoke(&store, rv["revision"].as_str().unwrap(), rv["project"].as_str().unwrap()).expect("revoke");
            std::thread::sleep(Duration::from_millis(150));
            println!("  step: revoke {} for {}", rv["revision"], rv["project"]);
        }
        if let Some(es) = step.get("expect_search") {
            total += 1;
            let rest = &server.as_ref().expect("server running").rest;
            let r = rest_search(
                rest,
                es["key"].as_str().unwrap(),
                es["query"].as_str().unwrap(),
                es.get("as_of").and_then(Value::as_str),
            );
            let status = r.get("status").and_then(Value::as_str).unwrap_or("missing");
            let revs: Vec<String> = r.get("passages").and_then(Value::as_array)
                .map(|ps| ps.iter().filter_map(|p| p.get("revision").and_then(Value::as_str).map(String::from)).collect())
                .unwrap_or_default();
            let mut ok = status == es["status"].as_str().unwrap();
            if let Some(inc) = es.get("must_include_rev").and_then(Value::as_array) {
                for want in inc {
                    ok = ok && revs.contains(&want.as_str().unwrap().to_string());
                }
            }
            if let Some(exc) = es.get("must_exclude_rev").and_then(Value::as_array) {
                for unwanted in exc {
                    ok = ok && !revs.contains(&unwanted.as_str().unwrap().to_string());
                }
            }
            let name = es.get("name").and_then(Value::as_str).unwrap_or("expect_search");
            println!("  [{}] {name} — status {status} revs {:?}", if ok { "PASS" } else { "FAIL" }, revs);
            if ok { passed += 1; }
        }
        if let Some(ec) = step.get("expect_citation") {
            total += 1;
            let rest = &server.as_ref().expect("server running").rest;
            let token = first_passage_token(&db_path, ec["revision"].as_str().unwrap());
            let got = rest_open(rest, ec["key"].as_str().unwrap(), &token);
            let want = ec["openable"].as_bool().unwrap();
            let ok = got == want;
            let name = ec.get("name").and_then(Value::as_str).unwrap_or("expect_citation");
            println!("  [{}] {name} — openable {got} want {want}", if ok { "PASS" } else { "FAIL" });
            if ok { passed += 1; }
        }
        if let Some(nl) = step.get("expect_no_leak") {
            total += 1;
            let rest = &server.as_ref().expect("server running").rest;
            let r = rest_search(
                rest,
                nl["key"].as_str().unwrap(),
                nl["query"].as_str().unwrap(),
                nl.get("as_of").and_then(Value::as_str),
            );
            let body = serde_json::to_string(&r).unwrap_or_default();
            let needles: Vec<String> = nl["strings"].as_array().unwrap().iter().filter_map(|s| s.as_str().map(String::from)).collect();
            let ok = !needles.iter().any(|n| body.contains(n));
            let name = nl.get("name").and_then(Value::as_str).unwrap_or("expect_no_leak");
            println!("  [{}] {name} — leak-free {ok}", if ok { "PASS" } else { "FAIL" });
            if ok { passed += 1; }
        }
    }
    if let Some(mut s) = server.take() {
        let _ = s.child.kill();
        let _ = s.child.wait();
    }
    println!("scenario: {passed}/{total} expectations passed");
    if passed == total { 0 } else { 1 }
}
