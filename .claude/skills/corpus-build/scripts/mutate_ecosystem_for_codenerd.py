#!/usr/bin/env python3
"""Deprecated write mutator; now performs a read-only source-residue audit.

The original port helper rewrote .agents/.claude trees and deleted/re-copied skill
directories. That behavior is unsafe in a shared checkout. Keep this filename only
for compatibility with old runbooks; it never writes.
"""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
ACTIVE = (
    ROOT / ".codex" / "skills" / "corpus-build",
    ROOT / ".codex" / "skills" / "arch-propose",
    ROOT / ".codex" / "agents",
)
TEXT_SUFFIXES = {".md", ".py", ".ps1", ".json", ".toml", ".yaml", ".yml"}
SKIP_NAMES = {
    "CHANGELOG.md",
    "journal.md",
    "source-parity.md",
    "pre-implementation-markers.md",
    "mutate_ecosystem_for_codenerd.py",
    "validate_architecture_corpora.py",
}
FORBIDDEN = re.compile(
    r"internal/testing/remediation|internal/app/server|internal/adktools|"
    r"web/dashboard|cmd/codenerd|\.grok/agents|gpt-5\.6-(?:sol|luna)|"
    r"\b(?:Storyworld|PageKit|Orval|GraphCAD|Marine Layer)\b",
    re.IGNORECASE,
)


def audit(root: Path = ROOT) -> list[dict[str, object]]:
    findings = []
    for base in ACTIVE:
        if not base.exists():
            continue
        for path in base.rglob("*"):
            if (
                not path.is_file()
                or path.suffix.lower() not in TEXT_SUFFIXES
                or path.name in SKIP_NAMES
                or "historical" in path.parts
                or "__pycache__" in path.parts
            ):
                continue
            for number, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
                if FORBIDDEN.search(line) and "has no `internal/testing/remediation`" not in line:
                    findings.append({
                        "path": path.relative_to(root).as_posix(),
                        "line": number,
                        "text": line.strip()[:300],
                    })
    return findings


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    findings = audit()
    result = {"status": "FAIL" if findings else "PASS", "write_mode": False, "findings": findings}
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(f"STATUS={result['status']} write_mode=false")
        for item in findings:
            print(f"{item['path']}:{item['line']}: {item['text']}")
    return 1 if findings else 0


if __name__ == "__main__":
    raise SystemExit(main())
