# Ganesh M1 benchmark report — quantity-dev.v1

- verdict: **DEVELOPMENT**
- mode: development
- questions file: `out/quantity-dev.json`
- corpus: `fixtures/corpus` (8 articles, 346 passages, 260369 snapshot bytes)
- incomplete run: false

## Summary

| Check | Result |
| --- | --- |
| Evidence coverage | 2/2 answerable hits |
| Insufficient evidence | 2/2 correct abstentions |
| Adapter agreement | 4/4 questions identical status + ordered IDs |
| Citation integrity | 12/12 citation openings resolved to returned passage+revision |
| Provider calls | 0 |

## Coverage by question tag

| Tag | Hits | Total |
| --- | --- | --- |
| quantity-supported | 2 | 2 |

## Per-question outcomes

| ID | Expected | REST status | Hit/correct | Adapter agree | Missing facts |
| --- | --- | --- | --- | --- | --- |
| qgo | ok | ok | hit | true |  |
| qrust | ok | ok | hit | true |  |
| qh25 | insufficient_context | insufficient_context | correct | true |  |
| qh26 | insufficient_context | insufficient_context | correct | true |  |

## Misses

None.

## Measured costs

| Measure | Value |
| --- | --- |
| Corpus size | 260369 bytes across 8 snapshots |
| Passages | 346 |
| Import time | 13.4 ms |
| Index rebuild time | 185.4 ms |
| Store size | 831488 bytes (incl. WAL) |
| REST query time | mean 17.0 ms, max 17.6 ms |
| MCP query time | mean 16.7 ms, max 17.1 ms |
| Response bytes | 12256 total across both adapters |
| Wall time | 973 ms |
| Hardware | macos/x86_64, Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz |
| Provider calls | 0 (no network access in the retrieval path; corpus is local frozen bytes) |

## Notes

- imported 8 articles, 346 passages (260369 bytes) in 13.4 ms; indexed 346 in 185.4 ms
- Reimport idempotency verified: every article reimported against the live store reported already-present.
