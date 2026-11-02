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

The 84-attempt comparison is complete. **No local performance signal and no acceptable prototype headroom were established.** The candidate remains experimental; the default build/capture path is unchanged and #34 stays open. This outcome does not authorize a daemon, uploader, clock change or installation.

Both artifacts use source `5eb607f1fc9271ca18dfae161a7769e8f398c3f3`, Go 1.27.3 and `-ldflags='-s -w'`:

| Artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| General CLI | 3,450,192 | `1512cd98f3bb48d3a6783490b0816f9c27092cf31e28884ce06b5a3793843f5a` |
| Dedicated collector | 3,281,368 | `c00231d7e5c9d60e8a289fbc32871f5cc30fa29cc07b12a2e88181d6bdb032a0` |

The candidate is 4.9% smaller. Its recorded dependency graph excludes `internal/s0` and `os/exec`; it shares `internal/capturecli`, the original reducer/spool and the OS process clock. The source move preserves the 750 ms constant, kernel creation-time lookup, output watchdog and receipt format. The shared package is CLI-specific: its process deadline/watchdog must not be reused as an in-process or server callback contract.

Parent launch-to-observed-exit times include failures and kills:

| Arm / stratum | n | Failed | Median ms | p95 ms | p99 ms |
| --- | ---: | ---: | ---: | ---: | ---: |
| General / first new file | 20 | 20 | 1,003.125 | 1,006.029 | 1,006.036 |
| Dedicated / first new file | 20 | 20 | 1,003.895 | 1,005.976 | 1,008.559 |
| General / warm | 20 | 7 | 176.534 | 1,006.157 | 1,006.214 |
| Dedicated / warm | 20 | 7 | 410.781 | 1,004.070 | 1,006.120 |
| General / short idle | 2 | 0 | 158.070 | 203.288 | 203.288 |
| Dedicated / short idle | 2 | 0 | 118.994 | 138.462 | 138.462 |

There were no added candidate failure pairs, unexpected output or record-classification violations in either arm. The paired first-execution median improvement was **0.017%**. Both first-execution distributions are largely censored by the one-second watchdog: small differences at termination are not uncapped execution-time improvements. The first/warm p95 comparisons technically satisfy the tail condition, but equal first-failure counts and the failed 20% improvement threshold make `local_signal=false`. Warm median latency worsened and remains visible.

Observed OS-start-to-termination first-execution p99 was 1,005.066 ms general and 1,007.122 ms dedicated. Every first execution failed; `headroom_probe=false` independently of the relative comparison. All 84 attempts have OS-clock observations. Across both arms, thirty returns succeeded with unknown/incomplete records, seventeen returned exit 1 and thirty-seven were killed. One failed general attempt left partial artifacts; no failed return had a confirmed expected record. Those failures were not replayed or replaced.

Each arm has 42 attempts. Actual short-idle gaps were 30.003–31.224 seconds. One-minute host-load samples ranged from 33.32 to 76.90; cache state remained uncontrolled. These results are specific to the declared conditions, not a population rate or proof that every host behaves this way. Do not pool this study with #34, use idle successes to clear first-launch failures, or infer a particular OS mechanism from the result.

## Implementation and validation record

The general CLI now delegates capture parsing/deadlines, hook exits and receipt output to `internal/capturecli`. `cmd/ganesh-capture` imports that adapter without preparation/discovery. `make build` retains the general binary; `make build-collector` exposes the explicitly experimental candidate. No client configuration is changed and no capture profile is enabled.

Correctness checks run all native/normalized CLI cases against development general, stripped general and dedicated artifacts. Handler-panic subprocess checks call the shared exit guard; the input-worker panic test moved into the shared package and still asserts a static error and zero metadata. Receipt-output timeout expectations use the declared exit code 124 rather than importing the implementation constant.

An initial compile check found that old test-only constant reference after the move; it was corrected before measurements. The first compiled CLI validation then failed its first native callback on all three artifacts (silent exit 1 after 1.144, 1.194 and 1.072 seconds). A later complete normal suite passed without changing collector runtime code or deadlines. The failed validations remain recorded here and do not disappear into the timing study or become hidden warm-up.

The repaired-pin native fixture passes with `--build-profile collector`: 41 synthetic notifications yield fifteen reduced records and all six ending-dispatch records. It logged fourteen non-blocking rejections, one more than the thirteen intentionally rejected cases: the initial `PreToolUse` callback returned exit 1 with empty adapter stderr, and the fixture's explicit duplicate recovered the record. That is an observed capture failure, not automatic retry or installed-profile acceptance. Its compile/run stage took 40.523 seconds.

The original-pin fixture also passes its expectations: the same 41 notifications yield fifteen records with thirteen expected rejections, while the invocation probe still demonstrates the original ending-dispatch suppression for cancelled/expired callers. Its compile/run stage took 47.717 seconds. Neither run installs a hook, executes an actual agent turn or establishes complete delivery.

Final checks pass: `go build ./...`, `go vet ./...`, `go test -p=1 ./...`, `go test -race -p=1 ./...`, and `CGO_ENABLED=0 go test -p=1 ./...`. The compiled CLI suite covers all three build profiles; core crash/durability checks remain green. Fifteen Python tests pass (eight historical accounting tests and seven new study-rule tests), including symmetric output/record vetoes, warm-tail rejection, missing clocks, sample completeness and budget enforcement. Formatting, Python syntax, local links and diff checks pass. Historical study files and the approved roadmap/execution sheet are unchanged.

The producer and all raw numeric/enum observations are retained privately in `.local/standalone-capture-36-20261102` (0700 directory, 0600 manifests/metrics). The artifact source revision, hashes, producer digests, dependency graph, order, load samples, clock gaps and all 84 outcomes remain available. No real project/transcript data or raw child output was stored. The earlier study's files and frozen rule were not changed. Further measurements require another frozen scope; this study's 84-attempt ceiling is exhausted.

```sh
python3 -B integration/capture/standalone_study.py summary .local/standalone-capture-36-20261102
python3 -B -m unittest discover -s integration/capture -p 'test_*.py'
```

The historical source/metrics are distinct from later builds carrying different VCS metadata. The timing result does not justify switching a client to the dedicated command. It also does not demonstrate that putting a receiver behind the same per-event Go wrapper would remove startup cost. An alternative invocation/capture architecture needs a separately reviewed scope and end-to-end evidence.

## PR #37 review follow-up

Post-review summary hardening now binds every scored cell to `stage=standalone36` and checks the summary manifest's study, cap and deadline against the same identity enforced before collection. Missing, foreign or mixed-stage cells cannot fill this study's expected sample, and a foreign manifest cannot be summarized as study 36. This is input-identity validation; no timing threshold, failure veto, original record or scoring calculation was changed. The retained 84-attempt summary is exactly equal before and after this change, with both gates still false. No capture was rerun. Seventeen Python tests now pass, including the added stage and manifest cases.

The owner-provided review reports one failed package on an initial race run under load around 44, followed by a passing rerun. This is reviewer-reported validation evidence, separate from the assistant's earlier passing runs and the frozen performance sample; its precise rejection cause is not established here.

The reviewer also traced initial smoke rejections to a 0755 spool directory. **Silent exit 1 is an outcome, not a diagnosis:** strict input admission, private-store checks, clock/deadline failure, capacity and other guarded failures share that signal. Do not infer a deadline miss solely from silence and exit status; use separately observed evidence, and preserve ambiguity when it is unavailable. Hook stdout/stderr remain empty. The disclosed warm-median change is unresolved and must not be attributed to load alone.
