use serde_json::Value;
fn main() {
    let mut a = std::env::args().skip(1);
    let corpus = a.next().unwrap();
    let keyp = a.next().unwrap();
    let filter: Vec<String> = a.by_ref().collect();
    let key: Value = serde_json::from_str(&std::fs::read_to_string(&keyp).unwrap()).unwrap();
    let mani: Value = serde_json::from_str(&std::fs::read_to_string(format!("{corpus}/manifest.json")).unwrap()).unwrap();
    let tmp = tempfile::tempdir().unwrap();
    let mut store = ganesh_core::store::Store::open(&tmp.path().join("q.db")).unwrap();
    let mut texts: std::collections::HashMap<String, (String, String)> = std::collections::HashMap::new();
    for e in mani["articles"].as_array().unwrap() {
        let art = e["article_id"].as_str().unwrap().to_string();
        let raw = std::fs::read_to_string(format!("{corpus}/{}", e["file"].as_str().unwrap())).unwrap();
        let bytes = std::fs::read(format!("{corpus}/{}", e["file"].as_str().unwrap())).unwrap();
        let entry: ganesh_core::types::ManifestEntry = serde_json::from_value(e.clone()).unwrap();
        store.import_snapshot(&entry, &bytes).unwrap();
        for p in ganesh_core::segment::segment(&raw) {
            texts.insert(format!("{art}-a1#p{}", p.seq), (art.clone(), p.text));
        }
    }
    store.rebuild_index().unwrap();
    let cfg = ganesh_core::search::RetrievalConfig::default();
    for q in key["questions"].as_array().unwrap() {
        let id = q["id"].as_str().unwrap().to_string();
        if !filter.is_empty() && !filter.contains(&id) { continue; }
        let question = q["question"].as_str().unwrap();
        let expect = q["expected_status"].as_str().unwrap_or("ok");
        let (_resp, dbg) = ganesh_core::search::search_debug(&store, question, "fccal", &cfg);
        let margin = dbg.trace.qa_margin.unwrap_or(f32::NEG_INFINITY);
        let span = dbg.trace.qa_span.clone();
        if margin < 0.0 || span.is_empty() {
            println!("{} | {} | qa-margin={:.2} | no-positive-span", id, expect, margin);
            continue;
        }
        let claim = ganesh_core::factcheck::to_declarative_claim(question, &span);
        let mut lines = Vec::new();
        for pid in &dbg.selected {
            if let Some((art, text)) = texts.get(pid) {
                let prem = format!("{art} — \n{text}");
                if let Some(n) = ganesh_core::factcheck::verify_pair(&prem, &claim) {
                    lines.push(format!("{pid}:e{:.3}", n.entailment));
                }
            }
        }
        println!("{} | {} | qa-margin={:.2} | claim=\"{}\" | {}", id, expect, margin, claim, lines.join(" "));
    }
}
