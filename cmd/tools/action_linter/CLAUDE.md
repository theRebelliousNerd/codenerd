# cmd/tools/action_linter - Action Drift Linter

This tool cross-checks action definitions across Mangle policy, router configuration, and VirtualStore to detect drift.

## Usage

```bash
go run ./cmd/tools/action_linter \
  -mg-root internal/core/defaults \
  -virtual-store internal/core/virtual_store.go
```

## File Index

| File | Description |
|------|-------------|
| `main.go` | Action drift linter cross-checking Mangle policy actions (next_action/*_next_action) against router default routes and VirtualStore ActionType values. Reports errors for unroutable actions, warnings for unused executors, and supports exemption files. |

## Cross-Checking Sources

1. **Mangle Policy** (`.mg` files): Actions emitted via `next_action` predicates
2. **Router Config**: `DefaultRouterConfig().DefaultRoutes`
3. **VirtualStore**: Supported `ActionType` constants

## Linting Rules

| Severity | Issue |
|----------|-------|
| Error | Policy emits action with no route |
| Error | Policy emits action with no executor |
| Warning | VirtualStore has executor never emitted by policy |

## Flags

| Flag | Description |
|------|-------------|
| `-mg-root` | Root directory for .mg policy files |
| `-virtual-store` | Path to virtual_store.go |
| `-fail-on-warn` | Exit non-zero on warnings |
| `-warn-unused-executors` | Warn on unused VirtualStore actions |
| `-exempt-file` | Action exemptions (glob patterns) |

## Dependencies

- `internal/shards/system` - DefaultRouterConfig

## Building

```bash
go run ./cmd/tools/action_linter
```

---

**Remember: Push to GitHub regularly!**
