# Best-sentence concentration reranker — STOP verdict, #188 retained

## Verdict: stop this ranking experiment; #188 remains the recommended build.

The candidate fails the retention rule on two of three required conditions:
no improvement on exposed v4, and one newly unsupported ok (v30) on exposed v2.

## Paired results (baseline #188 `8f273aaa`, candidate `dee4ab80`)

| Fixture | #188 | candidate | Movement |
|---|---|---|---|
| v4 coverage | 17/24 (wiki 9/16) | 17/24 (wiki 9/16) | **zero — byte-identical selections** |
| v4 abstention | 2/6 | 2/6 | none |
| rr-xfer end-to-end | 2/8 | **4/8** | +2 (r03 hit gained, r08 leak fixed) |
| rr-xfer coverage | 1/6 | 2/6 | +1 |
| exposed v2 | 28/30 | **27/30, v30 newly unsupported-ok** | **−1 + leak** |
| dx3 controls | 5/12 | 5/12 | none |
| event/action/chain/quantity | pass | pass | none |
| compound v04/v05/v06 | hit | hit | preserved |

## The smallest defeating counterexample: v30

"How many operators does the C language define?" — an insufficient control
that every iteration since #182 has used as the canary. The wh-activation rule
(`how`-initial questions) pulled quantity questions into the concentration
reranker; a passage dense with operator discussion scores high on
sentence-level term concentration because C documentation constantly mentions
operators in single sentences — but no sentence states a count. Selection
changed, coverage-only acceptance credited the denser evidence, and the
correct abstention became an unsupported ok. This is exactly the failure the
method specification warned against: topical concentration is not answer
presence, and for quantity questions the distinction is sharpest.

## What the transfer result shows (recorded honestly, not counted toward retention)

The same mechanism that broke v30 genuinely helped on unfamiliar documents:
r03 (Brooklyn Bridge main-span length) became a hit — the fact-bearing
sentence concentrates property terms — and r08 (bridge-wide wire total,
absent) stopped leaking because the promoted passages no longer echoed the
wrong scope. End-to-end transfer correctness doubled 2/8 → 4/8. But the
decision rule is structural: a selector that cannot tell concentrated topical
discussion from concentrated factual statements will leak exactly where the
insufficient controls are hardest. The negative v4 result additionally shows
the mechanism does not even move the target metric — every v4 selection was
byte-identical to #188 because the winning concentrations tie with what bm25
already picks.

## Operational verification

Raw and eligible pools identical across arms by construction (pool generation
untouched); REST/MCP agreement 30/30 on v4; deterministic replay verified;
latency unchanged (~50ms/query); zero provider calls; no model, no external
weights — pure local Rust.

## Fixture integrity note

The rr-xfer key was lost in an authoring-time crash before any outcome was
observed and regenerated deterministically from the frozen snapshots; the
regeneration is documented in the branch history. Preflight PASS (2 warnings,
both the intentional shared lead passage). Freeze commits: packet `b103e6f`
+ fixups; candidate pin `dee4ab80e42b491053c1780432398f15377b9235`.

## What this closes and what remains

This completes the ranking experiment line authorized by the scorecard: five
selection interventions (#182, #183, #185, #186+#188, and now sentence
concentration) have bounded the design space. Exactly one selector ever
improved v4 without cost: the graded date path of #188, whose gains are
real but small (wiki 7/16 → 9/16). The remaining six selection losses are
not term-visible in any formulation tried — they require distinguishing
answer-bearing sentences from topic-bearing sentences with a mechanism
class none of these iterations could express. The four abstention problems
(i01/i04/i05/i06) remain untouched and are independent of selection.

Replay: `GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 ganesh-bench run --questions fixtures/rr-xfer/key.json --corpus fixtures/rr-xfer --mode dev`
