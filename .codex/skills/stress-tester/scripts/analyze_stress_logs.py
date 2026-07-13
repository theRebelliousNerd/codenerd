#!/usr/bin/env python3
"""Generate bounded Markdown and JSON receipts from current codeNERD stress logs."""

from __future__ import annotations

import argparse
import importlib.util
import json
import re
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
PARSER = ROOT / ".codex" / "skills" / "log-analyzer" / "scripts" / "parse_log.py"

CRITICAL = {
    "panic": re.compile(r"(?:^|\s)panic(?::|\s|$)", re.IGNORECASE),
    "fatal_runtime": re.compile(r"fatal error:|runtime error:", re.IGNORECASE),
    "out_of_memory": re.compile(r"out of memory|cannot allocate memory", re.IGNORECASE),
    "concurrent_map": re.compile(r"concurrent map (?:read|write|iteration)", re.IGNORECASE),
    "data_race": re.compile(r"warning: data race|data race", re.IGNORECASE),
}
RESOURCE = {
    "timeout": re.compile(r"timeout|deadline exceeded", re.IGNORECASE),
    "queue_pressure": re.compile(r"queue (?:full|saturat)|capacity exceeded", re.IGNORECASE),
    "limit": re.compile(r"gas limit|resource limit|rate limit", re.IGNORECASE),
}
STRESS_SIGNAL = re.compile(r"assault|stress|soak|race|campaign|kernel|derivation|mangle", re.IGNORECASE)


def load_parser():
    if not PARSER.exists():
        raise FileNotFoundError(f"log parser not found: {PARSER}")
    spec = importlib.util.spec_from_file_location("codenerd_log_parser", PARSER)
    if spec is None or spec.loader is None:
        raise RuntimeError("could not load log parser module")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def parse_after(value: str | None) -> datetime | None:
    if not value:
        return None
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is not None:
        parsed = parsed.astimezone().replace(tzinfo=None)
    return parsed


def analyze(logs_dir: Path, after: datetime | None, max_examples: int) -> dict:
    parser = load_parser()
    files = sorted(logs_dir.glob("*.log")) if logs_dir.exists() else []
    levels: Counter[str] = Counter()
    categories: Counter[str] = Counter()
    critical: Counter[str] = Counter()
    resource: Counter[str] = Counter()
    examples: list[dict] = []
    total = 0
    stress_events = 0
    for path in files:
        for entry in parser.parse_log_file(str(path), after=after):
            total += 1
            levels[entry.level.lower()] += 1
            categories[entry.category] += 1
            message = entry.message
            matched = []
            for name, pattern in CRITICAL.items():
                if pattern.search(message):
                    critical[name] += 1
                    matched.append(name)
            for name, pattern in RESOURCE.items():
                if pattern.search(message):
                    resource[name] += 1
                    matched.append(name)
            if STRESS_SIGNAL.search(message) or entry.category in {"kernel", "campaign", "tester", "shards"}:
                stress_events += 1
            if matched and len(examples) < max_examples:
                examples.append({
                    "timestamp_ms": entry.timestamp_ms,
                    "category": entry.category,
                    "level": entry.level,
                    "signals": matched,
                    "message": message[:500],
                    "file": path.name,
                    "line": entry.line_number,
                })

    if not files or total == 0 or stress_events == 0:
        verdict = "no_signal"
    elif sum(critical.values()) > 0:
        verdict = "failed"
    elif levels["error"] > 0:
        verdict = "partial"
    else:
        verdict = "passed"
    return {
        "schema_version": 1,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "logs_dir": str(logs_dir),
        "after": after.isoformat() if after else None,
        "verdict": verdict,
        "log_file_count": len(files),
        "entry_count": total,
        "stress_signal_count": stress_events,
        "levels": dict(levels),
        "categories": dict(categories.most_common()),
        "critical_signals": dict(critical),
        "resource_signals": dict(resource),
        "examples": examples,
    }


def markdown(report: dict) -> str:
    lines = [
        "# Stress log analysis",
        "",
        f"- Verdict: **{report['verdict'].upper()}**",
        f"- Log files: {report['log_file_count']}",
        f"- Parsed entries: {report['entry_count']}",
        f"- Stress-relevant signals: {report['stress_signal_count']}",
        f"- Window start: {report['after'] or 'not constrained'}",
        "",
        "## Signals",
        "",
        "| Class | Signal | Count |",
        "|---|---|---:|",
    ]
    for classification in ("critical_signals", "resource_signals"):
        label = classification.replace("_signals", "")
        for name, count in report[classification].items():
            lines.append(f"| {label} | {name} | {count} |")
    if not report["critical_signals"] and not report["resource_signals"]:
        lines.append("| observed | none | 0 |")
    lines.extend(["", "## Bounded examples", ""])
    if not report["examples"]:
        lines.append("No matching examples were observed.")
    for item in report["examples"]:
        message = item["message"].replace("`", "'").replace("\n", " ")
        lines.append(f"- `{item['file']}:{item['line']}` [{item['level']}/{item['category']}] `{','.join(item['signals'])}` — {message}")
    lines.extend([
        "",
        "## Interpretation",
        "",
        "This receipt is a bounded signal summary. `NO_SIGNAL` means the available logs do not prove the target path was exercised. Use the log-analyzer skill for causal multi-category queries.",
    ])
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--logs-dir", type=Path, default=ROOT / ".nerd" / "logs")
    parser.add_argument("--after", help="ISO-8601 lower time bound")
    parser.add_argument("--max-examples", type=int, default=20)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if not 0 <= args.max_examples <= 200:
        parser.error("--max-examples must be between 0 and 200")
    try:
        report = analyze(args.logs_dir.resolve(), parse_after(args.after), args.max_examples)
    except (FileNotFoundError, RuntimeError, ValueError) as exc:
        parser.error(str(exc))
    rendered = markdown(report)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered, encoding="utf-8")
        sidecar = args.output.with_suffix(".json")
        sidecar.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
        print(f"verdict={report['verdict']} report={args.output} json={sidecar}")
    else:
        print(rendered, end="")
    return 2 if report["verdict"] == "failed" else 0


if __name__ == "__main__":
    raise SystemExit(main())
