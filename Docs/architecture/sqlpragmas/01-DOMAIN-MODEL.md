# sqlpragmas — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/sqlpragmas/` (complete internal coverage)
> **Implementation: `internal/sqlpragmas/` — 1 non-test .go, 2 tests, 0 .mg**


## Package

`internal/sqlpragmas/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `PragmaProfile` | `internal/sqlpragmas/pragmas.go:26` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `ApplyDefaultPragmas` | `internal/sqlpragmas/pragmas.go:60` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **SQLite pragma helpers for safe DB open**
