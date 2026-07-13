# observability — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/observability/` (complete internal coverage)
> **Implementation: `internal/observability/` — 2 non-test .go, 3 tests, 0 .mg**


## Primary package

`internal/observability/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Flight recorder and runtime metrics

## How to refresh

```powershell
rg "codenerd/internal/observability" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
