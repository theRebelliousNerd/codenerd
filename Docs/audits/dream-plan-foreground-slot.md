---
doc-class: deep-dive
subsystem: cli
implementation-status: proposed-continuation
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
continues: Docs/audits/slash-operation-slot-admission.md
related: Docs/audits/side-turn-ordering-and-cancellation-ownership.md
scope: approved dream-plan step admission to the foreground execution slot
---

# Approved plan steps must join foreground cancellation and settlement

## Decision and evidence boundary

Extend the existing foreground-slot contract to the actual command that executes an approved dream-plan step. The plan ledger correctly owns the step's scheduling and outcome, but the worker does not reserve the foreground execution slot or register its cancellation function. Ctrl+X consequently has no owner to cancel, and clearing the loading flag can admit queued normal work while the plan worker remains live.

This is a new producer-level exit under [slash-operation slot admission][slash-audit] and side-turn S3, not another general ownership design. The slash repair is already present. The current UI also correctly says that it has stopped **waiting**, not that the unowned task has stopped; do not revive the older false-“Stopped” claim.

Public main was re-resolved at approximately 08:43 UTC on 2026-10-30 to [388205138][main]. Source, current guidance, relevant audit status and the 0715/0747/0817 research additions were inspected. An independent source challenge supported this boundary. No CodeNERD program, Go test, model inference, benchmark or failure injection was run. The schedule below is an unexecuted source-derived witness, not an observed incident. No implementation code was changed during this review.

## Preserve the existing repairs

Commit [c3b5bef85][slash-fix] routes delegating slash work through `foregroundOperation`. That wrapper accepts an owner on the caller's thread, installs the actual task cancel when the command begins, preserves shutdown ancestry and usage labels, latches pre-start stops, recovers panics and releases only its own owner. The ordinary `processInput` route also joins the owner-keyed contract. These are implemented controls, not missing infrastructure. [Wrapper][foreground] [Ordinary admission][ordinary]

The [side-turn audit's as-built section][side-audit] records owner-qualified stop, begin/end fencing and queue wind-down. Its outstanding terminal-message identity limitation should remain separate. The present defect is earlier: a live plan worker has **no foreground owner at all**, so the existing queue gate has nothing to wait on.

The plan integration has also improved. It marks dispatched steps running, retains approved steps until they settle, runs exactly the offered step, and preserves earlier unverified outcomes in its final report. Those ledger semantics and their tests should remain intact. [Dispatcher and offer][dispatch] [Settlement][settlement] [Plan-ledger tests][plan-tests]

Finally, the current Ctrl+X branch checks whether cancellation found an owner. Without one it prints a truthful warning that the command cannot be canceled through that path and its result will still arrive. That disclosure reduces misleading reporting, but does not supply cancellation or single-foreground admission. [Current stop handler][keys]

## The missing admission edge

### 1. A valid plan reaches a separate execution entry

After ordinary text is entered, `handleSubmit` recognizes a pending dream plan and an execution trigger such as “do it.” It calls `handleDreamPlanExecution` before reaching its usual `processInput` command construction. `beginLiveTurn`, called earlier, only initializes the elapsed timer and activity display; it does not accept a `turnState` owner. [Submit split][submit] [Display-only helper][live-display]

The plan handler validates a current nonempty plan, approves it, calls `StartExecution`, initializes the continuation ledger and selects the first dependency-ready step. If those operations fail, it does not dispatch. On success it calls `dispatchSubtask(first)`. [Plan approval][plan-approval] [First-step dispatch][plan-start]

### 2. The dispatcher makes the UI busy without taking the slot

`dispatchSubtask` clears the pause offer, marks an owned plan step running, sets `isLoading = true`, sets a status label and returns a batch containing `executeSubtask(step)`. It does not call `turn.accept` or use `foregroundOperation`. [Dispatcher][dispatch]

`executeSubtask` creates a session-operation context and local cancel, checks the captured Model's `isInterrupted`, builds session context and calls the observed task executor. It never registers that cancel through `turn.begin` or installs an owner-keyed `turn.end`. [Worker][worker]

The session-operation context is a child of the shutdown context, with usage/session/progress metadata. That preserves full-application shutdown, but does not connect the task's local cancel to Ctrl+X. A source comment in the worker says this ancestry makes Ctrl+X cancel the in-flight call; the actual key handler now cancels the registered `turnState` owner, not the session root. [Context creation][operation-context] [Stop primitive][stop]

The preflight `isInterrupted` test is not a cancellation handle for work already inside `spawnTaskWithContext`. The primary witness lets the worker start before stopping, so it does not depend on a disputed pre-start timing window.

### 3. Ctrl+X cannot cancel an unregistered worker

In normal ChatView, with loading true, Ctrl+X marks interruption, calls `turn.stop()`, asserts `interrupt_requested` where a kernel exists and clears loading. `stopExecution(0)` returns false, so the handler reports “Stopped waiting” rather than “Stopping.” It does not cancel the session root or the worker's unregistered child context. [Key handler][keys] [Owner-qualified stop][stop]

This is not evidence that the task cannot stop by any means. Natural completion, cooperative internal stopping and full application shutdown remain possible. It is evidence that this key event does not deliver the context-cancellation signal through the intended foreground contract.

### 4. The queue can admit a successor before settlement

A normal message typed while this plan worker shows loading encounters no registered working label, so `submitWhileBusy` queues it. After Ctrl+X clears loading, outer Update sees queued work. `startQueuedTurn` requires loading false and owner zero; both are true although the plan worker is still pending. It can therefore call `processInput` for the queued request and reserve that new request's slot. [Busy input][busy] [Update drain][update] [Queue gate][queue]

The source establishes overlapping **admission** under the conditions below. It does not guarantee simultaneous provider sends or conflicting writes: downstream schedulers, budgets, task availability and permission gates still apply.

## Conditional production witness

Use an initially idle, initialized TUI in ordinary ChatView, with an empty foreground slot and a live session shutdown context. There is a valid pending dream plan whose first dependency-ready step is a read-oriented reviewer task, for example reviewing the retry policy. Approval is successful. A read step avoids requiring a particular mutation, approval modal or tool permission outcome.

1. Submit the normal execution trigger. Let the real `handleSubmit` → plan approval → dispatch command run the first plan worker.
2. Keep that worker pending inside a context-aware executor/provider wait. Its context remains live; no noncooperative dependency is necessary because the missing event is the cancel signal.
3. Submit ordinary request B while loading is true. With no working owner label, B is queued.
4. Press Ctrl+X before the plan worker settles. `turn.stop()` finds owner zero, the UI truthfully reports stopping only the wait, and loading becomes false.
5. Outer Update can schedule B through the empty foreground slot before the first plan worker returns. If the returned command is run while the first worker remains pending, B begins under its own owner while the first remains unowned.

Even without B, the cancellation-reachability defect stands. B demonstrates why this is not merely a cosmetic spinner issue. The schedule requires an operation still live at the key event; a task that already completed is a negative control, not evidence against the finding.

No particular live-model plan generation is claimed. The existing kernel-backed [plan-ledger fixture][plan-tests] stores reviewer/researcher steps and drives “do it” through `handleSubmit`; it establishes that this is an intended supported route. A future integration check should additionally obtain the plan through normal dream generation.

## Limits and nonduplication

- **Truthful stop is already fixed.** Retain the no-owner warning. Do not claim the current UI falsely proves worker termination.
- **Shutdown is distinct.** Ctrl+C/Ctrl+D invoke shutdown, which cancels `shutdownCtx`. Do not cancel the whole application merely to stop one plan step. A cancellation signal is not a guarantee that every descendant has joined. [Shutdown][shutdown]
- **The kernel interrupt remains useful.** `interrupt_requested` derives a continuation blocker. It is not the local cancel function for an already-awaited task request. Preserve it while fixing the missing ownership edge. [Continuation blocking][policy]
- **Plan approval is not tool authorization.** Existing permissions, exact human approval, restriction enforcement, worker verification and partial-result reporting remain independent. This finding establishes no forbidden effect.
- **Ledger conservation is separate.** The [dream-plan queue audit][plan-audit] concerns preserving and settling the approved manifest. Its current working ledger does not establish registration of the actual executor context in the foreground slot.
- **Recent delivery findings remain separate.** Delivery restriction transport concerns inherited effect obligations; delivery answer ownership concerns response projection. This witness starts at an approved plan, needs no Jev delivery round, and concerns execution admission/cancellation.
- **Stale terminal messages remain the existing S3 residual.** A later unowned result may disturb newer UI state, but that is not a second new finding here. The repair must not worsen it or misreport a canceled task as having made no changes.

## Required lifecycle refinement

Apply the repository's [typed, logic-owned execution guidance][guidance] through the existing owner machinery. No new planner, model call or competing permission authority is needed.

1. **Accept before scheduling.** An approved step that is about to run as foreground work must obtain its execution owner on the Update/caller thread. The accepted owner must exist before Ctrl+X or a queued request can observe the dispatch. Merely registering the cancel inside a later goroutine leaves the pre-start gap.
2. **Register the actual execution context.** At worker entry, bind the cancel controlling the task call to that owner, preserving session shutdown, usage, progress, model configuration and any correctly supplied request context. A latched stop must prevent new task execution. Do not globally add `turn.accept` to `sessionOperationContext`, whose callers include other lifetimes.
3. **Hold through settlement.** A stop request does not free the owner. Keep the queue fenced until the worker has returned and the required step outcome can be settled; release only that owner on completion, error, cancellation or recovered panic. A cancellation-ignoring dependency remains owned and visibly winding down rather than silently letting another foreground task replace it.
4. **Preserve typed plan settlement.** Reuse the slash wrapper's mechanics, but do not blindly substitute a generic `errorMsg` for a continuation result. The current ledger needs `ran` and its outcome to settle or reoffer the precise step. A pre-start stop should preserve the existing pending/held semantics; an in-flight error must retain known partial observations. Every exit must account for both execution ownership and plan-step disposition.
5. **Keep pauses and successors distinct.** An offered, not-running step should not hold a live execution cancel indefinitely. Resuming it accepts a new execution attempt for that same offered step. A completed step's induced follow-up and the remaining approved plan must continue through their existing ledger/work-mode rules, not be skipped or treated as consent to arbitrary effects.
6. **Do not replay work to repair bookkeeping.** No additional effect or provider call should be run merely to create a missing settlement message. Keep owner-qualified terminal handling compatible with the existing S3 work; separate task/step identity from request-answer identity.

The minimal repair should enumerate all `dispatchSubtask` callers, because this worker is also used outside dream approval. The acceptance claim here remains limited to the approved-plan route until those other producers are exercised.

## Existing coverage and proposed evaluation

The [foreground-operation tests][foreground-tests] drive the repaired slash route, verify cancel delivery, wind-down exclusion, pre-start stop latching and panic release. They prove the reusable machinery's intended contract, not that dream-plan dispatch uses it.

The plan-ledger tests exercise first-step completion, the approved tail, exact offered-step execution and aggregate unverified reporting. Their ordinary dispatches complete synchronously under the test executor; the inspected cases do not assert a registered foreground owner or hold a worker across Ctrl+X and queue drain. Keep both families and add the missing composition. These are observations about test definitions, not fresh test results.

Future deterministic evaluation should drive actual plan approval and the returned batch command with a recording executor that exposes its context and explicit start/settle barriers. Do not manually call `turn.accept` or `turn.begin` in the fixture; that would install the very edge under review.

| Case | Required observation |
|---|---|
| Valid approved reviewer step, worker pending, Ctrl+X | The actual worker context is canceled; owner remains held until settlement |
| B queued before stop or submitted during wind-down | No B task admission/start before A settles; B proceeds once afterward |
| Stop after dispatch but before worker entry | Accepted stop is latched; no task execution; exact plan step remains explicitly held/pending |
| Normal return, early error and recovered panic | Correct owner is released once; plan-step disposition is not lost |
| Cooperative cancel with partial observations | Known effects/results retained; no invented “nothing happened” or verified success |
| Cancellation-ignoring test dependency | Honest winding-down state and retained owner; no false join or forced deletion |
| Explicit/propose pause and later resume | Exact offered step executes under a new owner; unrelated work is not canceled |
| Plan continuation and induced obligations | Existing approved-tail conservation and aggregate unverified reporting remain |
| Full quit and approval-modal handling | Existing shutdown ancestry and modal key contract remain intact |
| Ordinary chat, repaired slash work, independent background jobs | Existing ownership remains correct; background lifetimes are not commandeered |
| Late stop/release for A after B owns the slot | Owner fencing prevents cancellation or release of B |

Record accepted owner, registered cancel, plan/step/attempt identity, context cancellation, worker settlement, queue state and actual task starts. Separate “cancel requested,” “worker returned” and “effect outcome known”; none implies the other automatically.

## Costs and acceptance

The narrow adapter requires no extra inference. It adds small owner/context bookkeeping and lifecycle branches, with maintenance cost across shared continuation producers. Keep locks around state transitions only, not task waits, and keep the UI responsive.

Correct serialization may increase B's wait compared with the defective overlap. That is the intended single-foreground behavior, not a regression to hide by releasing the slot early. Cancellation may save provider work, but no saving is measured; provider responsiveness and already-incurred cost remain outside this patch's guarantee. Measure stop-to-signal latency, signal-to-settlement latency, queue delay, duplicate starts, orphaned owner/step counts, allocations, and all calls/tokens under matched workloads.

Compare current main, a narrow ownership adapter and any broader lifecycle refactor with the same plan, scripted outputs, policy, work mode and task limits. A variant that disables plan execution, discards queued work, adds arbitrary deadlines or changes approval policy cannot pass merely because no overlap occurs.

**Acceptance:** the actual approved-plan worker joins the existing foreground accept/start/cancel/settle contract, Ctrl+X reaches its task context, and queued normal work cannot acquire that foreground slot before the worker settles—while the plan ledger, truthful notices and independent safety controls keep their current meaning.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[slash-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/slash-operation-slot-admission.md
[slash-fix]: https://github.com/theRebelliousNerd/codenerd/commit/c3b5bef85d247b1748125f5c171c034a20a6a820
[foreground]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/foreground_operation.go#L14-L50
[ordinary]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L71-L133
[side-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/side-turn-ordering-and-cancellation-ownership.md#as-built-status
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/continuation_ledger.go#L63-L98
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/continuation_ledger.go#L101-L244
[plan-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/dream_plan_ledger_test.go#L14-L145
[keys]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_key_handler.go#L60-L86
[submit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L173-L194
[live-display]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/glass_box.go#L113-L129
[plan-approval]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L685-L725
[plan-start]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L747-L771
[worker]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_continuation.go#L40-L92
[operation-context]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/operation_context.go#L10-L38
[stop]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/side_turn.go#L214-L235
[busy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/side_turn.go#L352-L380
[update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L24-L36
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/side_turn.go#L597-L628
[shutdown]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L21-L34
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/codedom_continuation.mg#L80-L96
[plan-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/dream-plan-queue-conservation.md
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md
[foreground-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/foreground_operation_test.go#L72-L185

