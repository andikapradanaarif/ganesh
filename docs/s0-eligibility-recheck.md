# Final first-party eligibility accounting

Issue [#46](https://github.com/andikapradanaarif/ganesh/issues/46) follows the
[initial origin screen](s0-origin-screen.md) and implements declaration accounting
for [roadmap A1](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a).
This is tooling preparation, not the actual one-hour S0b-2 recheck.

Freeze the initial inventory and six applicable-task definitions against the
separately saved need-definition digest. Those tasks are the later S0b-2
applicability cases (three within-project and three cross-project), separate
from S0's 30-task evaluation schedule. Source each task from the frozen needs.
Definitions and source-keyed rubric hashes remain private.

The final calculator must preserve H, show changed classifications, and count
only distinct needed eligible automatic lessons. It requires six lessons,
two projects, E/H at least 50%, and supported declarations for all six tasks.
An initial mixed-origin route cannot be cleared by later reclassification.
An assessable final failure routes to A2.3's separately funded decision;
inconclusive initial evidence cannot become a clean first-party go result.

Evidence references and hashes are declarations, not evidence verification or
semantic support. Every report keeps independent verification, experiment
acceptance, model authorization and production narrative false. No source,
transcript or model is read or called. A local calculator cannot prove collection
timing or globally enforce one assessment across devices/files; follow A1's
single one-hour collection window. Recomputing a report does not extend it.

## Freeze the declaration baseline

Prepare a private definition and retain the earlier `freeze-targets` digest:

```sh
go run ./cmd/ganesh s0 freeze-recheck \
  --input /path/to/private/recheck-definition.json \
  --expected-need-digest SAVED_NEED_DEFINITION_DIGEST \
  --output /path/to/private/recheck.frozen.json
```

The root has exactly `schema_version: 1`, the preparation `run_id`,
`initial_inventory`, and `applicable_tasks`. The run ID is a declared namespace,
not verification against a workspace's `run.json`. The initial inventory is
the retained day-one assessment, not a newly cleaned version. Retain its actual
collection evidence separately. If day-one M/H already selected A2.3, follow
that decision immediately; preparing these six tasks is not a prerequisite.

`initial_inventory` has `schema_version: 1`, `state`, `targets`, and `lessons`.
It uses the screen's meanings, with an exact shape: every field and array must
be present, including `lessons: []` and empty reference arrays. Each target has
`id`, `need`, `assessed`, `mixed`, `unknown`, `owner_only`, `reason_refs`, and
`required_lesson_ids`. Each lesson has the screen's `id`, `project`,
`derivation`, `origin`, `assurance`, `eligibility`, and
`eligibility_evidence_refs`. This does not change the older screen input format.

Each of the exactly six task definitions has:

| Field | Declaration |
| --- | --- |
| `id` | Distinct stable task label. |
| `stratum` | `within` or `cross`, exactly three of each. |
| `required_need_ids` | Nonempty distinct references into the frozen need inventory. Every dependency must be covered to support the task. |
| `prompt_sha256` | Exact private follow-on task prompt digest. |
| `source_keyed_rubric_sha256` | Predeclared source-keyed expected outcome/rubric digest. |
| `permitted_sources_sha256` | Digest of this task's permitted evidence/source map. |

Duplicate prompt/source pairs under renamed task IDs are rejected. Actual
task distinctness, source permission and semantic dependence are not verified.
The tasks must be genuine applicable follow-ons under A1/A2.2; synthetic clean
lessons or hand-authored memory cannot satisfy the real gate.

Freeze checks the exact need descriptions/identifiers against the saved need
digest and canonicalizes only ordering. The snapshot binds initial assessments,
six task definitions, algorithm version and need digest. Save it as a new 0600
file and retain the emitted `snapshot_digest` independently. Existing output
files are never overwritten. File sync precedes the freeze response; no host/
power-loss durability or complete client provenance is established by this
declaration file. A self-consistent replacement fails against the original pin.
Collection timing and honest initial declarations remain procedural assumptions.

## Account for the final declarations

At the permitted one-hour recheck, record final evidence in a private input:

```sh
go run ./cmd/ganesh s0 recheck-eligibility \
  --plan /path/to/private/recheck.frozen.json \
  --expected-digest SAVED_RECHECK_SNAPSHOT_DIGEST \
  --input /path/to/private/final-assessment.json
```

The final root has exactly `schema_version: 1`, `snapshot_digest`,
`final_inventory` (the same exact shape as the initial inventory), and
`task_evidence`. Need IDs/descriptions must remain identical; added, removed,
changed or re-cased needs fail before reporting. Assessments, eligible lessons
and evidence may change. Reports show before/after H/M/U/E and all four ratios,
distinct contributing lessons/projects and changed per-target classifications.
Owner-only needs remain in H. Partial need coverage contributes neither E nor
its lessons; unreferenced lessons do not inflate either threshold.

Each task-evidence row has exactly `task_id`, `applicability` (`supported`,
`unsupported`, `unknown`), and nullable `evidence_sha256`. A `supported`
declaration requires a digest; all provided digests must be distinct per-task
records. Missing rows become explicit unknown applicability. A shared batch
artifact needs separately addressed per-task records. Hashes do not establish
existence, meaningful support or independent corroboration. Even a supported
declaration cannot count unless all frozen need dependencies are currently E.

| State / route | Meaning and CLI exit |
| --- | --- |
| `declared_criteria_met` / `first_party_probe_candidate` | The declared four thresholds pass, and neither initial nor current routing blocks the first-party path. Exit 0; this is no experiment, funding or production authorization. |
| `declared_gate_not_met` / `a2_3_decision` | An initial M/H ≥50% route remains binding, or an assessable final requirement fails. Emit the report, then exit 1. A2.3 still requires its own scope/budget/funding decision. |
| `inconclusive` / `inconclusive` | Initial evidence was unassessable, or final evidence is unassessable on a previously permitted path. Emit the report, then exit 1. Do not count a missing recheck as a clean pass. An initial A2.3 route still dominates. |

Empty inventories cannot create a recheck snapshot; the original `screen`
command can still report them as inconclusive. Explicit unknown declarations
with evidence references are assessments, so a final E=0 outcome routes to the
separate A2.3 decision rather than an empty first-party experiment.

Reports include canonical snapshot/input digests and bounded stable IDs, counts,
flags and generated reasons. They omit need prose, project names and evidence
reference contents. Initial/current origins are screener declarations; the
calculator does not inspect manifests, closure, provenance, anchors, lifetimes
or applicability evidence. No optional owner contribution fills an automatic
coverage gap. The mandatory discovery-to-reuse/product gates remain separate.

Inputs use regular private 0600/0400 files, no final-component symlinks, 1 MiB
per file and depth 64. Duplicate, aliased, unknown or missing JSON keys and null
arrays are rejected before typed decoding. Task/target/lesson IDs use the
existing 1–80-character evaluation-label grammar. Hashes are lowercase SHA-256.
Limits: 1–1,024 needs, 2,048 lessons, 64 required lessons per need, and 32
references of at most 1,024 bytes per item; need prose is at most 8,192 bytes,
project labels 160 bytes, and origin/derivation/assurance labels 80 bytes.
Snapshot output is also capped at 1 MiB. Errors never echo input. The external
snapshot pin is checked before opening final metadata; the reporting API checks
it independently for callers outside the CLI. Inputs are never modified.

Synthetic fixture validation is reported after the checks complete. Actual
projects, run settings, corpus permission and recheck results remain unknown.
