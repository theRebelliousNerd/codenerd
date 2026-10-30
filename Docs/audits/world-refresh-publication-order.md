---
doc-class: deep-dive
subsystem: world-refresh-projection
implementation-status: partial
last-verified: 2026-10-30
verified-against: fef98230c20a5751e4c32a2fd2757832a9d39cdc
source-links-against: 32b8417bb9b0ec70a83b06226c67c906d0b30be0
verification-method: static-source-inspection
review-status: independently-challenged-documentation-draft
---

# Keep world-graph publication in refresh order

## Decision and evidence boundary

Preserve the new single-scanner architecture and its scan/application mutex, but extend the ordering contract through publication of the scanner-derived knowledge graph. `Scanner.Refresh` serializes scanning and kernel application; its caller publishes graph changes after that serialization ends. A delayed older graph publication can therefore reintroduce an edge after a newer refresh has correctly retired its support.

This is a **source-supported conditional interleaving**, not an observed incident or an executed regression. It requires two overlapping refresh consumers, a second change in a previously scanned source scope, and successful persistence/application at the relevant stages. A single serial caller does not produce this witness. No application code, tests, benchmarks, model inference, failure injection, or live database operation was run. This report is documentation only.

The original source pin is public main [`32b8417bb9b0ec70a83b06226c67c906d0b30be0`](https://github.com/theRebelliousNerd/codenerd/tree/32b8417bb9b0ec70a83b06226c67c906d0b30be0), resolved and inspected at approximately 05:07–05:10 UTC on October 30. At 05:12 UTC main had advanced to [`6cb5f88e00eaf939d10dee46c16c68baf4b53d57`](https://github.com/theRebelliousNerd/codenerd/commit/6cb5f88e00eaf939d10dee46c16c68baf4b53d57). Its only additional commit changes five chat continuation/state/test files, none of the decisive paths linked below; the callback, refresh coordinator, graph transaction and projection adapter were fetched again and are byte-identical. Source citations retain the original immutable pin. The recent retirement commit [`f3ad5fc`](https://github.com/theRebelliousNerd/codenerd/commit/f3ad5fc529ab8eb28f593c24c5dbcc1bafbba993) removed `world_model_ingestor`, added `Scanner.Refresh`, and attached the post-write callback. This report does not propose restoring the ingestor or a second full scanner.

An independent source challenge supported the graph-only ordering witness and required two qualifications retained here: the prior audit already proposed stale-publication fencing, so this is a residual in the current implementation of that contract, not a newly discovered class; and the cold-storage fact mirror is a separate upsert API, not established as a second ordering defect by this review.

## What is already correct

- **One scan/application owner.** `Scanner.Refresh` takes `refreshMu`, calls `ScanWorkspaceIncremental`, applies the result to the kernel, records scan statistics, and releases the mutex on return. Two calls through this scanner cannot perform those stages in the opposite order. [Refresh implementation][refresh]
- **Source retirement exists.** The incremental scanner obtains the previous fast facts for changed/deleted canonical paths before replacing the per-file cache. `ReplaceLinkFacts` projects old/current facts, omits retirement of triples still present in the current result, and sends retirement and publication together to the store. [Old-fact collection][old-facts], [projection][projection]
- **Each graph update is atomic.** `ReplaceWorldLinks` holds the store mutex and uses one transaction for its deletes and inserts. It marks projected rows and restricts retirement to projection-owned or recognized legacy-source rows. These protections are valuable; this finding is not a data race on a Go map or a missing SQL transaction. [Graph transaction][graph-write]
- **A single executor batch is serial.** `executeToolBatch` processes the model's calls in sequence. The witness below does not assume concurrent calls within one model response. [Batch entry and loop][batch]

The missing property is ordering **between** complete graph transactions. A SQL transaction started later can contain an older scan result.

## Exact production boundary

| Stage | Pinned implementation | Relevant contract |
|---|---|---|
| Boot attachment | `attachWorldRefresher` captures the boot scanner, kernel, store and workspace | Same scanner is reused by the VirtualStore callback |
| Observe/apply | `scanner.Refresh(...)` returns a result | The scanner mutex has been released when the caller continues |
| Mirror facts | `PersistFactsToKnowledge(res.NewFacts, ...)` | A separate operation, iterating stored facts; not part of the refresh critical section |
| Publish graph | `ReplaceLinkFacts(res.RetractFacts, res.NewFacts, "scan")` | No scan generation or expected-current identity is supplied |
| Store commit | `ReplaceWorldLinks(retire, publish)` | Serialization by transaction arrival, not by source observation order |
| Unchanged call | callback returns for `res.Unchanged` | No graph reconciliation or pending-publication replay |

These stages are consecutive in the [callback][callback], but are not one indivisible operation. The scanner updates its per-file database facts and saves the manifest before returning the delta. Thus a later refresh can read the newer per-file baseline while the earlier callback still has not published its graph. [Delta publication][delta-publish]

The callback is invoked after mutation observations in both the [VirtualStore action result path][action-sink] and the [tool-completion fact sink][tool-sink]. The registry captures the sink under a read lock, releases that lock, executes the handler, and invokes the sink without an operation-wide registry lock. [Registry execution][registry]

There is an actual overlapping production execution route: the campaign scheduler launches admitted tasks as goroutines, campaign dispatch calls the observed task executor, and inline delegated tasks use `CloneForTask`. That clone isolates conversational state but shares the VirtualStore and modular registry. This supports overlapping callbacks when the campaign admits multiple tasks and their successful tools emit edit records. It does **not** assert that every campaign has parallelism enabled, that every task writes, or that arbitrary conflicting writes are permitted. [Task launch][campaign-launch], [task bridge][campaign-bridge], [inline clone][task-clone], [shared dependencies][clone]

Chat scan/delegation callers also publish graph results after `Scanner.Refresh` returns. Any repair intended to cover all refresh consumers must inspect those paths, rather than locking only the new callback and leaving another writer outside the contract. This report does not need to establish that two particular chat commands can overlap. [Chat scan][chat-scan], [delegation scan][delegation-scan]

## Conditional interleaving: a retired import returns

Use a healthy initialized workspace, the real fast-cache store, the same scanner/kernel/store for both calls, and an ordinary readable Go source file `a.go`. The workspace contains an importable local package `b`. Start with a valid nonempty manifest and no `a.go -> b/b.go` dependency in either the fast cache or graph. Canonical path and import resolution must succeed. No cancellation or storage failure is needed.

Let `E` be the projected triple `(a.go, depends_on, b/b.go)`.

| Step | Operation | World cache / kernel | Knowledge graph |
|---|---|---|---|
| 1 | A successful authorized write adds the local import and its use to `a.go`; callback A starts. | Earlier generation | No E |
| 2 | A completes `Scanner.Refresh`. Its result includes the new `dependency_link`, which is now cached and applied. Pause A before `ReplaceLinkFacts`. | E supported | No E yet |
| 3 | The source changes again: remove that import and its use, leaving a valid import-free `a.go`. Another refresh consumer B starts. | A's cached facts are available as B's previous facts | No E |
| 4 | B completes scan/application, reads A's cached dependency into `RetractFacts`, and publishes its graph transaction. | E no longer supported | Retirement of absent E changes zero rows; transaction succeeds |
| 5 | Resume A's older `ReplaceLinkFacts`. | B's current world remains in place | A inserts E, with scanner-projection metadata |
| 6 | Refresh again without another filesystem change. | `Unchanged` | Callback returns before graph work; E remains |

At step 4, B does exactly what the ordered-retirement implementation asks. At step 5, A's insert does exactly what its older result asks. Neither individual transaction is torn. The composition is wrong because no boundary rejects or orders the stale result.

The per-file dependency is not assumed to exist only in a transient result: the scanner resolves imports before persisting each file's facts, explicitly so those edges can be retracted later. Its old-fact lookup makes B's retirement of A's cached dependency feasible. [Per-file import construction][import-construction], [old-fact collection][old-facts], [delta publication][delta-publish]

**A production-compatible realization without conflicting campaign leases:** let admitted task A write `a.go` and pause its post-write callback after scanning. The operator then edits `a.go` externally to remove the import. A second admitted task writes a different file and invokes its own callback; the workspace-wide census detects the operator's changed `a.go`, producing B's newer delta. Campaign task leases do not need to admit two agents writing the same file. This is a conditional schedule involving a legitimate external edit, not an assertion that external edits always trigger their own watcher. The second task's edit is the refresh trigger. All reads and writes must complete successfully for the stated result.

For a focused deterministic evaluation, barriers can reproduce the ordering directly at these boundaries. That would establish the API composition first; a separate entry-path fixture should establish the full callback route. Neither has been executed here.

## Effect and limits

`QueryKnowledgeGraph` reads the stored graph through `QueryLinks` and emits `knowledge_link` facts without comparing a scan generation. After the schedule, a query of `a.go` can therefore return E even though the successful newer scan no longer supports it. This is the narrow consumer consequence claimed. [Graph reader][graph-read]

The report does not establish a particular model answer, prompt inclusion, incorrect edit, safety bypass, or user-data loss. Live kernel world facts can remain correct in this witness; the stale representation is the durable graph projection. `PersistFactsToKnowledge` is shown only as an intervening stage outside the mutex: its cold-storage upsert/lifetime semantics are separate, and this report claims no corresponding stale-replacement defect there. No claim is made that successful JIT necessarily consumes this edge. Existing direct source views remain independent counterevidence.

An unchanged refresh demonstrably does not reconcile the projection. A later refresh is not automatically a repair: retirement is calculated from the then-current fast-cache facts, which may already lack E. Conversely, a deliberate authoritative graph reconciliation or a later source sequence that makes E part of a subsequent retirement can remove it. Permanent corruption and universal self-recovery are both stronger claims than this review supports.

## Relationship to earlier audits

The [scanner-graph-source-retirement audit][prior-retirement] requested removal of superseded source-derived support. The [audit queue][queue] records that repair at `4123cef79`. The existing [regression definition][retirement-test] scans, publishes, changes source, then scans and publishes again in order. Credit that implemented behavior.

This continuation isolates **publication inversion after a newer retirement**, made concrete in the newly shared post-write lifecycle. The prior retirement proposal already calls for revision/status-aware stale-publication fencing; the present finding is therefore a **residual implementation gap under that existing contract**, not a novel architectural class or a repeat claim that the graph only ever upserts. Its additional contribution is the pinned current-main handoff, the bounded interleaving and the missing ordered-publication regression. The [retrieval delta-fencing audit][prior-retrieval] is related precedent but concerns keyword-cache/domain/epoch writers, not this world-cache-to-SQL-graph handoff. The [world-cache format audit][prior-format] concerns lane-only row-encoding compatibility, not graph ordering. The current research branch adds EDB alias accounting and a mission-driven-loop proposal, neither this publication boundary. Deduplication is bounded to the inspected audit inventory and these relevant predecessors, not a universal novelty claim.

## Proposed contract and repair options

For a scanner-owned source scope S, an admitted graph publication must not make an older source observation current after a newer observation for S has settled. Completion should distinguish **world applied**, **graph projected**, **projection failed/pending**, and **superseded**. A monotonically numbered scan alone is not an acceptance check unless the writer and reader use it at the commit boundary.

1. **Serialize the complete observation-to-projection operation.** Put scan, kernel application and graph projection under one owning coordinator, and route every overlapping consumer through it. Keep the package boundary clean: `world` should not gain a reverse dependency on `core`; a callback or higher-level coordinator can own the composition. Locking only `ReplaceLinkFacts` merely repeats the SQL store's existing serialization and does not preserve scan order. Locking only `attachWorldRefresher` leaves chat/delegation callers outside it. Define lock ordering and do not call refresh recursively from its own projection hook.
2. **Publish revision-bound replacement scopes.** Attach producer/workspace/scope identity and a generation or exact source revision to the result. At graph commit, compare against the accepted state for that scope and reject superseded publication atomically. Record empty projections/tombstones too: when B removes E before A inserts it, checking only an existing edge cannot reject A because no edge exists yet. Per-scope fencing must preserve unrelated deltas. A naive global latest-generation check can discard useful older work for file X merely because a newer delta changed only file Y.
3. **Use an ordered publication queue with acknowledged receipts.** Enqueue while still holding the observation-order ownership, then apply graph transactions in that order. Bound retained work, expose pending/current status, and decide whether callers wait for acknowledgement. Enqueueing after `Refresh` returns retains the same reordering opportunity. Restart recovery needs durable sequence/receipt semantics if it claims to preserve pending projection work.
4. **Reconcile from the authoritative current world cache.** Treat the graph as a recoverable derived view, not a stream of indefinitely replayable deltas. Reconciliation needs an admitted coherent source snapshot and ownership-scoped replacement. It must not delete manual/ingest knowledge or assume that a partial scan proves absence. This may be an appropriate fallback after failed publication rather than the normal cost of every edit.

Whichever approach is selected, define failure recovery separately. The current callback can return an error after the scanner has advanced its cache; the next `Unchanged` result skips projection. Extending a lock does not by itself supply replay, crash atomicity, or a database/kernel transaction. Those are acceptance dimensions to specify, not additional independently reproduced findings in this report. Preserve the current distinction between an already-landed write and a failed observational refresh; never retry the write merely to regenerate its observation.

## Related primary research

[Lamport’s work on event ordering](https://lamport.azurewebsites.net/pubs/time-clocks.pdf) distinguishes the order of causally related events from arbitrary arrival order. It provides a useful conceptual basis for the ordering contract here; it does not by itself prove or implement a correct CodeNERD synchronization design.

## Costs and verification design

**Costs:** a larger critical section increases post-write latency and serializes graph/cold-storage I/O, while current scan counters exclude that tail. Generation fencing adds schema/state, scope identity, tombstone retention, migration and conflict handling. A queue adds memory/durable backlog, shutdown and replay obligations. Reconciliation adds reads, write amplification and possibly full-graph work. None has a measured throughput or token-saving benefit here. Compare total task cost and currentness, not only scan duration.

If execution is later authorized, use isolated temporary workspaces and databases, no live models or user files:

- Exercise the exact add → delayed A publication → remove → completed B publication → resumed A publication schedule. Require a real resolved local import, a nonempty starting manifest, successful kernel/cache updates and a negative graph query after settlement. Include a final unchanged refresh and reopen the graph store.
- Validate the production callback through the tool-fact sink and the action path. Use two admitted independent task callers for the production concurrency fixture, and preserve serial `executeToolBatch` as a negative control. Separate direct method/barrier evidence from end-to-end scheduler evidence.
- Retain ordinary serial add/remove and no-op controls; unrelated-file concurrent deltas; retained imports; legitimate later re-addition; file deletion; canonical path aliases; and manual/ingest edge preservation. For fencing, test the absent-row/tombstone case and ensure a newer Y-only delta cannot erase or wrongly reject valid X work.
- Challenge interruption/failure boundaries: before/after cache persistence, before/after kernel application, before/after graph commit, cancellation while waiting, and restart with pending publication. Distinguish a visible pending/degraded projection from a falsely current successful result. Verify no retry re-executes the original file mutation.
- Run existing source-retirement definitions unchanged as controls. Their presence is not a result from this pass. A race-detector pass alone does not establish ordering; use deterministic barriers and assert the final semantic relation.
- Record graph correctness against the admitted current source scope, stale-publication rejection, pending age, failed/replayed publications, lock wait, scan/apply/project latency separately, SQL rows read/written, allocations, retained state growth, and total completion latency. Compare current main with the smallest proposed repair under identical source histories and concurrency settings.

The useful outcome is a graph projection that cannot become older while a successful current refresh has already advanced the source world. Keep the retirement simplification; close its publication boundary rather than reintroducing a background duplicate scanner.

[refresh]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/world/refresh.go#L21-L47
[callback]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/system/world_refresh.go#L10-L36
[old-facts]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/world/incremental_scan.go#L352-L375
[import-construction]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/world/incremental_scan.go#L403-L415
[delta-publish]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/world/incremental_scan.go#L611-L687
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/core/virtual_store_predicates.go#L167-L235
[graph-write]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/store/local_graph_world.go#L29-L109
[graph-read]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/core/virtual_store_predicates.go#L239-L270
[action-sink]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/core/virtual_store_routing.go#L178-L185
[tool-sink]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/core/virtual_store_tool_facts.go#L45-L76
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/tools/registry.go#L376-L455
[batch]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/session/executor_tools.go#L926-L1050
[campaign-launch]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/campaign/orchestrator_tasks.go#L267-L280
[campaign-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/campaign/orchestrator_task_handlers.go#L31-L87
[task-clone]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/session/task_executor.go#L358-L408
[clone]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/session/executor.go#L690-L699
[chat-scan]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/cmd/nerd/chat/helpers_scan.go#L153-L192
[delegation-scan]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/cmd/nerd/chat/process_dream_delegation.go#L107-L141
[prior-retirement]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/Docs/audits/scanner-graph-source-retirement.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/Docs/audits/ASTRA_QUEUE.md
[retirement-test]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/internal/world/graph_source_retirement_test.go#L15-L109
[prior-retrieval]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/Docs/audits/retrieval-delta-publication-fencing.md
[prior-format]: https://github.com/theRebelliousNerd/codenerd/blob/32b8417bb9b0ec70a83b06226c67c906d0b30be0/Docs/audits/world-cache-rolling-format-admission.md

## As built

Source reinspection against working-tree base `fef98230c` supports the bounded
interleaving. The original report above describes the pre-repair path; the
following rulings describe the authored repair. No new commit identifiers are
available: these changes remain in the working tree for integration.

| Finding or prerequisite | Ruling against the base | Current disposition |
|---|---|---|
| Scan/application ownership ends before graph publication, allowing transaction inversion | **Real.** `internal/world/refresh.go`, `internal/system/world_refresh.go`, `cmd/nerd/chat/helpers_scan.go` and `process_dream_delegation.go` retained that handoff. | Scanner now owns observation through publication; all Refresh consumers inherit one attached publisher. Chat/delegation no longer publish returned deltas. |
| No-op refresh skips a failed or late projection | **Real.** The callback returned on `Unchanged` after the cache had advanced. | One failed result stays pending in memory and is retried before another scan, including an unchanged observation. Repeated failure prevents advancing to a newer delta. |
| Earlier source-retirement repair is absent | **Already fixed**, as the report itself credits: `4123cef79`, `ReplaceLinkFacts`, `ReplaceWorldLinks` and `graph_source_retirement_test.go`. | Keep its projection-owned retirement and atomic SQL replacement; document that transaction arrival does not establish observation order. |
| Shared production callbacks can overlap despite serial calls within a model batch | **Real prerequisite.** `orchestrator_tasks.go` launches admitted tasks as goroutines; `orchestrator_task_handlers.go` uses the observed executor; `task_executor.go` clones the executor while `executor.go` shares VirtualStore/registry dependencies. `executor_tools.go` still processes an individual batch serially. | The shared scanner is the owner, independent of caller concurrency or task write-set leases. No campaign scheduling policy changes. |
| Dependency retirement can be constructed from A's cached import | **Real prerequisite.** `incremental_scan.go` loads canonical old fast facts before replacement and resolves imports before persisting per-file rows. | Preserve that path. Publication now completes before a newer observer can consume its baseline. |
| Graph readers admit the stale edge without comparing source generations | **Real consumer boundary.** `QueryKnowledgeGraph` reads `QueryLinks` and emits the stored relation. | Order writers; no reader heuristic, global generation discard or per-edge tombstone is introduced. |
| Cold-mirror ordering is another established defect; permanent corruption or model harm is proven | **Not findings of this audit.** Its stated evidence explicitly excludes these stronger claims. | No such claims are added. Mirror and graph failures are joined, with graph retirement attempted even when the mirror fails. |

No substantive bounded finding was ruled wrong. The retained historical commit
claims were also checked locally: `f3ad5fc52` retires the ingestor and adds the
shared refresh boundary; `6cb5f88e0` changes only the five chat continuation/state
files listed in the report. The prior audit's revision-aware replacement
proposal and its queue entry are present. This repair selects the report's
complete-operation serialization option instead of adding revision storage.

Files: `internal/world/{fs.go,refresh.go,incremental_scan.go}` own cancellable
admission, application/publication receipts and a single pending result;
`internal/system/{world_refresh.go,factory.go}` install publication before boot;
`cmd/nerd/chat/{helpers_scan.go,process_dream_delegation.go}` remove duplicate
publication and serialize deep/partial observation through graph commit.
`internal/core/virtual_store_predicates.go` and
`internal/store/local_graph_world.go` clarify their transaction-only contract.
The as-built contract is in `Docs/architecture/world/IMPLEMENTED_SPEC.md`.
No Mangle predicates, registration entries or prompt atoms changed.

Tests authored:

- `TestRefresh_PublicationPrecedesNextObservation` in
  `internal/world/refresh_publication_test.go`: real scanner, kernel, fast cache,
  resolved local import and graph adapter; barriers around publication and
  waiting admission; cancellation, unrelated-file delta, unchanged refresh,
  re-addition, deletion, manual knowledge and reopening the SQL graph.
- `TestWorldRefreshAfterWriteUsesBootScanner` in
  `internal/system/ondemand_e2e_test.go`, with
  `internal/system/world_refresh_publication_test.go`: boot projection plus action and modular
  completion routes; SQL trigger aborts retirement after the write succeeds;
  repeated failure must remain pending, and an unchanged refresh must replay the
  retirement without re-executing the write. Before the repair, the callback
  lost that retirement and the no-op refresh left the edge in the graph.
  A separate cold-storage abort also requires healthy graph retirement to
  proceed while the combined publication remains visibly pending.

Status is **partial** because execution, including fail-before/pass-after and
race verification, remains outstanding. Only formatting and whitespace checks
were run; the failure descriptions above are source-derived expectations, not
observed test results. No throughput or latency measurements are claimed.
An admitted campaign concurrency fixture was not executed or added; the authored
tests separate direct ordering evidence from the existing production callback
routes, without claiming end-to-end campaign scheduling evidence.

The pending receipt supports recovery within the scanner's lifetime. It is not
a crash-durable outbox: restart reconciliation after cache/kernel/publication
interruption remains a separate requirement, as do pre-existing stale graph
reconciliation and coordination between independent scanners/processes. The
shared lock also does not add publication replay to the separate deep/partial
scan APIs. None of these boundaries is presented as a successful current graph.
