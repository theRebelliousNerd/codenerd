# northstar — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/northstar/` (complete internal coverage)
> **Implementation: `internal/northstar/` — 4 non-test .go, 6 tests, 0 .mg**


## Primary package

`internal/northstar/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: North-star goal tracking and alignment helpers

## How to refresh

```powershell
rg "codenerd/internal/northstar" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
