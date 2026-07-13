from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SKILL_ROOT = Path(__file__).resolve().parents[1]


def load_script(name: str):
    path = SKILL_ROOT / "scripts" / name
    spec = importlib.util.spec_from_file_location(f"stress_{path.stem}", path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class StressToolingTests(unittest.TestCase):
    def test_registry_is_argv_based_and_bounded(self):
        registry = json.loads((SKILL_ROOT / "references" / "profile-registry.json").read_text(encoding="utf-8"))
        self.assertGreaterEqual(len(registry["profiles"]), 5)
        self.assertGreater(registry["defaults"]["timeout_seconds"], 0)
        self.assertGreater(registry["defaults"]["max_wall_seconds"], 0)
        for profile in registry["profiles"].values():
            self.assertTrue(profile["commands"])
            for command in profile["commands"]:
                self.assertIsInstance(command, list)
                self.assertIn(command[0], {"go", "python"})

    def test_adversarial_checker_is_always_built_from_current_source(self):
        verifier = load_script("verify_adversarial.py")
        with tempfile.TemporaryDirectory() as raw, mock.patch.object(verifier.subprocess, "run") as run:
            run.return_value.returncode = 0
            run.return_value.stderr = ""
            checker_path = Path(raw) / "nerd-check.exe"
            checker_path.write_bytes(b"current-checker")
            checker, build = verifier.build_checker(Path(raw))
            self.assertEqual(checker.parent, Path(raw))
            self.assertEqual(run.call_args.args[0][:3], ["go", "build", "-o"])
            self.assertTrue(build["sha256"])

    def test_adversarial_oracle_rejects_crash_and_unexpected_exit(self):
        verifier = load_script("verify_adversarial.py")
        self.assertEqual(verifier.classify_result(1, "ERROR in fixture.mg: parse error"), (True, "expected_rejection"))
        self.assertEqual(verifier.classify_result(1, "panic: parser exploded"), (False, "checker_crash"))
        self.assertEqual(verifier.classify_result(1, "unknown command check-mangle"), (False, "unexpected_nonzero_exit"))
        self.assertEqual(verifier.classify_result(0, ""), (False, "accepted_invalid"))

    def test_adversarial_receipt_contract_names_head_tail_and_hash(self):
        source = (SKILL_ROOT / "scripts" / "verify_adversarial.py").read_text(encoding="utf-8")
        for field in ('"output_head"', '"output_tail"', '"output_sha256"', '"output_bytes"'):
            self.assertIn(field, source)

    def test_runner_uses_current_python_interpreter(self):
        runner = load_script("run_suite.py")
        rendered = runner.render_command(["python", "tool.py", "{artifact_dir}"], Path("receipt"))
        self.assertEqual(rendered[0], runner.sys.executable)
        self.assertEqual(rendered[2], "receipt")

    def test_package_validator_passes(self):
        validator = load_script("validate_skill.py")
        result = validator.validate()
        self.assertTrue(result["valid"], result["errors"])

    def test_log_analyzer_distinguishes_failure_and_no_signal(self):
        analyzer = load_script("analyze_stress_logs.py")
        with tempfile.TemporaryDirectory() as raw:
            logs = Path(raw)
            empty = analyzer.analyze(logs, None, 10)
            self.assertEqual(empty["verdict"], "no_signal")
            (logs / "2026-07-13_kernel.log").write_text(
                "2026/07/13 10:00:00.000001 [INFO] assault campaign started\n"
                "2026/07/13 10:00:01.000001 [ERROR] panic: concurrent map writes\n",
                encoding="utf-8",
            )
            failed = analyzer.analyze(logs, None, 10)
            self.assertEqual(failed["verdict"], "failed")
            self.assertEqual(failed["critical_signals"]["panic"], 1)

    def test_adversarial_inventory_is_measured(self):
        verifier = load_script("verify_adversarial.py")
        result = verifier.inventory()
        self.assertGreater(result["file_count"], 0)
        self.assertGreater(result["error_markers"], 0)

    def test_receipt_assertions_detect_critical_output(self):
        assertion = load_script("assert_receipt.py")
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            commands = root / "commands"
            commands.mkdir()
            (commands / "001.stdout.txt").write_text("panic: deliberate fixture\n", encoding="utf-8")
            (commands / "001.stderr.txt").write_text("", encoding="utf-8")
            receipt = root / "run.json"
            receipt.write_text(json.dumps({
                "verdict": "passed",
                "commands": [{
                    "argv": ["go", "test", "./internal/core/..."],
                    "stdout": "commands/001.stdout.txt",
                    "stderr": "commands/001.stderr.txt",
                }],
            }), encoding="utf-8")
            result = assertion.evaluate(receipt, {"passed"}, ["internal/core"], [], True)
            self.assertFalse(result["valid"])
            self.assertEqual(len(result["critical_matches"]), 1)


if __name__ == "__main__":
    unittest.main()
