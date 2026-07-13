# verification — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/verification/` (complete internal coverage)
> **Implementation: `internal/verification/` — 1 non-test .go, 3 tests, 0 .mg**


## Package

`internal/verification/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `QualityViolation` | `internal/verification/verifier.go:29` |
| `CorrectiveType` | `internal/verification/verifier.go:43` |
| `CorrectiveAction` | `internal/verification/verifier.go:53` |
| `ShardSelectionResult` | `internal/verification/verifier.go:61` |
| `VerificationResult` | `internal/verification/verifier.go:70` |
| `TaskVerifier` | `internal/verification/verifier.go:81` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `SetTaskExecutor` | `internal/verification/verifier.go:95` |
| `NewTaskVerifier` | `internal/verification/verifier.go:173` |
| `SetSessionContext` | `internal/verification/verifier.go:188` |
| `VerifyWithRetry` | `internal/verification/verifier.go:199` |

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

This package: **Verification utilities for agent outputs**
