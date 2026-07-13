# ux — TODO

> Last verified: **2026-07-13**  
> Prioritized backlog for package + wiring (docs-only tracking; not an implementation commitment).

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
