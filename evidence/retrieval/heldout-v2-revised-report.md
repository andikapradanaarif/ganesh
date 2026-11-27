# Ganesh M1 benchmark report — heldout.v2

- verdict: **FAIL**
- mode: held-out gate
- questions file: `/Users/andikaarif/Documents/p/playground/ganesh/.local/wt/m1-retrieval/fixtures/questions-v2/heldout.v2.json`
- corpus: `/Users/andikaarif/Documents/p/playground/ganesh/.local/wt/m1-retrieval/fixtures/corpus-v2` (4 articles, 137 passages, 135210 snapshot bytes)
- incomplete run: false
- scoring specification: `scoring.v2` (coverage 20/24, insufficient 6/6, adapter total 30, top 3)

## Summary

| Check | Result |
| --- | --- |
| Evidence coverage | 21/24 answerable hits |
| Insufficient evidence | 2/6 correct abstentions |
| Adapter agreement | 30/30 questions identical status + ordered IDs |
| Citation integrity | 156/156 citation openings resolved to returned passage+revision |
| Provider calls | 0 |

## Coverage by question tag

| Tag | Hits | Total |
| --- | --- | --- |
| comparison | 1 | 1 |
| multi-passage | 4 | 5 |
| ordinary-subject | 11 | 12 |
| quantity-supported | 0 | 1 |
| single-passage | 17 | 19 |
| symbolic-subject | 10 | 12 |
| two-subjects | 1 | 1 |

## Per-question outcomes

| ID | Expected | REST status | Hit/correct | Adapter agree | Missing facts |
| --- | --- | --- | --- | --- | --- |
| v01 | ok | insufficient_context | MISS | true | (abstained) |
| v02 | ok | ok | hit | true |  |
| v03 | ok | ok | hit | true |  |
| v04 | ok | ok | hit | true |  |
| v05 | ok | ok | hit | true |  |
| v06 | ok | ok | hit | true |  |
| v07 | ok | ok | hit | true |  |
| v08 | ok | ok | hit | true |  |
| v09 | ok | ok | hit | true |  |
| v10 | ok | ok | hit | true |  |
| v11 | ok | ok | hit | true |  |
| v12 | ok | ok | hit | true |  |
| v13 | ok | ok | hit | true |  |
| v14 | ok | ok | hit | true |  |
| v15 | ok | ok | hit | true |  |
| v16 | ok | ok | hit | true |  |
| v17 | ok | ok | hit | true |  |
| v18 | ok | ok | hit | true |  |
| v19 | ok | ok | MISS | true | C was created in 1972 by Dennis Ritchie |
| v20 | ok | ok | hit | true |  |
| v21 | ok | ok | hit | true |  |
| v22 | ok | insufficient_context | MISS | true | (abstained) |
| v23 | ok | ok | hit | true |  |
| v24 | ok | ok | hit | true |  |
| v25 | insufficient_context | ok | MISS | true |  |
| v26 | insufficient_context | insufficient_context | correct | true |  |
| v27 | insufficient_context | ok | MISS | true |  |
| v28 | insufficient_context | ok | MISS | true |  |
| v29 | insufficient_context | insufficient_context | correct | true |  |
| v30 | insufficient_context | ok | MISS | true |  |

## Misses

### v01 — Who conceived Python, and at which research institute in the Netherlands did that happen?

- expected status: `ok`; REST status: `insufficient_context`
- returned passages: (none)
- missing facts: (abstained)

### v19 — Who created the C programming language, and in what year?

- expected status: `ok`; REST status: `ok`
- returned passages: 1387656235#p11, 1387610051#p7, 1387656235#p49
- missing facts: C was created in 1972 by Dennis Ritchie

### v22 — How many reserved words did C89 have, and how many more words did C23 reserve?

- expected status: `ok`; REST status: `insufficient_context`
- returned passages: (none)
- missing facts: (abstained)

### v25 — How many keywords did C++23 add to the C++ language?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1387610051#p11, 1387610051#p1, 1387610051#p15
- missing detail the corpus lacks: The C++ article discusses standards and mentions keywords in compatibility text, but never counts the keywords added by C++23. Near-neighbour counts exist in the C article, making the question plausible but unsupported.

### v27 — In which Haskell version did lazy evaluation become optional rather than the default?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1387873129#p22, 1387873129#p10, 1387873129#p6
- missing detail the corpus lacks: The Haskell article describes lazy evaluation as a defining feature and discusses versions/extensions, but never states a version in which lazy evaluation became optional.

### v28 — How many members serve on the ISO working group that standardizes C++?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1387610051#p7, 1387610051#p8, 1387610051#p3
- missing detail the corpus lacks: The C++ article names the ISO working group and describes its meetings, but never states its membership count.

### v30 — How many operators does the C language define?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1386984050#p13, 1387656235#p2, 1387656235#p16
- missing detail the corpus lacks: The C article lists rich operator families but never states a total operator count.

## Measured costs

| Measure | Value |
| --- | --- |
| Corpus size | 135210 bytes across 4 snapshots |
| Passages | 137 |
| Import time | 8.8 ms |
| Index rebuild time | 100.8 ms |
| Store size | 483328 bytes (incl. WAL) |
| REST query time | mean 14.6 ms, max 17.1 ms |
| MCP query time | mean 15.0 ms, max 18.9 ms |
| Response bytes | 238024 total across both adapters |
| Wall time | 1925 ms |
| Hardware | macos/x86_64, Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz |
| Provider calls | 0 (no network access in the retrieval path; corpus is local frozen bytes) |

## Notes

- imported 4 articles, 137 passages (135210 bytes) in 8.8 ms; indexed 137 in 100.8 ms
- Reimport idempotency verified: every article reimported against the live store reported already-present.
