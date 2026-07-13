# usage — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/usage/` (complete internal coverage)
> **Implementation: `internal/usage/` — 2 non-test .go, 4 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 2 |
| Test files | 4 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/usage/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/usage/...
go test -race ./internal/usage/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/usage/usage_comprehensive_test.go` | 354 |
| `internal/usage/usage_tracker_test.go` | 299 |
| `internal/usage/usage_types_test.go` | 78 |
| `internal/usage/usage_tracker_context_test.go` | 34 |
