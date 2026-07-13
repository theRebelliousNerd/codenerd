# features — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/features/` (complete internal coverage)
> **Implementation: `internal/features/` — 1 non-test .go, 3 tests, 0 .mg**


## Package

`internal/features/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `FeaturesConfig` | `internal/features/features.go:44` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `DefaultFeaturesConfig` | `internal/features/features.go:107` |
| `FullyEnabledFeaturesConfig` | `internal/features/features.go:131` |
| `SetActive` | `internal/features/features.go:162` |
| `Summary` | `internal/features/features.go:175` |
| `Active` | `internal/features/features.go:193` |
| `IsDiffEvalEnabled` | `internal/features/features.go:229` |
| `IsFlightRecorderEnabled` | `internal/features/features.go:237` |
| `IsProvenanceEnabled` | `internal/features/features.go:245` |
| `IsSystemShardsEnabled` | `internal/features/features.go:256` |
| `IsPerShardFactsEnabled` | `internal/features/features.go:266` |
| `IsDarkModeEnabled` | `internal/features/features.go:273` |
| `IsOnboardingSkipped` | `internal/features/features.go:279` |
| `IsTaxonomyFastEnabled` | `internal/features/features.go:285` |
| `FastScanWorkers` | `internal/features/features.go:292` |
| `FastASTMaxBytes` | `internal/features/features.go:305` |
| `Error` | `internal/features/features.go:349` |

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

This package: **Feature flags and feature configuration defaults**
