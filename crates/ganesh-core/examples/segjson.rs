fn main() {
    let path = std::env::args().nth(1).unwrap();
    let raw = std::fs::read_to_string(&path).unwrap();
    let mut out = Vec::new();
    for p in ganesh_core::segment::segment(&raw) {
        out.push(serde_json::json!({"seq": p.seq, "section": p.section, "text": p.text}));
    }
    println!("{}", serde_json::to_string(&out).unwrap());
}
