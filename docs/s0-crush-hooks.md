# S0 Crush tool-hook adapter

Task: [#24](https://github.com/andikapradanaarif/ganesh/issues/24), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6).

`ganesh hook crush --spool DIR` receives a bounded native Crush tool-hook object on stdin, reduces it locally and commits only the existing [capture metadata](s0-local-capture.md). The command has empty stdout and stderr on success and failure. Exit 0 follows durable local storage; exit 1 means rejected, failed or ambiguous capture. No receipt JSON, context, permission decision, retry instruction or tool rewrite reaches the client.

This is a prototype adapter, with no hook installation or project enrollment. Every stored record remains `captured_local / unknown / incomplete`. Discovery still reports unsupported capture profiles. File provenance, exact read effects, closure/reconciliation, upload and the full P01 matrix remain pending. The approved roadmap and execution sheet are unchanged.

## Inspected contract

The profile `crush-tool-hooks-v1-aed2bae` is based on the owner's Crush fork source at [`aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f`](https://github.com/andikapradanaarif/crush/tree/aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f), specifically `internal/hooks/input.go`, `events.go`, `config.go`, `runner.go` and `internal/agent/hooked_tool.go`. It names a source schema, not an authenticated build claim.

The installed executable reports `v0.98.3-fork.0.20261027102508-630a3a9dbe00+dirty`; its build information reports revision `630a3a9dbe0095a0d06473d3e50889487a31a00e` and `vcs.modified=true`. It is not proven to match the inspected source. No installed-client or real-session acceptance is claimed.

Only these native events are admitted:

| Native event | Stored event / outcome | Meaning |
| --- | --- | --- |
| `PreToolUse` | `tool_attempt / unknown` | The runner reported an attempt; execution is not established. |
| `PostToolUse` | `tool_result / succeeded` | The runner reported success; file effects or semantic correctness are not verified. |
| `PostToolUseFailure` | `tool_result / failed` | The runner reported failure; partial effects are unknown. |

Require matching `event` and `hook_event_name`, a lowercase version/variant session UUID, a bounded ASCII call handle, nonempty string `cwd`/`tool_name` and object `tool_input`. Success requires string `tool_response`; failure permits string `error` (including its documented omission when empty). Unknown top-level fields, duplicate top-level keys, malformed types/encoding, mismatched aliases, unhandled events, trailing objects and bodies over 64 KiB are rejected. Arbitrary nested tool input is discarded as a whole, not interpreted. Known credential prefixes are rejected in the call-handle field; this is not a general secret scanner or a promise to detect every secret encoded as an ID.

Paths, tool names, inputs, outputs, errors and the original call handle are never stored or printed. Raw hook bodies are not hashed or retained. The adapter does not read environment values, files, transcripts, credentials, models or the network. A dedicated spool is required for this profile and installation; it must not be shared across unrelated clients/installations.

The event UUID is the first 128 bits of SHA-256 over `profile NUL session_id NUL tool_use_id NUL phase`, with UUID version 8 and variant bits set. `phase` is `attempt` or `result`. Case is preserved. Success and failure share the result identity, so contradictory outcomes conflict rather than overwriting. Exact reduced retries recover the same record/time. Differences in discarded raw content collapse to the same metadata: this does not prove raw-event identity, equal commands, invocation count or complete transcript coverage. These derived IDs are correlation handles, not authentication or anonymization.

## Deadline and client behavior

The adapter shares the primitive's 750 ms OS-process-start budget, including runtime startup, stdin, reduction, lock wait and sync. It inherits the macOS/CGO process-clock requirement and fails closed on unsupported builds. No handler-local clock resets or network work are added. There is no stdout receipt write to stall. OS scheduling and blocking filesystem calls retain the primitive's documented limits; a nonzero exit can occur after a record became durable, so repeat the same native identity rather than inventing one.

The inspected Crush hook configuration has a per-command `timeout` in seconds and no `async`, `asyncRewake` or `onFailure` fields. Its runner treats empty output with exit 0 or 1 as no decision; exits 2 and 49 have control effects and are never used by this adapter. The source wrapper skips subagent tool hooks. Session/lifecycle, permission, ancestry, instruction and compaction events are unsupported here. No configuration equivalence to the roadmap's full capture profile is claimed, and no settings are installed or silently relaxed.

## Validation

Synthetic unit fixtures cover strict reduction, discarded secret-bearing content, stable case-sensitive identities, durable duplicate recovery and conflicting outcomes. Compiled-command tests cover silent success/rejection, supported/unsupported clocks, an unchanged record after retry, rejected contradictory results, unsafe storage, malformed/oversized input, irrelevant flags and blocked stdin. The existing normalized-capture output/deadline tests still run.

On 2026-11-01, the source-runner integration fixture passed against the revision above on macOS amd64, Go 1.27.3, CGO enabled. It used the actual Crush `BuildPayload`, hook runner, shell execution and output aggregation with `timeout: 1`, plus synthetic events. Six notifications (attempt, duplicate attempt, success, conflicting failure, distinct failure and invalid session) produced exactly three reduced records and no aggregate decision, halt, context or rewrite. Contradiction and invalid-session callbacks were non-blocking errors with empty adapter stderr. No agent/model/provider, installed binary, real tool effect, user transcript or client configuration was exercised. This is source-runner evidence, not installed-client acceptance or a latency percentile.

To reproduce with that local source checkout and cached Go dependencies:

```sh
python3 integration/crush/test_runner.py /path/to/crush/source
```

The optional harness verifies the tracked revision/clean state, builds Ganesh, and adds a synthetic test through Go's temporary overlay mechanism. It does not edit the Crush checkout or fetch modules (`GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`). Temporary binary/overlay/spool files are removed. A missing dependency, incompatible platform or failing fixture is incomplete evidence, never a substituted pass.

These fixtures are separate from actual installed-client tests, per-installation latency measurements, production secret-scanner admission and S0 product acceptance. SessionEnd/competing-hook measurements remain unknown, and no lifecycle coverage is claimed from tool-only tests.
