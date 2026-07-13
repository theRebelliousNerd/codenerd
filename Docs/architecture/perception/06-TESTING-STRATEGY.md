# perception — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/perception/` (complete internal coverage)
> **Implementation: `internal/perception/` — 50 non-test .go, 48 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 50 |
| Test files | 48 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/perception/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/perception/...
go test -race ./internal/perception/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/perception/transducer_coverage_test.go` | 2080 |
| `internal/perception/xai_torture_test.go` | 1617 |
| `internal/perception/semantic_classifier_test.go` | 876 |
| `internal/perception/tracing_client_test.go` | 837 |
| `internal/perception/break_test.go` | 820 |
| `internal/perception/transducer_unit_test.go` | 820 |
| `internal/perception/transducer_llm_test.go` | 662 |
| `internal/perception/gemini_live_test.go` | 491 |
| `internal/perception/gemini_structured_test.go` | 412 |
| `internal/perception/understanding_adapter_test.go` | 395 |
