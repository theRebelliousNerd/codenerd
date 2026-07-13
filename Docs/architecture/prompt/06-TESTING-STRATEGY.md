# prompt — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/prompt/` (complete internal coverage)
> **Implementation: `internal/prompt/` — 25 non-test .go, 32 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 25 |
| Test files | 32 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/prompt/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/prompt/...
go test -race ./internal/prompt/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/prompt/atoms_test.go` | 1454 |
| `internal/prompt/compiler_test.go` | 1343 |
| `internal/prompt/budget_test.go` | 740 |
| `internal/prompt/assembler_test.go` | 738 |
| `internal/prompt/selector_test.go` | 729 |
| `internal/prompt/loader_test.go` | 622 |
| `internal/prompt/context_test.go` | 590 |
| `internal/prompt/resolver_test.go` | 587 |
| `internal/prompt/selector_gaps_test.go` | 507 |
| `internal/prompt/compiler_gaps_test.go` | 439 |
