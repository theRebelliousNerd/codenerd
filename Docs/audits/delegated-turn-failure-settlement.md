---
doc-class: deep-dive
subsystem: session
implementation-status: planned
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Failed delegated turns must retain what they already did

**Decision:** settle an admitted execution exactly once on every terminal path, including a hard error from the working loop. Preserve its measured result and primary failure together; use that same settlement for observation delivery, turn-owned fact cleanup, acceptance disposition, accounting and learning. Do not equate an unsuccessful model continuation with an empty execution.

**Status and source freeze:** source-inspected proposal on main at [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871), re-resolved on 2026-10-28 at approximately 08:38 UTC. The feature branch was rechecked at approximately 08:43 UTC at [`f37b8556a491e45cb7467f40160ad61ac1641fe2`](https://github.com/theRebelliousNerd/codenerd/commit/f37b8556a491e45cb7467f40160ad61ac1641fe2), not its earlier 08:15 revision. No runtime, test, model request, benchmark, failure injection, repository edit or implementation was performed. The examples are source-derived witnesses to validate, not reproduced production incidents. This report is local research only.

## 1. The user journey and the exact boundary

An ordinary TUI request takes `RouteDelegate`, runs a real session executor and returns an observed outcome. The normal path is not the residual `RouteNone` articulation helper. Its problem here is narrower: after useful tool work, a provider, continuation-policy, cancellation or post-edit failure can prevent the already-measured result from reaching the delegation owner.

The root [North Star](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#north-star) requires logic-owned completion and useful grounded feedback. [Session guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/agents.md) requires explicit lifecycle ownership and shared post-edit proof boundaries. The required capability is therefore not merely “the caller gets an error.” It is: **the caller gets the error and the honest partial state of the work, while the executive retires precisely that turn's obligations and learning receives the same outcome.**

| Boundary | Current source | Contract already present |
|---|---|---|
| TUI delegation | [process.go:450–542](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L450-L542) | Build task/session context; require a verifier for delegated mutations; normal read/query delegation uses observed task execution. |
| Observed adapter | [delegation.go:50–74](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation.go#L50-L74) | Prefer `ObservedTaskExecutor`; absence of observed evidence is not success. |
| Working executor | [task_executor.go:315–409](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L315-L409) | Preset intent, isolated clone or subagent, real `ProcessWithIntent`, structured observed return. |
| Native/Piggyback work | [executor_tools.go:269–391](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L269-L391) | Execute batch, accumulate measured result, consult continuation policy, feed tool results back and perform terminal verification. |
| Result settlement | [executor.go:1299–1373](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1299-L1373) | The ordinary return path closes acceptance, captures the kernel verdict, appends evidence summary, updates history, learning and turn records. The hard-error branch leaves before these stages. |

This is not a proposal to replace the working loop, restore arbitrary tool-call ceilings, bypass permission, or add another planner.

## 2. What is already repaired—and must not be reported as missing

- Worker retirement and cancellation latching now belong to the task owner. [SubAgent.Run:276–322](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/subagent.go#L276-L322) checks a latched stop request and reaches terminal publication. The [task-terminal-ownership audit](https://github.com/theRebelliousNerd/codenerd/blob/codenerd-research-docs/Docs/audits/task-terminal-ownership.md) concerns that lifecycle, not whether `ProcessWithIntent` supplied a populated result to publish.
- Current tool accounting calls `settleToolEffects` before returning a handler/validator error, then archives the working result ([executor_tools.go:1000–1045](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L1000-L1045)). [tool_effects.go:226–290](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects.go#L226-L290) records changed paths, write sequencing and receipts even when a failed call changed state. Thus the older [tool-effect receipt proposal](https://github.com/theRebelliousNerd/codenerd/blob/codenerd-research-docs/Docs/audits/tool-effect-receipts-and-validation-completeness.md) is not evidence that this lower boundary is still wholly unwired. The new finding is that a later hard loop error discards the enclosing result that holds those repairs.
- Soft tool failures that return normally from the loop do reach closure. [TestErroredTurnStillClosesAndRetractsItsFacts](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/errored_turn_closes_test.go#L14-L125) covers that shape: a tool fails, then `surfaceToolErrors` records the surviving failure. It does not inject a hard provider/continuation error after the tool batch. The test was read, not run.
- The residual TUI evidence-count/read-delivery defects are already in [tui-evidence-turn-ownership](https://github.com/theRebelliousNerd/codenerd/blob/codenerd-research-docs/Docs/audits/tui-evidence-turn-ownership.md). This report does not rely on that route.
- Session already proposes a joined execution receipt in [TODO: session-turn-execution-receipt-v1](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/session/TODO.md#L214-L252). Reuse its identity/accounting work; this diagnosis does not require a second receipt database.

### Other branch code checked, not just main

The repository branch listing at approximately 08:38 UTC contained `main`, `feat/conversation-brief`, and `codenerd-research-docs`. Main and the feature branch were freshly resolved and fetched; the research-branch row records its earlier pinned inspection. Code was checked at immutable commits, rather than inferred from branch names:

| Branch and pin | Checked code and conclusion |
|---|---|
| `feat/conversation-brief` at [`f37b8556a491e45cb7467f40160ad61ac1641fe2`](https://github.com/theRebelliousNerd/codenerd/commit/f37b8556a491e45cb7467f40160ad61ac1641fe2), committed 08:42:24 UTC | `executor.go`, `task_executor.go`, `subagent.go`, `observed_return.go`, `turn_writes.go`, `verifier.go` and chat `delegation.go` have identical Git blob SHAs to main. `executor_tools.go` differs only at an approval-predicate constant in a cleanup log, outside these findings. The [hard-error return and final settlement](https://github.com/theRebelliousNerd/codenerd/blob/f37b8556a491e45cb7467f40160ad61ac1641fe2/internal/session/executor.go#L1299-L1373) and [success feedback defers](https://github.com/theRebelliousNerd/codenerd/blob/f37b8556a491e45cb7467f40160ad61ac1641fe2/internal/session/executor.go#L1120-L1165) remain. F1–F3 are not already fixed there. |
| `codenerd-research-docs` at [`b6a72db2b8aba54b69e47cb2f6dace9fc7145a0b`](https://github.com/theRebelliousNerd/codenerd/commit/b6a72db2b8aba54b69e47cb2f6dace9fc7145a0b) | The code differs from current main in places, so it was not assumed to be a newer implementation. It still has [the nil hard-error return](https://github.com/theRebelliousNerd/codenerd/blob/b6a72db2b8aba54b69e47cb2f6dace9fc7145a0b/internal/session/executor.go#L1280-L1283), [the local-result success audit](https://github.com/theRebelliousNerd/codenerd/blob/b6a72db2b8aba54b69e47cb2f6dace9fc7145a0b/internal/session/executor.go#L1098-L1106), and cleanup deferred inside `checkHollowSuccess` at executor_tools.go:2163. |

The feature branch does add material conversational work: saved-intent approval, conversation-brief propagation, hold/approve lanes and background read-only delegation ([current process.go:163–218](https://github.com/theRebelliousNerd/codenerd/blob/f37b8556a491e45cb7467f40160ad61ac1641fe2/cmd/nerd/chat/process.go#L163-L218), [483–500](https://github.com/theRebelliousNerd/codenerd/blob/f37b8556a491e45cb7467f40160ad61ac1641fe2/cmd/nerd/chat/process.go#L483-L500)). Since the earlier inspection it advanced by five commits, adding Jev turn judgments, supporting configuration changes, plain-English retrieval with optional Jev relevance judging, review-finding checks and self-diagnosis triage ([comparison](https://github.com/theRebelliousNerd/codenerd/compare/31e27a9b1e1f61572d51736ae713bfb11e298716...f37b8556a491e45cb7467f40160ad61ac1641fe2)). That progress should be credited and coordinated with; it does not change the session settlement boundary. Its [TaskRequest adapter](https://github.com/theRebelliousNerd/codenerd/blob/f37b8556a491e45cb7467f40160ad61ac1641fe2/cmd/nerd/chat/delegation_routing.go#L325-L332) still omits structured Target/Constraint, so the appendix's narrower follow-on brief seam remains at this pin. The foreground feature paths also still return explicit errors for [verified execution failure](https://github.com/theRebelliousNerd/codenerd/blob/f37b8556a491e45cb7467f40160ad61ac1641fe2/cmd/nerd/chat/process.go#L556-L566) and [ordinary delegation failure](https://github.com/theRebelliousNerd/codenerd/blob/f37b8556a491e45cb7467f40160ad61ac1641fe2/cmd/nerd/chat/process.go#L637-L646).

These conclusions are bounded to the fetched commits and paths, not unpushed work, deleted branches or future changes. Recheck branch heads before implementation. Fresh source copies for the main/feature comparison are retained locally in `delegation_recheck_20261028/main/` and `delegation_recheck_20261028/f37b855/` (with the intermediate `5846386` feature snapshot under `feature/`); they are research inputs, not a checked-out or modified repository.

## 3. Finding F1: a hard loop error erases a measured partial result

`ProcessWithIntent` allocates `result := &ExecutionResult{}` at [executor.go:1104](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1104). The same pointer is handed into `runToolLoop`, where tool execution populates counters, writes, preimages, receipts and checks. The caller then does this at [lines 1302–1305](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1302-L1305):

```go
llmResponse, toolErrs, err := e.runToolLoop(..., result)
if err != nil {
    return nil, wrapToolLoopError(err)
}
```

The loop deliberately returns its current response and accumulated tool errors on failures after execution. Concrete exits include cancellation at a boundary, continuation policy failure/stop, a failed tool-result provider follow-up, nil follow-up response, forced-final failure, and post-edit verification errors ([executor_tools.go:232–295,315–391,421–431](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L232-L431)). The outer method drops the response, tool-error list and result pointer.

**Minimal source-derived witness:** an authorized tool call writes one harmless fixture file and records its effect; the next tool-result provider call returns a transport error. This is not a validator-before-effect case, an unauthorized call or a model-quality hypothesis. `WrittenPaths`, preimages, write counts and tool receipts existed, but the caller receives `nil,error`.

The loss continues through both task topologies:

- Inline tasks invoke `observedReturn(..., result)` and `withTurnWrites(..., result)` before surfacing the error ([task_executor.go:392–403](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L392-L403)). Both helpers stop immediately for nil results: [observed_return.go:47–84](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/observed_return.go#L47-L84), [turn_writes.go:19–31](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/turn_writes.go#L19-L31). The adapter adds the error string but cannot recover the measured changes or checks.
- Subagents call the same method, build their observation from the same nil result, and return an empty string on the hard error ([subagent.go:337–364](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/subagent.go#L337-L364)). Correct terminal-worker publication therefore publishes an impoverished outcome.

**Important limit:** this does not establish a false successful TUI answer. The TUI returns `errorMsg` for failed delegation ([process.go:591–593](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L591-L593)). The verifier also immediately returns an execution error and does not enter its judge-driven retry branch ([verifier.go:262–265](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L262-L265)). The demonstrated defect is **failure with lost partial-state evidence**, not silent top-level success or proved automatic duplicate mutation.

## 4. Finding F2: settlement feedback disagrees with the returned failure

The outer method's audit defer reads its local pointer rather than its unnamed returned values:

```go
success := result != nil && result.Error == nil
logging.Audit().TurnEnd(..., success)
```

See [executor.go:1120–1128](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1120-L1128). Returning `nil,error` does not assign nil to that local pointer or assign the error to `result.Error`. The separate prompt co-use defer likewise selects `OutcomeSuccess` unless `result == nil || result.Error != nil` ([1158–1165](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1158-L1165)). For the clean write followed by a provider transport error witness, the local result can still have `Error == nil`: caller failure, success audit and success prompt-co-use feedback disagree.

Scope this exactly. A loop path that itself sets `result.Error` will not satisfy that success predicate. Nor is there evidence here that every atom gets a co-use update; that depends on whether the compiler recorded selections for the turn. The source establishes the wrong settlement value where such selections exist.

The ordinary persistence/feedback path is also skipped. [persistTurn:2292–2345](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L2292-L2345) writes the turn cost/outcome, context feedback and turn record **before** checking whether a persister exists; the comments explicitly preserve these for delegated clones. A failed loop exits before invoking it. This does not prove billed provider usage disappeared—usage tracking is separate—but the task outcome denominator and execution feedback are omitted at this seam.

## 5. Finding F3: task retirement is not turn-fact retirement

`checkHollowSuccess` owns `defer e.cleanupTurnFacts()` ([executor_tools.go:2164–2168](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L2164-L2168)). The hard-error return never calls it. The cleanup explicitly retracts only recorded facts and clears executor-local creation records ([2270–2287](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L2270-L2287)).

This is not only a hypothetical fact generated by final closure. Before the model call, `prepareTurnCatalog` asserts the turn verb and an optional declared check ([turn_check.go:47–61](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/turn_check.go#L47-L61)); during changed writes, `settleToolEffects` asserts turn-owned effect/validation facts ([tool_effects.go:303–329](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects.go#L303-L329)). `assertTurnFact` records their identities in `e.turnFacts` ([executor.go:2576–2587](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L2576-L2587)).

Task cleanup does correctly retire working scopes, and preset intent/retrieval have independent defers. But [RetireWorkingScopes:262–282](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/working_context.go#L262-L282) retires observation archives, not these kernel facts. An isolated clone may disappear with the only recorded cleanup list while the shared kernel retains its turn-keyed rows. Unique turn keys mitigate false association with later turns; do **not** claim every leaked row poisons the next task. Retention, stale executive state and incomplete settlement remain the supported concerns.

An acceptance contract likewise has no terminal unverified report through this return. `closeAcceptanceEvidence` already provides that honest outcome and persistence ([change_evidence.go:280–301](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/change_evidence.go#L280-L301)); it is simply not reached.

## 6. Proposed contract: one terminal result, not a second execution

Names below are design notation, not new APIs already present.

1. **Admit one turn outcome owner.** Once the execution result exists, every terminal path returns or retains that same result. An error is an additional field/state, not permission to replace the result with nil. Distinguish pre-admission rejection, execution failure, canceled work and unsettled evidence.
2. **Capture failure before every consumer settles.** Preserve the primary typed error, current response only as explicitly incomplete diagnostic text, tool errors, measured receipts, writes and already-run checks. Never parse unfinished model prose into a completion claim or apply its control packet merely to obtain a summary.
3. **Separate closure from more work.** Terminal closure must not automatically invoke more model calls, repeat a pending tool batch, repair files or extend a canceled task. On a hard stop, mark unperformed checks and acceptance as unverified/unknown, preserving existing evidence. Go reports what happened; Mangle derives the result and unsatisfied obligations.
4. **Have one once-only finalization boundary.** It owns evidence disposition, verdict capture where possible, duration, observation handoff, accounting/learning, and exact per-turn retraction. Set final outcome before the audit/co-use consumers run. Define error precedence: the primary execution failure remains identifiable, while cleanup/persistence failures are joined or recorded separately rather than replacing it.
5. **Make cleanup unconditional but narrow.** Install exact turn-fact retirement before the first owned assertion. Preserve independent intent/retrieval/archive cleanup. Never solve this with predicate-wide retraction, which could delete a sibling's facts. Closure may read needed facts before retiring them; cleanup must still happen if verdict derivation or persistence fails.
6. **Keep canceled closure bounded and non-effectful.** In-memory settlement and exact cleanup should not require a canceled provider context to become live again. If persistence needs its own cleanup context, give it an explicit bounded lifecycle and no new tool/model authority. Distinguish failed persistence from a completed durable record.
7. **Deliver the same outcome to every consumer.** `ObservedTaskExecutor`, SubAgent terminal publication, verifier and TUI should retain partial effects on error. The TUI should show a failed/partial result with changed resources and check status, not a success header; it must not discard the structured outcome merely because the error is non-nil. Maintain the verifier's current no-blind-retry behavior for execution failures.
8. **Reuse existing receipts and stores.** Preserve `ExecutionResult`, `ToolReceipts`, `observation.Return`, acceptance reports and current turn accounting. A minimal in-memory/result closure can preserve the executor's measured evidence without designing crash-proof exactly-once execution. Durable restart recovery is a separate requirement, not a claim implied by a defer.

**Projection boundary:** the existing [`observedReturn` implementation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/observed_return.go#L47-L138) projects changed paths, checks, acceptance and other measured outcome fields, but does **not** project `ExecutionResult.ToolReceipts`. Returning the populated result on error would restore access to those existing projected fields and to the receipts inside the executor result; it would not by itself deliver per-call receipts to a parent. Receipt-through-parent delivery in this proposal therefore requires an explicit bounded projection/transport change, with identity, validation state and retention/redaction semantics preserved. That is a distinct acceptance step from the minimal non-nil-result repair.

Spec-first implementation should reconcile session lifecycle/failure/observability, CLI error projection and the verification adapter. Add model-facing explanations through JIT atoms where needed; deterministic error summaries and typed outcomes do not require another model call.

## 7. Proposed validation, not executed

| Fixture | Required evidence |
|---|---|
| Successful authorized read, then tool-result provider error | Non-nil partial result; failed terminal state; captured observation retained or explicit unavailable disposition; no fabricated final answer. |
| Successful authorized write, then provider error | Real bytes, WrittenPaths and preimage survive into observed return; receipt remains in the execution result and crosses to the parent only through the explicitly added projection/transport; no second write; checks not falsely passed. |
| Changed write whose post-validator rejects, then provider error | The already-fixed lower effect receipt survives the outer failure as well. |
| Cancellation after one completed call and before the next | Completed effect retained, pending call classified not executed/unknown as appropriate; no forced-final replay. |
| Working policy stop/error after a batch | Correct primary reason; failure/partial settlement rather than success feedback; no lost batch. |
| Post-edit mechanical verification failure | Build/test detail and changed paths preserved; typed verification sentinel preserved. |
| First provider call fails before tools | Zero effects but populated failed turn; no success audit/co-use settlement. |
| Soft tool failure followed by normal return | Preserve the existing errored-turn closure behavior. |
| Inline clone and isolated subagent | Equivalent structured error outcome; worker completion does not erase partial evidence. |
| Two concurrent turns, one fails | Only the failed turn's tracked facts are retired; sibling/world facts remain. |
| Acceptance/persistence/cleanup failure | Explicit unverified/delivery-failed state; primary failure preserved; no replay to repair logging. |
| Normal TUI RouteDelegate, both verifier and non-verifier lanes | Correct visible failure and partial result; no successful header or judge-driven retry on execution error. |

Use deterministic barriers/stubbed provider failures, production-shaped kernel/CodeDOM configuration and the normal TUI→TaskExecutor path. Assert effect bytes and every consumer's outcome, not just the final error string. Inspect generated tool-result pairing so a closure refactor cannot replay already-executed calls. The current soft-error test remains a control, not a substitute for these witnesses.

## 8. Full cost and adoption criteria

Compare the pinned baseline, a minimal result-preservation/exact-cleanup repair, and a unified terminal-outcome settlement. Keep model, provider, permissions, repository snapshot, CodeDOM/world-model configuration, context budget and injected failure position matched. If using the feature branch, keep Jev/brief/retrieval settings identical between treatment and control: its new upstream model calls are a separate intervention. Add a normal-success control and a hard-failure cohort; do not compare only completed tasks or count failure records as an increase in task failures.

The unit of analysis is the original user request through a terminal disposition and, when recovery is evaluated, its explicitly authorized recovery attempt. One failed executor call is not the whole cost of that request. Record a task/attempt identity before execution so this experiment does not rely on the very `persistTurn` rows the baseline skips.

| Cost or outcome | Required measurement |
|---|---|
| Model spend | Every provider request across perception, optional judges, JIT-related model work, worker, verification, repair and recovery; input/output tokens plus reasoning/cache fields when the provider reports them; failed-call usage and unavailable usage explicitly distinguished from zero. Keep per-purpose and total cost without double-counting nested spans. |
| Repeated work | Repeated source reads, repeated tool investigations, duplicate effect attempts and recovery model turns after the original failure. A new investigation is a cost even when it safely avoids repeating the write. |
| Latency | Initial request to visible failure/partial result, final failure to durable settlement, and request to recovered verified outcome; report medians and tails separately. Queue delay and provider latency are not closure CPU. |
| Host overhead | Closure CPU, exact fact-retraction counts/time, kernel derivation time, result-copy allocations, preimage/final-state IO, persistence bytes and latency. Include cancellation paths and a large write set, not only one-file fixtures. |
| Retention and privacy | Peak and post-terminal heap, live turn-fact count/bytes, retained preimages/receipts and failure-record bytes under the existing retention policy; raw source or secret retention is not an acceptable substitute for a redacted receipt. |
| Human recovery | Time to identify changed resources, decide what remains unverified, and resume or stop safely; measure it rather than claiming every missing result causes a duplicate edit. |
| Correctness | Retained measured effects, primary-error preservation, agreement among caller/kernel/audit/co-use/accounting, exact sibling-safe cleanup, visible failure, no fabricated checks, and no effect replay caused by closure. |

For economic comparison, report the resource vector first. If dollar conversion is wanted, price tokens using the actual provider/model and billing categories for the experiment; add measured infrastructure and, separately, a stated valuation of human recovery time. In symbols, `incremental total cost = added closure and retention cost + added reporting/recovery cost − avoided repeated work and recovery cost`. This is an accounting definition, not a measured saving. Do not add time, bytes and tokens without explicit prices, or double-count reasoning tokens already included in a billed output category.

Use paired failure injections at the same completed call boundary, across the Section 7 matrix, before any probabilistic model comparison. Capture the real fixture bytes and a unique call identity independently of model prose; count the same effect exactly once at the handler boundary. For live-model recovery comparisons, hold the request and permitted scope fixed, randomize treatment ordering, report attempts and uncertainty, and never authorize extra mutation merely to make a benchmark finish. Compare the read-only/no-prior-result branch and every contextual formatter branch separately so a happy-path average cannot conceal dropped constraints.

Adoption requires all deterministic state-agreement, no-replay, no-sibling-retraction and authority controls to pass, with normal-success semantics preserved. The owner should select a latency/retention budget before measuring; this report supplies no invented acceptable percentage or benchmark result. A minimal repair need not wait for durable crash recovery, but it also must not be described as crash-proof exactly-once execution. If durable publication fails, expose that separately while preserving the primary failure and the already-measured effects.

The repair may retain more honest diagnostic state and write more failure records; that cost is real. Bound/redact the output and reuse current observation retention. Success means accurate terminal-state agreement and recoverable work—not an increased success rate caused by relabeling failed tasks, omitting failed attempts from the denominator, or weakening evidence gates.

Suggested implementation slices:

- **DELEGATE-SETTLE-01:** preserve populated results on hard loop errors; return typed primary failure unchanged.
- **DELEGATE-SETTLE-02:** once-only terminal evidence/fact/accounting settlement; audit, co-use and turn records agree.
- **DELEGATE-SETTLE-03:** deliver partial state through both TaskExecutor topologies and TUI error rendering; add explicit bounded receipt projection/transport if parents are to receive per-call receipts.
- **DELEGATE-SETTLE-04:** matched normal-entry validation establishes no replay, no sibling cleanup and unchanged authority.

## Appendix: an independent follow-on request fidelity seam

This is adjacent to the same user journey but is **not the cause of F1–F3** and should have a separate acceptance slice. The original task-template concern is already documented in the journey corpus and TUI evidence audit; do not present it as new.

There is a narrower current branch bug: `formatShardTaskWithContext` first constructs `baseTask` with the current constraint, then its `/fix` after reviewer/tester, `/refactor` after reviewer, `/test` after coder, and `/debug` after tester/reviewer branches return newly formatted strings instead of that base ([delegation.go:85–142](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation.go#L85-L142)). Those early returns do not append the current constraint. `shardTypeToTaskRequest` then sets only `IntentVerb` and `Task`, not the available `Target`/`Constraint` fields ([delegation_routing.go:274–280](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation_routing.go#L274-L280)). The verifier's TaskRequest construction has the same two fields ([verifier.go:262](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L262)).

Thus a follow-on “fix these findings, but preserve the public API” can reach the worker with prior findings substituted for the current constraint at this explicit task boundary. The current [formatter regression tests](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation_fix_brief_test.go#L8-L50) exercise `formatShardTask`, not these prior-context branches. This is **not proof that no other shared context contains the constraint, or that an unauthorized edit occurs**. It establishes that the designated delegated brief and structured fields do not preserve it reliably.

Repair by keeping the original current objective and constraint separate from appended prior evidence, carrying resolved Target/Constraint through the existing TaskRequest fields, and testing all prior-result branches. Preserve the single authoritative persona/verb mapping rather than adding another table. Coordinate with [perception-restriction-continuity](https://github.com/theRebelliousNerd/codenerd/blob/codenerd-research-docs/Docs/audits/perception-restriction-continuity.md): preserving prose is necessary, but still does not implement a typed enforceable restriction at the action gate.

**Do not stop that repair at the TUI adapter.** The inline topology explicitly assigns `req.Constraint` to its preset ([task_executor.go:384–392](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L384-L392)). The isolated topology carries `req.TaskText()` in the spawn task but only `IntentVerb` and `IntentTarget` as structured intent ([452–466](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L452-L466)); `SubAgent.execute` reconstructs its preset without a constraint field ([subagent.go:327–339](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/subagent.go#L327-L339)), and the shared preset constructor initializes `Constraint` to empty ([task_executor.go:220–233](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L220-L233)). Correcting TaskRequest population would therefore restore the task prose but would not by itself establish structured-constraint parity across both topologies. Verify the actual worker preset, the JIT input and the final task text—not merely the formatter output. This is a bounded propagation finding, not evidence that the model cannot see the constraint anywhere else.

Proposed fidelity controls: each contextual branch with a nonempty current must/must-not constraint; no prior result; irrelevant prior findings; generic target resolved from findings; a constraint already present in TaskText; inline and isolated execution; verifier and non-verifier entry. Assert that current constraints survive byte-for-byte or by an explicitly specified lossless structured representation, prior evidence remains labeled as evidence, and no older finding silently overrides the current requested scope. A preserved sentence alone is not proof of enforceable permission.
