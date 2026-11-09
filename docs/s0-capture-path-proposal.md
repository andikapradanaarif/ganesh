# Capture path proposal

[Issue #75](https://github.com/andikapradanaarif/ganesh/issues/75), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6), prepares a next step for unresolved [#34](https://github.com/andikapradanaarif/ganesh/issues/34). **Proposal only. No new experiment, client modification or timing-contract change is authorized by this document.** The approved [execution sheet](s0-execution-sheet.md) and [roadmap](personal-second-memory-roadmap.md) remain unchanged.

## Problem and evidence

The [startup study](s0-capture-startup.md) observed three first executions spending 754.594–845.194 ms before main-package initialization, already beyond the collector's 750 ms allowance. Two other instrumented first executions were killed without phase observations. The [standalone comparison](s0-standalone-capture.md) failed all 20 first executions in each arm. The later [artifact/signing investigation](s0-capture-artifact-preparation.md) reproduced no rejection and established no signing benefit. These are different studies under different conditions; they cannot be pooled into a reliability rate or a diagnosis of an OS mechanism. Their attempt caps are exhausted.

The useful next question is whether capture can avoid starting a separate collector for every event while retaining reduction, durable receipt and failure isolation. No hosted service or additional safety probe answers that question. Evaluation remains separately blocked by [#60](https://github.com/andikapradanaarif/ganesh/issues/60): independent calibration labels and a permitted alternate-family grading route are still missing. Capture work alone cannot authorize a service build.

## Current invocation and clocks

Read-only inspection uses Crush repair revision [`3378f47a8398c79a10e35cc61c994160cc0ae3ea`](https://github.com/andikapradanaarif/crush/tree/3378f47a8398c79a10e35cc61c994160cc0ae3ea), the same source pin admitted by the [native fixture](s0-crush-hooks.md). This describes that source, not the currently installed client's build or behavior.

| Boundary | Current behavior | Timing consequence |
| --- | --- | --- |
| Crush `Runner.Run` | Matches hooks, builds environment and native JSON payload, then dispatches commands in parallel. | Payload construction precedes the per-command timeout. No current metric establishes the whole producer-to-receipt interval. |
| Crush `Runner.runOne` → `shell.Run` | Executes the configured command once with payload on stdin. Our fixture configures `timeout: 1` and the collector command. | The configured command timeout includes shell/child launch, but is not a guaranteed one-second return bound: the runner can wait an additional one-second `abandonGrace` after cancellation. |
| Ganesh `capturecli.captureInput` | Calls `processclock.Start`, rejects an expired process age, then uses only the remainder of 750 ms. | OS startup counts even when no handler code has run. Moving the clock into a handler would weaken this contract. |
| `crush.ReduceStream` → `capture.Spool.Append` | Validates and discards content, then writes metadata, syncs the file, publishes without overwrite and syncs directories. | A receipt follows durable publication. Cancellation or a lost return can leave a record without an observed acknowledgment. |

Sources: pinned [`Runner.Run` / `Runner.runOne`](https://github.com/andikapradanaarif/crush/blob/3378f47a8398c79a10e35cc61c994160cc0ae3ea/internal/hooks/runner.go); Ganesh [CLI adapter](../internal/capturecli/capture.go), [stream reducer](../internal/adapter/crush/stream.go), [spool](../internal/capture/spool.go) and [source-runner fixture](../integration/crush/runner_test.go.fixture). The runner grace observation is a source fact, not a new timing measurement. Future runs need an independent outer watchdog; configuring one second alone does not prove an overall one-second host bound.

## Alternatives

| Path | Removes per-event collector startup? | Remaining cost or risk | Recommendation |
| --- | --- | --- | --- |
| Existing general or standalone Go command | No. | First-launch headroom remains unresolved. | Preserve as the control; no new build tweak is justified by the completed studies. |
| Worker behind the same Go command | No. | Adds IPC, readiness and crash recovery while retaining the observed startup path. | Do not build this as a startup remedy. |
| Native in-process adapter in the pinned client | Structurally yes, if it directly invokes reduction and spool code without a shell/child. | Client coupling, shared crash domain, cancellation and potentially blocking filesystem calls; a different timing anchor needs owner approval. | Preferred **experimental** candidate, conditional on the decision below. No reliability benefit has been measured. |

A different lightweight IPC sender might avoid the Go command, but would add another implementation and worker lifecycle. It is deferred rather than presumed faster or safer. There is no daemon/uploader proposal in this pass.

## Proposed experiment, after approval

**Proposed ceiling: two focused hours, zero paid API spend, at most 32 synthetic callback attempts and eight client-host process launches.** These are unapproved ceilings, not remaining S0 allowance. Count implementation, preparation, failed builds, validation and analysis inside the effort ceiling. Prior cumulative S0 effort remains unknown. Stop at either ceiling and report incomplete; do not borrow attempts from closed studies or repeat failed callbacks to fill cells.

Use a temporary detached checkout of the exact repair pin. Build a disposable Go source overlay that routes synthetic producer events directly through the existing streaming reducer and spool. Ganesh's `internal` packages cannot be imported by a separate client module: the prototype must explicitly record any copied package files, hashes, import rewrites and overlay diff. Do not publish a new library API or silently replace the reducer with a permissive typed shortcut. Record both repos' revisions, toolchain, host/load, binary and overlay hashes before attempts. Remove the temporary checkout afterward; preserve content-free records privately.

The proposed timing contract is **750 ms from producer capture dispatch, before payload serialization, to observed durable receipt, within an independently enforced one-second callback return bound**. It applies only to this proposed in-process experiment. Preserve the existing executable's OS-process-start rule as the control; report the two anchors separately, never as equivalent measurements. Record client process launch-to-ready and launch-to-first-receipt separately, including failures before readiness. An already running client does not establish reliable client startup or capture of events before registration.

Before implementing or running, owner approval must resolve that anchor change. Then freeze one private protocol with source/configuration bindings, exact cell IDs, ordering, synthetic expected outcomes, fault boundaries, watchdog behavior and stop rules before execution. Reserve the 32 attempts as follows; each deliberate replay is another counted attempt:

| Stratum | Attempts | Required observations |
| --- | ---: | --- |
| First callback | 8 | Four control and four candidate, one per host launch; four matched pairs in counterbalanced order. Record every launch/readiness failure and do not replace a missing callback. |
| Warm callback | 8 | One follow-up in each same host immediately after its first callback attempt; a failed first attempt remains visible. |
| Idle callback | 8 | One further callback in each same host after at least 30 seconds without invoking capture; record the actual gap. This is not overnight-idle coverage. |
| Fault/replay | 8 | Candidate-only: unavailable/disabled adapter; unsafe spool; cancelled dispatch; injected write failure before publication; lost acknowledgment after publication; exact replay of that event; same identity with conflicting metadata; process crash before publication. |

Run the three callback strata in each of the eight host processes, then use remaining live hosts for fault cases and terminate one for the final crash case. If a host dies early, keep the launch and all affected cells failed/unavailable, consume their reserved slots and stop if the protocol cannot proceed. No replacement host launches. Use test-only injection for the post-publication lost acknowledgment and crash boundary; do not describe it as an observed production crash. Predeclare how on-disk state is inspected without executing an extra collector or exceeding the launch ceiling.

Every ordinary callback uses only bounded synthetic native fields. Preserve current size/depth limits, strict admission, metadata identities, private spool modes and capacity, exact-replay/conflict behavior, and `captured_local / unknown / incomplete` labels. Scan all generated artifacts and captured diagnostics for a planted synthetic secret and private path. Persist no raw payload or child output bodies; count unexpected output without saving its content. Capture must never supply a decision, halt, rewritten input/output or agent context. Client source logs and test failure paths need inspection too; silent Ganesh output alone does not establish that boundary.

The in-process adapter must not call `capturecli`, `os.Exit` or the CLI process watchdog. It has no worker: unavailable means the adapter was not registered or the local store cannot accept the event. Record the gap and return no capture success; do not fall back to an unmeasured subprocess, queue raw content, or install a background worker. Catch adapter panics at the adapter boundary without printing payloads. Cancellation cannot interrupt every filesystem syscall: detached work, late writes and shutdown must be bounded or explicitly marked incomplete. A goroutine timeout alone is not proof of cancellation or a safe crash boundary.

For the fault cases, distinguish no record, partial temporary file, published record without acknowledgment, and acknowledged durable record. An exact replay may recover an existing receipt; it does not demonstrate automatic replay by the client or recovery of an event never delivered. If safe fault injection or a bounded callback return cannot be implemented within the ceiling, stop without claiming a supported capture path.

Report counts and nearest-rank p95/p99 by arm and first/warm/idle stratum, all failed/ambiguous attempts, missing phases, launch-to-ready, dispatch-to-receipt and launch-to-first-receipt separately. With four ordinary samples per arm/stratum, these are descriptive observations, not population SLOs. New executable inodes do not establish cold cache. A positive prototype recommendation requires zero candidate ordinary-callback failures, no missing timing observations, every candidate dispatch-to-observed-receipt at most 600 ms, callback return within one second, all fault expectations met and no privacy/control/receipt regression. Even that result leaves installed-client coverage, competing hooks, broader lifecycle capture, provenance and product gates incomplete.

## Owner decision and next action

Review the proposed in-process timing anchor and the two-hour / 32-callback / eight-host-launch ceiling. Approval must explicitly cover both; merging this planning document alone is not experiment authorization. If approved, create an issue-linked experimental branch and freeze the detailed protocol before implementation and attempts. If the OS-process-start rule must remain the only admissible contract, do not implement this candidate; #34 remains unresolved and we need a separately scoped alternative. No requirement or current profile changes in either case.

This preparation performed source inspection and documentation checks only. It produced no latency result, installed profile, service, calibration acceptance or stage-gate pass.
