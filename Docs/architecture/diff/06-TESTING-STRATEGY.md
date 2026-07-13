# diff — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/diff/` (complete internal coverage)
> **Implementation: `internal/diff/` — 1 non-test .go, 2 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 1 |
| Test files | 2 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/diff/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/diff/...
go test -race ./internal/diff/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/diff/diff_test.go` | 483 |
| `internal/diff/diff_comprehensive_test.go` | 465 |
