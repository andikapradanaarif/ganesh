# M1 v3 fresh paired evaluation — final report (issue #171)

## 1. Did the candidate pass M1 on v3? **FAIL** — for both arms.

| Gate | Required | Reference (#168) | Candidate (#170) |
|---|---|---|---|
| Sufficient evidence | ≥20/24 | **21/24 PASS** | **21/24 PASS** |
| Insufficient evidence | 6/6 | 3/6 FAIL | 5/6 **FAIL** |
| Adapter agreement | 30/30 | 30/30 PASS | 30/30 PASS |
| Citation integrity | all open | 150/150 PASS | 138/138 PASS |
| Reproducibility/cost | replay, 0 provider | PASS, 0 calls | PASS, 0 calls |

The candidate fails the absolute abstention subcheck by one case (i03). First results preserved as-run in `evidence/contract/v3-*.json/md`.

## 2. What did PR #170 change on the same questions?

- **Improved: i01, i02** — unsupported-ok → correct abstention. i01: satellite K4's downlink was acquired but its outcome never recorded; the reference accepted the acquisition as completion evidence, the candidate refuses because a start/acquisition event cannot prove completion. i02: two satellites (K3, K4) in one entry with one undifferentiated completion sentence; the candidate refuses ambiguous-subject inheritance.
- **Regressed: none.** Zero regressions across all 30 questions.
- **Unchanged wrong in both:** i03 (both leak — see diagnosis), w09 (wrong-passage evidence), w10/w15 (false abstentions on Lua quantity/year lookups).
- Net effect on the abstention subcheck: 3/6 → 5/6. Coverage identical at 21/24; per-question end-to-end evidence correctness identical at 21/24; latency identical within noise (~8–14 ms/query, zero provider calls).

## Execution record

- Pins: reference `7959f1f0a0aa8730bf5c5c0a2fd08a087a881050` (flags `GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1`); candidate `72bfb11bbdf1dee24838e8c6ea082252a021fbe3` (same + `GANESH_ACTION_CHECK=1`). Separate clean worktree builds, `scoring.v2` contract for both.
- Freeze commit: `ce6dc3f34d802b83b5f10a72d747242a033e53a6` (branch `feat/171-m1-v3-evaluation`); key digest `0a9adf7b45f09dc6`; corpus 6 articles / 358 passages / 98,354 bytes; preflight action-probe reproduced 4/8 vs 7/8 exactly before v3 opened.
- **Key defect recorded honestly:** the first freeze keyed wiki acceptable-passage IDs from a hand replication of the segmentation spec that missed blank-line paragraph splitting. The defect was detected at the first run (systematic ID mismatch, 6/24), no outcome-dependent content changed; IDs were corrected mechanically from frozen corpus bytes using the exact frozen spec (`segment.rs`), all spans/questions/labels untouched, and both arms re-run as a deterministic replay. The synthetic-material IDs had been correct throughout, confirming the diagnosis.
- Corpus composition: 18 answerable wiki questions on fresh families (Perl, Ruby, Lua, Kotlin — seed 20261127 over TIOBE top-20 disjoint from v1/v2), 6 answerable + 4 insufficient on labeled synthetic operational logs (satellite ground station, lighthouse supply vessel — genuinely distinct domains, not renames), 2 insufficient wiki traps. Synthetic contribution: 6/24 answerable, 4/6 insufficient — reported separately as required.

## Smallest reproducible diagnosis (the FAIL)

**i03**: *"The telemetry verification was not performed."* — question: did it pass? Both arms leak. Chain-probe c07 ("was not run") abstains correctly because `run` is a cue stem the negation attaches to; `performed` is not in the cue lexicon, so the negation never binds to any cue — the sentence reads as an unnegated verify-class mention. The remaining gap is lexical coverage of negated activity verbs, one step deeper than the action support fixed in #170. Reproduce:

```
cd .local/wt/v3-cand && env GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 \
  cargo run --release -p ganesh-bench -- run --questions fixtures/corpus-v3/key.v3.json --corpus fixtures/corpus-v3 --report out.md --details out.json
```

The w10/w15 false abstentions (Lua quantity/year ordinary lookups) are a separate coverage-side item for a later issue; e08 (same-passage different events) remains the known chained-context limitation, reported alongside as required.

## Decision

FAIL → smallest reproducible diagnosis above. v3 outcomes are now exposed development evidence; any fix informed by them belongs to a later development issue, and further fresh evaluation would require a v4 packet. This 30-question result supports a bounded engineering decision only. Merge and M2 remain owner decisions.
