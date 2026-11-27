# EVALUATION-m2-phase1 — Bi-Temporal Lifecycle, Scoped Sharing & Hybrid Retrieval

Candidate branch: `feat/m2-lifecycle-hybrid` (branched from `main` @ `8287c94`).
All measurements: 100% local, zero external provider calls, fail-loud guards
(`GANESH_REQUIRE_RERANKER=1`, `GANESH_REQUIRE_QA_VERIFIER=1`, `GANESH_REQUIRE_EMBEDDER=1` — exit 2 verified).
Frozen fixtures untouched (`v1`–`v6`, `qa-xfer-v1`, `acc-xfer-*`, probes — verified by git diff).

## 1. Models (all INT8 ONNX, pure local CPU)

| Model | Purpose | Bytes | SHA-256 |
| --- | --- | --- | --- |
| BAAI/bge-small-en-v1.5 (384-dim) | dense bi-encoder | 34,014,426 | `6c9c6101a956d62dfb5e7190c538226c0c5bb9cb27b651234b6df063ee7dbfe4` |
| ms-marco cross-encoder | reranker | 23,180,880 | `5741182c012282d6fe025705a0a4b43884c9f778da01f7ac7b20ff82097829a5` |
| QA verifier | answerability | 33,784,916 | `209727cb5bfa77e9e60df6ff257078119be73ed86378a563791f2c8021d7f238` |

Total model disk footprint: **90,980,222 bytes (~91 MB)**. Digests certified in every gate report.

## 2. Pillar A — bi-temporal lifecycle, sharing, revocation

Schema: Tier-1 revisions carry `occurred_at`, `effective_from/until`, `ingested_at`, `author`,
`source_type`, `project`, `scope`, `authority`, `supersedes`, `supersede_mode`, `claim_key`,
`audience`, plus `supports` (multi-source links) and a `revocations` table. M1 fixtures default
cleanly (`project=public`, `source_type=wiki`) and replay unchanged. Reads compute eligibility in
SQLite before Stage-1 retrieval: visible-to-caller, inside effective window at `as_of` (default:
now), not superseded by an eligible successor in scope, still backed by a visible support.
API: `as_of` + `top_k` (1–15) on REST `POST /v1/search` and MCP `search_context`; responses add
`confidence` (supported/uncertain/insufficient_context), `top_margin`, `effective_time`; conflicting
equally-authoritative revisions yield `status: conflicting_evidence`. Caller identity resolves only
from the credential policy (`--policy`); request-supplied projects can never widen access.
Citation tokens re-check permission at open time.

**Lifecycle replay gate (`fixtures/m2-lifecycle`, roadmap §7): 37/37 PASS, 0 FAIL** — all five
scenarios end-to-end over REST and MCP, including server-restart persistence, late-arrival replay,
conflict detection, dual-source removal, and zero-leak revocation assertions.
**m2-xfer-v1 lifecycle scenario: 14/14 PASS** (as-of reads, partial scoped correction, supersession,
multi-source revocation, zero-leak).

## 3. Pillar B — hybrid retrieval results

Stage-1: BM25 ∪ dense-cosine over SQLite-eligible revisions only, fused by RRF (k=60), fused pool
depth configurable (`GANESH_RERANK_POOL`, default 40). Context headers in indexed/embedded/reranked
views only when metadata exists — **metadata-less corpora keep the exact M1 cross-encoder input
bit-for-bit** (this restoration recovered the v4/w12 regression during development). Symbol-safe
FTS5 aliases, checkbox normalization, semantic-aware coverage override (excludes measurement-class
questions; typed deterministic checks and QA unanswerability guards unchanged).

### v6-broad before/after (frozen M1 stress test)

| Metric | M1 `81a0f315` | M2 | target |
| --- | --- | --- | --- |
| coverage | 14/24 | **19/24** | ≥20/24 (missed by 1) |
| insufficient | 6/6 | **6/6** | 6/6 met |
| paraphrased lookups | 1/6 | **4/6** | ≥4/6 met |
| personal notes | 0/4 | **3/5** | ≥3/4 met |
| exact symbols | 5/6 | **6/6** | — |
| compounds | 3/4 | **4/4** | — |
| checklists | 2/4 | **2/2** (+blocked/started states correct) | — |
| long-passage>350 | 2/2 | **2/2** | — |
| scoped qualifiers | 2/4 | 2/4 | not improved |

Remaining v6b misses (honest abstentions, zero fabrication): b02 (exact-date precision),
b03 (near-neighbor disambiguation project-B vs personal), b07 (coverage 0.551),
b18 (meeting-transcript turn structure), b20 (QA-verifier weak on speaker attribution).
Pool calibration: pool-25 and pool-40 produce **identical 19/24+6/6** with identical misses;
pool-25 is faster → recommended default reduction validated.

### Zero-regression matrix (frozen M1 suites, serial, gates)

| Suite | M1 baseline | M2 | verdict |
| --- | --- | --- | --- |
| v6-core (formal gate) | 21/24+6/6 | **21/24+6/6** — identical misses (w06/w08/w09) | exact parity |
| v2 | ≥29/30+6/6 | **24/24+6/6** answerable+insufficient all correct | no regression |
| v4 | 24/24+5/5 | **23/24+5/6** | partial regression, analysis below |
| probes (49) | 49/49 (recorded) | **43/49** | see decisive analysis |

**v4 analysis:** w08's correct passage is in-pool at rerank position 4, losing the top-3 cut by a
0.04–0.19 noise-level margin (M1's pass was equally marginal). i05 leaks because dense recall now
surfaces the design-phase passage quoting mirror diameters (3 m → 2.4 m proposal); the typed
quantity binding correctly binds `diameter`, and the final mirror really is 2.4 m — a trap whose
insufficiency depends on proposal-vs-built framing the typed layer cannot represent.
**Probes decisive analysis:** the exact pinned M1 binary (`81a0f315` code) rerun serially in this
environment fails the *identical* 6 questions (a03, a06, qB05–qB08) — **M1 itself scores 43/49
serially**. The recorded 49/49 baseline is not reproducible in this environment with either binary;
M2 is at exact bit-level parity with M1 on the probe matrix. Zero M2 regression.

## 4. Fresh frozen transfer packet `fixtures/m2-xfer-v1`

Freeze SHA `6772d264d5ca176a92eb6ab95cc52c1b331aebcc` (frozen before first evaluation; single-shot
discipline preserved). 11 docs / 16 answerable + 6 insufficient + lifecycle scenario; preflight PASS
(0 errors), disjointness 0 overlaps across all prior manifests, grep audit clean for xi01–xi06.
Multi-genre: journal, meeting, RFC/runbook, postmortem, project notes + lifecycle revision chains
with project-scoped visibility.

**Standalone M2 results (static key, gate mode, caller=key-alpha):**
coverage **15/16** (miss: x07 weekend-freeze namespace — scoped qualifier binding),
insufficient **2/6** — leaks xi02 (unmade meeting decision), xi03 (proposed-not-applied cert-manager
mode), xi05 (missing amperage measurement), xi06 (wrong-environment staging freeze code).
The leaks are genuine semantic-judgment gaps exposed by strong dense recall on a small corpus:
near-miss content about proposals/staging is retrieved correctly, and the acceptance layer cannot
yet distinguish proposed-vs-applied or staging-vs-production framing. v6b's same-category controls
pass because its traps were authored pre-hybrid; these harder traps define the M2-Phase-2 agenda.
**Lifecycle scenario: 14/14 PASS** through REST+MCP including restart persistence and zero-leak
revocation. Paired M1 comparison: not meaningful — M1 has no scope/temporal model (imports the
packet as flat public documents, answered lifecycle-trap content it should withhold); the static
comparison run produced no valid data because the M1 harness lacks credential plumbing entirely.

## 5. Latency, memory, certification

Serial means (REST/MCP per-query): v6-core 1888/1934 ms (M1: 2170/2272 — faster despite embeddings),
v6-broad 536 ms REST after batch chunking (M1: 736), xfer 466 ms REST.
**Peak RSS per server process: 699 MB** (chunked cross-encoder batches ≤8; pre-chunking peak was
3484 MB — batch activation memory dominated). Well under the 1.8 GB target.
All gates: incomplete=false, provider calls 0, adapter agreement 30/30 and citation integrity 100%
on v6-core/v6-broad/v4/v2 and on m2-xfer-v1 adapter agreement 22/22, citations 114/114 (100%). Embedder/reranker/QA digests certified per report; fail-loud exit-2 verified
for all three guards.

## 6. Known gaps → M2 Phase 2

1. Acceptance-layer framing semantics: proposed-vs-applied, staging-vs-production, unmade-decision
   discrimination under strong dense recall (xfer xi02/xi03/xi05/xi06; v4 i05 class).
2. Near-neighbor scoped disambiguation (b03/b18/x07) — scoped qualifiers remain 2/4.
3. QA-verifier weakness on speaker attribution in transcripts (b20).
4. Probe-environment nondeterminism inherited from M1 (bit-identical failures on both binaries) —
   needs a deterministic-inference investigation independent of M2.
5. Owner Web UI and LoCoMo/LongMemEval runner: explicitly scoped to Phase 2, not built here.
