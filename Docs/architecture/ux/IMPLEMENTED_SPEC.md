# ux — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/ux/` (complete internal coverage)
> **Implementation: `internal/ux/` — 4 non-test .go, 4 tests, 0 .mg**


## 1. Purpose

UX helpers for CLI presentation

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/ux/` | Primary implementation |
| `Docs/architecture/ux/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (4 src / 4 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/ux/preferences.go` | 363 | source |
| `internal/ux/migration.go` | 281 | source |
| `internal/ux/user_state.go` | 123 | source |
| `internal/ux/doc.go` | 20 | source |

### Types (sampled)

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

### Functions (sampled)

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
