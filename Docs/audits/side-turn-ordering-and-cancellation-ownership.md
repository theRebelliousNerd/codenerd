---
doc-class: deep-dive
subsystem: cli
implementation-status: partially-implemented (see As-built status)
last-verified: 2026-10-28
verified-against: ac2f1fd3425dab9c14ff9066f9bf10cad02acc75
verification-method: static-source-inspection
supersedes: []
---

# Side turn ordering and foreground cancellation ownership

**Decision:** keep the new ability to talk while work runs, but give each submitted message and foreground execution a stable identity. Publish side-turn decisions and brief updates only against their captured owner and brief version. Treat cancellation as a request followed by terminal settlement, not as immediate permission to reuse the current-turn slot. Mangle already decides stop, defer and converse; any newly introduced ordering or prioritization semantics should also belong to policy. The current queue drain is implemented in Go, which should retain the mechanics of dispatch, identity, atomic publication and cancellation.

The active feature branch has real side-turn, queue and cancel plumbing. The residual issue is not that those features are absent. It is that asynchronous outcomes operate on mutable “current” state without identifying the execution or message they observed. This creates source-derived schedules in which an old stop targets a new turn, side-turn evidence mixes across messages, a stale brief replaces newer state, or a canceled turn clears its successor's cancellation handle.

**Evidence limits:** this is static diagnosis and proposed design, not a reproduced user incident. No codeNERD execution, tests, inference, failure injection, benchmark, code changes or GitHub writes were performed. The schedules below demonstrate permitted source interleavings to test; they do not measure incidence.

## Source freeze and scope

Inspected on 2026-10-28:

- Main: [356f65e0e6bb639adaa90ab36212daa714114871][main].
- Feature: [ac2f1fd3425dab9c14ff9066f9bf10cad02acc75][feature], committed 09:20:06 UTC and re-resolved unchanged at approximately 09:24 UTC. Main and the research branch were also unchanged at that final check. Initial inspection used 462153c (20 commits ahead and zero behind main); the final head adds two commits to that ancestor.
- Research branch: 5ff4212bf6e97347a34eec905a2eadbcd7a2897a.
- The branch-specific pull-request collection returned no entries for `theRebelliousNerd:feat/conversation-brief`; this is not a CI or merge-readiness finding.
- Both `side_turn.go` and `background.go` are additions relative to main. The new findings here concern the feature implementation, not an assertion that main already contains side turns.
- The feature's three commits after the earlier `777e93c` side-turn implementation add the conversation architecture document, a problems fixture, and configurable Jev uses/tuning. The two further commits, a7437da and ac2f1fd, implement adjacent articulation, restriction, evidence, learning and shutdown work. Comparing the initial and final pins shows `side_turn.go`, `work_hold.go`, `model_update.go`, `model_key_handler.go`, side policy and side tests unchanged. The changed `process.go` still registers and ends the foreground turn at lines 81–83 with no owner token. Its new result-fact identity baseline is not a side-message or foreground-execution generation. The new Jev switch is honored here: judgment-related witnesses apply when turn judgments run; owner and brief-version issues do not depend on Jev.

Read guidance includes root [agents.md][guidance] and [chat guidance][chat-guidance]. Root guidance places executive decisions in Mangle, requires JIT-first model behavior and spec-first change, and prioritizes normal production-path capability over artificial tool-count targets. The [conversation-first architecture document][architecture] defines the intended behavior and its named journeys; it is not itself proof that every concurrent schedule is covered.

## Existing capabilities and the boundary of this note

The feature explicitly separates side-turn intent and judgment predicates from the foreground bridge's `user_intent`; submits pre-work input to a queue; registers the foreground cancel; lets the kernel derive `/stop`, `/later` or `/converse`; and returns background job results through job-bearing completion messages. Those are useful mechanisms to preserve. [Side-turn implementation][side], [side-turn rules][policy], [foreground registration][process-owner]

This note does not duplicate:

- [Perception classification outcomes and recovery][perception-audit], which already identifies the non-nil error branch claiming that input is queued without enqueueing it, safe diagnostic concerns, and primary-classification/Jev cause separation. The unchanged error branch is a dependency to repair, not a new discovery here.
- [TUI evidence turn ownership][evidence-audit], which addresses residual action/result correlation and source delivery. Its broad request-ownership requirement is compatible with this note; the new source witnesses concern side-turn policy state, brief publication and the foreground cancel slot. The final feature snapshot already includes adjacent evidence repairs; do not read this cross-reference as claiming all older evidence findings remain open.
- [Task terminal ownership][terminal-audit] and [delegated failure settlement][settlement-audit], which concern worker retirement and preservation of partial execution evidence. The new shared TUI `turnState` is a different owner boundary; those lower-level repairs do not make its unqualified `end()` safe.
- [Background outcome and continuation ownership](background-outcome-and-continuation-ownership.md), a related audit of the background completion and follow-up chain. In particular, the nonempty background `FollowUp` being omitted when the chat is busy belongs there, not to this implementation queue.

The shared brief is demonstrably changed by these functions. That alone does **not** prove an already-running isolated delegate consumes the changed brief on its next model request. Verification must inspect the actual consumer request, including task clone/subagent wiring, rather than treating a parent-kernel query as delivery evidence.

## Finding S1 A delayed side stop can target a newer turn

`sideTurnMsg` contains input, reply, action and error, but no originating turn, generation, submission sequence or brief version. The command uses the session shutdown context, not the foreground turn's context. Once it returns, `applySideTurn` handles `/stop` by calling `m.turn.stop()`, which cancels whichever function is currently stored. [Message and command][side-command], [application][side-apply], [turn state][turn-state]

A source-derived witness:

1. Foreground turn A is working. The user submits S, a stop directed at A; side perception or its judgment call remains pending.
2. A finishes naturally. Its ordinary completion reaches Update and the UI starts queued/new foreground turn B.
3. S finishes and carries `Action="/stop"`.
4. Update applies S to the current `turnState`, which now stores B's cancel.

There is no owner comparison in this path. An outcome that was correct for A can cancel B without being reinterpreted or reauthorized for B. This witness does not require A to ignore cancellation: A can finish normally before the delayed stop is interpreted.

The same missing owner affects context publication earlier in the command. `recordBrief(intent.Brief)` runs before the side decision and before Update receives the result. A late side result can therefore overwrite a newer full turn's brief even if a future UI handler learns to reject stale side messages. Fencing only `applySideTurn` is insufficient. [Publication order][side-command], [brief replacement][brief-publication]

**Required contract:** capture the intended execution and brief revision at receipt; carry them through classification and policy decision; apply a stop only to that execution. If it already ended, acknowledge that fact rather than implicitly retargeting. Any deliberate “stop all present work” operation needs its own explicit scope and policy semantics, not an accidental mutable-current lookup. A stale conversational result may remain visible with attribution, but it cannot silently acquire authority over new work.

## Finding S2 Side policy facts and whole brief updates have no common publication boundary

Every side command can reach `decideSideTurn` independently. This does not depend on putting those commands in one Batch: Bubble Tea v1.3.10's [`handleCommands` implementation][tea-dispatch] starts each received command in its own goroutine, calls `cmd()` there, and sends its returned message afterward. The side command therefore mutates the kernel before the serial Update handler sees its result. It retracts the two global side predicates, builds the intent, then may wait for Jev. Only afterward does it assert its facts and query the unkeyed `side_turn_action`. [Decision sequence][side-decision]

The rules also have no message or owner argument. Any high `wants_to_stop` value produces stop; stop takes precedence over later. [Rule definitions][policy]

Consider two commands A and B while the same foreground turn works:

1. A retracts the shared side predicates and waits for its slower judgment.
2. B retracts them, receives its judgment, asserts B's facts and obtains B's action.
3. A receives its judgment and asserts A's facts **without another retraction**.
4. A queries a policy state that can contain B's judgment and A's intent.

The [per-call kernel mutexes][kernel-locks] do not make this multi-call sequence atomic. With B's high stop judgment still present, A can derive stop even if its own judgment says otherwise. With competing mutation and conversational intents, a result can be based on another message's work signal. Turning Jev off removes that wait but not the independently interleavable retract/assert/query calls.

Whole-brief replacement has a related but distinct lost-update problem. `briefContext` supplies the current rendered brief; `recordBrief` later retracts five predicates individually and asserts a whole replacement. It checks neither the version read nor whether the returned brief still applies to the foreground work. Two requests can read the same base brief, produce valid separate amendments, and complete in reverse order; the final whole replacement can erase the newer constraint. Readers can also observe a mixture or absence during the individual retractions. [Brief read and write][brief-publication], [brief predicates][brief-type]

An additional single-request ordering issue is visible without concurrency: the brief is replaced **before** the kernel decides that a request should be `/later`. A side message about distinct future work can thus publish its returned goal/constraints immediately while execution of that request is explicitly deferred. The source establishes that ordering, not that every model output changes the goal. Whether a particular amendment belongs to active work, future work, or conversation-wide preference needs an explicit policy contract.

The kernel already provides [transactional retract/assert publication][transaction]. Reuse that mechanism where appropriate, but do not mistake it for the entire solution. A transaction can prevent partial replacement; it cannot by itself identify which message a later unkeyed query belongs to or reject a stale whole-brief revision. Its documented partial assertion-rejection behavior also means callers must inspect errors rather than equating a returned transaction with a complete admitted snapshot.

**Required contract:** preserve immutable per-message evidence, derive a decision keyed by that message and execution, and commit only an admissible brief update against the expected version. Either use isolated decision facts or owner-keyed facts with precise retirement. Never hold a global kernel lock across network calls. Decide how to rebase, reclassify, or surface conflicting brief updates; do not silently merge contradictory natural-language constraints or apply “last completion wins.”

## Finding S3 Ctrl X releases the current slot before its owner has settled

Ctrl+X invokes the stored cancel, asserts `interrupt_requested`, sets `isLoading=false`, and displays “Stopped.” The outer Update wrapper starts a queued turn whenever that flag is false. `processInput` registers its cancel at entry and unconditionally defers `m.turn.end()`. That end clears the shared cancel and working label without knowing whose fields it clears. [Ctrl+X][ctrlx], [queue drain][update], [process registration][process-owner], [state methods][turn-state]

A source-derived cancellation schedule:

1. A is running and B is queued.
2. Ctrl+X requests cancellation of A and sets the UI idle.
3. The same outer Update dequeues B. B begins and stores its own cancel in `turnState`.
4. A is still unwinding. A's deferred `end()` executes and clears B's cancel and working label.
5. A's ordinary terminal message arrives; [assistant/response handlers][terminal-updates] and the [error handler][error-update] unconditionally set the UI idle, without a foreground owner. A further queued request can start despite B remaining active.

A late A `setWorking` can likewise overwrite the shared label after a newer begin. The mutex prevents unsynchronized access to the slot; it does not establish ownership of its contents. This is a logical race and need not appear as a Go data race.

The [Go context contract][go-cancel] specifically separates requesting cancellation from waiting for work to stop. The current source does not wait for the terminal acknowledgement before reusing the slot. This is distinct from whether the lower-level worker eventually retires correctly.

There is also a start-side window: registration happens when the asynchronous `processInput` command runs, not when Update accepts and marks its request loading. An interrupt before registration can find no cancel. That transition should be covered by an accepted/start/cancel latch, rather than relying on how quickly Bubble Tea schedules the command.

**Required contract:** accept and identify the foreground execution before dispatch; latch cancellation across start; distinguish cancel requested from settled; qualify begin, working-label update, end and terminal-message application by that execution. Keep the interface responsive while settling. Do not require blocking the UI event loop. If later work is allowed to overlap settlement, its facts, cancel, streaming messages and terminal updates must be independent; otherwise retain queued work until the predecessor's owned terminal event permits dispatch.

Queued work should not be silently discarded by a stop. Policy must distinguish “stop this task” from “stop this task and cancel its pending successors.” A known accepted request needs a visible terminal or retained pending disposition.

## Finding S4 FIFO storage does not preserve submission order

The `queued []string` container appends and removes from the front, but working-stage side inputs are added only after classification has completed and Update applies `/later`. Two user inputs received B then C can therefore enter the queue C then B. An input received before the foreground reaches working state is enqueued immediately, while a later side input uses another admission route. [Receipt and classification][side-command], [enqueue at application][side-apply], [queue methods][turn-state]

This is not a claim that every queue must use rigid FIFO. A user may intentionally reprioritize or cancel a request, and an independently admitted stop may need precedence. The defect is that provider completion timing currently supplies the implicit ordering, with no retained receipt sequence or policy-owned reordering explanation. For dependent requests such as “create the file” followed by “then add the section,” that distinction is user-visible.

Reserve a message identity and receipt sequence immediately, even while its disposition is pending. A queued record should retain origin, dependency/owner, original input, classification outcome, state and brief reference. Mangle can derive readiness and explicit priority; Go should execute that derived selection. Keep model calls out of the synchronization critical section. The simple slice can remain an implementation detail if the record and dispatcher enforce the contract; a new external queue system is not required.

## Design synthesis

A minimal contract needs four related identities, not four new services:

| Record | Minimum useful ownership |
|---|---|
| Submitted message | Message ID, receipt sequence, original text, originating execution or conversation scope |
| Foreground execution | Execution ID/generation, accepted/start/cancel-requested/settled state, cancel and terminal outcome |
| Side interpretation | Message ID, parent execution, brief base version, typed classification/Jev observations, derived disposition |
| Brief revision | Revision, originating message, applicable work scope, immutable contents, superseded revision |

The UI owner can serialize commits to its state, while kernel decisions consume stable owner-keyed facts. Background computation should return observations and proposed updates rather than mutate global conversational state on stale Model copies. Network completion order need not become dispatch order.

Reuse the existing request-owned evidence and terminal-outcome work. Do not create a parallel permission system, bypass action admission because a side message was accepted, or expand “brief update” into authorization for effects already forbidden. Changes that widen work require the existing work-mode and approval process. JIT atoms should describe any changed model-facing brief/update contract; Go should not supply a new prose planner.

An initial implementation can remain in memory. Crash/restart durability is a separate requirement; no cross-process exactly-once claim follows from adding IDs. If persistence is later required, distinguish unstarted queued requests from work whose effects may be unknown and preserve the existing effect receipts.

## Literature and runtime rationale

Three narrow references support the design, rather than establish a codeNERD performance result:

- [Bubble Tea v1.3.10][tea-batch] documents concurrent command batching without result-order guarantees. More directly, its [command dispatcher][tea-dispatch] runs separately received commands in independent goroutines before delivering their messages. The repository [pins that version][gomod]. Sequential Update handling is not proof that commands performing side effects before returning messages are serialized.
- [Go context cancellation][go-cancel] does not join the canceled operation. A terminal event or other owner acknowledgement must establish settlement before a shared slot is reused.
- [Lamport's ordering paper][lamport] distinguishes event causality from incidental arrival/clock order. The application here is modest: record receipt sequence and causal parentage so delayed model completions cannot redefine which task a control addressed. This does not require a distributed consensus system.

## Evaluation plan not executed

Existing [side-turn journey definitions][side-tests] cover one conversational update, one stop, one queued mutation, pre-work queueing and Ctrl+X. Their helper executes a side command to completion before proceeding; the Ctrl+X case verifies context cancellation and an idle flag, not predecessor settlement or successor ownership. Preserve those positive controls while adding deterministic overlap schedules.

Use recording transducers/judgment clients and channels as barriers. Do not rely on sleeps or live model timing. Keep normal TUI Update dispatch and real kernel rules in the fixture so a helper-only success cannot hide a boundary problem.

| Proposed schedule | Required observation |
|---|---|
| A completes naturally while its side stop is pending; B begins | Side stop cannot cancel B; reply accurately identifies already-ended A |
| A side result returns after B changed the brief | No stale replacement of B's brief; conflict/superseded disposition retained |
| Side A retracts then waits; side B completes with stop/later evidence | A's derived action depends only on A's admitted evidence and explicit policy context |
| Two side amendments read one brief and complete in reverse order | No silently lost constraint; chosen merge/rebase/reclassification is attributable |
| `/later` intent returns a distinct future-work goal | Deferred task cannot silently replace the active task's governing brief |
| Kernel observer reads during brief publication | One coherent old/new revision, or an explicit unavailable transition; no accidental partial brief |
| Ctrl+X with B queued; delay A cleanup until after attempted B start | Old end/status/result cannot clear B's cancel, label, busy or streaming state |
| Ctrl+X immediately after acceptance but before command entry | Accepted A is canceled or prevented from starting; no lost interrupt |
| Side B then C are received; C classifies first | Dispatch follows declared receipt/dependency/priority contract, not accidental completion order |
| Cancel only A versus cancel A and pending successors | Each message has the intended retained/canceled disposition; no silent drop or resurrection |
| Jev enabled, disabled, failed or delayed | Owner isolation holds in all cases; disabled judgments do not masquerade as negative judgments |
| Classification failure | Existing safe diagnostic/outcome contract and real queue ownership agree; no duplicate queue claim |
| Running isolated delegate receives an accepted amendment | Inspect its next actual model request and gate input; shared parent facts alone are insufficient |
| Read-only background completion during foreground cancellation | Job result remains attributable; follow-up follows the separate continuation-ownership contract |

Also cover multi-step handoffs, paused continuation, streaming and shutdown. Those paths can have different terminal message types and cannot inherit a pass from one direct delegation. Assert exactly one terminal settlement per accepted execution and no lost accepted messages; do not claim exactly-once external effects from these tests alone.

## Cost and acceptance criteria

The source does not establish a token or latency improvement. Adding IDs and state transitions has storage and bookkeeping cost; owner-keyed kernel facts may increase relation cardinality; atomic commits may reduce intermediate evaluations but need measurement. Serializing interpretation can simplify ordering while increasing time to acknowledge stop or a clarifying amendment. Version conflicts may require reclassification, and retaining pending records consumes memory.

The existing `/later` route can classify once as a side input and again when it becomes a full turn; Jev may likewise run at both stages. Do not automatically reuse old interpretation to save a call: work mode, brief, restrictions and relevant context may have changed. Compare safe reuse of immutable observations with explicit revalidation, and account for both passes.

Measure:

- Accepted messages with retained or terminal disposition, queue age, ordering violations and lost requests.
- Wrong-owner cancellations, stale terminal-state writes, mixed-message policy derivations and brief conflict outcomes.
- Time from stop receipt to cancellation request and actual settlement, separately.
- Time from a useful clarification to verified delivery in the running worker's next request.
- Logical perception/Jev calls, provider sends/retries, input/output/cache tokens, context bytes, kernel commits/evaluations and retained state.
- Time to first useful reply and correct final outcome, including p95 tail behavior and recovery turns.

Compare the current branch against ownership fencing alone, then atomic owner-scoped publication, then receipt-ordered admission with explicit priority. Keep the same work modes, model/configuration, tool authority and world-model/CodeDOM context. A small source-control fixture can establish correctness without inference; realistic normal-entry journeys are needed later to assess usefulness and cost.

Acceptance is not merely “no race detector warning.” It is: no result acts on a different owner; no old execution clears a successor; every accepted message remains accounted for; policy decisions use attributable evidence; and accepted brief changes reach the intended consumer without silent stale overwrite.

## Implementation queue

| Research ID | Deliverable | Checkable exit |
|---|---|---|
| SIDE-OWNER-01 | Owner-fenced side outcomes and cancellation | Delayed A stop cannot affect B; stale brief publication is rejected before mutation |
| SIDE-OWNER-02 | Isolated policy evidence and versioned brief commit | Interleaved calls never mix judgments; publication is coherent; conflicts are explicit |
| SIDE-OWNER-03 | Foreground accept cancel settle handshake | Pre-start cancellation is latched; old cleanup/results cannot clear newer execution state |
| SIDE-OWNER-04 | Receipt-owned queue and declared dispatch order | Out-of-order classifier completion does not silently reorder dependent requests |
| SIDE-OWNER-05 | Real consumer and lifecycle validation | Production TUI overlap/continuation journeys establish the intended behavior without weakening admission |

Start by reconciling the conversation-first, CLI and relevant core-policy specs with these ownership transitions. Preserve the branch's useful conversation-first behavior; the required change is not “make everything synchronous,” but “make asynchronous work unable to rewrite ownership.”

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/agents.md
[chat-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/Docs/architecture/conversation-first-agent.md
[side]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/side_turn.go
[side-command]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/side_turn.go#L132-L176
[side-decision]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/side_turn.go#L179-L221
[side-apply]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/side_turn.go#L224-L257
[turn-state]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/side_turn.go#L36-L129
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/internal/core/defaults/policy/side_turns.mg#L10-L35
[process-owner]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/process.go#L47-L83
[brief-publication]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/work_hold.go#L97-L136
[brief-type]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/internal/perception/brief.go#L25-L58
[transaction]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/internal/core/kernel_transactions.go#L39-L104
[ctrlx]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_key_handler.go#L56-L76
[update]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_update.go#L23-L32
[side-tests]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/journey_side_turn_test.go#L14-L139
[perception-audit]: https://github.com/theRebelliousNerd/codenerd/blob/5ff4212bf6e97347a34eec905a2eadbcd7a2897a/Docs/audits/perception-classification-outcomes-and-recovery.md
[evidence-audit]: https://github.com/theRebelliousNerd/codenerd/blob/5ff4212bf6e97347a34eec905a2eadbcd7a2897a/Docs/audits/tui-evidence-turn-ownership.md
[terminal-audit]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/Docs/audits/task-terminal-ownership.md
[settlement-audit]: https://github.com/theRebelliousNerd/codenerd/blob/5ff4212bf6e97347a34eec905a2eadbcd7a2897a/Docs/audits/delegated-turn-failure-settlement.md
[gomod]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/go.mod#L1-L12
[tea-batch]: https://pkg.go.dev/github.com/charmbracelet/bubbletea@v1.3.10#Batch
[tea-dispatch]: https://github.com/charmbracelet/bubbletea/blob/v1.3.10/tea.go#L333-L366
[go-cancel]: https://pkg.go.dev/context#CancelFunc
[lamport]: https://lamport.azurewebsites.net/pubs/time-clocks.pdf

[kernel-locks]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/internal/core/kernel_facts.go#L833-L843
[terminal-updates]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_update.go#L138-L201
[error-update]: https://github.com/theRebelliousNerd/codenerd/blob/ac2f1fd3425dab9c14ff9066f9bf10cad02acc75/cmd/nerd/chat/model_update.go#L263-L280


## As-built status

Verified and built 2026-10-28 on `main` after `c28a1212` (the feature branch is merged; `side_turn.go`, `work_hold.go`, `model_update.go` and the Ctrl+X handler were unchanged from the audited pin). Every finding was still present.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| S1: `sideTurnMsg` carries no owner; `/stop` cancels whatever cancel is stored | Real, fixed | `sideTurnMsg.Owner` captured at receipt (`side_turn.go:300`); `applySideTurn` calls `stopExecution(msg.Owner)` and reports an ended turn as already finished (`side_turn.go:418`, `:171`) |
| S1: `recordBrief` runs in the side command before the decision, unfenced | Real, fixed | Publication moved into the serialized kernel section after the decision and gated on owner, base brief and action (`decideSideTurn`, `side_turn.go:350`) |
| S2: side predicates are retracted, then a Jev wait, then assert without retract; decisions can mix | Real, fixed | Judgments are awaited first; retract, assert and query run in one transaction under `turnState.decide`, never across a model call (`side_turn.go:350`) |
| S2: whole-brief replacement retracts five predicates one by one with no version check | Real, fixed | `writeBrief` swaps in one `Transaction` (`work_hold.go:141`); side publication compares the rendered brief it was read with (content compare-and-swap) and reports a conflict instead of overwriting; `recordBrief` serializes with side sections (`work_hold.go:127`); `chatKernel` exposes `Transaction` (`model_types.go:603`) |
| S2: a `/later` message publishes its brief while its work is deferred | Real, fixed | `/later` never publishes; the full turn reads it again when it runs |
| S3: Ctrl+X sets idle and the queue drains while the canceled turn unwinds; its `end()` clears the successor | Real, fixed | `accept`/`begin`/`end`/`setWorking` are keyed by execution id (`side_turn.go:79-131`); `startQueuedTurn` requires the slot to be free (`side_turn.go:458`); Enter during wind-down queues (`model_key_handler.go:249`, `side_turn.go:279`) |
| S3: terminal messages set idle without an owner | Real, contained, not fixed at the message | A stale terminal message can still clear `isLoading`, but dispatch now reads the owned slot, so it cannot start a second foreground turn; see left open |
| S3: an interrupt before registration finds no cancel | Real, fixed | `processInput` accepts on the caller's thread (`process.go:50`); a stop before `begin` is latched and applied there; `end` is deferred first so the early guards release the slot |
| S4: side `/later` messages are queued at decision time, so classification order reorders them | Real, fixed | `reserve` at receipt, `settle` at decision; a pending head holds the queue (`side_turn.go:200-246`) |
| Stop must not silently discard queued successors | Ruled: Ctrl+X stops the running turn only; queued messages are kept and the stop notice counts them (`model_key_handler.go:75`) |
| Isolated delegate consumes an amended brief on its next request | Not verified, left open | Only the parent kernel's brief is checked |

### What was built

- Rulings: (1) one accepted foreground execution owns the slot; a newer accept supersedes an unended owner (logged), and an execution superseded or stopped before `begin` is canceled at `begin`. (2) A side turn's decision is isolated by serialization over a transactionally replaced fact set, not by owner-keyed predicates; the `.mg` rules are unchanged. (3) Brief versioning is a compare of the rendered brief, not a stored counter. (4) Queue order is receipt order; no priority policy was added. (5) Ctrl+X keeps the queue.
- `cmd/nerd/chat/side_turn.go`, `work_hold.go`, `process.go` (accept, begin, end, setWorking by id), `model_key_handler.go`, `model_types.go`.
- Spec: `Docs/architecture/conversation-first-agent.md`, "Talking while work runs".

### Tests

`cmd/nerd/chat/side_turn_ownership_test.go`: delayed stop after A ended does not cancel B; a stale side turn does not replace B's brief; a brief change during the read is not overwritten and is reported; `/later` keeps the active brief; two concurrent decisions where A waits on judgments while B decides stop yield `/converse` for A (deterministic: a context probe parks A at its wait); Ctrl+X with a queued message waits for the stopped turn, queues a newly typed message, then starts it, and a duplicate late `end` does not release the successor; a stop before start is latched; receipt order holds when the later message classifies first. The existing side-turn and classification-stop journeys pass with ids.

### Left open

- Terminal messages (`responseMsg`, `errorMsg`, ...) still carry no execution id, so a stopped turn's result can clear the spinner of a successor started by another path (`/go`, a background follow-up). Dispatch is fenced; the cosmetic flag is not. Wrapping every terminal message would change the message types many tests and handlers match on.
- The foreground turn's `recordBrief` (`process.go`) is serialized but not compare-and-swapped against the brief it perceived with; only side turns check their base.
- A message typed directly (not queued) after the foreground ends can start before an earlier side message still being classified.
- SIDE-OWNER-05 (delivery of a brief amendment into a running isolated delegate's next request) and the Section "Cost" measurements were not run.
