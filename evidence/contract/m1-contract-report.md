# Evidence contract + proposition-level acceptance — development report (issue #165)

**Explicit findings: (1) contract/scorer corrected — yes; (2) candidate warrants a fresh M1 evaluation — no; (3) candidate still fails the development check — yes.** Combination: contract corrected, candidate fails transfer.

Branch point: `ee81fd5` (PR #164 pin); intended review base: diff against `ee81fd5`, not the GitHub `main` base. All prior freezes, keys and results preserved. v3 unconsumed; M2 parked. Ranking, embeddings, database, top-three fixed throughout.

## 1. Contract and audit

`evidence/contract/evidence-contract.md`: ok = passages settle the fact — affirmative, explicit negative, or requested value; insufficient = material part unresolved; plus resolution tests (absence≠negative, no transfer across environment/actor/state, weaker epistemic states don't settle stronger ones, conflicts don't settle). Audit of all ten v1 transition cases: 7 confirmed, **qB06 relabeled insufficient→ok-negative (benchmark correction)**, qB02/qB07 split into unambiguous variants. New `key.v2.json` (12 cases) with span-level acceptable evidence, negative-answer spans, missing-fact + misleading-passage records for insufficient cases. Historical 9/10 preserved as recorded. Consequence: part of the previous experiment's apparent abstention improvement was a key artifact — qB06's abstention had been "correct" for the wrong reason.

## 2. Scorer: correct evidence as a condition of credit

Extended `ganesh-bench`: fixture validation rejects answerable cases with no acceptable evidence before the run (demonstrated: the legacy quantity key was rejected exit 2 until upgraded); span-level evidence checking inside the cited passage text; negative spans earn credit; reports distinguish status correct / evidence correct / end-to-end. Regression tests include the required case: right status + right passage ID but the sentence concerns a different environment → no credit. Public API unchanged; polarity labels stay evaluator-internal.

## 3. Comparison on identical pinned inputs

Reference: lexical acceptance (quantity binding held on in both arms; independently switchable). Candidate: deterministic proposition verdicts — support / contradict / unresolved — extracting predicate, object identifiers, environment and polarity from the question alone, with within-passage identifier inheritance for terse logs. Chosen over a pinned NLI model for transparent offline replay and reviewer-inspectable failure analysis; no new dependencies, sub-millisecond cost.

### Results (all development evidence)

| Input | Arm | status-ok | evidence | end-to-end | insufficient |
|---|---|---|---|---|---|
| transition v2 (12) | ref | 5/8 | 5/8 | 5/8 | 4/4 |
| transition v2 | prop | 6/8 | 6/8 | 6/8 | 2/4 |
| **transfer check (8)** | **ref** | 5/5 | 5/5 | 5/5 | **0/3** |
| **transfer check** | **prop** | **2/5** | 2/5 | 2/5 | **3/3** |
| quantity controls (8) | both | 6/6 | 6/6 | 6/6 | 2/2 |
| exposed v2 (30) | ref | 23/24 | 21/24 | 21/24 | 5/6 |
| exposed v2 | prop | 23/24 | 21/24 | 21/24 | **6/6** |

Changed decisions on the transfer check: t03/t06/t08 leak→correct abstention (the explicit-negative/unknown distinction works: "the integrity check was not run" correctly settles neither pass nor fail; the v27-style release-note case transfers). t01/t02/t04 hit→false abstention. Net end-to-end correct on the check: ref 5/8 vs prop 5/8 — **no improvement**. Quantity controls preserved exactly; exposed v2 abstention completes to 6/6 with no coverage cost.

## 4. Why it fails — the smallest defeating example

t01: "The backup of archive volume V12 started at 02:00 UTC. **The transfer completed at 02:41 UTC** and the integrity check passed." The decisive sentence is a terse continuation: it carries the revision identifier only by inheritance, and none of the question's object nouns. Binding strictly false-abstains it; binding loosely re-accepts qB08/t06-style leaks where the modifier belongs to a different property. This is the same lexical ceiling as qB07/qB08 in #163, now reproduced on independent transfer subjects — unresolved part: **within-passage coreference of object nouns**, i.e. evidence interpretation, not candidate retrieval and not the contract.

The negative-case machinery itself is sound: qB06 settles ok on "The team did not confirm revision R6" with verdict=contradicts and the decisive sentence traced; t02's failure is binding-side, not polarity-side.

## 5. Reproduce

```
cargo build --release --workspace
env GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 cargo run --release -p ganesh-bench -- run \
  --questions fixtures/transfer-check/key.json --corpus fixtures/transfer-check \
  --report out/xfer.md --details out/xfer.json --mode dev
# reference: drop GANESH_PROPOSITION_CHECK. probe subcommand prints relied-upon spans with verdicts.
```

Costs: ~2s per 8-12 question fixture run; exposed v2 ~1.8s/30 questions; zero provider calls, zero network, no model weights. The REST/MCP shared path is exercised on every run — affirmative (t05), negative-intended (t02), insufficient (t03/t06/t08) — adapter agreement 8/8, 12/12, 30/30 across inputs.

## Recommendation

Do not spend a fresh M1 evaluation on this candidate. The contract and scorer are the durable output: future candidates can now be measured on whether evidence actually settles the question — including explicit negatives — rather than on lexical coverage. The next attack on the binding problem is semantic (per the roadmap's deferred hybrid comparison) or structured passage context, not further lexical tightening.
