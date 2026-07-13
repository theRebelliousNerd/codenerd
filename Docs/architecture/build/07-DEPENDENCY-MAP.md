# build — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/build/` (complete internal coverage)
> **Implementation: `internal/build/` — 1 non-test .go, 2 tests, 0 .mg**


## Primary package

`internal/build/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Build-time environment helpers (CGO/sqlite flags, build env)

## How to refresh

```powershell
rg "codenerd/internal/build" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
