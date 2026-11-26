# Date-aware selection experiment — negative result, #178 retained

## Three lead answers

1. **Did Ganesh recover the Curiosity launch date? No.** w01 remains a miss.
2. **Did it distinguish the requested event from nearby dated events on
   unfamiliar documents? No measurable difference** — date-xfer outcomes are
   identical across #178, #182, and the candidate (2/9, coverage 1/7).
3. **What did it cost? Nothing — and that is the one positive finding.** Every
   control preserved exactly: v2 27/30 with v30 correctly insufficient, v04/05/06
   hits intact, event 12/12, action 8/8, chain 13/13, quantity 8/8, w07 hit
   preserved, zero new unsupported-ok, adapter agreement 30/30, identical pools.

## Paired results

| Fixture | #178 | #182 | candidate `13136b4c` |
|---|---|---|---|
| v4 coverage | 15/24 (wiki 7/16) | 18/24 | 15/24 (wiki 7/16) |
| date-xfer coverage | 1/7 | 1/7 | 1/7 |
| exposed v2 | 27/30 | 24/30 | 27/30 |
| unsupported-ok | baseline set | +v30 | baseline set only |

v4 case movements: w06 miss->hit, w10 hit->miss — a wash. z08 leaks in all three
arms equally (pre-existing acceptance-layer behavior: coverage-only acceptance
cannot tell the exact day is absent — the documented factual-abstention gap,
visible here exactly as predicted).

## Why the signal did not move outcomes

1. **Binary signal + bm25-order tiebreak lets junk win anyway.** For w01 the
   insertion scans eligible candidates in bm25 order and promotes the first
   signal-positive passage. Junk passages that merely mention launch plus a year
   (the music-video passage references the launch) tie the correct passage
   binary-wise and outrank it. The correct passage p2 never gets the slot.
2. **Deep correct passages are pool-bound.** z01/z05 key passages sit at ap11
   p49/p103 — deep in a large article; several never enter the bm25 top-50 pool
   at all, joining w08 as pool-side failures a selector cannot fix.
3. Slot-2-only insertion leaves the structure of #178 intact — which is exactly
   why it cost nothing, but also why one slot cannot overcome rank-1 junk plus a
   binary-tied signal.

## Decision: keep #178. The negative result is preserved with its mechanism.

What the three iterations now establish: within-article selection cannot be fixed
by term coverage (#182/#183) or by binary date-event tying (#184). The traces
point at two remaining honest directions, both out of scope here: (a) graded
answer-typed scoring where exactness and event-proximity are weighted rather
than binary, which requires care not to reopen the v30-class leak; (b) the
candidate-pool itself, which binds several cases (w08, z01, z05) before any
selector runs. The factual-abstention gap (z08/i04 class) remains the open
acceptance-side follow-up, grounded in the matched pair z01 (date present) vs
z08 (topic present, exact day absent).

## Reproduction

`GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 ganesh-bench run --questions fixtures/date-xfer/key.json --corpus fixtures/date-xfer --mode dev`
Freeze `56f12f42`; candidate pin `13136b4c`; baseline #178 `6b06393c`. Zero
provider calls throughout; deterministic replay verified.
