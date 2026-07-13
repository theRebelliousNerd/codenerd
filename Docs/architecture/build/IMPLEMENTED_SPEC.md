# build — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/build/` (complete internal coverage)
> **Implementation: `internal/build/` — 1 non-test .go, 2 tests, 0 .mg**


## 1. Purpose

Build-time environment helpers (CGO/sqlite flags, build env)

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/build/` | Primary implementation |
| `Docs/architecture/build/` | This full corpus |

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
| `internal/build/env.go` | 312 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `BuildConfig` | `internal/build/env.go:23` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `DefaultBuildConfig` | `internal/build/env.go:36` |
| `GetBuildEnv` | `internal/build/env.go:52` |
| `GetBuildEnvForTest` | `internal/build/env.go:90` |
| `GetBuildEnvForCompile` | `internal/build/env.go:104` |
| `MergeEnv` | `internal/build/env.go:300` |

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
