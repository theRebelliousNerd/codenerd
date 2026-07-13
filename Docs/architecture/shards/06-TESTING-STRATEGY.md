# shards — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/shards/` (complete internal coverage)
> **Implementation: `internal/shards/` — 18 non-test .go, 24 tests, 1 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 18 |
| Test files | 24 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/shards/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/shards/...
go test -race ./internal/shards/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/shards/system/base_coverage_test.go` | 1053 |
| `internal/shards/system/constitution_coverage_test.go` | 941 |
| `internal/shards/system/executive_coverage_test.go` | 819 |
| `internal/shards/consultation_test.go` | 268 |
| `internal/shards/system/mangle_repair_test.go` | 263 |
| `internal/shards/observer_integration_test.go` | 248 |
| `internal/shards/system/learning_test.go` | 248 |
| `internal/shards/system/planner_test.go` | 244 |
| `internal/shards/observer_manager_test.go` | 194 |
| `internal/shards/matching_test.go` | 168 |
