# Ganesh M1 benchmark report — dev.v1

- questions file: `fixtures/questions/dev.v1.json`
- corpus: `fixtures/corpus` (8 articles, 346 passages, 260369 snapshot bytes)
- incomplete run: false

## Summary

| Check | Result |
| --- | --- |
| Evidence coverage | 9/11 answerable hits |
| Insufficient evidence | 2/3 correct abstentions |
| Adapter agreement | 14/14 questions identical status + ordered IDs |
| Citation integrity | 66/66 citation openings resolved to returned passage+revision |
| Provider calls | 0 |

## Coverage by question tag

| Tag | Hits | Total |
| --- | --- | --- |
| multi | 3 | 3 |
| paraphrase | 1 | 3 |
| scope | 1 | 1 |
| single | 5 | 5 |

## Per-question outcomes

| ID | Expected | REST status | Hit/correct | Adapter agree | Missing facts |
| --- | --- | --- | --- | --- | --- |
| d01 | ok | ok | hit | true |  |
| d02 | ok | ok | hit | true |  |
| d03 | ok | ok | hit | true |  |
| d04 | ok | ok | hit | true |  |
| d05 | ok | ok | hit | true |  |
| d06 | ok | ok | hit | true |  |
| d07 | ok | ok | MISS | true | Variables are immutable by default unless the mut keyword is added |
| d08 | ok | ok | hit | true |  |
| d09 | ok | ok | hit | true |  |
| d10 | ok | ok | hit | true |  |
| d11 | ok | insufficient_context | MISS | true | (abstained) |
| d12 | insufficient_context | insufficient_context | correct | true |  |
| d13 | insufficient_context | insufficient_context | correct | true |  |
| d14 | insufficient_context | ok | MISS | true |  |

## Misses

### d07 — In Rust, how does a programmer declare a variable that can be modified after creation?

- expected status: `ok`; REST status: `ok`
- returned passages: 1386182972#p30, 1387609285#p41, 1386182972#p50
- missing facts: Variables are immutable by default unless the mut keyword is added

### d11 — Which automatic mechanism does Swift use to manage memory instead of tracing garbage collection?

- expected status: `ok`; REST status: `insufficient_context`
- returned passages: (none)
- missing facts: (abstained)

### d14 — How many apps on the Apple App Store are written primarily in Swift?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1385265998#p2, 1385265998#p3, 1385265998#p1
- missing detail the corpus lacks: The Swift article mentions the App Store only in relation to Swift 2.0 availability; no count or share of apps is given anywhere in the corpus.

## Measured costs

| Measure | Value |
| --- | --- |
| Corpus size | 260369 bytes across 8 snapshots |
| Passages | 346 |
| Import time | 11.3 ms |
| Index rebuild time | 161.1 ms |
| Store size | 815104 bytes (incl. WAL) |
| REST query time | mean 8.0 ms, max 10.5 ms |
| MCP query time | mean 8.0 ms, max 9.6 ms |
| Response bytes | 107312 total across both adapters |
| Wall time | 1088 ms |
| Hardware | macos/x86_64, Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz |
| Provider calls | 0 (no network access in the retrieval path; corpus is local frozen bytes) |

## Notes

- imported 8 articles, 346 passages (260369 bytes) in 11.3 ms; indexed 346 in 161.1 ms
- Reimport idempotency verified: every article reimported against the live store reported already-present.
