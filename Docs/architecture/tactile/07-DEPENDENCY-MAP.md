# tactile — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/tactile/` (complete internal coverage)
> **Implementation: `internal/tactile/` — 16 non-test .go, 12 tests, 0 .mg**


## Primary package

`internal/tactile/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Tactile routing / action-to-tool surfaces

## How to refresh

```powershell
rg "codenerd/internal/tactile" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
