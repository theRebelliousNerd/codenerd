#!/usr/bin/env python3
"""Detect stub-implementation patterns in a list of Go (or any text) files.

Scans each given file for four stub-signal classes:

  1. todo_marker         - TODO / FIXME / XXX comment markers
  2. panic_not_implemented - panic("not implemented") / panic(fmt.Sprintf(...not implemented...))
  3. stub_return         - a top-level `func` whose entire gofmt'd body is a single
                           trivial `return` statement (nil / zero-value / empty literal)
  4. placeholder_literal - string literals containing "not implemented", "coming soon",
                           "placeholder", "TBD", "stub only", etc.

This is a spot-check tool, not a proof: pattern 3 relies on gofmt convention (a
top-level function's closing brace sits at column 0) to approximate body
extraction without a real Go parser. A legitimately trivial one-line function
(e.g. a real getter that returns a constant) can be a false positive -- read the
surrounding code before treating a finding as a confirmed stub.

Usage:
    python detect_stubs.py --files internal/causal/chain.go internal/causal/graph.go
    python detect_stubs.py --list-file .corpus-build/results/WU-003_files.txt --json
    python detect_stubs.py --files internal/causal/chain.go --fail-on-find
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any


TODO_PATTERN = re.compile(r"//.*\b(TODO|FIXME|XXX)\b", re.IGNORECASE)

PANIC_NOT_IMPLEMENTED_PATTERN = re.compile(
    r"panic\s*\(\s*(?:fmt\.Sprintf\s*\()?\s*\"[^\"]*not\s+implemented[^\"]*\"",
    re.IGNORECASE,
)

PLACEHOLDER_LITERAL_PATTERN = re.compile(
    r"\"[^\"]*(not\s+implemented|coming\s+soon|placeholder|stub\s+only|TBD|to\s+be\s+implemented)[^\"]*\"",
    re.IGNORECASE,
)

# Top-level Go function: gofmt puts the closing brace of a top-level func at
# column 0. Non-greedy body capture up to the first such brace approximates
# the function extent well for gofmt'd source.
FUNC_PATTERN = re.compile(
    r"^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)[^\n{]*\{\n(.*?)\n\}",
    re.MULTILINE | re.DOTALL,
)

TRIVIAL_RETURN_PATTERN = re.compile(
    r"^\s*return(\s+(nil|0|0\.0|\"\"|false|true|[A-Za-z_][A-Za-z0-9_.]*\{\})"
    r"(\s*,\s*(nil|0|\"\"|false|true|[A-Za-z_][A-Za-z0-9_.]*\{\}))*)?\s*$"
)


def find_line_number(content: str, offset: int) -> int:
    return content.count("\n", 0, offset) + 1


def scan_todo_markers(path: Path, content: str) -> list[dict[str, Any]]:
    findings = []
    for match in TODO_PATTERN.finditer(content):
        line = find_line_number(content, match.start())
        findings.append({
            "pattern": "todo_marker",
            "line": line,
            "text": content.splitlines()[line - 1].strip(),
        })
    return findings


def scan_panic_not_implemented(path: Path, content: str) -> list[dict[str, Any]]:
    findings = []
    for match in PANIC_NOT_IMPLEMENTED_PATTERN.finditer(content):
        line = find_line_number(content, match.start())
        findings.append({
            "pattern": "panic_not_implemented",
            "line": line,
            "text": content.splitlines()[line - 1].strip(),
        })
    return findings


def scan_placeholder_literals(path: Path, content: str) -> list[dict[str, Any]]:
    findings = []
    for match in PLACEHOLDER_LITERAL_PATTERN.finditer(content):
        line = find_line_number(content, match.start())
        findings.append({
            "pattern": "placeholder_literal",
            "line": line,
            "text": content.splitlines()[line - 1].strip(),
        })
    return findings


def scan_stub_returns(path: Path, content: str) -> list[dict[str, Any]]:
    findings = []
    for match in FUNC_PATTERN.finditer(content):
        func_name = match.group(1)
        body = match.group(2)
        body_lines = [
            ln.strip() for ln in body.splitlines()
            if ln.strip() and not ln.strip().startswith("//")
        ]
        if len(body_lines) != 1:
            continue
        if TRIVIAL_RETURN_PATTERN.match(body_lines[0]):
            line = find_line_number(content, match.start(2))
            findings.append({
                "pattern": "stub_return",
                "line": line,
                "text": f"func {func_name}(...) {{ {body_lines[0]} }}",
                "func": func_name,
            })
    return findings


def scan_file(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {"file": str(path), "error": "file not found", "findings": []}
    try:
        content = path.read_text(encoding="utf-8", errors="replace")
    except OSError as exc:
        return {"file": str(path), "error": str(exc), "findings": []}

    findings: list[dict[str, Any]] = []
    findings.extend(scan_todo_markers(path, content))
    findings.extend(scan_panic_not_implemented(path, content))
    findings.extend(scan_placeholder_literals(path, content))
    findings.extend(scan_stub_returns(path, content))
    findings.sort(key=lambda f: f["line"])
    return {"file": str(path), "findings": findings}


def collect_files(args: argparse.Namespace) -> list[Path]:
    files: list[str] = list(args.files or [])
    if args.list_file:
        list_path = Path(args.list_file)
        if not list_path.exists():
            print(f"error: --list-file {list_path} not found", file=sys.stderr)
            sys.exit(2)
        files.extend(
            line.strip() for line in list_path.read_text(encoding="utf-8").splitlines()
            if line.strip() and not line.strip().startswith("#")
        )
    if not files:
        print("error: no files given (use --files or --list-file)", file=sys.stderr)
        sys.exit(2)
    return [Path(f) for f in files]


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Detect stub-implementation patterns (TODO markers, "
                     "panic(\"not implemented\"), trivial stub returns, "
                     "placeholder string literals) across a list of files.",
    )
    parser.add_argument("--files", nargs="+", help="Explicit file paths to scan")
    parser.add_argument(
        "--list-file",
        help="Text file with one path per line (blank lines and # comments ignored)",
    )
    parser.add_argument(
        "--fail-on-find",
        action="store_true",
        help="Exit 1 if any findings are reported (default: always exit 0)",
    )
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Pretty-print JSON (default: compact single-line JSON per file, "
             "still valid JSON overall via indent=2)",
    )
    args = parser.parse_args()

    paths = collect_files(args)
    results = [scan_file(p) for p in paths]
    total_findings = sum(len(r["findings"]) for r in results)

    output = {
        "files_scanned": len(paths),
        "total_findings": total_findings,
        "results": results,
    }

    indent = 2 if args.pretty or True else None
    print(json.dumps(output, indent=indent))

    if args.fail_on_find and total_findings > 0:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
