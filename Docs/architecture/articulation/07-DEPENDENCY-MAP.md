# articulation — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/articulation/` (complete internal coverage)
> **Implementation: `internal/articulation/` — 8 non-test .go, 7 tests, 0 .mg**


## Primary package

`internal/articulation/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Atoms→NL emission, Piggyback protocol, prompt assembly bridge

## How to refresh

```powershell
rg "codenerd/internal/articulation" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
