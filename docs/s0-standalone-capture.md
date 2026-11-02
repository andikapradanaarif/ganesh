# Standalone capture experiment

Issue [#36](https://github.com/andikapradanaarif/ganesh/issues/36), following [#34](https://github.com/andikapradanaarif/ganesh/issues/34). PR #35 is merged. Its 114-attempt study remains complete and unchanged.

## Frozen scope and protocol

Frozen before this experiment's measurements: 2026-11-02 06:29:33 UTC. This pass ends by **07:59:33 UTC** (90 minutes) and permits **84 performance attempts**. Correctness/build checks are separate and included in the effort ceiling. Cumulative S0 effort remains unknown; no broader stage-budget compliance is claimed.

Behavioral reference: general CLI at `7d8813b7d0186629132ea6e020a7833809ce1070`. Performance baseline: stripped general CLI after the shared implementation extraction. Candidate: an experimental stripped `ganesh-capture` executable built from that **same frozen source revision**, omitting preparation/discovery imports. Record the source revision, artifact hashes/sizes, dependency graphs, toolchain, host and load before launches. This same-source binding was clarified before any artifact measurements to avoid confounding the entry-point comparison with the shared-code move. Go 1.27.3, Darwin amd64 and CGO enabled are the observed preparation environment. No daemon, worker, uploader, IPC, network, client configuration or capture-profile activation is included.

Preserve the 750 ms OS-process-start budget and one-second parent watchdog from launch request. Neither entry point may reset its clock in handler logic, weaken admission, emit hook output, change identities/receipts or upgrade unknown/incomplete state. The general CLI stays the default build; the standalone artifact is explicitly experimental.

Use twenty matched blocks. Each block executes baseline and candidate in counterbalanced order (ten each, shuffled with seed **36**). Each artifact receives one execution of a new executable file followed immediately by one recorded warm execution of that same file: **80 attempts**. Then execute each artifact twice after at least 30 seconds without invoking that file: **four attempts**. Record actual idle gaps. No hidden warm-up, retries or replacement samples. Cache state is uncontrolled: new inode does not mean proven cold cache, and bias direction is unknown. Thirty seconds does not represent overnight idle.

Use only the existing fixed synthetic native tool-attempt envelope. Each attempt has its own private spool. Retain failures, kills, launch errors, partial artifacts and ambiguous records in the denominator. Store artifacts, hashes and non-content numeric/enum metrics in a new 0700 `.local` directory with 0600 manifests/metrics. Never retain child stdout/stderr bodies, process environments or real project/transcript inputs. Query OS birth time independently; missing observations remain unknown. No model or source-processing run is included.

## Prospective gates

Report first, warm and idle strata separately; do not pool their timing/dominance statistics. Report nearest-rank parent and OS-start-to-observed-termination p95/p99 over all attempts, failed counts and missing clock counts. Those termination times are conservative observations, not receipt-ready measurements. Record visible spool state separately from acknowledgment.

Veto any unexpected output, invalid record labels or exit-0 without an expected record in **either** arm. Require no candidate failure where its paired baseline succeeds, and no increase in candidate failures in any stratum. A local performance signal additionally requires:

- Candidate p95 no worse in both first and warm strata.
- Either fewer candidate first-execution failures or at least 20% median paired first-execution improvement, using `(baseline_ms - candidate_ms) / baseline_ms` over all twenty pairs.

An independent conservative **prototype headroom probe** requires zero candidate failures, no missing clock observations and observed OS-start-to-termination p99 ≤600 ms in every stratum. This leaves 150 ms within the collector's unchanged 750 ms allowance. The small sample cannot establish a population SLO. Passing this probe does not enable a supported installation, complete provenance/reconciliation or authorize narrative. If it fails, #34 remains open and the artifact stays experimental; do not expand into a persistent architecture under this scope.

Before presenting a build recommendation, run existing CLI/core safety and durability checks for both entry points, prove the dedicated dependency graph excludes S0/discovery, and run both exact-pin Crush fixtures with the candidate. Preserve unsupported-clock behavior and panic/output guards. No alternate artifact may be selected after scoring.

## Results

Not run. No performance signal or headroom has been established for the standalone collector.
