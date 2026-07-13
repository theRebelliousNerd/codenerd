# testing — Public API and Types

> Last verified: 2026-07-13  
> Package: `codenerd/internal/testing/context_harness`  
> Parent `codenerd/internal/testing` exports nothing.

## Import

```go
import "codenerd/internal/testing/context_harness"
```

## Modes & categories

| Symbol | Kind | File |
|--------|------|------|
| `EngineMode` | type string | `types.go` |
| `MockMode` | const `"mock"` | `types.go` |
| `RealMode` | const `"real"` | `types.go` |
| `ScenarioCategory` | type string | `types.go` |
| `CategoryMock` | const | `types.go` |
| `CategoryIntegration` | const | `types.go` |

## Core scenario model

| Type | Purpose | File |
|------|---------|------|
| `Scenario` | Full script: turns, checkpoints, expected metrics, mode, initial facts | `types.go` |
| `Turn` | One user/assistant message + intent + metadata + campaign phase | `types.go` |
| `TurnMetadata` | Files, symbols, errors, topics, back-reference flags | `types.go` |
| `Checkpoint` | AfterTurn query + must/avoid IDs + P/R floors + optional validators | `types.go` |
| `FeedbackValidation` | Min samples, helpful/noise predicates, boost bounds | `types.go` |
| `CompressionCheckpoint` | Trigger, min ratio, budget util, summary contains | `types.go` |
| `Metrics` | Aggregate encoding + retrieval + latency + memory | `types.go` |
| `TestResult` | Scenario, actual metrics, checkpoint results, pass/fail | `types.go` |
| `CheckpointResult` | Per-checkpoint retrieval accounting | `types.go` |
| `SimulatorConfig` | MaxTurns, TokenBudget, flags, Mode, UseLiveLLM | `types.go` |

## Engine surface

| Symbol | Purpose | File |
|--------|---------|------|
| `ContextEngine` | Interface: CompressTurn, RetrieveContext, stats, contexts, Reset, GetMode | `engine_interface.go` |
| `ActivationBreakdown` | 9-component scores + total | `engine_interface.go` |
| `CompressionStats` | Token ratio + optional summary fields | `engine_interface.go` |
| `ActivationValidation` | Min component thresholds + `ValidateActivation` | `engine_interface.go` |
| `ActivationValidationError` | Component expected/actual error | `engine_interface.go` |
| `MockContextEngine` | Fast CI engine | `mock_engine.go` |
| `NewMockContextEngine` | ctor | `mock_engine.go` |
| `RealIntegrationEngine` | Real activation (+ compressor/LLM fields) | `real_engine.go` |
| `NewRealIntegrationEngine` | ctor(kernel, LocalStore, LLMClient, CompressorConfig) | `real_engine.go` |
| `LiveLLMResponse` | Surface + feedback + tokens | `real_engine.go` |
| `(*RealIntegrationEngine).GenerateAssistantResponse` | Live mode | `real_engine.go` |

### ContextEngine methods (contract)

```go
CompressTurn(ctx, *Turn) ([]core.Fact, int, error)
RetrieveContext(ctx, query string, tokenBudget int) ([]core.Fact, error)
GetCompressionStats() (originalTokens, compressedTokens int)
GetActivationBreakdown(factID string) *ActivationBreakdown
SetCampaignContext(*internalcontext.CampaignActivationContext)
SetIssueContext(*internalcontext.IssueActivationContext)
SetBackReferenceContext(*internalcontext.BackReferenceActivationContext)
Reset() error
GetMode() EngineMode
```

## Orchestration

| Symbol | Purpose | File |
|--------|---------|------|
| `Harness` | Scenario map + run + report | `harness.go` |
| `NewHarness` | Basic ctor (mock engine default) | `harness.go` |
| `NewHarnessWithObservability` | Full tracer injection | `harness.go` |
| `(*Harness).SetContextEngine` | Inject real engine | `harness.go` |
| `(*Harness).RunScenario` | One scenario by ID | `harness.go` |
| `(*Harness).RunAll` | All + summary | `harness.go` |
| `(*Harness).ListScenarios` | IDs | `harness.go` |
| `SessionSimulator` | Turn executor | `simulator.go` |
| `NewSessionSimulator` | ctor | `simulator.go` |
| `(*SessionSimulator).SetObservability` | Wire tracers | `simulator.go` |
| `(*SessionSimulator).SetContextEngine` | Wire engine | `simulator.go` |
| `(*SessionSimulator).RunScenario` | Execute | `simulator.go` |

## Scenario registry

| Func | Returns | File |
|------|---------|------|
| `GetScenario(name)` | `*Scenario` or nil | `scenarios.go` |
| `AllScenarios()` | mock + integration | `scenarios.go` |
| `MockScenarios()` | 8 mock | `scenarios.go` |
| `ScenariosByCategory(cat)` | filtered | `scenarios.go` |
| `DebuggingMarathonScenario` … `ManglePolicyDebugScenario` | builders | `scenarios.go` |
| `CampaignPhaseTransitionScenario` … `ContextFeedbackLearningScenario` | builders | `scenarios_integration.go` |
| `IntegrationScenarios()` | 7 integration | `scenarios_integration.go` |

## Metrics & reporting

| Symbol | File |
|--------|------|
| `MetricsCollector` / `NewMetricsCollector` | `metrics.go` |
| `RecordCompression` / `RecordRetrieval` / `RecordTokenBudgetViolation` / `RecordMemory` / `Finalize` | `metrics.go` |
| `Reporter` / `NewReporter` | `reporter.go` |
| `(*Reporter).Report` / `ReportSummary` | `reporter.go` |

## Seeding & kernels

| Symbol | File |
|--------|------|
| `FactSeeder` / `NewFactSeeder` | `fact_seeder.go` |
| `SeedScenario`, `SeedCampaignContext`, `SeedIssueContext`, `SeedSymbolGraph`, `SeedDependencyLinks`, `SeedProjectPatterns`, `SeedFileTopology`, `Clear` | `fact_seeder.go` |
| `TestKernelFactory` / `NewTestKernelFactory` | `test_kernel_factory.go` |
| `CreateKernel`, `CreateIsolatedKernel` | `test_kernel_factory.go` |

`parseMangleFact` is package-private (used by factory/seeder).

## Observability types

| Symbol | File |
|--------|------|
| `FileLogger` / `NewFileLogger` / writers / `Close` / `GetSessionDir` | `file_logger.go` |
| `PromptInspector` / `PromptSnapshot` / `PromptAtom` / `ActivatedFact` / `ResponseSnapshot` / `ControlPacket` / `ContextFeedback` / `IntentClassification` | `inspector.go` |
| `JITTracer` / `CompilationSnapshot` / `CompiledAtom` | `jit_tracer.go` |
| `ActivationTracer` / `ActivationSnapshot` / `FactActivation` / campaign/issue/session contexts / `DependencyEdge` | `activation_tracer.go` |
| `CompressionVisualizer` / `CompressionEvent` / `CompressionSummary` | `compression_viz.go` |
| `PiggybackTracer` / `PiggybackEvent` | `piggyback_tracer.go` |
| `FeedbackTracer` / `FeedbackSnapshot` / `PredicateFeedbackState` / `PredicateScoreImpact` | `feedback_tracer.go` |

## Stability notes for callers

1. **Stable for CLI / external tests:** `Harness`, `SimulatorConfig`, scenario IDs, `ContextEngine` constructors, `FileLogger`, `Reporter`.  
2. **Volatile:** Mock atom generation, fuzzy semantic maps, enrichment token estimates (20 tokens/fact).  
3. **Do not import from production services.** CLI is the supported operator surface.  
4. Compile-time interface checks: `var _ ContextEngine = (*MockContextEngine)(nil)` and same for real.
