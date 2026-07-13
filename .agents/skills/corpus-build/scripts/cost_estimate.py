#!/usr/bin/env python3
"""Estimate token cost from a corpus-build plan.

Usage:
    python cost_estimate.py <plan_path>
    python cost_estimate.py .corpus-build/plans/causal_build_plan.json --json
"""

import argparse
import json
import sys
from pathlib import Path

# Cost per 1M tokens (USD) as of 2026-03
PRICING = {
    "opus_input": 15.00,
    "opus_output": 75.00,
    "sonnet_input": 3.00,
    "sonnet_output": 15.00,
    "haiku_input": 0.25,
    "haiku_output": 1.25,
}

# Estimated tokens per work unit type
WU_ESTIMATES = {
    1: {"model": "sonnet", "input_k": 50, "output_k": 15,
        "desc": "New Go package/file"},
    2: {"model": "sonnet", "input_k": 40, "output_k": 10,
        "desc": "Complete partial implementation"},
    3: {"model": "sonnet", "input_k": 30, "output_k": 10,
        "desc": "Unit tests"},
    4: {"model": "sonnet", "input_k": 50, "output_k": 15,
        "desc": "Integration tests"},
    5: {"model": "opus", "input_k": 80, "output_k": 20,
        "desc": "Cross-system tests"},
    6: {"model": "sonnet", "input_k": 40, "output_k": 12,
        "desc": "REST API endpoints"},
    7: {"model": "sonnet", "input_k": 40, "output_k": 12,
        "desc": "Frontend page agent"},
    8: {"model": "sonnet", "input_k": 30, "output_k": 8,
        "desc": "Mangle rules"},
    9: {"model": "sonnet", "input_k": 20, "output_k": 5,
        "desc": "System corpus docs"},
    10: {"model": "sonnet", "input_k": 30, "output_k": 8,
         "desc": "Wiring/integration"},
    11: {"model": "sonnet", "input_k": 40, "output_k": 12,
         "desc": "Protocol handlers"},
}

# Fixed pipeline costs (non-work-unit phases)
FIXED_COSTS = [
    {"phase": "-1", "desc": "Vision anchor (orchestrator)",
     "model": "sonnet", "input_k": 20, "output_k": 2},
    {"phase": "1", "desc": "Corpus reader",
     "model": "sonnet", "input_k": 100, "output_k": 20},
    {"phase": "2", "desc": "Corpus judge",
     "model": "opus", "input_k": 150, "output_k": 30},
    {"phase": "4", "desc": "Wiring auditor",
     "model": "sonnet", "input_k": 80, "output_k": 15},
    {"phase": "5", "desc": "Verification re-run",
     "model": "sonnet", "input_k": 50, "output_k": 10},
]


def compute_cost(model: str, input_k: float, output_k: float) -> float:
    ip = PRICING[f"{model}_input"]
    op = PRICING[f"{model}_output"]
    return (input_k * ip + output_k * op) / 1000.0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("plan", help="Path to build plan JSON")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()

    plan_path = Path(args.plan)
    if not plan_path.exists():
        print(f"Plan not found: {plan_path}", file=sys.stderr)
        sys.exit(1)

    with open(plan_path, encoding="utf-8") as f:
        plan = json.load(f)

    work_units = plan.get("work_units", [])

    phases = []

    # Fixed costs
    for fc in FIXED_COSTS:
        cost = compute_cost(fc["model"], fc["input_k"], fc["output_k"])
        phases.append({
            "phase": fc["phase"],
            "description": fc["desc"],
            "model": fc["model"],
            "input_tokens_k": fc["input_k"],
            "output_tokens_k": fc["output_k"],
            "cost_usd": round(cost, 2),
        })

    # Work unit costs
    wu_total = 0.0
    for wu in work_units:
        wu_type = wu.get("type", 1)
        est = WU_ESTIMATES.get(wu_type, WU_ESTIMATES[1])
        cost = compute_cost(
            est["model"], est["input_k"], est["output_k"],
        )
        wu_total += cost

    phases.append({
        "phase": "3",
        "description": f"Build ({len(work_units)} work units)",
        "model": "mixed",
        "input_tokens_k": sum(
            WU_ESTIMATES.get(wu.get("type", 1), WU_ESTIMATES[1])["input_k"]
            for wu in work_units
        ),
        "output_tokens_k": sum(
            WU_ESTIMATES.get(wu.get("type", 1), WU_ESTIMATES[1])["output_k"]
            for wu in work_units
        ),
        "cost_usd": round(wu_total, 2),
    })

    total = sum(p["cost_usd"] for p in phases)
    total_input = sum(p["input_tokens_k"] for p in phases)
    total_output = sum(p["output_tokens_k"] for p in phases)

    result = {
        "plan": str(plan_path),
        "work_units": len(work_units),
        "phases": phases,
        "total_input_tokens_k": total_input,
        "total_output_tokens_k": total_output,
        "total_cost_usd": round(total, 2),
        "confidence": "medium",
        "notes": "Estimates based on average token usage per work unit type. "
                 "Actual costs vary with subsystem complexity.",
    }

    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(f"Cost Estimate for {plan_path.name}")
        print(f"{'=' * 60}")
        for p in phases:
            print(f"  Phase {p['phase']:3s}  {p['description']:40s}"
                  f"  ${p['cost_usd']:.2f}")
        print(f"{'=' * 60}")
        print(f"  TOTAL: ${total:.2f}"
              f"  ({total_input}K input + {total_output}K output tokens)")
        print(f"  Work units: {len(work_units)}")
        print(f"  Confidence: {result['confidence']}")


if __name__ == "__main__":
    main()
