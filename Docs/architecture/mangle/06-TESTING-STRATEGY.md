# mangle — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/mangle/` (complete internal coverage)
> **Implementation: `internal/mangle/` — 21 non-test .go, 39 tests, 1 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 21 |
| Test files | 39 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/mangle/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/mangle/...
go test -race ./internal/mangle/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/mangle/torture_test.go` | 2825 |
| `internal/mangle/mangle_validation_test.go` | 1394 |
| `internal/mangle/engine_test.go` | 885 |
| `internal/mangle/synth/validate_test.go` | 867 |
| `internal/mangle/feedback/feedback_test.go` | 711 |
| `internal/mangle/synth/compile_test.go` | 691 |
| `internal/mangle/feedback/pre_validator_test.go` | 393 |
| `internal/mangle/synth/schema_test.go` | 353 |
| `internal/mangle/schema_validator_test.go` | 316 |
| `internal/mangle/feedback/types_test.go` | 293 |
