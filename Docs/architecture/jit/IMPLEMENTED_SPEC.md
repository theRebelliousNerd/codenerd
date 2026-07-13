# jit — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/jit/` (complete internal coverage)
> **Implementation: `internal/jit/` — 1 non-test .go, 1 tests, 0 .mg**


## 1. Purpose

JIT-related config/types supporting prompt compilation

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/jit/` | Primary implementation |
| `Docs/architecture/jit/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (1 src / 1 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/jit/config/types.go` | 59 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `EffectiveAgentRuntimeConfig` | `internal/jit/config/types.go:13` |
| `ToolLoopConfig` | `internal/jit/config/types.go:25` |
| `SafetyConfig` | `internal/jit/config/types.go:31` |
| `WorkspaceConfig` | `internal/jit/config/types.go:35` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `Validate` | `internal/jit/config/types.go:51` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Bridge |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
