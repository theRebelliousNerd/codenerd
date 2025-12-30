# world_state/ - World State Trigger Atoms

Atoms triggered by specific world state conditions.

## Files

| File | Trigger | Purpose |
|------|---------|---------|
| `diagnostics.yaml` | `/failing_tests`, `/compiler_errors` | Error handling guidance |
| `high_churn.yaml` | `/high_churn` | Recently modified files guidance |
| `large_refactor.yaml` | `/large_refactor` | Multi-file change coordination |
| `new_files.yaml` | `/new_files` | New file creation patterns |
| `security_issues.yaml` | `/security_issues` | Security vulnerability handling |

## Selection

World state atoms are selected via `world_states` selector:

```yaml
world_states: ["/failing_tests", "/compiler_errors"]
```

## World State Detection

World states are detected by:
- Kernel fact queries (`diagnostic(_, /error, _, _)`)
- File change monitoring
- Security scanners
- Test execution results

## Purpose

These atoms provide context-specific guidance when the codebase is in a particular state (broken build, security issue, etc.).
