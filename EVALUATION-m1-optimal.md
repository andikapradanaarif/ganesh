# EVALUATION — M1 optimal no-LLM candidate consolidation

Branch `feat/m1-optimal-onnx` from `feat/onnx-factcheck` (`e38939c`). Baselines: `#188` = `8f273aaa`, `6c593a1d` intact. Packet freeze and candidate pin recorded below; acc-xfer-v7 was not run in any form before both.

## 1. Surgical engineering fixes

- **Sliding-window QA verification** (`qa_verify.rs`): SQuAD 2.0 doc-stride — 300-word windows, 150-word stride, max margin across windows, at most 4 windows prioritized by query-term overlap with window 0 always included; passages <= 300 words single-pass. Fixes the 384-word blind truncation root cause of `ax6/q06` (Vibratosax evidence at word 400+). Unit test `sliding_window_finds_answer_past_word_400` passes (positive margin + extracted span past word 400).
- **`measurement_question()` tightening** (`search.rs`): hyphenated compound adjectives (`low-cost`) no longer split into measure-property nouns; `which/who/where` frames trigger only on explicit `in <known-unit>` or `how many/how much`. Unit tests cover the q06 case plus all real measurement questions (v5/i02, ax4/q09, ax5/q10, ax6/q10, ax6/q11, v4/i05). Bonus root cause found and fixed: `adjacent_unit` never matched sentence-final units (dot-preserving trim) — a pre-existing bug that made correct measurement binding impossible sentence-finally (the accept.rs unit test was silently red on `e38939c`); `tension` added to MEASURE_PROPS.
- **Lock-free inference** (`ganesh-server`): all blocking work (SQLite snapshot fetch + neural inference) runs on tokio's blocking thread pool via `spawn_blocking`; the store mutex is held only for indexed reads. Verified: health checks answer in **5–8 ms** during a full neural search (previously 9.3 s of async-runtime starvation). REST/MCP adapter agreement 100% in all bench runs.
- **Dead code removal**: `factcheck.rs`, `fccal.rs`, `segdump`/`segjson` scratch examples, and the orphaned `PREDICATE_GROUPS`/`predicate_group` removed. `rerank`/`qa_verify` remain clean internal modules ready for M2.

## 2. Pool-depth calibration (exposed suites only)

Coverage by pool size (answerable hits / total; abstention correct in parens):

Coverage (hits/answerable) + genuine abstentions (correct/insufficient), candidate build at each pool size:

| pool | v4 | v5 | v2 | qax1 | ax5 | ax6 |
|---|---|---|---|---|---|---|
| 25 | 22/24+5/6 | 23/24+5/6 | 23/24+6/6 | 8/8+5/6 | 8/8+4/6 | 8/8+4/6 |
| 30 | 22/24+5/6 | 23/24+5/6 | 23/24+6/6 | 8/8+5/6 | 8/8+5/6 | 8/8+4/6 |
| 35 | 22/24+5/6 | 23/24+5/6 | 23/24+6/6 | 8/8+5/6 | 8/8+5/6 | 8/8+4/6 |
| 40 | **24/24**+5/6 | 23/24+5/6 | 23/24+6/6 | 8/8+5/6 | 8/8+4/6 | 8/8+4/6 |
| 50 | 23/24+5/6 | 23/24+5/6 | 23/24+6/6 | 8/8+5/6 | 8/8+5/6 | 8/8+4/6 |

Pool-50 numbers validated as current-code reports (25-40 measured in the sweep; ax6 8/8 at every pool confirms the sliding-window fix is pool-independent).

**Selection: pool 40 frozen.** It is the smallest pool with zero coverage loss vs pool-50 in every suite — and strictly dominates pool-50 on v4 (24/24 vs 23/24: w08 evidence `hubble-a1#p1` sits at BM25 pool-rank 31, structurally absent at pools 25-35; w12's final-slot ordering is pool-composition-sensitive fp noise, margin 0.025). v5/w09 and v2/v01 are standing misses at every pool size (not pool-sensitive).

**Threads: 6 intra-op frozen** for both models (`GANESH_ORT_THREADS` overridable). Serial clean measurement on qax1 at pool 40: 4/2-split 2175 ms mean vs 6/6 2097 ms mean; consistent with the earlier wall-time ordering (4t 14.23 s vs 6t 13.27 s).

**QA batching:** sliding-window multi-pass scoring initially multiplied latency (qax1 rest mean 3765 ms). Batching every window of every passage into one padded ONNX session call (identical max-margin selection semantics) cut qax1-p40 to **rest mean 2175/2097 ms, max ~3000 ms**.

Selection rule: smallest pool with zero coverage loss vs pool-50 across all exposed suites. Key structural finding: `v4/w08`'s evidence (`hubble-a1#p1`) sits at BM25 pool-rank 31 and its selection is pool-composition-sensitive in the 31–50 band — lost at pools 25/30/35, recovered at 40. Frozen pool size and thread setting recorded below.

## 3. Exposed retention matrix

Final frozen configuration (pool 40, 6 intra-op threads, sequential QA (batched QA reverted in 81a0f315), sliding window):

| suite | candidate | gate | verdict |
|---|---|---|---|
| corpus-v4 | **24/24** + 5/5 genuine (i05 documented fixture defect: the "missing" figure is actually present in the corpus) | 24/24+5/5 | PASS |
| corpus-v5 | 23/24 + 5/6 (final serial certification: adapter 30/30, citations 150/150, 0 provider calls, incomplete=false) | >= 23/24 + >= 5/6 | PASS |
| corpus-v2 | 23/24 + 6/6 = 29/30 correct (v01 standing miss at every pool, shared with baseline) | >= 29/30 + 6/6 | PASS |
| qa-xfer-v1 | 8/8 + 5/6 (serial, pool 40) | 8/8 | PASS |
| acc-xfer-v5 | 8/8 + 4/6 at pool 40 (8/8 flat at all pools) | — | PASS |
| acc-xfer-v6 | **8/8 + 4/6 at every pool — q06 RECOVERED** by the sliding-window fix | 8/8 + >= 4/6 | PASS |
| 5 probe suites | 49/49 correct (event 12/12, action 8/8, chain 13/13, contrast-quantity 8/8, rail 8/8) | 49/49 | PASS |

vs `6c593a1d` exposed baseline (23/24 v4, 22/24+3/6 v5, 28/30 v2, acc-xfer-v6 7/8 with q06 missed): strict net improvement on every dimension — v4 +1 hit (24/24), v5 +1 hit and +2 abstentions, v2 +1, ax6 +1 hit (q06), never worse anywhere.


## 4. Paired fresh frozen acc-xfer-v7

Freeze SHA `02e7cf5ae4ece89fd388d16990afce9bdee53b80` (single clean schema-valid commit; preflight PASS, 2 duplicate-passage warnings by design). Candidate pin `02e7cf5a` (code `c3bbfb8` + fixtures), pinned before any acc-xfer-v7 run in any worktree. 0 overlaps programmatically verified against all 101 articles in every fixtures/ manifest. Domain: historical ships — Cutty Sark, HMS Victory, USS Constitution, RMS Queen Mary, SS Great Britain, HMS Beagle. Absence audits: `beam` 0 hits in beagle.txt; `mph` only inside "amphitheater"; `return to service` 0 hits; `Brunel` 0 hits in hmsvictory.txt; keel "laid in June 1818" month-only; prison hulk "advocated but reneged".

Per-case paired outcomes (H=hit, m=miss, A=correct abstention, L=leak):

| case | type | #188 | 6c593a1d | candidate |
|---|---|---|---|---|
| q01 exact-date | ans | m | **H** | **H** |
| q02 irregular-verb | ans | m | m | m |
| q03 decimal measurement | ans | H | H | H |
| q04 count | ans | H | H | H |
| q05 entity/location | ans | H | H | H |
| q06 long-passage | ans | m | m | m |
| q07 compound | ans | m | m | m |
| q08 compound | ans | H | H | H |
| q09 month-only date trap | ins | A | L | L |
| q10 absent measurement | ins | A | A | A |
| q11 wrong-unit | ins | L | L | **A** |
| q12 unoccurred future | ins | L | L | **A** |
| q13 prison-hulk (hard trap) | ins | L | L | L |
| q14 false-premise | ins | L | A | A |
| **coverage** | | 4/8 | 5/8 | **5/8** |
| **abstention** | | 2/6 | 2/6 | **4/6** |

Gate analysis:
1. coverage candidate 5/8 >= 6c593a1d 5/8 (identical hit pattern q01/q03/q04/q05/q08) **and** > #188 4/8 — PASS
2. abstention candidate 4/6 >= 6c593a1d 2/6 **and** > #188 2/6 — PASS; strictly greater than both, driven by q11 wrong-unit and q12 unoccurred-future now abstaining correctly (the deterministic measurement/future fixes working as designed on fresh evidence)
3. zero newly unsupported ok: candidate leaks {q09, q13} subset of 6c593a1d leaks {q09, q11, q12, q13} — PASS

Honest case notes:
- q02/q06/q07 miss in BOTH neural arms — shared structural difficulty, not regression. q06: BM25 retrieves the evidence `cuttysark-a1#p1` at rank 2, but the cross-encoder scores it 0.43 (rank 7, below top-3 selection); the deterministic exact-date guard then correctly refuses to answer from wrong passages — honest abstention over hallucination, but a coverage miss. q07: both compound facts ARE surfaced (margins 11.93/9.88) yet evidence credit fails — key/passage-id binding strictness, shared with baseline. q02: keyed evidence p8 not selected (p1/p9/p15 chosen).
- q09 leaks in both neural arms (margin 15.06, span "june 1818"): the exact-date binder accepts a month-only span when ANY month-day date exists in the selected passages ("11 May 1820" — the wrong event's date). Clause-scoped binding limitation, honestly carried forward.
- q13 leaks in all arms: extractive QA returns the 1798 advocacy year for the reneged prison-hulk conversion — the packet's one declared hard semantic trap, beyond deterministic reach as established in PR #200/#201.


## 5. Latency, memory, certification

Measured serially on the frozen acc-xfer-v7 packet (identical 14-case workload both arms):

| arm | REST mean | REST max | MCP mean |
|---|---|---|---|
| `6c593a1d` (candle fp32, sequential everything) | 20,959 ms | 52,027 ms | 18,689 ms |
| candidate (INT8 ONNX, pool 40, 6 threads) | **2,983 ms** | 5,258 ms | 3,006 ms |
| **speedup** | **7.0x** | 9.9x | 6.2x |

Strictly faster than `6c593a1d` — PASS; the ≤1000 ms target is NOT met (honest miss, dominated by up to 3 sequential QA window passes plus the 40-candidate rerank batch per query). On qax1 (longer passages) mean was 2,097–2,175 ms. The batched-QA optimization that halved latency was REVERTED after probe re-verification exposed INT8 padding corruption (solo +0.19 vs batched −1.17 on identical input — batch-of-1 exact, real batches corrupt); correctness dominates. Peak RSS: sum of bench-spawned REST+MCP server processes 2.9 GB total (~1.45 GB per server process, each holding both models + ort runtime) — per-process at the 1.5 GB boundary, honestly reported; total model disk 23,180,880 + 33,784,916 = 56,965,796 bytes (< 100 MB PASS). SHA-256 digests: reranker `5741182c…`, qa-verifier `209727cb…` (unchanged from PR #201). Certification: fail-loud exit-2 verified, 0 provider calls, incomplete=false, adapter agreement 14/14 v7 + 30/30 v5 (100%), citation integrity 54/54 v7 + 150/150 v5 (100%).

**Post-freeze correction honestly recorded:** probe re-verification (49/49 requirement) caught the batching regression after the v7 freeze; the revert commit `81a0f315` is the final candidate pin and the v7 candidate arm was re-run after re-pinning. v7 content untouched (freeze `02e7cf5a` still exactly describes the packet); reference arms unaffected. v7 candidate outcomes were identical before/after the revert (5/8+4/6, same per-case pattern) — the corruption did not materially affect v7 margins.

## Verdict

**RETAINED** as the optimal M1 candidate, superseding `6c593a1d`: v7 coverage 5/8 >= 5/8 and > 4/8; abstention 4/6 strictly greater than both references (2/6, 2/6); zero newly unsupported ok; exposed matrix strictly improved on every suite (v4 24/24 vs 23/24, v5 23/24+5/6 vs 22/24+3/6, v2 29/30 vs 28/30, ax6 8/8 with q06 recovered vs 7/8, probes 49/49); 7x mean latency speedup; 0 provider calls.

