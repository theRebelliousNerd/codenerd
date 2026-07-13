# diff — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/diff/` (complete internal coverage)
> **Implementation: `internal/diff/` — 1 non-test .go, 2 tests, 0 .mg**


## Package

`internal/diff/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `LineType` | `internal/diff/diff.go:42` |
| `Line` | `internal/diff/diff.go:52` |
| `Hunk` | `internal/diff/diff.go:59` |
| `FileDiff` | `internal/diff/diff.go:68` |
| `Engine` | `internal/diff/diff.go:78` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `NewEngine` | `internal/diff/diff.go:90` |
| `ComputeDiff` | `internal/diff/diff.go:107` |
| `ComputeDiff` | `internal/diff/diff.go:163` |
| `ClearCache` | `internal/diff/diff.go:368` |
| `ComputeWordLevelDiff` | `internal/diff/diff.go:374` |

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

This package: **Diff utilities for code change analysis**
