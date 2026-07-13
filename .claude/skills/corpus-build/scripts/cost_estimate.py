#!/usr/bin/env python3
"""Report plan shape and measured usage without inventing token or dollar costs.

The filename is retained for compatibility. Estimates were intentionally removed:
Codex lifecycle hooks expose usage only when the runtime supplies explicit fields.
"""

from __future__ import annotations

import argparse
import json
from collections import Counter
from pathlib import Path


def analyze_plan(path: Path) -> dict:
    plan = json.loads(path.read_text(encoding="utf-8"))
    units = plan.get("work_units", [])
    if not isinstance(units, list):
        raise ValueError("work_units must be a list")
    levels = Counter(str(unit.get("level", "unassigned")) for unit in units if isinstance(unit, dict))
    owners = Counter(str(unit.get("owner", "unassigned")) for unit in units if isinstance(unit, dict))
    missing_acceptance = [
        unit.get("id", "<unknown>")
        for unit in units
        if isinstance(unit, dict) and not unit.get("acceptance")
    ]
    return {
        "plan": str(path),
        "work_unit_count": len(units),
        "units_by_level": dict(sorted(levels.items())),
        "units_by_owner": dict(sorted(owners.items())),
        "max_declared_fan_out": max(levels.values(), default=0),
        "missing_acceptance": missing_acceptance,
        "estimated_tokens": None,
        "estimated_cost_usd": None,
        "measurement_policy": "usage is recorded only from explicit hook payloads",
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("plan", type=Path)
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    if not args.plan.is_file():
        parser.error(f"plan not found: {args.plan}")
    try:
        result = analyze_plan(args.plan)
    except (json.JSONDecodeError, ValueError) as exc:
        parser.error(str(exc))
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(f"work_units={result['work_unit_count']} max_fan_out={result['max_declared_fan_out']}")
        print("token_estimate=unavailable cost_estimate=unavailable")
        if result["missing_acceptance"]:
            print("missing_acceptance=" + ",".join(result["missing_acceptance"]))
    return 2 if result["missing_acceptance"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
