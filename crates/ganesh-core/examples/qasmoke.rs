fn main() {
    let cases = [
        ("When was Curiosity launched?", "Mars Science Laboratory — Launch\nCuriosity was launched from Cape Canaveral on November 26, 2011.", "supported"),
        ("On what exact calendar date was the Apollo program established?", "Apollo program — MSFC\nOn July 1, 1960, NASA established the Marshall Space Flight Center (MSFC) in Huntsville, Alabama.", "entity-swap"),
        ("What is the diameter of the James Webb primary mirror in inches?", "James Webb Space Telescope — Mirror\nWebb's primary mirror consists of 18 hexagonal mirror segments made of gold-plated beryllium.", "missing-unit"),
    ];
    for (q, p, label) in cases {
        match ganesh_core::qa_verify::verify_pair(q, p) {
            Some(v) => println!("{label}: margin={:.2} span=\"{}\" null={:.2} best={:.2}", v.margin, v.span, v.null_score, v.best_span_score),
            None => println!("{label}: MODEL-UNAVAILABLE"),
        }
    }
}
