#!/usr/bin/env python3
"""Classify codeNERD corpus-build candidate surfaces from the live registry."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
REGISTRY = ROOT / ".codex" / "skills" / "corpus-build" / "references" / "surfaces.yaml"


def parse_list(value: str) -> list[str]:
    value = value.strip()
    if not value.startswith("[") or not value.endswith("]"):
        return []
    return [part.strip().strip("'\"") for part in value[1:-1].split(",") if part.strip()]


def parse_registry(path: Path = REGISTRY) -> list[dict]:
    records: list[dict] = []
    current: dict | None = None
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        match = re.match(r"- id:\s*(.+)", line)
        if match:
            if current:
                records.append(current)
            current = {"id": match.group(1), "paths": []}
            continue
        if current is None:
            continue
        for key in ("category", "group", "name", "fix_owner"):
            prefix = f"{key}:"
            if line.startswith(prefix):
                current[key] = line[len(prefix):].strip()
        if line.startswith("paths:"):
            current["paths"] = parse_list(line.split(":", 1)[1])
        if line == "always: true":
            current["always"] = True
    if current:
        records.append(current)
    return records


def normalized_manifest(path: Path | None) -> dict:
    if path is None:
        return {}
    data = json.loads(path.read_text(encoding="utf-8"))
    return {
        "subsystem": str(data.get("subsystem", "")).strip("/"),
        "integration_points": [str(item).lower() for item in data.get("integration_points", [])],
        "touched_paths": [str(item).replace("\\", "/") for item in data.get("touched_paths", [])],
    }


def classify(root: Path, manifest: dict, records: list[dict]) -> dict:
    results = []
    errors = []
    terms = set(manifest.get("integration_points", []))
    touched = manifest.get("touched_paths", [])
    subsystem = manifest.get("subsystem", "")
    for item in records:
        paths = [path.replace("<subsystem>", subsystem) for path in item.get("paths", [])]
        existing = [path for path in paths if path and (root / path).exists()]
        haystack = " ".join([item.get("id", ""), item.get("name", ""), item.get("category", ""), *paths]).lower()
        point_match = any(term and term in haystack for term in terms)
        touch_match = any(
            touched_path.startswith(path.rstrip("/") + "/")
            or path.startswith(touched_path.rstrip("/") + "/")
            or touched_path.rstrip("/") == path.rstrip("/")
            for touched_path in touched
            for path in paths
            if path
        )
        required = bool(item.get("always")) or point_match or touch_match
        if not manifest:
            verdict = "DISCOVERED" if existing or not paths else "UNAVAILABLE"
        elif required and (existing or not paths):
            verdict = "REQUIRED_NEEDS_EVIDENCE"
        elif required:
            verdict = "REQUIRED_MISSING"
            errors.append(f"{item['id']}: required paths missing: {paths}")
        elif existing:
            verdict = "OPTIONAL"
        else:
            verdict = "N-A"
        results.append({**item, "paths": paths, "existing_paths": existing, "verdict": verdict})
    return {"valid": not errors, "errors": errors, "surfaces": results}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--manifest", type=Path, help="JSON run manifest")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    try:
        manifest = normalized_manifest(args.manifest)
        result = classify(args.root.resolve(), manifest, parse_registry())
    except (OSError, json.JSONDecodeError) as exc:
        parser.error(str(exc))
    result.update({"root": str(args.root.resolve()), "manifest": manifest, "registry": str(REGISTRY)})
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        for item in result["surfaces"]:
            print(f"{item['id']:24} {item['verdict']:24} {item.get('name', '')}")
        for error in result["errors"]:
            print(f"ERROR: {error}")
    return 0 if result["valid"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
