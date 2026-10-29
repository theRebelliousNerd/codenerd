---
doc-class: deep-dive
subsystem: cli-campaign
implementation-status: proposed
evidence-status: source-only
verification-method: static-source-inspection
last-verified: 2026-10-29
verified-against: bc610aeb9e8223823c9e84bc132de993d11199c2
supersedes: []
---

# Campaign completion needs an explicit terminal receipt

A single successful campaign started through the ordinary TUI can finish without producing the automatic completion transcript or retiring the campaign pane and its local handles. A legal schedule leaves every delivered progress snapshot short of `StatusCompleted`; the runner then returns without a terminal message, the completion event is deliberately informational at this UI boundary, and normal closure leaves the listener channels open. With that completed handle still installed, a later normal quit can also rewrite the campaign to paused.

This is a narrow production-publication addendum to the existing campaign UI ownership audit. It does not reopen the general cross-run ownership problem or claim that terminal receipts are a new idea. The proposed change is to bridge the actual `Run` result into one retained, owner-bound terminal receipt, independently of lossy progress. Preserve the previous audit's incarnation fence and the existing execution, verification, risk, and durability contracts.

## Evidence and scope

The production source is pinned to `bc610aeb9e8223823c9e84bc132de993d11199c2` in the public `theRebelliousNerd/codenerd` repository. Main still resolved to that revision at 21:20 UTC on 29 October 2026. No tests, application runs, shell commands, model inference, failure injection, or benchmarks were performed. The schedule below is a source-derived possibility, not an observed incident or an estimate of frequency.

In branch comparisons completed by 21:18 UTC, complete untruncated trees for the four sampled active code tips `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and `aa7988dec4c62116656ea722ff73b687a8509908` had the same blobs as the inspected pin for six relevant files: `cmd/nerd/chat/campaign.go`, `cmd/nerd/chat/model_update.go`, and `internal/campaign/orchestrator_execution.go`, `orchestrator_control.go`, `orchestrator_utils.go`, and `orchestrator_tasks.go`. The four additional files used for the quit extension below, `cmd/nerd/chat/model_key_handler.go`, `cmd/nerd/chat/model_lifecycle.go`, `internal/campaign/orchestrator_lifecycle.go`, and `internal/campaign/orchestrator_journal.go`, also matched at those tips. This is bounded parity evidence for this mechanism, not whole-branch equivalence or proof that no related fix exists anywhere.

The [earlier campaign UI contract][prior-contract] already proposes idempotent terminal receipts, and its [evaluation criterion][prior-evaluation] calls for reliable delivery. Its [concrete witness][prior-ui] is an A message mutating the current B slot. Here, only one accepted run is needed: a successful result may never become a terminal UI message. [Task terminal ownership][task-terminal] and [stream completion][stream-terminal] are conceptual predecessors at different worker and provider boundaries. Their ownership and evidence distinctions should be reused, not presented as a competing lifecycle.

Inventories for `research-main-20261029-2049` (`adb16e4f7e69318e3ed0a06854e04db4769783a8`), `research-main-20261029-1950` (`76d6bff4c8cb17a9da185750045bfa285afc79f9`), and `research-main-20261029-1018` (`75240b6649e9e7fce0c117ac5b03c6f4f0a95840`) were screened during this pass. The first and last inventories contained 81 and 116 top-level audit Markdown files respectively. Targeted predecessors and relevant architecture guidance were read; this is not a claim that every document received a full-text review. The screened material did not identify the exact single-run missing-terminal witness below. The contribution is its concrete production path and evaluation contract, not universal novelty.

## The production path has no authoritative final return message

Successful [campaign startup][start] creates a progress channel with capacity 100 and an event channel with capacity 200, decomposes the goal, admits the campaign with `SetCampaign`, and returns a started message. The [started-message handler][start-update] installs the campaign and both channels, clears loading, and schedules execution and event listening.

The [runner bridge][run-bridge] starts `orch.Run(ctx)` in a background goroutine, uses the shutdown context, participates in the goroutine wait group, and defers `orch.Close()`. It logs non-cancellation errors, but does not return the successful or failed `Run` result to the TUI as a terminal message. The command it returns only listens for progress.

The [progress listener][listeners] produces `campaignCompletedMsg` when it receives a snapshot whose status is completed or when its channel closes. A failed-status snapshot produces an error message. An ordinary progress snapshot is displayed and the [update handler rearms][progress-update] the listener.

The separate event listener does receive orchestrator events. However, [risk rendering][risk-render] intentionally returns empty text and a nonterminal flag for every non-risk event. The [event handler][event-update] consequently ignores `EventCampaignCompleted` and rearms. The existing [non-risk-event test][risk-test] explicitly asserts that a completion event stays silent in that renderer. This is an intentional local presentation policy whose composition with the progress-only terminal bridge is unsafe; it is not evidence that risk rendering itself should indiscriminately render all events.

When a genuine `campaignCompletedMsg` does arrive, the [completion handler][completion-update] clears the campaign references and channels, hides the pane, and appends the completion transcript. The gap is producing that message from every settled successful run, not the existence of a completion renderer.

## A legal successful schedule without a terminal receipt

Use one ordinary accepted campaign. Dependencies, planning, admission, policy validation, and risk preflight succeed. Tasks genuinely succeed; phase checkpoints pass; the kernel derives every phase done; and any declared acceptance command passes. For this witness, all relevant kernel updates, audit writes, and final snapshot and journal persistence succeed. There is no cancellation, pause, deadline expiry, second campaign, storage failure, or unverified-work shortcut.

Then schedule the final portion as follows:

1. The last successful task completes its task-state update and calls `emitProgress` while the campaign is still active. It then saves and sends its task result. [Task completion order][task-order].
2. The phase loop drains the active task results. Only when no tasks remain active and the kernel derives their completion does it run the passing checkpoint and complete the phase. Thus the successful task's progress emission precedes phase settlement and campaign completion. [Result drain and checkpoint][task-drain].
3. Any remaining pre-completion progress is consumed or queued with a nonterminal campaign status. Arrange the heartbeat so it never samples completed status: for example, its last emission occurs before the completed transition and no further heartbeat tick is selected before shutdown of that loop. This does not require the entire campaign to fit inside one heartbeat interval.
4. The main loop reaches the successful acceptance branch, performs its final observation and root-write sweep, sets `StatusCompleted`, persists the campaign successfully, emits `EventCampaignCompleted`, and returns nil. There is no unconditional final `emitProgress` in this branch. [Successful completion][success].
5. The deferred execution cleanup cancels and joins the heartbeat and pause watcher. The heartbeat's progress production is periodic, so the chosen schedule supplies no completed-status sample. [Heartbeat lifetime][heartbeat-lifetime]; [periodic emission][heartbeat].
6. The TUI runner closes the orchestrator. [`Close`][close] releases observer and ownership resources; it expressly does not stop execution and does not close the progress or event channels.
7. The event listener receives the authentic completion event, but its handler treats it as non-risk information and rearms. The progress listener drains any remaining nonterminal snapshots and rearms. Both channels remain open with no remaining producer required to publish a terminal observation.

This witness does not need a full queue or a dropped event. Even successful delivery of the completion event does not invoke the completion handler. It also does not depend on alleging a Go data race.

There is an important rescuing schedule: if a heartbeat samples completed status after the transition and successfully sends it before the heartbeat is joined, the progress listener can produce the terminal UI message. Heartbeat timing therefore explains why this is a possible missing receipt, not an assertion that every campaign exhibits it. The [nonblocking publishers][publish] can drop a progress or event send when its channel is full, so a coincident heartbeat is not an unconditional guarantee either.

## What remains correct and visible

The witness does not leave the core campaign running. The successful branch records completed status; [status publication][status] refreshes the canonical kernel fact and records a completion audit event. The successful-persistence precondition preserves the final snapshot and journal. The normal execution defer clears `isRunning`. Absence of a TUI receipt is not absence of a successful execution result.

Completion can also remain visible without the expected terminal transcript. [Manual status rendering][manual-status] reads the shared campaign's status, and the [sidebar renderer][panel] does so too. A subsequent render, including one prompted by the completion event itself, can therefore show completed status even while the pane remains installed. Old progress-specific display data may remain until another update; the narrow claim is missed automatic terminal notification and retirement, not that the operator can never discover completion or that every visible field must say active.

Ordinary chat is not necessarily blocked: startup already cleared loading. Existing [live controls][live-controls] consult `GetProgress`, require matching campaign identity, and check status before pause or resume. Do not infer that a retained TUI handle makes a completed campaign controllable as running.

The [hard-risk path][event-update] already renders a delivered blocking event and stands the campaign UI down. Preserve that behavior. This memo does not establish false verification, a false `/done`, bypassed acceptance, permission or safety bypass, incorrect task execution, or a broken campaign ownership lock. The successful completion state is preserved at this point in the witness; the later quit extension below changes that state.

Durability is separate from successful execution. [`persistCampaign`][persist] records snapshot-write failure, emits a failure event, and places a storage hold; it does not return that failure through the current nil-returning success branch. The successful-persistence witness avoids claiming a new storage defect. A future terminal receipt must nevertheless carry actual durability evidence rather than treating nil from `Run` as proof that every write committed.

## A later normal quit can rewrite completed status

Extend the same single-run schedule only after `Run` and its deferred `Close` have finished. No replacement campaign is installed, no other shutdown has already consumed `shutdownOnce`, and the missing receipt leaves `m.campaignOrch` pointing to this completed orchestrator. The user then presses Ctrl+C or Ctrl+D.

The [global exit handler][quit-key] calls `performShutdown`, whose [wrapper][shutdown-wrapper] invokes `Shutdown`. Despite its comment saying “if running,” the [campaign shutdown branch][shutdown] checks only whether the handle is non-nil, then calls `Stop`. Current [`Stop`][stop] unconditionally sets any loaded campaign to `StatusPaused` and calls `saveCampaign`; it does not first preserve a completed or failed status.

Consequently, this legal extension changes the completed in-memory status to paused and attempts the canonical kernel refresh. With a successful stop-time save, it writes a paused snapshot and journal record through the [ordinary persistence path][save]. The previously recorded completion audit event is not erased. Initial completion persistence and stop-time persistence both succeed in this witness: this is a later lifecycle rewrite, not a failed disk write. It requires no second campaign and does not establish that any task will be dispatched again after restart.

A received completion message is an important control: its handler clears `campaignOrch`, so this particular shutdown branch cannot call `Stop` through the retired handle. This consequence strengthens the need to retire a settled run, but teardown must also preserve terminal status independently of UI message timing. Resource release is not a reason to manufacture a paused execution outcome.

## Bridge the final result separately from progress

Use the accepted run's existing owner and settlement boundary. Do not introduce another scheduler or a second lifecycle authority.

A suitable terminal record has an immutable accepted-run incarnation, logical campaign ID, terminal execution outcome and error classification, final status and bounded snapshot or durable snapshot reference, durability state, and cleanup state. These are proposed fields, not existing declarations. Logical ID alone is insufficient when the same campaign is resumed or accepted again; retain the [prior UI audit's incarnation fence][prior-contract].

The production bridge should naturally return a terminal message from a `tea.Cmd` that owns execution and settlement, while progress and event commands run alongside it. If the existing tracked goroutine is retained, its owner must publish into a dedicated retained terminal slot, and a command must return that record. In either form, do not make successful publication depend on a future periodic progress tick.

Required behavior is:

- Join owned writers before freezing the final snapshot and publishing one terminal outcome for the accepted run. Preserve actual error, cancellation, pause, blocked, and durability-unknown states; do not convert them into unqualified success.
- Publish terminal evidence through a dedicated non-droppable path, such as an acceptance-reserved one-shot result slot plus retained owner record. Progress may be coalesced or dropped; terminal settlement may not share that policy.
- Make UI application idempotent. Validate the incarnation before changing the current pane or handles, and retain a stale run's outcome under its own owner rather than applying it to a replacement or discarding it.
- Let the worker settle even when the UI has quit or stopped consuming. Do not replace a dropped terminal with an unbounded blocking send. Retain the outcome through the appropriate existing result or durable-record mechanism, and bound notification and cleanup resources.
- Distinguish cleanup timeout from proof of termination. A join that has not established writer quiescence cannot justify a final immutable success snapshot. Retain uncertainty and the necessary owner record instead of silently forgetting live work.
- Make teardown status-preserving for already completed or failed runs, even if their UI receipt is delayed. An actual stop of an active run may still establish paused status; mere release of a settled run's resources may not.
- Keep reporting and cleanup within existing authorization. No extra LLM call is required or justified to infer that a run ended.

A one-shot result channel alone is not crash durability, and a channel close alone is not an execution verdict. The receipt should preserve references to existing verified outcomes and persistence evidence, not invent a competing storage log.

## Why smaller apparent fixes are insufficient

| Option | Useful change | Remaining limitation |
|---|---|---|
| Handle `EventCampaignCompleted` in the TUI | Closes the demonstrated path when that event is delivered | Current event publication is best effort and can drop. Identity, immutable final evidence, error outcomes, and retained delivery still need a contract. |
| Add one final `emitProgress` | Makes a completed snapshot more likely to reach the current listener | The send can drop. It also races with UI departure or replacement and still uses a progress channel as terminal authority. |
| Increase heartbeat frequency or buffer size | May reduce some delays or drops | Neither proves that a completed sample is produced and consumed before teardown. More polling is not a correctness guarantee. |
| Defer `Stop` on success | Closes the channels | Current [`Stop`][stop] writes paused status and saves before closing. It would rewrite successful completion and is not neutral cleanup. |
| Return and retain the actual settled result | Supplies a terminal source independent of progress timing | Requires owner-aware application, bounded retention, explicit durability semantics, and shutdown handling. |

Handling a reliable terminal event could be an equivalent implementation if it gains those guarantees. The proposal is a contract, not a requirement for a particular new channel type.

## Costs and evaluation to run

The owner check and terminal metadata are O(1) per message. A one-shot receipt adds O(1) state per accepted run, but snapshot copying scales with the bounded snapshot size and total retention scales with retained runs. Specify retention limits, eviction eligibility, and recovery access; do not call an unbounded campaign copy constant-size.

Measure terminal-notification latency from actual settlement, copying and allocation, queue occupancy, waiting goroutines, shutdown and cleanup latency, retained bytes, duplicate UI effects, and human recovery time. A reliable result path can move work from incidental heartbeat handling into explicit settlement; its end-to-end cost has not been measured. Preventing uncertainty-driven reruns may save work, but this audit does not claim a measured token or latency improvement.

Compare the current bridge, completion-event handling alone, and an explicit retained terminal bridge under the same model, plan, policy, verification obligations, checkpoints, tools, authorization, and execution budget. The mechanism itself must add zero model calls. If later end-to-end evaluation uses model inference, report total input, output, reasoning where available, cache reads and writes, failed and canceled attempts, retries, wall time, and monetary cost under matched budgets. Do not credit hidden retries or discarded attempts as efficiency.

Proposed schedule-controlled cases are:

1. **No completed heartbeat.** Exercise normal campaign command dispatch, the real start/run bridge, listener commands, and `Update`. Arrange successful final work and persistence but no heartbeat sample after completed status. Require one owner-correct terminal transcript and pane retirement without another user command.
2. **Heartbeat rescue.** Deliver a genuine completed-status heartbeat before join. The explicit bridge must not duplicate the transcript or cleanup when its own receipt arrives.
3. **Queue saturation.** Fill progress and event queues, including at terminal publication. Separate terminal retention must survive best-effort drops without deadlocking the worker.
4. **Done before consumption.** After the UI accepts startup and launches execution, let the run finish before it consumes the first progress or terminal result. Terminal application must remain ordered by owner and idempotent, not timing luck.
5. **Duplicates and stale owners.** Replay terminal observations, delay A until B becomes current, and reuse the same logical campaign ID with a different incarnation. Preserve the existing ownership contract; do not clear B or erase A's result.
6. **Real non-success outcomes.** Exercise genuine risk block, pause, error, cancellation, failed checkpoint or acceptance, and final snapshot or journal failure. Preserve their distinctions and never render a generic successful completion because channels closed or `Run` returned nil without durability evidence.
7. **Application quit.** Exercise the complete missing-receipt schedule, then normal Ctrl+C or Ctrl+D after `Run` and `Close` finish. With healthy storage, require normal quit not to rewrite completed status; if teardown persists anything, it must preserve that status. Compare received-receipt then quit, and verify that an actual active-run stop still records pause. Also quit during execution and after settlement but before terminal consumption; establish bounded cleanup and retained evidence without requiring a departed UI to drain a channel.

Use deterministic adapters only to control admissible planning results, clocks, and scheduling; keep the production route under test. The existing [frame contract test][frame-test] manually injects `campaignCompletedMsg`; that proves only what the handler does when given a message, not that the runner produces one. The existing non-risk silence test is useful evidence of the current contract, not a delivery test.

The acceptance criterion is one truthful, retained terminal outcome per accepted run, correct owner-specific UI application, no dependency on a completed heartbeat, no unsafe success on error or uncertain durability, and no additional model inference. These tests and comparisons are proposed and unrun.

[prior-ui]: https://github.com/theRebelliousNerd/codenerd/blob/76d6bff4c8cb17a9da185750045bfa285afc79f9/Docs/audits/campaign-ui-run-ownership.md#L11-L15
[prior-contract]: https://github.com/theRebelliousNerd/codenerd/blob/76d6bff4c8cb17a9da185750045bfa285afc79f9/Docs/audits/campaign-ui-run-ownership.md#L75-L89
[prior-evaluation]: https://github.com/theRebelliousNerd/codenerd/blob/76d6bff4c8cb17a9da185750045bfa285afc79f9/Docs/audits/campaign-ui-run-ownership.md#L115-L115
[task-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/Docs/audits/task-terminal-ownership.md#L11-L29
[stream-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/Docs/audits/stream-completion-contract.md#L11-L23
[start]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/campaign.go#L32-L88
[start-update]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/model_update.go#L312-L327
[run-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/campaign.go#L91-L143
[listeners]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/campaign.go#L148-L192
[progress-update]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/model_update.go#L330-L346
[risk-render]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/campaign.go#L196-L218
[event-update]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/model_update.go#L349-L369
[risk-test]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/campaign_risk_test.go#L146-L150
[completion-update]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/model_update.go#L379-L388
[task-order]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_tasks.go#L461-L470
[task-drain]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_tasks.go#L141-L239
[success]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_execution.go#L181-L243
[heartbeat-lifetime]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_execution.go#L111-L127
[heartbeat]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_execution.go#L323-L336
[close]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_control.go#L74-L98
[publish]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_utils.go#L120-L152
[status]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_utils.go#L155-L195
[manual-status]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/campaign.go#L332-L383
[panel]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/campaign.go#L440-L472
[live-controls]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/commands_handlers.go#L715-L789
[persist]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_lifecycle.go#L113-L139
[stop]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_control.go#L47-L71
[quit-key]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/model_key_handler.go#L20-L26
[shutdown-wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/model_lifecycle.go#L111-L118
[shutdown]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/model_lifecycle.go#L21-L44
[save]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/campaign/orchestrator_lifecycle.go#L63-L110
[frame-test]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/tui_frame_contract_e2e_test.go#L968-L982

