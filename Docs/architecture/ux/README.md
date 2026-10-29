# ux — Architecture Corpus (`internal/ux`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Source package: `internal/ux/`  
> Mangle sources: **none** (no local `.mg`)

## Scope

This corpus documents **user-experience state and progressive guidance storage** for codeNERD:

- Workspace-scoped preferences at `.nerd/preferences.json` (schema version `2.0`)
- User journey state machine (`new` → `onboarding` → `learning` → `productive` → `power`)
- Onboarding gate / migration helpers used by the chat TUI
- Disclosure / experience mapping helpers for progressive help

It is **not**:

- The Bubble Tea TUI itself (`cmd/nerd/chat`, `cmd/nerd/ui`) — see [cli corpus](../cli/)
- User config / engines (`internal/config`) — guidance *types* live there; journey *persistence* lives here
- Kernel, VirtualStore, shards, or prompt JIT — UX is a **side channel**, not on the OODA hot path

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Flagship living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machine |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and functions |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream / downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Chat boot, onboarding, help, tips |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Concurrency, privacy, fail-open rules |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, coverage, gaps |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging / metrics surface (honest) |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild log |

> Note: older auto-inventory stubs (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-UX.md`, etc.) may still exist in this folder from prior generation passes. **This README and the table above are authoritative** for the rebuild contract.

## Role in fact-flow

```
user_intent → kernel → next_action → VirtualStore → articulation
                    ↑
              UX is parallel: observes / stores preference state;
              never asserts Mangle facts or routes actions.
```

Chat boot loads `PreferencesManager`; onboarding and progressive help **read** journey state. The executive path does not consult `internal/ux`.

## Verify

```powershell
go test ./internal/ux/...
```

Optional (callers):

```powershell
go test ./cmd/nerd/chat/... -count=1
```

## Package size (2026-07-13)

| Kind | Count |
|------|------:|
| Non-test `.go` | 4 |
| Test `.go` | 4 |
| `.mg` | 0 |
| Approx. production LOC | ~790 |

## Related corpora

- [cli](../cli/) — TUI surfaces that consume UX state
- [config](../config/) — `GuidanceLevel`, `ExperienceLevel`, `OnboardingState` types
- [features](../features/) — `NERD_SKIP_ONBOARDING` / `SkipOnboarding` flag
- [init](../init/) — separate `preferences.json` readers (agent selection; dual schema risk)
