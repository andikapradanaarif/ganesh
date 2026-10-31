# S0 execution sheet

**Status: not run.** Start with inventory and capture. Use [roadmap A1](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a); Part II is conditional, not day-one work. Policy below is fixed; complete S0 run inputs before its held-out answers. A2.3 fields govern only that optional experiment and do not block S0.

## Record the run inputs

| Input | Fill during preparation |
|---|---|
| 3–5 permitted personal projects | P1: ___ · P2: ___ · P3: ___ · optional P4/P5: ___ |
| Capture profile | Client/build: ___ · device/runner: ___ · config/script revision: ___ · transcript access: ___ |
| Cold-start time | ___ ms · sample count and cold/warm mix: ___ |
| Process-start-to-durable-receipt latency | p95: ___ ms · p99: ___ ms · failures/timeouts: ___ |
| Effective SessionEnd timeout | Overall: ___ ms · collector host: ___ ms · configuration source: ___ |
| Competing SessionEnd hooks | Identities/timeouts: ___ · joint-run result: ___ |
| Models | Answering exact ID: ___ · grading exact ID/family: ___ |
| Limits | Total API spend: ___ · per-answer time: ___ · output/reasoning cap: ___ · calibration/retry allowance: ___ |
| Frozen evidence | Corpus, held-out tasks/rubrics, separate calibration tasks, permission map, arm variants, counterbalanced order/sample seeds, clean-session setup and configuration digest: ___ |
| A2.3 ceiling and decision | Cap: ___ focused days, US$___; fund/don’t-fund decision by ___ (default: end of S0 day 1); decision: pending / fund / don’t fund |

## Day one

1. **One-hour screen:** freeze high-priority lesson/rationale targets. `H` = all; `M` = known mixed; `U` = unknown/unobserved targets not already in M; `E` = distinct targets covered by eligible automatically derivable lessons. Report **M/H, U/H, (M+U)/H, E/H** and counts. Keep both reason flags when relevant, without double counting.
2. **2–4-hour capture cap:** local metadata collector; no `async`/`asyncRewake`; **750 ms from OS process start, including interpreter/runtime cold start, inside 1 s host timeout**. Record the measurements above; if p99 is tight, prefer a small compiled collector and remeasure within the cap. Priority: receipts/scrubbing and write/failed-write → denials/interruption → subagent → compaction → optional directory events. Test crash recovery and competing SessionEnd hooks; margin is not a durability guarantee. Untested/cloud/device coverage stays unknown. No cloud upload or gateway build.
3. **Freeze the route:** `M/H ≥50%` → A2.3 scope/budget decision immediately. Otherwise allow one one-hour recheck before S0b-2: ≥6 eligible lessons, ≥2 projects, **E/H ≥50%**, and six applicable tasks (3 within/3 cross). Failure → A2.3 decision. **S0/appropriate deterministic tests continue while funding is pending.** Blank caps are unfunded; A2.3 setup/model runs require numeric caps and a recorded fund decision. Unfunded/infeasible or a missed decision deadline → pause that narrative branch for this run. The separate cap includes setup, extraction, consumer/adversarial/repair runs and grading; exhausting it means incomplete, with no borrowing from S0/S0b-2. No automatic typed-only product. Empty inventory is inconclusive.

## Tasks, arms and cost

| Role | Within / cross | Diagnostic subset |
|---|---|---|
| Coder | 7 / 3 | 2 / 2 |
| Planner | 7 / 3 | 2 / 2 |
| Reviewer | 6 / 4 | 2 / 2 |
| **Total** | **20 / 10** | **6 / 6** |

**0:** native instructions/existing notes + own raw tools, no memory. **A:** 0 + typed cards. Run both on all 30 tasks. On the same 12-task subset: **B** packages A; **C** adds reference prose; **D** searches scrubbed summaries; **F** gets that entire same corpus. Full context is mandatory. Existing notes stay identical; no owner-written answers enter the automatic arm.

**108 base answers.** Trigger only when D or F introduces **no new critical error versus A** and either (a) beats A by ≥1 task or (b) ties completion with ≥20% fewer tokens. Then rerun **both** on the frozen eligible-origin subset: **132 total**. No hidden truncation; fresh sessions and matched permissions. A2.3’s 36 consumer answers are separate.

**30–56 hours / 5–7 days:** screen 1; capture 2–4; sources/calibration 8–12; tasks/rubrics 8–15; runs/device checks 5–10; grading/analysis 6–14. Count all tokens, failures and delays. Exceeding limits means incomplete, not omitted controls.

## Score and stop

- Source-keyed oracle separate from implementation verifier. Claude answers → OpenAI-family grader; OpenAI answers → Anthropic-family grader. Pin/calibrate exact IDs; model agreement is not truth. Owner checks all critical/unsupported flags, disagreements/unknowns, plus six primary pairs and three diagnostic sets. Time is inside grading hours.
- Primary A−0 signal: ≥3/30 extra completions, zero introduced critical errors, ≤25% token increase. Flag regression if **0-only wins minus A-only wins ≥2** in either stratum; report one-task losses too. Arm A must complete **≥21/30 (70%)**; below that blocks a pilot-benefit pass even with +3/30. Report 0 against the same floor for comparison; 0 below it does not itself block A. Report both strata/access-only wins. D/F cannot confirm a 0/A regression. This earns another probe, not a build.
- **S0b-2 typed pilot:** preselect exactly 60 distinct natural cases from any larger pool using a frozen frame/mix/seed before inspecting correctness. No duplicates or outcome-based replacements; all 60 must be adjudicated. ≤1 noncritical error passes this filter; 2 fail. Critical errors fail independently, including outside-sample findings; unresolved cases are incomplete. For 1/60, one-sided 95% upper bound = **7.66%**, not proof of 2% quality.
- Missing controls, unsupported grading, unsafe disclosure or budget overrun → stop affected work/report incomplete. LoCoMo leadership and engine forks are deferred. Automatic discovery-to-reuse remains a separate mandatory product gate; S0 cannot satisfy it alone.
