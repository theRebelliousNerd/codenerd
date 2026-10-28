# autopoiesis internals

Verified 2026-09-20 against the `main` working tree (docs-only change; no
Go files modified). All line references are to `internal/autopoiesis/`.

## The loop (ouroboros.go)

`ExecuteWithConfig` (`ouroboros.go:343`) drives stages named by `LoopStage`
(`LoopStage.String`, `autopoiesis_types.go:144`) and returns a `LoopResult`
(`autopoiesis_types.go:89`). Transitions can be simulated
(`simulateTransition`, `ouroboros.go:693`); halting and convergence are
`shouldHalt` (`:945`) and `hasConverged` (`:959`), with stability tracked by
`updateStability` (`:973`) and scored by `stabilityScore` (`:1348`).
Successful tools commit through `commitTool` (`:793`); per-iteration panic
containment is `handlePanic` (`:923`). `GenerateToolFromCode` (`:1228`) is
the entry for code that already exists, and `sanitizeToolName` (`:1209`)
normalizes names before registration.

## Jev tool checks (tool_jev.go)

As built 2026-10-28 (switch `typesafe.tool_checks`;
`Docs/architecture/jev-turn-judgments.md`, "Tool checks").

- **Before generating.** `ExecuteWithConfig` asks `existingToolFor`
  (`ouroboros.go:399`). It asks Jev whether a registered tool already does
  what the need asks (`learnjudge.CheckDuplicate`).
  - On a yes, the loop returns that tool as `LoopResult.ReusedTool`
    (`autopoiesis_types.go`) and generates nothing.
  - `recordGenerationLearning` skips a reuse.
- **After the safety check.** `codeMismatch` (`ouroboros.go:538`) asks whether
  the code does what was asked and whether it does something unrequested
  (`learnjudge.CheckCode`).
  - A mismatch becomes a `ViolationRequestMismatch` (`checker.go`). It goes
    back through `RegenerateWithFeedback` under the safety retry budget.
- **`GenerateToolFromCode`.** Both checks run before its safety check
  (`ouroboros.go:1355`). It refuses a duplicate or a mismatch with Jev's
  reason.
- **The decisions.** They are the truth tables `decideDuplicate` and
  `decideCode` (`tool_jev.go:125`, `:174`), read against the typesafe yes and
  no thresholds.
- **No answer.** A missing, failed or switched-off Jev is no answer: the loop
  generates and accepts as before, and logs why.

## Detection (tool_detection.go)

`DetectToolNeed` (`:94`) builds a `ToolNeed` (`:54`); I/O types are coerced
by `coerceToolNeedTypes` (`:45`) and `coerceToolIOType` (`:30`), and the
need can be refined with the LLM (`refineToolNeedWithLLM`, `:172`).
`enforceToolNeedContract` (`tool_generation.go:55`) pins the handoff into
generation.

## Generation (tool_generation.go)

`GenerateTool` (`:64`) funnels into `generateToolCode` (`:362`), which has
two backends: `generateToolCodeLegacy` (`:371`) and
`generateToolCodeWithJIT` (`:437`). Feedback regeneration has sibling paths
in `RegenerateWithFeedback` (`:139`), `regenerateToolCodeWithFeedback`
(`:220`), and the `...Legacy` / `...JIT` variants (`:234`, `:297`).
Tests come from `generateTestCode` (`:502`) with `generateFallbackTests`
(`:531`) as backup; schemas from `generateSchema` (`:696`); stage
time-boxing from `stageBudget` (`:723`) and `describeStageTimeout` (`:750`).

## Safety (checker.go, tool_validation.go)

`SafetyChecker.Check` (`checker.go:261`) walks AST facts from
`ExtractASTFacts` (`:182`), propagates panic-call information
(`propagatePanicCalls`, `:207`), and judges imports against an allow-list
built by `buildAllowedPackages` (`:402`); violations are classified
(`classifyImportViolation`, `:504`) and described (`describeViolation`,
`:518`), with kinds in `ViolationType` (`:81`) rendered by `String`
(`:95`). Policy text comes from `goSafetyPolicy` (`:21`) loaded by
`loadGoSafetyPolicy` (`:42`); reload failure surfaces as
`PolicyLoadError` (`:174`). Failures render for the refiner via
`FormatViolationsForFeedback` (`:778`). Before safety, structure is checked
by `ToolGenerator.validateCodeAST` (`tool_validation.go:47`), which uses
`extractFunctionSignatures` (`:284`), `checkFunctionBody` (`:195`), and
`findUsedImports` (`:259`).

## Compilation (compiler.go)

`Compile` (`:36`) validates size (`validateToolSize`, `:151`) and
identifiers (`isValidGoIdentifier`, `:170`), locates a toolchain
(`detectGoBinary`, `:270`; `checkGoToolchain`, `:301`), reuses cached
binaries (`checkCachedBinary`, `:317`) or stores new ones
(`cacheBinaryCopy`, `:344`), and runs them via `executeToolBinary`
(`:220`). `CompileAndRegister` (`:371`) is the composed path;
`GenerateAndCompileFallback` (`:449`) the degraded path. `ValidateGoCode`
(`:531`) has an offline fallback, `validateGoCodeOffline` (`:594`), and
`generateStdoutParser` (`:184`) bridges binary output back to strings.

`ToolCompiler.writeWrapper` (`tool_compiler.go`) wraps a non-`main`
`func(context.Context, string) (string, error)` in the generated-stdin-v1
envelope. A returned result is encoded whether or not an error came with it,
so a partial result survives beside the `error` field; the error still exits
1 (verified 2026-10-28, `tool_compiler_partial_test.go`). Binaries compiled
before that keep the old wrapper until regenerated.

## Execution (registry.go, yaegi_executor.go)

`RuntimeRegistry.Register` (`registry.go:127`) stores tools; `Execute`
(`:48`) runs them; `Get` / `List` / `Has` / `Remove`
(`:156` / `:172` / `:183` / `:193`) manage them; `Restore` (`:207`)
revives persisted entries; registration notifies `ToolRegisteredCallback`
(`:14`). The interpreter path validates imports (`validateImports`,
`yaegi_executor.go:216`), wraps snippets (`wrapCode`, `:265`), picks an
entry point (`resolveEntryPoint`, `:165`, over `entryPointCandidates`,
`:191`) against the policy allow-list (`getAllowedPackages`, `:280`, via
`AllowedPackages`, `:101`). `executeByPolicy` (`ouroboros.go:1098`)
chooses the mode (`ToolExecutionMode.String`, `ouroboros.go:1124`).

## Adversarial testing (panic_maker.go, thunderdome.go)

`PanicMaker.GenerateAttacks` (`panic_maker.go:79`) builds `AttackVector`s
(`:45`) from a prompt (`buildAttackPrompt`, `:115`), parses
(`parseAttackResponse`, `:163`) and filters (`filterAttacks`, `:188`)
them, with `generateFallbackAttacks` (`:222`) when the LLM is unavailable.
`Thunderdome.Battle` (`thunderdome.go:108`) stages the tool in an arena
(`prepareArena`, `:186`), finds a callable entry (`findEntryPointCall`,
`:389`, aided by `isContextParam` / `isStringParam` / `isErrorResult`,
`:499` / `:512` / `:521`), runs each attack (`runAttack`, `:530`), and
formats the outcome for the refiner (`FormatBattleResultForFeedback`,
`:606`). Outcomes re-enter the kernel as facts
(`buildThunderdomeResultFacts`, `thunderdomeOutcomeAtom`,
`ouroboros.go:1387`, `:1378`, categorized by
`sanitizeThunderdomeCategory`, `:1360`).

## Learning and patterns (feedback.go, patterns.go, autopoiesis_feedback.go)

`RecordLearning` (`feedback.go:391`) persists `ToolLearning` (`:360`) with
size clamps (`truncate`, `:599`; `clamp`, `:606`); `GenerateMangleFacts`
(`:515`) re-exports learnings as kernel facts. `PatternDetector`
(`patterns.go:16`) accumulates `DetectedPattern`s (`:23`) in
`RecordExecution` (`:45`) with `calculatePatternConfidence` (`:151`), read
back via `GetPatterns` (`:123`) and `GetToolPatterns` (`:137`), and
surfaced on the façade (`GetToolPatterns` / `GetAllPatterns`,
`autopoiesis_feedback.go:83`, `:88`). Quality scoring is
`EvaluateToolQuality` (`:73`) with an LLM variant (`:78`);
`ExecuteAndEvaluate` (`:310`) composes run plus score, and
`ShouldRefineTool` (`:93`) gates refinement on recent executions
(`getRecentExecutionsForTool`, `:145`).

## Tool-generation audit records (traces.go)

`TraceCollector` owns `ToolGenerationRecord` values describing requests,
declared design metadata, generated artifacts, usage counts, and observed
outcomes. `RecordPrompt` retains the supplied prompts. `RecordResponse` records
only token usage and elapsed time; it neither stores the response body nor
extracts or reconstructs intermediate deliberation. `FinalizeTrace` records the
artifact and outcome. Records persist in `tool_generation_records.json`;
pre-change extracted-prose files remain untouched and are not loaded.
`AnalyzeGenerations` aggregates success rates, quality, issues, and explicitly
supplied design metadata into `GenerationAudit` and `OptimizationOpportunity`
values. `LogInjector` adds entry, exit, error, timing, API-call, and iteration
logging to generated code; `ValidateLogging` checks those requirements.

## Prompt evolution (prompt_evolution/evolver.go)

The judge receives task requests, actions, execution results, source references,
and usage counts. Provider thought summaries are excluded from the execution
record and judge prompt. New feedback rows leave the legacy `thought_summary`
column empty; historical values are excluded on read.

`PromptEvolver` (`:144`) runs `RunEvolutionCycle` (`:284`), records
outcomes (`RecordExecution`, `:253`), gates cycles on `ShouldRunEvolution`
(`:831`), serves pins (`servingPinFor`, `:422`), stores atoms
(`storeEvolvedAtom`, `:435`; `marshalGeneratedAtom`, `:461`), promotes or
rejects (`PromoteAtom`, `:550`; `RejectAtom`, `:601`), with config
validated up front (`validateEvolverConfig`, `:82`;
`DefaultEvolverConfig` at `:67`). Usage feeds back through
`RecordAtomUsage` (`:619`) and `applyExecutionOutcomeLocked` (`:815`);
stats via `GetStats` (`:701`) and `ExportStats` (`:868`).

## Metrics (metrics.go)

`AutopoiesisMetrics` (`:24`) renders via `ExportMetrics` (`:53`),
`Fields` (`:102`), and `String` (`:121`); per-run latency is recorded by
`recordGenerationLatency` (`ouroboros.go:1078`) into `OuroborosStats`
(`autopoiesis_types.go:172`).
