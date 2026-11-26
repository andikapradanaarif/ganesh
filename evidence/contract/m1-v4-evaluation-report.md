# M1 v4 decision evaluation — final report (issue #179)

## Did #178 pass M1 on v4? **FAIL** — and the reference #176 fails identically.

| Gate | Required | Reference #176 | Candidate #178 |
|---|---|---|---|
| Answerable coverage | ≥20/24 | 15/24 FAIL | 15/24 FAIL |
| Correct abstention | 6/6 | 2/6 FAIL | 2/6 FAIL |
| Adapter agreement | 30/30 | 30/30 PASS | 30/30 PASS |
| Citation integrity | all | 168/168 PASS | 168/168 PASS |
| Reproducibility/cost | replay, 0 provider | deterministic PASS, 0 calls | deterministic PASS, 0 calls |

## What did #178 change relative to #176 on identical questions?

**Nothing — the arms agree on every one of the 30 cases.** v4 contains no case that discriminates the e08 mechanism in the candidate's favor: the one e08-class insufficient case (i01) leaks in *both* arms. The historical finding stands: #178 fixed e08 and retained every reported control on the exposed fixtures (reproduced again in preflight: action-probe 7/8→8/8, rail-probe 8/8→8/8). On fresh v4 evidence the difference neither helped nor hurt; the evaluation is decided by the absolute gate, which both arms miss by a wide margin.

## Every miss (both arms identical)

**Coverage misses (9, all wiki, all selection-side):** w01, w02, w03, w05, w06, w08, w11, w12, w14. Verified mechanism on samples: the key passage is in the candidate pool but ranked outside the top three (w01: curiosity-a1#p2 in-pool, unselected; w08: hubble-a1#p1 same). First failure = top-three selection/ranking, before acceptance is even consulted. Acceptance then compounds it: coverage-only acceptance on non-action questions cannot detect that the selected passages lack the answer, so the system confidently returns wrong evidence.

**Insufficient leaks (4):** i01 (synthetic, e08-class — leaks in both arms; the candidate's clause mechanism does not catch this variant), i04/i05/i06 (wiki traps — coverage-only acceptance has no verification path for ordinary factual questions, so high-overlap passages missing the exact date/number/end-of-mission fact are accepted).

**Correct results worth noting:** all 8 synthetic answerable cases pass in both arms; i02/i03 (ambiguous continuation, impossible-outcome) correctly abstain; wiki w04/w07/w09/w10/w13/w15/w16 pass. Public-vs-synthetic: wiki answerable 7/16, synthetic answerable 8/8, insufficient 2/6.

Cost: mean latency 50.7ms/query (max 92.3) on the 205,902-byte corpus; zero provider calls; replay byte-identical statuses and ordered IDs.

## Execution record

Freeze commit `996b7a5daa87824bebdfe7b0ef65b3ace96349f5` (branch `feat/179-v4-evaluation`), key digest `4e3f9949a38c74dc`; arms `ef6c5086` (#176) and `6b06393c` (#178), same four flags, scorer `scoring.v2`, separate clean builds. Parse-time key format defect (tags type) caught before any exposure, corrected, re-frozen — no evaluated question was exposed at that point. Preflight on the frozen packet: PASS with only intentional distinct-fact overlap warnings (w11/w12, w13/w14). Key inspection caught and fixed one dateless span (w16) before freeze. First as-run artifacts preserved in `evidence/contract/v4-asrun-*`. Preflight arm-verification on exposed evidence exact: action-probe 7/8 vs 8/8 (e08 discrimination present), rail-probe 8/8 both.

## Follow-up completed (per the FAIL row)

One follow-up issue opened for the best-supported cause — **top-three selection ranks wrong passages for ordinary factual lookups** — with the w01/w07 contrasting pair and a bounded experiment scope. The secondary acceptance-side finding (no verification path for non-action questions) is recorded in that issue as explicitly out of its scope. Stopped before tuning on v4; no v5 created. v4 cases are now exposed for diagnosis only.
