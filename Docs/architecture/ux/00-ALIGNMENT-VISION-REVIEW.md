# ux — Alignment / Vision Review

> Last verified: **2026-07-13**  
> Scored against codeNERD north star + package `doc.go` design principles.  
> Scale: **1–5** (5 = fully aligned with evidence in tree).

## Summary

| Dimension | Score | One-line |
|-----------|------:|----------|
| Inversion of control (LLM creative / logic executive) | 4 | Correctly stays off executive path |
| Constitutional safety | 3 | Not a gate; defaults are privacy-safe |
| Non-blocking observation | 5 | Package and callers do not block OODA |
| Existing-user respect | 5 | Migration → productive + onboarding complete |
| Adaptive guidance (stated goal) | 2 | Rules exist; metric feed + transition loop unwired |
| Single source of UX truth | 2 | Dual with config onboarding + init preferences |
| JIT / prompt discipline | n/a | No LLM-facing surface in package |
| Testability | 4 | Solid unit coverage of core paths |
| Observability | 2 | No package logs/metrics; only local counters |
| Wiring completeness | 2 | Boot/onboarding yes; learning loop partial |

**Weighted overall: ~3.2 / 5** — strong substrate, incomplete productization.

## Dimension detail

### 1. Inversion of control — **4/5**

**Evidence:** No imports of `internal/core`, no Mangle, no VirtualStore routes. Chat uses UX only for presentation and preference persistence.

**Gap:** None architectural; remaining risk is future feature creep asserting journey state into the kernel without Decl/policy review.

### 2. Constitutional safety — **3/5**

**Evidence:** Telemetry off by default; onboarding skip is explicit feature flag; no elevated privileges; file writes under workspace `.nerd/`.

**Gap:** Not part of `permitted(...)`. Corrupt-prefs migration overwrites without backup. `Get()` returns shared pointer (mutation safety).

### 3. Non-blocking / parallel concern — **5/5**

**Evidence:** `doc.go` principle 1; boot continues after preference load warnings; wizard is a TUI mode, not a kernel gate for tool execution after skip/complete.

### 4. Existing-user respect — **5/5**

**Evidence:** `createExistingUserPreferences` sets `StateProductive`, `OnboardingCompleted=true`, seeded steps including `existing_user_migration`; invalid JSON also routes to existing-user path. Tests: `TestMigratePreferencesExistingUser`.

### 5. Adaptive guidance — **2/5**

**Evidence for intent:** `ShouldTransition` thresholds; `GetDisclosureLevel`; guidance prefs in schema.

**Evidence of shortfall:** Grep shows no production `RecordSessionStart`, `CheckJourneyTransition`, `RecordCorrection`, or `GetDisclosureLevel` callers outside tests. Journey rarely advances automatically in real use.

### 6. Single source of truth — **2/5**

**Evidence of split:**

- `internal/config` carries `OnboardingState`, `GuidanceConfig` on user config  
- `internal/ux` carries journey + guidance in preferences.json  
- `internal/init` reads/writes agent selection on same path with its own types  

Wizard updates both UX prefs guidance and (in `model_update.go`) `m.Config.Guidance` on complete — dual write of related concepts.

### 7. JIT prompt atoms — **n/a**

Package does not assemble prompts. Chat progressive help is hand-written markdown, not atom-selected. Future “contextual help via LLM” would need atoms under `internal/prompt/atoms/`, not growth of this package’s Go strings.

### 8. Testability — **4/5**

Unit tests cover defaults, persistence, metrics, corrections, first-run, migration branches, env skip, transitions, disclosure. Missing: concurrent Load/Save stress; corrupt partial JSON preservation; integration with chat wizard (lives in `cmd/nerd/chat`).

### 9. Observability — **2/5**

No `logging.Category*` usage in `internal/ux`. Boot logs preference load failure in chat (`CategoryBoot`). Local metrics are product analytics, not ops telemetry.

### 10. Wiring completeness — **2/5**

| Wire | Present? |
|------|----------|
| PreferencesManager at chat boot | Yes |
| ShouldShowOnboarding | Yes |
| MigratePreferences | Yes (non-first-run) |
| Skip/complete onboarding | Yes |
| Help/tips journey read | Yes |
| Session start metrics | No |
| Auto journey transition | No |
| Intent correction capture | No |
| DisclosureLevel consumers | No |

## North-star verdict

**Keep** this package as a **thin, durable UX memory**. Do **not** move executive policy here. Prioritize closing the adaptive loop (metric writers + transition checks) and consolidating preferences ownership before inventing new journey states.

## Evidence anchors

| Claim | Path |
|-------|------|
| Design principles | `internal/ux/doc.go` |
| Schema + manager | `internal/ux/preferences.go` |
| Transitions | `internal/ux/user_state.go` |
| Migration / onboarding | `internal/ux/migration.go` |
| Skip flag | `internal/features/features.go` (`IsOnboardingSkipped`) |
| Chat consumers | `cmd/nerd/chat/{session,session_boot,session_shared_boot,onboarding_wizard,model_update,help_renderer,tips}.go` |
