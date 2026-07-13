#!/usr/bin/env python3
"""Validate the active stress-tester package and Codex agent wiring."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
SKILL = ROOT / ".codex" / "skills" / "stress-tester" / "SKILL.md"
REGISTRY = SKILL.parent / "references" / "profile-registry.json"
AGENT = ROOT / ".codex" / "agents" / "stress-tester.toml"


def validate() -> dict:
    errors: list[str] = []
    warnings: list[str] = []
    required = [
        SKILL,
        SKILL.parent / "AGENTS.md",
        REGISTRY,
        SKILL.parent / "scripts" / "preflight.py",
        SKILL.parent / "scripts" / "run_suite.py",
        SKILL.parent / "scripts" / "analyze_stress_logs.py",
        SKILL.parent / "scripts" / "assert_receipt.py",
        SKILL.parent / "scripts" / "verify_adversarial.py",
        SKILL.parent / "references" / "artifact-contract.md",
        SKILL.parent / "references" / "testing-strategy.md",
        SKILL.parent / "references" / "workflows" / "README.md",
        SKILL.parent / "agents" / "openai.yaml",
        ROOT / ".codex" / "skills" / "log-analyzer" / "scripts" / "parse_log.py",
    ]
    for path in required:
        if not path.exists():
            errors.append(f"missing required path: {path.relative_to(ROOT)}")

    skill_text = SKILL.read_text(encoding="utf-8") if SKILL.exists() else ""
    if not re.search(r"(?ms)^---\s*\n.*?^name:\s*stress-tester\s*$.*?^description:\s*.+?^---\s*$", skill_text):
        errors.append("SKILL.md frontmatter is missing name or description")
    if len(skill_text.splitlines()) > 500:
        errors.append("SKILL.md exceeds 500 lines; move detail to references")
    frontmatter = skill_text.split("---", 2)[1] if skill_text.startswith("---") and skill_text.count("---") >= 2 else ""
    if re.search(r"(?m)^(?!name:|description:)[a-zA-Z_][\w-]*:", frontmatter):
        errors.append("SKILL.md frontmatter may contain only name and description")
    if (SKILL.parent / "CHANGELOG.md").exists():
        errors.append("CHANGELOG.md is auxiliary clutter; keep operational history in receipts")
    for banned in (".claude/skills/stress-tester", "/tmp/stress", "799 predicates"):
        if banned in skill_text:
            errors.append(f"active SKILL.md contains stale literal: {banned}")

    try:
        registry = json.loads(REGISTRY.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        errors.append(f"invalid profile registry: {exc}")
        registry = {"profiles": {}}
    profiles = registry.get("profiles", {})
    if len(profiles) < 5:
        errors.append("profile registry must define at least five pressure profiles")
    allowed_executables = {"go", "python"}
    defaults = registry.get("defaults", {})
    if not isinstance(defaults.get("max_wall_seconds"), int) or defaults.get("max_wall_seconds", 0) <= 0:
        errors.append("registry defaults require a positive max_wall_seconds")
    for name, profile in profiles.items():
        if not profile.get("description") or not profile.get("risk"):
            errors.append(f"profile {name} lacks description or risk")
        if not isinstance(profile.get("max_wall_seconds", defaults.get("max_wall_seconds")), int):
            errors.append(f"profile {name} has invalid max_wall_seconds")
        commands = profile.get("commands")
        if not isinstance(commands, list) or not commands:
            errors.append(f"profile {name} has no commands")
            continue
        for command in commands:
            if not isinstance(command, list) or not command or not all(isinstance(part, str) for part in command):
                errors.append(f"profile {name} has malformed argv")
                continue
            if command[0] not in allowed_executables:
                errors.append(f"profile {name} uses unapproved executable {command[0]}")
            if any(re.search(r"[;&|><]", part) for part in command):
                errors.append(f"profile {name} contains shell metacharacters")

    for script in (SKILL.parent / "scripts").glob("*.py"):
        try:
            compile(script.read_text(encoding="utf-8"), str(script), "exec")
        except (OSError, SyntaxError) as exc:
            errors.append(f"Python compile failed for {script.name}: {exc}")

    agent_text = AGENT.read_text(encoding="utf-8") if AGENT.exists() else ""
    if not AGENT.exists():
        errors.append("missing .codex/agents/stress-tester.toml")
    if 'path = "C:/CodeProjects/codeNERD/.codex/skills/stress-tester"' not in agent_text:
        errors.append("stress-tester agent is not attached to the upgraded .codex skill")
    if 'path = "C:/CodeProjects/codeNERD/.codex/skills/log-analyzer"' not in agent_text:
        errors.append("stress-tester agent is not attached to the native Codex log-analyzer skill")
    if not re.search(r'^model = "gpt-5\.6(?:-terra)?"$', agent_text, re.MULTILINE):
        warnings.append("stress-tester agent does not use the current gpt-5.6 family")

    workflow_count = len(list((SKILL.parent / "references" / "workflows").rglob("*.md"))) - 1
    adversarial_files = len(list((SKILL.parent / "assets" / "mangle-adversarial").rglob("*.mg")))
    if workflow_count <= 0:
        warnings.append("historical workflow library is empty")
    if adversarial_files <= 0:
        errors.append("Mangle adversarial corpus is empty")

    return {
        "schema_version": 1,
        "valid": not errors,
        "errors": errors,
        "warnings": warnings,
        "measurements": {
            "skill_lines": len(skill_text.splitlines()),
            "profile_count": len(profiles),
            "historical_workflow_count": max(workflow_count, 0),
            "adversarial_mangle_file_count": adversarial_files,
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    result = validate()
    rendered = json.dumps(result, indent=2)
    print(rendered)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered + "\n", encoding="utf-8")
    return 0 if result["valid"] else 2


if __name__ == "__main__":
    raise SystemExit(main())
