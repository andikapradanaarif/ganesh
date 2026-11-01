import contextlib
import io
import json
from pathlib import Path
import tempfile
import time
import unittest
from unittest import mock

import measure_startup as measure


class MeasurementAccounting(unittest.TestCase):
    def test_phase_gate_includes_early_rejections_without_ready(self):
        results = [
            {"arm": "trace", "startup_ms": 800, "trace_ns": [800000000, 800100000, 800200000, 0, 0, 0, 801000000]},
            {"arm": "trace", "startup_ms": 5, "trace_ns": [5000000, 6000000, 7000000, 8000000, 9000000, 10000000, 11000000]},
            {"arm": "trace", "exit": -9},
        ]
        fractions = measure.startup_fractions(results)
        self.assertEqual(len(fractions), 2)
        self.assertAlmostEqual(fractions[0], 800 / 801)
        self.assertEqual(fractions[1], 0.5)

    def test_private_creation_never_overwrites(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "plan.json"
            measure.write_new(path, {"frozen": True})
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            with self.assertRaises(FileExistsError):
                measure.write_new(path, {"frozen": False})
            self.assertEqual(measure.load(path), {"frozen": True})

    def test_launch_failure_remains_in_denominator_without_diagnostic_contents(self):
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(measure, "END", time.time() + 60):
            root = Path(directory)
            with mock.patch.object(measure.subprocess, "Popen", side_effect=OSError("ghp_SYNTHETIC_SECRET")):
                result = measure.attempt(root, None, root / "missing", "diagnosis", "baseline", "new_file_first", 0)
            self.assertFalse(result["success"])
            self.assertTrue(result["launch_failed"])
            self.assertIsNone(result["os_to_exit_ms"])
            self.assertEqual(len(measure.rows(root)), 1)
            self.assertNotIn("SYNTHETIC_SECRET", (root / "attempts.jsonl").read_text())
            self.assertEqual((root / "attempts.jsonl").stat().st_mode & 0o777, 0o600)

    def test_failed_tail_and_unknown_clock_are_not_dropped(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for number in range(20):
                measure.save_attempt(root, {"stage": "comparison", "arm": "baseline", "category": "new_file_first",
                    "parent_ms": number + 1 if number < 19 else 1000,
                    "os_to_exit_ms": number + 1 if number < 19 else None,
                    "success": number < 19})
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                measure.summary(root)
            result = json.loads(output.getvalue())
            group = result["groups"][0]
            self.assertEqual(result["attempts"], 20)
            self.assertEqual(group["n"], 20)
            self.assertEqual(group["failures"], 1)
            self.assertEqual(group["parent_p95_ms"], 19)
            self.assertEqual(group["parent_p99_ms"], 1000)
            self.assertEqual(group["os_missing"], 1)

    def test_time_cap_prevents_another_launch(self):
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(measure, "END", 0):
            with mock.patch.object(measure.subprocess, "Popen") as launch:
                with self.assertRaises(RuntimeError):
                    measure.attempt(Path(directory), None, Path(directory) / "unused", "diagnosis", "baseline", "warm", 0)
                launch.assert_not_called()
            self.assertEqual(measure.rows(Path(directory)), [])

    def test_attempt_cap_prevents_another_launch(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for number in range(measure.LIMIT):
                measure.save_attempt(root, {"attempt": number})
            with mock.patch.object(measure.subprocess, "Popen") as launch:
                with self.assertRaises(RuntimeError):
                    measure.attempt(root, None, root / "unused", "diagnosis", "baseline", "warm", 0)
                launch.assert_not_called()


if __name__ == "__main__":
    unittest.main()
