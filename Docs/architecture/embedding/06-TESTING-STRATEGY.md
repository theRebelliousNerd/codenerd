# embedding — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/embedding/` (complete internal coverage)
> **Implementation: `internal/embedding/` — 6 non-test .go, 7 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 6 |
| Test files | 7 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/embedding/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/embedding/...
go test -race ./internal/embedding/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/embedding/ollama_coverage_test.go` | 486 |
| `internal/embedding/engine_coverage_test.go` | 448 |
| `internal/embedding/task_selector_coverage_test.go` | 409 |
| `internal/embedding/ollama_ensure_test.go` | 176 |
| `internal/embedding/genai_coverage_test.go` | 124 |
| `internal/embedding/task_selector_test.go` | 60 |
| `internal/embedding/genai_bench_test.go` | 44 |
