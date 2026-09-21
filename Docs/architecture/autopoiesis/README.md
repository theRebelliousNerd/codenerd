# autopoiesis

Verified 2026-09-20 against the `main` working tree (this rewrite touches
docs only; no Go files changed). Line references are to
`internal/autopoiesis/` unless stated otherwise.

## What it is

Package `autopoiesis` (`autopoiesis.go:1-6`) implements self-modification:
detecting a missing capability, generating a Go tool for it, safety-checking
and compiling the result, executing it, and learning from the outcome. The
`Orchestrator` is a thin façade spread over `autopoiesis_tools.go`,
`autopoiesis_feedback.go`, `autopoiesis_agents.go`, and
`autopoiesis_kernel.go`; the work happens in single-purpose engines for
detection (`tool_detection.go`), generation (`tool_generation.go`,
`toolgen.go`), AST safety (`checker.go`, `tool_validation.go`), compilation
(`compiler.go`), the runtime registry (`registry.go`), the interpreted
fallback (`yaegi_executor.go`), the generate-check-run loop (`ouroboros.go`),
adversarial testing (`panic_maker.go`, `thunderdome.go`), and
feedback/learning (`feedback.go`, `patterns.go`, `traces.go`, `metrics.go`).
Prompt self-improvement lives in the `prompt_evolution` subpackage
(`prompt_evolution/evolver.go:144`, `PromptEvolver`).

## The pipeline in one pass

1. Detect: `Orchestrator.DetectToolNeed` (`autopoiesis_tools.go:20`) calls
   `DetectToolNeed` (`tool_detection.go:94`), which returns a `ToolNeed`
   (`tool_detection.go:54`) with I/O types coerced by
   `coerceToolNeedTypes` (`tool_detection.go:45`) and refinable with the LLM
   (`refineToolNeedWithLLM`, `tool_detection.go:172`).
2. Generate: `Orchestrator.GenerateTool` (`autopoiesis_tools.go:49`) calls
   `GenerateTool` (`tool_generation.go:64`), which emits a `GeneratedTool`
   (`tool_generation.go:16`) plus tests (`generateTestCode`, `:502`) and a
   schema (`generateSchema`, `:696`) under per-stage budgets (`stageBudget`,
   `:723`).
3. Check: `Orchestrator.CheckToolSafety` (`autopoiesis_tools.go:240`) calls
   `SafetyChecker.Check` (`checker.go:261`), reporting `SafetyReport` /
   `SafetyViolation` (`checker.go:62`, `:71`); structural validation also
   exists in `ToolGenerator.validateCodeAST` (`tool_validation.go:47`).
4. Compile: `ToolCompiler.Compile` (`compiler.go:36`) validates
   (`ValidateGoCode`, `:531`), reuses cached binaries (`checkCachedBinary`,
   `:317) or stores new ones (`cacheBinaryCopy`, `:344`).
   `CompileAndRegister` (`:371`) is the composed path;
   `GenerateAndCompileFallback` (`:449`) the degraded one.
5. Persist and load: `ToolGenerator.WriteTool` writes the file atomically
   (`writeFileAtomic`, `toolgen.go:85`) and `RegisterTool` (`toolgen.go:101`)
   puts it in the `RuntimeRegistry` (`Register`, `registry.go:127`);
   `LoadExistingTools` (`toolgen.go:122`) rehydrates the registry from disk
   and `hotReload` (`ouroboros.go:983`) swaps new versions in.
6. Run: `Orchestrator.ExecuteGeneratedTool` (`autopoiesis_tools.go:209`)
   calls `OuroborosLoop.ExecuteTool` (`ouroboros.go:1042`); mode dispatch is
   `executeByPolicy` (`ouroboros.go:1098`) over `RuntimeTool.Execute`
   (`registry.go:48`) or the Yaegi interpreter
   (`YaegiExecutor.ExecuteToolCode`, `yaegi_executor.go:107`).
7. Learn: `Orchestrator.RecordExecution` (`autopoiesis_feedback.go:18`)
   feeds `LearningStore.RecordLearning` (`feedback.go:391`);
   `ShouldRefineTool` (`autopoiesis_feedback.go:93`) decides,
   `ToolRefiner.Refine` (`feedback.go:113`) regenerates with feedback
   (`RegenerateWithFeedback`, `tool_generation.go:139`), and learnings flow
   back into prompts (`AggregateLearningsForPrompt`,
   `autopoiesis_feedback.go:188`) and the kernel (`SyncLearningsToKernel`,
   `autopoiesis_kernel.go:291`).

## Integration surface

- Construction: `NewOrchestrator` (`autopoiesis_orchestrator.go:51`); stats
  via `GetOuroborosStats` (`autopoiesis_tools.go:214`) and `ExportMetrics`
  (`metrics.go:53`).
- Kernel bridge: `SetKernel` (`autopoiesis_kernel.go:21`) accepts a
  `types.Kernel`; the package asserts facts (`assertToolRegistered`, `:198`;
  `assertMissingTool`, `:228`; `GenerateMissingToolFacts`,
  `ouroboros.go:1299`; `GenerateToolRegistrationFacts`, `:1318`) and reads
  policy back (`ShouldGenerateTool`, `autopoiesis_kernel.go:533`;
  `ShouldRefineToolByKernel`, `:548`; `QueryNextAction`, `:507`). Rates
  cross the boundary as int64 (`normalizePercent`, `:338`).
- Injection seams: `PromptAssembler` (`autopoiesis_types.go:26`),
  `ToolSynthesizer` (`:37`), `ToolRegisteredCallback` (`registry.go:14`),
  `SetAgentDefinitionWriter` (`autopoiesis_agents.go:268`),
  `SetPromptAssembler` on the generator (`toolgen.go:54`) and the refiner
  (`feedback.go:269`), `SetLearningsContext` on the generator
  (`toolgen.go:46`) and the loop (`ouroboros.go:1195`).
- Agents: specs on disk via `writeAgentSpec`
  (`autopoiesis_agents.go:20`), listed with `ListAgents` (`:118`), memory
  updated with `UpdateAgentMemory` (`:173`).

## How it is verified

`go build` and `go test ./...` pass on the verification date. Engine tests
live alongside the code. One placement quirk is documented in
WIRING-AND-NOT-BUILT: `TestThunderdomeArena` is defined in the production
file `thunderdome.go:293`, not in a `_test.go` file.
