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
    for e in mani["articles"].as_array().unwrap() {
        let bytes = std::fs::read(format!("{corpus}/{}", e["file"].as_str().unwrap())).unwrap();
        let entry: ganesh_core::types::ManifestEntry = serde_json::from_value(e.clone()).unwrap();
        store.import_snapshot(&entry, &bytes).unwrap();
    }
    store.rebuild_index().unwrap();
    let cfg = ganesh_core::search::RetrievalConfig::default();
    for q in key["questions"].as_array().unwrap() {
        let id = q["id"].as_str().unwrap().to_string();
        if !filter.is_empty() && !filter.contains(&id) { continue; }
        let question = q["question"].as_str().unwrap();
        let (_resp, dbg) = ganesh_core::search::search_debug(&store, question, "decdbg", &cfg);
        println!("== {} [{}] sel=[{}]", id, question, dbg.selected.join(" "));
        println!("   decision: {} | {}", dbg.trace.decision, dbg.trace.reasons.join(" ; "));
        for m in dbg.trace.quantity_mentions.iter().take(4) {
            println!("   qty: {} `{}` excl={} | {}", m.passage_id, m.token, m.exclusion.clone().unwrap_or("-".into()), &m.sentence[..m.sentence.len().min(90)]);
        }
    }
}
