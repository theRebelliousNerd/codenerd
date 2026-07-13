# usage — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/usage/` (complete internal coverage)
> **Implementation: `internal/usage/` — 2 non-test .go, 4 tests, 0 .mg**


## Package

`internal/usage/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `Tracker` | `internal/usage/usage_tracker.go:17` |
| `UsageData` | `internal/usage/usage_types.go:6` |
| `UsageEvent` | `internal/usage/usage_types.go:13` |
| `AggregatedStats` | `internal/usage/usage_types.go:26` |
| `TokenCounts` | `internal/usage/usage_types.go:36` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `NewTracker` | `internal/usage/usage_tracker.go:26` |
| `Load` | `internal/usage/usage_tracker.go:56` |
| `Save` | `internal/usage/usage_tracker.go:93` |
| `Track` | `internal/usage/usage_tracker.go:108` |
| `Stats` | `internal/usage/usage_tracker.go:161` |
| `NewContext` | `internal/usage/usage_tracker.go:191` |
| `FromContext` | `internal/usage/usage_tracker.go:196` |
| `WithShardContext` | `internal/usage/usage_tracker.go:205` |
| `Add` | `internal/usage/usage_types.go:43` |

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

This package: **Usage / token accounting helpers**
