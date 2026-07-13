# browser — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/browser/` (complete internal coverage)
> **Implementation: `internal/browser/` — 3 non-test .go, 6 tests, 0 .mg**


## Primary package

`internal/browser/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Browser automation / Rod session management and honeypot surfaces

## How to refresh

```powershell
rg "codenerd/internal/browser" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
