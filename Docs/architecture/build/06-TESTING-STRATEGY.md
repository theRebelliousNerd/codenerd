# build — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/build/` (complete internal coverage)
> **Implementation: `internal/build/` — 1 non-test .go, 2 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 1 |
| Test files | 2 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/build/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/build/...
go test -race ./internal/build/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/build/env_gaps_test.go` | 427 |
| `internal/build/env_test.go` | 211 |
