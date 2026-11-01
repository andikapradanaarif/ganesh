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

Not run. No startup contributor, improvement or supported-profile result has been established by this plan.
