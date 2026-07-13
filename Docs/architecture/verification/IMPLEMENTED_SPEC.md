# verification — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/verification/` (complete internal coverage)
> **Implementation: `internal/verification/` — 1 non-test .go, 3 tests, 0 .mg**


## 1. Purpose

Verification utilities for agent outputs

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/verification/` | Primary implementation |
| `Docs/architecture/verification/` | This full corpus |

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
| `internal/verification/verifier.go` | 819 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `QualityViolation` | `internal/verification/verifier.go:29` |
| `CorrectiveType` | `internal/verification/verifier.go:43` |
| `CorrectiveAction` | `internal/verification/verifier.go:53` |
| `ShardSelectionResult` | `internal/verification/verifier.go:61` |
| `VerificationResult` | `internal/verification/verifier.go:70` |
| `TaskVerifier` | `internal/verification/verifier.go:81` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `SetTaskExecutor` | `internal/verification/verifier.go:95` |
| `NewTaskVerifier` | `internal/verification/verifier.go:173` |
| `SetSessionContext` | `internal/verification/verifier.go:188` |
| `VerifyWithRetry` | `internal/verification/verifier.go:199` |

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
