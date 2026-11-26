# Ganesh M1 benchmark report — heldout.v4

- verdict: **FAIL**
- mode: held-out gate
- questions file: `fixtures/corpus-v4/key.v4.json`
- corpus: `fixtures/corpus-v4` (5 articles, 449 passages, 205486 snapshot bytes)
- incomplete run: false
- scoring specification: `scoring.v2` (coverage 20/24, insufficient 6/6, adapter total 30, top 3)

## Summary

| Check | Result |
| --- | --- |
| Evidence coverage | 15/24 answerable hits |
| Insufficient evidence | 2/6 correct abstentions |
| Adapter agreement | 30/30 questions identical status + ordered IDs |
| Citation integrity | 168/168 citation openings resolved to returned passage+revision |
| Provider calls | 0 |

## Coverage by question tag

| Tag | Hits | Total |
| --- | --- | --- |
| answerable | 15 | 24 |
| synthetic | 8 | 8 |
| wiki | 7 | 16 |

## Per-question outcomes

| ID | Expected | REST status | Hit/correct | Adapter agree | Missing facts |
| --- | --- | --- | --- | --- | --- |
| w01 | ok | ok | MISS | true | launched from Cape Canaveral, Florida, on November 26, 2011 |
| w02 | ok | ok | MISS | true | landed on Aeolis Palus inside Gale crater on Mars on August 6, 2012 |
| w03 | ok | ok | MISS | true | landed on Aeolis Palus inside Gale crater |
| w04 | ok | ok | hit | true |  |
| w05 | ok | ok | MISS | true | crossed the heliopause and entered interstellar space on August 25, 20 |
| w06 | ok | ok | MISS | true | launched by NASA on September 5, 1977 |
| w07 | ok | ok | hit | true |  |
| w08 | ok | ok | MISS | true | launched into low Earth orbit in 1990 |
| w09 | ok | ok | hit | true |  |
| w10 | ok | ok | hit | true |  |
| w11 | ok | ok | MISS | true | launched on July 30, 2020 |
| w12 | ok | ok | MISS | true | landed in Jezero Crater |
| w13 | ok | ok | hit | true |  |
| w14 | ok | ok | MISS | true | made 72 flights in nearly three years |
| w15 | ok | ok | hit | true |  |
| w16 | ok | ok | hit | true |  |
| s01 | ok | ok | hit | true |  |
| s02 | ok | ok | hit | true |  |
| s03 | ok | ok | hit | true |  |
| s04 | ok | ok | hit | true |  |
| s05 | ok | ok | hit | true |  |
| s06 | ok | ok | hit | true |  |
| s07 | ok | ok | hit | true |  |
| s08 | ok | ok | hit | true |  |
| i01 | insufficient_context | ok | MISS | true |  |
| i02 | insufficient_context | insufficient_context | correct | true |  |
| i03 | insufficient_context | insufficient_context | correct | true |  |
| i04 | insufficient_context | ok | MISS | true |  |
| i05 | insufficient_context | ok | MISS | true |  |
| i06 | insufficient_context | ok | MISS | true |  |

## Misses

### w01 — When was the Curiosity rover launched?

- expected status: `ok`; REST status: `ok`
- returned passages: curiosity-a1#p19, curiosity-a1#p86, curiosity-a1#p32
- missing facts: launched from Cape Canaveral, Florida, on November 26, 2011

### w02 — When did Curiosity land on Mars?

- expected status: `ok`; REST status: `ok`
- returned passages: curiosity-a1#p16, curiosity-a1#p34, curiosity-a1#p80
- missing facts: landed on Aeolis Palus inside Gale crater on Mars on August 6, 2012

### w03 — Where on Mars did Curiosity land?

- expected status: `ok`; REST status: `ok`
- returned passages: curiosity-a1#p16, curiosity-a1#p34, curiosity-a1#p80
- missing facts: landed on Aeolis Palus inside Gale crater

### w05 — When did Voyager 1 cross the heliopause?

- expected status: `ok`; REST status: `ok`
- returned passages: voyager-a1#p70, voyager-a1#p81, voyager-a1#p80
- missing facts: crossed the heliopause and entered interstellar space on August 25, 20

### w06 — When was Voyager 1 launched?

- expected status: `ok`; REST status: `ok`
- returned passages: voyager-a1#p35, voyager-a1#p37, voyager-a1#p34
- missing facts: launched by NASA on September 5, 1977

### w08 — In which year was the Hubble Space Telescope launched?

- expected status: `ok`; REST status: `ok`
- returned passages: hubble-a1#p39, hubble-a1#p10, hubble-a1#p116
- missing facts: launched into low Earth orbit in 1990

### w11 — When was the Perseverance rover launched?

- expected status: `ok`; REST status: `ok`
- returned passages: perseverance-a1#p28, perseverance-a1#p27, perseverance-a1#p7
- missing facts: launched on July 30, 2020

### w12 — In which crater did Perseverance land?

- expected status: `ok`; REST status: `ok`
- returned passages: perseverance-a1#p10, perseverance-a1#p29, perseverance-a1#p36
- missing facts: landed in Jezero Crater

### w14 — How many flights did Ingenuity make in total?

- expected status: `ok`; REST status: `ok`
- returned passages: perseverance-a1#p39, perseverance-a1#p3, hubble-a1#p37
- missing facts: made 72 flights in nearly three years

### i01 — Did the germination test of seed lot G12 finish on January 7?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: seedlog-a1#p4, seedlog-a1#p5, seedlog-a1#p6

### i04 — On what exact calendar date did Congress approve funding for the Large Space Telescope?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: hubble-a1#p15, hubble-a1#p9, hubble-a1#p18

### i05 — What is the diameter of Hubble's main mirror in meters?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: hubble-a1#p53, hubble-a1#p7, hubble-a1#p139

### i06 — On what date did the Curiosity rover's mission end?

- expected status: `insufficient_context`; REST status: `ok`
- returned passages: curiosity-a1#p43, curiosity-a1#p62, voyager-a1#p80

## Measured costs

| Measure | Value |
| --- | --- |
| Corpus size | 205486 bytes across 5 snapshots |
| Passages | 449 |
| Import time | 25.6 ms |
| Index rebuild time | 431.1 ms |
| Store size | 786432 bytes (incl. WAL) |
| REST query time | mean 50.7 ms, max 92.3 ms |
| MCP query time | mean 54.0 ms, max 143.9 ms |
| Response bytes | 99660 total across both adapters |
| Wall time | 3862 ms |
| Hardware | macos/x86_64, Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz |
| Provider calls | 0 (no network access in the retrieval path; corpus is local frozen bytes) |

## Notes

- imported 5 articles, 449 passages (205486 bytes) in 25.6 ms; indexed 449 in 431.1 ms
- Reimport idempotency verified: every article reimported against the live store reported already-present.
