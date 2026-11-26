# M1 v2 paired evaluation — original vs revised retriever

Issue: #161 · Draft PR: #162 · Prior baseline: #159 / PR #160
Corpus freeze commit: `449ac5bbf251e1fdbb67cdef76ebab2d480caed4`

## Explicit verdict: FAIL

The revised retriever **does not pass M1** on the fresh frozen v2 evaluation.
It passes evidence coverage (21/24, above the 20/24 bar) but fails the
insufficient-evidence requirement badly (2/6, below 6/6). The run was complete,
not partial; therefore the verdict is FAIL rather than INCOMPLETE.

| Check | M1 pass condition | Revised result | Outcome |
| --- | --- | --- | --- |
| Evidence coverage | ≥20/24 | 21/24 | PASS |
| Insufficient evidence | 6/6 | 2/6 | **FAIL** |
| Adapter agreement | 30/30 | 30/30 | PASS |
| Citation integrity | every returned passage on both adapters | 156/156 | PASS |
| Reproducibility | clean import/index rebuild reproduce ordered results and statuses | byte-identical outcomes across clean replays | PASS |
| Provider calls | 0 | 0 | PASS |

## Frozen inputs and pins

- v2 manifest: `fixtures/corpus-v2/manifest.json`, SHA-256
  `848d1dc464dfee46c79237c6a6ed3763df087dea962bc1b33bb7211b321948a8`
- v2 key: `fixtures/questions-v2/heldout.v2.json`, SHA-256 `f40f536cf9c9e48f3da847ceb5f537d055514ed124eddf3601cc09fd11616759`
- v2 scoring specification: `fixtures/questions-v2/scoring-spec.json`, SHA-256 `433c27c4e7fbe2e9bc7768e7e83784785119fc790cd5b8d2128ad07b27d0757d`
- Freeze record: `fixtures/questions-v2/FREEZE.md`
- Original retriever pin: `cfb29ea71dc54d2eda89470ee5e8ebb22ffb8bb0`
- Revised retriever pin: `e2bd4442897898e1a1a1b4d4ce45feaaa386b77e`
- Both pins were recorded before either system's v2 outcomes were inspected.
- The exact pinned revised build was run separately and produced outcomes
  byte-identical to the fixture-commit run used for artifact convenience.

Exact frozen replay command, run in each pinned build with the v2 fixtures
available:

```
cargo build --release --workspace
cargo run --release -p ganesh-bench -- run \
  --questions fixtures/questions-v2/heldout.v2.json \
  --corpus fixtures/corpus-v2 \
  --report out/heldout-v2-report.md \
  --details out/heldout-v2-details.json
```

Clean-checkout reproduction shape:

```
git clone https://github.com/andikapradanaarif/ganesh.git ganesh-v2-check
cd ganesh-v2-check
git checkout 449ac5bbf251e1fdbb67cdef76ebab2d480caed4   # frozen v2 fixtures
git worktree add /tmp/ganesh-v2-orig cfb29ea71dc54d2eda89470ee5e8ebb22ffb8bb0
git worktree add /tmp/ganesh-v2-rev  e2bd4442897898e1a1a1b4d4ce45feaaa386b77e
# build in each worktree, then run the frozen command with absolute paths to
# this checkout's fixtures/questions-v2 and fixtures/corpus-v2
```

## Head-to-head result

| Retriever | Coverage | Correct insufficient | Adapter | Citations | Verdict |
| --- | --- | --- | --- | --- | --- |
| Original pin `cfb29ea` | 18/24 | 4/6 | 30/30 | 144/144 | FAIL |
| Revised pin `e2bd444` | 21/24 | 2/6 | 30/30 | 156/156 | FAIL |

Separate findings:

1. The revision materially improved evidence coverage: +3 net answerable hits
   on identical fresh questions (18→21), crossing the coverage bar.
2. The same revision materially weakened abstention: −2 net correct
   insufficient results (4→2). Subject-aware deeper selection finds more
   lexically related material, but that additional material also makes
   plausible unsupported questions look covered.
3. The comparison is paired on identical corpus/questions/scorer, so these are
   attributable case-level effects rather than differences in test difficulty.

## Paired case outcomes

| Case | Expected | Original | Revised | Change |
| --- | --- | --- | --- | --- |
| v01 | ok | hit/correct | miss/leak | regressed |
| v02 | ok | hit/correct | hit/correct | unchanged |
| v03 | ok | miss/leak | hit/correct | improved |
| v04 | ok | hit/correct | hit/correct | unchanged |
| v05 | ok | miss/leak | hit/correct | improved |
| v06 | ok | hit/correct | hit/correct | unchanged |
| v07 | ok | miss/leak | hit/correct | improved |
| v08 | ok | hit/correct | hit/correct | unchanged |
| v09 | ok | hit/correct | hit/correct | unchanged |
| v10 | ok | hit/correct | hit/correct | unchanged |
| v11 | ok | hit/correct | hit/correct | unchanged |
| v12 | ok | hit/correct | hit/correct | unchanged |
| v13 | ok | hit/correct | hit/correct | unchanged |
| v14 | ok | hit/correct | hit/correct | unchanged |
| v15 | ok | hit/correct | hit/correct | unchanged |
| v16 | ok | hit/correct | hit/correct | unchanged |
| v17 | ok | hit/correct | hit/correct | unchanged |
| v18 | ok | miss/leak | hit/correct | improved |
| v19 | ok | miss/leak | miss/leak | unchanged |
| v20 | ok | hit/correct | hit/correct | unchanged |
| v21 | ok | hit/correct | hit/correct | unchanged |
| v22 | ok | miss/leak | miss/leak | unchanged |
| v23 | ok | hit/correct | hit/correct | unchanged |
| v24 | ok | hit/correct | hit/correct | unchanged |
| v25 | insufficient_context | hit/correct | miss/leak | regressed |
| v26 | insufficient_context | miss/leak | hit/correct | improved |
| v27 | insufficient_context | hit/correct | miss/leak | regressed |
| v28 | insufficient_context | hit/correct | miss/leak | regressed |
| v29 | insufficient_context | hit/correct | hit/correct | unchanged |
| v30 | insufficient_context | miss/leak | miss/leak | unchanged |

Improved: v03, v05, v07, v18 (answerable), v26 (insufficient).
Regressed: v01 (answerable), v25, v27, v28 (insufficient).
Unchanged: the other 21 cases, including persistent misses v19, v22, v30.

## Measured costs

| Measure | Original | Revised |
| --- | --- | --- |
| Corpus | 135,210 bytes, 4 snapshots | same |
| Passages | 137 | 137 |
| Import time | 9.6 ms | 7.0 ms |
| Index rebuild | 97.7 ms | 86.2 ms |
| Store size | 471,040 bytes incl. WAL | 483,328 bytes incl. WAL |
| REST query time | mean 8.0 ms, max 10.6 ms | mean 13.7 ms, max 30.6 ms |
| MCP query time | mean 8.3 ms, max 9.7 ms | mean 13.4 ms, max 15.7 ms |
| Response bytes | 214,442 | 238,024 |
| Wall time | 1,744 ms | 1,769 ms |
| Hardware | macOS x86_64, Intel Core i7-9750H @ 2.60GHz | same |
| Provider calls | 0 | 0 |

The revised system's deeper candidate pool explains most of the latency and
response-byte increase.

## Every remaining revised-system miss

### Answerable misses (3/24)

- **v01 — Python conception/institute: false abstention.** Correct passage
  `1386984050#p2` was ranked first, but the question-only terms `research` and
  `institute` are not in the evidence text (`Centrum Wiskunde & Informatica`),
  leaving IDF coverage 0.66 below threshold. Likely cause: lexical paraphrase
  gap in question phrasing. Smallest change worth testing: do not count
  low-value institutional/type nouns in the coverage denominator, or use a
  hybrid semantic retriever. This is a fresh example of the paraphrase class.
- **v19 — C creator/year: wrong passages.** The subject `C` is a single letter
  and is still discarded; `programming language` is shared across all articles.
  Correct lead evidence `1387656235#p1` loses to other C passages and cross-subject
  noise. Likely cause: bare single-letter symbolic subjects remain unrepresentable
  even though punctuation-bearing forms such as C++/C# are now canonicalized.
  Smallest change worth testing: treat a capitalized single-letter language
  mention followed by “programming language” as a subject phrase, not a token.
- **v22 — C reserved-word counts: false abstention.** The relevant passages use
  digits for C89 (`32`) but spell out `fifteen` for C23. The quantity rule
  requires a digit token, so the system abstains despite correct passages and
  full lexical coverage. Likely cause: quantity-evidence rule is too narrow.
  Smallest change worth testing: recognize number words as quantity evidence.

### Insufficient-evidence leaks (4/6)

- **v25 — C++23 keyword count.** The C++ article discusses standards and
  keywords but never counts added keywords. Digits from version years satisfy
  the shallow numeric-evidence rule. Likely cause: numeric evidence is not
  required to be near the measured noun. Smallest change: proximity-aware
  quantity evidence.
- **v27 — optional lazy evaluation in Haskell.** Passages genuinely about lazy
  evaluation cover all lexical terms, but none says it became optional. This is
  a semantic missing-detail case beyond the current lexical coverage signal.
- **v28 — ISO WG21 membership count.** Working-group passages contain years and
  meeting counts/digits, satisfying shallow quantity evidence while never
  stating membership count. Likely cause: same numeric-proximity defect as v25.
- **v30 — C operator count.** Operator-list passages cover every lexical term
  but never state a total. Likely cause: same quantity/proximity defect; digits
  elsewhere in passages are not evidence of an operator count.

## Development evidence that led here

Exposed-v1 measurements are recorded in `evidence/retrieval/change-log.md` and
the copied reports. In short: symbolic identifier normalization fixed h15;
subject-aware deep selection improved coverage but showed complement-selection
risks; exact units plus quantity-evidence abstention fixed h25/h26 without
false-abstaining two supported numeric controls. The v2 results confirm the
coverage benefit and expose the abstention cost at fresh scale.

## Smallest next experiment (because this is a FAIL)

Design a proximity-aware quantity-evidence experiment:

1. recognize digit tokens and a bounded list of number words as quantities;
2. for `how many`/`how much` questions, require the quantity to occur in the
   same sentence or a small token window as the measured noun phrase terms
   (`keywords`, `members`, `operators`, `bytes`, etc.), rather than anywhere in
   the passage;
3. test it first on exposed supported controls (`Go 25 keywords`, `i32 32
   bits`, v22 C89/C23 counts) and near-identical unsupported cases (v25, v28,
   v30, h26), tracking false abstentions and unsupported `ok` responses;
4. validate any claim only on a fresh frozen held-out set.

A second independent candidate is subject-phrase handling for bare `C`, but it
affects fewer cases than the quantity defect. Multi-passage paraphrase gaps
(v01 and the exposed h01/h12/h19 class) remain candidates for a later hybrid
lexical+semantic comparison.

No M2 fixture proposal is made: M1 did not pass.
