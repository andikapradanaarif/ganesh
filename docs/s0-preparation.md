# S0 run preparation

Task: [#15](https://github.com/andikapradanaarif/ganesh/issues/15), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). The [roadmap](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a) and [execution sheet](s0-execution-sheet.md) remain authoritative.

This first Go CLI prepares a private run record and discovers local version information. It does not read project contents or transcripts, install hooks, capture events, call models or deploy a service.

## Commands

```sh
go build -o bin/ganesh ./cmd/ganesh
bin/ganesh doctor
mkdir -p -m 700 .local/s0
bin/ganesh s0 prepare --dir .local/s0/day-one
bin/ganesh s0 check --input .local/s0/day-one/run.json --phase inventory
```

Flags are scoped to each command: `prepare` accepts `--dir`; `check` accepts `--input` and `--phase`; `freeze-targets` accepts `--input` and `--output`; `screen` accepts `--input` and `--baseline`. Unrelated flags are rejected before any record is read or written. `doctor` accepts no flags.

`doctor` probes only `go version`, `codex --version`, `claude --version`, `devin --version` and `crush --version`, using binaries found on PATH. Each probe has a two-second timeout and 2 KiB stdout limit. Only an exact recognized version string is recorded; raw output and stderr are discarded. Devin's reported build hash and Crush's fork/prerelease/build suffixes are preserved in the version identity. Missing binaries are `not_on_path`, not proof that the client is absent everywhere. These are discovery candidates, not a supported-client list: no observation selects a client or grants capture eligibility. Other clients and runners remain unknown until inspected.

If a probe's context has expired when its result is checked, it is conservatively reported as `probe_cancelled`, even if valid-looking version output arrived at that boundary.

`prepare` creates a new 0700 workspace containing 0600 `run.json` and `targets.json` files. The parent must already exist; existing workspaces are never reused or overwritten. A failed preparation can leave partial files for inspection and is never reported as successful. This configuration writer makes no capture or remote-durability promise.

`run.json` has an actual generated run identity and preparation timestamp. Project selection, capture installation, transcript permission evidence, configuration/measurements, models, budgets, frozen evidence and the optional A2.3 caps remain unknown. `screen_started_at` remains null. `targets.json` starts empty and `not_screened`; preparation does not start the one-hour screen or establish any origin counts.

The workspace is ignored by git. Keep paths, permissions, evidence and later transcript-derived records private; do not copy them into public issues or commits. Local version discovery does not authorize source enrollment or reads.

The next local tools are [need freezing and initial origin accounting](s0-origin-screen.md). They use the same private `targets.json`; real input selection and the actual bounded screen remain separate tasks.

## Select inputs when their phase needs them

For the actual one-hour inventory, select 3–5 personal projects with explicit local-read permission and the first capture client/installation. Record their absolute paths in `projects`, each with `local_read_permission` set to `allowed` only after owner selection. Record the observed selected client/build and permitted transcript-access evidence in `capture`. No project content or source path is opened by `check`.

The `inventory` check reports these missing inputs without requiring model or spending settings. This keeps held-out-run settings from blocking the day-one screen. Until selection is supplied, do not guess which repositories or transcripts may be inspected.

Before held-out answers, use:

```sh
bin/ganesh s0 check --input .local/s0/day-one/run.json --phase heldout
```

This additionally requires declared capture configuration/measurement references (with gaps explicitly recorded), exact answering/grading IDs and grading family, spend/time/output/reasoning/retry limits, and a reference to the frozen corpus/tasks/rubrics/permissions/variants/order/seeds/digest. Optional A2.3 funding fields do not block S0; blank caps remain unfunded. Apply its separate funding deadline and decision before any A2.3 setup or model work.

Both checks are **presence checks**. They do not independently verify permissions, source provenance, model-family independence, sample/rubric completeness, frozen files or capture coverage. A successful check never grants production narrative eligibility or launches work. The full execution-sheet conditions still govern each run.

Repeated missing-input reasons appear once, in first-occurrence order. Project-path deduplication compares `filepath.Clean` strings case-sensitively; it does not inspect filesystem identity, symlinks or case aliases. Those checks belong to later permitted source mapping.

The JSON result also includes `warnings`. If the opened `run.json` mode grants group or other read permission, `run_record_group_or_other_readable` is emitted without the path or content. This is advisory: it does not change readiness, chmod the file or grant permission. It checks descriptor mode bits only, not ACLs, parent-directory access or effective privacy; an empty warning list is not a privacy attestation.

## Remaining day-one work

After selection, inspect the actual permitted transcripts and installed schemas during the bounded screen; freeze high-priority targets before counting origins. Record H/M/U/E and their reasons under A1. Then implement/test only the selected client's bounded metadata capture profile, within the fixed 2–4-hour cap and smoke priority. The collector, its one-second host / 750 ms process-start deadline, receipts, scrubbing and latency/SessionEnd measurements are not implemented by this preparation CLI.

Count this setup work in S0 effort. Neither an empty target inventory nor successful tooling tests completes P01/P02. No held-out answers, capture installation or release suite is claimed complete.
