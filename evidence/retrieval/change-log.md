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
