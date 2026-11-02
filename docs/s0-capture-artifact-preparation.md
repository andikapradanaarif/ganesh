# Capture artifact preparation investigation

Issue #44 follows the unresolved first-launch failures in #34 and blocks full
validation of PR #43. It does not extend either completed startup study.

## Frozen pass

Frozen before measurement on 2026-11-02 UTC: 90 focused minutes, ending 19:50
UTC, with at most 18 synthetic capture launches. No provider/model calls or
API spend. Correctness suites are separate validation; their initial failures
must also be reported. The overall S0 effort remaining is unknown.

The first six launches cover development, stripped general and stripped
standalone artifacts, each once without instrumentation and once with an
overlay-only phase trace. Trace observations and production timings remain
separate. The trace uses an inherited descriptor and static phase categories;
hook stdout/stderr remain silent. Missing phases stay unknown.

If startup remains the contributor, twelve launches compare unchanged unsigned
artifacts with build-time ad hoc signed artifacts: three profiles, two blocks,
one first execution per arm and block. Order alternates by profile and block.
Keep every failure, kill, missing clock and ambiguous post-commit outcome.
Record source/artifact hashes, preparation cost, exit/output counts, parent
elapsed time and record state privately. No retries or hidden warm-up.

Candidate gate: zero candidate failures, unexpected output or invalid records;
no candidate-only failure pair; every candidate completion at most 600 ms.
This small local probe does not qualify an installed profile or establish a
population reliability rate. Build/sign/hash operations touch files; a first
execution is not proven cold cache. Host load and cache state are uncontrolled.

Preserve the 750 ms OS-process-start budget and one-second host timeout. No
clock reset, daemon/uploader, OS security-setting change, enrolled source,
owner signing identity or narrative activation is authorized by this pass.
Ad hoc signing would establish neither authorship nor honest capture. Private
spool permissions, reduction, durability and idempotency remain mandatory.

The local Go 1.27.3 linker source, `cmd/link/internal/ld/lib.go`, defines
`NeedCodeSign` as Darwin **and ARM64**. The observed Darwin/AMD64 development
artifact is unsigned. This motivates a build-preparation experiment; it does
not establish the cause of the delay.

## Results

Pending. No profile or production narrative is enabled.
