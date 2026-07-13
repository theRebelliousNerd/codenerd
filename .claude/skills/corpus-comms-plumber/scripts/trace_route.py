#!/usr/bin/env python3
"""Trace a codeNERD symbol or action across likely execution surfaces."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

SURFACES = (
    "internal/core",
    "internal/perception",
    "internal/articulation",
    "internal/session",
    "internal/shards",
    "internal/mcp",
    "internal/tools",
    "cmd/nerd",
)

TEXT_SUFFIXES = {".go", ".mg", ".md", ".toml", ".json", ".yaml", ".yml"}
SKIP_NAMES = {"debug_program_ERROR.mg"}


def scan(root: Path, needle: str, limit: int = 200) -> tuple[list[dict[str, object]], int]:
    matches: list[dict[str, object]] = []
    total = 0
    folded = needle.casefold()
    for surface in SURFACES:
        base = root / surface
        if not base.exists():
            continue
        for path in base.rglob("*"):
            if (
                not path.is_file()
                or path.suffix.lower() not in TEXT_SUFFIXES
                or path.name in SKIP_NAMES
            ):
                continue
            try:
                lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
            except OSError:
                continue
            for number, line in enumerate(lines, 1):
                if folded in line.casefold():
                    total += 1
                    if len(matches) < limit:
                        matches.append(
                            {
                                "path": path.relative_to(root).as_posix(),
                                "line": number,
                                "text": line.strip()[:300],
                                "surface": surface,
                            }
                        )
    return matches, total


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="backslashreplace")
    parser = argparse.ArgumentParser()
    parser.add_argument("needle", help="Action, predicate, type, function, or route fragment")
    parser.add_argument("--root", default=".", help="Repository root")
    parser.add_argument("--json", action="store_true")
    parser.add_argument("--limit", type=int, default=200, help="Maximum matches to emit")
    args = parser.parse_args()

    if not 1 <= args.limit <= 5000:
        parser.error("--limit must be between 1 and 5000")

    root = Path(args.root).resolve()
    matches, total = scan(root, args.needle, args.limit)
    payload = {
        "root": str(root),
        "needle": args.needle,
        "total_matches": total,
        "emitted_matches": len(matches),
        "truncated": total > len(matches),
        "matches": matches,
    }

    if args.json:
        print(json.dumps(payload, indent=2))
    else:
        if not matches:
            print(f"NO_MATCH: {args.needle}")
        for match in matches:
            print(f"{match['path']}:{match['line']}: {match['text']}")
        if total > len(matches):
            print(f"TRUNCATED: emitted {len(matches)} of {total} matches")

    return 0 if total else 1


if __name__ == "__main__":
    raise SystemExit(main())
