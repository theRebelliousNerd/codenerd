# S3 — Forcing is policy over facts; no tool-call count is a completion criterion

Seam S3 of the unblocking program. Serves `req_no_count_as_completion` and
`req_obligation_fixpoint`; addresses `risk_forcing_in_go`.

Base branch `dogfood/c2-closure`. The first agent worked against `03508b55`
and hung after the design; its WIP (`9660e5f1`) was rebased onto the base tip
`a287131f` (clean, no conflicts) in worktree branch
`worktree-agent-a10a856bcbd24ceef`.

## Status

- last updated: 2026-09-18 (complete)
- done:
  - read the vision section (`agents.md`), `internal/mangle/agents.md`, and the
    verification table of `Docs/journeys/02-forcing-and-completion.md`
    (rows 1-15, 35-37, 64-66, refutation 6)
  - audit of every producer and consumer (below)
  - design fixed (below)
  - the Go and Mangle changes (below)
  - the tests (fail-before and pass-after recorded below)
  - the runs and the docs
- open:
  - the items under Open at the end of this file

## Producers and consumers today

### What is already policy (kept, not rewritten)

`internal/context/working_set.mg` decides continuation from facts the loop
asserts. Six stops/finalizes and three nudges, with every threshold already a
fact:

| Rule | Line | Body reads |
|---|---|---|
| `working_stop(/repeated_cycle)` | `:80` | `working_control(/yes, _)` |
| `working_stop(/tool_failures)` | `:81` | `working_control(_, Failed), Failed >= 3` |
| `working_stop(/read_only_stall)` | `:85-87` | `working_progress(/write, Rounds, 0, _, _)`, `working_stall_rounds(24)` `:58` |
| `working_finalize(/verify_after_write)` | `:97-99` | `working_progress`, `working_finalize_rounds(16)` `:57` |
| `working_regime(/commit)` x3 | `:60,68,76` | `working_progress`, `working_commit_rounds(16)` `:56` |
| `working_nudge(/implement\|/verify\|/conclude)` | `:102-108` | `working_progress`, `working_nudge_rounds(8)` `:55` |

The Go side of that contract is *measurement*, and it stays: `working_control/2`
and `working_progress/5` are asserted at `internal/context/working_set.go:180-186`
inside `WorkingSet.Continue`, from the `WorkingProgress` struct that
`toolBudgetController.workingProgress` (`tool_budget_controller.go:334`) computes
and `executor_tools.go:275` supplies. (Refutation 6 of the study: the assertion
site is `working_set.go:180-186`, not the controller, and deleting the producer
silences **five** of the six stops, not three.) So the counting survives; the
ceilings go.

### What is Go-only (the "tool budget") — the delete list

| Thing | Where | Note |
|---|---|---|
| iteration ceiling | `executor_tools.go:207` `for iter := 0; openRounds \|\| iter < budget.iterationLimit; iter++` | |
| controller construction | `tool_budget_controller.go:60` `newToolBudgetController` | |
| adaptive extension | `tool_budget_controller.go:192-238` `maybeExtend`, `toolBudgetExtensionDecision:51`, `toolBudgetObservation.novel/successes/errors`, `seenEvents`, `progressSinceExtension`, `writesSinceExtension`, `verifiesSinceExtension`, `hasSuccessfulWrite`, `baseLimit`, `iterationLimit`, `hardLimit`, `extensionSize`, `maxExtensions`, `extensions` | plus the two extension log lines at `executor_tools.go:429,440` |
| call ceiling | `executor_tools.go:792-815` (`maxToolCalls`, `openCalls`, `"tool call budget exceeded for this turn"`, `"<name>: budget exceeded"`) | `:794` already showed a fully open loop exists behind config |
| count nudge prose | `tool_budget_controller.go:304-330` `nudge` (`"Orchestrator budget: %d tool calls and %d rounds remain."` `:310`), re-appended each round at `executor_tools.go:253-260` | |
| forced-final prose | `executor_tools.go:599,603` `readOnlyBudgetExhaustedNudge` / `writeBudgetExhaustedNudge` | reworded, not deleted |
| tail log + error | `executor_tools.go:411-415` `"Tool iteration budget reached: ..."`, `:434` `"tool iteration budget exhausted (...)"` | |
| config | `core_limits.max_tool_calls`, `max_tool_iterations`, `adaptive_tool_budget`, `tool_iteration_extension_size`, `max_tool_iteration_extensions`, `tool_loop_repeat_threshold` (`internal/config/limits.go:17-47,119-133,138-152`) | |
| executor config | `ExecutorConfig.MaxToolCalls:252`, `MaxToolIterations:257`, `ProgressDrivenTools:261`, `AdaptiveToolBudget:267`, `ToolIterationExtensionSize:271`, `MaxToolIterationExtensions:274`, `ToolLoopRepeatThreshold:278`; defaults `executor.go:382-386` | study row 14 / refutation 4: the fields span `:250-278`, not `:257-265` |
| propagation | `internal/system/factory.go:2000-2040`, `cmd/nerd/cmd_campaign.go:1359-1376`, `internal/session/spawner.go` (inherits via `SetConfig`) | |
| benchmark pins | `cmd/tools/change_benchmark/main.go:167-169` | the `minimal` baseline's own caps stay; they are the comparison, not codeNERD |
| helper | `executor_tools.go:2204` `effectiveMaxToolCalls`, `executor.go:382-386` `defaultMaxToolCalls`/`defaultMaxToolIterations`/`defaultToolIterationExtensionSize`/`defaultMaxToolIterationExtensions`/`defaultToolLoopRepeatThreshold` | |

### The architect's own `.nerd/config.json` (read-only, main checkout)

`core_limits` contains exactly six keys: `max_total_memory_mb`,
`max_concurrent_shards`, `max_concurrent_api_calls`, `max_session_duration_min`,
`max_facts_in_kernel`, `max_derived_facts_limit`.

**None of the six removed keys is set.** Re-read 2026-09-18 after the change:
still none. Nothing has to be deleted from the live config for it to keep
loading.

### Where the working set is (not) constructed

`beginWorkingLoop` (`working_context.go:101-124`) returns a no-op context when
`e.workingWorld == nil`, so a turn with no kernel view runs the tool loop with
no `WorkingSet` and therefore **no policy stops at all**. `WorkingSet` itself is
already nil-world tolerant — only `Select`'s dependency slice reads the world
(`working_set.go:270,304`), and `Continue` (the policy call) reads none of it.
The second gate is `ExecutorConfig.ProgressDrivenTools`, consulted at
`executor_tools.go:182`, `work_steps.go:161` and `executor_tools.go:794`; it is
set true unconditionally by `factory.go:2015` and inherited by spawned agents,
so in production it is vestigial — but it is a Go flag that can silence policy,
which is exactly `risk_forcing_in_go`.

## Design

1. **The loop is unbounded by count.** `for iter := 0; ; iter++`. Its exits are
   `ctx.Err()`, the policy's `working_stop` (error: task unresolved), the
   policy's `working_finalize` (break to the forced-final path), the wall-clock
   exploration cutoff (`FinalAnswerReserve`, a user constraint that stays), the
   model returning no tool calls, and a client that cannot consume tool results.
2. **`toolBudgetController` becomes `workingMeter`** (`tool_budget_controller.go`
   -> `working_meter.go`). It keeps exactly the measurement the policy reads:
   the round/write/since-write/since-verify counters and the trace signatures
   behind `repeatedTailCycle`. Everything that existed to grant extra rounds
   goes with the extensions. The file is renamed because nothing named "budget"
   survives this change; every caller is repointed in the same commit.
3. **The repeat threshold becomes a fact.** `working_repeat_threshold(2).` in
   `working_set.mg`, beside the other four thresholds, read back through
   `WorkingSet.RepeatThreshold` the way `TranscriptRounds` reads
   `working_transcript_rounds` (a base fact answers `QueryFacts`, not `Query`).
   The loop hands the value to the meter. It is **not** re-exposed as config:
   the four thresholds it sits next to are not config either, and a threshold
   the user can lower is a count ceiling wearing a different hat.
4. **No "budget" vocabulary reaches the model.** The count nudge and its
   re-append are deleted outright; the policy-derived nudges keep their Go
   wording function (`workingNudgeText`, `workingRegimeText`) and never mention
   counts of remaining calls. The two forced-final constants are reworded to
   "Exploration is closed for this turn" and renamed to match. Every stop the
   user sees names the derivation, as `executor_tools.go:288` already did.
5. **A working set on every tool-loop path.** `beginWorkingLoop` no longer
   refuses when the world is nil; a workspace root is the only requirement, and
   `NewWorkingSet` takes the nil world (policy needs none). With no root and no
   world there is nothing to build on, so the loop degrades as before; with a
   world and no root the existing "working context requires a workspace" error
   is unchanged, because that combination is a misconfiguration.
   `ProgressDrivenTools` is deleted, so the only remaining question is whether a
   working loop exists.
6. **Removed config keys fail loudly and by name.** `LoadUserConfig` scans the
   raw `core_limits` object before decoding and returns an error naming the
   removed key and what replaced it. This is not a compatibility path: the key
   is not decoded, not stored and not honoured — the load fails.

## Changes

### The policy

- `internal/context/working_set.mg`: `Decl working_repeat_threshold(N)` and the
  base fact `working_repeat_threshold(2).`, beside the four spans it belongs
  with. The comment records that it was `core_limits.tool_loop_repeat_threshold`
  until today.
- `internal/context/working_set.go`: `WorkingSet.RepeatThreshold` reads it back
  through `QueryFacts` (a base fact does not answer `Query`), the way
  `TranscriptRounds` reads `working_transcript_rounds`. Below 2 is an error,
  not a clamp: a threshold of 1 would call every round a cycle.

### The loop

- `executor_tools.go`: `for iter := 0; ; iter++`. The loop's only exits are
  `ctx.Err()`, the derived `working_stop` (returns "task unresolved"), the
  derived `working_finalize` (the one `break`, to the forced-final path), the
  wall-clock exploration cutoff, the model returning no tool calls, and a
  client that cannot consume tool results.
- The `maybeExtend` block and its two "Adaptive tool budget ..." log lines are
  gone; so is the `if finalizeReason == ""` warn after the loop, which is now
  unreachable — `break` is the only non-`return` exit and it only fires with a
  finalize reason. The forced-final failure error names the derivation
  (`working policy finalized the turn (verify_after_write) after N executed
  tool call(s)`) instead of `tool iteration budget exhausted (...)`.
- `executeToolBatch`: the per-turn call ceiling, `openCalls`,
  `effectiveMaxToolCalls` and the `"tool call budget exceeded for this turn"` /
  `"<name>: budget exceeded"` results are deleted. Every call the model makes
  is executed.
- `describeWorkingStop` (in `working_meter.go`) renders a stop for the user:
  the rule by name (`working_stop(/read_only_stall)`) and the facts that
  satisfied it (`working_progress(/write, 24, 0, _, _) with
  working_stall_rounds`). An empty reason is reported as a policy bug, not a
  ceiling.
- `newToolBudgetController` -> `newWorkingMeter(repeatThreshold)`, file
  `tool_budget_controller.go` -> `working_meter.go`; the extension arithmetic,
  the ceilings and the count nudge went with it, the measurement stayed.
  `appendToolBudgetNudge` -> `appendWorkingNudge`.

### The working set on every path

- `working_context.go`: `beginWorkingLoop` no longer refuses on a nil world. A
  declared workspace root is the whole requirement; `Continue` — the policy
  call, and every stop — reads no world.
- New `Executor.workingLoopWorkspace()` is that root, and it is the **declared**
  `WorkspaceRoot` only, never `workspaceForVerification()`'s fallback to a root
  discovered from the process's working directory. Discovery answers "which
  tree do I compile"; it must not answer "where do I persist this turn's
  observations". Every production boot declares one
  (`internal/system/factory.go` resolves it before `SetConfig`, and
  `resolveWorkspaceRoot` always yields something), so chat and shard both get a
  working set. The first attempt used the discovered root and every test in the
  tree started writing `.nerd/context/*.db` into the repo — runtime debris of
  exactly the kind `.gitignore` already calls out for `internal/campaign/.nerd/`.
- New `Executor.workingLoopAvailable()` is the single definition of "this turn
  will have a working loop". `planTurnSteps` (`work_steps.go:161`) used
  `workingWorld != nil && ProgressDrivenTools`; it runs *before*
  `beginWorkingLoop` installs the loop, so it asks the predicate rather than
  reading the context (checked: `executor_tools.go:61` vs `:82`).
- `beginWorkingLoop` also stopped dereferencing a nil `*prompt.CompilationContext`.
  The nil-world early return used to hide it; the Piggyback and forced-final
  paths pass nil, and shard ID and intent target only name the scope and the
  focus, so their absence narrows the working set and never the policy.
- `ProgressDrivenTools` is deleted, so no Go flag can silence policy.
- **`runToolLoopPass` refuses to run without a working loop.** With no ceiling
  and no policy the loop has nothing to end it, so a turn with no declared
  workspace fails loudly (`tool loop requires a working continuation policy and
  this turn has none`) instead of spinning. This is the design's point 1 taken
  seriously: "always progress-driven" has to mean the loop cannot run any other
  way.

### The config

- `internal/config/limits.go`: the six keys, their validation and their
  defaults are deleted from `CoreLimits`. `removedToolBudgetKeys` +
  `rejectRemovedCoreLimitKeys` name each removed key and why it is gone.
- `internal/config/user_config.go`: the rejection runs in `LoadUserConfig`
  *before* `decodeStrictJSON`, so the user sees "core_limits sets 1 removed
  tool-budget key: "max_tool_iterations" (no count of rounds ends a turn;
  delete the key) ..." rather than the decoder's bare `unknown field`.
- `internal/session/executor.go`: `MaxToolCalls`, `MaxToolIterations`,
  `ProgressDrivenTools`, `AdaptiveToolBudget`, `ToolIterationExtensionSize`,
  `MaxToolIterationExtensions`, `ToolLoopRepeatThreshold`, their five
  `default*` constants and their entries in `DefaultExecutorConfig` are gone.
  What remains bounding a turn is wall-clock only.
- `internal/system/factory.go`: the six-key forwarding block and the "Tool loop
  budget: ..." boot line are gone; `execCfg` now carries `WorkspaceRoot` and
  the boot line says continuation is derived.
- `cmd/nerd/cmd_campaign.go`: `applyCampaignExecutorBudget` deleted. **It was
  dead** — grepping `cmd/` and the whole tree found no caller but its own test
  (`cmd_campaign_tool_budget_test.go`, also deleted). Campaign executors come
  from the cortex boot, which is `factory.go`'s `SetConfig`, so nothing loses
  its `WorkspaceRoot`. This contradicts the audit's "propagation" row, which
  listed it as a live site; recorded here rather than silently.
- `cmd/tools/change_benchmark/main.go`: the three pins on codeNERD's executor
  are gone. `maxCalls` (the `boundedClient` LLM-spend cap, both modes) and
  `maxTools` (the `minimal` baseline's own per-turn cap) stay — they are the
  comparison, not codeNERD.

## Tests (fail-before / pass-after evidence)

Fail-before method (the shared stash stack is never used): temporary WIP
commit of the non-test changes, then `git checkout dogfood/c2-closure --
<non-test paths>` — or, where reverting the base file would break the build
rather than the assertion, a temporary in-place reinstatement of exactly the
pre-change behaviour — run the new test, then `git checkout HEAD -- <paths>`.

### New tests

| Test | File | Fail-before method |
|---|---|---|
| `TestRemovedConfigKeys_FailLoudly` | `internal/config/core_limits_load_test.go` | base `limits.go`+`user_config.go` |
| `TestRepeatThreshold_IsAFact` | `internal/context/working_repeat_threshold_test.go` | base `working_set.go`+`.mg` |
| `TestToolLoop_NeverStopsOnCallCount` | `internal/session/working_loop_gating_test.go` | ceilings reinstated |
| `TestToolLoop_StopsOnDerivedReadOnlyStall` (N and N-1) | same | ceilings reinstated |
| `TestToolLoop_StopsOnDerivedRepeatedCycle` | same | ceilings + count nudge reinstated |
| `TestToolLoop_StopsOnDerivedToolFailures` | same | (rewrite of the existing stop test; asserts the derivation by name) |
| `assertNoBudgetVocabulary` + `budgetVocabulary` | same | used by every loop test, by `TestDescribeWorkingStop_NamesTheDerivation`, `TestWorkingNudgeText_ReportsWhatTheTurnDidNotWhatIsLeft` and `TestExplorationClosedNudges_TellTheModelDifferentThings` — this is `TestNoBudgetVocabularyReachesTheModel` spread over the paths that can actually produce the wording, rather than one test that can only check a sample |
| `TestExecuteToolBatch_ExecutesEveryCallPastTheOldCeiling` | `internal/session/executor_budget_exhaustion_test.go` | call ceiling reinstated |
| `TestWorkingMeter_*`, `TestDescribeWorkingStop_*`, `TestWorkingNudgeText_*` | `internal/session/working_meter_test.go` | new symbols; compile failure on base |

### Fail-before output

`internal/config`, base `limits.go` + `user_config.go` restored — every one of
the eight subtests fails:

```
--- FAIL: TestRemovedConfigKeys_FailLoudly/max_tool_iterations
    core_limits_load_test.go:125: core_limits.max_tool_iterations still loads; a removed key must not be silently ignored
    ... (the other five keys identically)
--- FAIL: TestRemovedConfigKeys_FailLoudly/all_six_at_once_are_all_named
    core_limits_load_test.go:147: a config of nothing but removed keys still loads
--- FAIL: TestRemovedConfigKeys_FailLoudly/the_replacement_for_the_repeat_threshold_is_named
    core_limits_load_test.go:160: tool_loop_repeat_threshold still loads
```

`internal/context`, base `working_set.go` + `working_set.mg` restored:

```
internal\context\working_repeat_threshold_test.go:20:16: w.RepeatThreshold undefined (type *WorkingSet has no field or method RepeatThreshold)
FAIL	codenerd/internal/context [build failed]
```

`internal/session`, with the pre-change behaviour reinstated in
`executor_tools.go` — the 8-round `for` bound, the 50-call batch ceiling with
its `"tool call budget exceeded for this turn"` result, and the per-round
`"Orchestrator budget: N tool calls and M rounds remain."` nudge:

```
--- FAIL: TestExecuteToolBatch_ExecutesEveryCallPastTheOldCeiling
    errors = [uncapped_batch_probe: budget exceeded x10], want none
--- FAIL: TestToolLoop_NeverStopsOnCallCount
    runToolLoop: working policy finalized the turn () after 9 executed tool call(s): forced final answer failed: ... — 60 rounds of progress is not a reason to stop
--- FAIL: TestToolLoop_StopsOnDerivedRepeatedCycle
    the model was handed budget vocabulary "budget": "observed\n\n[orchestrator] Orchestrator budget: 49 tool calls and 7 rounds remain."
--- FAIL: TestToolLoop_StopsOnDerivedReadOnlyStall/the_stall_span_derives_the_stop
    err = <nil>, want the working policy to stop a change task that never wrote, by name
--- FAIL: TestToolLoop_StopsOnDerivedReadOnlyStall/one_round_short_of_it_is_not_a_stop
    executed = 9, want 23
```

All pass after the change (`go test ./internal/session/ -run 'TestToolLoop_|...'` ok).

### Existing tests that encoded ceilings or extensions

| Test | Disposition |
|---|---|
| `TestToolBudgetController_GrantsOnlyBoundedProgressExtensions` | **deleted** — extensions are gone |
| `TestToolBudgetController_RefusesRepeatedTraceCycle` | **rewritten** as `TestWorkingMeter_RepeatedTailCycleUsesThePolicySpan` (now also covers period-2, a wider span, and a span below 2) |
| `TestToolBudgetController_ReReadWithChangedEvidenceIsProgress` | **rewritten** into the same test's "changed evidence is progress" subtest |
| `TestToolBudgetController_RefusesErrorsWithoutProgress` | **rewritten** as `TestWorkingMeter_FailedCallIsNotProgress` |
| `TestToolBudgetNudge_IsProgressiveAndAdvertisesBatchCodeDOM` | **deleted** — it asserted "18 tool calls", "3 tool calls", "1 rounds" reach the model |
| `TestToolBudgetController_WriteOriented_NovelReadRefusesWithReadOnlyStall` | **deleted** — the stall is a policy rule, covered end-to-end by `TestToolLoop_StopsOnDerivedReadOnlyStall` |
| `TestToolBudgetController_WriteOriented_WriteGrantsAndResetsCounters` | **rewritten** as `TestWorkingMeter_CountsWhatThePolicyReads` (the counters survive; the grant does not) |
| `TestToolBudgetController_WriteOriented_PostWriteVerificationGrants` | **rewritten** into the same test's verification assertion |
| `TestToolBudgetController_ReadOnlyTask_NovelReadGrants` | **deleted** |
| `TestRunToolLoop_ExtendsProgressingTurnAndFeedsRemainingBudget` | **deleted** — its whole assertion was that "[orchestrator] Orchestrator budget:" reached the model |
| `TestToolBudgetController_NilDeniesWithoutPanic` (`session_uplift_test.go`) | **rewritten** as `TestWorkingMeter_NilIsSafeToObserve` |
| `TestRunToolLoop_ProgressDriven_NudgesOnlyWhenACeilingExists` | **deleted** — "a ceiling from core_limits keeps the nudge" is the behaviour being removed |
| `TestRunToolLoop_ProgressDriven_StopsOnRepeatedToolFailures` | **rewritten** as `TestToolLoop_StopsOnDerivedToolFailures`; now asserts `working_stop(/tool_failures)` and `working_control(_, 3)` by name instead of "stopped by policy" |
| `TestRunToolLoop_ProgressDriven_StopsAChangeTaskThatOnlyReads` | **rewritten** as `TestToolLoop_StopsOnDerivedReadOnlyStall` with the N-1 case added |
| `TestRunToolLoop_ProgressDriven_NudgesAReadTaskToConclude`, `..._FinalizesAChangeTaskThatWroteThenDrifted`, `..._ClosesReadingUnderTheCommitRegime` | kept; only the harness lost its ceiling settings |
| `TestBudgetExhaustedNudges_TellTheModelDifferentThings` | **rewritten** as `TestExplorationClosedNudges_TellTheModelDifferentThings` + a no-budget-vocabulary assertion |
| `TestExecuteToolBatchPiggyback_ReportsEverySkippedCall` | **rewritten** — cancellation, not a ceiling, is now the only reason a call is skipped |
| `TestRunToolLoop_ForcedFinalRunsPostEditBuildGate` | **rewritten** — reaches the forced-final path through `working_finalize(/verify_after_write)` (write, then 16 drifting rounds) instead of `MaxToolIterations=1` |
| `TestExecutor_Process_MaxToolCallsExceeded` | **rewritten** as `TestExecutor_Process_LargeToolBatchIsNotCutOffByACount`; asserts the opposite |
| `TestJourney_InfiniteReaderEndsBoundedAndHonest` | **rewritten** — bound is `working_stall_rounds(24)` and the error must name `read_only_stall`; was `<= 40 (MaxToolCalls)` |
| `TestExecutorConfigSnapshotConcurrentSet` | repointed to `RepairMaxAttempts` (it used `MaxToolCalls` as an arbitrary second field) |
| `TestSpawner_Spawn_InheritsExecutorConfig`, `..._DefaultExecutorConfig_WhenUnconfigured`, `TestSpawner_SetExecutorConfig_NilIsNoop` | repointed to `ToolTimeout`/`RepairMaxAttempts`/`FinalAnswerReserve` |
| `TestRunToolLoop_ReservesTimeForFinalVerdict` | kept; lost its two ceiling settings — the wall clock is the constraint under test |
| `TestDefaultCoreLimits_EnableBoundedAdaptiveToolBudget` and the rest of `internal/config/limits_tool_budget_test.go` | **file deleted** |
| `TestApplyCampaignExecutorBudget_PropagatesAdaptivePolicy` | **file deleted** with the dead function it tested |
| `TestLoadUserConfig_ValidExplicitLimitsLoadUnchanged` | six removed keys dropped from its fixture |

### Tests that had to learn the production shape

A working loop now exists on every tool-loop turn, and with one active the
*initial* generation goes through `CompleteWithToolResults` as a one-message
working request rather than through `CompleteWithTools`
(`executor.go:1392-1395`, unchanged — these turns simply never had a loop
before). Several doubles scripted only `CompleteWithTools` and so saw their
first response dropped:

| Test | Change |
|---|---|
| `TestRunToolLoop_PiggybackInitialEnvelopeIsPromoted`, `..._PiggybackSecondTurnPromoted` | script the initial turn on `CompleteWithToolResults`; new `historyCarriesToolResults` helper tells the initial generation from a follow-up, and `CompleteWithTools` now fails the test if called |
| `TestRunToolLoop_UsesOneClientForTheWholeTurn` | `recordingLLMClient` serves `firstTurnToolCalls` from `CompleteWithToolResults` on a history with no tool results |
| `TestRunToolLoop_ReservesTimeForFinalVerdict` | `deadlineToolLoopClient` likewise; it used to block on `ctx.Done()` from the very first call and time the turn out before a tool ran |
| `TestExecutor_Process_ToolExecution`, `..._SafetyGate`, `..._EmptyToolCallArgs`, `..._LargeToolBatchIsNotCutOffByACount`, `TestRunToolLoop_RejectsNilFollowupResponse`, `TestRunToolLoop_ReservesTimeForFinalVerdict`, `TestSubAgent_Execute_SurfacesToolExecutionError`, `newRoutingExecutor` | given `WorkspaceRoot = t.TempDir()`; they ran a tool loop with no declared workspace, which is now refused |
| `TestForceFinalAnswer_PendingCallPairedBeforeNudge` | looks for "Exploration is closed for this turn" instead of "tool budget"/"exploration budget" |

### Behind the `integration` build tag

`go test ./...` never compiles `tests/e2e/` (`//go:build integration`), so these
were repointed against `go vet -tags integration ./tests/e2e/`, which is clean:

| Test | Change |
|---|---|
| `TestE2E_SchedulerSession_ResourceExhaustion_InfiniteToolLoop` | was "MaxToolIterations limit must break infinite tool requests" with `MaxToolIterations = 5` and a 7-LLM-call ceiling; now every round calls a tool that does not exist, so `working_stop(/tool_failures)` ends it at the third failed round and the error must name the derivation |
| `TestE2E_ContractViolation_ToolSpamming`, `TestE2E_ResourceExhaustion_HugeToolOutput` | dropped `MaxToolCalls = 2`, gained a declared `WorkspaceRoot`; the 10-call batch now executes in full and the turn is bounded by its context |
| `TestE2E_SessionKernel_InvalidConfigRejection` | dropped `cfg.MaxToolCalls = -1 // Invalid`; there is no such field to make invalid |
| `TestE2E_SessionExecutor_InfiniteToolLoop_MaxToolCalls` | left alone — despite the name it asserts the non-`ToolResultsProvider` degradation to one pass, which is unchanged |

## Docs

- `Docs/journeys/02-forcing-and-completion.md` — the "Every config key" table
  now carries a superseded-by-S3 banner. The table itself is left as written:
  it is the verified record of what the code was, and rewriting it would
  destroy the evidence this seam was derived from.
- `Docs/journeys/00-journey-map.md` row 44 and `Docs/architecture/**` are left
  alone for the same reason (and `Docs/architecture` is orientation, never
  evidence — `ce05e8fe`).
- **`.claude/rules/nerd-config-schema.md` is not in this worktree.** `.claude/`
  is gitignored except a few entries and the rule file is untracked, so it
  exists only in the main checkout, which this agent must not touch. The study
  already recorded that its `core_limits` description never listed the six keys
  ("Memory MB, concurrent shards/API, session minutes, kernel fact ceiling"), so
  it is *accidentally* correct after this change and nothing there is now
  wrong — but see Open.

## Commits

On `worktree-agent-a10a856bcbd24ceef`, based on `a287131f`:

| Commit | What |
|---|---|
| `3b609615` | `feat(context)`: `working_repeat_threshold` as a policy fact + `WorkingSet.RepeatThreshold` + its test. Additive to one package, so the base tree compiles against it unchanged. |
| `2fe4bc01` | `refactor(session)!`: the loop, the meter, the config keys, every caller, every test. One breaking change, built clean. |
| (this commit) | `docs(journeys)`: this log and the superseded banner. |

The base branch `dogfood/c2-closure` moved 10+ commits (S4 and S15) while this
seam was in flight, so the squash targeted the fixed SHA `a287131f` rather than
the branch name — resetting onto the moving tip would have reverted the other
sessions' work.

## Full test run

`go build ./...` clean. `go vet` clean on `internal/session`,
`internal/context/...`, `internal/config/...`, `internal/core/...`,
`internal/system`, `cmd/nerd`, `cmd/tools/change_benchmark`.

Required set, all ok: `internal/session` (104s), `internal/context` (193s),
`internal/config`, `internal/core` (294s), `internal/core/defaults`,
`internal/core/defaults/policy`, `internal/core/shards`, `cmd/nerd`,
`cmd/nerd/chat`, `cmd/nerd/ui`.

Full `go test ./...`: **87 ok, 0 failing** (91 packages, 4 with no test files).
`go vet -tags integration ./tests/e2e/` clean.

Three non-findings worth recording, all in packages this change does not
touch and all passing when re-run:

`TestEnsureOnDemandShardsNeverDoubleSpawns` (`internal/core/shards`) failed
once — "expected exactly 1 active shard, got 0" — in a run with five packages
in parallel. It passes `-count=5` alone and the whole package passes; it was
green in both full `go test ./...` runs. `internal/core/shards` is
byte-identical to the base here.

`TestCodexCLIClient_RunHealthProbe_SkillMissingAfterSuccessfulExec`
(`internal/perception`) failed once with `Failure=exec_failed, want
skill_missing` after 10s — its fake CLI could not be spawned inside a 5s
context under full-suite load. It passes in isolation; `internal/perception`
is untouched.

The first full run reported
`FAIL codenerd/internal/browser 601.200s`, a 10-minute package timeout in
`TestStart_WhenLaunchBinaryNotFound_ShouldReturnError` waiting inside
`go-rod/launcher.Launch`. `internal/browser` is byte-identical to the base in
this worktree. Re-run alone it passes in 0.5s, and the whole package in 13s —
it was a first-use Chromium launch, not a regression. (The same window also
produced two transient `could not import ...: The system cannot find the file
specified` build failures from the shared `GOCACHE`, with 114 GB free on C:;
another session was almost certainly trimming the cache concurrently.)

## Open

- `.claude/rules/nerd-config-schema.md` could not be updated from this worktree
  (untracked, main checkout only). It never listed the six removed keys, so it
  is not now wrong, but a line saying `core_limits` carries no tool-call or
  tool-round limit — and that the tool loop is bounded by
  `internal/context/working_set.mg`, not by config — would stop the next reader
  looking for the knob.
- `cmd/nerd/cmd_campaign.go`'s `applyCampaignExecutorBudget` was dead before
  this change (no caller but its own test). Deleting it was right, but the
  reason it was dead is unexamined: either the CLI campaign paths really do get
  their executor config from the cortex boot, or a wiring gap has been sitting
  there since `66585382`. Worth a look, not in S3's scope.
- The refusal to run a tool loop without a working loop is new surface. Every
  production path declares a workspace, and every in-tree test now does, but a
  library consumer constructing an `Executor` without one now gets an error
  where they previously got an unbounded — and before that, a count-bounded —
  loop. That is the intended fail-closed, and it is worth one live run to
  confirm no boot path reaches `runToolLoopPass` before `SetConfig`.
- `working_finalize` is the only way out of the loop that is not a `return`, so
  the forced-final tail is now reachable by exactly one derivation. If a future
  policy adds a second finalize reason, nothing needs to change; if one adds a
  `working_stop` the loop cannot name, `describeWorkingStop` reports it as a
  policy bug rather than inventing a reason.
