# Issue #181 — selection experiment report (development, exposed v4)

## Lead results

1. **How many of the 24 could the original pool answer in three passages? 23/24
   overall, 15/16 wiki.** The candidate pool (bm25 top-50) almost always contains
   the keyed passages; the failure is top-three selection. Only w08 (`hubble-a1#p1`)
   never enters the pool — the single true candidate-retrieval failure.
2. **Where were the nine public-article answers lost?** Eight of nine at top-three
   selection: the keyed passage sits in the pool at ranks 3–7 but the complementary
   selector prefers short passages dense in repeated topic words (w01: correct
   `curiosity-a1#p2` at rank 6 loses to p19/p86/p32 — mass-at-launch, a music video,
   bacterial spores). One (w08) never becomes a candidate.
3. **Did the intervention help on unfamiliar documents? Yes but not enough to
   retain it.** Body-coverage selection improved transfer coverage 8/9 → 9/9 with
   no new unsupported-ok there, and exposed-v4 coverage 15/24 → 18/24 — but it
   regressed exposed-v2 controls 27/30 → 24/30 and introduced one new unsupported
   ok (v30). Per the decision rule the pinned baseline remains the recommended
   behavior. **Negative result, retained as evidence.**

## Pins and reproduction

| Item | Value |
|---|---|
| Reference | `6b06393c` (PR #178) — pre-change behavior |
| Candidate | this branch + body-coverage selection (uncommitted experiment, committed with this report) |
| Flags | `GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1` |
| v4 freeze | `996b7a5d` (exposed development evidence) |
| Transfer freeze | `256de1b` — Eiffel Tower + Marie Curie, 11 cases, preflight PASS |
| Replay | `ganesh-bench run --questions <key> --corpus <dir> --mode dev` |

## Mechanism diagnosis

The stemmed index text injects article title and section terms into every
passage, so within one article distinct-term coverage ties across all passages
and raw bm25 length/frequency noise decides. Short junk passages (118–175 chars)
outrank the longer fact-dense lead (282 chars) even though the lead covers more
distinct query terms in its body. First implementation attempt (coverage ordering
on title-injected index text) confirmed the tie: no v4 movement at all.
Body-only coverage breaks the tie toward fact-dense passages — validated on v4
and transfer — but the same signal misfires on the v2 controls.

## Paired measurements (reference → candidate)

| Fixture | Reference | Candidate | Verdict |
|---|---|---|---|
| v4 coverage | 15/24 (wiki 7/16) | **18/24 (wiki 10/16)** | +3 |
| v4 unsupported-ok | i01,i04,i05,i06 | same 4, none new | neutral |
| transfer coverage | 8/9 | **9/9** | +1, i22 leak pre-existing in both |
| exposed v2 | 27/30 | **24/30** | **−3, v30 newly unsupported-ok** |
| action-probe | 8/8 | 8/8 | preserved |
| chain-probe | 13/13 | 13/13 | preserved |
| rail transfer-check | 8/8 | 8/8 | preserved |
| event-probe | 12/12 | 12/12 | preserved |
| quantity contrast | 8/8 | 8/8 | preserved |

REST/MCP agreement 30/30 on v4; deterministic replay verified (statuses and
ordered passages byte-identical across repeat runs); zero provider calls.

### v2 paired detail

- v04/v05/v06: lost hits — ok but wrong passages. v04 is a compound question
  (reference implementation of Python + its implementation language): the
  body-coverage first pick is a single-fact passage and complementarity never
  recovers the second facet.
- v19: gained hit (improvement).
- v30 ("How many operators does C define", insufficient control): newly leaks —
  body-dense passages about C operators now selected; coverage-only acceptance
  cannot tell the count is absent. This is the documented acceptance-side
  verification gap, triggered by the new selection.

## Why the baseline stays

The rule: improvement must come without new unsupported-ok responses. v30 is a
new unsupported answer, and the exposed-v2 control regression (−3) shows the
mechanism trades compound-question complementarity away. The v4/transfer gains
are real but inseparable from these costs in this iteration's single change.

## Next mechanism supported by the traces

1. Complementarity-preserving coverage: keep the bm25 first pick (protects
   compound questions) and apply body-coverage ordering only inside the marginal
   complementarity walk, so facet-diverse passages still win over topic-word-dense
   duplicates without disturbing multi-fact assembly.
2. w08 is the one true candidate-retrieval failure (key passage never in pool);
   a separate bounded candidate-retrieval issue could address it.
3. The acceptance-side verification gap for ordinary facts (v30 class) is the
   follow-up issue — grounding pair: w01-class where one source supplies the
   requested detail and a near neighbour only mentions the topic.

## Cost/compliance

Zero provider calls. Deterministic replay and REST/MCP agreement verified per
run (see final summary). Query latency unchanged (~50 ms on the v4 corpus).
