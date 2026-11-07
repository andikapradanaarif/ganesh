# S0 day-one project and origin inventory

Task: [#48](https://github.com/andikapradanaarif/ganesh/issues/48), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). Follows [roadmap A1](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a) and the [execution sheet](s0-execution-sheet.md). Evidence and need text stay private under `.local/s0/issue-48-20261103/`; this document reports bounded aggregates only.

## Recorded run inputs

- Selected projects (3): nexus, aurum, borneo — recorded in the private run record with the owner's earlier selection messages.
- Owner permission: local read + transcript-processing/disclosure grant recorded 2026-11-07 (`permission-grant-20261107055500.json`) before any source was read.
- Selected capture client/build/installation: crush `v0.99.1-fork.1`, local macOS user installation.
- `ganesh s0 check --phase inventory` reports `inputs_present: true`. Model IDs, spend/time/token limits and held-out evidence remain null — not invented.

## Observed transcript availability (metadata only)

| Client store | Nexus | Aurum | Borneo |
| --- | --- | --- | --- |
| Devin sessions | 61 sessions | 0 | 2 |
| Crush project store | present | absent | absent |
| Codex rollouts | 0 | 0 | 0 |

Devin transcript records contain system/user/agent steps but no individual tool-call records, so per-session read classes are unresolved beyond what titles/text reveal. Git history in all three repos carries only owner commit identities; per the roadmap that still cannot establish first-party authorship of generated narrative.

## One-hour screen result

Frozen need inventory: 10 targets, digest `9c544011b20481a06916cb66b3d81440900c6fbbb495b1a139550fbd3d347737` (frozen before origin counting; the report verified the same digest).

| Count | Ratio | |
| --- | --- | --- |
| H=10, M=1, U=7, E=1 | M/H | 0.10 |
| | U/H | 0.70 |
| | (M+U)/H | 0.80 |
| | E/H | 0.10 |

**Route: `single_recheck_pending`** — M/H is below the 50% early-routing threshold, so no immediate A2.3 scope/funding decision is triggered; the single final first-party eligibility recheck is still required at S0b-2. This is not a first-party pass.

Findings:

- One known-mixed target: the nexus review-protocol need demonstrably requires third-party reviewer narrative (multiple PR-review sessions).
- One eligible covered target: the borneo fork-targeting need is supplied by one declared eligible automatically derivable lesson (owner-stated directive, `first_party_only`, `client_attested`). Distinct needed eligible lessons: 1; distinct eligible projects: 1 — far below the later recheck gate (≥6 lessons across ≥2 projects, E/H≥50%, six applicable tasks).
- Seven targets are unknown/unresolved coverage, dominated by: zero aurum transcripts in any proposed client, unresolved read classes in devin transcripts, and unproven provenance of repo notes. One owner-only target (capture installation selection) is retained in H without automatic coverage.

## Limitations

- All classifications are screener declarations. `evidence_independently_verified`, `production_narrative_enabled` and `model_runs_authorized` remain false; private file modes (0700/0600), non-overwrite behavior and git exclusion were verified with the existing tools.
- Unknown coverage cannot count as eligible; the E=1 declaration does not imply any capture, provenance or reconciliation capability.
- Effort: screen evidence collection, freeze and report completed inside the one-hour bound (private record timestamps 05:57Z–06:05Z, 2026-11-07). Overall remaining S0 effort stays unasserted.

## Remaining gaps and next steps

- Issue [#34](https://github.com/andikapradanaarif/ganesh/issues/34) still blocks capture enrollment; its timing-study caps are exhausted and profiles/narrative remain disabled — a new reviewed scope is required there.
- Next under S0 #6: select the actual capture installation and run the 2–4-hour smoke priority only after the #34 scope question resolves; then freeze held-out inputs (corpus, tasks/rubrics, models, limits, ordering) before any answers.
