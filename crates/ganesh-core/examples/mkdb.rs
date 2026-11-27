fn main() {
    let corpus = std::env::args().nth(1).expect("corpus dir");
    let db = std::env::args().nth(2).expect("db path");
    let manifest_text = std::fs::read_to_string(format!("{corpus}/manifest.json")).expect("manifest");
    let manifest: ganesh_core::types::Manifest = serde_json::from_str(&manifest_text).expect("parse");
    let mut store = ganesh_core::store::Store::open(std::path::Path::new(&db)).expect("open");
    for entry in &manifest.articles {
        let bytes = std::fs::read(format!("{corpus}/{}", entry.file)).unwrap();
        let _ = store.import_snapshot(entry, &bytes).expect("import");
    }
    store.rebuild_index().expect("rebuild");
    println!("db ready");
}
