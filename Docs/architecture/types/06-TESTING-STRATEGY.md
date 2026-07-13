# types — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/types/` (complete internal coverage)
> **Implementation: `internal/types/` — 5 non-test .go, 4 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 5 |
| Test files | 4 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/types/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/types/...
go test -race ./internal/types/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/types/types_comprehensive_test.go` | 528 |
| `internal/types/extract_test.go` | 221 |
| `internal/types/types_test.go` | 174 |
| `internal/types/shard_test.go` | 19 |
