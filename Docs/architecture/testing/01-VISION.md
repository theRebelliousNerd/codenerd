# testing — Vision

> Last verified: 2026-07-13  
> Package: `internal/testing` / `context_harness`

## Product intent

codeNERD’s differentiator is **infinite context via semantic compression + logic-directed retrieval**. Without a harness that can *fail a build* when recall collapses after 50 turns, that claim is aspirational marketing.

The Context Test Harness is the **regression oracle for long-horizon memory**:

1. Simulate realistic multi-turn coding sessions (debug, implement, refactor, campaign, TDD).
2. Compress each turn into structured facts (mock enrichment or real activation path).
3. At checkpoints, ask the retrieval system questions a human would ask mid-session.
4. Score precision / recall / F1 and emit glass-box traces for failures.

## Target architecture (steady state)

```
┌─────────────────────────────────────────────────────────────┐
│  nerd test-context / go test / CI job                       │
└───────────────────────────┬─────────────────────────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Harness                                                    │
│   • scenario registry (mock | integration | adversarial)    │
│   • dual ContextEngine (mock fast / real production pieces) │
│   • SessionSimulator turn loop                              │
│   • advanced checkpoint validators (activation, compression,│
│     feedback learning) — all enforced, not just typed       │
└───────────────┬─────────────────────────────┬───────────────┘
                │                             │
                ▼                             ▼
     FileLogger + tracers              Reporter (console|json)
                │
                ▼
     .nerd/context-tests/session-*/   (artifacts for humans + CI)
```

### Dual-mode contract (vision)

| Mode | Engine | Speed | Trust level | CI default |
|------|--------|-------|-------------|------------|
| **mock** | Simplified scoring + fact enrichment | Fast | Structural / smoke | Yes |
| **real** | `ActivationEngine` + kernel + optional live LLM | Slow | Behavioral fidelity | Nightly / gated |
| **live** | real + `GenerateAssistantResponse` | Slowest | End-to-end piggyback feedback | Manual / weekly |

### Scenario taxonomy (vision)

| Category | Purpose |
|----------|---------|
| mock | CI-safe multi-turn scripts with enrichment metrics |
| integration | Phase paging, issue tiers, budget overflow, dependency graph, verb boosts, ephemeral filters, feedback learning |
| adversarial (future) | Context bombing, rapid topic thrash, prompt-injection-like noise facts |
| replay (future) | Load real `.nerd/logs/` conversations as scenarios |

## Success criteria

A change to activation scoring, fact predicates, compression, or campaign paging is **not merge-ready** if:

- Mock suite regressions on recall floors for back-reference questions fail without an intentional threshold change.
- Real-mode integration scenarios that exercise the changed component fail on checkpoint or activation-component validators.
- Observability channels stop writing (silent glass-box regression).

## Non-goals

- Replacing `go test` for ordinary unit tests in other packages.
- Full OODA / VirtualStore / tool-permission simulation (belongs in session/campaign assault tests).
- Becoming a general LLM evaluation harness (SWE-bench *style* scenarios only as *context* stress, not as a leaderboard product).
- Shipping production runtime features under `internal/testing` — this package must stay test-only.

## Relationship to other systems

| System | Relationship |
|--------|----------------|
| `internal/context` | **System under test** (ActivationEngine, Compressor config) |
| `internal/core` | Fact store / kernel for LoadFacts |
| `internal/perception` | LLM client for live mode |
| `internal/store` | LocalStore passed into real engine |
| `internal/prompt` | Observed via tracers (target: real JIT compile traces) |
| `cmd/nerd` | Operator entry (`test-context`) |
| Campaign assault | Sibling stress path; harness is context-focused, assault is multi-system |

## North-star phrasing

> The model describes; the kernel decides; the harness **measures whether the past still exists** when the model asks for it.
