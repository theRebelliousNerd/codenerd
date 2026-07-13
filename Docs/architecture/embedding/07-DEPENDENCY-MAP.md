# embedding — Dependency Map

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/embedding/` (complete internal coverage)
> **Implementation: `internal/embedding/` — 6 non-test .go, 7 tests, 0 .mg**


## Primary package

`internal/embedding/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: Embedding engines (including Ollama) and vector generation

## How to refresh

```powershell
rg "codenerd/internal/embedding" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
