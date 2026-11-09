# Disposable repository-head verifier probe

[Issue #67](https://github.com/andikapradanaarif/ganesh/issues/67), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6), prepares the roadmap's **commit existence without deployment** case alongside the [issue-state probe](s0-issue-state-probe.md). [Build §5](personal-second-memory-roadmap.md#build-5-subject-authority-and-preventive-verification) requires a configured repository API resolving a named protected branch for `repository.head`; a commit lookup alone proves neither that head nor deployment. This is appropriate offline deterministic preparation under the [execution sheet](s0-execution-sheet.md), not a connector, service or stage pass.

## Frozen expectations

Before implementing or running the candidate, freeze [43 complete synthetic cases](../internal/safetyprobe/repository_head_v1.json), schema `synthetic-repository-head-v1`, SHA-256 `9e4775604cbbf263c79859cfc2ccee02501c072f3779454a5ec29ec9ddf12db4`. Each row contains its policy, simulated clock, candidate, optional observation and manually specified expected outcome. A local Go producer materializes these records without running a candidate. The expectations are separate from the implementation but authored by the same assistant; they are not independent human adjudication or held-out evidence.

All source IDs, repository/branch names, commit IDs, revisions, times and prose are synthetic. A configured fake source/channel is a premise, not proof of API authentication or real branch protection. The v1 commit grammar is exactly 40 lowercase hexadecimal characters; it is deliberately narrower than a general repository identifier parser.

## Exact predicate

Support only `repository.head` for the configured repository and branch, without an environment qualifier. Require the configured source and structured protected-branch channel, explicitly true protection status, matching repository/branch/commit/revision, valid bounded revision IDs and valid source-change/check times. Unknown protection and known false protection are withheld separately. Commit lookup, tag lookup and repository prose cannot substitute for branch resolution.

The frozen policy uses a 900-second recheck bound, borrowing the explicit volatile CI/issue/head default in [Build §6](personal-second-memory-roadmap.md#build-6-verified-typed-publication-and-injection-boundary). A check exactly 900 seconds old passes; 901 seconds is stale. Nonpositive/future/reversed times fail. The probe does not implement publication or the separate display-expiry rule.

| Coverage | Expected behavior |
| --- | --- |
| Exact head, changed head, freshness boundary | Support only the matching commit ID |
| Commit existence, tag or prose without branch resolution | Withhold unsupported evidence channels |
| Deployment/policy inference or environment-qualified head | Withhold unsupported subjects/qualifiers |
| Foreign source/repository/branch or protection false/unknown | Withhold unsupported authority/scope |
| Wrong commit/revision, old revision after A→B→A, absent evidence | Withhold mismatched or missing support |
| Invalid policy, timestamps, typed commit/revision IDs | Withhold the specified rejection reason |
| Tool/deployment instructions in source prose | Carry only the structured commit ID; never copy prose |

The expected set contains four supported-head outcomes and 39 withheld outcomes. Supporting a head predicate does not establish deployment, correctness, organizational policy, narrative eligibility, disclosure permission or authority to act.

## Coverage boundary

This adds a narrow commit/head predicate slice. Together with the issue-state probe it does not complete the nine-case P03-D matrix. The seven other named cases remain incomplete: green CI for an old commit; unsupported decisions in PR comments; agent echo/corroboration; full source-text/tool-call handling; renamed/deleted sources; owner pin contradictions; and full unversioned A→B→A lifecycle handling. Local revision/prose negatives are not those complete lifecycle or consumer tests.

Permissions, real API parsing/authentication, coequal conflicts, chronological storage, provenance, capture reconciliation, publication/display expiry, deletion/repair, secret scanning and unattended operation also remain outside this probe. No model/service calls, credentials, new corpus enrollment or paid API spending are authorized. Calibration #60, capture #34 and stage gates remain unresolved. The approved roadmap and execution sheet remain unchanged.

## Execution status

The expected records are frozen before implementation. Candidate execution and validation results will be recorded after running the separate comparison scorer; this initial freeze claims no passing result.
