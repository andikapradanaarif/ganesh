# S0 day-one project and origin inventory

Task: [#48](https://github.com/andikapradanaarif/ganesh/issues/48), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). Follows [roadmap A1](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a) and the [execution sheet](s0-execution-sheet.md). Evidence and need text stay private under `.local/s0/issue-48-20261103/`; this document reports bounded aggregates only.

## Recorded run inputs

- Selected projects (5): nexus, aurum, borneo (owner selection 2026-11-03) plus valo-backend and kiosk (owner addition 2026-11-07, verbatim retained in the private grant record). Within the 3–5 bound.
- Owner permission: local read + transcript-processing/disclosure grants recorded 2026-11-07 (`permission-grant-20261107055500.json`, `permission-grant-20261107062000.json`) before any source was read.
- Selected capture client/build/installation: crush `v0.99.1-fork.1`, local macOS user installation.
- `ganesh s0 check --phase inventory` reports `inputs_present: true`. Model IDs, spend/time/token limits and held-out evidence remain null — not invented.

## Observed transcript availability (metadata only)

| Client store | Nexus | Aurum | Borneo | Valo-backend | Kiosk |
| --- | --- | --- | --- | --- | --- |
| Devin sessions | 61 | 0 | 2 | 74 (tree) | 13 |
| Crush project store | present | absent | absent | present | present |
| Codex rollouts | 0 | 0 | 0 | 0 | 2 |

Devin transcript records contain system/user/agent steps but no individual tool-call records, so per-session read classes are unresolved beyond what titles/text reveal; codex rollouts do record tool events. Git history in all five repos carries only owner commit identities (kiosk also shows 4 `Devin`-authored commits); per the roadmap that still cannot establish first-party authorship of generated narrative.

## One-hour screen result (operative, expanded inventory)

An initial 3-project screen (10 targets, digest `9c544011…`) ran first and is retained as superseded evidence: H=10, M=1, U=7, E=1. After the owner added valo-backend and kiosk, the inventory was re-frozen and re-screened within the same day-one bound. The operative screen:

Frozen need inventory: 17 targets, digest `3d8711a818f695c9a20d1ccdcbc5fe0303e08e15ca5c90688a1ed7ea424f54ba` (frozen before origin counting; the report verified the same digest).

| Count | Ratio | |
| --- | --- | --- |
| H=17, M=3, U=12, E=1 | M/H | 0.18 |
| | U/H | 0.71 |
| | (M+U)/H | 0.88 |
| | E/H | 0.06 |

**Route: `single_recheck_pending`** — M/H is below the 50% early-routing threshold, so no immediate A2.3 scope/funding decision is triggered; the single final first-party eligibility recheck is still required at S0b-2. This is not a first-party pass.

Findings:

- Three known-mixed targets: the nexus, valo and kiosk review-protocol needs demonstrably require third-party review narrative (PR-review sessions consuming reviewer feedback across all three active repos).
- One eligible covered target: the borneo fork-targeting need is supplied by one declared eligible automatically derivable lesson (owner-stated directive, `first_party_only`, `client_attested`). Distinct needed eligible lessons: 1; distinct eligible projects: 1 — far below the later recheck gate (≥6 lessons across ≥2 projects, E/H≥50%, six applicable tasks).
- Twelve targets are unknown/unresolved coverage, dominated by: zero aurum transcripts in any proposed client, unresolved read classes in devin transcripts, and unproven provenance of repo notes. One owner-only target (capture installation selection) is retained in H without automatic coverage.

## Limitations

- All classifications are screener declarations. `evidence_independently_verified`, `production_narrative_enabled` and `model_runs_authorized` remain false; private file modes (0700/0600), non-overwrite behavior and git exclusion were verified with the existing tools.
- Unknown coverage cannot count as eligible; the E=1 declaration does not imply any capture, provenance or reconciliation capability.
- Effort: screen evidence collection, both freezes and both reports completed inside the one-hour bound (private record timestamps 05:57Z–06:2xZ, 2026-11-07). Overall remaining S0 effort stays unasserted.

## Remaining gaps and next steps

- Issue [#34](https://github.com/andikaarif/ganesh/issues/34) still blocks capture enrollment; its timing-study caps are exhausted and profiles/narrative remain disabled — a new reviewed scope is required there.
- Next under S0 #6: select the actual capture installation and run the 2–4-hour smoke priority only after the #34 scope question resolves; then freeze held-out inputs (corpus, tasks/rubrics, models, limits, ordering) before any answers.
