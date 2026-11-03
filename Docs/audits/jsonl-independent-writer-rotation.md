---
doc-class: deep-dive
subsystem: measurement-jsonl
implementation-status: proposed
last-verified: 2026-11-02
verified-against: f2b6d19aa415cfef10ba64125adfbfd580117c46
verification-method: static-source-inspection
---

# JSONL rotation ownership across independent writers

## Decision and evidence boundary

Keep bounded measurement retention, but make append and rotation share an explicit cross-writer ownership contract. The current JSONL appender keeps an open descriptor and private size accounting. A different appender can rotate the pathname twice while that descriptor remains open. On a local POSIX filesystem, the quiet appender can then successfully write a **new** record to an unlinked old file, without incrementing its error or dropped-generation counters. The normal reader, which opens only the current pathname and its `.1` backup, cannot retrieve that record.

This is distinct from deliberately retiring old history. In the witness below, the missing record is created **after** the old generation has been removed. It never enters either pathname the reader enumerates. No crash, malformed JSON, failed write, simultaneous rename, or provider failure is required.

The review pins `theRebelliousNerd/codenerd` main at [`f2b6d19aa415cfef10ba64125adfbfd580117c46`][baseline]. All behavioral claims are source-derived and conditional. No application, inference, tests, benchmarks, failure injection, or repository changes were performed. The only authored artifact is this scratch audit. Existing tests are inspected coverage, not passes established by this review.

## 1 Scope and prior work

The affected implementation is `internal/jsonl.Appender`, used by broker receipts and prompt-atom selection journals. It is not every JSONL writer in the repository. Glass Box NDJSON, tactile audit logs, general logging rotation, and session transcripts have separate implementations and contracts.

The appender's own comment says independent processes in one workspace may append without coordination, accepts loss of a crash tail, and calls the data measurement rather than a ledger of record. The broker architecture separately declares older receipt retention intentionally lossy. Those are legitimate choices, but neither describes a live writer sending fresh records into a file that normal readers cannot name. [Appender contract][appender], [retention contract][retention].

This audit preserves and distinguishes the existing work:

- The co-use observation-budget audit concerned resident atom/category state. Current code has an atom limit and refusal/truncation accounting; the queue marks that work fixed. Disk-generation identity is a different resource boundary. [Prior co-use audit][prior-couse], [current resident controls][couse-bounds], [audit queue][queue].
- Detailed usage work added normalized accounting, a settlement barrier, and `FileSink.Completeness` using write failures and overwritten-generation counts. Those additions remain useful. The present issue is the **scope** of those counters when another writer owns rotation. It is not a renewed claim that provider cache detail universally disappears. [Usage audit as built][prior-usage], [sink completeness][filesink].
- The Windows atomic-replacement and reader-sharing repairs are present. They let cooperating readers coexist with replacement; they do not notify all appenders that their destination inode has changed. [Atomic replace][replace], [Windows reader][windows-open].
- Sessionlog durability/progress and Glass Box observer-isolation audits address other storage and latency boundaries. Neither establishes the cross-process append/rotate contract examined here. [Sessionlog audit][prior-sessionlog], [Glass Box audit][prior-glassbox].

A bounded audit-directory search found those adjacent discussions, not a dedicated independent-writer rotation witness. This is not a claim of exhaustive historical coverage. Root guidance calls for specification-first changes and actual production wiring evidence; scoped system guidance requires explicit handle ownership and rollback. [Root guidance][guidance], [system guidance][system-guidance].

## 2 Actual producers and readers

### Broker receipts

Normal shared chat boot calls `BootCortexWithConfig`. Its perception-initialization step calls `configureBrokerMeter` and `configureCoUseLog` before constructing clients. Receipt configuration opens `.nerd/meter/receipts.jsonl` under the workspace with `broker.NewFileSink`, which calls `jsonl.Open`. If opening fails, boot logs the failure and retains in-process metering rather than refusing the session. This explicit degraded mode is not the silent-success witness below. [Chat boot][chat-boot], [factory wiring][factory], [log configuration][boot-logs], [FileSink][filesink].

The client factory installs the broker; `InstallBroker` obtains `broker.Default().ConfigFor`, whose sink is the configured fan-out. During settlement, the broker records spend in its ledger and emits a receipt; `MultiSink.Record` invokes its sinks synchronously, including the ring and file sink. `FileSink.Record` calls `Appender.Append`. Consequently, a legitimate provider response can finish and its process-local accounting can advance even if its journal destination has become detached from the live pathname. This does not imply inference was unmetered in memory or that execution authorization changed. [Factory metering][client-factory], [broker installation][install], [sink composition][default-meter], [settlement][settle], [fan-out][receipt].

The separate `nerd meter` command resolves the workspace path and calls `ReadReceiptLog`; `nerd meter epochs` uses those receipts for its analysis. These readouts consume disk history, not the other process's ring or ledger. Missing positive-spend receipts can reduce a disk-derived sum. The direction of resulting distributions or model-comparison bias is workload-dependent, not a measured saving. [Receipt readout][meter-read], [epoch analysis][meter-epochs].

### Atom co-use selections

The compiler's successful `Compile` result calls `CoUse().ObserveAtoms`, including the cache-aware entry path. The session executor establishes a turn identity and defers `CoUse().Settle` with an outcome derived from the turn's result and error. Settlement folds the selected atoms into resident tallies, then appends each settled selection outside the tally mutex. `appendToLog` writes a `SelectionRecord` with time, outcome, IDs, and categories through the same JSONL appender. Both success and failure outcomes can be persisted; the journal is not merely a log of successful turns despite some introductory comments. [Compilation observation][compiler], [executor settlement][executor], [recorder settlement][couse-settle], [selection persistence][couse-persist].

`nerd meter atoms` opens `.nerd/meter/atom-selections.jsonl` via `LoadSelections`, replays records into a fresh recorder, and computes its report there. A record absent from the named generations is absent from that replay even if the producing process already updated its live tally. That can change the sampled selection/outcome population; no universal direction of lift or category-alignment bias follows. [Replay][couse-replay], [atom readout][meter-atoms].

These are **two separate files sharing one mechanism**, not receipts and selections interleaved in a single file. The strongest production condition is two live processes using the same workspace journal path. Process-local singletons, replacement-close behavior, and identity-conditioned detach reduce some same-process ownership failures, but do not coordinate independently running processes. No second process or existing workspace workload was run for this audit.

## 3 The mechanism and the conditional POSIX witness

### State belongs to an object rather than the named journal

`jsonl.Open` opens the pathname with `O_CREATE|O_WRONLY|O_APPEND`, samples its size with `f.Stat`, and retains that descriptor. Each appender has its own mutex, `written`, threshold, first error, error count, and dropped-generation count. `Append` marshals a record and newline, then under that object's mutex:

1. Returns if its handle is nil.
2. Rotates only if its private `written + len(line)` exceeds its threshold.
3. Writes through the retained handle and adds the returned byte count to its private `written`.
4. Records an error only when marshaling, rotation, or writing reports one.

There is no pathname-versus-descriptor identity check before an ordinary append, no shared size refresh, and no cross-process transaction around append plus rotation. [Open and append][appender].

Rotation closes **that appender's** handle, counts an existing `.1`, replaces `path` with `path.1`, opens a new current file, and resets its private size to zero. `atomicfile.Replace` has a process-local striped destination mutex, but that mutex covers replacement, not all appends or the entire close/replace/reopen sequence. On non-Windows platforms its replacement function is `os.Rename`. The witness needs no concurrent replacement calls and therefore does not depend on defeating the striped lock. [Rotation][rotate], [replacement lock][replace-lock], [POSIX implementation][replace-posix].

### Ordered witness with no failing system call

Assume a writable local POSIX filesystem with ordinary rename/open-file semantics, no extra hard links to the initial journal, and successful operations. Let `P` be one journal path and `I0`, `I1`, `I2` different file identities. Begin with no backup for clarity.

| Step | Named current P | Named backup P.1 | A handle | B handle |
|---|---|---|---|---|
| Both independently open the initial file | I0 | absent | I0 | I0 |
| A completes its first rotation and writes | I1 | I0 | I1 | I0 |
| A fills its new generation and completes its second rotation | I2 | I1 | I2 | I0 with no name |
| B appends a fresh marker below its private threshold | I2 | I1 | I2 | marker appended to unnamed I0 |

A and B have independent descriptors referring initially to the same file; they need not share a numeric descriptor or an open-file-description object. B remains quiet while A writes enough to complete the two rotations. If B's cached size is `sB`, its next serialized record has byte length `r` including newline, and its threshold is `M`, require **`sB + r <= M`**. Otherwise B takes its own rotate/reopen path first and this exact witness does not apply.

At the first rename, B's open reference still targets I0, now the backup. At the second rename, replacing that backup removes I0's final directory link, but B's descriptor remains valid. Linux documents that rename does not retarget existing descriptors; POSIX unlink semantics preserve the file contents while open references remain. Append mode chooses the end of the **referenced file**, not the newest file now named P. Thus a full successful B write can land in I0 after both rotations. [Linux rename][os-rename], [open-file identity][os-open], [POSIX unlink][os-unlink], [POSIX append semantics][os-write].

Now pause all writers and call the production reader. `jsonl.Read` opens P.1 then P by name, obtaining I1 and I2. It has no reference to I0 and no record index identifying B's missing marker. If the retained generations contain valid JSON, the reader can return other records, zero truncations, and no error while omitting the newest B record. This claim requires no race during the read. Closing B later releases its old reference; it does not copy the marker into a retained generation. A preexisting external reader or forensic descriptor access is outside this named-reader contract, so this is not a claim of impossibility of all recovery. [Reader][read].

### Why the existing loss counters do not describe the same thing

For B, the marker's write can succeed, `written` advances, `Failures()` remains `(0,nil)`, and `Dropped()` remains zero because B performed no rotation. A `FileSink` wrapping B consequently returns a `Completeness` whose `Complete()` is true, even though a receipt it was given cannot be returned through the normal file reader. There is no public successful-record counter in `Appender`; the precise observations are successful underlying write/private byte accounting and unchanged failure/drop counters. [Appender accounting][appender], [counter accessors][counters], [completeness implementation][filesink].

**A does not also report a complete history:** in this sequence its second rotation observes an existing backup and increases its dropped count. That correctly records intentional retirement of an old generation. It does not identify B's later, fresh write, and A's in-memory counter is not a durable workspace-wide health manifest read by `nerd meter`.

The freshness distinction is decisive: old records already in I0 may legitimately age out when A replaces the backup. B's new marker did not exist at that cut. Treating its disappearance as ordinary old-history retention makes retention depend on a stale descriptor's age rather than the newly appended record's publication.

## 4 Boundaries and protections to retain

- **One appender with many goroutines:** its mutex serializes its own size check, rotation, and append. This is a useful positive control, not the independent-writer case.
- **Sequential close and reopen:** opening anew sees the current pathname. The stale-handle interval ends when B successfully reopens, including its own successful rotation, but missing earlier records are not reconstructed.
- **Only one external rotation:** B still writes into a named backup. That does not establish global chronological ordering, but it is not yet the unnamed-file witness.
- **No rotation during overlapping lifetimes:** the two-rotation failure condition is absent. Ordinary append behavior still depends on the filesystem and write contract.
- **Explicit failures:** marshal/write/rotation errors are counted; readers disclose malformed generations, distinguish missing files from other open errors, and retain records decoded before damage. Go's `File.Write` reports an error for an incomplete write, so the defect is not an ignored nil-error short-write assumption. [Error and read handling][counters], [reader][read], [Go Write contract][go-write].
- **Lifecycle repairs:** receipt sink replacement closes the old sink, detach compares installed identity, and Cortex close drains broker settlement before detaching. Co-use log replacement/identity-conditioned detach likewise closes owned handles. These controls are local lifecycle protections, not cross-process rotation locks. `Drain` observes settlement completion; it neither verifies pathname reachability nor performs a storage sync. [Meter lifecycle][meter-lifecycle], [Cortex close][close], [co-use lifecycle][couse-persist], [drain][drain].
- **Platform scope:** Windows uses different replacement APIs and reader sharing flags. This audit does not transfer the successful-unlinked-write witness to Windows. Network filesystems require their own matrix; Linux explicitly warns that concurrent `O_APPEND` on NFS can have different guarantees. No all-platform line-atomicity or crash-durability conclusion is established. [Windows implementation][windows-replace], [Windows reader][windows-open], [Linux open][os-open].

Private size accounting also does not enforce a strict aggregate-file byte cap: each writer advances its cached count only for its own writes after opening. An oversized individual record is written after at most one rotation and is not split or rejected by the threshold. Therefore neither the default 20 MiB threshold nor two pathnames proves a hard 40 MiB total bound, particularly with open unlinked files consuming storage. These are cost/accounting qualifications, not a measured disk-exhaustion incident. [Threshold and writes][appender], [rotation][rotate].

## 5 Proposed contract and repair choices

First specify what a completed append promises. For best-effort measurement it can remain weaker than crash durability, but it should distinguish:

- accepted for processing;
- successfully written to a reader-discoverable generation;
- deliberately retired by the retention policy;
- dropped or failed before publication;
- crash-durable, if separately requested.

A successful fresh append should enter a discoverable generation before later retention is allowed to retire it. No per-object zero-error observation should certify workspace-wide completeness. A readout should state its coverage window and known loss/uncertainty rather than interpret absent evidence as zero activity.

### Coordinated shared journal

One option is a stable sidecar lock shared by **all** appenders, enclosing active-file validation/open, size observation, any rotation, and the record write. Under that lock, a cached descriptor must be checked against the current generation and reopened when stale; alternatively open/close the current file for each transaction. Retaining the old handle under a newly added lock is not enough. Merely checking identity before writing without shared exclusion also leaves a check-to-rotation race.

The lock must not itself be renamed or deleted during rotation, or writers can lock different underlying objects. Lock-domain identity must account for supported path aliases and filesystem semantics. Advisory coordination only protects cooperating writers; rolling upgrades with an old unlocked writer need an explicit compatibility policy. The repository already has `internal/filelock` implementations for POSIX and Windows, but their `Acquire` blocks without a context. Reusing them on the receipt completion path is therefore a design decision, not a free fix. [Existing file-lock API][filelock], [Windows lock API][filelock-windows], [advisory lock semantics][os-flock].

A read-consistent two-generation snapshot is a separate requirement. If needed, readers must participate long enough to capture stable generation references or use a manifest protocol. Holding a global lock through decoding an entire journal would introduce avoidable writer delay. Sorting surviving rows by timestamp alone neither recovers missing rows nor establishes cross-process causal order.

### Independent writer generations or a single owner

Another option is writer-specific, uniquely identified segments, with readers merging an explicit set and retention governed across that set. This avoids different writers replacing each other's current file, but creates reader enumeration, ordering, stale-writer cleanup, aggregate storage budgeting, identity, and compatibility obligations. Per-process caps without a workspace cap multiply disk use as process count grows.

A single writer service can own append and retention, but introduces IPC availability, queue limits, acknowledgments, restart recovery, and a process lifecycle. Moving disk work to a bounded queue does not alone fix independent rotation; the eventual writer still needs sound generation ownership. It also changes what `Record` returning and `Drain` mean. Queue admission is not persisted publication.

For every option, retain prompt outcomes, receipt usage metadata, existing reader corruption disclosure, and Mangle-controlled execution policy. A journal outage should not silently become an instruction to replay an already-completed model or tool action.

### Completeness and durability must be separate

Writer IDs and monotonically increasing sequence numbers, coupled with generation/retention metadata, can make gaps and intentional retirement explainable. Such metadata needs its own atomic update and crash-ordering contract; a nontransactional sidecar can disagree with data. Local counters remain useful diagnostics but should be named and documented as local. Cross-process readouts cannot infer a complete census merely because every observed row parsed.

Adding `fsync` does not repair writing to the wrong inode. Nor does a successful append currently mean crash persistence: the appender intentionally uses no sync. If a stronger mode is required, specify data and namespace synchronization separately, including rotation/recreation and recovery ordering. Linux documents that syncing a file does not by itself ensure its directory entry is durable. Retain the cheaper best-effort mode if it matches the measurement need; do not silently convert this repair into a financial-ledger durability promise. [Current durability choice][appender], [file and directory sync][os-fsync].

## 6 Costs and fair evaluation

The baseline is not cost-free or strictly nonblocking. JSON marshaling allocates a whole line; append holds its mutex across synchronous file I/O, and rotation adds close/stat/replace/open work. Receipt fan-out is on completion, while co-use persistence is outside the tally mutex but still runs synchronously from settlement. Eliminating `fsync` reduces one class of cost; it does not make arbitrary filesystem latency bounded. [Append][appender], [receipt sink contract][receipt], [co-use settlement][couse-settle].

A repair must account for lock wait and fairness, extra open/stat/close calls, filesystem metadata contention, sync latency where chosen, queue memory and drops, batching delay, additional segment/manifest bytes, reader merge and decoding cost, shutdown drain, crash recovery, mixed-version deployment, and Windows/network-filesystem behavior. Correctly retaining records may increase storage and reported token totals compared with a baseline that loses them. That is restored evidence, not an efficiency regression or an inference-cost reduction.

Proposed evaluations are **unrun**:

| Evaluation | Control and acceptance evidence |
|---|---|
| Deterministic stale-handle witness | Use two independently opened appenders with explicit barriers: both open I0, A completes exactly two rotations, B writes its fresh marker with cached size plus line at or below threshold, then stop mutation and read. Record each writer's errors/drops and current/backup identities. Require the repaired contract to preserve the fresh marker or explicitly reject/drop it with observable status, not silently claim complete publication. |
| Separate-process coverage | Repeat with two processes and the real appender/file sink, not two goroutines sharing one object. Keep an independent expected-record ledger for the harness. Do not mistake one process's path mutex for cross-process exclusion. |
| Receipt and atom integration | Exercise production boot wiring with deterministic provider/selection inputs and the normal disk readers. Verify live accounting, named-file replay, truncation flags, and coverage status independently. No live inference is needed to establish file identity behavior. |
| Positive controls | Preserve one-appender concurrent writes, sequential reopen, no rotation, one rotation, normal old-generation retirement, reader-held backup, marshal/write errors, and normal close/detach behavior. Add B-at-threshold and B-over-threshold cases to enforce the witness boundary. |
| Failure and lifecycle matrix | Delay or fail lock acquisition, replace, open, write, metadata publication, and optional sync; cancel shutdown or kill a writer at defined stages. Assert the selected loss/durability contract, owned-handle cleanup, bounded queued state, and recoverability. Test supported Windows and filesystem modes separately. |
| Performance comparison | Hold record sizes, writer counts, byte thresholds, retention horizon, sync policy, disk/cache state, payloads, and delivered-record coverage constant. Report producer latency distributions, completion/settlement delay, throughput, CPU, allocations, RSS, disk bytes, lock waits, drops, replay time, and shutdown time. |

The present test suite contains useful but narrower witnesses: concurrent append workers share one `Appender`; the reopening test closes A before opening B; repeated rotation and reader-held backup tests use one writer. `FileSink.Completeness` is tested with one sink doing its own rotations. None of those inspected cases creates the ordered independent-writer sequence above. [JSONL tests][tests], [completeness test][sink-test].

For downstream model or prompt evaluations, compare equally covered samples and report incomplete telemetry separately. Do not reward a variant for apparently lower spend caused by missing receipts, or claim better co-use structure from a selectively missing population. If the workload later requires live-model evaluation, include failed attempts, retries, repair calls, preparation, wall time, and task-quality outcomes; this source audit establishes none of those gains.

## Acceptance boundary

A focused repair is complete when independent writers cannot silently publish fresh records outside the reader's discoverable generation set, intentional retention is distinguishable from publication failure, local counters do not overclaim global completeness, and the chosen latency/storage/durability policy is explicit and tested through the real receipt and selection paths. This preserves bounded telemetry without demanding infinite retention, universal crash durability, or a change to execution authority.

[baseline]: https://github.com/theRebelliousNerd/codenerd/tree/f2b6d19aa415cfef10ba64125adfbfd580117c46
[appender]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/jsonl/jsonl.go#L24-L113
[retention]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/Docs/architecture/broker/WIRING-AND-NOT-BUILT.md#L95-L123
[prior-couse]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/Docs/audits/jit-co-use-observation-budget.md
[couse-bounds]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/prompt/couse.go#L29-L281
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/Docs/audits/ASTRA_QUEUE.md#L25-L38
[prior-usage]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/Docs/audits/detailed-usage-wiring-and-verified-efficiency.md#L113-L138
[filesink]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/broker/filesink.go#L16-L95
[replace]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/atomicfile/atomicfile.go#L215-L224
[windows-open]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/atomicfile/open_windows.go#L10-L35
[prior-sessionlog]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/Docs/audits/sessionlog-durability-progress-responsiveness.md#L1-L24
[prior-glassbox]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/Docs/audits/glassbox-sink-isolation-and-export-health.md#L1-L21
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/agents.md#L15-L110
[system-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/system/agents.md#L1-L28
[chat-boot]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/cmd/nerd/chat/session_shared_boot.go#L89-L111
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/system/factory.go#L1086-L1099
[boot-logs]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/system/broker_meter.go#L28-L129
[client-factory]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/perception/client_factory.go#L830-L845
[install]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/perception/broker_install.go#L21-L46
[default-meter]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/broker/default.go#L153-L175
[settle]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/broker/broker.go#L215-L287
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/broker/receipt.go#L9-L85
[meter-read]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/cmd/nerd/cmd_meter.go#L18-L110
[meter-epochs]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/cmd/nerd/cmd_meter.go#L390-L415
[compiler]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/prompt/compiler_pipeline.go#L16-L31
[executor]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/session/executor.go#L1375-L1396
[couse-settle]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/prompt/couse.go#L204-L236
[couse-persist]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/prompt/couse_persist.go#L17-L114
[couse-replay]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/prompt/couse_persist.go#L111-L148
[meter-atoms]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/cmd/nerd/cmd_meter.go#L502-L543
[rotate]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/jsonl/jsonl.go#L115-L145
[replace-lock]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/atomicfile/atomicfile.go#L118-L145
[replace-posix]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/atomicfile/replace_other.go#L1-L11
[read]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/jsonl/jsonl.go#L196-L255
[counters]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/jsonl/jsonl.go#L147-L193
[meter-lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/broker/default.go#L112-L165
[close]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/system/cortex_close.go#L283-L319
[drain]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/broker/inflight.go#L8-L73
[windows-replace]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/atomicfile/replace_windows.go#L26-L85
[filelock]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/filelock/lock_other.go#L10-L33
[filelock-windows]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/filelock/lock_windows.go#L20-L45
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/jsonl/jsonl_test.go#L66-L412
[sink-test]: https://github.com/theRebelliousNerd/codenerd/blob/f2b6d19aa415cfef10ba64125adfbfd580117c46/internal/broker/filesink_test.go#L123-L143
[os-rename]: https://man7.org/linux/man-pages/man2/rename.2.html
[os-open]: https://man7.org/linux/man-pages/man2/open.2.html
[os-unlink]: https://man7.org/linux/man-pages/man3/unlink.3p.html
[os-write]: https://man7.org/linux/man-pages/man3/write.3p.html
[go-write]: https://pkg.go.dev/os#File.Write
[os-flock]: https://man7.org/linux/man-pages/man2/flock.2.html
[os-fsync]: https://man7.org/linux/man-pages/man2/fsync.2.html

