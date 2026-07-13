# ux — Architectural Principles

> Last verified: **2026-07-13**  
> Binding for work in `internal/ux` and its callers. Derived from `doc.go` + north star + observed code.

## 1. Side channel, never executive

UX state may **influence presentation** (help density, tips, wizard). It must **not** decide action permission, VirtualStore routing, or Mangle derivation. If journey must affect policy, assert a reviewed general-purpose fact through the kernel path with `Decl` and `permitted` — do not special-case inside this package.

## 2. Non-blocking observation

Preference load failure is a warning, not a fatal boot error. Missing files yield defaults. Onboarding is skippable. Never insert synchronous UX I/O on the critical path of tool execution without an explicit product decision.

## 3. Existing users are not new users

If `.nerd/` already exists, migration and first-run logic treat the human as established (`productive`, onboarding complete). Schema upgrades must not re-trigger the welcome wizard.

## 4. Versioned preferences document

`PreferencesVersion` (`"2.0"`) is the contract. Any field shape change either:

- remains backward compatible, or  
- bumps version and extends `migrateFromOldVersion` (preserve what can be preserved).

## 5. Opt-in for surveillance-like behavior

Telemetry fields default **false**. Local metrics for product adaptation are acceptable; network export requires explicit enablement. Do not “phone home” from this package.

## 6. Deterministic journey rules

Promotion thresholds live in pure Go (`ShouldTransition`), covered by tests. Do not ask an LLM “is this user productive now?” for state changes.

## 7. Fail open toward less guidance on error

If preferences cannot load, prefer skipping onboarding noise / using safe defaults over blocking the workspace. (`ShouldShowOnboarding` returns false on load error for non-first-run paths.)

## 8. One writer for `.nerd/preferences.json` (target; enforce toward)

All durable mutations should go through `PreferencesManager` (or a future shared API it owns). Partial-struct marshals from other packages violate this principle and are technical debt.

## 9. Thread safety at the manager boundary

Mutations lock. Callers must not mutate the pointer returned by `Get()` without coordinating locks (prefer mutator methods). Future work: return defensive copies if multi-goroutine writers appear.

## 10. Adaptive guidance decreases over time

Guidance and disclosure should trend toward quieter UX as metrics show competence. Manual override via `GuidanceLevel` / `GuidanceNone` always wins over journey-derived verbosity.

## 11. Prefer reusing mapping helpers

Do not fork journey→experience or journey→disclosure switches in every consumer. Call `GetExperienceLevelFromPreferences` / `GetDisclosureLevel` or extract a shared mapper used by both package and chat.

## 12. Wiring audit before deletion

Dormant APIs (`RecordSessionStart`, `CheckJourneyTransition`, `RecordCorrection`, `GetDisclosureLevel`) may be unfinished product loops. Grep callers and plan wiring before removing “unused” symbols.

## Anti-principles

- Growing this package into a second config system for engines/providers.  
- Embedding large natural-language tip banks as Mangle rules.  
- Blocking tool use until onboarding completes when user requested skip or CI set skip flag.  
- Silent network telemetry.
