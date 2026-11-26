# M1 evidence-acceptance experiments — development report (issue #163)

**Final status: development stopped before fresh evaluation** — the best candidate
fails screening criterion 1. Per the issue protocol this is a complete outcome;
no fresh held-out set was consumed.

## Starting point

- Branch `feat/163-m1-evidence-acceptance` stacked on unmerged #162 tip `4831aab`.
- Behavioral reference reproduced exactly before any change: exposed-v2 replay of
  the refactor (flags off) yields byte-identical outcomes to revised pin `e2bd444`
  (coverage 21/24, insufficient 2/6).
- v1/v2 freezes, keys, reports and pins untouched. Ranking, candidate pool and
  selection unchanged throughout: every measured difference is attributable to
  the acceptance decision.

## What was built

1. `crates/ganesh-core/src/accept.rs`: acceptance extracted into a traced decision
   function. Every rule records the sentence span it relied on; the dev-only probe
   prints candidate pool, selected top three, decision reasons, quantity/state spans,
   and (evaluator-side only) whether key passages entered pool/selection. Production
   response contract unchanged.
2. Experiment A (`GANESH_QUANTITY_BINDING=1`): a quantity counts only when a digit
   literal or number word shares a sentence with the measured property extracted
   from the question surface, in a version-consistent sentence. Year-like literals
   and digits embedded in version/identifier tokens are excluded; hyphenated
   digit-units (`32-bit`) count.
3. Experiment B (`GANESH_TRANSITION_CHECK=1`): state/transition questions need a
   sentence containing the object terms with a cue of the required epistemic class
   (propose/attempt/transition/verify), correct polarity (negation scan) and
   environment match. Embedded-cue suppression: "proposed deploying" is not
   verification. Cue-aware coverage treats cue terms as covered by class-compatible
   evidence vocabulary.

## Contrast fixtures (written before implementation, baselines recorded)

`fixtures/contrast-quantity/` (8 cases) and `fixtures/contrast-transition/` (10 cases),
synthetic CC0 text. Baselines: quantity 4/8 correct (both leaks present), transition
5/10. Fixture wording iterations documented in git history.

## Results

### Contrast sets (final)

| Set | Baseline | Candidate | Defeated by |
|---|---|---|---|
| quantity | 4/8 | **8/8** | — |
| transition | 5/10 | **9/10** | qB08 leak |

Experiment A correctly: binds 12 to Novolang 1.0, "eighteen" to 2.0, rejects both
for the 3.0 count question (version mismatch + year/version exclusions), rejects
the meetings count for the members question, accepts the Zetalang count and both
bit widths. Experiment B correctly distinguishes proposed/attempted/verified,
staging-vs-production, negation and paraphrase — but leaks qB08.

### Exposed v2 (30 questions, fixed ranking)

| Configuration | Coverage | Correct abstention | Unsupported ok |
|---|---|---|---|
| reference (off) | 21/24 | 2/6 | 4 |
| A only | 21/24 | 5/6 | 1 |
| B only | 21/24 | 3/6 | 3 |
| **A+B** | **21/24** | **6/6** | **0** |

Attribution is exact: A fixes v25/v28/v30 and nothing else; B fixes v27 and nothing
else; no coverage regressions on v2. The exposed-v2 gate even passes end-to-end
(exit 0) — but exposed evidence cannot establish M1, which is exactly why the
screening rule exists.

### Exposed v1 (44 questions, dev mode)

| Configuration | Coverage | Correct abstention |
|---|---|---|
| reference | 29/35 | 7/9 |
| A+B | 28/35 | 8/9 |

B fixes the h30 leak but causes one new false abstention (h09): a coverage-side
cost of stricter evidence requirements on ordinary questions carrying cue vocabulary.

### v22 diagnosis (explicitly requested)

With binding on, the acceptance layer now does exactly what the issue asked:
`C23 reserved fifteen more words` — the number word *fifteen* binds to the
version-correct sentence while `c89`/`c99` digits in version tokens are excluded.
v22 still fails end-to-end, but the blocker moved: key passage `1387656235#p24`
enters the candidate pool yet is crowded out of the top three (p28/p27/p25
selected). **The remaining v22 failure is selection/ranking, not acceptance.**

## Screening decision

| Criterion | Result |
|---|---|
| 1. Both contrast sets correctly distinguished | **FAIL** — transition 9/10; qB08 leaks |
| 2. Fewer unsupported ok on exposed v2 with ≥20/24 coverage | PASS (0 < 4; 21/24) |
| 3. Adapter agreement, citations, determinism, zero provider calls | PASS (all runs 30/30, citations exact, 0 calls) |
| 4. Reviewer-explainable decisions | PASS — see traces in `evidence/acceptance/*` |

Criterion 1 fails → **stop before fresh evaluation**. No v3 corpus was frozen or consumed.

## The counterexample that defeated the candidate

qB07 vs qB08 form a tension pair no lexical configuration separates:

- qB07 "Did the Northstar **team** verify revision R7 in production?" — `team` is
  agent context the evidence sentence legitimately omits; requiring all object
  terms false-abstains.
- qB08 "Did **lazy** evaluation become optional?" — `lazy` is the critical modifier;
  the eager-evaluation sentence contains `evaluation` but not `lazy`. Lenient
  object binding (strict majority) accepts it — a leak.

Both are ordinary content nouns lexically; only their semantic role differs
(agent context vs property modifier). Positional heuristics fail both directions
(pre-cue terms are the object in qB08, the agent in qB07). The h09 false
abstention is the same phenomenon on real corpus text.

**Classification: evidence interpretation** — specifically a limitation of
lexical sentence-binding heuristics, not candidate retrieval (the decisive
sentences are present in the selected passages) and not coverage calibration.
This is the honest boundary of the lexical baseline and direct evidence for the
roadmap's deferred embedding/hybrid comparison: a semantic scorer could weigh
`lazy` vs `team` by their role in the claim rather than their surface rarity.

## Reproduce

```
cargo build --release --workspace
# experiment matrix on exposed v2 (development evidence)
GANESH_QUANTITY_BINDING=1 GANESH_TRANSITION_CHECK=1 \
  cargo run --release -p ganesh-bench -- run \
  --questions fixtures/questions-v2/heldout.v2.json --corpus fixtures/corpus-v2 \
  --report out/ab.md --details out/ab.json
# contrast diagnostics with relied-upon spans
GANESH_TRANSITION_CHECK=1 cargo run --release -p ganesh-bench -- probe \
  --questions fixtures/contrast-transition/key.json --corpus fixtures/contrast-transition
```

Costs: benchmark wall ~1.8s per 30-question run, REST mean ~14ms, zero provider
calls, import/index unchanged from #162 (acceptance adds sentence analysis on
three passages per query, sub-millisecond).

## What the next agent should know

The most valuable replayable artifact: run the transition probe and compare
qB04/qB05/qB07 (accepted, with the exact confirming sentence printed) against
qB02/qB03/qB06 (refused, with the proposal/attempt/negated/staging spans printed).
That is Ganesh responsibly telling you "this was verified" versus "the available
evidence only says it was discussed" — with the caveat, demonstrated by qB08, that
lexical binding cannot yet tell *which* property a modifier belongs to, and that
gap is semantic, not lexical.
