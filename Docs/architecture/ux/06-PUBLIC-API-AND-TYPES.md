# ux — Public API and Types

> Last verified: **2026-07-13**  
> Source of truth: `internal/ux/*.go` exports only (no unexported private API listing beyond notes).

## Package

```go
package ux // import "codenerd/internal/ux"
```

## Constants

| Name | Value / type | File |
|------|--------------|------|
| `PreferencesVersion` | `"2.0"` | preferences.go |
| `StateNew` | `"new"` | user_state.go |
| `StateOnboarding` | `"onboarding"` | user_state.go |
| `StateLearning` | `"learning"` | user_state.go |
| `StateProductive` | `"productive"` | user_state.go |
| `StatePower` | `"power"` | user_state.go |
| `DisclosureMinimal` | iota 0 | user_state.go |
| `DisclosureStandard` | 1 | user_state.go |
| `DisclosureVerbose` | 2 | user_state.go |
| `DisclosureTutorial` | 3 | user_state.go |

## Types

### `UserJourneyState` (`string`)

Journey position. See constants above.

### `UserMetrics`

| Field | JSON | Role |
|-------|------|------|
| `SessionsCount` | `sessions_count` | Session starts |
| `CommandsExecuted` | `commands_executed` | Command counter |
| `ClarificationsNeeded` | `clarifications_needed` | Parse/clarification friction |
| `HelpRequests` | `help_requests` | Help usage |
| `SuccessfulTasks` | `successful_tasks` | Completions |
| `ErrorsEncountered` | `errors_encountered` | Errors |
| `LastSession` | `last_session` | RFC3339 timestamp |
| `ClarificationRate` | `clarification_rate` | Documented computed; not auto-maintained on Increment |

**Method:** `func (m *UserMetrics) ShouldTransition(current UserJourneyState) (UserJourneyState, bool)`

### `DisclosureLevel` (`int`)

**Method:** `func (d DisclosureLevel) String() string` → `minimal|standard|verbose|tutorial|unknown`

### Preference schema types

| Type | Purpose |
|------|---------|
| `UserPreferences` | Root document |
| `JourneyPrefs` | Journey + onboarding flags/steps |
| `GuidancePrefs` | Level + boolean toggles |
| `TelemetryPrefs` | Opt-in flags |
| `LearnedPatterns` | Corrections + command prefs map |
| `IntentCorrection` | Original/corrected pair + reinforcement |
| `CommandPrefs` | Default flags/target per command |
| `AgentSelectionPrefs` | Accepted/rejected agents, auto-accept |
| `PreferencesManager` | Thread-safe load/save facade |
| `MigrationResult` | Migration outcome report |

### `MigrationResult` fields

- `WasMigrated bool`  
- `FromVersion`, `ToVersion string`  
- `PreservedData []string`  
- `DefaultsApplied []string`

## PreferencesManager API

| Method | Signature (abbrev) | Notes |
|--------|--------------------|-------|
| Constructor | `NewPreferencesManager(workspace string) *PreferencesManager` | Path under `.nerd/preferences.json` |
| `Load` | `() error` | Missing → defaults, no error |
| `Save` | `() error` | Creates directory |
| `Get` | `() *UserPreferences` | Live pointer or defaults |
| `GetJourneyState` | `() UserJourneyState` | Empty state → `StateNew` |
| `SetJourneyState` | `(state) error` | Sets transition timestamp |
| `IncrementMetric` | `(metric string) error` | Known keys only |
| `RecordCorrection` | `(original, corrected string) error` | Reinforces duplicates |
| `GetGuidanceLevel` | `() config.GuidanceLevel` | Empty → `GuidanceNormal` |
| `SetGuidanceLevel` | `(level) error` | |
| `CompleteOnboardingStep` | `(step string) error` | Idempotent append |
| `SkipOnboarding` | `() error` | Sets skip time + `learning` |
| `MarkOnboardingComplete` | `() error` | Complete flag + `learning` |
| `IsOnboardingComplete` | `() bool` | Complete **or** skipped |

### Known metric keys for `IncrementMetric`

- `sessions_count`  
- `commands_executed`  
- `clarifications_needed`  
- `help_requests`  
- `successful_tasks`  
- `errors_encountered`  

Any other string → error.

## Package-level functions

| Func | Role |
|------|------|
| `DefaultUserPreferences() *UserPreferences` | New-user defaults |
| `GetDisclosureLevel(state, guidance) DisclosureLevel` | Pure mapping |
| `MigratePreferences(workspace) (*MigrationResult, error)` | Ensure schema on disk |
| `IsFirstRun(workspace) bool` | No `.nerd` directory |
| `ShouldShowOnboarding(workspace) bool` | Feature flag + first-run + complete check |
| `GetUserJourneyState(workspace) UserJourneyState` | Load or `StateNew` |
| `RecordSessionStart(workspace) error` | ++sessions + LastSession + Save |
| `CheckJourneyTransition(workspace) (UserJourneyState, bool, error)` | Maybe promote + Save |
| `GetExperienceLevelFromPreferences(workspace) config.ExperienceLevel` | Journey → experience |

## Dependencies visible in signatures

- `config.GuidanceLevel`  
- `config.ExperienceLevel`  

Callers must import `codenerd/internal/config` when setting levels.

## Stability notes

- JSON field names are a **de facto public API** for any external editor of preferences.json.  
- Unexported migrators (`migrateFromOldVersion`, etc.) are testable from the same package only.  
- `PreferencesManager` fields (`mu`, `path`, `preferences`) are unexported — correct encapsulation.
