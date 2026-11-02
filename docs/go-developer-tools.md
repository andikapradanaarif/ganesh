# Go developer tools

Active developer tooling uses Go. `cmd/ganesh-dev` is separate from the general CLI and the experimental capture-only binary; it is never installed as a hook.

Run the native Crush fixtures from this repository:

```sh
go run ./cmd/ganesh-dev crush-test --source /path/to/crush/source --build-profile stripped
```

The runner accepts only the two inspected immutable revisions documented in [the adapter contract](s0-crush-hooks.md). It requires clean tracked source, no untracked Go files and absent overlay targets. It uses Go overlays without changing the checkout, disables module downloads and automatic toolchain selection, and bounds each build/test stage to 60 seconds with a 20-second Go test runtime cap. Ambient `GOFLAGS` or `GODEBUG` are rejected; `GOWORK=off` prevents an ambient workspace changing the build. Profiles are exactly `development` (default), `stripped` and experimental `collector`.

The fixture output is synthetic. The runner preserves failures and performs no automatic retry or fallback. Cold-launch rejection and explicit duplicate recovery must still be reported separately. Passing does not qualify an installed client, complete capture profile or narrative eligibility.

Read the completed studies without launching any collectors:

```sh
go run ./cmd/ganesh-dev study-summary --study 34 --dir .local/capture-startup-34-20261102-a
go run ./cmd/ganesh-dev study-summary --study 36 --dir .local/standalone-capture-36-20261102
```

These directories are private local evidence, excluded from git. Another checkout needs its own permitted copy of the original records; the commands do not create data. Only studies 34 and 36 are supported. The report wraps `metrics` and `decision` with a schema version and study ID. Successful JSON output can describe an incomplete or invalidly bound study; check `decision.status`. Such reports have no metrics, and all profile/signal fields remain false. A parsing, permission or I/O failure returns a static error and exit 1.

Reads require a 0700 directory and regular 0600 `plan.json`/`attempts.jsonl` files, rejecting final-component symlinks and symlinked files. Ancestor paths and the same-user host remain trusted. Bounds are 1 MiB per file, 64 KiB per attempt line, 1,000 parsed rows and 64 JSON nesting levels. Duplicate keys, unknown fields, absent required scoring values and malformed types fail. No input prose, source paths or child errors are copied into summary errors. Counts and numbers remain recorded evidence, not independently verified measurements. Do not modify a dataset while summarizing it; this is a reader for closed studies, without a transaction across files or cryptographic attestation.

The manifest must match the completed study's source revision, seed, cap and deadline; study 36 also requires its explicit study ID. The sample must contain every expected stage/arm/category/block exactly once and valid positive parent timings. These input checks supplement the original summaries without changing their numeric rules. Percentiles use nearest rank and include failed/censored attempts. Missing OS clocks stay visible. Study 34 preserves its candidate-only control veto and first-execution tail gate; study 36 preserves its symmetric controls, first/warm tail gates and separate headroom probe. Neither rule enables a capture profile. The reader does not revalidate source artifacts, producer digests or the collector's reported outcomes.

Build and run the active Go checks:

```sh
go build ./...
go vet ./...
go test -p=1 ./...
go test -race -p=1 ./...
CGO_ENABLED=0 go test -p=1 ./...
```

The Python sources under `integration/capture` and `integration/crush/test_runner.py` stay at their original paths as historical records. Historical commands and test counts in the completed study documents describe those runs; use this page for current workflows. The closed performance producers remain expired. Do not edit them, extend their attempt caps or run a new study through them. Current Go commands and tests never invoke Python. The archive retains older scripts' environment/runtime requirements for historical reproduction; it adds no Python requirement to current build, test, fixture or summary commands.

## Migration evidence, 2026-11-02

On macOS amd64 with Go 1.27.3 and CGO enabled, the Go reports matched the retained historical evaluators exactly after JSON decoding: every metric and decision for study 34's 114 attempts and study 36's 84 attempts. A one-time read-only reference check imported the Python evaluators with bytecode generation disabled; it generated no new timing attempts. Study 34 still recommends the stripped build under its original rule, while study 36's local-signal and headroom gates remain false. All capture profiles remain disabled and [issue #34](https://github.com/andikapradanaarif/ganesh/issues/34) remains open.

The Go native runner with profile `stripped` passed against `aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f` and `3378f47a8398c79a10e35cc61c994160cc0ae3ea`. Each main fixture delivered 41 synthetic notifications and retained 15 records, with 13 intended non-blocking rejections and `decision=none` throughout. No extra callback rejection or automatic retry occurred in these two runs. The baseline separately reproduced both cancelled/expired-context ending suppressions; the repaired pin passed both ending-delivery cases. The compile/test stages took 21.989 and 31.882 seconds respectively, within the 60-second cap; these are validation durations, not collector latency measurements.

The approved roadmap, execution sheet, historical producers, fixtures and recorded datasets remain unchanged. No client configuration, installed binary, capture eligibility or production policy changes. Work is tracked in [issue #38](https://github.com/andikapradanaarif/ganesh/issues/38).

Validation passed: Go build, vet, normal/race/CGO-disabled suites, document target/diff checks and both native runs above. Unit fixtures cover exact commands/environment/timeouts, source-gate failures, incomplete and contaminated samples, distinct historical vetoes, failed tails, missing clocks, private-file boundaries, duplicate/case-aliased keys and content-free errors. The capture-only dependency list still excludes `internal/devtools`, `internal/s0` and `os/exec`.
