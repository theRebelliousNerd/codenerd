# articulation — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/articulation/` (complete internal coverage)
> **Implementation: `internal/articulation/` — 8 non-test .go, 7 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 8 |
| Test files | 7 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/articulation/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/articulation/...
go test -race ./internal/articulation/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/articulation/prompt_assembler_test.go` | 941 |
| `internal/articulation/emitter_test.go` | 523 |
| `internal/articulation/emitter_boundary_test.go` | 316 |
| `internal/articulation/json_scanner_test.go` | 239 |
| `internal/articulation/emitter_extra_test.go` | 86 |
| `internal/articulation/emitter_helpers_test.go` | 86 |
| `internal/articulation/stream_parser_test.go` | 46 |
