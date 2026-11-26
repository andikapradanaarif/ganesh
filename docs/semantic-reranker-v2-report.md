# Cross-encoder v2 (title+section context, hybrid coverage slot) — REJECTED by frozen rule; closest miss in program history

## Verdict: keep #188. The candidate fails the frozen retention rule on exactly one newly unsupported ok (v30) and a net exposed-v2 regression — despite posting the strongest numbers ever recorded on the target metrics.

## Step-1 correction (delivered before implementation)

Posted on PR #193. Confirmed by binary timestamps: the #193 REST/MCP numbers came from a stale pooler-less `ganesh-server` (22:25) vs rebuilt probe (22:28). ganesh-mcp is a thin ureq relay with no ganesh-core dependency — only server+bench freshness matter. The retracted "response-assembly bug" does not exist; `search()` is structurally `search_debug().0` (now pinned by a compile-time parity test). TRUE 0386a9e4 paired counts: v4 27/30 total, coverage 21/24 with **wiki 16/16** (all seven misses recovered), abstention 6/6; v2 26/30 with v04/v05/v06/v19 all hit and false abstains v01/v03/v13/v23; event-probe 7/12 with false abstains v03/v04/v06/v11/v12. The incomplete 7a2391f freeze acknowledged and superseded by the sem-xfer-v2 freeze `c621177b` — verified complete in one commit, ancestor of the candidate pin `b08692dea1a9cb9f4ff000bf6acfd0d1a136b221`.

## Candidate (pin b08692de, frozen before sem-xfer-v2 outcomes)

Same predeclared model/runtime as #193 (ms-marco-MiniLM-L-6-v2, Apache-2.0, sha256 821d1aa6…, candle CPU fp32, zero provider calls). Two reviewed fixes only: (1) pair text `"{title} — {section}\n{text}"`; (2) hybrid final slot picking the highest-scored candidate adding an uncovered query term — justified pre-outcome by true-0386a9e4's coverage-drop false abstains v01/v03/v13/v23. Plus the parity guard and workspace-build discipline.

## Paired results (freshly built workspace binaries, verified synchronized timestamps)

| Suite | #188 | candidate | |
|---|---|---|---|
| v4 coverage | 17/24 (wiki 9/16) | **23/24 (wiki 15/16, synthetic 8/8)** | bar is 20/24 |
| v4 abstention | 2/6 | **4/6** | i04/i06 fixed; i01/i05 leak in both arms |
| v4 total | 19/30 | **27/30** | |
| sem-xfer-v2 total | 7/14 | **8/14** | |
| sem-xfer-v2 coverage | 5/11 | 6/11 | |
| event-probe | 12/12 | **12/12** | true-0386a9e4 was 7/12 — heading fix confirmed |
| action/chain/quantity | 8/8, 13/13, 8/8 | 8/8, 13/13, 8/8 | preserved |
| exposed v2 | 28/30 | **26/30** | net regression |
| date-xfer-v3 | 5/12 | 6/12 | |

Protected hits: w07 ✓ w10 ✓ v04/v05/v06 ✓ s01–s08 **all hit** ✓ — every protected hit preserved. v4 movements vs #188: +w01 +w03 +w06 +w08 +w11 +w14, zero answerable regressions anywhere on v4.

## The defeating case and its exact stage

**v30** ("How many operators does the C language define?") — correctly insufficient under #188, unsupported-ok under the candidate. This is the only newly unsupported ok anywhere across all eight suites (u14/i01/i05 leak identically in both arms and are not new). Failure stage: **acceptance's coverage-sufficiency conflation, not the semantic selection**. The cross-encoder correctly surfaces operator-discussion passages as topically relevant — they ARE the best-matching passages; the defect is that idf-union-coverage acceptance treats query-term coverage as evidentiary sufficiency. The hybrid coverage slot intensifies this for insufficient questions (it deliberately backfills term coverage) even while it rescues false abstains. The same conflation cuts both ways, proven within one evaluation: on i04/i06 the improved selection now abstains correctly because misleading passages lost coverage, while on v30 improved selection breaks a correct abstention because misleading passages gained it. v2 false abstains v01/v03/v23 persist in the same mechanism's other direction.

## Abstention honesty audit

v4 insufficient rows: i01 leak (both arms — activity outcome), i02 correct, i03 correct, **i04 correct (leaked under #188)**, i05 leak (both arms — measurement absent), **i06 correct (leaked under #188)**. Transfer: u12/u13 correct abstentions, u14 leaks in BOTH arms (wrong-activity polarity is an acceptance gap no selection policy can reach). Abstentions here are coverage-driven; none yet constitutes genuine factual-sufficiency verification of event/scope/precision identity. Ordinary factual abstention still needs its own bounded fix before v5 — the candidate's 4/6 is progress but accidental in mechanism.

## Operations

Pool generation untouched → raw/eligible pools identical by construction; REST/MCP adapter agreement 30/30 on v4; citation openings all resolved; deterministic fp32 CPU replay; model footprint 90.9 MB + 0.7 MB tokenizer; zero provider calls; warm query latency mean 2744 ms / max 5326 ms (~50 forward passes per query, unbatched) vs ~50 ms baseline — a 55× cost, the documented price of the mechanism.

## Product-decision close

1. PR #193 stale-server numbers corrected: wiki was never 15/16 — true 0386a9e4 already hit **16/16**; compound v06 was never broken.
2. title+section context resolved s04/s05/s07 completely and event-probe returned to **12/12** — the review's mechanism diagnosis confirmed exactly.
3. The candidate does NOT qualify under the frozen rule: one newly unsupported ok (v30) + v2 net regression (28→26). #188 remains the recommended build.
4. What the evidence now supports for the owner's next decision: selection is solved in principle on every measured dimension (coverage 23/24, transfer improved, all protected hits intact); the remaining blocker is ONE mechanism — acceptance conflating term coverage with sufficiency in both directions. A bounded acceptance fix (verify requested-value presence rather than term coverage) is the single evidenced step before v5; without it, every further selection improvement will keep trading v30-style leaks against false-abstain recoveries.

Replay: `GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 cargo build --release --workspace && ganesh-bench run --questions fixtures/sem-xfer-v2/key.json --corpus fixtures/sem-xfer-v2 --mode dev` (weights: cross-encoder/ms-marco-MiniLM-L-6-v2 safetensors+config+tokenizer into fixtures/reranker-model/, sha256 821d1aa6…).
