# session — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/session/` (complete internal coverage)
> **Implementation: `internal/session/` — 6 non-test .go, 14 tests, 0 .mg**


## Primary package

`internal/session/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Session execution loop and clean executor

## How to refresh

```powershell
rg "codenerd/internal/session" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
