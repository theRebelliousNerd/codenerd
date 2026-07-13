# Autopoiesis — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Module: `codenerd`  
> Primary sources: `internal/autopoiesis/`, `internal/autopoiesis/prompt_evolution/`  
> Scale (approx., non-test vs test): **~45** non-test Go files; **~30** test files; **0** local `.mg` (policy embedded via `internal/core` defaults: `go_safety.mg`, `schemas_state.mg`)

## 1. Overview

**Autopoiesis** (Greek: self-creation) is codeNERD’s subsystem for detecting capability gaps and producing durable runtime capabilities without shipping every utility as first-party code. It does **not** replace the Mangle kernel as executive. The LLM proposes tool source, attack vectors, refinements, and prompt atoms; logic and safety gates decide what may compile, register, and run.

### What it owns

1. **Need detection** — campaign complexity, persistent agents, missing tools (heuristics + optional LLM).  
2. **Ouroboros tool lifecycle** — generate → safety → Thunderdome → simulate → compile → register → execute.  
3. **Learning loops** — execution feedback, pattern detection, refinement, quality profiles, reasoning traces.  
4. **Kernel I/O** — assert/query facts (`tool_registered`, `missing_tool_for`, `delegate_task`, learnings, agents).  
5. **Prompt evolution (SPL)** — subpackage that judges executions and proposes new prompt atoms/strategies.

### What it does *not* own

| Concern | Real owner |
|---------|------------|
| Campaign execution | `internal/campaign` |
| `permitted(...)` policy | `internal/core/defaults/policy/` |
| Tool *routing* for normal OODA actions | VirtualStore / shards |
| Primary interactive UX | `cmd/nerd/chat`, `cmd/nerd/ui` |
| Persistent store schema for tools | On-disk under `.nerd/tools` (not Vectryx) |

### High-level control flow

```
user input / delegate_task(/tool_generator, Cap, /pending)
        │
        ▼
 Orchestrator.Analyze | QuickAnalyze | ProcessKernelDelegations
        │
        ├─ ComplexityAnalyzer ──► ActionStartCampaign (payload only)
        ├─ PersistenceAnalyzer ─► ActionCreateAgent ──► .nerd/agents/
        └─ ToolGenerator / OuroborosLoop
                │
                ▼
   Proposal → SafetyChecker(go_safety.mg + AST facts)
                │
                ▼
   Thunderdome (PanicMaker attacks) ── kill? → regenerate
                │
                ▼
   Mangle state machine (schemas_state.mg) simulation
                │
                ▼
   ToolCompiler → RuntimeRegistry → onToolRegistered
                │
                ▼
   Parent kernel facts (AutopoiesisBridge) + LearningStore
```

Fact-flow placement (session):

```
user input → perception → user_intent → kernel next_action
  → (optional) generate_tool / delegate_task
  → Orchestrator / Ouroboros → tool_registered / tool_binary_path
  → VirtualStore can execute generated tools via SetToolExecutor
  → articulation / TUI
```

---

## 2. Implementation status

| Component | Status | Evidence |
|-----------|--------|----------|
| `Orchestrator` hub | **Implemented** | `autopoiesis_orchestrator.go` |
| Modular orchestrator surface | **Implemented** | `autopoiesis_*.go` split (types, kernel, delegation, agents, analysis, tools, feedback, profiles, helpers) |
| Complexity analysis | **Implemented** | `complexity.go` heuristic + LLM |
| Persistence / agent specs | **Implemented** | `persistence.go`, `autopoiesis_agents.go` |
| Tool need detection | **Implemented** | `tool_detection.go`, patterns in helpers |
| Tool code generation | **Implemented** | `tool_generation.go`, `tool_templates.go`, `toolgen.go` |
| Ouroboros transactional loop | **Implemented** | `ouroboros.go` (`Execute` / `ExecuteWithConfig`) |
| SafetyChecker + AST→facts | **Implemented** | `checker.go` + embedded `go_safety.mg` |
| Thunderdome + PanicMaker | **Implemented** | `thunderdome.go`, `panic_maker.go` |
| ToolCompiler + RuntimeRegistry | **Implemented** | `tool_compiler.go`, `runtime_registry.go` |
| Yaegi executor (alt path) | **Implemented** | `yaegi_executor.go` (stdlib-only sandbox) |
| Quality / patterns / refine | **Implemented** | `quality.go`, `patterns.go`, `feedback.go` |
| Profiles / traces / log inject | **Implemented** | `profiles.go`, `traces.go`, orchestrator wrappers |
| Kernel bridge wiring | **Implemented** | `SetKernel`, `AutopoiesisBridge` in `internal/core/kernel_utils.go` |
| Boot wiring | **Implemented** | `internal/system/factory.go` `initAutopoiesisAndBrowser` |
| Chat hot path | **Implemented** | `cmd/nerd/chat/process.go` QuickAnalyze / generate_tool |
| CLI systems status | **Implemented** | `cmd/nerd/cmd_systems.go`, instruction ProcessKernelDelegations |
| UI dashboard | **Implemented** | `cmd/nerd/ui/autopoiesis_page.go` |
| Prompt evolution SPL | **Implemented** | `prompt_evolution/*` |
| Campaign pregen consumers | **Implemented** | `internal/campaign/tool_pregenerator.go`, `intelligence_gatherer.go` |
| Full parity: Yaegi vs compile path | **Partial** | Compile/binary is primary; Yaegi is alternate/safety-oriented |
| Universal kernel-gated generate_tool | **Partial** | Mix of direct chat paths and `delegate_task` |
| Agent runtime scheduling | **Partial** | Specs written to disk; ongoing scheduler not this package’s full job |
| E2E full Ouroboros+LLM | **Partial** | Strong unit coverage; e2e emphasizes kernel contracts/smoke |

**Overall:** living production subsystem — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/autopoiesis/
  autopoiesis.go                 # package marker / modularization note
  autopoiesis_types.go           # Config, LoopStage, RuntimeTool, actions
  autopoiesis_orchestrator.go    # NewOrchestrator, Config, JIT attach
  autopoiesis_kernel.go          # SetKernel, fact asserts, code-element queries
  autopoiesis_delegation.go      # ProcessKernelDelegations, listener
  autopoiesis_analysis.go        # Analyze, QuickAnalyze, ExecuteAction
  autopoiesis_tools.go           # Detect/Generate/Ouroboros wrappers
  autopoiesis_feedback.go        # RecordExecution, refine, traces wrappers
  autopoiesis_profiles.go        # Quality profiles
  autopoiesis_agents.go          # Agent CRUD on disk
  autopoiesis_helpers.go         # throttling, patterns, JSON helpers
  ouroboros.go                   # transactional state machine
  checker.go                     # SafetyChecker
  tool_detection.go | tool_generation.go | tool_compiler.go
  tool_templates.go | tool_validation.go | toolgen.go
  runtime_registry.go | yaegi_executor.go
  complexity.go | persistence.go
  quality.go | patterns.go | feedback.go | profiles.go
  traces.go | thunderdome.go | panic_maker.go
  prompt_evolution/              # SPL subpackage
    evolver.go | judge.go | classifier.go | atom_generator.go
    feedback_collector.go | strategy_store.go | types.go
  README.md
  *_test.go                      # extensive unit coverage
```

### 3.2 Largest non-test sources (approx. lines)

| Path | ~Lines | Role |
|------|-------:|------|
| `ouroboros.go` | 1100+ | State machine: proposal/audit/thunderdome/sim/commit |
| `traces.go` | ~1000 | Reasoning traces + log injection |
| `quality.go` | ~770 | QualityEvaluator heuristics + LLM |
| `prompt_evolution/evolver.go` | ~770 | SPL orchestration |
| `checker.go` | ~720 | AST facts + Mangle safety policy |
| `tool_generation.go` | ~650 | LLM code/test generation |
| `thunderdome.go` | ~620 | Adversarial arena |
| `feedback.go` | ~600 | Refiner + LearningStore |
| `tool_templates.go` | ~590 | Scaffold templates |
| `prompt_evolution/strategy_store.go` | ~570 | Strategy DB |
| `persistence.go` | ~550 | Persistent agent need detection |
| `autopoiesis_kernel.go` | ~450 | Kernel facts + queries |
| `autopoiesis_feedback.go` | ~450 | Orchestrator learning surface |
| `tool_compiler.go` | ~360 | `go build` forge |
| `autopoiesis_orchestrator.go` | ~370 | Construction + wiring |
| `profiles.go` | ~370 | Per-tool quality expectations |
| `runtime_registry.go` | ~200 | Register / restore / Execute |

### 3.3 Subsystems by responsibility

| Subsystem | Types | Persistence |
|-----------|-------|-------------|
| Detection | `ComplexityAnalyzer`, `PersistenceAnalyzer`, `ToolGenerator.DetectToolNeed` | none (ephemeral analysis) |
| Generation | `ToolGenerator`, templates, validation | `.nerd/tools/*.go` |
| Governance | `OuroborosLoop` + internal `mangle.Engine` | loop-local facts; parent kernel on success |
| Safety | `SafetyChecker`, embedded policy | none |
| Adversarial | `PanicMaker`, `Thunderdome` | temp work dir |
| Runtime | `RuntimeRegistry`, `RuntimeTool.Execute` | `.compiled/` binaries |
| Learning | `QualityEvaluator`, `PatternDetector`, `LearningStore`, `ToolRefiner` | `.learnings/` |
| Profiles | `ProfileStore`, `ToolQualityProfile` | `.profiles/` |
| Traces | `TraceCollector`, `LogInjector` | `.traces/` |
| Agents | `AgentCreator`, disk layout | `.nerd/agents/` |
| SPL | `PromptEvolver` et al. | strategies / atoms via prompt system |

---

## 4. Orchestrator (control plane)

### 4.1 Construction

`NewOrchestrator(client LLMClient, config Config)` (`autopoiesis_orchestrator.go`):

- Builds `ComplexityAnalyzer`, `ToolGenerator`, `PersistenceAnalyzer`, `AgentCreator`.  
- Builds `OuroborosLoop` with Thunderdome enabled by default, 100KB max tool size, 300s compile/execute timeouts.  
- Builds quality/learning/trace/profile stores under `config.ToolsDir` subdirs.  
- Optionally enables Gemini grounding/thinking via `research.GroundingHelper` / `ThinkingHelper` when the client supports them.  
- Registers `SetOnToolRegistered` callback → `assertToolRegistered`.  
- Calls `RefreshLearningsContext()` so the first generation sees restored learnings.

`DefaultConfig(workspaceRoot)`:

| Field | Default |
|-------|---------|
| `ToolsDir` | `<ws>/.nerd/tools` |
| `AgentsDir` | `<ws>/.nerd/agents` |
| `MinConfidence` | 0.6 |
| `MinToolConfidence` | 0.75 |
| `EnableToolGeneration` | true |
| `MaxToolsPerSession` | 3 |
| `MaxLearningFacts` | 1000 |
| Target OS/Arch | from `internal/config` tool generation defaults + user config |

### 4.2 Analyze vs QuickAnalyze

| API | LLM | Use |
|-----|-----|-----|
| `Analyze` | yes if `EnableLLM` | full `AnalysisResult` + prioritized `AutopoiesisAction`s |
| `QuickAnalyze` | no (heuristics) | chat hot path: campaign / persistent hints; kernel code-element counts can raise complexity |
| `ShouldTriggerCampaign` / `ShouldCreatePersistentAgent` | heuristic/LLM paths | chat process |

**Important:** `ActionStartCampaign` in `ExecuteAction` is a **no-op** here — campaign start is owned by the campaign package; analysis only recommends.

### 4.3 Action types

```go
ActionNone, ActionStartCampaign, ActionGenerateTool, ActionCreateAgent, ActionDelegateToShard
```

`ExecuteAction` implements tool generation (direct `ToolGenerator` write/register, **not** full Ouroboros) and agent creation. Full hardened generation is `ExecuteOuroborosLoop`.

### 4.4 Kernel-mediated generation

`ProcessKernelDelegations` (`autopoiesis_delegation.go`):

1. `QueryPredicate("delegate_task")`  
2. Match shard `/tool_generator` or `tool_generator`, status `/pending` or `pending`  
3. Build `ToolNeed` (confidence/priority 1.0)  
4. `ouroboros.Execute`  
5. Assert `tool_registered…` or `tool_generation_failed` / `tool_delegation_complete`  

`StartKernelListener` polls on a ticker (chat lifecycle can cancel).

Chat also has **direct** paths: on `next_action` key `generate_tool`, `DetectToolNeed` + `GenerateTool` (lighter path than Ouroboros) in `cmd/nerd/chat/process.go`.

---

## 5. Ouroboros Loop (deep dive)

### 5.1 Stages (`LoopStage`)

| Stage | Meaning |
|-------|---------|
| `StageDetection` | Need accepted / nil checks |
| `StageSpecification` | LLM generates (or regenerates with feedback) |
| `StageSafetyCheck` | `SafetyChecker.Check` |
| `StageThunderdome` | PanicMaker attacks + Battle |
| `StageSimulation` | Differential Mangle transition validation |
| `StageCompilation` | Compile + register (commit) |
| `StageRegistration` / `StageExecution` / `StageComplete` | post-commit lifecycle markers |
| `StagePanic` | panic recovery path |

### 5.2 Protocol (`ExecuteWithConfig`)

Documented in source as:

1. **Proposal** — `GenerateTool` / `RegenerateWithFeedback`; optional Mangle sanitizer on embedded logic.  
2. **Audit** — safety; retry up to `RetryConfig.MaxRetries` (default 3).  
3. **Thunderdome** — if enabled; kill → regenerate up to `MaxPanicRetries` (default 2).  
4. **Simulation** — `simulateTransition` via `mangle.NewDifferentialEngine`.  
5. **Commit** — `commitTool` (write, compile, registry, callback).  
6. **Hot-reload** — optional `hotReload` facts in internal engine; orchestrator asserts `tool_hot_loaded` / `tool_version` to parent.

**Halt conditions:** Mangle policy (`shouldHalt`) for max iterations, retries, stagnation, degradation.

**Panic recovery:** deferred `recover` → `handlePanic` records penalties/stats.

### 5.3 Internal Mangle engine

`NewOuroborosLoop` creates a **dedicated** `mangle.Engine` (not the session kernel), loads embedded `schemas_state.mg` from `core.GetDefaultContent`, restores registry from disk. Parent-kernel facts are asserted only through Orchestrator callbacks / wrappers.

### 5.4 Compile path

`ToolCompiler.Compile` (`tool_compiler.go`):

- Temp module, rewrite `package tools` → `main`, wrapper entry if needed.  
- `go mod tidy` + `go build` with `GOOS`/`GOARCH` from config.  
- Optional `replace codenerd=<WorkspaceRoot>`.  
- Output under `.compiled/`, SHA-256 hash.

`RuntimeTool.Execute`: absolute binary path required; JSON stdin `{"input":...}`; JSON stdout `{output,error}`; scrubbed env via `toolExecutionEnv()`.

### 5.5 Yaegi alternate

`YaegiExecutor` interprets `RunTool(input string) (string, error)` with a **stdlib whitelist** (no `os`, `net`, `syscall`, `unsafe`). Exists to avoid compile hangs/dependency hell; not the primary Ouroboros commit path.

---

## 6. Safety surface

### 6.1 SafetyChecker

- Loads embedded `go_safety.mg` at init (`core.GetDefaultContent("go_safety.mg")`).  
- `ExtractASTFacts` walks Go AST → `ast_import` / `ast_call` style facts; propagates panic callees.  
- Allowed packages built from config (`AllowNetworking`, `AllowFileSystem`, `AllowExec`).  
- `SafetyReport{Safe, Violations, Score}`; violation types include forbidden import, dangerous call, unsafe pointer, reflection, CGO, exec, panic, goroutine leak, policy, PanicMaker kill.

### 6.2 Forbidden classes (policy + design)

As documented in package README and enforced by policy/AST:

| Class | Examples / reason |
|-------|-------------------|
| Dangerous imports | `unsafe`, `syscall`, `os/exec`, often `net/http` when networking disallowed |
| Dangerous calls | recursive delete, unsafe pointers |
| Resource attacks | Thunderdome memory/timeouts |
| Session gas | `MaxToolsPerSession`, `MaxLearningFacts`, Ouroboros `MaxIters` |

### 6.3 Defaults that matter

Orchestrator’s OuroborosConfig: `AllowNetworking: false`, `AllowFileSystem: true`, `AllowExec: true` — tools may use FS/exec depending on policy details; networking off by default.

---

## 7. Learning & quality

### 7.1 Execution feedback

`RecordExecution` (`autopoiesis_feedback.go`):

1. `QualityEvaluator.Evaluate` if needed  
2. `PatternDetector.RecordExecution`  
3. `LearningStore.RecordLearning`  
4. Assert `tool_learning` / `tool_known_issue`  
5. `RefreshLearningsContext` into generators  

Quality dimensions (`QualityAssessment`): Completeness, Accuracy, Efficiency, Relevance; issue types include incomplete, pagination, rate_limit, auth, etc.

### 7.2 Refinement

`ShouldRefineTool` uses average quality & pattern confidence. `RefineTool` / `ToolRefiner` regenerate code with feedback history (JIT assembler when attached).

### 7.3 Profiles

Per-tool `ToolQualityProfile` (duration bands, output size, caching, tool type taxonomy: quick_calculation, data_fetch, background_task, …). Generated via LLM and stored under `.profiles/`.

### 7.4 Traces

`ReasoningTrace` captures prompts, chain-of-thought extraction, success/failure; `LogInjector` injects mandatory logging into generated code.

---

## 8. Prompt evolution (`prompt_evolution/`)

Separate package under autopoiesis implementing **System Prompt Learning**:

```
Execute → Evaluate (TaskJudge) → Evolve (meta-prompt / AtomGenerator)
  → Integrate (JIT / strategy store)
```

Key types (`types.go`): `ErrorCategory`, `ProblemType`, `ExecutionRecord`, `AgentAction`.  
`PromptEvolver` (`evolver.go`) config: min failures, interval, max atoms, confidence threshold, auto-promote, strategy refine threshold, judge model default `gemini-3-pro`.

Wired from chat: `cmd/nerd/chat/delegation.go`, `commands_evolution.go`, session boot (`session_boot.go` / `session_shared_boot.go`).

---

## 9. Kernel fact surface (parent kernel)

Asserted / consumed (representative; see `autopoiesis_kernel.go`, delegation, tools):

| Predicate | When |
|-----------|------|
| `tool_registered(Name, Timestamp)` | register / restore sync |
| `tool_hash`, `tool_capability`, `tool_description`, `tool_binary_path` | register |
| `tool_hot_loaded`, `tool_version` | hot reload |
| `missing_tool_for(Intent, Cap)` | DetectToolNeed |
| `tool_learning`, `tool_known_issue` | execution learnings |
| `agent_created`, `agent_purpose`, `agent_schedule`, `agent_trigger`, `agent_has_memory` | agents |
| `delegate_task` (query) | kernel delegation |
| `tool_generation_failed`, `tool_delegation_complete` | delegation outcomes |

`tool_registered` is treated as **persistent** in core fact categories (survives disk load paths).

Bridge: `core.NewAutopoiesisBridge(kernel)` implements `types.KernelInterface`.

---

## 10. Integration map

### 10.1 Boot

`internal/system/factory.go` → `initAutopoiesisAndBrowser`:

```go
autopoiesisConfig := autopoiesis.DefaultConfig(bctx.workspace)
bctx.poiesis = autopoiesis.NewOrchestrator(bctx.llmClient, autopoiesisConfig)
bridge := core.NewAutopoiesisBridge(bctx.kernel)
bctx.poiesis.SetKernel(bridge)
// Ouroboros → VirtualStore tool generator + executor
```

Cortex exposes `Orchestrator *autopoiesis.Orchestrator`.

### 10.2 Chat / CLI / UI

| Surface | Behavior |
|---------|----------|
| `chat/process.go` | `QuickAnalyze`, campaign/persist hints, `generate_tool` action |
| `chat/helpers_tools.go` | List/Get/Execute/Evaluate, Ouroboros `generateTool` |
| `chat/commands_tools.go` | tool listing when autopoiesis present |
| `chat/model_key_handler.go` | Alt+A dashboard refresh from patterns/learnings |
| `ui/autopoiesis_page.go` | Patterns + Learnings tabs |
| `cmd_instruction.go` | `ProcessKernelDelegations` after instruction runs |
| `cmd_systems.go` | Autopoiesis status subcommands |
| `verification.verifier` | imports package (safety/quality reuse) |
| `campaign/*` | tool pregeneration / intelligence uses types |

### 10.3 Interfaces for DI / cycles

- `KernelInterface` / `KernelFact` aliases of `types.*`  
- `PromptAssembler` — articulation JIT without import cycle  
- `ToolSynthesizer` — mockable Ouroboros  
- `LLMClient` — local minimal complete API  

---

## 11. Throttling & gas

| Control | Location | Purpose |
|---------|----------|---------|
| `MaxToolsPerSession` | Config / helpers | hard cap per session |
| `MinToolConfidence` | `shouldGenerateToolNeed` | avoid low-confidence spam |
| `ToolGenerationCooldown` | Config | rate limit |
| `MaxLearningFacts` | SyncLearningsToKernel | retract/bound learning facts |
| Ouroboros `MaxIters` / retries | ExecuteConfig | halt infinite loops |
| Thunderdome timeout/memory | ThunderdomeConfig | sandbox budget |

---

## 12. Testing (summary)

Package tests cover: complexity heuristics, safety checker, Ouroboros happy paths / panic persistence, feedback store races, delegation, profiles, toolgen gaps, thunderdome normalize, Yaegi paths (where present), prompt_evolution unit tests.

E2E: `tests/e2e/autopoiesis_kernel_ouroboros_integration_test.go` — bridge binding, NaN floats, retract overlap, rapid asserts, nil kernel, rebind, atom coercion (contract resilience more than full LLM generation).

Commands:

```bash
go test ./internal/autopoiesis/...
go test ./internal/autopoiesis/prompt_evolution/...
go test ./tests/e2e/ -run Autopoiesis
```

---

## 13. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md). Headline honest gaps:

1. Dual generation paths (light `GenerateTool` vs full Ouroboros) can diverge in safety depth.  
2. Campaign action is advisory only inside autopoiesis.  
3. Agent specs on disk ≠ always-on scheduler.  
4. Internal Ouroboros engine vs parent kernel fact parity still partial by design (hot-reload GAP-019 partially fixed).  
5. Yaegi vs binary execution policy not unified.  
6. Thunderdome “compile for arena” path is heavy; failures may skip adversarial phase.  
7. Prompt evolution auto-promote needs careful ops discipline (JIT pollution risk).

---

## 14. Observability (summary)

Primary logger: `logging.CategoryAutopoiesis` / `logging.Autopoiesis` / `AutopoiesisDebug`. Stage banners in Ouroboros (`=== OUROBOROS LOOP START ===`). Stats: `OuroborosStats`, `ThunderdomeStats`. UI: patterns/learnings dashboard. Traces under `.traces/`.

Details: [11-OBSERVABILITY.md](11-OBSERVABILITY.md).

---

## 15. Design principles (short)

1. LLM proposes; Mangle/safety disposes.  
2. Prefer kernel-mediated `delegate_task` for multi-system tool generation.  
3. Default deny for dangerous tool capabilities (networking off, safety policy on).  
4. Persist capabilities (binaries + facts) so sessions restore tools.  
5. Learn from executions; inject learnings into next prompts.  
6. Cap gas (session tools, learning facts, loop iters).  
7. JIT-first for prompt behavior (SPL → atoms, not hard-coded shard prose).  

Full set: [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md).

---

## 16. File map: modularization history

`autopoiesis.go` documents a prior 2295-line monolith split into modular `autopoiesis_*.go` files; all remain package `autopoiesis`. Prefer new orchestration methods on `*Orchestrator` near the relevant module file rather than re-growing a god file.

---

## 17. Related PRD comments

`ouroboros.go` carries an embedded Symbiogen-style PRD header mandating transactional semantics, stagnation detection, and panic-as-fact. Treat the **code path** as authority if comments drift; last verified 2026-07-13 code still implements proposal/audit/simulation/commit with Mangle halt checks.

---

**End of living implemented spec.** For API tables see [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md); for wiring journals see [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md).
