# Precision-aware factual acceptance — negative result, #188 retained

## Four lead answers

1. **Which unsupported answers did Ganesh stop serving? None.** i04/i05/i06 leak
   identically (abstention unchanged at 2/6); on acc-xfer the candidate leaks
   MORE, not less — q09/q10 newly unsupported alongside pre-existing q08.
2. **Which supported answers did it recover or lose?** It recovered nothing on
   v4 and lost controls everywhere else: exposed v2 collapsed 28/30 -> 24/30
   with four unsupported-ok including **v30** — the exact control every prior
   iteration protected; quantity controls dropped 8/8 -> 6/8 (qA05/qA06 leak);
   date-xfer-v3 control v12 newly leaks.
3. **Did the distinction transfer? No — it inverted.** On the frozen transfer
   packet the verification gate passes wrong-scope evidence: q09 asks a
   bridge-wide strand total (absent) but 'each cable has 27,572 strands'
   satisfies numeric+property co-occurrence — precisely the wrong-property trap
   the contract named, and the naive gate cannot see it.
4. **What still blocks M1:** factual acceptance, but harder than estimated —
   sentence-level co-occurrence verification is strictly weaker than the
   existing tuned coverage+binding logic. Selection backlog stands at wiki
   9/16 with the ranking backlog documented in #188; i01 remains the
   activity-interpretation control.

## Why it failed — smallest counterexamples

- q09: 'How many strands across ALL cables' — verification finds 'Each cable is
  made of 27,572 strands of wire' (numeric + property 'strand') and delivers.
  The scope mismatch (per-cable vs bridge-wide) is invisible to co-occurrence.
- v30: 'How many operators does C define' — numeric-dense passages about C
  exist; co-occurrence passes where the carefully tuned binding logic abstained.
  Replacing the coverage decision instead of augmenting it destroyed correct
  behavior that took four iterations to build.
- i04: exact-precision gating never fired because wrong-event tier-3 dates in
  the selected passages score positive — the verification answers 'is there any
  tied date' rather than 'is the requested milestone's exact date present',
  which is the actual question.

## Verdict: negative. #188 remains the recommended behavior in full.

The correct next mechanism, if this line continues: verification must be
additive scrutiny on top of the coverage decision (withhold-only, never
deliver-override), and scope/property identity must be verified rather than
term co-occurrence — a substantially harder problem than this iteration budgeted.

Freeze `34cc2d62`; candidate pin `50b87cbd`; baseline #188 `8f273aaa` /
report head `1c6484ac`. Retrieval identical across arms by construction.
Zero provider calls.
