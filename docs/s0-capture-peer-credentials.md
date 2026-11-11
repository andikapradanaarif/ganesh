# Connected capture peer credentials

[Issue #107](https://github.com/andikapradanaarif/ganesh/issues/107) adds an optional [credential predicate](../internal/captureipc/admission.go) before the existing [client](s0-capture-metadata-exchange.md) or [receiving peer](s0-capture-metadata-peer.md). It compares two caller-supplied expected numbers with kernel-reported socket fields. It does not implement application authentication or start/manage a worker.

## Expected values and connection ownership

`captureipc.AdmitPeer(ctx, conn, expected)` returns the **same** connected `*net.UnixConn` only if its reported PID and UID both equal `expected captureipc.PeerIdentity`. The caller must obtain those expected values through separately trusted process-handle/lifecycle custody before this check. Do not read a socket's values and submit them as their own expected policy; do not derive policy from a socket path, UUID, request body or untrusted client claim.

PID must be positive and at most 2³¹−1. UID zero means **root**, not a wildcard; the all-ones UID sentinel rejects. There is no default same-user admission, group-only rule, process scan, fallback or credential-based source promotion. A wrong expected PID still rejects a same-UID connection.

The helper takes exclusive ownership during the check. On success the caller resumes ownership and may hand the identical socket to `NewClient` or `Peer.ServeOne`. On every ordinary rejection or recovered panic it closes the provided connection and returns nil. Do not use that socket concurrently or reuse it after rejection. Nil/unconfigured/closed/non-stream sockets fail closed. Only already connected Unix **stream** sockets are accepted; no socket path lookup or connection establishment is performed.

The helper reads/writes no socket payload, changes no deadlines and calls neither `File` nor `dup`. `SyscallConn.Control` protects the original descriptor against Go-side close/reuse while querying it. The supplied context must already have a deadline; cancellation/expiry is checked before and after descriptor/credential access. Kernel calls remain synchronous and are not made interruptible by a goroutine or a reset clock. Cancellation can race the final check/return; there is no atomic or real-time completion guarantee. A later client/peer handoff remains responsible for its own deadline checks.

## OS fields and their limits

| Backend | Exact fields read | Scope |
| --- | --- | --- |
| [Darwin reader](../internal/captureipc/credentials_darwin.go) | `LOCAL_PEERCRED` xucred UID with known version zero, then `LOCAL_PEERPID` | Apple saves peer credentials separately; its PID option reads the peer socket's `last_pid`. The pair is not an atomic current-process identity. |
| [Linux reader](../internal/captureipc/credentials_linux.go) | `SO_PEERCRED` UID and PID | Linux reports credentials saved at connect/listen/socketpair time. These do not authenticate each later writer. |
| [Other platforms](../internal/captureipc/credentials_unsupported.go) | None | Return static rejection; no same-UID or PID-only fallback. |

These semantics were checked against pinned Apple [socket credential/PID implementation](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/uipc_usrreq.c) and [xucred definition](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/sys/ucred.h), and the [Linux Unix-domain socket reference](https://man7.org/linux/man-pages/man7/unix.7.html). The helper deliberately promises only equality of the reported fields. It does not infer their history or declare them interchangeable across operating systems.

Descriptor inheritance/passing, process credential changes, process exit and PID reuse can separate these observations from the current writer or intended application. Concurrent same-user tampering remains outside this helper's protection. There is no executable digest/signature, process-birth token, owner-channel authentication, current-liveness check or automatic invalidation when a worker exits. A future supervisor must bind expected values to the intended managed process, constrain descriptor custody and revoke stale policy/connections; a cached PID/UID allowlist is insufficient.

In particular, Linux connection-time credential semantics mean a socketpair or listener created by a parent and inherited by a child can report the parent's credentials. A future child policy must not substitute that observed parent PID merely to pass. Its connection setup and process-handle expectations need a compatible, independently checked lifecycle.

Even an exact match does not establish durable storage, capture completeness, reconciled inputs, first-party origin, semantic authority or permission to publish. Existing `NewClient` and `Peer.ServeOne` remain low-level caller-admitted APIs; adding this helper does not automatically enforce admission for every caller. Their v1 metadata/receipt wire is unchanged, with local `unknown / incomplete` labels retained.

## Failures and dependency

A normal mismatch, unsupported/failed query, invalid policy/socket, exotic context error or synchronous panic returns **nil plus static `ErrAdmission`** and closes the supplied connection. `ErrDeadline` identifies a missing caller deadline; actual standard cancellation/expiry errors pass through. No observed PID/UID, pathname, syscall errno, wrapped diagnostic, socket bytes or panic body is returned or logged. The completion guard also handles legacy `panic(nil)` behavior. The client/peer receive no admission error frame or implicit retry.

The transport adds the pinned direct dependency **`golang.org/x/sys v0.50.0`**, with `go.sum` checksums. Its [Darwin wrapper](https://github.com/golang/sys/blob/v0.50.0/unix/syscall_darwin.go) supplies the credential structure interface missing from Go's standard `syscall` API, avoiding handwritten unsafe layouts and cgo. The dependency is used only in the OS transport readers; domain/application capture code remains independent of it. The module needs Go 1.26; this repository keeps its Go 1.27 baseline. No frozen experiment producer, rule, recorded hash or private run input is rewritten; old frozen runners' module pins remain their historical values, not silently updated to authorize new executions.

## Executable evidence and remaining work

[Admission tests](../internal/captureipc/admission_test.go) supply expected values independently using the current process's OS identifiers or a started child process handle. They check exact PID/UID matching, invalid and mismatched policy (including root not being a wildcard), wrong socket types and closed descriptors, missing/cancelled/expired/exotic/panicking contexts, nil-panic cleanup and generic kernel query failures. A preloaded socket with expired read/write deadlines proves credential admission preserves both deadlines and unread bytes and returns the identical connection.

The separate-process fixture launches the test binary with a minimal explicit environment, no inherited model credentials and bounded child/accept deadlines. The parent obtains the child's PID from its process handle, accepts the local connection, then checks the kernel fields. Exact child identity succeeds; substituting the parent's PID for the same-UID child or a different expected UID rejects. The latter is a policy-mismatch test, not a claim of executing a second real user account. All child processes are waited or killed on cleanup; output is discarded. These are correctness fixtures, not native hook hosts, performance measurements or a use of exhausted experiment launch slots.

Admitted sockets complete a real existing client/peer/private-spool exchange; a wrong client expectation closes the connection before append. The fixture does not establish a live-client profile, trustworthy source history or process-isolation safety. Worker launch/readiness, expected-identity custody, resource limits, stop/crash/restart/revocation, full-client timing and provenance/reconciliation remain separate work.

| Target | Evidence from this change |
| --- | --- |
| Darwin amd64 | Normal/race/CGO-disabled and legacy nil-panic tests executed, including real child-process checks. |
| Darwin arm64 | Test binary cross-compiled; not executed. |
| Linux amd64 | Test binary cross-compiled; runtime behavior remains unqualified here. |
| FreeBSD amd64 | Unsupported-reader stub/test binary cross-compiled; not executed and no admission support claimed. |

Validation passes: `go test -p=1 ./...`, affected race and CGO-disabled tests, legacy admission/peer/client `panicnil=1` regressions, `go vet ./...`, `go build -p=1 ./...`, `go mod verify`, scoped formatting and diff checks. All 62 relative file-target occurrences in README and the three capture IPC documents resolve. The child fixture initially raced a rejected socket's closure before its first marker write in a CGO-disabled run. It now synchronizes the first byte before admission and leaves a second byte queued to verify no payload consumption; ten repetitions of that corrected credential test pass before final suite validation. No failed fixture was treated as credential or capture evidence.

Capture profiles remain disabled; [#34](https://github.com/andikapradanaarif/ganesh/issues/34), [#60](https://github.com/andikapradanaarif/ganesh/issues/60) and S0 remain incomplete. No model calls, source enrollment, installation, daemon/listener command, gateway or narrative activation is added. Old allowances stay consumed, total prior focused effort remains unknown and recorded overruns stand. The [execution sheet](s0-execution-sheet.md) and [roadmap](personal-second-memory-roadmap.md) are unchanged.
