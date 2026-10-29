# ux — Current State

> Last verified: **2026-07-13**  
> Inventory is 1:1 with `internal/ux/` on disk.

## Package footprint

```
internal/ux/
├── doc.go                 # package docs / principles
├── preferences.go         # schema + PreferencesManager
├── migration.go           # migrate, first-run, session helpers
├── user_state.go          # journey, metrics rules, disclosure
├── preferences_test.go
├── migration_test.go
├── migration_extra_test.go
└── user_state_test.go
```

| Metric | Value |
|--------|------:|
| Non-test Go files | 4 |
| Test Go files | 4 |
| Mangle / YAML / embed | 0 |
| Subpackages | 0 |
| Approx. production LOC | ~787 (`doc` 20 + `preferences` 363 + `migration` 281 + `user_state` 123) |

## File roles

### `doc.go` (20 lines)

Documents purpose: journey tracking, progressive disclosure, contextual help triggers, preferences schema, migration. States four design principles (non-blocking, opt-in, respectful, adaptive). References external plan file name `noble-sprouting-emerson.md` (historical plan; not a live package path).

### `preferences.go` (363 lines) — hotspot

- Schema version constant `PreferencesVersion = "2.0"`  
- Full JSON-tagged preference tree  
- `PreferencesManager` with path `{workspace}/.nerd/preferences.json`  
- CRUD-style methods: Load, Save, Get, journey, guidance, metrics, corrections, onboarding steps  
- `DefaultUserPreferences()` — new-user defaults (telemetry off, guidance normal, journey `new`)

### `migration.go` (281 lines) — hotspot

- `MigratePreferences` decision tree (new / existing / current / old / corrupt)  
- `IsFirstRun`, `ShouldShowOnboarding`  
- Workspace helpers: `GetUserJourneyState`, `RecordSessionStart`, `CheckJourneyTransition`, `GetExperienceLevelFromPreferences`  
- Integrates `features.IsOnboardingSkipped()`

### `user_state.go` (123 lines)

- `UserJourneyState` string enum  
- `UserMetrics` + `ShouldTransition` thresholds  
- `DisclosureLevel` + `GetDisclosureLevel` + `String()`

## Hotspots for change

1. **preferences.go** — any schema field addition bumps version and migration.  
2. **migration.go** — existing-user heuristics and feature-flag gate.  
3. **user_state.go** — transition thresholds are product-sensitive.

## Behavioral inventory (what works today)

| Behavior | Status in tree |
|----------|----------------|
| Create default prefs for empty workspace | Yes (`Load` in-memory; `MigratePreferences` can persist) |
| Persist prefs to disk | Yes (`Save`) |
| Existing `.nerd` without prefs → productive | Yes |
| Version match short-circuit | Yes |
| Preserve `agent_selection` on old migrate | Yes |
| Env skip onboarding | Yes (`NERD_SKIP_ONBOARDING`) |
| Wizard skip / complete → disk | Yes (chat) |
| Progressive help from journey | Yes (chat maps state → experience) |
| Auto journey promotion in sessions | **API only** |
| Session counter on chat open | **API only** (`RecordSessionStart` unused) |
| Intent correction reinforcement | **API only** |
| DisclosureLevel in help | **Unused** |

## Test inventory

| File | Covers |
|------|--------|
| `preferences_test.go` | Defaults, load/save guidance, metrics, corrections, onboarding steps/skip/complete |
| `migration_test.go` | First run, new/existing migrate, onboarding gate+env, session start, transition, experience map |
| `migration_extra_test.go` | Old version preserve; empty → StateNew |
| `user_state_test.go` | Transition cases; disclosure; unknown String |

## External coupling snapshot

```
internal/ux
  ├── config      (GuidanceLevel, ExperienceLevel)
  └── features    (IsOnboardingSkipped)

cmd/nerd/chat ──imports──► internal/ux
internal/init ──same path──► preferences.json (own types; not import ux)
```

## Known dualisms (current, not aspirational)

1. **Guidance** lives in both `UserPreferences.Guidance` and `config.GuidanceConfig`.  
2. **Onboarding** flags exist in UX journey prefs and `config.OnboardingState`.  
3. **Experience** can be derived from journey (`GetExperienceLevelFromPreferences`) or set in onboarding wizard / config.  
4. **Agent selection** JSON keys are written by init helpers and preserved by UX migration — two owners.

## Line-count ranking (production)

1. `preferences.go` — 363  
2. `migration.go` — 281  
3. `user_state.go` — 123  
4. `doc.go` — 20  

Small package: almost all “architecture” is these four files; no secondary subsystems.
