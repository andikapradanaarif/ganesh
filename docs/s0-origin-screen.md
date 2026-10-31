# Initial S0 origin accounting

Task: [#17](https://github.com/andikapradanaarif/ganesh/issues/17), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). Follow [roadmap A1](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a) and the [execution sheet](s0-execution-sheet.md).

These local commands freeze the handoff-need definitions and calculate the initial route from supplied screener assessments. They do not read transcripts or repositories, inspect cited evidence, establish capture support, start model runs or authorize a production content lane. Actual project/client choices can remain pending while this tooling is prepared.

## Freeze the needs before counting origins

Use the private workspace from [run preparation](s0-preparation.md). Its existing empty `targets.json` remains readable and explicitly inconclusive.

During actual S0 preparation, record every distinct high-priority lesson/rationale need as a target with an exact `id` and `need` description. Do not derive this list from held-out answers, remove owner-only needs or invent eligible lessons. Preserve the same target inventory at the later recheck.

```sh
go build -o bin/ganesh ./cmd/ganesh
bin/ganesh s0 freeze-targets \
  --input .local/s0/day-one/targets.json \
  --output .local/s0/day-one/needs-frozen.json
```

The snapshot preserves exact case and description text and canonicalizes only target order. It has private 0600 permissions and cannot overwrite an existing file. Empty or duplicate need definitions are rejected. Its SHA-256 digest binds the definitions, not the truth of assessments or independently proven collection timing. Follow A1's timing requirements; creating another snapshot is not permission to change the denominator.

All local JSON inputs are limited to 1 MiB. Unknown fields, trailing objects, invalid JSON and invalid UTF-8 are rejected with content-free errors.

## Record the declared assessments

The root has `schema_version: 1`, `state` of `not_screened` or `screened`, `targets`, and optional `lessons`. Set `state` to `screened` only when recording the actual bounded screen. An unscreened inventory may contain bare need definitions, but not origin or lesson assessments.

Each target contains:

| Field | Meaning |
| --- | --- |
| `id`, `need` | Definition retained exactly from the frozen snapshot. |
| `assessed` | Whether the screener has assessed the needed origin/coverage. Omission or false counts as unknown. |
| `mixed`, `unknown` | Preserve both underlying reason flags. Known mixed takes precedence in the disjoint M/U totals. |
| `owner_only` | Retain the need in H, but do not count it as automatic coverage. |
| `reason_refs` | Nonempty references to retained private assessment evidence whenever an origin/owner-only assessment is supplied. Raw evidence content is not copied into the report. |
| `required_lesson_ids` | Every lesson required to cover this need completely. Partial eligible coverage does not count the whole target as E. |

An explicit `mixed`, `unknown` or `owner_only` declaration with reason references counts as an assessment for initial routing even when `assessed` is false. This includes the legitimate outcome that transcripts were unavailable. It does not clear unknown coverage: `assessed: false` still excludes the target from E. Bare definitions without assessment flags remain inconclusive.

Each lesson has distinct `id`, actual originating `project`, `derivation`, `origin`, `assurance`, `eligibility`, and `eligibility_evidence_refs`. `eligibility` is `eligible`, `ineligible` or `unknown`. A declared eligible automatically derivable lesson must have `derivation: automatic`, `origin: first_party_only`, assurance `server_observed` or `client_attested`, and nonempty evidence references. Inconsistent eligible declarations are rejected. Owner-authored, synthetic, mixed or unknown substitutes cannot count as eligible automatic coverage.

The screener must establish those declarations using the roadmap's actual evidence requirements, including reconciled coverage and file provenance for client-derived lessons. These commands check field consistency and reference presence only; they do not prove that the references exist, establish first-party origin or verify meaning. Labels and an existing anchor cannot substitute for that evidence.

Keep all real need descriptions and evidence records private. Only summary IDs, flags, counts and generated reason codes are emitted; raw descriptions and reason-reference contents are not echoed. IDs should be non-sensitive stable labels.

## Produce the initial report

```sh
bin/ganesh s0 screen \
  --input .local/s0/day-one/targets.json \
  --baseline .local/s0/day-one/needs-frozen.json
```

For a nonempty inventory the frozen baseline is required. Changed, added or removed definitions fail before reporting. Reordering alone does not change identity. The command reads but does not modify the inventory or snapshot.

The report contains H/M/U/E, M/H, U/H, (M+U)/H and E/H together, plus each target's mixed/unknown/coverage flags and generated reasons. E counts distinct targets supported by all their required declared-eligible lessons; it is never calculated by subtracting M and U. Distinct lessons/projects count only those contributing to E; unreferenced candidates do not inflate coverage.

| Route | Interpretation |
| --- | --- |
| `inconclusive` | Empty inventory, or no explicit assessment declarations. Empty ratios are null, not zero. The command emits the report and exits with status 1. A nonempty inventory with only bare, unassessed definitions retains H and U/H = 1. |
| `a2_3_decision` | M/H ≥50%. Move narrative work to the separately scoped/budgeted funding decision; S0 and applicable deterministic work continue. This does not fund or start A2.3. |
| `single_recheck_pending` | Below the early mixed-origin threshold. The one permitted final first-party eligibility recheck is still required at S0b-2; this is not a first-party go result. |

High U/H or (M+U)/H remains visible but does not become a second automatic early threshold. The later six-lesson/two-project/E/H/six-applicable-task gate is not implemented by this initial-screen command.

Every report states `evidence_independently_verified: false`, `production_narrative_enabled: false`, and `model_runs_authorized: false`. A successful reporting command does not mean the experiment or product passed.

Automated tests use synthetic fixtures and independently stated expected counts. They are not real S0 results. No actual transcript screen or capture integration is completed by this task.
