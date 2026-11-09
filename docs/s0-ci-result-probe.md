# Disposable CI-result verifier probe

[Issue #69](https://github.com/andikapradanaarif/ganesh/issues/69), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6), prepares the **green CI for an old commit** case from roadmap [A2.1](personal-second-memory-roadmap.md#a21-s0b-1-deterministic-verifiers-and-safety-three-to-four-focused-days). [Build §5](personal-second-memory-roadmap.md#build-5-subject-authority-and-preventive-verification) limits `ci.result` to the configured repository/workflow and exact commit/run. [Build §6](personal-second-memory-roadmap.md#build-6-verified-typed-publication-and-injection-boundary) prohibits relabeling a historical pass as current-head green without a current-head dependency. The [execution sheet](s0-execution-sheet.md) permits this bounded offline preparation while model-dependent work remains pending.

## Frozen expectations

Before candidate implementation or execution, freeze [75 complete synthetic cases](../internal/safetyprobe/ci_result_v1.json), schema `synthetic-ci-result-v1`, SHA-256 `21aa3c78597550950562341ebfe310d38b7b381da0139912b8d476b58c1d549b`. Each row contains its policy, simulated clock, candidate, optional run/head observations and manually specified expected outcome. A local Go producer materializes these inputs without evaluating a candidate. Expected outcomes are separate from the implementation but same-author, previously inspected regression data, not independent human adjudication or held-out evidence.

All repositories, workflow/run IDs, attempts, commit/revision IDs, times, source declarations and prose are synthetic. Configured fake APIs and protection status establish no authenticated provenance. The probe admits no external files, credentials, network/model calls or live corpus. No service is built and no paid API spending is authorized.

## Exact predicate and qualification

Support only `ci.result` for the configured repository/workflow, exact run ID, positive attempt, full 40-lowercase-hex commit ID, revision and completed conclusion. V1 deliberately supports only `success` and `failure`; other statuses/conclusions are withheld. A supported failure is accurate result evidence, not success. A single workflow result never means every workflow/test passed or that deployment is ready.

- `commit` qualification supports only the exact commit/run result. Branch/head-revision and environment qualifiers are forbidden. A freshly checked old run may remain supported for its old commit even when the branch moved; supplied head data is irrelevant to this narrow claim.
- `current_head` qualification additionally requires a configured head policy for the same repository, a named branch, explicit head revision and fresh matching protected-branch evidence. The candidate reuses the [head verifier](s0-repository-head-probe.md) with no changes to its behavior or fixtures. A run for an older commit, missing/stale head, protection failure or changed revision is withheld. No fallback to commit-qualified success occurs when a requested current-head dependency fails.

Run completion must precede or equal source update, update must precede or equal check, and check must not be in the future. All times must be positive. Both CI and head checks use their configured 900-second bound; exactly 900 seconds is admitted and 901 is stale. Old completion time alone does not make a freshly checked exact run result stale. Publication and the separate display-expiry policy remain unimplemented.

| Frozen coverage | Expected behavior |
| --- | --- |
| Exact success/failure, old completion with fresh check | Support only the qualified exact result |
| Old green run after head moved | Allow its commit-qualified result; withhold current-head inference |
| Matching new head and matching run | Permit the exact current-head-qualified workflow result |
| Wrong workflow/run/attempt/commit/revision/conclusion | Withhold mismatched evidence |
| Missing/stale/foreign/unprotected/unversioned head dependency | Withhold current-head qualification |
| Source prose, aggregate/deployment/environment claims | Ignore prose; withhold unsupported authority/qualifiers |
| Invalid policies, IDs, incomplete statuses, times and freshness boundaries | Apply manually specified outcomes |

Expected totals are eight supported commit-qualified results, four supported current-head-qualified results and 63 withheld outcomes. The verifier receives no expected outcomes or case names.

## Coverage boundary

This adds a narrow CI predicate slice alongside the issue-state and head probes. The other six named P03-D cases remain incomplete: unsupported decisions in PR comments; agent echo/corroboration; full source-text/tool-call handling; renamed/deleted sources; owner pin contradictions; and full unversioned A→B→A lifecycle handling. These local snapshots do not complete chronological storage or lifecycle tests.

The subsequent [decision-authority probe](s0-decision-authority-probe.md) adds a separate narrow registry/PR-comment slice without changing these CI fixtures or completing the broader P03-D matrix.

Real API parsing/authentication, atomic cross-source snapshots and publication-time race handling, coequal conflicts, permissions, source provenance, capture, expiry, deletion/repair, secret scanning, consumer behavior and unattended operation remain unimplemented. Fresh consistent fake snapshots do not prove real integration or atomicity. Publication and stage-pass flags stay false. Calibration #60 and capture #34 remain unresolved; approved baseline documents and previous frozen fixtures are preserved.

## Execution status

The expected set was committed at `dcf3fed` before candidate implementation. Run the fixed embedded set with:

```sh
go run ./cmd/ganesh-dev ci-result-probe
```

The no-input command reports **75 matches / zero mismatches**, comprising eight supported commit-qualified results, four supported current-head-qualified results and 63 withheld outcomes. An intentionally always-current-head-green candidate matches only three cases and yields **72 mismatches** under the separate scorer. That mutation also mismatches valid old-commit successes because their qualification is narrower. Appending even one newline to fixture bytes fails the pinned digest.

Output contains fixed schema/status, aggregate counts, the fixture digest and `synthetic: true`, `publication_authorized: false`, `stage_gate_passed: false`. It omits fixture identities, commit/run/workflow values, source prose and paths. Valid mismatch output uses `status: fixture_mismatch` and exits 1; invalid embedded input or failed output writes return a static error. Matching this inspected set is synthetic regression evidence only.

Full Go tests, affected normal/race/vet, CLI argument/output checks, documentation targets and diff checks pass. The previous issue-state/head fixtures and CLI outputs remain unchanged (24/24 and 43/43 respectively), as do the approved baseline documents and original private calibration artifacts. No model/service call or stage pass occurred.
