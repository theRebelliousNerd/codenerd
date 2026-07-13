#!/usr/bin/env python3
"""Quick gap audit: architecture corpus vs source code.

Usage:
    python subsystem_audit.py <subsystem>
    python subsystem_audit.py causal --json
"""

import argparse
import json
import re
import sys
from pathlib import Path


def find_project_root() -> Path:
    cwd = Path.cwd()
    for parent in [cwd, *cwd.parents]:
        if (parent / "go.mod").exists():
            return parent
    return cwd


def count_docs(corpus_dir: Path) -> dict:
    counts = {
        "total": 0, "implemented_spec": False, "foundation": 0,
        "deep_dives": 0, "cross_cutting": 0, "governance": 0,
    }
    if not corpus_dir.exists():
        return counts
    cc_kw = ["frontend", "dependency", "rbac", "testing-alignment",
             "cross-system", "telemetry"]
    for f in corpus_dir.iterdir():
        if not f.is_file() or f.suffix != ".md":
            continue
        counts["total"] += 1
        name = f.name
        if name == "IMPLEMENTED_SPEC.md":
            counts["implemented_spec"] = True
        elif re.match(r"0[0-4]-", name):
            counts["foundation"] += 1
        elif re.match(r"0[5-9]-|[1-9][0-9]-", name):
            if any(k in name.lower() for k in cc_kw):
                counts["cross_cutting"] += 1
            else:
                counts["deep_dives"] += 1
        elif name in ("TODO.md", "OPEN-QUESTIONS.md"):
            counts["governance"] += 1
    return counts


def count_source(source_dir: Path) -> dict:
    counts = {"go_files": 0, "test_files": 0, "lines": 0,
              "benchmarks": 0, "exported_funcs": 0}
    if not source_dir.exists():
        return counts
    func_re = re.compile(r"^func\s+[A-Z]", re.MULTILINE)
    bench_re = re.compile(r"^func\s+Benchmark", re.MULTILINE)
    for f in source_dir.rglob("*.go"):
        try:
            c = f.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        lines = c.count("\n")
        if f.name.endswith("_test.go"):
            counts["test_files"] += 1
            counts["benchmarks"] += len(bench_re.findall(c))
        else:
            counts["go_files"] += 1
            counts["lines"] += lines
            counts["exported_funcs"] += len(func_re.findall(c))
    return counts


def count_priority(path: Path, section_pattern=None) -> dict:
    p = {"p0": 0, "p1": 0, "p2": 0, "p3": 0}
    if not path.exists():
        return p
    try:
        content = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return p
    if section_pattern:
        m = re.search(section_pattern, content, re.DOTALL)
        content = m.group(1) if m else ""
    for k in p:
        p[k] = len(re.findall(rf"\b{k.upper()}\b", content))
    return p


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("subsystem")
    parser.add_argument("--json", action="store_true")
    parser.add_argument("--source", help="Override source path")
    args = parser.parse_args()

    root = find_project_root()
    corpus = root / "docs" / "architecture" / args.subsystem
    source = (root / args.source) if args.source else (
        root / "internal" / args.subsystem)

    docs = count_docs(corpus)
    src = count_source(source)
    s12 = count_priority(
        corpus / "IMPLEMENTED_SPEC.md",
        r"##\s+12\.?\s+Recommended Uplifts(.*?)(?=##\s+13\.|$)",
    )
    todo = count_priority(corpus / "TODO.md")

    result = {
        "subsystem": args.subsystem,
        "corpus_exists": corpus.exists(),
        "source_exists": source.exists(),
        "has_implemented_spec": docs["implemented_spec"],
        "docs": docs, "source": src,
        "section_12": s12, "todo": todo,
        "readiness": (
            "READY" if docs["implemented_spec"] and src["go_files"] > 0
            else "CORPUS_ONLY" if docs["implemented_spec"]
            else "CODE_ONLY" if src["go_files"] > 0
            else "EMPTY"
        ),
    }

    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(f"Subsystem: {args.subsystem}")
        print(f"Corpus: {docs['total']} docs"
              f" (SPEC: {'yes' if docs['implemented_spec'] else 'NO'})")
        print(f"Source: {src['go_files']} files"
              f" ({src['lines']} lines)"
              f" {src['test_files']} tests"
              f" {src['benchmarks']} benchmarks")
        print(f"Section 12: P0={s12['p0']} P1={s12['p1']}"
              f" P2={s12['p2']} P3={s12['p3']}")
        print(f"TODO: P0={todo['p0']} P1={todo['p1']}"
              f" P2={todo['p2']} P3={todo['p3']}")
        print(f"Readiness: {result['readiness']}")


if __name__ == "__main__":
    main()
