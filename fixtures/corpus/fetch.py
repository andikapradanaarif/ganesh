#!/usr/bin/env python3
"""One-time snapshot fetcher (dev-time tool, not part of the offline replay path).
Fetches plaintext extracts (exsectionformat=wiki) plus revision metadata for the
seeded selection, writes verbatim snapshot bytes and the manifest skeleton."""
import json, time, urllib.request, urllib.parse, hashlib, os, datetime

UA = "ganesh-corpus-snapshot/1.0 (offline retrieval research fixture; https://github.com/andikapradanaarif/ganesh/issues/159)"
sel = json.load(open("fixtures/corpus/selection.json"))
today = datetime.date.today().isoformat()
entries = []
for title in sel["selected"]:
    q = urllib.parse.urlencode({
        "action": "query", "prop": "extracts|info", "explaintext": "true",
        "exsectionformat": "wiki", "format": "json", "titles": title, "redirects": "1"})
    req = urllib.request.Request("https://en.wikipedia.org/w/api.php?" + q,
                                 headers={"User-Agent": UA})
    with urllib.request.urlopen(req) as r:
        data = json.load(r)
    page = list(data["query"]["pages"].values())[0]
    text = page["extract"]
    raw = text.encode("utf-8")
    slug = page["title"].replace(" (programming language)", "").replace(" Sharp", "-sharp").replace("+", "plusplus").replace(" ", "-").lower()
    fname = f"{slug}.txt"
    path = f"fixtures/corpus/snapshots/{fname}"
    with open(path, "wb") as f:
        f.write(raw)
    canon = "https://en.wikipedia.org/wiki/" + urllib.parse.quote(page["title"].replace(" ", "_"))
    entries.append({
        "article_id": slug, "title": page["title"], "pageid": page["pageid"],
        "revision": str(page["lastrevid"]),
        "revision_url": f"{canon}?oldid={page['lastrevid']}",
        "publisher": "Wikipedia contributors, English Wikipedia",
        "url": canon, "license": "CC BY-SA 4.0",
        "license_url": "https://creativecommons.org/licenses/by-sa/4.0/",
        "permission": "Text of English Wikipedia articles is licensed under CC BY-SA 4.0, which permits storage and redistribution with attribution; attribution via publisher, article URL and revision URL recorded here.",
        "retrieved": today, "bytes": len(raw),
        "sha256": hashlib.sha256(raw).hexdigest(), "file": f"snapshots/{fname}",
        "family": "dev" if page["title"] in [t for t in sel["dev_family"]] or page["title"] in sel["dev_family"] else "heldout",
    })
    print(f"{page['title']:40s} rev={page['lastrevid']} bytes={len(raw)}")
    time.sleep(1)
manifest = {
    "version": "corpus.v1",
    "created": today,
    "frame": "14 widely-known general-purpose programming languages (see selection.json frame)",
    "seed": sel["seed"],
    "exclusions": [
        "Articles with plaintext extracts shorter than 15000 bytes (none triggered)",
        "Sections titled 'See also', 'References', 'Notes', 'Further reading', 'External links' and their subsections are excluded from passage segmentation at import time (navigational/bibliographic noise, recorded in FREEZE.md)"
    ],
    "sampling": "selection.json (seeded shuffle of frame, first 8 selected, seeded dev/heldout assignment)",
    "snapshot_format": "Verbatim plaintext bytes of the English Wikipedia API plaintext extract (action=query, prop=extracts, explaintext=true, exsectionformat=wiki) for the recorded revision. Section headings appear as lines like '== Heading ==' / '=== Subheading ==='.",
    "articles": entries,
}
with open("fixtures/corpus/manifest.json", "w") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
