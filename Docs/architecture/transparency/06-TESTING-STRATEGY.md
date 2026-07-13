# transparency — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/transparency/` (complete internal coverage)
> **Implementation: `internal/transparency/` — 8 non-test .go, 9 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 8 |
| Test files | 9 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/transparency/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/transparency/...
go test -race ./internal/transparency/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/transparency/transparency_comprehensive_test.go` | 901 |
| `internal/transparency/event_bus_test.go` | 140 |
| `internal/transparency/explainer_test.go` | 94 |
| `internal/transparency/shard_observer_test.go` | 90 |
| `internal/transparency/glass_box_helpers_test.go` | 63 |
| `internal/transparency/transparency_test.go` | 56 |
| `internal/transparency/glass_box_events_test.go` | 54 |
| `internal/transparency/safety_reporter_test.go` | 49 |
| `internal/transparency/error_classifier_test.go` | 42 |
