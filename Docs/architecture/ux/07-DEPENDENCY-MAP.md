# ux — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/ux/` (complete internal coverage)
> **Implementation: `internal/ux/` — 4 non-test .go, 4 tests, 0 .mg**


## Primary package

`internal/ux/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: UX helpers for CLI presentation

## How to refresh

```powershell
rg "codenerd/internal/ux" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
