#!/usr/bin/env python3
"""Apply the frozen engineering rubric; does not launch collectors."""

import json
import math
from pathlib import Path
import statistics
import sys

from measure_startup import percentile, rows


def evaluate(results):
    expected = set()
    for arm, count in [("baseline", 10), ("trace", 5)]:
        for block in range(count):
            for category in ["new_file_first", "warm"]:
                expected.add(("diagnosis", arm, category, block))
    for arm in ["baseline", "candidate"]:
        for category, count in [("new_file_first", 20), ("warm", 20), ("idle_30s", 2)]:
            for block in range(count):
                expected.add(("comparison", arm, category, block))
    keys = [(r["stage"], r["arm"], r["category"], r["block"]) for r in results]
    valid_times = all(type(r["parent_ms"]) in (int, float) and math.isfinite(r["parent_ms"]) and r["parent_ms"] > 0 for r in results)
    if len(keys) != 114 or set(keys) != expected or not valid_times:
        return {"status": "incomplete", "attempts": len(results), "build_recommendation": False,
                "capture_profile_enabled": False}
    paired = {}
    for r in results:
        if r["stage"] == "comparison":
            paired.setdefault((r["category"], r["block"]), {})[r["arm"]] = r
    added = []
    improvements, first_baseline, first_candidate = [], [], []
    for (category, block), arms in sorted(paired.items()):
        baseline, candidate = arms["baseline"], arms["candidate"]
        if baseline["success"] and not candidate["success"]:
            added.append({"category": category, "block": block})
        if category == "new_file_first":
            b, c = baseline["parent_ms"], candidate["parent_ms"]
            improvements.append((b - c) / b)
            first_baseline.append(b)
            first_candidate.append(c)
    arms = {arm: [r for r in results if r["stage"] == "comparison" and r["arm"] == arm]
            for arm in ["baseline", "candidate"]}
    failures = {arm: sum(not r["success"] for r in group) for arm, group in arms.items()}
    unexpected = sum(any(r["output_bytes"].values()) for r in arms["candidate"])
    invalid_records = sum(r["record"] == "invalid" or r["exit"] == 0 and r["record"] != "present_unknown_incomplete" for r in arms["candidate"])
    median = statistics.median(improvements)
    baseline_p95, candidate_p95 = percentile(first_baseline, .95), percentile(first_candidate, .95)
    fewer_failures = failures["candidate"] < failures["baseline"]
    speed_signal = median >= .20 and candidate_p95 <= baseline_p95
    recommended = not added and not unexpected and not invalid_records and (fewer_failures or speed_signal)
    return {"status": "complete_local_comparison", "attempts": len(results),
        "failures": failures, "added_failure_pairs": added,
        "unexpected_output_attempts": unexpected, "invalid_candidate_records": invalid_records,
        "paired_first_improvements": improvements, "paired_median_improvement": median,
        "first_p95_ms": {"baseline": baseline_p95, "candidate": candidate_p95},
        "fewer_failures": fewer_failures, "speed_signal": speed_signal,
        "build_recommendation": bool(recommended), "capture_profile_enabled": False}


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("provide the private run directory")
    print(json.dumps(evaluate(rows(Path(sys.argv[1]))), indent=2))
