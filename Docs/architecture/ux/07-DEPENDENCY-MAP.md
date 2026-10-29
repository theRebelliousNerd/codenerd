# ux — Dependency Map

> Last verified: **2026-07-13**  
> Evidence from imports and reverse-grep of `codenerd/internal/ux`.

## Upstream (what `internal/ux` imports)

```
internal/ux
├── codenerd/internal/config     # GuidanceLevel, ExperienceLevel
├── codenerd/internal/features   # IsOnboardingSkipped (migration.go only)
└── stdlib
    ├── encoding/json
    ├── fmt
    ├── os
    ├── path/filepath
    ├── slices
    ├── sync
    └── time
```

| Import | Files | Why |
|--------|-------|-----|
| `internal/config` | preferences.go, migration.go, user_state.go | Shared UX enum types defined in config package |
| `internal/features` | migration.go | Onboarding skip resolution |
| stdlib only otherwise | all | JSON persistence, FS, mutex |

**Does not import:** core, mangle, perception, articulation, session, store, shards, prompt, logging, tools.

## Downstream (who imports `internal/ux`)

| Consumer | Files (evidence) | Usage |
|----------|------------------|-------|
| `cmd/nerd/chat` | `session.go`, `session_boot.go`, `session_shared_boot.go` | Construct/load `PreferencesManager` into model / boot result |
| `cmd/nerd/chat` | `onboarding_wizard.go` | `ShouldShowOnboarding`, skip/complete, guidance |
| `cmd/nerd/chat` | `model_update.go` | `MigratePreferences` on non-first-run |
| `cmd/nerd/chat` | `model_types.go` | Field types `*ux.PreferencesManager` |
| `cmd/nerd/chat` | `help_renderer.go` | Journey → experience for progressive help |
| `cmd/nerd/chat` | `tips.go` | Journey-aware tip gating |
| `cmd/nerd/chat` | `testutil_test.go`, helpers tests | Test fixtures |

No other `internal/*` package imports `ux` (as of 2026-07-13 grep).

## Lateral / file-path competitors (not Go deps)

These do **not** import the package but touch the same artifact path:

| Package | Path | Behavior |
|---------|------|----------|
| `internal/init` | `.nerd/preferences.json` | `LoadPreferences`, agent preference save/load with **separate** types |
| `cmd/nerd/chat/session_boot_helpers.go` | preferences.json | Parse/agent-related boot messaging |
| `internal/config` | `.nerd/config.json` | Parallel onboarding/guidance configuration |

```
                ┌─────────────────┐
                │ .nerd/          │
                │ preferences.json│
                └────────┬────────┘
           ┌─────────────┼─────────────┐
           ▼             ▼             ▼
     internal/ux   internal/init   chat helpers
     (full schema) (agent slice)   (ad-hoc parse)
```

## Dependency direction vs north star

Healthy:

- UX → config types only (no cycle)  
- CLI presentation → UX  
- Kernel independent of UX  

Unhealthy if introduced:

- UX → core/kernel (would couple side channel to executive)  
- core → UX for permissioning (wrong layer)

## Suggested dependency policy

| Allowed | Disallowed without design review |
|---------|----------------------------------|
| ux → config, features, stdlib | ux → core, mangle, perception |
| cmd/* → ux | internal/session → ux for tool gates |
| Future: perception → ux only to **write** corrections | ux asserting VirtualStore actions |

## Version / module

Module path: `codenerd` (see go.mod). Package import path always `codenerd/internal/ux`.
