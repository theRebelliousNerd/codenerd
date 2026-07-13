# world — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/world/` (complete internal coverage)
> **Implementation: `internal/world/` — 37 non-test .go, 31 tests, 1 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 37 |
| Test files | 31 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/world/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/world/...
go test -race ./internal/world/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/world/parser_test.go` | 1248 |
| `internal/world/holographic_test.go` | 951 |
| `internal/world/dataflow_test.go` | 802 |
| `internal/world/scan_edge_test.go` | 520 |
| `internal/world/fs_test.go` | 458 |
| `internal/world/dataflow_cache_test.go` | 452 |
| `internal/world/dataflow_multilang_test.go` | 452 |
| `internal/world/test_dependency_test.go` | 356 |
| `internal/world/parser_factory_test.go` | 344 |
| `internal/world/ast_test.go` | 297 |
