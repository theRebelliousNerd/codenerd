#!/usr/bin/env python3
"""Inject an agent's live shared MEMORY.md during Codex SubagentStart."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path


MAX_MEMORY_BYTES = 256 * 1024
AGENT_TYPE_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$")


def repository_root() -> Path:
    return Path(__file__).resolve().parents[2]


def memory_path(root: Path, agent_type: str) -> Path | None:
    if not AGENT_TYPE_PATTERN.fullmatch(agent_type):
        return None

    memory_root = (root / ".claude" / "agent-memory").resolve()
    candidates = [agent_type]
    hyphenated = agent_type.replace("_", "-")
    if hyphenated != agent_type:
        candidates.append(hyphenated)

    for candidate in candidates:
        path = (memory_root / candidate / "MEMORY.md").resolve()
        if memory_root not in path.parents:
            continue
        if path.is_file():
            return path
    return None


def hook_output(payload: object, root: Path) -> dict:
    if not isinstance(payload, dict):
        return {}
    agent_type = payload.get("agent_type")
    if not isinstance(agent_type, str):
        return {}

    path = memory_path(root, agent_type)
    if path is None:
        return {}
    size = path.stat().st_size
    if size > MAX_MEMORY_BYTES:
        return {
            "systemMessage": (
                f"Agent memory was not injected because MEMORY.md is {size} bytes, "
                f"above the {MAX_MEMORY_BYTES}-byte safety limit."
            )
        }

    try:
        content = path.read_bytes().decode("utf-8-sig")
    except (OSError, UnicodeError) as exc:
        return {"systemMessage": f"Agent memory could not be loaded: {exc}"}
    if not content.strip():
        return {}

    return {
        "hookSpecificOutput": {
            "hookEventName": "SubagentStart",
            "additionalContext": content,
        }
    }


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, UnicodeError):
        print("{}")
        return 0
    print(json.dumps(hook_output(payload, repository_root()), ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
