# context — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/context/` (complete internal coverage)
> **Implementation: `internal/context/` — 9 non-test .go, 11 tests, 1 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 9 |
| Test files | 11 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/context/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/context/...
go test -race ./internal/context/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/context/activation_test.go` | 678 |
| `internal/context/compressor_test.go` | 264 |
| `internal/context/serializer_test.go` | 202 |
| `internal/context/budget_helpers_test.go` | 152 |
| `internal/context/compressor_accessors_test.go` | 115 |
| `internal/context/token_counter_extra_test.go` | 108 |
| `internal/context/feedback_store_test.go` | 71 |
| `internal/context/activation_setters_test.go` | 63 |
| `internal/context/feedback_store_scoring_test.go` | 55 |
| `internal/context/mocks_test.go` | 49 |
