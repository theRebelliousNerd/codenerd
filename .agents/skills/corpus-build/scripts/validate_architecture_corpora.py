#!/usr/bin/env python3
"""Validate codeNERD Docs/architecture spine corpora (code-grounded).

Exit 0 if acceptance structure holds; exit 1 with diagnostics otherwise.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

SPINE = [
    "core",
    "mangle",
    "perception",
    "articulation",
    "prompt",
    "session",
    "shards",
    "campaign",
    "config",
    "store",
    "tools",
    "cli",
]

REQUIRED = [
    "README.md",
    "00-ALIGNMENT-VISION-REVIEW.md",
    "01-DOMAIN-MODEL.md",
    "04-INVARIANTS-AND-GATES.md",
    "IMPLEMENTED_SPEC.md",
    "TODO.md",
    "_progress.md",
]

BAD_PATTERNS = re.compile(
    r"storyworld|pagekit|Orval|\bwormhole\b|no code exists yet|Overall: 0% complete",
    re.I,
)

PATH_RE = re.compile(r"`((?:internal|cmd)/[A-Za-z0-9_./-]+)`")


def find_root() -> Path:
    cwd = Path.cwd()
    for p in [cwd, *cwd.parents]:
        if (p / "go.mod").exists() and (p / "Docs" / "architecture").exists():
            return p
    return cwd


def main() -> int:
    root = find_root()
    arch = root / "Docs" / "architecture"
    errors: list[str] = []

    index = arch / "INDEX.md"
    if not index.exists():
        errors.append("missing Docs/architecture/INDEX.md")
    else:
        text = index.read_text(encoding="utf-8", errors="replace")
        if "seeded empty" in text.lower() and "Realized" not in text:
            errors.append("INDEX still placeholder-only")
        for name in SPINE:
            if f"]({name}/)" not in text and f"`{name}`" not in text and f"| {name} |" not in text:
                # allow link form [name](name/)
                if f"[{name}]" not in text:
                    errors.append(f"INDEX missing spine entry: {name}")

    journal = arch / "DARK-FACTORY-JOURNAL.md"
    if not journal.exists():
        errors.append("missing Docs/architecture/DARK-FACTORY-JOURNAL.md")

    for name in SPINE:
        base = arch / name
        if not base.is_dir():
            errors.append(f"missing corpus dir: {name}")
            continue
        for req in REQUIRED:
            if not (base / req).exists():
                errors.append(f"missing {name}/{req}")
        if not list(base.glob("02-CURRENT-STATE*")):
            errors.append(f"missing {name}/02-CURRENT-STATE*")
        if not list(base.glob("03-GAP-ANALYSIS*")):
            errors.append(f"missing {name}/03-GAP-ANALYSIS*")

        spec = base / "IMPLEMENTED_SPEC.md"
        if spec.exists():
            body = spec.read_text(encoding="utf-8", errors="replace")
            if BAD_PATTERNS.search(body):
                errors.append(f"honesty defect in {name}/IMPLEMENTED_SPEC.md")
            # At least one real internal/ or cmd/ path that exists
            ok_path = False
            for m in PATH_RE.finditer(body):
                rel = m.group(1).rstrip("/")
                if (root / rel).exists():
                    ok_path = True
                    break
            if not ok_path:
                errors.append(f"no resolvable internal/cmd path in {name}/IMPLEMENTED_SPEC.md")

    # Scan all md for Vectryx-only / false pre-impl
    for md in arch.rglob("*.md"):
        body = md.read_text(encoding="utf-8", errors="replace")
        if BAD_PATTERNS.search(body):
            # allow journal to mention wormhole only if... no, flag all
            rel = md.relative_to(root).as_posix()
            # false positive: "ported from Vectryx" is OK in skills not here
            if "storyworld" in body.lower() or "pagekit" in body.lower() or "orval" in body.lower():
                errors.append(f"Vectryx-only term in {rel}")
            if "no code exists yet" in body.lower() or "Overall: 0% complete" in body:
                errors.append(f"false pre-impl zeroing in {rel}")

    if errors:
        print("FAIL")
        for e in errors:
            print(f" - {e}")
        return 1
    print(f"PASS: {len(SPINE)} spine corpora OK under {arch}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
