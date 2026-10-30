---
doc-class: deep-dive
subsystem: campaign-storage-admission
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: source-inspection-only
supersedes: []
---

# Carry task-completion save failures into the campaign storage hold

**Decision:** apply the existing storage-health publication contract at the successful task-completion save boundary. Keep a completed task completed; report that its progress was not durably saved, and let the existing kernel storage hold prevent new dispatch until persistence recovers.

**Evidence:** static source review at [main `388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), including independent review. No tests, application execution, inference, training, local Git or repository writes were performed. The event sequence below is a conditional, unexecuted fixture.

## Narrow finding

`runSingleTask` marks a successful task complete and then calls `o.saveCampaign()` directly, ignoring its returned error. The low-level writer logs failures, but it does not publish `campaign_storage_uncertain` or the operator-facing snapshot-failure event. Those belong to `persistCampaign`, which this completion path does not call. The worker sends a successful task result after the failed save. [Completion call site][completion], [writer and wrapper][writer]

Consequently, a same-phase dependent task can become eligible and be dispatched **after this known save failure**, provided no autosave or other wrapped persistence attempt has intervened and no independent admission gate blocks it. The kernel is not being bypassed: it correctly answers from the completion state it received, without the missing storage-health observation. [Scheduler][scheduler], [dependency and storage policy][policy]

This is a targeted coverage continuation of an existing contract, not a new generic recovery architecture, a campaign terminal-publication issue, or proof of a reproduced data-loss incident. It concerns new work admission between a failed completion save and the next storage-health publication.

## Credit the existing repair

The earlier hardening audit replaced nine discarded save calls with `persistCampaign` so the failure names its checkpoint and emits `EventSnapshotWriteFailed`. Its listed sites include task **failure**, autosave, phase completion and campaign completion, but not this successful task-completion caller. [Hardening B4][b4]

The campaign generation/recovery audit subsequently records the added kernel storage hold: a failed snapshot write should hold new dispatch as a wait; a landed write should release it. The campaign wiring specification describes that feature as built. The implementation and its test substantiate the wrapper path, and this proposal preserves it. The remaining caller uses the lower-level writer instead. [Prior recovery audit, closing note][prior], [current wiring contract][spec], [storage-hold test][hold-test]

The main audit corpus, relevant QA and research inventories through `research-main-20261030-1619` were screened for this boundary. This is a source-confirmed residual of the existing storage-hold work, not a claim that storage failures were previously undiscovered. The latest research branch is documentation-only. All five active non-research heads inspected retain the same direct `o.saveCampaign()` after `applyLearnings`; whole-file equivalence is not asserted.

| Inspected active head | Completion-save observation |
|---|---|
| `51cfd42649552866d4aa54c53b68acf5740eb09b` — QA orchestrator | Raw save; returned error discarded |
| `a53cfc308e8836c72370adaa46deba3f7b00180d` — store lane | Same |
| `80af1863d3a3aa46a722ac8b0c8a0985582d4995` — session lane | Same |
| `aa7988dec4c62116656ea722ff73b687a8509908` — OAuth lane | Same |
| `767a3291d6287b238417f2466592a9002810d05a` — QA cartographer | Same |

## Real production chain

1. **TUI ownership.** Normal campaign construction supplies the live task executor, host executor and workspace. The owned background run invokes the orchestrator. No special parser-only or dormant entry point is required. [Construction][construction], [run startup][chat]
2. **Actual successful task.** `runSingleTask` first calls `executeTaskWithRollback`. A mutating task has already owed its execution, micro-checkpoint and applicable modification validation; this witness requires those to succeed. It does not substitute model prose for completion. [Worker][worker], [execution wrapper][task-wrapper]
3. **Live completion publication.** `completeTask` updates the task's status to completed, increments the campaign count and records its result/context. The status reaches the kernel through `updateTaskStatus`; the witness requires these publications to succeed. [Status and completion][status]
4. **Dropped persistence outcome.** The same worker emits task-completed/progress information, takes the campaign mutex, calls the raw writer and ignores the error, unlocks, and sends a successful `taskResult`. [Completion boundary][completion]
5. **Eligibility consumer.** The phase loop drains that result, removes the worker from its active map and asks the kernel for eligible tasks. The dependency rule no longer blocks a dependent of the now-completed task. The storage rule would withhold it if `campaign_storage_uncertain` had been asserted; this caller has not asserted it. [Scheduler][scheduler], [policy][policy]
6. **Dispatch.** The scheduler still acquires the write-set lease, checks task status, updates the dependent to in-progress and launches its worker. `getEligibleTasks` is kernel-authoritative and fails closed on query errors; no old in-memory fallback is needed for this sequence. [Dispatch][dispatch], [kernel-authoritative consumer][eligibility]

The substantiated consequence is another task's admission despite a known persistence failure, not inevitable execution of its tools. Its own authority, snapshot, validation, cancellation and other task gates can still refuse effects.

## Conditional witness

Use an ordinary campaign with a live phase containing two valid tasks, **A → B**, where B has a hard dependency on A:

- A is admitted and completes genuinely, including its applicable task checks.
- B is pending, has no other unmet dependency, retry backoff, risk block or write conflict, and has valid declared work.
- Kernel queries and completion assertions are healthy. There is no pre-existing storage hold.
- The completion save fails at a persistence operation, while the workspace and other services needed to admit B remain available.
- No successful autosave, failed wrapped autosave, phase-boundary save, cancellation or manual pause intervenes before B's scheduling decision.
- For a clean serial fixture, use a supported single-task concurrency setting and ordinary file tasks, avoiding the research/test concurrency boost. No additional concurrent campaign is needed.

A precise failure choice is a journal-open error on A's completion save, after campaign startup and prior saves succeeded. That returns before a new snapshot is written; the previous committed generation remains the available snapshot. The source has an `osOpenJournal` seam suitable for a future controlled test. A live failure of a campaign-specific persistence path is the production condition; no particular filesystem fault was induced here. [Writer ordering][writer], [journal open boundary][journal]

| Event | Live state and observation |
|---|---|
| A finishes successfully | A is completed in campaign memory/kernel; B's dependency can clear |
| A's completion save returns an error | Writer logs the cause; this caller publishes no storage-hold fact |
| A's result reaches the phase loop | A leaves the active map; its result still represents successful execution |
| B is considered | Healthy kernel can derive B eligible; no storage uncertainty was supplied |
| B is dispatched | More work is admitted before a later persistence attempt repairs or publishes the storage condition |

The same-phase pending B matters: the phase is not yet done, so this does not rely on crossing a phase or campaign terminal boundary. The schedule is conditional; a timely autosave can eliminate it.

A crash is **not required** to establish the admission gap. If a crash does occur before a later save lands, existing recovery admission will judge the available journal/snapshot evidence. Do not infer that recovery necessarily replays A, duplicates a particular external effect or permanently loses all progress. Those depend on where the failure occurred, later writes and the recovery class. This memo stops at the missing runtime hold.

## Safeguards and competing paths

- **Failure is logged.** `saveCampaign` reports directory, serialization, journal, snapshot and commit-journal errors. This is not wholly silent. The missing pieces are the completion-boundary storage fact and the wrapper's operator-facing event, not every diagnostic.
- **Autosave is real.** The run starts an owned heartbeat/autosave worker. Autosave calls `persistCampaign`; it can publish a hold if storage remains unavailable or save the current state and release a hold after recovery. The default autosave interval is one minute, not an immediate post-completion handshake. A heartbeat progress update alone is not a save. [Autosave][autosave], [defaults][defaults]
- **Existing hold works when supplied.** The policy withholds `eligible_task` while preserving waiting work, so disk trouble need not falsely become “all tasks blocked.” Already-running tasks may finish and pause/cancel remain available. [Policy][policy]
- **Journal integrity is separate.** Uncertain appends are reconciled before another append. An open failure does not mark `journalUncertain` because it wrote no bytes. In either case, that field governs append recovery, not task eligibility, and it does not substitute for the omitted storage fact. Preserve the existing journal checks and atomic snapshot protocol. [Journal reconciliation][journal], [snapshot writer][snapshot]
- **Completion is not undone.** The task's successful effects and observed result remain real. Turning a save failure into a synthetic worker failure could trigger retries or compensation of already-completed work; that is not the proposed repair.
- **Other gates remain authoritative.** A held dependency, missing state, failed query, lease conflict, unavailable snapshot, permission denial or cancellation can prevent B independently. No authorization bypass is claimed.
- **No permanent unbounded-run claim.** A later wrapped save, explicit pause/cancel or independent failure can stop or recover the path. The finding requires a particular interval between completion and the next relevant persistence event.
- **Publication itself can fail.** `holdForStorage` logs if it cannot assert the fact. That is a separate uncertainty requiring an explicit safe policy, not proof that invoking the wrapper always guarantees a held scheduler under arbitrary kernel failure. The main witness uses a healthy kernel. [Hold publisher][hold]

## Proposed contract

### Make the existing observation unavoidable at continued-execution boundaries

Every persistence attempt after which a campaign may continue scheduling must settle through one shared storage-outcome publication contract:

- On failure, retain the live successful work, identify the checkpoint, emit bounded failure diagnostics and publish the uncertainty observation before the owner treats that continuation boundary as settled.
- On successful committed persistence, publish recovery/release consistently, without claiming that a failed write which may have landed was certainly absent.
- Keep low-level writer errors available to admission paths that already return them and stop. Do not naively wrap the writer inside itself or require every legitimate raw writer caller to use a void wrapper.
- Classify call sites explicitly: constructor/resume admission that returns failure, continued execution requiring health publication, and cleanup whose outcome is retained separately.

For the primary call site, reuse the existing locked `persistCampaign` behavior with a task-completion checkpoint identity, or give the common persistence operation a typed outcome consumed by the owner. The narrow fix should not add a parallel Go scheduler or reinterpret task success as failure. Mangle continues to decide eligibility from the published storage state.

### Be precise about the boundary being guaranteed

The first exit criterion is **no new admission after this completion save is known to have failed and before storage recovery**, under the healthy-kernel conditions above. It is not an automatic claim of durable-before-dispatch ordering for every concurrent completion.

A stronger protocol in which all dependent dispatch waits for a committed generation needs its own publication ordering, concurrent-worker and already-admitted-work contract. Preserve that distinction rather than labeling a one-call-site correction a full durability transaction.

Hold publication failure must remain visible and receive a deliberate owner-level disposition; never report “held” merely because a helper was invoked. Recovery should not enqueue the same completed task again solely to make its bookkeeping durable.

### Preserve evidence and observability

Separate execution success, persistence success and campaign completion in receipts. A useful record carries campaign/task/checkpoint identity, attempted generation/checksum, last known committed generation, failure stage, uncertainty class and whether the hold was actually published. Do not log file contents, credentials or unbounded task output to diagnose a storage error.

Retain recovery-class validation, owner locks, torn-journal preservation and atomic replacement. The existing snapshot/journal remains the durable authority; this continuation does not require a second persistence system.

## Alternatives, costs and failure modes

| Approach | Benefit | Cost or limitation |
|---|---|---|
| Current raw save and eventual autosave | No additional implementation | Leaves a window where a known failure is absent from scheduling policy |
| Route completion through the existing wrapper | Smallest direct coverage repair; reuses current event and policy | Requires call-site coverage and hold-publication error accounting; not full concurrent commit ordering |
| Typed persistence receipt for every continued-execution boundary | Stronger observability and enforceable integration contract | More API/caller changes, event/schema compatibility and lifecycle tests |
| Fail/retry the completed task on save error | Superficially stops its normal result | Can replay or undo real successful effects; not equivalent to a storage wait |
| Pause every completion until a committed generation | Stronger durability boundary | Potential throughput/lock-latency cost and broader scheduling redesign; evaluate separately |

The minimal route does not inherently add a model call, embedding request or extra snapshot write: the completion caller already performs that write. It adds/centralizes event emission, hold facts, kernel derivation and release handling on failure/recovery. Measure mutex duration, storage IO, event volume, derivation cost, scheduling latency and recovery time. On a storage outage, reduced task throughput is intended containment, not a performance improvement.

Model/tool spend may be avoided because work waits rather than extending an undurable run. That benefit is unmeasured and must be weighed against legitimate-work delay, false holds, recovery retries, human intervention and any additional retained diagnostic artifacts. Cap repeated error reporting without erasing distinct failed generations or claiming a suppressed log means storage recovered.

## Fair evaluation — not run

Extend the existing storage-hold test with the real **successful task-completion caller**, not only a direct `persistCampaign("autosave")` invocation. Start with a valid two-task A → B plan and healthy real kernel. Use deterministic fault injection after startup and a barrier around the completion save and B's admission; do not depend on wall-clock sleeps or model misbehavior.

| Control | Required observation |
|---|---|
| Successful A completion save | B remains eligible under normal policy; no unnecessary hold |
| Completion journal-open failure, healthy kernel | Failure logged and published; B not newly dispatched before recovery |
| Snapshot creation/write/rename failure after requested journal event | Correct uncertainty class and hold; existing recovery evidence retained |
| Commit-journal failure after snapshot replacement | Do not claim the snapshot is absent; hold and later reconcile |
| Persistent outage until autosave | Wait, not false campaign failure; no new mutation admission |
| Recovery save lands | Hold released and B resumes without replaying A |
| Autosave intervenes before B | Both old and repaired paths credited for their actual mitigation |
| B already running when failure occurs | Preserve declared already-owned-work behavior; no fictional rollback |
| Cancellation/pause during hold | Owner joins workers and retains recoverable state; no deadlock |
| Kernel rejects hold assertion | Explicit publication failure, not an unsupported “dispatch held” claim |
| Constructor/resume raw-save failure | Existing error-return admission remains fail-closed |
| Query errors, write conflicts, backoff, risk/permission denial | Independent gates retain their existing behavior |

Compare current main, the minimal shared-wrapper route and any stronger generation handshake on identical task outcomes, permissions, storage fault point and concurrency. Measure the ordering of save failure, hold publication, task result and B's dispatch; live versus durable task state; false holds; new work admitted during uncertainty; retry counts; kernel/IO overhead; model/tool usage and operator time. A later separately authorized crash/restart evaluation can assess recovery consequences, but it is not needed to prove or refute this narrow runtime seam.

**Acceptance boundary:** successful task effects remain successful, but a failed completion save cannot disappear between the writer's error and the kernel's new-work admission decision. Restore coverage of the existing storage-hold contract rather than replacing it.

[completion]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L464-L477
[writer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_lifecycle.go#L62-L140
[scheduler]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L141-L167
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L250-L321
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_tasks.mg#L19-L128
[b4]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/2026-09-09-system-hardening/03-STUBS-AND-SWALLOWED-ERRORS.md#L307-L331
[prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-generation-admission-and-recovery.md#L159-L172
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/campaign/WIRING-AND-NOT-BUILT.md#L204-L206
[hold-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/storage_hold_test.go#L9-L50
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_planning.go#L84-L139
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign.go#L90-L153
[worker]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L415-L477
[task-wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L52-L108
[status]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L625-L691
[eligibility]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_phases.go#L67-L106
[journal]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_journal.go#L35-L141
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_journal.go#L165-L211
[autosave]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_execution.go#L319-L381
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/campaign.go#L174-L186
[hold]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/storage_hold.go#L8-L42

