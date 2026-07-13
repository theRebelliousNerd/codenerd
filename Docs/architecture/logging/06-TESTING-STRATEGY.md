# logging — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/logging/` (complete internal coverage)
> **Implementation: `internal/logging/` — 4 non-test .go, 5 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 4 |
| Test files | 5 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/logging/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/logging/...
go test -race ./internal/logging/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/logging/coverage_boost_test.go` | 1208 |
| `internal/logging/logging_comprehensive_test.go` | 743 |
| `internal/logging/logger_test.go` | 451 |
| `internal/logging/audit_coverage_test.go` | 326 |
| `internal/logging/audit_benchmark_test.go` | 30 |
