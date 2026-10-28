---
doc-class: deep-dive
subsystem: session
implementation-status: partial
last-verified: 2026-10-28
verified-against: 8cd6e1f8804709996b46e43641a3ffe464d2088a
verification-method: static-source-inspection
supersedes: []
---

# A canceled waiter must not own the worker's last cleanup

**Decision:** make terminal publication and heavy-worker retirement an obligation of the task owner, independent of whether a caller remains to poll. Couple that with a latched cancellation/start handshake. Keep the existing rule that live agents cannot be removed, and keep task completion evidence distinct from worker termination.

**Evidence status:** source inspection and proposed design. No implementation, tests, inference, training, profiling, failure injection or live task execution was performed. The sequences below are source-derived witnesses to evaluate, not reproduced incidents. Statements are pinned to the header revision, not a claim about later main.

Main freshness: compared the original inspection pin `4947e87e8d6a8d41e5cc7af5a71ec0675831dce2` with main at 03:46 UTC on 2026-10-28. The six newer commits did not change the cited lifecycle implementation or guidance; the document is repinned to the resolved main head above.

## 1. Capability and scope

Long-running coding sessions must be able to cancel a delegated task, preserve its actual partial outcome, and continue without accumulating abandoned worker objects. The operator stopping a wait does not make a worker disappear, nor prove that it made no changes. Reliable task ownership reduces avoidable retained state and makes retry decisions better informed without granting new authority.

The [repository North Star](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/agents.md#north-star) puts executive decisions in Mangle and demands bounded context with progress-driven execution. [Session contributor guidance](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/agents.md) explicitly requires ownership for Spawner reservations, SubAgent state, persistence and teardown. This proposal closes one lifecycle boundary rather than introducing a second orchestration policy.

The existing [session feature-card reconciliation](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/Docs/architecture/session/TODO.md#wave-2-reconciliation-2026-09-25-verified-against-the-code) correctly credits the common production stack and declines a generic lifecycle manifest without a consumer. It also declines replacing polling merely on unmeasured contention. Neither should be revived as busywork. The concrete consumer here is the lifecycle owner that must settle an accepted task when a result reader leaves.

The September hardening report already recognized that `StopAll` cancels rather than joins and can miss an agent before Running ([historical open items, “Still open: the Windows-only mock panic”](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/Docs/audits/2026-09-09-system-hardening/05-OPEN-ITEMS.md#still-open-the-windows-only-mock-panic)). Those historical experiments were not rerun here. The new refinement is a precise owner/publication/retention contract around the current JITExecutor result cache, including the path on which its existing bound never applies.

This is distinct from [campaign generation recovery](campaign-generation-admission-and-recovery.md), which concerns durable campaign snapshots/journals, and [tool-effect receipts](tool-effect-receipts-and-validation-completeness.md), which concerns evidence of individual effects. Terminal task settlement should preserve and reference that evidence, not duplicate its storage or infer it from prose.

## 2. Current mechanisms that should survive

| Boundary | Inspected source | Current protection |
|---|---|---|
| Production wiring | `initFinalExecutors`, [internal/system/factory.go:2592–2650](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/system/factory.go#L2592-L2650) | Normal boot creates the Spawner and JITExecutor and connects task delegation. This is not an unwired test-only executor. |
| Admission reservation | `Spawn`, [internal/session/spawner.go:352–389](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/spawner.go#L352-L389) | Reserves capacity before expensive config generation; canceled compilation does not silently degrade into a new empty-config agent. |
| Registration before start | `executeAsyncInternal`, [internal/session/task_executor.go:432–468](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/task_executor.go#L432-L468) | Creates a result placeholder before starting the agent, avoiding a fast completion being overwritten by initialization. |
| Single start | `Run`, [internal/session/subagent.go:230–243](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/subagent.go#L230-L243) | Atomic Idle→Running transition prevents a second Run from executing the same instance. |
| Measured return preserved | `execute`, [internal/session/subagent.go:281–325](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/subagent.go#L281-L325) | Captures structured observations while the ExecutionResult is available, separately from final response text. |
| Worker-local retirement | `Run`, [internal/session/subagent.go:257–278](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/subagent.go#L257-L278) | Retires working scopes after execution, stores result/error and then exposes terminal state. Do not claim every archive survives cancellation. |
| Normal retrieval reaps | `getObserved`, [internal/session/task_executor.go:482–530](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/task_executor.go#L482-L530) | A terminal read copies the observed outcome into the result cache, then removes the terminal Spawner entry. |
| Bounded completed-result cache | `cacheCompletedResult`, [internal/session/task_executor.go:538–559](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/task_executor.go#L538-L559), bound at `:268–272` | Keeps at most 256 completed entries in FIFO order, does not evict in-flight placeholders, and avoids duplicate eviction slots. This is an in-memory cache, not crash-durable storage despite nearby “durable home” wording. |
| Live workers cannot be forgotten | `Remove`, [internal/session/spawner.go:563–585](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/spawner.go#L563-L585) | Only terminal agents are removed. Do not “fix” retention by deleting running agents. |

Existing test definitions cover duplicate starts, completed-cache bounds and normal result reaping ([internal/session/session_uplift_test.go:28–72,110–200](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/session_uplift_test.go#L28-L200)). Their existence is evidence of intended contracts, not a claim this study ran them.

## 3. Finding T1: retirement depends on a future reader

`executeAsyncInternal` installs `TaskResult{Completed:false}`. The worker's `Run` closure writes its own state, but does not publish into the JITExecutor cache. Publication and removal occur when `getObserved` sees terminal state. `waitObserved`'s cancellation branch instead calls `Spawner.Stop`, then returns `ctx.Err()` immediately ([task_executor.go:568–599](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/task_executor.go#L568-L599)). That branch does not join the worker or arrange a later terminal read.

**Conditional sequence:** a task is running; its waiter cancels; the stop signal is delivered; the waiter returns; the worker eventually reaches Failed or Completed; nobody polls its ID again. The Spawner can retain the terminal SubAgent and its executor/history/observed return, while JITExecutor still labels its separate cache entry incomplete. Because `cacheCompletedResult` was never called, the task is absent from `completedOrder` and the 256-completed bound does not retire it. Unretrieved successful async tasks have the same ownership problem without cancellation.

This is not a claim that all canceled tasks continue consuming model tokens. A cooperative worker can stop correctly and still be retained. Conversely, a provider/tool ignoring cancellation may remain running; that is a different state and cannot be called reaped merely because `Stop` returned nil.

`countActive` excludes terminal agents but traverses the entire Spawner map ([spawner.go:614–623](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/spawner.go#L614-L623)). Consequently retained terminals need not exhaust active capacity, but they can grow retained memory and the work of later registry scans. No retained-byte amount or latency impact was measured.

Calling `Cleanup` blindly is not an equivalent fix. It removes terminal agents without transferring their results, potentially leaving an incomplete result placeholder whose worker is gone. The current code comments explain why normal retrieval first preserves the return. The correct repair makes that transfer independent of polling.

## 4. Finding T2: stop intent is not latched before cancellation installation

`executeAsyncInternal` starts `go agent.Run(...)`, even though `Run` itself starts the task goroutine. Before `Run` runs, the registered agent is Idle. Inside `Run`, the atomic transition to Running precedes installing `s.cancel`. `Stop` only reads and invokes the current cancellation function; if it is nil, Stop returns nil without retaining stop intent ([subagent.go:230–243,343–355](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/subagent.go#L230-L355)). `StopAll` also filters out non-Running agents ([spawner.go:526–539](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/session/spawner.go#L526-L539)).

**Proposed witness:** pause startup after registry publication but before `Run`, request `Stop(taskID)`, then allow startup with a still-live execution context. The current stop request is not remembered. Another seam pauses after the Running transition but before cancellation installation. A corrected contract should either prevent execution or expose a cancel-requested state consumed before dispatch.

The context qualification matters. The normal synchronous helper passes the same context into spawn and wait; if that context is canceled, the child inherits its cancellation even if this particular Stop signal is missed. The sharper witness uses explicit Stop or a separately canceled waiting context while the execution context remains live. This is a lifecycle/API gap, not evidence of a current unauthorized tool effect or a safety-gate bypass.

## 5. Proposed lifecycle contract

All names below are design notation, not current APIs or schema declarations.

```text
TaskHandle:
  task_id, owner_incarnation, request_digest
  execution_context_owner, admission_reservation
  lifecycle: admitted | starting | running | terminal_pending_publication | terminal
  cancel_requested, cancel_reason, cancel_requested_at
  immutable_terminal_result, done_signal
  delivery_state, result_retention_state

TerminalResult:
  task_id, attempt_id, worker_status, stop_reason
  observed_return, effect_receipt_references
  verification_status, outcome_knownness
  started_at, finished_at
```

Worker status and task verification are different axes. A worker returning normally does not itself prove acceptance criteria. Cancellation may leave artifact changes, failed checks or unknown external effects. Preserve the existing structured observation and its qualifiers; do not synthesize “no effect” or “task complete” from a canceled wait.

### 5.1 Admission and start handshake

1. Reserve bounded capacity and construct the owned task handle. Establish the task cancellation function and done signal before the task ID becomes externally usable. If setup fails, release the reservation exactly once.
2. Register the result slot and worker under one owner protocol before starting actual execution. Preserve the existing register-before-fast-completion protection.
3. Latch explicit cancellation in the handle regardless of whether execution has started. Start checks the latched request and caller context at the state transition; a prestart cancellation produces an explicit terminal-not-started result.
4. Use one start transition and one execution goroutine. Calling `Run` synchronously when it only schedules work can eliminate an unnecessary scheduling window, but it is not a substitute for a correctly synchronized cancel latch.
5. Keep admission accounting, runtime config, authority and policy gates unchanged. A task handle is not permission to execute tools.

### 5.2 Worker-owned settlement

The task owner, not the waiter, must arrange exactly one finalization attempt for every accepted task. A worker completion callback or an owner-managed join watcher can perform this transfer; neither requires a busy background sweep.

```text
on actual worker termination:
  capture immutable result and measured partial outcome
  finish worker-owned cleanup; retain evidence of cleanup failure
  publish terminal result into owner registry/cache under version check
  remove heavy terminal worker only after result publication is established
  release active reservation and temporary references exactly once
  signal terminal availability to all waiters
```

Keep callbacks outside worker/registry locks; define lock order and duplicate-completion behavior. A publication failure must retain a bounded recoverable owner record or backpressure admission, not silently drop the result or replay the task. Finalization after a canceled caller needs an owner cleanup context with its own bounded resource contract, not an unbounded `context.Background` escape. A timeout reports incomplete cleanup; it is not evidence that owned work has ended.

The existing in-memory result cache can remain the minimal publication destination. Do not require a new database merely to fix this lifecycle. If cross-process recoverability is desired later, add a separately designed durable record and refer to effect receipts; an in-memory done channel does not supply crash recovery.

### 5.3 Waiting, canceling and retention

Separate two concepts in the interface: “stop waiting” and “cancel this task.” Preserve existing API behavior through an explicitly cancel-on-wait-cancellation adapter if compatibility requires it. New multi-subscriber observation should not let one viewer's timeout accidentally cancel work another owner still needs. This is a proposed API refinement, not a claim current callers intended detach semantics.

The done signal is published after the terminal snapshot is visible. Readers get immutable copies or owned views with current read authorization. Terminal polling can remain supported; replacing the 100 ms timer is not the reason for this work. It becomes an optional latency/CPU optimization once completion already has a signal.

Use both count and byte budgets for terminal retention, plus explicit disposition for unread results: retained, delivered, expired, or archived-reference. Never evict genuinely running work under the completed-result policy. Never preserve an expired terminal as an eternal in-flight placeholder. If retention pressure cannot be handled without losing a required outcome, apply backpressure or authorized archival rather than retaining unlimited worker graphs. Tombstones or stable expired-result errors should prevent a late reader from mistaking expiry for “still running.”

Go owns synchronization, cancellation observation, actual cleanup and publication. Mangle consumes typed host lifecycle facts to decide continuation, allowed retry and unresolved obligations. A model cannot assert its own completion or erase a cancel request. This design does not move executive task policy into Go or authorize automatic retries.

## 6. Shutdown and noncooperative dependencies

This is not a whole-application teardown redesign. The narrow owner should nevertheless expose admission closure and a joinable set of accepted tasks so its caller can stop new work, request cancellation, then know which work actually terminated. Idle admitted tasks and in-progress construction must be accounted for, not just `ListActive`'s Running subset.

Do not delete a live worker after a grace period to make a registry look empty. Go cannot safely force-kill an arbitrary goroutine. A noncooperative external process may need an already-authorized process-level termination contract; an in-process operation may require retaining ownership and reporting incomplete shutdown. Dependency handles must not be advertised safe to close merely because a cancel was sent. The September report's warning that descendants may outlive terminal SubAgent state remains relevant: the acceptance design must enumerate actual owned descendants, not assume a state bit joins everything.

## 7. Costs and fair alternatives

Let A be currently live tasks, U terminated but unobserved tasks, and R retained terminal results. The current source path can retain A+U SubAgent graphs while only published results enter the R≤256 cache bound. A proposed owner-driven path targets A live graphs plus bounded R compact results and bounded tombstones. This is a retention model, not a measured memory saving. Actual graphs contain shared pointers as well as unique history and results; do not charge the entire shared kernel separately for every task.

Measure lifecycle CPU and lock time; startup/terminal notification latency; retained unique bytes; result copy cost; cleanup duration; outstanding descendants; provider tokens after cancellation; and human work needed to recover partial outcomes. An event signal costs its handle and synchronization; a watcher costs a goroutine while its task is live. A durable result store, if selected, adds IO, serialization, retries, retention and privacy costs. Cancellation cannot undo already billed generation or external effects.

Compare:

1. Current reader-driven reaping, with its existing normal success protections.
2. Minimal owner-driven terminal transfer while retaining current polling and cache policy.
3. The same transfer plus latched start/cancel and event-driven wait.
4. Optional bounded archival for unread outcomes, only when the use case requires it.

Do not infer that eliminating timer wakeups is the main win. The correctness target is outcome transfer without a future reader; the performance target is avoiding unnecessary retained state and recovery work. Hold model, authority, task allowance and acceptance checks constant.

## 8. Proposed evaluation, not executed

| Deterministic fixture | Required observation |
|---|---|
| Cancel waiter, then allow worker to terminate, never poll again | Terminal owner result established; heavy worker retired; placeholder no longer in-flight. |
| Successful async task with no retrieval | Same publication/retention contract, not a permanently retained executor. |
| Stop before Run, and after Running before cancel installation | Latched cancellation survives both schedules; no unreported start. |
| Same canceled context versus separate waiter context | Preserve documented semantics and distinguish inherited cancellation from explicit stop. |
| Immediate completion after registration | Result cannot be overwritten by an incomplete placeholder. |
| Duplicate Run, duplicate finalization, concurrent readers | One actual execution, one canonical outcome, idempotent transfer. |
| Publication/storage failure after tool effects | No task replay; observed partial outcome retained or explicit unresolved delivery state. |
| More than retention capacity of canceled/unretrieved tasks | Count and byte bounds respected without disguising live work as expired. |
| Cleanup failure or cancellation-ignoring dependency | Not falsely reported joined; ownership and diagnostic state preserved. |
| Late reader after expiry | Stable explicit expired/archived disposition, not endless running/not-found ambiguity. |
| Registered production delegation route | Uses the owner contract end to end, not only a direct test callback. |

Use controlled barriers rather than sleeps to hit startup/publication races. Existing double-run and normal-reaping regressions remain controls. Separate worker terminal publication from joining descendants; verify both at their defined boundaries. Whole-session trials should mix success, cancellation and abandoned readers across thousands of tasks, with a fixed workload and measured retention over time. Report unresolved outcomes, false-completion rate, task success, p50/p95/p99 latency, all token/host costs and uncertainty. No gain is established until this is done.

## 9. Primary foundations and bounded implications

- [Go `context.CancelFunc`](https://pkg.go.dev/context#CancelFunc) distinguishes requesting cancellation from waiting for work to stop. It supports the split in this design; it does not prove any particular CodeNERD dependency observes cancellation.
- [Go `errgroup.Group.Wait`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Wait) provides a clear model of waiting for an owned group to return. It does not automatically join goroutines spawned outside that group or supply task-result persistence.
- [The Go memory model, synchronization](https://go.dev/ref/mem#Synchronization) explains why terminal result visibility needs a synchronized publication boundary. A channel close or protected state transition can supply that boundary; naming a field `Completed` cannot.

These are established concurrency principles adapted to a source-observed ownership seam, not a novelty claim.

## 10. Buildable proposal queue

| Research ID | Narrow deliverable | Checkable exit |
|---|---|---|
| TASK-OWNER-01 | Terminal transfer independent of result polling | Canceled and never-polled tasks publish outcome and retire heavy state after actual termination. |
| TASK-OWNER-02 | Latched start/cancel handshake | Both prestart cancellation windows have deterministic results, without weakening gates. |
| TASK-OWNER-03 | Explicit result disposition and retention | No terminal task remains an unlimited in-flight placeholder; late-reader behavior documented. |
| TASK-OWNER-04 | Join semantics for the owning component | Reports actual remaining work; does not claim cancellation or timeout equals termination. |
| TASK-OWNER-05 | Whole-session value | Demonstrates bounded retention and accurate partial outcomes at unchanged task quality and authority. |

The first milestone is simple to state: an accepted task still has someone responsible for its final outcome after the person waiting for it has gone away.

## As built (2026-10-28)

- **T1, worker-owned settlement.** `SubAgent.finish` stores the terminal result, sets the state, closes `Done()`, then calls the owner's `SetOnTerminal` callback, exactly once (`sync.Once`) and outside every lock. `JITExecutor.executeAsyncInternal` installs `settleTerminal`, which caches the observed return under the completed-results bound and removes the registry entry. A waiter that cancels no longer leaves a terminal agent in the Spawner or an incomplete placeholder in the cache. `getObserved` shares `settleTerminal`; both paths are idempotent. Test: `TestJITExecutor_CanceledWaiterStillSettlesTheTask`.
- **T2, latched stop.** `Stop` records `stopRequested` under the lock that `Run` holds while installing `cancel`. A stop that lands before the install ends the task as stopped-before-start (`context.Canceled`, state Failed, model never called), instead of running on a live context. `executeAsyncInternal` calls `Run` directly, since `Run` itself schedules the work. Test: `TestSubAgent_StopBeforeRunIsHonoured`.
- **Waiting** is a select on `Done()` instead of a 100 ms poll, in both `SubAgent.WaitWithContext` and `JITExecutor.waitObserved`. A canceled wait still stops the task (the existing cancel-on-wait contract).
- **Deferred:**
  - separate "stop waiting" from "cancel task" in the API;
  - byte budgets for terminal retention;
  - a bounded owner context for cleanup that runs after cancellation.
