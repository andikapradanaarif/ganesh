fn main() {
    let cases = [
        ("premise-supported", "Mauna Loa — Eruption\nThe next successful ascent was made on January 29, 1834, by the expedition.", "Question: On what exact date was the next successful ascent of Mauna Loa made in 1834? Answer: january 29, 1834"),
        ("premise-inverted", "Mauna Loa — History\nIn February of that year Menzies attempted a direct course for the summit, before they were turned away by the thickness of the forest.", "Question: In what year did the first attempt to summit Mauna Loa succeed? Answer: 1793"),
        ("premise-entityswap", "Curie — Nobel\nShe won the 1911 Nobel Prize in Chemistry for the discovery of the elements radium and polonium.", "Question: In what year did Pierre Curie win the Nobel Prize in Chemistry? Answer: 1911"),
    ];
    for (label, prem, hyp) in cases {
        match ganesh_core::nli_verify::verify_pair(prem, hyp) {
            Some(n) => println!("{label}: entail={:.3} neutral={:.3} contra={:.3}", n.entailment, n.neutral, n.contradiction),
            None => println!("{label}: MODEL-UNAVAILABLE"),
        }
    }
}
