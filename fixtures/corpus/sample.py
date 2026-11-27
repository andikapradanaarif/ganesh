#!/usr/bin/env python3
"""Corpus sampling for the Ganesh M1 fixture (dev-time tool, not part of the replay path).

Frame: 14 widely-known general-purpose programming languages with substantial
English Wikipedia coverage and heavy conceptual overlap (typing, memory
management, concurrency, tooling), making cross-article distractors plausible.
Seed: 20261126 (retrieval date). Method: seeded shuffle of the frame, take the
first 8, then seeded assignment into development (4) and held-out (4) families.
Exclusion rule: an article whose plaintext extract is shorter than 15000 bytes
is excluded and replaced by the next article in shuffled order.
"""
import json, random, sys

FRAME = ["C (programming language)", "C++", "C Sharp (programming language)",
         "Go (programming language)", "Haskell", "Java (programming language)",
         "JavaScript", "Kotlin (programming language)", "PHP", "Python (programming language)",
         "Ruby (programming language)", "Rust (programming language)",
         "Swift (programming language)", "TypeScript"]

rng = random.Random(20261126)
order = FRAME[:]
rng.shuffle(order)
selected = order[:8]
rng2 = random.Random(20261126 + 1)
split = selected[:]
rng2.shuffle(split)
print(json.dumps({
    "frame": FRAME, "seed": 20261126, "shuffled_order": order,
    "selected": selected,
    "dev_family": sorted(split[:4]), "heldout_family": sorted(split[4:]),
}, indent=2))
