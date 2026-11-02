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

The first six launches all completed: production parent elapsed was 435.2,
414.5 and 400.1 ms; instrumented elapsed was 464.5, 427.9 and 427.2 ms. The
instrumented OS-birth-to-package-init intervals were 411.7, 377.6 and 378.0 ms,
roughly 88% of their completion intervals. No startup rejection was reproduced
in these six launches. Every launch produced one unknown/incomplete record,
with no hook output or retained planted token.

Preparation correction before the remaining twelve launches: the private
driver initially required a new `startup_budget` rejection to select comparison,
which was narrower than the frozen protocol's “startup remains the contributor.”
It stopped after six launches without running a candidate. Preserve that driver
and all six results. A separate comparison-only driver uses the measured dominant
startup interval and the existing rejection evidence as the selection basis;
it verifies those six records and consumes only the remaining twelve slots.
The 18-launch cap, deadline, comparison order and candidate gate are unchanged.
This is a disclosed procedural correction, not a reproduced diagnosis of the
specific previous failures or a change to their classification.

All twelve comparison launches also completed with one valid reduced record,
zero output and no retained planted token. The two blocks used different build
IDs. In each block, each profile's signed artifact started from its unsigned
artifact's bytes. Ad hoc signing plus strict verification occurred before
either arm's execution, with no certificate, keychain identity or timestamp
service. No capture launch was replayed.

| First-execution arm | n | Median parent elapsed | Nearest-rank p95 / p99 | Failures |
| --- | --- | --- | --- | --- |
| Unsigned | 6 | 412.607 ms | 511.166 / 511.166 ms | 0 |
| Ad hoc signed | 6 | 411.737 ms | 496.657 / 496.657 ms | 0 |

Mean paired signed-minus-unsigned elapsed was -2.605 ms, with mixed pair
directions. The local ≤600 ms candidate gate passes; the unsigned arm passes
the same numerical bounds. These observations do not establish a signing
benefit or diagnose the previous failed callbacks. **Select no build/runtime
change.** The observed startup sensitivity in #34 remains unresolved, and no
installed profile or production narrative is enabled.

Reported build time ranged from 752.5 to 1,390.1 ms for comparison artifacts;
signing plus verification added 66.3–101.6 ms. Those preparation fields include
build/sign/verify commands, not all driver work such as copying, hashing and
record inspection. No preparation execution warms a capture callback, but
file access can affect caches. Host load around the end was 3.23 / 3.34 / 7.41
(1/5/15-minute averages); earlier higher-load failures are not matched controls.
Do not attribute their difference to load or signing.

The 18-launch measurement cap is exhausted. Both original private producers
are retained byte-for-byte as completed experiment sources:

- [Diagnosis producer](../integration/capture/artifact44/diagnose.go.fixture),
  SHA-256 `94469439f26cb85985676eb0ed5d589c43951c0cc36c914c057183335bb816e7`.
- [Comparison producer](../integration/capture/artifact44/compare.go.fixture),
  SHA-256 `bbd9213c1cf87bb485c71d689f31363cb379da0868cf2a4a06e6cffea377a3a0`.

These are historical sources, not active developer commands or permission to
repeat the closed study. Their `.local/capture-artifact-44` records/spools stay
outside version control. The initial plan hash is
`b3847970e51e2f82fbb50c2b22b82df331e3748fe510f40bc020a5534c3d9290`;
the corrected comparison-plan hash is
`433c31caf133fad57ee121c991b7a21cef6bba1477322fd44c1ff06a3118a813`.
Each plan binds its producer and then-current protocol digest. They do not
authenticate the host, source or measurements. The runtime source was unchanged
from `22e8514`; only this protocol changed before measurement.

## Correctness validation

On PR #43 runtime head `932c449`, one new serial pass each of build, vet, full
normal, full race and CGO-disabled full tests passed. Both exact-pin native
fixtures are checked separately below. These validation executions are outside
the 18-launch measurement sample. They do not erase the earlier failed normal/
race validations documented in [PR #43's result-reporting record](https://github.com/andikapradanaarif/ganesh/pull/43)
or qualify the profile. No clock, test assertion, runtime deadline or build
preparation changed to obtain those passes. No repeat-until-pass loop was used.

Both exact source pins passed using the Go native runner and the experimental
collector profile: baseline `aed2bae` reproduced suppressed TurnEnd dispatch;
repaired `3378f47` delivered the endings. Each processed 41 synthetic
notifications into 15 reduced records. Expected malformed/conflicting fixtures
returned non-blocking errors; every runner decision remained `none`.

The collector dependency graph still excludes S0/developer packages and
`os/exec`. Documentation links/diff and archived producer hashes were checked.
Only this report and the completed producers are added by PR #45; no runtime,
test assertion, build defaults or approved baseline document changes.
