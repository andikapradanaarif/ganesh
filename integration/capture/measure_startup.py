#!/usr/bin/env python3
"""Issue #34's frozen synthetic experiment; raw child output is never retained.

Usage: python3 integration/capture/measure_startup.py prepare|diagnose|compare|summary DIR
Only Darwin/CGO is supported. See docs/s0-capture-startup.md before running.
"""

import ctypes
import datetime as dt
import hashlib
import json
import math
import os
from pathlib import Path
import random
import selectors
import shutil
import statistics
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[2]
BASELINE = "6efdbce26bdcb879cb2c7bb4bdaaed8ea11ea075"
END = dt.datetime(2026, 11, 1, 19, 43, 43, tzinfo=dt.timezone.utc).timestamp()
LIMIT = 114
BODY = b'{"event":"PreToolUse","hook_event_name":"PreToolUse","session_id":"00000000-0000-4000-8000-000000000002","cwd":"/synthetic","tool_name":"write","tool_input":{},"tool_use_id":"call-1"}'
ENV = dict(os.environ, GOPROXY="off", GOSUMDB="off", GOTOOLCHAIN="local", CGO_ENABLED="1")


def write_new(path, value):
    with open(path, "x", opener=lambda p, f: os.open(p, f, 0o600)) as output:
        json.dump(value, output, sort_keys=True)
        output.write("\n")


def load(path):
    return json.loads(path.read_text())


def rows(directory):
    path = directory / "attempts.jsonl"
    return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []


def save_attempt(directory, result):
    with open(directory / "attempts.jsonl", "a", opener=lambda p, f: os.open(p, f, 0o600)) as output:
        output.write(json.dumps(result, sort_keys=True) + "\n")
        output.flush()
        os.fsync(output.fileno())


def command(args, cwd=None):
    subprocess.run(args, cwd=cwd, env=ENV, check=True, timeout=60,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def artifact(path, flags):
    return {"sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            "bytes": path.stat().st_size, "flags": flags}


def prepare(directory):
    if sys.platform != "darwin" or os.environ.get("GOFLAGS") or os.environ.get("GODEBUG"):
        raise RuntimeError("unsupported platform or ambient build/debug options")
    directory.mkdir(mode=0o700)
    source = directory / "source"
    command(["git", "worktree", "add", "--detach", str(source), BASELINE], ROOT)
    command(["go", "build", "-o", str(directory / "baseline"), "./cmd/ganesh"], source)
    command(["cc", "-x", "c", "-dynamiclib", "-o", str(directory / "clock.dylib"),
             str(ROOT / "integration/capture/process_start.c.fixture")])
    # Only the diagnostic artifact contains these marks and the trace descriptor.
    original = (source / "cmd/ganesh/capture.go").read_text()
    replacements = {
        "\tstart, err := processclock.Start()": "\tstartupMark(1)\n\tstart, err := processclock.Start()\n\tstartupMark(2)",
        "\tcase result := <-ready:": "\tcase result := <-ready:\n\t\tstartupMark(3)",
        "\treceipt, err := (capture.Spool{Directory: directory}).Append(ctx, metadata)": "\treceipt, err := (capture.Spool{Directory: directory}).Append(ctx, metadata)\n\tstartupMark(4)",
        "\tif !emitReceipt {": "\tstartupMark(5)\n\tif !emitReceipt {",
    }
    for before, after in replacements.items():
        if original.count(before) != 1:
            raise RuntimeError("diagnostic source binding changed")
        original = original.replace(before, after)
    (directory / "capture_trace.go").write_text(original)
    main = (source / "cmd/ganesh/main.go").read_text()
    binding = "func commandExitCode(args []string, execute func() error, diagnostic io.Writer) int {"
    if main.count(binding) != 1:
        raise RuntimeError("diagnostic dispatcher binding changed")
    (directory / "main_trace.go").write_text(main.replace(binding, binding + "\n\tdefer startupFlush()"))
    overlay = {"Replace": {
        str(source / "cmd/ganesh/capture.go"): str(directory / "capture_trace.go"),
        str(source / "cmd/ganesh/main.go"): str(directory / "main_trace.go"),
        str(source / "cmd/ganesh/startup_trace.go"): str(ROOT / "integration/capture/startup_trace.go.fixture"),
    }}
    write_new(directory / "overlay.json", overlay)
    command(["go", "build", "-overlay", str(directory / "overlay.json"), "-o",
             str(directory / "trace"), "./cmd/ganesh"], source)
    write_new(directory / "plan.json", {
        "baseline": BASELINE, "attempt_cap": LIMIT, "deadline_utc": END,
        "toolchain": subprocess.check_output(["go", "version"], env=ENV, text=True).strip(),
        "platform": os.uname().sysname + " " + os.uname().machine,
        "artifacts": {name: artifact(directory / name, []) for name in ["baseline", "trace"]},
        "cache_state": "uncontrolled", "seed": 34,
    })
    print("Prepared baseline and separate instrumented artifact; no attempts run", flush=True)


class Clock:
    def __init__(self, directory):
        self.lib = ctypes.CDLL(str(directory / "clock.dylib"))
        self.lib.ganesh_observe_start.argtypes = [ctypes.c_int,
            ctypes.POINTER(ctypes.c_longlong), ctypes.POINTER(ctypes.c_longlong)]
        self.lib.ganesh_observe_start.restype = ctypes.c_int

    def start_ns(self, pid):
        seconds, micros = ctypes.c_longlong(), ctypes.c_longlong()
        if self.lib.ganesh_observe_start(pid, ctypes.byref(seconds), ctypes.byref(micros)):
            return None
        return seconds.value * 1_000_000_000 + micros.value * 1000


def record_state(spool):
    if not spool.exists():
        return "absent"
    files = list(spool.glob("*.json"))
    if not files:
        return "partial_artifacts"
    if len(files) != 1 or files[0].stat().st_size > 65536:
        return "invalid"
    try:
        data = load(files[0])
        if (data["state"], data["origin"], data["coverage"], data["metadata"]["event"],
                data["metadata"]["outcome"]) == ("captured_local", "unknown", "incomplete", "tool_attempt", "unknown"):
            return "present_unknown_incomplete"
    except (ValueError, KeyError):
        pass
    return "invalid"


def attempt(directory, clock, executable, stage, arm, category, block, gap=None):
    existing = rows(directory)
    if len(existing) >= LIMIT or time.time() >= END:
        raise RuntimeError("frozen attempt/time ceiling reached")
    spool = directory / ("spool-%03d" % len(existing))
    trace = arm == "trace"
    trace_read, trace_write = os.pipe() if trace else (None, None)
    env = dict(ENV)
    if trace:
        env["GANESH_TEST_STARTUP_FD"] = str(trace_write)
    start = time.monotonic()
    try:
        child = subprocess.Popen([str(executable), "hook", "crush", "--spool", str(spool)],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env,
            pass_fds=(trace_write,) if trace else ())
    except OSError:
        if trace:
            os.close(trace_read)
            os.close(trace_write)
        result = {"attempt": len(existing), "stage": stage, "arm": arm, "category": category,
            "block": block, "parent_ms": (time.monotonic() - start) * 1000,
            "os_to_exit_ms": None, "finished_ns": time.time_ns(), "exit": None,
            "launch_failed": True, "success": False, "killed": False, "record": "absent",
            "output_bytes": {"stdout": 0, "stderr": 0}, "idle_gap_s": gap, "load": os.getloadavg()}
        save_attempt(directory, result)
        return result
    os_start = clock.start_ns(child.pid)
    if trace:
        os.close(trace_write)
    try:
        child.stdin.write(BODY)
        child.stdin.close()
    except BrokenPipeError:
        # An early deadline rejection is an outcome, not a missing attempt.
        try:
            child.stdin.close()
        except BrokenPipeError:
            pass
    selector = selectors.DefaultSelector()
    for kind, pipe in [("stdout", child.stdout), ("stderr", child.stderr)]:
        selector.register(pipe, selectors.EVENT_READ, kind)
    if trace:
        selector.register(trace_read, selectors.EVENT_READ, "trace")
    counts, trace_body = {"stdout": 0, "stderr": 0}, bytearray()
    killed = False
    while selector.get_map() or child.poll() is None:
        if child.poll() is None and time.monotonic() - start >= 1:
            child.kill()
            killed = True
        for key, _ in selector.select(0.005):
            value = os.read(key.fd, 8192)
            if not value:
                selector.unregister(key.fileobj)
                continue
            if key.data == "trace":
                if len(trace_body) + len(value) > 4096:
                    child.kill()
                    killed = True
                else:
                    trace_body.extend(value)
            else:
                counts[key.data] += len(value)
    child.wait()
    finish_ns, elapsed = time.time_ns(), (time.monotonic() - start) * 1000
    selector.close()
    child.stdout.close()
    child.stderr.close()
    if trace:
        os.close(trace_read)
    state = record_state(spool)
    result = {"attempt": len(existing), "stage": stage, "arm": arm, "category": category,
        "block": block, "parent_ms": elapsed,
        "os_to_exit_ms": (finish_ns - os_start) / 1e6 if os_start else None,
        "finished_ns": finish_ns, "output_bytes": counts, "exit": child.returncode,
        "killed": killed, "parent_overrun": elapsed > 1000, "record": state,
        "load": os.getloadavg(), "idle_gap_s": gap}
    result["success"] = child.returncode == 0 and not killed and elapsed <= 1000 and not any(counts.values()) and state == "present_unknown_incomplete"
    if trace_body:
        try:
            marks = json.loads(trace_body)
            if len(marks) != 7 or any(type(value) is not int for value in marks):
                raise ValueError()
            result["trace_ns"] = marks
            result["startup_ms"] = (marks[0] - os_start) / 1e6 if os_start else None
            result["ready_ms"] = (marks[5] - os_start) / 1e6 if os_start and marks[5] else None
            result["phase_ms"] = [(marks[i + 1] - marks[i]) / 1e6 if marks[i] and marks[i + 1] else None for i in range(6)]
        except (ValueError, TypeError):
            result["trace_invalid"] = True
    save_attempt(directory, result)
    return result


def pair(directory, clock, arm, stage, block, keep=False):
    executable = directory / ("copy-%s-%s-%d" % (stage, arm, block))
    with executable.open("xb") as output, (directory / arm).open("rb") as original:
        shutil.copyfileobj(original, output)
    executable.chmod(0o500)
    attempt(directory, clock, executable, stage, arm, "new_file_first", block)
    last = attempt(directory, clock, executable, stage, arm, "warm", block)
    if not keep:
        executable.unlink()
    return executable, last


def diagnose(directory):
    if rows(directory):
        raise RuntimeError("do not overwrite/repeat partially completed measurements")
    write_new(directory / "runner.json", {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
        for path in [Path(__file__).resolve(), ROOT / "integration/capture/process_start.c.fixture", ROOT / "integration/capture/startup_trace.go.fixture", ROOT / "docs/s0-capture-startup.md"]})
    clock = Clock(directory)
    for arm, count in [("baseline", 10), ("trace", 5)]:
        for block in range(count):
            pair(directory, clock, arm, "diagnosis", block)
        print("Completed diagnosis: " + arm, flush=True)
    write_new(directory / "diagnosis.done", {"attempts": 30})


def compare(directory):
    if len(rows(directory)) != 30 or not (directory / "diagnosis.done").exists():
        raise RuntimeError("diagnosis incomplete or comparison already attempted")
    traced = [r for r in rows(directory) if r["arm"] == "trace" and r.get("ready_ms") and r.get("startup_ms") is not None]
    if not traced or statistics.median(r["startup_ms"] / r["ready_ms"] for r in traced) < 0.5:
        raise RuntimeError("startup dominance gate not met; no candidate selected")
    write_new(directory / "decision.json", {"candidate": "strip_symbols", "basis": "measured startup dominates ready interval"})
    command(["go", "build", "-ldflags=-s -w", "-o", str(directory / "candidate"), "./cmd/ganesh"], directory / "source")
    write_new(directory / "candidate.json", artifact(directory / "candidate", ["-ldflags=-s -w"]))
    order = [False] * 10 + [True] * 10
    random.Random(34).shuffle(order)
    clock, idle = Clock(directory), {}
    for block, reverse in enumerate(order):
        for arm in (["candidate", "baseline"] if reverse else ["baseline", "candidate"]):
            executable, last = pair(directory, clock, arm, "comparison", block, block == 19)
            if block == 19:
                idle[arm] = executable, last
        if block % 5 == 4:
            print("Completed comparison blocks: %d/20" % (block + 1), flush=True)
    for block in range(2):
        for arm in ["baseline", "candidate"]:
            executable, last = idle[arm]
            gap = (time.time_ns() - last["finished_ns"]) / 1e9
            time.sleep(max(0, min(30, 30 - gap)))
            gap = (time.time_ns() - last["finished_ns"]) / 1e9
            last = attempt(directory, clock, executable, "comparison", arm, "idle_30s", block, gap)
            idle[arm] = executable, last
            print("Completed idle observation: " + arm, flush=True)
    write_new(directory / "comparison.done", {"attempts": 114})
    command(["git", "worktree", "remove", str(directory / "source")], ROOT)


def percentile(values, q):
    return sorted(values)[math.ceil(len(values) * q) - 1] if values else None


def summary(directory):
    results, table = rows(directory), []
    groups = sorted(set((r["stage"], r["arm"], r["category"]) for r in results))
    for stage, arm, category in groups:
        group = [r for r in results if (r["stage"], r["arm"], r["category"]) == (stage, arm, category)]
        times = [r["parent_ms"] for r in group]
        spans = [r["os_to_exit_ms"] for r in group if r["os_to_exit_ms"] is not None]
        table.append({"stage": stage, "arm": arm, "category": category, "n": len(group),
            "failures": sum(not r["success"] for r in group),
            "parent_median_ms": statistics.median(times), "parent_p95_ms": percentile(times, .95), "parent_p99_ms": percentile(times, .99),
            "os_p95_ms": percentile(spans, .95), "os_p99_ms": percentile(spans, .99),
            "os_missing": len(group) - len(spans)})
    print(json.dumps({"attempts": len(results), "groups": table}, indent=2))


def main():
    if len(sys.argv) != 3 or sys.argv[1] not in ("prepare", "diagnose", "compare", "summary"):
        raise RuntimeError("invalid measurement command")
    mode, directory = sys.argv[1], Path(sys.argv[2]).resolve()
    if mode != "summary" and time.time() >= END:
        raise RuntimeError("frozen effort window expired")
    if mode != "prepare" and (directory.stat().st_mode & 0o777 != 0o700 or directory.is_symlink()):
        raise RuntimeError("measurement directory must be private")
    {"prepare": prepare, "diagnose": diagnose, "compare": compare, "summary": summary}[mode](directory)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        # Do not format subprocess errors, child output, paths or environments.
        print("startup investigation incomplete: " + type(error).__name__, file=sys.stderr)
        raise SystemExit(1)
