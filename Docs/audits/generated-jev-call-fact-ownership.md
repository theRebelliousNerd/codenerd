# Jev call settlement needs call scoped cleanup

## Finding and decision

**One generated call can remove another call's temporary Jev evidence before it is classified.** At main `6cbe3962fbdb24a1639b26e71f8096e4f4330048`, `recordCallHit` asserts a tuple containing tool name, execution identity and probability, then classifies that exact tool/call pair. Its cleanup nevertheless uses `RetractFact`, whose intentional contract matches only predicate and first argument. For two distinct calls of the same tool, one cleanup can remove both tuples. The second call then contributes `Unsettled` instead of the landed or missed result its valid judgment would have produced. [Call settlement][settlement], [retraction contract][broad].

The focused repair is to make cleanup match the owned call observation, using the existing exact-retraction surface rather than changing global `RetractFact` semantics. Keep the executive classification in Mangle. Do not turn this into global model-call serialization, automatic retirement, or a substitute for the separate execution-accounting idempotency repair.

This is a conditional static source finding, not a reproduced incident or measured reliability loss. Main was revalidated on 30 October 2026 at 17:44 UTC. Root and scoped guidance, the audit queue, active branches and research through [1722 at `490b0d5`][latest-research] were reviewed. That branch adds three audit documents, not implementation; main remains the revision carrying the four recent restart/reload/recurse repairs. Independent source challenge corroborated the cleanup mechanism and the native-lock lifetime. No application execution, tests, inference or repository edits were performed.

## Required conditions

The primary witness uses two separately admitted executions A and B of the same still-registered generated tool G:

- Their request keys are distinct and valid, and both use the shared factory/orchestrator kernel. Neither is a duplicate or feedback-only retry.
- Both native processes start, return without backend error, and pass the actual named-result validator. A semantic miss below is **not** a failed native process.
- Jev is configured with `AutopoiesisChecks` enabled, returns valid output judgments, and the kernel and ProfileStore are attached. The separate `ToolChecks` switch alone does not enable this route.
- Relevant policy and threshold facts are available and stable. Both assertions and all queries succeed. The schedule needs overlap of the later evidence-settlement work, not simultaneous native execution.
- ProfileStore locking and publication succeed. LearningStore publication also succeeds; no storage failure, crash, restart or retry is needed.

These conditions exclude several real protections. An absent or failed judge does not enter this settlement path. A failed backend or validator gets the early failure assessment and does not reach output judging. A same-key live duplicate shares its receipt. Serialized calls, calls of different tools, or B reading its classification before A removes its observation do not realize the stated loss. [Judge configuration and invocation][judge], [early failure branches][quality], [request rendezvous][factory].

## Production wiring and the ownership mismatch

### Native serialization does not serialize feedback

The factory attaches the orchestrator to `bctx.kernel`, constructs one generated executor and installs it on VirtualStore. VirtualStore preserves the exact authorization envelope, registered binary/protocol identity, preflight and named validator before calling that backend. [Factory installation][boot], [typed bridge][bridge].

The adapter's admission mutex protects its call map and lifecycle. It rendezvouses by `request.Key()`; it does not hold a tool-name-wide lock through quality evaluation. A distinct key creates its own live-call record, then unlocks before backend work. Once the backend returns, the adapter performs validation and calls the feedback recorder. [Admission and ordering][factory].

There is a meaningful per-tool protection inside `ExecuteGeneratedBackend`: it acquires `loop.toolOperations`, checks the exact generated identity and runs the native binary. Its deferred release occurs when that method returns. `RecordGeneratedExecution` is a subsequent method, reconstructing typed feedback and calling `EvaluateWithProfile` before durable recording. Thus A can be in feedback/Jev work while B completes its separately serialized backend and reaches feedback. The witness does not weaken or evade the native operation lock. [Backend lock and recording boundary][backend].

Both profile-aware and generic successful assessments call `outputIssues`. The configured output judge asks under the existing judgment timeout and, on a valid answer, calls `recordCallHit` with `OutputGivesWhatWasAsked` before returning the judgments to quality evaluation. There is no settlement-wide lock around assertion, classification and cleanup. The orchestrator read lock in `recordCallHit` only snapshots its kernel, Jev and ledger pointers. [Evaluation][quality], [judge attachment][attachment], [judge callback][judge], [settlement][settlement].

### The fact has a call identity but cleanup ignores it

The asserted observation is:

`tool_call_hit(G, executionID, Percent)`

The policy derives `tool_call_landed(G, executionID)` at or above the yes threshold and `tool_call_missed(G, executionID)` at or below the no threshold. `callOutcome` queries each whole relation, then filters the returned rows for the exact tool/call pair. An intermediate probability legitimately settles neither way; that behavior must remain. [Policy][judgment-policy], [readback][settlement].

The cleanup passes the complete tuple to `kernel.RetractFact(hit)`, but RealKernel explicitly matches only its predicate and first argument. The source even describes removing all facts for a particular tool as an intended use. This is a **caller/API mismatch**, not a claim that core's documented first-argument behavior is wrong. The per-method mutex orders individual operations, not the caller's compound sequence. [Broad retraction][broad].

The same behavior reaches production Cortex: `RetractFact` selects targets and forwards through the shard to the inner kernel. Its policy-adoption read lock excludes generation replacement during that operation; it is not a multi-operation call-settlement lock. The current manifests leave this call-observation family unowned, so it uses the cortex catch-all; `config_param` is shared. The witness does not require a split join or a missing threshold copy. [Cortex forwarding][cortex], [shard forwarding][shard], [catch-all routing][route], [manifests and shared configuration][manifests].

## Minimal interleaving

Assume A has a valid semantically missed judgment and B a valid landed judgment, both from mechanically successful executions. Use distinct execution IDs, and keep thresholds unchanged.

| Step | A | B | Temporary support |
|---|---|---|---|
| 1 | Asserts its hit tuple and completes both classification reads as Missed | Not yet asserting | A |
| 2 | Paused before cleanup | Asserts its distinct landed hit tuple; pauses before its first classification query | A and B |
| 3 | Calls first-argument `RetractFact` | Still paused | Neither tuple remains |
| 4 | Records one Missed and finishes its ledger replay | Still paused | Neither |
| 5 | Finished | Queries landed and missed; neither contains B, so readback returns Unsettled | Neither |
| 6 | Finished | Cleanup finds nothing; records one Unsettled and replays the ledger | Neither |

This ordering avoids depending on concurrent ledger replay or an ambiguous partial query. After retraction, subsequent queries evaluate the changed kernel state; there is no durable per-call classification cache that supplies B's removed support. [Retraction and rebuild][broad], [query evaluation][query].

`NoteJevCall` increments the selected outcome, including `Unsettled`. `Judged()` and replayed `tool_jev_tally` include only Landed plus Missed. With healthy publication, the wrong classification therefore survives as aggregate ledger state and is replayed after restart. The existing mutex, cross-process file lock, disk reload/merge and atomic publication can correctly preserve that wrong input; they do not reconstruct the observation that was removed before classification. [Tally mutation and replay][tally], [ledger synchronization][ledger], [atomic profile publication][save].

Both actual execution receipts and their ordinary durable learning can still succeed. The judge callback returns its output judgments independently of this tally readback; the cleanup loss is not proof of a false process result, lost raw execution output, failed LearningStore acknowledgment or changed quality score. The narrower affected boundary is the separate tool-value evidence. [Recording][backend], [durable feedback][feedback].

## Consequence at the real consumer

This ledger feeds `ToolLedgerFacts`, `EvaluateToolValue` and the tool-value report, not merely telemetry. The report reads the kernel's evidence, classes, reasons and retirement proposals; the TUI renders it. [Projection][facts], [report][report], [surface][surface].

A bounded stock-policy example starts with G having:

- at least ten legitimately metered calls already;
- five landed and five missed prior settled judgments;
- current generated registration, no unique-capability declaration and no independent dead reason.

After one new semantic miss A and one new hit B, correct accounting is six/six, a tie. Under the interleaving above it is five/six, plus one additional Unsettled. The policy derives `/misses_mark` only in the latter case. With the default ten-call judged floor and metered-call floor satisfied, it can derive `/misses_results`, `/dead` and a generated-tool retirement proposal. No scan/cost evidence or duplicate/harness verdict is needed for this example. [Majority rules][judgment-policy], [floors and missed-results reason][value-policy], [defaults][defaults], [proposal rule][proposal].

This is not a claim that the tool is objectively good or that Jev's judgments are correct. It shows the difference between the policy's conclusion over the two admitted observations and its conclusion over the corrupted tally. Losing a missed observation instead can bias the other way. Many overlaps will not cross any classification threshold.

**Nothing here removes a tool automatically.** Actual retirement still requires an explicit caller, a derived proposal, the work-mode approval broker/grant and dispatch, plus registry/artifact checks. Source retention also remains. Existing retirement/version concerns are separate findings, not impact established by this witness. [Retirement controls][retirement].

## Relationship to prior work

The [1316 feedback accounting audit][retry-audit] owns a different failure: one execution is judged and counted again after its later LearningStore publication fails and feedback repair reenters evaluation. It is a sequential same-ID **overcount**, requiring a failed publication and retry. This continuation uses distinct executions, healthy publication and concurrent cleanup to **lose a settled classification**. Carrying an idempotency key to the ledger does not restore a call already misread as Unsettled; exact cleanup does not deduplicate replayed tally writes. Both contracts can be repaired together without presenting either as a new discovery of the other.

The [expert-selection temporary-fact audit][expert-audit] already establishes the general need for invocation-owned temporary evidence. Its central witness concerns equal shared extension tuples that set semantics coalesce; even exact deletion can remove another invocation's identical support. Here call identity already makes the two tuples distinct. The narrower gap is using a first-argument removal API despite that identity. A reference-counted global fact store is not necessary to close this witness.

The older [fact API inventory][api-audit] already documents first-argument versus exact retraction. Credit that guidance; do not change `RetractFact` globally and break callers that intentionally use entity-wide cleanup. Existing serial per-call truth-table and persisted-ledger/reload tests are useful controls, but their read source does not exercise this assert/read/cleanup interleaving. [Truth-table test][truth-test], [reload test][reload-test].

## Proposed contract and implementation bounds

Update the affected autopoiesis/tool-value specification before implementation. A settlement must retain its owned observation until its complete classification read finishes, then remove only that observation. Cleanup of A must preserve B and unrelated tool observations, even when A and B name the same generated tool.

The smallest candidate is the existing `types.Kernel.RetractExactFactsBatch` with the single asserted `hit` tuple. It is present on the interface and routed by Cortex; RealKernel compares canonical complete tuples. Current `PercentFromRatio` already supplies the numeric representation used by this fact. Test identity and normalization parity rather than assuming all ad hoc representations are interchangeable. [Interface][interface], [exact implementation][exact], [Cortex exact routing][cortex].

Do not move threshold classification into Go as a shortcut. Preserve current Mangle rules and legitimate unknown/unsettled behavior. Do not add a second model request to replace evidence the host can preserve. Merely adding another call ID does not fix cleanup while that ID remains outside the retraction key.

A narrowly scoped serialization alternative could cover the complete assert/read/cleanup sequence, but it must cover every producer sharing that kernel. An orchestrator-local mutex is weaker if multiple orchestrators share it. Holding the native tool lock across Jev or disk publication would add unnecessary head-of-line blocking and is not required by this distinct-tuple witness. A private classification snapshot is another design, with policy-generation/configuration capture, lifetime and copy costs; it is more than the demonstrated minimum.

Define cleanup failure behavior explicitly. Log enough bounded identity to distinguish failure from a legitimate uncertain judgment, and retry or reconcile only owned observations if a cleanup repair path is introduced. Do not erase all observations for G as recovery. Do not rerun the native process to repair a transient fact. Same-key concurrent assessment, deliberate reassessment revisions and the durable exactly-once accounting window remain the separate 1316 contract.

Exact cleanup alone is not a blanket claim of coherent concurrent tool-value reporting. `ReplaceJevFacts` and report projection have their own multi-operation replacement sequences, and policy reloads can occur between separate settlement operations. Keep the primary witness fixed-policy and order ledger replay as shown. Broader snapshot/publication coherence needs its own evidence and contract rather than an unsupported claim that one deletion change solves every concurrency concern.

### Rollout and historic state

No fact arity, provider protocol or ledger format change is required by exact cleanup alone. Retain the existing legitimate unsettled tally. Current aggregates do not identify which historical Unsettled increments came from interference, so do not relabel all of them as landed, subtract them, or guess a correction from metered-call counts. Any reassessment must be explicit, bounded, based on retained evidence and coordinated with idempotent accounting; it must not execute the tool again.

Preserve request authorization and identity, process/validation separation, live receipt deduplication, feedback repair without effects, shutdown cancellation/join, persistence diagnostics, sample floors, unique-capability protection and governed retirement.

## Costs and fair evaluation

The exact-batch API allocates a canonical-key set and filters facts, while first-argument removal uses a simpler comparison. Both already incur kernel mutation/rebuild work when facts are removed. Do not assume exact cleanup is faster, or claim a synchronous full fixpoint on every cleanup; evaluation timing and kernel optimizations matter. The correctness change need not add an LLM call, another native invocation or another persisted input/output body.

Measure key construction, allocation, dirty/evaluation traffic, CPU, latency and contention under realistic fact counts. Serialization would add queueing and cancellation interactions; private snapshots add setup and retained-state costs. Preserving observations may change legitimate classifications and subsequent human decisions; reduced retries, better tools or token savings are hypotheses, not established benefits.

Proposed evaluation, **not run**:

1. **Deterministic minimal witness.** Use barriers around the real assertion/query/retraction calls, not sleeps. A finishes classification, B asserts and pauses, A cleans up, then B reads. Compare current and repaired code with real policy on RealKernel and production-configured Cortex. Expect A Missed and B Landed after repair, not a fabricated successful return from a mock kernel.
2. **Actual generated route.** Through factory plus VirtualStore, admit two distinct authorized requests for G, observe two native effects and actual successful validation, and install deterministic valid Jev responses at the judgment seam. Demonstrate feedback overlap after backend-lock release without requiring native processes to overlap. Capture terminal receipts, assessments and reopened ledger state separately.
3. **Negative and rescue controls.** Same tool serially; different tools concurrently; B reading before A cleanup; genuine intermediate probability; disabled/failed Jev; missing policy/threshold or assertion/query failure; backend/validation failure; pre-start refusal; acknowledged same-key duplicate. Preserve each branch's existing semantics rather than treating every absent judgment as a failure.
4. **Consumer fidelity.** Build the legitimate five/five history and metered-call floor through real measurement paths. Compare settled tallies, `tool_value_evidence`, reason, class and proposal after both calls finish and after reopening/replay. Include unique capability, too-few samples and independent dead reasons. Do not infer automatic retirement from a proposal.
5. **Cleanup and coexistence.** Check exact surviving tuples for the same tool and other tools, normalized percent values, cancellation and cleanup errors, and no retained temporary facts after ordinary completion. Keep the separate same-ID retry/idempotency regression; fixing one contract must not mask the other.
6. **Fair costs.** Hold provider responses, tools, policy, authority, fact corpus, initial ledger and overlap schedule constant. Report classification mismatches, unsettled counts by cause where observable, native starts, judgment attempts, ledger writes, cleanup/evaluation counts, allocations and p50/p95/p99 latency. Do not call reduced work a win if it suppresses a legitimate observation or abandons durable feedback.

**Acceptance boundary:** one call's cleanup cannot remove another distinct call's observation before classification. Successful independent judgments reach the intended landed/missed tally under the policy in scope, while genuine uncertainty, truthful execution receipts, persistence contracts and human-controlled retirement remain intact.

[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/tool_value.go#L39-L105
[broad]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/kernel_facts.go#L912-L961
[latest-research]: https://github.com/theRebelliousNerd/codenerd/commit/490b0d5511da8bb41326917c70737f331548ed1b
[judge]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_jev.go#L29-L85
[quality]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/quality.go#L221-L318
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/system/factory_tool_executor.go#L66-L160
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/system/factory.go#L2006-L2018
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/virtual_store_generated.go#L12-L104
[backend]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_profiles.go#L87-L163
[attachment]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_orchestrator.go#L329-L332
[judgment-policy]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/jev_learning.mg#L89-L138
[cortex]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/cortex_kernel.go#L547-L589
[shard]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/kernel_shard.go#L221-L238
[route]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/cortex_kernel.go#L211-L252
[manifests]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/shards/registration.go#L37-L605
[query]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/kernel_query.go#L40-L65
[tally]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/tool_ledger_jev.go#L74-L185
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/tool_ledger.go#L97-L147
[feedback]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_feedback.go#L37-L95
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/tool_ledger_facts.go#L55-L111
[report]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/tool_ledger_report.go#L90-L178
[surface]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/tool_value.go#L12-L34
[value-policy]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/tool_value.mg#L107-L174
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/config/tool_value.go#L34-L63
[proposal]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/tool_value.mg#L253-L263
[retirement]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/tool_retirement.go#L131-L222
[retry-audit]: https://github.com/theRebelliousNerd/codenerd/blob/30684dc18e934f12e50db05035febe7692174692/Docs/audits/generated-feedback-jev-accounting-idempotency.md
[expert-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/expert-selection-temporary-fact-ownership.md
[api-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/MANGLE_FACT_APIS_INVENTORY.md
[truth-test]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/tool_value_test.go#L33-L64
[reload-test]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/tool_ledger_jev_test.go#L118-L149
[interface]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/types/interfaces.go#L10-L33
[exact]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/kernel_facts.go#L999-L1039
[save]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/profiles.go#L364-L389

