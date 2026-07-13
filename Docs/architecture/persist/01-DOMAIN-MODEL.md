# persist — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/persist/` (complete internal coverage)
> **Implementation: `internal/persist/` — 1 non-test .go, 4 tests, 0 .mg**


## Package

`internal/persist/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `Codec` | `internal/persist/factsnap/factsnap.go:48` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `Write` | `internal/persist/factsnap/factsnap.go:61` |
| `WriteCodec` | `internal/persist/factsnap/factsnap.go:67` |
| `Read` | `internal/persist/factsnap/factsnap.go:153` |
| `LegacyJSON` | `internal/persist/factsnap/factsnap.go:184` |
| `CanonicalPath` | `internal/persist/factsnap/factsnap.go:198` |

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

This package: **Persistence helpers bridging stores and runtime**
