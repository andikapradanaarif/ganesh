# Disposable issue-state verifier probe

[Issue #65](https://github.com/andikapradanaarif/ganesh/issues/65), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6), prepares one deterministic predicate from roadmap [Build §5](personal-second-memory-roadmap.md#build-5-subject-authority-and-preventive-verification) and [A2.1](personal-second-memory-roadmap.md#a21-s0b-1-deterministic-verifiers-and-safety-three-to-four-focused-days). The [execution sheet](s0-execution-sheet.md) permits appropriate deterministic work while model-dependent inputs remain pending. This is a disposable probe, not a service, connector, publication gate or full S0b-1 execution.

## Frozen inputs and independent expectations

Before implementing or running the candidate verifier, freeze [24 synthetic cases](../internal/safetyprobe/issue_state_v1.json), schema `synthetic-issue-state-v1`, SHA-256 `dfef7a7c5c0df95f57ec09a8e14b51fe2e1342d8c0cc57ea7eb426ecb4244298`. Every case contains a complete candidate, optional complete observation and manually specified expected outcome. A local Go producer materialized these inputs; it does not run the candidate or derive expected answers from it. These expectations are separate from the implementation, authored by the same assistant and open to review; they are not independent human adjudication or held-out evidence.

All objects, revisions, timestamps and prose are synthetic. The configured fake source is `fixture.issue-api`, repository `synthetic/project`, and the current-check bound is 900 seconds (the roadmap's 15-minute default). Fixture time is a simulated integer clock, not a measurement of a real API. A configured fake channel/source establishes no authenticated real-world provenance.

## Exact predicate

The candidate can support only the enum `issue.state`, `open` or `closed`, for the exact configured repository and positive issue ID. Require matching candidate/observation state and nonempty revision, a structured issue-API channel and configured source, valid change/check times and a check no older than 900 seconds. The exact 900-second boundary is admitted; 901 seconds is stale. Future checks, future source changes or a check before the source change fail. Revisions are bounded opaque identifiers, not inferred event order or independent evidence.

A positive result means only that the declared structured observation matches that exact enum predicate. It cannot establish resolution, deployment readiness, source truth, narrative eligibility, publication permission or authority to run a tool. Prose is never used to support a value. No network calls or action capabilities exist in this probe.

| Frozen group | Expected behavior |
| --- | --- |
| Exact open/closed, freshness boundary | Support only the matching enum |
| Resolution/deployment inferred from closure | Withhold unsupported subjects |
| Comment or unconfigured source/repository | Withhold unsupported authority/scope |
| Different repository/issue/value/revision | Withhold mismatched evidence |
| Missing evidence or revision | Withhold; absence cannot become false or resolved |
| Stale/future/reversed check times | Withhold invalid current support |
| Reopened issue or old revision after A→B→A | Withhold mismatched state/revision; do not infer chronology |
| Tool instructions in source prose | Preserve only the structured enum; prose cannot enter the result |
| Instructions in state/revision fields | Reject values outside the typed grammar |

## Coverage boundary

This is preparation for the **issue-closure-without-resolution** predicate only. The remaining eight named P03-D verifier cases are incomplete: commit existence without deployment; green CI for an old commit; unsupported decisions in PR comments; agent echo/corroboration; full source-text/tool-call handling; renamed/deleted sources; owner pin contradictions; and full unversioned A→B→A lifecycle handling. The local revision/prose negatives do not complete those broader cases.

Chronological state storage, coequal conflicts, permissions, disclosure, typed publication, taint, provenance, secret scanning, capture reconciliation, lifecycle/deletion, control races, unattended operation and repair are also unimplemented here. Missing coverage stays incomplete. Source support is not product value; neither this probe nor a passing fixture permits a hosted build, narrative activation or a stage pass. Calibration [#60](https://github.com/andikapradanaarif/ganesh/issues/60) and capture [#34](https://github.com/andikapradanaarif/ganesh/issues/34) remain unresolved. No paid API spending is authorized.

## Execution status

Expectations are frozen before candidate implementation. Candidate results and validation will be recorded after execution; no passing result is claimed by this initial freeze.
