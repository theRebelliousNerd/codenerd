#!/usr/bin/env python3
"""Inventory and verify documented Mangle corpus stage expectations."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import subprocess
import tempfile
import time
from collections import Counter
from pathlib import Path

from repo_paths import find_repo_root


ROOT = find_repo_root(__file__)
CORPUS = ROOT / ".codex" / "skills" / "stress-tester" / "assets" / "mangle-adversarial"
EXPECTATIONS = CORPUS / "expected-stages.json"
EXPECTED_REJECTION = re.compile(r"(?m)^ERROR in .+:")
EXPECTED_SUCCESS = re.compile(r"(?m)^OK: .+")
CRITICAL_FAILURE = re.compile(
    r"(?:^|\s)panic(?::|\s|$)|fatal error:|runtime error:|out of memory|"
    r"concurrent map (?:read|write|iteration)|warning: data race",
    re.IGNORECASE | re.MULTILINE,
)
STAGE_MARKERS = (
    ("stratify-fail", re.compile(r"stratification failed:", re.IGNORECASE)),
    ("analyze-fail", re.compile(r"failed to analyze schema:", re.IGNORECASE)),
    ("parse-fail", re.compile(r"failed to parse schema:", re.IGNORECASE)),
)


def inventory() -> dict:
    files = sorted(CORPUS.rglob("*.mg"))
    categories = Counter(path.relative_to(CORPUS).parts[0] for path in files)
    error_markers = 0
    correct_markers = 0
    for path in files:
        text = path.read_text(encoding="utf-8", errors="replace")
        error_markers += text.count("# ERROR:")
        correct_markers += text.count("# CORRECT:")
    expected_stages, expectation_errors = load_expected_stages(files)
    return {
        "file_count": len(files),
        "categories": dict(sorted(categories.items())),
        "error_markers": error_markers,
        "correct_markers": correct_markers,
        "expected_stages": expected_stages,
        "expected_stage_counts": dict(sorted(Counter(expected_stages.values()).items())),
        "expectations_valid": not expectation_errors,
        "expectation_errors": expectation_errors,
        "files": files,
    }


def load_expected_stages(files: list[Path]) -> tuple[dict[str, str], list[str]]:
    """Expand the harness oracle and require exact fixture coverage."""
    try:
        document = json.loads(EXPECTATIONS.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return {}, [f"could not read expected-stage manifest: {exc}"]
    if not isinstance(document, dict) or document.get("schema_version") != 1 or not isinstance(document.get("stages"), dict):
        return {}, ["expected-stage manifest has an unsupported schema"]

    expected: dict[str, str] = {}
    errors: list[str] = []
    allowed_stages = {"parse-fail", "analyze-fail", "stratify-fail", "clean"}
    for stage, patterns in document["stages"].items():
        if stage not in allowed_stages or not isinstance(patterns, list):
            errors.append(f"invalid expected-stage entry: {stage}")
            continue
        for pattern in patterns:
            if not isinstance(pattern, str) or ".." in Path(pattern).parts:
                errors.append(f"unsafe expected-stage pattern: {pattern!r}")
                continue
            matches = [path for path in CORPUS.glob(pattern) if path.is_file() and path.suffix == ".mg"]
            if not matches:
                errors.append(f"expected-stage pattern matched no fixtures: {pattern}")
            for path in matches:
                key = path.relative_to(CORPUS).as_posix()
                if key in expected:
                    errors.append(f"fixture is classified more than once: {key}")
                expected[key] = stage

    actual = {path.relative_to(CORPUS).as_posix() for path in files}
    for key in sorted(actual - expected.keys()):
        errors.append(f"fixture has no expected stage: {key}")
    for key in sorted(expected.keys() - actual):
        errors.append(f"expected-stage manifest names missing fixture: {key}")
    return dict(sorted(expected.items())), errors


def build_checker(temp_dir: Path) -> tuple[Path, dict]:
    checker = temp_dir / "nerd-check.exe"
    env = os.environ.copy()
    env["CGO_ENABLED"] = "1"
    headers = ROOT / "sqlite_headers"
    if headers.exists():
        local_header_flag = f"-I{headers.as_posix()}"
        flags = env.get("CGO_CFLAGS", "")
        if local_header_flag not in flags.split():
            env["CGO_CFLAGS"] = f"{local_header_flag} {flags}".strip()
    started = time.monotonic()
    result = subprocess.run(
        ["go", "build", "-tags", "sqlite_vec", "-o", str(checker), "./cmd/nerd"],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=300,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(f"failed to build checker: {result.stderr[-2000:]}")
    return checker, {
        "duration_seconds": round(time.monotonic() - started, 3),
        "sha256": hashlib.sha256(checker.read_bytes()).hexdigest(),
        "argv": ["go", "build", "-tags", "sqlite_vec", "-o", "<isolated-checker>", "./cmd/nerd"],
    }


def classify_result(
    exit_code: int | None,
    output: str,
    expected_stage: str,
    timed_out: bool = False,
) -> tuple[bool, str]:
    if timed_out:
        return False, "timeout"
    if CRITICAL_FAILURE.search(output):
        return False, "checker_crash"
    if expected_stage not in {"parse-fail", "analyze-fail", "stratify-fail", "clean"}:
        return False, "unknown_expected_stage"
    observed = observed_stage(output)
    if expected_stage == "clean":
        if exit_code == 0 and observed == "clean":
            return True, "expected_clean_acceptance"
        if exit_code != 0 and EXPECTED_REJECTION.search(output):
            return False, "rejected_clean_fixture"
        return False, "unexpected_clean_result"
    if exit_code == 0 and observed == "clean":
        return False, "accepted_invalid"
    if exit_code != 0 and EXPECTED_REJECTION.search(output) and observed == expected_stage:
        return True, "expected_rejection"
    if observed in {"parse-fail", "analyze-fail", "stratify-fail"}:
        return False, "unexpected_stage"
    if exit_code == 0:
        return False, "accepted_invalid"
    if EXPECTED_REJECTION.search(output):
        return False, "stage_not_observed"
    return False, "unexpected_nonzero_exit"


def observed_stage(output: str) -> str | None:
    """Return a CLI-observable pipeline stage, requiring its real diagnostic."""
    for stage, marker in STAGE_MARKERS:
        if marker.search(output):
            return stage
    if EXPECTED_SUCCESS.search(output) and not EXPECTED_REJECTION.search(output):
        return "clean"
    return None


def execute(files: list[Path], timeout: int, expected_stages: dict[str, str]) -> dict:
    results = []
    with tempfile.TemporaryDirectory(prefix="codenerd-mangle-check-") as raw_temp:
        checker, build = build_checker(Path(raw_temp))
        for path in files:
            started = time.monotonic()
            try:
                result = subprocess.run(
                    [str(checker), "check-mangle", str(path)],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    timeout=timeout,
                    check=False,
                )
                output = result.stdout + result.stderr
                relative = path.relative_to(CORPUS).as_posix()
                expected_stage = expected_stages[relative]
                passed, classification = classify_result(result.returncode, output, expected_stage)
                results.append({
                    "file": relative,
                    "expected_stage": expected_stage,
                    "observed_stage": observed_stage(output),
                    "passed": passed,
                    "rejected": expected_stage != "clean" and passed,
                    "classification": classification,
                    "exit_code": result.returncode,
                    "duration_seconds": round(time.monotonic() - started, 3),
                    "output_bytes": len(output.encode("utf-8", errors="replace")),
                    "output_sha256": hashlib.sha256(output.encode("utf-8", errors="replace")).hexdigest(),
                    "output_head": output[:1000],
                    "output_tail": output[-1000:],
                })
            except subprocess.TimeoutExpired as exc:
                output = ((exc.stdout or "") + (exc.stderr or "")) if isinstance(exc.stdout, str) else ""
                relative = path.relative_to(CORPUS).as_posix()
                expected_stage = expected_stages[relative]
                passed, classification = classify_result(None, output, expected_stage, timed_out=True)
                results.append({
                    "file": relative,
                    "expected_stage": expected_stage,
                    "observed_stage": observed_stage(output),
                    "passed": passed,
                    "rejected": expected_stage != "clean" and passed,
                    "classification": classification,
                    "exit_code": None,
                    "timed_out": True,
                    "duration_seconds": round(time.monotonic() - started, 3),
                    "output_bytes": len(output.encode("utf-8", errors="replace")),
                    "output_sha256": hashlib.sha256(output.encode("utf-8", errors="replace")).hexdigest(),
                    "output_head": output[:1000],
                    "output_tail": output[-1000:],
                })
    return {"build": build, "results": results}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--execute", action="store_true", help="Build the production checker and verify every documented stage")
    parser.add_argument("--timeout", type=int, default=20, help="Per-fixture timeout in seconds")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if args.timeout <= 0:
        parser.error("--timeout must be positive")
    stats = inventory()
    files = stats.pop("files")
    expected_stages = stats["expected_stages"]
    report = {
        "schema_version": 1,
        "scope": "static_only",
        "runtime_hazards_exercised": False,
        "scope_note": "Checks CLI parse, analysis, stratification, and clean static loading. Runtime hazard execution and executor termination containment are separate and unexercised by this verifier.",
        "valid_inventory": bool(files) and stats["error_markers"] > 0 and stats["expectations_valid"],
        **stats,
    }
    exit_code = 0 if report["valid_inventory"] else 2
    if args.execute:
        if not report["valid_inventory"]:
            report["execution"] = {"blocked": True, "error": "fixture inventory or expected-stage manifest is invalid"}
        else:
            try:
                execution = execute(files, args.timeout, expected_stages)
                results = execution["results"]
                failed = [item for item in results if not item["passed"]]
                report["execution"] = {
                    "expected": "18 documented static failures reject at their exact stage; 6 runtime-hazard examples pass static scaffolding checks; crashes, timeouts, and stage mismatches fail",
                    "scope": "static_only",
                    "runtime_hazards_exercised": False,
                    "build": execution["build"],
                    "checked": len(results),
                    "passed": sum(item["passed"] for item in results),
                    "static_rejections": sum(item["rejected"] for item in results),
                    "clean_static_acceptances": sum(item["classification"] == "expected_clean_acceptance" for item in results),
                    "failed_or_unexpected": failed,
                    "results": results,
                }
                if failed:
                    exit_code = 2
            except (RuntimeError, FileNotFoundError, subprocess.TimeoutExpired) as exc:
                report["execution"] = {"blocked": True, "error": str(exc)}
                exit_code = 3
    rendered = json.dumps(report, indent=2)
    print(rendered)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered + "\n", encoding="utf-8")
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
