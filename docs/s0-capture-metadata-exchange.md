# Local capture metadata exchange

[Issue #81](https://github.com/andikapradanaarif/ganesh/issues/81) adds an internal [wire codec](../internal/captureipc/frame.go) and [one-shot Unix stream client](../internal/captureipc/client.go). This is a component for a possible storage-isolation path, not a worker, daemon, installed hook or service. The [synchronous collector](s0-native-capture-collector.md) remains unchanged and still cannot interrupt blocked filesystem I/O.

A future composition could set an approved caller deadline, reduce the native event locally, connect to an independently admitted worker, exchange metadata and wait for that worker's durable receipt. This issue implements only the metadata wire/exchange component. It neither moves the current collector's append into another process nor qualifies that future composition. No worker or application integration is built or started.

## Connection and deadline ownership

`captureipc.NewClient(conn)` takes exclusive ownership of an already connected `*net.UnixConn`. Only Unix **stream** sockets are admitted. The caller must establish the connection and independently authorize/authenticate the peer before handing it over. There is no dial, path selection, peer-credential check, enrollment or worker lifecycle in this package. Do not wrap the same connection in multiple clients or continue using it directly. `Close` releases an unused connection and can interrupt an active exchange.

The first `Submit(ctx, metadata)` consumes and closes the connection, even when context/metadata admission fails. Subsequent or concurrent submissions return `ErrUsed` without disturbing the first call. The client sends no retries, opens no replacement connection and accepts only one response frame. Bytes after that frame cannot become an acknowledgment for a future request because the connection is closed and never reused.

A caller deadline must exist before `Submit`. It is carried unchanged onto the socket; the package chooses neither its duration nor its timing anchor. A complete future capture profile must include local reduction and connection establishment in its approved budget too. The CLI's OS-process-start rule is unchanged. A missing deadline or context already cancelled/expired at admission prevents dispatch.

An `AfterFunc` cancellation callback advances the socket deadline to interrupt pending read/write I/O, including when the original deadline is far in the future. This is descriptor cleanup, not asynchronous capture or storage. The callback may finish concurrently with connection closure; standard Unix connection operations support that concurrency. Context errors are rechecked after write/read/receipt decoding and suppress late success. Socket scheduling and the final context-check/return race prevent an atomic real-time guarantee.

## Version-one frames

Each direction uses a four-byte unsigned big-endian length followed by **1–4,096 body bytes**, at most 4,100 bytes including the header. Zero/oversized lengths reject before body allocation. Partial headers/bodies and disconnected peers yield a static exchange error. Frame boundaries use exact reads; writes handle partial progress. No response EOF is required, so a peer cannot hold a valid reply open merely by withholding connection closure.

The request body is the complete canonical JSON encoding of validated `capture.Metadata`: `schema_version`, `event_id`, `session_id`, `event`, `outcome`, in the existing Go struct order. Only the existing reduced metadata schema is accepted. Raw native envelopes, tool arguments, outputs and transcript text are not inputs to this API. Source authority and event-ID relationships remain separate caller responsibilities; syntax does not authenticate an event.

The response is one flat JSON object with exactly eight fields:

| Field | Required value |
| --- | --- |
| `schema_version` | JSON integer token `1`. |
| `request_sha256` | Lowercase SHA-256 of the full canonical request body. |
| `event_id` | Exact requested event ID. |
| `captured_at` | Nonzero canonical UTC RFC3339Nano timestamp. |
| `state` | `captured_local`. |
| `origin` | `unknown`. |
| `coverage` | `incomplete`. |
| `duplicate` | Explicit JSON boolean. |

Object field order may vary. Duplicate keys (including escaped spellings), unknown/missing/case-aliased keys, nulls, wrong types, invalid UTF-8, nested values and trailing JSON values/garbage within the frame reject. Canonical UTC timestamps and the integer version token are deliberately specific to this version. `EncodeReceipt` validates and encodes a peer declaration; it performs no storage. Future schema evolution needs an explicit protocol change, not silent tolerance.

The digest binds the full request, including outcome/session fields, rather than the event ID alone. This prevents accepting a reply for conflicting metadata sharing an event ID. **It is an unkeyed content digest, not authentication or evidence of durability.** A peer that sees the request can fabricate a matching receipt. The client validates only the declaration's format, request binding and local unknown/incomplete labels. Treating it as evidence of a durable capture requires separately established peer trust and a correctly implemented durable worker; neither is supplied here.

## Failure and evidence limits

All failures return a zero receipt. Missing deadlines produce `ErrDeadline`; consumed clients produce `ErrUsed`; invalid metadata/frames produce `ErrFrame`; other connection failures and recovered panics produce static `ErrExchange`. Standard context cancellation/expiry errors pass through. Underlying socket paths, parser diagnostics, raw bytes, panic bodies and error wrappers are not returned or logged. The completion guard also handles `panic(nil)` under legacy `GODEBUG=panicnil=1`.

A timeout, cancellation or disconnect **after sending** is ambiguous about peer effects. The peer may finish later or already have published a record; closing the socket does not cancel its storage syscall or delete a capture. An explicit retry on a new connection would need the same admitted metadata and the worker's idempotency contract. This package neither performs that replay nor establishes automatic recovery. It adds no queue, source trust, provenance, permission or publication promotion.

[Codec tests](../internal/captureipc/frame_test.go) cover framing limits, one-byte reads, truncation, malformed/ambiguous receipt fields, full-request digest binding, promoted state rejection and content-free I/O failures. [Local socket tests](../internal/captureipc/client_test.go) use only private synthetic peers: normal/duplicate declarations, single-use/overlap, admission without dispatch, partial/disconnected/wrong replies, explicit cancellation with a far-future deadline, partial-response expiry, runtime error privacy and nil panics. In the blocked-peer fixture the client returns a zero receipt on cancellation while the fake storage stage remains blocked; releasing that stage later cannot change the client's result.

Validation passes: normal/race/CGO-disabled package tests, the legacy nil-panic regression, scoped vet, `go build -p=1 ./...`, formatting, relative links and diff checks. Existing reducer, spool, collector, CLI, archived experiment sources and owner-supplied baseline documents remain unchanged.

These are deterministic correctness tests on local sockets, not startup measurements, population latency claims, process-isolation evidence or a new use of the closed native experiment's attempt allowance. Test listeners are temporary and private; no production listener command, worker process or installed profile exists. Actual worker identity/lifecycle, bounded resources across process failure, durable acknowledgment/replay and full-client operation remain open. Capture profiles stay disabled; [#34](https://github.com/andikapradanaarif/ganesh/issues/34), [calibration #60](https://github.com/andikapradanaarif/ganesh/issues/60) and S0 remain incomplete. The [execution sheet](s0-execution-sheet.md) and [roadmap](personal-second-memory-roadmap.md) are unchanged, and no service or narrative activation follows.
