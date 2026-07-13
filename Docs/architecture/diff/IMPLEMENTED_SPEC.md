# diff — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/diff/` (complete internal coverage)
> **Implementation: `internal/diff/` — 1 non-test .go, 2 tests, 0 .mg**


## 1. Purpose

Diff utilities for code change analysis

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/diff/` | Primary implementation |
| `Docs/architecture/diff/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (1 src / 2 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/diff/diff.go` | 378 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `LineType` | `internal/diff/diff.go:42` |
| `Line` | `internal/diff/diff.go:52` |
| `Hunk` | `internal/diff/diff.go:59` |
| `FileDiff` | `internal/diff/diff.go:68` |
| `Engine` | `internal/diff/diff.go:78` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `NewEngine` | `internal/diff/diff.go:90` |
| `ComputeDiff` | `internal/diff/diff.go:107` |
| `ComputeDiff` | `internal/diff/diff.go:163` |
| `ClearCache` | `internal/diff/diff.go:368` |
| `ComputeWordLevelDiff` | `internal/diff/diff.go:374` |

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
