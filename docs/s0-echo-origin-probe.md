# Disposable agent-echo origin probe

[Issue #73](https://github.com/andikapradanaarif/ganesh/issues/73), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6), prepares the **two agents echoing one false source** case in roadmap [A2.1](personal-second-memory-roadmap.md#a21-s0b-1-deterministic-verifiers-and-safety-three-to-four-focused-days). [Build §5](personal-second-memory-roadmap.md#build-5-subject-authority-and-preventive-verification) says agents echoing one origin are not independent corroboration. The bounded tool→agent→agent cases also exercise declared ancestry preservation; they do not complete real trusted-tool or model-consumer integration. The [execution sheet](s0-execution-sheet.md) permits this offline deterministic preparation while model-dependent work remains pending.

## Frozen expectations

Before candidate implementation or execution, freeze [50 complete synthetic graphs and manually specified outcomes](../internal/safetyprobe/echo_origin_v1.json), schema `synthetic-echo-origin-v1`, SHA-256 `d4bff21a0ee3620c1815c1f4e322878cddc1b31c9a9ad7cfcf23759a865334ee`. A local Go producer materializes those records without evaluating the candidate. Expected outcomes are separate from the tracer but same-author inspected regressions, not independent human adjudication or held-out evidence.

All projects, nodes, origin identities, classes, revisions, authentication/author labels and prose are synthetic declarations. The untrusted source is seeded with a false fixture-service deployment statement; its falsity is a scenario premise, not something the tracer detects. Declared provenance is not attestation or proof that real ancestry is complete. No source/model/service call, actual corpus enrollment, credentials or paid API spending is involved.

## Exact trace contract

The input is one closed single-project snapshot with source, tool, agent and copy nodes, parent IDs and selected target IDs. Validate the whole snapshot, including unselected nodes. Bounds are 1–32 nodes, 1–8 target entries, at most eight parent entries per node and at most 16 nodes on any source-to-target ancestry path. Duplicate parent/target entries are admitted within those bounds and cannot inflate counts. IDs use the existing bounded token grammar.

Sources have no parents and declare a source/object/revision identity plus `untrusted_narrative`, `unknown` or `structured_observation` class. Derived nodes require ancestry; their claimed class, source fields, authentication/author labels and prose cannot replace their parents' origins. Trace selected ancestry through a DAG, regardless of node/parent order.

Count distinct declared origins by **source + object**, not by agent, tool, node ID or revision. Count origin revisions separately by source + object + revision. Alias nodes with the same identity collapse; two revisions of one object remain one origin. Root-class counts are per distinct origin, with no scalar upgrade. Conflicting classes for one canonical origin make the snapshot incomplete rather than choosing a stronger class.

Unselected valid origins do not enter selected counts. Missing IDs/parents/targets, missing ancestry/origin metadata, foreign projects, invalid kinds/classes, cycles, conflicts or exceeded bounds return incomplete lineage with zero partial counts. Memoized ancestry height checks must enforce the depth bound independently of traversal order.

A valid trace returns `level: reported`, even for multiple distinct origins or a declared structured root. It never establishes independence, truth, authority, eligibility or publication permission. Distinct declared origin identities may still hide shared ancestry or aliases outside the snapshot. All outcomes retain `independence_established: false`; invalid snapshots use `level: unknown`.

| Frozen coverage | Expected behavior |
| --- | --- |
| Two agents repeat one seeded false source | One untrusted origin, reported only |
| Authenticated-tool echo or forged derived class/source metadata | Original root classification and identity remain |
| Aliases, diamonds and duplicate edges/targets | Union ancestry without multiplying evidence |
| Different revisions of one origin | One origin, separate revision count |
| Two distinct declared origins | Count both, but do not establish independence |
| Explicit unknown/structured roots and mixed classes | Preserve separate declared class counts without verification |
| Prose instructions | No change to ancestry, classification or authority |
| Missing/cyclic/conflicting/foreign/oversized snapshot | Incomplete lineage; no partial denominator |

Expected totals are 23 reported lineages and 27 incomplete snapshots. The candidate receives no case name or expected outcome.

## Coverage boundary

This is a declared-lineage slice alongside the four predicate probes, not a full P03-D pass. The other four named cases remain incomplete: full source-text/tool-call handling, renamed/deleted sources, owner pin contradictions and full unversioned A→B→A lifecycle handling. A fixed source/tool/agent graph is not evidence of real client capture, authenticated origin, hidden dependencies, semantic truth or an LLM's resistance to instruction adoption.

Live provenance/capture reconciliation, root enrollment/alias discovery, source-specific independent verification, permissions, transactional publication, conflict/lifecycle/expiry/deletion/repair and unattended operation remain outside this probe. Previous probes and approved baseline documents are preserved. Publication/stage flags stay false, and #34 capture/#60 calibration remain unresolved.

## Execution status

Expectations are frozen before implementation. Candidate execution and validation results will be recorded after running the separate scorer; this initial freeze claims no passing result.
