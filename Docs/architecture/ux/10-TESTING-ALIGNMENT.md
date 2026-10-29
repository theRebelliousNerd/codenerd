# ux — Testing Alignment

> Last verified: **2026-07-13**

## Commands

```powershell
go test ./internal/ux/...
go test ./internal/ux/... -count=1 -v
```

Optional consumer coverage:

```powershell
go test ./cmd/nerd/chat/... -count=1
```

## Test files

| File | Tests |
|------|-------|
| `preferences_test.go` | `TestDefaultUserPreferences`, `TestPreferencesManagerLoadSave`, `TestIncrementMetric`, `TestRecordCorrection`, `TestOnboardingFlow` |
| `migration_test.go` | `TestIsFirstRun`, `TestMigratePreferencesNewUser`, `TestMigratePreferencesExistingUser`, `TestShouldShowOnboarding`, `TestRecordSessionStart`, `TestCheckJourneyTransition`, `TestGetExperienceLevelFromPreferences` |
| `migration_extra_test.go` | `TestMigrateFromOldVersionPreservesData`, `TestGetUserJourneyStateDefaultsToNew` |
| `user_state_test.go` | `TestUserMetricsShouldTransition`, `TestGetDisclosureLevel`, `TestDisclosureLevelString` |

## Coverage map (behavior × test)

| Behavior | Covered? |
|----------|----------|
| Default version + guidance + auto-accept | Yes |
| Load missing → defaults; save; reload | Yes |
| Unknown metric error | Yes |
| Correction reinforcement | Yes |
| Complete step idempotent | Yes |
| Skip → complete; Mark complete | Yes |
| First run / not first run | Yes |
| New user migrate → StateNew | Yes |
| Existing user migrate → Productive + metrics seed | Yes |
| Env skip onboarding | Yes |
| Completed user hides onboarding | Yes |
| RecordSessionStart increments + timestamp | Yes |
| Learning→Productive transition persist | Yes |
| Power → ExperienceExpert | Yes |
| Old agent_selection preserve | Yes |
| Empty workspace journey New | Yes |
| Transition new→onboarding, learning→productive, productive→power | Yes |
| Disclosure power/learning/none | Yes |
| Disclosure unknown String | Yes |
| Concurrent Load/Save | **No** |
| Corrupt JSON migrate path | **No** dedicated unit (logic exists) |
| Partial write / disk full | **No** |
| `MigratePreferences` already current version no-op | **No** explicit test |
| Integration: wizard + prefs file | In chat tests (indirect), not ux package |
| Feature skip via config file (not env) | Env tested; config path via features package tests |

## Alignment with package principles

| Principle | Test evidence |
|-----------|---------------|
| Existing users productive | `TestMigratePreferencesExistingUser` |
| Opt-in skip | `TestShouldShowOnboarding` env |
| Adaptive thresholds | `TestUserMetricsShouldTransition`, `TestCheckJourneyTransition` |
| Non-blocking defaults | Load-missing paths in manager tests |

## Gaps (testing)

1. **Golden JSON fixtures** for schema 2.0 documents (readable contract for dual writers).  
2. **Race detector** run (`-race`) on manager under parallel Get/Set.  
3. **Corrupt JSON** unit for migrate recreate path.  
4. **Current-version no-op** (`WasMigrated=false`).  
5. **End-to-end chat onboarding** asserting disk prefs after skip/complete (belongs in `cmd/nerd/chat` or integration).  
6. **Dual-writer clobber** regression once single-owner is enforced.

## Quality bar for new code

- New metric keys: unit test Increment + any transition impact.  
- Schema field: update Default + migration preserve test if needed.  
- Threshold change: update `user_state_test.go` expectations.  
- New package helper used by chat: add package test first, then consumer test.
