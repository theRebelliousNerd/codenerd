# persist — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/persist/` (complete internal coverage)
> **Implementation: `internal/persist/` — 1 non-test .go, 4 tests, 0 .mg**


## 1. Purpose

Persistence helpers bridging stores and runtime

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/persist/` | Primary implementation |
| `Docs/architecture/persist/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (1 src / 4 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/persist/factsnap/factsnap.go` | 287 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `Codec` | `internal/persist/factsnap/factsnap.go:48` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `Write` | `internal/persist/factsnap/factsnap.go:61` |
| `WriteCodec` | `internal/persist/factsnap/factsnap.go:67` |
| `Read` | `internal/persist/factsnap/factsnap.go:153` |
| `LegacyJSON` | `internal/persist/factsnap/factsnap.go:184` |
| `CanonicalPath` | `internal/persist/factsnap/factsnap.go:198` |

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
