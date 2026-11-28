//! M2 lifecycle replay gate (roadmap section 7). Drives a live server through
//! the five lifecycle scenarios end-to-end over BOTH REST and MCP, including
//! revocation immediacy, restart persistence, late-arrival replay, conflict
//! surfacing, scoped partial correction and zero-leak assertions.

use ganesh_core::store::{delete_revision, revoke, Store};
use ganesh_core::types::Manifest;
use serde_json::{json, Value};
use std::io::{BufRead, BufReader, Write};
use std::process::{Child, Command, Stdio};
use std::sync::mpsc;
use std::time::Duration;

struct Ctx {
    checks: Vec<(String, bool, String)>,
}

impl Ctx {
    fn check(&mut self, name: &str, ok: bool, detail: impl Into<String>) {
        self.checks.push((name.to_string(), ok, detail.into()));
        let mark = if ok { "PASS" } else { "FAIL" };
        println!("  [{mark}] {name} — {}", self.checks.last().unwrap().2);
    }
}

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

fn rest_search(rest: &str, key: &str, query: &str, as_of: Option<&str>, top_k: Option<usize>) -> Value {
    let mut body = json!({ "query": query, "key": key });
    if let Some(d) = as_of {
        body["as_of"] = json!(d);
    }
    if let Some(k) = top_k {
        body["top_k"] = json!(k);
    }
    match ureq::post(&format!("{rest}/v1/search")).send_json(body) {
        Ok(resp) => resp.into_json::<Value>().unwrap_or(json!({"status":"decode_error"})),
        Err(e) => json!({"status": format!("http_error: {e}")}),
    }
}

fn rest_open(rest: &str, key: &str, token: &str) -> Result<Value, u16> {
    match ureq::get(&format!("{rest}/v1/citations/{token}?key={key}")).call() {
        Ok(resp) => Ok(resp.into_json::<Value>().unwrap_or(json!({}))),
        Err(ureq::Error::Status(code, _)) => Err(code),
        Err(_) => Err(0),
    }
}

struct Mcp {
    child: Child,
    stdin: std::process::ChildStdin,
    rx: mpsc::Receiver<String>,
    id: u64,
}

impl Mcp {
    fn spawn(rest: &str, key: &str) -> Mcp {
        let mut child = Command::new(sibling_binary("ganesh-mcp"))
            .env("GANESH_REST_URL", rest)
            .env("GANESH_API_KEY", key)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
            .expect("spawn ganesh-mcp");
        let stdin = child.stdin.take().unwrap();
        let stdout = child.stdout.take().unwrap();
        let (tx, rx) = mpsc::channel();
        std::thread::spawn(move || {
            for line in BufReader::new(stdout).lines() {
                match line {
                    Ok(l) => {
                        if tx.send(l).is_err() {
                            break;
                        }
                    }
                    Err(_) => break,
                }
            }
        });
        let mut m = Mcp { child, stdin, rx, id: 0 };
        m.call("initialize", json!({"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"lifecycle","version":"0"}}));
        m.notify("notifications/initialized");
        m
    }
    fn notify(&mut self, method: &str) {
        let msg = json!({"jsonrpc":"2.0","method":method});
        let _ = writeln!(self.stdin, "{}", serde_json::to_string(&msg).unwrap());
        let _ = self.stdin.flush();
    }
    fn call(&mut self, method: &str, params: Value) -> Option<Value> {
        self.id += 1;
        let id = self.id;
        let msg = json!({"jsonrpc":"2.0","id":id,"method":method,"params":params});
        writeln!(self.stdin, "{}", serde_json::to_string(&msg).unwrap()).ok()?;
        self.stdin.flush().ok()?;
        loop {
            let line = self.rx.recv_timeout(Duration::from_secs(30)).ok()?;
            let v: Value = serde_json::from_str(&line).ok()?;
            if v.get("id").and_then(Value::as_u64) == Some(id) {
                return v.get("result").cloned();
            }
        }
    }
    fn search(&mut self, query: &str, as_of: Option<&str>) -> Value {
        let mut args = json!({"query": query});
        if let Some(d) = as_of {
            args["as_of"] = json!(d);
        }
        let res = self.call("tools/call", json!({"name":"search_context","arguments":args}));
        match res {
            Some(r) => {
                let text = r.pointer("/content/0/text").and_then(Value::as_str).unwrap_or("");
                serde_json::from_str(text).unwrap_or(json!({"status":"mcp_decode_error","raw":text}))
            }
            None => json!({"status":"mcp_error"}),
        }
    }
    fn open(&mut self, token: &str) -> Value {
        let res = self.call("tools/call", json!({"name":"open_citation","arguments":{"citation":token}}));
        match res {
            Some(r) => {
                let text = r.pointer("/content/0/text").and_then(Value::as_str).unwrap_or("").to_string();
                let err = r.get("isError").and_then(Value::as_bool).unwrap_or(false);
                if err {
                    json!({"status":"error","raw":text})
                } else {
                    serde_json::from_str(&text).unwrap_or(json!({"status":"mcp_decode_error","raw":text}))
                }
            }
            None => json!({"status":"mcp_error"}),
        }
    }
}

impl Drop for Mcp {
    fn drop(&mut self) {
        let _ = self.child.kill();
    }
}

/// First passage id of a revision, looked up in the store — robust against
/// segmentation numbering details.
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

fn status_of(v: &Value) -> String {
    v.get("status").and_then(Value::as_str).unwrap_or("missing").to_string()
}

fn revision_set(v: &Value) -> Vec<String> {
    v.get("passages")
        .and_then(Value::as_array)
        .map(|ps| ps.iter().filter_map(|p| p.get("revision").and_then(Value::as_str).map(String::from)).collect())
        .unwrap_or_default()
}

fn body_text(v: &Value) -> String {
    serde_json::to_string(v).unwrap_or_default()
}

const SECRETS: &[&str] = &["SECRETXVZ", "QXZSECRET", "/private/alpha-repo"];

fn leak_free(body: &str) -> bool {
    !SECRETS.iter().any(|s| body.contains(s))
}

pub fn run_lifecycle(corpus_dir: &str) -> i32 {
    let mut cx = Ctx { checks: vec![] };
    let manifest: Manifest = serde_json::from_str(
        &std::fs::read_to_string(format!("{corpus_dir}/manifest.json")).expect("manifest"),
    ).expect("parse manifest");
    let tmp = tempfile::tempdir().expect("tmpdir");
    let db_path = tmp.path().join("lifecycle.db");
    let policy_path = std::path::PathBuf::from(format!("{corpus_dir}/policy.json"));

    let import = |store: &mut Store, rev: &str| {
        let entry = manifest.articles.iter().find(|a| a.revision == rev).expect("entry");
        let bytes = std::fs::read(format!("{corpus_dir}/snapshots/{}", entry.file)).expect("snapshot bytes");
        store.import_snapshot(entry, &bytes).expect("import");
    };

    // ---- setup: initial revision set -------------------------------------
    {
        let mut store = Store::open(&db_path).expect("open store");
        for rev in ["deployrules-r1", "cacheev-s1", "cacheev-s2", "cacheclaim-r1", "alphapriv-r1", "publesson-r1"] {
            import(&mut store, rev);
        }
        store.rebuild_index().expect("rebuild");
    }

    let mut server = spawn_server(&db_path, &policy_path);
    let mut mcp_a = Mcp::spawn(&server.rest, "key-alpha");
    let mut mcp_b = Mcp::spawn(&server.rest, "key-beta");

    // ================= STEP 1: original rule + permitted publication =======
    println!("STEP 1 — original rule and publication");
    let r = rest_search(&server.rest, "key-alpha", "deployment manual approval release captain", None, None);
    cx.check("s1.alpha-sees-original-rule", status_of(&r) == "ok" && revision_set(&r).contains(&"deployrules-r1".to_string()), format!("status {} revs {:?}", status_of(&r), revision_set(&r)));

    let rb = rest_search(&server.rest, "key-beta", "generated client schema regenerate lesson", None, None);
    let b_revs = revision_set(&rb);
    cx.check("s1.beta-reads-published-lesson", status_of(&rb) == "ok" && b_revs.contains(&"publesson-r1".to_string()), format!("status {} revs {:?}", status_of(&rb), b_revs));

    let cit = first_passage_token(&db_path, "publesson-r1");
    let opened = rest_open(&server.rest, "key-beta", &cit);
    cx.check("s1.beta-opens-publication-citation", opened.is_ok(), format!("{:?}", opened.is_ok()));

    let rb2 = rest_search(&server.rest, "key-beta", "transkript generated client private repo path alpha", None, None);
    cx.check("s1.beta-no-private-access", status_of(&rb2) == "insufficient_context", format!("status {}", status_of(&rb2)));
    cx.check("s1.beta-zero-leak", leak_free(&body_text(&rb2)) && leak_free(&body_text(&rb)), "no secret strings in any beta body");

    let mb = mcp_b.search("generated client schema regenerate lesson", None);
    cx.check("s1.mcp-beta-reads-publication", status_of(&mb) == "ok", format!("status {}", status_of(&mb)));
    let mb2 = mcp_b.search("transkript private alpha repo", None);
    cx.check("s1.mcp-beta-zero-leak", leak_free(&body_text(&mb2)) && status_of(&mb2) == "insufficient_context", format!("status {}", status_of(&mb2)));

    // ================= STEP 2: partial correction + as-of + late arrival ===
    println!("STEP 2 — partial correction, historical reads, late arrival");
    {
        let mut store = Store::open(&db_path).expect("open");
        import(&mut store, "deployrules-r2");
        store.rebuild_index().expect("rebuild");
    }
    std::thread::sleep(Duration::from_millis(150));

    let may = rest_search(&server.rest, "key-alpha", "does staging require manual approval", Some("2026-05-15"), None);
    cx.check("s2.may-staging-returns-r1", revision_set(&may).contains(&"deployrules-r1".to_string()) && !revision_set(&may).contains(&"deployrules-r2".to_string()), format!("revs {:?}", revision_set(&may)));
    let jun = rest_search(&server.rest, "key-alpha", "does staging require manual approval", Some("2026-06-10"), None);
    cx.check("s2.june-staging-returns-r2", revision_set(&jun).contains(&"deployrules-r2".to_string()) && !revision_set(&jun).contains(&"deployrules-r1".to_string()), format!("revs {:?}", revision_set(&jun)));
    let prod = rest_search(&server.rest, "key-alpha", "does production require manual approval", Some("2026-06-10"), None);
    cx.check("s2.june-production-still-r1", revision_set(&prod).contains(&"deployrules-r1".to_string()), format!("revs {:?}", revision_set(&prod)));
    let now = rest_search(&server.rest, "key-alpha", "does staging require manual approval", None, None);
    cx.check("s2.current-staging-returns-r2", revision_set(&now).contains(&"deployrules-r2".to_string()), format!("revs {:?}", revision_set(&now)));

    // Late arrival, made genuine: DELETE the older revision first, then
    // reimport it — so r1 truly arrives in the store AFTER its successor r2.
    // (A plain reimport returns AlreadyPresent and writes nothing, which
    // cannot distinguish read-time supersession from ingestion-order luck.)
    {
        let mut store = Store::open(&db_path).expect("open");
        delete_revision(&store, "deployrules-r1").expect("delete r1");
        import(&mut store, "deployrules-r1");
        store.rebuild_index().expect("rebuild");
    }
    std::thread::sleep(Duration::from_millis(150));
    let may2 = rest_search(&server.rest, "key-alpha", "does staging require manual approval", Some("2026-05-15"), None);
    let jun2 = rest_search(&server.rest, "key-alpha", "does staging require manual approval", Some("2026-06-10"), None);
    let prod2 = rest_search(&server.rest, "key-alpha", "does production require manual approval", Some("2026-06-10"), None);
    cx.check("s2.late-arrival-no-change", revision_set(&may2) == revision_set(&may) && revision_set(&jun2) == revision_set(&jun) && revision_set(&prod2) == revision_set(&prod), "all three reads identical after late reimport");

    let ma = mcp_a.search("does staging require manual approval", Some("2026-06-10"));
    cx.check("s2.mcp-adapter-agrees", revision_set(&ma).contains(&"deployrules-r2".to_string()), format!("mcp revs {:?}", revision_set(&ma)));

    // ================= STEP 3: conflict + narrowing ========================
    println!("STEP 3 — conflicting evidence and scope narrowing");
    {
        let mut store = Store::open(&db_path).expect("open");
        import(&mut store, "incident77-x");
        import(&mut store, "incident77-y");
        store.rebuild_index().expect("rebuild");
    }
    std::thread::sleep(Duration::from_millis(150));
    let conf = rest_search(&server.rest, "key-alpha", "what was the root cause of incident 77", None, Some(6));
    let crevs = revision_set(&conf);
    cx.check("s3.conflict-status", status_of(&conf) == "conflicting_evidence", format!("status {}", status_of(&conf)));
    cx.check("s3.conflict-cites-both", crevs.contains(&"incident77-x".to_string()) && crevs.contains(&"incident77-y".to_string()), format!("revs {:?}", crevs));

    {
        let mut store = Store::open(&db_path).expect("open");
        import(&mut store, "incident77-x2");
        store.rebuild_index().expect("rebuild");
    }
    std::thread::sleep(Duration::from_millis(150));
    let prodc = rest_search(&server.rest, "key-alpha", "what was the root cause of incident 77 in production", None, Some(6));
    let prevs = revision_set(&prodc);
    cx.check("s3.production-single-answer", status_of(&prodc) != "conflicting_evidence" && prevs.contains(&"incident77-y".to_string()) && !prevs.contains(&"incident77-x".to_string()) && !prevs.contains(&"incident77-x2".to_string()), format!("status {} revs {:?}", status_of(&prodc), prevs));

    // ================= STEP 4: dual-source removal + zero-leak revocation ==
    println!("STEP 4 — multi-source removal, revocation, restart persistence");
    // positive controls: beta can resolve claim and open citations now
    let pre = rest_search(&server.rest, "key-beta", "cache fleet instances capacity planning", None, None);
    cx.check("s4.beta-sees-claim-initially", status_of(&pre) == "ok", format!("status {}", status_of(&pre)));
    let claim_cit = first_passage_token(&db_path, "cacheclaim-r1");
    let s1_cit = first_passage_token(&db_path, "cacheev-s1");
    let s2_cit = first_passage_token(&db_path, "cacheev-s2");
    cx.check("s4.beta-opens-citations-pre", rest_open(&server.rest, "key-beta", &claim_cit).is_ok() && rest_open(&server.rest, "key-beta", &s2_cit).is_ok(), "claim+s2 citations open");

    // 4a: remove one of two independent supports entirely
    {
        let store = Store::open(&db_path).expect("open");
        revoke(&store, "cacheev-s1", "alpha").expect("revoke");
        revoke(&store, "cacheev-s1", "beta").expect("revoke");
    }
    std::thread::sleep(Duration::from_millis(150));
    let post_a = rest_search(&server.rest, "key-beta", "cache fleet instances capacity planning", None, None);
    cx.check("s4.claim-survives-one-removal", status_of(&post_a) == "ok", format!("status {}", status_of(&post_a)));
    cx.check("s4.removed-source-citation-404", rest_open(&server.rest, "key-beta", &s1_cit).is_err(), "s1 citation dead");

    // 4b: withdraw the only remaining support permitted to beta
    {
        let store = Store::open(&db_path).expect("open");
        revoke(&store, "cacheev-s2", "beta").expect("revoke");
    }
    std::thread::sleep(Duration::from_millis(150));
    let post_b = rest_search(&server.rest, "key-beta", "cache fleet instances capacity planning", None, None);
    cx.check("s4.beta-insufficient-after-revoke", status_of(&post_b) == "insufficient_context", format!("status {}", status_of(&post_b)));
    cx.check("s4.beta-claim-citation-dead", rest_open(&server.rest, "key-beta", &claim_cit).is_err(), "claim citation 404");
    cx.check("s4.beta-s2-citation-dead", rest_open(&server.rest, "key-beta", &s2_cit).is_err(), "s2 citation 404");
    let post_b_asof = rest_search(&server.rest, "key-beta", "cache fleet instances capacity planning", Some("2026-02-15"), None);
    cx.check("s4.beta-asof-also-withheld", status_of(&post_b_asof) == "insufficient_context", format!("status {}", status_of(&post_b_asof)));
    let still_a = rest_search(&server.rest, "key-alpha", "cache fleet instances capacity planning", None, None);
    cx.check("s4.alpha-still-resolves", status_of(&still_a) == "ok", format!("status {}", status_of(&still_a)));
    cx.check("s4.zero-leak-beta", leak_free(&body_text(&post_b)) && leak_free(&body_text(&post_b_asof)) && !body_text(&post_b).contains("c7i"), "no c7i/secret leakage");

    // restart persistence
    drop(mcp_a);
    drop(mcp_b);
    let _ = server.child.kill();
    let _ = server.child.wait();
    std::thread::sleep(Duration::from_millis(300));
    server = spawn_server(&db_path, &policy_path);
    mcp_a = Mcp::spawn(&server.rest, "key-alpha");
    mcp_b = Mcp::spawn(&server.rest, "key-beta");

    let rb_post = rest_search(&server.rest, "key-beta", "cache fleet instances capacity planning", None, None);
    cx.check("s4.restart-beta-still-withheld", status_of(&rb_post) == "insufficient_context", format!("status {}", status_of(&rb_post)));
    cx.check("s4.restart-citation-still-dead", rest_open(&server.rest, "key-beta", &claim_cit).is_err(), "claim citation 404 after restart");
    let ra_post = rest_search(&server.rest, "key-alpha", "cache fleet instances capacity planning", None, None);
    cx.check("s4.restart-alpha-still-resolves", status_of(&ra_post) == "ok", format!("status {}", status_of(&ra_post)));
    let mcpb = mcp_b.search("cache fleet instances capacity planning", None);
    cx.check("s4.mcp-beta-withheld", status_of(&mcpb) == "insufficient_context" && leak_free(&body_text(&mcpb)), format!("status {}", status_of(&mcpb)));
    let mcpa = mcp_a.search("cache fleet instances capacity planning", None);
    cx.check("s4.mcp-alpha-resolves", status_of(&mcpa) == "ok", format!("status {}", status_of(&mcpa)));
    let mcp_open = mcp_b.open(&claim_cit);
    cx.check("s4.mcp-beta-citation-dead", mcp_open.get("status").and_then(Value::as_str) == Some("error"), format!("{:?}", mcp_open.get("status")));

    // ================= STEP 5: owner correction + replay + audit + deletion =
    println!("STEP 5 — correction survives replay, audit, deletion");
    {
        let mut store = Store::open(&db_path).expect("open");
        import(&mut store, "deployrules-r3");
        store.rebuild_index().expect("rebuild");
    }
    std::thread::sleep(Duration::from_millis(150));
    let r3read = rest_search(&server.rest, "key-alpha", "does staging require manual approval", None, None);
    cx.check("s5.owner-correction-live", revision_set(&r3read).contains(&"deployrules-r3".to_string()), format!("revs {:?}", revision_set(&r3read)));

    // replay the old captures — again genuine: delete first so the r1/r2
    // replays truly re-arrive after the owner correction r3 is live.
    {
        let mut store = Store::open(&db_path).expect("open");
        delete_revision(&store, "deployrules-r1").expect("delete r1");
        delete_revision(&store, "deployrules-r2").expect("delete r2");
        import(&mut store, "deployrules-r1");
        import(&mut store, "deployrules-r2");
    }
    std::thread::sleep(Duration::from_millis(150));
    let r3read2 = rest_search(&server.rest, "key-alpha", "does staging require manual approval", None, None);
    cx.check("s5.replay-cannot-undo", revision_set(&r3read2).contains(&"deployrules-r3".to_string()), format!("revs {:?}", revision_set(&r3read2)));

    // audit trail present
    {
        let store = Store::open(&db_path).expect("open");
        let count: i64 = store.conn.query_row("SELECT COUNT(*) FROM audit_events", [], |r| r.get(0)).unwrap_or(0);
        let revoked: i64 = store.conn.query_row("SELECT COUNT(*) FROM audit_events WHERE event = 'revoke'", [], |r| r.get(0)).unwrap_or(0);
        cx.check("s5.audit-trail", count >= 12 && revoked >= 3, format!("{count} events, {revoked} revocations"));
    }

    // deletion under retention: remove the private note entirely
    let priv_cit = first_passage_token(&db_path, "alphapriv-r1");
    {
        let store = Store::open(&db_path).expect("open");
        let removed = delete_revision(&store, "alphapriv-r1").expect("delete");
        cx.check("s5.deletion-executed", removed >= 1, format!("{removed} passages removed"));
    }
    std::thread::sleep(Duration::from_millis(150));
    let gone = rest_search(&server.rest, "key-alpha", "transkript generated client private repo", None, None);
    cx.check("s5.deleted-everywhere", status_of(&gone) == "insufficient_context" && rest_open(&server.rest, "key-alpha", &priv_cit).is_err(), format!("status {}", status_of(&gone)));

    // ---------------- summary ---------------------------------------------
    let total = cx.checks.len();
    let passed = cx.checks.iter().filter(|(_, ok, _)| *ok).count();
    println!("lifecycle replay: {passed}/{total} checks passed");
    for (name, ok, detail) in &cx.checks {
        if !ok {
            println!("  FAILED: {name} — {detail}");
        }
    }
    if passed == total { 0 } else { 1 }
}
