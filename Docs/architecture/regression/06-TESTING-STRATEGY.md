# regression — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/regression/` (complete internal coverage)
> **Implementation: `internal/regression/` — 1 non-test .go, 1 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 1 |
| Test files | 1 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/regression/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/regression/...
go test -race ./internal/regression/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/regression/battery_test.go` | 102 |
