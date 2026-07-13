# regression — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/regression/` (complete internal coverage)
> **Implementation: `internal/regression/` — 1 non-test .go, 1 tests, 0 .mg**


## Primary package

`internal/regression/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Regression harness utilities

## How to refresh

```powershell
rg "codenerd/internal/regression" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
