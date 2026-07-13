#!/usr/bin/env python3
"""Measure one architecture corpus against its live codeNERD source paths."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
SOURCE_PATH = re.compile(r"`((?:internal|cmd)/[A-Za-z0-9_./-]+)`")


def document_counts(corpus: Path) -> dict:
    files = list(corpus.rglob("*.md")) if corpus.exists() else []
    return {
        "markdown_files": len(files),
        "implemented_spec": (corpus / "IMPLEMENTED_SPEC.md").exists(),
        "todo": (corpus / "TODO.md").exists(),
        "open_questions": (corpus / "OPEN-QUESTIONS.md").exists(),
        "progress": (corpus / "_progress.md").exists(),
    }


def discover_sources(corpus: Path, explicit: list[str]) -> list[Path]:
    if explicit:
        return [(ROOT / value).resolve() for value in explicit]
    spec = corpus / "IMPLEMENTED_SPEC.md"
    if not spec.exists():
        return []
    paths = []
    for value in SOURCE_PATH.findall(spec.read_text(encoding="utf-8", errors="replace")):
        path = (ROOT / value.rstrip("/")).resolve()
        if path.exists() and path not in paths:
            paths.append(path)
    return paths


def source_counts(paths: list[Path]) -> dict:
    go_files: set[Path] = set()
    test_files: set[Path] = set()
    mangle_files: set[Path] = set()
    lines = 0
    for base in paths:
        candidates = [base] if base.is_file() else list(base.rglob("*"))
        for path in candidates:
            if not path.is_file():
                continue
            if path.suffix == ".go":
                (test_files if path.name.endswith("_test.go") else go_files).add(path)
                if not path.name.endswith("_test.go"):
                    lines += path.read_text(encoding="utf-8", errors="replace").count("\n") + 1
            elif path.suffix in {".mg", ".gl"}:
                mangle_files.add(path)
    return {
        "go_files": len(go_files),
        "test_files": len(test_files),
        "mangle_files": len(mangle_files),
        "go_source_lines": lines,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("subsystem")
    parser.add_argument("--source", action="append", default=[], help="Repository-relative source path; repeatable")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    corpus = ROOT / "Docs" / "architecture" / args.subsystem
    docs = document_counts(corpus)
    sources = discover_sources(corpus, args.source)
    measured = source_counts(sources)
    result = {
        "subsystem": args.subsystem,
        "corpus": str(corpus),
        "corpus_exists": corpus.exists(),
        "documents": docs,
        "source_paths": [str(path.relative_to(ROOT)) for path in sources],
        "source": measured,
        "readiness": (
            "READY_FOR_GAP_AUDIT" if docs["implemented_spec"] and sources
            else "CORPUS_WITHOUT_RESOLVABLE_SOURCE" if docs["implemented_spec"]
            else "SOURCE_WITHOUT_IMPLEMENTED_SPEC" if sources
            else "INSUFFICIENT_EVIDENCE"
        ),
    }
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(f"subsystem={args.subsystem} readiness={result['readiness']}")
        print(f"docs={docs['markdown_files']} source_paths={len(sources)} go={measured['go_files']} tests={measured['test_files']} mangle={measured['mangle_files']}")
    return 0 if corpus.exists() else 1


if __name__ == "__main__":
    raise SystemExit(main())
