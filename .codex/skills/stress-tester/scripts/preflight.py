#!/usr/bin/env python3
"""Read-only preflight for codeNERD stress testing."""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]


def probe(argv: list[str], timeout: int = 10) -> dict[str, object]:
    try:
        result = subprocess.run(
            argv,
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
        output = (result.stdout or result.stderr).strip().splitlines()
        return {
            "available": True,
            "exit_code": result.returncode,
            "version": output[0][:300] if output else "",
        }
    except FileNotFoundError:
        return {"available": False, "exit_code": None, "version": ""}
    except subprocess.TimeoutExpired:
        return {"available": True, "exit_code": None, "version": "probe timed out"}


def collect() -> dict[str, object]:
    required = [
        "go.mod",
        "cmd/nerd",
        "internal/core",
        "internal/mangle",
        "internal/core/defaults/schemas.mg",
        ".codex/skills/stress-tester/references/profile-registry.json",
    ]
    paths = {path: (ROOT / path).exists() for path in required}
    tools = {
        "python": probe([sys.executable, "--version"]),
        "go": probe(["go", "version"]),
        "git": probe(["git", "--version"]),
    }

    git_status = probe(["git", "status", "--short"], timeout=20)
    try:
        raw_status = subprocess.run(
            ["git", "status", "--short"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=20,
            check=False,
        ).stdout.splitlines()
    except (FileNotFoundError, subprocess.TimeoutExpired):
        raw_status = []

    logs_dir = ROOT / ".nerd" / "logs"
    log_files = list(logs_dir.glob("*.log")) if logs_dir.exists() else []
    disk = shutil.disk_usage(ROOT)
    debug_dump = ROOT / "debug_program_ERROR.mg"

    blockers = []
    if not all(paths.values()):
        blockers.append("required repository paths are missing")
    if not tools["go"]["available"]:
        blockers.append("Go toolchain is unavailable")
    if not tools["python"]["available"]:
        blockers.append("Python is unavailable")
    if disk.free < 2 * 1024**3:
        blockers.append("less than 2 GiB free disk space")

    return {
        "schema_version": 1,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "repository": str(ROOT),
        "ready": not blockers,
        "blockers": blockers,
        "required_paths": paths,
        "tools": tools,
        "git": {
            "probe": git_status,
            "dirty_entry_count": len(raw_status),
            "dirty_entries_preview": raw_status[:20],
        },
        "runtime": {
            "nerd_exe_present": (ROOT / "nerd.exe").exists(),
            "sqlite_header_present": (ROOT / "sqlite_headers" / "sqlite3.h").exists(),
            "log_file_count": len(log_files),
            "debug_dump_present": debug_dump.exists(),
            "free_disk_bytes": disk.free,
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="Optional JSON receipt path")
    args = parser.parse_args()
    result = collect()
    rendered = json.dumps(result, indent=2)
    print(rendered)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered + "\n", encoding="utf-8")
    return 0 if result["ready"] else 2


if __name__ == "__main__":
    raise SystemExit(main())
