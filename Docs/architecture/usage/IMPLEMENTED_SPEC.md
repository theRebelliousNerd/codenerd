# usage — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/usage/` (complete internal coverage)
> **Implementation: `internal/usage/` — 2 non-test .go, 4 tests, 0 .mg**


## 1. Purpose

Usage / token accounting helpers

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/usage/` | Primary implementation |
| `Docs/architecture/usage/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (2 src / 4 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/usage/usage_tracker.go` | 210 | source |
| `internal/usage/usage_types.go` | 47 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `Tracker` | `internal/usage/usage_tracker.go:17` |
| `UsageData` | `internal/usage/usage_types.go:6` |
| `UsageEvent` | `internal/usage/usage_types.go:13` |
| `AggregatedStats` | `internal/usage/usage_types.go:26` |
| `TokenCounts` | `internal/usage/usage_types.go:36` |

### Functions (sampled)

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
