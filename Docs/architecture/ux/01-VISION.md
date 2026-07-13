# ux — Vision

> Last verified: **2026-07-13**  
> This is the **target** product/architecture vision for `internal/ux`, not a claim that every item is already wired.

## Product intent

codeNERD should feel smart without being loud:

- **New users** get a short, skippable onboarding and tutorial-density help.  
- **Returning builders** never re-do a wizard because a schema changed.  
- **Power users** get quiet UI, dense commands, no tip spam.  
- **The system** adapts over time from *observed behavior* (sessions, success, clarifications, help), not from a one-time self-report alone.

UX state is the **durable memory of the human’s relationship with the product**, stored next to the workspace (`.nerd/preferences.json`), not in cloud profiles by default.

## Architectural placement

```
┌──────────────────────────────────────────────────────────┐
│  cmd/nerd/chat  (presentation: help, tips, wizards)      │
└─────────────┬────────────────────────────────────────────┘
              │ read/write PreferencesManager
              ▼
┌──────────────────────────────────────────────────────────┐
│  internal/ux  (journey, guidance prefs, local metrics)   │
└─────────────┬────────────────────────────────────────────┘
              │ optional future: structured signals
              ▼
┌──────────────────────────────────────────────────────────┐
│  OODA loop (perception → kernel → VirtualStore → …)      │
│  ← must remain independent; UX never becomes executive   │
└──────────────────────────────────────────────────────────┘
```

### Target principles (product)

1. **Observe, don’t seize control** — metrics and journey never deny `permitted` actions.  
2. **One preferences document** — single schema owner for `.nerd/preferences.json`.  
3. **Progressive disclosure as a pure function** of journey + guidance override.  
4. **Learning from corrections** — when the user fixes a parse/intent, record it and feed retrieval/perception as structured facts later (not free-text fuzzy Mangle rules).  
5. **Privacy first** — telemetry remains opt-in; local metrics stay local unless user enables export.  
6. **Deterministic transitions** — thresholds in code/tests, not LLM judgment of “expertise.”

## Target capabilities

| Capability | Vision outcome |
|------------|----------------|
| Journey FSM | Automatic promotion with explicit user override (stay beginner / force power) |
| Onboarding | Wizard writes milestones into `CompletedSteps`; skip is first-class |
| Help | Uses `GetDisclosureLevel` or shared mapper — one code path |
| Tips | Rate-limited, journey-aware; power users silent |
| Metrics | Session boot, command success/fail, help, clarification hooks write counters |
| Corrections | Perception/chat correction UX calls `RecordCorrection` |
| Migration | Versioned, preserves agent + learned patterns; backup on corrupt |
| Feature flags | Keep `NERD_SKIP_ONBOARDING` for CI/dark-factory |

## Non-goals

- Replacing `internal/config` engine/provider configuration  
- Implementing the Bubble Tea UI or slash-command registry  
- Asserting journey state as Mangle policy (unless a future general-purpose fact is designed with Decl + safety review)  
- Vectryx/cloud identity sync (out of scope for this package’s local model)

## Success metrics (engineering)

- Every session increments `sessions_count` (or documented successor).  
- Journey transitions covered by integration tests against chat boot.  
- Zero second schema writer for the same JSON fields.  
- Disclosure helper used by help **or** removed after deliberate consolidation.  
- Documented mapping between `config.Guidance*` and `ux.GuidancePrefs`.

## Relation to north star

The model remains the creative center for *answers*; the kernel remains executive for *actions*. UX vision is about **how much scaffolding the human sees**, not who decides tool permission. When UX influences LLM behavior, that influence must enter through **JIT prompt atoms / selection**, not ad-hoc strings embedded in this package.
