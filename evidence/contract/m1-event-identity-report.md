# Event identity: which activity owns an outcome (issue #175)

**Verdict: the clause-level candidate satisfies every development condition. Recommend v4 fresh evaluation — not executed, left for owner decision.**

Comparison base: PR #174 reviewed at `12d172a310d961fe87f1ab7e8a463b4d22d2fb71`. Attribution corrected first (PR #174 comment): exposed-v3 rows are #168 = 24/30, #170 = 26/30, #174 revision = 27/30; no 24/30 run ever occurred under a #170 configuration — that figure was #168 mislabeled. As-run evidence preserved.

## The product answer

A receiving agent can now tell, for "the inspection finished while the repair continued": the inspection finished (the finish cue shares its clause with the inspection), the repair was still underway at that moment (the repair clause says continued), and the repair's eventual completion stays unknown unless a later entry records it. Clause-level event identity holds across clause order, subject swap, and competing activities — while valid single-subject continuations and later explicit outcomes still resolve.

## The candidate

Smallest approach justified by the traces: clause-level binding, no model. Sentences split on `;`, `while` (either capitalization, either position), and the comma that separates inverted while-clauses. Positive action support requires the queried activity's object terms to share the clause carrying the outcome cue; a competing clause cannot donate its outcome. Single-clause and chained continuation sentences are unaffected, so the Meridian/restore-log fixes are preserved. Continuation chaining is NOT disabled — v11/v12 (valid continuation, later-outcome settlement) pass, proving the control.

## Paired results (pinned #174 → clause candidate, zero provider calls)

| Fixture | #174 | candidate | unsupported-ok |
|---|---|---|---|
| event-probe (frozen pre-implementation) | 8/12, leaks v02/v05/v07 | **12/12** | 3→**0** |
| activity-probe | 7/10, leak a08 | 8/10 | 1→**0** (a08 fixed) |
| chain-probe | 13/13 | 13/13 | 0→0 |
| transfer-check (Meridian) | 8/8 | 8/8 | 0→0 |
| exposed v3 | 27/30 | 27/30 | 0→0 |
| action-probe | 7/8, leak e08 | 7/8, leak e08 | unchanged |

Changed decisions: v02/v05/v07 leak→correct abstention; v03 false-abstention→correct (continue/underway cues); a08 leak→correct abstention. No regressions anywhere: every previously correct decision in every fixture preserved, including all v3 coverage hits and correct abstentions. REST/MCP agreement and citation integrity hold on all inputs (adapter 12/12, citations 54/54 on event-probe); deterministic replay; zero provider calls; ~8-14ms/query unchanged.

## Decision-rule evaluation (carried from issue #173)

- i03 fixed with correct traces, v3 coverage hits and abstentions preserved, no new unsupported-ok: **yes**
- improves end-to-end correctness on the frozen development fixture with no new unsupported-ok: **yes** — 8/12 → 12/12
- competing-activity control handled correctly: **yes** — including reversed clause order and subject swap
- original e08 reported separately: **still leaks** — its validation clause never mentions the export, so no clause carries both the cue and the activity; the remaining mechanism involves subject-term inheritance through chaining and is the documented next problem
- pre-freeze validator run before implementation: yes — preflight PASS with only legitimate same-passage warnings

**All conditions met: recommend v4. Execution, merge, and M2 remain owner decisions.**

## Remaining limitations

1. e08-original: outcome borrowing when the owning activity is entirely absent from the outcome clause (inheritance-mediated, not clause-mediated).
2. a03/a06: "cancelled before execution"/"never took place" should settle occurrence questions negatively — activity-phrasing equivalence still open.
3. w10/w15: false abstentions on ordinary Lua quantity/year lookups — retrieval-side, out of acceptance scope.

Reproduce: `env GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 cargo run --release -p ganesh-bench -- run --questions fixtures/event-probe/key.json --corpus fixtures/event-probe --report out.md --details out.json --mode dev` (drop flags progressively for earlier arms; pinned #174 = same flags at commit 12d172a).
