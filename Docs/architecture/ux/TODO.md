# ux — TODO

> Last verified: **2026-09-25** (lane B wave 3; status table below, original backlog kept for reference)  
> Prioritized backlog for package + wiring (docs-only tracking; not an implementation commitment).

## Status 2026-09-25

| # | Status | Evidence |
|---|--------|----------|
| 1 | Closed | `PreferencesManager.Save` (`internal/ux/preferences.go`) read-modify-writes: its own keys are replaced, every key `nerd init` and `SaveAgentPreferences` wrote is kept. In-process there is one writer: chat routes metrics and the onboarding wizard through the boot-loaded `m.preferencesMgr` (`cmd/nerd/chat/ux_journey.go` `uxPrefs`). `TestSave_WhenOtherWritersOwnKeys_ShouldKeepThem` |
| 2 | Closed | `Save` and `init.SaveAgentPreferences` write through `internal/atomicfile`; a file that does not parse is moved to `preferences.json.corrupt`, not discarded (`TestSave_WhenExistingFileIsCorrupt_ShouldMoveItAside`) |
| 3 | Closed | `openSessionRecord` runs once per chat session, after the migration, and calls `PreferencesManager.RecordSessionStart` |
| 4 | Closed | `recordUXMetric`: every submitted input (`commands_executed`), every completed turn (`successful_tasks`), every error panel (`errors_encountered`), every clarification (`clarifications_needed`) |
| 5 | Closed | `closeSessionRecord` (called first in `Model.Shutdown`) saves the counts and runs `PreferencesManager.CheckJourneyTransition`. `TestUXJourney_WhenASessionEarnsIt_ShouldMoveTheUserOn` |
| 6 | Closed | `/help` counts `help_requests`. `TestUXJourney_WhenHelpIsSubmitted_ShouldCountACommandAndAHelpRequest` |
| 7 | Open | which of the UX journey and config `OnboardingState` is authoritative is a product decision; untouched |
| 8 | Open | there is no intent-correction flow in chat to connect `RecordCorrection` to |
| 9 | Open | depends on 8 |
| 10 | Closed | `ux.ExperienceLevelForState` / `GetExperienceLevelFromPreferences` / `GetUserJourneyState`; `help_renderer.go` and `tips.go` no longer carry the switch |
| 11 | Closed | `HelpRenderer.WithGuidance` renders at `ux.GetDisclosureLevel(journey, guidance)`, so guidance "none" collapses help; the footer names the level (`DisclosureLevel.String`). `TestHelpRenderer_WhenGuidanceIsNone_ShouldShowTheMinimalReference` |
| 12 | Closed | `Get` returns a shallow copy |
| 13 | Closed | `migrateFromOldVersion` carries `learned_patterns` and `metrics` over. `TestMigratePreferences_WhenOldSchemaHasHistory_ShouldPreserveIt` |
| 14 | Closed | `openSessionRecord` logs the `MigrationResult` (or the migration error) |
| 15 | Already done | `migration_test.go`, `migration_extra_test.go`, `preferences_test.go` |
| 16-20 | Open | not started this pass |

The package-level `RecordSessionStart(workspace)` and `CheckJourneyTransition(workspace)`
became `PreferencesManager` methods: each loaded its own manager, and a second
in-process manager saving over the session's would lose one side's counts.

## P0 — Correctness

1. **Single writer for `.nerd/preferences.json`**  
   - Inventory: `internal/ux`, `internal/init`, chat boot helpers  
   - Route agent selection through `PreferencesManager` or shared RMW  
   - Add regression test that full schema survives agent-pref update  

2. **Atomic preferences write**  
   - Write temp file + rename to avoid FM-03 half-written JSON  

## P1 — Close adaptive loop

3. Call `RecordSessionStart` once per chat session open (after migrate).  
4. Increment `commands_executed` / `successful_tasks` / `errors_encountered` from real chat/executor edges.  
5. Call `CheckJourneyTransition` at session end or after N commands.  
6. Wire `/help` → `help_requests` metric.  
7. Authoritative onboarding: pick UX journey **or** config `OnboardingState`; sync the other.  

## P1 — Learning hooks

8. Connect intent-correction UX (if any) to `RecordCorrection`.  
9. Define whether corrections feed perception/retrieval as structured facts (north-star path).  

## P2 — API hygiene

10. Replace duplicated journey→experience switches in `help_renderer.go` / `tips.go` with `GetExperienceLevelFromPreferences` or shared helper.  
11. Use `GetDisclosureLevel` in help progressive render **or** delete/deprecate with comment.  
12. Defensive copy in `Get()` if multi-goroutine mutation is expected.  
13. Preserve `learned_patterns` + `metrics` in `migrateFromOldVersion`.  

## P2 — Observability & tests

14. Log `MigrationResult` at boot (Info).  
15. Unit test: current version no-op; corrupt JSON migrate; `-race` on manager.  
16. Golden JSON fixture for schema 2.0.  

## P3 — Product polish

17. User-facing override: pin journey state (stay beginner / force power).  
18. Implement or remove `TelemetryPrefs` consumer.  
19. Expose metrics summary in a status slash command (optional).  
20. Refresh `doc.go` plan reference if `noble-sprouting-emerson.md` is obsolete.  

## Explicit non-work

- Do not add Mangle rules for fuzzy UX matching.  
- Do not gate VirtualStore tools on onboarding completion.  
- Do not merge this package into `internal/config` without a design review (types can stay in config; persistence should stay clear).  
