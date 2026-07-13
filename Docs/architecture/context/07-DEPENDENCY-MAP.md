# context — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/context/` (complete internal coverage)
> **Implementation: `internal/context/` — 9 non-test .go, 11 tests, 1 .mg**


## Primary package

`internal/context/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Context activation, scoring, and window management

## How to refresh

```powershell
rg "codenerd/internal/context" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
