from __future__ import annotations

import importlib.util
import json
import re
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path
from unittest import mock


SKILL_ROOT = Path(__file__).resolve().parents[1]
SCRIPT_ROOT = SKILL_ROOT / "scripts"
sys.path.insert(0, str(SCRIPT_ROOT))


def load_script(name: str):
    path = SKILL_ROOT / "scripts" / name
    spec = importlib.util.spec_from_file_location(f"stress_{path.stem}", path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
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
        race = registry["profiles"]["race"]
        self.assertEqual(race["timeout_seconds"], 1800)
        self.assertEqual(race["max_wall_seconds"], 3600)
        race_tests = [command for command in race["commands"] if command[:2] == ["go", "test"]]
        self.assertEqual(len(race_tests), 1)
        self.assertIn("-timeout=20m", race_tests[0])

    def test_adversarial_checker_is_always_built_from_current_source(self):
        verifier = load_script("verify_adversarial.py")
        with tempfile.TemporaryDirectory() as raw, mock.patch.object(verifier.subprocess, "run") as run:
            run.return_value.returncode = 0
            run.return_value.stderr = ""
            checker_path = Path(raw) / "nerd-check.exe"
            checker_path.write_bytes(b"current-checker")
            checker, build = verifier.build_checker(Path(raw))
            self.assertEqual(checker.parent, Path(raw))
            self.assertEqual(run.call_args.args[0][:4], ["go", "build", "-tags", "sqlite_vec"])
            self.assertEqual(run.call_args.kwargs["env"]["CGO_ENABLED"], "1")
            self.assertTrue(build["sha256"])

    def test_root_discovery_uses_repository_markers(self):
        paths = load_script("repo_paths.py")
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            (root / "go.mod").write_text("module fixture\n", encoding="utf-8")
            (root / "internal" / "logging").mkdir(parents=True)
            (root / "internal" / "logging" / "logger.go").write_text("package logging\n", encoding="utf-8")
            nested = root / "a" / "b"
            nested.mkdir(parents=True)
            anchor = nested / "tool.py"
            anchor.write_text("", encoding="utf-8")
            self.assertEqual(paths.find_repo_root(anchor), root)

    def test_log_parser_handles_live_formats_filtering_and_bounded_malformed_lines(self):
        parser = load_script("log_parser.py")
        with tempfile.TemporaryDirectory() as raw:
            path = Path(raw) / "run_kernel.log"
            before = int(datetime(2025, 12, 31, 23, 59, 59, tzinfo=timezone.utc).timestamp() * 1000)
            current = int(datetime(2026, 1, 1, 0, 0, 1, tzinfo=timezone.utc).timestamp() * 1000)
            records = [
                json.dumps({"ts": before, "cat": "kernel", "lvl": "error", "msg": "old panic"}),
                "not-json-and-not-a-live-log",
                "x" * 1100,
                json.dumps({"ts": current, "cat": "kernel", "lvl": "info", "msg": "assault started", "file": "kernel.go", "line": 7}),
                "2026/01/01 00:00:02.000000 [INFO] stress load complete",
            ]
            path.write_text("\n".join(records) + "\n", encoding="utf-8")
            entries = list(parser.parse_log_file(path, after=datetime(2026, 1, 1, tzinfo=timezone.utc), max_line_bytes=1024))
            self.assertEqual(len(entries), 2)
            self.assertEqual((entries[0].category, entries[0].message, entries[0].source_line), ("kernel", "assault started", 7))
            self.assertEqual((entries[1].category, entries[1].level, entries[1].message), ("kernel", "info", "stress load complete"))
            self.assertEqual((entries[0].line_number, entries[1].line_number), (4, 5))

    def test_analyzer_cli_runs_from_foreign_cwd_and_applies_time_window(self):
        script = SCRIPT_ROOT / "analyze_stress_logs.py"
        with tempfile.TemporaryDirectory() as raw:
            foreign = Path(raw) / "unrelated" / "cwd"
            logs = Path(raw) / "logs"
            foreign.mkdir(parents=True)
            logs.mkdir()
            before = int(datetime(2025, 12, 31, 23, 59, 59, tzinfo=timezone.utc).timestamp() * 1000)
            current = int(datetime(2026, 1, 1, 0, 0, 1, tzinfo=timezone.utc).timestamp() * 1000)
            (logs / "run_kernel.log").write_text(
                "\n".join([
                    json.dumps({"ts": before, "cat": "kernel", "lvl": "error", "msg": "panic: old event"}),
                    json.dumps({"ts": current, "cat": "kernel", "lvl": "info", "msg": "assault started"}),
                    "2026/01/01 00:00:02.000000 [INFO] stress load complete",
                ]) + "\n",
                encoding="utf-8",
            )
            filtered = foreign / "filtered.md"
            result = subprocess.run(
                [sys.executable, str(script), "--logs-dir", str(logs), "--after", "2026-01-01T00:00:00Z", "--output", str(filtered)],
                cwd=foreign,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            filtered_report = json.loads(filtered.with_suffix(".json").read_text(encoding="utf-8"))
            self.assertEqual((filtered_report["verdict"], filtered_report["entry_count"]), ("passed", 2))

            unfiltered = foreign / "unfiltered.md"
            result = subprocess.run(
                [sys.executable, str(script), "--logs-dir", str(logs), "--output", str(unfiltered)],
                cwd=foreign,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 2, result.stderr)
            unfiltered_report = json.loads(unfiltered.with_suffix(".json").read_text(encoding="utf-8"))
            self.assertEqual(unfiltered_report["critical_signals"]["panic"], 1)

    def test_adversarial_oracle_rejects_crash_and_unexpected_exit(self):
        verifier = load_script("verify_adversarial.py")
        self.assertEqual(verifier.classify_result(1, "ERROR in fixture.mg: failed to parse schema: parse error", "parse-fail"), (True, "expected_rejection"))
        self.assertEqual(verifier.classify_result(1, "ERROR in fixture.mg: failed to analyze schema: unsafe variable", "parse-fail"), (False, "unexpected_stage"))
        self.assertEqual(verifier.classify_result(1, "panic: parser exploded", "parse-fail"), (False, "checker_crash"))
        self.assertEqual(verifier.classify_result(1, "unknown command check-mangle", "analyze-fail"), (False, "unexpected_nonzero_exit"))
        self.assertEqual(verifier.classify_result(0, "OK: fixture.mg", "parse-fail"), (False, "accepted_invalid"))
        self.assertEqual(verifier.classify_result(0, "OK: fixture.mg", "clean"), (True, "expected_clean_acceptance"))
        self.assertEqual(verifier.classify_result(1, "ERROR in fixture.mg: unexpected", "clean"), (False, "rejected_clean_fixture"))
        self.assertEqual(verifier.classify_result(1, "ERROR in fixture.mg: unsupported error", "analyze-fail"), (False, "stage_not_observed"))

    def test_adversarial_manifest_matches_authoritative_go_stage_table(self):
        verifier = load_script("verify_adversarial.py")
        gate = SKILL_ROOT.parents[2] / "internal" / "mangle" / "corpus_gate_test.go"
        source = gate.read_text(encoding="utf-8")
        match = re.search(
            r"var adversarialExpectations = map\[string\]adversarialStage\{(.*?)\n\}",
            source,
            re.S,
        )
        self.assertIsNotNone(match, "authoritative Go adversarial stage table was not found")
        stage_names = {
            "stageParseFail": "parse-fail",
            "stageAnalyzeFail": "analyze-fail",
            "stageStratFail": "stratify-fail",
            "stageClean": "clean",
        }
        go_expectations = {
            fixture: stage_names[stage]
            for fixture, stage in re.findall(
                r'^\s*"([^\"]+)"\s*:\s*(stage\w+)\s*,?', match.group(1), re.M
            )
        }
        self.assertEqual(set(go_expectations.values()), set(stage_names.values()))
        self.assertEqual(verifier.inventory()["expected_stages"], go_expectations)

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

    def test_validator_cli_runs_from_foreign_cwd(self):
        with tempfile.TemporaryDirectory() as raw:
            foreign = Path(raw)
            output = foreign / "validation.json"
            result = subprocess.run(
                [sys.executable, str(SCRIPT_ROOT / "validate_skill.py"), "--output", str(output)],
                cwd=foreign,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertTrue(json.loads(output.read_text(encoding="utf-8"))["valid"])

    def test_preflight_and_profile_runner_run_from_foreign_cwd(self):
        with tempfile.TemporaryDirectory() as raw:
            foreign = Path(raw)
            preflight_output = foreign / "preflight.json"
            preflight = subprocess.run(
                [sys.executable, str(SCRIPT_ROOT / "preflight.py"), "--output", str(preflight_output)],
                cwd=foreign,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(preflight.returncode, 0, preflight.stderr + preflight.stdout)
            preflight_report = json.loads(preflight_output.read_text(encoding="utf-8"))
            self.assertTrue(preflight_report["ready"], preflight_report["blockers"])
            self.assertEqual(Path(preflight_report["repository"]), SKILL_ROOT.parents[2])

            preview = subprocess.run(
                [sys.executable, str(SCRIPT_ROOT / "run_suite.py"), "--profile", "structural"],
                cwd=foreign,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(preview.returncode, 0, preview.stderr + preview.stdout)
            preview_report = json.loads(preview.stdout)
            self.assertEqual(preview_report["verdict"], "dry_run")
            self.assertTrue(Path(preview_report["artifact_dir"]).is_relative_to(SKILL_ROOT.parents[2]))

    def test_production_profiles_use_sqlite_vec_and_enable_cgo(self):
        runner = load_script("run_suite.py")
        registry = json.loads((SKILL_ROOT / "references" / "profile-registry.json").read_text(encoding="utf-8"))
        self.assertTrue(runner.uses_sqlite_vec(registry["profiles"]["build"]))
        self.assertTrue(runner.uses_sqlite_vec(registry["profiles"]["smoke"]))
        self.assertTrue(runner.uses_sqlite_vec(registry["profiles"]["adversarial"]))
        env = runner.command_environment({"CGO_ENABLED": "0"}, runner.ROOT, sqlite_vec=True)
        self.assertEqual(env["CGO_ENABLED"], "1")
        if (runner.ROOT / "sqlite_headers").exists():
            self.assertIn("sqlite_headers", env["CGO_CFLAGS"])

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
        self.assertTrue(result["expectations_valid"], result["expectation_errors"])
        self.assertEqual(result["expected_stage_counts"], {
            "analyze-fail": 6,
            "clean": 6,
            "parse-fail": 11,
            "stratify-fail": 1,
        })
        self.assertEqual(sum(result["expected_stage_counts"].get(stage, 0) for stage in ("parse-fail", "analyze-fail", "stratify-fail")), 18)
        self.assertEqual(result["expected_stage_counts"]["clean"], 6)

    def test_adversarial_report_declares_static_only_scope(self):
        with tempfile.TemporaryDirectory() as raw:
            output = Path(raw) / "inventory.json"
            result = subprocess.run(
                [sys.executable, str(SCRIPT_ROOT / "verify_adversarial.py"), "--output", str(output)],
                cwd=raw,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["scope"], "static_only")
            self.assertFalse(report["runtime_hazards_exercised"])
            self.assertIn("executor termination containment", report["scope_note"])
            self.assertEqual(report["expected_stage_counts"]["clean"], 6)

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
