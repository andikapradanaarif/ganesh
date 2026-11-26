# Slot-2/3 body-marginal selection experiment — negative result

## Decision: keep #178 as the recommended behavior.

## Ceiling measurement (step 2, pre-implementation)

First-pick-constrained ceiling on exposed v4 wiki questions: **15/16 — identical
to the full-pool ceiling**. Locking #178's actual first pick costs nothing in
principle: for every wiki question coverable from the pool, the remaining two
slots suffice. w01: first pick `curiosity-a1#p19` (mass-at-launch, no date);
the missing fact (launch date) is supplied by `curiosity-a1#p2`, in-pool at
rank 6, available for slots 2-3. Overall constrained ceiling 23/24 >= 20/24, so
the design was authorized to proceed.

## Formulation recorded before running

The existing marginal walk credits coverage measured on the stemmed index text
(title + section + body). Because the article title is injected into every
passage, subject terms tie within an article. The candidate keeps pool, order,
first pick (best bm25), top-3 limit, and acceptance logic exactly as in #178,
and changes only slots two and three: marginal gain counts query terms present
in the passage **body** that are not yet body-covered by already-selected
passages. Ties break by original bm25 rank (deterministic). #182's global
body-coverage reordering is preserved as a comparison arm (separate binary).

## Paired results (#178 -> NEW; #182 as comparison arm)

| Fixture | #178 | #182 | NEW |
|---|---|---|---|
| v4 coverage | 15/24 (wiki 7/16) | 18/24 (wiki 10/16) | **14/24 (wiki 7/16)** |
| v4 unsupported-ok | i01,i04,i05,i06 | same | same — none new |
| sel-xfer transfer (frozen with candidate pin) | 4/10, cov 2/7 | 6/10, cov 4/7 | 4/10, cov 2/7 |
| exposed v2 | 27/30 | 24/30 (v30 leak) | **27/30, v30 correctly insufficient** |
| v04/v05/v06 compound controls | hit | lost | **all hit — preserved** |
| event/action/chain | pass | pass | pass (12/12, 8/8, 13/13) |
| rank-xfer (exposed) | 9/11 | 10/11 | 10/11 (i22 pre-existing in all arms) |

## Why it failed — the smallest explaining case

w01. The first pick is junk (`curiosity-a1#p19`, mass-at-launch). The correct
passage p2's unique body contribution over the other candidates is essentially
the bare term `rover` — but nearly every passage in the Curiosity article
mentions "the rover" in its body, so **body-term coverage ties within a single
article just as title-injected coverage did**. The marginal signal is exhausted:
topically central vocabulary is ubiquitous, and what actually distinguishes the
fact-bearing passage (the presence of the date, not the presence of the topic
words) is invisible to term coverage. Consequently slots 2-3 fill by bm25 rank
exactly as before — v4 coverage even dropped 15/24 -> 14/24 on tie-break noise,
and transfer coverage matched #178 exactly (2/7). The first pick was verified
identical to #178; pool identical; no new unsupported-ok anywhere; v30
correctly stays insufficient_context.

**What limited the result: the marginal signal itself** — not the fixed first
pick (ceiling 15/16 with it locked) and not downstream acceptance (selections
did not change). #182's positive v4 movement came from moving the first pick,
which is precisely what this design froze — and #182 paid for it with the v30
leak and the compound-question regressions this iteration avoided. The tradeoff
is structural: term-coverage signals cannot separate fact-bearing passages from
topic-bearing passages inside one article. The evidence now points at
answer-typed selection (a when-question needs a passage containing a date
expression near the queried activity) as the only mechanism class the traces
support — a larger change, not authorized here.

## Costs/compliance

Zero provider calls. REST/MCP agreement and deterministic replay verified on
every run in this iteration's matrix. w08 remains documented as the sole
candidate-pool miss (separate retrieval issue). v5 unconsumed.
