#!/usr/bin/env python3
"""Synchronize corpus-build common references across governed Codex micro-skills."""

from __future__ import annotations

import argparse
import hashlib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
SKILLS = ROOT / ".codex" / "skills"
CANON = SKILLS / "corpus-build" / "references" / "common"
HEADER = "<!-- SYNCED from corpus-build/references/common/{name} sha256:{digest} -- DO NOT EDIT -->\n\n"


def render(name: str, text: str) -> str:
    return HEADER.format(name=name, digest=hashlib.sha256(text.encode()).hexdigest()[:12]) + text


def targets() -> list[Path]:
    return [
        path
        for path in sorted(SKILLS.glob("corpus-*"))
        if path.is_dir() and path.name != "corpus-build" and (path / "SKILL.md").exists()
    ]


def synchronize(check: bool) -> tuple[list[str], int]:
    if not CANON.is_dir():
        return [f"missing canonical common directory: {CANON}"], 0
    canon = {path.name: path.read_text(encoding="utf-8") for path in CANON.iterdir() if path.is_file()}
    drift: list[str] = []
    writes = 0
    for skill in targets():
        common = skill / "references" / "common"
        for name, text in canon.items():
            path = common / name
            expected = render(name, text)
            actual = path.read_text(encoding="utf-8", errors="replace") if path.exists() else None
            if actual != expected:
                drift.append(path.relative_to(ROOT).as_posix())
                if not check:
                    common.mkdir(parents=True, exist_ok=True)
                    path.write_text(expected, encoding="utf-8", newline="\n")
                    writes += 1
        if common.exists():
            for path in common.iterdir():
                if path.is_file() and path.name not in canon:
                    drift.append(path.relative_to(ROOT).as_posix() + " (orphan)")
    return drift, writes


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    drift, writes = synchronize(args.check)
    if args.check:
        print(f"targets={len(targets())} drift={len(drift)}")
        for path in drift:
            print(f"DRIFT: {path}")
        return 1 if drift else 0
    print(f"targets={len(targets())} updated={writes}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
