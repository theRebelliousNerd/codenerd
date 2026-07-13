# autopoiesis — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/autopoiesis/` (complete internal coverage)
> **Implementation: `internal/autopoiesis/` — 37 non-test .go, 30 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 37 |
| Test files | 30 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/autopoiesis/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/autopoiesis/...
go test -race ./internal/autopoiesis/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/autopoiesis/prompt_evolution/prompt_evolution_test.go` | 1479 |
| `internal/autopoiesis/feedback_test.go` | 1059 |
| `internal/autopoiesis/persistence_test.go` | 887 |
| `internal/autopoiesis/ouroboros_test.go` | 865 |
| `internal/autopoiesis/toolgen_test.go` | 745 |
| `internal/autopoiesis/complexity_test.go` | 551 |
| `internal/autopoiesis/quality_test.go` | 508 |
| `internal/autopoiesis/thunderdome_harness_test.go` | 408 |
| `internal/autopoiesis/templates_coverage_test.go` | 252 |
| `internal/autopoiesis/helpers_coverage_test.go` | 220 |
