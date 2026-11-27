# Retrieval improvement change log — exposed v1 development evidence

All measurements in this file use questions already exposed by the v1 study.
They are development evidence only, not acceptance evidence. The combined
measurement file is generated at `out/exposed.v1.json` from the frozen dev and
held-out v1 keys and is never supplied to retrieval.

Baseline original retriever on exposed v1: coverage 28/35, correct
insufficient 6/9, adapter agreement 44/44, citations 216/216.

## Change 1 — symbolic identifier normalization

- Failing example: h15 asked whether a C# class can inherit from multiple base
  classes, but `C#` tokenized to a discarded one-character `c`; the top three
  were JavaScript, Java, and Ruby inheritance passages.
- Prediction: generating consistent canonical aliases for symbolic or
  punctuation-bearing identifiers on both passage and query sides would make
  the C# subject retrievable and bring `1387643334#p21` into the top three.
- Implementation: canonical alias tokens such as `csharp`, `cplusplus`, and
  `nodejs` are added alongside component tokens. Written-out forms (`C Sharp`,
  `C plus plus`) receive the same aliases. Ordinary component words remain
  indexed, preserving comparison queries and non-symbolic text.
- Actual exposed-set result: coverage stayed 28/35 and correct insufficient
  stayed 6/9. h15 changed from miss to hit with `1387643334#p21` first.
  Citation openings remained complete (210/210; the total is lower because d06
  now abstained and returned no passages).
- Regression: d06 changed from hit to false abstention. Its correct passage
  remains rank 1, but an incidental ranking shuffle changed the third union
  coverage passage; the question-only word `date` is absent from the evidence
  text and pushed IDF coverage to 0.71 below the 0.75 threshold. This is a
  coverage-boundary regression, not a loss of the correct passage.
- Decision: retain. The subject-scoping defect is decisive and general; the
  d06 boundary failure is recorded for abstention work.

## Change 2 — deeper candidates, complement selection, and subject restriction

- Failing examples: h01/h12/h19 required evidence split across two passages.
  Diagnosis at depth 20: h12 and h19 had required passages in the candidate
  pool but crowded out; h01 had `1386442349#p4` in the pool but
  `1386442349#p1` absent even at depth 20.
- Prediction: a deeper pool plus complementary selection would choose passages
  covering different facets; restricting that selection to a single explicitly
  mentioned subject would remove cross-language noise while preserving
  zero-subject and multi-subject queries.
- Actual first complement-only result: coverage fell to 27/35 and correct
  insufficient to 4/9. Correct passages often add no *new lexical terms* even
  though they add the missing fact, so marginal term coverage rewarded noise;
  increased union coverage also made h27/h30 leak. This negative result is the
  reason pure complement selection was not retained by itself.
- Actual subject-aware result: coverage 29/35, correct insufficient 6/9. h01
  improved partially by adding the correct release passage (`...#p4`) but still
  lacks the designers passage; h12/h19 remain fact-coverage misses because the
  correct passages are lexically redundant with already-selected passages.
  h25 became a correct abstention after selection was restricted to C#.
  Regression: h30 changed from correct abstention to unsupported `ok` because
  complementary coverage saturated the threshold.
- Decision: retain the subject restriction and deep pool as a net improvement;
  record h01/h12/h19 as lexical/paraphrase candidates for a later hybrid
  comparison rather than claim query decomposition can solve them. Subquery
  probes still ranked required passages only 5th–11th, so decomposition alone
  does not reliably overcome the lead-passage/length and paraphrase gap.

## Change 3 — exact quantity units and quantity-evidence abstention

- Failing examples: h26 asked for an initial goroutine stack size in bytes;
  stemming conflated the unit `bytes` with the type `byte`, and separate
  mentions of goroutines, stacks, and bytes produced full union coverage. h25
  asked for a C# keyword count while near-neighbour keyword counts existed for
  other languages.
- Prediction: preserving exact singular/plural quantity units and requiring
  quantity-like evidence for `how many`/`how much` questions would catch h26
  and h25 without false-abstaining genuinely supported numeric questions.
- Supported counterparts tested: “How many keywords does the Go language
  define?” (supported by `1386442349#p12`) and “How many bits of storage does
  the Rust i32 type take?” (supported by `1386182972#p24`). Both remained hits
  (2/2). Near-identical unsupported h25/h26 both abstained (2/2).
- Actual exposed-set result: coverage unchanged at 29/35; correct insufficient
  improved from 6/9 to 7/9. No exposed answerable question became a false
  abstention. h26 is now correct. Remaining leaks are h30 (a version/status
  transition question) and d14 (the previously documented lexically
  indistinguishable App Store count).
- Decision: retain. This is a narrow rule using only the caller-visible query
  and passage text, not the scoring key.
