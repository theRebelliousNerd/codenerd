# tools — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/tools/` (complete internal coverage)
> **Implementation: `internal/tools/` — 25 non-test .go, 21 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 25 |
| Test files | 21 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/tools/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/tools/...
go test -race ./internal/tools/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/tools/research/research_coverage_test.go` | 1912 |
| `internal/tools/research/research_test.go` | 398 |
| `internal/tools/shell/execute_test.go` | 389 |
| `internal/tools/core/search_test.go` | 364 |
| `internal/tools/codedom/elements_test.go` | 359 |
| `internal/tools/core/file_ops_test.go` | 352 |
| `internal/tools/codedom/lines_test.go` | 293 |
| `internal/tools/shell/shell_integration_test.go` | 281 |
| `internal/tools/codedom/impact_test.go` | 231 |
| `internal/tools/registry_test.go` | 227 |
