# ux — Safety and Invariants

> Last verified: **2026-07-13**

## Scope of safety

This package is **not** a constitutional gate. Safety here means:

- Workspace filesystem integrity  
- Privacy defaults  
- Concurrent access correctness  
- Not interfering with action authorization  

Kernel-level `permitted(...)` / default-deny lives elsewhere (`internal/core` policy).

## Invariants

### I1 — Preferences path is workspace-local

Path is always `filepath.Join(workspace, ".nerd", "preferences.json")`. No absolute-user-home global store inside this package.

### I2 — Schema version identity

`UserPreferences.Version` for new docs is `PreferencesVersion` (`"2.0"`). Migration sets `ToVersion` to that constant.

### I3 — Existing `.nerd` never treated as brand-new onboarding target for migration create path

When prefs missing but `.nerd` exists → productive + onboarding complete. Invalid JSON → same.

### I4 — Onboarding complete is dual-condition

`IsOnboardingComplete` ≡ `OnboardingCompleted || OnboardingSkippedAt != ""`.

### I5 — Telemetry off by default

`DefaultUserPreferences` sets `Telemetry.Enabled=false` and `AnonymousUsage=false`.

### I6 — Known metrics only

`IncrementMetric` rejects unknown names (prevents silent no-ops / schema pollution).

### I7 — GuidanceNone forces minimal disclosure

`GetDisclosureLevel` short-circuits to `DisclosureMinimal` when guidance is none, regardless of journey.

### I8 — Mutex covers manager state

All Load/Save/mutators take the manager lock. Package helpers that need multi-step ops take locks carefully (`RecordSessionStart` updates LastSession under lock then Save).

### I9 — Feature-flag skip wins

If onboarding is feature-skipped, wizard must not show (`ShouldShowOnboarding` first check).

### I10 — No action denial

No API returns “refuse to run tools.” Failures degrade UX features only.

## Concurrency hazards

| Hazard | Detail | Mitigation |
|--------|--------|------------|
| Shared `Get()` pointer | Callers can mutate fields without lock | Prefer mutator methods; document read-only |
| Multiple manager instances | Last writer wins on disk | Hold single manager per session where possible |
| Partial external writers | `init` marshaling subset | Single-owner principle (gap) |

## Privacy

- Local metrics (sessions, commands, …) stay on disk under workspace.  
- Intent corrections store free text of original/corrected parse — treat as potentially sensitive workspace data (same class as chat history).  
- No network client in package.

## Filesystem permissions

- Directory create: `0755`  
- File write: `0644`  
- No secret key storage in this schema (API keys belong in config/env).

## Failure policy (safety-relevant)

| Event | Policy |
|-------|--------|
| Corrupt preferences JSON on migrate | Recreate productive (may drop data) — prefer continuity of use over empty new-user funnel |
| Load error on onboarding check | Do not force wizard |
| Save error in wizard | Errors discarded with `_ =` in chat — user may not persist onboarding (acceptability tradeoff) |

## Constitutional boundary

Do **not** implement:

- “User must complete onboarding before `permitted(tool)`” inside this package  
- Journey state as a hard policy input without Mangle Decl + review  

If product later requires “beginner sandbox,” that belongs in policy with explicit facts, not hidden in preferences.json alone.

## Mangle surface

**None.** No Decl, no rules, no aggregation. Non-gap.
