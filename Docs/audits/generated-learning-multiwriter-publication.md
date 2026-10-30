---
doc-class: deep-dive
subsystem: autopoiesis-system
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
supersedes: []
---

# Preserve generated learning across independent publishers

## Finding and scope

Two independently initialized autopoiesis learning stores for the same workspace can each successfully publish a generated execution, while the later publication removes the earlier execution from the durable snapshot. The writes need not overlap. Each store starts with its own snapshot, modifies that snapshot, and atomically replaces the common file without first adopting intervening committed events.

The important distinction is **successful atomic publication by one instance versus preservation of acknowledged events across later independent publishers**. This finding does not dispute that the first acknowledgment followed a real successful write. It shows why that acknowledgment alone does not establish that an unrelated later publication will retain its record. No retention policy, deletion, corruption, crash, malformed identity or failed validation is needed in the narrow witness.

The current generated execution route, exact admission, live deduplication, synchronous feedback and atomic-file protections already work at their stated boundaries. Preserve them. The proposed repair is authoritative event admission and publication across store instances, not another execution backend or an exactly-once claim for external effects.

**Evidence status:** static source inspection of public main [3882051][main], rechecked unchanged at approximately 11:41 UTC and 11:45 UTC on October 30, 2026. An independent source challenge supported the mechanism and identified the prior-art qualifications credited below. No application, model inference, tests, benchmarks or repository modifications were performed. The witness is a source-derived sequence, not a reproduced incident. No observed data loss, repeated native effect, coding-quality regression or latency measurement is claimed.

## Prior work and the distinct boundary

The [1117 generated receipt retention study][retention] addresses completed body residency in the Cortex adapter and learning store, together with full-history rewrite cost. It explicitly leaves cross-process coordination separate. This report concerns **event conservation between publishers**, even for two tiny records and a short-lived run. A bounded hot cache would not repair it, and a serialized transaction would not by itself solve the retention study's memory bounds.

The [generated observation fidelity audit][observation] credits partial-return and stderr-delivery repairs. No diagnostic projection defect is needed here. The [May feedback QA][historical] already identified synchronous whole-map saving under a lock and unsafe getter aliases in an older implementation; it did not establish the current two-instance acknowledged-event overwrite. Do not present generic JSON rewrite cost as a new discovery or replace today's durable acknowledgment with its suggested asynchronous dirty flush.

The generic distinction between atomic replacement and serialized read-modify-write is also already documented by the [edit-observation precondition audit][edit-prior]; its as-built ledger credits the repair for cooperating CodeDOM writers. The [usage TODO][usage-prior] records a cross-process read-merge-write repair, and current usage code implements a file lock plus disk reload and baseline/delta aggregation. ProfileStore has a related protocol, discussed below. This repository does not generally lack cross-process locks or awareness of lost updates. The distinct finding is their absence from the **mandatory generated LearningStore event/acknowledgment boundary**, not a new general concurrency principle.

Current [autopoiesis specification][spec] requires a correlated, synchronously published learning record for each started generated execution and explicitly excludes a general durable crash/restart exactly-once claim. The proposed conservation contract is narrower than such a guarantee. Main's audit queue and active research branch additions through [research-main-20261030-1117][research] were inspected for overlap. The latest branch contains documentation only; no inspected addition implements this publication protocol.

## Production ownership and reachability

### Independent owners can address one durable file

The factory creates a new orchestrator for a booted Cortex and installs its generated executor in VirtualStore. `DefaultConfig(workspace)` sets `ToolsDir` to the workspace's `.nerd/tools`, and each orchestrator constructs a new `LearningStore` below that directory at `.learnings`. The constructor allocates separate maps and calls `load()` once. Consequently, independently booted owners of the same workspace address the same `tool_learnings.json` while holding independent store mutexes and snapshots. [Factory][factory], [default path][default-path], [store construction][construction], [store initialization][store-init].

`GetOrBootCortex` is not a workspace-wide interprocess singleton. Its cache and semaphore are package globals. In one process, equal normalized configuration/credential keys reuse an existing Cortex; changed effective configuration or credentials produce another key and boot another instance. The source explicitly acknowledges that several Cortex instances can be live simultaneously. Across separate CLI processes the cache is independent. The normal chat boot calls `GetOrBootCortex` and connects `SessionExecutor.Process`, rather than obtaining a shared autopoiesis store service. [Cache admission][cache], [multi-instance ownership comment][multi-instance], [normal chat entry][chat].

The conditional production witness requires two such owners to boot successfully against the same workspace before either publishes the new events. It does not claim every invocation creates a second owner, or that every environment permits every pair of concurrent boots. Same-key calls returning the same cached Cortex are a negative control. Two owners using different workspaces are another.

### Both events still need ordinary generated-tool admission

For a legitimately registered and offered generated tool, the session invokes VirtualStore's typed generated bridge. VirtualStore checks request, host executable/protocol identity, exact authorization and preflight before calling the backend. The adapter validates the joined process result and synchronously records feedback. Session success requires a complete matching validation and durable acknowledgment. [Consumer][consumer], [bridge][bridge], [adapter][adapter].

`RecordGeneratedExecution` captures the request fingerprint, canonical input, output streams, process outcome and validation result; `RecordExecutionDurable` passes that event to the store. The lost-update witness starts after these genuine prerequisites succeed. It needs distinct legitimate execution IDs, not forged same-ID requests, simultaneous tool execution or permission widening. Native effects can occur sequentially. [Receipt conversion][conversion], [durable handoff][handoff].

The existing real-boot fixture explicitly admits a harmless generated tool, calls `Process` and reopens the learning store. It is a useful acceptance starting point, but its explicit catalog and permission setup must not be misrepresented as evidence that every default request offers generated tools. This review read that test; it did not run it. [Normal boot fixture][boot-test].

## Exact publication mechanism

`RecordLearningDurable` takes `ls.mu`, checks `ls.loadErr`, and checks duplicate/conflicting execution identity against **that instance's** `ls.executions`. It clones that instance's `ls.learnings`, updates the aggregate, copies its acknowledgment and feedback maps, and inserts the new record. It then JSON-encodes a complete Version-2 snapshot and passes those already-computed bytes to `saveBytes`. Only a successful save updates the instance's live maps. There is no reload of the shared file or generation comparison between lock acquisition and publication. [Local admission and aggregation][local-admission], [complete snapshot publication][publication].

`saveBytes` creates the directory and calls `atomicfile.WriteFile`. That helper deliberately provides a complete replacement rather than a torn file: unique temporary file, byte write, temporary-file sync, close, mode setting and replacement. A containing-directory sync is attempted best-effort. Its striped path mutex orders writers **inside one process**, and only around writing the supplied bytes. It does not re-read or merge their histories. [Store save][save], [atomic helper][atomic].

These controls are valuable. The failure here is not interleaved JSON, a reused temporary filename, missing atomic replacement or a claim that successful rename always implies a stronger power-loss guarantee. A perfectly formed stale snapshot can atomically replace a newer one. Even adding a cross-process mutex solely inside `WriteFile` would not fix the sequential witness: the stale bytes were derived before that lock, and there is no authoritative read-modify-write step inside it.

## Minimal conditional witness

Let A and B be separate `LearningStore` objects backed by the same initially empty, writable directory. Both initialize before the first event. Let `x` and `y` be two valid generated execution identities for the same tool, with distinct fingerprints and small nonempty feedback bodies. Both processes genuinely start and finish, validation succeeds, and all persistence operations return success. Use no additional publication after the steps below.

| Step | A's in-memory event set | B's in-memory event set | Durable snapshot |
|---|---|---|---|
| Both stores initialize | empty | empty | absent or empty valid history |
| A records x and receives `Durable=true` | x | empty | x |
| B subsequently records y and receives `Durable=true` | x | y | y |
| C opens the store after B returns | unchanged | unchanged | C loads y, not x |

B's computation begins from its empty `ls.learnings`, `ls.executions` and `ls.feedback`, not A's published file. Its resulting aggregate for the tool has one execution; the reopened store likewise has one, not the two acknowledged executions. The snapshot lacks x's acknowledgment and body. Generalizing from an existing common history H gives `H + x` followed by `H + y`, not the intended conservation of `H + x + y`. [Initialization][store-init], [aggregation][local-admission], [publication][publication], [reopen][reopen].

There is a useful follow-on observation, not an additional prerequisite: A can still return its remembered acknowledgment for x. Its duplicate branch consults its local acknowledgment map and returns without another save. `ExecutionAcknowledgment` and `ExecutionFeedback` also read instance memory. The adapter's already-durable feedback-retry branch similarly returns its retained receipt. None of these local lookups repairs the shared file overwritten by B. They establish what A remembers having published, not a fresh observation that x is present in the current shared snapshot. [Local duplicate admission][local-admission], [memory lookups][lookups], [adapter retry][retry].

This does not mean x is immediately gone from all memory, logs or working observations: A still holds its receipt and feedback in this trace. Nor does it prove that no future A publication could restore x from that retained state. The supported result is that a fresh opener after B's successful publication cannot recover x from the designated durable learning snapshot, without any declared retirement operation.

## Existing controls and limits on impact

- **One instance is serialized correctly.** Multiple goroutines sharing one store use its mutex. `TestGeneratedFeedbackConcurrentPublicationRetainsEveryRecord` expressly constructs one store and sends sixteen workers through it, then reopens. That is a valuable control, not coverage of separate snapshots. [Existing test][concurrency-test].
- **Failure does not silently advance local state.** Publication failure returns a nondurable acknowledgment and leaves live learning unchanged; retry can then succeed. Preserve this behavior and its test. [Publication ordering][publication], [failure control][failure-test].
- **Corrupt history fails closed.** Loading invalid history records a durability error and preserves/quarantines the file. The witness instead uses valid snapshots; weakening corruption handling would not fix it. [Loader][reopen].
- **Live effect deduplication remains intact within its owner.** Equal calls share the adapter receipt and feedback repair does not execute another process. The proposed change must not turn a publication conflict into effect replay. [Live adapter][adapter], [retry][retry].
- **The restart fence remains important.** The backend refuses when it knows a persisted execution but lacks its live receipt. Losing an acknowledgment can remove evidence that this check consumes on a later open, but this report does not demonstrate a normally reachable replay of the same old request identity, an automatic second execution, or a cross-process exactly-once defect beyond the stated record-loss sequence. Session scopes and exact admission still matter. [Restart fence][restart-fence].

The direct impact is durable evidence and aggregate conservation. A future reader can see incomplete execution history and aggregates reflecting only one independent publisher. Whether that changes refinement decisions, tool selection, user-visible answers or costs in a live workload is unmeasured. The initial A acknowledgment was not fabricated: it followed a successful write. The unsupported assumption is that independent later writers preserve its event without a shared transaction or explicit retirement protocol.

## Proposed contract and repair options

The storage boundary needs one authority for duplicate/conflict admission and event publication across every cooperating owner of a canonical store. For a successful new event publication, the resulting committed history must preserve previously committed independent events unless a separate, explicit retention transaction retires their bodies under the applicable policy. Identity fencing and body retention remain different concerns.

A minimal file-based design would acquire a store-scoped interprocess lock, load and validate the latest committed snapshot under that lock, resolve duplicate/conflicting identity against that authoritative snapshot, apply the new event once to its aggregates, atomically publish, and only then update the instance's view and acknowledge success. The lock must span **read, admission, update and replacement**, not only the last write. Rechecking duplicate identity solely in old memory is insufficient.

Do not merge aggregate counters by blindly summing every store's totals: their common history would be counted repeatedly. Apply the newly admitted event once to the latest aggregate, or derive aggregates from an authoritative event ledger with a defined legacy baseline. Preserve exact fingerprints, input/output fidelity, quality semantics and conflict rejection. Pure lookup freshness should be specified separately; fixing new-event publication does not automatically make every existing `GetLearning` call observe another owner's latest update.

There is a nearby precedent, but not a drop-in repair: ProfileStore's ledger mutation takes a file lock, adopts disk ledger state, applies new changes and saves. Its comments explicitly describe preventing another process's counts from being dropped. It also deliberately continues as last-writer-wins if its lock fails because those measurements are best-effort. **Do not copy that fail-open policy into mandatory durable execution learning.** [ProfileStore transaction][profile-precedent]. Existing POSIX and Windows file locks block without a context parameter, so cancellation, bounded lock acquisition and shutdown ownership require an explicit design. [POSIX lock][filelock], [Windows lock][filelock-windows].

Usage's repair similarly computes this instance's contribution against a baseline before merging with disk totals; its bounded recent event ring is intentionally not merged, and lock/read failures can degrade to its previous best-effort behavior. That is appropriate prior art to credit, not proof that generated execution identities and exact feedback bodies may be discarded or that aggregate summation alone would preserve them. [Current usage transaction][usage-source].

| Option | What it establishes | Costs and limitations |
|---|---|---|
| One process-local store object per canonical path | Removes independent snapshots within that process | Does not coordinate other CLI processes; reference ownership and teardown still matter |
| Lock and authoritative reload around file transaction | Can preserve events across cooperating publishers while retaining the current layout | Additional full-file reads/decodes, serialized publication, lock-failure/cancellation handling; existing history-size costs remain |
| Transactional event store with unique execution identity and aggregate update | Atomic event admission and aggregate conservation without replacing all history per event | Schema migration, indexes, crash recovery, transaction failure handling and operational complexity; does not grant effect exactly-once |
| Explicit exclusive owner for the entire store lifetime | Prevents another cooperating publisher from entering | Restricts concurrent runtimes; requires clear admission errors, ownership recovery and product approval rather than silently disabling useful parallel work |

Do not hold the storage lock while invoking a model, launching a generated tool or waiting for arbitrary refinement. The event's quality/validation result is already available before store publication in the current route. Keep failed-publication receipts recoverable and retry publication only. An asynchronous flush may be appropriate for optional telemetry, but cannot return the existing synchronous durable acknowledgment before the event reaches its authoritative commit boundary.

### Migration and compatibility

Version-2 snapshots contain both full events and per-tool aggregate history; older accepted layouts contain aggregate learnings without complete execution lineage. Preserve that baseline rather than inventing missing execution IDs or recomputing historical totals solely from newer events. A new event store should import exact known identities and bodies idempotently, retain a recoverable old snapshot until selection succeeds, and use an explicit format/version boundary. [Versioned loader][reopen].

Mixed old/new writers are a real rollout risk. An old binary that still overwrites the snapshot without joining the new protocol can undo a file-lock repair. Define whether publication requires all writers upgraded, a format/version admission barrier, or a single selected store with old writers refused. Do not imply advisory locks protect against nonparticipating writers. Preserve workspace ownership, permissions, corruption recovery and redacted model disclosure.

Coordinate with the separate retention proposal so compaction cannot race insertion, erase identity fences or resurrect retired bodies. Cross-instance stale publication must not become a hidden way to prune history. This proposal does not require immediate wholesale storage replacement; compare a narrowly locked reload against a transactional store before choosing the larger change.

## Fair evaluation plan not run

1. **Pin the minimal sequence.** Construct A and B against the same path before either publication; record distinct x then y sequentially; open C. Require both acknowledgments, both exact bodies and aggregate count two. Repeat over nonempty H and reverse publisher order. This isolates stale ownership without timing-dependent concurrent writes.
2. **Keep the existing single-instance control.** One store with multiple goroutines must retain its established behavior. One writer reopened only after the preceding writer commits must also conserve history. Separate workspaces must remain independent.
3. **Test authoritative idempotency.** Independent stores submit the same ID/fingerprint and then conflicting fingerprints. Require one credit for the equal event and a visible conflict for the unequal event, with no replacement of the original. Duplicate acknowledgment must be grounded in the selected authoritative store contract, not a stale local map.
4. **Exercise real boot consumers.** Boot two legitimate owners of one workspace before either tool call. Use admitted harmless generated fixtures through actual session/VirtualStore/factory paths, distinct request IDs, real effects and validation, and deterministic provider responses. Assert both native effects occur once and both durable feedback records survive a fresh reopen. A boot or catalog refusal is a prerequisite failure, not evidence the conservation behavior passed.
5. **Separate processes and platforms.** Repeat the store-level witness in separate processes, then exercise the production route on supported POSIX and Windows configurations. Include lock contention, cancellation, lock failure, failed replacement, interrupted publication and shutdown joining. Do not weaken fail-closed behavior to obtain a green test.
6. **Migration and old-writer controls.** Import Version-2 and legacy aggregate-only data, interrupt and resume migration, retry imports, and attempt an unsupported old-writer publication. Preserve known fingerprints, bodies and baseline counts without double credit or silent empty-history recovery.
7. **Measure cost fairly.** Compare baseline, file transaction and transactional event-store variants with the same event sizes, history sizes and publisher counts. Report publication latency distribution, lock wait/cancellation latency, bytes read/written, peak memory, reopen time and retry rates. Distinguish this correctness repair from the 1117 retention/cost work; do not claim a coding-quality or performance improvement without those measurements.

Acceptance is conservation of every acknowledged independent event under the supported writer protocol, correct idempotent aggregation, honest failure/recovery and unchanged no-reexecution behavior. Neither a single-store concurrency test nor a successful atomic file replacement alone establishes that contract.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[research]: https://github.com/theRebelliousNerd/codenerd/commit/c3a596c8e9444a523b3368a25843cb3bad2de112
[retention]: https://github.com/theRebelliousNerd/codenerd/blob/c3a596c8e9444a523b3368a25843cb3bad2de112/Docs/audits/generated-call-receipt-retention.md
[observation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/generated-tool-observation-fidelity.md#L172-L203
[historical]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/autopoiesis/2026-05-15_12-23-22-AM-EST_feedback_boundary_analysis.md#L19-L28
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/autopoiesis/WIRING-AND-NOT-BUILT.md#L108-L138
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1990-L2018
[default-path]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_orchestrator.go#L188-L214
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_orchestrator.go#L277-L305
[store-init]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L284-L325
[cache]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L254-L376
[multi-instance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L472-L488
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_chat.go#L139-L164
[consumer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_tools.go#L2812-L2834
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/virtual_store_generated.go#L14-L105
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L66-L161
[conversion]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_profiles.go#L133-L163
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_feedback.go#L37-L81
[boot-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_generated_feedback_test.go#L511-L592
[local-admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L351-L459
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L463-L501
[save]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L652-L657
[atomic]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/atomicfile/atomicfile.go#L118-L212
[reopen]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L605-L649
[lookups]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L520-L549
[retry]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L182-L225
[concurrency-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/generated_feedback_test.go#L58-L91
[failure-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/generated_feedback_test.go#L38-L55
[restart-fence]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_profiles.go#L89-L106
[profile-precedent]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger.go#L104-L179
[filelock]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/filelock/lock_other.go#L10-L32
[filelock-windows]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/filelock/lock_windows.go#L19-L44
[edit-prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/edit-observation-precondition-continuity.md#L217-L226
[usage-prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/usage/TODO.md#L50-L64
[usage-source]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/usage/usage_tracker.go#L528-L586
