# session — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/session/` (complete internal coverage)
> **Implementation: `internal/session/` — 6 non-test .go, 14 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 6 |
| Test files | 14 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/session/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/session/...
go test -race ./internal/session/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/session/executor_process_test.go` | 835 |
| `internal/session/task_executor_test.go` | 563 |
| `internal/session/executor_test.go` | 306 |
| `internal/session/subagent_test.go` | 279 |
| `internal/session/spawner_test.go` | 278 |
| `internal/session/mocks_test.go` | 269 |
| `internal/session/spawner_improvements_test.go` | 263 |
| `internal/session/spawner_gaps_test.go` | 258 |
| `internal/session/semantic_compressor_test.go` | 181 |
| `internal/session/executor_boundary_test.go` | 178 |
