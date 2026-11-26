# Bounded evidence-sufficiency acceptance experiment — verdict: REJECTED, #188 retained

Candidate pin `26a07d774e3be697fa33385c71d4c39c4c227af8` (branch `feat/acc-suff-v2`, freeze
`1ed8542403cfec3a148fca657fcabbae9e0d21a2` ancestor-verified, preflight PASS 0 problems).
Selection stack frozen from `b08692de`; only `accept.rs` and its `search.rs` inputs changed.
All four evidence flags on; zero provider calls; full `--workspace` builds with synchronized
server/bench binaries.

## Step-1 diagnostics (recorded before design)

Cross-encoder logits do NOT cleanly separate supported from unsupported: i01 top-1 logit 9.11
exceeds answerable hits (w01 8.44); insufficient i04/i05/i06 sit at 3.1–7.2 overlapping the
answerable band. The optional logit floor was therefore omitted per the predeclared rule.
Exact mechanisms confirmed: i01 leaked because `ENV_STEMS` blanket-stripped the stem `test`
from `act_terms`, so `act_only_elsewhere` never saw the queried activity noun; i04/i06 abstained
only accidentally through `idf-union-coverage` (0.667/0.709) on framing words; v2 false abstains
v01/v03 (0.657/0.727) were pure coverage artifacts with gold passages already top-3.

## Predeclared acceptance changes

1. Measurement recognition: what-is-property / in-unit forms join how-many/much as quantity
   questions (predeclared property/unit lexicons).
2. Quantity binding requires quantifying-window evidence (8 tokens of measured property/unit),
   excludes pronominal "one" (of/instead/use/when/as… contexts) and code-literal 0/1 near
   operator punctuation; comma-grouped numerals are quantities; dotted decimals adjacent to the
   measured property are measurements, not versions.
3. Exact-date precision: exact-date/day triggers require a tier-3 adjacent month-day date tied
   to queried event terms in the sentence — or the dated-log heading — of a selected passage.
4. Environment stems count as environments only when prepositional in the query ("in
   production", "to staging") so activity nouns ("germination test") survive attribution.
5. Small linguistic framing list (kind/type/exact/calendar/day/date/country/information) no
   longer penalizes coverage, typed verification carrying sufficiency instead.

22 unit tests pass, covering all five spec-required behaviors plus parity and regression tests.

## Results (freshly built --workspace binaries, all arms)

| suite | #188 | b08692de | candidate |
|---|---|---|---|
| v4 coverage | 17/24 | 23/24 | **23/24** (w12 still selection-limited) |
| v4 abstention | 2/6 | 4/6 | **5/6** — i05 leaks |
| v2 total correct | 28/30 | 26/30 | **28/30** — v04/v05/v06 hit, **v30 abstains**, v03/v13/v23 recovered, v01 still false-abstains, v28 leaks (as in prior arms) |
| acc-xfer-v2 coverage | 5/10 | 5/10 | **6/10** |
| acc-xfer-v2 abstention | 5/6 | 5/6 | **5/6** — q08 leaks in ALL arms |
| event-probe | 12/12 | 12/12 | **12/12** |
| action-probe | 8/8 | 8/8 | **8/8** |
| chain-probe | 13/13 | 13/13 | **13/13** |
| rail-probe | 8/8 | 8/8 | **8/8** |
| contrast-quantity | 8/8 | 8/8 | **7/8 — qA06 regressed** |
| date-xfer-v3 / sem-xfer-v2 | — | — | diagnostic, not gating |

v4 per-case: i01 now abstains through activity-clause attribution (typed), i04/i06 through
exact-date-precision verification (typed — no longer accidental coverage), i02/i03 unchanged,
i05 still leaks. acc-xfer-v2 misses: q05 false-abstains (Porter irregular-verb mismatch:
query "sink" stems to `sink`, evidence "sank" stems to `sank`, so the exact-date event tie
never forms), q06/q10/q14 evidence/selection misses, q16 false-premise abstains correctly;
q08 (never-occurred deorbit year) leaks in all three arms — no typed mechanism addresses
event-non-occurrence.

Operations: adapter agreement 30/30, citations 150/150, deterministic replay, 0 provider
calls, warm latency mean ≈2.6 s (v4) / ≈3.7 s (acc-xfer-v2), max ≈5 s, fp32 CPU, 91 MB weights.

## Frozen-rule verdict: REJECTED — #188 remains the recommended build

Three independent gate failures:
1. **i05 unsupported-ok on v4** violates the zero-unsupported-ok requirement (smallest
   defeating case). Diameter-in-meters recognition fires, but the quantifying window still
   admits incidentally-nearby numerics in mirror-discussion prose — 8 tokens is too wide for
   dense measurement text, and the rule as frozen cannot be tuned post-hoc.
2. acc-xfer-v2 abstention 5/6 does not improve over #188's 5/6 (coverage did improve 5/10→6/10).
3. contrast-quantity regression qA06 — the tightened window/binding excludes a legitimate
   quantifying shape the probe keys.

Exact failure stage for the smallest defeating case: acceptance-layer quantity binding —
windowed-proximity evidence is necessary but not sufficient for measurement verification in
prose dense with other quantities.

Honest residue for v5: q08-class event-non-occurrence and u14-class false-premise inversion
remain unreachable by every typed mechanism built so far; q05 shows exact-date verification
additionally needs morphological tolerance for irregular verbs. Selection remains solved;
acceptance is improved but not solved.
