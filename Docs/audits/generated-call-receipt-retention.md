---
doc-class: deep-dive
subsystem: system-autopoiesis-session
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
supersedes: []
---

# Generated-call terminal receipts need an explicit retention lifecycle

## Decision

Separate the lifetime of an execution identity from the lifetime of its full diagnostic body. Coordinate that distinction across the Cortex-owned generated-call adapter and the autopoiesis learning store; changing only one owner does not bound retained body memory.

The generated execution and durable-feedback route is already wired. The remaining source-established gap is that completed calls continue to occupy the adapter's live-call table, while the durable learning store also retains their full feedback bodies in memory and republishes its complete history for each new event. There is no corresponding completed-body retirement boundary in these owners. This matters for long-lived runtimes and reused workspaces even when individual calls, model requests and working-context archives are bounded.

This is **not a measured memory leak, an observed out-of-memory incident, or a claim that arbitrary tool requests can bypass admission**. It is a source-derived retained-state and publication-cost finding. No runtime, tests, model inference, benchmarks or repository modifications were performed. The future evaluations below have not been run. This scratch document proposes behavior; it does not report an implemented repair.

## Source freeze and novelty boundary

Public main was revalidated at approximately 11:13 UTC and again at 11:16 UTC on October 30, 2026 as [3882051][main]. It remains the same implementation revision as the preceding pass. The latest inspected research branch, [research-main-20261030-1049][research1049], adds documentation, not an implementation of this lifecycle. An independent source review corroborated the two-owner accumulation and qualified the prior-work and bounded-history controls below; it did not execute the proposed witnesses.

The [generated-tool observation fidelity audit][prior-observation] already identified partial-return and stderr delivery defects. Its as-built ledger credits those repairs, and current session code returns the labelled, redacted generated observation. Keep that improvement. This report concerns the full backend receipt and learning-history owners **after** successful delivery, not missing diagnostics or a request-visible tool catalog. It also does not reopen working-handle allocation, working-observation retirement, tool-effect validation completeness, prompt-evolution evidence accounting or the 1049 atom filename-identity continuation.

That earlier audit already called for task-owned observations and honest unavailability after retirement. Historical [May feedback QA][historical-qa], marked remediated, also identified synchronous full-map JSON saving under the learning-store write lock. This report does not claim that generic cost observation is new or repeat its unmeasured system-freeze rhetoric. Its suggested asynchronous dirty flushing cannot simply replace today's synchronous durable-acknowledgment contract.

The distinct addition here is the concrete non-retiring `calls` table composed with the separate, fully resident **per-execution full-body** `LearningStore.feedback` history, and the replay-safe coordinated retirement needed by today's contract. Whole-snapshot publication amplifies that newer body-history cost. Novelty is limited to the inspected guidance, audit inventory and research additions, not every historical conversation.

## What is actually reachable and already protected

The factory constructs the autopoiesis orchestrator, installs `newOrchestratorToolExecutor` in VirtualStore, and transfers that adapter to Cortex ownership. Its lifetime is explicitly shared with the long-lived Ouroboros machinery, not the individual tool-call context. The orchestrator separately owns a `LearningStore` under its tools directory's `.learnings` directory. [Factory ownership][factory], [Cortex field transfer][cortex-transfer], [learning-store construction][store-construction].

For an actually registered and admitted generated tool, session dispatch calls `ExecuteGeneratedToolCall`. VirtualStore validates the typed request, host registration identity, command/arguments, exact authorization, validator availability and preflight before invoking the backend. The successful consumer also requires matching request identity, a started process, completed/passed validation and a matching durable feedback acknowledgment. Missing bridges fail closed. [Session consumer][consumer], [executive bridge][bridge].

Reachability is conditional on those real prerequisites. The existing `TestGeneratedFactoryNormalBootProcessDurableLearning` source uses real `BootCortexWithConfig` and `Process`, but explicitly supplies an allowed generated fixture and exact permission facts; it then inspects the effect and reopened learning. This is a useful acceptance starting point, **not** evidence that every default model request offers generated tools, and it was read rather than run here. [Existing production-shaped fixture][normal-fixture].

The current design deliberately provides several protections:

- Equal live keys with equal fingerprints share a single execution and terminal receipt; conflicting fingerprints are refused. Duplicates do not add another map entry or increment durable learning twice. [Live deduplication][live-dedup], [durable deduplication][durable-dedup].
- Process, validation and feedback outcomes remain distinct. A started process is synchronously recorded even when cancellation occurs; feedback repair uses the retained terminal receipt and cannot launch another process. [Terminal capture][terminal], [feedback-only retry][retry].
- If durable execution identity exists but its live receipt is unavailable, the backend refuses with a reconciliation error. It does not simply execute again. Corrupt learning state also prevents normal execution. [Restart fence][restart-fence].
- Shutdown closes admission, cancels and joins admitted execution/recording/refinement before dependencies close. Preserve that ownership and join behavior. [Adapter close][adapter-owner], [Cortex close ordering][close].

These are required contracts in the [system guidance][system-guidance] and [generated-tool guidance][generated-guidance], not expendable cache behavior.

## Finding: full terminal bodies have no coordinated retirement owner

### 1. The adapter's live-call map lasts as long as the adapter

`orchestratorToolExecutor` owns `map[string]*generatedLiveCall`; each value owns its fingerprint, completion channel and complete `GeneratedToolReceipt`. New admitted keys are inserted before execution. The deferred terminal handler stores the whole receipt and closes the completion channel; it does not delete or compact the entry. `RetryGeneratedFeedback` updates the same retained value. There is no count, byte, age or scope-retirement condition in this map's implementation. [Owner and constructor][adapter-owner], [insertion and terminal capture][terminal], [retry][retry].

The key is `ScopeID + "/" + AuthorizationID`, and the receipt includes the full request, stdout, stderr, decoded output, errors, validation state and acknowledgment. A session executor normally reuses its generated scope until its session identity changes; it does not mint one per tool call or per turn. Distinct call IDs therefore accumulate distinct entries even within one healthy long-lived scope. Changing scope only changes future keys; it does not notify the adapter to retire earlier keys. [Request/receipt types][receipt-types], [scope construction][scope].

This is specific to the typed `ExecuteGenerated` route. The legacy `ExecuteTool` method does not insert into this table; do not generalize the table claim to every tool invocation. `Close` stops and joins work but does not clear this map. The supported concern is retention while its owning runtime is alive; this analysis does not assert that an unreachable closed Cortex can never be garbage-collected. [Legacy route][legacy], [adapter close][adapter-owner].

### 2. Durable feedback is a second full-body resident owner

`RecordGeneratedExecution` converts the receipt into feedback containing canonical input, decoded output, stdout, stderr, exit/validation/capture status and identity context. `RecordExecutionDurable` passes it to the learning store. [Receipt-to-feedback conversion][conversion], [publication call][publication-call].

That store owns both `executions`, a map of durable identity acknowledgments, and `feedback`, a map of full `ExecutionFeedback` values. For each previously unseen execution, it copies those maps, deep-copies the new feedback through JSON, adds the event, serializes a Version-2 snapshot containing all learnings, acknowledgments and feedback, atomically saves it, then replaces the live maps. On reopen it reads and decodes the entire snapshot and restores both maps. No terminal-body pruning or lazy body lookup occurs in these paths. [Resident store schema][store-schema], [publication][publication], [reopen and atomic save][reopen].

Therefore, eviction from `orchestratorToolExecutor.calls` alone cannot bound the generated pipeline's total retained body memory. Restarting the process also does not retire the durable history: a subsequent store constructor reloads it. Durable evidence is valuable, but persistence does not require every historical body to remain resident forever.

### 3. Other existing bounds do not bound this history

| Existing bound or lifecycle | What it protects | What it does not establish |
|---|---|---|
| Canonical generated arguments are at most 100 KiB | Per-request argument admission | A bound on the sum of retained requests |
| Each runtime stdout/stderr capture is at most 10 MiB; overflow is an explicit backend error | Per-process capture and truthful truncation | A bound on the number of terminal stream bodies |
| Known issues and anti-pattern lists have a 100-item per-tool cap | Aggregate learning-list growth | A cap on `executions` or `feedback` entries |
| PatternDetector drops the first 100 history entries when its visible history exceeds 1000 | A separate recent-feedback count window | Retirement of either unbounded owner, or a precise bound on physical retained backing storage |
| Working scopes can retire their observation archives | Task-owned model recall storage | Deletion from the adapter map or autopoiesis learning snapshot |
| Call deadlines, cancellation and shutdown join | Active-work lifetime | Completed-body retirement while the runtime remains open |

Sources: [argument bound][receipt-types], [stream capture][capture], [learning-list cap][list-cap], [pattern history][pattern-history], [working-scope retirement][working-retirement], [shutdown][close]. Pattern history receives newly acknowledged feedback after successful publication, not each duplicate replay. It is another body holder to account for: retiring the two unbounded owners does not imply immediate reclamation of bodies still in this recent window or its backing storage. [Post-publication pattern handoff][publication-call]. The executor intentionally does not stop a task after an arbitrary tool-call count. Retention should not reintroduce that rejected completion rule. [Executor contract][scope].

## Conditional witness and supported impact

Start a healthy, long-lived Cortex with a legitimately offered, registered, authorized generated tool. Let it complete `N` sequential calls using distinct valid call IDs and modest, nonempty, varying result bodies. Let validation and durable publication succeed for every call. No race, failure, maximum-size output, short lease or unauthorized caller is required.

By the source paths above:

1. Each new call adds one adapter entry, and each terminal entry still references its full receipt after its caller leaves.
2. Each successful durable publication adds one full feedback body and acknowledgment to the learning maps.
3. Every subsequent publication encodes and saves the full accumulated snapshot. Finishing the turn or retiring an unrelated working archive does not remove these owners' entries.
4. Reopening the learning store loads the accumulated durable history again.

For distinct retained bodies, this is body retention proportional to history size, plus per-entry metadata. Under a fixed positive average event size and continued successful unique recording, repeatedly serializing the whole growing snapshot implies cumulative encoded/written work proportional to `1 + 2 + ... + N`, rather than an incremental append per event. This is an algorithmic consequence, **not measured latency, disk traffic or a proven user-visible slowdown**. Atomic-file mechanics and storage characteristics affect actual cost.

Do not multiply stdout, stderr and output lengths into a purported precise heap total. Some string fields may share backing data; decoding and the durable JSON deep-copy introduce other allocations; map/channel/error overhead and transient serialization buffers also matter. Existing output caps make each stream finite. Actual retained heap, peak allocation, pause time, startup cost and storage amplification require measurement. A small or short-lived generated-tool workload may never experience a material problem.

## Proposed retention contract

Specify the contract in affected system/autopoiesis/session documentation before changing ownership. A useful state split is:

| State | Required treatment |
|---|---|
| Admitted/in flight | Keep one joined execution owner; never evict a waiter into a second process. |
| Terminal, feedback not durably acknowledged | Keep the exact recoverable receipt or an equally durable pending-publication record. Retry publication only. Storage pressure is a visible operational condition, not permission to lose the evidence or rerun the effect. |
| Terminal, durable, body hot | Serve authorized duplicate/diagnostic readers from the same identity and truthful outcome. Bound the hot body cache independently of the lifetime of the identity. |
| Terminal, durable, body cold or retired | Preserve enough identity/outcome metadata to reject conflicts and prevent accidental reexecution. Retrieve an authorized cold body if the contract promises one; otherwise return explicit unavailable/reconciliation status. Never manufacture a successful empty receipt. |
| Owner scope closed | Close admission for that scope, join its users, settle pending publication, then apply the declared body-retention policy. Old keys must not become fresh executable requests merely because storage was reclaimed. |

**Do not add a naive LRU deletion as the entire repair.** For a durably recorded call, the existing backend fence would turn a cache miss into a reconciliation refusal, not automatically replay it; that is safe against this particular reexecution but changes live duplicate behavior. For an unresolved publication, deleting the live receipt loses the current feedback-retry source. Refusals that never started a process also currently have a terminal replay disposition and should not silently become new attempts after eviction.

The cold representation must preserve the fingerprint, executable/protocol/workspace identity needed by the chosen contract, process/validation/feedback distinctions, error classification, capture completeness and exact body references. The present `ExecutionFeedback` is useful evidence but is not automatically a lossless serialization of every `GeneratedToolReceipt` field or error type. Do not silently treat it as a drop-in receipt deserializer. The present interface has no retirement method; wiring an owner-close signal is new work, not a configuration change. [Types and interfaces][receipt-types].

Keep identity fencing independent from bulky bodies. A compact in-memory tombstone map is a helpful intermediate step but still grows by event count. To make a strict long-history resident-state claim, use a bounded cache over a durable identity index, or a closed-scope admission contract that safely rejects retired identities without retaining every body. Preserve the current restart ambiguity rule until a separately specified recoverable receipt contract is actually implemented.

Authorized raw diagnostics and model-visible redacted observations need different disclosure rules. Moving raw bodies to another store must not grant broader recall or expose secrets. Specify workspace ownership, body retention/deletion, cold-read authorization, corruption handling and failure visibility. Retirement policy should be explicit executive/operational policy; Go should implement measured sizes and safe transitions, not decide that the user's task is done.

## Options, costs and rollout

| Option | Benefit | Limitation and implementation cost |
|---|---|---|
| Keep current behavior; document and instrument | No compatibility change; all current duplicate/feedback behavior remains | History-sized residency and full-snapshot publication remain; operational restart does not shrink a reused workspace's loaded history |
| Compact durable terminal adapter entries only | Reduces adapter-held bodies without discarding identities | LearningStore still owns full history; not an end-to-end memory bound; replay needs truthful cold/unavailable handling |
| Bounded hot bodies plus incremental durable identity/body storage | Can bound hot residency and avoid full-history rewrite per event | Schema, indexes, crash consistency, compaction, I/O latency, corruption recovery, access control and cache metrics become explicit obligations |
| Scope-aware body retirement, with durable fencing and an approved retention policy | Reclaims task-ended bodies and minimizes sensitive history | Long-lived session scopes still require a byte/age policy; retirement must not race readers or pending publication; older diagnostics may become explicitly unavailable |

A staged implementation can first expose counts and estimated body bytes separately for active calls, terminal retry-pending calls, hot terminal bodies and durable history. Avoid logging raw content. Add owner/version-safe retirement and cold lookup before claiming a global memory bound. Metrics alone are not a repair, and deleting evidence is not an acceptable hidden backpressure mechanism.

For a new persistent layout, migrate Version-2 snapshots without re-crediting executions, changing fingerprints or losing aggregate statistics. Preserve a recoverable old snapshot until the new store has been validated and atomically selected. Cover interruption, insufficient storage, corrupt records and mixed-version reopen. Do not infer missing receipt fields or silently downgrade a durable acknowledgment. Define disk retention separately: moving bodies out of RAM does not bound total disk use.

The minimum design need not include a universal tool receipt database or cross-process exactly-once guarantee. Start with these two actual owners and their consumers. Cross-process coordination, arbitrary external tool effects and recovery from execution-before-publication crashes remain separate contracts unless explicitly taken on.

## Fair evaluation plan — not run

Use the existing real factory/VirtualStore/Process route with harmless generated fixtures and deterministic provider responses. Legitimately satisfy catalog and exact authorization prerequisites; do not inject impossible model calls or replace the whole bridge with a fake receipt. Separate small deterministic lifecycle controls from longer resource measurements.

1. **History growth:** compare current baseline, adapter-only compaction, and coordinated retention using identical tools, call counts, input/output sizes and workspace histories. Include repeated same-ID calls as a no-growth control and unique IDs as the growth case. Measure each owner independently; show that apparent adapter savings are not merely moved into `LearningStore.feedback`.
2. **Replay and conflicts:** concurrent equal calls execute once; mismatched fingerprints refuse. Repeated terminal success, backend failure and validation failure preserve outcomes before/after hot-body eviction. A missing cold body returns an honest error, never another effect.
3. **Publication failure:** force atomic-save failure after the process starts, then recover storage and retry feedback. Exactly one native effect, one durable learning credit and the original input/output/outcome must remain. Attempt eviction during failure and prove evidence is pinned or durably transferred.
4. **Retirement races:** pause a duplicate reader, feedback retry or scope close at controlled boundaries. No use-after-retirement, lost result, new process admission or shutdown orphan is permitted. Preserve cancellation and join guarantees.
5. **Reopen and migration:** reopen a large Version-2 history, migrate, interrupt migration, reopen again, and query both recent and retired identities. Preserve aggregate counts, conflict detection, body availability policy and the restart fence; report exactly which old fields cannot be reconstructed.
6. **Resources:** measure retained heap after quiescence, peak allocation during publication, bytes encoded/written, save latency distribution, reopen time, disk footprint and cold-read latency across history sizes. Separate logical body bytes from allocator/RSS behavior. Include small realistic outputs as well as capture-bound cases; do not rely solely on worst-case 10 MiB streams.
7. **Semantics/privacy:** verify unchanged generated observation, validation and durable-feedback behavior, including redacted model projection, truncated captures and nonzero exits. Cold-body lookup must not broaden workspace/task access. Native and Piggyback consumers must not lose their required outcome fields.

Success means a declared and demonstrated resident-body bound, an explicit durable-retention policy and unchanged no-reexecution/feedback-correctness controls, with acceptable measured storage and lookup costs. It does not mean reducing prompt size, making the model more accurate, or proving all generated effects exactly once. Retain baseline results and adverse cases; do not describe a synthetic cache win as a measured coding-quality improvement.

## Bottom line

The important feedback wiring exists and should be credited. Its long-lived owners now need a lifecycle distinction between **an execution that must remain known** and **a full body that must remain resident**. Preserve the identity and recovery guarantees while bounding or deliberately retiring bodies across both owners; an adapter-only deletion or a new tool-call limit is not that contract.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[research1049]: https://github.com/theRebelliousNerd/codenerd/commit/9ac1e81ed2b43d6e8f78af8ee55e2c384aa3de08
[prior-observation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/generated-tool-observation-fidelity.md#L172-L203
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1990-L2018
[cortex-transfer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L3022-L3035
[store-construction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_orchestrator.go#L277-L305
[consumer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_tools.go#L2812-L2834
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/virtual_store_generated.go#L14-L105
[normal-fixture]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_generated_feedback_test.go#L511-L592
[live-dedup]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L90-L126
[durable-dedup]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L351-L384
[terminal]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L127-L161
[retry]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L182-L225
[restart-fence]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_profiles.go#L87-L121
[adapter-owner]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L15-L56
[close]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/cortex_close.go#L63-L70
[system-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/agents.md#L34-L39
[generated-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/agents.md
[receipt-types]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/types/generated_tool.go#L69-L173
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L301-L325
[legacy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L268-L290
[conversion]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_profiles.go#L133-L163
[publication-call]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_feedback.go#L37-L70
[store-schema]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L284-L325
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L463-L501
[reopen]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L605-L657
[capture]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/runtime_registry.go#L410-L503
[list-cap]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L329-L332
[working-retirement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/working_context.go#L218-L258
[historical-qa]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.quality_assurance/autopoiesis/2026-05-15_12-23-22-AM-EST_feedback_boundary_analysis.md#L19-L28
[pattern-history]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/patterns.go#L44-L58
