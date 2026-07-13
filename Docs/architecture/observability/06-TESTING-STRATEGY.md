# observability — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/observability/` (complete internal coverage)
> **Implementation: `internal/observability/` — 2 non-test .go, 3 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 2 |
| Test files | 3 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/observability/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/observability/...
go test -race ./internal/observability/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/observability/flight_recorder_lifecycle_test.go` | 147 |
| `internal/observability/flight_recorder_test.go` | 121 |
| `internal/observability/runtime_metrics_test.go` | 101 |
