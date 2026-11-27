#!/usr/bin/env python3
"""Reference implementation of the frozen passage segmentation spec (see SPEC below).
Generates fixtures/corpus/passages/<slug>.json used for evidence-key authoring.
The Rust importer must reproduce these records exactly (conformance test).

SEGMENTATION SPEC (corpus.v1):
1. Input: verbatim snapshot bytes decoded as UTF-8.
2. A heading line matches ^(=+) +(.*?) +\\1 *$ with 2-4 '=' characters; level = count of '='.
   Lines that look like headings but have unbalanced/triple-digit markers are plain text.
3. Maintain a heading stack; a level-N heading replaces all entries at depth >= N.
   Text before the first heading belongs to the root section "" (the article lead).
4. Excluded sections: any heading whose title is exactly one of 'See also', 'References',
   'Notes', 'Further reading', 'External links'. All content under such a heading and its
   subsections is skipped until a heading of equal or higher level (fewer or equal '=').
5. Paragraphs: maximal runs of non-empty lines separated by blank lines, lines joined
   with a single newline character. Heading lines are never part of a paragraph.
6. Each included paragraph is one passage. seq numbers are assigned in document order
   starting at 1 counting only included passages. Passage ID = '<revision>#p<seq>'.
7. Each passage records: seq, section path (list of ancestor heading titles, empty for lead),
   and verbatim text.
"""
import json, re, os, sys

HEADING = re.compile(r"^(=+) +(.*?) +\1 *$")
EXCLUDED = {"See also", "References", "Notes", "Further reading", "External links"}

def segment(text):
    passages = []
    stack = []           # list of (level, title)
    excluded_depth = None
    buf = []
    def flush():
        nonlocal buf
        if buf:
            if excluded_depth is None:
                passages.append({
                    "seq": len(passages) + 1,
                    "section": [t for _, t in stack],
                    "text": "\n".join(buf),
                })
        buf = []
    for line in text.split("\n"):
        m = HEADING.match(line)
        if m and 2 <= len(m.group(1)) <= 4:
            flush()
            level, title = len(m.group(1)), m.group(2)
            while stack and stack[-1][0] >= level:
                stack.pop()
            if excluded_depth is not None and level <= excluded_depth:
                excluded_depth = None
            stack.append((level, title))
            if title in EXCLUDED and excluded_depth is None:
                excluded_depth = level
            continue
        if line.strip() == "":
            flush()
        else:
            buf.append(line)
    flush()
    return passages

manifest = json.load(open("fixtures/corpus/manifest.json"))
os.makedirs("fixtures/corpus/passages", exist_ok=True)
total = 0
for a in manifest["articles"]:
    text = open(f"fixtures/corpus/{a['file']}", encoding="utf-8").read()
    ps = segment(text)
    out = []
    for p in ps:
        out.append({
            "id": f"{a['revision']}#p{p['seq']}",
            "seq": p["seq"],
            "section": " > ".join(p["section"]) if p["section"] else "(lead)",
            "chars": len(p["text"]),
            "text": p["text"],
        })
    with open(f"fixtures/corpus/passages/{a['article_id']}.json", "w") as f:
        json.dump({"article_id": a["article_id"], "revision": a["revision"],
                   "family": a["family"], "passages": out}, f, indent=1)
        f.write("\n")
    total += len(out)
    print(f"{a['article_id']:12s} {len(ps):4d} passages  family={a['family']}")
print("total passages:", total)
