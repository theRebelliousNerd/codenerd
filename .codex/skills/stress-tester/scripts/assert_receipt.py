#!/usr/bin/env python3
"""Assert deterministic properties of a persisted stress profile receipt."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


CRITICAL = re.compile(
    r"(?:^|\s)panic(?::|\s|$)|fatal error:|runtime error:|out of memory|"
    r"concurrent map (?:read|write|iteration)|warning: data race",
    re.IGNORECASE | re.MULTILINE,
)


def evaluate(
    receipt_path: Path,
    expected: set[str],
    command_fragments: list[str],
    artifacts: list[str],
    no_critical_output: bool,
) -> dict:
    errors: list[str] = []
    try:
        receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return {"valid": False, "errors": [f"cannot read receipt: {exc}"]}
    verdict = str(receipt.get("verdict", ""))
    if expected and verdict not in expected:
        errors.append(f"verdict {verdict!r} not in {sorted(expected)}")
    commands = receipt.get("commands", [])
    rendered = [" ".join(item.get("argv", [])) for item in commands if isinstance(item, dict)]
    for fragment in command_fragments:
        if not any(fragment in command for command in rendered):
            errors.append(f"no command contains {fragment!r}")
    base = receipt_path.parent
    for relative in artifacts:
        path = (base / relative).resolve()
        try:
            path.relative_to(base.resolve())
        except ValueError:
            errors.append(f"artifact escapes receipt directory: {relative}")
            continue
        if not path.exists():
            errors.append(f"missing artifact: {relative}")
    critical_matches = []
    if no_critical_output:
        for item in commands:
            if not isinstance(item, dict):
                continue
            for stream in ("stdout", "stderr"):
                relative = item.get(stream)
                if not relative:
                    continue
                path = base / relative
                if not path.is_file():
                    errors.append(f"missing command {stream}: {relative}")
                    continue
                text = path.read_text(encoding="utf-8", errors="replace")
                match = CRITICAL.search(text)
                if match:
                    critical_matches.append({"path": relative, "signal": match.group(0)[:120]})
        if critical_matches:
            errors.append(f"critical output signals found: {len(critical_matches)}")
    return {
        "valid": not errors,
        "receipt": str(receipt_path),
        "verdict": verdict,
        "command_count": len(commands),
        "critical_matches": critical_matches,
        "errors": errors,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("receipt", type=Path)
    parser.add_argument("--expect-verdict", action="append", default=[])
    parser.add_argument("--require-command", action="append", default=[])
    parser.add_argument("--require-artifact", action="append", default=[])
    parser.add_argument("--no-critical-output", action="store_true")
    args = parser.parse_args()
    result = evaluate(
        args.receipt.resolve(),
        set(args.expect_verdict),
        args.require_command,
        args.require_artifact,
        args.no_critical_output,
    )
    print(json.dumps(result, indent=2))
    return 0 if result["valid"] else 2


if __name__ == "__main__":
    raise SystemExit(main())
