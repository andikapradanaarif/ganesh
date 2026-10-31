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

REVISION = "aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f"


def main():
    if len(sys.argv) != 2:
        raise SystemExit("provide the pinned local Crush source directory")
    source = Path(sys.argv[1]).resolve(strict=True)
    root = Path(__file__).resolve().parents[2]
    revision = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=source, text=True
    ).strip()
    if revision != REVISION:
        raise SystemExit("Crush source revision does not match the inspected profile")
    if subprocess.run(["git", "diff", "--quiet", "HEAD", "--"], cwd=source).returncode:
        raise SystemExit("Crush tracked source has local changes")
    if (source / "internal/hooks/ganesh_adapter_test.go").exists():
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
    with tempfile.TemporaryDirectory(prefix="ganesh-crush-runner-") as temp:
        binary = str(Path(temp) / "ganesh")
        subprocess.run(
            ["go", "build", "-o", binary, "./cmd/ganesh"],
            cwd=root, env=env, check=True, timeout=60,
        )
        overlay = Path(temp) / "overlay.json"
        overlay.write_text(json.dumps({"Replace": {
            str(source / "internal/hooks/ganesh_adapter_test.go"):
                str(root / "integration/crush/runner_test.go.fixture")
        }}))
        env["GANESH_TEST_BINARY"] = binary
        subprocess.run(
            ["go", "test", "-count=1", "-overlay", str(overlay),
             "./internal/hooks", "-run", "^TestGaneshToolHookAdapter$", "-v"],
            cwd=source, env=env, check=True, timeout=60,
        )


if __name__ == "__main__":
    main()
