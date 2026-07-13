# core — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/core/` (complete internal coverage)
> **Implementation: `internal/core/` — 78 non-test .go, 107 tests, 129 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 78 |
| Test files | 107 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/core/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/core/...
go test -race ./internal/core/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/core/virtual_store_workflows_coverage_test.go` | 1378 |
| `internal/core/virtual_store_actions_coverage_test.go` | 1355 |
| `internal/core/virtual_store_codedom_coverage_test.go` | 1265 |
| `internal/core/coverage_boost_test.go` | 1092 |
| `internal/core/kernel_facts_gaps_test.go` | 944 |
| `internal/core/transaction_manager_test.go` | 858 |
| `internal/core/api_scheduler_test.go` | 767 |
| `internal/core/transaction_manager_gaps_test.go` | 715 |
| `internal/core/validator_paranoid_test.go` | 682 |
| `internal/core/virtual_store_python_test.go` | 677 |
