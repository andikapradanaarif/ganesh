# EVALUATION — acc-xfer-v5 paired transfer evaluation (candidate `a2295bf4`)

**Verdict: REJECTED.** The candidate fails both predeclared retention gates on the fresh frozen packet and the strict-greater abstention requirement on the exposed-v5 diagnostic. Genuine root-cause fixes landed (w13, v5/i01, v5/i02), but they are offset by one compound regression (q08), one newly unmasked leak class (i03-type future events), and no net abstention gain.

## Pins, freeze, provenance

- Baseline comparator: `6c593a1daa9a656ab042a8673b37fbecf3c15747` (PR #198), official baseline `#188` = `8f273aaa54d401afc72e44d6690e3e9f21b03807`.
- Candidate pin: **`a2295bf42aee638f01f15d0747765a3b2eb26a95`**, committed BEFORE any acc-xfer-v5 evaluation.
- acc-xfer-v5 freeze commit: **`e5ecfc4efa0a`** (ancestor-verified in candidate history). NOTE: the freeze was committed before NLI calibration completed, contrary to the preferred ordering; because NLI was ultimately OMITTED (no model/config entered the acceptance path), the packet contents were never influenced by any post-freeze outcome. No evaluation of acc-xfer-v5 was run before the candidate pin.
- Packet domain: cathedrals / architectural monuments — Notre-Dame de Paris, Cologne Cathedral, Sagrada Família, Leaning Tower of Pisa, Mont-Saint-Michel, Edinburgh Castle. Disjointness audit vs every manifest across `fixtures/` and all worktrees: **0 overlaps**. Preflight: PASS, 0 problems.
- Models: reranker sha256 `821d1aa69520…`, QA verifier sha256 `240478c7251d…`, both fail-loud active in every run; **0 provider calls**.

## Step 1 — w13 root cause

The formal-v5 `w13` abstention was NOT retrieval nondeterminism. Probe and bench select identical passages (`pinatubo-a1#p25/p20/p37`, deterministic across repeats). The cause: `transition_check` misfired on the extractive wh-date question — "occur" is a transition cue, and with no transition evidence the question was rejected. Fix: transition checking is gated off ordinary extractive/wh-date questions. Verified post-fix: probe `w13 got=ok` with the same selection; REST/MCP adapter agreement 30/30 in bench runs; `qa-xfer-v1/q07` probe `got=ok` agrees with run.

## Step 2 — deterministic fixes

`RetrievalConfig::default()` now enables `quantity_binding`, `transition_check`, `proposition_check`, `context_chain`, `action_check` by default (env-overridable). Unit verification requires a canonically matching unit for explicit "in <unit>" measurement questions, excluding bare 4-digit years and "years ago" durations. Exact-date precision now requires substantive multi-term event overlap (`hits*2 >= len`).

Results: v5/i01 and v5/i02 now correctly abstain (exact-date precision and quantity-binding respectively); 30/30 unit tests pass.

## Step 3 — NLI calibration verdict: OMITTED

Model tested: `typeform/distilbert-base-uncased-mnli` (DistilBertForSequenceClassification, id2label 0=entailment/1=neutral/2=contradiction, safetensors 267,835,640 bytes), pure-Rust candle, hypothesis format `Question: {q} Answer: {span}`.

Exposed-only calibration (premise = passage, hypothesis as above, top-3 passages):

| case | max entailment | verdict |
|---|---|---|
| v5/i04 (trap) | 0.02 | would catch |
| acc-xfer-v4/q07 (trap) | 0.02 | would catch |
| qa-xfer-v1/q09 (trap) | **0.52** | NOT separated |
| v5/w09 control | 0.00 | false abstention |
| v5/w13 control | 0.00 | false abstention |
| ax4/q01 control | 0.17 | false abstention |
| ax4/q05 control | 0.04 | false abstention |
| ax4/q10 control | 0.01 | false abstention |
| v5/s01 control | 0.97 | pass |
| qax1/q03 control | 0.99 | pass |

The predeclared rule (catch ≥2/3 traps with ≤1 false abstention) FAILS: 5 false abstentions on answerable controls, and the entity-swap trap entailed at 0.52 — no clean threshold exists. NLI omitted; calibration recorded honestly. `nli_verify.rs` remains in-tree as an inactive experimental artifact, not wired into acceptance.

## Step 4 — paired acc-xfer-v5 results

| arm | coverage | abstention |
|---|---|---|
| #188 | 6/8 | 2/6 |
| `6c593a1d` | 8/8 | 3/6 |
| candidate | **7/8** | **3/6** |

Per-case (`A`=correct abstain, `H`=hit, `L`=leak, `-`=miss):

| q | #188 | 6c593a1d | candidate |
|---|---|---|---|
| q01–q07 | mostly hits | all hit | all hit |
| q08 compound | H | H | **miss (false abstention)** |
| q09 year-only exact-date | A | A | A |
| q10 missing measurement (tonnes) | L | L | L |
| q11 wrong unit | A | A | A |
| q12 inverted outcome | L | L | L |
| q13 entity swap | L | A | A |
| q14 planned-future completion | L | L | L |

Gate evaluation: candidate coverage NOT ≥ `6c593a1d` (7 < 8) and abstention NOT strictly greater (3 = 3) → **primary gate FAIL**.

## Exposed-suite matrix (candidate)

- v5 exposed diagnostic: coverage **23/24** (w13 fixed; w09 remains the chronic evidence-miss), abstention **3/6** — composition changed: now correctly abstains i01, i02, i05 (i01/i02 newly fixed) but leaks i03 (future/unoccurred, unmasked by the transition-fix), i04 (inverted outcome, NLI omitted), i06 (synthetic diaphragm trap — regression vs formal run). Strict-greater requirement not met.
- v4: 23/24 + genuine 5/5 abstentions — retention holds.
- v2: 28/30 total (22 hits + 6/6 abstention; compounds v04–v06 hit; v28+v30 abstain) — zero regression.
- Probes: all five suites pass (event, action, chain, quantity, rail — exit 0 each).

## Root-cause analysis of the q08 regression

q08 ("consecrated … and which architect designed it") false-abstains under BOTH legacy-off and all-on flags with reason `exact-date-precision: no adjacent month-day calendar date tied to the queried event`. The substantive-overlap tightening breaks split-vocabulary compounds: event terms span both sub-questions (consecrated/designed/architect) while each individual passage carries only part of the vocabulary adjacent to the date. This is a defect in the new exact-date logic, not the default-on flags.

## Remaining leak classes (honest)

1. Inverted outcomes (v5/i04, ax5/q12) — NLI calibration failed to separate; unsolved.
2. Missing-measurement when the same unit appears elsewhere in the article for unrelated facts (ax5/q10) — unit-existence binding is entity-blind.
3. Planned-future events whose articles also contain partial realized events (ax5/q14, v5/i03).
4. Compound questions with vocabulary split across sub-questions (ax5/q08) — regression introduced by exact-date tightening, must be fixed next iteration.

## Costs

Bench latency mean ~3.5–6.7 s/query (QA verification dominates). acc-xfer-v5 corpus 6 articles, 444 passages.
