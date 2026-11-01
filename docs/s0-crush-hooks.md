# S0 Crush tool-hook adapter

Tasks: [#24](https://github.com/andikapradanaarif/ganesh/issues/24), streaming follow-up [#26](https://github.com/andikapradanaarif/ganesh/issues/26), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6).

`ganesh hook crush --spool DIR` receives a bounded native Crush tool-hook object on stdin, reduces it locally and commits only the existing [capture metadata](s0-local-capture.md). The command has empty stdout and stderr on success, handled failure and recovered handler panic. Exit 0 follows durable local storage; exit 1 means rejected, failed or ambiguous capture. No receipt JSON, context, permission decision, retry instruction or tool rewrite is intentionally emitted. Unrecoverable process-failure limits are described below.

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

Require matching `event` and `hook_event_name`, a lowercase version/variant session UUID, a bounded ASCII call handle, nonempty string `cwd`/`tool_name` and object `tool_input`. Success requires string `tool_response`; failure permits string `error` (including its documented omission when empty). Unknown top-level fields, duplicate top-level keys, malformed types/encoding, mismatched aliases, unhandled events, trailing objects and bodies over the raw native cap below are rejected. Arbitrary nested tool input is discarded as a whole, not interpreted. Known credential prefixes are rejected in the call-handle field; this is not a general secret scanner or a promise to detect every secret encoded as an ID.

The native command now streams the **entire raw envelope** under a separate **4 MiB** limit, including leading/trailing whitespace. It validates and discards large strings and nested values instead of assembling the raw body or tool output in memory. JSON container depth is limited to **64**, including the root; encoded root field names are limited to **256 bytes**, and encoded retained event/alias/session/call strings to **1,024 bytes**, including quotes. The normalized `ganesh capture` input limit remains **64 KiB**. These are prototype admission choices, not measured coverage promises.

Crush embeds full tool output in `PostToolUse`. The earlier 64 KiB raw-input gate therefore rejected otherwise small metadata from larger tool results. The streaming path admits those results within its new bounds; over-cap, malformed, truncated, trailing or over-depth input still fails before a successful record is written. It requires genuine EOF and never accepts a cut-off prefix. A slow or unclosed input still exhausts the original 750 ms process budget. The producer still constructs its native envelope and environment; only Ganesh's reduction is streamed. No real-traffic loss rate has been measured.

The skipper accepts JSON grammar and UTF-8 while retaining only bounded control strings. Discarded nested object member names may repeat, matching the earlier `encoding/json` behavior; duplicate root names, including escaped aliases, remain rejected. Escaped lone surrogates follow `encoding/json` v1 syntax/replacement behavior. Provider call handles containing characters outside `[a-zA-Z0-9_-]` (for example `.`, `:` or `/`), or longer than 160 bytes, still cause rejection.

Paths, tool names, inputs, outputs, errors and the original call handle are never stored or printed. Raw hook bodies are not hashed or retained. The adapter does not read environment values, files, transcripts, credentials, models or the network. A dedicated spool is required for this profile and installation; it must not be shared across unrelated clients/installations.

Crush's `BuildEnv` separately copies extracted `command` and `file_path` values into `CRUSH_TOOL_INPUT_*` variables before launching a hook. Those raw values can therefore reach the process environment and be visible under same-user inspection even though Ganesh never reads or persists them. Local reduction does not remove this upstream protocol exposure.

The event UUID is the first 128 bits of SHA-256 over `profile NUL session_id NUL tool_use_id NUL phase`, with UUID version 8 and variant bits set. `phase` is `attempt` or `result`. Case is preserved. Success and failure share the result identity, so contradictory outcomes conflict rather than overwriting. Exact reduced retries recover the same record/time. Differences in discarded raw content collapse to the same metadata: this does not prove raw-event identity, equal commands, invocation count or complete transcript coverage. These derived IDs are correlation handles, not authentication or anonymization.

## Deadline and client behavior

The adapter shares the primitive's 750 ms OS-process-start budget, including runtime startup, stdin, reduction, lock wait and sync. It inherits the macOS/CGO process-clock requirement and fails closed on unsupported builds. No handler-local clock resets or network work are added. There is no stdout receipt write to stall. OS scheduling and blocking filesystem calls retain the primitive's documented limits; a nonzero exit can occur after a record became durable, so repeat the same native identity rather than inventing one.

The inspected Crush hook configuration has a per-command `timeout` in seconds and no `async`, `asyncRewake` or `onFailure` fields. Its runner treats empty output with exit 0 or 1 as no decision; exits 2 and 49 have control effects. The adapter's normal and recovered-handler paths return only 0 or 1. The source wrapper skips subagent tool hooks. Session/lifecycle, permission, ancestry, instruction and compaction events are unsupported here. No configuration equivalence to the roadmap's full capture profile is claimed, and no settings are installed or silently relaxed.

The CLI recovers panics in the hook handler's goroutine into silent exit 1 without formatting the panic value or stack. This prevents those panics from becoming Go's default exit 2 and a Crush denial. Recovery does not contain runtime fatal errors, failures before the guard starts, explicit process termination or panics in unguarded other goroutines. The input/reduction worker has its own guard, returning a static failure without formatting a panic value. Uncontained failures may still produce runtime diagnostics or a client control exit; this prototype is not a process supervisor or a universal non-blocking crash boundary. A panic after local publication also remains an ambiguous capture, resolved by exact retry.

If another pre-tool hook denies or halts a call, the inspected `hookedTool` returns before post hooks run. Its attempt record can remain `unknown` indefinitely; absence of a result does not establish denial, cancellation, non-execution or harmless failure. This adapter has no permission-event or reconciliation path to resolve it.

## Validation

Synthetic unit fixtures cover strict reduction, discarded secret-bearing content, stable case-sensitive identities, durable duplicate recovery and conflicting outcomes. Compiled-command tests cover silent success/rejection, supported/unsupported clocks, an unchanged record after retry, rejected contradictory results, unsafe storage, malformed/oversized input, irrelevant flags and blocked stdin. The existing normalized-capture output/deadline tests still run.

Subprocess fixtures exercise the production exit-code dispatcher with success, an error, a secret-bearing panic, `panic(nil)`, a recoverable bounds panic and a reader panic on the input worker. They require empty combined stdout/stderr, exit 0 for success and exit 1 otherwise. Panic injection exists only in test code; the production CLI has no panic trigger. Runtime fatal failures and containment of unguarded goroutines are not tested or claimed.

On 2026-11-01, the source-runner integration fixture passed against the revision above on macOS amd64, Go 1.27.3, CGO enabled. It used the actual Crush `BuildPayload`, hook runner, shell execution and output aggregation with `timeout: 1`, plus synthetic events. Six notifications (attempt, duplicate attempt, success, conflicting failure, distinct failure and invalid session) produced exactly three reduced records and no aggregate decision, halt, context or rewrite. Contradiction and invalid-session callbacks were non-blocking errors with empty adapter stderr. No agent/model/provider, installed binary, real tool effect, user transcript or client configuration was exercised. This is source-runner evidence, not installed-client acceptance or a latency percentile.

To reproduce with that local source checkout and cached Go dependencies:

```sh
python3 integration/crush/test_runner.py /path/to/crush/source
```

The optional harness verifies the tracked revision/clean state, builds Ganesh, and adds a synthetic test through Go's temporary overlay mechanism. It does not edit the Crush checkout or fetch modules (`GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`). Temporary binary/overlay/spool files are removed. A missing dependency, incompatible platform or failing fixture is incomplete evidence, never a substituted pass.

These fixtures are separate from actual installed-client tests, per-installation latency measurements, production secret-scanner admission and S0 product acceptance. SessionEnd/competing-hook measurements remain unknown, and no lifecycle coverage is claimed from tool-only tests.

### Streaming follow-up, 2026-11-01

The native source-runner fixture now sends ten synthetic notifications and checks six retained records, including one large attempt, one large success, its duplicate and one large failure. The added argument/output/error values each contain at least 128 KiB of discarded text. All aggregate decisions remain `none`, with no context/halt/rewrite. The compiled CLI separately tests large values, valid over-cap envelopes, invalid UTF-8 after a large prefix, and a complete JSON object whose stdin never closes; none of the rejected cases can create a successful record. An input-worker panic also produces silent exit 1. The event-ID profile is unchanged, so previously accepted callbacks retain their retry/conflict identity.

One native harness attempt exceeded its 60-second compile/run cap without a test-start message; its cause was not established. That attempt is recorded as a timeout. The harness now prints stage durations and adds a 20-second Go test-runtime limit while keeping the 60-second stage caps and one-second per-hook timeout. The subsequent run passed: Ganesh build 1.257 s, Crush compile/run stage 23.808 s, actual fixture test 0.81 s. These times include test/build machinery as labelled and are not per-hook latency percentiles.

The latest 15-second fuzz session completed 384,598 executions with two workers and no observed mismatch. Its independent `encoding/json` syntax/token oracle uses `UseNumber` so that float64 range does not impose meaning on discarded JSON numbers; raw UTF-8 and container depth are checked separately. Fuzzing is bounded evidence, not exhaustive parser assurance.

A ten-iteration-per-case benchmark on macOS amd64, Go 1.27.3 measured:

| Generated discarded output | Allocated bytes/op | Allocations/op | Reduction time/op |
| --- | --- | --- | --- |
| 1 KiB | 6,764 | 42 | 39,843 ns |
| 128 KiB | 6,764 | 42 | 1,182,919 ns |
| 1 MiB | 7,338 | 44 | 8,879,618 ns |

The reader generates repeated bytes rather than constructing the large value. Allocation includes that reader setup, bounded control fields and metadata/ID construction. It excludes process startup, storage and client envelope construction, and measures allocation volume rather than maximum RSS. No whole-payload allocation growth was observed in these cases. The skipper uses a 4 KiB read buffer, bounded retained control strings, fixed string-validation scratch space and at most 64 container levels. It does not build discarded scalar strings or objects. The selected Go build's standard JSON skip path still buffers scalar string tokens, so this bounded grammar skipper is checked against independent standard-library parsing rather than claiming `SkipValue` alone provides that memory behavior.

Reproduce the allocation sample with:

```sh
go test ./internal/adapter/crush -run '^$' -bench '^BenchmarkReduceStream$' -benchmem -benchtime=10x
```

Build, vet, normal/race/CGO-disabled suites and the native fixture pass. Limits and withheld context remain visible: no installed capture profile or product gate is enabled, no real workload loss rate is known, and over-cap or late input still fails closed.
