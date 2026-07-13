# ux — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/ux/` (complete internal coverage)
> **Implementation: `internal/ux/` — 4 non-test .go, 4 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 4 |
| Test files | 4 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/ux/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/ux/...
go test -race ./internal/ux/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/ux/migration_test.go` | 156 |
| `internal/ux/preferences_test.go` | 102 |
| `internal/ux/user_state_test.go` | 50 |
| `internal/ux/migration_extra_test.go` | 46 |
