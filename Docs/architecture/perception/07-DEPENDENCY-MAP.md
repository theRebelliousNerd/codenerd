# perception — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/perception/` (complete internal coverage)
> **Implementation: `internal/perception/` — 50 non-test .go, 48 tests, 0 .mg**


## Primary package

`internal/perception/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: NL→atoms transduction, semantic classification, LLM clients

## How to refresh

```powershell
rg "codenerd/internal/perception" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
