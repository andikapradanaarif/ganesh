# S0 local tooling

Implementation issue: [#3](https://github.com/andikapradanaarif/ganesh/issues/3). The supplied roadmap and execution sheet remain the experiment contract. This page describes the implemented preparation and spool foundation; it does not replace their tasks, grading, budgets or stop rules.

Go is selected for this bounded implementation. The observed development environment is Go 1.27.3 on macOS amd64. Codex 0.166.0 is installed; that observation neither selects it as the capture client nor proves any hook/transcript support. No personal project or capture installation has been enrolled.

## Build and prepare a run

```sh
go build -o bin/ganesh ./cmd/ganesh
mkdir -m 700 .local
bin/ganesh s0 init --output .local/run-inputs.json
bin/ganesh s0 check --input .local/run-inputs.json
```

`init` creates a private JSON template without overwriting an existing file. Project, client, model, budget and evidence values are unknown. `check` lists missing fields and exits unsuccessfully while required inputs remain unfilled. It checks presence, not the truth of permission declarations or frozen evidence. There is no command that starts model answers.

Blank optional A2.3 funding fields do not block S0 preparation. They remain unfunded and cannot authorize A2.3 setup or model runs.

## Screen an evidence inventory

Create a private JSON inventory with these fields:

| Field | Meaning |
| --- | --- |
| `targets` | Frozen needs, each with distinct `id`, `mixed`, `unknown`, and `eligible_lesson_ids`. Preserve both reason flags; mixed takes precedence in the M/U reporting partition. |
| `lessons` | Distinct `id`, actual `project`, `automatic`, and nonempty `eligibility_evidence_refs`. Only needed lessons linked to targets count. |
| `applicable_tasks` | Distinct `id`, required `target_ids`, and `stratum` of `within_project` or `cross_project`. All required targets must be covered for a task to count. |
| `recheck_count` | Zero for initial preparation; one for the single final screen. More than one is rejected. |

```sh
bin/ganesh s0 route --input .local/inventory.json
bin/ganesh s0 route --input .local/final-inventory.json --final
```

These commands calculate M/H, U/H, (M+U)/H and E/H from the supplied screener records. They do not inspect source references or independently verify provenance. The screener must establish eligibility from evidence before recording it. They are local accounting tools, not production validators.

An empty inventory is inconclusive with null ratios. M/H ≥50% routes immediately to the A2.3 decision. Below that threshold the initial result leaves the single recheck pending. A final screen needs six distinct needed eligible automatic lessons, two projects, E/H ≥50%, and three covered applicable tasks in each project stratum. Any failed final requirement routes to the A2.3 decision. This command can recompute a recorded screen; it does not authorize another evidence-collection/recheck window. Every result explicitly leaves production narrative disabled.

The sample identifiers in automated tests are synthetic fixtures and must not be copied into an actual run inventory. No actual one-hour screen has been completed.

## Local metadata spool prototype

`ganesh capture --spool DIRECTORY` reads a normalized metadata object on stdin. It is a development interface for a future local client adapter. **Do not install it as a raw client hook.** No client adapter or hook configuration is supplied yet.

Only five fields are admitted:

- `schema_version`: integer `1`.
- `event_id` and `session_id`: lowercase UUID-shaped identifiers. Event IDs must be assigned once by the integration and retained on retries.
- `event`: one of `session_start`, `session_end`, `turn_stop`, `tool_attempt`, `tool_result`, `subagent_start`, `subagent_stop`, `pre_compact`, `post_compact`, `instructions_observed`.
- `outcome`: one of `observed`, `succeeded`, `failed`, `denied`, `interrupted`, `unknown`.

Unknown fields, unsupported strings, malformed identities, multiple JSON objects and input over 64 KiB are rejected. Raw text, file paths, command arguments, tool output and errors cannot enter the record. Errors exclude supplied input content. A future adapter must reduce/scrub raw input locally before calling this interface; this prototype is not a general-purpose secret scanner.

The collector has no network or model work. On macOS builds with CGO enabled it obtains its process creation time from the OS, including loader/runtime startup, and budgets 750 ms from that time. Reading stdin is deadline bounded. It checks cancellation around storage operations, but cannot guarantee that a filesystem syscall completes before a host kill. There is no successful receipt after an observed deadline failure. Other process-clock profiles fail closed; the run-preparation commands remain usable.

The spool uses a private directory, 0600 records, a process lock, fsynced temporary files, non-overwriting hard-link publication and directory fsync before returning a receipt. Exact retries recover the prior event; changed metadata under the same event ID fails. Capacity defaults to 64 MiB and overflow rejects admission. It provides process-restart durability with that storage intact, not remote protection or power-loss certification. Use local storage supporting these operations; other filesystems are untested.

A missing receipt is ambiguous: the event may already be linked. Retry the exact envelope and identity. An interrupted attempt may leave a scrubbed temporary record; these count toward capacity. Automated orphan cleanup, seven-day expiry, upload, installation controls and purge propagation are not implemented in this prototype. Do not use it for unattended production capture.

Every stored record and receipt reports only `captured_local`, with `origin=unknown` and `coverage=incomplete`. It cannot establish tool-call correlation, exact file-version provenance, root/child linkage, non-tool coverage, closure, reconciliation or narrative eligibility. These are explicit later integration gates.

## Verification and pending evidence

```sh
go test ./...
go test -race ./...
go vet ./...
```

Tests cover the routing partition/thresholds, final eligibility requirements, unsafe payload rejection, spool identity conflicts, concurrent duplicate admission, private permissions, capacity and cancellation. Subprocesses exit without cleanup around file-sync/link/directory-sync/receipt boundaries, then retry to check recovery. Compiled CLI checks exercise unknown preparation inputs, rejection without leaking seeded payloads, exact retry and blocked-input timeout.

Those are prototype tests, not P01, P03-D, P04-L or REL-suite completion. Pending real S0 evidence includes selected project permissions, client/build/configuration, representative installed schemas, the timed one-hour screen, actual hook receipts and fixed-priority smoke matrix, joint SessionEnd contention, process latency cold/warm measurements, real event correlation, and the frozen held-out task/model/budget/grading inputs. No held-out answers or paid model runs have started.

### Development smoke measurements, 2026-10-31

The compiled macOS amd64 CLI was invoked in 50 fresh processes, each with a synthetic normalized event and a distinct generated event UUID, writing to one temporary local spool. All 50 succeeded. OS-process-start-to-durable-receipt latency was p95 **133.992 ms**, p99 **368.616 ms**; parent-observed elapsed latency across all attempts was p95 **135.391 ms**, p99 **369.838 ms**. These are descriptive nearest-rank percentiles over a small development sample, not an SLO or supported-client result.

Page-cache state was uncontrolled; cold-start time was not separately measured. No host hook, one-second host timeout or competing SessionEnd hook was exercised. The detailed non-content attempt records are in the ignored local file `.local/prototype-capture-smoke.json`. Real per-installation measurements remain pending.
