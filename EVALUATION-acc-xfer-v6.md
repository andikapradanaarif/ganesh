# EVALUATION — acc-xfer-v6 paired transfer evaluation

Candidate pin: `acf4c9c44a97218b63c4200b9cc2ed88198d7fd3` · Packet freeze: `52649220a81173244cbdf8cfca182248da2aa254 (after two honest parse-schema refreezes, zero outcomes observed at each step)` (committed BEFORE any acc-xfer-v6 evaluation; candidate pinned after freeze, before evaluation) · Baselines: `#188` = `8f273aaa`, `6c593a1d` intact.

## 1. INT8 ONNX Runtime upgrade

- Reranker: `cross-encoder/ms-marco-MiniLM-L-6-v2` INT8 dynamic-quantized ONNX — `model-int8.onnx` **23,180,880 bytes**, sha256 `5741182c012282d6fe025705a0a4b43884c9f778da01f7ac7b20ff82097829a5`.
- QA verifier: `deepset/minilm-uncased-squad2` INT8 dynamic-quantized ONNX — `model-int8.onnx` **33,784,916 bytes**, sha256 `209727cb5bfa77e9e60df6ff257078119be73ed86378a563791f2c8021d7f238`.
- Runtime: `ort` (ONNX Runtime CPU execution provider), candle fully removed, single-pass length-sorted batching over the whole candidate pool, zero network at runtime, 0 provider calls.
- Measured (exposed v5+v4, 54 queries): REST mean **1191 ms** / max 2744 ms; MCP mean 1199 ms / max 2611 ms. Peak RSS **1.32 GB** (< 1.5 GB target). Latency is dominated by the 50-candidate rerank batch; pool-size reduction to 25/20 cuts wall time substantially (probe wall 1.9 s pool-25, 1.7 s pool-20 vs higher at 50) but pool size was left at the frozen 50 for correctness parity in this evaluation. The 600 ms target is NOT met at pool 50 — honest shortfall, see readiness section.
- INT8 fidelity smoke: QA margin fp32 15.44 vs INT8 14.31, identical span.

## 2. Deterministic acceptance fixes

- All five evidence flags default-on (`quantity_binding`, `transition_check`, `proposition_check`, `context_chain`, `action_check`), env-overridable.
- `transition_check` activation: occurrence verbs (`occur`/`happen`) never activate; on extractive wh-questions only strict operational-state cues activate. Fixes `w13` while preserving `i06`.
- `requested_unit` verification with full physical unit alias table + circular-binding guard (unit tokens excluded from measured-property witnesses) + decimal-version disambiguation. Verified: `v5/i02`, `ax4/q09`, `ax5/q10` abstain; `ax5/q03` (172.5 m) hits.
- Clause-scoped `exact_date_query` substantive overlap: compound split on coordinator boundaries, overlap required only in the triggered clause. Verified: `ax5/q08` hits, `ax5/q09` abstains.
- Future-tense date guard: explicit future cues required for `will`-questions. Verified: `v5/i03` abstains.

## 3. Fact-checking encoder calibration — OMITTED (predeclared gate)

Model: `MoritzLaurer/DeBERTa-v3-small-mnli-fever-docnli-ling-2c` (the spec's exact `…-anli-ling-wanli` repo does not exist, 401; closest real FEVER-trained small variant substituted and documented), INT8 ONNX 172,231,063 bytes, sha256 `c3759c1ff4f186c3b0690876f264b30ec01acd206346fedca869918fac8654fa`, id2label `{0: entailment, 1: not_entailment}`.

Sample generated declarative claims:
- "The first attempt of the Vancouver Expedition to summit Mauna Loa succeeded in 1839."
- "Pierre Curie won the Nobel Prize in Chemistry in 1911."
- "The first attempt to complete the construction of Cologne Cathedral succeeded in 1814."
- "The Leaning Tower of Pisa was reopened to the public on 15 december 2001."

Exposed calibration at tau=0.5 (max entailment over top-3 passages):

| case | max entail | outcome |
|---|---|---|
| v5/i04 (trap) | 0.042 | caught |
| ax5/q12 (trap) | 0.143 | caught |
| ax4/q07 (trap) | 0.371 | caught |
| ax3/q08 (trap) | 0.023 | caught |
| qax1/q09 (trap) | **0.972** | NOT caught |
| v5/w16 control | 0.094 | false abstention |
| v5/s01 control | 0.162 | false abstention |
| v5/s06 control | 0.224 | false abstention |
| ax5/q01 control | 0.012 | false abstention |
| ax5/q02 control | 0.018 | false abstention |

Predeclared gate: (a) requires >=3/4 primary traps with <=1 false abstention across all v5+v4+v2 answerable controls. Traps: 3/4 caught (i04, q12, q07; q09 entity-swap uncatchable). But **5 false abstentions** on controls — even grammatically perfect claims (ax5/q01/q02) score <=0.02 entailment on real full-passage premises. No threshold separates. **Factcheck OMITTED honestly**; model archived outside the pipeline; `factcheck.rs` remains inert (model absent -> `available()` false).

## 4. Exposed-suite retention matrix (candidate, INT8 ONNX)

| suite | result | gate |
|---|---|---|
| v4 | **24/24 + 5/5 genuine abstentions** | PASS (best ever; i05 erratum answered as desired) |
| v5 exposed | **23/24 + 5/6** (leaks: i04 inverted-outcome only; miss w09) | PASS (coverage >= 23/24 exactly met, abstention strictly > 3/6) |
| v2 | **29/30** (23 hits + 6/6 abstention, v04-v06 compounds hit, v28+v30 abstain) | PASS |
| qa-xfer-v1 | **8/8 + 5/6** (sole leak q09 entity-swap) | — |
| acc-xfer-v5 exposed | **7/8 + 5/6** (leak q12 inverted-outcome) | — |
| acc-xfer-v4 exposed | 7 hits + q04/q09 correct abstentions; leaks q03/q07 | — |
| probes | event 12/12, action 8/8, chain 13/13, contrast-quantity 8/8, rail 8/8 = **49/49** | PASS |

## 5. Paired fresh frozen acc-xfer-v6 — results

Packet: musical-instrument domain — Piano, Violin, Saxophone, Theremin, Harpsichord, Pipe organ. 0 overlaps vs every manifest (programmatically verified across fixtures/ and all worktrees). Preflight PASS, 0 problems (3 benign duplicate-fact warnings from compound reuse). Grep audit: zero kilogram hits anywhere in corpus (q10/q11 clean), zero tanner-invent collisions (q13), Maffei only year 1711 (q09), Teflon experiment explicitly abandoned 1982 (q12), zero production-resumption mentions (q14).

| arm | coverage | abstention | leaks | answerable misses |
|---|---|---|---|---|
| #188 | 6/8 | 3/6 | q09, q12, q13 | q01, q07 |
| 6c593a1d | 8/8 | 4/6 | q12, q13 | — |
| candidate | **7/8** | **4/6** | q12, q13 | q06 |

Per-case outcomes are identical across arms except: q01/q07 (exact-date compound pair) — #188 misses both, baseline and candidate hit; q06 (realized-sibling Vibratosax) — baseline hits, **candidate misses** (regression); q09 (year-only exact-date) — #188 leaks, baseline and candidate correctly abstain; q10, q11, q14 correctly abstain in ALL three arms; q12 (inverted-outcome Teflon) and q13 (entity-swap Tanner) leak in ALL three arms — structural, not regressions.

## VERDICT: REJECTED

Predeclared retention gates on acc-xfer-v6:
1. candidate coverage >= 6c593a1d: **FAIL** (7/8 vs 8/8 — q06 regression)
2. candidate coverage > #188: PASS (7/8 vs 6/8)
3. abstention strictly > 6c593a1d: **FAIL** (parity 4/6)
4. abstention strictly > #188: FAIL (parity — 4/6 vs 3/6 IS strictly greater, but moot given gate 3)
5. zero newly unsupported ok vs 6c593a1d: PASS (none — q12/q13 leaks shared across all arms)

Both primary gates (coverage-parity and strict abstention improvement) fail. The candidate is REJECTED despite: the sub-second-class INT8 ONNX upgrade being real (18-30x faster than candle fp32 at parity correctness), the exposed matrix being the strongest measured to date (v4 24/24+5/5 perfect, v5 23/24+5/6, v2 29/30+6/6, 49/49 probes), and the packet quality being verified. The deterministic acceptance machinery does not yet generalize beyond its tuning distribution — the same conclusion as PR #200, now measured on a second independent frozen packet.

## 6. Costs and certification

- Fail-loud guards verified: `GANESH_REQUIRE_RERANKER=1` / `GANESH_REQUIRE_QA_VERIFIER=1` exit 2 on missing weights.
- Digests logged in every report: reranker sha256 `5741182c…`, qa-verifier sha256 `209727cb…`.
- 0 provider calls; incomplete run: false.
- Peak RSS 1.32 GB; REST mean 1191 ms at pool 50.
