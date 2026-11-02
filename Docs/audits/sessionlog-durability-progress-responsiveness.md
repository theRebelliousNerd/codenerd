# Sessionlog durability cost and progress responsiveness

## Conclusion and evidence status

**Retain the transcript crash-consistency repair. Separate live progress reads from its storage critical section, then measure whether long-history write amplification warrants a different storage format.** The current writer deliberately publishes a complete JSON snapshot per event. Its size-proportional memory, temporary-space and I/O costs are already acknowledged in `GAP-SESSIONLOG-TRANSCRIPT-CRASH-CONSISTENCY`; this note does not present them as a newly discovered defect. It develops the cumulative cost model and a distinct cross-task coupling: a task progress query waits on every same-session transcript's mutex before checking task identity, while append holds that mutex through full-history encoding and synchronous file publication. [Existing contract and acknowledged cost][gap], [append][append], [progress selection][progress].

**Status:** source-derived analysis and proposed evaluation, not a measured production incident or benchmark. All codeNERD source citations are pinned to `theRebelliousNerd/codenerd` commit **`e933c03b3b02813c0862a2210a01eec4d9bf95ad`**, inspected on 2026-11-02. No application execution, repository edits, tests, inference, fault injection or timing measurements were performed. The only authored artifact is this scratch Markdown document. Existing test assertions are described as inspected coverage, not a test pass established here.

The relevant object is `.nerd/sessions/<session>/<agent>/transcript.json`, the always-on delegated-agent and resident-service record. Main-chat `session.json`, the UI conversation saver, SQLite memory, provider prompt history and debug-only exchange files are different ownership and persistence paths. This is not an argument to truncate model context, disable diagnostics or move executive policy out of Mangle. [Package record kinds][contract], [scope definitions][scope], [repository guidance][guidance].

| Question | Supported conclusion |
|---|---|
| Is whole-history work present? | Yes, directly in the pinned write path and already documented. |
| Does it accumulate quadratically for approximately equal-sized events? | Yes, under the explicit byte-cost model below; this is mathematical accounting, not a latency measurement. |
| Can unrelated task or service publication delay a task progress query? | Yes, a valid source-level interleaving follows from lock selection before task matching. |
| Is a deadlock or a frozen UI demonstrated? | No. No lock-cycle proof or runtime timing was established. |
| Is direct Ctrl+X blocked on `turn.decide`? | Not by this chain: direct stop takes `turn.mu`, not `turn.decide`. |
| Are cheaper alternatives already proven equivalent? | No. Proposed alternatives need the same durability, recovery, ordering, compatibility and ownership controls. |

## Current production path and synchronization boundary

The boot adapter installs the session store and retention policy before resident shards, independent of the debug exchange switch. `RunAgent` and `RunService` set the transcript task association, record input before running the callback, and record the final return/error through a defer, including a recovered panic. Inline delegated execution wraps its real `ProcessWithIntent` call with `RunAgent`; isolated subagent execution uses the same wrapper. [Boot adapter][boot], [agent wrapper][agent], [inline caller][inline], [subagent caller][subagent].

Broker observation synchronously calls `CallStarted`; settlement synchronously calls `Assistant` or `CallFailed`. The native executor's `executeToolBatch` flushes accumulated tool results before the next call and once at return. Consequently, logging is on an execution handoff, not merely a best-effort goroutine launched after the model or tool step has been released. The transcript preserves useful completed-effect evidence, including when a run ends mid-batch. [Broker observation][broker-start], [settled response][broker-end], [tool result flush][tool-flush].

For a healthy already-open transcript, one append performs the following ordered work:

1. Acquire `Transcript.mu`; retain it until append returns.
2. Allocate the next sequence and timestamp, then update the compact in-memory `progressState`.
3. Encode the new event with the line-array representation.
4. Open and read the entire current `transcript.json` through `atomicfile.Open`.
5. Decode its ordered JSON tree, decode the new event, append it to `events`, and encode the whole extended document. Ordered/unknown fields and exact `json.Number` text are intentionally retained.
6. Obtain the existing file mode, enter the atomic-file path lock, create a unique sibling temporary file, write all snapshot bytes, `Sync` the temporary file, close it, apply mode, and replace the destination.
7. Attempt containing-directory open and `Sync` on a best-effort basis; return through the held transcript mutex.

The first append additionally opens/validates or creates a header snapshot. A recognized legacy torn tail is repaired atomically before new recording proceeds. A healthy subsequent append does not repeat that recovery pass, but still reads, decodes and re-encodes the whole current snapshot. [Append and open][append-open], [snapshot writer][writer], [ordered representation][ordered], [atomic publication][atomic].

`atomicfile` also uses 64 process-local lock stripes, so unrelated destinations can collide independently of the transcript-progress issue. Windows replacement has a bounded retry budget for particular sharing failures and best-effort temporary cleanup can wait on failures. Those mechanisms are correctness/portability protections, not evidence of a measured delay here. Their local retry budgets do not impose a wall-clock bound on the enclosing read, encoding, mutex wait or `Sync`. [Atomic path stripes][stripes], [Windows replacement][windows].

## The distinct cross-task progress coupling

`Store.ProgressForTask(session, task)` first resolves the session and gathers every transcript belonging to that session under `Store.mu`. It releases the store mutex before walking that list. For **each** transcript it then locks `t.mu`, checks `t.task == task`, copies progress only on a match, and unlocks. The function does **not** hold all transcript mutexes simultaneously; it visits them sequentially. But it must acquire the mutex even to reject an unrelated task. The list order originates from a map and is not a task-priority order. [Progress selection][progress].

A minimal adverse schedule is therefore:

1. Task B's transcript begins a large append and holds `B.mu` during ordered decode, encoding, file writing or synchronization.
2. A report for task A in the same session collects A and B as candidates.
3. The report encounters B and waits for `B.mu`; only after that wait can it discover that B belongs to a different task and skip it.
4. The report eventually returns A's scalar progress even though none of B's transcript text was needed.

This remains possible when A has no matching transcript yet. It also applies to resident-service transcripts: `WithService` clears the task while retaining a diagnostic transcript, so the service is excluded from the returned task list only **after** its transcript mutex has been acquired. The resident-service test verifies exclusion from task progress and the delegated live feed, not responsiveness during a blocked service append. [Service scope][scope], [service exclusion test][service-test].

The production chat path increases the consequence of this wait. `runningWorkReport` holds `turn.decide` while constructing the report; its locked variant invokes `ProgressForTask` for foreground work and `jobProgress` for background jobs. Every natural-language side turn constructs that report before `ParseIntentWithContext`. A side turn classified by the kernel as `/status` requests another report while `decideSideTurn` holds the same decision mutex. Other side-turn decision sections can therefore queue behind storage work for a transcript irrelevant to the requested task. Multiple background-job reports can repeat the same same-session candidate scan. [Running work report][report], [background progress][background], [side-turn preparation][side-input], [side-turn decision][side-decision].

The conclusion is deliberately bounded: natural-language status, steering and stop requests can be delayed before perception or decision by this source-level dependency. No delay distribution or user-visible freeze has been measured. Direct Ctrl+X uses `stopExecution` and `turn.mu`, not this decision lock; do not claim that this progress-reader chain prevents direct cancellation from being requested. Separately, a synchronous append already in progress does not inspect a cancellation context, so requesting cancellation is not a promise that an in-flight filesystem operation immediately terminates. [Direct stop][stop], [append][append].

Compact `LiveEvent` emission is a separate channel and several transcript methods emit it before append. That can keep some UI updates available; it does not remove the `ProgressForTask`/`turn.decide` dependency. A report is already a collection of per-transcript observations at different instants, not an atomic all-agent snapshot. A safer reader need not invent a stronger consistency contract than that. [Live progress][live], [progress selection][progress].

## Whole-history cumulative cost model

Let `N` be the number of successful appended events in one transcript; `S_i` its serialized size after event `i`; `S_0` the initialized header document; and `e_i` the incremental serialized contribution of event `i`, including separators in the simplified model. Assume a stable representation, ordinary bounded JSON nesting and no external writer. The source performs approximately:

```text
read bytes       R_N = sum(i=1..N) S_(i-1)
snapshot writes  W_N = sum(i=1..N) S_i
S_i                 = S_0 + sum(j=1..i) e_j

R_N = N*S_0 + sum(j=1..N) (N-j)*e_j
W_N = N*S_0 + sum(j=1..N) (N-j+1)*e_j
```

Thus an early large result is revisited on every later append. A late equally large result has less cumulative amplification. With approximately constant event size `e`, both totals are `Theta(e*N^2 + N*S_0)`, while final retained history is only `Theta(e*N + S_0)`. Per-event decoding/encoding is history-proportional under these assumptions, and cumulative allocation traffic follows the repeated traversals. This is not a claim that every possible pathological JSON shape has the same constant or that all bytes hit physical media. [Writer][writer], [ordered decoding and encoding][ordered].

**Illustrative accounting, not a benchmark:** ignoring header/scaffolding, 10,000 events contributing 2,000 bytes each finish at 20 MB, but generate about 100.01 GB of logical snapshot writes and 99.99 GB of logical history reads. The example predicts neither wall time nor device writes. Page cache can satisfy reads; filesystem journaling, copy-on-write, compression, storage caches and writeback change physical traffic. Sequence-length changes and real event-size distributions change exact totals. Failed publication attempts and recovery add work beyond this successful-append model.

The complete cost envelope includes:

- **Synchronous latency:** mutex wait, history read, JSON decode, event processing, full snapshot encode, atomic path-lock wait, temporary write, file synchronization, replacement and best-effort directory synchronization. The per-event flush count remains proportional to event count; byte amplification is a different axis. Initial creation or legacy repair adds publication work.
- **Peak memory:** full input bytes, decoded ordered objects/arrays and encoded output coexist during an append, with temporary buffers and allocator/GC overhead. The model is `O(S_i)` live data for ordinary fixed-depth transcripts, not a measured peak-RSS multiplier. Concurrent writers on different transcripts can sum those working sets.
- **Peak storage:** the old snapshot and new temporary snapshot coexist. Long-lived readers can keep replaced generations allocated until their file handles close; crashes and failed cleanup may leave temporary debris. Session retention is not per-event compaction.
- **Progress query cost:** scan the store's transcript map, collect the selected session's candidates, acquire each candidate mutex, copy only matching scalar state, then insertion-sort matching results. With `T` store transcripts, `T_s` same-session candidates and `K` matches, non-wait work is `O(T + T_s + K^2)` in the straightforward worst case. Wait time depends on publication schedules, not on the size of the returned report. This query does not itself read or decode transcript files.
- **Other operations:** cold reopen and search still read history; recovery and JSON export have their own passes. Debug exchange recording adds independent overhead and must be controlled in comparison. No intrinsic model inference is required for any of these persistence operations.

The existing implementation buys a straightforward complete-file reader contract and evidence retention across interrupted publication. For short transcripts, low event rates or fast storage, that simplicity may be the right trade. For long-lived services and long tasks, the weighted-history model justifies measurement; it does not establish that storage currently dominates provider latency or total task cost.

## Guarantees and failure semantics that must survive optimization

The accepted contract is a complete `codenerd.transcript/1` snapshot for each successful publication, with the previous committed snapshot preserved until replacement is ready. Keep the schema, line-array round trip, ordered header/extensions, exact numeric values, event ordering, sequence continuation and file-mode behavior. Keep recognized legacy torn-tail recovery and refusal to rewrite unrelated or malformed earlier content. Search may recover a recognized prefix without modifying the original; unfinished temporary snapshots are not authoritative records. Existing old-reader, concurrent-reader, interrupted-tail, identity-refusal and failed-append retry tests encode valuable regression obligations. [Accepted contract][gap], [recovery implementation][recovery], [search][search], [durability tests][durability-tests].

Do not overstate the acknowledgement boundary. Public transcript methods return no error; nil recorders and broken recorders can return without publication. On open failure, append marks the recorder broken and counts a recording failure. On later marshal/write failure it decrements the allocated sequence and counts the failure without marking the recorder broken, permitting later attempts. `Store.fail` warns once and counts later failures so model work continues. Therefore “method returned” is not, by itself, a caller-observable durable acknowledgement. Here **acknowledged/committed event** means an event whose publication is known to have succeeded at the storage boundary, not every invocation of a void method. A future explicit receipt must preserve the deliberate diagnostic-failure availability policy unless separately changed. [Append failure handling][append], [failure accounting][failures].

`prog.observe` happens before encoding and storage. Its changes are not rolled back when those operations fail, although sequence allocation is rolled back. Live operational progress can therefore report an observation absent from committed transcript history. This is a semantic fact to preserve or explicitly redesign, not a reason to call progress a durability receipt. A short-lock progress snapshot must carry operational meaning; if a durable frontier is added, identify it separately. No log failure may authorize replay of an already-completed tool effect, invent a passed test or substitute for Mangle's acceptance/permission decisions. [Append][append], [session safety guidance][session-guidance].

The current successful path synchronizes the temporary file and attempts directory synchronization, but directory errors are ignored by design. Linux's `fsync` documentation distinguishes file persistence from containing-directory persistence and notes that synchronization blocks until the device reports completion. Hardware, operating system and filesystem assumptions matter; a source inspection or process-kill experiment cannot certify physical power-loss behavior on every platform. Preserve the current stated boundary rather than claiming perfect durability or weakening it implicitly. [Atomic publication][atomic], [fsync semantics][fsync], [existing limitation][gap].

Important failure and contention cases include full disk/quota, read-only directory, transient open/read error, failed write/close/sync/chmod/replace, uncooperative Windows reader handles, corrupted legacy input, cancellation during publication, and temporary cleanup failure. Large snapshots can encounter allocation pressure or need much more free space than the new event alone. These are mechanisms and evaluation cases, not observed failures. Keep bounded warning behavior and honest degraded recording status; do not turn a logger failure into a retry of the model's or tool's effect.

Single-Store ownership remains a precondition. The atomic-file lock protects an individual replacement, not the whole transcript read-modify-write across independent Store instances or processes. Two independent writers can each derive a new snapshot from the same old history. Moving work outside locks or adding background publication must not accidentally introduce that race inside the supported single-owner case. [Transcript ownership][ownership], [atomic path stripes][stripes].

## Proposed options and their tradeoffs

### Short-lock progress snapshots without a storage migration

The narrow first candidate is to give task identity and compact progress their own synchronization, separate from the serialized transcript writer. An immutable progress snapshot published atomically, or a small progress mutex held only while copying/updating scalar state, can serve readers without acquiring the I/O-bearing append mutex. A store-owned session/task index can avoid scanning or touching unrelated candidates, provided `setTask`, reassociation, service detachment and lifetime cleanup maintain the index atomically. These are proposed designs, not shipped APIs.

Keep one writer ordering authority for sequence allocation, recovery, append and publication. Do not merely unlock `t.mu` before disk I/O while allowing a second append to read or publish the same base snapshot: that can lose events or publish an older snapshot after a newer one. The writer mutex may remain long-held while the distinct progress snapshot stays cheap to read. Publish observed progress with its existing operational semantics; optionally expose a separate successfully published sequence and failure state, without conflating those concepts.

This option addresses cross-task responsiveness and much same-task report contention, but **does not remove whole-history read/write amplification or the append caller's synchronous latency**. It also requires defining snapshot freshness and keeping task identity coherent with the copied counters. If storage still stalls, status can remain responsive and say what is known rather than waiting to produce a falsely authoritative receipt. Mangle continues deciding status/steering disposition; Go only supplies a typed observation. A bounded sample age or stale marker is preferable to an undocumented promise of instantaneous global consistency.

Moving expensive collection outside `turn.decide` is a separate possible refinement, not a free unlock: the current critical section protects side-turn facts and brief decisions, and `runningWorkWrote` mutates/query facts. Any off-lock sample must be tied to execution owner/generation, revalidated before publication, and leave kernel transaction ownership intact. Start with progress isolation unless measurements justify this broader change. [Report ownership][report], [decision ownership][side-decision].

### Retain snapshots but reduce redundant work

A writer-owned cached representation, streaming encoder or immutable event representation may reduce decoding and allocation constants. Full-document replacement still writes `S_i` per event, still needs synchronization and temporary space, and must retain unknown extensions, numeric fidelity, mode handling and safe reopen. Keeping an entire cached history consumes resident memory; permitting external file edits would require a coherent invalidation/ownership policy. This is a candidate if measured decode/GC dominates, not a claim that caching eliminates the quadratic logical write total.

### Append-only records with compatible reads and export

A framed append log can make new-event write bytes approximately proportional to the new event plus framing, avoiding repeated rewriting of the prefix. LevelDB's primary log-format specification is a useful design reference: it uses fixed-size blocks, record length/type/checksum, and explicit fragments for larger records. That is a framing and corruption-navigation precedent, not a drop-in durability, acknowledgement or exactly-once protocol. A checksum alone does not make a torn write atomic. [LevelDB log format][leveldb].

A codeNERD design would need an identity/schema header, ordered event sequence, bounded frame lengths, integrity validation, clear handling of the incomplete final frame and an explicit committed frontier. Recovery must not silently resynchronize across arbitrary interior corruption and call the omitted events a complete transcript. Preserve the original evidence and report degraded coverage where recovery cannot establish continuity. A record surviving a crash without its acknowledgement may be retained as an uncertain/newly recovered observation; it is not permission to replay its tool effect.

Preserve `transcript.json` compatibility through a deliberate reader migration or a generated snapshot/export, not by leaving a stale JSON file that looks authoritative. Define which source owns truth, whether and how legacy JSON is imported, what old readers see, and the cutover generation. Search must retain text/line fidelity and identity association. A log plus snapshot/index adds checkpoint, retention, migration and interrupted-compaction protocols. Appending per-event bytes may reduce write amplification, but cold scan/replay remains `O(S)` without a trustworthy index. A shared log can aid batching while creating retention, privacy, ordering and noisy-neighbor coupling across agents; per-agent logs retain isolation but cannot share one file flush automatically.

### Group commit without silently changing acknowledgement

Two options must be distinguished. **Synchronous group commit** can wait for one shared flush and acknowledge every covered event only after that flush; it need not lose previously acknowledged events merely because multiple events share a barrier. **Asynchronous acknowledgement before flush** explicitly permits loss of a recent acknowledged suffix and changes the durability contract. PostgreSQL documents both the throughput/latency trade of waiting to gather a group and the different semantics of asynchronous commit. Those mechanisms are precedents, not PostgreSQL performance numbers transferable to this logger. [PostgreSQL group commit][pg-group], [PostgreSQL acknowledgement settings][pg-ack].

An otherwise serial agent that blocks after each event supplies little opportunity for same-agent batching. A per-agent file still needs its own appropriate synchronization; a queue does not magically combine several files into one durable transaction. If batching is adopted, define maximum queue bytes, backpressure, shutdown drain, flush errors, cancellation ownership, batch ordering and when waiting callers return. Count queue residence as latency. No finite periodic flush interval alone is a hard loss bound under stalled storage. Do not remove per-event flushes or acknowledge earlier merely to make a benchmark faster without explicit contract approval.

## Fair evaluation and acceptance gates

All evaluation below is proposed and unrun. Compare the pinned current writer with isolated progress snapshots first; compare storage alternatives separately so a reader-lock improvement is not misattributed to a format change. Use identical event content, order, identities, unknown fields, event-size distributions, task concurrency, debug setting, retention policy and durability/acknowledgement level. Keep failed and abandoned attempts in denominators. A weaker guarantee is a distinct product option, not an apples-to-apples speedup.

| Evaluation axis | Required controls and observations |
|---|---|
| History scale | Short and long transcripts; equal-sized events; one large early result versus the same result late; many tiny call-start/failure events; realistic native batches. Verify predicted logical byte totals separately from physical I/O. |
| Reader isolation | A progress query while unrelated task B is paused inside publication; same test with a taskless resident service; A with zero matches; same-task reader; unrelated session. Use synchronization barriers rather than fragile sleep-based races. |
| Chat composition | Exercise `runningWorkReport` and real side-turn preparation/decision with offline provider doubles and actual kernel ownership. Measure time to perception admission, decision, status reply and steering application, not just `ProgressForTask`. Test direct Ctrl+X independently. |
| Publication correctness | Previous-reader snapshot remains complete; concurrent same-Store appends retain all successful events and order; failed append consumes no committed sequence; later retry works; initial mode/umask and extensions/numbers survive. |
| Legacy recovery | Existing torn-tail cut points, including multibyte text and an entire event missing the closing tail; read-only search; reopen before append; identity mismatch, malformed middle and duplicate header refusal. |
| Fault boundaries | Inject failures before/during temp write, file sync, close, mode change and replacement; process interruption around publication; directory-sync unsupported/failure separately. Use a filesystem/VM crash harness for any OS/power-loss claim. |
| Platform and storage | Supported Linux and Windows paths, compatible and incompatible open readers, warm/cold cache, local storage and representative slow/storage-pressure profiles. Record filesystem, mount and device conditions. |
| Alternative lifecycle | Bounded queues, clean shutdown and abrupt exit, full-disk backpressure, migration interruption, index/snapshot mismatch, checkpoint/compaction interruption, retention and stale JSON readers. |

A deterministic isolation test should hold B inside an injected storage boundary and prove that A's compact progress is available without releasing B. The baseline should demonstrate the wait with the same fixture; the candidate should not require shortened history, omitted events, disabled synchronization or a different task-selection policy to pass. Service exclusion must remain correct. This is a causal acceptance criterion rather than an arbitrary millisecond threshold; realistic latency budgets still need representative measurements afterward.

Measure append p50/p95/p99 and maximum latency, separate `t.mu` and atomic path-lock wait/hold time, report latency and `turn.decide` hold time, CPU, allocations/GC, peak RSS, logical and physical bytes, synchronization count/time, replacement retry time, peak temporary storage and failure counts. Also measure restart, search and export latency so work is not merely displaced to diagnosis. For queue designs report backlog bytes, oldest pending age and acknowledgement delay. Report no-fault cost separately from fault-conditioned preservation of committed events, duplicates/gaps, misattribution and recovery coverage.

The successful-event oracle should be independent of the file being tested and of `prog.observe`: record storage outcomes and expected per-agent sequences in the harness, then compare recovered history after each fault. Do not call the final in-memory event count durable evidence. Existing durability tests provide meaningful controls but do not establish p99 latency, cross-task lock isolation, every atomic-file failure point or universal power-loss behavior. [Inspected tests][durability-tests], [progress update timing][append].

For any downstream task-quality claim, hold model/task/sampling/tool permissions and total budgets fixed; count input, cached input, cache writes, output/reasoning where available, retries, cancellations, subprocess work and missing-meter uncertainty. The direct modeled savings here are local serialization/I/O and blocked time, not proven token or dollar savings. Better operator responsiveness, fewer diagnostic reruns and less deadline erosion are hypotheses to measure, not results.

## Recommended decision and scope boundary

1. Preserve the existing crash-consistency and recognized-tail-recovery contract.
2. Specify and test task-progress isolation from transcript storage, including taskless services and production side-turn composition, without claiming progress is a durable receipt.
3. Measure history-weighted logical I/O and end-to-end latency on representative long runs. Use the measurements to choose whether short-lock progress alone is sufficient, whether snapshot constant factors matter, or whether a framed log warrants its migration and recovery complexity.
4. If changing publication or acknowledgement, update the session architecture contract first and retain the existing durability controls as independent gates. Do not trade away old-event accessibility, mode/schema compatibility, event ordering or permission/acceptance ownership for a favorable throughput chart.

This note adds a bounded responsiveness analysis and explicit cumulative accounting to an acknowledged storage tradeoff. It does not reopen the repaired in-place-tail-overwrite defect as if still present, claim that all progress paths share a single mutex, or confuse diagnostic transcript state with task completion authority. Root, session, chat and architecture guidance were inspected; all proposals remain distinct from implementation and evidence. [Architecture documentation guidance][architecture], [existing repair][gap].

## Source references

[gap]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/Docs/architecture/session/03-GAP-ANALYSIS.md#L124-L162
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/session/agents.md#L1-L24
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/Docs/architecture/AGENTS.md#L50-L111
[contract]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/doc.go#L1-L30
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/scope.go#L15-L63
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/system/session_log_boot.go#L12-L45
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/agent.go#L8-L40
[inline]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/session/task_executor.go#L420-L470
[subagent]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/session/subagent.go#L318-L345
[broker-start]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/broker/broker.go#L367-L375
[broker-end]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/broker/exchange_log.go#L102-L134
[tool-flush]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/session/executor_tools.go#L931-L1018
[ownership]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/transcript.go#L48-L68
[append]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/transcript.go#L178-L206
[append-open]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/transcript.go#L178-L275
[writer]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/transcript.go#L278-L318
[ordered]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/lines.go#L26-L120
[atomic]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/atomicfile/atomicfile.go#L147-L247
[stripes]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/atomicfile/atomicfile.go#L81-L144
[windows]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/atomicfile/replace_windows.go#L26-L146
[progress]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/progress.go#L106-L156
[service-test]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/progress_test.go#L231-L257
[report]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/cmd/nerd/chat/running_work.go#L31-L103
[background]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/cmd/nerd/chat/background.go#L375-L383
[side-input]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/cmd/nerd/chat/side_turn.go#L383-L419
[side-decision]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/cmd/nerd/chat/side_turn.go#L434-L527
[stop]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/cmd/nerd/chat/side_turn.go#L214-L236
[live]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/progress.go#L158-L209
[failures]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/store.go#L141-L165
[recovery]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/transcript.go#L320-L405
[search]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/search.go#L108-L160
[durability-tests]: https://github.com/theRebelliousNerd/codenerd/blob/e933c03b3b02813c0862a2210a01eec4d9bf95ad/internal/sessionlog/transcript_durability_test.go#L20-L340
[leveldb]: https://github.com/google/leveldb/blob/99b3c03b3284f5886f9ef9a4ef703d57373e61be/doc/log_format.md
[pg-group]: https://www.postgresql.org/docs/18/wal-configuration.html
[pg-ack]: https://www.postgresql.org/docs/18/runtime-config-wal.html#GUC-SYNCHRONOUS-COMMIT
[fsync]: https://man7.org/linux/man-pages/man2/fsync.2.html
