# M1 benchmark freeze (freeze.v1)

Frozen on 2026-11-26, before any retrieval implementation existed and before
the first held-out run. Changes after this commit invalidate the freeze for
held-out acceptance; later runs are development evidence only.

## 1. Corpus

- Corpus version: `corpus.v1`, manifest `fixtures/corpus/manifest.json`,
  manifest SHA-256: `c9377063cc47566637317cf96f73a15ac4e4452eaef77b09e59401fb64f74cef`.
- Frame: 14 widely-known general-purpose programming languages (recorded in
  `fixtures/corpus/selection.json`). Seed 20261126; seeded shuffle; first 8
  selected; seeded family assignment. No article fell under the 15000-byte
  exclusion threshold, so no replacements occurred.
- Dev family: java, php, rust, swift. Held-out family: c-sharp, go, javascript, ruby.
  Each article contributes exactly one revision; all revisions of an article
  family are on one side of the split by construction.
- Questions in `dev.v1.json` draw evidence only from dev-family articles;
  questions in `heldout.v1.json` only from held-out-family articles. Retrieval
  always searches the full corpus, so cross-family near-neighbour distractors
  are present for both sets.
- Excluded sections at import time: headings titled exactly `See also`,
  `References`, `Notes`, `Further reading`, `External links`, including their
  subsections.

## 2. Passage segmentation (spec corpus.v1, authoritative)

1. Input: verbatim snapshot bytes decoded as UTF-8.
2. A heading line matches `^(=+) +(.*?) +\1 *$` with 2-4 `=` characters;
   level = count of `=`. Unbalanced lookalikes are plain text.
3. Maintain a heading stack; a level-N heading replaces all entries at depth >= N.
   Text before the first heading belongs to the root section (the article lead).
4. Excluded sections: any heading whose title is exactly one of the excluded
   titles above. All content under such a heading and its subsections is
   skipped until a heading of equal or higher level.
5. Paragraphs: maximal runs of non-empty lines separated by blank lines, lines
   joined with a single newline. Heading lines are never part of a paragraph.
6. Each included paragraph is one passage. `seq` numbers are assigned in
   document order starting at 1, counting only included passages.
   Passage ID = `<revision>#p<seq>`.
7. `fixtures/corpus/passages/<article>.json` is the reference enumeration used
   to author the keys; the Rust importer must reproduce it exactly
   (conformance is test-enforced).

## 3. Evidence keys

- `dev.v1.json`: 11 answerable + 3 insufficient-evidence questions (dev family).
- `heldout.v1.json`: 24 answerable + 6 insufficient-evidence questions (held-out family).
- Each answerable question records required facts; each fact lists acceptable
  passage IDs. Any one acceptable passage per fact suffices ("or equivalents").
- Keys are inputs to the scorer only. They are never imported into the search
  index nor included in any agent/retrieval input.

## 4. Scoring rules

- **Answerable hit**: the response status is `ok` AND for every required fact
  at least one acceptable passage ID appears in the returned top-three passage
  IDs. An abstention (`insufficient_context`) on an answerable question is a miss.
  A hit on the right article with the wrong paragraph scores zero.
- **Insufficient correct**: the response status is exactly `insufficient_context`.
- **Adapter agreement**: for each question, REST and MCP responses agree on
  status and the ordered list of passage IDs exactly.
- **Citation integrity**: every citation returned by either adapter is opened
  through the same adapter and must resolve to the same passage ID and revision
  it was returned for.
- Partial executions (any question unanswered by either adapter) make the run
  incomplete, not a failure or pass.

## 5. Acceptance counts

| Check | Pass condition |
| --- | --- |
| Evidence coverage | >= 20 of 24 answerable questions are hits |
| Insufficient evidence | 6 of 6 insufficient questions return `insufficient_context` |
| Adapter agreement | 30 of 30 questions agree between REST and MCP |
| Citation integrity | 100% of returned citations resolve to the returned passage and revision |
| Reproducibility | clean import + index rebuild reproduces outcomes; provider calls = 0 |

## 6. Exact replay command

From a clean checkout of the branch, with the corpus files present:

```
cargo build --release --workspace
cargo run --release -p ganesh-bench -- run \
  --questions fixtures/questions/heldout.v1.json \
  --report out/heldout-report.md
```

The bench command must, in one invocation: create a fresh temporary store,
import the pinned corpus with digest verification, rebuild the search index,
start the REST service on a loopback port, execute every question through the
real REST `POST /v1/search` endpoint and through the real MCP stdio bridge
`search_context` tool, open every returned citation through both adapters,
verify snapshot digests, score against the frozen key, and write the
per-question report plus measured costs. The benchmark runs offline with zero
provider calls.

## 7. Tuning protocol

Ranking and abstention parameters may be calibrated using `dev.v1.json` only.
The held-out set is run once against the frozen system. If the baseline misses,
the failure analysis is published; any question that informed a fix makes later
results on it development evidence, and a new improvement claim requires fresh
held-out cases.

## 8. Quickstart examples (must appear in the report/PR)

- One successful lookup replayed end to end through REST and MCP.
- One `insufficient_context` outcome replayed end to end through REST and MCP.
