# shards — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/shards/` (complete internal coverage)
> **Implementation: `internal/shards/` — 18 non-test .go, 24 tests, 1 .mg**


## Primary package

`internal/shards/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Domain and system shard implementations + registration

## How to refresh

```powershell
rg "codenerd/internal/shards" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
