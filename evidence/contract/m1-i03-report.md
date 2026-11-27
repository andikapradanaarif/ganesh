# v3 erratum, pre-freeze preflight, and negated-activity experiment (issue #173)

**Verdict: useful partial fix. v4 not recommended yet — the competing-activity control fails in both arms and the new fixture shows no improvement. v4 remains unconsumed.**

Base: candidate `72bfb11` (PR #170) + v3 freeze `ce6dc3f`; v3 pinned results and first-run artifacts preserved read-only.

## 1. Erratum (in `evidence/contract/v3-erratum.md`)

Verified directly: **Ruby overlaps v1** (w06/w07/w08/w18 not fresh-family; sensitivity descriptive only, verdicts unchanged); **w06/w18 duplicate one fact** in ruby-a1#p6, and w06's keyed span lacks the date it asks about — scorer credited span presence, a key-quality defect; **key repair transparency**: only `acceptable` ID fields changed between freezes, questions/labels/spans/corpus bytes byte-identical, published results are a deterministic replay under the corrected key. The 30-question denominator and FAIL verdict stand. Paired improvements attributed to the synthetic satellite log (i01, i02).

## 2. Pre-freeze preflight — `ganesh-bench preflight --questions K --corpus C`

Production importer/segmenter; never ranks or queries. Demonstrated on historical packets: **catches the Ruby family overlap as a freeze-blocking problem**; warns on w06/w18 and all shared-passage keys (legitimate pairs like s01/s02/s04 surface as review warnings, not blockers — several genuinely different questions can share one passage); verifies digests, resolves locators, checks span presence, rejects answerable cases without evidence, and applies a date/count sufficiency heuristic. Known heuristic limit: w06's span contains the digits "0.95" (a version, not the date), so the lexical check passes it — actor/outcome/date-type judgment still needs human review, documented in the output. The original segmentation mismatch is caught by construction: hand-keyed IDs that don't match production segmentation surface as unresolved locators or absent spans. activity-probe passes clean as the disposable fixture. First-run artifacts untouched.

## 3. i03 diagnosis — the handoff hypothesis was wrong

The trace shows negation binds fine: "The telemetry verification was not performed" is detected negated. The actual leak: the satellite heading **"Pass March 14" injects the literal token `pass`** into every sentence's chained context, and action-aware support read augmented context — so "Acquisition occurred" counted as pass-evidence. Context for binding was being reused as evidence of the action itself.

## 4. The bounded revision and paired results

Fix: action support and contradiction matching use **only the sentence's own stems**; headings/antecedents remain binding context but are never action evidence. Negated-activity verbs (perform/cancel/skip) added to cue coverage.

| Input | pinned #170 | revision |
|---|---|---|
| exposed v3 — #168 reference (development evidence) | 24/30, 3 unsupported-ok (i01,i02,i03) | — |
| exposed v3 — #170 pinned candidate | 26/30, 1 unsupported-ok (i03) | **27/30, 0 unsupported-ok** — i03 fixed, no regressions |
| activity-probe (frozen pre-implementation) | 7/10, leak a08 | 7/10, leak a08 — **identical** |

Decision-rule evaluation: i03 fixed with correct trace ✓; v3 preserved with zero unsupported-ok ✓; **but no improvement on the frozen fixture** (7/10 = 7/10) and **the competing-activity control a08 leaks in both arms** ("the inspection finished while the repair continued" still lets the repair borrow the inspection's outcome — the e08 class, reported separately as required). a03/a06 false-abstain in both arms: "cancelled before execution"/"never took place" should settle "did it take place" negatively but no cue chain connects activity-verbs to take-place phrasing.

## Smallest surviving counterexamples

1. **a08**: competing activities in one paragraph — same-passage subjecthood of events, the e08 mechanism, unsolved.
2. **a03/a06**: explicit-negative activity phrasing (cancelled/never took place) settles occurrence questions — requires activity-predicate equivalence beyond current action groups.

REST/MCP agreement, citation integrity, deterministic replay, zero provider calls hold throughout. Commands: preflight as above; evaluation `ganesh-bench run --questions fixtures/activity-probe/key.json --corpus fixtures/activity-probe` with the four experiment flags. Fixture digest sha256 key.json `4366bef645c3…` (frozen pre-implementation in commit before the fix).
