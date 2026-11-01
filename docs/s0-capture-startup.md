# S0 capture startup investigation

Issue [#34](https://github.com/andikapradanaarif/ganesh/issues/34), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). This is a synthetic local performance investigation, not installed-client acceptance. The approved roadmap and execution sheet are unchanged.

## Frozen plan, before measurements

Frozen on 2026-11-01 at 18:13:43 UTC (2026-11-02 in Asia/Jakarta). Baseline source: `6efdbce26bdcb879cb2c7bb4bdaaed8ea11ea075`. Observed toolchain/host: Go 1.27.3, Darwin amd64, CGO enabled, Apple clang 17.0.0. Record exact artifact digests and load samples in the private run record.

This pass has a 90-minute effort ceiling ending at 19:43:43 UTC and at most 114 performance attempts. Existing correctness suites and the two pinned source-runner checks are separate validation, included in the effort ceiling and never pooled with performance attempts. Cumulative S0 effort and remaining stage budget are unknown; this pass does not establish compliance with the overall stage cap or authorize a larger product trial. On exhaustion, report incomplete rather than expand the experiment.

No real projects, client transcripts, hooks or model inputs are used. Only the fixed synthetic tool-attempt envelope is supplied. Artifacts, spools and numeric/non-content metrics stay in a new private directory under `.local`; no raw native envelope, process environment or child output body is retained. Output byte counts are sufficient. Build/network toolchain downloads are disabled.

1. **Diagnosis:** ten new-file first executions, each immediately followed by a recorded warm execution of the same file (20 baseline attempts). Five equivalent pairs use a test-only instrumented build (10 diagnostic attempts). No execution is hidden as warm-up.
2. **Instrumentation:** an overlay-only helper records main-package initialization, capture entry, OS-clock return, reduction completion, spool completion and receipt-ready timestamps to a separate inherited descriptor. It never enters the production binary or its stdout/stderr. The observer independently queries the same macOS process-info clock before reaping the child. Instrumented results are diagnostic and are not pooled with production latency or success rates.
3. **Candidate selection:** identify the dominant measured interval first. If startup before main-package initialization dominates, compare the smallest build-only candidate: the same entry point with `-ldflags='-s -w'`. No daemon, persistent worker, clock change or retry is introduced. Do not adopt it merely because its binary is smaller. If this lever is unsuitable or gives no useful improvement, report the bounded result; another architecture needs a new scope.
4. **Matched comparison:** twenty blocks, with baseline/candidate order counterbalanced (ten each, shuffled with seed 34). Each artifact receives a new-file first execution followed immediately by a recorded warm execution (80 attempts). Reuse the unchanged baseline artifact; do not rebuild it between arms.
5. **Idle diagnostic:** two launches per artifact after at least 30 seconds without executing that artifact (four attempts). Record the actual gap. This short interval does not represent an overnight cold launch.

Every attempt keeps the original 750 ms OS-process-start budget. The parent watchdog is one second from the launch request, including process creation. Any overrun, launch failure, kill, rejection, unexpected output or ambiguous post-commit result remains in the denominator. Exact replay is not automatic delivery and is not used to repair primary outcomes.

Cache state is uncontrolled. “New file” means a new executable inode copied from the frozen artifact, not proven cold page/cache/signature state. “Warm” means only that this same file was just invoked. Do not purge global caches, warm an artifact before its first measured execution, or attribute all startup time to a particular OS mechanism without evidence.

## Report and decision rules

Report counts and nearest-rank p95/p99 for parent launch-to-exit and observed OS-start-to-exit, separated by arm/category, including failed attempts. Report missing OS-clock observations separately. Receipt-ready and phase distributions come only from instrumented attempts; unavailable phases remain unknown. Record record-presence separately from successful return, because exit 1 or a kill may occur after durable publication.

A candidate earns a build recommendation only with no added failures, unexpected output, privacy/receipt/state regressions, and either fewer failed attempts or at least 20% lower paired median first-execution parent latency with no worse first-execution p95. The paired statistic is the median of `(baseline_ms - candidate_ms) / baseline_ms` over all twenty first-execution pairs. Any candidate failure where its paired baseline succeeds is an added failure; warm and idle outcomes are checked too. Report all paired values, warm/idle results and small-sample uncertainty. This is a local engineering decision rule, not a population SLO or cold-cache guarantee. If the rule fails, retain the baseline and report incomplete/no demonstrated remedy.

Run the CLI/core durability and admission checks and both exact-pin native source fixtures for any selected change. Confirm over-budget/interrupted attempts cannot claim a successful receipt or clean coverage. The collector remains `captured_local / unknown / incomplete`; source/installation capture profiles and narrative remain disabled regardless of this performance result.

## Results

Diagnosis completed with all 30 planned attempts retained. The initial preparation invocation failed on a Python private-file writer API mismatch before any captures ran; its partial artifacts remain private, and the corrected preparation used a new directory. The writer/accounting regression tests pass.

The unchanged baseline failed all ten new-file first executions and three of ten immediate warm executions under the one-second parent limit. The instrumented artifact failed all five first executions and two of five warm executions. Three first-execution failures returned traces showing 754.594, 821.243 and 845.194 ms before main-package initialization, already beyond 750 ms; two first executions were killed before returning a trace and their phase is unknown. Three successful warm traces show reduction below 0.4 ms and spool completion in 55–71 ms. These separate populations must not be pooled or filtered to successful captures.

Before candidate execution, the analysis helper was corrected to include early-rejection final timestamps when receipt-ready is absent. Its initial ready-only calculation would have selected only successful diagnostic traces, contrary to the frozen requirement to retain failures. The original attempt records and runner digest are retained, and the decision records the correction/new analysis digest. No sample, threshold, counterbalanced order, deadline or attempt cap changes. Eight traces have observed completion; two remain unknown. Startup before the main package is a measured contributor; a specific OS mechanism has not been established.

### Matched comparison

All 114 planned attempts are accounted for: 30 diagnostic attempts and 84 matched/idle attempts. Each matched arm has **42** attempts, not 44: twenty first executions, twenty immediate warm executions and two idle observations. The short idle gaps were 30.004–30.695 seconds. Baseline/candidate order was counterbalanced ten blocks each. One-minute host-load observations ranged from 10.49 to 48.84 across the experiment. Cache state remained uncontrolled. Diagnostic and matched results are separate populations; the earlier diagnostic loss rate is not the matched baseline.

The artifacts use identical production Go source at the baseline commit; only linker flags differ:

| Artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| Baseline, default linker flags | 5,372,016 | `3bba007bfea6ce10f423c8c0977ad61a13faaf96620fb74a00f2539f8c995dce` |
| Candidate, `-ldflags='-s -w'` | 3,450,192 | `8fe6117327c38c8261c02cb2fc8e06ebadb4e0c2567662eee4c1362d01339adc` |

The candidate is 35.8% smaller. These exact hashes identify the measured artifacts; later builds with different version metadata are not automatically the same performance sample.

Parent launch-to-observed-exit latency below includes failures and kills:

| Arm / category | n | Failed | Median ms | p95 ms | p99 ms |
| --- | ---: | ---: | ---: | ---: | ---: |
| Baseline / new-file first | 20 | 5 | 693.380 | 1,000.639 | 1,005.053 |
| Candidate / new-file first | 20 | 4 | 618.282 | 1,004.159 | 1,005.783 |
| Baseline / warm | 20 | 1 | 81.520 | 163.666 | 1,004.447 |
| Candidate / warm | 20 | 0 | 74.390 | 487.945 | 547.238 |
| Baseline / idle | 2 | 0 | 66.785 | 71.974 | 71.974 |
| Candidate / idle | 2 | 0 | 59.990 | 62.163 | 62.163 |

The paired median first-execution improvement is **5.27%**, below the frozen 20% speed threshold. First-execution p95/p99 did not improve. The limited build-recommendation rule passes through its **fewer-failures** branch: six versus four failures, with no candidate failure where its paired baseline succeeded. There were no unexpected stdout/stderr bytes or invalid candidate records. This small local result does not establish a population failure-rate reduction, and the warm p95 regression remains visible.

All 114 attempts have an observed OS creation time. OS-start-to-observed-termination p99 for first executions is 1,004.218 ms baseline and 1,004.737 ms candidate, leaving **negative** margins against 750 ms (−254.218 and −254.737 ms). These include killed attempts and parent observation overhead, not receipt-ready times. They establish no acceptable first-launch headroom. Warm/idle results cannot clear that failure.

Across all separate cohorts, there were 84 exit-0 returns, sixteen exit-1 returns and fourteen parent kills. One exit-1 attempt left partial artifacts; another left a visible `unknown / incomplete` record. Its acknowledgment/durability remains ambiguous and it is counted as failed, not recovered. Failed matched attempts left no record. All successful returns retained the required unknown/incomplete labels; no late observed exit was classified as a successful return. Existing exact-retry/crash fixtures cover explicit recovery without treating it as automatic delivery.

### Result and reproducibility

`make build` now selects the smaller linker profile. Application code, the 750 ms budget, silent hook behavior and one-second source-hook timeout are unchanged. Development and stripped builds run the same CLI correctness fixtures. This is a limited artifact improvement; **cold-launch reliability remains unresolved and issue #34 stays open**. No capture profile, installation or narrative lane is enabled. Further performance attempts need a new frozen scope; this run's 114-attempt ceiling is exhausted.

The private record is `.local/capture-startup-34-20261102-a`: frozen plan/toolchain/artifact hashes, original producer digest, analysis correction, all per-attempt numeric metrics and retained synthetic spools. Metrics/manifests are 0600 under a 0700 directory; they are excluded from git. No actual project/transcript data was admitted. The earlier failed preparation remains separate and had no performance attempts.

The one-off producer can execute only inside the frozen time window; do not edit its ceiling to rerun this same experiment after seeing results. A separately frozen future run needs its own recorded plan and code/configuration revision. Read-only summaries remain available:

```sh
python3 integration/capture/measure_startup.py summary .local/capture-startup-34-20261102-a
python3 -B integration/capture/analyze_startup.py .local/capture-startup-34-20261102-a
python3 -B -m unittest discover -s integration/capture -p 'test_*.py'
```

The private data is not shipped with the repository. The analysis helper rejects missing/duplicate cells or incomplete samples, includes failed tails, checks added paired failures and always reports capture profiles disabled. The unchanged roadmap/execution-sheet gates still govern any later installation.

### Validation

`make build`, `go vet ./...`, `go test -p=1 ./...`, `go test -race -p=1 ./...` and `CGO_ENABLED=0 go test -p=1 ./...` pass. The CLI fixture exercises both development and stripped builds, including strict native admission, exact retries, silent exits, deadline/blocked-I/O behavior and unchanged receipt/state labels. Eight offline Python accounting/decision tests pass, including failed-tail retention, launch-failure privacy, early-rejection phase analysis, sample completeness, adverse tails and added paired failures.

Both exact Crush pins pass the source fixture with `--build-profile stripped`: 41 notifications produce fifteen reduced records at each pin, with thirteen expected non-blocking rejections and no host timeouts in those validation runs. The baseline pin reproduces ending-callback suppression; the merged repair delivers all six ending records. Compile/run stages were 36.281 s baseline and 20.191 s repaired. These correctness checks are separate from the 114 performance attempts and do not clear the observed first-launch failures.

Python syntax, Go formatting, local links and diff checks pass. Both temporary source checkouts were removed; no installed client or source configuration was changed. The roadmap/execution sheet are unchanged, and no held-out product answers or model calls ran. The bounded engineering comparison is complete; satisfactory cold-launch headroom remains unproven.
