# testing — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/testing/` (complete internal coverage)
> **Implementation: `internal/testing/` — 21 non-test .go, 8 tests, 0 .mg**


## 1. Purpose

Internal test helpers and harness utilities

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/testing/` | Primary implementation |
| `Docs/architecture/testing/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **70%** |
| Exported types (sampled) | Implemented | **70%** |
| Tests | Implemented | **70%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 70%** as living package (21 src / 8 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/testing/context_harness/simulator.go` | 1096 | source |
| `internal/testing/context_harness/scenarios.go` | 818 | source |
| `internal/testing/context_harness/scenarios_integration.go` | 493 | source |
| `internal/testing/context_harness/real_engine.go` | 484 | source |
| `internal/testing/context_harness/inspector.go` | 406 | source |
| `internal/testing/context_harness/mock_engine.go` | 395 | source |
| `internal/testing/context_harness/activation_tracer.go` | 320 | source |
| `internal/testing/context_harness/file_logger.go` | 293 | source |
| `internal/testing/context_harness/jit_tracer.go` | 281 | source |
| `internal/testing/context_harness/compression_viz.go` | 222 | source |
| `internal/testing/context_harness/feedback_tracer.go` | 212 | source |
| `internal/testing/context_harness/test_kernel_factory.go` | 178 | source |
| `internal/testing/context_harness/engine_interface.go` | 176 | source |
| `internal/testing/context_harness/reporter.go` | 171 | source |
| `internal/testing/context_harness/types.go` | 163 | source |
| `internal/testing/context_harness/piggyback_tracer.go` | 162 | source |
| `internal/testing/context_harness/harness.go` | 160 | source |
| `internal/testing/context_harness/fact_seeder.go` | 158 | source |
| `internal/testing/context_harness/metrics.go` | 135 | source |
| `internal/testing/harness_subsystem.go` | 3 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `ActivationTracer` | `internal/testing/context_harness/activation_tracer.go:14` |
| `ActivationSnapshot` | `internal/testing/context_harness/activation_tracer.go:28` |
| `FactActivation` | `internal/testing/context_harness/activation_tracer.go:53` |
| `CampaignContext` | `internal/testing/context_harness/activation_tracer.go:74` |
| `IssueContext` | `internal/testing/context_harness/activation_tracer.go:82` |
| `SessionContext` | `internal/testing/context_harness/activation_tracer.go:90` |
| `DependencyEdge` | `internal/testing/context_harness/activation_tracer.go:98` |
| `CompressionVisualizer` | `internal/testing/context_harness/compression_viz.go:13` |
| `CompressionEvent` | `internal/testing/context_harness/compression_viz.go:27` |
| `CompressionSummary` | `internal/testing/context_harness/compression_viz.go:159` |
| `ContextEngine` | `internal/testing/context_harness/engine_interface.go:13` |
| `ActivationBreakdown` | `internal/testing/context_harness/engine_interface.go:50` |
| `CompressionStats` | `internal/testing/context_harness/engine_interface.go:69` |
| `ActivationValidation` | `internal/testing/context_harness/engine_interface.go:82` |
| `ActivationValidationError` | `internal/testing/context_harness/engine_interface.go:163` |
| `FactSeeder` | `internal/testing/context_harness/fact_seeder.go:11` |
| `FeedbackTracer` | `internal/testing/context_harness/feedback_tracer.go:13` |
| `FeedbackSnapshot` | `internal/testing/context_harness/feedback_tracer.go:27` |
| `PredicateFeedbackState` | `internal/testing/context_harness/feedback_tracer.go:47` |
| `PredicateScoreImpact` | `internal/testing/context_harness/feedback_tracer.go:198` |
| `FileLogger` | `internal/testing/context_harness/file_logger.go:12` |
| `Harness` | `internal/testing/context_harness/harness.go:12` |
| `PromptInspector` | `internal/testing/context_harness/inspector.go:14` |
| `PromptSnapshot` | `internal/testing/context_harness/inspector.go:34` |
| `PromptAtom` | `internal/testing/context_harness/inspector.go:58` |
| `ActivatedFact` | `internal/testing/context_harness/inspector.go:67` |
| `ResponseSnapshot` | `internal/testing/context_harness/inspector.go:222` |
| `ControlPacket` | `internal/testing/context_harness/inspector.go:240` |
| `ContextFeedback` | `internal/testing/context_harness/inspector.go:251` |
| `IntentClassification` | `internal/testing/context_harness/inspector.go:259` |
| `JITTracer` | `internal/testing/context_harness/jit_tracer.go:12` |
| `CompilationSnapshot` | `internal/testing/context_harness/jit_tracer.go:26` |
| `CompiledAtom` | `internal/testing/context_harness/jit_tracer.go:58` |
| `MetricsCollector` | `internal/testing/context_harness/metrics.go:9` |
| `MockContextEngine` | `internal/testing/context_harness/mock_engine.go:20` |
| `PiggybackTracer` | `internal/testing/context_harness/piggyback_tracer.go:11` |
| `PiggybackEvent` | `internal/testing/context_harness/piggyback_tracer.go:25` |
| `RealIntegrationEngine` | `internal/testing/context_harness/real_engine.go:18` |
| `LiveLLMResponse` | `internal/testing/context_harness/real_engine.go:324` |
| `Reporter` | `internal/testing/context_harness/reporter.go:11` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `NewActivationTracer` | `internal/testing/context_harness/activation_tracer.go:20` |
| `TraceActivation` | `internal/testing/context_harness/activation_tracer.go:107` |
| `NewCompressionVisualizer` | `internal/testing/context_harness/compression_viz.go:19` |
| `VisualizeCompression` | `internal/testing/context_harness/compression_viz.go:54` |
| `VisualizeSummary` | `internal/testing/context_harness/compression_viz.go:177` |
| `ValidateActivation` | `internal/testing/context_harness/engine_interface.go:97` |
| `Error` | `internal/testing/context_harness/engine_interface.go:169` |
| `NewFactSeeder` | `internal/testing/context_harness/fact_seeder.go:16` |
| `SeedScenario` | `internal/testing/context_harness/fact_seeder.go:21` |
| `SeedCampaignContext` | `internal/testing/context_harness/fact_seeder.go:39` |
| `SeedIssueContext` | `internal/testing/context_harness/fact_seeder.go:62` |
| `SeedSymbolGraph` | `internal/testing/context_harness/fact_seeder.go:94` |
| `SeedDependencyLinks` | `internal/testing/context_harness/fact_seeder.go:110` |
| `SeedProjectPatterns` | `internal/testing/context_harness/fact_seeder.go:126` |
| `SeedFileTopology` | `internal/testing/context_harness/fact_seeder.go:140` |
| `Clear` | `internal/testing/context_harness/fact_seeder.go:154` |
| `NewFeedbackTracer` | `internal/testing/context_harness/feedback_tracer.go:19` |
| `TraceFeedback` | `internal/testing/context_harness/feedback_tracer.go:58` |
| `TraceScoreImpact` | `internal/testing/context_harness/feedback_tracer.go:164` |
| `NewFileLogger` | `internal/testing/context_harness/file_logger.go:36` |
| `GetPromptWriter` | `internal/testing/context_harness/file_logger.go:135` |
| `GetJITWriter` | `internal/testing/context_harness/file_logger.go:140` |
| `GetActivationWriter` | `internal/testing/context_harness/file_logger.go:145` |
| `GetCompressionWriter` | `internal/testing/context_harness/file_logger.go:150` |
| `GetPiggybackWriter` | `internal/testing/context_harness/file_logger.go:155` |
| `GetSummaryWriter` | `internal/testing/context_harness/file_logger.go:160` |
| `GetFeedbackWriter` | `internal/testing/context_harness/file_logger.go:165` |
| `GetSessionDir` | `internal/testing/context_harness/file_logger.go:170` |
| `Close` | `internal/testing/context_harness/file_logger.go:175` |
| `NewHarness` | `internal/testing/context_harness/harness.go:31` |

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
