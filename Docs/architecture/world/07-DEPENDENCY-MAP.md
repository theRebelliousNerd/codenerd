# world — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/world/` (complete internal coverage)
> **Implementation: `internal/world/` — 37 non-test .go, 31 tests, 1 .mg**


## Primary package

`internal/world/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: World model: filesystem topology, AST/symbol projection

## How to refresh

```powershell
rg "codenerd/internal/world" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
