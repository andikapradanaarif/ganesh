# Disposable decision-authority verifier probe

[Issue #71](https://github.com/andikapradanaarif/ganesh/issues/71), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6), prepares the **unsupported decision in a PR comment** case from roadmap [A2.1](personal-second-memory-roadmap.md#a21-s0b-1-deterministic-verifiers-and-safety-three-to-four-focused-days). [Build §5](personal-second-memory-roadmap.md#build-5-subject-authority-and-preventive-verification) assigns decision authority only to an explicitly designated structured registry for its decision ID/state and permitted enum. PR prose may support an attributed report, but authenticated transport or an owner author label cannot turn it into that registry.

This is bounded offline deterministic preparation permitted by the [execution sheet](s0-execution-sheet.md). The positive registry is entirely fake: no actual owner registry, source grant, project decision or enrollment is introduced. No connector/service, credentials, model call or paid API spending is involved.

## Frozen expectations

Before candidate implementation or execution, freeze [67 complete synthetic cases](../internal/safetyprobe/decision_authority_v1.json), schema `synthetic-decision-authority-v1`, SHA-256 `e3a9e4d6172cfd44dd1549b7c544bdb9613a0acc8813c25a3c46728263971189`. Each case contains its configured policy, simulated clock, candidate, optional observation and manually specified expected outcome. A local Go producer materializes these records without evaluating a candidate. Expected outcomes are separate from the candidate but same-author inspected regressions, not independent human adjudication or held-out evidence.

All objects, author labels, source/authentication/writer-control declarations, values, revisions, times and prose are synthetic. Those declarations are premises, not proven identity, authorization or provenance.

## Exact predicate

Support only the configured `decision.fixture_strategy` adoption record, for its configured repository/registry and decision ID, in state `adopted`, with a permitted typed value and exact revision. The fake policy explicitly designates the source and defines the permitted vocabulary (`option_a` / `option_b`). Unknown/false designation is withheld. Response prose and claimed author role cannot designate a source or widen the vocabulary.

Require the structured-registry channel and configured source, explicit mock authentication and controlled-writer status, and matching repository/registry/key/decision/revision/value. Unknown and known-false authentication/writer controls are withheld separately. Authenticated PR comments, descriptions and agent reports remain unsupported channels regardless of author label or their prose. Withholding here means no authoritative-decision support; an attributed working report is outside this probe.

Only adopted records qualify. Proposed/rejected/superseded records and malformed states are withheld. A supported result carries both the exact enum value and `state: adopted`; it does not establish implementation correctness, deployment readiness or an independent semantic/organizational truth. Environment, convention and deployment claims are unsupported.

The configured recheck bound is 604,800 seconds (seven days), borrowing the enduring convention/decision default in [Build §6](personal-second-memory-roadmap.md#build-6-verified-typed-publication-and-injection-boundary). Adoption/update/check times must be positive and ordered, with no future check. The exact seven-day boundary is admitted; one additional second is stale. An old adoption with a fresh unchanged check remains a record of the same decision. Publication and the separate 30-day display expiry are unimplemented; no owner review timestamp is a freshness input.

| Frozen coverage | Expected behavior |
| --- | --- |
| Exact adopted registry record and alternate permitted value | Support only the exact value and adoption state |
| Authenticated/owner-labeled PR prose or self-designation | Withhold unsupported channel/designation |
| Missing/false designation, authentication or writer control | Withhold missing authority premises |
| Wrong source/repository/registry/key/decision/revision/value | Withhold mismatched or unconfigured support |
| Proposed/rejected/superseded or malformed states | Never promote to adoption |
| Instructions in prose, IDs, values or vocabulary | Ignore prose and reject values outside typed policy |
| Invalid policy, times and exact freshness boundaries | Apply manually specified outcomes |

Expected totals are five supported adoption records and 62 withheld outcomes. The candidate receives no case names or expected outcomes.

## Coverage boundary

This adds one narrow decision-authority slice alongside the issue/head/CI probes; it does not complete P03-D. The five other named cases remain incomplete: agent echo/corroboration, full source-text/tool-call handling, renamed/deleted sources, owner pin contradictions and full unversioned A→B→A lifecycle handling. Local prose/revision negatives do not complete those chains or consumer tests.

The subsequent [echo-origin probe](s0-echo-origin-probe.md) adds a separate declared-lineage slice for agent/tool echoes. It leaves these decision fixtures and the broader P03-D gate unchanged.

Real owner/source enrollment, registry/owner-command integration, authenticated parsing, provenance, conflict/transition rules, policy revocation propagation, transactional publication, capture, expiry/deletion/repair and unattended operation remain outside this probe. Mock true authentication/writer controls are not attestation. Publication/stage flags remain false; #34 capture and #60 calibration remain unresolved. Approved baseline documents and earlier frozen fixtures remain unchanged.

## Execution status

The expectations were committed at `91e6043` before candidate implementation. Run the fixed embedded set with:

```sh
go run ./cmd/ganesh-dev decision-authority-probe
```

The no-input command reports **67 matches / zero mismatches**, with five supported adoption records and 62 withheld. The separate scorer compares kind, reason, adoption state and value against the frozen records. Intentionally always-adopt and always-deny candidates each yield **63 mismatches**; a candidate omitting the adoption state yields **67 mismatches**. Modified fixture bytes fail the pinned digest.

Output contains fixed schema/status, aggregate counts, the fixture digest and `synthetic: true`, `publication_authorized: false`, `stage_gate_passed: false`. It omits fixture identities, enum values, source prose, author labels and paths. A valid mismatch report uses `status: fixture_mismatch` and exits 1; invalid embedded input or failed output writes use a static error. Matching expectations establishes only this inspected synthetic regression result.

Full Go tests, affected normal/race/vet, CLI argument/output checks, documentation targets and diff checks pass. Earlier issue/head/CI fixtures, code and CLI results remain unchanged (24/24, 43/43 and 75/75). Approved baseline documents and original private calibration artifacts are preserved. No actual registry enrollment, model/service call, publication or stage pass occurred.
