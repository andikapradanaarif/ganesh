#!/usr/bin/env python3
"""Issue #36's separate frozen comparison; legacy study #34 is unchanged."""

import datetime as dt
import hashlib
import json
import math
import os
from pathlib import Path
import random
import shutil
import statistics
import subprocess
import sys
import time

from measure_startup import ENV, artifact, command, load, percentile, rows, write_new
from observe_standalone import Budget, Clock, observe

ROOT = Path(__file__).resolve().parents[2]
END = dt.datetime(2026, 11, 2, 7, 59, 33, tzinfo=dt.timezone.utc).timestamp()
LIMIT = 84


def prepare(directory):
    if sys.platform != "darwin" or os.environ.get("GOFLAGS") or os.environ.get("GODEBUG"):
        raise RuntimeError("unsupported environment")
    command(["git", "diff", "--quiet", "HEAD", "--"], ROOT)
    if subprocess.check_output(["git", "ls-files", "--others", "--exclude-standard", "--", "*.go"], cwd=ROOT):
        raise RuntimeError("untracked Go source")
    directory.mkdir(mode=0o700)
    deps = subprocess.check_output(["go", "list", "-deps", "./cmd/ganesh-capture"], cwd=ROOT, env=ENV, text=True).splitlines()
    if "github.com/andikapradanaarif/ganesh/internal/s0" in deps or "os/exec" in deps:
        raise RuntimeError("standalone dependency boundary violated")
    for arm, package in [("baseline", "./cmd/ganesh"), ("candidate", "./cmd/ganesh-capture")]:
        command(["go", "build", "-ldflags=-s -w", "-o", str(directory / arm), package], ROOT)
    command(["cc", "-x", "c", "-dynamiclib", "-o", str(directory / "clock.dylib"),
             str(ROOT / "integration/capture/process_start.c.fixture")])
    source = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    write_new(directory / "plan.json", {"study": 36, "source": source, "deadline_utc": END,
        "attempt_cap": LIMIT, "seed": 36, "cache_state": "uncontrolled", "candidate_dependencies": deps,
        "toolchain": subprocess.check_output(["go", "version"], env=ENV, text=True).strip(),
        "artifacts": {arm: artifact(directory / arm, ["-ldflags=-s -w"]) for arm in ["baseline", "candidate"]},
        "producer_digests": {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in [Path(__file__).resolve(), ROOT / "integration/capture/observe_standalone.py",
                ROOT / "integration/capture/measure_startup.py", ROOT / "integration/capture/process_start.c.fixture",
                ROOT / "docs/s0-standalone-capture.md"]}})
    print("Prepared same-source artifacts; no native attempts run", flush=True)


def run(directory):
    plan = load(directory / "plan.json")
    if plan["study"] != 36 or plan["attempt_cap"] != LIMIT or plan["deadline_utc"] != END or rows(directory):
        raise RuntimeError("frozen binding changed or run already attempted")
    for file, digest in plan["producer_digests"].items():
        if hashlib.sha256((ROOT / file).read_bytes()).hexdigest() != digest:
            raise RuntimeError("producer changed after preparation")
    for arm, info in plan["artifacts"].items():
        if artifact(directory / arm, [])["sha256"] != info["sha256"]:
            raise RuntimeError("artifact changed after preparation")
    budget, clock = Budget(END, LIMIT), Clock(directory)
    order = [False] * 10 + [True] * 10
    random.Random(36).shuffle(order)
    idle = {}
    for block, reverse in enumerate(order):
        for arm in (["candidate", "baseline"] if reverse else ["baseline", "candidate"]):
            executable = directory / ("copy-%s-%02d" % (arm, block))
            with executable.open("xb") as output, (directory / arm).open("rb") as original:
                shutil.copyfileobj(original, output)
            executable.chmod(0o500)
            observe(directory, clock, executable, arm, "new_file_first", block, budget)
            last = observe(directory, clock, executable, arm, "warm", block, budget)
            if block == 19:
                idle[arm] = executable, last
            else:
                executable.unlink()
        if block % 5 == 4:
            print("Completed blocks: %d/20" % (block + 1), flush=True)
    for block in range(2):
        for arm in ["baseline", "candidate"]:
            executable, last = idle[arm]
            gap = (time.time_ns() - last["finished_ns"]) / 1e9
            time.sleep(max(0, min(30, 30 - gap)))
            gap = (time.time_ns() - last["finished_ns"]) / 1e9
            last = observe(directory, clock, executable, arm, "idle_30s", block, budget, gap)
            idle[arm] = executable, last
            print("Completed idle observation: " + arm, flush=True)
    write_new(directory / "complete.json", {"attempts": len(rows(directory))})


def evaluate(results):
    expected = {("standalone36", arm, category, block) for arm in ["baseline", "candidate"]
        for category, count in [("new_file_first", 20), ("warm", 20), ("idle_30s", 2)] for block in range(count)}
    keys = [(r.get("stage"), r["arm"], r["category"], r["block"]) for r in results]
    if len(keys) != LIMIT or set(keys) != expected or any(not math.isfinite(r["parent_ms"]) or r["parent_ms"] <= 0 for r in results):
        return {"status": "incomplete", "attempts": len(results), "local_signal": False,
                "headroom_probe": False, "capture_profile_enabled": False}
    groups, first, failures = {}, {}, {}
    for arm in ["baseline", "candidate"]:
        failures[arm] = {}
        for category in ["new_file_first", "warm", "idle_30s"]:
            group = [r for r in results if (r["arm"], r["category"]) == (arm, category)]
            times = [r["parent_ms"] for r in group]
            spans = [r["os_to_exit_ms"] for r in group if r["os_to_exit_ms"] is not None and math.isfinite(r["os_to_exit_ms"]) and r["os_to_exit_ms"] > 0]
            failures[arm][category] = sum(not r["success"] for r in group)
            groups[(arm, category)] = {"n": len(group), "failures": failures[arm][category],
                "median_ms": statistics.median(times), "p95_ms": percentile(times, .95), "p99_ms": percentile(times, .99),
                "os_p99_ms": percentile(spans, .99), "os_missing": len(group) - len(spans)}
        first[arm] = {r["block"]: r for r in results if r["arm"] == arm and r["category"] == "new_file_first"}
    controls = {arm: {"unexpected_output": sum(any(r["output_bytes"].values()) for r in results if r["arm"] == arm),
        "invalid_records": sum(r["record"] == "invalid" or r["exit"] == 0 and r["record"] != "present_unknown_incomplete" for r in results if r["arm"] == arm)}
        for arm in ["baseline", "candidate"]}
    by_cell = {(r["arm"], r["category"], r["block"]): r for r in results}
    added = [{"category": category, "block": block} for category, count in [("new_file_first", 20), ("warm", 20), ("idle_30s", 2)]
        for block in range(count) if by_cell[("baseline", category, block)]["success"] and not by_cell[("candidate", category, block)]["success"]]
    improvements = [(first["baseline"][i]["parent_ms"] - first["candidate"][i]["parent_ms"]) / first["baseline"][i]["parent_ms"] for i in range(20)]
    median = statistics.median(improvements)
    controls_ok = not any(value for arm in controls.values() for value in arm.values())
    no_increase = all(failures["candidate"][category] <= failures["baseline"][category] for category in ["new_file_first", "warm", "idle_30s"])
    tails_ok = all(groups[("candidate", category)]["p95_ms"] <= groups[("baseline", category)]["p95_ms"] for category in ["new_file_first", "warm"])
    signal = controls_ok and not added and no_increase and tails_ok and (failures["candidate"]["new_file_first"] < failures["baseline"]["new_file_first"] or median >= .20)
    headroom = controls_ok and all(groups[("candidate", c)]["failures"] == 0 and groups[("candidate", c)]["os_missing"] == 0 and groups[("candidate", c)]["os_p99_ms"] <= 600 for c in ["new_file_first", "warm", "idle_30s"])
    return {"status": "complete_local_comparison", "attempts": len(results), "controls": controls,
        "groups": {arm + "/" + category: value for (arm, category), value in groups.items()},
        "added_failure_pairs": added, "paired_first_improvements": improvements, "paired_median_improvement": median,
        "tail_gates_passed": tails_ok, "local_signal": bool(signal), "headroom_probe": bool(headroom),
        "capture_profile_enabled": False}


def evaluate_bound(plan, results):
    # Summary accepts the same manifest identity required before collection.
    # This validates input identity without changing the frozen score rule.
    if plan.get("study") != 36 or plan.get("attempt_cap") != LIMIT or plan.get("deadline_utc") != END:
        return {"status": "invalid_study_binding", "attempts": len(results),
                "local_signal": False, "headroom_probe": False, "capture_profile_enabled": False}
    return evaluate(results)


def main():
    if len(sys.argv) != 3 or sys.argv[1] not in ("prepare", "run", "summary"):
        raise RuntimeError("invalid study command")
    mode, directory = sys.argv[1], Path(sys.argv[2]).resolve()
    if mode != "summary":
        Budget(END, LIMIT).check(0)
    if mode == "prepare":
        prepare(directory)
    elif directory.stat().st_mode & 0o777 != 0o700:
        raise RuntimeError("study directory is not private")
    elif mode == "run":
        run(directory)
    else:
        print(json.dumps(evaluate_bound(load(directory / "plan.json"), rows(directory)), indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("standalone study incomplete: " + type(error).__name__, file=sys.stderr)
        raise SystemExit(1)
