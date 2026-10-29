# ux — Implemented Spec

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — code-grounded  
> Package: `codenerd/internal/ux`  
> Implementation: **4 non-test `.go`**, **4 test files**, **0 `.mg`**

## 1. Purpose

`internal/ux` is the **workspace-local UX state substrate** for codeNERD. It persists and mutates:

1. **User journey state** — where the human sits on the onboarding→mastery arc  
2. **Guidance preferences** — how much help the product should show  
3. **Local metrics** — counts that can drive automatic journey transitions  
4. **Learned patterns** — intent corrections and per-command defaults (schema ready; sparse callers)  
5. **Agent selection prefs** — accept/reject history preserved across migrations  

It deliberately sits **beside** the perception→kernel→VirtualStore loop rather than inside it. Package comment (`doc.go`):

- Non-blocking: never modifies the perception fallback chain  
- Opt-in: features can be disabled via config / feature flags  
- Respectful: existing workspaces skip onboarding and start as `productive`  
- Adaptive: guidance should decrease as experience rises  

Primary durable artifact: **`.nerd/preferences.json`** (schema version **`2.0`**, constant `PreferencesVersion`).

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Preferences schema + manager | **Implemented** | Load/Save/Get, mutex, defaults |
| Journey states + transition rules | **Implemented** | Pure logic in `UserMetrics.ShouldTransition` |
| Disclosure level mapping | **Implemented** | `GetDisclosureLevel` — **no production callers** (tests only) |
| Migration + first-run detection | **Implemented** | New vs existing vs versioned migrate |
| Onboarding complete / skip | **Implemented** | Used by chat onboarding wizard |
| Session metric recording API | **Implemented** | `RecordSessionStart` — **not wired** into chat boot |
| Automatic journey transitions | **Implemented API** | `CheckJourneyTransition` — **not wired** into chat loop |
| Intent correction learning | **Implemented API** | `RecordCorrection` — **no production caller** found |
| Metric increments (commands, errors, …) | **Implemented API** | Only `sessions_count` via `RecordSessionStart`; others uncalled |
| Experience-level mapping helper | **Implemented** | `GetExperienceLevelFromPreferences` — chat reimplements mapping inline |
| Telemetry prefs fields | **Schema only** | `Telemetry.Enabled` default false; no exporter |
| Mangle / kernel / VirtualStore | **N/A** | No Decl, no facts, no routes |
| Progressive disclosure in help/tips | **Partial (consumers)** | Chat maps journey→experience; does not call `GetDisclosureLevel` |

**Overall (heuristic):** schema + managers ~**90%**; end-to-end adaptive UX loop ~**45–55%** (storage strong, feedback wiring incomplete).

## 3. Source inventory

| Path | ~Lines | Role |
|------|-------:|------|
| `internal/ux/doc.go` | 20 | Package overview + design principles |
| `internal/ux/preferences.go` | 363 | Schema, `PreferencesManager`, defaults, metrics/corrections API |
| `internal/ux/migration.go` | 281 | Migrate, first-run, onboarding gate, session + transition helpers |
| `internal/ux/user_state.go` | 123 | Journey states, metrics transition rules, disclosure levels |
| `internal/ux/preferences_test.go` | ~100 | Defaults, load/save, metrics, corrections, onboarding steps |
| `internal/ux/migration_test.go` | ~155 | First-run, migrate new/existing, onboarding gate, session, transition, experience |
| `internal/ux/migration_extra_test.go` | ~45 | Old-version preserve + empty workspace journey |
| `internal/ux/user_state_test.go` | ~50 | `ShouldTransition`, disclosure mapping, `String()` |

No subpackages. No YAML/JSON fixtures in-package (tests use `t.TempDir()`).

## 4. Domain model (summary)

### 4.1 Journey states (`user_state.go`)

| Constant | Value | Meaning (from comments) |
|----------|-------|-------------------------|
| `StateNew` | `"new"` | First-time user (no `.nerd/` or default prefs) |
| `StateOnboarding` | `"onboarding"` | Actively in welcome wizard |
| `StateLearning` | `"learning"` | Early sessions; more guidance |
| `StateProductive` | `"productive"` | Comfortable with basics |
| `StatePower` | `"power"` | Advanced; minimal guidance |

### 4.2 Transition thresholds (`UserMetrics.ShouldTransition`)

| From | To | Condition |
|------|-----|-----------|
| `new` | `onboarding` | `CommandsExecuted >= 1` |
| `onboarding` | (none here) | Handled by onboarding complete/skip APIs |
| `learning` | `productive` | `SessionsCount >= 15` ∧ `SuccessfulTasks >= 20` ∧ clarification rate `< 0.15` |
| `productive` | `power` | `SessionsCount >= 50` ∧ `CommandsExecuted >= 200` ∧ `HelpRequests < 5` |

Clarification rate = `ClarificationsNeeded / SuccessfulTasks` when `SuccessfulTasks > 0`.

### 4.3 Disclosure levels

| Level | Typical journey |
|-------|-----------------|
| `DisclosureTutorial` | new / onboarding (default branch) |
| `DisclosureVerbose` | learning |
| `DisclosureStandard` | productive |
| `DisclosureMinimal` | power **or** `config.GuidanceNone` override |

### 4.4 Preferences document shape

Top-level `UserPreferences`:

| Field | JSON key | Purpose |
|-------|----------|---------|
| `Version` | `version` | Schema migration key (`"2.0"`) |
| `UserJourney` | `user_journey` | State, timestamps, onboarding flags, steps |
| `Guidance` | `guidance` | Level + hint toggles (`config.GuidanceLevel`) |
| `Telemetry` | `telemetry` | Opt-in flags (default off) |
| `Metrics` | `metrics` | Local counters |
| `LearnedPatterns` | `learned_patterns` | Corrections + command prefs |
| `AgentSelection` | `agent_selection` | Accepted/rejected agents, auto-accept |

Path: `{workspace}/.nerd/preferences.json` via `NewPreferencesManager(workspace)`.

### 4.5 Experience mapping

`GetExperienceLevelFromPreferences` maps:

| Journey | `config.ExperienceLevel` |
|---------|--------------------------|
| power | expert |
| productive | advanced |
| learning | intermediate |
| default (new/onboarding) | beginner |

Chat help/tips **duplicate** this switch instead of calling the helper.

## 5. Control flows

### 5.1 Preferences load/save

```
NewPreferencesManager(workspace)
        │
        ▼
     Load() ── missing file ──► DefaultUserPreferences() (in memory only)
        │ parse error ──► error (no silent recreate in Load)
        │ success ──► pm.preferences
        ▼
  mutators (SetJourneyState, IncrementMetric, …) under mu.Lock
        ▼
     Save() ── MkdirAll(.nerd) ── MarshalIndent ── WriteFile 0644
```

`Get()` is RLock and returns defaults if never loaded (does not persist).

### 5.2 Migration (`MigratePreferences`)

```
                    ┌─ no prefs file
MigratePreferences ─┤
                    │
        .nerd exists? ──yes──► createExistingUserPreferences
                    │           (state=productive, onboarding done,
                    │            sessions=20, commands=50)
                    └──no───► createNewUserPreferences (state=new)

        prefs file exists
              │
         parse fail ──► createExistingUserPreferences
              │
         version == "2.0" ──► no-op (WasMigrated=false)
              │
         else ──► migrateFromOldVersion
                    (preserve agent_selection fields;
                     force productive + onboarding complete)
```

### 5.3 Onboarding gate (`ShouldShowOnboarding`)

1. If `features.IsOnboardingSkipped()` (`NERD_SKIP_ONBOARDING` env or config `skip_onboarding`) → **false**  
2. If not first run (`.nerd` exists): load prefs; show only if `!IsOnboardingComplete()`  
3. If first run → **true**  
4. Load errors → **false** (fail closed for wizard / skip onboarding)

`IsOnboardingComplete` = `OnboardingCompleted || OnboardingSkippedAt != ""`.

### 5.4 Chat integration (consumer, not this package)

| Call site | Behavior |
|-----------|----------|
| `cmd/nerd/chat/session.go` | Construct + `Load` `PreferencesManager` into model |
| `session_boot.go` / `session_shared_boot.go` | Same at boot; attach `PreferencesMgr` to boot result |
| `onboarding_wizard.go` `checkFirstRun` | `ShouldShowOnboarding` → `onboardingCheckMsg` |
| `model_update.go` | First-run → start wizard; else `MigratePreferences` |
| wizard skip/complete | `SkipOnboarding` / `MarkOnboardingComplete` + guidance level |
| `help_renderer.go` / `tips.go` | Journey state → experience-level progressive UX |

## 6. Public API surface (export inventory)

### Types

`MigrationResult`, `UserPreferences`, `JourneyPrefs`, `GuidancePrefs`, `TelemetryPrefs`, `LearnedPatterns`, `IntentCorrection`, `CommandPrefs`, `AgentSelectionPrefs`, `PreferencesManager`, `UserJourneyState`, `UserMetrics`, `DisclosureLevel`

### Constants / vars of note

`PreferencesVersion`, `StateNew|Onboarding|Learning|Productive|Power`, `DisclosureMinimal|Standard|Verbose|Tutorial`

### Functions / methods that matter

| Symbol | File | Production use |
|--------|------|----------------|
| `NewPreferencesManager` | preferences.go | Chat boot, wizard, help, tips |
| `(*PreferencesManager).Load/Save/Get` | preferences.go | Widespread in consumers |
| `GetJourneyState` / `SetJourneyState` | preferences.go | Consumers + transition helper |
| `GetGuidanceLevel` / `SetGuidanceLevel` | preferences.go | Wizard maps experience → guidance |
| `SkipOnboarding` / `MarkOnboardingComplete` / `IsOnboardingComplete` | preferences.go | Wizard |
| `CompleteOnboardingStep` | preferences.go | Tests; not seen in wizard step list persistence beyond migration seeds |
| `IncrementMetric` / `RecordCorrection` | preferences.go | Tests only (prod) |
| `DefaultUserPreferences` | preferences.go | Internal + migration |
| `MigratePreferences` | migration.go | Chat non-first-run path |
| `IsFirstRun` | migration.go | Via `ShouldShowOnboarding` |
| `ShouldShowOnboarding` | migration.go | `checkFirstRun` |
| `GetUserJourneyState` | migration.go | Tests / potential callers |
| `RecordSessionStart` | migration.go | **Unwired** |
| `CheckJourneyTransition` | migration.go | **Unwired** |
| `GetExperienceLevelFromPreferences` | migration.go | Underused (chat inlines) |
| `ShouldTransition` | user_state.go | Via `CheckJourneyTransition` |
| `GetDisclosureLevel` | user_state.go | **Tests only** |

Unexported: `createNewUserPreferences`, `createExistingUserPreferences`, `migrateFromOldVersion` (tested via `migration_extra_test.go`).

## 7. Integration map

### Upstream (imports)

| Package | Use |
|---------|-----|
| `codenerd/internal/config` | `GuidanceLevel`, `ExperienceLevel` |
| `codenerd/internal/features` | `IsOnboardingSkipped()` |
| stdlib | `encoding/json`, `os`, `path/filepath`, `sync`, `time`, `slices`, `fmt` |

### Downstream (importers of `internal/ux`)

| Package / path | Role |
|----------------|------|
| `cmd/nerd/chat` | Primary consumer (boot, wizard, help, tips, model types) |

**Not imported by** `internal/core`, shards, perception, articulation, session executor, VirtualStore.

### Sibling / competing surfaces (wiring audit)

| Surface | Risk |
|---------|------|
| `internal/config` `OnboardingState` / `GuidanceConfig` on user config | Parallel UX state in `.nerd/config.json` vs `preferences.json` |
| `internal/init` `LoadPreferences` / agent prefs helpers | Own `UserPreferences` type + same file path — schema dualism |
| `cmd/nerd/chat/session_boot_helpers.go` | May parse `preferences.json` independently for agent selection |

## 8. Concurrency model

`PreferencesManager` uses `sync.RWMutex`:

- Write paths: `Load`, `Save`, mutators  
- Read paths: `Get`, `GetJourneyState`, `GetGuidanceLevel`, `IsOnboardingComplete`  

Caveats:

- `Get()` returns the live pointer (not a deep copy) — callers can race if they mutate fields without holding the manager lock  
- `RecordSessionStart` locks for `LastSession` then unlocks before `Save` (Save re-locks) — fine for single-threaded chat; not a multi-writer journal  
- Package-level helpers construct a **new** manager per call (no process-wide singleton)

## 9. Safety posture (package scope)

- **Not a constitutional gate.** No `permitted(...)`, no Mangle Decl.  
- Telemetry defaults **off** (`Enabled: false`, `AnonymousUsage: false`).  
- Onboarding skip via env is intentional for CI/automation.  
- Migration of corrupt JSON treats user as existing (productive) rather than new — avoids re-wizarding veterans; may overwrite corrupt data.  
- Fail-open for guidance: load failures generally mean less UX intervention, not blocked tool execution.

## 10. Gaps pointer

Deep gap analysis: [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md). Headline gaps:

1. Metrics that drive transitions are largely never incremented in production.  
2. `CheckJourneyTransition` / `RecordSessionStart` unused at session boundaries.  
3. `GetDisclosureLevel` unused; help uses experience switch instead.  
4. Dual preferences writers (`init` vs `ux`) on one JSON path.  
5. `LearnedPatterns` / intent corrections have no perception feedback loop.

## 11. Verify commands

```powershell
go test ./internal/ux/...
go test ./internal/ux/... -count=1 -v
```

## 12. North-star placement

| North-star idea | UX package role |
|-----------------|-----------------|
| LLM creative center | UX may shape **how much** guidance is shown to the human, not model prompts |
| Mangle executive | **Out of band** — must not become a second executive |
| JIT prompt atoms | Not applicable inside this package today |
| Wiring before deletion | Keep “unused” APIs until callers (or deliberate deprecation) — several are dormant by design debt, not dead code |

## 13. File → concept index

| Concept | Home |
|---------|------|
| Schema version | `PreferencesVersion` in preferences.go |
| Thread-safe I/O | `PreferencesManager` |
| Journey enum | `UserJourneyState` |
| Auto-promotion rules | `UserMetrics.ShouldTransition` |
| Progressive disclosure | `GetDisclosureLevel` |
| First-run / migrate | migration.go |
| Feature-flag skip | `features.IsOnboardingSkipped` |

---

*This document is the flagship for `Docs/architecture/ux/`. Prefer it over older inventory stubs when they conflict.*
