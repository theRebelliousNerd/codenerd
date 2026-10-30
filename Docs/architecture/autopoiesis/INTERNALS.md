# autopoiesis internals

Verified 2026-09-20 against the `main` working tree (docs-only change; no
Go files modified). All line references are to `internal/autopoiesis/`.

## Learned-strategy runtime context contract

The strategy provider must forward the prompt compiler's runtime context to
`ProblemClassifier.ClassifyWithContext`. Jev's configured problem timeout
still bounds the call; pattern fallback remains available when Jev supplies no
answer. Preserve the existing empty classification shard argument and cache
identity for strategy lookups: this repair changes runtime ownership, not the
classification evidence. Usage tracking, observers and exchange turn/task
correlation must follow the shared compilation's originating context. Compiler
shutdown cancels that work; canceling an individual waiting caller does not.

As built, `prompt_evolution/strategy_atoms.go:64` accepts the runtime context
and forwards it at line 78. `prompt_evolution/strategy_context_test.go:128` uses the real prompt
compiler, production kernel adapter, evolver provider and TypeSafe client to
check accounting and exchange correlation. `prompt_evolution/strategy_context_lifecycle_test.go:19`
and `prompt_evolution/strategy_context_controls_test.go:17` cover cancellation, join ordering,
cache reuse and fallback controls. These fixtures are authored; their execution
and race verification remain pending.

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

## Jev autopoiesis checks (autopoiesis_jev.go, quality_jev.go)

As built 2026-10-28 (switch `typesafe.autopoiesis_checks`;
`Docs/architecture/jev-turn-judgments.md`, "Autopoiesis checks"). They
replace the keyword quick checks `QuickAnalyze`, `ShouldTriggerCampaign` and
`ShouldCreatePersistentAgent`, which are deleted.

- **Campaign and persistent agent.** The chat's section 1.8
  (`cmd/nerd/chat/process.go:749`) calls `SuggestForRequest`
  (`autopoiesis_jev.go:118`) on every turn that is not a direct answer.
  - Jev reads the request (`learnjudge.JudgeRequestScope`): several
    deliverables, staged work, many files, work that continues after the
    request. They land as `autopoiesis_judgment(Name, Percent)`.
  - The keyword heuristics (`ComplexityAnalyzer.Analyze`,
    `PersistenceAnalyzer.Analyze`) still run; a match lands as
    `autopoiesis_heuristic(/campaign | /persistent)`.
  - The kernel decides (`policy/jev_autopoiesis.mg`):
    `autopoiesis_campaign_suggested(Source)` and
    `persistent_agent_suggested(Kind, Source)`. A Jev judgment, whatever it
    says, outranks the heuristic; the heuristic decides only a request Jev
    did not judge.
  - A derived campaign runs the clarifier in place of the turn, as the
    keyword match did before; a persistent agent is a warning naming
    `/define-agent`. Both name their evidence (`jev` or `heuristic`).
  - The previous request's evidence is retracted before each request.
- **Tool output quality.** `QualityEvaluator.outputIssues`
  (`quality_jev.go:38`) replaces the pattern loop in `Evaluate` and
  `EvaluateWithProfile`. Jev reads the output (`learnjudge.JudgeToolOutput`):
  an error instead of a result, partial results, more pages, a rate limit.
  - A judgment at or above `judgment_yes_percent` is the issue; below it, no
    issue, even where the old pattern would match ("found 3 lines: ERROR ..."
    is a result, not a failure).
  - `empty_or_minimal` is an exact test of the output and always runs.
- **Plumbing.** `NewOrchestrator` builds the client from
  `Config.UserConfig.TypeSafe` (`jevUseFrom`, `autopoiesis_jev.go:36`);
  `SetJev` (`:53`) replaces it.
- **No answer.** Jev unconfigured, switched off, failed or timed out (bounded
  by `turn_judgment_timeout`) is no evidence: the heuristics decide, and the
  log says so (`jev: autopoiesis checks unanswered ..., the keyword heuristic
  decides`).
- **Tests.** `autopoiesis_jev_test.go`: the kernel truth table row by row,
  the three no-answer fallbacks, evidence that does not outlive its request,
  and the output truth table.

## Detection (tool_detection.go)

`DetectToolNeed` (`:94`) asks the kernel whether the request is a capability
gap (`decideToolGap`, `tool_need.go:77`). The Orchestrator installs that decider
when it attaches its kernel. Jev's `asks_for_tool`,
`needs_missing_operation` and `failed_for_missing_tool` judgments and the
`missingCapabilityPatterns` match become `tool_need_judgment` and
`tool_need_pattern` facts. `policy/jev_learning.mg` derives
`tool_gap_observed`, and the regex counts only when Jev gave no judgment. A
failed attempt alone is never a gap. Until 2026-10-28 any non-empty failed
attempt made one. Without a kernel, the pattern alone decides, and the log says
so.

Only after a gap does the LLM write the spec (`refineToolNeedWithLLM`, `:161`).
It no longer returns `needs_new_tool`. I/O types are coerced by
`coerceToolNeedTypes` (`:45`) and `coerceToolIOType` (`:30`), and
`enforceToolNeedContract` (`tool_generation.go:55`) pins the handoff into
generation.

Tool value (`tool_value.go`): per call, `tool_call_hit` comes from the
output-judgment request. Per tool, `JudgeToolValue` (`:72`, shown by
`nerd autopoiesis tools`) asserts `tool_duplicate_judgment` and
`tool_harness_judgment`. The kernel derives `tool_value_evidence` for the value
rules. See Docs/architecture/jev-turn-judgments.md, "Tool-need trigger" and
"Tool value".

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

Jev in the cycle (`prompt_evolution/jev*.go`, switches `task_verdicts`,
`problem_types` and `atom_checks`; Docs/architecture/jev-turn-judgments.md):

- Step 1, judging, is a cascade (`evaluateBatch`, `jev_verdict.go:166`). Jev
  answers did-what-was-asked, stopped-early, one file-needed question per
  changed file, and a failure-kind Choice with its companion.
  `verdictFromJudgments` (`:73`) is the truth table. The LLM judge
  (`judge.go`) judges only records Jev leaves unsettled or did not answer. A
  failure Jev reads as no mistake of the agent's is `NO_AGENT_MISTAKE`.
- The problem type is Jev's Choice with a companion (none fits gives `other`).
  It is cached per task (`classifyWithJev`, `classifier.go:57`), and the regex
  classifier is the logged fallback.
- Step 3, buying a generation: `agentMistakes` drops `NO_AGENT_MISTAKE`
  failures. `groupWarrantsEvolution` buys a generation only when Jev reads one
  shared, preventable cause. Otherwise the group counts in
  `GroupsWithoutSharedCause` and its evidence stays unconsumed.
- Step 4, auto-promotion: `promotionHolds` (`jev_promotion.go:39`) holds an atom
  that describes one past task or goes against a promoted atom. While the check
  is on, promotion at record time waits for the cycle.

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
