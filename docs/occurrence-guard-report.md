# Subject-bound occurrence guard + fail-loud reranker — verdict: REJECTED, #188 retained

Candidate pin `3aeba5c6bd0adc4fdbe5e3f08fd6977031cbbabe` (freeze ancestor-verified; acc-xfer-v4
frozen in two pre-code commits, preflight PASS 0 problems, grep-audit verified with the q03
fiction-only Apollo-18 mention manually inspected). All candidate runs with
`GANESH_REQUIRE_RERANKER=1`; every report logs `reranker: active (model fixtures/reranker-model,
sha256 821d1aa69520)`; zero provider calls; synchronized workspace builds.

## Paired results (model-verified)

| suite | #188 | 0c298c29 | candidate |
|---|---|---|---|
| acc-xfer-v4 coverage | 3/8 | 6/8 | **6/8** |
| acc-xfer-v4 abstention | 1/4 | 0/4 | **1/4** |
| v4 coverage / genuine abstention | 17/24 | 23/24 + 5/5 | **23/24 + 5/5** |
| v2 total / abstention | 28/30 | 29/30 + 6/6 | **29/30 + 6/6** |
| probes | — | 49/49 | **49/49 (ev12 ac8 ch13 qt8 rp8)** |
| exposed acc-xfer-v2 abstention | — | 5/6 | **6/6 — q08 ISS-deorbit FIXED** |

acc-xfer-v4 per-case: q01 Apollo-11 realized sibling hit, q02 Europa-Clipper realized date hit,
q04 Europa-Clipper planned Jupiter-orbit **correctly abstains** (the target class, fixed on an
unseen article), q05 irregular-verb hit, q08 framing hit, q10 entity hit, q11 compound hit;
misses q06 (decimal m/s — selection/measurement binding), q12 (JWST exact date); leaks q03
(Apollo 18 — occurrence guard inert when neither planned nor realized sentences bind: the
fiction mention is undated, coverage passes), q07 (exact-date precision — newly unsupported vs
#188), q09 (mirror-inches measurement absence).

## Frozen-rule verdict: REJECTED — #188 retained

Gates passed: v4 23/24 + 5/5 genuine typed ✓; v2 29/30 with 6/6 abstention ✓; probes 49/49 ✓;
fail-loud verified (exit 2, FATAL message, active digest logged in every report) ✓; exposed
acc-xfer-v2 q08 fixed to 6/6 without breaking q05 or any probe ✓.

**Defeating gate:** acc-xfer-v4 abstention parity — candidate 1/4 equals #188's 1/4 rather than
improving, and q07 is newly unsupported vs #188. **Smallest defeating case: q07** — exact-date
precision leak introduced on the frozen transfer packet; failure stage: acceptance-layer
exact-date verification on semi-structured establishment prose. No post-outcome tuning applied.

## v5 readiness assessment

The occurrence-guard mechanism itself transferred exactly as designed: Europa Clipper's
planned-orbit case abstains on first exposure, ISS-deorbit now abstains on exposed acc-xfer-v2
(6/6) with zero answerable regressions anywhere (49/49 probes, 29/30 v2, 23/24 v4). The residual
risk surface for v5 is now precisely bounded to two documented classes: (1) occurrence-guard
inertness when no dated sentence of either kind binds the event (q03 — coverage alone passes);
(2) exact-date precision on establishment/prose-edge cases (q07). Both are acceptance-layer,
small, and named — selection and every other verification class remain green.
