# Offline answering-client request profile

This Go fixture builds the pinned Crush commit `86897c56a304bac133650a6055ab1d091e6b8f83` with a mandatory [request guard](../../../internal/devtools/requestguard/guard.go) in a new private source snapshot. It leaves the source checkout, its uncommitted changes and the installed binary unchanged. Only `go.mod`, `go.sum`, `main.go` and `internal/` are archived; account configuration is not loaded.

From the Ganesh repository root, with that commit and dependencies already cached:

```sh
GOPROXY=off GOSUMDB=off go run ./integration/crush/requestprofile \
  -source /path/to/crush \
  -out /absolute/path/to/new-private-run
```

The output directory must not exist; its parent must exist. macOS `sandbox-exec` must pass a loopback-only policy before the client launches. There is no live endpoint, model-call or credential option. Each fixture generates its own dummy provider, synthetic prompt and isolated configuration/data. The private candidate binary is never installed.

The shared guard covers native Responses provider calls, including auxiliary requests. It serializes complete responses, caps dispatch count and requested output, forbids identical payload replay, disables HTTP connection reuse/environment proxies, and checks usage before delivering a bounded buffered response to the SDK. Input tokens are a post-response stop threshold, not a preflight estimate. Missing or invalid usage stays unknown. A durable metadata-only journal records starts before forwarding and finishes before success is returned. A start without a finish needs reconciliation and remains potentially dispatched with unknown usage. Ledger creation is exclusive; reruns cannot overwrite it.

The [adapter](adapter.go.txt) makes this experimental process terminate with exit code **78** on a guard failure. This deliberately abandons the isolated session after syncing available evidence, preventing SDK retries/backoff and fallback after errors. An invalid or missing guard configuration also exits before dispatch. This build is unsuitable for an interactive session containing unrelated work. Ordinary client stdout is never a successful experiment result after exit 78, even if partial text exists.

Five actual-client fixtures check HTTP 503, successful SSE plus auxiliary usage, missing usage, a one-request cap and missing guard configuration. Every child has a 35-second launcher deadline; the guard has a 25-second lifetime. The complete build/fixture command is bounded to eight minutes. Build and child logs, journal, dummy config, binary, source snapshot and report stay in the private output directory; the console prints progress only. Byte counts/hashes are metadata, not token estimates. Go tests independently cover additional limits, cancellation, serialization and journal faults:

```sh
go test -race ./internal/devtools/requestguard
go vet ./internal/devtools/requestguard ./integration/crush/requestprofile
```

This qualifies a bounded synthetic, single-process transport behavior. It does not establish real-provider compatibility, enforce provider billing, qualify project/tool isolation, measure model accuracy, prove complete context fit or implement cross-process whole-run accounting. A new process gets new counters; no 108/132-answer batch may use these counters as a global budget. Existing S0 permissions, model/grader acceptance and whole-run approval are still required. See the [S0 execution profile](../../../docs/s0-execution-profile.md).
