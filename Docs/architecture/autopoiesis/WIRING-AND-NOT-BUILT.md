# autopoiesis: wiring and what is NOT built

Verified 2026-09-20 against the `main` working tree (docs-only change; no
Go files modified). Read from `internal/autopoiesis/` (façade files,
`ouroboros.go`, `registry.go`, `toolgen.go`, `feedback.go`, `traces.go`,
`thunderdome.go`, `panic_maker.go`) and the `prompt_evolution`
subpackage.

## Wired and reachable (inside the package)

- Façade delegates: every `Orchestrator` method in
  `autopoiesis_tools.go` forwards to an engine (`DetectToolNeed`, `:20`;
  `GenerateTool`, `:49`; `ExecuteOuroborosLoop`, `:120`;
  `WriteAndRegisterTool`, `:106`; `ExecuteGeneratedTool`, `:209`;
  `CheckToolSafety`, `:240`; `ListGeneratedTools`, `:219`;
  `HasGeneratedTool`, `:234`; `GetToolInfo`, `:229`;
  `GetOuroborosStats`, `:214`). The feedback façade
  (`autopoiesis_feedback.go`) does the same for record, evaluate, refine,
  trace, and audit operations (`:18`-`:465`).
- Learning round-trip: `RecordLearning` (`feedback.go:391`) →
  `GetLearning` / `GetAllLearnings` (`:491` / `:498`) →
  `AggregateLearningsForPrompt` (`autopoiesis_feedback.go:188`) →
  `RefreshLearningsContext` (`:296`); kernel export via
  `GenerateLearningFacts` (`:305`) and `SyncLearningsToKernel`
  (`autopoiesis_kernel.go:291`).
- Kernel fact interface: `GenerateMissingToolFacts` /
  `GenerateToolCapabilityFacts` / `GenerateToolRegistrationFacts`
  (`ouroboros.go:1299` / `:1306` / `:1318`) produce facts; `assertToKernel`
  (`autopoiesis_kernel.go:180`) is the single write path behind
  `assertToolRegistered` (`:198`), `assertMissingTool` (`:228`),
  `assertToolHotReloaded` (`:236`), `assertToolLearning` (`:250`),
  `assertToolKnownIssue` (`:256`), and `assertAgentCreated` (`:262`).
  Policy reads back through `ShouldGenerateTool` (`:533`),
  `ShouldRefineToolByKernel` (`:548`), `QueryNextAction` (`:507`), and the
  `Query*` introspection methods (`:361`-`:443`). Existing tools sync at
  startup (`syncExistingToolsToKernel`, `:33`) with drift reported by
  `VerifyKernelToolParity` (`:110`) as a `ToolParityReport` (`:83`).
- Disk to registry: `WriteTool` (`toolgen.go:63`) plus `writeFileAtomic`
  (`:85`) → `RegisterTool` (`:101`) → `RuntimeRegistry.Register`
  (`registry.go:127`); restarts rehydrate via `LoadExistingTools`
  (`toolgen.go:122`) and `Restore` (`registry.go:207`); version swaps go
  through `hotReload` (`ouroboros.go:983`) with versions from
  `versionFromBinding` (`:1022`).
- Adversarial results re-enter the loop as facts:
  `buildThunderdomeResultFacts` (`ouroboros.go:1387`) via
  `thunderdomeOutcomeAtom` (`:1378`); `Battle` (`thunderdome.go:108`)
  consumes `AttackVector`s defined in `panic_maker.go:45`, so the
  maker-to-arena path is type-checked, not conventional.
- The package consumes `types.Kernel` (`SetKernel`,
  `autopoiesis_kernel.go:21`) and an `LLMClient` (e.g.
  `NewTraceCollector`, `traces.go:139`); it produces Mangle facts for the
  kernel to decide on. Session and command wiring that triggers generation
  lives outside this package and was not re-traced in this pass — the
  seams listed above are the complete set it can hang off.

## Exists but not pinned to a caller in this pass

- Tracing variants `GenerateToolWithTracing`
  (`autopoiesis_feedback.go:404`) and `ExecuteOuroborosLoopWithTracing`
  (`:433`) sit beside the plain paths (`GenerateTool`,
  `autopoiesis_tools.go:49`; `ExecuteOuroborosLoop`, `:120`); which
  callers prefer them was not established here. Treat both as live.
- Dual codegen backends (`generateToolCodeLegacy` vs
  `generateToolCodeWithJIT`, `tool_generation.go:371` vs `:437`), dual
  refiner paths (`refineLegacy` vs `refineWithJIT`, `feedback.go:128` vs
  `:189`), and dual validators (`ValidateGoCode` vs
  `validateGoCodeOffline`, `compiler.go:531` vs `:594`) all ship;
  selection logic was not pinned to a caller in this pass.
- `PanicMaker` and `Thunderdome` are fully wired internally, but whether
  the main loop battles every generation was not pinned to the loop body
  in this pass.
- Agent management (`ListAgents`, `GetAgent`, `DeleteAgent`,
  `UpdateAgentMemory`, `autopoiesis_agents.go:118` / `:151` / `:167` /
  `:173`) persists specs and YAML (`writeAgentSpec`, `:20`;
  `writeAgentPromptsYAML`, `:274`) with an `AgentDefinitionWriter` seam
  (`:268`); no in-package caller drives the agent lifecycle — agents are
  created through `executeAgentCreation` (`:238`) only when the analysis
  stage emits that action.

## Assumed by the design, not done by the code

- `autopoiesis.go` is a stub: package doc (lines 1-6) plus
  `var _ = time.Now` (line 32) keeping the `time` import alive. All
  behaviour lives in the satellite files.
- Its modularization note (lines 13-29) is stale and must not be quoted:
  it sizes `autopoiesis_types.go` at "~170 lines" but declarations run
  past line 335 (`LearnedPattern`, `autopoiesis_types.go:328-335`); it
  sizes `autopoiesis_feedback.go` at "~380" but methods run past line 465
  (`ExecuteOuroborosLoopWithTracing`, `:433-465`); it sizes
  `autopoiesis_kernel.go` at "~380" but methods run to line 560
  (`ShouldRefineToolByKernel`, `:548-560`); it sizes
  `autopoiesis_agents.go` at "~210" but functions run past line 327
  (`renderFallbackAgentPromptsYAML`, `:308-327`); it sizes
  `autopoiesis_tools.go` at "~190" but methods run past line 242
  (`CheckToolSafety`, `:240-242`).
- `TestThunderdomeArena` (`thunderdome.go:293`) is a test function
  committed inside a production file, so test-only code ships in the same
  file as `Battle`.
- Names promise autonomy; the code structures assistance. The loop
  generates, checks, and refines, but every policy decision crosses the
  kernel boundary (`ShouldGenerateTool`, `QueryNextAction`,
  `autopoiesis_kernel.go:533`, `:507`) — with no kernel attached the
  engines are libraries, not an agent. `LoopResult` and `OuroborosStats`
  (`autopoiesis_types.go:89`, `:172`) report outcomes; they do not act.

## Wave 2 reconciliation (2026-09-25, verified against the code)

| Claim above | Classification | Evidence |
|---|---|---|
| `autopoiesis.go` is a stub whose modularization note is stale | fixed | Commit bdf2213: the per-file line counts and the `var _ = time.Now` import keeper are gone; the package doc names the files by concern and says where policy decisions cross the kernel. |
| `TestThunderdomeArena` is a test function committed in a production file | not true | It is text inside the harness template that `Thunderdome.generateTestHarnessWithCall` (`thunderdome.go`) returns; `prepareArena` writes that text into the arena's `harness_test.go` and `runAttack` runs it with `-test.run=TestThunderdomeArena`. No test function ships in the package. (The older `generateTestHarness` wrapper is on the dead-code baseline; `generateTestHarnessWithCall` is the live one.) |
| Which callers use the tracing variants | pinned | `GenerateToolWithTracing` is called by the quality-profile path (`autopoiesis_profiles.go`) and itself calls `ExecuteOuroborosLoopWithTracing` (`autopoiesis_feedback.go`). |
| Dual codegen / refiner / validator backends: selection not pinned | pinned | `generateToolCode` and `ToolRefiner.Refine` take the JIT path when `jitEnabled` and the assembler is `JITReady`, else the legacy prompt (`tool_generation.go`, `feedback.go`). `validateGoCodeOffline` no longer exists. |
| Whether the main loop battles every generation | pinned: yes, when enabled | `ExecuteOuroborosLoop` enters the Thunderdome whenever the loop was built with `EnableThunderdome`, which the orchestrator sets when it builds the loop (`autopoiesis_orchestrator.go`) -- `ouroboros.go`, the `StageThunderdome` block: PanicMaker generates attacks, `Battle` runs them, and each result re-enters as `thunderdome_result` facts. A PanicMaker failure skips the battle with a warning. |
| Agent management has no caller | still true, open | `ListAgents`, `GetAgent`, `DeleteAgent` and `UpdateAgentMemory` have no production caller. Agents are created by `executeAgentCreation` and then never listed, read, updated or deleted. Wiring them needs an operator surface (`cmd/nerd`, lane B) or a kernel-derived agent lifecycle; neither exists to wire to. |
