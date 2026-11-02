from pathlib import Path
import tempfile
import time
import unittest
from unittest import mock

from observe_standalone import Budget, observe
from standalone_study import evaluate


def fixture():
    return [{"arm": arm, "category": category, "block": block,
        "parent_ms": 100 if arm == "baseline" else 50, "os_to_exit_ms": 90 if arm == "baseline" else 40,
        "success": True, "exit": 0, "record": "present_unknown_incomplete",
        "output_bytes": {"stdout": 0, "stderr": 0}}
        for arm in ["baseline", "candidate"]
        for category, count in [("new_file_first", 20), ("warm", 20), ("idle_30s", 2)]
        for block in range(count)]


class StandaloneRules(unittest.TestCase):
    def test_complete_signal_and_headroom_do_not_enable_a_profile(self):
        result = evaluate(fixture())
        self.assertTrue(result["local_signal"])
        self.assertTrue(result["headroom_probe"])
        self.assertFalse(result["capture_profile_enabled"])

    def test_controls_are_symmetric(self):
        for arm in ["baseline", "candidate"]:
            data = fixture()
            next(r for r in data if r["arm"] == arm)["output_bytes"]["stderr"] = 1
            self.assertFalse(evaluate(data)["local_signal"])
            self.assertFalse(evaluate(data)["headroom_probe"])

    def test_warm_tail_vetoes_an_improved_first_stratum(self):
        data = fixture()
        for r in data:
            if r["arm"] == "candidate" and r["category"] == "warm" and r["block"] >= 18:
                r["parent_ms"] = 200
        result = evaluate(data)
        self.assertGreater(result["paired_median_improvement"], .20)
        self.assertFalse(result["local_signal"])
        self.assertFalse(result["tail_gates_passed"])

    def test_missing_clock_and_adverse_headroom_are_not_a_pass(self):
        for span in [None, 601]:
            data = fixture()
            for r in data:
                if r["arm"] == "candidate" and r["category"] == "new_file_first":
                    r["os_to_exit_ms"] = span
            self.assertFalse(evaluate(data)["headroom_probe"])

    def test_partial_or_duplicate_sample_is_incomplete(self):
        data = fixture()
        self.assertEqual(evaluate(data[:-1])["status"], "incomplete")
        data[-1] = data[0]
        self.assertEqual(evaluate(data)["status"], "incomplete")

    def test_launch_failure_is_retained_without_contents(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            with mock.patch("observe_standalone.subprocess.Popen", side_effect=OSError("ghp_SYNTHETIC_SECRET")):
                result = observe(root, None, root / "missing", "baseline", "new_file_first", 0, Budget(time.time() + 60, 84))
            self.assertFalse(result["success"])
            self.assertTrue(result["launch_failed"])
            self.assertNotIn("SYNTHETIC_SECRET", (root / "attempts.jsonl").read_text())

    def test_budget_stops_before_launch(self):
        with tempfile.TemporaryDirectory() as temp, mock.patch("observe_standalone.subprocess.Popen") as launch:
            with self.assertRaises(RuntimeError):
                observe(Path(temp), None, Path(temp) / "unused", "baseline", "warm", 0, Budget(0, 84))
            launch.assert_not_called()
