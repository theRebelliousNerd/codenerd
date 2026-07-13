# init — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/init/` (complete internal coverage)
> **Implementation: `internal/init/` — 16 non-test .go, 7 tests, 1 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 16 |
| Test files | 7 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/init/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/init/...
go test -race ./internal/init/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/init/init_coverage_test.go` | 1360 |
| `internal/init/typeu_coverage_test.go` | 563 |
| `internal/init/agents_knowledge_helpers_test.go` | 147 |
| `internal/init/init_test.go` | 101 |
| `internal/init/interactive_display_test.go` | 81 |
| `internal/init/scanner_dependencies_test.go` | 69 |
| `internal/init/scanner_test.go` | 61 |
