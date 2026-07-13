# testing — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/testing/` (complete internal coverage)
> **Implementation: `internal/testing/` — 21 non-test .go, 8 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 21 |
| Test files | 8 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/testing/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/testing/...
go test -race ./internal/testing/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/testing/context_harness/feedback_test.go` | 286 |
| `internal/testing/context_harness/simulator_test.go` | 168 |
| `internal/testing/context_harness/tracer_helpers_test.go` | 97 |
| `internal/testing/context_harness/seeder_logger_test.go` | 89 |
| `internal/testing/context_harness/reporter_test.go` | 68 |
| `internal/testing/context_harness/file_logger_test.go` | 64 |
| `internal/testing/context_harness/metrics_test.go` | 57 |
| `internal/testing/context_harness/helpers_extra_test.go` | 35 |
