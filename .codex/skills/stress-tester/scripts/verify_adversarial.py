#!/usr/bin/env python3
"""Inventory, and optionally execute, the intentionally-invalid Mangle corpus."""

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


ROOT = Path(__file__).resolve().parents[4]
CORPUS = ROOT / ".codex" / "skills" / "stress-tester" / "assets" / "mangle-adversarial"
EXPECTED_REJECTION = re.compile(r"(?m)^ERROR in .+:")
CRITICAL_FAILURE = re.compile(
    r"(?:^|\s)panic(?::|\s|$)|fatal error:|runtime error:|out of memory|"
    r"concurrent map (?:read|write|iteration)|warning: data race",
    re.IGNORECASE | re.MULTILINE,
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
    return {
        "file_count": len(files),
        "categories": dict(sorted(categories.items())),
        "error_markers": error_markers,
        "correct_markers": correct_markers,
        "files": files,
    }


def build_checker(temp_dir: Path) -> tuple[Path, dict]:
    checker = temp_dir / "nerd-check.exe"
    env = os.environ.copy()
    headers = ROOT / "sqlite_headers"
    if headers.exists() and not env.get("CGO_CFLAGS"):
        env["CGO_CFLAGS"] = f"-I{headers.as_posix()}"
    started = time.monotonic()
    result = subprocess.run(
        ["go", "build", "-o", str(checker), "./cmd/nerd"],
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
        "argv": ["go", "build", "-o", "<isolated-checker>", "./cmd/nerd"],
    }


def classify_result(exit_code: int | None, output: str, timed_out: bool = False) -> tuple[bool, str]:
    if timed_out:
        return False, "timeout"
    if exit_code == 0:
        return False, "accepted_invalid"
    if CRITICAL_FAILURE.search(output):
        return False, "checker_crash"
    if not EXPECTED_REJECTION.search(output):
        return False, "unexpected_nonzero_exit"
    return True, "expected_rejection"


def execute(files: list[Path], timeout: int) -> dict:
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
                rejected, classification = classify_result(result.returncode, output)
                results.append({
                    "file": str(path.relative_to(CORPUS)),
                    "rejected": rejected,
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
                rejected, classification = classify_result(None, output, timed_out=True)
                results.append({
                    "file": str(path.relative_to(CORPUS)),
                    "rejected": rejected,
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
    parser.add_argument("--execute", action="store_true", help="Run check-mangle; inventory-only by default")
    parser.add_argument("--timeout", type=int, default=20, help="Per-fixture timeout in seconds")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    stats = inventory()
    files = stats.pop("files")
    report = {"schema_version": 1, "valid_inventory": bool(files) and stats["error_markers"] > 0, **stats}
    exit_code = 0 if report["valid_inventory"] else 2
    if args.execute:
        try:
            execution = execute(files, args.timeout)
            results = execution["results"]
            failed = [item for item in results if not item["rejected"]]
            report["execution"] = {
                "expected": "every intentionally-invalid file produces an expected checker rejection without crash or timeout",
                "build": execution["build"],
                "checked": len(results),
                "rejected": sum(item["rejected"] for item in results),
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
