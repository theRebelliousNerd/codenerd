# regression — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/regression/` (complete internal coverage)
> **Implementation: `internal/regression/` — 1 non-test .go, 1 tests, 0 .mg**


## 1. Purpose

Regression harness utilities

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/regression/` | Primary implementation |
| `Docs/architecture/regression/` | This full corpus |

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
| `internal/regression/battery.go` | 138 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `Battery` | `internal/regression/battery.go:20` |
| `Task` | `internal/regression/battery.go:27` |
| `Result` | `internal/regression/battery.go:35` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `LoadBattery` | `internal/regression/battery.go:44` |
| `RunBattery` | `internal/regression/battery.go:58` |
| `DefaultBatteryPath` | `internal/regression/battery.go:136` |

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
