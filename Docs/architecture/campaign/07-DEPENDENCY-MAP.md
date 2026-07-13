# campaign — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/campaign/` (complete internal coverage)
> **Implementation: `internal/campaign/` — 44 non-test .go, 29 tests, 1 .mg**


## Primary package

`internal/campaign/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Multi-phase goal orchestration, decomposition, context paging

## How to refresh

```powershell
rg "codenerd/internal/campaign" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
