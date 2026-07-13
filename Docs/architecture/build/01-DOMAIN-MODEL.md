# build — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/build/` (complete internal coverage)
> **Implementation: `internal/build/` — 1 non-test .go, 2 tests, 0 .mg**


## Package

`internal/build/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `BuildConfig` | `internal/build/env.go:23` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `DefaultBuildConfig` | `internal/build/env.go:36` |
| `GetBuildEnv` | `internal/build/env.go:52` |
| `GetBuildEnvForTest` | `internal/build/env.go:90` |
| `GetBuildEnvForCompile` | `internal/build/env.go:104` |
| `MergeEnv` | `internal/build/env.go:300` |

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

This package: **Build-time environment helpers (CGO/sqlite flags, build env)**
