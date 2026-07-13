#!/usr/bin/env python3
"""Record an auditable, intentional skip of an applicable corpus-build
phase or wiring surface.

Every intentional skip of something the registry (or a phase gate) says IS
applicable must go through this script — a silent pass is never acceptable
per PLAN-corpus-build-wiring-checklist.md's verdict model ("Intentional
skips of applicable surfaces require a skip record"). Appends one JSON line
to `.corpus-build/skips.jsonl` (created if absent) with schema:

    {"ts": "<UTC ISO-8601>", "run_id": "...", "phase_or_surface": "...",
     "reason": "...", "actor": "..."}

Usage:
    python record_skip.py --run-id <run_id> --target <phase_or_surface> \\
        --reason "<why this is being skipped>" --actor <agent-or-human-name>

Refuses to write a row with an empty/whitespace-only reason (exit code 1).
Python 3 stdlib only. Windows-safe paths (pathlib throughout).
"""

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path


def find_project_root() -> Path:
    cwd = Path.cwd()
    for parent in [cwd, *cwd.parents]:
        if (parent / "go.mod").exists():
            return parent
    return cwd


def append_skip(
    skips_path: Path, run_id: str, target: str, reason: str, actor: str,
) -> dict:
    row = {
        "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "run_id": run_id,
        "phase_or_surface": target,
        "reason": reason,
        "actor": actor,
    }
    skips_path.parent.mkdir(parents=True, exist_ok=True)
    with open(skips_path, "a", encoding="utf-8") as fh:
        fh.write(json.dumps(row, default=str))
        fh.write("\n")
    return row


def main():
    parser = argparse.ArgumentParser(
        description="Append an auditable skip record to .corpus-build/skips.jsonl",
    )
    parser.add_argument("--run-id", required=True, help="The active corpus-build run_id")
    parser.add_argument(
        "--target", required=True,
        help="The phase name or registry surface id being skipped "
             "(e.g. 'Phase 5.5 codegen gate' or 'B4-grpc')",
    )
    parser.add_argument(
        "--reason", required=True,
        help="Why this applicable phase/surface is intentionally not being done",
    )
    parser.add_argument(
        "--actor", required=True,
        help="Agent name or human identity recording the skip",
    )
    parser.add_argument(
        "--skips-file",
        help="Override the default .corpus-build/skips.jsonl path (mainly for tests)",
    )
    args = parser.parse_args()

    reason = args.reason.strip()
    if not reason:
        print(
            "error: --reason must not be empty or whitespace-only - "
            "a silent skip is never acceptable (PLAN-corpus-build-wiring-checklist.md "
            "verdict model)",
            file=sys.stderr,
        )
        sys.exit(1)

    run_id = args.run_id.strip()
    if not run_id:
        print("error: --run-id must not be empty", file=sys.stderr)
        sys.exit(1)

    target = args.target.strip()
    if not target:
        print("error: --target must not be empty", file=sys.stderr)
        sys.exit(1)

    actor = args.actor.strip()
    if not actor:
        print("error: --actor must not be empty", file=sys.stderr)
        sys.exit(1)

    if args.skips_file:
        skips_path = Path(args.skips_file)
    else:
        skips_path = find_project_root() / ".corpus-build" / "skips.jsonl"

    row = append_skip(skips_path, run_id, target, reason, actor)
    print(f"Recorded skip to {skips_path}: {json.dumps(row, default=str)}")


if __name__ == "__main__":
    main()
