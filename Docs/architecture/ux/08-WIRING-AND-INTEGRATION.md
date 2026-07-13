# ux — Wiring and Integration

> Last verified: **2026-07-13**  
> How `internal/ux` is registered/called in the running binary. No kernel registration — pure library used by chat.

## Boot wiring

### Interactive session construction (`cmd/nerd/chat/session.go`)

1. `prefsMgr := ux.NewPreferencesManager(workspace)`  
2. `prefsMgr.Load()` — warn/print on error, continue  
3. Store on `Model.preferencesMgr`

### Async/full boot (`session_boot.go`, `session_shared_boot.go`)

Same pattern during boot pipeline; result includes:

```text
PreferencesMgr: prefsMgr  // on bootCompleteMsg / shared boot struct
```

Boot also logs via `logging.CategoryBoot` when load fails (chat side, not ux package).

## First-run / migration wiring

```
boot complete
    │
    ▼
checkFirstRun(workspace)  ──tea.Cmd──►  ux.ShouldShowOnboarding(workspace)
    │
    ▼
onboardingCheckMsg
    │
    ├── IsFirstRun true  ──► startOnboarding()  (TUI wizard)
    │
    └── IsFirstRun false ──► ux.MigratePreferences(workspace)  // silent
```

Sources:

- `onboarding_wizard.go` — `checkFirstRun`  
- `model_update.go` — `onboardingCheckMsg` / `onboardingCompleteMsg` handlers  
- Triggered after boot path that batches `checkFirstRun` (`model_update.go` ~line 614 area)

## Onboarding wizard write path

| User action | UX API |
|-------------|--------|
| Skip | `SkipOnboarding` + `Save` |
| Complete | `MarkOnboardingComplete` + optional `SetGuidanceLevel` from experience choice + `Save` |

Guidance mapping on complete (wizard):

| Experience chosen | Guidance set |
|-------------------|--------------|
| beginner | `GuidanceVerbose` |
| intermediate | `GuidanceNormal` |
| advanced / expert | `GuidanceMinimal` |

`model_update.go` also mirrors guidance into `m.Config.Guidance` when present — dual write to config object in memory.

## Progressive presentation wiring

### Help (`help_renderer.go`)

`NewHelpRenderer(workspace)`:

1. `NewPreferencesManager` + `Load`  
2. `GetJourneyState` → map to `config.ExperienceLevel`  
3. `renderProgressiveHelp` switches on experience  

Does **not** call `GetDisclosureLevel` or `GetExperienceLevelFromPreferences`.

### Tips (`tips.go`)

`NewTipGenerator(workspace)` same journey→experience mapping.

- `StatePower` → never show tips  
- New/beginner → higher random tip probability  
- Rate limit 1 minute  

## Feature-flag wiring

```
features.IsOnboardingSkipped()
  env: NERD_SKIP_ONBOARDING
  config: features.skip_onboarding
  default: false
        │
        ▼
ux.ShouldShowOnboarding  early return false
```

Documented on `internal/features/features.go` as serving chat first-run wizard / `internal/ux`.

## What is **not** wired

| API | Expected integration point | Status |
|-----|----------------------------|--------|
| `RecordSessionStart` | Chat open / session ID create | Absent |
| `CheckJourneyTransition` | End of turn / session | Absent |
| `IncrementMetric("commands_executed")` | After command | Absent |
| `IncrementMetric("successful_tasks")` | Executor success | Absent |
| `IncrementMetric("help_requests")` | `/help` | Absent |
| `RecordCorrection` | Intent correction UX | Absent |
| `GetDisclosureLevel` | Help/tips | Absent |
| `CompleteOnboardingStep` | Wizard step tracker | Migration seeds only / tests |
| PreferencesMgr on Model | Continuous mutators during chat | Mostly load-at-boot; wizard creates new managers |

## OODA / VirtualStore

```
user_intent → perception → kernel → next_action → VirtualStore → articulation
                                                              ▲
                                              no UX facts or routes
```

UX is not a VirtualStore backend and does not register shards.

## Init package interaction

`internal/init` reads/writes agent selection keys on the same JSON path **without** importing `ux`. Migration in `ux` knows how to preserve `agent_selection` when upgrading old documents. Runtime race: if init saves a narrow struct after ux saved full schema, fields can be lost (gap P0).

## Registration checklist (for implementers)

- [x] Chat model holds PreferencesManager  
- [x] First-run check uses ShouldShowOnboarding  
- [x] MigratePreferences for existing users  
- [x] Wizard mutates onboarding + guidance  
- [ ] Session metrics loop  
- [ ] Transition check loop  
- [ ] Single writer for preferences.json  
- [ ] Shared disclosure helper in help/tips  

## Related corpora

- CLI corpus for TUI modes and slash commands  
- Features corpus for skip flag resolution  
- Config corpus for parallel guidance/onboarding types  
- Init corpus for competing preferences writers  
