# EVALUATION — v6 formal M1 gate + multi-genre stress test

Candidate pin: **`81a0f315`** (exact retained M1 build; `git diff 81a0f315..HEAD -- crates/` is empty, 0 lines — no code modified). Freeze SHA: **`8c98552a9e7c6f973e36aae80fe2da5d2b81f1ce`** (both packets frozen in one clean commit before any run; single-shot discipline honored — no question, key, manifest, threshold, or code edited after either run). Mode: held-out **gate**, `GANESH_REQUIRE_RERANKER=1` + `GANESH_REQUIRE_QA_VERIFIER=1`, both runs `incomplete: false`, **0 provider calls**, sequential serial execution (no overlap).

Certified model digests in both reports: reranker `fixtures/reranker-model/model-int8.onnx` int8-onnx sha256 `5741182c0122…`; qa-verifier `fixtures/qa-verifier-model/model-int8.onnx` int8-onnx sha256 `209727cb5bfa…`.

## Pre-freeze audits

**Disjointness (programmatic):** ZERO title/pageid/article-id overlaps against every `manifest.json` in `fixtures/` and `.local/wt/*/fixtures/` (107 prior titles).

- v6-core articles: Terracotta Army (pageid 148653), Tutankhamun (30437), Pompeii (21476593), Angkor Wat (189351) — ancient archaeology/wonders, a fresh domain disjoint from programming languages, space, volcanoes, cathedrals, instruments, ships — plus synthetic CC0 Lockmere canal lock operations log (fresh operational domain disjoint from lighthouse/seed-vault/waterworks/fire-lookout/rail-yard/pipeline/backup/ground-station/kiln/cableway/balloon/supply-vessel/archive/handoff).
- v6-broad docs: personal_notes, meeting_transcript, personal_project_b, engineering_rfc, incident_postmortem, release_checklist — all synthetic CC0 with designed near-neighbor vocabulary overlap.

**Preflight:** PASS with 0 problems on both packets; every `acceptable` locator resolves against production ingestion chunking; all bytes/sha256 verified on disk.

**Anti-erratum grep audit (all 12 insufficient controls):** i01 — search-begin exists only as year 1915, no calendar date anywhere; i02 — `deep`/`depth` hits are unrelated (emperor's tomb construction, tephra layers, depth-signage); i03 — zero `hands` hits; i04 — Yang Zhifa appears only in terracottaarmy.txt, zero in tutankhamun.txt; i05 — zero curing-verified hits; i06 — only the repainting *proposal* exists. bi01 — zero implemented-autoscaling hits (documented as proposed-and-rejected); bi02 — zero staging-freeze hits (RFC explicitly exempts staging); bi03 — zero airline hits; bi04 — zero watt/power-supply hits; bi05 — zero tracker-switch-date hits (proposal explicitly undecided); bi06 — zero runbook-verified hits (documented as started-only).

## v6-core — formal M1 gate: **PASS**

| check | result | required |
|---|---|---|
| evidence coverage | **21/24** | ≥ 20/24 PASS |
| insufficient evidence | **6/6** | 6/6 PASS |
| adapter agreement | 30/30 | 30/30 PASS |
| citation integrity | 126/126 (100%) | 100% PASS |
| provider calls | 0 | 0 PASS |
| incomplete | false | — |

Per-question: hits w01–w05, w07, w10–w16, s01–s08 (all 8 synthetic). Misses (all honest abstentions, zero wrong answers):

- **w06** (long-passage, reopening date at word ~390 of tutankhamun-a1#p110): abstained — evidence deep in a 394-word passage; consistent with the known neural-selection ceiling (same class as v7/q06).
- **w08** (compound split-vocabulary exact-date, France/Cambodia/Siam): abstained — compound evidence binding across clauses.
- **w09** (decimal measurement, 2.5 km of walls): abstained — measurement binding on the kilometre unit in a multi-measurement passage.

Notable: **6/6 insufficient correct with zero leakage on the formal gate** — the year-only trap, missing-measurement, wrong-unit, false-premise entity-swap, and both synthetic unverified/unapproved controls all abstain correctly. The abstention machinery generalizes to a fully fresh domain.

## v6-broad — multi-genre stress test: coverage **FAIL** (14/24), abstention **6/6**

| check | result | required |
|---|---|---|
| evidence coverage | **14/24** | ≥ 20/24 FAIL |
| insufficient evidence | **6/6** | 6/6 PASS |
| adapter agreement | 30/30 | 30/30 PASS |
| citation integrity | 84/84 (100%) | 100% PASS |
| provider calls | 0 | 0 PASS |
| incomplete | false | — |

Per-challenge breakdown:

| challenge type | score | detail |
|---|---|---|
| exact symbols/flags/env/errors | 5/6 | b05 1.6.2, b09 DEPLOYCTL_MAX_PARALLEL, b11 flags, b12 ERR_QUORUM_TIMEOUT, b13 kubectl hit; **b08 ENOINVENV miss** |
| long-passage past word 350 | **2/2** | b14 (alert timing, ~word 390), b15 (scale-back, ~word 440) both HIT — sliding window works on realistic engineering text |
| compounds | 3/4 | b11, b15, b16 hit; b21 miss |
| scoped qualifiers | 2/4 | b06, b10 hit; b03 (Project A), b18 (staging decision) miss |
| meeting decisions | 2/3 | b17, b19 hit; b18 miss |
| checklist completion/state | 2/4 | b22, b24 hit; b21, b23 miss |
| paraphrased lookups | **1/6** | only b06 hit; b01/b04/b07/b20/b23 all miss |
| personal-notes genre overall | **0/4** | b01–b04 all abstain |
| all 6 insufficient controls | **6/6** | zero leakage, incl. proposed-not-applied, wrong-environment, started-not-verified |

Every miss is an honest abstention with zero passages returned — **no wrong answers and no leaks anywhere in either packet** (12/12 insufficient correct overall). The failure mode is recall, never fabrication.

Root-cause analysis of the misses:

- **Paraphrased/personal genre (b01–b04, b07, b20, b23):** BM25 lexical retrieval depends on shared vocabulary; questions phrased with natural synonyms ("jogging" vs "running", "abandon" vs "gave up", "fly" vs "flight") share few stems with the note text, so the evidence never enters the candidate pool — and the deterministic honesty layer correctly refuses to answer rather than guessing. First-person casual journal prose compounds this: entries lack explicit subjects ("Booked the flight…" with no noun phrase matching "the user"), and QA-span extraction trained on formal Wikipedia-style text degrades on casual register.
- **b08 (ENOINVENV):** the error-code answer sits inside punctuation-heavy CLI prose; `--env`/`--canary` flag tokens fragment under stemming, and the exact-date/quantity deterministic machinery has no grip — evidence likely never selected.
- **b18 (staging-scope decision):** the answer exists only in casual dialogue-turn form ("my proposal is we adopt deployctl for staging only") — speaker-turn formatting and first-person proposal phrasing defeat both retrieval and extraction, even though the decision is verbatim.
- **b21 (checklist compound who+time):** compound binding across two facts in structured checklist syntax.

## Latency (serial, clean)

| packet | REST mean | REST max | MCP mean | MCP max |
|---|---|---|---|---|
| v6-core (688 passages, 5 articles) | 2,170 ms | 5,349 ms | 2,272 ms | 7,680 ms |
| v6-broad (36 passages, 6 small docs) | **736 ms** | 2,259 ms | 747 ms | 2,454 ms |

Latency scales with corpus size/candidate count, as expected from the pool-40 rerank batch plus sequential QA verification. No generative provider calls anywhere; cost is pure local CPU.

## M1→M2 gap summary

The two packets together draw a precise picture of the M1 ceiling:

1. **Lexical retrieval is the primary recall bottleneck.** Paraphrased natural-language lookups (1/6) and casual first-person genres (0/4 on personal notes) fail at the BM25 stage before any neural or deterministic machinery runs. This is the strongest concrete motivation for **M2 hybrid dense embeddings** — semantic recall exactly where lexical overlap fails.
2. **Honesty is fully generalizing.** 12/12 insufficient correct across a fresh formal domain AND six realistic assistant/engineering failure modes, zero unsupported answers anywhere — the deterministic abstention layer is domain-independent. M2 can build confidence metadata (3-tier) on top of this foundation rather than repairing it.
3. **Long-passage extraction is solved engineering** (2/2 past-word-350 hits in v6-broad; ax6 8/8) but **selection** remains the long-passage failure point in large corpora (w06): evidence exists deep in a passage yet the cross-encoder ranks it below the selection cutoff — a candidate for M2 longer-context rerankers.
4. Register/genre robustness (casual dialogue turns, first-person notes, structured checklist syntax) is a distinct gap from paraphrase: even verbatim-present facts (b18, b21, b23) fail when formatting departs from expository prose.

## Verdicts

- **v6-core formal M1 gate: PASS** — 21/24 + 6/6, all certification checks green.
- **v6-broad stress test: coverage FAIL (14/24), abstention PASS (6/6)** — the honest, diagnostic outcome the stress test was designed to produce: the M1 build is safe everywhere and useful in engineering genres, but not yet a general personal-assistant memory.
