# logging — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/logging/` (complete internal coverage)
> **Implementation: `internal/logging/` — 4 non-test .go, 5 tests, 0 .mg**


## Primary package

`internal/logging/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Categorized logging system for debug/diagnostics

## How to refresh

```powershell
rg "codenerd/internal/logging" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
