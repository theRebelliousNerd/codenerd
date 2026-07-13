#!/usr/bin/env python3
"""Generate dependency DAG from a corpus-build plan.

Reads a build plan JSON, produces a topologically sorted level
assignment for work units, detects cycles, and identifies reserved files.

Usage:
    python build_dag.py <plan_path>
    python build_dag.py .corpus-build/plans/causal_build_plan.json --json
"""

import argparse
import json
import sys
from graphlib import TopologicalSorter, CycleError
from pathlib import Path


def load_plan(path: Path) -> dict:
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def build_graph(work_units: list) -> dict:
    """Build adjacency dict from work unit dependencies."""
    graph = {}
    for wu in work_units:
        wid = wu["id"]
        deps = wu.get("dependencies", [])
        graph[wid] = set(deps)
    return graph


def assign_levels(graph: dict) -> dict:
    """Topological sort into parallel levels."""
    try:
        ts = TopologicalSorter(graph)
        ts.prepare()
    except CycleError as e:
        return {"error": "cycle_detected", "detail": str(e)}

    levels = {}
    level_num = 0
    while ts.is_active():
        ready = list(ts.get_ready())
        for node in ready:
            levels[node] = level_num
            ts.done(node)
        level_num += 1

    return levels


def find_reserved_files(work_units: list) -> list:
    """Find files targeted by multiple work units."""
    file_targets = {}
    for wu in work_units:
        wid = wu["id"]
        for f in wu.get("files_to_modify", []):
            file_targets.setdefault(f, []).append(wid)
        for f in wu.get("files_to_create", []):
            file_targets.setdefault(f, []).append(wid)

    return [
        {"file": f, "work_units": wus}
        for f, wus in file_targets.items()
        if len(wus) > 1
    ]


def detect_cycles(graph: dict) -> list:
    """Return cycle members if any exist."""
    try:
        ts = TopologicalSorter(graph)
        ts.prepare()
        while ts.is_active():
            ready = list(ts.get_ready())
            for n in ready:
                ts.done(n)
        return []
    except CycleError as e:
        return list(str(e).split("'")[1::2])


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("plan", help="Path to build plan JSON")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()

    plan = load_plan(Path(args.plan))
    work_units = plan.get("work_units", [])

    if not work_units:
        print("No work units in plan.", file=sys.stderr)
        sys.exit(1)

    graph = build_graph(work_units)
    cycles = detect_cycles(graph)

    if cycles:
        result = {
            "error": "cycle_detected",
            "cycle_members": cycles,
            "suggestion": "Extract shared interfaces into a Level 0 "
                          "work unit to break the cycle.",
        }
        if args.json:
            print(json.dumps(result, indent=2))
        else:
            print(f"CYCLE DETECTED: {cycles}", file=sys.stderr)
            print("Break by extracting shared interfaces "
                  "into Level 0.", file=sys.stderr)
        sys.exit(1)

    levels = assign_levels(graph)
    reserved = find_reserved_files(work_units)

    # Group work units by level
    level_groups = {}
    for wid, lvl in levels.items():
        level_groups.setdefault(lvl, []).append(wid)

    result = {
        "levels": [
            {"level": lvl, "work_units": sorted(wus)}
            for lvl, wus in sorted(level_groups.items())
        ],
        "reserved_files": reserved,
        "total_levels": len(level_groups),
        "total_work_units": len(work_units),
        "cycles_detected": [],
    }

    if args.json:
        print(json.dumps(result, indent=2))
    else:
        for lvl_info in result["levels"]:
            wus = ", ".join(lvl_info["work_units"])
            print(f"Level {lvl_info['level']}: {wus}")
        if reserved:
            print(f"\nReserved files ({len(reserved)}):")
            for r in reserved:
                print(f"  {r['file']} <- {r['work_units']}")
        print(f"\n{result['total_levels']} levels, "
              f"{result['total_work_units']} work units")


if __name__ == "__main__":
    main()
