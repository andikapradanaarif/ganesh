# Graded sentence-level date selection — partial result, #178 retained

## Four lead answers

1. **Actual top-50 pool ranks**: w01/w02/w03 rank 6, w05 rank 4, w06 rank 6,
   **w08 rank 31 — IN-POOL** (correcting the record across #182/#183/#185; the
   probe printed only 10 of 50 candidates), w11 rank 5, w12 rank 7, w14 rank 3.
   Every v4 wiki miss is selection-side. date-xfer: z01 rank 21, z02 rank 18,
   z04 rank 16, z06 rank 47; only z05 and z07 are genuinely pool-absent.
2. **Curiosity's launch date: still not recovered** (w01 miss), but v4 public
   coverage improved 7/16 -> 8/16 (w05 recovered) **with zero losses** — w07,
   w10, s07 all preserved, no traded hits.
3. **Transfer: flat — 1/8 coverage identical to #178 on date-xfer-v2**, and the
   probe reveals why: the fixture itself cannot discriminate. bm25 returns
   -0.00 for every candidate on the small 2-article corpus — when query terms
   appear in most passages of a tiny corpus, FTS5 inverse document frequency
   collapses and ranking degenerates to insertion order. The transfer group
   measures corpus-scale artifacts, not selector quality. Additionally u05
   shows a vocabulary-mismatch false abstain ("lift off" never appears in a
   source that says "launched").
4. **Cost on existing controls: negative cost — v2 improved 27/30 -> 28/30**,
   v30 correctly insufficient, v04/v05/v06 hits intact, event 12/12, action
   8/8, chain 13/13, quantity 8/8, zero new unsupported-ok anywhere, adapter
   agreement 30/30, five new unit tests pass.

## Verdict: partial result. v4 improves without demonstrated transfer; #178 remains the recommended behavior.

Per the decision rule, improvement on exposed v4 alone is insufficient. The
exact losing mechanism on transfer is identified and is not the selector:
bm25 degeneracy at small corpus scale. This is a fixture-design finding for
any future transfer group — groups need enough documents that IDF is
meaningful, or the evaluation measures nothing about ranking.

## What the iteration establishes

- The graded signal works where ranking is non-degenerate: v4 gained w05 at
  zero cost, the first selection improvement in four iterations that traded
  nothing away. w01 specifically remains blocked because junk passages tying
  launch+year score close to the correct passage and outrank it within the
  single promoted slot.
- Pool-rank correction: all v4 wiki misses are in-pool (w08 at rank 31); the
  problem is entirely selection. The factual-abstention gap (u09/z08/i04
  class) remains the open acceptance-side follow-up — even perfect selection
  cannot pass M1 without it.
- Fixture hygiene fixed: real pageids/revisions (Sputnik 1: 28484/1387871662,
  Sputnik 2: 81978/1384930248), clean plaintext, distinct keyed passages,
  preflight PASS.

## Reproduction

Freeze `c12a1977`; candidate pin `6c76978c`; baseline #178 `6b06393c`.
`GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 ganesh-bench run --questions fixtures/date-xfer-v2/key.json --corpus fixtures/date-xfer-v2 --mode dev`
Zero provider calls; deterministic replay verified.
