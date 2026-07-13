# Dependency-DAG and shared-checkout rules

## Levels

1. **Contracts** — requirement IDs, Go interfaces/types, Mangle declarations, prompt-atom IDs, and persistence schemas.
2. **Foundation** — package-local implementation that depends only on pinned contracts.
3. **Behavior** — additional implementation lanes with disjoint write sets.
4. **Tests and intents** — unit/integration tests plus registration intents; may run in parallel only when paths do not overlap.
5. **Serial wiring** — one owner applies intents to contested registries and executes reachability tests.
6. **Review and reconcile** — critic, defense, consumables, repo gate, then doc-auditor.

A unit cannot share a level with one of its transitive dependencies. `scripts/build_dag.py` must reject cycles and references to unknown unit IDs.

## Shared-checkout discipline

Codex agents in this project share the same workspace. Before dispatch:

- compare exact write paths across ready units;
- serialize overlapping files and directories;
- cap write-heavy concurrency at three;
- tell every worker to preserve unrelated user edits;
- never let workers create their own subagents;
- re-read a contested file immediately before the serial edit.

## Registration intents

Workers do not concurrently edit shared hubs. They emit `.corpus-build/intents/<wu-id>.json`:

```json
{
  "work_unit": "WU-014",
  "target_file": "internal/shards/registration.go",
  "operation": "register_shard",
  "symbol": "RegisterExampleShard",
  "required_imports": [],
  "verification": "go test -count=1 ./internal/shards/..."
}
```

Common contested targets:

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`
- `internal/core/virtual_store.go` and routing splits
- `internal/shards/registration.go`
- `cmd/nerd/main.go`
- `.codex/config.toml` and `.codex/hooks.json`
- `Docs/architecture/INDEX.md`

The wiring worker validates intent shape, rejects conflicting operations, applies them sequentially, and records the final file:line plus command result.

## Failure ordering

When a level fails, do not dispatch dependent levels. Route the smallest failing command to the owning unit. After its bounded repair budget:

1. critic confirms the failure is causal;
2. dispatcher writes a remediation packet under `.quality_assurance/remediation/` using `Docs/jules-patch-remediation-prompt.md`;
3. the run remains failed/partial until a verified repair returns.
