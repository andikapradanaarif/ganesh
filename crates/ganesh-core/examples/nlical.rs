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
        let (_resp, dbg) = ganesh_core::search::search_debug(&store, question, "nlical", &cfg);
        let margin = dbg.trace.qa_margin.unwrap_or(f32::NEG_INFINITY);
        let span = dbg.trace.qa_span.clone();
        let mut lines = Vec::new();
        if margin >= 0.0 && !span.is_empty() {
            let hyp = format!("Question: {question} Answer: {span}");
            for pid in &dbg.selected {
                if let Some((art, text)) = texts.get(pid) {
                    let prem = format!("{art} — \n{text}");
                    if let Some(n) = ganesh_core::nli_verify::verify_pair(&prem, &hyp) {
                        lines.push(format!("{pid}:e{:.2}/n{:.2}/c{:.2}", n.entailment, n.neutral, n.contradiction));
                    }
                }
            }
        }
        println!("{} | {} | qa-margin={:.2} span=\"{}\" | {}", id, expect, margin, span.chars().take(36).collect::<String>(), lines.join(" "));
    }
}
