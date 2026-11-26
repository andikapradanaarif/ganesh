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
| Evidence coverage | 18/24 answerable hits |
| Insufficient evidence | 4/6 correct abstentions |
| Adapter agreement | 30/30 questions identical status + ordered IDs |
| Citation integrity | 144/144 citation openings resolved to returned passage+revision |
| Provider calls | 0 |

## Coverage by question tag

| Tag | Hits | Total |
| --- | --- | --- |
| comparison | 1 | 1 |
| multi-passage | 2 | 5 |
| ordinary-subject | 9 | 12 |
| quantity-supported | 0 | 1 |
| single-passage | 16 | 19 |
| symbolic-subject | 9 | 12 |
| two-subjects | 1 | 1 |

## Per-question outcomes

| ID | Expected | REST status | Hit/correct | Adapter agree | Missing facts |
| --- | --- | --- | --- | --- | --- |
| v01 | ok | ok | hit | true |  |
| v02 | ok | ok | hit | true |  |
| v03 | ok | insufficient_context | MISS | true | (abstained) |
| v04 | ok | ok | hit | true |  |
| v05 | ok | ok | MISS | true | PyPy uses a just-in-time compiler |
| v06 | ok | ok | hit | true |  |
| v07 | ok | insufficient_context | MISS | true | (abstained) |
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
| v18 | ok | ok | MISS | true | Hugs is a bytecode interpreter |
| v19 | ok | ok | MISS | true | C was created in 1972 by Dennis Ritchie |
| v20 | ok | ok | hit | true |  |
| v21 | ok | ok | hit | true |  |
| v22 | ok | ok | MISS | true | C89 has 32 reserved words |
| v23 | ok | ok | hit | true |  |
| v24 | ok | ok | hit | true |  |
| v25 | insufficient_context | insufficient_context | correct | true |  |
| v26 | insufficient_context | ok | MISS | true |  |
| v27 | insufficient_context | insufficient_context | correct | true |  |
| v28 | insufficient_context | insufficient_context | correct | true |  |
| v29 | insufficient_context | insufficient_context | correct | true |  |
| v30 | insufficient_context | ok | MISS | true |  |

## Misses

### v03 — Since Python 3.0, what kind of division does the single slash operator always represent?

- expected status: `ok`; REST status: `insufficient_context`
- returned passages: (none)
- missing facts: (abstained)

### v05 — Which Python implementation uses a just-in-time compiler, and which transpiler compiles a superset of Python to C?

- expected status: `ok`; REST status: `ok`
- returned passages: 1386984050#p29, 1386984050#p26, 1386984050#p28
- missing facts: PyPy uses a just-in-time compiler

### v07 — Who created C++, and in what year was it first released?

- expected status: `ok`; REST status: `insufficient_context`
- returned passages: (none)
- missing facts: (abstained)

### v18 — Which Haskell implementation is a bytecode interpreter, and which two Haskell dialects target the Java virtual machine?

- expected status: `ok`; REST status: `ok`
- returned passages: 1387873129#p17, 1387873129#p15, 1386984050#p27
- missing facts: Hugs is a bytecode interpreter

### v19 — Who created the C programming language, and in what year?

- expected status: `ok`; REST status: `ok`
- returned passages: 1387656235#p11, 1387610051#p7, 1387610051#p1
- missing facts: C was created in 1972 by Dennis Ritchie

### v22 — How many reserved words did C89 have, and how many more words did C23 reserve?

- expected status: `ok`; REST status: `ok`
- returned passages: 1387656235#p28, 1387656235#p27, 1387656235#p25
- missing facts: C89 has 32 reserved words

### v26 — How many bits wide is the default integer type in Python?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1386984050#p25, 1387656235#p31, 1386984050#p1
- missing detail the corpus lacks: The Python article discusses arbitrary-precision decimals and machine integers in implementations, but never states a default integer bit width for Python integers.

### v30 — How many operators does the C language define?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1386984050#p13, 1387656235#p2, 1387656235#p16
- missing detail the corpus lacks: The C article lists rich operator families but never states a total operator count.

## Measured costs

| Measure | Value |
| --- | --- |
| Corpus size | 135210 bytes across 4 snapshots |
| Passages | 137 |
| Import time | 9.6 ms |
| Index rebuild time | 97.7 ms |
| Store size | 471040 bytes (incl. WAL) |
| REST query time | mean 8.0 ms, max 10.6 ms |
| MCP query time | mean 8.3 ms, max 9.7 ms |
| Response bytes | 214442 total across both adapters |
| Wall time | 1744 ms |
| Hardware | macos/x86_64, Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz |
| Provider calls | 0 (no network access in the retrieval path; corpus is local frozen bytes) |

## Notes

- imported 4 articles, 137 passages (135210 bytes) in 9.6 ms; indexed 137 in 97.7 ms
- Reimport idempotency verified: every article reimported against the live store reported already-present.
