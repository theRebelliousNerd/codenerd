---
doc-class: deep-dive
subsystem: session-context-observation
implementation-status: planned
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Working-context supersession must describe evidence returned, not reads requested

## Decision

Keep the bounded read codec, durable recall, append-only request ledger and Mangle-owned compaction. Change the evidence supplied to supersession: a later read may replace an earlier observation only when its **returned, admitted content** demonstrably covers the earlier content at the relevant revision. Requested line bounds and a failed attempt are not coverage witnesses.

The current source has two composed failures at this boundary:

1. A successful wide `read_file` can return only a bounded prefix, while its ledger metadata claims the entire requested range. That metadata can retire an earlier observation of a tail the later result never showed.
2. A failed `read_file` is recorded with the same requested-span machinery. Although its durable record preserves `Failed`, the ledger fact projection does not use that field. An error-only broad attempt can therefore supersede successful content evidence.

This changes what the next solving request carries, not whether the old bytes exist anywhere. Compaction replaces the old body with its recall pointer; it does not delete its durable record. No false visible completion, authorization bypass, or inevitable wrong final answer is established.

**Evidence status:** static source and test-definition inspection only. No codeNERD execution, tests, model inference, implementation, failure injection, benchmark or implementation mutation was performed. The examples are unexecuted source-derived witnesses. This document is a research proposal, not an implemented repair.

## Source freeze and nonduplication

The branch inventory was checked at approximately 18:38 UTC and again at 18:43 UTC on October 28, 2026:

- main: [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871)
- active `feat/conversation-brief`: [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`](https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c)
- current research branch: [`cc0e7c60d292cd0286a3e410a937c0e63b3286cf`](https://github.com/theRebelliousNerd/codenerd/commit/cc0e7c60d292cd0286a3e410a937c0e63b3286cf)
- historical research branch: [`c592ff223cd207fca4c29ae793d5eba57aa4781d`](https://github.com/theRebelliousNerd/codenerd/commit/c592ff223cd207fca4c29ae793d5eba57aa4781d)

The five central files—`internal/session/working_context.go`, `internal/tools/core/file_ops.go`, `internal/observation/fileread.go`, `internal/context/working_set.go` and `internal/context/working_set.mg`—have identical Git blob identities on main and feature. This is not a regression introduced by the feature. The surrounding executor differs; its shared accounting/error path was separately read on feature, with main's existing error-recording path also inspected.

Root, session, tools and articulation guidance were read. The [session contract](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/agents.md) explicitly says a later read supersedes an earlier span it covers, while whole observations remain recoverable. The [root North Star](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md#north-star) assigns retention to Mangle and measurement/transport to Go. This proposal corrects the measurement contract rather than moving retention decisions into a parallel Go executive.

Adjacent current and historical audits were compared:

| Existing work | Boundary not repeated here |
|---|---|
| [Context-ledger prefix costs](https://github.com/theRebelliousNerd/codenerd/blob/cc0e7c60d292cd0286a3e410a937c0e63b3286cf/Docs/audits/context-ledger-prefix-costs.md) | Optimizes revision collection and ordered compaction while preserving existing selection. This finding challenges the input coverage evidence, not that optimization. |
| [Research observation identity](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/research-observation-identity-and-admission.md) | Remote results inherit workspace identities and lose source/page provenance. Here both observations really concern the same unchanged local file. |
| [Workspace search/navigation fidelity](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/workspace-search-navigation-fidelity.md) | Lexical occurrence→relation/address projection and source reread consistency. No lexical search or source race is needed here. |
| [Edit-observation preconditions](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/edit-observation-precondition-continuity.md) | Preservation of edit conditions and commit-time source identity. This finding requires no mutation or conditional edit. |
| [Generated-tool observation fidelity](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/generated-tool-observation-fidelity.md) | Producer partial-return/stderr loss. Here the successful bytes were returned and retained, then wrongly classified as redundant. |
| TUI evidence ownership, delegated failure settlement, and current native-response/continuation studies | Turn/result ownership, terminal settlement or response-control delivery. This occurs inside a healthy ordinary worker before final settlement. |

Searches of the inspected audit material for `observedSpan`, `wholeFileSpanEnd`, requested-span and supersession terminology did not locate this composed defect. The novelty claim is limited to that inspected corpus, not every historical discussion. The [context TODO](https://github.com/theRebelliousNerd/codenerd/blob/cc0e7c60d292cd0286a3e410a937c0e63b3286cf/Docs/architecture/context/TODO.md#L20-L33) correctly credits the ledger and recall implementation; this is not an unwired-feature claim.

## Actual route and what already works

A normal delegated TUI task uses the observed task executor, whose inline task-owned clone invokes `ProcessWithIntent`. The session's normal working loop executes tools, records results, and prepares the next provider request through the working ledger. This is not the residual TUI `execution_result` count-difference route. [Delegation adapter](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation.go#L50-L74), [task execution](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L352-L409).

The native batch calls `executeAndRecordToolCall`; that shared native/Piggyback boundary settles effects and calls `recordWorkingResult` before returning a tool error. Errors remain `IsError` in the model-facing result, and nonempty returned output is preserved with them. These are real safeguards, not missing work. [Batch and shared accounting](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L943-L1052).

`read_file` reads the file, validates numeric bounds, constructs a `precondition.Read` with the captured content, and invokes `observation.EncodeRead`. The codec bounds the projection and accurately labels omitted lines and byte cuts in its returned text. Its edit-precondition handle and the working ledger's recall handle are separate facilities. [Read implementation](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/core/file_ops.go#L142-L231), [projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/observation/fileread.go#L106-L200), [honest display](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/observation/fileread.go#L275-L305).

The defect is not that the displayed read pretends to be complete. It is that **the executive receives different coverage semantics from the model**.

## Finding A: requested range becomes false replacement evidence

`recordWorkingResult` stores `Start` and `End` from `observedSpan(call)`. That helper uses the call's requested `start_line` and `end_line`; an absent positive end becomes `wholeFileSpanEnd = 1_000_000_000`. Neither the returned `FileReadResult.Start/End` nor its `RegionCut`, omitted content, or outcome enters this calculation. [Recorder](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L409-L429), [requested-span helper](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L515-L537).

The codec does not necessarily return those requested lines. `budgetRegion` reduces an overlarge requested anchor to the line ceiling and then removes lines for the byte ceiling; an oversized remaining line can itself be cut. The default limits are 400 lines and 24 KiB, although production uses resolved configured values. [Defaults](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/observation/fileread.go#L76-L95), [line/byte budgeting](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/observation/fileread.go#L209-L241).

`WorkingSet.Ledger` publishes each stored positive end as `working_span`. Mangle then derives `working_superseded` when a later observation on the same entity/revision has enclosing bounds. At compaction, a superseded observation is dropped even inside the protected recent-round window. [Fact projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_set.go#L243-L271), [supersession](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_set.mg#L201-L219), [compaction/drop](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_set.mg#L249-L300).

### Unexecuted stable-source witness

Use an unchanged, authorized plain-text file of 1,000 short, distinguishable lines, no structural model, and limits of 400 lines/24 KiB. Give each tool call a distinct ID.

1. Successfully read lines 700–710. The returned body contains the task-relevant tail detail; the working record's requested span is 700–710.
2. Successfully read the same file with no bounds. The codec returns lines 1–400, with an honest omission notice and no tail content. The working metadata nevertheless becomes 1–1,000,000,000.
3. Let the ledger cross its configured compaction ceiling while keeping the actual provider window sufficiently large to isolate policy compaction from emergency window archiving.

The span rule marks the first observation superseded. The host consumes the eviction decision and replaces its body with `compactedResultHandle` in the next request. The new read did not provide replacement evidence for lines 700–710. [Decision consumption](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L744-L784), [request projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L577-L604).

An explicit request for 1–1,000 has the same issue; missing bounds are not essential. Smaller configured line ceilings and byte-limited projections create analogous cases. Snapping outward to a larger code element can cause the inverse mismatch—metadata understates actual coverage—but does not fix overclaimed coverage.

## Finding B: failure is retained, but excluded from the retention decision

`WorkingRecord` has a `Failed` field and the store persists/loads it. The session sets it from `toolErr != nil`. The ledger projection asserts observation, digest and span facts for these records without a failure qualification, and the supersession rules have no success/coverage-validity antecedent. [Record type](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_store.go#L23-L41), [metadata retrieval](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_store.go#L315-L349), [ledger projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_set.go#L259-L264).

A separate deterministic witness replaces step 2 above with valid JSON arguments naming the same existing file and `start_line: 1.5`, with no end. The read handler explicitly rejects fractional bounds. The attempt still goes through shared result recording; `observedSpan`'s failed integer coercion leaves start at 1 and end at its whole-file sentinel. The result can contain only an error yet supply a covering span at the unchanged file revision. This does not depend on a transient filesystem failure or a model inventing successful output. [Numeric refusal](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/core/file_ops.go#L175-L185), [record before error return](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L1042-L1052).

The fixture must reach `executeAndRecordToolCall`: a malformed JSON argument string caught in the earlier `ArgsError` branch, or a batch call skipped because its context already ended, is a counterexample, not this route. Inspect the actual recorded failure and record metadata rather than assuming every failure is archived identically. [Earlier bypasses](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L920-L944).

The same-kind supersession rule is an additional review obligation: merely suppressing `working_span` on failures does not prevent a later failed retry of an identical request from replacing an earlier successful observation via request-kind equality. A complete repair must make every replacement rule distinguish diagnostics/attempt recency from content-evidence replacement. This is source-visible policy structure, not a claim that the fractional-bound fixture exercises the same-kind rule.

## Scope, counterexamples and limits

- Below the compaction ceiling the body remains in the ledger. Supersession may be derived, but the demonstrated request loss requires an eviction boundary.
- The old observation remains in durable task-local storage and its pointer is sent. This is **unnecessary loss from the active model context**, not irreversible data loss. Recall can recover it during the owning scope's lifetime.
- Recall protection is conditional on `working_live`, which excludes superseded observations. A wrongly superseded result is therefore not guaranteed the protection normally afforded a recalled live result while the purported replacement remains in the ledger. This is a conditional repeated-eviction risk, not proof of an infinite loop. [Liveness and recall protection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_set.mg#L272-L300).
- A genuinely encompassing successful read at the same revision is a valid replacement and must remain one. Identical successful bodies also permit useful deduplication.
- A changed revision should be treated through freshness rules, not as the stable-source fixture here. This report does not diagnose the separate time between acquisition and revision sampling.
- Search/outline results intentionally have no `read_file` span. An outline naming an omitted region is not its full content.
- Empty files, out-of-range clamping, byte-truncated single lines and failures with useful partial output require explicit coverage semantics, not blanket classification as either full content or no useful evidence.
- Tool errors remain errors; the existing gates and terminal verification remain in force. No measured coding failure frequency or performance regression is claimed.

## Proposed contract

1. **Producer-owned coverage.** Return a host-owned structured observation sidecar from the read/projection boundary: exact captured source identity, actual projected ranges, complete/partial line boundaries, omitted-domain information and outcome. Keep requested range separately for diagnostics. Do not infer executive facts by reparsing model-facing prose or by interpreting tool arguments as results.
2. **Typed replacement eligibility.** Publish measured coverage/outcome facts alongside the record. Let Mangle derive whether a new observation can supersede an old one. A failed attempt can be the latest diagnostic without becoming replacement evidence. Useful partial output needs explicit verified coverage; `Failed` alone does not say which bytes are safe to use.
3. **Conservative first increment.** Until structured coverage is available, omit span-based replacement when actual coverage cannot be established and prevent failures from retiring successful content through any rule. Retain archive/window budgets; unknown coverage does not mandate unlimited retention.
4. **Separate exact deduplication from supersession.** Same request at the same file revision is not proof of equivalent successful content. Preserve body-digest deduplication only under a compatible outcome/provenance contract; do not accidentally replace a successful result with a diagnostic.
5. **Preserve recall and ownership.** Keep bodies immutable, task-scoped and recoverable. Fix selected-context classification without replaying reads/effects merely to reconstruct metadata. Existing persisted records lacking trustworthy coverage should be unknown, not retroactively declared complete.
6. **Keep policy/execution authority intact.** No wider tool catalog, no shell workaround, no weakened permission or post-edit gate, and no mandatory new model call. Update session/context/observation specs before implementation; model-facing explanations remain JIT-first where new behavior needs instruction.

## Existing tests and proposed evaluation — not run

The existing [covered/repeated-read ledger test](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/working_ledger_test.go#L67-L88) supplies spans directly. It usefully establishes the policy's containment behavior, not that production read projections justify those spans. The [long-file codec test](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/core/read_codec_test.go#L238-L268) checks the bounded read and outline, not subsequent ledger decisions. Existing [request/recall tests](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_ledger_request_test.go#L106-L153) correctly demonstrate recoverable archived bodies. None of these facts is a claim that tests were run here.

Use the real registered `read_file`, ordinary session accounting and request preparation, with a deterministic provider capturing requests rather than ignoring tool definitions. A small valid ledger ceiling can isolate the boundary; repeat an end-to-end cohort with ordinary production configuration and sufficient distinct observations to trigger real compaction. Preserve CodeDOM/world context and the actual offered/authorized tool path in TUI integration cases.

| Evaluation | Required oracle |
|---|---|
| Narrow tail, then successful default/broad capped read | Later actual content excludes tail; earlier body is not classified as covered. |
| Narrow content, then recorded fractional-bound failure | Error stays error; prior content not retired by the failure. |
| Same-argument successful read then controlled failed retry | Same-kind rule cannot replace content with a diagnostic. |
| Successful truly encompassing read | Legitimate supersession remains; no blanket retention regression. |
| Byte ceiling, oversized line, UTF-8 boundary | No complete-line claim for a partial line; exact safely rendered coverage. |
| Snapped element, explicit range, clamped range, empty file | Recorded metadata agrees with actual projected content, not requested coordinates. |
| Below ceiling, policy compaction, emergency input-window archiving | Distinct mechanisms and reasons remain visible; unavoidable archiving is not mis-scored as this defect. |
| Recall after false-replacement candidate, then another compaction | Correct liveness/retention; exact recoverable body and stable owner identity. |
| ArgsError or canceled-before-execution control | Do not falsely claim the recording route was exercised. |
| Failure with genuine partial content | Diagnostic preserved and only verified partial coverage can replace anything. |
| Main/feature, native/Piggyback | Same evidence meaning at the shared boundary; proper transport framing retained. |

Inspect four levels independently: tool projection, stored metadata, derived `working_superseded`/`working_evict`, and actual next-provider request. Check durable recall separately. Final answer prose alone is not an oracle. Then use paired tasks whose correct answer requires the tail detail, measuring grounded answer/verified-task outcomes independently from transport correctness. Include unsuccessful, blocked and unresolved tasks in every denominator.

The [End-to-End Arguments in System Design](https://web.mit.edu/saltzer/www/publications/endtoend/endtoend.pdf) argues that application-level correctness must be checked at endpoints that possess the necessary knowledge. Applied here, a request's bounds do not prove which bytes reached the consumer; a producer-owned coverage witness is a design inference, not a measured result.

## Costs and alternatives

Compare A current behavior, B conservative disabling of unsupported coverage replacement, and C structured producer coverage plus outcome-aware policy. Option B is simpler but may retain more redundant reads, increasing prompt bytes or forcing ordinary budget eviction. C adds sidecar propagation, storage migration, fact cardinality and policy/test complexity; noncontiguous ranges may cost more than one interval per observation. Keep representation bounded and explain omissions rather than silently coalescing gaps into covered content.

Account for source/projection hashing, serialization, host CPU, allocations, metadata/storage bytes, kernel evaluation, compaction frequency, provider prefix-cache disruption, input/output/cache/reasoning usage, recalls, rereads, tool calls, model rounds, human recovery and end-to-end latency. Report p50/tails and cohort sizes, not only totals. Reuse the already-captured source/projection rather than rereading the file merely for metadata; never claim that a new read reconstructs the original observation.

Retaining previously misclassified evidence can increase immediate context cost while reducing repair turns. Extra retained content can also hurt under tight budgets. Correct failure labels may retain diagnostics and content together. No numerical savings or performance win is established by source inspection; compare equal-authority, equal-acceptance workloads and report useful completion separately from conservative refusal.

## Exit

A successful repair conserves the meaning of observed content across bounded read, archive metadata, policy and next request: omitted or failed content never serves as a replacement witness; real coverage still enables deduplication; all bodies remain bounded/recoverable under their existing ownership; and the measured total cost is justified by verified task outcomes. The immediate architectural unit is an honest **coverage-and-outcome witness**, not another planner or a larger context window.

## Appendix: bind an observation’s revision to the bytes acquired

**Evidence:** static inspection only; no implementation, tests, provider inference or benchmarks executed. Main is pinned at `356f65e0e6bb639adaa90ab36212daa714114871`; feature at `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`. The affected `working_context.go`, `file_ops.go`, `working_set.go`, `working_set.mg`, `working_revision.go` and CodeDOM `elements.go` have identical Git blob identities across those heads. The feature’s shared executor also still runs the tool before recording its observation. [Feature boundary](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L984-L1052).

### Distinct finding: a later source digest can certify earlier returned bytes

The earlier findings concern coverage and outcome on an unchanged source. This appendix concerns **which source version the returned content belongs to**, even when the read succeeds and its coverage is accurate. It also differs from the historical workspace-search study’s match-versus-projection reread: this witness can use an ordinary `read_file` whose projection is already coherent, then lose the version binding during session adoption.

`read_file` acquires content, constructs `precondition.Read` from that buffer, and renders the observation from it. The producer therefore possesses the bytes whose identity should date the observation. [Acquisition and projection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/core/file_ops.go#L159-L231).

The shared executor receives the rendered string, settles effects and then calls `recordWorkingResult`. That recorder independently obtains `loop.set.Revision(observed)` and stores the earlier returned body with this newly sampled revision. The body is not used to establish that source revision. [Shared boundary](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L978-L1045), [record construction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/working_context.go#L409-L424).

For a file entity, `Revision` reopens and hashes the current file. For an element, it rereads and parses current source before retrieving that element’s revision. These are valid **current-source measurements**, but they do not identify the bytes the preceding tool acquired. [Revision collection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/working_revision.go#L55-L187).

#### Unexecuted concurrency witness

Use an authorized, small plain-text file whose entire returned content fits the projection limits:

1. The registered `read_file` acquires version **A**.
2. An external or cooperating writer atomically replaces that file with different version **B**, after acquisition but before the recorder’s revision read. The read result continues to contain A.
3. `recordWorkingResult` samples B and saves `(body derived from A, revision hash(B))`.
4. Leave the file at B and recall the archived observation.

`Recall` compares its stored revision with a fresh current revision; both are hash(B), so its JSON reports `stale:false` while returning A’s body. The ledger likewise receives the saved B revision and current B revision. Its staleness rules do not derive stale for this mismatch; absent another independent supersession condition, the observation qualifies as live. [Recall](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/working_set.go#L107-L121), [fact projection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/working_set.go#L243-L271), [staleness](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/working_set.mg#L186-L199), [liveness](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/working_set.mg#L272-L283).

Unlike the earlier span finding, false freshness does not require compaction. Below the ceiling, `working_restate` also depends on `working_stale`, so this misbound record does not produce the correction it would produce with an acquisition-bound A revision. [Restatement rule](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/working_set.mg#L408-L422).

The element route has the same boundary: the producer renders an element’s revision and source from its captured model, but the recorder resolves the ref again and independently measures revision afterward. An unchanged key whose body changes during that interval can be stamped with the later element revision. [Element rendering](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/codedom/elements.go#L742-L813), [post-call resolution](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/working_context.go#L375-L387).

#### Production concurrency scope

This is **not** a claim that adjacent calls in one native tool batch execute concurrently. `executeToolBatch` iterates calls and synchronously completes `executeAndRecordToolCall` before advancing to the next call. A later write in that same batch cannot land between the preceding read and its recorder through this loop. [Sequential batch](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L901-L937).

There is nevertheless a source-supported in-process concurrency route, not only an external-editor hypothesis. Campaign scheduling launches eligible tasks in separate goroutines up to its concurrency limit. A nonmutating task with no explicit `WriteSet` returns no resolved write set, and an empty set acquires no lease. Thus a read-only task and an independently eligible writer can overlap when policy/dependencies permit both and capacity exceeds one. The writer’s write-set protection is not a reader snapshot lease. [Task scheduling](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/campaign/orchestrator_tasks.go#L229-L266), [lease acquisition](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/campaign/orchestrator_tasks.go#L459-L478), [nonmutating write-set resolution](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/campaign/orchestrator_tasks.go#L548-L573).

The session invokes `guardWrite` only for write-mutation tools; the guard’s documented role is obtaining leases for mutation targets, not holding an acquired read stable until archival. Preserve that useful writer coordination. Do not call the absence of a reader lease a permission failure or assume every campaign overlaps these tasks. [Write-only guard boundary](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L984-L1005), [guard contract](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/write_guard.go#L15-L23). The required schedule remains a conditional, unexecuted witness; a single-task or dependency-serialized campaign is a counterexample to this internal concurrency route.

### Scope and safeguards

- **The timing is essential.** Mutation after the record has been correctly stamped A is a control: a later B measurement should detect staleness. An unchanged source is another control.
- No claim is made about frequency, a reproduced bad answer, or an actual user incident.
- This is not a permission bypass or proof that stale edits will be accepted. File-read preconditions retain the captured read and compare it against bytes supplied by the editing caller. Where an edit passes and enforces that handle, this separate protection remains useful. [Captured-read retention](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/precondition/precondition.go#L201-L219), [verification](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/precondition/precondition.go#L222-L295).
- A current focus view may independently expose B, but does not repair the archive’s false version binding. A later mutation may eventually make the misbound record stale, but that does not retroactively identify A.
- A content digest identifies acquired bytes; it does not promise that those bytes remain current at delivery. The proposed contract must preserve that distinction rather than promise an atomic workspace snapshot. Capturing one buffer also does not, by itself, prove an atomic read against arbitrary in-place external writers; that stronger acquisition question is separate from attaching an unrelated later digest.

### Contract extension

Extend the earlier producer-owned coverage witness with **acquisition-bound source identity**:

1. Carry canonical source identity, acquired source revision, projection coverage, outcome and projected-body digest together through the tool-accounting boundary.
2. Derive the acquired revision from the exact source buffer/model used to render the result—not a subsequent reopen. The existing `precondition.Read` and CodeDOM model already supply appropriate inputs.
3. Keep **observed revision** separate from **current revision**. Mangle should compare those measurements and derive freshness, retention and reacquisition needs.
4. If provenance is unavailable, preserve useful historical output with an explicit unknown binding; do not substitute a later live digest as proof of acquisition.
5. Treat reads and mutations separately. A mutation’s post-state receipt must not be substituted for the pre-state evidence from which a read or decision was made.
6. Preserve task-local immutable archives, current permission checks and recall handles. Do not replay an effect or introduce another archive merely to recover missing metadata.

This complements, rather than replaces, actual-coverage and outcome-aware supersession. It does not require serializing all reader/writer work: preserving the acquired version allows useful historical evidence and honest stale detection without a new global lock. A stronger stable-source admission requirement would be a separate policy decision, with its blocking and retry costs declared.

### Proposed validation — not run

Use deterministic synchronization between acquisition and recorder stamping, with the real registered producer and shared session accounting:

| Fixture | Required oracle |
|---|---|
| Read A → replace with B → record | Archived acquired revision identifies A; current comparison identifies B; no false `stale:false`. |
| Record A → replace with B | Existing ordinary stale detection remains correct. |
| No source mutation | Stable observation remains current without needless reacquisition. |
| Element key unchanged, body changed in the interval | Stored revision remains the producer’s A element revision. |
| Unrelated element changed | Preserve existing element-level identity semantics. |
| Delete/rename before recording | Retain acquired identity; report current absent/unavailable separately. |
| Read precondition used after A→B | Existing stale-edit protection remains distinct and effective. |
| Below ceiling, compaction, recall | Inspect archived metadata, derived facts and actual next-provider request separately. |
| Eligible campaign reader without a write set plus permitted writer | Exercise the cross-task schedule; retain writer leases and existing permissions. |
| Same native batch; concurrency limit one; hard task dependency | Do not claim an internal overlap where execution is serialized. |
| Native and Piggyback on main/feature | Same observation meaning despite transport framing. |

The existing recall test verifies restoration of original ownership, and revision tests verify changes between separately measured epochs; neither fixture shown injects mutation between producer acquisition and recorder stamping. [Recall fixture](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/working_context_test.go#L314-L365), [revision fixtures](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/working_revision_test.go#L119-L184).

### Costs and acceptance

Passing captured provenance adds metadata propagation and ownership complexity, but can avoid the recorder’s extra source read/hash or element parse. It does **not** eliminate later current-source validation. Measure acquired bytes, duplicate reads/parses, hashing, buffer lifetime, storage, kernel facts, request bytes, stale notices, recalls, repair turns and end-to-end latency.

Compare current behavior, conservative unknown-provenance handling, and acquisition-bound metadata. Hold permissions, projection limits, model and acceptance criteria constant. Unknown handling may increase rereads; precise binding may reduce them. Include the source of concurrency, edit rate and serialized controls in the cohort description. If a stronger read-lease design is compared, charge contention, writer delay, cancellation and reader starvation rather than calling serialization free. Neither savings nor quality gains are established here.

**Exit:** a successful observation cannot acquire a newer source revision merely because that version existed when the recorder ran; original bytes remain recoverable, current-source judgments remain separate, and existing edit and context-budget safeguards remain intact.
