# ux — Failure Modes

> Last verified: **2026-07-13**

## FM-01: Missing preferences file

| | |
|--|--|
| **Trigger** | First load; file never created |
| **Behavior** | `Load` installs in-memory defaults; no error |
| **User impact** | None until something needs durable state |
| **Mitigation** | `MigratePreferences` or `Save` after mutations; wizard Save paths |

## FM-02: Unwritable `.nerd` directory

| | |
|--|--|
| **Trigger** | Permissions, disk full, path on read-only FS |
| **Behavior** | `Save` / migrate create paths return error |
| **User impact** | Onboarding state may not persist; next run re-prompts or re-defaults |
| **Mitigation** | Chat often ignores Save errors (`_ =`); surface error to user on critical wizard complete |

## FM-03: Corrupt / invalid JSON

| | |
|--|--|
| **Trigger** | Manual edit, crash mid-write, dual writer partial file |
| **Behavior** | `Load` returns parse error; `MigratePreferences` recreates **existing-user** productive prefs |
| **User impact** | Loss of learned patterns / custom guidance; no re-onboarding funnel |
| **Mitigation** | Atomic write (temp + rename); backup `.bak` before recreate; single writer |

## FM-04: Version skew / unknown version string

| | |
|--|--|
| **Trigger** | Future version or empty version field |
| **Behavior** | Any version ≠ `"2.0"` enters `migrateFromOldVersion` |
| **User impact** | Forced productive; agent_selection preserved if present; other fields may reset to defaults |
| **Mitigation** | Explicit per-version migrators; preserve `learned_patterns` and `metrics` |

## FM-05: Dual-writer clobber

| | |
|--|--|
| **Trigger** | `init` saves narrow agent prefs after `ux` saved full schema (or reverse) |
| **Behavior** | Last marshal wins; missing fields drop |
| **User impact** | Lost journey/metrics or lost agent selection |
| **Mitigation** | Single owner RMW; typed union schema |

## FM-06: Onboarding skip flag stuck

| | |
|--|--|
| **Trigger** | `NERD_SKIP_ONBOARDING=1` left set in shell profile |
| **Behavior** | `ShouldShowOnboarding` always false |
| **User impact** | New users never see wizard (may be desired for CI) |
| **Mitigation** | Document flag; prefer features config for permanent opt-out |

## FM-07: Load error hides incomplete onboarding

| | |
|--|--|
| **Trigger** | Prefs exist but unreadable; not first run |
| **Behavior** | `ShouldShowOnboarding` → false |
| **User impact** | User who never finished onboarding never retried |
| **Mitigation** | Distinguish parse error vs missing; warn in TUI |

## FM-08: Journey never advances

| | |
|--|--|
| **Trigger** | Metrics not incremented; `CheckJourneyTransition` never called |
| **Behavior** | State frozen at new/learning/productive seed |
| **User impact** | Help density wrong for long-term users who started as new |
| **Mitigation** | Wire session/command metrics (P1 gap) |

## FM-09: Shared pointer mutation races

| | |
|--|--|
| **Trigger** | Goroutine A reads `Get()` while B mutates fields without lock |
| **Behavior** | Data race / torn reads |
| **User impact** | Corrupt in-memory prefs; bad Save |
| **Mitigation** | Defensive copy on Get; only mutators write |

## FM-10: RecordSessionStart without prior Save defaults

| | |
|--|--|
| **Trigger** | `Load` missing file leaves defaults in memory; Start increments and Saves |
| **Behavior** | Actually creates file — usually fine |
| **User impact** | May create prefs before migration existing-user logic runs if ordered wrong |
| **Mitigation** | Boot order: migrate first, then record session |

## FM-11: Wizard Save ignored errors

| | |
|--|--|
| **Trigger** | Disk failure during skip/complete |
| **Behavior** | UI shows complete message; disk unchanged |
| **User impact** | Wizard reappears next launch (if first-run still true) or state inconsistent |
| **Mitigation** | Check Save error; show toast |

## FM-12: Guidance dual-write inconsistency

| | |
|--|--|
| **Trigger** | Wizard updates prefs + in-memory Config; only one persisted |
| **Behavior** | Restart may show different guidance than last session end |
| **User impact** | Confusing tip/help density |
| **Mitigation** | Single authoritative store; config derived or synced |

## Severity summary

| ID | Severity | Likelihood today |
|----|----------|------------------|
| FM-05 dual writer | High | Medium (init + ux coexist) |
| FM-08 no advance | Medium | High (by design gap) |
| FM-03 corrupt | Medium | Low–medium |
| FM-11 ignored Save | Medium | Low |
| FM-06 skip stuck | Low | Env-dependent |
| FM-09 races | Low | Chat mostly single-threaded UI |
