---
doc-class: deep-dive
subsystem: recurse-control-lifecycle
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 4230878370369ad15a32658e622761d156ec29c6
verification-method: static-source-and-test-definition-inspection
supersedes: []
---

# Preserve external recurse stop through startup

## Finding and decision

**A successfully submitted external stop for a visibly running recurse owner can be erased as a stale request during that same owner's startup.** The shared driver acquires the lock that status interprets as running, optionally waits for mission quiescence, and then unconditionally removes the workspace's stop file. The external stop command writes that file without a run identity. Nothing distinguishes an old leftover from a new request submitted after the current owner acquired its lock. [Startup ordering][startup], [running and request protocol][control], [external command][cli-stop].

Mission startup supplies a concrete production interval for this ordering, rather than requiring an unexplained delay between two adjacent instructions. A TUI mission joins the existing Cortex's active knowledge refresh before admission. While that join is pending, the recurse lock is already held and the external file-based stop remains available. If the refresh then settles and startup succeeds, the new stop file is deleted before the first ordinary boundary check. [Chat mission wiring][chat-mission], [mission admission][mission-begin], [host join][quiescence].

The recommendation is to make request retirement and run admission one owner-aware control protocol. A request submitted for the current owner must either remain pending until that owner consumes it or produce an explicit failed/stale-target response. Prior-run cleanup must not delete it. Keep the existing distinction between a graceful external boundary stop and local immediate context cancellation.

This is a conditional source-composed correctness finding, not a reproduced incident, measured stop-loss rate, demonstrated unauthorized edit, or claim that all stops fail. Public main was resolved at 05:37 UTC on November 4, 2026 to [`4230878370369ad15a32658e622761d156ec29c6`][revision] and rechecked unchanged at 05:45 UTC. All codeNERD links below pin that revision. This review read guidance, architecture, source, relevant prior reports and test definitions. No application execution, builds, tests, gates, model inference, benchmarks or failure injection were performed.

## Intended behavior and current control protocol

The recurse journey makes owner stopping part of its unattended-loop contract. The CLI describes the external command as ending the loop after the current attempt is judged, while Ctrl+C cancels the attempt. The helper likewise describes its request as applying at the next boundary. A startup with no attempt yet in flight should not require the owner to discover that a successful stop submission was discarded and send it again. [Journey][journey], [CLI contract][cli-contract], [request helper][control].

The implementation has three distinct observations:

1. **Owner present:** `acquireRecurseLock` obtains the OS lock and writes a PID. `recurseRunning` tries that lock and reports true when another holder prevents acquisition.
2. **Request present:** `RequestRecurseStop` writes `stop\n` to a fixed workspace path. `recurseStopRequested` checks only whether that path exists.
3. **Request consumed:** `boundary` sees the file, removes it, appends `stepStopped`, and returns `ErrRecurseStopped`.

The lock and the request identify the workspace, but the stop file carries no owner generation, request ID, publication sequence, or state distinguishing startup from an older run. The PID is not read by the stop helper and is not a request binding. [Control implementation][control], [boundary consumer][boundary].

The CLI reports that the request was submitted after the write succeeds; that is **not an acknowledgement from the owner that it has stopped**. The finding does not conflate the two. It concerns a successfully published request that the same live owner's subsequent cleanup removes without consuming. The chat external-stop adapter makes the same window visible: it first checks `Running`, then writes the same file. The primary witness can use the simpler CLI request and does not require a second TUI. [CLI stop][cli-stop], [chat external stop][chat-stop].

The normal owning TUI's `stopRecurse` is different: it invokes its retained cancel function. Deleting a file cannot undo that context cancellation. This report is about the external file-based route, not a regression in that local stop method. [Local stop][local-stop].

## Production trace and conditional witness

Use a healthy existing TUI with a valid, clean Git workspace and a Cortex that has a knowledge refresher. No prior stop file, unsettled mission, in-flight attempt or competing recurse owner is required. All workspace, configuration, session, kernel and later Git/journal operations succeed. The owner context remains live; no local stop, exit key, signal or deadline independently ends the run.

Start a valid `/recurse --mission "Improve this workspace"` invocation. Require an already active refresh whose cancellation cleanup has not yet completed when mission startup calls the suspension hook. This is an explicit scheduling condition, not a claim that every TUI has such a refresh or that cancellation normally takes a long time.

1. **Enter the shared driver.** The chat adapter constructs `RecurseMission` with `m.cortex.SuspendKnowledgeRefresh` and calls `RunRecurseCycles`. That driver acquires the workspace recurse lock before calling `Mission.begin`. A separate status reader can now report `Running: true`. No mission attempt has been dispatched. [Chat adapter][chat-mission], [startup][startup], [status][control].
2. **Wait at the real quiescence boundary.** `begin` validates configuration and prior state, then calls the hook. The host increments its pause count, cancels the active refresh, and waits for the refresh's completion channel or the supplied context. This is required joining, not a missing quiescence implementation. [Mission begin][mission-begin], [host suspension][quiescence].
3. **Submit an external stop while that join is pending.** In another shell, target the exact same workspace with the supported `nerd campaign recurse stop` command. Require `RequestRecurseStop` to return nil and its file to exist. The CLI prints its normal request confirmation. The request was created after this run's lock acquisition and while this same owner remains live. [CLI request][cli-stop], [file producer][control].
4. **Let the refresh settle.** The hook returns its lease. Mission startup records its state, obtains its resource admission, and then creates its timeout context. Writing the stop file has not canceled the supplied context, so it does not force the host's context-canceled branch or the mission's `ctx.Err()` check. This is also why the witness needs the refresh to eventually settle rather than assuming an indefinite hang. [Host join][quiescence], [mission continuation][mission-begin].
5. **Erase the fresh request.** After `begin` returns, the driver reaches the unconditional `os.Remove(recurseStopPath(root))`, whose comment classifies any file there as left over from an ended run. The just-created file is the file removed. Startup does not first inspect or record it as a stop for the current owner. [Stale cleanup][startup].
6. **Pass the first boundary without this request.** Given successful remaining recovery/Git/journal initialization and positive sweep admission, `passes` calls `boundary` before `pass`. With the context live, mission admission allowed and no second stop submission, the absent file makes `boundary` return nil. A first pass can then derive its graph and start its initial runnable workspace gates. Later model work additionally requires the ordinary graph, finding, mission-design and execution admissions; this report does not assume those succeed. [First boundary][passes], [initial pass measurement][pass-measure].

The minimal decisive observation is a lost control request: **held current lock → successful new stop publication → current-owner startup deletion → first boundary does not return `ErrRecurseStopped` because of that request**. A deterministic test can observe passage into the first baseline measurement without invoking a model or making an edit.

A mission has its own finite envelope. The finding does not claim that this particular mission runs forever, exceeds its enforced limits, or necessarily commits anything. Ordinary non-mission recurse contains the same lock-before-cleanup ordering with a smaller scheduling window and can run without a pass bound. The wider mission interval makes the shared protocol defect easier to qualify; it is not a claim that CLI mission boot always has an active refresher. The CLI's deliberate pre-boot suppression is a counterexample to that particular long-window setup. [Mission specification][mission-spec], [shared startup][startup].

## Existing safeguards and bounded novelty

Preserve the controls that already work:

- The recurse lock excludes a second recurse owner on the same workspace. The witness uses one owner and a supported control client, not two competing writers.
- A request submitted after startup cleanup is left for a later boundary. The existing stop fixture submits from inside the executor, verifies the attempt is judged, and then requires `ErrRecurseStopped`.
- Local context cancellation reaches mission admission and quiescence. The host refuses mission admission when that context is canceled while waiting. File publication alone does not invoke that cancellation.
- The knowledge-refresh suspension has a real cancellation-and-join protocol, nested lease ownership and admission closure. Its test already demonstrates a refresh that has observed cancellation but has not yet completed. This report does not weaken that join to make startup faster.
- A startup error, already-canceled context, expired mission, later stop request or ordinary terminal condition can prevent further work. Those outcomes do not prove that the first submitted request was honored.

The inspected lock and control tests establish exclusion and mid-attempt stopping, not a request submitted after lock acquisition but before stale cleanup. The refresh test establishes that its join waits for completion, not that recurse preserves an external stop across that join. These are valuable components whose composition needs a new fixture; their definitions were read, not executed. [Control tests][control-tests], [refresh test barrier][refresh-tests].

The current audit inventory and queue, relevant report bodies, and targeted searches for the stop-path producer/consumer and startup stop behavior did not identify this dedicated witness. This is a bounded main-branch novelty screen, not a claim about every branch or private discussion. [Audit queue][queue].

- **TUI shutdown ownership** concerns global exit failing to cancel/join a live loop and progress backpressure. Here the UI stays open, no progress channel needs to fill, and a real external request is created then erased before an attempt.
- **Compensation write ownership** concerns which effects a canceled/refused attempt may restore. This witness precedes executor dispatch and requires no workspace mutation or contested write.
- **Recovery attempt identity** concerns joining a durable in-flight cycle to Git history. Here no earlier in-flight cycle or commit match is needed.
- **Mission recovery receipt census** concerns admission past unresolved historical receipts. This witness uses clean history and a new owner control request.
- **Campaign/recurse scheduling ownership** concerns competing execution owners. The single-owner witness needs no shared-kernel scheduling interference.

Those reports remain separate contracts; this report neither reopens their historical source claims nor implies their proposed repairs have all shipped. [Shutdown audit][prior-shutdown], [compensation audit][prior-compensation], [recovery audit][prior-recovery], [mission census audit][prior-census], [scheduling audit][prior-scheduling].

## Proposed contract and bounded design choices

Update the owning recurse/campaign lifecycle specification before implementation.

**Required invariant:** once a control client successfully submits a stop for a published current owner, that request cannot be retired as prior-run debris. It must remain associated with that owner until consumed, superseded by an explicit owner action, or reported as an unfulfilled/failed submission. Define startup as part of the controllable lifecycle rather than silently exposing a running lock while treating new requests as stale.

There are two reasonable bounded designs to evaluate:

1. **Conservative stop persistence.** Treat a stop file found after acquiring ownership as a pending request and stop before entering a new pass. An explicit restart procedure can clear old requests under a coordinated ownership transition. This is small containment, but changes the current automatic-stale-clear behavior and must be disclosed: a genuinely leftover request can make a fresh start stop immediately. Do not silently erase it merely to preserve restart convenience.
2. **Owner-addressed requests.** Publish a unique run generation and lifecycle state under a defined control synchronization point. A control client resolves that owner and publishes a request addressed to its generation; startup retires only records proven to belong to previous generations. Check that same generation at boundaries and retain a consumed/settled disposition. An owner transition during submission must produce a retryable stale-target/not-running result or a specified binding, never an unqualified success for an unidentified owner.

The generation must identify a run, not merely a PID that can be reused. Mission IDs alone do not cover ordinary recurse and currently appear after the quiescence interval. A request record's successful filesystem write is not owner acknowledgement; status should distinguish requested, consumed and settled where the protocol supports them. The local direct-cancel route and the external graceful boundary route need not become identical.

**Do not just move the unconditional removal earlier.** Moving it before quiescence shortens this particular window but leaves the interval after lock acquisition and before removal. Moving it before the owner lock can let a losing second starter delete the active owner's stop. Correctness requires a stated admission/retirement ordering or a conservative persistence policy.

Preserve Mangle's existing sweep and mission decisions. Go should supply exact owner/request/lifecycle observations and act on explicit owner interrupts; it should not invent a replacement task scheduler or use the model to decide whether to honor a stop. No new model, judge or embedding request is required.

Do not trade quiescence for responsiveness by returning before the refresh has settled. Go explicitly distinguishes signaling cancellation from waiting for completion. If a future design makes startup stops interrupt that wait, it must still retain the lease and join/settle already admitted work before releasing ownership. The minimal fix need only preserve and consume the request at the first supported boundary; bounded stop latency during a stuck dependency is a separate requirement. [Go cancellation contract](https://pkg.go.dev/context#CancelFunc), [current join][quiescence].

## Costs, failure modes and unrun evaluation

A conservative persistence fix has low machinery cost but can require explicit owner action after an old stop. An owner-addressed protocol adds state serialization, atomic publication, validation, control synchronization, compatibility for existing stop files and recovery across crashes. It increases local filesystem IO and retained metadata; qualification must include the repository's Windows and non-Windows lock implementations. No latency, reliability or token-saving improvement has been measured. [Lock implementations][unix-lock], [Windows lock][windows-lock].

Important failures to specify are: a control client races an owner transition; a request write fails or is partial; ownership metadata cannot be read; an owner crashes after publication or consumption; multiple stop clients submit; a stale request survives restart; cleanup fails; a context cancellation overlaps a file stop; and a graceful stop is mistakenly implemented as immediate rollback. Go documents that `WriteFile` is a sequence of system calls that can fail after a partial write, so a structured successor record needs an explicit publication/validation rule rather than assuming a JSON write is indivisible. [Go file-write contract](https://pkg.go.dev/os#WriteFile).

Proposed evaluations, **all unrun**:

1. **Deterministic shared-driver regression.** Use a valid temporary Git workspace, real kernel policy, valid mission admission and a quiescence callback parked on explicit entered/release barriers. After entered, confirm the owner lock is held, submit the stop through `RequestRecurseStop`, observe success and file presence, then release quiescence. Record request retirement, first boundary, first gate/executor dispatch, journal and final outcome. Current-source expectation is request removal and no stop at that boundary; corrected behavior must consume the stop before new pass work.
2. **Production-composition fixture.** Exercise the normal TUI mission adapter and actual Cortex suspension with a scripted local acquisition source, using the existing cancellation-observed/finish barrier pattern. Submit from the external CLI command against the same canonical workspace. Keep the UI alive and draining. Use no live provider. This establishes the adapter wiring that a callback-only test cannot.
3. **Ordering matrix.** Cover requests before ownership, after ownership but before cleanup, during quiescence, after cleanup but before first pass, during an attempt, during verdict/compensation, and after terminal settlement. State which requests bind to which owner. Preserve the existing mid-attempt test's “judge, then stop” outcome.
4. **Ownership and restart controls.** Include no-request startup, a genuinely stale request, losing second starter, two concurrent request clients, old-generation request versus a new owner, ordinary non-mission recurse, CLI mission boot suppression, nil/failed quiescence, and local direct cancellation. A design that always refuses startup must not count as a correct fix.
5. **Failure and portability controls.** Qualify supported Windows/POSIX paths and lock behavior, same-workspace identity, malformed/partial records, write/removal/read errors, and owner crash boundaries. Do not treat a same-process fixture as proof of cross-process filesystem ordering.

Report source revision, owner/request identities, lock acquisition/publication/retirement order, command return and actual terminal disposition, gates/attempts started after request, any workspace effects, and correctness of preserved mid-attempt settlement. Measure request-to-consumption and request-to-settlement latency separately, startup overhead, filesystem operations/bytes, lock wait, CPU/memory, and any extra gate/model work. Include refused and failed cases; a fast command return is not a successful stop.

**Acceptance:** a successfully submitted request for the current startup owner survives quiescence and prevents new pass work at the first supported boundary, while ordinary startup, explicit restart, mid-attempt graceful stopping, local cancellation and single-owner exclusion retain their specified behavior. Runtime reproduction, incidence, platform coverage and performance remain unestablished.

## Pinned sources

[revision]: https://github.com/theRebelliousNerd/codenerd/commit/4230878370369ad15a32658e622761d156ec29c6
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/campaign/recurse_cycle.go#L384-L450
[control]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/campaign/recurse_control.go#L28-L113
[boundary]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/campaign/recurse_cycle.go#L453-L475
[passes]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/campaign/recurse_cycle.go#L485-L513
[pass-measure]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/campaign/recurse_cycle.go#L515-L575
[chat-mission]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/cmd/nerd/chat/campaign_recurse.go#L157-L224
[mission-begin]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/campaign/recurse_mission.go#L99-L171
[quiescence]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/system/knowledge_refresh_scope.go#L86-L144
[cli-stop]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/cmd/nerd/cmd_campaign_recurse.go#L95-L125
[cli-contract]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/cmd/nerd/cmd_campaign_recurse.go#L51-L91
[local-stop]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/cmd/nerd/chat/campaign_recurse.go#L328-L339
[chat-stop]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/cmd/nerd/chat/campaign_recurse.go#L342-L356
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/Docs/journeys/10-forever-loop.md#L178-L223
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L86-L96
[control-tests]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/campaign/recurse_control_test.go#L11-L109
[refresh-tests]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/system/knowledge_refresh_scope_test.go#L30-L163
[unix-lock]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/campaign/recurse_lock_other.go
[windows-lock]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/internal/campaign/recurse_lock_windows.go
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/Docs/audits/ASTRA_QUEUE.md
[prior-shutdown]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/Docs/audits/recurse-tui-shutdown-ownership.md
[prior-compensation]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/Docs/audits/recurse-compensation-write-ownership.md
[prior-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/Docs/audits/recurse-recovery-attempt-identity.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/Docs/audits/recurse-mission-recovery-receipt-census.md
[prior-scheduling]: https://github.com/theRebelliousNerd/codenerd/blob/4230878370369ad15a32658e622761d156ec29c6/Docs/audits/campaign-recurse-scheduling-ownership.md

