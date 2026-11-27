# Ganesh M1 benchmark report — heldout.v3

- verdict: **FAIL**
- mode: held-out gate
- questions file: `fixtures/corpus-v3/key.v3.json`
- corpus: `fixtures/corpus-v3` (6 articles, 358 passages, 98354 snapshot bytes)
- incomplete run: false
- scoring specification: `scoring.v2` (coverage 20/24, insufficient 6/6, adapter total 30, top 3)

## Summary

| Check | Result |
| --- | --- |
| Evidence coverage | 21/24 answerable hits |
| Insufficient evidence | 5/6 correct abstentions |
| Adapter agreement | 30/30 questions identical status + ordered IDs |
| Citation integrity | 138/138 citation openings resolved to returned passage+revision |
| Provider calls | 0 |

## Coverage by question tag

| Tag | Hits | Total |
| --- | --- | --- |
| answerable | 21 | 24 |
| synthetic | 6 | 6 |
| wiki | 15 | 18 |

## Per-question outcomes

| ID | Expected | REST status | Hit/correct | Adapter agree | Missing facts |
| --- | --- | --- | --- | --- | --- |
| w01 | ok | ok | hit | true |  |
| w02 | ok | ok | hit | true |  |
| w03 | ok | ok | hit | true |  |
| w04 | ok | ok | hit | true |  |
| w05 | ok | ok | hit | true |  |
| w06 | ok | ok | hit | true |  |
| w07 | ok | ok | hit | true |  |
| w08 | ok | ok | hit | true |  |
| w09 | ok | ok | MISS | true | Lua was created in 1993 by Roberto Ierusalimschy |
| w10 | ok | insufficient_context | MISS | true | (abstained) |
| w11 | ok | ok | hit | true |  |
| w12 | ok | ok | hit | true |  |
| w13 | ok | ok | hit | true |  |
| w14 | ok | ok | hit | true |  |
| w15 | ok | insufficient_context | MISS | true | (abstained) |
| w16 | ok | ok | hit | true |  |
| w17 | ok | ok | hit | true |  |
| w18 | ok | ok | hit | true |  |
| s01 | ok | ok | hit | true |  |
| s02 | ok | ok | hit | true |  |
| s03 | ok | ok | hit | true |  |
| s04 | ok | ok | hit | true |  |
| h01 | ok | ok | hit | true |  |
| h02 | ok | ok | hit | true |  |
| i01 | insufficient_context | insufficient_context | correct | true |  |
| i02 | insufficient_context | insufficient_context | correct | true |  |
| i03 | insufficient_context | ok | MISS | true |  |
| i04 | insufficient_context | insufficient_context | correct | true |  |
| i05 | insufficient_context | insufficient_context | correct | true |  |
| i06 | insufficient_context | insufficient_context | correct | true |  |

## Misses

### w09 — Who created the Lua programming language?

- expected status: `ok`; REST status: `ok`
- returned passages: lua-a1#p57, lua-a1#p58, lua-a1#p108
- missing facts: Lua was created in 1993 by Roberto Ierusalimschy

### w10 — Approximately how many lines of C code implement Lua version 5.5.0?

- expected status: `ok`; REST status: `insufficient_context`
- returned passages: (none)
- missing facts: (abstained)

### w15 — In which year was the Lua programming language created?

- expected status: `ok`; REST status: `insufficient_context`
- returned passages: (none)
- missing facts: (abstained)

### i03 — Did the telemetry verification pass for the March 14 pass of satellite K3?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: satlog-a1#p6, satlog-a1#p2, satlog-a1#p5

## Measured costs

| Measure | Value |
| --- | --- |
| Corpus size | 98354 bytes across 6 snapshots |
| Passages | 358 |
| Import time | 8.7 ms |
| Index rebuild time | 136.8 ms |
| Store size | 417792 bytes (incl. WAL) |
| REST query time | mean 9.1 ms, max 13.1 ms |
| MCP query time | mean 9.2 ms, max 13.0 ms |
| Response bytes | 64130 total across both adapters |
| Wall time | 784 ms |
| Hardware | macos/x86_64, Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz |
| Provider calls | 0 (no network access in the retrieval path; corpus is local frozen bytes) |

## Notes

- imported 6 articles, 358 passages (98354 bytes) in 8.7 ms; indexed 358 in 136.8 ms
- Reimport idempotency verified: every article reimported against the live store reported already-present.
