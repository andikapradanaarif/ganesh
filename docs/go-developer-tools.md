# Go developer tools

Active developer tooling uses Go. `cmd/ganesh-dev` is separate from the general CLI and the experimental capture-only binary; it is never installed as a hook.

Run the native Crush fixtures from this repository:

```sh
go run ./cmd/ganesh-dev crush-test --source /path/to/crush/source --build-profile stripped
```

The runner accepts only the two inspected immutable revisions documented in [the adapter contract](s0-crush-hooks.md). It requires clean tracked source, no untracked Go files and absent overlay targets. It uses Go overlays without changing the checkout, disables module downloads and automatic toolchain selection, and bounds each build/test stage to 60 seconds with a 20-second Go test runtime cap. Ambient `GOFLAGS` or `GODEBUG` are rejected; `GOWORK=off` prevents an ambient workspace changing the build. Profiles are exactly `development` (default), `stripped` and experimental `collector`.

The fixture output is synthetic. The runner preserves failures and performs no automatic retry or fallback. Cold-launch rejection and explicit duplicate recovery must still be reported separately. Passing does not qualify an installed client, complete capture profile or narrative eligibility.

The Python sources under `integration/capture` and `integration/crush/test_runner.py` stay at their original paths as historical records. The closed performance producers remain expired. Do not edit them, extend their attempt caps or run a new study through them. The Go summary port and migration validation are tracked in [issue #38](https://github.com/andikapradanaarif/ganesh/issues/38).
