# S0 declared-result reporting

[Issue #42](https://github.com/andikapradanaarif/ganesh/issues/42) adds offline accounting for [roadmap A1](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a), using the [frozen evaluation plan](s0-evaluation-plan.md). It reads declared verdicts and usage; it never grades an answer, runs a model, verifies source permission or accepts the experiment.

```sh
go run ./cmd/ganesh s0 report-evaluation --plan /path/to/private/evaluation.frozen.json --expected-digest SAVED_FREEZE_DIGEST --input /path/to/private/results.json
```

The snapshot is checked against the separately retained freeze digest before result metadata is read. The ledger has exactly `schema_version: 1`, `snapshot_digest` (matching that pin) and a `results` array. An empty array is a valid incomplete declaration; it creates no outcomes. Each row has these exact fields:

| Field | Declaration |
| --- | --- |
| `answer` | Exact frozen `{task_id, arm, summary_corpus}` slot. |
| `outcome` | `pass`, `fail` or `indeterminate`, supplied by independent grading outside this analyzer. |
| `critical_error_ids` | A unique list of up to 64 case-sensitive IDs, or `null` when unknown. `[]` explicitly means no declared critical errors. |
| `answering_tokens` | Nonnegative integer aggregate for every answering/tool call and retry in this slot, or `null`. |
| `elapsed_ms` | Nonnegative integer total elapsed time including failed attempts/retries, or `null`. |
| `cost_micro_usd` | Nonnegative integer answering/tool cost in millionths of a US dollar, or `null`. |
| `evidence_sha256` | Lowercase SHA-256 of this slot's retained verdict/usage/attempt record, or `null` when unavailable. |

The evidence record must identify its task, arm, corpus and fresh execution/session evidence. Evidence hashes cannot be reused across slots; a shared batch artifact needs separately addressed per-slot records. This rejects obvious replay but does not authenticate records or prove fresh sessions. The analyzer reads none of those artifacts. Missing evidence, unknown critical errors or an indeterminate outcome makes the grade unresolved. Usage can remain unknown independently of grading. Missing rows and unknown values never become failed tasks or zero usage.

Use pass/fail only for final externally adjudicated verdicts. Unresolved critical/unsupported flags, oracle–model disagreements and unknown outcomes remain indeterminate. The retained evidence must preserve the required oracle, model and owner decisions; model agreement alone is not adjudication, and a hash does not establish that these checks occurred.

All values are declarations. The independent source-keyed oracle, calibrated alternate-family grader, owner samples, full-context fit, permissions, budgets and actual run identity still need the execution-sheet checks. Calibration, preparation, extraction, grading and other overhead costs must be reconciled separately; the reported answering/tool aggregates are not a claim of complete experiment cost or budget compliance.

Primary counts compare 0/A on the same frozen tasks, separately for 20 within-project and 10 cross-project cases. Partial paired counts use only fully resolved pairs and retain the fixed expected denominator; percentages, introduced-error totals and regression flags stay unknown until all pairs in that comparison resolve. A critical error is introduced when its ID appears for a task in the candidate but not in that same task's baseline. Equal aggregate counts cannot hide a different error or a move to another task.

Coverage also shows known critical warnings in unresolved rows and counts unknown critical-error sets. Paired token, elapsed and answering-cost totals are shown independently for each arm/stratum whenever all values in that scope are known; otherwise that total is null. Known subtotals and missing-usage counts remain visible in base/conditional coverage. Summed elapsed time is slot time, not parallel experiment wall time.

`primary.declared_numeric_criteria` evaluates A's 21/30 floor, at least three net completions, no introduced critical errors, no two-task net regression in either stratum and at most 25% more total answering tokens. Arm 0's 21/30 floor is reported separately and does not gate A. Unknown primary grades or tokens leave the numeric criterion incomplete. `declared_cross_project_access_wins` lists resolved A-only cross-project wins whose frozen access declaration says 0 lacks the required information. This does not prove actual disclosure permission or establish within-project continuity.

B/C/D/F comparisons use exactly the 12 diagnostics and their matching A answers. Only D/F can trigger the eligible-origin rerun: no introduced critical errors and at least one more completion, or equal completion with at least 20% fewer answering tokens. All 12 paired grades must resolve. A completion win can establish this mechanical trigger with unknown tokens, while the overall metadata remains incomplete. A token-tie branch needs known usage and a positive baseline total; zero-versus-zero does not establish percentage savings. Integer products use full-width arithmetic; there is no floating-point threshold rounding.

The report accounts for all 108 base slots and the planned 24 conditional slots. `conditional_required_by_declared_trigger` is true, false or null (unknown). If triggered, all conditional slots must be present with resolved grades and known usage before metadata is complete. Conditional rows unsupported by the current trigger are visible as a protocol-review condition. Full-to-eligible D/F comparisons report policy-loss outcomes and usage on the same tasks, including six within/six cross cases. They never clear a 0/A regression or trigger another rerun.

States are `incomplete_base_metadata`, `conditional_metadata_required`, `unexpected_conditional_metadata` and `metadata_complete`. The CLI emits the report then exits 1 for the first three; malformed or inconsistent input fails before report emission. `metadata_complete` exits 0 even when numeric criteria are not met: the exit status concerns accounting completeness, not pilot benefit. Every report explicitly keeps `experiment_accepted`, `evidence_independently_verified`, `model_runs_authorized` and `production_narrative_enabled` false.

Inputs use the private, exact-shape evaluation reader: regular 0600/0400 files, no final-component symlinks, 1 MiB per file and the shared depth-64 bound. Reject unplanned/duplicate slots, invalid IDs, duplicate critical IDs, reused evidence hashes, negative numbers and overflow. Numeric totals must fit signed 64-bit integers; this is a representation limit, not a run budget. Row and critical-ID ordering is canonicalized for `results_digest`. Task-ID/critical-ID case and all values remain significant. Errors are static and never echo invalid content. Reports include bounded IDs only from the pinned plan; actual answers, rubrics and source bodies are absent.

This is metadata accounting for the standard S0 plan. It supplies no actual project/model/budget/result identifiers, enables no capture or narrative lane, and establishes neither P01/P02 acceptance nor the mandatory automatic-memory product case. Startup reliability remains tracked in [#34](https://github.com/andikapradanaarif/ganesh/issues/34); no closed timing study is extended.

## Validation, 2026-11-03 (Asia/Jakarta)

The result-analysis/domain and CLI contract fixtures pass, including the exact floor/net/token boundaries, per-stratum regression/access accounting, per-task critical identities, unknowns, matched diagnostics, trigger OR rule, conditional completeness/policy loss, canonical identity, malformed metadata and overflow. Build, vet and the CGO-disabled full suite pass on Go 1.27.3, macOS amd64. The dedicated collector dependency graph still excludes S0/developer/local-record packages and `os/exec`; capture code and deadlines are unchanged.

Initial full normal and race validation was incomplete under #34. Each suite was attempted once and retried once unchanged, serially. The following first-callback rejections returned silent exit 1 while their outer contexts remained active; exact causes and record/receipt outcomes are unconfirmed. No race-detector warning appeared.

| Suite attempt | Observed rejected profiles and parent-observed callback duration |
| --- | --- |
| Normal initial | Stripped 928.638 ms; standalone 785.974 ms |
| Race initial | Development 778.295 ms |
| Normal retry | Stripped 764.709 ms; standalone 774.426 ms |
| Race retry | Development 768.591 ms; stripped 762.563 ms |

These are correctness-validation observations, not a timing study or profile qualification. The initial failures kept PR #43 in draft.

Follow-up under [#44](https://github.com/andikapradanaarif/ganesh/issues/44), at runtime head `932c449`: after a separately frozen 18-launch diagnosis/artifact comparison, one new serial pass each of build, vet, full normal (`go test -p=1 -count=1 ./...`), full race (`go test -race -p=1 -count=1 ./...`) and CGO-disabled full tests passed. Both exact-pin native runner fixtures also passed: baseline `aed2bae` reproduced TurnEnd suppression; repaired `3378f47` delivered endings; each produced 15 reduced records from 41 synthetic notifications. Collector dependency isolation and documentation/diff checks pass.

No runtime code, test assertion, build preparation, clock or deadline changed between the failed and passing validations, and no repeat-until-pass loop was used. The follow-up measurement had zero failures in both unsigned and ad hoc signed arms; it established no signing benefit and selected no build change. Its producers/results are recorded in [PR #45](https://github.com/andikapradanaarif/ganesh/pull/45), separately from these correctness checks. PR #43's validation criterion is now met for this run and it is ready for review. #34 remains open: these passes neither explain the earlier failures nor qualify an installed capture profile.

The compiled CLI was separately checked with explicitly synthetic metadata: 108 rows declaring A=21/0=18 produce `metadata_complete` and a met numeric filter, while all acceptance/authorization flags remain false. An empty ledger produces 108 missing slots, an incomplete criterion, JSON output and exit 1. No answer was graded and no model was called. Actual S0 inputs/results and cumulative stage effort remain unknown; the approved documents and historical experiments are unchanged.
