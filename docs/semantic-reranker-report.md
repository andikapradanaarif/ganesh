# Local cross-encoder passage reranker — REJECTED for retention by the frozen rule; most informative result of the program

## Verdict: keep #188. The candidate fails the frozen retention rule on protected-hit preservation, despite being the first intervention to recover the selection losses and touch both formal M1 bars.

## Predeclared configuration (frozen before outcomes)

- Model: cross-encoder/ms-marco-MiniLM-L-6-v2, license Apache-2.0, HuggingFace revision main as of 2026-11-26, safetensors sha256 `821d1aa69520101d6e0737f78a042ae25b19e5cb9160701909d10434f4aeb0ae`, 90.9 MB fp32, BERT-architecture 6 layers hidden 384.
- Runtime: candle 0.8 (pure Rust), CPU, zero provider calls, zero network at runtime. Weights vendored outside git at `fixtures/reranker-model/`; download command + digest in replay instructions.
- Tokenization/pairing: `[CLS] question [SEP] passage [SEP]`; question truncated to 64 whitespace tokens, passage to first 384, sequence cap 512.
- Selection policy: every eligible candidate scored for every question including date and quantity questions; selection order from cross-encoder logits alone (bm25 ordering and #188 date promotion bypassed inside selection); greedy top-three with token-Jaccard ≥ 0.7 diversity skip (predeclared compound preservation), refill in reranker order; three-passage limit unchanged. Candidate generation, the top-50 pool, subject eligibility, acceptance, and the REST/MCP contract byte-identical to #188. Model absent → exact #188 fallback (keeps unit tests hermetic).
- Calibration check pre-pinning: relevant pair +9.39, irrelevant −9.67 — expected ms-marco logit range (initial pooler-less draft scored compressed values; corrected before any benchmark outcome was inspected).
- Freeze chronology: xfer2 packet (Statue of Liberty / Empire State Building, 11 cases) committed complete in one commit — snapshots, manifest, key, scoring-spec — verified ancestor of the candidate pin. Candidate pin `0386a9e47d49b4eb58c58d815a5ff75a8eff49a5`. The #192-era freeze defect (key lost in authoring crash, regenerated pre-outcome) is superseded by this correct chronology and corrected in the record here.

## Corrected scorecard (deliverable 1, regenerated from one full-pool trace)

`docs/v4-scorecard-corrected.json` — every row derived from the same machine-readable full-pool probe trace (the earlier artifact mixed the 10-row human preview with full-list key lines; w08 was recorded ABSENT there while the full-list join showed rank 31 — that inconsistency is now corrected):

- raw-pool ceiling 24/24, eligible-pool ceiling 24/24, selected 17/24, credited 17/24
- w08 verified at raw rank 31, eligible, unselected — regression check: it is the only keyed passage below rank 10
- all seven misses route through top-three selection with the keyed passage in-pool and eligible (w01 rank 6, w03 rank 6 — distinct facts sharing a passage — w06 rank 6, w08 rank 31, w11 rank 5, w12 rank 7, w14 rank 3); zero retrieval failures, zero eligibility-filter losses
- w01/w03 counted as distinct facts throughout. #192's miss routes: byte-identical selections to #188, hence identical routes.

## Paired results (baseline #188 `8f273aaa`, candidate `0386a9e4`)

| Fixture | #188 | candidate |
|---|---|---|
| v4 answerable coverage | 17/24 (wiki 9/16) | **20/24 (wiki 15/16)** |
| v4 abstention | 2/6 | **6/6** |
| v4 unsupported-ok | 4 leak (i01/i04/i05/i06) | **zero** |
| xfer2 end-to-end | 4/11 | **6/11** |
| xfer2 coverage | 3/8 | 4/8 |
| exposed v2 | 27/30–28/30 | 25/30 — **v06 compound control lost** |
| event probe | 12/12 | **7/12** |
| action/chain/quantity | 8/8, 13/13, 8/8 | 8/8, 13/13, 8/8 preserved |
| date-xfer-v3 | 5/12 | 4/12 |
| unsupported-ok everywhere | — | **zero newly unsupported in every suite** |

v4 movements vs #188: +w01 +w03 +w06 +w08 +w11 +w14 (six of seven misses recovered; only w12 uncredited), −s04 −s05 −s07 (synthetic log hits lost). w07/w10 preserved.

## Why retention fails as frozen

The rule requires preserving protected hits. v06 — a designated compound control — is lost, the synthetic seedlog hits s04/s05/s07 are lost, and the event probe halves (12/12 → 7/12). Zero newly unsupported ok and the target-domain gains cannot offset those losses under the rule as written. No post-outcome tuning was applied.

## The decisive diagnostic: the loss is downstream of the reranker, in response assembly

v06 is the smallest defeating case, and it does not defeat the mechanism the experiment tested: the probe trace shows the reranker selected BOTH keyed passages (`1386984050#p20` rank-1 in-selection, `#p23` rank-3 in-selection) with decision ok — yet REST/MCP returned `p20, p4, p2`. Server latency 6.3 s/query proves cross-encoder inference ran in the serving process (so no model-path fallback). The correct selections were made and then replaced by bm25-ordered passages before delivery — a post-selection assembly/backfill stage eating the reranker's choices. w12 shows the same shape: keyed passage `perseverance-a1#p1` rank-7 in-selection under the reranker, decision ok, yet uncredited at the API. Under #188 the same assembly passed because bm25-order selection and bm25-order assembly coincided. This wiring mismatch — not the semantic model — is the documented earlier stage that prevented the evidence from reaching the caller for the lost cases. For the six recovered wiki misses the decisive spans were within the 384-token truncation and correctly ranked.

The event-probe and synthetic losses share a second plausible mechanism recorded for completeness: ms-marco training distributes toward natural-language evidence sentences; telegraphic log lines score lower even when decisive. Both mechanisms are documented; neither was tuned against.

## Operations

Adapter agreement 30/30 v4, citation openings 42/42 transfer, zero provider calls everywhere, deterministic replay (same inputs → same logits, fp32 CPU), model footprint 90.9 MB + tokenizer 0.7 MB, warm query latency ~6 s/query single-threaded CPU (50 forward passes per question, no batching) versus ~50 ms baseline — the documented cost.

## What this establishes (the product-decision substance)

First, the corrected measurement: eligible pool 24/24 — selection was the whole answerable-loss story, and semantic scoring recovered 6 of 7 misses on exposed v4 and improved unfamiliar-document correctness 4/11 → 6/11 with zero newly unsupported ok anywhere. The mechanism class works; both M1 bars were touched simultaneously for the first time (20/24 coverage, 6/6 abstention on exposed v4). Second, the losses concentrate where a response-assembly stage reorders or replaces the reranker's chosen passages, plus a domain mismatch on telegraphic synthetic text. Per the frozen rule the candidate is REJECTED and #188 remains the recommended build; the assembly-stage finding and the corrected scorecard are the durable contributions. A follow-up that aligns response assembly with selection order — without touching the frozen packet or thresholds — is the smallest evidenced next step, and is the owner's decision.

Replay: `GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 ganesh-bench run --questions fixtures/xfer2/key.json --corpus fixtures/xfer2 --mode dev` (weights: download model.safetensors/config.json/tokenizer.json from cross-encoder/ms-marco-MiniLM-L-6-v2 into fixtures/reranker-model/, verify sha256 821d1aa6…).
