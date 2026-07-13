# jit — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/jit/` (complete internal coverage)
> **Implementation: `internal/jit/` — 1 non-test .go, 1 tests, 0 .mg**


## Package

`internal/jit/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `EffectiveAgentRuntimeConfig` | `internal/jit/config/types.go:13` |
| `ToolLoopConfig` | `internal/jit/config/types.go:25` |
| `SafetyConfig` | `internal/jit/config/types.go:31` |
| `WorkspaceConfig` | `internal/jit/config/types.go:35` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `Validate` | `internal/jit/config/types.go:51` |

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

This package: **JIT-related config/types supporting prompt compilation**
