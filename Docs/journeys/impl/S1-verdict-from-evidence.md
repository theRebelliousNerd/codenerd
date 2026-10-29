# S1 — The verdict is derived from evidence, never guessed from prose

Seam S1 of the codeNERD unblocking program.
North-star capability `cap_derived_verdict` ("Success and failure are derived from evidence
facts - builds, tests, gates - never guessed from the prose of a result"); requirement
`req_verdict_from_evidence` (`.nerd/northstar.json`).

## Status

- last updated: 2026-09-18
- done:
  - trace of symptoms A, B, C, D; symptom B's producer named from the live log and the
    brief's premise about it refuted with the ordering that refutes it
  - typed verdict crosses observation.Return, types.ShardResult and the chat delegation path
  - `injectShardResultFacts` derives Status / `pending_test` / `pending_review` from evidence;
    all three substring heuristics deleted
  - continuation summary and banner are functions of the verdict; the unconditional
    "Completed %d steps successfully." is deleted
  - symptom B fixed at its producer (`surfaceToolErrors`)
  - `recordBuildState` written — the wiring gap the coordinator flagged — so `build_state`
    and `test_state` are asserted from the gates and retracted per turn
  - fail-before evidence captured for every new test; pass-after green
  - `go build ./...`, `go vet` on the changed packages, targeted package runs, and a full
    `go test ./...`: 87 packages ok, 0 failing
- open: see "Open" below (four items, none blocking)

## Trace

### Symptom B — named producer

The chain that produced
`shard execution failed: tool execution failed: run_build: modular tool execution failed: exit status 1`:

- `cmd/nerd/cmd_direct_actions.go:414` wraps whatever `cortex.SpawnTaskWithTarget` returned.
- `internal/system/factory.go:428` → `TaskExecutor.Execute`.
- `internal/session/task_executor.go` `executeObserved`: the `err != nil` branch wraps with
  `execution failed: %w`; the **`result.Error != nil` branch returns `result.Error`
  unwrapped**. The observed string has no `execution failed:` segment, so the error came from
  `result.Error`, not from a returned error.
- The only assignment to `result.Error` with that exact format was
  **`internal/session/executor.go:1093-1094`**:

  ```go
  if len(toolErrs) > 0 && strings.TrimSpace(result.Response) == "" {
      result.Error = fmt.Errorf("tool execution failed: %s", strings.Join(toolErrs, "; "))
  }
  ```

The brief's premise ("the result text says `Evidence: checks_passed`, so the response was not
empty, so the producer is elsewhere") is **refuted by the ordering**: that sentence is not the
model's. `internal/session/change_evidence.go:105` (`appendEvidenceReport`) *appends*
`"\n\nWrote %d file(s): ...\nEvidence: %s. Requested behavior remains unverified..."` to
`result.Response`, and `appendEvidenceReport` is called ten lines **after** the emptiness test.
So the model returned no final text, the heuristic fired on the empty response, and only
afterwards did the runtime glue its own evidence sentence onto it. That is why the CLI printed
a non-empty "Partial result (failed)" next to a non-zero exit.

Live log
(`C:\CodeProjects\codeNERD\.nerd\logs\20260918_035947.892298800_054828_000001_9eac1a_session.log`):

```
00:05:46 [INFO]  Executing modular tool: run_build with 2 args
00:05:54 [ERROR] Tool call run_build failed: modular tool execution failed: exit status 1
00:06:28 [WARN]  Working policy finalized the turn (verify_after_write) after 49 executed tool call(s); forcing a final answer
00:07:07 [WARN]  build verification FAILED in 8.429s: ... work_steps.go:189:67: result.Intent.FocusFiles undefined
00:08:12 [DEBUG] build verification passed in 13.843s
00:08:12 [INFO]  build repair converged after 2 attempt(s)
00:09:49 [DEBUG] test verification passed in 1m36.763s
00:13:05 [DEBUG] build verification passed in 9.384s
00:14:30 [DEBUG] test verification passed in 1m24.823s
00:14:32 [INFO]  turn_cost session=... turn=1 ... outcome=/failed
```

Every closing gate passed. The turn was still `/failed`, because a tool error from eight
minutes earlier, fully recovered from, decided the verdict. **A mid-turn tool error is not
evidence about the final state of the workspace; the closing gate is.**

### Symptom A/C — status guessed from prose

`cmd/nerd/chat/process_continuation.go:167-222` `injectShardResultFacts` set `shard_result`'s
Status by substring: `"TODO"`/`"FIXME"` → `/incomplete`; a coder result lacking the word
`"test"` → `/code_generated` plus an unconditional `pending_test`; a reviewer result containing
`"issue"` → `pending_review`. `process_continuation.go:155-161` then printed
`"Completed %d steps successfully."` whenever the kernel derived no further continuation — a
function of *the absence of a next step*, not of the verdict of this one.

### Symptom D — the structured verdict exists and was dropped

`ExecutionResult` carries `TurnOutcome` (`/done|/hollow|/failed|/unverified`, set by
`captureTurnOutcome`, `internal/session/executor_memory.go:128`), `ChangeStage`,
`BuildCheck`/`TestCheck`, `Acceptance`, `WrittenPaths`, `UntestedPaths`. `observedReturn`
carried Output/Failure/Changed/Build/Tests/Findings/Notes across — and dropped `TurnOutcome`,
`ChangeStage`, `Acceptance` and `UntestedPaths`. `types.ShardResult` was prose plus an error.

### Wiring gap — `build_state` was declared and never asserted (coordinator addendum, confirmed)

- `internal/session/executor.go:216-219` documented `perTurnBuildStateFacts` as "asserted this
  turn via `recordBuildState`".
- `grep -rn recordBuildState` over the whole tree: **the only hit was that comment.** The field
  was written nowhere and read nowhere — not even by the cleanup that claimed to retract it.
- The only `build_state` assertions in the repo were in `internal/session/turn_done_test.go:39`.
- So the `!build_state(/failing)` conjunct of `turn_executed`
  (`internal/core/defaults/policy/coder_safety.mg:114`) excluded nothing in production: a turn
  whose build failed could still be `turn_executed`, and with an acceptance contract,
  `turn_done`.

`test_state/1` (`internal/core/defaults/schemas_execution.mg:16`), read by `commit_gate.mg`,
`tdd_loop.mg` and `context_compilation.mg`, was never asserted from `TestCheck` either.

## Design

**Decision (required by the brief): the chat delegation path consumes the typed outcome
through `ExecuteObserved`, not through a widened `types.ShardResult`.**

Reason: `observation.Return` is built by `observedReturn` directly from `ExecutionResult`,
which is the object the kernel's verdict is recorded on. `types.ShardResult` is filled at
`internal/core/shards/manager_spawn.go:520` from `agent.Execute(ctx, task) (string, error)` —
a boundary where *no* typed verdict exists on either side, because `types.ShardAgent`
(`internal/types/interfaces.go:170`) returns a bare string. Routing the chat through
`ShardResult` would have meant inventing the verdict at exactly the boundary with no evidence:
moving the guess rather than removing it. `ShardResult` still gains the typed fields
(requirement 1) and fills them with what that path honestly observed.

### A. The verdict crosses the observation boundary

`observation.Return` gains `Outcome`, `Stage`, `Acceptance *Acceptance` and `Untested`;
`observedReturn` fills all four from `ExecutionResult`. `Untested` is a field rather than only
the prose Note built from it because requirement 2 derives `pending_test` by joining on the
paths, not by parsing the sentence. An empty `Outcome` means "the producer had no verdict" and
every consumer treats it as unverified.

### B. Chat delegation calls ExecuteObserved

`ObservedTaskExecutor` gains `ExecuteObservedWithContext(ctx, req, sessionCtx, priority)`,
mirroring `TaskExecutor.ExecuteWithContext`; both observed entry points delegate to the single
`executeObserved`, so the structured return can never drift from the string beside it.
`Model.spawnTaskWithContext` returns `observation.Return`; the five callers that want prose
read `.Output`. An executor that does not implement the interface still runs the task and
returns an observation carrying only the output.

### C. `types.ShardResult` carries the typed outcome

New `Outcome types.MangleAtom` and `Stage string`, filled at every constructor
(`manager_spawn.go:520` and the queue-stress mock). The ShardManager path fills `/failed` on
error and `/unverified` otherwise — never `/done`, because "it did not error" is the whole of
what that path observed. `nerd spawn` reads it: the result heading now says
`Shard Result (unverified):` instead of claiming an unqualified result.

### D. `injectShardResultFacts` derives Status

The `shard_result/5` shape does not change, so the Decl at
`internal/core/defaults/schemas_misc.mg:188` and every rule in
`internal/core/defaults/policy/codedom_continuation.mg` stand unedited. Only the derivation
changes:

| evidence | Status |
|---|---|
| `err != nil` or `Outcome == /failed` | `/failed` |
| `Outcome == /hollow` | `/incomplete` (routes back to the same shard) |
| `Outcome == /done` | `/complete` |
| `Outcome == /unverified` and Go source changed | `/code_generated` |
| `Outcome == /unverified`, no Go source changed | `/unverified` |
| no typed outcome at all | `/unverified` — never `/complete` |

`pending_test` is owed when `Untested` is non-empty, or when non-test Go source changed and no
test run was observed. `pending_review` is owed by structured `Findings`. All three substring
tests are deleted.

### E. The continuation summary states the derived verdict

`continuationSummary(steps, ret)` renders the verdict and then names the evidence — what
changed, what the build said, what the tests said, what acceptance said, how many written files
have no test alongside. `continuationOutcomeFor` maps `/hollow`, `/unverified` and an absent
verdict onto a new `continuationUnverified`, which is now the **zero value** of
`continuationOutcome`: a producer that states no outcome has observed none, and the checkmark
is reserved for a verdict. The banner gains a `⚠️ N step(s) ran; the work is not verified.`
arm.

The same claim in the dream multi-step renderer (`**Status**: Complete` for any step that did
not throw, and `%d/%d steps completed successfully`) is replaced by the derived verdict and by
`%d/%d steps ran, %d verified`.

### F. Symptom B — a recovered mid-turn tool error is not a failed turn

The inline heuristic becomes `surfaceToolErrors`, whose predicate
`turnRecoveredFromToolErrors` asks the closing evidence in this order: an affirmative gate
failure is the tool error confirmed; a gate that ran and passed is direct evidence about the
final workspace and outranks anything that failed earlier; only when no gate ran does the
model's closing answer decide, because then it is the only signal there is.

### G. `build_state` / `test_state` are asserted from the gate

`recordBuildState` — the function the field's comment already promised — asserts
`build_state(/passing|/failing)` from `BuildCheck.Verdict()` and `test_state(/passing|/failing)`
from `TestCheck.Verdict()`, called from `assertTurnEvidence` before the `turn_evidence` fact so
`!build_state(/failing)` can exclude `turn_executed` on the same pass. Only affirmative
verdicts are asserted: a skipped, canceled or indeterminate gate asserts nothing, because
`/passing` would be the guess this seam deletes and `/failing` would fail every turn on a
machine without a Go toolchain. The facts are tracked in `perTurnBuildStateFacts` and retracted
by `cleanupPerTurnCoverageFacts`, so a red build cannot block every later turn in the session.

## Changes

| file | change |
|---|---|
| `internal/observation/subagent.go` | `Return` gains `Outcome`, `Stage`, `Acceptance`, `Untested`; new `Acceptance` type |
| `internal/session/observed_return.go` | `observedReturn` fills them; `ObservedTaskExecutor` gains `ExecuteObservedWithContext` |
| `internal/session/task_executor.go` | `JITExecutor.ExecuteObservedWithContext` |
| `internal/session/executor.go` | `surfaceToolErrors` replaces the empty-response heuristic; `recordBuildState` added and called from `assertTurnEvidence` |
| `internal/session/change_evidence.go` | `surfaceToolErrors` + `turnRecoveredFromToolErrors` |
| `internal/session/executor_tools.go` | `cleanupPerTurnCoverageFacts` retracts `perTurnBuildStateFacts` |
| `internal/types/shard.go` | `ShardResult` gains `Outcome`, `Stage` |
| `internal/core/shards/manager_spawn.go` | `recordResult` fills the verdict it observed |
| `cmd/nerd/cmd_spawn.go` | `spawnSystemShardAndWait` returns `types.ShardResult`; the heading states the verdict |
| `cmd/nerd/chat/delegation.go` | `spawnTaskWithContext` returns `observation.Return` via `ExecuteObservedWithContext` |
| `cmd/nerd/chat/process_continuation.go` | derivation rewritten; substring heuristics deleted; summary derived |
| `cmd/nerd/chat/model_types.go` | `continuationUnverified` added and made the zero value |
| `cmd/nerd/chat/model_update.go` | the unverified banner arm |
| `cmd/nerd/chat/process.go`, `process_dream.go`, `process_dream_delegation.go`, `process_knowledge.go` | callers read `.Output`; the dream multi-step renderer states the verdict |

No `.mg` file changed: the fact shapes are unchanged, which is why the Decls and the
continuation rules did not have to move. That the *derived* facts still feed the *shipped*
rules is proven through a real kernel, not asserted (see the corpus test below).

## Tests (fail-before / pass-after evidence)

New: `internal/session/verdict_from_evidence_test.go`,
`cmd/nerd/chat/verdict_from_evidence_test.go`. Modified to the new contract:
`cmd/nerd/chat/continuation_done_test.go` (its `completedZeroValue` case asserted the
behaviour this seam deletes — that a producer stating no outcome renders under the checkmark).

**Method (no stash — the stash stack is shared with the main checkout and the other
implementer's worktree).** The implementation was committed first (`188532da`). The
pre-change *rule* was then pasted back into each new function body, keeping the signature so
the new tests still compile, the tests were run, and the files were restored with
`git checkout HEAD -- <paths>`. This gives a behavioural failure rather than a compile
failure: the test discriminates the old rule from the new one.

### Fail-before (old rules restored)

```
--- FAIL: TestShardResultStatus_DerivedFromOutcomeNotProse
    /proseSaysTODOButOutcomeIsDone        shardResultStatus = "/code_generated", want "/complete"
    /proseSaysFIXMEButOutcomeIsDone       shardResultStatus = "/code_generated", want "/complete"
    /unverifiedIsNeverComplete            shardResultStatus = "/complete", want "/code_generated"
    /unverifiedWithNoChangeIsUnverified   shardResultStatus = "/code_generated", want "/unverified"
    /hollowOwesTheWorkToTheSameShard      shardResultStatus = "/code_generated", want "/incomplete"
    /failedOutcome                        shardResultStatus = "/code_generated", want "/failed"
    /errorBeatsEverything                 shardResultStatus = "/code_generated", want "/failed"
    /absentVerdictIsUnverifiedNotComplete shardResultStatus = "/code_generated", want "/unverified"

--- FAIL: TestPendingTestOwed_DerivedFromEvidenceNotVocabulary
    /noTestWordButTestsRanAndPassed       pendingTestOwed = true, want false
    /goSourceChangedAndNothingRanTheTests pendingTestOwed = false, want true
    /markdownOnlyTurnOwesNothing          pendingTestOwed = true, want false
    /testFileOnlyTurnOwesNothing          pendingTestOwed = true, want false
    /noChangeOwesNothing                  pendingTestOwed = true, want false

--- FAIL: TestContinuationSummary_NeverSaysSuccessfullyForUnverifiedStep
    an unverified step must never be summarized as successful, got "Completed 1 steps successfully."
    the summary must state the verdict, got "Completed 1 steps successfully."
    summary must name the evidence "build passed" / "tests did not run" / "no acceptance contract"
    an /unverified step must render as continuationUnverified, got 1

--- FAIL: TestContinuationSummary_HollowAndDone
    a hollow step must never be summarized as successful, got "Completed 2 steps successfully."
    a /hollow step must not render as completed, got 1

--- FAIL: TestInjectShardResultFacts_DerivesContinuationThroughRealKernel
    /unverifiedWriteWithNoTestRunOwesATesterStep  shard_result status = "/complete", want "/code_generated"
    /prosewithTODOOnAVerifiedTurnDerivesNothing   shard_result status = "/incomplete", want "/complete"
    /structuredFindingsOweAReview                 the corpus must derive a reviewer subtask from shard_result + pending_review

--- FAIL: TestUpdate_ContinuationDoneOutcome
    /unverified                expected "⚠️", got "✅ All 1 steps complete.\n\nRan 1 step(s); the work is not verified."
    /zeroValueRendersUnverified expected "⚠️", got "✅ All 3 steps complete.\n\n..."

--- FAIL: TestObservedReturn_CarriesKernelVerdict
    Outcome = "", want "/unverified" — the kernel's verdict must cross the boundary
    Stage = "", want "checks_passed"
    Acceptance must cross the boundary when the executor recorded one

--- FAIL: TestRecoveredToolError_IsNotAFailedTurn
    /buildFailedMidTurnFinalGatesPassedNoClosingProse  error=true (tool execution failed: run_build: ... exit status 1), want error=false
    /finalBuildFailedDespiteClosingProse               error=false (<nil>), want error=true
    /finalTestsFailed                                  error=false (<nil>), want error=true

--- FAIL: TestBuildFailure_ExcludesTurnExecuted
    expected exactly one build_state fact asserted from BuildCheck, got 0
--- FAIL: TestPassingBuildDerivesTurnExecuted
    expected one build_state fact, got []
--- FAIL: TestPerTurnBuildStateIsRetracted
    expected build_state asserted for this turn, got 0
```

Note which assertions fired in the `TestRecoveredToolError` block: the old rule failed the
recovered turn **and** passed the two turns whose closing gate was red. The new rule is not
merely more permissive — it moves the decision onto the evidence in both directions.

### Pass-after

```
ok  	codenerd/internal/session	3.004s     (all new session tests, -v, every subtest PASS)
ok  	codenerd/cmd/nerd/chat	3.304s     (all new chat tests + TestUpdate_ContinuationDoneOutcome)
```

The corpus test (`TestInjectShardResultFacts_DerivesContinuationThroughRealKernel`) runs on a
real `core.NewRealKernel()` with the shipped default corpus loaded — no stub — and asserts that
the derived `shard_result` / `pending_test` / `pending_review` rows still drive
`has_pending_subtask` and `should_auto_continue` through the unedited rules, and that the words
"TODO" and "issue" no longer create follow-up work.

## Full test run

```
go build ./...                                                          clean
go vet ./internal/session/ ./internal/observation/ ./cmd/nerd/chat/ \
       ./internal/core/... ./cmd/nerd/ ./internal/types/                clean

go test ./internal/session/ ./internal/observation/ ./cmd/nerd/chat/ \
        ./internal/core/... ./cmd/nerd/
  ok codenerd/internal/session            ok codenerd/internal/observation
  ok codenerd/cmd/nerd/chat  55.3s        ok codenerd/internal/core  262.6s
  ok codenerd/internal/core/defaults      ok codenerd/internal/core/defaults/policy
  ok codenerd/internal/core/shards        ok codenerd/cmd/nerd  62.0s

go test ./...                            87 packages ok, 0 failing
```

The first full run showed three failures in `internal/perception`
(`TestClaudeCLI_SchemaMode`, `TestCodexCLIClient_RunHealthProbe_Success`,
`TestCodexCLIClient_RunHealthProbe_SkillMissingAfterSuccessfulExec`) — all three are
external-CLI probes with 5s/30s timeouts that expire when the whole suite runs in parallel on a
loaded box. Checked rather than assumed: `git log a3657ab2..dogfood/c2-closure -- internal/perception`
is empty (no base change), the three tests pass `-count=3` in isolation on this branch, and the
second full `go test ./...` was clean.

## Open

1. **The base branch moved during this work** — `dogfood/c2-closure` advanced from `a3657ab2`
   (what this branch is built on) to `a287131f`. One of those commits, `aeefc45e` *"fix(chat):
   the shard receives the user's words with the target, on every verb and both paths"*, edits
   `cmd/nerd/chat/delegation.go`, which this change also edits. Whoever integrates should expect
   a conflict in that one file; nothing else overlaps. Not merged here, per the brief.
2. **`ResultToFacts` still reads `err == nil` as `shard_success`**
   (`internal/core/shards/manager_spawn.go:669-673`). It is not a status derived from *text*, so
   it is outside this seam's literal rule, but it is the same class of claim: a shard that
   returned without erroring is recorded as a success with no evidence behind it. The fact is
   consumed by the chat's cross-turn context.
3. **The chat's own `ShardResult`** (`cmd/nerd/chat/model_types.go`) still carries findings
   extracted from prose, and `ShardResultPayload` does not carry the typed outcome. Nothing
   derives a *status* from either — they feed the blackboard that formats the next shard's task
   — so widening them now would add a field with no consumer. It becomes worth doing when a
   consumer needs the prior step's verdict.
4. **`observation.ReportedTests(prior.RawOutput)`** (`cmd/nerd/chat/delegation.go:180`) does read
   a test verdict out of prose, deliberately, and labels it `SourceReported` rather than
   `SourceObserved`. Left as is: the distinction is the design, and the label is what keeps a
   claim from being read as evidence.
