#!/usr/bin/env python3
"""Build a measured codeNERD architecture context index."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
ARCH = ROOT / "Docs" / "architecture"
DEFAULT_OUTPUT = ROOT / ".corpus-build" / "context_index.json"
TAG_BLOCK = re.compile(r"<!--\s*NERD_FEATURE\s*(.*?)-->", re.DOTALL | re.IGNORECASE)
FIELD = re.compile(r"^\s*([A-Za-z0-9_-]+)\s*:\s*(.*?)\s*$", re.MULTILINE)
CODE_PATH = re.compile(r"`((?:internal|cmd)/[A-Za-z0-9_./-]+)`")


def frontmatter(text: str) -> dict[str, str]:
    if not text.startswith("---"):
        return {}
    parts = text.split("---", 2)
    if len(parts) < 3:
        return {}
    return {match.group(1): match.group(2).strip(" '\"") for match in FIELD.finditer(parts[1])}


def feature_blocks(text: str) -> list[dict[str, str]]:
    return [
        {match.group(1): match.group(2).strip(" '\"") for match in FIELD.finditer(block.group(1))}
        for block in TAG_BLOCK.finditer(text)
    ]


def build_index(root: Path = ROOT) -> tuple[dict, list[str]]:
    architecture = root / "Docs" / "architecture"
    docs: dict[str, dict] = {}
    features: dict[str, dict] = {}
    errors: list[str] = []
    for path in sorted(architecture.rglob("*.md")):
        if "compiled" in path.parts:
            continue
        text = path.read_text(encoding="utf-8", errors="replace")
        rel = path.relative_to(root).as_posix()
        metadata = frontmatter(text)
        blocks = feature_blocks(text)
        source_paths = sorted(set(match.group(1).rstrip("/") for match in CODE_PATH.finditer(text)))
        ids = []
        for block in blocks:
            feature_id = block.get("id") or block.get("feature_id")
            if not feature_id:
                errors.append(f"feature tag without id in {rel}")
                continue
            if feature_id in features and features[feature_id]["owner_doc"] != rel:
                errors.append(f"duplicate feature id {feature_id}: {features[feature_id]['owner_doc']} and {rel}")
                continue
            ids.append(feature_id)
            features[feature_id] = {
                "owner_doc": rel,
                "status": block.get("status") or metadata.get("status"),
                "source_paths": source_paths,
            }
        if metadata or ids or source_paths:
            docs[rel] = {
                "subsystem": path.relative_to(architecture).parts[0] if path.parent != architecture else "_root",
                "feature_ids": ids,
                "status": metadata.get("status"),
                "source_paths": source_paths,
            }
    unresolved = sorted(
        {source for doc in docs.values() for source in doc["source_paths"] if not (root / source).exists()}
    )
    payload = {
        "schema_version": 1,
        "architecture_root": str(architecture),
        "document_count": len(docs),
        "feature_count": len(features),
        "documents": docs,
        "features": features,
        "unresolved_source_paths": unresolved,
    }
    return payload, errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--check", action="store_true", help="Validate and print; do not write")
    args = parser.parse_args()
    payload, errors = build_index(args.root.resolve())
    result = {**payload, "errors": errors, "valid": not errors}
    if args.check:
        print(json.dumps(result, indent=2))
    else:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
        print(f"valid={result['valid']} documents={result['document_count']} features={result['feature_count']} output={args.output}")
    return 0 if result["valid"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
