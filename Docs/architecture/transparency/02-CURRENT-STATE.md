# transparency — Current State

> Last verified: 2026-07-13  
> Source of truth: `internal/transparency/`

## 1. Package identity

| Field | Value |
|-------|-------|
| Import path | `codenerd/internal/transparency` |
| Mangle sources | **0** (pure Go) |
| Package comment | `doc.go` |
| Config type | `config.TransparencyConfig` in `internal/config/ux.go` |
| Tier (architecture index) | T2 |

## 2. File inventory

### 2.1 Non-test sources (8)

| Path | ≈Lines | Role |
|------|-------:|------|
| `internal/transparency/explainer.go` | 327 | Derivation / decision / operation summary formatting |
| `internal/transparency/safety_reporter.go` | 306 | Safety violation history + explanations |
| `internal/transparency/shard_observer.go` | 305 | Shard phase state machine + observers |
| `internal/transparency/event_bus.go` | 286 | `GlassBoxEventBus` batching/dispatch |
| `internal/transparency/error_classifier.go` | 266 | Error categories + remediation |
| `internal/transparency/transparency.go` | 213 | `TransparencyManager` coordinator |
| `internal/transparency/glass_box_events.go` | 140 | Event types, categories, `ToolEventBus` |
| `internal/transparency/doc.go` | 20 | Package docs / principles |

**Total non-test ≈ 1,863 lines.**

### 2.2 Tests (9)

| Path | Focus |
|------|-------|
| `error_classifier_test.go` | Safety/timeout classify + recovery guide |
| `event_bus_test.go` | Immediate emit, filter, flush, unsubscribe, clear turn |
| `explainer_test.go` | Trace/fact/decision, QuickExplain, operation summary |
| `glass_box_events_test.go` | Event string, categories, ToolEventBus |
| `glass_box_helpers_test.go` | Category helpers, verbose bus, explainer setters |
| `safety_reporter_test.go` | Violation types, ExplainSafetyAction |
| `shard_observer_test.go` | Lifecycle, phase strings, durations |
| `transparency_test.go` | Manager enable/toggle/status/format error |
| `transparency_comprehensive_test.go` | Broad table-driven coverage of all major types |

### 2.3 Not present

- No `agents.md` under package  
- No YAML/JSON schemas inside package  
- No `.mg` rules  
- No CLI binary of its own  

## 3. Component map (as implemented)

```
┌─────────────────────────────────────────────────────────────┐
│                 TransparencyManager (opt-in)                │
│  config.TransparencyConfig · Enable/Disable/Toggle/Status   │
│     │                              │                        │
│     ▼                              ▼                        │
│ ShardObserver                  SafetyReporter               │
│ phases · history · observers   violations · FormatViolation │
│                                                             │
│ FormatError → ClassifyError (always classifies; verbose     │
│               format gated by Enabled && VerboseErrors)     │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│              Glass Box path (separate instances)            │
│  GlassBoxEvent / GlassBoxCategory                           │
│  GlassBoxEventBus — batch or immediate, filter, stats       │
│  ToolEvent / ToolEventBus — always-on, simple channel       │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Explainer (stateless-ish helpers; not held by Manager)      │
│  ExplainTrace · ExplainFact · ExplainDecision · QuickExplain│
│  FormatOperationSummary                                     │
│  depends on: codenerd/internal/mangle                       │
└─────────────────────────────────────────────────────────────┘
```

**Important structural fact:** `TransparencyManager` does **not** own the Glass Box or Tool buses. Chat boot creates those independently and injects them into VirtualStore / ShardManager / system shards. Manager and buses are **sibling** concerns in one package.

## 4. Hotspots

| Hotspot | Why it matters |
|---------|----------------|
| `event_bus.go` Emit path | Every concurrent producer; batching vs verbose fork |
| `virtual_store_routing.go` emitToolAndRoutingEvents | Primary production emitter for tools/routing |
| `manager.go` emitShardEvent | Shard lifecycle lines in TUI |
| `glass_box.go` (chat) | Subscribe, drain, render contract |
| `ClassifyError` switch | Heuristic quality of user recovery |
| `SetTransparencyManager` as `any` | Wiring smell / dormant phase tracking |

## 5. Config surface (authoritative in config package)

From `internal/config/ux.go` `TransparencyConfig`:

| Field | JSON | Default (`DefaultTransparencyConfig`) |
|-------|------|----------------------------------------|
| `Enabled` | `enabled` | `false` |
| `ShardPhases` | `shard_phases` | `true` |
| `StreamReasoning` | `stream_reasoning` | `false` |
| `SafetyExplanations` | `safety_explanations` | `true` |
| `JITExplain` | `jit_explain` | `false` |
| `OperationSummaries` | `operation_summaries` | `true` |
| `VerboseErrors` | `verbose_errors` | `true` |
| `GlassBoxEnabled` | `glass_box_enabled` | (zero value unless set in user config) |
| `GlassBoxDisabled` | `glass_box_disabled` | opt-out |
| `GlassBoxCategories` | `glass_box_categories` | empty = all |
| `GlassBoxVerbose` | `glass_box_verbose` | false |

Note: `NewTransparencyManager(nil)` builds an in-package default with `Enabled: false` and only sets ShardPhases/SafetyExplanations/VerboseErrors true—**does not** copy full `DefaultTransparencyConfig` Glass Box fields (those are handled by chat init, not Manager).

User-config template in `user_config.go` may enable more flags aggressively for new workspaces—treat runtime `GetTransparencyConfig()` as truth per workspace.

## 6. Exported surface summary

See [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) for full tables. Headline types:

- `TransparencyManager`
- `GlassBoxEventBus`, `GlassBoxEvent`, `GlassBoxCategory`, `GlassBoxBusStats`
- `ToolEventBus`, `ToolEvent`
- `ShardObserver`, `ShardExecution`, `ShardPhase`, `PhaseUpdate`, `PhaseObserver`
- `SafetyReporter`, `SafetyViolation`, `SafetyViolationType`
- `Explainer`, `OperationSummary`
- `ClassifiedError`, `ErrorCategory`
- Free funcs: `ClassifyError`, `GetRecoveryGuide`, `ExplainSafetyAction`, `QuickExplain`, `FormatOperationSummary`, `AllCategories`, `ValidCategory`, constructors `New*`

## 7. Downstream consumers (summary)

| Consumer | Uses |
|----------|------|
| `cmd/nerd/chat` | Manager, both buses, Explainer, categories |
| `internal/core` VirtualStore | GlassBox + ToolEvent emit |
| `internal/core/shards` ShardManager | GlassBox emit; stores Manager as `any` |
| `internal/shards/system` | ToolEventBus (router), GlassBox setters on base |

## 8. Honest completeness

| Area | State |
|------|-------|
| Bus + event types | **Production** |
| Tool always-on channel | **Production** |
| Explainer formatting | **Production** (needs traces supplied by callers) |
| Error classifier | **Production** (heuristic) |
| Safety reporter types/format | **Production**; feed partial |
| ShardObserver | **Implemented + tested**; lifecycle feed thin vs Glass Box |
| Manager status / slash toggle | **Production** |
| Config flags fully enforced | **Partial** |

Overall: **realized living package (~85–90% of declared package scope; lower if counting every config flag as required)**.
