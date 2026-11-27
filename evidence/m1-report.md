# M1 final report — evidence retrieval from a fixed corpus

Issue: #159 · Milestone: M1 — Evidence retrieval from a fixed corpus
Branch: `feat/159-m1-passage-retrieval` · Benchmark system commit: `605328b`
Freeze commit (pre-implementation): `d17069d`

## Verdict: FAIL

The held-out run completed in full (`incomplete=false`), but the frozen system
fell short of two acceptance criteria. Per the frozen scoring rules, no
post-hoc adjustment was made: the numbers below are the single frozen run, and
the analysis under "Misses" is diagnostic only. Any system changed in response
to these results must be revalidated on fresh held-out cases before it can
claim improvement.

| Check | Pass condition | Result | Outcome |
| --- | --- | --- | --- |
| Evidence coverage | ≥ 20/24 answerable | **19/24** | **FAIL** |
| Insufficient evidence | 6/6 explicit `insufficient_context` | **4/6** | **FAIL** |
| Adapter agreement | same status + ordered passage IDs on 30/30 | 30/30 | PASS |
| Citation integrity | every citation opens to expected passage/revision | 150/150 | PASS |
| Reproducibility | clean import + index rebuild reproduce outcomes | byte-identical across two clean replays | PASS |
| Provider calls | zero | 0 | PASS |

## Frozen inputs

- Corpus: `corpus.v1`, 8 Wikipedia snapshots, manifest
  `fixtures/corpus/manifest.json`, manifest SHA-256
  `c9377063cc47566637317cf96f73a15ac4e4452eaef77b09e59401fb64f74cef`
  (reverified byte-for-byte at report time).
- Frame/seed/split: 14-language frame, seed 20261126, family split —
  dev = Java/PHP/Rust/Swift, held-out = C#/Go/JavaScript/Ruby.
- Keys: `fixtures/questions/heldout.v1.json` (24 answerable h01–h24,
  6 insufficient h25–h30), frozen before implementation existed.
- Scoring rules + acceptance counts: `fixtures/questions/FREEZE.md`.
- Frozen replay command:

```
cargo build --release --workspace
cargo run --release -p ganesh-bench -- run \
  --questions fixtures/questions/heldout.v1.json \
  --report out/heldout-report.md
```

The harness performs a clean import into a fresh store, rebuilds the index,
runs all questions through the real REST and MCP entry points, opens every
returned citation through both adapters, and writes the per-question report.
Artifacts: `evidence/heldout-report.md`, `evidence/heldout-details.json`.

## Measured costs (held-out run)

| Measure | Value |
| --- | --- |
| Corpus size | 260,369 bytes across 8 snapshots |
| Passages | 346 |
| Import time | 18.4 ms |
| Index rebuild time | 171.5 ms |
| Store size | 815,104 bytes (incl. WAL) |
| REST query time | mean 7.5 ms, max 13.2 ms |
| MCP query time | mean 7.6 ms, max 9.6 ms |
| Response bytes | 201,272 total across both adapters |
| Wall time (30 questions × 2 adapters + citation opens) | 1,340 ms |
| Hardware | macOS x86_64, Intel Core i7-9750H @ 2.60GHz |
| Provider calls | 0 — no network access anywhere in the retrieval path |

## Per-question outcome summary

Answerable hits (19): h02–h11, h14, h16–h18, h20–h24.
Answerable misses (5): h01, h12, h13 (false abstention), h15, h19.
Insufficient correctly detected (4): h27, h28, h29, h30.
Insufficient leaked (2): h25, h26.
REST/MCP agreed on status and ordered passage IDs for all 30 questions;
all 150 opened citations resolved to the expected passage and revision.

## Misses — required evidence, actual output, likely cause, smallest change worth testing

### h01 — Go designers + Go 1.0 release month/year (multi-passage)

- Required: `1386442349#p1` (designed 2007 by Griesemer, Pike, Thompson) and
  `1386442349#p4` (version 1.0, March 2012).
- Actual: `1386442349#p8`, `1386442349#p41`, `1387553929#p12` — the last is a
  **PHP** passage. Neither required passage in top-3.
- Likely cause: multi-fact dilution. The query's term mass is spread across two
  target passages; each is individually a weak BM25 match (top scores only
  −13.6…−14.4), so passages dense in one fact's vocabulary — including
  cross-family noise — outrank them.
- Smallest change worth testing: decompose compound questions into per-fact
  subqueries and union the per-fact top results, or retrieve deeper than 3 and
  re-rank for coverage of disjoint term clusters. *(Class: ranking)*

### h12 — ECMAScript 6 year + JavaScript creator (multi-passage)

- Required: `1387609285#p6`/`#p24` (ES6 in 2015) and
  `1387609285#p1`/`#p2`/`#p55` (Brendan Eich, 1995).
- Actual: `1387609285#p6` at rank 1 (fact 1 covered), then `#p4`, `#p5` —
  same-edition overview passages that share vocabulary but do not state the
  creator. No Eich passage in top-3.
- Likely cause: same multi-fact dilution as h01 — fact 2's passages lost to
  fact-1-dense neighbours.
- Smallest change worth testing: same as h01.

### h13 — COOL expansion + first standard body (false abstention)

- Required: `1387643334#p4` (COOL = C-like Object Oriented Language) and
  `1387643334#p1` (Ecma standard).
- Actual: explicit `insufficient_context`. Probe shows the correct article
  dominated ranking — acceptable passage `#p4` at rank 1 — but IDF-weighted
  query coverage was 0.70, just under the frozen 0.75 abstention threshold,
  so the system abstained. The unmatched weight came from question-idiom
  vocabulary ("stand" as in "stand for", "body") that evidence passages
  naturally do not contain. Independently, `#p1` (fact 2) was absent from the
  top-3 — so even without the abstention this is a multi-fact coverage miss.
- Likely cause: abstention denominator inflated by idiom verbs + multi-fact
  dilution. Note also the "C#" token loss below applies to this question.
- Smallest change worth testing: down-weight or exclude low-specificity idiom
  verbs from the coverage denominator — but any threshold/weighting change is
  exactly the kind of coupling the freeze prohibits validating on these cases.

### h15 — C# multiple inheritance (single-passage, wrong-language results)

- Required: `1387643334#p21` (no multiple inheritance; any number of interfaces).
- Actual: `1387609285#p24` (JavaScript), `1386054413#p12` (Java),
  `1387629546#p16` (Ruby) — inheritance passages from the three wrong languages.
- Likely cause: **the tokenizer drops the question's only scoping token.**
  "C#" tokenizes to "c", which is discarded as a single non-digit character,
  so nothing ties the query to the C# article; inheritance vocabulary is
  shared across all four held-out languages and the correct passage — proven
  retrievable, since h18 ranked `#p21` first — has no scoping advantage.
  This failure class was invisible during development because no dev-family
  language (Java/PHP/Rust/Swift) has a symbolic name.
- Smallest change worth testing: normalize symbolic language names during
  tokenization (`c#` → `csharp`, `c++` → `cplusplus`) in both indexer and
  query pipeline. *(Class: tokenization — the clearest single defect.)*

### h19 — Ruby developer + two initially proposed names (multi-passage)

- Required: `1387629546#p1` (mid-1990s, Yukihiro "Matz" Matsumoto) and
  `1387629546#p3` (Coral and Ruby).
- Actual: `#p3` at rank 1 (fact 2 covered), then `#p5`, `#p4`. `#p1` absent —
  though proven retrievable (h22 ranked it third).
- Likely cause: multi-fact dilution, amplified by lexical overlap — "develop"
  stems match "development" in several Ruby passages, so term coverage does
  not imply fact coverage.
- Smallest change worth testing: same as h01.

### h25 — C# reserved keyword count (insufficient-evidence leak)

- Missing detail: the C# article never counts its reserved keywords;
  near-neighbour counts exist for other languages (Go 25, JavaScript 50–60)
  by design.
- Actual: `ok` with `1387609285#p13` — the **JavaScript** keyword-count
  passage — at rank 1, plus Rust `#p34`, JavaScript `#p41`. Query coverage
  1.00: every content word ("reserved", "keywords", "define") is well covered.
- Likely cause: compound — (a) the "C#" token loss removes the scoping term
  whose absence should have been the abstention signal; (b) the question is
  lexically indistinguishable from answerable near-neighbours, the same
  inherent ceiling documented on dev case d14.
- Smallest change worth testing: the `csharp` token normalization (h15) —
  if the scoping token survives, its absence of support becomes visible to
  coverage-based abstention. Whether that suffices is a fresh-held-out question.

### h26 — goroutine initial stack bytes (insufficient-evidence leak)

- Missing detail: the Go article discusses goroutine scheduling but never
  states an initial stack size.
- Actual: `ok` with `1386182972#p45` (**Rust** — "values are allocated on the
  stack"), `1386442349#p23` (goroutine multiplexing), `1386442349#p14` (Go's
  `byte` type). Query coverage 1.00.
- Likely cause: stem conflation — "bytes" stems to "byte", matching byte-type
  passages, and "allocated"/"stack" match the Rust passage verbatim. The rare
  critical term that should have been absent ("bytes" as a unit) exists in the
  corpus for unrelated reasons, so coverage-based abstention cannot fire.
- Smallest change worth testing: keep unit/numeric-qualified terms unstemmed
  or require exact surface form for them in the coverage check. Again —
  validation needs fresh held-out cases.

## Failure classes

1. **Multi-fact dilution** (h01, h12, h19, contributory h13): single-shot
   bag-of-words retrieval cannot guarantee that evidence split across two
   passages both lands in top-3. 4 of 5 answerable misses involve this class.
2. **Symbolic-name tokenization** (h15 decisive, h25/h13 contributory):
   "C#" loses its scoping token. The single clearest defect, and one the
   dev/held-out family split was designed to surface — no dev family has a
   symbolic name.
3. **Coverage-based abstention ceiling** (h25, h26 inherent; h13 boundary):
   lexically plausible missing-detail questions can reach coverage 1.00 via
   near-neighbour passages and stem collisions; threshold tuning cannot
   separate them (established on dev before the freeze).

## Reproducibility verification

The replay was executed a second time from a clean store with identical
outcomes — per-question status and ordered passage IDs byte-identical across
both runs (`out/heldout-details.json` vs `out/heldout-details-r2.json`).
The second execution is verification of determinism only; results above are
the first frozen run.

## Development evidence (pre-freeze, dev set only)

`evidence/dev-report.md`: coverage 9/11, insufficient 2/3, adapter agreement
14/14, citations 66/66. Dev misses d07 (paraphrase gap), d11 (false
abstention), d14 (inherent lexical leak) documented the same failure classes
that the held-out run confirmed at scale.

## What this leaves behind (per the miss protocol)

Specific reproducible retrieval problems, all replayable with the frozen
command above:

1. Compound questions split across two passages lose to single-fact dense
   passages — reproduce with h01/h12/h19.
2. Any question scoped by "C#" loses its scope token — reproduce with h15,
   where three wrong-language passages outrank the correct one.
3. Lexically plausible missing-detail questions saturate coverage — reproduce
   with h25/h26.

No M2 fixture proposal accompanies this report: the M1 bar was not met.
