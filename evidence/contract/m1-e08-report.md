# Resolving the original e08 failure (issue #177)

## What evidence now lets Ganesh tell a receiving agent that the export itself finished?

Only evidence where the export's own completion is stated — in a clause that names the export, or in an unambiguous continuation of an entry that introduces the export alone. The invariant now enforced: **a claimed outcome must belong to the asked activity, explicitly or through unambiguous continuation; sharing a dataset ID, date, or passage is never sufficient, and when a sentence's own text places the queried activity in a clause that carries no outcome cue — "while the export continued" — inherited context cannot override that attribution.** The validation's finish stays the validation's.

## Corrected #176 record

PR #176 verdict corrected to *useful partial fix; v4 not yet warranted* (comment posted). Positive results preserved: event-probe 8/12→12/12, a08 fixed, continuation controls held. Transfer-group record corrected: the 12-case event-probe was the only group frozen before candidate development — no separate transfer group existed.

## The exact accepting path on e08 (traced on pinned #176)

Passage exportlog-a1#p7, sentence "The validation finished at 09:30 UTC while the export continued." Activity terms: `export` is a **subject** term — filtered out of the object set, which held only `[7,d7,dataset,may]`. Values on the accepting path: `cue_in_own=true` ("finished" present), `cue_clause_has_obj=false` (the validation clause names neither dataset nor D7), `obj_only_elsewhere=false` — vacuously, because no object term appears in *either* clause; they arrive only through chaining. The branch `cue_clause_has_obj || !obj_only_elsewhere` therefore passed with **no affirmative link between the export and completion** — the hypothesized empty-eligible-object bypass, confirmed by controlled variation.

## The bounded fix

For multi-clause sentences, activity terms now include subject terms: if the sentence's own text places any question activity term in a clause carrying no outcome cue for the asked predicate, support is rejected. Pure continuations (the activity absent from every clause) still inherit — Meridian t01/t02 preserved. The comma-split that enables inverted while-clauses is restricted to while-constructions after it caught the introductory phrase in "In release 4.0, tail-call optimization became optional" (t07 regression found and fixed within this iteration).

## Paired outcomes — pinned #176 → candidate (identical passages, zero provider calls)

| Fixture | pinned #176 | candidate |
|---|---|---|
| action-probe (contains original e08) | 7/8, leak e08 | **8/8 — e08 correct** |
| event-probe | 12/12 | 12/12 |
| rail-probe (frozen transfer group) | 8/8 | 8/8 |
| activity-probe | 8/10 | 8/10 (a03/a06 pre-existing false abstentions) |
| chain-probe | 13/13 | 13/13 |
| transfer-check (Meridian) | 8/8 | 8/8 |
| exposed v3 | 27/30 | 27/30, zero unsupported-ok |
| quantity | 8/8 | 8/8 |

Changed decisions: e08 unsupported-ok → correct abstention; t07 false-abstention introduced by the first comma-split iteration was found and fixed before this record. **No regressions in the final candidate: every previously correct outcome in every fixture retained.** REST/MCP agreement, citations, deterministic replay, zero provider calls hold throughout (~8-14ms/query unchanged).

## Promotion rule — applied exactly

1. Both original failures resolved: **yes** — a08 stays correct, e08 becomes correct, mechanism verified in the traces above.
2. Direct completion, explicit negatives, valid continuations, later explicit outcomes, subject swaps, ambiguous continuations: **all correct** (event-probe 12/12, rail-probe 8/8).
3. Improvement on the frozen transfer group: **cannot be established — the pinned reference already scores 8/8 perfectly on rail-probe** (freeze commit `3e84326`, cases r01–r08, paired outcomes 8/8 = 8/8). The group was frozen before implementation and not rewritten after outcomes were seen.
4. All controls retained: **yes**.

**Verdict: useful fix with verified mechanism; v4 recommendation is not earned through this iteration** — the promotion rule requires demonstrated improvement on the frozen transfer group, which its ceiling makes impossible, and the rule does not permit successive transfer groups. The owner may judge whether the verified e08 mechanism plus regression-free controls justify a v4 packet on other grounds; the evidence is all here. v4 remains unconsumed. Merge and M2 remain owner decisions.

## Remaining limitations

a03/a06 (cancelled/never-took-place phrasing false-abstains), w10/w15 (retrieval-side Lua misses), and the general depth ceiling of lexical clause analysis. Reproduce: `env GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 cargo run --release -p ganesh-bench -- run --questions fixtures/action-probe/key.json --corpus fixtures/action-probe --report out.md --details out.json --mode dev`; pinned reference at commit `ef6c5086ee4dc26b758ae35f6683d3ac4a54b2d0` with identical flags.
