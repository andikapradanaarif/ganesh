# Ganesh M1 benchmark report — exposed.v1

- verdict: **DEVELOPMENT**
- mode: development
- questions file: `out/exposed.v1.json`
- corpus: `fixtures/corpus` (8 articles, 346 passages, 260369 snapshot bytes)
- incomplete run: false

## Summary

| Check | Result |
| --- | --- |
| Evidence coverage | 29/35 answerable hits |
| Insufficient evidence | 6/9 correct abstentions |
| Adapter agreement | 44/44 questions identical status + ordered IDs |
| Citation integrity | 216/216 citation openings resolved to returned passage+revision |
| Provider calls | 0 |

## Coverage by question tag

| Tag | Hits | Total |
| --- | --- | --- |
| multi | 7 | 11 |
| near-neighbour | 5 | 5 |
| paraphrase | 6 | 8 |
| scope | 5 | 5 |
| single | 11 | 11 |

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
| h01 | ok | ok | MISS | true | Go was originally designed in 2007 by Robert Griesemer, Rob Pike, and Ken Thompson |
| h02 | ok | ok | hit | true |  |
| h03 | ok | ok | hit | true |  |
| h04 | ok | ok | hit | true |  |
| h05 | ok | ok | hit | true |  |
| h06 | ok | ok | hit | true |  |
| h07 | ok | ok | hit | true |  |
| h08 | ok | ok | hit | true |  |
| h09 | ok | ok | hit | true |  |
| h10 | ok | ok | hit | true |  |
| h11 | ok | ok | hit | true |  |
| h12 | ok | ok | MISS | true | JavaScript was created by Brendan Eich in 1995 |
| h13 | ok | insufficient_context | MISS | true | (abstained) |
| h14 | ok | ok | hit | true |  |
| h15 | ok | ok | hit | true |  |
| h16 | ok | ok | hit | true |  |
| h17 | ok | ok | hit | true |  |
| h18 | ok | ok | hit | true |  |
| h19 | ok | ok | MISS | true | Ruby development began in the mid-1990s by Yukihiro "Matz" Matsumoto in Japan |
| h20 | ok | ok | hit | true |  |
| h21 | ok | ok | hit | true |  |
| h22 | ok | ok | hit | true |  |
| h23 | ok | ok | hit | true |  |
| h24 | ok | ok | hit | true |  |
| h25 | insufficient_context | insufficient_context | correct | true |  |
| h26 | insufficient_context | ok | MISS | true |  |
| h27 | insufficient_context | insufficient_context | correct | true |  |
| h28 | insufficient_context | insufficient_context | correct | true |  |
| h29 | insufficient_context | insufficient_context | correct | true |  |
| h30 | insufficient_context | ok | MISS | true |  |

## Misses

### d07 — In Rust, how does a programmer declare a variable that can be modified after creation?

- expected status: `ok`; REST status: `ok`
- returned passages: 1386182972#p30, 1386182972#p50, 1386182972#p49
- missing facts: Variables are immutable by default unless the mut keyword is added

### d11 — Which automatic mechanism does Swift use to manage memory instead of tracing garbage collection?

- expected status: `ok`; REST status: `insufficient_context`
- returned passages: (none)
- missing facts: (abstained)

### d14 — How many apps on the Apple App Store are written primarily in Swift?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1385265998#p2, 1385265998#p3, 1385265998#p1
- missing detail the corpus lacks: The Swift article mentions the App Store only in relation to Swift 2.0 availability; no count or share of apps is given anywhere in the corpus.

### h01 — Who originally designed the Go programming language, and in which month and year was Go version 1.0 released?

- expected status: `ok`; REST status: `ok`
- returned passages: 1386442349#p8, 1386442349#p41, 1386442349#p4
- missing facts: Go was originally designed in 2007 by Robert Griesemer, Rob Pike, and Ken Thompson

### h12 — In what year was ECMAScript 6 published, and who originally created JavaScript?

- expected status: `ok`; REST status: `ok`
- returned passages: 1387609285#p6, 1387609285#p4, 1387609285#p5
- missing facts: JavaScript was created by Brendan Eich in 1995

### h13 — What did C#'s original name COOL stand for, and which body first approved C# as an international standard?

- expected status: `ok`; REST status: `insufficient_context`
- returned passages: (none)
- missing facts: (abstained)

### h19 — Who developed Ruby, and which two names were initially proposed for the language?

- expected status: `ok`; REST status: `ok`
- returned passages: 1387629546#p3, 1387629546#p5, 1387629546#p4
- missing facts: Ruby development began in the mid-1990s by Yukihiro "Matz" Matsumoto in Japan

### h26 — How many bytes of stack memory is a new goroutine initially allocated in Go?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1386442349#p23, 1386442349#p14, 1386442349#p27
- missing detail the corpus lacks: The Go article discusses goroutine implementation and multiplexing onto OS threads but never states an initial stack size.

### h30 — In which Ruby version did pattern matching first become a stable, non-experimental feature?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: 1387629546#p9, 1387629546#p4, 1387629546#p16
- missing detail the corpus lacks: The Ruby article records pattern matching as experimental in Ruby 2.7.0 but never states a version in which it became stable.

## Measured costs

| Measure | Value |
| --- | --- |
| Corpus size | 260369 bytes across 8 snapshots |
| Passages | 346 |
| Import time | 15.9 ms |
| Index rebuild time | 182.8 ms |
| Store size | 831488 bytes (incl. WAL) |
| REST query time | mean 19.4 ms, max 30.2 ms |
| MCP query time | mean 20.1 ms, max 38.2 ms |
| Response bytes | 297436 total across both adapters |
| Wall time | 2602 ms |
| Hardware | macos/x86_64, Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz |
| Provider calls | 0 (no network access in the retrieval path; corpus is local frozen bytes) |

## Notes

- imported 8 articles, 346 passages (260369 bytes) in 15.9 ms; indexed 346 in 182.8 ms
- Reimport idempotency verified: every article reimported against the live store reported already-present.
