---
doc-class: deep-dive
subsystem: cli-core
implementation-status: planned
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
feature-compared-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
verification-method: static-source-inspection
supersedes: []
---

# Approved dream plans need one execution ledger

## Decision

Keep the full approved plan owned until every admitted step has an explicit disposition. A worker returning without a newly derived follow-up obligation is not evidence that the rest of its approved plan has run. Likewise, confirming a displayed next step must dispatch that exact step, not the head of an unrelated backlog.

The current TUI mixes two meanings of `pendingSubtasks`: the entire approved dream plan, including its already-dispatched first step, and a queue of evidence-derived successors awaiting confirmation. The worker completion path consults only kernel-derived evidence obligations. When it finds none, the terminal UI handler clears the whole queue. When it finds a successor and pauses, it appends that successor behind the untouched first step, so the next Enter can dispatch the first step again instead of the step the UI offered.

These are source-confirmed control-flow and ownership defects on both current implementation branches. They do not require a malformed plan, provider failure, race, stale repository, or model ignoring its instructions. The recommended repair is a shared plan and obligation ledger with policy-derived eligibility and aggregate terminal state, not another planner, larger model, or unbounded retry loop.

**Evidence status:** static production-source and test-definition inspection only. No codeNERD builds, tests, application journeys, model inference, executable fixtures, failure injection, or benchmarks were run. No repository implementation was changed. This document is a proposal, not an implemented repair. Conditional traces and evaluations below are not reproduced incidents or passed tests.

## Source freeze and scope

Branch heads were read at approximately 19:37 UTC and rechecked unchanged at approximately 19:41 UTC on October 28, 2026.

| Surface | Immutable revision | Role |
|---|---|---|
| Main | [356f65e0e6bb639adaa90ab36212daa714114871][main] | Current implementation baseline |
| Active feature | [c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c][feature] | Conversation-first work modes; same queue defect |
| Current research | [622ab196245cfa4a12b49393657f3b0dc42ace6b][research] | Current audit inventory checked for duplication |
| Historical research | [c592ff223cd207fca4c29ae793d5eba57aa4781d][historical] | Historical audit inventory and related reports |

The root [North Star][root] places planning, orchestration and completion in the Mangle executive. Scoped [session guidance][session-guidance] requires measured, revision-bound completion and preserved cancellation; [core guidance][core-guidance] distinguishes mechanical execution from accepted behavior. These are intended architectural constraints, not evidence that this dream-plan integration already satisfies them. The current CLI specification lists dream and continuation helpers as specialized branches, but does not establish their end-to-end queue contract. [CLI implementation map][cli-spec]

The affected path is **approval and execution of an extracted dream plan**. It is not `executeMultiStepTask`, the session edit-site planner, campaign DAG execution, or feature background job chaining. These paths have different schedulers and must not be conflated.

### Relationship to existing research

The audit inventories of both research branches were retrieved and their Markdown bodies searched for the dream-plan entry, queue, manager advancement and completion terms. Relevant adjacent reports and the journey map were read. No matching queue-conservation finding was found in that inspected corpus. This is not a claim that every historical repository file has been exhaustively reviewed.

- [Continuation obligation generation][generation-prior] already uses approved dream execution as a reachable entry to the continuation protocol. Its finding is that an old tester result suppresses a newly generated obligation sharing a description. Here the original plan tail is not consulted even with no generated obligation at all.
- [Planned work conservation][plan-prior] concerns the session edit planner truncating an oversized plan. This finding needs only two steps, neither truncated nor rejected, already stored and shown to the user.
- [Background outcome and continuation ownership][background-prior] concerns a completed asynchronous job and its successor. This finding occurs in the separate dream-plan/continuation message flow without a background job.
- [Multi-step fallback conditions][fallback-prior] concerns conditions erased from `TaskStep` and a nil-error dependency gate in `executeMultiStepTask`. The dream path does not call that executor.
- [Delegated-turn failure settlement][failure-prior] concerns lost measured work on errors and already documents the contextual formatter's constraint loss. Neither is rediscovered here: the minimal queue witness uses a normal nil-error return.
- The older [journey map][journey] identifies orchestration and task-rewrite concerns. Its old prose-derived continuation-status description is not current behavior; the present host uses observed returns. This report preserves that improvement.

## The production path

### A real producer stores the plan

`handleDreamState` converts consultation results, calls `core.ExtractDreamPlan`, and stores a nonempty extracted plan in `dreamPlanManager`. That is the live producer, rather than an assumed external caller. [Main producer lines 348–375][producer]

`DreamPlan` owns the plan ID, all subtasks, lifecycle state, approval and completion times, and counters. `DreamSubtask` includes its own ID, order, task, action, target, description, mutation flag, status and dependency indices. `StorePlan` retains this object and asserts `dream_plan` and `dream_plan_subtask` facts. [Plan types][plan-types], [storage and fact publication][store-plan]

When a pending plan exists and a natural-language execution trigger is recognized, `handleSubmit` calls `handleDreamPlanExecution`. The trigger is separate from feature `/go`; it is not the brief-only interpretation defect. [Main submit lines 211–214][submit], [feature submit lines 211–214][feature-submit]

### Approval creates two inconsistent execution representations

The handler approves the manager's plan, copies all its subtasks into the TUI's simpler `Subtask` type, shows the whole plan and its step count, and calls `StartExecution`. The manager becomes `executing`. It then assigns the **entire** copied slice to `m.pendingSubtasks`, sets the progress counter to one, and directly calls `executeSubtask` for `subtasks[0]`. It does not remove that first entry from the queue. [Main handler lines 692–757][approve], [feature handler lines 714–778][feature-approve], [manager start lines 128–144][manager-start]

The conversion retains only ID, Description, ShardType and IsMutation. It does not carry `DependsOn`, `Task`, `Action`, `Target` or the manager's status. This matters to repair design: merely draining the slice would not automatically restore the manager's dependency-aware selection. `GetNextPendingSubtask` already has dependency checks, but the TUI path selects index zero directly and does not call that manager selection API. [Conversion][approve], [existing dependency selector][selector]

### Worker completion asks a different source of truth

`executeSubtask` runs the requested shard through `spawnTaskWithContext`, receives an `observation.Return`, and injects measured result/debt facts. It then queries `should_auto_continue` and `has_pending_subtask`. Those default rules derive work from `pending_test`, `pending_fix`, or an incomplete shard result. They do not join `dream_plan_subtask`, the approved plan tail or the TUI queue. [Worker lines 83–141][worker], [default continuation policy][policy]

The host's debt producers are concrete: `Untested`, production-Go writes without an observed test run, structured findings, and the `/hollow` result mapping. The host does not need to misparse result prose for this issue. [Observed fact/debt producer][debt]

If no eligible evidence follow-up is returned, `executeSubtask` emits `continuationDoneMsg` based on **this step's** observed outcome. It never examines the approved-plan queue before doing so. A hard error also emits a done message, with a failed outcome; the minimal witness does not need that branch. [Worker terminal paths][worker]

### Terminal UI cleanup discards the plan queue

The Update handler for `continuationDoneMsg` stores the completed result, clears loading and progress counters, and unconditionally sets `m.pendingSubtasks = nil`. It uses the message's single-step outcome for its terminal banner. It neither asks whether approved steps remain nor advances the dream manager. [Main Update lines 416–441][terminal], [feature Update lines 452–477][feature-terminal]

The manager's advancement APIs exist: `MarkSubtaskRunning`, `MarkSubtaskComplete`, `MarkSubtaskFailed`, and completion/archive handling. None is called by the inspected approval → worker → Update chain. Repository code search for `MarkSubtaskComplete` and `GetNextSubtask` found the manager, model helpers and tests, not a production TUI caller. More importantly, the actual event handlers above show the omission directly. `HasPendingPlan` is true only for `pending`, not `executing`. [Manager APIs][manager-advance], [manager pending guard][pending-guard], [manager completion][manager-terminal]

Thus the plan bytes can remain in the manager while the runnable UI queue disappears. The claim is **lost execution ownership and inconsistent lifecycle state**, not universal deletion of all plan data.

## Two source-derived witnesses

### Quiet first step abandons the remaining plan

Use a valid approved plan with at least two distinct steps, a normal initialized executor and kernel, and no unrelated outstanding continuation facts.

1. The handler stores `[S1, S2]` in the queue and dispatches S1 directly. S1 remains the queue head.
2. S1 returns without a Go error, with no structured findings, no untested paths and no new production-Go test debt. A documentation-only write is a useful candidate; a reviewer return without findings is another, subject to the plan fixture's actual first step. Its observed outcome can be `/unverified`; `/done` is not required.
3. No `has_pending_subtask` derives. The worker returns `continuationDoneMsg` with step count one.
4. Update clears `[S1, S2]` and resets progress. S2 was never dispatched by this plan owner. The manager remains `executing`, with no advancement from this path.

If S1 is `/unverified`, the current banner honestly says the work is unverified, but still loses the approved tail. If S1 is `/done`, the UI additionally renders “All 1 step(s) complete and verified,” even though it previously showed a two-step plan. That is a conditional aggregate-reporting consequence, not a claim that every reachable first step receives `/done` or that every model answer falsely declares the entire goal finished. [Terminal rendering][terminal]

The disappearance is not a cap or cost-control decision: no configured plan window was exceeded, and the second task was already admitted. The plan is not resumed by the ordinary pending-plan trigger because its manager is now `executing`; the emptied queue also gives ordinary Enter nothing to consume. A new user request or a separate plan-management action can still change state, but neither is automatic conservation of this approved run.

### Confirmation dispatches a different step from the displayed offer

Use the same two-step plan, but let S1 produce one ordinary evidence obligation F—for example, a structured finding that raises a coder fix. Select a mode that pauses before F.

1. S1 is still queue head when its result yields `continueMsg(F)`.
2. Update displays “Next: F” and appends F, producing `[S1, S2, F]`.
3. Empty Enter produces `confirmContinueMsg`. Its handler removes and executes `pendingSubtasks[0]`, which is S1, not F.

This is a dispatch-identity mismatch. It does not depend on which of several facts the kernel chooses: the fixture has just one eligible follow-up. It establishes a second worker dispatch of S1, not inevitable duplicate external effects; the worker's live capability, exact permission and idempotency mechanisms still apply. A user intending to confirm F has not received a faithful execution of the displayed choice. [Main pause and confirmation handlers][pause], [feature pause and confirmation handlers][feature-pause], [empty Enter entry][enter]

Main pauses in Confirm mode, or Breakpoint mode for a successor marked as a mutation. Feature uses `pausesBefore`: Explicit pauses for every successor, Propose for mutations, Yolo never. The branch's work-mode change is real but does not change the queue aliasing. [Feature work-mode policy][work-mode]

## Branch parity and exclusions

`internal/core/dream_plan_manager.go` and `codedom_continuation.mg` are byte-identical across main and feature. The entire relevant `executeSubtask` body, including the no-debt terminal path, is unchanged. That file differs later in obligation-description normalization and display helpers; those changes do not consult the plan queue.

The feature handler changes the displayed work-mode label, while retaining full-slice queue initialization and direct first dispatch. Its Update changes the pause-policy call but preserves append-to-tail, head-based confirmation and unconditional terminal clearing. Relevant anchors are included beside main above.

Keep the following limits explicit:

- A one-step plan has no omitted tail, although the manager-state and confirmation-replay problems can still matter if it raises debt.
- No pending plan means this approval branch is not taken. Failed approval/start returns before worker execution.
- Existing evidence-generated continuation remains useful. A new follow-up is not the same object as an admitted plan step; it must not accidentally stand in for the plan tail.
- No claim here establishes a permission bypass, an unauthorized write, actual duplicated effect, cross-session race, failed code behavior, or measured defect frequency.
- A stopped or failed plan need not keep executing. Its remainder must remain explicitly stopped, canceled, blocked or resumable according to policy, not silently disappear as a side effect of generic worker cleanup.
- Fixing this queue does not repair description-generation discharge, step-condition loss, original-task fidelity, or background ownership. Those retain their own contracts.

## Proposed execution contract

All names in this section describe proposed states, not implemented predicates or APIs.

### One owner and an explicit transition record

Use a plan-owned execution record keyed by session, plan ID, plan generation and step ID. Preserve the approved step manifest, dependency structure and original scope. Keep the attempt ID separate from step identity so an explicit retry is distinguishable from duplicate delivery.

Give each admitted step exactly one current disposition: pending, offered, running, terminal with measured outcome, blocked, canceled or explicitly superseded. Do not equate a nil Go error, `/unverified`, a skipped step and accepted behavior. The existing observed return is the evidence input; Mangle derives eligibility, retry/stop and aggregate completion.

An evidence-generated obligation is separately identified and linked to the step/attempt that raised it. It can be inserted before a dependent plan successor when policy requires that order, but must not replace or consume an unrelated planned step. Existing anti-repeat rules and authority restrictions remain in force. The generation-sensitive obligation research remains a dependency for distinguishing genuinely new debt.

### Claims and confirmations must name the same work

Admission should atomically move the selected step from pending/offered into an in-flight claim before returning a command. It must not leave a runnable copy at the queue head. Completion settles only that claim, once. A duplicate completion message cannot advance counters or trigger another dispatch.

A pause offer stores the exact `(plan, generation, step or obligation, attempt, scope)` it displays. Enter or `/continue` consumes that offer, not an arbitrary backlog element. If the offer is stale, replaced, canceled or already consumed, report that state; do not run a different step. Keep backlog presentation separate from the currently confirmable offer.

Plan approval and per-step confirmation must retain their current intended distinction. The repair should not add an extra approval to an already authorized first step or silently make one approval cover future unapproved mutations. It should faithfully apply the configured mode and exact action permission checks.

### Completion is a property of the manifest

The absence of an evidence follow-up permits consideration of the next planned step; it is not plan completion. Derive terminal plan state only after the admitted manifest and induced obligations have explicit dispositions, and no work is running or offered. A plan can terminate failed, canceled or blocked without claiming verified success. A success banner must account for every required step's accepted evidence, not just the last return.

Synchronize the manager and UI from that owned state. A worker error, user stop, source change or query failure should leave an honest remainder and a specific resumption/termination disposition. Do not silently reinterpret kernel-query failure as an empty plan. Preserve cancellation, known effects and existing post-edit proof gates; do not replay a completed effect to repair a missed ledger publication.

### Repair choices and costs

1. **Containment only:** before clearing, detect unresolved approved plan steps, stop honestly and retain them for explicit recovery. Separate the pause offer from the backlog. This can prevent silent loss and wrong-step confirmation without claiming automatic plan execution, but does not deliver the full intended workflow.
2. **Recommended shared owner:** reuse the existing plan model and measured-return machinery, with Mangle-owned next/terminal decisions and a coherent Update-thread driver. Record the full identity and disposition once; derive both UI state and manager progress from it. This is more work than changing a slice index but closes the two failure modes together.
3. **Avoid an independent FIFO patch:** removing S1 and blindly draining the rest is insufficient. It bypasses dependency/verification disposition, cannot reconcile induced debt, and leaves the manager unsynchronized. Do not replace the defect with unconditional execution.

The repair adds state, identity validation, lifecycle transitions, compatibility tests and potentially persisted records. A bounded manifest needs storage proportional to admitted steps and edges; retention must be bounded without deleting undisposed work. Correctly executing the previously omitted tail can increase tokens, tool calls and elapsed time. Reduced replay can save work, but no savings are measured here. These costs should be reported against completion of the same authorized manifest, not credited as a regression merely because the broken baseline stopped after one step. No additional LLM interpretation is required for the deterministic ownership transition itself. Durability across process restart is a separate scope decision; an in-memory fix must not advertise durable recovery.

## Primary foundations and their limits

Gottschalk, van der Aalst, Jansen-Vullers and Verbeek's [Protos2CPN paper, section 3.3][workflow-paper] separates a workflow's ability to finish from proper termination and task reachability. The design inference here is narrow: checking the latest task's outcome cannot establish that an admitted workflow has no remaining work. This does not make codeNERD a Petri-net implementation or prove semantic code correctness; the report borrows a useful control-flow criterion, not the paper's tooling or a claimed formal proof.

Lamport's [Specifying Systems, chapters 4 and 8][tla-book] distinguishes transition safety from eventual progress, including FIFO examples and fairness assumptions. Applied here, “confirmation dispatches the offered ID at most once” and “terminal success leaves no required step undisposed” are safety properties. “An eligible authorized pending step is eventually dispatched or explicitly blocked” is a separate progress obligation, conditioned on provider availability and user confirmation. Merely retaining the queue forever would satisfy conservation but not useful progress. These are proposed invariants and assumptions; no TLA+ model or checker was run.

## Evaluation program not run

### Deterministic production entry coverage

Use the real embedded continuation policy and the normal `handleSubmit`/approval → returned command → Update sequence, with an observed recording executor. Seed a valid pending plan for scheduling fixtures; separately exercise actual dream extraction as an integration fixture. Direct calls to manager methods or a synthetic final message alone bypass the handoff under review.

| Fixture | Required observation |
|---|---|
| Two independent steps, S1 returns nil error and `/unverified`, no generated debt | S2 remains owned and is dispatched or explicitly blocked under policy; no silent clear |
| Same with S1 `/done` | No aggregate complete/verified banner while S2 is pending |
| S1 produces one fix obligation, main Confirm and feature Explicit | Offered ID equals the next dispatched ID; S1 is not replayed merely because it remained in backlog |
| Mutation obligation in main Breakpoint and feature Propose | Same exact-offer rule; per-mode pause behavior preserved |
| Main Auto and feature Yolo | Progress preserves original tail and induced debt without adding confirmation or replay |
| Single-step plan with and without generated debt | No phantom step; manager and UI agree on the actual terminal state |
| Dependency-bearing plan | No successor dispatch before its specified eligibility condition; preserve failure and skipped semantics explicitly |
| S1 hard error, cancellation or policy stop | Known partial effects retained; remainder explicitly blocked/canceled/resumable; no false success or unconditional continuation |
| Duplicate Enter, duplicate completion, late result after plan replacement | One owned dispatch/settlement; stale messages cannot consume another offer or plan |
| Kernel query/assert failure | Explicit execution-state failure, not empty-work completion |
| No plan, empty plan, invalid approval state | Existing nonexecution/error responses remain |
| Ordinary evidence-only continuation without a dream plan | Existing measured-debt behavior and anti-repeat bounds remain |

Capture plan/step/attempt identity, manifest size, pending/offered/running/terminal counts, each recorded dispatch, manager status/counters, exact pause text, consumed offer ID and terminal banner. Independently instrument the executor boundary; do not infer call count from model prose. Assert conservation across every transition, not only at the end. Review the semantics of dependencies on failed, skipped and unverified predecessors before selecting expected behavior.

Existing tests do not establish this combined contract. `TestUpdate_ContinuationDoneOutcome` injects a final message into a fresh model and checks its banner and cleared queue; it does not construct a pending dream manifest and run approval plus a worker. Manager tests explicitly call `MarkSubtaskComplete` and test that API, not whether TUI completion calls it. They are useful component controls, not end-to-end assurance. These test definitions were read, not executed. [Terminal UI test][terminal-test], [manager tests][manager-tests]

### Bounded state exploration

Before probabilistic model comparison, enumerate short event histories for one to three plan steps and zero to two induced obligations: approve, claim, return, offer, confirm, cancel, replace, duplicate and delayed message. Check exact-offer identity, at-most-one active claim per attempt, manifest conservation and no successful terminal state with required remaining work. Keep ordinary dependency order, an explicit blocked state and a no-plan control. A bounded model can reveal control defects without claiming proof over unbounded plans, external effects or arbitrary model behavior.

### Fair workflow comparison

If later authorized, run paired normal-TUI journeys with the same accepted plan, workspace snapshot, provider/model, tool envelope, work mode and user confirmations. Include full extraction and approval after controlled scheduling fixtures pass. Randomize treatment order for live-model runs; keep failures, canceled runs and unverified results in the denominator.

Score admitted-step disposition coverage, required-step acceptance, undisposed tail count, offered/dispatched identity mismatches, repeated dispatches without explicit retry, and manager/UI terminal agreement. Measure total tokens, tool calls, wall time and confirmation burden per **whole admitted plan**. Report whether higher cost comes from finally executing formerly dropped work. A lower cost achieved by abandoning the tail is not a win; a higher “success” rate from relabeling unverified steps is not a win either.

The initial acceptance target is deterministic: every admitted step remains owned, and every confirmation dispatches the offered work. Better model output or faster completion is a later empirical question.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[research]: https://github.com/theRebelliousNerd/codenerd/tree/622ab196245cfa4a12b49393657f3b0dc42ace6b/Docs/audits
[historical]: https://github.com/theRebelliousNerd/codenerd/tree/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits
[root]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#north-star
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/agents.md
[core-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/agents.md
[cli-spec]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/cli/IMPLEMENTED_SPEC.md#L402-L427
[generation-prior]: https://github.com/theRebelliousNerd/codenerd/blob/622ab196245cfa4a12b49393657f3b0dc42ace6b/Docs/audits/continuation-obligation-generation.md
[plan-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/planned-work-conservation.md
[background-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/background-outcome-and-continuation-ownership.md
[fallback-prior]: https://github.com/theRebelliousNerd/codenerd/blob/622ab196245cfa4a12b49393657f3b0dc42ace6b/Docs/audits/multistep-fallback-condition.md
[failure-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/delegated-turn-failure-settlement.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/journeys/00-journey-map.md
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_dream.go#L348-L375
[plan-types]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dream_plan.go#L20-L53
[store-plan]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dream_plan_manager.go#L33-L78
[submit]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_handlers.go#L211-L214
[feature-submit]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_handlers.go#L211-L214
[approve]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_handlers.go#L692-L757
[feature-approve]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_handlers.go#L714-L778
[manager-start]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dream_plan_manager.go#L128-L144
[selector]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dream_plan.go#L88-L107
[worker]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_continuation.go#L61-L154
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/codedom_continuation.mg#L16-L70
[debt]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_continuation.go#L168-L303
[terminal]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_update.go#L416-L441
[feature-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L452-L477
[manager-advance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dream_plan_manager.go#L174-L248
[pending-guard]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dream_plan_manager.go#L88-L92
[manager-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dream_plan_manager.go#L298-L318
[pause]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_update.go#L368-L412
[feature-pause]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L411-L448
[enter]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_handlers.go#L105-L113
[work-mode]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_mode.go#L44-L54
[workflow-paper]: https://www.vdaalst.com/publications/p447.pdf#page=9
[tla-book]: https://lamport.azurewebsites.net/tla/book-21-07-04.pdf
[terminal-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/continuation_done_test.go#L8-L78
[manager-tests]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dream_plan_manager_test.go#L237-L307
