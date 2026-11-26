# Bug-fixed graded date selection — RETAINED for further M1 development

## Four lead answers

1. **w01 not yet recovered, but v4 public coverage moved 7/16 -> 9/16** (total
   coverage 15/24 -> 17/24; total score 17/30 -> 19/30). w02 and w05 recovered,
   zero hits traded — w07, w10, s07 all preserved. w01 remains blocked: the
   correct passage p2 at pool rank 6 still loses the promoted slot to junk whose
   launch-plus-year sentence scores within tie range.
2. **BM25 non-degenerate on date-xfer-v3** (6 articles, realistic IDF) and the
   selector improved both measures over #178: selected coverage 2/11 -> 4/11,
   end-to-end 3/12 -> 5/12, with correct exact-date disambiguation on v04/v08
   (Titov/Vostok 2 vs the backup-Titov trap inside the Vostok 1 article).
3. **Cost: nothing.** v2 improved 27/30 -> 28/30 (v19 hit preserved,
   v04/v05/v06 intact), v30 correctly insufficient, event 12/12, action 8/8,
   chain 13/13, quantity 8/8, zero new unsupported-ok on any fixture, adapter
   agreement 30/30, candidate pools identical by construction, 11 unit tests
   pass, deterministic replay, zero provider calls.
4. **Remaining gaps before v5**: (a) selection — w01/w03/w06/w08/w11/w12/w14
   still miss; the single promoted slot cannot overcome close-scoring junk, and
   some correct passages score zero because event terms are sparse in the exact
   date sentence; (b) acceptance — the 2/6 abstention gap is untouched: u09/z08/
   i01/i04/i05/i06 leak class persists, and v02/v09/v10 on date-xfer-v3 are
   false abstentions where selected evidence was insufficiently verified; even
   perfect selection cannot pass M1 until abstention clears its bar.

## Pre-implementation failure traces (#186, confirmed)

- w01 promoted perseverance-a1#p1 over curiosity-a1#p2: the `max_by` tie-break
  returned the LAST maximal element in iteration order, i.e. the worst-bm25
  candidate among ties; combined with unweighted subject matching (`rover` in
  the query tied both rovers' articles), a Perseverance passage beat the
  Curiosity lead.
- w02 promoted curiosity-a1#p49 over p2, w06 voyager-a1#p65 over p1, w08
  hubble-a1#p170 over p1: same inversion plus stray-digit false tier-3 dates
  (a bare number near a month name counted as a full date).
- u02 promoted sp1-a1#p46 over sp2-a1#p1 and u04 sp2-a1#p6 over sp1-a1#p1:
  unweighted subject count treated 'sputnik' equally across sibling articles.
- Correction to #186's report: BM25 collapsed to -0.00 only on u01 on
  date-xfer-v2, not u02-u10; those misses were the tie-break inversion and
  unweighted subject matching, not IDF degeneracy.

## The three fixes (exactly as predeclared)

1. Tie-break: scan eligible in bm25 order, strictly-higher score replaces best —
   equal scores keep the better-ranked candidate (unit-tested).
2. date_tier: adjacency-required month+day (`Month DD`/`DD Month`, ordinals);
   tier 3 = full date with year, 2.5 = month+day without year, 2 = month+year,
   1 = bare year. 'Voyager 2 operated in April' scores 0 (unit-tested).
3. Subject binding: IDF-weighted sentence-level token credit — distinctive
   curiosity/hubble outweigh generic rover/telescope (unit-tested);
   cross-article explicit mentions preserved (Voyager 2 answered inside the
   Voyager 1 article — w07 still hits).

## Paired results

| Fixture | #178 | #186 | candidate `8f273aaa` |
|---|---|---|---|
| v4 coverage | 15/24 (wiki 7/16) | 16/24 (wiki 8/16) | **17/24 (wiki 9/16)** |
| date-xfer-v3 coverage | 2/11 | 4/11 | 4/11 |
| date-xfer-v3 end-to-end | 3/12 | 5/12 | 5/12 |
| exposed v2 | 27/30 | 28/30 | 28/30 |
| probes (event/action/chain/quantity) | pass | pass | pass |
| new unsupported-ok | — | — | **zero** |

date-xfer-v3 per-case (candidate): hits v03/v04/v05/v08; v02/v09/v10 false
abstains (acceptance-layer, recorded); v01 ok-but-miss (selection, recorded);
v12 correct abstention; v11 fallback correct (non-date query — verified #178
identical path).

## Follow-up opened

Ordinary factual abstention — the sentence-level (subject, event,
date-precision) primitive built here is directly reusable in acceptance: a
question's required precision is already computed at selection time; acceptance
can require the same tier of evidence rather than topic-word coverage alone.

Freeze `9c4c7fc9`; candidate pin `8f273aaa`; baseline #178 `6b06393c`.
Replay: `GANESH_PROPOSITION_CHECK=1 GANESH_QUANTITY_BINDING=1 GANESH_CONTEXT_CHAIN=1 GANESH_ACTION_CHECK=1 ganesh-bench run --questions fixtures/date-xfer-v3/key.json --corpus fixtures/date-xfer-v3 --mode dev`
