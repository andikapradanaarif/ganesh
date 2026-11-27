use serde_json::Value;
fn main() {
    let mut a = std::env::args().skip(1);
    let corpus = a.next().unwrap();
    let keyp = a.next().unwrap();
    let key: Value = serde_json::from_str(&std::fs::read_to_string(&keyp).unwrap()).unwrap();
    let mani: Value = serde_json::from_str(&std::fs::read_to_string(format!("{corpus}/manifest.json")).unwrap()).unwrap();
    let tmp = tempfile::tempdir().unwrap();
    let mut store = ganesh_core::store::Store::open(&tmp.path().join("q.db")).unwrap();
    for e in mani["articles"].as_array().unwrap() {
        let path = format!("{corpus}/{}", e["file"].as_str().unwrap());
        let bytes = std::fs::read(&path).unwrap();
        let entry: ganesh_core::types::ManifestEntry = serde_json::from_value(e.clone()).unwrap();
        store.import_snapshot(&entry, &bytes).unwrap();
    }
    store.rebuild_index().unwrap();
    let cfg = ganesh_core::search::RetrievalConfig::default();
    for q in key["questions"].as_array().unwrap() {
        let id = q["id"].as_str().unwrap();
        let question = q["question"].as_str().unwrap();
        let expect = q["expected_status"].as_str().unwrap_or("ok");
        let (resp, _dbg) = ganesh_core::search::search_debug(&store, question, "qacal", &cfg);
        let mut margins = Vec::new();
        for p in &resp.passages {
            let text = format!("{} — {}\n{}", p.article_id, p.section, p.text);
            if let Some(v) = ganesh_core::qa_verify::verify_pair(question, &text) {
                margins.push(format!("{}:{:.2}[{}]", p.passage_id, v.margin, v.span.chars().take(40).collect::<String>()));
            }
        }
        let best = resp.passages.iter().filter_map(|p| {
            let text = format!("{} — {}\n{}", p.article_id, p.section, p.text);
            ganesh_core::qa_verify::verify_pair(question, &text).map(|v| v.margin)
        }).fold(f32::NEG_INFINITY, f32::max);
        println!("{} | expect={} status={} | bestmargin={:.2} | {}", id, expect, resp.status, best, margins.join(" "));
    }
}
