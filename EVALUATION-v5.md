# Formal M1 held-out gate v5 — single-shot evaluation

- Candidate pin: `6c593a1d` (exact retained candidate; `git diff 6c593a1d..HEAD -- crates/` empty)
- Freeze commit: `81c311e4252b83d17301e8221057d83c4ac5c69a` (fixtures/corpus-v5, preflight PASS 0 problems)
- Corpus: 4 volcano-domain Wikipedia articles (Krakatoa, Mauna Loa, Mount St. Helens, Mount Pinatubo) + 1 synthetic CC0 lighthouse service log — disjointness audit 0 overlaps vs all prior manifests
- Anti-erratum grep audit: all 6 insufficient questions verified genuinely unanswerable pre-freeze (i01 only year 1611 exists, no calendar date; i02 depth value absent; i03 no eruption prediction; i04 first attempt explicitly "turned away"; i05 zero repaint mentions; i06 diaphragm replacement only "began", never completed)

## FORMAL VERDICT: FAIL

| check | result | required | pass |
|---|---|---|---|
| evidence coverage | 22/24 | ≥20/24 | PASS |
| insufficient evidence | 3/6 | 6/6 | **FAIL** |
| adapter agreement | 30/30 | 30/30 | PASS |
| citation integrity | 156/156 | 1.0 | PASS |
| provider calls | 0 | 0 | PASS |

Certification: mode held-out gate, incomplete=false, reranker active (sha256 821d1aa69520), qa-verifier active (sha256 240478c7251d), Provider calls 0.

## Answerable misses (2)
- w09 (St. Helens exact 1980 eruption date): status ok but no evidence credit — selection returned sthelens-a1#p2/p17/p30, keyed acceptable was p28. Retrieval/selection variance, not acceptance.
- w13 (Pinatubo climactic eruption exact date): false abstention — zero passages selected (rest_ids empty). Retrieval-side recall failure, not the QA verifier.

## Insufficient leaks (3)
- i01 (exact calendar date of the 1611 Krakatoa map): leaked via krakatoa-a1#p8/p17/p22 — the year 1611 is present and answer-shaped; only the exact day is absent. SQuAD2 extracts positive-margin year spans; exact-date-precision remains a residual class.
- i02 (caldera floor depth in meters): leaked via maunaloa-a1#p14/p16/p33 — answer-shaped measurement content nearby.
- i04 (year the first Vancouver Expedition summit attempt succeeded): leaked via maunaloa-a1#p47/p48/p52 — the narrative carries 1793/1794 dates for attempts that failed; inverted-outcome separation beyond extractive verification.

Correct abstentions: i03 (future eruption year), i05 (synthetic absent repaint action), i06 (synthetic incomplete diaphragm action).

## Costs
- Wall time 6m02s for 30 questions (REST+MCP passes); REST mean/max 5934/11137 ms, MCP mean/max 5985/10581 ms per query (QA verification dominates latency; runs once per adapter pass).

## Honest conclusion
The retained candidate fails the formal v5 gate on the abstention axis (3/6, needed 6/6). The leaks are concentrated in wiki-prose semantic-precision classes (exact-date precision, measurement absence near answer-shaped numbers, inverted outcomes) — consistent with the documented residuals of extractive span-vs-null verification. No tuning or retries performed per protocol.
