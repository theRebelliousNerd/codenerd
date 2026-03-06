#!/usr/bin/env python3
"""Deterministic readiness probes for codeNERD's codex exec backend."""

from __future__ import annotations

import argparse
import concurrent.futures
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any, Dict


PROBE_SCHEMA = {
    "type": "object",
    "additionalProperties": False,
    "required": ["probe_status", "schema_supported", "mode"],
    "properties": {
        "probe_status": {"type": "string", "enum": ["ok"]},
        "schema_supported": {"type": "boolean", "enum": [True]},
        "mode": {"type": "string", "enum": ["noninteractive"]},
    },
}


def repo_root() -> Path:
    path = Path(__file__).resolve()
    for parent in path.parents:
        if (parent / "go.mod").exists() and (parent / ".agents").exists():
            return parent
    raise RuntimeError("Unable to locate codeNERD workspace root from script path")


def skill_path(root: Path, skill_name: str) -> Path:
    return root / ".agents" / "skills" / skill_name / "SKILL.md"


def build_prompt(skill_name: str, use_skill: bool) -> str:
    prompt = (
        "CODEX_EXEC_PROBE: Return only JSON that sets probe_status to ok, "
        "schema_supported to true, and mode to noninteractive."
    )
    if use_skill:
        return f"${skill_name}\n\n{prompt}"
    return prompt


def run_codex_exec(root: Path, skill_name: str, use_skill: bool, model: str) -> Dict[str, Any]:
    with tempfile.TemporaryDirectory(prefix="codex-probe-") as tmp_dir:
        out_path = Path(tmp_dir) / "last-message.json"
        schema_path = Path(tmp_dir) / "schema.json"
        schema_path.write_text(json.dumps(PROBE_SCHEMA, indent=2), encoding="utf-8")

        cmd = [
            "codex",
            "exec",
            "-",
            "--model",
            model,
            "--sandbox",
            "read-only",
            "--color",
            "never",
            "--json",
            "--disable",
            "shell_tool",
            "--output-last-message",
            str(out_path),
            "--output-schema",
            str(schema_path),
        ]

        proc = subprocess.run(
            cmd,
            input=build_prompt(skill_name, use_skill),
            text=True,
            capture_output=True,
            cwd=str(root),
            check=False,
        )

        output = out_path.read_text(encoding="utf-8").strip() if out_path.exists() else ""
        parsed = None
        if output:
            try:
                parsed = json.loads(output)
            except json.JSONDecodeError:
                parsed = output

        return {
            "command": cmd,
            "returncode": proc.returncode,
            "stdout": proc.stdout.strip(),
            "stderr": proc.stderr.strip(),
            "output": parsed,
        }


def probe_auth(args: argparse.Namespace) -> Dict[str, Any]:
    root = repo_root()
    result = run_codex_exec(root, args.skill_name, args.use_skill, args.model)
    result["workspace_root"] = str(root)
    result["skill_path"] = str(skill_path(root, args.skill_name))
    result["skill_available"] = skill_path(root, args.skill_name).exists()
    return result


def probe_skill(args: argparse.Namespace) -> Dict[str, Any]:
    root = repo_root()
    path = skill_path(root, args.skill_name)
    return {
        "workspace_root": str(root),
        "skill_name": args.skill_name,
        "skill_path": str(path),
        "skill_available": path.exists(),
        "openai_yaml_available": (path.parent / "agents" / "openai.yaml").exists(),
    }


def probe_schema(args: argparse.Namespace) -> Dict[str, Any]:
    root = repo_root()
    result = run_codex_exec(root, args.skill_name, args.use_skill, args.model)
    result["schema_expected"] = PROBE_SCHEMA
    return result


def probe_concurrency(args: argparse.Namespace) -> Dict[str, Any]:
    root = repo_root()

    def task(_: int) -> Dict[str, Any]:
        return run_codex_exec(root, args.skill_name, args.use_skill, args.model)

    with concurrent.futures.ThreadPoolExecutor(max_workers=args.parallelism) as pool:
        results = list(pool.map(task, range(args.parallelism)))

    return {
        "workspace_root": str(root),
        "parallelism": args.parallelism,
        "successes": sum(1 for item in results if item["returncode"] == 0),
        "failures": sum(1 for item in results if item["returncode"] != 0),
        "results": results,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Probe codeNERD codex exec readiness")
    parser.add_argument("--skill-name", default="codenerd-codex-exec")
    parser.add_argument("--model", default="gpt-5.3-codex")
    parser.add_argument("--use-skill", action="store_true", default=False)

    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("auth")
    subparsers.add_parser("skill")
    subparsers.add_parser("schema")

    concurrency = subparsers.add_parser("concurrency")
    concurrency.add_argument("--parallelism", type=int, default=2)

    return parser.parse_args()


def main() -> int:
    args = parse_args()

    handlers = {
        "auth": probe_auth,
        "skill": probe_skill,
        "schema": probe_schema,
        "concurrency": probe_concurrency,
    }

    try:
        result = handlers[args.command](args)
    except Exception as exc:  # pragma: no cover - defensive CLI surface
        json.dump({"error": str(exc)}, sys.stdout, indent=2)
        sys.stdout.write("\n")
        return 1

    json.dump(result, sys.stdout, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
