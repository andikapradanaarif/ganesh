# Ganesh M1 v2 held-out freeze

Frozen before either retriever configuration was run on the v2 questions.

## Corpus

- Corpus version: `corpus.v2`, manifest `fixtures/corpus-v2/manifest.json`,
  manifest SHA-256: `848d1dc464dfee46c79237c6a6ed3763df087dea962bc1b33bb7211b321948a8`.
- Families: C++, Python, Haskell, and C. These are disjoint from every family
  used for v1 tuning or diagnosis. TypeScript (9,375 bytes) and Kotlin
  (12,105 bytes) were excluded by the recorded 15,000-byte minimum and replaced
  with the remaining unexposed frame members Haskell and C.
- Permissions: English Wikipedia article text, CC BY-SA 4.0; publisher, URLs,
  retrieval date, revisions, bytes and per-article digests are in the manifest.

## Questions and key

- `fixtures/questions-v2/heldout.v2.json`: 24 answerable questions v01-v24
  and 6 insufficient-evidence questions v25-v30.
- The set includes symbolic subjects (C++, C), ordinary subjects (Python,
  Haskell), multi-passage questions, a legitimate two-subject comparison, a
  supported quantity question, and plausible missing details.
- The key is evaluation input only. It is not indexed and is not supplied to
  the REST service, MCP bridge, importer, or retrieval implementation.

## Scoring and replay

- `fixtures/questions-v2/scoring-spec.json` freezes the acceptance counts:
  at least 20/24 answerable hits, 6/6 correct insufficient-context results,
  30/30 adapter agreement, complete citation integrity, top-three limit, zero
  provider calls.
- Exact replay command for each retriever build:

```
cargo build --release --workspace
cargo run --release -p ganesh-bench -- run \
  --questions fixtures/questions-v2/heldout.v2.json \
  --corpus fixtures/corpus-v2 \
  --report out/heldout-v2-report.md \
  --details out/heldout-v2-details.json
```

## Pinned retriever configurations

- Original retriever: commit `cfb29ea71dc54d2eda89470ee5e8ebb22ffb8bb0`
  (hardened v1 baseline; pre-symbolic-identifier retrieval).
- Revised retriever: commit `e2bd4442897898e1a1a1b4d4ce45feaaa386b77e` (subject-aware selection, symbolic
  identifier aliases, exact quantity units, quantity-evidence abstention).

These pins were recorded before inspecting either version's outcomes on v2.
After exposure, v2 outcomes may be used for diagnosis only; improvement claims
require a fresh held-out set.
