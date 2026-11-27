# Backup-log false-abstention diagnosis and bounded chain fix (issue #167)

**Verdict: useful diagnosis / partial fix — stop without fresh M1 evaluation.** The mechanism is verified; the candidate misses two decision-rule conditions. v3 unconsumed.

Comparison base: `774567599233e371e5f019df18b34ac2d08e8823` (PR #166). Branch `feat/167-evidence-chain`, diff against that commit. Baseline reproduced before any change: pinned #166 candidate 2/5 answerable, 3/3 insufficient, t01/t02/t04 false abstentions — exact match.

## The three answers

### 1. Why did Ganesh reject the direct April 5 sentence?

Not cross-sentence reasoning — **the date lives in the section heading, which retrieval indexes but acceptance never receives.** The question's day token (`5`) and month (`april`) are object/identifier terms of the proposition; the evidence sentence "The scheduled backup of archive volume V12 did not start" carries the object, identifier, action and negation but not the date. Object binding requires the identifier-like terms, fails, and the sentence is rejected before polarity is even evaluated. Controlled variation confirmed: injecting the heading tokens into acceptance text flips t04 with no other change. Same root cause for the date component of t01/t02, plus missing object-noun context in their continuation sentences. Classification: **missing context**, not incorrect interpretation and not late rejection.

### 2. Can it now carry the right subject across sentences without borrowing another subject's outcome?

Partially. The bounded fix (`GANESH_CONTEXT_CHAIN=1`): section headings augment acceptance context, and continuation sentences inherit antecedent context **only within one passage entry and only when the entry mentions exactly one subject identifier**; a cue sentence with its own identifier is never augmented, and two-identifier entries disable chaining. On Meridian all three target cases recovered with correct traces (t01 affirmative chain, t02 explicit-negative chain, t04 direct negative). On the fresh Kestrel diagnostic subjects c01/c02/c05/c06 recovered too; the two-subjects control c03 and heading boundary c08 hold. **But the subject-switch control c04 leaks** — not through chaining: the *start* sentence binds (correctly, with date context) and class-level support counts "started" as verify-class evidence for a *completion* question. The defeating example is predicate-agnostic support inside the verify class, exposed — not caused — by the chain fix. c07 leaks the same way. So: yes for dated continuations with a single subject; no where the question's specific predicate matters and only a different verify-class event is recorded.

### 3. Did the change transfer strongly enough to justify spending v3?

**No.** Decision rule: recovers t01/t02/t04 ✓; preserves the three #166 abstentions ✓ (3/3 with zero unsupported-ok on the original check — 8/8 total there); quantity controls 8/8 ✓; exposed v2 unchanged (21/24 hits, 6/6 abstention) ✓; **but fails the ambiguous-subject control (c04)** and **does not improve the frozen transfer group** (x-group 4/5 in both arms; x05 unrecovered). Per the rule: partial fix, stop.

## Paired outcomes (development evidence, zero provider calls)

| Input | Arm | hits | correct abst. | unsup. ok | false abst. | total correct |
|---|---|---|---|---|---|---|
| transfer-check (8) | pinned | 2/5 | 3/3 | 0 | 3 | 5/8 |
| transfer-check | chain | **5/5** | 3/3 | 0 | **0** | **8/8** |
| chain-probe (13) | pinned | 3/7 | 5/6 | 1 | 4 | 8/13 |
| chain-probe | chain | **7/7** | 3/6 | 3 | 0 | 10/13 |
| quantity (8) | both | 6/6 | 2/2 | 0 | 0 | 8/8 |
| exposed v2 (30) | both | 21/24 | 6/6 | 0 | 1 | 27/30 |

Changed decisions on chain-probe: c01/c02/c05/c06 false-abstention→correct; c04/c07 correct-abstention→leak (predicate-agnostic support); x05 unrecovered (log genuinely silent — correct behavior would be abstention; both arms leak it via the same support looseness, pre-existing).

## What each change bought

1. **Heading preservation** — fixes date-in-heading rejection (t04 mechanism; also required for c01/c02). Cost: none observed.
2. **Single-subject antecedent chaining** — fixes continuation-sentence binding (t01/t02, c01/c02, restore jobs already correct). Cost: none observed on controls.
3. **Contradiction made predicate-exact** (this issue, before chaining): "did not complete" no longer contradicts "the check passed" — the t03 leak from the first chain prototype is closed and stays closed.
4. **Unfixed and now precisely located**: positive *support* is still verify-class-agnostic — "started" supports a completion question. The correct rule needs predicate-aware support without losing legitimate class implications (deployed+checklist-passed supporting verified-running). That is the smallest next experiment, and it is evidence interpretation, not retrieval.

## Reproduce

```
cargo build --release --workspace
env GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 \
  cargo run --release -p ganesh-bench -- run --questions fixtures/chain-probe/key.json \
  --corpus fixtures/chain-probe --report out/cp.md --details out/cp.json --mode dev
# pinned candidate: drop GANESH_CONTEXT_CHAIN. probe prints the relied-upon sentences.
```

Fixture digest: chain-probe manifest at `fixtures/chain-probe/manifest.json` (CC0 synthetic). Costs: ~2s per fixture run, REST/MCP agreement 8/8 + 13/13 verified across affirmative (t01), explicit negative (t02/t04) and unknown (t03) representatives; citation openings exact; zero provider calls; zero model/runtime additions.

Traces for claimed fixes point to sufficient evidence: t01 relies on "The transfer completed at 02:41 UTC and the integrity check passed" chained to the V12 antecedent under the April 3 heading; t02 on "The transfer to the secondary site did not complete…" (negated, explicit negative); t04 on the direct sentence with heading date. All resolve to the pinned revision `meridian-t1` with passage locators.
