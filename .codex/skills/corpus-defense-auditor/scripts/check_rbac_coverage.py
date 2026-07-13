#!/usr/bin/env python3
"""Check codeNERD constitutional-permission wiring.

The filename is retained for source-package continuity; codeNERD uses Mangle
constitutional policy rather than Vectryx REST RBAC.
"""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


def check(root: Path) -> dict[str, object]:
    schema_dir = root / "internal" / "core" / "defaults"
    policy = schema_dir / "policy" / "constitution.mg"
    schema = schema_dir / "schemas_safety.mg"
    validator = root / "internal" / "mangle" / "schema_validator.go"

    errors: list[str] = []
    evidence: dict[str, object] = {}

    for path in (policy, schema, validator):
        if not path.exists():
            errors.append(f"missing {path.relative_to(root).as_posix()}")

    policy_text = policy.read_text(encoding="utf-8", errors="replace") if policy.exists() else ""
    schema_text = schema.read_text(encoding="utf-8", errors="replace") if schema.exists() else ""
    validator_text = validator.read_text(encoding="utf-8", errors="replace") if validator.exists() else ""

    decl_count = schema_text.count("Decl permitted(")
    evidence["permitted_declaration_count"] = decl_count
    if decl_count != 1:
        errors.append(f"expected one permitted declaration in schemas_safety.mg, found {decl_count}")

    required = sorted(set(re.findall(r"requires_permission\((/[^)]+)\)", policy_text)))
    evidence["requires_permission_actions"] = required

    checks = {
        "has_permitted_rules": "permitted(" in policy_text,
        "has_default_deny": "!permitted(" in policy_text and "action_denied(" in policy_text,
        "requires_are_dangerous": bool(
            re.search(
                r"dangerous_action\(ActionType\)\s*:-\s*requires_permission\(ActionType\)",
                policy_text,
                re.MULTILINE,
            )
        ),
        "validator_owns_permitted": '"permitted"' in validator_text and "core-owned" in validator_text,
        "validator_owns_safe_action": '"safe_action"' in validator_text and "core-owned" in validator_text,
    }
    evidence.update(checks)
    for name, ok in checks.items():
        if not ok:
            errors.append(f"constitutional check failed: {name}")

    return {
        "root": str(root),
        "status": "FAIL" if errors else "PASS",
        "errors": errors,
        "evidence": evidence,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    result = check(Path(args.root).resolve())
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(f"STATUS={result['status']}")
        for error in result["errors"]:
            print(f"ERROR: {error}")
        for key, value in result["evidence"].items():
            print(f"{key}={value}")
    return 1 if result["errors"] else 0


if __name__ == "__main__":
    raise SystemExit(main())

