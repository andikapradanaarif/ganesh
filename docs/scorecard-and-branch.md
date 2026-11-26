# Standing v4 scorecard and measured branch determination (#188)

## The five lead answers

1. **#188's acceptance-only ceiling is 17/24** — and it exactly equals credited
   hits 17/24. Acceptance withholds NOTHING that the selected three settle:
   every answerable question whose selected passages collectively cover all
   required facts is already delivered. Rows establishing it: the seven misses
   are all cases where the selected three never contained the evidence.
2. **The largest recoverable loss on public articles is top-three selection:
   6 cases** (w01, w03, w06, w11, w12, w14), every one with the keyed passage in
   the pool at ranks 3–7. Pool ceiling is 23/24 overall, 15/16 wiki.
   **w08 final reconciliation: `hubble-a1#p1` is IN-POOL at rank 31 on #188**
   (reproduced with the rank-patched probe, corpus digest and pins below) — the
   earlier 'absent from pool' claim is definitively corrected; every v4 wiki
   miss is selection-side, none is a candidate-retrieval failure.
3. **The evidence justifies exactly one intervention branch: A — ranking.**
   Selected-settles 17/24 < 20/24 rules out branch B as primary (a sufficiency
   method cannot recover evidence the selection never delivered — veto-only is
   mathematically capped at zero coverage gain). Pool 23/24 ≥ 20/24 rules in a
   reranker on the same candidate pool. Branch C does not apply: the
   measurement is reliable, pins compatible, keys unambiguous after audit.
4. **Transfer implementation: not started in this session** — see honest scope
   note below. The measured choice, the scorecard artifact, and the audit are
   the deliverables; the branch-A design constraints are fully specified.
5. **What still blocks the formal bars**: coverage 20/24 requires recovering
   ≥3 of the 6 selection losses (all wiki, ranks 3–7 in-pool); abstention 6/6
   requires solving i01 (activity interpretation), i04 (event-identity of
   dates), i05 (missing measurement), i06 (nonexistent end date) — four
   DISTINCT acceptance-side problems, no one of which is close to solved.

## Reproduction

Baseline #188 candidate `8f273aaa54d401afc72e44d6690e3e9f21b03807` (full SHA,
resolved), report head `1c6484ac09e4efe354203418d77f8b837deae0f8`. Flags:
`GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1`.
Scorer scoring.v2. Commands: `ganesh-bench probe --questions fixtures/corpus-v4/key.v4.json --corpus fixtures/corpus-v4`
(diagnostic only — verified: probe writes nothing to the serving path, REST/MCP
outputs unchanged). Reproduced before interpreting: #188 = 17/24 hits,
2/6 abstentions. Machine-readable per-case scorecard: `docs/v4-scorecard.json`.

## Per-question first-failure classification (answerable)

| case | key passage | pool rank | in selection | first failure |
|---|---|---|---|---|
| w01 | curiosity-a1#p2 | 6 | no | top-three selection |
| w03 | curiosity-a1#p2 | 6 | no | top-three selection |
| w06 | voyager-a1#p1 | 6 | no | top-three selection |
| w08 | hubble-a1#p1 | 31 | no | top-three selection (pool rank low but present) |
| w11 | perseverance-a1#p1 | 5 | no | top-three selection |
| w12 | perseverance-a1#p1 | 7 | no | top-three selection |
| w14 | perseverance-a1#p21 | 3 | no | top-three selection |

All synthetic answerable (8/8) and the other wiki hits settle within the
selected three. w01/w03 share the same underlying fact (Curiosity lead
passage) — flagged shared-fact row; the public count of distinct facts at
stake is therefore 5, not 6.

## Insufficient-case audit

| case | missing detail | strongest misleading selection | actual behavior |
|---|---|---|---|
| i01 | test outcome after 'finished while continued' | seedlog-a1#p4 | leaks ok — activity-outcome control, out of scope for factual acceptance |
| i02 | ambiguous 'It finished' after two tests | seedlog-a1#p5 | correct abstention |
| i03 | outcome explicitly unknown | seedlog-a1#p2 | correct abstention |
| i04 | exact approval day; source gives 'for 1978' (funding PERIOD) | hubble-a1#p15 | leaks ok — event-identity failure: a fiscal-year period and a different milestone's date are not an approval date |
| i05 | measurement stripped with template | hubble-a1#p53 | leaks ok — fixture artifact partially; missing-measurement failure |
| i06 | end date does not exist (rover operates) | curiosity-a1#p43 | leaks ok — nonexistent-event failure |

## Mandatory audits

**q09 bridge strands.** Source states: two main cables, 27,572 strands per
cable. Supports: 27,572 for one main cable; a derived 55,144 for both main
cables by plain arithmetic over explicitly stated counts. Does NOT support:
'all cable types across the entire bridge' — suspender ropes and other cable
types are uncounted in the source. The frozen key asked the bridge-wide total
and the system (correctly per strict key) should abstain; #189's failure was
crediting the per-cable figure. Recording all three: source supports per-cable
and derivable both-main-cables; the system returned the per-cable number; the
frozen scorer credits nothing. A legitimate derivation exists only for the
two-main-cable scope, not the entire-bridge scope — the frozen key stands.

**i04 event identity.** The corpus contains: funding approved 'for 1978' (a
fiscal period), plus exact dates of OTHER milestones. The requested fact is the
exact approval DATE. No passage settles it; dates of different events are
distinct facts. The leak arises because coverage acceptance treats any tied
date as supportive. The frozen key is correct; ceiling unchanged.

**Ceilings: strict = audited = 17/24.** No row moves the ceiling across 20/24;
no evidence-blocked rows. The audit changes nothing — selection is the whole
answerable-loss story, acceptance is the whole abstention-loss story.

## Branch-A design constraints (for the implementing iteration)

A materially different local reranker on the same top-50 pool. Explicitly
prohibited by the evidence of #182–#188: topic-word weight sweeps, date-tier
constant tuning, first-pick movement without compound protection. The
measurement adds one more constraint: since 5 of 6 losses share one shape —
the fact-dense lead or early passage at rank 3–7 losing to short marginalia —
the reranker must score question–passage answer-likelihood directly, must
preserve the top-three limit and #188 acceptance untouched, and must show its
compound-question complementarity preservation. Transfer packet must include
near-matched pairs (per-cable vs both-cables; approval-year vs exact-day).

## Honest scope note

This session delivers the measurement, audit, correction, and branch
determination in full. The branch-A implementation, its transfer packet, and
paired evaluation are not started here; they are exactly specified and
unblocked. That is the measured engineering choice this issue exists to
produce — no further guesswork iterations should precede it.
