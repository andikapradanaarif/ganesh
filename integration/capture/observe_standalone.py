"""Native launch observation reused for a separately frozen study.

The immutable issue-34 producer stays unchanged. This observer has an explicit
per-study budget; it never edits or bypasses the historical producer's caps.
"""

from dataclasses import dataclass
import os
import selectors
import subprocess
import time

from measure_startup import BODY, ENV, Clock, record_state, rows, save_attempt


@dataclass(frozen=True)
class Budget:
    deadline: float
    attempts: int

    def check(self, used):
        if time.time() >= self.deadline or used >= self.attempts:
            raise RuntimeError("frozen study ceiling reached")


def observe(directory, clock, executable, arm, category, block, budget, gap=None):
    stage = "standalone36"
    existing = rows(directory)
    budget.check(len(existing))
    spool = directory / ("spool-%03d" % len(existing))
    env = dict(ENV)
    start = time.monotonic()
    try:
        child = subprocess.Popen([str(executable), "hook", "crush", "--spool", str(spool)],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env,
            pass_fds=())
    except OSError:
        result = {"attempt": len(existing), "stage": stage, "arm": arm, "category": category,
            "block": block, "parent_ms": (time.monotonic() - start) * 1000,
            "os_to_exit_ms": None, "finished_ns": time.time_ns(), "exit": None,
            "launch_failed": True, "success": False, "killed": False, "record": "absent",
            "output_bytes": {"stdout": 0, "stderr": 0}, "idle_gap_s": gap, "load": os.getloadavg()}
        save_attempt(directory, result)
        return result
    os_start = clock.start_ns(child.pid)
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
    counts = {"stdout": 0, "stderr": 0}
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
            counts[key.data] += len(value)
    child.wait()
    finish_ns, elapsed = time.time_ns(), (time.monotonic() - start) * 1000
    selector.close()
    child.stdout.close()
    child.stderr.close()
    state = record_state(spool)
    result = {"attempt": len(existing), "stage": stage, "arm": arm, "category": category,
        "block": block, "parent_ms": elapsed,
        "os_to_exit_ms": (finish_ns - os_start) / 1e6 if os_start else None,
        "finished_ns": finish_ns, "output_bytes": counts, "exit": child.returncode,
        "killed": killed, "parent_overrun": elapsed > 1000, "record": state,
        "load": os.getloadavg(), "idle_gap_s": gap}
    result["success"] = child.returncode == 0 and not killed and elapsed <= 1000 and not any(counts.values()) and state == "present_unknown_incomplete"
    save_attempt(directory, result)
    return result
