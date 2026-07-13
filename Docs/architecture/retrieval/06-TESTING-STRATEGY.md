# retrieval — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/retrieval/` (complete internal coverage)
> **Implementation: `internal/retrieval/` — 4 non-test .go, 6 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 4 |
| Test files | 6 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/retrieval/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/retrieval/...
go test -race ./internal/retrieval/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/retrieval/tiered_context_coverage_test.go` | 357 |
| `internal/retrieval/sparse_test.go` | 293 |
| `internal/retrieval/sparse_integration_test.go` | 205 |
| `internal/retrieval/sparse_search_test.go` | 205 |
| `internal/retrieval/tiered_context_test.go` | 75 |
| `internal/retrieval/sparse_bench_test.go` | 17 |
