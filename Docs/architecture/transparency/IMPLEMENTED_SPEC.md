# transparency — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/transparency/` (complete internal coverage)
> **Implementation: `internal/transparency/` — 8 non-test .go, 9 tests, 0 .mg**


## 1. Purpose

Transparency event bus / glass-box observability

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/transparency/` | Primary implementation |
| `Docs/architecture/transparency/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (8 src / 9 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/transparency/explainer.go` | 327 | source |
| `internal/transparency/safety_reporter.go` | 306 | source |
| `internal/transparency/shard_observer.go` | 305 | source |
| `internal/transparency/event_bus.go` | 286 | source |
| `internal/transparency/error_classifier.go` | 266 | source |
| `internal/transparency/transparency.go` | 213 | source |
| `internal/transparency/glass_box_events.go` | 140 | source |
| `internal/transparency/doc.go` | 20 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `ErrorCategory` | `internal/transparency/error_classifier.go:9` |
| `ClassifiedError` | `internal/transparency/error_classifier.go:79` |
| `GlassBoxEventBus` | `internal/transparency/event_bus.go:15` |
| `GlassBoxBusStats` | `internal/transparency/event_bus.go:279` |
| `Explainer` | `internal/transparency/explainer.go:12` |
| `OperationSummary` | `internal/transparency/explainer.go:285` |
| `GlassBoxCategory` | `internal/transparency/glass_box_events.go:12` |
| `GlassBoxEvent` | `internal/transparency/glass_box_events.go:34` |
| `ToolEvent` | `internal/transparency/glass_box_events.go:99` |
| `ToolEventBus` | `internal/transparency/glass_box_events.go:109` |
| `SafetyViolationType` | `internal/transparency/safety_reporter.go:10` |
| `SafetyViolation` | `internal/transparency/safety_reporter.go:40` |
| `SafetyReporter` | `internal/transparency/safety_reporter.go:53` |
| `ShardPhase` | `internal/transparency/shard_observer.go:11` |
| `ShardExecution` | `internal/transparency/shard_observer.go:43` |
| `PhaseUpdate` | `internal/transparency/shard_observer.go:74` |
| `PhaseObserver` | `internal/transparency/shard_observer.go:83` |
| `ShardObserver` | `internal/transparency/shard_observer.go:88` |
| `TransparencyManager` | `internal/transparency/transparency.go:14` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `Prefix` | `internal/transparency/error_classifier.go:41` |
| `String` | `internal/transparency/error_classifier.go:60` |
| `Error` | `internal/transparency/error_classifier.go:87` |
| `Unwrap` | `internal/transparency/error_classifier.go:92` |
| `Format` | `internal/transparency/error_classifier.go:97` |
| `ClassifyError` | `internal/transparency/error_classifier.go:114` |
| `GetRecoveryGuide` | `internal/transparency/error_classifier.go:218` |
| `NewGlassBoxEventBus` | `internal/transparency/event_bus.go:40` |
| `Enable` | `internal/transparency/event_bus.go:50` |
| `Disable` | `internal/transparency/event_bus.go:55` |
| `IsEnabled` | `internal/transparency/event_bus.go:62` |
| `SetVerbose` | `internal/transparency/event_bus.go:67` |
| `IsVerbose` | `internal/transparency/event_bus.go:74` |
| `SetCategories` | `internal/transparency/event_bus.go:81` |
| `Subscribe` | `internal/transparency/event_bus.go:93` |
| `Unsubscribe` | `internal/transparency/event_bus.go:102` |
| `Emit` | `internal/transparency/event_bus.go:122` |
| `EmitImmediate` | `internal/transparency/event_bus.go:165` |
| `Flush` | `internal/transparency/event_bus.go:196` |
| `ClearTurn` | `internal/transparency/event_bus.go:235` |
| `Close` | `internal/transparency/event_bus.go:249` |
| `Stats` | `internal/transparency/event_bus.go:262` |
| `NewExplainer` | `internal/transparency/explainer.go:18` |
| `SetMaxDepth` | `internal/transparency/explainer.go:26` |
| `SetShowDetails` | `internal/transparency/explainer.go:31` |
| `ExplainTrace` | `internal/transparency/explainer.go:36` |
| `ExplainFact` | `internal/transparency/explainer.go:157` |
| `ExplainDecision` | `internal/transparency/explainer.go:188` |
| `QuickExplain` | `internal/transparency/explainer.go:245` |
| `FormatOperationSummary` | `internal/transparency/explainer.go:296` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
