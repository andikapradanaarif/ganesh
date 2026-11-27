//! Conformance: the Rust segmenter must reproduce fixtures/corpus/passages/*.json
//! exactly — the reference enumeration used to author the frozen evidence keys.

use ganesh_core::segment::segment;
use serde::Deserialize;

#[derive(Deserialize)]
struct RefFile {
    revision: String,
    passages: Vec<RefPassage>,
}

#[derive(Deserialize)]
struct RefPassage {
    id: String,
    seq: u32,
    section: String,
    text: String,
}

#[test]
fn segmenter_matches_reference_enumeration() {
    let corpus = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../../fixtures/corpus");
    let manifest: serde_json::Value = serde_json::from_str(
        &std::fs::read_to_string(corpus.join("manifest.json")).unwrap(),
    )
    .unwrap();
    for article in manifest["articles"].as_array().unwrap() {
        let slug = article["article_id"].as_str().unwrap();
        let file = article["file"].as_str().unwrap();
        let text = std::fs::read_to_string(corpus.join(file)).unwrap();
        let reffile: RefFile = serde_json::from_str(
            &std::fs::read_to_string(corpus.join(format!("passages/{slug}.json"))).unwrap(),
        )
        .unwrap();
        let passages = segment(&text);
        assert_eq!(
            passages.len(),
            reffile.passages.len(),
            "{slug}: passage count mismatch ({} vs {})",
            passages.len(),
            reffile.passages.len()
        );
        for (got, want) in passages.iter().zip(reffile.passages.iter()) {
            let want_id = format!("{}#p{}", reffile.revision, want.seq);
            assert_eq!(want.id, want_id, "{slug} seq {} id shape", want.seq);
            assert_eq!(got.seq, want.seq, "{slug} seq ordering");
            let section = if got.section.is_empty() {
                "(lead)".to_string()
            } else {
                got.section.join(" > ")
            };
            assert_eq!(section, want.section, "{slug}#p{} section", want.seq);
            assert_eq!(&got.text, &want.text, "{slug}#p{} text", want.seq);
        }
    }
}

#[test]
fn fts5_available() {
    let conn = rusqlite::Connection::open_in_memory().unwrap();
    conn.execute_batch("CREATE VIRTUAL TABLE t USING fts5(x); INSERT INTO t VALUES ('hello world');")
        .unwrap();
    let n: i64 = conn
        .query_row("SELECT COUNT(*) FROM t WHERE t MATCH 'hello'", [], |r| r.get(0))
        .unwrap();
    assert_eq!(n, 1);
}
