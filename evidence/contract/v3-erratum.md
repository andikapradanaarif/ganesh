# v3 evaluation erratum (descriptive corrections to PR #172)

The published 30-question denominator and FAIL verdict stand. These corrections are descriptive; paired outcome counts are unchanged.

1. **Ruby overlaps v1.** v1's corpus includes "Ruby (programming language)" (en.wikipedia). v3 questions w06, w07, w08, w18 are therefore not fresh-family evidence, despite different snapshot bytes. Fresh families in v3: Perl, Lua, Kotlin only. Sensitivity check (descriptive, not rescored): removing the four Ruby questions leaves coverage 17/20 answerable hits for both arms and does not change either arm's gate verdict. The paired improvements (i01, i02) occurred in the synthetic satellite log and are unaffected.

2. **w06/w18 duplicate one fact.** Both ask Ruby's first public release date and key the same passage ruby-a1#p6. w06's keyed span ("first public release of Ruby 0.95 was announced") does **not** contain the date (21 December 1995) needed to answer; w18's span does. The scorer credited span presence, so w06 could earn evidence credit on a dateless span — a key-quality defect, not a scorer logic error. Net effect: the packet effectively contains 29 distinct facts, not 30.

3. **Key repaired after first exposure.** Original freeze key vs corrected: only the `acceptable` passage-ID fields changed (hand replication had missed blank-line paragraph splitting). Questions, expected labels, spans, misleading passages, and corpus bytes are byte-identical between the two freezes (verified field-by-field). The published results are a deterministic replay under the corrected key; the first-run outputs under the original key are preserved read-only in evidence/contract/v3-*.json. No outcome-dependent content changed.
