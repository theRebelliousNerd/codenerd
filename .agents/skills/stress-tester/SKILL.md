---
name: stress-tester
description: Evidence-driven stress, race, recovery, and Mangle-adversarial validation for codeNERD through its bounded native stress profiles.
---

# codeNERD Stress Tester

Use the native stress-tester control plane at `.codex/skills/stress-tester/` for
all executable checks. Read its `SKILL.md`, scoped `AGENTS.md`, and profile
registry before selecting a run. The `.codex` package owns the validator,
preflight, bounded runner, local log parser, receipt checks, and adversarial
fixtures; this entry keeps the discoverable skill aligned with that contract.

## Safe operating contract

- Run preflight and validate the package before applying load.
- Start with the narrowest deterministic profile and inspect its dry-run command.
- Execute only with `--execute`; keep the registered command and total-run limits.
- Preserve the run receipt, bounded output, and source logs for every failure.
- Treat an empty or irrelevant log window as `NO_SIGNAL`, never as a pass.
- Never clear logs, overwrite receipts, launch an unbounded soak, or run a paid
  LLM-backed assault as incidental validation.
- For each failure, record the first causal error, smallest reproducer, and
  violated invariant, then rerun the exact bounded profile after repair.

## Native control plane

From the repository root, use the supported paths. Use `python` on Windows and
`python3` on systems where Python 3 is named that way:

```bash
python3 .codex/skills/stress-tester/scripts/preflight.py
python3 .codex/skills/stress-tester/scripts/validate_skill.py
python3 .codex/skills/stress-tester/scripts/run_suite.py --profile structural
python3 .codex/skills/stress-tester/scripts/run_suite.py --profile smoke --execute
python3 .codex/skills/stress-tester/scripts/analyze_stress_logs.py
```

The registered profiles are defined in
`.codex/skills/stress-tester/references/profile-registry.json`. Production CLI
build profiles enable `sqlite_vec` and use the repository SQLite headers when
available. The included analyzer parses live logger text and JSON records; it
does not claim that a signal count proves a subsystem was exercised.

## Scenario references

The scenario designs remain in this skill's existing `references/` tree. Treat
them as threat models and verify commands against the active profile registry
and current CLI before use.

- [Workspace isolation](references/workflows/09-cli-workspace-matrix/workspace-isolation.md)
- [Full CLI surface](references/workflows/09-cli-workspace-matrix/full-cli-surface.md)
- [Polyglot application vehicle](references/workflows/09-cli-workspace-matrix/polyglot-app-vehicle.md)
- [One-shot CLI exit](references/workflows/09-cli-workspace-matrix/one-shot-cli-exit.md)
- [Dual LLM routing](references/workflows/09-cli-workspace-matrix/dual-llm-routing.md)
- [Define-agent flags](references/workflows/09-cli-workspace-matrix/define-agent-flags.md)
