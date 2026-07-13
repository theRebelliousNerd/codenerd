#!/usr/bin/env python3
"""Build a deterministic codeNERD corpus remediation packet."""

from __future__ import annotations

import argparse
import json
from datetime import datetime, timezone
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--attempt-id", required=True)
    parser.add_argument("--wu", required=True)
    parser.add_argument("--subsystem", required=True)
    parser.add_argument("--gate-log", required=True)
    parser.add_argument("--spec-refs", nargs="+", required=True)
    parser.add_argument("--contract")
    parser.add_argument("--verify-cmd", required=True)
    parser.add_argument("--allowed-files", nargs="*", default=[])
    parser.add_argument("--forbidden-files", nargs="*", default=[])
    parser.add_argument("--prior-attempt", action="append", default=[])
    parser.add_argument("--rollback")
    parser.add_argument("--out-dir", default=".corpus-build/jules")
    parser.add_argument("--stdout-only", action="store_true")
    args = parser.parse_args()

    gate_path = Path(args.gate_log)
    if not gate_path.exists():
        parser.error(f"gate log does not exist: {gate_path}")

    packet = {
        "schema": "codenerd.corpus-remediation.v1",
        "attempt_id": args.attempt_id,
        "work_unit_id": args.wu,
        "subsystem": args.subsystem,
        "created_at": datetime.now(timezone.utc).isoformat(),
        "gate": {
            "log_path": gate_path.as_posix(),
            "excerpt": gate_path.read_text(encoding="utf-8", errors="replace")[-12000:],
            "verification_command": args.verify_cmd,
        },
        "spec_refs": args.spec_refs,
        "contract": args.contract,
        "scope": {
            "allowed_files": args.allowed_files,
            "forbidden_files": args.forbidden_files,
        },
        "prior_attempts": args.prior_attempt,
        "rollback": args.rollback,
        "required_evidence": [
            "exact diff",
            "verification command output",
            "remaining failures",
            "scope-deviation report",
        ],
        "dispatch": {
            "status": "prepared",
            "note": "Preparation is not evidence of Jules submission or completion.",
        },
    }

    rendered = json.dumps(packet, indent=2, sort_keys=True)
    if args.stdout_only:
        print(rendered)
        return 0

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    target = out_dir / f"{args.attempt_id}.json"
    target.write_text(rendered + "\n", encoding="utf-8")
    print(target.as_posix())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
