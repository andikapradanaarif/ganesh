# Local capture metadata peer

[Issue #105](https://github.com/andikapradanaarif/ganesh/issues/105) adds the receiving counterpart to the [metadata client](s0-capture-metadata-exchange.md). The internal [Peer](../internal/captureipc/peer.go) handles caller-admitted connections against the existing [durable spool](s0-local-capture.md). It is a synchronous component, with no listener command, daemon, process supervisor, peer-credential check, client enrollment or installed hook. No process-isolation or capture-timing claim follows.

## Ownership and admission

`captureipc.NewPeer(capture.Spool{Directory: privateDirectory})` configures one reusable peer. Construction opens no files or sockets. `peer.ServeOne(ctx, conn)` takes exclusive ownership of one already connected `*net.UnixConn`, including its deadlines and closure on every return. Nil connections reject; provided connections close even on nil context, unconfigured peer, missing/expired deadline, invalid socket or busy rejection. Do not pass the same connection concurrently or use it again after handoff.

The caller must admit/authenticate the connecting client separately. Only Unix **stream** sockets are accepted. A socket path, UUID, successful append or unkeyed request digest cannot establish client identity, source authority, complete capture or first-party origin. This implementation has no path selection, dial, listener, credentials or enrollment API.

The caller supplies a deadline before entry; the identical context object reaches `Spool.Append`. The peer neither chooses a duration nor receives the remote client's context/deadline through the metadata protocol. A future worker must define its own admitted request budget and lifecycle. The current client's callback budget must also include reduction and connection establishment; neither this method nor a successful socket test qualifies it.

Reuse **one peer instance** to admit at most one exchange at a time. Overlapping calls on other exclusively owned connections return `ErrBusy` without reading or appending and close those rejected connections. No queue or detached append is created. Separate peer instances have separate guards; creating a peer per request would not impose a worker-wide bound. Listener/accept limits, connection admission before this method and process-wide resource control remain future work.

## Request, append and acknowledgment

The wire contract remains version one: a four-byte big-endian length followed by 1–4,096 body bytes. `ServeOne` reads one frame, strictly decodes the existing five-field reduced metadata schema, then requires the bytes to equal its canonical encoding. Whitespace/reordered/escaped alternative encodings reject even if semantically equivalent. Duplicate/unknown/case-aliased/missing/null/wrong-type fields, trailing values, invalid UTF-8, raw tool objects and invalid identity/event/outcome values cannot reach storage. This is deliberately the current client's canonical profile, not a tolerant general JSON endpoint.

After admission, the peer calls the existing `Spool.Append(ctx, metadata)` synchronously. Only a successful append with a matching event ID, nonzero timestamp and `captured_local / unknown / incomplete` labels can produce an acknowledgment. The existing receipt encoder binds the **entire canonical request digest** and emits the same eight-field response the client already checks. No frame/schema change or alternate success format is added.

The spool supplies record/file/directory synchronization, capacity and privacy checks, exact replay and conflicting-identity rejection. A successful peer return means the append returned a valid local receipt and one complete receipt frame was written to the socket. It does not prove that the client observed the frame. Exact replay on a new explicitly admitted connection preserves the original timestamp with `duplicate: true`; conflicting metadata cannot overwrite the record. The peer and client perform no replay automatically.

Each connection is consumed after the first request/response. There is no second request loop, EOF wait, request drain or error frame. Additional pipelined data never reaches a second append; closing a connection with unread data may discard output. Use a new admitted connection for a later explicit attempt. Do not infer failed storage from a failed exchange or dropped acknowledgment.

## Cancellation, failures and limits

The socket deadline is set from the supplied context, and cancellation advances it through `context.AfterFunc` to interrupt pending descriptor I/O. The callback performs no capture/storage work. The peer checks cancellation/expiry before I/O, after reading, before append, after append, before reply and after writing. Standard cancellation/expiry errors take precedence over normally returned stage failures where those checks occur, including an expired-but-not-yet-notified context. Cancellation during storage cannot release the admission guard until storage actually returns.

`Spool.Append` can remain blocked in a filesystem operation beyond its context deadline. The peer remains blocked and busy in that case. The separate client can still cancel its own socket wait and return a zero receipt. This does not interrupt, roll back or detach storage. A late append can publish a durable record without acknowledgment; owner-controlled explicit replay is the recovery mechanism, not guaranteed delivery. Socket close or cancellation does not establish absence of a durable effect.

| Local result | Meaning |
| --- | --- |
| `nil` | Valid append receipt and one complete response frame written; client observation remains separate. |
| `ErrBusy` | Another exchange still holds this peer's admission; rejected connection closed without storage. |
| `ErrDeadline` | No caller deadline; no request/storage admission. |
| `ErrFrame` | Invalid frame or noncanonical/invalid reduced metadata; no append. |
| `context.Canceled` / `context.DeadlineExceeded` | Cancellation/expiry observed; durable effect may already exist if append was entered. |
| `ErrExchange` | Other admission/socket/storage/receipt failure or recovered panic; durable effects may be ambiguous. |

All storage errors, including wrappers and known storage sentinels, normalize to local `ErrExchange`. No private path, payload, parser diagnostic, panic body or wrapped store message is returned or logged, and no error body is sent to the client. The separate-frame completion guard handles `panic(nil)` under legacy `GODEBUG=panicnil=1` and releases admission during unwinding. Runtime faults also become generic exchange failure; they need controlled diagnosis rather than exposed panic text. Failures before reply emission send no success frame. Once writing begins, some or all receipt bytes may already be sent even if the peer subsequently returns an error; that error cannot revoke them. The client checks the reply under its own independent context.

Cancellation may race the final context check, socket write and return. Successful local handling is not an atomic real-time return guarantee. This code does not guarantee a one-second callback, bound arbitrary filesystem syscall duration or provide a worker kill/restart policy. The CLI's OS-process-start timing rule, collector/reducer/spool and both archived studies remain unchanged.

## Executable evidence and next boundary

[Peer tests](../internal/captureipc/peer_test.go) use temporary private synthetic sockets and spools. They check real client-to-spool capture, 0700/0600 modes, original timestamp on exact replay, unchanged bytes on conflict, publication followed by a lost acknowledgment and subsequent explicit replay, strict/no-append malformed frames, original context/deadline propagation, closed connections after admission failure, cancellation of pending reads, datagram rejection, one-request consumption, expired/exotic/panicking contexts, wrapped storage errors, invalid/promoted receipts and release after errors or nil panics.

The blocked-storage fixture uses channel barriers: after storage enters, both client and peer contexts are cancelled. The client returns while storage remains blocked; sixteen other admitted-connection attempts get `ErrBusy` and no additional append. Only explicit fixture release lets the original append finish and the peer become reusable. Test watchdogs orchestrate correctness and drain blocked work; they are not latency samples or an extension of the exhausted capture studies.

Validation passes: `go test -p=1 ./...`, affected race and CGO-disabled tests, legacy `panicnil=1` peer/client regressions, `go vet ./...`, `go build -p=1 ./...`, scoped formatting and `git diff --check`. All 50 relative file-target occurrences in README, this document and the metadata exchange document resolve. Source-fidelity review checked the request bound/canonical admission, append-before-reply order, original context, static failure mapping and the documented post-write ambiguity against the implementation. No hook host, performance experiment, model request or installed profile was run.

The receiving component does not authenticate either endpoint or start the separate storage process. Worker admission/lifecycle, process failure/capacity, bounded accept resources, startup cost, competing hooks, full-client operation and provenance/reconciliation remain unqualified. Capture profiles stay disabled; [#34](https://github.com/andikapradanaarif/ganesh/issues/34), [#60](https://github.com/andikapradanaarif/ganesh/issues/60) and S0 remain incomplete. Prior focused effort stays unknown and recorded overruns stand. No old timing allowance is renewed. The [execution sheet](s0-execution-sheet.md) and [roadmap](personal-second-memory-roadmap.md) are unchanged; no service or narrative activation follows.
