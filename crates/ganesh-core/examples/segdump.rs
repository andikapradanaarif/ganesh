fn main() {
    let path = std::env::args().nth(1).unwrap();
    let pat = std::env::args().nth(2).unwrap_or_default().to_lowercase();
    let raw = std::fs::read_to_string(&path).unwrap();
    for p in ganesh_core::segment::segment(&raw) {
        let low = p.text.to_lowercase();
        if pat.is_empty() || low.contains(&pat) {
            let flat: String = p.text.chars().take(150).collect::<String>().replace('\n', " ");
            println!("p{}|{}", p.seq, flat);
        }
    }
}
