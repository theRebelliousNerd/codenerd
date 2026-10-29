# Integration Execution Audit

Fast, heuristic checks for Go code that is constructed but never executed,
channels without consumers, unhandled Bubble Tea messages, unwired receiver
fields, suspicious blocking calls, and references that are immediately lost.

```powershell
python -B cmd/tools/integration_audit/audit_execution.py --component core --json
python -B cmd/tools/integration_audit/test_audit_execution.py
```

`--component` matches exact path segments and ignores runtime/worktree mirrors.
In `--json` mode stdout is JSON only, progress goes to stderr, and ERROR findings
produce exit code 1. Warnings and INFO findings are heuristic review candidates;
they do not fail the command.
