# campaign — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/campaign/` (complete internal coverage)
> **Implementation: `internal/campaign/` — 44 non-test .go, 29 tests, 1 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 44 |
| Test files | 29 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/campaign/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/campaign/...
go test -race ./internal/campaign/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/campaign/context_pager_test.go` | 1260 |
| `internal/campaign/decomposer_test.go` | 1058 |
| `internal/campaign/shard_advisory_board_test.go` | 722 |
| `internal/campaign/replan_test.go` | 653 |
| `internal/campaign/orchestrator_task_handlers_test.go` | 629 |
| `internal/campaign/risk_scoring_test.go` | 611 |
| `internal/campaign/types_test.go` | 600 |
| `internal/campaign/assault_tasks_test.go` | 543 |
| `internal/campaign/edge_case_detector_test.go` | 461 |
| `internal/campaign/tool_pregenerator_test.go` | 416 |
