# features — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/features/` (complete internal coverage)
> **Implementation: `internal/features/` — 1 non-test .go, 3 tests, 0 .mg**


## 1. Purpose

Feature flags and feature configuration defaults

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/features/` | Primary implementation |
| `Docs/architecture/features/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (1 src / 3 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/features/features.go` | 350 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `FeaturesConfig` | `internal/features/features.go:44` |

### Functions (sampled)

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

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
