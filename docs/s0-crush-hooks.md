# S0 Crush tool-hook adapter

Tasks: [#24](https://github.com/andikapradanaarif/ganesh/issues/24), streaming follow-up [#26](https://github.com/andikapradanaarif/ganesh/issues/26), permission observations [#28](https://github.com/andikapradanaarif/ganesh/issues/28), turn observations [#30](https://github.com/andikapradanaarif/ganesh/issues/30), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6).

`ganesh hook crush --spool DIR` receives a bounded native Crush tool-hook object on stdin, reduces it locally and commits only the existing [capture metadata](s0-local-capture.md). The command has empty stdout and stderr on success, handled failure and recovered handler panic. Exit 0 follows durable local storage; exit 1 means rejected, failed or ambiguous capture. No receipt JSON, context, permission decision, retry instruction or tool rewrite is intentionally emitted. Unrecoverable process-failure limits are described below.

This is a prototype adapter, with no hook installation or project enrollment. Every stored record remains `captured_local / unknown / incomplete`. Discovery still reports unsupported capture profiles. File provenance, exact read effects, closure/reconciliation, upload and the full P01 matrix remain pending. The approved roadmap and execution sheet are unchanged.

## Inspected contract

The profile `crush-tool-hooks-v1-aed2bae` is based on the owner's Crush fork source at [`aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f`](https://github.com/andikapradanaarif/crush/tree/aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f), specifically `internal/hooks/input.go`, `events.go`, `config.go`, `runner.go`, `internal/agent/hooked_tool.go`, `internal/app/hooks.go`, `internal/permission/permission.go` and `internal/agent/turn_hooks.go`. It names a source schema, not an authenticated build claim.

The installed executable reports `v0.98.3-fork.0.20261027102508-630a3a9dbe00+dirty`; its build information reports revision `630a3a9dbe0095a0d06473d3e50889487a31a00e` and `vcs.modified=true`. It is not proven to match the inspected source. No installed-client or real-session acceptance is claimed.

Only these native events are admitted:

| Native event | Stored event / outcome | Meaning |
| --- | --- | --- |
| `PreToolUse` | `tool_attempt / unknown` | The runner reported an attempt; execution is not established. |
| `PostToolUse` | `tool_result / succeeded` | The runner reported success; file effects or semantic correctness are not verified. |
| `PostToolUseFailure` | `tool_result / failed` | The runner reported failure; partial effects are unknown. |
| `PermissionRequest` | `permission_request / observed` | The client reported showing a prompt for this call. |
| `PermissionResult` | `permission_result / granted`, `denied` or `cancelled` | The client reported a prompt outcome; execution and final permission state are unknown. |
| `TurnEnd` | `turn_stop / observed` | The client reported a normal turn ending; task success and session closure are unknown. |
| `StopFailure` | `turn_stop / failed` | The client reported a non-cancellation turn error; its cause and partial effects are unknown. |
| `Interrupt` | `turn_stop / interrupted` | The client reported a turn interruption; session closure and absence of effects are unknown. |

All admitted events require matching `event` and `hook_event_name`, a lowercase version/variant session UUID and nonempty string `cwd`. Tool and permission events additionally require a bounded ASCII call handle, nonempty string `tool_name` and object `tool_input`. `PostToolUse` requires string `tool_response`; `PostToolUseFailure` permits string `error` (including its documented omission when empty). Permission events forbid both result/error fields. `PermissionResult` requires exactly one string `decision` equal to `granted`, `denied` or `cancelled`; every other admitted event forbids `decision`. The native `allow`/`deny` output-control spellings are not accepted as input outcomes. Turn notifications forbid all tool/permission fields: `TurnEnd` permits optional string `last_assistant_message`, `StopFailure` permits optional string `error`, and `Interrupt` permits neither. Empty optional strings are allowed; `null` is rejected. Assistant text is forbidden on tool/permission events. Unknown top-level fields, duplicate top-level keys, malformed types/encoding, mismatched aliases, unhandled events, trailing objects and bodies over the raw native cap below are rejected. Arbitrary nested tool input is discarded as a whole, not interpreted. Known credential prefixes are rejected in the call-handle field; this is not a general secret scanner or a promise to detect every secret encoded as an ID.

The native command now streams the **entire raw envelope** under a separate **4 MiB** limit, including leading/trailing whitespace. It validates and discards large strings and nested values instead of assembling the raw body or tool output in memory. JSON container depth is limited to **64**, including the root; encoded root field names are limited to **256 bytes**, and encoded retained event/alias/session/call/decision strings to **1,024 bytes**, including quotes. The normalized `ganesh capture` input limit remains **64 KiB**. These are prototype admission choices, not measured coverage promises.

Crush embeds full tool output in `PostToolUse`. The earlier 64 KiB raw-input gate therefore rejected otherwise small metadata from larger tool results. The streaming path admits those results within its new bounds; over-cap, malformed, truncated, trailing or over-depth input still fails before a successful record is written. It requires genuine EOF and never accepts a cut-off prefix. A slow or unclosed input still exhausts the original 750 ms process budget. The producer still constructs its native envelope and environment; only Ganesh's reduction is streamed. No real-traffic loss rate has been measured.

The skipper accepts JSON grammar and UTF-8 while retaining only bounded control strings. Discarded nested object member names may repeat, matching the earlier `encoding/json` behavior; duplicate root names, including escaped aliases, remain rejected. Escaped lone surrogates follow `encoding/json` v1 syntax/replacement behavior. Provider call handles containing characters outside `[a-zA-Z0-9_-]` (for example `.`, `:` or `/`), or longer than 160 bytes, still cause rejection.

Paths, tool names, inputs, outputs, assistant text, errors and the original call handle are never stored or printed. Raw hook bodies are not hashed or retained. The adapter does not read environment values, files, transcripts, credentials, models or the network. A dedicated spool is required for this profile and installation; it must not be shared across unrelated clients/installations.

Crush's `BuildEnv` separately copies extracted `command` and `file_path` values into `CRUSH_TOOL_INPUT_*` variables before launching a hook. Those raw values can therefore reach the process environment and be visible under same-user inspection even though Ganesh never reads or persists them. Local reduction does not remove this upstream protocol exposure.

The event UUID is the first 128 bits of SHA-256 over `profile NUL session_id NUL tool_use_id NUL phase`, with UUID version 8 and variant bits set. `phase` is `attempt`, `result`, `permission_request`, `permission_result/` followed by the exact admitted outcome, or `turn/` followed by the native ending type. Turn markers use an empty call component because no turn/call ID is supplied. Case is preserved. Success and failure share the result identity, so contradictory outcomes conflict rather than overwriting. Exact reduced retries recover the same record/time. Differences in discarded raw content collapse to the same metadata: this does not prove raw-event identity, equal commands, invocation count or complete transcript coverage. These derived IDs are correlation handles, not authentication or anonymization. Permission identity semantics are bounded as described below.

## Deadline and client behavior

The adapter shares the primitive's 750 ms OS-process-start budget, including runtime startup, stdin, reduction, lock wait and sync. It inherits the macOS/CGO process-clock requirement and fails closed on unsupported builds. No handler-local clock resets or network work are added. There is no stdout receipt write to stall. OS scheduling and blocking filesystem calls retain the primitive's documented limits; a nonzero exit can occur after a record became durable, so repeat the same native identity rather than inventing one.

The inspected Crush hook configuration has a per-command `timeout` in seconds and no `async`, `asyncRewake` or `onFailure` fields. Its runner treats empty output with exit 0 or 1 as no decision; exits 2 and 49 have control effects. The adapter's normal and recovered-handler paths return only 0 or 1. The source wrapper skips subagent tool hooks. SessionStart/SessionEnd, TurnStart, ancestry, instruction and compaction events remain unsupported. Turn-ending observations are scoped below and do not establish a complete lifecycle. No configuration equivalence to the roadmap's full capture profile is claimed, and no settings are installed or silently relaxed.

The CLI recovers panics in the hook handler's goroutine into silent exit 1 without formatting the panic value or stack. This prevents those panics from becoming Go's default exit 2 and a Crush denial. Recovery does not contain runtime fatal errors, failures before the guard starts, explicit process termination or panics in unguarded other goroutines. The input/reduction worker has its own guard, returning a static failure without formatting a panic value. Uncontained failures may still produce runtime diagnostics or a client control exit; this prototype is not a process supervisor or a universal non-blocking crash boundary. A panic after local publication also remains an ambiguous capture, resolved by exact retry.

If another pre-tool hook denies or halts a call, the inspected `hookedTool` returns before post hooks run. Its attempt record can remain `unknown` indefinitely; absence of a result does not establish denial, cancellation, non-execution or harmless failure. The added permission notifications cover only prompts actually reported by the client. Earlier hook denials may emit none, and reconciliation remains absent.

## Validation

Synthetic unit fixtures cover strict reduction, discarded secret-bearing content, stable case-sensitive identities, durable duplicate recovery and conflicting outcomes. Compiled-command tests cover silent success/rejection, supported/unsupported clocks, an unchanged record after retry, rejected contradictory results, unsafe storage, malformed/oversized input, irrelevant flags and blocked stdin. The existing normalized-capture output/deadline tests still run.

Subprocess fixtures exercise the production exit-code dispatcher with success, an error, a secret-bearing panic, `panic(nil)`, a recoverable bounds panic and a reader panic on the input worker. They require empty combined stdout/stderr, exit 0 for success and exit 1 otherwise. Panic injection exists only in test code; the production CLI has no panic trigger. Runtime fatal failures and containment of unguarded goroutines are not tested or claimed.

On 2026-11-01, the source-runner integration fixture passed against the revision above on macOS amd64, Go 1.27.3, CGO enabled. It used the actual Crush `BuildPayload`, hook runner, shell execution and output aggregation with `timeout: 1`, plus synthetic events. Six notifications (attempt, duplicate attempt, success, conflicting failure, distinct failure and invalid session) produced exactly three reduced records and no aggregate decision, halt, context or rewrite. Contradiction and invalid-session callbacks were non-blocking errors with empty adapter stderr. No agent/model/provider, installed binary, real tool effect, user transcript or client configuration was exercised. This is source-runner evidence, not installed-client acceptance or a latency percentile.

To reproduce with that local source checkout and cached Go dependencies:

```sh
python3 integration/crush/test_runner.py /path/to/crush/source
```

The optional harness verifies an exact allowlisted revision and clean tracked state, builds Ganesh, and adds synthetic tests through Go's temporary overlay mechanism. It does not edit the Crush checkout or fetch modules (`GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`). Temporary binary/overlay/spool files are removed. A missing dependency, incompatible platform or failing fixture is incomplete evidence, never a substituted pass.

These fixtures are separate from actual installed-client tests, per-installation latency measurements, production secret-scanner admission and S0 product acceptance. SessionEnd/competing-hook measurements remain unknown, and the earlier tool-only tests establish no turn or session-closure coverage.

### Streaming follow-up, 2026-11-01

The #26 native source-runner fixture sent ten synthetic notifications and checked six retained records, including one large attempt, one large success, its duplicate and one large failure. The added argument/output/error values each contain at least 128 KiB of discarded text. All aggregate decisions remain `none`, with no context/halt/rewrite. The compiled CLI separately tests large values, valid over-cap envelopes, invalid UTF-8 after a large prefix, and a complete JSON object whose stdin never closes; none of the rejected cases can create a successful record. An input-worker panic also produces silent exit 1. The event-ID profile is unchanged, so previously accepted callbacks retain their retry/conflict identity.

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

### Permission-observation contract

The inspected `permissionHooks` adapter serializes `req.Params` into tool input, copies `SessionID`, `ToolCallID` and `ToolName`, and sets `Decision` from the `permission.Outcome` enum. It omits `PermissionRequest.ID`. The supported subset requires the same valid session/call handles and object input as tool events; nil params serialize as `null` and remain unsupported. This is a deliberate coverage gap, not a clean session claim.

The dedicated request fields `Action`, `Description`, `Path`, `Danger` and `GrantKey` are not forwarded. Related data may occur inside `Params`, but Ganesh discards it with the other native content. Stored observations cannot reconstruct what permission was requested.

Because the payload has no prompt ID, each session/call retains at most one shown-prompt marker and one record for each of `granted`, `denied` and `cancelled`. Exact repeats recover existing records. Multiple outcomes may belong to separate prompts for the same call; they receive distinct fixed enum phases and coexist. They do not establish prompt count/order, which grant is current, independent corroboration or a resolved contradiction. Captured timestamps are local storage times, not prompt occurrence times. No result requires a previously captured request, because missing/reordered delivery is possible.

`wrapToolsWithHooks` skips subagent tool hooks, while the global permission observer can still emit their prompt notifications. Composite child-session IDs containing `$$` are rejected by the UUID requirement. A notification carrying a valid parent-session UUID and call handle can be retained without a matching attempt/result. That observation establishes neither subagent coverage nor parent/child ancestry.

The permission service's observer covers shown prompts. Auto-approved calls can bypass it; a pre-tool hook may block execution before the permission service is reached. Missing permission callbacks therefore say nothing about permission, execution or complete coverage. `cancelled` describes an abandoned prompt, not proof of a turn/session interruption or that no partial tool effects occurred. A reported grant cannot widen Ganesh grants, trigger a tool, certify a fact or make narrative eligible. The adapter continues to emit no hook decision or context.

Metadata schema v1 keeps its five-field JSON shape with additive `permission_request`/`permission_result` events. Only `permission_request/observed` and `permission_result/{granted,denied,cancelled}` are admitted; the new `granted`/`cancelled` outcomes are rejected for old event types. Existing record and tool-ID semantics are unchanged. New readers admit old records; old binaries reject the new enum values. A rollback cannot silently interpret or repair them as older tool events.

Fixed independent hash fixtures, strict event/outcome pair tests and reopened-spool retry tests cover the new contract. The compiled CLI verifies one shown-prompt marker plus all three distinct outcomes, exact repeats, cross-event rejection and no native content or decision output. Existing tool success/failure conflict tests still pass. A 15-second permission-decision fuzz run completed 35,085 executions against independently specified event/enum expectations with no observed mismatch.

The #28 expanded pinned source-runner fixture passed on 2026-11-01: 20 synthetic notifications yielded ten records, including one shown-prompt marker and three outcomes for the same call, without context, halt, rewrite or approval/denial output. Missing outcomes and `allow` as an input outcome were non-blocking failures with empty adapter stderr. The fixture runtime was 1.43 s across all notifications; this is not per-hook latency or installed-profile evidence.

Build, vet, normal and race suites pass. The first CGO-disabled suite attempt exceeded the outer three-second subprocess timeout before returning its expected unsupported-clock rejection; its cause was not established. An isolated `CGO_ENABLED=0 go test ./... -count=1` passed with unchanged deadlines and confirmed no store on the unsupported build. Both attempts are retained in this evidence summary. These tests do not install hooks, exercise real prompts or establish the permission service's runtime behavior on an enrolled client.

### Turn-ending observation contract

The inspected `turnEndEvent` helper emits `Interrupt` when `errors.Is(err, context.Canceled)`, `StopFailure` for any other non-nil error, and `TurnEnd` otherwise. A deadline or another error reaching this helper is not automatically an interruption. Error strings may be empty and omitted by the serializer. Normal endings may have an assistant message or omit it when no message is available. These source rules identify a reported ending type, not the semantic success of the work or a proven provider-specific failure.

The native payload has no turn ID, turn-start binding or callback identity. Ganesh keeps at most one marker of each ending type per session: the phase is `turn/TurnEnd`, `turn/StopFailure` or `turn/Interrupt`, with an empty call component. Repeated same-type endings, even with different assistant/error text, recover the first record. Different types coexist and may describe different turns. Counts, chronology, matching starts, current turn state and individual retry identity are unrecoverable. Storage timestamps are not turn occurrence times.

`fireTurnHook` skips calls marked `NonInteractive`; this does not provide general subagent capture. A turn ending does not terminate the session, prove transcript flush or reconcile input coverage. Markers stay `captured_local / unknown / incomplete`, with no clean-prefix or narrative-availability exception. The existing metadata-v1 `turn_stop` and outcomes are reused; existing tool/permission IDs and validation rules remain unchanged.

The pinned producer has a specific cancellation gap: its success path passes a live caller context to `fireTurnHook`, so cancellation racing finish can suppress `TurnEnd` before Ganesh receives input. The error-ending path already detaches cancellation. The producer fix is implemented in [Crush PR #423](https://github.com/andikapradanaarif/crush/pull/423), tracked by [Crush #422](https://github.com/andikapradanaarif/crush/issues/422); it is separate from Ganesh's input admission. The original `aed2bae` baseline still contains the gap. A candidate patch does not update the installed client or enable a capture profile. A missing callback cannot be reconstructed or relabelled as interruption by Ganesh.

The coarse set, collapse of absent/empty discarded text and exclusion of `TurnStart` are intentional scope/privacy choices. The normalized development primitive also retains its legacy enum grammar, including `turn_stop / succeeded`; this adapter never emits that pair, and it provides no task-success or publication evidence. Tightening old record admission would require an explicit compatibility change.

Fixed Python-computed identities, field-isolation, large discarded-text and reopened-spool fixtures pass. The compiled CLI checks omitted/empty/large optional text, duplicate markers, separate tool/permission records, foreign-field rejection, silent exits and unchanged unknown/incomplete labels. A 15-second fuzz run completed 199,302 executions with two workers against independent event/optional-field expectations, with no observed mismatch.

The #30 pinned source-runner fixture passed on 2026-11-01: 28 synthetic notifications produced thirteen records, including one marker for each ending type. Omitted and large source optional fields work; repeated same-type callbacks collapse. Invalid turn/tool mixtures fail with exit 1 and empty adapter stderr. Every aggregate remains without decision/context/halt/rewrite. Fixture runtime was 1.41 s across all callbacks; the compile/run stage was 10.751 s. These are labelled fixture timings, not per-hook latency percentiles or installed-profile evidence.

Build, vet, normal/race/CGO-disabled suites, local link and diff checks pass. Source inspection and synthetic runner checks do not exercise a real agent turn, its producer/error paths, installed-client acceptance or task outcomes. Session closure, full lifecycle coverage, reconciliation and narrative eligibility remain pending.

### Cancellation-race repair and dependency evidence

The producer patch centralizes `context.WithoutCancel` for `TurnEnd`, `StopFailure` and `Interrupt` in `fireTurnHook`. Both producer paths use that rule. `TurnStart` remains cancellation-sensitive, `NonInteractive` exclusion stays in place, and the hook runner still enforces configured timeouts. Regression cases directly exercise cancelled/expired ending dispatch; the old helper produces no marker, while the patch delivers each ending type. Additional controls check cancelled starts, existing non-interactive behavior and the ending hook's own one-second timeout.

The integration harness accepts exactly two source commits: original baseline `aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f` and candidate repair `d05cce5db2b09f3b4a89f3359b1953c96207e9b1`. There is no arbitrary-revision override. Both keep the same wire schema and Ganesh identity namespace. The added agent-package overlay calls the actual producer dispatcher with cancelled/expired contexts and invokes Ganesh as its hook command. A test-only, content-free probe records command invocation before Ganesh runs: the baseline must leave both probe and spool absent, distinguishing producer suppression from consumer rejection. That expected counterexample is not corrected-producer qualification. For the repair it must invoke the command and produce all three reduced ending markers in each of two isolated cancelled/expired-context cases. Separate stores prevent per-session deduplication from hiding a missing callback in either case. No assistant/error text is retained and no origin/coverage upgrade is allowed. It uses no model or real session.

Validation completed against both immutable source commits. The baseline producer-to-consumer fixture reproduced missing `TurnEnd` callbacks for cancelled and expired callers. The repair delivered all three reduced markers in each isolated context case (six records across the two stores), with unchanged source/session IDs and unknown/incomplete labels. The repaired ending-dispatch fixture took 0.43 s across its calls; it exercises the actual dispatcher and Ganesh command, not a real model turn.

The ordinary 28-notification transport fixture retained thirteen records in both initial runs. The baseline run observed one one-second host timeout on an initial tool-attempt callback; the repair run observed an initial tool-attempt rejection with an unconfirmed cause. Their explicit duplicate callbacks recovered the missing records. A subsequent baseline run with the invocation probe reproduced producer suppression but failed the ordinary transport fixture: two host timeouts left twelve records instead of thirteen, with three failed callbacks. Causes beyond the observed timeouts were not established. These are recorded capture failures, not zero-loss or installed-profile acceptance. The harness now serializes packages with `-p=1` to isolate timed capture tests from other package compilation/tests; per-hook, process and stage deadlines are unchanged. Separate cancelled/expired-case stores prevent duplicate recovery from hiding a missing ending callback in the repair regression.

The final serialized runs pass against both pins with the invocation probe enabled: the baseline leaves the probe and spool absent for both caller contexts; the repair produces all six expected records. Both ordinary transport fixtures retain thirteen records. Ending-dispatch fixture runtime is 0.01 s for the baseline and 0.35 s for the repair; complete compile/run stages take 20.747 s and 24.850 s respectively. These are fixture observations under the serialized test load, not latency percentiles or evidence that the earlier capture failures are eliminated.

Crush's targeted turn-hook tests pass normally, with the race detector and with CGO disabled; its hook-engine suite, agent/hook vet checks and CGO-disabled client build also pass. Six direct ending-dispatch regression cases failed before the patch and pass after it. Start cancellation, non-interactive exclusion and configured-timeout controls remain green. This is scoped evidence, not a full-repository test claim.

The repair PR and these fixtures do not merge source code, replace an installed executable, establish full capture, or enable narrative. The remaining review observations are retained contract choices: per-session sets and absent/empty collapse preserve bounded metadata/privacy; `TurnStart` is outside this slice; the legacy normalized enum grammar is unchanged.
