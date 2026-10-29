---
doc-class: research-proposal
subsystem: tui-foreground-operation-admission
implementation-status: proposed-not-implemented
last-verified: 2026-10-29
verified-against: 9dcb01b9736bf4763aa3ac391d29f133d73c6611
verification-method: static-source-inspection-only
---

# Slash operation slot admission

## Summary and decision

Extend the existing foreground ownership contract to asynchronous slash commands that present themselves as foreground work. The concrete witness is `/spawn reviewer review README.md`: its handler sets `isLoading`, but its task never reserves the `turnState` slot or registers its cancel there. In ordinary ChatView, Ctrl+X can therefore report “Stopped” and clear the loading flag without canceling the task's context. If the task remains pending, a subsequent normal message can acquire the apparently free foreground slot and be scheduled before it settles. This does not guarantee simultaneous provider sends or conflicting effects; downstream schedulers, budgets, and permissions still apply. The missing admission is established before any model output or approved effect is needed. [Slash handler][spawnhandler], [task closure][spawn], [stop handler][keys], [slot behavior][slot].

This is a bounded acceptance extension to the already fixed side-turn S3 work, not a recurrence of the stale `end` race and not a new general ownership audit. Preserve the existing owner-keyed primitives and route foreground slash work through them before scheduling its task command. Do not register every `sessionOperationContext` globally: session lifetime and foreground ownership are different contracts.

This proposal is source-only. No tests, application runtime, model experiments, implementation were performed. The witness is a control-flow argument, not an observed provider call, output, hang, write, or benchmark.

## Review snapshot and scope

The review baseline is main at `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, rechecked on 2026-10-29 at 12:38 UTC. Code links below are immutable links to that commit. The active code-branch heads `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and `aa7988dec4c62116656ea722ff73b687a8509908` were unchanged in the review snapshot. At 12:45 UTC, `model_handlers.go`, `operation_context.go`, and `side_turn.go` were verified byte-identical to main on all three; the earlier 12:14 UTC check established the same for `delegation.go` and `commands_handlers_analysis.go`. Research1018 at `4809540f31d7af8f42c8cdbd95dcc5ad3c2d7b40` adds the separate Legislator dispatch proposal. These observations are not claims about later branch contents.

The neighboring documents retain their existing scope:

- [Side-turn ordering and cancellation ownership][sideaudit] records S1 through S4 as fixed, including caller-thread acceptance, owner-keyed release, pre-start stop latching, and queue wind-down. This proposal adds slash-entry coverage to that contract. Its already-open terminal-message identity item remains there.
- [Articulation operation ownership][articulationaudit] covers detached stream lifetime and restrictions, rather than foreground slash admission.
- [Background outcome and continuation ownership][backgroundaudit] covers background stop and outcome ownership. Background jobs must not be commandeered by this fix.
- [TUI Legislator dispatch parity][legislatoraudit] concerns selection of the registered service backend. The present finding is cancellation reachability and single-slot admission, independent of which backend performs a task.

Queued slash-command interpretation is excluded. The successor in this witness is a normal natural-language message. The separate `/legislate` loading-state behavior is also excluded. Other users of the same helper are inventory candidates, not evidence that every slash command should become a foreground turn; initialization, ingestion, and campaigns have distinct lifecycle requirements.

## Existing contract and correct path

The architecture says foreground work is accepted when `processInput` builds its command, before execution; cancellation, working labels, and release name that execution. A stopped turn retains the slot while winding down, and a stop before cancel registration is latched. [Foreground contract][architecture].

The ordinary natural-language path implements this distinction. `processInput` calls `turn.accept()` on its caller's thread. Its returned closure first defers `turn.end(turnID)`, then establishes its context and calls `turn.begin(turnID, cancel)`. `begin` cancels a stopped or superseded execution; `end` releases only its own owner. `startQueuedTurn` requires both a clear loading flag and owner zero. [Normal admission and context][process], [owner primitives][slot], [queue gate][queuegate].

Those are positive controls, not defective machinery to replace. A generic claim that normal chat cancellation, stop-before-start, or stale-end protection is broken would contradict the inspected source. The gap is a foreground-looking entry point that never joins those controls.

## Conditional source witness

Use an initially idle, ordinary ChatView, not ApprovalView. There is no preexisting foreground owner, background job, or continuation required. The session shutdown context is alive; task executor and shard manager are available. Submit `/spawn reviewer review README.md` and actually execute the task child of the returned Bubble Tea batch. Let task A still be pending when Ctrl+X arrives, for example while a context-aware task or provider request awaits a result. This read-oriented example requires no approved write, particular model choice, model output, or noncooperative provider.

1. **Slash entry bypasses normal admission.** `handleSubmit` recognizes a leading slash and returns `handleCommand` before its normal loading and `processInput` path. The command registry dispatches `/spawn` to `handleCmdSpawn`. [Submit split][submit], [command registration][commands].
2. **The UI becomes busy without an owner.** `handleCmdSpawn` extracts the type and task, sets `isLoading = true`, and returns `tea.Batch(m.spinner.Tick, m.spawnShard(...))`. It performs no `turn.accept`. [Handler][spawnhandler].
3. **The task receives a session child, not a registered foreground cancel.** `spawnShard` creates `sessionOperationContext`, defers its local cancel, and calls `spawnTask`. Neither that closure nor the raw helper calls `accept`, `begin`, or `end`. The raw helper adds model context and forwards the context to `taskExecutor.Execute`. The default factory constructs a JIT executor, whose inline route clones the executor and invokes `ProcessWithIntent` with its context. [Task closure][spawn], [raw helper][helper], [factory][factory], [JIT route][jit].
4. **Session context ancestry does not connect Ctrl+X.** `sessionOperationContext` parents a new `WithCancel` on `shutdownCtx`, or Background when absent, and adds usage, session, and chat labels. Despite its comment mentioning Ctrl+X, it does not register its cancel in `turnState`. In this witness the shutdown parent is still alive. [Context construction][operationcontext].
5. **Ctrl+X has no foreground cancel to call.** Outside the approval modal, the global handler sees loading true, calls `m.turn.stop()`, ignores its Boolean result, asserts `interrupt_requested` when a kernel exists, clears loading, and publishes “Stopped”. `stop()` targets the current owner; owner zero makes `stopExecution` return false. It does not cancel the shutdown root or A's independently held child cancel. A's context is therefore not canceled by this key path. [Key routing and stop][keys], [stop implementation][slot].
6. **A successor can be admitted before A settles.** With no queued successor, the cancellation defect already stands. If A is still pending and the user then submits normal message B, Enter sees loading false and owner zero and uses `handleSubmit`; B can enter `processInput`. Actual overlapping execution additionally requires B's returned task command to be scheduled before A settles. [Enter admission][enter], [normal admission][process].

There is also a queue variant, but it is not necessary to prove lost cancellation. If normal message B was submitted while A showed loading, the empty working label sends B to the queue. After the same Ctrl+X event, outer `Update` sees loading false and a nonempty queue; `startQueuedTurn` sees owner zero and returns B's command. The intended wind-down fence cannot wait on an execution that never owned the slot. [Busy submission][busy], [post-update drain][update], [queue gate][queuegate].

This conclusion does not require an indefinite hang. It remains true for a fully context-aware task: the missing event is the cancel signal itself. Normal completion, provider deadlines, working-policy stall controls, and full application shutdown can still end work. Their existence does not make this key request reach A.

## Controls and limits of the claim

ApprovalView consumes all keys except the global exits before the Ctrl+X handler. The witness deliberately excludes it. Ctrl+C and Ctrl+D call shutdown; `Shutdown` signals `shutdownCancel`, so a session-derived operation retains a root cancellation path. This establishes a signal, not a guarantee that every worker is joined or that every provider immediately terminates. [Modal and exit routing][keys], [shutdown][shutdown].

The `interrupt_requested` fact derives `continuation_blocked` and prevents automatic continuation in the inspected policy. It is not a reference to the cancel function created by `sessionOperationContext`, and this source path does not demonstrate cancellation of the currently awaited JIT provider request through that fact. Existing permission, tool-safety, and provider controls remain in force; a clear loading flag is not authorization for an otherwise forbidden effect. [Continuation policy][continuation], [context construction][operationcontext].

After A returns, `spawnShard` still records its execution and publishes its result or error. The current `assistantMsg` handler clears loading and publishes content without an execution identity. That can matter if A and B overlap, but generic stale-terminal handling is already the explicit residual item in the side-turn audit; it is not presented as a second new finding here. [Result publication][spawn], [assistant message handling][assistant], [existing residual item][sideaudit].

The existing journey fixture manually calls `turn.accept` and `turn.begin` and sets a working label. Its Ctrl+X case and the ownership tests thus exercise an already-registered foreground turn. The wind-down and pre-start latch tests support the fixed mechanism, not the actual slash-entry boundary. The operation-context test checks usage and session attribution, not Ctrl+X connectivity. These are observations about test source; no tests were run. [Journey fixture and stop case][journey], [ownership tests][ownershiptests], [context test][contexttest].

## Proposed ownership boundary

First classify command lifetimes explicitly. Cheap synchronous commands completed on the UI thread do not need an asynchronous owner. A long-running command presented as the current foreground task must reserve the existing slot on UI admission, before its task command can start. An intentionally independent background job keeps its own lifecycle and cancellation controls.

For foreground slash work, use a command descriptor or narrow wrapper that carries the accepted owner and its operation context through the actual task:

- Reserve the owner at the admission boundary, not inside the worker after scheduling. Preserve the existing stop-before-start latch.
- At worker entry, register the cancel that governs the actual task and ensure a latched stop prevents useful work from starting. Defer owner-keyed release before early exits; preserve panic handling so completion, error, and recovered panic cannot strand the slot.
- Keep the owner until the task has settled. A stop request may clear a spinner, but must not make the slot free. The successor queue must wait for owner-keyed settlement.
- Preserve shutdown ancestry, usage and session labels, model selection, restrictions, and authorization contexts. The wrapper must not replace them with a fresh Background context.
- Report “Stopping” while cancellation has only been requested, and “Stopped” only when the relevant operation has settled. An absent owner must not be silently treated as proof of cancellation.
- Keep parallel specialist children beneath one foreground owner where that is the intended task structure. Do not serialize independent background jobs into that slot or cancel unrelated work.

Do not solve this by automatically accepting a turn inside `sessionOperationContext`. That helper expresses session ancestry and is used beyond this foreground command; blanket registration would conflate scopes and could steal a live foreground slot. Do not cancel the entire application to stop a single foreground operation. Do not add an arbitrary task deadline as a substitute for the missing ownership edge.

No task mutation should be replayed to repair UI state or cancellation accounting. Preserve already-produced partial results with their originating operation. Owner-fenced terminal messages can be developed with the side-turn audit's existing open item; they are not a prerequisite for inventing a new global message architecture.

## Cost and tradeoffs

The minimal change adds local owner IDs, context registration, small command metadata, and existing mutex operations. It requires no extra LLM call. Keep locks around short state transitions, never around provider waits or heavy work, and do not block the UI thread awaiting teardown.

Correct serialization can increase B's wait: that is the intended single-foreground contract, not an incidental performance regression to optimize away. Cancellation may avoid unnecessary work or calls, but no savings were measured. Provider responsiveness still depends on cooperation and existing timeouts; the teardown bound is unknown. The maintenance cost is an explicit command-lifecycle inventory, adapters that preserve context, and boundary tests as commands evolve.

## Future evaluation

The decisive ordering is admission, start, stop, and settlement. A future deterministic test should drive the real `handleSubmit` path for `/spawn reviewer review README.md`, execute the actual batch task child, and supply a fake task executor that captures the context and waits under explicit test control. It should not pre-register an owner as a shortcut.

| Case | Required observation |
|---|---|
| Stop while the actual slash task is pending | Captured task context is canceled; the owner remains held until explicit task settlement |
| Normal B queued before or submitted after stop | No B task starts before A settles; B starts exactly once afterward |
| Stop after handler return but before worker entry | The accepted owner's stop is latched and prevents task work |
| Normal completion, early error, recovered panic | Each releases exactly its own owner without stranding or clearing a successor |
| Owner-bound repeated or delayed stop for A | No cancellation of a later owner B; a new physical Ctrl+X intentionally targets the current owner |
| Full quit | Root cancellation still reaches the operation |
| Approval modal | Its separately specified key behavior is preserved |
| Existing controls | Permission, tool-safety, provider timeout, and working-budget behavior remain intact |
| Scope controls | Ordinary chat stays correct; synchronous commands remain usable; background campaigns keep their own owner; specialist children inherit the foreground cancellation |

Compare the current path, a minimal foreground wrapper, and any later message-identity extension under the same fixed entry path and configuration. Only report call counts, elapsed time, memory, or token costs after later measurement. No experiment or implementation is authorized by this proposal.

The acceptance decision is narrow: include explicit foreground slash execution in the existing S3 admission contract and its tests. Keep backend dispatch, queued slash interpretation, detached streaming, and background lifecycle work with their respective owners.

[submit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_handlers.go#L104-L201
[commands]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands.go#L176-L177
[spawnhandler]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers_analysis.go#L573-L594
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/delegation.go#L759-L824
[helper]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/delegation.go#L35-L47
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L2700
[jit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/task_executor.go#L358-L448
[operationcontext]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/operation_context.go#L10-L35
[keys]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_key_handler.go#L20-L80
[enter]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_key_handler.go#L247-L259
[slot]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/side_turn.go#L76-L184
[busy]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/side_turn.go#L279-L294
[queuegate]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/side_turn.go#L455-L467
[update]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L23-L31
[assistant]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L138-L162
[process]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L48-L92
[shutdown]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_lifecycle.go#L18-L34
[continuation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/codedom_continuation.mg#L54-L79
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/journey_side_turn_test.go#L14-L145
[ownershiptests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/side_turn_ownership_test.go#L181-L230
[contexttest]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/operation_context_test.go#L15-L38
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/4809540f31d7af8f42c8cdbd95dcc5ad3c2d7b40/Docs/architecture/conversation-first-agent.md#L82-L99
[sideaudit]: https://github.com/theRebelliousNerd/codenerd/blob/4809540f31d7af8f42c8cdbd95dcc5ad3c2d7b40/Docs/audits/side-turn-ordering-and-cancellation-ownership.md#L229-L264
[articulationaudit]: https://github.com/theRebelliousNerd/codenerd/blob/4809540f31d7af8f42c8cdbd95dcc5ad3c2d7b40/Docs/audits/articulation-operation-ownership.md
[backgroundaudit]: https://github.com/theRebelliousNerd/codenerd/blob/4809540f31d7af8f42c8cdbd95dcc5ad3c2d7b40/Docs/audits/background-outcome-and-continuation-ownership.md
[legislatoraudit]: https://github.com/theRebelliousNerd/codenerd/blob/4809540f31d7af8f42c8cdbd95dcc5ad3c2d7b40/Docs/audits/tui-legislator-dispatch-parity.md

