//! Offline benchmark harness. One invocation: fresh store, verified import,
//! index rebuild, real REST service, real MCP stdio bridge, per-question
//! execution through both adapters, citation opening through both adapters,
//! scoring against the frozen key, markdown report.

use ganesh_core::store::{ImportOutcome, Store};
use ganesh_core::types::Manifest;
use serde::Deserialize;
use serde_json::{json, Value};
use std::io::{BufRead, BufReader, Write};
use std::process::{Child, ChildStdin, ChildStdout, Command, Stdio};
use std::sync::mpsc;
use std::time::{Duration, Instant};

#[derive(Debug, Deserialize)]
struct KeyFile {
    version: String,
    questions: Vec<KeyQuestion>,
}

#[derive(Debug, Deserialize)]
struct KeyQuestion {
    id: String,
    question: String,
    tags: Vec<String>,
    expected_status: String,
    required_facts: Vec<KeyFact>,
    #[serde(default)]
    missing_detail: Option<String>,
}

#[derive(Debug, Deserialize)]
struct KeyFact {
    fact: String,
    acceptable: Vec<String>,
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let mut questions_path = None;
    let mut report_path = None;
    let mut corpus_dir = "fixtures/corpus".to_string();
    let mut details_path = None;
    let mut i = 1;
    if args.len() >= 2 && args[1] == "probe" {
        // ganesh-bench probe --questions dev.v1.json [--corpus fixtures/corpus]
        // Development-time coverage analysis: prints per-question coverage diagnostics.
        let mut qpath = None;
        let mut cdir = "fixtures/corpus".to_string();
        let mut j = 2;
        while j < args.len() {
            match args[j].as_str() {
                "--questions" => { qpath = args.get(j + 1).cloned(); j += 2; }
                "--corpus" => { cdir = args.get(j + 1).cloned().unwrap(); j += 2; }
                other => { eprintln!("unknown argument: {other}"); std::process::exit(2); }
            }
        }
        probe(&qpath.expect("--questions required"), &cdir);
        return;
    }
    if args.len() < 2 || args[1] != "run" {
        eprintln!("usage: ganesh-bench run --questions <key.json> --report <out.md> [--corpus fixtures/corpus] [--details out.json]");
        std::process::exit(2);
    }
    i = 2;
    while i < args.len() {
        match args[i].as_str() {
            "--questions" => { questions_path = args.get(i + 1).cloned(); i += 2; }
            "--report" => { report_path = args.get(i + 1).cloned(); i += 2; }
            "--corpus" => { corpus_dir = args.get(i + 1).cloned().unwrap(); i += 2; }
            "--details" => { details_path = args.get(i + 1).cloned(); i += 2; }
            other => { eprintln!("unknown argument: {other}"); std::process::exit(2); }
        }
    }
    let questions_path = questions_path.expect("--questions required");
    let report_path = report_path.expect("--report required");

    let code = run(&questions_path, &report_path, &corpus_dir, details_path.as_deref());
    std::process::exit(code);
}

struct McpClient {
    child: Child,
    stdin: ChildStdin,
    rx: mpsc::Receiver<String>,
    next_id: u64,
}

impl McpClient {
    fn spawn(rest_url: &str) -> McpClient {
        let bin = sibling_binary("ganesh-mcp");
        let mut child = Command::new(bin)
            .env("GANESH_REST_URL", rest_url)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
            .expect("spawn ganesh-mcp");
        let stdin = child.stdin.take().unwrap();
        let stdout: ChildStdout = child.stdout.take().unwrap();
        let (tx, rx) = mpsc::channel();
        std::thread::spawn(move || {
            let reader = BufReader::new(stdout);
            for line in reader.lines() {
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
        McpClient { child, stdin, rx, next_id: 0 }
    }

    fn call(&mut self, method: &str, params: Value) -> Result<Value, String> {
        self.next_id += 1;
        let id = self.next_id;
        let msg = json!({ "jsonrpc": "2.0", "id": id, "method": method, "params": params });
        writeln!(self.stdin, "{}", serde_json::to_string(&msg).unwrap()).map_err(|e| e.to_string())?;
        self.stdin.flush().map_err(|e| e.to_string())?;
        loop {
            let line = self
                .rx
                .recv_timeout(Duration::from_secs(30))
                .map_err(|_| "mcp timeout".to_string())?;
            let v: Value = serde_json::from_str(&line).map_err(|e| format!("mcp decode: {e}"))?;
            if v.get("id").and_then(Value::as_u64) == Some(id) {
                if let Some(err) = v.get("error") {
                    return Err(format!("mcp error: {err}"));
                }
                return Ok(v.get("result").cloned().unwrap_or(Value::Null));
            }
        }
    }

    fn notify(&mut self, method: &str) {
        let msg = json!({ "jsonrpc": "2.0", "method": method });
        let _ = writeln!(self.stdin, "{}", serde_json::to_string(&msg).unwrap());
        let _ = self.stdin.flush();
    }
}

impl Drop for McpClient {
    fn drop(&mut self) {
        let _ = self.child.kill();
    }
}

fn sibling_binary(name: &str) -> std::path::PathBuf {
    let exe = std::env::current_exe().unwrap();
    let dir = exe.parent().unwrap();
    let candidate = dir.join(name);
    if candidate.exists() {
        return candidate;
    }
    // Fall back to cargo target layout differences (release vs debug).
    let alt = dir.parent().map(|p| p.join(name));
    if let Some(a) = alt {
        if a.exists() {
            return a;
        }
    }
    candidate
}

fn run(questions_path: &str, report_path: &str, corpus_dir: &str, details_path: Option<&str>) -> i32 {
    let t0 = Instant::now();
    let mut log_lines: Vec<String> = Vec::new();

    // 1. Fresh store.
    let tmp = tempfile::tempdir().expect("tempdir");
    let db_path = tmp.path().join("ganesh.db");

    // 2. Verified import.
    let manifest_text = std::fs::read_to_string(format!("{corpus_dir}/manifest.json")).expect("manifest");
    let manifest: Manifest = serde_json::from_str(&manifest_text).expect("parse manifest");
    let mut store = Store::open(&db_path).expect("open store");
    let import_start = Instant::now();
    let mut total_passages = 0usize;
    let mut corpus_bytes = 0u64;
    for entry in &manifest.articles {
        let path = format!("{corpus_dir}/{}", entry.file);
        let bytes = std::fs::read(&path).unwrap_or_else(|_| panic!("read {path}"));
        corpus_bytes += bytes.len() as u64;
        match store.import_snapshot(entry, &bytes).expect("import") {
            ImportOutcome::Imported { passages } => total_passages += passages,
            ImportOutcome::AlreadyPresent { .. } => panic!("fresh store must not report already-present"),
        }
    }
    let import_ms = import_start.elapsed().as_secs_f64() * 1000.0;

    // 3. Index rebuild.
    let rebuild_start = Instant::now();
    let indexed = store.rebuild_index().expect("rebuild");
    let rebuild_ms = rebuild_start.elapsed().as_secs_f64() * 1000.0;
    drop(store);
    log_lines.push(format!("imported {} articles, {total_passages} passages ({corpus_bytes} bytes) in {import_ms:.1} ms; indexed {indexed} in {rebuild_ms:.1} ms", manifest.articles.len()));

    // Idempotency probe: reimport every article against the same store and require AlreadyPresent.
    {
        let mut store2 = Store::open(&db_path).expect("reopen");
        for entry in &manifest.articles {
            let bytes = std::fs::read(format!("{corpus_dir}/{}", entry.file)).unwrap();
            match store2.import_snapshot(entry, &bytes).expect("reimport") {
                ImportOutcome::AlreadyPresent { .. } => {}
                ImportOutcome::Imported { .. } => panic!("reimport must be idempotent (AlreadyPresent)"),
            }
        }
    }

    // 4. Start REST service.
    let server_bin = sibling_binary("ganesh-server");
    let mut server = Command::new(server_bin)
        .arg("--db").arg(&db_path)
        .arg("--bind").arg("127.0.0.1:0")
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
        .expect("spawn server");
    let mut stdout = BufReader::new(server.stdout.take().unwrap());
    let mut line = String::new();
    stdout.read_line(&mut line).expect("server readiness line");
    let rest_addr = line.trim().strip_prefix("listening ").expect("listening line").to_string();
    let rest = format!("http://{rest_addr}");

    // 5. MCP handshake.
    let mut mcp = McpClient::spawn(&rest);
    let init = mcp.call("initialize", json!({
        "protocolVersion": "2024-11-05",
        "capabilities": {},
        "clientInfo": { "name": "ganesh-bench", "version": "0.1.0" }
    })).expect("mcp initialize");
    assert!(init.get("serverInfo").is_some(), "initialize result");
    mcp.notify("notifications/initialized");
    let tools = mcp.call("tools/list", json!({})).expect("tools/list");
    let tool_names: Vec<&str> = tools["tools"].as_array().unwrap().iter().filter_map(|t| t["name"].as_str()).collect();
    assert!(tool_names.contains(&"search_context") && tool_names.contains(&"open_citation"), "required tools present: {tool_names:?}");

    // 6. Execute questions.
    let key_text = std::fs::read_to_string(questions_path).expect("questions");
    let key: KeyFile = serde_json::from_str(&key_text).expect("parse questions");

    #[derive(Default)]
    struct QOutcome {
        id: String,
        question: String,
        tags: Vec<String>,
        expected: String,
        rest_status: String,
        mcp_status: String,
        rest_ids: Vec<String>,
        mcp_ids: Vec<String>,
        hit: Option<bool>,
        insufficient_correct: Option<bool>,
        adapter_agree: bool,
        citations_checked: usize,
        citations_ok: usize,
        missing_facts: Vec<String>,
        latency_rest_ms: f64,
        latency_mcp_ms: f64,
        response_bytes_rest: usize,
        response_bytes_mcp: usize,
    }

    let mut outcomes: Vec<QOutcome> = Vec::new();
    let mut incomplete = false;

    for q in &key.questions {
        let mut oc = QOutcome {
            id: q.id.clone(),
            question: q.question.clone(),
            tags: q.tags.clone(),
            expected: q.expected_status.clone(),
            ..Default::default()
        };

        // REST search.
        let t = Instant::now();
        let rest_resp = ureq::post(&format!("{rest}/v1/search")).send_json(json!({ "query": q.question }));
        oc.latency_rest_ms = t.elapsed().as_secs_f64() * 1000.0;
        let rest_json: Value = match rest_resp {
            Ok(r) => {
                let s = r.into_string().unwrap_or_default();
                oc.response_bytes_rest = s.len();
                serde_json::from_str(&s).unwrap_or(json!({"status":"decode_error"}))
            }
            Err(e) => { incomplete = true; json!({"status": format!("rest_error:{e}")}) }
        };
        oc.rest_status = rest_json["status"].as_str().unwrap_or("?").to_string();
        oc.rest_ids = rest_json["passages"].as_array().map(|a| {
            a.iter().filter_map(|p| p["passage_id"].as_str().map(String::from)).collect()
        }).unwrap_or_default();

        // MCP search.
        let t = Instant::now();
        let mcp_result = mcp.call("tools/call", json!({
            "name": "search_context",
            "arguments": { "query": q.question }
        }));
        oc.latency_mcp_ms = t.elapsed().as_secs_f64() * 1000.0;
        let mcp_json: Value = match mcp_result {
            Ok(res) => {
                let text = res["content"][0]["text"].as_str().unwrap_or("");
                oc.response_bytes_mcp = text.len();
                serde_json::from_str(text).unwrap_or(json!({"status":"decode_error"}))
            }
            Err(e) => { incomplete = true; json!({"status": format!("mcp_error:{e}")}) }
        };
        oc.mcp_status = mcp_json["status"].as_str().unwrap_or("?").to_string();
        oc.mcp_ids = mcp_json["passages"].as_array().map(|a| {
            a.iter().filter_map(|p| p["passage_id"].as_str().map(String::from)).collect()
        }).unwrap_or_default();

        oc.adapter_agree = oc.rest_status == oc.mcp_status && oc.rest_ids == oc.mcp_ids;

        // Citation opening through both adapters.
        let check_openings = |adapter_json: &Value, via_mcp: bool, mcp: &mut McpClient, oc: &mut QOutcome| {
            if let Some(passages) = adapter_json["passages"].as_array() {
                for p in passages {
                    let token = p["citation"].as_str().unwrap_or("");
                    let want_pid = p["passage_id"].as_str().unwrap_or("");
                    let want_rev = p["revision"].as_str().unwrap_or("");
                    if token.is_empty() { continue; }
                    oc.citations_checked += 1;
                    let opened: Option<Value> = if via_mcp {
                        mcp.call("tools/call", json!({
                            "name": "open_citation", "arguments": { "citation": token }
                        })).ok().and_then(|res| {
                            res["content"][0]["text"].as_str().and_then(|t| serde_json::from_str(t).ok())
                        })
                    } else {
                        ureq::get(&format!("{rest}/v1/citations/{token}")).call().ok()
                            .and_then(|r| r.into_json::<Value>().ok())
                    };
                    if let Some(o) = opened {
                        if o["passage_id"].as_str() == Some(want_pid) && o["revision"].as_str() == Some(want_rev) {
                            oc.citations_ok += 1;
                        }
                    }
                }
            }
        };
        check_openings(&rest_json, false, &mut mcp, &mut oc);
        check_openings(&mcp_json, true, &mut mcp, &mut oc);

        // Scoring.
        if q.expected_status == "ok" {
            let mut missing = Vec::new();
            if oc.rest_status != "ok" {
                oc.hit = Some(false);
                missing.push("(abstained)".to_string());
            } else {
                for f in &q.required_facts {
                    if !f.acceptable.iter().any(|a| oc.rest_ids.iter().any(|id| id == a)) {
                        missing.push(f.fact.clone());
                    }
                }
                oc.hit = Some(missing.is_empty());
            }
            oc.missing_facts = missing;
        } else {
            oc.insufficient_correct = Some(oc.rest_status == "insufficient_context");
        }

        outcomes.push(oc);
    }

    // 7. Aggregate.
    let answerable: Vec<&QOutcome> = outcomes.iter().filter(|o| o.expected == "ok").collect();
    let insufficient: Vec<&QOutcome> = outcomes.iter().filter(|o| o.expected != "ok").collect();
    let hits = answerable.iter().filter(|o| o.hit == Some(true)).count();
    let ins_correct = insufficient.iter().filter(|o| o.insufficient_correct == Some(true)).count();
    let agree = outcomes.iter().filter(|o| o.adapter_agree).count();
    let cit_checked: usize = outcomes.iter().map(|o| o.citations_checked).sum();
    let cit_ok: usize = outcomes.iter().map(|o| o.citations_ok).sum();
    let total_rest_ms: f64 = outcomes.iter().map(|o| o.latency_rest_ms).sum();
    let total_mcp_ms: f64 = outcomes.iter().map(|o| o.latency_mcp_ms).sum();
    let resp_bytes: usize = outcomes.iter().map(|o| o.response_bytes_rest + o.response_bytes_mcp).sum();
    let db_bytes = std::fs::metadata(&db_path).map(|m| m.len()).unwrap_or(0)
        + std::fs::metadata(db_path.with_extension("db-wal")).map(|m| m.len()).unwrap_or(0);
    let total_ms = t0.elapsed().as_secs_f64() * 1000.0;

    let mut per_tag: std::collections::BTreeMap<String, (usize, usize)> = std::collections::BTreeMap::new();
    for o in &answerable {
        for tag in &o.tags {
            let e = per_tag.entry(tag.clone()).or_default();
            e.1 += 1;
            if o.hit == Some(true) { e.0 += 1; }
        }
    }

    // 8. Report.
    let mut r = String::new();
    r.push_str(&format!("# Ganesh M1 benchmark report — {}\n\n", key.version));
    r.push_str(&format!("- questions file: `{questions_path}`\n- corpus: `{corpus_dir}` ({} articles, {total_passages} passages, {corpus_bytes} snapshot bytes)\n- incomplete run: {incomplete}\n\n", manifest.articles.len()));
    r.push_str("## Summary\n\n");
    r.push_str(&format!("| Check | Result |\n| --- | --- |\n"));
    r.push_str(&format!("| Evidence coverage | {hits}/{} answerable hits |\n", answerable.len()));
    r.push_str(&format!("| Insufficient evidence | {ins_correct}/{} correct abstentions |\n", insufficient.len()));
    r.push_str(&format!("| Adapter agreement | {agree}/{} questions identical status + ordered IDs |\n", outcomes.len()));
    r.push_str(&format!("| Citation integrity | {cit_ok}/{cit_checked} citation openings resolved to returned passage+revision |\n"));
    r.push_str(&format!("| Provider calls | 0 |\n\n"));

    r.push_str("## Coverage by question tag\n\n| Tag | Hits | Total |\n| --- | --- | --- |\n");
    for (tag, (h, t)) in &per_tag {
        r.push_str(&format!("| {tag} | {h} | {t} |\n"));
    }
    r.push('\n');

    r.push_str("## Per-question outcomes\n\n| ID | Expected | REST status | Hit/correct | Adapter agree | Missing facts |\n| --- | --- | --- | --- | --- | --- |\n");
    for o in &outcomes {
        let verdict = match (o.hit, o.insufficient_correct) {
            (Some(h), _) => if h { "hit" } else { "MISS" }.to_string(),
            (_, Some(c)) => if c { "correct" } else { "MISS" }.to_string(),
            _ => "?".to_string(),
        };
        let missing = if o.missing_facts.is_empty() { "".into() } else { o.missing_facts.join("; ") };
        r.push_str(&format!("| {} | {} | {} | {} | {} | {} |\n",
            o.id, o.expected, o.rest_status, verdict, o.adapter_agree,
            missing.replace('|', "\\|").chars().take(120).collect::<String>()));
    }
    r.push('\n');

    r.push_str("## Misses\n\n");
    let mut any_miss = false;
    for o in &outcomes {
        let missed = o.hit == Some(false) || o.insufficient_correct == Some(false);
        if missed {
            any_miss = true;
            r.push_str(&format!("### {} — {}\n\n", o.id, o.question));
            r.push_str(&format!("- expected status: `{}`; REST status: `{}`\n", o.expected, o.rest_status));
            r.push_str(&format!("- returned passages: {}\n", if o.rest_ids.is_empty() { "(none)".into() } else { o.rest_ids.join(", ") }));
            if !o.missing_facts.is_empty() {
                r.push_str(&format!("- missing facts: {}\n", o.missing_facts.join("; ")));
            }
            if let Some(md) = key.questions.iter().find(|q| q.id == o.id).and_then(|q| q.missing_detail.as_ref()) {
                r.push_str(&format!("- missing detail the corpus lacks: {md}\n"));
            }
            r.push('\n');
        }
    }
    if !any_miss {
        r.push_str("None.\n\n");
    }

    r.push_str("## Measured costs\n\n");
    r.push_str(&format!("| Measure | Value |\n| --- | --- |\n"));
    r.push_str(&format!("| Corpus size | {corpus_bytes} bytes across {} snapshots |\n", manifest.articles.len()));
    r.push_str(&format!("| Passages | {total_passages} |\n"));
    r.push_str(&format!("| Import time | {import_ms:.1} ms |\n"));
    r.push_str(&format!("| Index rebuild time | {rebuild_ms:.1} ms |\n"));
    r.push_str(&format!("| Store size | {db_bytes} bytes (incl. WAL) |\n"));
    r.push_str(&format!("| REST query time | mean {:.1} ms, max {:.1} ms |\n",
        total_rest_ms / outcomes.len() as f64,
        outcomes.iter().map(|o| o.latency_rest_ms).fold(0.0, f64::max)));
    r.push_str(&format!("| MCP query time | mean {:.1} ms, max {:.1} ms |\n",
        total_mcp_ms / outcomes.len() as f64,
        outcomes.iter().map(|o| o.latency_mcp_ms).fold(0.0, f64::max)));
    r.push_str(&format!("| Response bytes | {resp_bytes} total across both adapters |\n"));
    r.push_str(&format!("| Wall time | {total_ms:.0} ms |\n"));
    r.push_str(&format!("| Hardware | {} |\n", hardware()));
    r.push_str("| Provider calls | 0 (no network access in the retrieval path; corpus is local frozen bytes) |\n\n");

    r.push_str("## Notes\n\n");
    for l in &log_lines {
        r.push_str(&format!("- {l}\n"));
    }
    r.push_str("- Reimport idempotency verified: every article reimported against the live store reported already-present.\n");

    std::fs::create_dir_all(std::path::Path::new(report_path).parent().unwrap_or(std::path::Path::new("."))).ok();
    std::fs::write(report_path, &r).expect("write report");

    if let Some(dp) = details_path {
        let details: Vec<Value> = outcomes.iter().map(|o| json!({
            "id": o.id, "expected": o.expected, "rest_status": o.rest_status, "mcp_status": o.mcp_status,
            "rest_ids": o.rest_ids, "mcp_ids": o.mcp_ids, "hit": o.hit, "insufficient_correct": o.insufficient_correct,
            "adapter_agree": o.adapter_agree, "citations_checked": o.citations_checked, "citations_ok": o.citations_ok,
            "latency_rest_ms": o.latency_rest_ms, "latency_mcp_ms": o.latency_mcp_ms,
        })).collect();
        std::fs::write(dp, serde_json::to_string_pretty(&json!({
            "questions": questions_path, "outcomes": details,
            "summary": {
                "hits": hits, "answerable": answerable.len(),
                "insufficient_correct": ins_correct, "insufficient": insufficient.len(),
                "adapter_agree": agree, "total": outcomes.len(),
                "citations_checked": cit_checked, "citations_ok": cit_ok,
                "incomplete": incomplete,
            }
        })).unwrap()).ok();
    }

    let _ = server.kill();
    println!("report written to {report_path}");
    println!("coverage {hits}/{}, insufficient {ins_correct}/{}, adapter {agree}/{}, citations {cit_ok}/{cit_checked}, incomplete={incomplete}",
        answerable.len(), insufficient.len(), outcomes.len());
    if incomplete { 3 } else { 0 }
}

fn hardware() -> String {
    let arch = std::env::consts::ARCH;
    let os = std::env::consts::OS;
    let model = std::process::Command::new("sysctl")
        .args(["-n", "machdep.cpu.brand_string"])
        .output()
        .ok()
        .and_then(|o| String::from_utf8(o.stdout).ok())
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty())
        .or_else(|| {
            std::process::Command::new("uname").arg("-m").output().ok()
                .and_then(|o| String::from_utf8(o.stdout).ok())
                .map(|s| s.trim().to_string())
        })
        .unwrap_or_else(|| "unknown".into());
    format!("{os}/{arch}, {model}")
}

fn probe(questions_path: &str, corpus_dir: &str) {
    use ganesh_core::search::{search_debug, RetrievalConfig};
    let manifest_text = std::fs::read_to_string(format!("{corpus_dir}/manifest.json")).expect("manifest");
    let manifest: Manifest = serde_json::from_str(&manifest_text).expect("parse manifest");
    let tmp = tempfile::tempdir().expect("tempdir");
    let mut store = Store::open(&tmp.path().join("probe.db")).expect("open");
    for entry in &manifest.articles {
        let bytes = std::fs::read(format!("{corpus_dir}/{}", entry.file)).unwrap();
        let _ = store.import_snapshot(entry, &bytes).expect("import");
    }
    store.rebuild_index().expect("rebuild");
    let key_text = std::fs::read_to_string(questions_path).expect("questions");
    let key: KeyFile = serde_json::from_str(&key_text).expect("parse questions");
    let cfg = RetrievalConfig::default();
    println!("id    expected             union  top    terms  top-candidates");
    for q in &key.questions {
        let (_resp, dbg) = search_debug(&store, &q.question, "probe", &cfg);
        let cands: Vec<String> = dbg.candidates.iter().take(3).map(|(id, s)| format!("{id}({s:.3})")).collect();
        println!("{:<5} {:<20} u:{:.2} iu:{:.2} i2:{:.2}  {}",
            q.id, q.expected_status, dbg.union_coverage,
            dbg.idf_union_coverage, dbg.idf2_union_coverage, cands.join(" "));
        println!("      unmatched: {}", dbg.unmatched_terms.join(","));
    }
}
