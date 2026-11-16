# Offline answering-client request profile

This Go fixture builds the pinned Crush commit `86897c56a304bac133650a6055ab1d091e6b8f83` with a mandatory [request guard](../../../internal/devtools/requestguard/guard.go) in a new private source snapshot. It leaves the source checkout, its uncommitted changes and the installed binary unchanged. Only `go.mod`, `go.sum`, `main.go` and `internal/` are archived; account configuration is not loaded.

From the Ganesh repository root, with that commit and dependencies already cached:

```sh
GOPROXY=off GOSUMDB=off go run ./integration/crush/requestprofile \
  -source /path/to/crush \
  -out /absolute/path/to/new-private-run
```

The output directory must not exist; its parent must exist. macOS `sandbox-exec` must pass a loopback-only policy before the client launches. There is no live endpoint, model-call or credential option. Each fixture generates its own dummy provider, synthetic prompt and isolated configuration/data. The private candidate binary is never installed.

The guard covers native Responses provider calls, including auxiliary requests. It serializes complete responses, caps dispatch count and requested output, forbids identical payload replay within a process, disables HTTP connection reuse/environment proxies, and checks usage before delivering a bounded buffered response to the SDK. Input tokens are a post-response stop threshold, not a preflight estimate. Missing or invalid usage stays unknown. Durable metadata-only journals record starts before forwarding and finishes before success is returned.

A [shared run ledger](../../../docs/s0-run-accounting.md) is mandatory in this candidate. Its frozen header binds limits, exact profiles, allowed slots and deadline; each fresh process must supply the same path/digest and an unused slot. An OS lock spans a synced request reservation through its finish. Aggregate usage, request/output ceilings, unknown or unfinished requests and expiry survive process exit. Client initialization never creates a replacement ledger or downgrades to local counters. The Go library also supports bounded non-streaming Chat Completions so grading can share the same ledger; separate Go process tests exercise that path.

The [adapter](adapter.go.txt) makes this experimental process terminate with exit code **78** on a guard failure. This deliberately abandons the isolated session after syncing available evidence, preventing SDK retries/backoff and fallback after errors. An invalid or missing guard configuration also exits before dispatch. This build is unsuitable for an interactive session containing unrelated work. Ordinary client stdout is never a successful experiment result after exit 78, even if partial text exists.

Thirteen actual-client cases cover the prior five fixtures, a missing run binding, and shared request/output/input limits, unknown usage, expiry, slot reuse, contention and a killed client. Each case has a 35-second launcher deadline, each guard a 25-second lifetime, and a frozen shared deadline. The complete build/fixture command is bounded to eight minutes. Build and child logs, journals, dummy config, binary, source snapshot and report stay private; the console prints progress only. Byte counts/hashes are metadata, not token estimates. Go tests independently cover mixed answer/grading processes, additional limits and accounting faults:

```sh
go test -race ./internal/devtools/requestguard
go vet ./internal/devtools/requestguard ./integration/crush/requestprofile
```

This qualifies synthetic shared accounting across the bound processes. It does not establish real-provider compatibility, provider billing, project/tool isolation, model accuracy, complete context fit or approval of the 108/132-answer experiment. It depends on a trusted local filesystem; deliberate ledger rollback or clients bypassing this transport are outside the guarantee. Existing S0 permissions, model/grader acceptance and one whole-run approval remain required. See the [S0 execution profile](../../../docs/s0-execution-profile.md).

## Context and tool probe

The accounting fixture's disabled-tool list does not qualify an exact tool palette. At this pin, the client uses concrete names such as `lsp_rename`; broad names such as `lsp` are not a wildcard. The client also appends default context paths, discovers skills and can expose notebook tools. File inventory and a clean-looking configuration therefore do not prove what reaches the model.

Select one separate synthetic probe without rerunning the 13 accounting cases:

```sh
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./integration/crush/requestprofile \
  -suite context-tools \
  -source /path/to/cached/crush \
  -out /absolute/path/to/new-private-context-probe
```

The probe uses the same pinned/guarded build and loopback-only synthetic server. Its disposable Git repository prevents ancestor project discovery. Global configuration/data/state and global skill discovery use isolated directories; notebook and automatic LSP are explicitly disabled. A coder-agent allowlist selects exactly `glob`, `grep`, `ls`, `view`. The final answering tool policy is **not** selected by this fixture: the real experiment still requires normal authorized own-project repository/CI/issue/source access matched across arms.

The observer checks the actual outgoing Responses input and tool definitions. Every tool-bearing request must include all four synthetic markers: an explicitly configured file, root `AGENTS.md` added by defaults, an explicitly configured nested instruction file and the configured global context. A separate unlisted file's marker must be absent from every request. The palette must contain exactly the four allowed function names, with no duplicates, unknown tool forms or extra mutation/delegation/memory tools. Auxiliary requests without tools are recorded separately and cannot qualify the probe alone; malformed or forbidden content in a later request still fails the result. Reports contain names and marker booleans, never request/context bodies.

Unit checks cover missing context, extra mutation/memory tools, missing/duplicate/unknown tools, markers hidden in unrelated metadata, auxiliary-only observations and later invalid requests. These check the observer's refusal behavior; they do not establish runtime success. The owner-run actual-client probe remains pending until its report is reviewed. A passing probe would establish only this synthetic context/palette wiring: it executes no tools and cannot qualify filesystem/network permissions, actual owner notes/configuration, provider access/accuracy, request fit or the full S0 profile. All earlier reports and allowances remain unchanged.
