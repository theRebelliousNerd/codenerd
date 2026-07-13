# 06 — Public API and Types: Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Focus: exported symbols that integrators actually use  
> Package: `autopoiesis` and `prompt_evolution`

This is not a full `go doc` dump. Prefer reading the cited files for full method lists.

---

## 1. Construction & config

| Symbol | File | Notes |
|--------|------|-------|
| `Config` | `autopoiesis_types.go` | Tools/agents dirs, confidence, caps, OS/arch, MaxLearningFacts |
| `DefaultConfig(workspaceRoot string) Config` | `autopoiesis_orchestrator.go` | Merges user tool-gen config |
| `NewOrchestrator(client LLMClient, config Config) *Orchestrator` | same | Full subsystem graph |
| `OuroborosConfig` / `DefaultOuroborosConfig` | `ouroboros.go` | Loop-level safety/timeouts/Thunderdome |
| `NewOuroborosLoop(client, config) *OuroborosLoop` | `ouroboros.go` | Usually via Orchestrator |

## 2. Orchestrator surface (integrator-facing)

### Kernel

| Method | File |
|--------|------|
| `SetKernel` / `GetKernel` | `autopoiesis_kernel.go` |
| `SyncLearningsToKernel` | same |
| `ShouldGenerateTool` / `ShouldRefineToolByKernel` | same |
| `QueryCodeElementCount` / `QueryFilesInScope` / `QueryNextAction` | same |
| `RecordCodeEditOutcome` | same |

### Analysis / actions

| Method | File |
|--------|------|
| `Analyze` / `QuickAnalyze` | `autopoiesis_analysis.go` |
| `ExecuteAction` | same |
| `ShouldTriggerCampaign` / `ShouldCreatePersistentAgent` | same |

### Tools / Ouroboros

| Method | File |
|--------|------|
| `DetectToolNeed` / `GenerateTool` / `WriteAndRegisterTool` | `autopoiesis_tools.go` |
| `ExecuteOuroborosLoop` / `ExecuteGeneratedTool` | same |
| `ListTools` / `ListGeneratedTools` / `GetToolInfo` / `HasGeneratedTool` | same |
| `CheckToolSafety` / `GetOuroborosStats` | same |
| `GetOuroborosLoop` | `autopoiesis_orchestrator.go` |
| `CompileTool` | same |
| `SetPromptAssembler` | same |

### Delegation

| Method | File |
|--------|------|
| `ProcessKernelDelegations` | `autopoiesis_delegation.go` |
| `StartKernelListener` | same |

### Learning / quality / traces / profiles

| Method | File |
|--------|------|
| `RecordExecution` / `EvaluateToolQuality*` / `Get*Patterns` / `ShouldRefineTool` / `RefineTool` | `autopoiesis_feedback.go` |
| `GetToolLearning` / `GetAllLearnings` / `AggregateLearningsForPrompt` / `RefreshLearningsContext` | same |
| `ExecuteAndEvaluate` / `ExecuteAndEvaluateWithProfile` | feedback + profiles |
| Trace helpers (`StartToolTrace` … `GenerateToolWithTracing`) | `autopoiesis_feedback.go` |
| Profile helpers | `autopoiesis_profiles.go` |

### Agents

| Method | File |
|--------|------|
| `ListAgents` / `GetAgent` / `DeleteAgent` / `UpdateAgentMemory` | `autopoiesis_agents.go` |

## 3. Core types

### Loop & runtime

```text
RuntimeTool{Name, Description, BinaryPath, Hash, Schema, RegisteredAt, ExecuteCount}
LoopResult{Success, ToolName, Stage, Error, SafetyReport, CompileResult, ToolHandle, Duration}
CompileResult{Success, OutputPath, Hash, CompileTime, Errors, Warnings}
LoopStage  // detection … complete + simulation + panic
OuroborosStats{ToolsGenerated, ToolsCompiled, ToolsRejected, SafetyViolations, … Thunderdome*}
```

### Actions & analysis

```text
AnalysisResult{Complexity, NeedsCampaign, ToolNeeds, Persistence, Actions, …}
AutopoiesisAction{Type, Priority, Description, Payload}
ActionType // StartCampaign | GenerateTool | CreateAgent | DelegateToShard
QuickResult{NeedsCampaign, NeedsPersistent, NeedsTool, ComplexityLevel, TopAction}
```

### Tool need & generated

```text
ToolNeed{Name, Purpose, InputType, OutputType, Triggers, Priority, Confidence, Reasoning}
GeneratedTool{Name, Package, Description, Code, TestCode, Schema, …}
ToolSchema / ParamSchema
```

### Safety

```text
SafetyReport{Safe, Violations, ImportsChecked, CallsChecked, Score}
SafetyViolation{Type, Location, Description, Severity}
ViolationType / ViolationSeverity
SafetyChecker  // NewSafetyChecker, Check, ExtractASTFacts
```

### Quality & learning

```text
QualityAssessment / QualityIssue / IssueType / ImprovementSuggestion
ExecutionFeedback / UserFeedback
DetectedPattern
ToolLearning (via LearningStore)
ToolQualityProfile / ToolType / PerformanceExpectations …
ReasoningTrace
```

### Complexity

```text
ComplexityLevel // Simple | Moderate | Complex | Epic
ComplexityResult / ComplexityAnalyzer
LLMClient // Complete, CompleteWithSystem
```

### Interfaces

```text
KernelInterface / KernelFact   // aliases of types.*
PromptAssembler
ToolSynthesizer
ToolRegisteredCallback
```

## 4. Supporting constructors (tests & advanced)

| Symbol | File |
|--------|------|
| `NewComplexityAnalyzer` | `complexity.go` |
| `NewPersistenceAnalyzer` | `persistence.go` |
| `NewToolGenerator` | toolgen / generation files |
| `NewSafetyChecker` | `checker.go` |
| `NewRuntimeRegistry` | `runtime_registry.go` |
| `NewToolCompiler` | `tool_compiler.go` |
| `NewThunderdome` / `NewThunderdomeWithConfig` | `thunderdome.go` |
| `NewPanicMaker` | `panic_maker.go` |
| `NewQualityEvaluator` | `quality.go` |
| `NewPatternDetector` | `patterns.go` |
| `NewToolRefiner` / `NewLearningStore` | `feedback.go` |
| `NewYaegiExecutor` | `yaegi_executor.go` |
| `NewTraceCollector` / `NewLogInjector` | `traces.go` |

## 5. Fact helper generators

| Func | File | Output style |
|------|------|--------------|
| `GenerateMissingToolFacts` | `ouroboros.go` | string facts |
| `GenerateToolCapabilityFacts` | same | |
| `GenerateToolRegistrationFacts` | same | |

## 6. `prompt_evolution` public surface (summary)

| Symbol | Role |
|--------|------|
| `PromptEvolver` / `EvolverConfig` / `DefaultEvolverConfig` | orchestrate SPL |
| `TaskJudge` (judge.go) | LLM-as-judge |
| `FeedbackCollector` | record outcomes |
| `StrategyStore` | problem-type strategies |
| `AtomGenerator` | emit prompt atoms |
| `Classifier` | problem/error classification |
| `ErrorCategory`, `ProblemType`, `ExecutionRecord`, … | `types.go` |

Chat imports this package as `prompt_evolution` / `pe`.

## 7. Integration contracts (do not break casually)

1. **JSON tool I/O:** `{"input": string}` → `{"output": string, "error"?: string}`.  
2. **Delegate task arity:** shard, capability, status.  
3. **Percent scaling:** learning metrics normalized 0–100 for Mangle policy (`normalizePercent`).  
4. **Capability naming:** `normalizeCapabilityName` on register.  
5. **Tool name sanitization** before compile.

## 8. Not for external packages

Unexported helpers (`shouldGenerateToolNeed`, AST emitter internals, stage Mangle record helpers) may change without notice. Prefer Orchestrator methods.
