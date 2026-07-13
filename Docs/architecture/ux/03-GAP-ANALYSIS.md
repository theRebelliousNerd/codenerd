# ux — Gap Analysis

> Last verified: **2026-07-13**  
> Spec/vision vs living code. Priorities: P0 (correctness/safety), P1 (product loop), P2 (hygiene).

## Matrix

| Area | Vision / stated design | Reality | Gap? | Priority |
|------|------------------------|---------|------|----------|
| Preferences schema 2.0 | Durable UX prefs | Implemented + versioned | No | — |
| Non-blocking | Never block OODA | True | No | — |
| Existing user skip wizard | Productive + complete | Implemented + tested | No | — |
| Feature-flag skip | CI / automation | `NERD_SKIP_ONBOARDING` | No | — |
| Session metrics | Drive transitions | `RecordSessionStart` unwired | **Yes** | P1 |
| Command/success/error metrics | Populate counters | `IncrementMetric` only tested | **Yes** | P1 |
| Auto journey transition | Promote learning→productive→power | `CheckJourneyTransition` unwired | **Yes** | P1 |
| Onboarding→learning on first command | `ShouldTransition(new)` | No chat call increments `commands_executed` | **Yes** | P1 |
| Progressive disclosure helper | `GetDisclosureLevel` | Tests only; help reimplements | **Yes** | P2 |
| Experience helper reuse | Single map | Chat inlines switch in help/tips | **Yes** | P2 |
| Intent corrections | Learn from user | No production `RecordCorrection` | **Yes** | P1 |
| Command preferences map | Per-command defaults | Schema only | **Yes** | P2 |
| Telemetry | Optional anonymous | Fields only, no client | Partial (safe) | P2 |
| Single prefs owner | One writer | `init` + `ux` + ad-hoc chat parse | **Yes** | P0 |
| Shared pointer safety | Immutable reads | `Get()` returns live struct | **Yes** | P2 |
| Corrupt JSON | Preserve / backup | Recreate productive | Soft gap | P2 |
| Config dual onboarding | One truth | config + preferences | **Yes** | P1 |
| Mangle involvement | None required | None | Non-gap | — |
| Kernel integration | Observe only | Observe only | Non-gap | — |

## Non-gaps (do not “fix” as bugs)

- **No Mangle Decl in package** — correct for a non-executive UX store.  
- **Telemetry default false** — intentional privacy posture.  
- **Onboarding handled in `cmd/nerd/chat`** — UX supplies state; TUI owns UX interaction.  
- **Small package size** — not a maturity failure; substrate is intentionally thin.

## P0: Preferences file dual ownership

**Risk:** `internal/init` load/save agent preferences and `internal/ux` full-schema save can clobber fields when marshaling different structs over the same path.

**Mitigation direction:** Make `ux` the sole schema owner; init calls `PreferencesManager` or shared DTO; never marshal a partial type over the full document without read-modify-write of unknown keys.

## P1: Adaptive loop open circuit

```
[metrics writers] ──X──► counters ──X──► CheckJourneyTransition ──► disclosure/help
       │                     ▲
       └──── currently missing at chat/session edges
```

Without writers, users stay at whatever state migration or wizard left them (`new`/`learning`/`productive` seed). Wizard can set guidance level once; automatic mastery progression does not happen.

**Concrete wire candidates (docs only — not implementing):**

- Chat session start → `RecordSessionStart`  
- Successful task completion / executor result → `successful_tasks` / `errors_encountered`  
- Clarification UI → `clarifications_needed`  
- `/help` → `help_requests`  
- Periodic or post-turn → `CheckJourneyTransition`

## P1: Intent learning unused

`RecordCorrection` + reinforcement count are tested but never called from perception correction or chat “that’s not what I meant” flows. North star prefers embeddings/retrieval then structured facts — this API is a reasonable local cache but is orphaned.

## P1: Config vs preferences dualism

Onboarding completion can exist only in one place while the other still says “incomplete,” depending on which subsystem is consulted. Align:

- Either UX journey is authoritative for onboarding gate (today `ShouldShowOnboarding` uses UX),  
- Or config `OnboardingState.SetupComplete` is derived, not independently authoritative.

## P2: API consolidation

| Duplicate logic | Locations |
|-----------------|-----------|
| Journey → experience | `GetExperienceLevelFromPreferences` vs `help_renderer.go` vs `tips.go` |
| Guidance vs disclosure | `GetDisclosureLevel` vs help category switch |

Either wire the helpers or mark them as library API for external tools and stop reimplementing.

## Spec vs reality heatmap

| Completeness | Areas |
|--------------|-------|
| Strong | Schema, manager, migration, first-run, skip flag, unit tests |
| Medium | Chat onboarding integration, progressive help by experience |
| Weak | Metric pipeline, auto transitions, learned patterns, telemetry export, single-writer guarantee |

## Recommended sequencing

1. **P0** — audit all `preferences.json` writers; funnel through `ux` or shared RMW.  
2. **P1** — wire `RecordSessionStart` + one success/error metric + `CheckJourneyTransition` at session end.  
3. **P1** — document authoritative onboarding fields; demote config duplicate or sync on write.  
4. **P2** — collapse disclosure/experience helpers into chat.  
5. **P2** — decide fate of `LearnedPatterns` (wire or freeze as reserved schema).
