# store — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/store/` (complete internal coverage)
> **Implementation: `internal/store/` — 39 non-test .go, 44 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 39 |
| Test files | 44 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/store/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/store/...
go test -race ./internal/store/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/store/trace_store_test.go` | 571 |
| `internal/store/vector_store_search_test.go` | 524 |
| `internal/store/vector_store_batch_test.go` | 393 |
| `internal/store/vector_store_test.go` | 370 |
| `internal/store/archival_test.go` | 328 |
| `internal/store/trace_store_integration_test.go` | 300 |
| `internal/store/cold_storage_integration_test.go` | 247 |
| `internal/store/local_graph_test.go` | 224 |
| `internal/store/tool_cleanup_extra_test.go` | 223 |
| `internal/store/local_session_integration_test.go` | 173 |
