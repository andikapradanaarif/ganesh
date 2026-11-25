//! Passage segmentation implementing the frozen spec in fixtures/questions/FREEZE.md section 2.
//! The conformance test in this crate verifies byte-identical output against
//! fixtures/corpus/passages/*.json, the reference enumeration used to author the keys.

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Passage {
    pub seq: u32,
    /// Ancestor heading titles; empty for the article lead.
    pub section: Vec<String>,
    pub text: String,
}

/// Parse a heading line per spec step 2: `^(=+) +(.*?) +\1 *$` with 2-4 '='.
/// Returns (level, title) or None.
fn parse_heading(line: &str) -> Option<(usize, String)> {
    let n = line.bytes().take_while(|b| *b == b'=').count();
    if !(2..=4).contains(&n) {
        return None;
    }
    // At least one space after the opening run.
    let after_open = &line[n..];
    if !after_open.starts_with(' ') {
        return None;
    }
    // Trailing spaces are allowed after the closing run.
    let trimmed_end = line.trim_end_matches(' ');
    // The line must end with exactly n '=' characters.
    let tail = trimmed_end.bytes().rev().take_while(|b| *b == b'=').count();
    if tail != n {
        return None;
    }
    let close_start = trimmed_end.len() - n;
    // At least one space before the closing run.
    if close_start <= n || !trimmed_end.as_bytes()[close_start - 1].is_ascii_whitespace() {
        return None;
    }
    let title = trimmed_end[n..close_start].trim();
    if title.is_empty() {
        return None;
    }
    Some((n, title.to_string()))
}

const EXCLUDED: [&str; 5] = ["See also", "References", "Notes", "Further reading", "External links"];

/// Segment snapshot text into passages per the frozen spec.
pub fn segment(text: &str) -> Vec<Passage> {
    let mut passages = Vec::new();
    let mut stack: Vec<(usize, String)> = Vec::new();
    let mut excluded_depth: Option<usize> = None;
    let mut buf: Vec<String> = Vec::new();

    macro_rules! flush {
        () => {
            if !buf.is_empty() {
                if excluded_depth.is_none() {
                    let seq = passages.len() as u32 + 1;
                    passages.push(Passage {
                        seq,
                        section: stack.iter().map(|(_, t)| t.clone()).collect(),
                        text: buf.join("\n"),
                    });
                }
                buf.clear();
            }
        };
    }

    for line in text.split('\n') {
        if let Some((level, title)) = parse_heading(line) {
            flush!();
            while let Some((l, _)) = stack.last() {
                if *l >= level {
                    stack.pop();
                } else {
                    break;
                }
            }
            if let Some(d) = excluded_depth {
                if level <= d {
                    excluded_depth = None;
                }
            }
            if EXCLUDED.contains(&title.as_str()) && excluded_depth.is_none() {
                excluded_depth = Some(level);
            }
            stack.push((level, title));
            continue;
        }
        if line.trim().is_empty() {
            flush!();
        } else {
            buf.push(line.to_string());
        }
    }
    flush!();
    passages
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn heading_parsing() {
        assert_eq!(parse_heading("== History =="), Some((2, "History".into())));
        assert_eq!(parse_heading("=== Types ==="), Some((3, "Types".into())));
        assert_eq!(parse_heading("== 2006–2009: Early years =="), Some((2, "2006–2009: Early years".into())));
        assert_eq!(parse_heading("not a heading"), None);
        assert_eq!(parse_heading("= x ="), None);
        assert_eq!(parse_heading("===== too deep ====="), None);
        assert_eq!(parse_heading("==unspaced=="), None);
    }

    #[test]
    fn excludes_reference_sections() {
        let text = "lead para\n\n== Body ==\n\nkept\n\n== References ==\n\ndropped\n\n=== sub ===\n\nalso dropped\n\n== After ==\n\nkept again\n";
        let ps = segment(text);
        let texts: Vec<&str> = ps.iter().map(|p| p.text.as_str()).collect();
        assert_eq!(texts, vec!["lead para", "kept", "kept again"]);
    }
}
