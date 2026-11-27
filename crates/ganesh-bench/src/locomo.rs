//! LoCoMo benchmark track (M2 phase 2, roadmap section 7): offline, zero-LLM
//! retrieval/abstention evaluation of Ganesh against the published LoCoMo
//! conversational-memory dataset. Every session is ingested with first-class
//! M2 metadata (occurred_at=session date, source_type=conversation, speakers
//! preserved per turn); questions are evaluated against a live ganesh-server
//! over REST with configurable top_k. An optional --reader-cmd hook allows a
//! future apples-to-apples reader-LLM comparison without touching the harness.

use ganesh_core::store::Store;
use ganesh_core::types::ManifestEntry;
use serde_json::{json, Value};
use std::io::{BufRead, BufReader};
use std::process::{Child, Command, Stdio};
use std::time::Instant;

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
    let addr = line.trim().strip_prefix("listening ").expect("listening line").to_string();
    Server { child, rest: format!("http://{addr}") }
}

fn rss_kb(pid: u32) -> u64 {
    std::fs::read_to_string(format!("/proc/{pid}/status"))
        .or_else(|_| -> Result<String, std::io::Error> {
            // macOS fallback: ps
            let out = Command::new("ps").args(["-o", "rss=", "-p", &pid.to_string()]).output()?;
            Ok(String::from_utf8_lossy(&out.stdout).to_string())
        })
        .ok()
        .and_then(|s| {
            s.lines()
                .find(|l| l.starts_with("VmRSS"))
                .and_then(|l| l.split_whitespace().nth(1).and_then(|v| v.parse().ok()))
                .or_else(|| s.trim().parse().ok())
        })
        .unwrap_or(0)
}

/// "1:56 pm on 8 May, 2023" -> "2023-05-08"
fn normalize_date(s: &str) -> String {
    let months = [
        ("january", "01"), ("february", "02"), ("march", "03"), ("april", "04"),
        ("may", "05"), ("june", "06"), ("july", "07"), ("august", "08"),
        ("september", "09"), ("october", "10"), ("november", "11"), ("december", "12"),
    ];
    let low = s.to_lowercase();
    let toks: Vec<&str> = low.split(|c: char| !c.is_alphanumeric()).filter(|w| !w.is_empty()).collect();
    let mut day = "";
    let mut mon = "";
    let mut yr = "";
    for (i, t) in toks.iter().enumerate() {
        for (name, num) in months {
            if *t == name {
                mon = num;
                if i > 0 && toks[i - 1].chars().all(|c| c.is_ascii_digit()) {
                    day = toks[i - 1];
                }
                if i + 1 < toks.len() && toks[i + 1].len() == 4 {
                    yr = toks[i + 1];
                }
            }
        }
    }
    if !day.is_empty() && !mon.is_empty() && !yr.is_empty() {
        format!("{yr}-{mon}-{:0>2}", day)
    } else {
        s.to_string()
    }
}

const CATEGORY_NAMES: &[(&str, &str)] = &[
    ("1", "single-hop"),
    ("2", "temporal"),
    ("3", "multi-hop"),
    ("4", "open-domain"),
    ("5", "adversarial"),
];

pub fn run_locomo(data_path: &str, top_k: usize, max_convs: usize, out_path: &str, reader_cmd: &str) -> i32 {
    let raw = std::fs::read_to_string(data_path).expect("read locomo data");
    let convs: Vec<Value> = serde_json::from_str(&raw).expect("parse locomo");
    let tmp = tempfile::tempdir().expect("tmpdir");
    let snap_dir = tmp.path().join("snapshots");
    std::fs::create_dir_all(&snap_dir).unwrap();

    let mut report = json!({
        "mode": "locomo-mode1-offline-retrieval",
        "top_k": top_k,
        "reader_hook": if reader_cmd.is_empty() { Value::Null } else { json!(reader_cmd) },
        "conversations": [],
        "provider_calls": 0,
    });
    let mut cat_stats: serde_json::Map<String, Value> = serde_json::Map::new();
    let mut global_lat: Vec<f64> = Vec::new();
    let mut ingest_total_ms = 0.0f64;
    let mut store_bytes_total = 0u64;
    let mut peak_rss_kb = 0u64;
    let mut cites_checked = 0usize;
    let mut cites_ok = 0usize;
    let mut qa_total = 0usize;

    for (ci, conv) in convs.iter().enumerate() {
        if ci >= max_convs {
            break;
        }
        let c = &conv["conversation"];
        let sample_id = conv["sample_id"].as_str().unwrap_or("conv");
        let spk_a = c["speaker_a"].as_str().unwrap_or("");
        let spk_b = c["speaker_b"].as_str().unwrap_or("");

        // Build snapshot files: one per session, each turn a timestamped,
        // dia-id-tagged speaker line so evidence dia_ids are verbatim-visible.
        let mut entries: Vec<ManifestEntry> = Vec::new();
        let sess_keys: Vec<String> = c
            .as_object()
            .unwrap()
            .keys()
            .filter(|k| k.starts_with("session_") && !k.ends_with("_date_time") && c[k].is_array())
            .cloned()
            .collect();
        let mut sess_keys = sess_keys;
        sess_keys.sort_by_key(|k| k.trim_start_matches("session_").parse::<usize>().unwrap_or(0));
        for sk in &sess_keys {
            let date_raw = c[format!("{sk}_date_time")].as_str().unwrap_or("");
            let date = normalize_date(date_raw);
            let turns = c[sk].as_array().unwrap();
            let mut text = format!("Conversation {sample_id} {sk} between {spk_a} and {spk_b} on {date}\n");
            for t in turns {
                let dia = t["dia_id"].as_str().unwrap_or("");
                let speaker = t["speaker"].as_str().unwrap_or("");
                let body = t["text"].as_str().unwrap_or("");
                text.push_str(&format!("[{dia}] [{date}] {speaker}: {body}\n"));
            }
            let file = format!("snapshots/{sample_id}-{sk}.txt");
            let bytes = text.as_bytes().to_vec();
            std::fs::write(snap_dir.join(format!("{sample_id}-{sk}.txt")), &bytes).unwrap();
            entries.push(ManifestEntry {
                article_id: format!("{sample_id}-{sk}"),
                title: format!("{sample_id} {sk}"),
                pageid: 0,
                revision: format!("{sample_id}-{sk}-a1"),
                revision_url: String::new(),
                publisher: "locomo".to_string(),
                url: String::new(),
                license: "CC-BY-NC".to_string(),
                license_url: String::new(),
                permission: "benchmark fixture".to_string(),
                retrieved: String::new(),
                bytes: bytes.len() as u64,
                sha256: ganesh_core::citation::sha256_hex(&bytes),
                file: file.clone(),
                family: "locomo".to_string(),
                effective_from: None,
                effective_until: None,
                occurred_at: Some(date),
                author: format!("{spk_a} & {spk_b}"),
                source_type: "conversation".to_string(),
                project: sample_id.to_string(),
                scope: String::new(),
                authority: 0,
                supersedes: None,
                supersede_mode: String::new(),
                claim_key: String::new(),
                audience: String::new(),
                supports: vec![],
            });
        }

        // Ingest + index (timed).
        let t0 = Instant::now();
        let db_path = tmp.path().join(format!("{sample_id}.db"));
        let policy_path = tmp.path().join(format!("{sample_id}-policy.json"));
        std::fs::write(
            &policy_path,
            json!({"keys": {"owner-key": {"installation": "bench", "projects": [sample_id], "owner": true}}}).to_string(),
        )
        .unwrap();
        {
            let mut store = Store::open(&db_path).expect("open");
            for entry in &entries {
                let bytes = std::fs::read(snap_dir.join(entry.file.trim_start_matches("snapshots/"))).unwrap();
                store.import_snapshot(entry, &bytes).expect("import");
            }
            store.rebuild_index().expect("rebuild");
        }
        ingest_total_ms += t0.elapsed().as_secs_f64() * 1000.0;
        store_bytes_total += std::fs::metadata(&db_path).map(|m| m.len()).unwrap_or(0);

        let mut server = spawn_server(&db_path, &policy_path);
        let mut conv_lat: Vec<f64> = Vec::new();
        let mut conv_hits: std::collections::HashMap<String, usize> = std::collections::HashMap::new();
        let mut conv_totals: std::collections::HashMap<String, usize> = std::collections::HashMap::new();
        let mut abstain_correct = 0usize;
        let mut abstain_total = 0usize;
        let mut uncertain_cnt = 0usize;

        for qa in conv["qa"].as_array().unwrap() {
            let cat = qa["category"].to_string();
            let question = qa["question"].as_str().unwrap_or("");
            let evidence: Vec<String> = qa["evidence"]
                .as_array()
                .map(|a| a.iter().filter_map(|e| e.as_str().map(String::from)).collect())
                .unwrap_or_default();

            let t = Instant::now();
            let resp = ureq::post(&format!("{}/v1/search", server.rest))
                .send_json(json!({"query": question, "key": "owner-key", "top_k": top_k.max(10)}))
                .ok()
                .and_then(|r| r.into_json::<Value>().ok())
                .unwrap_or(json!({"status": "error"}));
            let lat = t.elapsed().as_secs_f64() * 1000.0;
            conv_lat.push(lat);
            qa_total += 1;

            let status = resp["status"].as_str().unwrap_or("?");
            let confidence = resp["confidence"].as_str().unwrap_or("");
            if confidence == "uncertain" {
                uncertain_cnt += 1;
            }
            let texts: Vec<String> = resp["passages"]
                .as_array()
                .map(|a| {
                    a.iter()
                        .map(|p| p["text"].as_str().unwrap_or("").to_string())
                        .collect()
                })
                .unwrap_or_default();

            // Citation integrity spot check on the first returned passage.
            if let Some(tok) = resp["passages"].as_array().and_then(|a| a.first()).and_then(|p| p["citation"].as_str()) {
                cites_checked += 1;
                if ureq::get(&format!("{}/v1/citations/{tok}?key=owner-key", server.rest)).call().is_ok() {
                    cites_ok += 1;
                }
            }

            if cat == "5" {
                abstain_total += 1;
                if status == "insufficient_context" || confidence == "uncertain" {
                    abstain_correct += 1;
                }
            }

            // Recall@k: every evidence dia_id must appear within the top-k passages.
            let r3 = evidence.iter().all(|e| texts.iter().take(3).any(|t| t.contains(e)));
            let r5 = evidence.iter().all(|e| texts.iter().take(5).any(|t| t.contains(e)));
            let r10 = evidence.iter().all(|e| texts.iter().take(10).any(|t| t.contains(e)));
            *conv_totals.entry(cat.clone()).or_default() += 1;
            if r10 {
                *conv_hits.entry(format!("{cat}|r10")).or_default() += 1;
            }
            if r5 {
                *conv_hits.entry(format!("{cat}|r5")).or_default() += 1;
            }
            if r3 {
                *conv_hits.entry(format!("{cat}|r3")).or_default() += 1;
            }
        }

        peak_rss_kb = peak_rss_kb.max(rss_kb(server.child.id()));
        server.child.kill().ok();
        server.child.wait().ok();

        global_lat.extend(conv_lat.iter().cloned());
        for (cat, tot) in &conv_totals {
            let e = cat_stats.entry(cat.clone()).or_insert(json!({"total":0,"r3":0,"r5":0,"r10":0}));
            e["total"] = json!(e["total"].as_u64().unwrap_or(0) + *tot as u64);
            for k in ["r3", "r5", "r10"] {
                e[k] = json!(e[k].as_u64().unwrap_or(0) + conv_hits.get(&format!("{cat}|{k}")).copied().unwrap_or(0) as u64);
            }
        }
        let entry = cat_stats.entry("__abstention__".to_string()).or_insert(json!({"total":0,"correct":0,"uncertain":0}));
        entry["total"] = json!(entry["total"].as_u64().unwrap_or(0) + abstain_total as u64);
        entry["correct"] = json!(entry["correct"].as_u64().unwrap_or(0) + abstain_correct as u64);
        entry["uncertain"] = json!(entry["uncertain"].as_u64().unwrap_or(0) + uncertain_cnt as u64);

        report["conversations"].as_array_mut().unwrap().push(json!({
            "sample_id": sample_id,
            "sessions": entries.len(),
            "qa": conv_totals.values().sum::<usize>(),
            "mean_latency_ms": conv_lat.iter().sum::<f64>() / conv_lat.len().max(1) as f64,
        }));
        println!("conv {sample_id}: {} sessions, {} qa done", entries.len(), conv_totals.values().sum::<usize>());
    }

    global_lat.sort_by(|a, b| a.partial_cmp(b).unwrap());
    let mean = global_lat.iter().sum::<f64>() / global_lat.len().max(1) as f64;
    let p95 = global_lat.get((global_lat.len() as f64 * 0.95) as usize).copied().unwrap_or(0.0);
    report["aggregate"] = json!({
        "categories": cat_stats,
        "qa_total": qa_total,
        "latency_mean_ms": mean,
        "latency_p95_ms": p95,
        "ingest_total_ms": ingest_total_ms,
        "store_bytes_total": store_bytes_total,
        "peak_rss_mb": peak_rss_kb as f64 / 1024.0,
        "citations_checked": cites_checked,
        "citations_ok": cites_ok,
    });
    std::fs::write(out_path, serde_json::to_string_pretty(&report).unwrap()).unwrap();
    println!("locomo report written to {out_path}");
    0
}
