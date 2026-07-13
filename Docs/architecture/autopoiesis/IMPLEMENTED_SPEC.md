# autopoiesis — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/autopoiesis/` (complete internal coverage)
> **Implementation: `internal/autopoiesis/` — 37 non-test .go, 30 tests, 0 .mg**


## 1. Purpose

Self-improvement: Ouroboros tool generation, SafetyChecker, Thunderdome

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/autopoiesis/` | Primary implementation |
| `Docs/architecture/autopoiesis/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 85%** as living package (37 src / 30 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/autopoiesis/ouroboros.go` | 1133 | source |
| `internal/autopoiesis/traces.go` | 994 | source |
| `internal/autopoiesis/prompt_evolution/evolver.go` | 770 | source |
| `internal/autopoiesis/quality.go` | 770 | source |
| `internal/autopoiesis/checker.go` | 722 | source |
| `internal/autopoiesis/tool_generation.go` | 656 | source |
| `internal/autopoiesis/thunderdome.go` | 626 | source |
| `internal/autopoiesis/feedback.go` | 599 | source |
| `internal/autopoiesis/tool_templates.go` | 592 | source |
| `internal/autopoiesis/prompt_evolution/strategy_store.go` | 573 | source |
| `internal/autopoiesis/persistence.go` | 554 | source |
| `internal/autopoiesis/prompt_evolution/atom_generator.go` | 504 | source |
| `internal/autopoiesis/prompt_evolution/feedback_collector.go` | 489 | source |
| `internal/autopoiesis/autopoiesis_feedback.go` | 459 | source |
| `internal/autopoiesis/autopoiesis_kernel.go` | 454 | source |
| `internal/autopoiesis/prompt_evolution/judge.go` | 404 | source |
| `internal/autopoiesis/profiles.go` | 372 | source |
| `internal/autopoiesis/autopoiesis_orchestrator.go` | 370 | source |
| `internal/autopoiesis/prompt_evolution/classifier.go` | 363 | source |
| `internal/autopoiesis/tool_compiler.go` | 359 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `Orchestrator` | `internal/autopoiesis/autopoiesis_orchestrator.go:23` |
| `KernelFact` | `internal/autopoiesis/autopoiesis_types.go:21` |
| `KernelInterface` | `internal/autopoiesis/autopoiesis_types.go:26` |
| `PromptAssembler` | `internal/autopoiesis/autopoiesis_types.go:34` |
| `ToolSynthesizer` | `internal/autopoiesis/autopoiesis_types.go:45` |
| `ToolRegisteredCallback` | `internal/autopoiesis/autopoiesis_types.go:83` |
| `RuntimeTool` | `internal/autopoiesis/autopoiesis_types.go:86` |
| `LoopResult` | `internal/autopoiesis/autopoiesis_types.go:97` |
| `CompileResult` | `internal/autopoiesis/autopoiesis_types.go:109` |
| `LoopStage` | `internal/autopoiesis/autopoiesis_types.go:119` |
| `OuroborosStats` | `internal/autopoiesis/autopoiesis_types.go:162` |
| `Config` | `internal/autopoiesis/autopoiesis_types.go:181` |
| `AnalysisResult` | `internal/autopoiesis/autopoiesis_types.go:197` |
| `AutopoiesisAction` | `internal/autopoiesis/autopoiesis_types.go:220` |
| `ActionType` | `internal/autopoiesis/autopoiesis_types.go:228` |
| `CampaignPayload` | `internal/autopoiesis/autopoiesis_types.go:255` |
| `QuickResult` | `internal/autopoiesis/autopoiesis_types.go:262` |
| `AgentMemory` | `internal/autopoiesis/autopoiesis_types.go:275` |
| `Learning` | `internal/autopoiesis/autopoiesis_types.go:285` |
| `LearnedPattern` | `internal/autopoiesis/autopoiesis_types.go:297` |
| `SafetyChecker` | `internal/autopoiesis/checker.go:32` |
| `SafetyReport` | `internal/autopoiesis/checker.go:39` |
| `SafetyViolation` | `internal/autopoiesis/checker.go:48` |
| `ViolationType` | `internal/autopoiesis/checker.go:56` |
| `ViolationSeverity` | `internal/autopoiesis/checker.go:102` |
| `ComplexityLevel` | `internal/autopoiesis/complexity.go:20` |
| `ComplexityResult` | `internal/autopoiesis/complexity.go:30` |
| `ComplexityAnalyzer` | `internal/autopoiesis/complexity.go:41` |
| `LLMClient` | `internal/autopoiesis/complexity.go:46` |
| `ExecutionFeedback` | `internal/autopoiesis/feedback.go:31` |
| `UserFeedback` | `internal/autopoiesis/feedback.go:64` |
| `ToolRefiner` | `internal/autopoiesis/feedback.go:78` |
| `RefinementRequest` | `internal/autopoiesis/feedback.go:86` |
| `RefinementResult` | `internal/autopoiesis/feedback.go:95` |
| `LearningStore` | `internal/autopoiesis/feedback.go:353` |
| `ToolLearning` | `internal/autopoiesis/feedback.go:360` |
| `OuroborosLoop` | `internal/autopoiesis/ouroboros.go:88` |
| `OuroborosConfig` | `internal/autopoiesis/ouroboros.go:110` |
| `RetryConfig` | `internal/autopoiesis/ouroboros.go:151` |
| `ExecuteConfig` | `internal/autopoiesis/ouroboros.go:165` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `ListAgents` | `internal/autopoiesis/autopoiesis_agents.go:92` |
| `GetAgent` | `internal/autopoiesis/autopoiesis_agents.go:125` |
| `DeleteAgent` | `internal/autopoiesis/autopoiesis_agents.go:141` |
| `UpdateAgentMemory` | `internal/autopoiesis/autopoiesis_agents.go:147` |
| `Analyze` | `internal/autopoiesis/autopoiesis_analysis.go:14` |
| `ExecuteAction` | `internal/autopoiesis/autopoiesis_analysis.go:103` |
| `QuickAnalyze` | `internal/autopoiesis/autopoiesis_analysis.go:154` |
| `ShouldTriggerCampaign` | `internal/autopoiesis/autopoiesis_analysis.go:200` |
| `ShouldCreatePersistentAgent` | `internal/autopoiesis/autopoiesis_analysis.go:220` |
| `ProcessKernelDelegations` | `internal/autopoiesis/autopoiesis_delegation.go:21` |
| `StartKernelListener` | `internal/autopoiesis/autopoiesis_delegation.go:158` |
| `RecordExecution` | `internal/autopoiesis/autopoiesis_feedback.go:18` |
| `EvaluateToolQuality` | `internal/autopoiesis/autopoiesis_feedback.go:73` |
| `EvaluateToolQualityWithLLM` | `internal/autopoiesis/autopoiesis_feedback.go:78` |
| `GetToolPatterns` | `internal/autopoiesis/autopoiesis_feedback.go:83` |
| `GetAllPatterns` | `internal/autopoiesis/autopoiesis_feedback.go:88` |
| `ShouldRefineTool` | `internal/autopoiesis/autopoiesis_feedback.go:93` |
| `RefineTool` | `internal/autopoiesis/autopoiesis_feedback.go:123` |
| `GetToolLearning` | `internal/autopoiesis/autopoiesis_feedback.go:176` |
| `GetAllLearnings` | `internal/autopoiesis/autopoiesis_feedback.go:181` |
| `AggregateLearningsForPrompt` | `internal/autopoiesis/autopoiesis_feedback.go:188` |
| `RefreshLearningsContext` | `internal/autopoiesis/autopoiesis_feedback.go:296` |
| `GenerateLearningFacts` | `internal/autopoiesis/autopoiesis_feedback.go:305` |
| `ExecuteAndEvaluate` | `internal/autopoiesis/autopoiesis_feedback.go:310` |
| `StartToolTrace` | `internal/autopoiesis/autopoiesis_feedback.go:341` |
| `RecordTracePrompt` | `internal/autopoiesis/autopoiesis_feedback.go:346` |
| `RecordTraceResponse` | `internal/autopoiesis/autopoiesis_feedback.go:351` |
| `FinalizeTrace` | `internal/autopoiesis/autopoiesis_feedback.go:356` |
| `UpdateTraceWithFeedback` | `internal/autopoiesis/autopoiesis_feedback.go:361` |
| `GetToolTraces` | `internal/autopoiesis/autopoiesis_feedback.go:366` |

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
