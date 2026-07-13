# persist — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/persist/` (complete internal coverage)
> **Implementation: `internal/persist/` — 1 non-test .go, 4 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 1 |
| Test files | 4 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/persist/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/persist/...
go test -race ./internal/persist/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/persist/factsnap/factsnap_test.go` | 243 |
| `internal/persist/factsnap/codec_parity_test.go` | 120 |
| `internal/persist/factsnap/factsnap_codec_test.go` | 54 |
| `internal/persist/factsnap/legacy_test.go` | 41 |
