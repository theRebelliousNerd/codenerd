# Glass Box observer isolation: optional export is still on executive critical paths

**Repository:** `theRebelliousNerd/codenerd`  
**Reviewed revision:** `247f75e4722170cf5eaf2e205f9bbf4f9ae9925f` (`main` snapshot supplied for this review)  
**Review mode:** source and documentation inspection only. No application, model inference, tests, benchmarks, or failure injection were run; no repository files were changed. The failure sequences below are source-derived conditions, not reproduced incidents.

## Decision in brief

Keep Glass Box and its machine-readable export, but make the observer boundary genuinely nonblocking for executive callers. Today, subscriber channels drop rather than block, while registered sinks are called synchronously. The built-in NDJSON sink performs buffered-writer I/O and a flush on every event. Consequently, an optional exporter can delay an already-executed action's return and subsequent world refresh, or delay a non-system shard's launch while its manager mutex remains held. A different failure mode—an I/O error that returns promptly—can silently disable useful export for the lifetime of the sink because the buffered writer retains its error.

These are two related observer-isolation gaps, not evidence of an authorization bypass, fabricated execution success, or a currently observed production hang. The relevant configuration is an enabled bus with a dispatched category and a registered sink whose underlying writer becomes slow, blocks, or fails. With `CODENERD_GLASSBOX_NDJSON` unset, the built-in file exporter is not attached. With a healthy fast writer, the same synchronous path can complete normally.

The recommended direction is bounded, observable sink admission followed by owned export work outside bus and executive locks. This must preserve the existing lock-order and shutdown fixes, truthful action outcomes, and the distinction between optional telemetry loss and failed execution. It must also state the overload and shutdown tradeoff honestly: finite memory, lossless export, and bounded producer latency cannot all be guaranteed against indefinitely stalled storage.

## 1. Contract and scope

The package's current `EventSink` contract explicitly says implementations must not block. `dispatchLocked`, however, directly calls each sink's `Write` after its nonblocking channel-send loop. The interface returns no error and accepts no context. The built-in `NDJSONSink` takes its own mutex, serializes the event, calls `Write`, `WriteByte`, and `Flush`, and discards their errors. Swallowing export errors prevents those errors from becoming action errors; it does not make the I/O nonblocking. [EventSink and dispatch](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/event_bus.go#L53-L116), [NDJSON write](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/ndjson_sink.go#L84-L117).

This matters under the repository's own executive/observer separation, not merely an external preference for asynchronous logging. Current transparency guidance requires nonblocking subscribers, retains the nonblocking sink contract, fixes the lock order as `bufferMu` before `mu`, and gives `Close` terminal ownership and timer-join obligations. The architecture safety document also describes emission as having no unbounded wait. Older architecture inventory and gap tables are not all current; this review uses implementation and the appended as-built lock/shutdown contract where they differ. [Current package guidance](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/agents.md), [safety invariants](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/Docs/architecture/transparency/09-SAFETY-AND-INVARIANTS.md), [current lock/shutdown contract](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/Docs/architecture/transparency/03-GAP-ANALYSIS.md#L114-L130).

**Nonduplication boundary.** The existing event-bus repair addresses lock acquisition order, terminal close, and timer ownership. Its as-built note explicitly leaves external sink obligations open. This audit concerns synchronous external observer work even when that repaired lock order is followed correctly. The execution-outcome projection audit concerns reporting a failed typed outcome as success; here the outcome can remain correct but its delivery is delayed. The world-refresh publication audit concerns coherent scan/publication ordering; here an exporter can prevent the caller from reaching the subsequent refresh at all. Neither prior repair should be reverted or described as absent. [Execution-outcome audit](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/Docs/audits/system-execution-outcome-projection.md), [world-refresh audit](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/Docs/audits/world-refresh-publication-order.md), [audit queue](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/Docs/audits/ASTRA_QUEUE.md).

## 2. Production wiring and two independent critical paths

### 2.1 Normal chat boot makes this more than a helper-only concern

Shared chat boot constructs a `GlassBoxEventBus`, enables it, constructs the separate tool-event bus, and injects both into the VirtualStore; it also injects the Glass Box bus into the shard manager. The bus constructor calls `attachEnvNDJSONSink`. That function is a no-op when `CODENERD_GLASSBOX_NDJSON` is empty, reports an open/setup failure to stderr, and registers the file sink only after construction succeeds. The file constructor creates parent directories as needed and opens an append-mode file. [Boot wiring](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/cmd/nerd/chat/session_shared_boot.go#L175-L203), [bus constructor](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/event_bus.go#L68-L79), [file sink and environment attachment](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/ndjson_sink.go#L58-L72), [attachment](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/ndjson_sink.go#L139-L151).

The demonstrated wiring is this chat boot path. Constructor comments about headless export do not prove that every campaign or headless entrypoint constructs and injects this bus. This review does not assert those additional routes. It also does not equate a UI display toggle or the separate transparency manager's settings with disabling the bus.

### 2.2 Action completion: an effect can finish before optional export prevents return

The source order in `RouteActionResult` is significant:

1. Existing boot, dreamer, constitutional, and exact permission checks precede handler dispatch.
2. `executeAction` runs. `actionDuration` is captured immediately after that handler returns.
3. A handler error is converted to a failed result, success facts are removed, and completion is recorded before returning the error. Otherwise applicable post-action validation runs and may reject the result.
4. The normal path calls `recordActionCompletion` before `refreshWorldAfterFacts` and before returning the typed result.
5. Completion injects its facts, prunes history, emits the audit action-complete call, and invokes `emitToolAndRoutingEvents`.
6. That helper emits the separate tool event and records the operation before calling `gbus.EmitImmediate` for the routing event.

[Admission and dispatch](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/core/virtual_store_routing.go#L35-L156), [validation, completion, and refresh order](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/core/virtual_store_routing.go#L159-L225), [tool/routing emission](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/core/virtual_store_routing.go#L258-L328).

**Conditional witness:** an allowed action completes its handler and preceding completion steps, its routing category is admitted, and the attached NDJSON writer then stops returning from an underlying write. `RouteActionResult` cannot reach its return while that call remains blocked. On the normal path, the following world refresh also has not run. An actual file change may already have landed; completion facts and an audit call precede this point. The tool-event channel may already contain the result. Thus the consequence is not necessarily “no record” or “action never happened”: it is a split between an already-performed effect and a caller still waiting on optional observation.

The helper snapshots its bus pointers under `v.mu.RLock` and releases that lock before emitting. This is **not** a claim that the sink retains the VirtualStore mutex. Nor does the sink error rewrite `result.Success`. An implementation change must never retry the original effect merely to recover an export record, or turn export loss into an execution failure.

The timing boundaries are also different, not inherently incorrect. `actionDuration` measures the handler, excluding validation, completion export, and world refresh; the outer deferred `RouteAction` timer spans the full call and therefore includes an export delay. A comparison using only event `Duration` could miss this tail. [Timing and handler boundary](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/core/virtual_store_routing.go#L35-L37), [handler duration](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/core/virtual_store_routing.go#L138-L143).

### 2.3 Shard spawn: the same observer can retain the manager lock before launch

`SpawnAsyncWithContext` acquires `sm.mu` and defers its release for the call. After admission and agent construction, it inserts the agent into `sm.shards` and emits a spawn audit call. For a non-system shard it then calls `sm.emitShardEvent`, which uses `EmitImmediate`. This occurs before `spawnStart` is captured, before the operation observer's start notification, and before the execution goroutine is launched. [Manager lock](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/core/shards/manager_spawn.go#L190-L195), [spawn ordering](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/core/shards/manager_spawn.go#L368-L411), [immediate shard emitter](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/core/shards/manager.go#L108-L123).

With an admitted non-system spawn and a dispatched shard category, a stalled sink can therefore hold the manager mutex, delay returning the shard ID, delay launching that shard, and delay other manager operations requiring the same mutex. The child execution timeout is created later inside the goroutine; it does not bound this pre-launch observer call. `spawnStart` likewise excludes the export delay. This is independent of an action-completion stall and does not require the new shard to have made a model call. Resident system-service spawns skip this particular ping; failed admission also does not reach it. [Execution timeout location](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/core/shards/manager_spawn.go#L411-L454).

## 3. Why the bus amplifies a slow observer

`emit` acquires `bufferMu`, then `mu.RLock`. For an immediate event or verbose mode, it calls `dispatchLocked` while holding both. Channel subscribers are tried first with `select/default`; sinks follow serially. Consequently, a channel can already have received the current event before a sink stalls, but later emitters and sinks behind that sink cannot progress through the same dispatch. [Emission](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/event_bus.go#L244-L271), [dispatch](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/event_bus.go#L102-L116).

Ordinary batching does not remove this exposure. A batch-limit flush runs synchronously in the emitting call; a timer flush runs in its timer worker but still holds `bufferMu` and takes `mu.RLock` while dispatching every buffered event. Each NDJSON event still flushes separately. The timer may move the first blocked write off a particular emitter, but once it holds the buffer lock it can block subsequent producers behind it. [Batching and flush](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/event_bus.go#L268-L326).

The retained locks affect more than export: `Stats`, `Flush`, `ClearTurn`, `Disable`, and `Close` need the buffer lock; subscription, category, and sink-list changes require the bus write lock. A sink that remains blocked can thus make status and lifecycle operations wait too. `Close` itself flushes and invokes owned sink closers synchronously before releasing the locks and joining timer workers. The existing lock-order discipline avoids its repaired inversion but cannot impose an I/O deadline. [Lifecycle and statistics](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/event_bus.go#L328-L406).

Neither `EmitImmediate` nor `EventSink.Write` accepts a context. Canceling the action's context does not directly interrupt an already-running sink write. Calling the current bus or sink `Close` is not a general escape hatch: those methods first need mutexes that the blocked path may already hold. This establishes a potentially unbounded wait **if the underlying write remains blocked**, not a claim that every ordinary file write hangs or that recovery is impossible. A writer that resumes can release the chain normally.

## 4. Prompt I/O failure is a different, silent-loss failure mode

A full or otherwise failing output destination need not block. The writer can return an error promptly, allowing the executive to continue, while export becomes unusable. Go's `bufio.Writer` retains a write/flush error; subsequent writes and flushes return that error until reset. Its implementation also treats a short write without an error as `io.ErrShortWrite`. The NDJSON sink ignores all three write/flush return values and has no reset/reopen or error-state reporting path. A transient underlying error can therefore become a persistent sink-level export outage even if the underlying destination later recovers. [Go buffered-writer contract](https://pkg.go.dev/bufio#Writer), [Go buffered-writer implementation](https://go.dev/src/bufio/bufio.go), [sink implementation](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/ndjson_sink.go#L84-L135).

This need not erase earlier complete records. A partial write can leave a truncated trailing record, while earlier lines remain useful. Future calls can still spend time building JSON that the sticky-error writer rejects. `NDJSONSink.Close` also suppresses its final flush error and only returns an underlying close error, if any; the bus discards sink-close errors. A successful return from these paths is not an export-delivery receipt.

The existing counters should **not** be called incorrect. `Delivered` explicitly counts successful subscriber-channel sends, once per subscriber; `Dropped` counts full-channel misses. `SinkCount` counts registrations. These fulfill their documented scope. They simply do not report successful sink writes, retained writer errors, incomplete records, or export lag. Healthy channel-delivery statistics and an attached sink can coexist with a failed file export. [Statistics definitions](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/event_bus.go#L370-L406).

Per-event flush is a real benefit for reducing bytes left in the userspace buffer if the process crashes after a completed flush. It is not an `fsync`/power-loss durability guarantee: this sink makes no `Sync` call. Replacing it with asynchronous or batch export changes the crash-tail exposure and should not be presented as a free improvement.

## 5. Existing protections and what inspection does not establish

Already present and worth preserving:

- Authorization and exact permitted-action checks precede execution; typed handler/validation failures retain their failure semantics.
- Channel sends are nonblocking, with explicit delivered/drop counters; the separate tool bus can deliver independently of Glass Box display state.
- Disabled or category-filtered events avoid sink dispatch. NDJSON turn filtering also skips nonmatching events once it acquires its own lock.
- Environment-sink construction failure is visible on stderr and does not register a broken sink.
- The bus uses the repaired lock order, rejects post-close admission, closes late registrations, stops/joins timer work, and has deterministic lifecycle/lock regression tests.
- The sink serializes access and makes its close idempotent. Those are useful concurrency properties even though mutex serialization does not bound external I/O.

The existing tests inspect channel overflow accounting, concurrent emission, healthy NDJSON file output, turn filtering, and environment attachment. The lock-order suite uses a recording sink and explicit contention/timer controls, including native-child watchdogs; it does not establish isolation from a slow or failed NDJSON writer. No test was executed during this audit, and no result is claimed for the proposed failure scenarios. [Observability tests](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/observability_test.go), [lock-order and lifecycle tests](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/event_bus_lock_order_test.go), [basic event-bus tests](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/event_bus_test.go).

This review establishes the call and lock dependencies. It does not establish how often configured deployments hit slow storage, the installed filesystem behavior, measured latency or loss rates, model-quality changes, or net cost savings.

## 6. Proposed repair contract, not an implementation prescription

### Separate observer admission from export work

Make executive-facing emission admit an immutable event snapshot into a bounded observer queue, then return without waiting for underlying file/network I/O. Use an owned worker per sink, or equivalent isolation, so one failed exporter does not stall other sinks. Define capacity in bytes as well as events: the event type carries strings and has no universal payload-size bound, even though individual producers truncate some details. A fixed event-count queue alone does not bound retained bytes. [Event shape](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/internal/transparency/glass_box_events.go#L35-L59).

Do not launch an unbounded goroutine per event. That substitutes unbounded memory, pending writes, ordering ambiguity, and shutdown leaks for visible blocking. Merely releasing the bus locks before calling a sink is also insufficient: it reduces bus contention but still blocks the emitter, including a caller that holds `sm.mu`. The producer-facing operation itself needs bounded behavior.

The current contract says sinks receive every dispatched event. A bounded, nonblocking queue requires an explicit overload policy rather than silently weakening that promise. Choose whether to reject newest, evict older, prioritize selected categories, or disable a failed exporter, and expose the loss. If a deployment needs lossless durable evidence rather than optional diagnostics, it needs a separately specified persistence boundary; a finite spool cannot promise losslessness against an indefinitely unavailable destination.

### Keep lifecycle ownership real

Retain terminal admission closure, no resurrection, one owner for each sink, and joined workers/timers. Snapshotting sinks outside locks needs a lifetime protocol so `Close` cannot race an in-flight use or close an owned writer twice. Specify healthy drain behavior and an explicit degraded outcome if shutdown abandons queued telemetry. Do not report a successful full drain merely because a deadline elapsed.

For arbitrary `io.Writer`, an interface wrapper cannot manufacture a cancellation guarantee. Its contract has no context or deadline operation. A timeout around a goroutine can stop waiting while leaving the actual write and worker alive. Hard bounded shutdown therefore requires a suitably interruptible/deadline-capable transport, an ownership design that can actually interrupt that transport, or stronger process isolation. State which guarantee is supported rather than assuming every `Close` breaks every `Write`. [Go writer interface](https://pkg.go.dev/io#Writer).

### Make exporter health observable without recursing into the failed exporter

Track sink-specific admission, queue depth/bytes, oldest-event age, writes completed to the underlying writer, overload drops, first/last error, and drain/close outcome. Distinguish those from channel counters and from durable storage acknowledgment. Status should remain readable without waiting on the blocked I/O mutex. Report degradation through a bounded independent path; logging an exporter failure back into the same exporter risks recursive failure or another stall.

Define recovery deliberately. Replacing/resetting a sticky-error writer may abandon retained bytes. Retrying a partially written JSON record can duplicate a prefix or record. Document whether records have recoverable sequence IDs and whether the reader tolerates a truncated final line; do not claim exactly-once export without a corresponding protocol. No recovery operation should replay the original tool action.

Preserve event IDs, categories, turn association, and filtering. Specify whether the new queue preserves current dispatch order or introduces an explicit ID-order policy. Existing buffered and immediate paths are not a basis for claiming universal strict ID-order delivery: an immediate event can be dispatched before an older buffered one. Observer isolation should not quietly invent a stronger ordering guarantee or an unrelated reordering project.

### Avoid unrelated expansion

This repair does not require a new model call, a new permission policy, an OpenTelemetry dependency, per-turn campaign export, or a rewrite of world publication. The transparency TODO already distinguishes completed channel/drop/export plumbing from separate lifecycle/export-ownership work. Keep this change focused on the observer boundary and its truthful delivery contract. [Transparency TODO](https://github.com/theRebelliousNerd/codenerd/blob/247f75e4722170cf5eaf2e205f9bbf4f9ae9925f/Docs/architecture/transparency/TODO.md).

## 7. Fair, future evaluation

The following is a proposal, not a report of executed tests.

**First use deterministic writer fixtures.** A writer should signal that its `Write` has started and wait on an explicit release channel. Observe bounded producer/status progress while it is paused; then always release and join it. Use a native-child watchdog for cases that intentionally probe blocking so a failed implementation cannot strand the test process. Test immediate, verbose, batch-limit, and timer-flush paths separately. Include a healthy channel and a second healthy sink to distinguish producer isolation, channel delivery, and per-sink isolation.

**Cover failure and recovery separately from slowness.** Inject a first-write error, a short write, a partial write with error, and a writer that subsequently becomes healthy. Check sticky-error reporting, truthful per-sink counters, parseability of complete records, documented recovery, and no false drain-success report. Exercise `Close`, `Disable`, `Flush`, `Stats`, late registration, and concurrent emitters. Retain the existing deterministic lock-order and timer-join controls; passing a race detector alone is insufficient for liveness.

**Exercise the actual caller boundaries.** Through normal boot/injection and production admission wiring, use an allowed deterministic local action with one externally countable effect and no model inference. Confirm that export degradation does not replay the effect, change its typed success/failure, skip required fact handling, or prevent the subsequent world-refresh path under the proposed contract. Separately, use an admitted non-system scripted shard to probe return/launch and another manager operation while export is paused. Include system-service and denied-admission controls. Do not bypass constitutional checks or replace the entire VirtualStore/manager with a helper mock and call that production validation.

**Use symmetric controls.** Compare current and proposed implementations with the same event stream, event sizes, category/turn filters, filesystem or writer fixture, concurrency, warmup, and output-retention requirement. Include export unset, bus disabled, category excluded, healthy export, slow export, failed export, and overload. Match queue/drop/durability policy where possible; when the proposal buys bounded latency by losing records or increasing crash-tail exposure, report that difference rather than labeling it a pure speedup.

**Measure the right boundaries.** Record handler time separately from full `RouteActionResult` return time, spawn call return and launch delay, concurrent manager-operation delay, status responsiveness, export lag, complete records, lost/partial/duplicate records, queue bytes, CPU, allocations, goroutines, open handles, and shutdown/drain outcome. Event `Duration` alone omits the action export tail, and `spawnStart` begins after the spawn ping. Use fixed seeds/workloads and repeated runs; report distributions and failures, not just an average.

For an eventual end-to-end cost/quality evaluation, keep model, prompt/context, repository state, permissions, task set, tool policy, and terminal success criteria matched. Count retries and any downstream work caused by changed visibility. The mechanism itself requires no inference, but there is no basis here to claim token savings, better task success, or a measured performance win.

## 8. Costs, tradeoffs, and acceptance criteria

The current synchronous design is simple: export ordering follows dispatch, no extra sink queue exists, and per-event flushing minimizes userspace buffering after a successful write. Its latency cost includes serialization and each sink's synchronous write/flush, with shared locks held across that work. Batching events at the bus does not batch the sink's per-event flushes.

An isolated exporter adds bounded queue memory, worker scheduling, synchronization, possible event copying, health-state bookkeeping, and more complex close/recovery logic. Batch writes can reduce write/flush overhead but increase export lag and the crash tail. Dropping on overload protects executive progress but loses telemetry; preserving all telemetry can require storage and a different availability contract. These are design costs to measure, not reasons to retain an undocumented blocking boundary.

Accept a repair only when:

1. Slow or failed optional export cannot make the selected action-completion or non-system spawn caller wait for underlying I/O, nor retain bus/manager locks on behalf of that I/O.
2. Original execution/validation/permission outcomes remain unchanged, effects are not replayed, and required follow-on world work is not replaced by telemetry success.
3. Memory/work admission is bounded, overload and sink failure are distinguishable from channel drops, and healthy observers remain independent.
4. Shutdown and recovery promises match the supported writer contract; no leaked worker is disguised as a joined one and no abandoned data is reported as fully flushed.
5. Existing lock-order, terminal-close, timer-join, filtering, channel-delivery, and outcome protections remain covered.
6. Any performance or task-cost claim is accompanied by matched measurements and its telemetry-loss/durability tradeoff.

**Bottom line:** the existing channel and lifecycle repairs are real. The remaining boundary is the built-in synchronous sink: optional observation can still become executive waiting, while swallowed I/O errors can make export silently stop. Fixing that boundary should buy bounded, truthful observer behavior—not new action authority or unearned claims of lossless, free telemetry.
