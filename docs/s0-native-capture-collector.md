# Synchronous native capture collector

[Issue #79](https://github.com/andikapradanaarif/ganesh/issues/79) follows the [native experiment](s0-native-capture-experiment.md) with reusable internal Go code. The [collector](../internal/adapter/crush/collector.go) is an experimental adapter boundary, not an installed hook, public library API or supported capture profile. The closed experiments are unchanged; this implementation introduces no new timing study.

`crush.NewCollector(capture.Spool{Directory: privateDirectory})` returns one collector. `Capture(ctx, build)` synchronously invokes an admitted native payload builder, streams its result through the existing strict reducer and appends the reduced metadata to the existing durable spool. The builder receives the same context as storage and must perform synchronous serialization only, with no detached work, context retention or side effects. Reader ownership remains with the caller; the collector does not close caller-owned readers.

The caller must supply a deadline **before entering the collector and before serialization**. The collector carries that deadline unchanged, checks cancellation/expiry between stages and before returning a receipt, and rejects a missing deadline. It selects neither the deadline duration nor its timing anchor. The CLI's 750 ms OS-process-start rule remains unchanged; this code does not import its process clock, watchdog or exit handling. A future native profile still needs its own approved timing/integration contract.

Each collector admits at most one call at a time. An overlapping eligible call immediately returns `ErrCollectorBusy` without invoking its builder or waiting in a queue. Reuse the same collector where that admission limit is needed; separate instances do not share it. There are no collector-created goroutines, workers, subprocesses, implicit retries or background capture. Busy rejection loses the event unless a caller explicitly replays it; it is not delivery or coverage evidence.

## Receipt and failure contract

A successful receipt must match the reduced event identity, carry a nonzero capture timestamp and retain `captured_local / unknown / incomplete`. The collector rejects unbound receipts and attempts to promote local capture into service acceptance, verified origin or complete coverage. The [spool contract](s0-local-capture.md) still determines durability, private modes, capacity, exact replay and conflicting metadata. UUID shape and local receipts do not authenticate a client or establish semantic truth.

| Outcome | Result |
| --- | --- |
| Valid capture or exact replay | Original local receipt; exact replay has `duplicate: true`. |
| No caller deadline | Zero receipt and `ErrCollectorDeadline`. |
| Overlapping call on this collector | Zero receipt and `ErrCollectorBusy`; no serialization or append. |
| Cancelled or expired caller context | Zero receipt and the standard context error. |
| Invalid native envelope | Zero receipt and the existing static `ErrPayload`. |
| Known storage rejection | Zero receipt and the exact static storage sentinel. |
| Builder/reader/storage panic, unknown error or invalid receipt | Zero receipt and static `ErrCollector`; no panic body or wrapped diagnostic. |

Every error returns a zero receipt, even if a store returns success after cancellation/deadline or a panic follows publication. **Zero receipt does not mean no durable effect.** An existing record may require exact replay; the collector neither silently deletes it nor invents an acknowledgment. Wrapped storage errors are normalized rather than exposed, so their private diagnostic text cannot escape via this API. No output, logging or client decision/context channel is added.

## Cancellation limitation

The boundary is deliberately synchronous. Cancellation is checked between operations; it cannot interrupt every builder, reader or filesystem syscall. A blocked append can keep `Capture` blocked past the context deadline. It keeps this collector busy until it actually returns; cancellation does not release the admission guard or detach unfinished work. Once the append returns, an expired/cancelled context suppresses its receipt.

Consequently this API **does not guarantee a one-second callback return** and does not solve stalled-I/O isolation in a live client. The earlier experiment's whole-host SIGKILL is not reused. Installing this collector as an unqualified hook would leave that host-safety requirement unresolved. Capture profiles remain disabled and [#34](https://github.com/andikapradanaarif/ganesh/issues/34) remains open.

## Executable evidence and next gate

[Collector tests](../internal/adapter/crush/collector_test.go) exercise private synthetic spool storage/replay/conflict, strict native admission and byte bounds, unsafe directories/capacity, unchanged context/deadline before serialization, missing/expired/cancelled contexts, cancellation during build/read/storage, late successful append rejection, overlapping calls, panic/error privacy, invalid receipts and gate release after failure. The non-cooperative append fixture stays blocked after cancellation and returns only when explicitly released. It proves the documented limitation rather than claiming hard cancellation safety. Unit deadlines orchestrate correctness; they are not latency samples, percentiles or an extension of the closed 32-callback experiment.

Validation passes: affected normal/race tests and vet, affected CGO-disabled tests, `go build -p=1 ./...`, source formatting, relative document links and diff checks. These checks neither run a native hook host nor qualify a process clock or capture profile. The original reducer/spool, CLI and archived experiment sources remain unchanged.

No client source/configuration is modified, and no new hook-host or collector performance subprocess is launched. Production activation, full-client lifecycle/competing hooks, provenance/reconciliation and unattended operation remain unproven. [Calibration/evaluation #60](https://github.com/andikapradanaarif/ganesh/issues/60) still needs independent labels and a permitted alternate-family grader. This work earns no service, narrative or S0 stage-gate claim. The approved [execution sheet](s0-execution-sheet.md) and [roadmap](personal-second-memory-roadmap.md) are unchanged.
