# ux — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/ux/` (complete internal coverage)
> **Implementation: `internal/ux/` — 4 non-test .go, 4 tests, 0 .mg**


## Package

`internal/ux/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `MigrationResult` | `internal/ux/migration.go:15` |
| `UserPreferences` | `internal/ux/preferences.go:19` |
| `JourneyPrefs` | `internal/ux/preferences.go:43` |
| `GuidancePrefs` | `internal/ux/preferences.go:52` |
| `TelemetryPrefs` | `internal/ux/preferences.go:60` |
| `LearnedPatterns` | `internal/ux/preferences.go:66` |
| `IntentCorrection` | `internal/ux/preferences.go:72` |
| `CommandPrefs` | `internal/ux/preferences.go:80` |
| `AgentSelectionPrefs` | `internal/ux/preferences.go:86` |
| `PreferencesManager` | `internal/ux/preferences.go:94` |
| `UserJourneyState` | `internal/ux/user_state.go:8` |
| `UserMetrics` | `internal/ux/user_state.go:28` |
| `DisclosureLevel` | `internal/ux/user_state.go:93` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `MigratePreferences` | `internal/ux/migration.go:26` |
| `IsFirstRun` | `internal/ux/migration.go:183` |
| `ShouldShowOnboarding` | `internal/ux/migration.go:190` |
| `GetUserJourneyState` | `internal/ux/migration.go:211` |
| `RecordSessionStart` | `internal/ux/migration.go:220` |
| `CheckJourneyTransition` | `internal/ux/migration.go:238` |
| `GetExperienceLevelFromPreferences` | `internal/ux/migration.go:262` |
| `NewPreferencesManager` | `internal/ux/preferences.go:101` |
| `Load` | `internal/ux/preferences.go:108` |
| `Save` | `internal/ux/preferences.go:132` |
| `Get` | `internal/ux/preferences.go:159` |
| `GetJourneyState` | `internal/ux/preferences.go:170` |
| `SetJourneyState` | `internal/ux/preferences.go:179` |
| `IncrementMetric` | `internal/ux/preferences.go:194` |
| `RecordCorrection` | `internal/ux/preferences.go:223` |
| `GetGuidanceLevel` | `internal/ux/preferences.go:255` |
| `SetGuidanceLevel` | `internal/ux/preferences.go:264` |
| `CompleteOnboardingStep` | `internal/ux/preferences.go:277` |
| `SkipOnboarding` | `internal/ux/preferences.go:299` |
| `MarkOnboardingComplete` | `internal/ux/preferences.go:315` |
| `IsOnboardingComplete` | `internal/ux/preferences.go:331` |
| `DefaultUserPreferences` | `internal/ux/preferences.go:337` |
| `ShouldTransition` | `internal/ux/user_state.go:40` |
| `GetDisclosureLevel` | `internal/ux/user_state.go:74` |
| `String` | `internal/ux/user_state.go:110` |

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

This package: **UX helpers for CLI presentation**
