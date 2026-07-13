# 02 — Current State: Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Package: `internal/autopoiesis/`

## 1. Inventory summary

| Class | Count (approx.) | Notes |
|-------|----------------:|-------|
| Non-test `.go` (root package) | ~37 | Modular orchestrator + Ouroboros + safety/learn |
| Non-test `.go` (`prompt_evolution/`) | ~8 | SPL |
| Test `.go` | ~30 | Root + prompt_evolution + benches |
| Local `.mg` | 0 | Uses embedded core defaults |
| Package `README.md` | 1 | Architecture 2.0.0 note (JIT-driven) |

## 2. Role of each major file

### Orchestrator modular surface

| File | Role |
|------|------|
| `autopoiesis.go` | Package doc + modularization map |
| `autopoiesis_types.go` | Config, stages, RuntimeTool, actions, agent memory types |
| `autopoiesis_orchestrator.go` | `Orchestrator`, `NewOrchestrator`, JIT attach, `CompileTool` |
| `autopoiesis_kernel.go` | Kernel set/sync/assert/query helpers |
| `autopoiesis_delegation.go` | `ProcessKernelDelegations`, listener |
| `autopoiesis_analysis.go` | `Analyze`, `QuickAnalyze`, `ExecuteAction` |
| `autopoiesis_tools.go` | Tool/Ouroboros public wrappers |
| `autopoiesis_feedback.go` | Learning/eval/trace wrappers |
| `autopoiesis_profiles.go` | Profile CRUD + evaluate-with-profile |
| `autopoiesis_agents.go` | Disk agent lifecycle |
| `autopoiesis_helpers.go` | Confidence throttle, sort, JSON/hash helpers |

### Tool lifecycle

| File | Role |
|------|------|
| `tool_detection.go` | `ToolNeed`, pattern maps, `DetectToolNeed` |
| `tool_generation.go` | LLM generate/regenerate, schema |
| `tool_templates.go` | Code scaffolds |
| `tool_validation.go` | Static validation helpers |
| `tool_compiler.go` | `go build` forge |
| `toolgen.go` | Generator scaffolding utilities |
| `runtime_registry.go` | Menagerie: register, restore, execute |
| `yaegi_executor.go` | Interpreted stdlib-only execute path |
| `ouroboros.go` | Transactional loop + internal Mangle engine |
| `checker.go` | SafetyChecker + AST fact extraction |
| `thunderdome.go` | Adversarial arena |
| `panic_maker.go` | Attack vector generation |

### Analysis & learning

| File | Role |
|------|------|
| `complexity.go` | Campaign/complexity heuristics + LLM |
| `persistence.go` | Persistent agent need detection |
| `quality.go` | QualityEvaluator + issue taxonomy |
| `patterns.go` | PatternDetector |
| `feedback.go` | ToolRefiner, LearningStore |
| `profiles.go` | ToolQualityProfile store |
| `traces.go` | TraceCollector, LogInjector |

### Subpackage

| File | Role |
|------|------|
| `prompt_evolution/types.go` | Categories, problem types, records |
| `prompt_evolution/evolver.go` | Main SPL orchestrator |
| `prompt_evolution/judge.go` | LLM-as-judge |
| `prompt_evolution/classifier.go` | Problem/error classification |
| `prompt_evolution/atom_generator.go` | Atom creation from failures |
| `prompt_evolution/feedback_collector.go` | Execution outcome collection |
| `prompt_evolution/strategy_store.go` | Strategy database |

## 3. Hotspots (change carefully)

1. **`ouroboros.go`** — stage ordering, halt, stats, sim/commit; PRD header constraints.  
2. **`checker.go` + `go_safety.mg` (core defaults)** — security boundary for generated code.  
3. **`autopoiesis_kernel.go`** — fact predicate contracts with policy/schemas.  
4. **`runtime_registry.go` Execute** — process isolation / path security.  
5. **Chat `process.go` generate_tool branch** — can skip Ouroboros.  
6. **`factory.go` initAutopoiesisAndBrowser** — boot identity of Orchestrator.

## 4. Config & defaults (live)

From `DefaultConfig` / `DefaultOuroborosConfig` / Thunderdome defaults:

- Tools/agents under workspace `.nerd/`  
- Min tool confidence 0.75; max 3 tools/session  
- Thunderdome on; 5s attack timeout; 100MB memory limit default  
- Compile/execute timeouts 300s  
- Max tool source 100KB  

User tool generation OS/arch can override via `internal/config` user config.

## 5. Consumers (runtime)

| Consumer | Path |
|----------|------|
| Cortex boot | `internal/system/factory.go` |
| Chat process / tools / evolution | `cmd/nerd/chat/*` |
| UI dashboard | `cmd/nerd/ui/autopoiesis_page.go` |
| Instruction CLI | `cmd/nerd/cmd_instruction.go` |
| Systems CLI | `cmd/nerd/cmd_systems.go` |
| Campaign | `internal/campaign/tool_pregenerator.go`, `intelligence_gatherer.go` |
| Verification | `internal/verification/verifier.go` |
| E2E | `tests/e2e/autopoiesis_kernel_ouroboros_integration_test.go` |

## 6. Maturity snapshot

| Area | Maturity |
|------|----------|
| Ouroboros stages | High (implemented end-to-end) |
| Kernel fact sync | High for register/learn; partial for all internal engine facts |
| Learning loop | High unit coverage |
| Agent persistence specs | Medium (disk CRUD; scheduling external) |
| SPL | Medium–high code; ops policy for auto-promote |
| Sandboxing | Medium (policy + Thunderdome + Yaegi option; not full container) |

## 7. On-disk artifact contract

See README layout. Registry **restore** requires both source `.go` and compiled binary; orphan binaries are skipped. Description restored from `// Description:` header comments when present.
