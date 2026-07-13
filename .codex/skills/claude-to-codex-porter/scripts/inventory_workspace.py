#!/usr/bin/env python3
"""Inventory Claude Code and Codex workspace surfaces before a migration."""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
import tomllib
from pathlib import Path
from typing import Any


CLAUDE_SURFACES = (
    ".claude/agents",
    ".claude/commands",
    ".claude/hooks",
    ".claude/plugins",
    ".claude/prompts",
    ".claude/rules",
    ".claude/skills",
    ".claude/settings.json",
    ".claude/settings.local.json",
    ".claude/mcp.json",
)

CODEX_SURFACES = (
    "AGENTS.md",
    "AGENTS.override.md",
    ".agents/skills",
    ".agents/plugins/marketplace.json",
    ".codex/agents",
    ".codex/config.toml",
    ".codex/hooks.json",
    ".codex/rules",
    ".codex/skills",
    "plugins",
)


def file_digest(path: Path) -> str:
    """Return a SHA-256 digest for one file."""
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def surface_record(root: Path, relative: str) -> dict[str, Any]:
    """Describe one expected workspace surface without following it elsewhere."""
    path = root / relative
    record: dict[str, Any] = {
        "path": relative,
        "exists": path.exists(),
        "kind": "missing",
    }
    if path.is_file():
        record.update(kind="file", files=1, bytes=path.stat().st_size)
    elif path.is_dir():
        files = [item for item in path.rglob("*") if item.is_file()]
        record.update(
            kind="directory",
            files=len(files),
            bytes=sum(item.stat().st_size for item in files),
        )
    return record


def skill_inventory(root: Path, relative: str) -> dict[str, dict[str, Any]]:
    """Index direct child skill packages beneath a candidate skill root."""
    skills_root = root / relative
    if not skills_root.is_dir():
        return {}

    skills: dict[str, dict[str, Any]] = {}
    for child in sorted(skills_root.iterdir(), key=lambda item: item.name.lower()):
        skill_md = child / "SKILL.md"
        if child.is_dir() and skill_md.is_file():
            skills[child.name] = {
                "path": skill_md.relative_to(root).as_posix(),
                "sha256": file_digest(skill_md),
            }
    return skills


def configured_skills(config: dict[str, Any]) -> list[dict[str, Any]]:
    """Extract normalized skills.config entries from parsed Codex config."""
    entries = config.get("skills", {}).get("config", [])
    if not isinstance(entries, list):
        return []
    result = []
    for item in entries:
        if isinstance(item, dict) and isinstance(item.get("path"), str):
            result.append(
                {
                    "path": item["path"],
                    "enabled": item.get("enabled", True),
                }
            )
    return result


def custom_agent_inventory(root: Path, errors: list[str]) -> dict[str, dict[str, Any]]:
    """Parse repository custom-agent TOMLs without assuming they are registered."""
    agents_root = root / ".codex" / "agents"
    if not agents_root.is_dir():
        return {}
    agents: dict[str, dict[str, Any]] = {}
    for path in sorted(agents_root.glob("*.toml"), key=lambda item: item.name.lower()):
        try:
            payload = tomllib.loads(path.read_text(encoding="utf-8"))
        except (OSError, UnicodeError, tomllib.TOMLDecodeError) as exc:
            errors.append(f"cannot parse {path.relative_to(root).as_posix()}: {exc}")
            continue
        agents[path.stem] = {
            "path": path.relative_to(root).as_posix(),
            "name": payload.get("name"),
            "description": payload.get("description"),
            "model": payload.get("model"),
            "has_developer_instructions": bool(payload.get("developer_instructions")),
        }
    return agents


def configured_agents(config: dict[str, Any]) -> list[dict[str, str]]:
    """Extract normalized [agents.<name>] config_file registrations."""
    entries = config.get("agents", {})
    if not isinstance(entries, dict):
        return []
    result = []
    for name, item in sorted(entries.items()):
        if isinstance(item, dict) and isinstance(item.get("config_file"), str):
            result.append({"name": name, "config_file": item["config_file"]})
    return result


def build_report(root: Path) -> dict[str, Any]:
    """Build a deterministic report for the selected repository root."""
    root = root.resolve()
    errors: list[str] = []
    warnings: list[str] = []

    config_path = root / ".codex" / "config.toml"
    config: dict[str, Any] = {}
    if config_path.is_file():
        try:
            config = tomllib.loads(config_path.read_text(encoding="utf-8"))
        except (OSError, UnicodeError, tomllib.TOMLDecodeError) as exc:
            errors.append(f"cannot parse .codex/config.toml: {exc}")

    roots = {
        ".agents/skills": skill_inventory(root, ".agents/skills"),
        ".codex/skills": skill_inventory(root, ".codex/skills"),
    }
    duplicate_names = sorted(set(roots[".agents/skills"]) & set(roots[".codex/skills"]))
    divergent_duplicates = [
        name
        for name in duplicate_names
        if roots[".agents/skills"][name]["sha256"]
        != roots[".codex/skills"][name]["sha256"]
    ]
    if duplicate_names:
        warnings.append(
            f"{len(duplicate_names)} skill names exist in both repository roots; "
            "Codex does not merge duplicate packages"
        )
    if divergent_duplicates:
        warnings.append(
            f"{len(divergent_duplicates)} duplicate skill names have different SKILL.md content"
        )

    custom_agents = custom_agent_inventory(root, errors)
    registered_agents = configured_agents(config)
    registered_files = {
        Path(item["config_file"].replace("\\", "/")).name for item in registered_agents
    }
    unregistered_agents = sorted(
        name for name, item in custom_agents.items() if Path(item["path"]).name not in registered_files
    )
    incomplete_agents = sorted(
        name
        for name, item in custom_agents.items()
        if not item["name"] or not item["description"] or not item["has_developer_instructions"]
    )
    if unregistered_agents:
        warnings.append(f"{len(unregistered_agents)} custom agent TOMLs are not registered in .codex/config.toml")
    if incomplete_agents:
        warnings.append(f"{len(incomplete_agents)} custom agent TOMLs lack required identity/instructions fields")

    return {
        "root": str(root),
        "claude_surfaces": [surface_record(root, item) for item in CLAUDE_SURFACES],
        "codex_surfaces": [surface_record(root, item) for item in CODEX_SURFACES],
        "skill_roots": roots,
        "duplicate_skill_names": duplicate_names,
        "divergent_duplicate_skill_names": divergent_duplicates,
        "configured_skills": configured_skills(config),
        "custom_agents": custom_agents,
        "configured_agents": registered_agents,
        "unregistered_custom_agents": unregistered_agents,
        "incomplete_custom_agents": incomplete_agents,
        "errors": errors,
        "warnings": warnings,
    }


def print_human(report: dict[str, Any]) -> None:
    """Print a compact human-readable inventory."""
    print(f"root: {report['root']}")
    for label in ("claude_surfaces", "codex_surfaces"):
        print(f"\n{label}:")
        for item in report[label]:
            if item["exists"]:
                print(
                    f"  {item['path']}: {item['kind']} "
                    f"({item.get('files', 0)} files, {item.get('bytes', 0)} bytes)"
                )
    print("\nskill_roots:")
    for root_name, skills in report["skill_roots"].items():
        print(f"  {root_name}: {len(skills)}")
    print(f"  duplicate names: {len(report['duplicate_skill_names'])}")
    print(f"  divergent duplicates: {len(report['divergent_duplicate_skill_names'])}")
    print(f"configured skill entries: {len(report['configured_skills'])}")
    print(f"custom agent TOMLs: {len(report['custom_agents'])}")
    print(f"configured agent entries: {len(report['configured_agents'])}")
    print(f"unregistered custom agents: {len(report['unregistered_custom_agents'])}")
    print(f"incomplete custom agents: {len(report['incomplete_custom_agents'])}")
    for warning in report["warnings"]:
        print(f"warning: {warning}")
    for error in report["errors"]:
        print(f"error: {error}", file=sys.stderr)


def parse_args() -> argparse.Namespace:
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path.cwd())
    parser.add_argument("--json", action="store_true", dest="as_json")
    return parser.parse_args()


def main() -> int:
    """Run the inventory command."""
    args = parse_args()
    report = build_report(args.root)
    if args.as_json:
        print(json.dumps(report, indent=2, sort_keys=True))
    else:
        print_human(report)
    return 1 if report["errors"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
