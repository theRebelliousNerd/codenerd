# Generated-tool learning snapshots must supersede their predecessors

## Finding

**The live feedback path publishes successive cumulative learning snapshots as independent current facts.** At main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), `RecordExecutionDurable` updates the durable running aggregate, then asserts `tool_learning(Tool, Executions, SuccessPercent, QualityPercent)` without removing the tool's preceding aggregate. The stock policy classifies every matching row; it does not select the latest execution count. Once an earlier snapshot crossed the poor-quality threshold, later recovery of the current running mean does not remove that earlier support for `tool_needs_refinement`.

The bounded consequence is **stale refinement eligibility and contradictory quality bands**, not demonstrated unsafe tool replacement. A recovered tool can be both poor and acceptable in the kernel while its durable current aggregate is acceptable. The automatic factory checks that stale eligibility after subsequent durably recorded executions; an actual model call additionally requires live admission, a usable JIT assembler, and a successful path to the completion client.

This is distinct from the [1227 proposal-owner audit][proposal]: that audit starts with legitimate eligibility and follows a request carrying empty source to a discarded proposal. This one stops earlier, at the producer-to-policy contract. Correcting proposal ownership would not remove obsolete eligibility; correcting eligibility would not supply source or consume proposals. Neither fix should automatically apply generated code.

**Review basis:** static inspection on 2026-10-30. Main, root/scoped guidance, current audit inventory and queue, and active branches through `research-main-20261030-1227` were checked first. Main and existing code/QA branch tips were unchanged; 1227 was [`4585e7fa4732597bd37e0bfe36357f0a3ba71031`](https://github.com/theRebelliousNerd/codenerd/commit/4585e7fa4732597bd37e0bfe36357f0a3ba71031), four documentation commits ahead. Independent static challenge corroborated the six-outcome witness and the separation between correct durable aggregate and stale kernel eligibility. No application, tests, compiler, model inference, runtime probes, or repository modifications were performed. Arithmetic and scenarios below are source-derived predictions, not measured incidents.

## The live producer and the actual consumer

### 1. Real generated execution reaches the learning producer

Normal boot attaches the orchestrator to the production kernel and installs its generated executor in VirtualStore. That adapter executes the admitted backend, preserves pre-start refusal semantics, validates a started execution, and synchronously records terminal feedback. Only a durable acknowledgment without a feedback error starts background refinement. [`factory.go:1990–2018`][factory], [`factory_tool_executor.go:148–160`][record-trigger].

`RecordGeneratedExecution` constructs feedback from the joined native receipt. Success requires no backend error and completed, passing validation. It obtains the real quality assessment and calls `RecordExecutionDurable`; a never-started process is refused as learning evidence. [`autopoiesis_profiles.go:133–163`][receipt-feedback].

The learning store serializes a per-instance update, checks execution identity, stages a new aggregate, publishes its durable snapshot, and only then swaps the in-memory maps. Its counters and averages are genuinely cumulative: `TotalExecutions` increments and `AverageQuality` incorporates the new score. This report does not allege that those running-average formulas are the defect. [`feedback.go:353–422`][aggregate], [`feedback.go:461–497`][durable].

### 2. The projection appends old and new aggregate states

After durable recording, `RecordExecutionDurable` retrieves the current `ToolLearning` and calls `assertToolLearning`. It also publishes known issues and refreshes the generation learning context. There is no aggregate retirement in this live method. `assertToolLearning` converts the ratios to integer percentages and calls `assertToKernel`, which delegates directly to `kernel.Assert`. [`autopoiesis_feedback.go:65–94`][live-projection], [`autopoiesis_kernel.go:217–230`][assert-helper], [`autopoiesis_kernel.go:286–296`][learning-helper].

`RealKernel.Assert` adds a new fact if its full canonical tuple is new. Its explicit predicate-specific upsert exception is `system_heartbeat`, not `tool_learning`. A changed execution count therefore makes a distinct tuple even if the percentages remain unchanged. The existing EDB limit still applies; this is not a claim of literally unlimited kernel storage. [`kernel_facts.go:491–508`][dedup], [`kernel_facts.go:566–607`][assert].

The sharded production path does not introduce a hidden per-tool replacement. `tool_learning` and `tool_known_issue` are world-owned; `CortexKernel.Assert` routes to target shards, whose local assertion calls the inner `RealKernel.Assert`. [`registration.go:154–177`][ownership], [`cortex_kernel.go:461–484`][cortex-assert], [`kernel_shard.go:149–168`][shard-assert].

### 3. The stock rules treat each snapshot as present truth

The shipped rules derive:

- poor quality from any `tool_learning` row with at least three executions and quality below 50;
- acceptable quality from any row with at least three executions and quality at least 50 but below 80;
- good quality from any row with at least three executions and quality at least 80;
- refinement need from poor quality, with separate known-pagination and known-incompleteness routes.

No latest-count, generation, or current-snapshot premise distinguishes a replaced aggregate from its successor. [`policy/autopoiesis.mg:101–137`][policy]. `ShouldRefineToolByKernel` asks whether any `tool_needs_refinement` answer exists for that tool; the factory uses that method before starting its bounded refinement request. [`autopoiesis_kernel.go:575–586`][eligibility], [`factory_tool_executor.go:227–265`][refinement].

This is **not stale derived-fact reuse by the evaluator**. Current evaluation deliberately reconstructs derived state from the EDB, including fresh aggregate/negation semantics, and supports named-predicate cone evaluation. Re-evaluation correctly derives poor quality again because the obsolete aggregate remains a source fact. Preserve that evaluator repair rather than adding an unrelated reset or claiming its truth maintenance is absent. [`kernel_eval.go:274–315`][evaluation].

## Six-outcome witness with explicit score assumptions

Use one legitimately admitted generated binary throughout one live Cortex lifetime; no tool replacement, second writer, crash, fabricated aggregate, or query-order dependence is required. All calls must pass their normal capability, permission, immutable binary/protocol identity, preflight, and validation boundaries. Use distinct execution identities, working feedback storage, successful kernel assertions, and no independent pagination/incompleteness refinement premise.

For a concrete score witness:

1. The tool has no quality-expectation profile. That is a supported state: `GetProfile` returns nil for absent or ledger-only entries, and `EvaluateWithProfile` falls back to ordinary evaluation. [`profiles.go:158–167`][profile-absence], [`autopoiesis_profiles.go:33–40`][profile-fallback].
2. Its name, for example `echo_rows`, matches none of the built-in documentation/search/list completeness hints. Jev output judgment is disabled or unconfigured; the documented pattern fallback applies. [`quality.go:193–217`][hints], [`quality_jev.go:30–75`][output-issues].
3. Three distinct executions actually start and fail. Each receives quality 0.1 and `IssuePartialFailure`; this early evaluator branch does not introduce `IssuePagination` or `IssueIncomplete`. The aggregate at count three is therefore quality 10%, success 0%. [`quality.go:221–239`][failure-score].
4. Three later executions of the same binary succeed, pass the actual named-result validator, take less than one second, and produce at least 100 bytes of clean output matching none of the issue patterns. With no completeness hint, completeness is 0.8; efficiency is 1.0; the ordinary formula yields `0.8×0.4 + 1.0×0.2 + 0.4 = 0.92`. These output/timing/judgment conditions are assumptions, not observed measurements. [`quality.go:242–260`][success-score], [`quality.go:563–615`][generic-scores], [`quality.go:152–190`][patterns].

The current running mean after all six is `(3×0.1 + 3×0.92) / 6 = 0.51`, published as 51%. Success is `3/6`, or 50%. Percentage conversion is already correct and integer-typed. A fourth clean success would move quality farther above the boundary to approximately 56.86%, published as 57%, without removing the old poor row. [`mangle_scale.go:34–44`][percent].

| Source state | Relevant row | Stock consequence |
|---|---|---|
| Earlier cumulative snapshot | `tool_learning("echo_rows", 3, 0, 10)` | poor; refinement needed |
| Current durable cumulative snapshot | `tool_learning("echo_rows", 6, 50, 51)` | acceptable |
| Live kernel after ordinary append publication | both rows, plus intermediate snapshots | poor **and** acceptable; refinement still needed |

The successful outcomes need not repair the tool's code: failures can be input-dependent or transient. This witness concerns a recovered **running quality measure**, not proof that every real defect has disappeared. Historical failures remain valid evidence and should remain recoverable. They should not remain an obsolete copy of a relation intended to represent the current cumulative measure.

The stored known-issue list can legitimately retain `partial_failure`; that issue is not one of the stock policy's separate pagination/incompleteness triggers. This deliberately avoids claiming that improving the average must clear all reasons for refinement. If another legitimate premise still requires refinement, the expected result is continued eligibility for that reason, not an unconditional false answer.

### Model cost is a further conditional consequence

After the recovered aggregate is published, the automatic factory still sees the old poor row through the policy query. If its lifetime is open, the per-call context remains usable, JIT assembly succeeds, and the client accepts the request, it can make another completion request. Disabling JIT or encountering an assembly/client error can prevent that model call without correcting the stale kernel answer. The established defect is therefore stale eligibility; completion count, latency and money are unmeasured. Background scheduling also affects which call's check observes which aggregate, so this report does not assert an exact request count for the six outcomes.

The separate proposal-owner gap means current successful refinement proposals are discarded. Thus this source trace does not show a stale decision changing an installed binary, weakening a permission, or corrupting a user file.

## Existing repair helper and important limits

`SyncLearningsToKernel` already expresses the intended functional-update semantics: for each tool it retracts `tool_learning` by first argument, then asserts the current aggregate. It similarly refreshes known issues. This is a real, relevant building block. `RealKernel.RetractFact` intentionally matches predicate plus first argument, so that retraction does remove earlier snapshots of the selected tool without removing another tool's rows. [`autopoiesis_kernel.go:327–359`][sync], [`kernel_facts.go:912–961`][retract].

However, symbol search at this pin found its declaration, documentation and a test comment, but no production invocation. `SetKernel` synchronizes registered tools and persisted Jev evidence, not this learning-snapshot helper. The live feedback method directly asserts instead. A comment suggesting periodic calls is not a scheduled publisher. [`autopoiesis_kernel.go:17–35`][set-kernel].

Simply adding a call to the existing helper after every execution is not a complete repair:

- It scans all tools for a single-tool update and performs separate retraction and assertion operations; readers can observe an intermediate missing row.
- It ignores publication errors. A durable learning acknowledgment is evidence of learning-store publication, not proof that the executive projection succeeded.
- The store's update lock does not cover the later `GetLearning` and kernel publication. Two live calls can read revisions N and N+1 and publish them in reverse order. A naive retract-then-assert replacement can leave only N. This interleaving is a proposed repair hazard; the basic stale-history witness is sequential and needs no race.
- Broader kernel reset, predicate-wide deletion, or clearing all known issues would erase unrelated or still-valid evidence. Those are not justified remedies.

The EDB fact cap bounds eventual admission. `MaxLearningFacts` currently controls `code_edit_outcome` pruning, not these `tool_learning` snapshots. Exact feedback replay also does not increment the store's counters and equal tuple assertion is a no-op. These protections limit growth but do not make obsolete aggregate rows current or remove their policy support. [`feedback.go:373–383`][aggregate], [`autopoiesis_kernel.go:488–529`][edit-pruning], [EDB admission][dedup].

## Proposed contract and rollout

Update the autopoiesis/core capability specification before implementation. State whether `tool_learning` is a current projection or a historical observation relation. Existing functional-update code and aggregate consumers support the former; if intentional historical latching or hysteresis is desired, model that as a separate explicit policy premise with a resolution contract, not as accidental survival of old averages.

### Preferred current-projection contract

1. **One selected current aggregate per tool identity domain.** A committed aggregate revision supersedes all older current projections for that tool, while the durable execution history remains intact. Preserve the present name-based aggregation scope unless a separate versioning decision changes it; no binary-version migration is necessary to reproduce or fix this witness.
2. **Ordered, coherent publication.** Carry a monotonic per-tool aggregate revision or equivalent authoritative ordering from the committed store snapshot to the projection. Serialize publication for that tool or reject superseded publications so an older completion cannot replace a newer snapshot. Do not hold a global learning lock over model calls or serialize unrelated tools.
3. **Replace before derived consumers decide.** Use a scoped replacement/transaction capability rather than a visible retract/append gap. The target is one reader-visible current snapshot or an explicit unavailable projection, not a healthy-looking incomplete state. Preserve integer percentage typing and stock policy derivation; do not move the threshold decision into a competing Go heuristic.
4. **Treat errors as a separate projection outcome.** Check retraction/assertion/commit results, record pending or failed projection, and provide idempotent reconciliation from durable learning without replaying the native effect. Do not revoke a genuinely durable store acknowledgment merely because a later projection failed, and do not claim that acknowledgment proves executive adoption. Automatic refinement should not proceed on an unverified stale projection after such a failure.
5. **Reconcile existing accumulated state.** On the chosen adoption boundary, replace historical `tool_learning` rows with the authoritative current snapshot for each owned tool, preserving other tools and independently valid known issues. Make the operation repeatable. Define behavior for missing/corrupt durable state rather than silently publishing an empty successful history.

Existing kernel transactions can batch scoped retraction and assertion under one shard lock and invalidate once. They are useful primitives, not an unconditional database rollback guarantee: individual assertion rejection is reported after accepted changes, and Cortex transactions have no cross-shard rollback. The publisher must account for actual partial-failure semantics and verify/reconcile accordingly. [`kernel_transactions.go:84–192`][transaction], [`cortex_kernel.go:849–853`][cortex-transaction].

### Alternative: explicitly versioned history plus a current view

If retaining immutable kernel snapshots is valuable, introduce a declared revision and derive the current aggregate before quality classification. Every quality/refinement consumer must use that current view. A maximum count can select a sequential history only within a defined epoch; resets, imports and identity changes need explicit ordering. Also bound/retire old kernel history independently of durable evidence retention. Adding a latest-row filter alone can fix classification while leaving unnecessary source-row growth and publication-error handling unresolved.

Do not erase valid known pagination/incompleteness reasons merely because average quality recovers. Issue resolution and model-generated refinement adoption remain separate contracts.

## Costs and fair evaluation

### Costs to compare

- **Current behavior:** every distinct successful learning update can add another aggregate tuple until bounded admission intervenes. Re-evaluation must account for those source rows, and obsolete eligibility may cause additional bounded refinement attempts. Neither throughput degradation nor model spend was measured.
- **Scoped replacement:** requires retirement, cache invalidation and fresh derivation of affected policy. `rebuild` invalidates cached atoms and marks named predicates dirty; evaluation can use its current cone path or fall back. Do not describe every replacement as an unavoidable whole-corpus recompilation. Batch one coherent tool update where appropriate. [`kernel_eval.go:449–501`][rebuild], [current evaluation][evaluation].
- **Ordering/reconciliation:** a per-tool publisher adds coordination, revision state and failure recovery. Coalescing may legitimately publish only the latest committed aggregate, but must preserve durable raw evidence and provide an explicit projection-freshness boundary for decisions.
- **Historical-view alternative:** adds schema/rule and migration complexity, aggregation/join work, and a separate history-retention cost. It may be appropriate when historical kernel reasoning is a real requirement, not merely to avoid retiring a stale tuple.

### Proposed acceptance tests and measurements — not run

1. **Normal producer to real rules:** drive the six-outcome sequence through the generated adapter with actual started native outcomes and real validation, then through normal Boot/Process coverage. Use the production domain Cortex as well as focused RealKernel checks. Do not stub `tool_needs_refinement`, inject a finished aggregate, or force a model-quality answer as the primary witness. Observe reopened learning, current EDB projection, derived quality bands and the factory's eligibility check separately.
2. **Recovery and deterioration:** after recovery, poor/refinement should disappear when no other premise supports them. Then add sufficient new poor outcomes and prove they can legitimately return. Also cover a never-poor tool and boundaries around 49/50 and 79/80 without changing percentage semantics.
3. **Valid independent reasons:** preserve a genuinely supported pagination or incompleteness refinement reason after average recovery. Verify another tool's aggregate and eligibility are untouched by the first tool's replacement.
4. **Replay and failed durability:** duplicate feedback must not increment counts or multiply current snapshots; failed store publication must not advance the current projection. Retry publication without executing the native tool again. A successful durable store write followed by projection failure needs an observable repair state, not an implicit successful policy update.
5. **Concurrent ordering:** use barriers after aggregate snapshot acquisition and before publication. Publish N+1 before releasing N; the final selected aggregate must remain N+1. Exercise idempotent reconciliation, cancellation, shutdown, and no unrelated-tool serialization.
6. **Isolate the completion consequence:** separately enable a deterministic JIT-ready refinement client and drain/barrier earlier work before comparing eligibility and completion admission after recovery. Disabled JIT is a negative control for model calls, not a fix for stale eligibility. Preserve the existing cancellation/join test and proposal-only safety behavior.
7. **Measure both correctness and cost:** report current-projection row count, stale-support count, projection lag/errors, affected-cone/full-fallback evaluation time, allocations, unrelated-tool latency, and actual admitted refinement requests/tokens. Compare scoped replacement and a versioned current view at equal histories and policy, not by dropping outcomes or lowering quality thresholds. Final task outcomes must be evaluated independently of the model's predicted gain.

The existing [`TestRecordExecution_RefreshesLearningsContext`][existing-test] verifies that a learning fact is emitted with integer quality 95 and that generation context is refreshed. It uses an assertion-recording mock and one outcome; it cannot establish supersession in a real kernel. Existing [normal Boot/Process durable-learning coverage][boot-test] establishes important production reachability but uses one execution, not a poor-to-recovered transition. Retain both and add the missing temporal boundary.

## Relationship to prior work

The [autopoiesis wiring document][wiring] already exposes learning export and the sync helper; this report makes the unwired functional-update boundary and its stock-policy consequence concrete. The earlier [generated observation-fidelity audit][observation] concerns protocol/observation projection, not aggregate replacement. The [receipt-retention audit][retention] and [multiwriter publication audit][multiwriter] concern different owners and durable-state guarantees; this witness uses one process, one live store and a small finite history. The [working-observation supersession audit][working] concerns source-span replacement in the context ledger, not tool quality aggregates.

**Acceptance boundary:** after a healthy committed learning update and successful projection, the executive classifies the tool from its selected current aggregate, while retained historical evidence remains available in its proper role. No obsolete aggregate should independently keep a current poor-quality classification alive. Legitimate issue policy, durable evidence, exact execution authority, model budgets, and safe proposal/adoption separation must remain intact.

[proposal]: https://github.com/theRebelliousNerd/codenerd/blob/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits/generated-tool-refinement-candidate-continuity.md
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1990-L2018
[record-trigger]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L148-L160
[receipt-feedback]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_profiles.go#L133-L163
[aggregate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L353-L422
[durable]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L461-L497
[live-projection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_feedback.go#L65-L94
[assert-helper]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_kernel.go#L217-L230
[learning-helper]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_kernel.go#L286-L296
[dedup]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/kernel_facts.go#L491-L508
[assert]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/kernel_facts.go#L566-L607
[ownership]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/shards/registration.go#L154-L177
[cortex-assert]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/cortex_kernel.go#L461-L484
[shard-assert]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/kernel_shard.go#L149-L168
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/autopoiesis.mg#L101-L137
[eligibility]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_kernel.go#L575-L586
[refinement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L227-L265
[evaluation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/kernel_eval.go#L274-L315
[profile-absence]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/profiles.go#L158-L167
[profile-fallback]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_profiles.go#L33-L40
[hints]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality.go#L193-L217
[output-issues]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality_jev.go#L30-L75
[failure-score]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality.go#L221-L239
[success-score]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality.go#L242-L260
[generic-scores]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality.go#L563-L615
[patterns]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality.go#L152-L190
[percent]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/types/mangle_scale.go#L34-L44
[sync]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_kernel.go#L327-L359
[retract]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/kernel_facts.go#L912-L961
[set-kernel]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_kernel.go#L17-L35
[edit-pruning]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_kernel.go#L488-L529
[transaction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/kernel_transactions.go#L84-L192
[cortex-transaction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/cortex_kernel.go#L849-L853
[rebuild]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/kernel_eval.go#L449-L501
[existing-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/orchestrator_test.go#L110-L154
[boot-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_generated_feedback_test.go#L511-L590
[wiring]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/autopoiesis/WIRING-AND-NOT-BUILT.md#L16-L35
[observation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/generated-tool-observation-fidelity.md
[retention]: https://github.com/theRebelliousNerd/codenerd/blob/c3a596c8e9444a523b3368a25843cb3bad2de112/Docs/audits/generated-call-receipt-retention.md
[multiwriter]: https://github.com/theRebelliousNerd/codenerd/blob/921628ea7b1af635cb9c99d4055f6feed2a5efeb/Docs/audits/generated-learning-multiwriter-publication.md
[working]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/working-observation-supersession-contract.md
