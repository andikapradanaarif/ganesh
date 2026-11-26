# Surgical acceptance fixes experiment — verdict: REJECTED, #188 retained

Candidate pin `0c298c294e6aae63f1892d6a1ad42de7aaf482f5` on branch `feat/acc-suff-v3` from
`26a07d77`. Freeze chain `baaee7e2` + two pre-evaluation format corrections (scoring.v2 schema,
`family` field) — all fixture-only, corrected before any acc-xfer-v3 outcome was readable.
25 unit tests pass; full `--workspace` synchronized builds; zero provider calls; model digest
`821d1aa6…` verified at runtime in every candidate run.

**Reproducibility correction:** `fixtures/reranker-model/` is untracked (91 MB) and does not
carry into fresh worktrees; the reranker then silently falls back to BM25 ordering. An early
accx3 matrix ran in that fallback and was discarded. All numbers below are model-verified runs.
This silent-fallback hazard is the same class as the PR #193 stale binary and is recorded as a
process lesson.

## Three surgical fixes (predeclared)

1. `version_override` strictness: a digit.digit token is un-excluded only when pure decimal
   immediately followed (1–2 surface tokens) by an explicit measurement unit, not preceded by
   version/release/in/v or a capitalized software name, and not a question-mentioned version.
2. Irregular-verb canonicalization (35 general English groups) in exact-date event binding.
3. Past-tense occurrence guard: planned/future-modal dated sentences never settle did-shaped
   year/date questions; typed `date-occurrence` abstention when only planned mentions exist.

## Results (all model-verified)

| suite | #188 | 26a07d77 | candidate |
|---|---|---|---|
| v4 coverage | 17/24 | 23/24 | **23/24** |
| v4 genuine abstention (i01,i02,i03,i04,i06) | — | 5/5 | **5/5 typed** |
| v4 raw key abstention | 2/6 | 5/6 | **5/6** — i05 "leak" is the documented erratum: hubble-a1#p15 "2.4 m" makes i05 answerable; candidate correctly answers it |
| v2 total | 28/30 | 28/30 | **29/30** — 6/6 abstention, v28+v30 both abstain, only v01 false-abstains |
| acc-xfer-v3 coverage | 5/9 | 8/9 | **8/9** |
| acc-xfer-v3 abstention | 6/7 | 6/7 | **6/7** — q08 leaks in ALL three arms |
| probes | — | — | **event 12/12, action 8/8, chain 13/13, quantity 8/8 (qA06 restored), rail 8/8** |
| exposed acc-xfer-v2 | — | — | hit 7 correct 5; q05 now PASSES (irregular-verb fix confirmed on transfer), q08 still leaks |

acc-xfer-v3 misses: q11 compound abstains (exact-date tie or selection on the two-facet query);
q08 leak mechanism: the occurrence guard scans selected passages for any realized dated sentence
sharing event terms — Artemis I's realized flight and Apollo-era landing prose share
land/astronaut/Moon with the query, so `realized=true` even though no realized Artemis III
landing exists. Event identity (which subject the dated event belongs to) is deeper than the
surgical guard.

Operations: adapter agreement verified, citations clean, deterministic replay, 0 provider calls,
warm latency mean ≈2.7 s (v4) / ≈4.7 s (ax3), max ≈6.9 s unbatched CPU.

## Frozen-rule verdict: REJECTED — #188 retained

Gates passed: v4 ≥20/24 with 5/5 genuine typed abstention ✓; v2 29/30 with 6/6 abstention
(v28+v30) ✓; all five probes 100% with qA06 restored ✓; zero newly unsupported ok anywhere ✓.

**Single defeating gate:** acc-xfer-v3 abstention must improve over #188 — candidate matches
#188's 6/7 exactly (q08 leaks in both) rather than improving, and #188's 6/7 is not ceiling, so
the parity exception does not apply. **Smallest defeating case: q08** — failure stage:
acceptance-layer event identity. The planned-future guard works mechanically (verified in unit
tests and on the q08-free cases) but cannot distinguish the queried subject's planned event from
other realized events sharing generic event terms in the same article.

## v5 readiness

Three of the four frozen gates now pass with margin, and every fix transferred: versions never
bind as counts (v28/qA06 class solved with probe restoration), irregular-verb exact-date matching
works on unseen articles (q05), framing false-abstains stay fixed (v03/v13/v23, 29/30). The one
remaining abstention failure class is **event-identity occurrence verification** — separating
"subject X's event happened" from "a similar event happened to someone" — which also covers the
q16/u14 false-premise family. That is the single named blocker a v5 acceptance iteration must
solve; everything else measured is green.
