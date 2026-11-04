---
doc-class: deep-dive
subsystem: cli-recurse-lifecycle
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: e91820914b6ab744c8f061c603248c83839912c1
verification-method: static-source-and-test-definition-inspection
supersedes: []
---

# Own TUI recurse through shutdown

## Finding and scope

**The interactive `/recurse` loop is not owned by the shutdown path that the global exit keys invoke.** Its context starts from `context.Background()`; its attempt orchestrator remains local to the loop; and neither the loop nor its event-drain goroutine is enrolled in the model's background wait group. `Model.Shutdown` cancels the separate shutdown context, stops the ordinary campaign handle, waits for registered work, and closes shared resources. None of those steps establishes that this recurse loop has received cancellation or settled. [Recurse start and run][recurse-start], [attempt runner][attempt-runner], [shutdown][shutdown].

The associated progress sink is part of the same lifecycle boundary. It performs unconditional sends to a bounded UI channel. If UI consumption stops and that channel fills, the loop or its attempt event drain can block before returning and releasing its resources. Merely adding a call to the recurse cancel function would therefore not complete the shutdown repair. [Progress writer][writer], [event-drain join][attempt-runner].

This is a conditional static integration finding, not a reproduced hang, data-loss incident, post-exit commit, provider overrun, or measured reliability regression. The witness requires an admitted, still-live loop when the interactive exit key is processed. It does not apply merely because a user typed `/recurse`, and it does not claim that already-settled loops keep running. The process can still terminate: termination of the process is different from completion of the loop's cooperative cancellation and compensation protocol.

Public main was rechecked at approximately 05:18 UTC on November 4, 2026 and remained [`e91820914b6ab744c8f061c603248c83839912c1`][revision]. All codeNERD repository links below pin that revision. This review inspected public guidance, architecture, source, related audits and existing test definitions. No repository code, tests, model inference, benchmarks or failure injection ran. No implementation or repository change is included.

## The intended boundary already exists

The recurse journey describes one shared CLI/chat improvement loop, with owner cancellation as its stopping authority. The chat adapter explicitly describes local stop and pause as cancellation of an in-flight attempt. The shared driver has a corresponding cancellation path: after the executor returns, it checks its context and attempts ownership-aware compensation before returning. These are useful existing mechanisms to connect, not reasons to create a second loop or replace the ratchet. [Journey][journey], [local stop][local-stop], [driver contract][driver], [cancellation compensation][compensation].

The model's shutdown comment promises graceful background shutdown before `tea.Quit`. The ordinary campaign bridge already derives its run context from `m.shutdownCtx` when available and registers its execution with `m.goroutineWg`. The recurse adapter uses a different bridge and does neither. This comparison establishes a concrete missing integration, rather than a general preference for a different concurrency style. [Model shutdown][shutdown], [ordinary campaign bridge][managed-run].

The Go context contract matters at this boundary: cancellation propagates from parent to child, and invoking a cancel function does not wait for its work to end. A child of `Background` is not a child of the model's shutdown context. The proposed repair consequently needs both cancellation ownership and a separately observed completion boundary. [Go context documentation][go-context].

## Source-supported exit witness

Use a healthy interactive session and a Git workspace that passes recurse admission. Required services are present; configuration parsing, workspace preparation and relevant policy checks succeed. There is actual admitted work, not just an attempted start. No ordinary managed campaign needs to exist, no second recurse loop is needed, and no other failure or cancellation has already ended this loop.

1. **Install a live recurse run.** `startRecurseCampaign` creates a cancelable context rooted at `Background`, places the cancel function and a 64-line channel in `recurseState`, and returns a started message. The message handler installs that state and schedules the run plus a one-line listener. The state has no join/completion handle. [Start][recurse-start], [state][state], [started-message handling][messages].
2. **Enter an attempt.** The execute callback creates an attempt campaign and a fresh local orchestrator, then calls `orch.Run(ctx)`. It defers orchestrator closure, joins a local event-drain goroutine, and releases attempt resources as it returns. It does not install this orchestrator in `m.campaignOrch`. The factory uses shared session services and explicitly assigns orchestrator ownership to its caller. [Attempt runner][attempt-runner], [factory][factory].
3. **Process an ordinary global exit key while the loop remains live.** `handleKeyMsg` handles Ctrl+C and Ctrl+D before view-specific interception. It synchronously calls `performShutdown` and returns `tea.Quit`. The witness concerns this key-event route, not every possible operating-system signal or abrupt kill. [Exit keys][exit-keys], [shutdown wrapper][shutdown-wrapper].
4. **Observe what shutdown owns.** Shutdown cancels `m.shutdownCtx`, optionally calls `m.campaignOrch.Stop()`, and waits for `m.goroutineWg` with its existing timeout. Recurse's independently rooted context does not become canceled through that root, its local orchestrator is not the ordinary campaign handle, and its run is not in that wait group. Shutdown contains no separate recurse cancellation or join. It then closes the Cortex or legacy shard manager. [Shutdown][shutdown].
5. **Keep outcome claims conditional.** The recurse context can remain live as shared services are shut down. Depending on where execution is, dependency shutdown may cause a later error, a step may finish, or process exit may interrupt settlement. The source proves the missing cancellation/join relationship; it does not prove which subsequent effect wins a particular schedule. The driver's explicit mid-attempt compensation branch requires `ctx.Err() != nil` after executor return, which the model-root cancellation alone does not establish for this context. [Compensation][compensation], [resource teardown][cortex-close].

The UI framework does not supply the missing join. The repository pins Bubble Tea v1.3.10. That release launches commands in separate goroutines and deliberately does not wait for individual commands during shutdown. Its command API also does not automatically inject the model's context into a command closure. The closure must carry and settle its own execution lifetime. [Pinned dependency][go-mod], [Bubble Tea command execution][tea-commands].

The practical risk is loss of the graceful-stop boundary for a loop authorized to execute checks, edit workspace files and adopt verified changes. The user can leave the UI without this adapter establishing that its loop has stopped or that the current attempt has reached a recorded disposition. Other component shutdown hooks can reduce exposure; they do not prove completion of the recurse driver and its own cleanup.

## Why progress must not own settlement

`listenRecurseLines` receives one line and returns one UI message. The next receiver is scheduled by handling that message. While the exit-key handler is synchronously inside `Shutdown`, that normal rearming path is not running; after the UI exits it does not resume. A listener already issued can consume one more line, so a precise fixture must account for that receive instead of assuming zero remaining capacity. [Listener][listener], [rearming][messages], [exit keys][exit-keys].

For the bounded blocking witness, let all currently issued receives complete, with their UI messages not yet handled. Fill the remaining channel capacity with valid nonblank progress lines, then make one further progress write. The writer holds its mutex while sending and has no cancellation alternative. The extra write cannot finish without another receive. A larger channel moves this point; it does not remove the dependency. The witness is a legal scheduling condition, not an assertion that every ordinary quit fills the channel. [Writer][writer].

Two source paths make the blockage relevant to settlement:

- The driver writes progress synchronously. Its attempt-start message follows the in-flight record and precedes executor invocation; its result message precedes removal of the in-flight record. Blocking either send can keep the driver from reaching its next transition or returning through its deferred lock release. An in-flight record in these cases does not itself demonstrate an uncommitted workspace mutation. [Synchronous log writer][driver-log], [attempt start][attempt-start], [result publication][result-publication], [driver lock][driver].
- The attempt's event goroutine writes completed/failed task messages through the same writer. If it is blocked inside that write, closing its local `done` channel cannot make it select the stop case. The execute callback then waits on `drained` while unwinding. Until the callback returns, the outer driver's post-executor cancellation/compensation branch cannot run. This remains a problem even if the model begins canceling the recurse context. [Event drain and join][attempt-runner], [compensation][compensation].

This is not a proposal to make durable journal acknowledgement lossy. Conversational progress is a projection; the journal, attempt settlement and owned-write compensation carry different responsibilities. The Go project's pipeline guidance documents the same general requirement: a producer must be able to stop when its downstream consumer leaves. It does not validate this particular implementation. [Go pipeline cancellation][go-pipelines].

## Existing safeguards and prior work

Preserve these controls and distinctions:

- **Explicit local stop works differently from exit.** `stopRecurse` calls the loop's own cancel function. The test definitions cover local stop and campaign pause, and the shared driver contains compensation logic. That does not qualify the missing exit wiring, but it is important counterevidence against claiming that recurse cannot be canceled at all. [Local stop][local-stop], [existing control tests][control-tests].
- **Automatic self-restart is not the same witness.** Rebuild handling requests a recurse boundary stop; restart eligibility treats a non-nil recurse state as busy. This report does not claim that the supported restart path deliberately cuts a live attempt. Keep its boundary-stop/resume semantics separate from immediate local cancellation. [Restart protocol][restart].
- **Other shutdown owners exist.** Cortex close stops and joins several kinds of work and stops shards before releasing later resources. The finding is not that the entire application lacks shutdown control, nor that every underlying worker necessarily ignores exit. The missing owner is the recurse run and its local drain. [Cortex close][cortex-close].
- **Recovery is separate from graceful exit.** Retaining in-flight intent and later recovery does not prove that the current process canceled and joined this run. Do not broaden this report into the already audited filesystem-compensation or crash-recovery contracts.
- **Related audits do not establish this boundary.** Campaign UI run ownership concerns which run may update a UI slot. Campaign/recurse scheduling ownership concerns admission into a shared kernel. Campaign terminal publication concerns the ordinary campaign's final result. Their witnesses differ from a single live recurse run on global exit. The compensation audit explicitly excludes shutdown cancellation from its subject. [UI ownership audit][ui-audit], [scheduling audit][scheduling-audit], [terminal audit][terminal-audit], [compensation audit][compensation-audit].

The audit inventory, targeted source/document searches, CLI architecture and the above adjacent reports did not identify a dedicated current-main report of this recurse exit-and-progress dependency. This is a bounded novelty assessment, not proof that no issue, branch or private discussion has ever mentioned it.

## Proposed contract and bounded repair

Keep the repair in the existing chat adapter and shutdown integration. It should add no model decision, second orchestrator, new inference call or independent improvement loop.

1. **Admit under the session lifetime.** Establish run ownership before launching executable work; use a context descended from the model's shutdown lifetime. Refuse or settle a start that races shutdown. Account for the pending-start interval as well as already installed state.
2. **Cancel and join the actual run.** Retain a completion signal independent of delivery of `recurseFinishedMsg`. On global exit, stop further admission, request cancellation of the owned loop, and wait for executor return, owned compensation or an explicit unresolved disposition, local drains and attempt cleanup before releasing their dependencies.
3. **Separate progress from completion.** A missing UI receiver must not prevent cancellation or settlement. Choose an explicit bounded policy for conversational lines: cancellation-aware sends, coalescing/dropping with an omission indicator, or a separately owned drain. Preserve reliable terminal and recovery evidence outside a lossy display queue. Do not close a channel from shutdown while producers can still send.
4. **Name incomplete cleanup honestly.** Cancellation request, execution completion and successful compensation are distinct states. If cleanup cannot settle within the product's approved shutdown allowance, preserve the evidence needed for recovery and report the unresolved owner. A timeout is not proof that effects ended, and a warning followed by resource closure must not be called a clean join.
5. **Retain command semantics.** Explicit local stop still cancels; automatic restart still waits for an attempt boundary and retains resume arguments; ordinary campaign shutdown remains independently owned. Repeated shutdown must remain safe.

Replacing only `Background` with `shutdownCtx` fixes propagation but not joining. Calling only `m.recurse.cancel()` has the same limitation. Adding only a wait group can wait on a producer whose UI consumer is already unavailable. Increasing the buffer, skipping deferred cleanup, or unconditionally closing the progress channel is not a settlement contract.

## Evaluation to run, not results

Use deterministic barriers and fake execution/teardown hooks first. Avoid live providers, actual user workspaces and real executable replacement for the core witness. Inspect actual context state and execution completion, not just a mocked cancel-function count or disappearance of the UI pane.

| Proposed case | Required observations |
|---|---|
| Ctrl+C and Ctrl+D during one admitted attempt | Actual run context is canceled; no next attempt is admitted; completion/compensation disposition is established before dependent resource closure. |
| Exit while recurse start is pending | Pending work cannot publish a live unowned run after shutdown; any admitted work has a completion owner. |
| Full progress channel with no rearmed consumer | A blocked loop write and a blocked event-drain write each settle under cancellation; join does not require another UI update. |
| Partially buffered lines and one already-issued receive | Correct stop behavior without assuming an empty/full queue or a particular command scheduling order. |
| Executor returns owned writes on cancellation | Compensation follows the existing ownership checks; unrelated user changes survive; journal/in-flight disposition agrees with the actual outcome. |
| Executor or cleanup refuses to settle | Shutdown reports the exact unresolved work under an explicit bounded policy; it does not manufacture successful compensation or silently equate timeout with completion. |
| Last result line or partial-line flush at exit | Progress does not block terminal evidence, in-flight disposition or resource release. |
| Normal completion and explicit local stop | Existing result summary, local cancellation and release behavior remain intact. |
| Automatic rebuild/restart and platform control | Existing attempt-boundary stop and resume behavior remains; unsupported automatic restart stays unsupported. |
| Repeated shutdown, ordinary campaign, no active recurse | Idempotence and other lifetime owners remain correct. |

The present adapter tests exercise local stop, message streaming and a small writer input that fits in its buffer. They do not establish global-exit cancellation, join-before-close ordering or cancellation under full-channel backpressure. Those observations come from reading the definitions, not running them. [Existing tests][control-tests], [writer test][writer-test].

Compare the current bridge, context-only propagation, and the full ownership-plus-progress repair under identical fake work, output volumes, schedules, budgets and cleanup obligations. Record cancellation-to-run-return latency, compensation duration, teardown ordering, blocked goroutines, peak queued bytes, dropped/coalesced progress, terminal evidence, and CPU/memory overhead. The mechanism should add zero inference calls. If a later authorized live evaluation is useful, hold task/model/gate budgets constant and include all failed and canceled work in token, wall-time and monetary accounting. Do not credit skipping compensation, dropping durable records or abandoning unresolved work as a speedup.

Expected implementation costs are bounded owner state, cancellation-aware progress handling and explicit lifecycle tests. Joining can increase visible exit time because work formerly abandoned must now settle; cleanup may involve filesystem and storage IO. Coalescing trades display detail for bounded memory, whereas retaining every line requires a separately managed sink and storage policy. No latency, token, safety or reliability gain has been measured.

**Exit criterion:** global interactive exit demonstrably cancels the owned recurse run, establishes its terminal or explicitly unresolved disposition, and cannot have that settlement depend on continued UI progress consumption. The source changes and the above execution evidence would both be needed before this report could be marked fixed.

## Pinned sources

[revision]: https://github.com/theRebelliousNerd/codenerd/commit/e91820914b6ab744c8f061c603248c83839912c1
[state]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_recurse.go#L30-L38
[recurse-start]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_recurse.go#L157-L224
[attempt-runner]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_recurse.go#L227-L265
[writer]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_recurse.go#L297-L326
[listener]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_recurse.go#L280-L295
[messages]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_recurse.go#L368-L398
[local-stop]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_recurse.go#L328-L339
[shutdown]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/model_lifecycle.go#L18-L103
[shutdown-wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/model_lifecycle.go#L111-L118
[exit-keys]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/model_key_handler.go#L18-L34
[managed-run]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign.go#L151-L209
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_planning.go#L79-L140
[driver]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/internal/campaign/recurse_cycle.go#L353-L390
[driver-log]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/internal/campaign/recurse_cycle.go#L478-L483
[attempt-start]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/internal/campaign/recurse_cycle.go#L1226-L1288
[compensation]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/internal/campaign/recurse_cycle.go#L1288-L1322
[result-publication]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/internal/campaign/recurse_cycle.go#L1548-L1572
[cortex-close]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/internal/system/cortex_close.go#L48-L128
[restart]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/self_restart.go#L100-L166
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/Docs/journeys/10-forever-loop.md#L1-L14
[control-tests]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_recurse_test.go#L66-L127
[writer-test]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/cmd/nerd/chat/campaign_recurse_test.go#L129-L146
[go-mod]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/go.mod#L1-L15
[ui-audit]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/Docs/audits/campaign-ui-run-ownership.md
[scheduling-audit]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/Docs/audits/campaign-recurse-scheduling-ownership.md
[terminal-audit]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/Docs/audits/campaign-terminal-publication.md
[compensation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/e91820914b6ab744c8f061c603248c83839912c1/Docs/audits/recurse-compensation-write-ownership.md
[go-context]: https://pkg.go.dev/context
[go-pipelines]: https://go.dev/blog/pipelines
[tea-commands]: https://github.com/charmbracelet/bubbletea/blob/v1.3.10/tea.go#L331-L371

