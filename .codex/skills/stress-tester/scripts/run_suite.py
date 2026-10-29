#!/usr/bin/env python3
"""Preview or execute bounded deterministic codeNERD stress profiles."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

from repo_paths import find_repo_root


ROOT = find_repo_root(__file__)
REGISTRY = ROOT / ".codex" / "skills" / "stress-tester" / "references" / "profile-registry.json"


def load_registry() -> dict:
    with REGISTRY.open(encoding="utf-8") as handle:
        return json.load(handle)


def probe_text(argv: list[str]) -> str:
    try:
        result = subprocess.run(argv, cwd=ROOT, capture_output=True, text=True, timeout=20, check=False)
        return result.stdout.strip() if result.returncode == 0 else ""
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return ""


def render_command(argv: list[str], artifact_dir: Path) -> list[str]:
    rendered = [part.replace("{artifact_dir}", str(artifact_dir)) for part in argv]
    if rendered and rendered[0] == "python":
        rendered[0] = sys.executable
    return rendered


def uses_sqlite_vec(profile: dict) -> bool:
    """Whether a profile builds the production sqlite_vec variant."""
    for command in profile.get("commands", []):
        if command[:2] == ["go", "build"] and "./cmd/nerd" in command:
            if "-tags" in command:
                index = command.index("-tags")
                if index + 1 < len(command) and "sqlite_vec" in command[index + 1].split(","):
                    return True
            if any(part == "-tags=sqlite_vec" for part in command):
                return True
        if command and command[0] == "python" and any(part.endswith("verify_adversarial.py") for part in command) and "--execute" in command:
            return True
    return False


def command_environment(base_env: dict[str, str], repo_root: Path, sqlite_vec: bool) -> dict[str, str]:
    """Prepare the compiler environment for a selected profile."""
    env = base_env.copy()
    if sqlite_vec:
        env["CGO_ENABLED"] = "1"
        sqlite_headers = repo_root / "sqlite_headers"
        if sqlite_headers.exists():
            local_header_flag = f"-I{sqlite_headers.as_posix()}"
            flags = env.get("CGO_CFLAGS", "")
            if local_header_flag not in flags.split():
                env["CGO_CFLAGS"] = f"{local_header_flag} {flags}".strip()
    return env


def bounded_text(raw: bytes, limit: int) -> tuple[str, bool]:
    truncated = len(raw) > limit
    if truncated:
        raw = raw[:limit]
    return raw.decode("utf-8", errors="replace"), truncated


def run_command(
    argv: list[str],
    timeout: int,
    max_output: int,
    env: dict[str, str],
    command_dir: Path,
    ordinal: int,
) -> dict:
    started = datetime.now(timezone.utc)
    started_clock = time.monotonic()
    timed_out = False
    blocked = False
    try:
        result = subprocess.run(
            argv,
            cwd=ROOT,
            env=env,
            capture_output=True,
            timeout=timeout,
            check=False,
        )
        exit_code = result.returncode
        stdout_raw, stderr_raw = result.stdout, result.stderr
    except FileNotFoundError as exc:
        exit_code = None
        stdout_raw, stderr_raw = b"", str(exc).encode()
        blocked = True
    except subprocess.TimeoutExpired as exc:
        exit_code = None
        stdout_raw = exc.stdout or b""
        stderr_raw = exc.stderr or b""
        timed_out = True

    duration = time.monotonic() - started_clock
    stdout, stdout_truncated = bounded_text(stdout_raw, max_output)
    stderr, stderr_truncated = bounded_text(stderr_raw, max_output)
    stem = f"{ordinal:03d}"
    stdout_path = command_dir / f"{stem}.stdout.txt"
    stderr_path = command_dir / f"{stem}.stderr.txt"
    stdout_path.write_text(stdout, encoding="utf-8")
    stderr_path.write_text(stderr, encoding="utf-8")
    receipt = {
        "argv": argv,
        "started_at": started.isoformat(),
        "duration_seconds": round(duration, 3),
        "exit_code": exit_code,
        "timed_out": timed_out,
        "blocked": blocked,
        "stdout": str(stdout_path.relative_to(command_dir.parent)),
        "stderr": str(stderr_path.relative_to(command_dir.parent)),
        "stdout_truncated": stdout_truncated,
        "stderr_truncated": stderr_truncated,
    }
    (command_dir / f"{stem}.json").write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
    return receipt


def run_preflight(artifact_dir: Path) -> dict:
    """Run the repository preflight and persist a machine-readable gate."""
    receipt_path = artifact_dir / "preflight.json"
    stderr_path = artifact_dir / "preflight.stderr.txt"
    argv = [sys.executable, str(Path(__file__).with_name("preflight.py")), "--output", str(receipt_path)]
    try:
        result = subprocess.run(
            argv,
            cwd=ROOT,
            capture_output=True,
            timeout=30,
            check=False,
        )
        stderr_path.write_bytes(result.stderr)
        try:
            payload = json.loads(receipt_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            return {
                "ready": False,
                "exit_code": result.returncode,
                "receipt": str(receipt_path.relative_to(artifact_dir)),
                "error": f"preflight receipt unreadable: {exc}",
            }
        return {
            "ready": result.returncode == 0 and bool(payload.get("ready")),
            "exit_code": result.returncode,
            "receipt": str(receipt_path.relative_to(artifact_dir)),
            "blockers": payload.get("blockers", []),
        }
    except (FileNotFoundError, subprocess.TimeoutExpired) as exc:
        stderr_path.write_text(str(exc), encoding="utf-8")
        return {
            "ready": False,
            "exit_code": None,
            "receipt": str(receipt_path.relative_to(artifact_dir)),
            "error": str(exc),
        }


def write_summary(run: dict, artifact_dir: Path) -> None:
    lines = [
        "# codeNERD stress profile receipt",
        "",
        f"- Verdict: **{run['verdict'].upper()}**",
        f"- Profile: `{run['profile']}`",
        f"- Repetitions: {run['repeat']}",
        f"- Commands completed: {len(run['commands'])}",
        f"- Preflight ready: {run['preflight']['ready']}",
        f"- Wall-clock budget: {run['max_wall_seconds']} seconds",
        f"- Started: {run['started_at']}",
        "",
        "| # | Exit | Seconds | Command |",
        "|---:|---:|---:|---|",
    ]
    for index, item in enumerate(run["commands"], 1):
        exit_value = "timeout" if item["timed_out"] else ("blocked" if item["blocked"] else item["exit_code"])
        command = " ".join(item["argv"]).replace("|", "\\|")
        lines.append(f"| {index} | {exit_value} | {item['duration_seconds']} | `{command}` |")
    (artifact_dir / "summary.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    registry = load_registry()
    profiles = registry["profiles"]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--profile", choices=sorted(profiles), default="smoke")
    parser.add_argument("--list-profiles", action="store_true")
    parser.add_argument("--execute", action="store_true", help="Execute; default is a dry-run preview")
    parser.add_argument("--repeat", type=int, default=1)
    parser.add_argument("--keep-going", action="store_true")
    parser.add_argument("--artifact-dir", type=Path)
    parser.add_argument(
        "--max-wall-seconds",
        type=int,
        help="Lower the profile's registered whole-run timeout; cannot raise it",
    )
    args = parser.parse_args()

    if args.list_profiles:
        for name, profile in profiles.items():
            print(f"{name:12} {profile['risk']:14} {profile['description']}")
        return 0
    if not 1 <= args.repeat <= 100:
        parser.error("--repeat must be between 1 and 100")

    profile = profiles[args.profile]
    registered_wall_limit = int(profile.get("max_wall_seconds", registry["defaults"]["max_wall_seconds"]))
    if args.max_wall_seconds is not None and not 1 <= args.max_wall_seconds <= registered_wall_limit:
        parser.error(f"--max-wall-seconds must be between 1 and {registered_wall_limit} for this profile")
    max_wall_seconds = args.max_wall_seconds or registered_wall_limit

    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
    artifact_dir = (args.artifact_dir or ROOT / ".nerd" / "campaigns" / "stress-tester" / f"{stamp}-{args.profile}").resolve()
    commands = [render_command(command, artifact_dir) for command in profile["commands"]]

    if not args.execute:
        print(json.dumps({
            "verdict": "dry_run",
            "profile": args.profile,
            "risk": profile["risk"],
            "repeat": args.repeat,
            "max_wall_seconds": max_wall_seconds,
            "artifact_dir": str(artifact_dir),
            "commands": commands,
        }, indent=2))
        return 0

    if artifact_dir.exists() and any(artifact_dir.iterdir()):
        parser.error(f"artifact directory is not empty; refusing to overwrite receipts: {artifact_dir}")
    artifact_dir.mkdir(parents=True, exist_ok=True)
    command_dir = artifact_dir / "commands"
    command_dir.mkdir(exist_ok=True)
    env = command_environment(os.environ, ROOT, uses_sqlite_vec(profile))

    defaults = registry["defaults"]
    timeout = int(profile.get("timeout_seconds", defaults["timeout_seconds"]))
    max_output = int(defaults["max_output_bytes"])
    stop_on_failure = bool(defaults["stop_on_failure"]) and not args.keep_going
    git_status_text = probe_text(["git", "status", "--short"])
    git_status = git_status_text.splitlines()
    run = {
        "schema_version": 1,
        "profile": args.profile,
        "risk": profile["risk"],
        "repeat": args.repeat,
        "started_at": datetime.now(timezone.utc).isoformat(),
        "repository": str(ROOT),
        "profile_description": profile["description"],
        "command_timeout_seconds": timeout,
        "max_wall_seconds": max_wall_seconds,
        "max_output_bytes_per_stream": max_output,
        "registry_sha256": hashlib.sha256(REGISTRY.read_bytes()).hexdigest(),
        "git_commit": probe_text(["git", "rev-parse", "HEAD"]),
        "git_dirty_entry_count": len(git_status),
        "git_status_sha256": hashlib.sha256(git_status_text.encode("utf-8")).hexdigest(),
        "go_version": probe_text(["go", "version"]),
        "commands": [],
        "verdict": "running",
    }
    run["preflight"] = run_preflight(artifact_dir)
    run_path = artifact_dir / "run.json"
    run_path.write_text(json.dumps(run, indent=2) + "\n", encoding="utf-8")

    if not run["preflight"]["ready"]:
        run["verdict"] = "blocked"
        run["completed_at"] = datetime.now(timezone.utc).isoformat()
        run["artifact_dir"] = str(artifact_dir)
        run_path.write_text(json.dumps(run, indent=2) + "\n", encoding="utf-8")
        write_summary(run, artifact_dir)
        print(f"verdict=blocked artifact_dir={artifact_dir}")
        return 2

    ordinal = 0
    stop = False
    run_clock = time.monotonic()
    for iteration in range(1, args.repeat + 1):
        for command in commands:
            remaining = max_wall_seconds - (time.monotonic() - run_clock)
            if remaining <= 0:
                run["verdict"] = "failed"
                run["wall_budget_exhausted"] = True
                stop = True
                break
            ordinal += 1
            command_timeout = min(timeout, max(1, int(remaining)))
            receipt = run_command(command, command_timeout, max_output, env, command_dir, ordinal)
            receipt["iteration"] = iteration
            run["commands"].append(receipt)
            if receipt["blocked"]:
                run["verdict"] = "blocked"
                stop = stop_on_failure
            elif receipt["timed_out"] or receipt["exit_code"] != 0:
                run["verdict"] = "failed"
                stop = stop_on_failure
            print(f"[{ordinal}] exit={receipt['exit_code']} timeout={receipt['timed_out']} {' '.join(command)}")
            run_path.write_text(json.dumps(run, indent=2) + "\n", encoding="utf-8")
            if stop:
                break
        if stop:
            break

    if run["verdict"] == "running":
        run["verdict"] = "passed"
    run["completed_at"] = datetime.now(timezone.utc).isoformat()
    run["artifact_dir"] = str(artifact_dir)
    run_path.write_text(json.dumps(run, indent=2) + "\n", encoding="utf-8")
    write_summary(run, artifact_dir)
    print(f"verdict={run['verdict']} artifact_dir={artifact_dir}")
    return 0 if run["verdict"] == "passed" else 2


if __name__ == "__main__":
    raise SystemExit(main())
