# regression — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/regression/` (complete internal coverage)
> **Implementation: `internal/regression/` — 1 non-test .go, 1 tests, 0 .mg**


## Package

`internal/regression/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `Battery` | `internal/regression/battery.go:20` |
| `Task` | `internal/regression/battery.go:27` |
| `Result` | `internal/regression/battery.go:35` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `LoadBattery` | `internal/regression/battery.go:44` |
| `RunBattery` | `internal/regression/battery.go:58` |
| `DefaultBatteryPath` | `internal/regression/battery.go:136` |

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

This package: **Regression harness utilities**
