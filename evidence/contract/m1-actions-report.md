# Recorded actions vs verified outcomes — development report (issue #169)

**Explicit verdict: candidate warrants fresh M1 evaluation** — all stop-rule conditions met. v3 remains unconsumed; execution, merge and M2 are owner decisions.

Comparison base: `7959f1f0a0aa8730bf5c5c0a2fd08a087a881050` (PR #168). Ranking, top-three, corpus, citations, quantity handling fixed; zero provider calls throughout. Baseline reproduced before changes: chain-probe unsupported-ok exactly {c04, c07, x05}, sharing one cause — positive support was verify-class-agnostic, so a *start* event counted as evidence for completion/outcome questions.

## What a receiving agent can now safely conclude

- **started** = work began, nothing more. Finish/completion questions abstain (c04, x05, e04). Completion never follows from a start event.
- **finished/completed** = work done, in either wording — finish/complete are justified equivalents (e02). Completion directionally entails the action started, never the reverse.
- **explicitly did not finish / was not run** = the negative answer, delivered with the passage — settles the asked question directly (t02, e03, e07).
- **was not run** ≠ failed ≠ passed: outcome questions (did it pass?) stay unresolved when the activity never happened (c07, e06); "the integrity check failed" settles the pass question negatively through outcome antonymy (c05).
- **verified** requires an observation, not an event: deployment-after-checklist is not post-deployment verification (qB05 benchmark correction in key.v3); "the check confirmed running" is.

## The bounded candidate

`GANESH_ACTION_CHECK=1` on top of the #168 configuration: positive support requires an action-exact or justified-equivalent predicate for the same subject and scope. Documented inference: direction finish→start only; counterexample where it must abstain — "the export finished" cannot answer whether a *different* export or date started, and a start never answers a finish. Contradiction stays predicate-exact, extended with outcome antonymy (failed settles did-it-pass). Documented limitation: finish vocabulary must now engage the analysis, so cue detection covers finish/fail/begin/reach — general event vocabulary, no fixture-specific terms.

## Paired outcomes (pinned reference → candidate)

| Input | pin total | candidate total | unsup-ok pin→cand | false-abstain |
|---|---|---|---|---|
| Meridian transfer-check (8) | 8/8 | 8/8 | 0→0 | 0 |
| chain-probe (13) | 10/13 | **13/13** | 3→**0** | 0 |
| action-probe transfer (8) | 4/8 | **7/8** | 4→**1** | 0 |
| quantity controls (8) | 8/8 | 8/8 | 0→0 | 0 |
| transition v3 (12) | 8/12 | 10/12 | 3→1 | 1 |
| exposed v2 (30) | 27/30 | 27/30 | 0→0 | 1 |

Changed decisions: c04/c07/x05 leak→correct abstention (the three target failures); e04/e05/e06 leak→correct abstention on the frozen export transfer group; qB05/qB07b fixed on the corrected v3 key. No regressions anywhere: every previously correct decision preserved. REST/MCP agreement true on all inputs; citation opening verified for affirmative (t01), explicit negative (t02/t04) and unknown (t03) representatives.

## Stop-rule evaluation

- correctly abstains c04/c07/x05: **yes**
- preserves Meridian 8/8, quantity controls, heading-boundary (c08) and ambiguous-subject (c03) protections: **yes**
- improves end-to-end correctness on the frozen transfer group with no new unsupported ok: **yes** — 4/8→7/8; the one remaining leak (e08) pre-existed in the pinned reference
- valid citations, correct decisive traces, REST/MCP agreement: **yes** — t01's trace relies on the completion sentence chained to its V12 antecedent under the April 3 heading; t02/t04 rely on the explicit-negative sentences; traces and citations resolve to pinned revisions

**The candidate warrants a fresh M1 evaluation. v3 is not consumed by this issue.**

## Smallest surviving counterexample

e08: "The export of dataset D7 started at 09:00 UTC. **The validation finished at 09:30 UTC** while the export continued." Chained single-subject context lets the validation's finish stand in for the export's finish — same passage, different events, lexically indistinguishable subjects-of-events. This is the next evidence-interpretation problem (event subjecthood within chained entries), preserved frozen in the action-probe group for whoever continues.

## Reproduce

```
cargo build --release --workspace
# candidate
env GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 \
  cargo run --release -p ganesh-bench -- run --questions fixtures/action-probe/key.json \
  --corpus fixtures/action-probe --report out/ap.md --details out/ap.json --mode dev
# pinned reference: drop GANESH_ACTION_CHECK=1
```

Costs: ~2s per fixture run, ~1.8s per 30-question v2 run, zero provider calls, zero new dependencies. Fixture digests in `fixtures/action-probe/manifest.json` (CC0 synthetic). Bench JSON details copied under `evidence/contract/`.
