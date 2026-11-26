# Baseline hardening evidence — PR #160

This hardening preserves the original M1 result. It makes the benchmark gate
executable and citation accounting strict; it does not change retrieval.

## Citation accounting

Before: a passage with a missing or empty citation was skipped before the
citation denominator was incremented. After: every passage object returned by
each adapter contributes exactly one expected citation opening. Missing,
empty, malformed, unresolvable, or wrong-passage openings all fail that one
check.

Regression coverage in `crates/ganesh-bench/src/main.rs` tests:

- three returned passages, one lacking a citation: exactly 2/3 openings pass;
- empty, malformed, and wrong-passage citations: exactly 0/3 openings pass.

Verification replay of the original v1 frozen inputs with the hardened scorer
(label: verification of the original result, not a new evaluation):

```
cargo run --release -p ganesh-bench -- run \
  --questions fixtures/questions/heldout.v1.json \
  --report out/hardened-v1-report.md \
  --details out/hardened-v1-details.json
```

Outcome: exit 1, machine verdict FAIL; coverage 19/24, insufficient 4/6,
adapter agreement 30/30, citations 150/150, incomplete=false. The stricter
accounting found no discrepancy in the original citation count, so no erratum
is required.

## Executable verdict contract

Gate mode is the default for `ganesh-bench run`. It reads the frozen scoring
specification (default: `scoring-spec.json` beside the questions file),
enforces expected roster denominators and the top-three limit, prints a final
machine-readable JSON object, and uses distinct exit codes:

- 0 PASS
- 1 FAIL
- 3 INCOMPLETE
- 2 usage error

Empty or partial rosters, inconsistent scoring specifications, malformed
adapter responses, top-three violations, and adapter transport errors all make
the run INCOMPLETE rather than PASS. Development mode (`--mode dev`) preserves
diagnostics without enforcing the held-out roster.

Controlled fixtures under `fixtures/gate/` demonstrated all three paths:

| Fixture | Expected path | Observed |
| --- | --- | --- |
| `pass.questions.json` | PASS | exit 0, verdict PASS |
| `fail.questions.json` | FAIL | exit 1, verdict FAIL |
| `incomplete.questions.json` | INCOMPLETE | exit 3, verdict INCOMPLETE |

## Cleanup

- Removed 2,737 accidentally tracked `target/` files and tracked `out/`
  reports from the index. They remain generated, ignored local artifacts.
- Curated evidence remains under `evidence/`.
- Added `fixtures/corpus/README.md` recording that `fetch.py`, `sample.py`,
  and `segment.py` are historical provenance for the frozen v1 fixtures only.
  They are outside the Rust build/service/replay path and must not alter the
  frozen corpus.

## Clean-checkout verification

Commands run in a fresh single-branch clone:

```
git clone --branch feat/159-m1-passage-retrieval --single-branch \
  https://github.com/andikapradanaarif/ganesh.git /tmp/ganesh-m1-clean-check
cd /tmp/ganesh-m1-clean-check
cargo build --release --workspace
cargo run --release -p ganesh-bench -- run \
  --questions fixtures/questions/heldout.v1.json \
  --report out/heldout-report.md \
  --details out/heldout-details.json
```

Verified commit: `cfb29ea71dc54d2eda89470ee5e8ebb22ffb8bb0`.
Build succeeded, including the REST and MCP binaries launched by the harness.
Replay exited 1 with machine verdict FAIL and the original v1 accounting:
coverage 19/24, insufficient 4/6, adapter agreement 30/30, citations 150/150,
provider calls 0. This was verification of the frozen original result, not a
new held-out evaluation.
