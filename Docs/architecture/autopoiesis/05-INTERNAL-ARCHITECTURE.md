# 05 — Internal Architecture: Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Package: `internal/autopoiesis`

## 1. Component diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                         Orchestrator                             │
│  config · client · kernel · throttle counters                    │
│                                                                  │
│  complexity ── persistence ── agentCreate                        │
│  toolGen ──── ouroboros (ToolSynthesizer)                        │
│  evaluator · patterns · refiner · learnings · profiles           │
│  traces · logInjector · promptAssembler                          │
│  grounding · thinking (optional Gemini helpers)                  │
└───────────────┬─────────────────────────────┬────────────────────┘
                │                             │
                ▼                             ▼
     Disk: .nerd/tools · agents      Parent KernelInterface
                                     (AutopoiesisBridge)
                │
                ▼
┌───────────────────────────────────────────┐
│              OuroborosLoop                │
│  toolGen · safetyChecker · compiler       │
│  registry · sanitizer · engine (local)    │
│  panicMaker · thunderdome                 │
└───────────────────────────────────────────┘
```

## 2. End-to-end data flows

### 2.1 Chat hot path (analysis only)

```
processInput
  → QuickAnalyze(input, target)
      → ComplexityAnalyzer.Analyze (heuristic)
      → QueryCodeElementCount / QueryFilesInScope (kernel)
      → PersistenceAnalyzer.Analyze
  → optional ShouldTriggerCampaign / ShouldCreatePersistentAgent
  → (campaign UX / agent creation elsewhere)
```

### 2.2 Chat generate_tool action (light path)

```
next_action generate_tool
  → DetectToolNeed → assert missing_tool_for
  → GenerateTool → WriteTool / RegisterTool (ToolGenerator)
  # may not run full Ouroboros stages
```

### 2.3 Explicit Ouroboros (helpers / slash)

```
DetectToolNeed
  → ExecuteOuroborosLoop
      → RefreshLearningsContext
      → ouroboros.Execute
      → assertToolRegistered + assertToolHotReloaded
      → recordGenerationLearning
```

### 2.4 Kernel delegation path

```
Policy asserts delegate_task(/tool_generator, Cap, /pending)
  → ProcessKernelDelegations | StartKernelListener
  → generateToolFromDelegation
  → ouroboros.Execute
  → assertToolRegistered + tool_delegation_complete
```

### 2.5 Execution + learning path

```
ExecuteGeneratedTool / ExecuteAndEvaluateWithProfile
  → RuntimeTool.Execute (subprocess JSON I/O)
  → QualityEvaluator
  → PatternDetector + LearningStore
  → tool_learning / tool_known_issue facts
  → optional RefineTool
```

### 2.6 Prompt evolution path (subpackage)

```
ExecutionRecord (chat delegation)
  → FeedbackCollector / TaskJudge
  → Classifier (problem + error category)
  → AtomGenerator / StrategyStore
  → PromptEvolver cycle (interval + min failures)
  → JIT atom registration (prompt package)
```

## 3. Ouroboros state machine

```
                 ┌─────────────┐
                 │  Detection  │ need validated
                 └──────┬──────┘
                        ▼
              ┌─────────────────┐
         ┌───►│ Specification   │◄── regenerate w/ feedback
         │    └────────┬────────┘
         │             ▼
         │    ┌─────────────────┐
         │    │ Safety Check    │── fail & retries left ──┐
         │    └────────┬────────┘                         │
         │             │ pass                             │
         │             ▼                                  │
         │    ┌─────────────────┐                         │
         │    │ Thunderdome     │── kill & retries left ──┤
         │    └────────┬────────┘                         │
         │             │ survive / skip                   │
         │             ▼                                  │
         │    ┌─────────────────┐                         │
         │    │ Simulation      │── fail → return         │
         │    └────────┬────────┘                         │
         │             ▼                                  │
         │    ┌─────────────────┐                         │
         │    │ Compile/Register│── fail → return         │
         │    └────────┬────────┘                         │
         │             ▼                                  │
         │         Complete / hotReload                   │
         │                                                │
         └──────── shouldHalt?  / max iters ──────────────┘
```

Mangle facts in the **local** engine track iterations, retries, panic_maker_verdict, battle_hardened, halt conditions (via `schemas_state.mg`).

## 4. Key type clusters

### Need / action

- `ToolNeed`, `ComplexityResult`, `PersistenceNeed`, `AnalysisResult`, `AutopoiesisAction`, `QuickResult`

### Tool artifacts

- `GeneratedTool`, `ToolSchema`, `CompileResult`, `RuntimeTool`, `LoopResult`

### Safety / combat

- `SafetyReport`, `SafetyViolation`, `AttackVector`, `AttackResult`, `BattleResult`

### Learning

- `ExecutionFeedback`, `QualityAssessment`, `DetectedPattern`, `ToolLearning`, `ToolQualityProfile`, `ReasoningTrace`

### Agents

- `AgentSpec` (via persistence/agent creator), `AgentMemory`, `Learning`, `LearnedPattern`

## 5. Concurrency model

| Component | Sync |
|-----------|------|
| `Orchestrator.mu` | kernel, counters, assembler |
| `OuroborosLoop.mu` | stats, callback |
| `RuntimeRegistry.mu` | tool map |
| `PatternDetector` / `LearningStore` / profile stores | internal RWMutex |
| `StartKernelListener` | dedicated goroutine + ticker |
| `RuntimeTool.ExecuteCount` | atomic |

Chat lifecycle cancels autopoiesis listener via context (`model_lifecycle.go`).

## 6. Disk layout semantics

| Path | Writer | Reader |
|------|--------|--------|
| `.nerd/tools/*.go` | ToolGenerator / commit | compiler, restore |
| `.nerd/tools/.compiled/*` | ToolCompiler | registry Execute/Restore |
| `.nerd/tools/.learnings/` | LearningStore | RefreshLearningsContext |
| `.nerd/tools/.profiles/` | ProfileStore | QualityEvaluator |
| `.nerd/tools/.traces/` | TraceCollector | analysis helpers |
| `.nerd/agents/<name>/` | AgentCreator / orchestrator | ListAgents, boot agent discovery (system factory may also scan agents for JIT) |

## 7. Error propagation style

Ouroboros returns `*LoopResult` with `Success`, `Stage`, `Error` rather than only `error` — callers must inspect stage. Delegation converts failure to `error` + kernel failure facts. Safety returns structured `SafetyReport`.

## 8. Extension points

| Hook | Purpose |
|------|---------|
| `SetKernel` | attach parent logic |
| `SetPromptAssembler` | JIT prompts for gen/refine |
| `SetOnToolRegistered` | already used by orchestrator for fact sync |
| `ToolSynthesizer` interface | tests / alternate synthesizers |
| `LLMClient` | provider swap |

New stages should extend `LoopStage` carefully and update `String()` + stats.
