#!/usr/bin/env python3
"""Validate one or more codeNERD architecture corpora against live evidence."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
ARCH = ROOT / "Docs" / "architecture"
REQUIRED = ("README.md", "IMPLEMENTED_SPEC.md", "TODO.md", "OPEN-QUESTIONS.md", "_progress.md")
SOURCE_PATH = re.compile(r"`((?:internal|cmd)/[A-Za-z0-9_./-]+)`")
SOURCE_RESIDUE = re.compile(r"\b(?:Storyworld|PageKit|Orval|GraphCAD|Marine Layer)\b", re.IGNORECASE)


def validate(corpus: Path) -> dict:
    errors: list[str] = []
    warnings: list[str] = []
    if not corpus.is_dir():
        return {"corpus": str(corpus), "valid": False, "errors": ["corpus directory does not exist"], "warnings": []}
    for name in REQUIRED:
        if not (corpus / name).is_file():
            errors.append(f"missing {name}")
    markdown = list(corpus.rglob("*.md"))
    if not markdown:
        errors.append("corpus contains no Markdown")
    for path in markdown:
        text = path.read_text(encoding="utf-8", errors="replace")
        if SOURCE_RESIDUE.search(text):
            errors.append(f"source-repository residue in {path.relative_to(corpus).as_posix()}")
    spec = corpus / "IMPLEMENTED_SPEC.md"
    if spec.exists():
        text = spec.read_text(encoding="utf-8", errors="replace")
        cited = [match.group(1).rstrip("/") for match in SOURCE_PATH.finditer(text)]
        resolvable = [path for path in cited if (ROOT / path).exists()]
        if not cited:
            warnings.append("IMPLEMENTED_SPEC.md contains no internal/cmd code citations")
        elif not resolvable:
            errors.append("IMPLEMENTED_SPEC.md code citations do not resolve")
    return {"corpus": str(corpus), "valid": not errors, "errors": errors, "warnings": warnings, "markdown_files": len(markdown)}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", action="append", type=Path, help="Corpus directory; repeatable")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    corpora = [path.resolve() for path in args.corpus] if args.corpus else [path for path in ARCH.iterdir() if path.is_dir()]
    results = [validate(path) for path in sorted(corpora)]
    payload = {"valid": all(item["valid"] for item in results), "corpora": results}
    if args.json:
        print(json.dumps(payload, indent=2))
    else:
        for item in results:
            print(f"{'PASS' if item['valid'] else 'FAIL'} {item['corpus']}")
            for error in item["errors"]:
                print(f"  ERROR: {error}")
            for warning in item["warnings"]:
                print(f"  WARNING: {warning}")
    return 0 if payload["valid"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
