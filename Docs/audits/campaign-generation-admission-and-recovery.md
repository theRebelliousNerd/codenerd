---
doc-class: deep-dive
subsystem: campaign
implementation-status: planned
last-verified: 2026-10-28
verified-against: 24eb5faaa65a4f55cdc6061e809b09fc25d8376a
supersedes: []
---

# Campaign recovery: admit a durable generation, not merely parse a snapshot

## Decision and scope

**Proposal, not an implemented change or an executed incident:** make campaign resume a single, evidence-preserving admission protocol. Every production door should recover and classify the same durable generation before it changes campaign status, hydrates the kernel, or schedules work. Resume must distinguish a valid committed state, an interrupted publication, storage uncertainty, and a legacy snapshot lacking journal evidence. None is equivalent to “JSON parsed.”

This serves the north star's long-running reliability and evidence-backed status. It is not a proposal for arbitrary task-count limits, replacing Mangle decisions with Go decisions, or declaring filesystem persistence equivalent to exactly-once external effects. Go observes storage and executes admitted transitions; the kernel derives whether the evidence and policy permit continuation.

This pass inspected main at the pinned commit, root and campaign guidance, the campaign architecture corpus, the research branch's existing subsystem documents, and the historical journal QA. No code, tests, campaign, model call, or filesystem-failure experiment was run. Existing effect-receipt and verification documents concern what happened to workspace artifacts; this document concerns which **campaign state generation** may be trusted on recovery. Those contracts must eventually join, not be duplicated.

## What is already implemented, and what remains distinct

| Surface | Source-grounded state |
|---|---|
| Durable writer | `saveCampaign` writes a requested journal record, then a verified temporary snapshot and replacement, then a committed record. See [`internal/campaign/orchestrator_lifecycle.go:91–136`, `saveCampaign`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_lifecycle.go#L91-L136) and [`internal/campaign/orchestrator_journal.go:108–155`, `writeCampaignSnapshotAtomic`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_journal.go#L108-L155). |
| Failure visibility | Persistence failure is **not silent**: [`internal/campaign/orchestrator_lifecycle.go:154–165`, `persistCampaign`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_lifecycle.go#L154-L165) emits `EventSnapshotWriteFailed` and returns no error. The writer's failure therefore does not itself stop the caller. |
| Restart normalization | [`internal/campaign/orchestrator_lifecycle.go:171–239`, `resetInProgress`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_lifecycle.go#L171-L239) commits the kernel reset before mutating live statuses; [`internal/campaign/orchestrator_execution.go:72–76`, `Run`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_execution.go#L72-L76) propagates reset rejection. This already-fixed atomicity gap is **not** the proposal here. |
| Durable pause control | [`internal/campaign/pause_durable.go:23–43`, `RequestPause` and `ClearPauseRequest`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/pause_durable.go#L23-L43) use a separate control file rather than overwriting the owner snapshot. Preserve that ownership separation. |
| Operator diagnostics | [`internal/campaign/journal_ops.go:211–285`, `VerifyCampaignJournal`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/journal_ops.go#L211-L285) validates the journal prefix and compares snapshot bytes with the last commit checksum. This is implemented tooling, not an automatic resume prerequisite in the inspected CLI path. |
| Production resume | [`cmd/nerd/cmd_campaign.go:643–743`, `runCampaignResume`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/cmd/nerd/cmd_campaign.go#L643-L743) selects parsed candidates, clears pause, changes paused status on disk before Cortex boot, later calls `SetCampaign`, `PrepareResume`, and `Run`. |

### 1. The CLI is a second snapshot writer

`runCampaignResume` changes a paused snapshot to active at lines 675–685, before Cortex boot can fail at lines 702–705. Its helper [`cmd/nerd/cmd_campaign.go:983–992`, `writeCampaignJSON`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/cmd/nerd/cmd_campaign.go#L983-L992) uses write-temp/rename but neither the campaign journal nor explicit file/directory sync. Failed-campaign resume also uses that helper after `PrepareResume` at lines 732–735.

Static witness: start with a paused snapshot matching a committed journal hash; the early helper succeeds; Cortex boot fails. Snapshot content has changed without its corresponding journal event, and no campaign has started. This is a **conditional reachable sequence**, not a reported production failure. The source establishes the ordering; an authorized fault-injection run would establish actual behavior on a platform.

Candidate loading [`cmd/nerd/cmd_campaign.go:955–980`, `loadResumeCandidates`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/cmd/nerd/cmd_campaign.go#L955-L980) parses JSON without journal admission. [`internal/campaign/orchestrator_lifecycle.go:61–87`, `SetCampaign`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_lifecycle.go#L61-L87) recovers journal sequence, hydrates facts, and saves the supplied snapshot. A successful later save can normalize the discrepancy without preserving an explicit account of why the previous snapshot and journal disagreed. `LoadCampaign` similarly loads snapshot JSON and recovers sequence without checksum admission (same file, lines 14–57).

### 2. A failed append is not necessarily an absent append

[`internal/campaign/orchestrator_journal.go:33–85`, `appendJournalEventLocked`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_journal.go#L33-L85) increments sequence before serialization, open, write, and sync. If an open fails before any bytes, a later successful save can leave a sequence gap. If sync fails, bytes may already be present: blindly decrementing the counter or retrying the same event is not a safe general fix.

The sequence-before-marshal issue was already identified in the repository's [June 13 journal QA, section B1](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/.quality_assurance/campaign/2026-06-13_04-26-53-EST_orchestrator_journal_boundary_analysis.md); this document does not claim its discovery. The refinement is to connect **production continuation after storage failure** with **recovery admission**, distinguish definitely-not-written from uncertain durability, and require a reconcilable operation identity rather than merely moving an increment.

[`internal/campaign/orchestrator_journal.go:157–223`, `recoverJournalSequence`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_journal.go#L157-L223) stops at nonconsecutive sequence/checksum/identity errors, rewrites the valid prefix, discards rewrite errors, and stores the recovered sequence. A transient failed append followed by successful later snapshots can therefore make the later journal suffix unacceptable on reload even when a newer snapshot exists. This is not proof that the snapshot itself is lost. The more exact risk is loss of diagnostic history plus an unresolved relationship between retained journal prefix and admitted snapshot.

Current recovery uses `bufio.Reader`, not the 8 MiB Scanner limitation described in older QA; do not repeat that stale diagnosis. It still retains all valid lines in memory, so startup memory remains proportional to journal size (same symbol, lines 170–213).

Repair itself needs an evidence-preserving boundary. [`internal/campaign/orchestrator_journal.go:225–247`, `writeJournalLinesAtomic`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_journal.go#L225-L247) falls back from failed atomic replacement to opening the original journal with `O_TRUNC`. An interruption or failure during that rewrite can damage even the valid prefix; the caller discards the returned error. Preserve the original bytes before destructive repair, and do not admit a recovered generation unless repair and its required durability barriers actually succeed.

### 3. Publication interruption has more than one outcome

The save protocol permits termination after snapshot replacement but before the committed journal record. In that window the latest snapshot is new, not necessarily the previous committed one. Nevertheless [`internal/campaign/journal_ops.go:393–412`, `RenderJournalReplay`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/journal_ops.go#L393-L412) says an uncommitted last request means resume will fall back to the previous snapshot. The inspected loaders do not implement that general fallback.

[`internal/campaign/orchestrator_journal.go:256–289`, `renameAtomicReplace`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/campaign/orchestrator_journal.go#L256-L289) improves synchronous replacement failure by moving the old file aside and restoring on retry failure. A process death after moving old aside is different: the synchronous retry-error restoration branch is never reached, and the named snapshot may be absent while the backup exists. The inspected candidate loader ignores that backup naming pattern. This conditional window matters on paths taking the fallback; it is not a claim that normal POSIX replacement always has it.

## Proposed contract: one generation admission boundary

Use a versioned record containing campaign identity, generation, parent generation, operation ID, snapshot hash, schema version, and publication phase. A checksum detects accidental inconsistency; it is not authentication against a writer capable of replacing both files.

1. **Acquire and establish ownership.** A resume operation must not race an existing owner. Use the project's ownership mechanism if one exists after broader inspection; otherwise make single-writer admission a separate required capability. An in-process mutex alone cannot establish cross-process ownership. This pass does not claim a complete repository-wide owner-lock audit.
2. **Inspect without destroying evidence.** Read current snapshot, recorded generations, journal prefix, and any replacement candidates. Produce a recovery report before truncation or status writes. Preserve damaged bytes under a bounded retention policy, with explicit disk-full behavior.
3. **Classify.** `committed_match`, `prepared_snapshot_present`, `previous_generation_present`, `conflict_or_corruption`, `storage_unreadable`, or `legacy_unjournaled`. Neither parse success nor absence of an error is an admission verdict. All labels here are proposed observations, not existing predicates.
4. **Derive the action.** Feed classification and operator policy into Mangle. A verified generation may hydrate; an interrupted publication may be reconciled under an explicit rule; conflicting candidates block effects. Legacy import creates an explicit provenance boundary rather than manufacturing prior verification.
5. **Publish a transition once.** All doors submit resume intent to the same owner. Only the owner changes durable status, using a unique operation ID and expected parent generation. Record `resume_requested` separately from `active`; `active` should not imply work began merely because boot was attempted.
6. **Separate durable and applied watermarks.** Let `D` be the durable accepted generation and `K` the generation hydrated into the kernel. Dispatch requires `D = K` and current authorization. After durable publication but before hydration, restart can retry hydration idempotently. On kernel rejection, retain the durable pending transition and return its cause; do not schedule or label the runtime active. A prepared generation is not silently promoted to completed.
7. **Reconcile uncertain appends.** After partial-write/sync uncertainty, stop extending the same log until inspection identifies whether the operation ID exists in a validated prefix. Readable bytes are not proof of durability: accepting `D` or resuming dispatch additionally requires successful re-sync or a successful transactional commit, including applicable directory sync and the chosen platform's stated durability contract. If those barriers remain unavailable or fail, preserve the uncertain classification. Do not assume rollback of an atomic counter repairs disk state. Error-returning reconciliation must complete before sequence reuse.

Two reasonable storage designs should remain open: immutable generation snapshots plus a small manifest and explicit publication state; or a transactional SQLite record holding snapshot bytes and transition metadata, with JSON as an export. Neither makes arbitrary workspace writes atomic with campaign metadata. Interrupted tasks still need effect reconciliation and current evidence checks before replay; use the separate effect-receipt contract rather than promising exactly-once task execution.

### Wiring obligations

- CLI candidate selection becomes discovery only; no direct status rewrite before admission. Attach at `runCampaignResume` and replace the writer bypass with owner transition submission.
- `SetCampaign` remains useful for creating a new campaign, but imported or recovered state needs a distinct provenance-aware entry path. Do not let “set” erase a mismatched prior generation before reporting it.
- Existing `persistCampaign` should publish storage-health observations and last durable generation. Kernel policy can halt new mutation dispatch on uncertainty while allowing observation, diagnosis and already-owned safe shutdown; this is evidence-based containment, not an arbitrary duration limit.
- Journal verify, replay, resume, and UI must consume the same classifier. Show “last requested,” “last committed,” “loaded generation,” and “unknown” separately.
- Preserve the already-correct pause ownership and reset transaction behavior. This proposal is not permission to rewrite those protections.

## Costs, efficiency, and alternatives

For `W` snapshot saves of mean serialized size `S`, the current writer performs about `W*S` snapshot bytes written, `W*S` verification bytes read, and two journal records per save; hashing and serialization are linear in snapshot bytes. The normal path explicitly syncs both journal writes and the snapshot file, plus supported directory syncs. The source makes no platform-independent latency guarantee. Let journal bytes be `J`; current prefix recovery costs `O(J)` time and retains `O(J)` bytes of valid lines before a repair rewrite. These costs occur while the orchestrator lock protects persistence at its callers.

An immutable-generation design retaining `R` snapshots costs approximately `R*S + J` storage, plus manifest writes, candidate validation, and garbage collection. Transactional storage adds database/page/WAL overhead, checkpoint work, migration cost, backup/restore requirements and contention; do not call it cheaper without measurement. A retained recovery index can reduce ordinary startup scanning only if its hash/version is validated and the remaining tail is reconciled; corrupted-index fallback must still be budgeted.

Report total cost as

`C_total = C_original_work + C_persist + C_recovery + C_replayed_work + C_reverification + C_operator_repair`.

Track input/output/cache tokens and model calls separately for original and repeated work; add bytes, sync latency, CPU, peak memory, wall time, and human review minutes. A durability check can increase startup latency while reducing expensive repeated agent work. Compare verified completed campaigns per full cost, not journal writes per second or tokens of successful survivors only. Adaptive checkpoint batching must state the recoverable-work window and cannot acknowledge a generation before the configured durability contract holds.

Primary-source basis: [SQLite atomic commit](https://www.sqlite.org/atomiccommit.html) explains that crash-safe publication depends on an explicit protocol and storage assumptions, not merely a temporary filename. [SQLite WAL](https://www.sqlite.org/wal.html) separates durable log commit from checkpointing and documents same-host/network-filesystem constraints; using it is a design alternative, not a universal recommendation. [Go `os.Rename`](https://pkg.go.dev/os#Rename) documents platform-specific restrictions and non-Unix atomicity limitations. These established techniques motivate the contract; no new storage algorithm is claimed.

## Failure matrix and fair evaluation plan — not executed

Use the same campaign workload, models, tool permissions, task parallelism, retry policy, and acceptance criteria for baseline and candidates. Include production CLI resume, not only direct writer unit tests. Deterministic short traces establish protocol correctness; separately measure real workloads and storage environments.

| Injection or case | Required observation |
|---|---|
| Paused CLI resume, successful early write, failed Cortex boot | No ownerless “active” publication; old durable generation preserved; boot failure reported. |
| Journal open failure before bytes, then later successful attempt | No unclassified sequence gap or accepted suffix later discarded; operation outcome explicit. |
| Partial write or file/directory sync failure | Uncertainty reported; no blind sequence decrement; subsequent dispatch waits for reconciliation. |
| Termination before rename, after rename, before commit marker | Each window classified correctly; no promise of previous-state fallback when new bytes are present. |
| Fallback old-to-backup move followed by death | Recovery finds and validates candidates without silently skipping the campaign. |
| Valid JSON with wrong snapshot hash; wrong campaign; altered generation | No hydration or overwrite before evidence report. |
| Corrupt journal tail and failure to preserve/repair it | Admission returns the repair/storage error; no sequence reset pretending repair succeeded. |
| Duplicate resume requests or two owners | One admitted transition; stale expected-generation writer refused; no lost update. |
| Kernel hydration rejection after durable transition | No task effects; durable/applied watermarks remain distinguishable and retryable. |
| Long journal, large campaign, low free space, platform sync limitations | Bounded stated resource behavior; limitations visible rather than a false durable-success label. |

Measure acknowledged-generation loss, unclassified recovery, duplicate task effects, stale evidence reuse, recovery completeness, and operator diagnosis accuracy. False-positive blocks and time-to-recover count alongside safety failures. Count every injected failure and abandoned campaign in the denominator. Evaluate no-fault overhead separately from fault-conditioned benefit; report paired distributions and confidence intervals, not unsupported universal speedups.

## Buildable gap queue

| Proposed gap | Exit criterion |
|---|---|
| GAP-CAMPAIGN-RECOVERY-ADMISSION | Every production resume door classifies snapshot/journal provenance before status mutation, hydration, or dispatch; mismatched generations cannot schedule. |
| GAP-CAMPAIGN-SINGLE-PUBLISHER | CLI early boot failure leaves prior durable status intact; owner transitions use one publication contract and expected generation. |
| GAP-CAMPAIGN-UNCERTAIN-APPEND | Before-write and after-write failures have distinguishable reconciled outcomes; no successful suffix disappears merely because a failed attempt consumed a sequence. |
| GAP-CAMPAIGN-RECOVERY-EVIDENCE | Failed repair is surfaced; damaged suffix retained or explicit retention failure reported; verify/replay/resume agree on the selected generation. |

Acceptance requires later authorized implementation and fault-injection evidence. This document provides neither an executed test result nor a claim that CodeNERD is currently lossless across crashes.
