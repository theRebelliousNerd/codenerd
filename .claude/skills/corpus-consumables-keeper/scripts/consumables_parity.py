#!/usr/bin/env python3
"""Audit codeNERD Codex agent registrations, skill attachments, and duplicate skills."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import tomllib
from pathlib import Path


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def audit(root: Path) -> dict[str, object]:
    errors: list[str] = []
    warnings: list[str] = []
    config_path = root / ".codex" / "config.toml"
    agents_dir = root / ".codex" / "agents"

    if not config_path.exists():
        errors.append("missing .codex/config.toml")
        config: dict[str, object] = {}
    else:
        try:
            config = tomllib.loads(config_path.read_text(encoding="utf-8"))
        except (OSError, tomllib.TOMLDecodeError) as exc:
            errors.append(f"invalid .codex/config.toml: {exc}")
            config = {}

    registered = config.get("agents", {}) if isinstance(config, dict) else {}
    if not isinstance(registered, dict):
        registered = {}

    registered_paths: set[Path] = set()
    for name, entry in registered.items():
        if not isinstance(entry, dict) or "config_file" not in entry:
            continue
        target = (root / ".codex" / str(entry["config_file"])).resolve()
        registered_paths.add(target)
        if not target.exists():
            errors.append(f"agent {name}: missing config_file {target}")
            continue
        try:
            payload = tomllib.loads(target.read_text(encoding="utf-8"))
        except (OSError, tomllib.TOMLDecodeError) as exc:
            errors.append(f"agent {name}: invalid TOML: {exc}")
            continue
        for field in ("name", "description", "developer_instructions"):
            if not isinstance(payload.get(field), str) or not payload[field].strip():
                errors.append(f"agent {name}: missing required {field}")
        if payload.get("name") != name:
            errors.append(f"agent {name}: TOML name is {payload.get('name')!r}")

    skill_path_re = re.compile(r'^path\s*=\s*["\'](.+?)["\']\s*$', re.MULTILINE)
    if agents_dir.exists():
        for agent in sorted(agents_dir.glob("*.toml")):
            if agent.resolve() not in registered_paths:
                errors.append(f"unregistered custom agent: {agent.name}")
            text = agent.read_text(encoding="utf-8", errors="replace")
            for raw in skill_path_re.findall(text):
                path = Path(raw)
                target = path if path.is_absolute() else (agent.parent / path)
                if not target.exists():
                    errors.append(f"{agent.name}: missing attached skill {raw}")

    agents_skills = root / ".agents" / "skills"
    codex_skills = root / ".codex" / "skills"
    divergent: list[str] = []
    if agents_skills.exists() and codex_skills.exists():
        left = {p.name: p for p in agents_skills.iterdir() if p.is_dir()}
        right = {p.name: p for p in codex_skills.iterdir() if p.is_dir()}
        for name in sorted(left.keys() & right.keys()):
            lskill = left[name] / "SKILL.md"
            rskill = right[name] / "SKILL.md"
            if lskill.exists() and rskill.exists() and digest(lskill) != digest(rskill):
                divergent.append(name)
        if divergent:
            warnings.append(
                "same-name .agents/.codex skills diverge: " + ", ".join(divergent)
            )

    return {
        "root": str(root),
        "registered_agents": sum(
            1 for value in registered.values() if isinstance(value, dict) and "config_file" in value
        ),
        "errors": errors,
        "warnings": warnings,
        "divergent_duplicate_skills": divergent,
        "status": "FAIL" if errors else "PASS",
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    result = audit(Path(args.root).resolve())
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(f"STATUS={result['status']}")
        for item in result["errors"]:
            print(f"ERROR: {item}")
        for item in result["warnings"]:
            print(f"WARNING: {item}")
    return 1 if result["errors"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
