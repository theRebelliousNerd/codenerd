# S1 — The verdict is derived from evidence, never guessed from prose

Seam S1 of the codeNERD unblocking program.
North-star capability `cap_derived_verdict` ("Success and failure are derived from evidence
facts - builds, tests, gates - never guessed from the prose of a result"); requirement
`req_verdict_from_evidence` (`.nerd/northstar.json`).

## Status

- last updated: 2026-09-18 (in progress)
- done:
  - worktree fast-forwarded to base `dogfood/c2-closure` (a3657ab2)
  - trace of symptoms A, B, C, D complete; symptom B producer named from the live log
  - design settled
- open:
  - implement A–G
  - fail-before / pass-after evidence
  - full test run

## Trace

### Symptom B — named producer

The chain that produced
`shard execution failed: tool execution failed: run_build: modular tool execution failed: exit status 1`:

- `cmd/nerd/cmd_direct_actions.go:414` wraps whatever `cortex.SpawnTaskWithTarget` returned.
- `internal/system/factory.go:428` → `TaskExecutor.Execute`.
- `internal/session/task_executor.go:344-358` `executeObserved`: the `err != nil` branch wraps
  with `execution failed: %w`; the **`result.Error != nil` branch returns `result.Error`
  unwrapped**. The observed string has no `execution failed:` segment, so the error came
  from `result.Error`, not from a returned error.
- The only assignment to `result.Error` with that exact format is
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
`result.Response`, and `appendEvidenceReport` is called at `executor.go:1103` — **ten lines
after** the emptiness test. So the model returned no final text, the heuristic fired on the
empty response, and only afterwards did the runtime glue its own evidence sentence onto it.
That is why the CLI printed a non-empty "Partial result (failed)" next to a non-zero exit.

Live log confirming the sequence
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

Every closing gate passed. The turn was still `/failed`, because a tool error that happened
eight minutes earlier and was fully recovered from decided the verdict. **The mid-turn tool
error is not evidence about the final state of the workspace; the closing gate is.**

### Symptom A/C — status guessed from prose

`cmd/nerd/chat/process_continuation.go:167-222` `injectShardResultFacts` sets `shard_result`'s
Status by substring: `"TODO"`/`"FIXME"` → `/incomplete`; a coder result lacking the word
`"test"` → `/code_generated` plus an unconditional `pending_test`; a reviewer result containing
`"issue"` → `pending_review`. `process_continuation.go:155-161` then prints
`"Completed %d steps successfully."` whenever the kernel derives no further continuation —
the string is a function of *the absence of a next step*, not of the verdict of this one.

### Symptom D — the structured verdict exists and is dropped

`ExecutionResult` (`internal/session/executor.go:733-816`) carries `TurnOutcome`
(`/done|/hollow|/failed|/unverified`, set by `captureTurnOutcome`,
`internal/session/executor_memory.go:128`), `ChangeStage`, `BuildCheck`/`TestCheck`,
`Acceptance`, `WrittenPaths`, `UntestedPaths`. `observedReturn`
(`internal/session/observed_return.go:39-105`) carries Output/Failure/Changed/Build/Tests/
Findings/Notes across — and drops `TurnOutcome`, `ChangeStage`, `Acceptance` and
`UntestedPaths`. `types.ShardResult` (`internal/types/shard.go:99-105`) is prose plus an error.

### Wiring gap — `build_state` is declared, never asserted (coordinator addendum, confirmed)

- `internal/session/executor.go:216-219` documents `perTurnBuildStateFacts` as "asserted this
  turn via `recordBuildState`".
- `grep -rn recordBuildState` over the whole tree: **the only hit is that comment.** The field
  is written nowhere and read nowhere — not even by the cleanup that claims to retract it.
- The only `build_state` assertions in the repo are in `internal/session/turn_done_test.go:39`.
- Therefore the `!build_state(/failing)` conjunct of
  `turn_executed(Verb)` (`internal/core/defaults/policy/coder_safety.mg:114`) excludes nothing
  in production: a turn whose build failed can still be `turn_executed`, and with an acceptance
  contract, `turn_done`.

`test_state/1` (`internal/core/defaults/schemas_execution.mg:16`) has the same shape and is
read by `commit_gate.mg`, `tdd_loop.mg` and `context_compilation.mg`; the executor never
asserts it from `TestCheck` either.

## Design

**Decision (required by the brief): the chat delegation path consumes the typed outcome
through `ExecuteObserved`, not through a widened `types.ShardResult`.**

Reason: `observation.Return` is built by `observedReturn` directly from `ExecutionResult`,
which is the object the kernel's verdict is actually recorded on. `types.ShardResult` is
filled at `internal/core/shards/manager_spawn.go:520` from `agent.Execute(ctx, task) (string,
error)` — a boundary where *no* typed verdict exists on either side, because `types.ShardAgent`
(`internal/types/interfaces.go:170`) returns a bare string. Routing the chat through
`ShardResult` would therefore have required inventing the verdict at exactly the boundary that
has no evidence, i.e. moving the guess rather than removing it. `ShardResult` still gains the
typed fields (requirement 1) and fills them with what that path honestly observed — `/failed`
on error, `/unverified` otherwise — so no consumer can read "no error" as "verified".

### A. The verdict crosses the observation boundary

`observation.Return` gains `Outcome`, `Stage`, `Acceptance *Acceptance` and `Untested`.
`observedReturn` fills all four from `ExecutionResult`. `Untested` is a field rather than the
existing prose Note because requirement 2 derives `pending_test` from it.

### B. Chat delegation calls ExecuteObserved

`ObservedTaskExecutor` gains `ExecuteObservedWithContext(ctx, req, sessionCtx, priority)`,
mirroring `TaskExecutor.ExecuteWithContext`; both observed entry points delegate to the single
`executeObserved`. `Model.spawnTaskWithContext` and `Model.spawnTask` return
`observation.Return` instead of `string`; callers that want the prose read `.Output`.

### C. `types.ShardResult` carries the typed outcome

New `Outcome types.MangleAtom` and `Stage string`, filled at every constructor.

### D. `injectShardResultFacts` derives Status

The `shard_result/5` fact shape does not change (so the Decl at
`internal/core/defaults/schemas_misc.mg:188` and every rule in
`internal/core/defaults/policy/codedom_continuation.mg` stay as they are); only the Status
derivation changes:

| evidence | Status |
|---|---|
| `err != nil` or `Outcome == /failed` | `/failed` |
| `Outcome == /hollow` | `/incomplete` (same shard continues) |
| `Outcome == /done` | `/complete` |
| `Outcome == /unverified` and the turn changed artifacts | `/code_generated` |
| `Outcome == /unverified`, nothing changed | `/unverified` |
| no typed outcome at all | `/unverified` — never `/complete` |

`pending_test` is asserted when the turn changed Go artifacts **and** (tests did not run **or**
`Untested` is non-empty). `pending_review` is asserted from structured `Findings`, not from the
substring "issue". Every substring test is deleted.

### E. The continuation summary states the derived verdict

`continuationSummary(steps, ret)` renders from the outcome and names the evidence
("build passed, tests passed", "build passed, tests did not run", ...). `/unverified` and
`/hollow` never render the word "successfully". The unconditional
`"Completed %d steps successfully."` is deleted.

### F. Symptom B — a recovered mid-turn tool error is not a failed turn

`executor.go:1093` stops testing the response for emptiness and tests the evidence:
tool errors fail the turn only when the turn shows no closing evidence — i.e. neither a final
response nor a passing closing gate. A turn whose post-edit build and tests passed has answered
the question the tool error raised.

### G. `build_state` / `test_state` are asserted from the gate (wiring gap)

`recordBuildState` is written — the function the field's comment already promised — asserting
`build_state(/passing|/failing)` from `BuildCheck.Verdict()` and `test_state(/passing|/failing)`
from `TestCheck.Verdict()` (only for verdicts that are actually pass/fail; skipped and
indeterminate assert nothing, because "not proven" is not "failing"). Facts are tracked in
`perTurnBuildStateFacts` and retracted with the other per-turn facts, so a red build cannot
block later turns forever.

## Changes

(in progress)

## Tests (fail-before / pass-after evidence)

(pending)

## Full test run

(pending)

## Open

(pending)
