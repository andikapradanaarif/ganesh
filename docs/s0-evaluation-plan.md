# S0 evaluation planning

[Issue #40](https://github.com/andikapradanaarif/ganesh/issues/40) adds local preparation tools for [roadmap A1](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a) and the [execution sheet](s0-execution-sheet.md). This freezes a declared plan and its schedule. Actual screening, corpus preparation, independent grading, model execution and product acceptance remain pending.

`s0 prepare --dir DIR` now adds `evaluation.json` alongside `run.json` and `targets.json`. It binds the generated run ID and leaves the seed unselected, artifact hashes empty, and calibration/held-out task lists empty. It invents no projects, clients, models, budgets or task content. Existing prepared workspaces remain untouched; the new command accepts a separately prepared definition for an existing run too.

After filling permitted, real inputs, freeze and inspect the standard plan:

```sh
go run ./cmd/ganesh s0 freeze-evaluation --input /path/to/private/run/evaluation.json --output /path/to/private/run/evaluation.frozen.json
go run ./cmd/ganesh s0 check-evaluation --input /path/to/private/run/evaluation.frozen.json --expected-digest SAVED_FREEZE_DIGEST
```

The input schema is the generated template. Each task supplies `id`, `role`, `stratum`, `diagnostic`, `tags`, `prompt_sha256`, `rubric_sha256`, `source_keyed_oracle_sha256`, `permitted_sources_sha256` and an explicit `baseline_can_access_required_information` boolean. IDs are case-sensitive ASCII letters/digits/underscore/hyphen, starting with a letter/digit, up to 80 characters. Role and stratum use the exact values below. Tags are unique members of `changed_state`, `workflow`, `gotcha`, `mistaken_premise`, `abstention`, `local_exception` and `irrelevant_memory`; they may overlap across tasks.

| Role | Within / cross tasks | Diagnostic within / cross |
| --- | --- | --- |
| `coder` | 7 / 3 | 2 / 2 |
| `planner` | 7 / 3 | 2 / 2 |
| `reviewer` | 6 / 4 | 2 / 2 |
| Total | 20 / 10 | 6 / 6 |

Provide an explicit unsigned 32-bit `schedule_seed` (zero is valid), and 1–30 distinct calibration task IDs that do not overlap held-out IDs. `bindings` holds lowercase 64-character SHA-256 declarations for the corpus, preselected eligible corpus/filter, unchanged native instructions/notes, typed cards, variants/renderer, permission/access map, model/limit record, calibration, grading/spot-check protocol, clean-session setup and full-context preflight. The grading artifact must retain the source-keyed oracle, calibrated alternate-family grader and the required preselected owner samples. Hash already permitted scrubbed artifacts locally; the planner reads neither their contents nor named projects and cannot establish their validity.

The tool supports the standard 30-task plan only. Repeated role/prompt/source tuples are rejected even under different IDs. An empty, reduced or incorrectly stratified definition fails rather than receiving standard-plan counts. A reduced pilot still needs the roadmap's separate, pre-answer declaration and recalculated protocol.

The schedule contains 60 primary `0`/`A` slots and 48 `B`/`C`/`D`/`F` slots on exactly the frozen diagnostics: **108 base answers**. It also freezes **24 conditional D/F slots** on those same 12 tasks using the eligible corpus: **132 answers only if the later history-win trigger fires**. Conditional slots are planning data; this command never evaluates the trigger or executes them. An empty eligible corpus remains an operator-recorded case with no-summary counterparts, not permission to omit the conditional comparison.

`summary_corpus` identifies only the summary/prose input. `A` and `B` still receive their frozen typed cards; `C` uses the reference prose; `D` and `F` use the identical full authorized corpus, then the identical eligible subset if triggered. `F` is always present. Preparing a digest does not establish context fit or authorize truncation, disclosure or a model call. Existing held-out run checks and every execution-sheet requirement remain in force.

The `sha256-order-v1` algorithm ranks tasks using SHA-256 of its versioned prefix, big-endian seed and length-prefixed purpose/task ID. Primary pairs alternate which arm goes first within each stratum (10/10 within, 5/5 cross). Diagnostic order rotates B/C/D/F so every arm occupies each position three times overall; conditional D/F order is balanced 6/6. Rank ties use exact task ID. Task IDs, tags and calibration IDs are canonically sorted for snapshot identity, making input array order irrelevant. Counterbalance covers these arm orders; project/role-stratified grading samples remain in the separate grading artifact.

The snapshot records the full definition, versioned algorithm and both schedules. Its digest hashes compact JSON in the declared struct field order with the digest field empty. Retain the digest printed at freeze time separately from the snapshot, then supply it to `check-evaluation --expected-digest`. The checker compares that pin and reconstructs the entire canonical snapshot, rejecting changed digests, definitions, algorithms, seed, schedules or counts. A self-consistent regenerated plan fails against the original digest. JSON whitespace, escape spellings and object key order are not identity; decoded string case and values are. Snapshots themselves must retain canonical array order. Schema version and algorithm changes require a new contract.

Inputs are bounded to 1 MiB, regular 0600 or private read-only 0400 files, 32 JSON nesting levels and exact schema keys. Missing/unknown/duplicate/case-aliased keys, wrong types, unselected fields and invalid bounds fail with static errors. Final-component file symlinks are rejected; ancestor paths and the same-user host remain trusted. A new snapshot is created exclusively with mode 0600 (subject to a stricter umask) and file synchronization. The destination's parent must already exist. An existing file is never overwritten, and failed writes can leave an inspectable partial file. This is preparation storage, not the capture spool's crash-recovery/durable-receipt contract.

Freeze/check output contains only state, digest, counts and explicit `evidence_independently_verified: false`, `model_runs_authorized: false`, `production_narrative_enabled: false`, with the trigger `not_evaluated`. Actual definitions/snapshots remain private and outside git. The planner checks declared identities and shape; it cannot prove permission, semantic task distinctness, oracle independence, eligibility, model availability, unchanged artifacts or that freezing preceded answers. Re-freezing to another path remains an operator-controlled action, never evidence that the original baseline was preserved.

Capture reliability remains open in [#34](https://github.com/andikapradanaarif/ganesh/issues/34). This tooling consumes no performance-study attempts and grants no installed capture profile, narrative lane, service build or next-stage acceptance.
