#!/usr/bin/env python3
"""Test a pinned local Crush source runner with synthetic events, offline.

Usage: python3 integration/crush/test_runner.py /path/to/crush/source
No source files, client settings, transcripts or installed binaries are changed.
"""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

BASELINE_REVISION = "aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f"
ENDING_FIX_REVISION = "d05cce5db2b09f3b4a89f3359b1953c96207e9b1"


def main():
    if len(sys.argv) != 2:
        raise SystemExit("provide the pinned local Crush source directory")
    source = Path(sys.argv[1]).resolve(strict=True)
    root = Path(__file__).resolve().parents[2]
    revision = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=source, text=True
    ).strip()
    if revision not in (BASELINE_REVISION, ENDING_FIX_REVISION):
        raise SystemExit("Crush source revision does not match the inspected profile")
    if subprocess.run(["git", "diff", "--quiet", "HEAD", "--"], cwd=source).returncode:
        raise SystemExit("Crush tracked source has local changes")
    replacements = {
        str(source / "internal/hooks/ganesh_adapter_test.go"):
            str(root / "integration/crush/runner_test.go.fixture"),
        str(source / "internal/agent/ganesh_turn_ending_test.go"):
            str(root / "integration/crush/turn_ending_test.go.fixture"),
    }
    if any(Path(target).exists() for target in replacements):
        raise SystemExit("integration overlay target already exists")
    untracked_go = subprocess.check_output(
        ["git", "ls-files", "--others", "--exclude-standard", "--", "*.go"],
        cwd=source, text=True,
    )
    if untracked_go:
        raise SystemExit("Crush source has untracked Go files")
    # Do not download dependencies or use a provider. Missing local Go modules
    # make this optional integration check incomplete, never a simulated pass.
    env = dict(os.environ, GOPROXY="off", GOSUMDB="off", GOTOOLCHAIN="local")
    env["GANESH_TEST_FIXED_ENDINGS"] = "1" if revision == ENDING_FIX_REVISION else "0"
    print(f"Source revision: {revision}; corrected ending dispatch: {env['GANESH_TEST_FIXED_ENDINGS']}", flush=True)
    with tempfile.TemporaryDirectory(prefix="ganesh-crush-runner-") as temp:
        binary = str(Path(temp) / "ganesh")
        started = time.monotonic()
        print("Building Ganesh fixture binary (60 s cap)", flush=True)
        subprocess.run(
            ["go", "build", "-o", binary, "./cmd/ganesh"],
            cwd=root, env=env, check=True, timeout=60,
        )
        print(f"Ganesh build completed in {time.monotonic() - started:.3f} s", flush=True)
        overlay = Path(temp) / "overlay.json"
        overlay.write_text(json.dumps({"Replace": replacements}))
        env["GANESH_TEST_BINARY"] = binary
        started = time.monotonic()
        print("Compiling/running pinned Crush fixture (60 s cap; test runtime 20 s)", flush=True)
        # Serialize packages so timed capture checks do not compete with other
        # package compilation/tests. Per-hook and process deadlines stay fixed.
        subprocess.run(
            ["go", "test", "-p=1", "-count=1", "-overlay", str(overlay),
             "./internal/hooks", "./internal/agent", "-run", "^TestGanesh(ToolHookAdapter|TurnEndingCancellation)$", "-timeout=20s", "-v"],
            cwd=source, env=env, check=True, timeout=60,
        )
        print(f"Crush fixture stage completed in {time.monotonic() - started:.3f} s", flush=True)


if __name__ == "__main__":
    main()
