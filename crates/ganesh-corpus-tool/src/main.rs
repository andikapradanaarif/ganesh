//! Corpus fixture tool: fetch pinned public article snapshots and enumerate
//! frozen passages. This is fixture preparation tooling only; the offline
//! benchmark consumes the frozen output and never invokes this tool.

use ganesh_core::citation::sha256_hex;
use serde_json::{json, Value};
use std::fmt::Write as _;

fn urlencode(s: &str) -> String {
    let mut out = String::new();
    for b in s.bytes() {
        if b.is_ascii_alphanumeric() || matches!(b, b'-' | b'_' | b'.' | b'~') {
            out.push(b as char);
        } else {
            write!(out, "%{b:02X}").unwrap();
        }
    }
    out
}

fn slug(title: &str) -> String {
    title.replace(" (programming language)", "")
        .replace(" Sharp", "-sharp")
        .replace("++", "plusplus")
        .replace("+", "plus")
        .replace(' ', "-")
        .to_ascii_lowercase()
}

fn today() -> String {
    std::process::Command::new("date").arg("+%F").output().ok()
        .and_then(|o| String::from_utf8(o.stdout).ok())
        .map(|s| s.trim().to_string()).unwrap_or_else(|| "unknown".into())
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    if args.len() < 2 {
        eprintln!("usage: ganesh-corpus-tool fetch --out <dir> --titles <a,b> --frame <text> --seed <n> | segment --corpus <dir>");
        std::process::exit(2);
    }
    match args[1].as_str() {
        "fetch" => fetch(&args[2..]),
        "segment" => segment(&args[2..]),
        other => { eprintln!("unknown command {other}"); std::process::exit(2); }
    }
}

fn opt(args: &[String], name: &str) -> Option<String> {
    args.windows(2).find(|w| w[0] == name).map(|w| w[1].clone())
}

fn fetch(args: &[String]) {
    let out = opt(args, "--out").expect("--out");
    let titles = opt(args, "--titles").expect("--titles");
    let frame = opt(args, "--frame").unwrap_or_else(|| "recorded public frame".into());
    let seed: u64 = opt(args, "--seed").and_then(|s| s.parse().ok()).unwrap_or(0);
    std::fs::create_dir_all(format!("{out}/snapshots")).unwrap();
    let mut entries = Vec::new();
    for title in titles.split(',').map(str::trim).filter(|t| !t.is_empty()) {
        let query = format!(
            "action=query&prop=extracts%7Cinfo&explaintext=true&exsectionformat=wiki&format=json&titles={}&redirects=1",
            urlencode(title)
        );
        let url = format!("https://en.wikipedia.org/w/api.php?{query}");
        let text = ureq::get(&url)
            .set("User-Agent", "ganesh-corpus-snapshot/2.0 (offline retrieval research fixture)")
            .call().unwrap_or_else(|e| panic!("fetch {title}: {e}"))
            .into_string().unwrap();
        let data: Value = serde_json::from_str(&text).unwrap();
        let page = data["query"]["pages"].as_object().unwrap().values().next().unwrap().clone();
        let extract = page["extract"].as_str().expect("extract");
        let raw = extract.as_bytes();
        let actual_title = page["title"].as_str().unwrap().to_string();
        let slug = slug(&actual_title);
        let fname = format!("{slug}.txt");
        std::fs::write(format!("{out}/snapshots/{fname}"), raw).unwrap();
        let canon = format!("https://en.wikipedia.org/wiki/{}", actual_title.replace(' ', "_"));
        let rev = page["lastrevid"].as_u64().expect("lastrevid");
        entries.push(json!({
            "article_id": slug,
            "title": actual_title,
            "pageid": page["pageid"].as_u64().unwrap(),
            "revision": rev.to_string(),
            "revision_url": format!("{canon}?oldid={rev}"),
            "publisher": "Wikipedia contributors, English Wikipedia",
            "url": canon,
            "license": "CC BY-SA 4.0",
            "license_url": "https://creativecommons.org/licenses/by-sa/4.0/",
            "permission": "Text of English Wikipedia articles is licensed under CC BY-SA 4.0, which permits storage and redistribution with attribution; attribution via publisher, article URL and revision URL recorded here.",
            "retrieved": today(),
            "bytes": raw.len(),
            "sha256": sha256_hex(raw),
            "file": format!("snapshots/{fname}"),
            "family": "heldout-v2",
        }));
        println!("{} rev={} bytes={}", actual_title, rev, raw.len());
        std::thread::sleep(std::time::Duration::from_secs(1));
    }
    let manifest = json!({
        "version": "corpus.v2",
        "created": today(),
        "frame": frame,
        "seed": seed,
        "exclusions": [
            "Articles with plaintext extracts shorter than 15000 bytes are excluded",
            "Sections titled 'See also', 'References', 'Notes', 'Further reading', 'External links' and their subsections are excluded from passage segmentation"
        ],
        "sampling": "remaining unexposed members of the corpus.v1 seeded frame; C++, Python, TypeScript and Kotlin selected as disjoint v2 families",
        "snapshot_format": "Verbatim plaintext bytes of the English Wikipedia API plaintext extract (action=query, prop=extracts, explaintext=true, exsectionformat=wiki) for the recorded revision. Section headings appear as lines like '== Heading ==' / '=== Subheading ==='.",
        "articles": entries,
    });
    std::fs::write(format!("{out}/manifest.json"), serde_json::to_string_pretty(&manifest).unwrap() + "\n").unwrap();
}

fn segment(args: &[String]) {
    let corpus = opt(args, "--corpus").expect("--corpus");
    let manifest: Value = serde_json::from_str(&std::fs::read_to_string(format!("{corpus}/manifest.json")).unwrap()).unwrap();
    std::fs::create_dir_all(format!("{corpus}/passages")).unwrap();
    let mut total = 0usize;
    for a in manifest["articles"].as_array().unwrap() {
        let bytes = std::fs::read(format!("{corpus}/{}", a["file"].as_str().unwrap())).unwrap();
        let text = std::str::from_utf8(&bytes).unwrap();
        let ps = ganesh_core::segment::segment(text);
        let out: Vec<Value> = ps.iter().map(|p| {
            let id = format!("{}#p{}", a["revision"].as_str().unwrap(), p.seq);
            json!({
                "id": id,
                "seq": p.seq,
                "section": if p.section.is_empty() { "(lead)".to_string() } else { p.section.join(" > ") },
                "chars": p.text.len(),
                "text": p.text,
            })
        }).collect();
        total += out.len();
        let doc = json!({
            "article_id": a["article_id"], "revision": a["revision"], "family": a["family"], "passages": out,
        });
        let file = format!("{corpus}/passages/{}.json", a["article_id"].as_str().unwrap());
        std::fs::write(file, serde_json::to_string_pretty(&doc).unwrap() + "\n").unwrap();
        println!("{} {} passages", a["article_id"].as_str().unwrap(), out.len());
    }
    println!("total passages: {total}");
}
