---
doc-class: deep-dive
subsystem: context-perception-cli
implementation-status: partially-implemented (see As-built status)
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Conversation history coverage at the compression handoff

## Decision

Treat compression as a representation change that needs a coverage contract, not as a Boolean license to remove old messages. Every model-facing conversation view should say which source messages it represents, which are available only by reference, and which are unavailable. Narrowing the raw suffix is safe only when that view has been built and admitted for the particular consumer.

The source supports three connected gaps: perception's earlier history cut is not accompanied by a compressed-history view; articulation still cuts its history when building that view fails; and the compressor's recorded subset and atom-only representation do not certify coverage of the full conversation. A static test currently checks the presence of a compression gate, not these end-to-end properties.

**Status:** source inspection and proposed validation only. No tests, builds, live model calls, fault injection, performance measurements or implementation were performed. The witnesses below are deductions from control flow, not observed model mistakes.

## 1. Source freeze, scope and existing work

Main was resolved to [356f65e0][main], and feat/conversation-brief to [c2d39e53][feature]. The comparison reports the feature 27 commits ahead and zero behind main. Source claims below use these immutable pins.

The [root contract][root] requires recoverable context, bounded requests, JIT-first model-facing behavior and policy-owned context decisions. The [perception guidance][perception-guidance] distinguishes model interpretation from executive authority; [prompt guidance][prompt-guidance] distinguishes saved evidence from actual inclusion. The [architecture document standard][standard] requires current code witnesses rather than inherited prose. Those instructions were read before this proposal.

This is not a proposal to increase every window or resend every transcript. It concerns the transition between representations and the evidence that makes that transition honest.

| Adjacent work already read | Boundary left there |
|---|---|
| [Context ledger prefix costs][ledger-audit] | Revision batching and bounded tool-result eviction cost. This note concerns conversation messages, not tool-ledger prefix arithmetic. |
| [Retrieval freshness, coverage and context value][retrieval-audit] | Repository search coverage and cache invalidation. A message's presence in an archive does not establish delivery to perception. |
| [Articulation admission and control fidelity][control-audit] | Accepted versus rejected controls, no replay, and preserved control dispositions. Those feature fixes are credited; general dialogue content still is not equivalent to its control packet. |
| [Perception restriction continuity][restriction-audit] | Current-turn enforcement and restriction propagation. The current action gate must not depend on successful recall; this proposal does not replace that boundary. |
| [TUI evidence turn ownership][tui-audit] | Host read observations and their answering-request delivery. That repaired carrier is separate from old conversation coverage. |
| Conversation brief semantic fidelity and progressive disclosure | Accepted brief-item meaning, applicability and essential-item disclosure. This proposal does not redesign the brief; it covers messages and answers outside its five-field work summary. |
| Terminal knowledge synthesis grounding | Evidence transfer into the post-consultation answer. That separate terminal phase is not the compaction handoff studied here. |

The existing [context wiring document][context-wiring] correctly traces production compressor construction and chat callers. The older [gap matrix][context-gaps] still carries superseded “uncalled” descriptions, so they were not used as current behavior evidence. The current [chat-history parity test][parity-test] is especially relevant existing coverage, discussed below. The precise consumer-delivery/coverage mismatch was not found in these inspected audits; that is a bounded novelty claim, not a claim that no earlier discussion ever mentioned conversation memory.

## 2. What exists, on which branch

| Seam | Main | Feature |
|---|---|---|
| Perception history selection | [processInput 144–184][main-perception] chooses all Model.history or getRecentTurns(compressor window). | [processInput 141–200][feature-perception] retains the same selection and calls perception through briefContext. |
| Perception prompt | [BuildPrompt 261–352][main-prompt] renders ambient context, strategic context, semantic examples, bounded history and current input. | [BuildPrompt 261–358][feature-prompt] additionally renders SessionContext.Brief, whole. Neither renderer consumes SessionContext.CompressedHistory. |
| Articulation handoff | [processInput 809–827][main-articulation] attempts GetContextString, then selects the raw suffix regardless of its error. | [processInput 886–905][feature-articulation] retains that behavior and also carries typed workspace evidence. |
| Compressor feed | [processInput 923–941][main-feed] launches ProcessTurnWithAppliedControls in the completed articulation path. | [processInput 998–1012][feature-feed] does likewise, with corrected constraint and control-disposition metadata. |
| Compressed turn representation | [processTurn 114–141][main-representation] counts user/assistant prose but stores structured atoms and controls. | [processTurn 127–158][feature-representation] keeps that design and adds truthful failed-control outcomes. |

The feature's running brief is a real improvement. [briefContext 100–105][brief-context] creates a SessionContext carrying the rendered current brief, and [BuildPrompt 348–353][brief-render] renders it. It may preserve a goal or decision older than the raw window. It is not evidence that every old answer, explanation, alternative, quoted example or conversational referent survives. Nor is a kernel-backed current brief by itself a claim of restart persistence.

The classification policy deliberately has its own limits: default five prior entries and 14,000 content/reasoning bytes, whole-entry eviction, and retention of the newest entry even if it alone exceeds that budget. [ClassificationConfig and defaults 8–46][classification-config]. Its one-shot lack of recall is explicitly documented there.

**Important qualification:** the production context-window default is also five ([DefaultContextWindowConfig 151–168][production-default]), passed into the compressor by [newSessionCompressor 345–356][compressor-boot]; the compressor's own default agrees ([DefaultConfig 42–64][compressor-default]). Therefore activating compression does not necessarily shrink the default classifier's final raw-entry count: its own downstream window may already have selected the same five. Any claim of an unconditional default-path “five fewer turns” would be false. The issues are replacement delivery, upstream omission accounting and differing configured windows, not that simplistic count.

Also, getRecentTurns counts entries in the Message slice, not user/assistant exchange pairs ([process_follow_up.go 15–20][recent-helper]); the compressor's recent entries are compressed completed exchanges. A future coverage API must name its units explicitly rather than inheriting the ambiguous word “turn.”

## 3. Gap H1: a compression gate is not delivery of replacement history

In the feature, the host narrows history before calling either the firewall or the direct transducer ([processInput 141–200][feature-perception]). The firewall forwards that history to the transducer ([Perceive 487–513][firewall]). The transducer's prompt builder receives the brief but does not render a compressed-history field ([BuildPrompt 261–358][feature-prompt]). The host comment that compressed context handles the rest is not implemented by this consumer.

This establishes no turn-current compressor-summary handoff in the traced perception path. It does **not** establish that perception is ignorant of every old fact: the running brief, learned examples, strategic context or JIT-selected kernel facts can independently carry a relevant fact. The missing property is a connection from a removed message to a consumer-visible representation, not an assertion that all incidental recovery is impossible.

Two narrow source-derived witnesses separate the issues:

1. Configure the classifier to retain ten small prior entries and the compressor to retain five. Before activation, the host supplies all history and the classifier can select ten. After activation, the host supplies only five; the downstream policy cannot recover entries six through ten. A discriminating referent appearing only in that interval is no longer in this history input, even though the configured classification budget would have admitted it.
2. With equal five-entry defaults and a longer history, the raw content selected may be unchanged. However, classificationHistoryBlock sees only the upstream suffix once compression is active. Its omission notice is calculated from that supplied slice ([selection and notice 370–439][classification-selection]), so it cannot describe messages already removed by the host. If the supplied five all fit, the builder sees no local omission even though the wider conversation exists.

The failure risk is not merely weaker prose quality. A request such as “compare that with your earlier second option” can be routed or answered from a view that lacks the referent and does not fully disclose why. The source does not prove a wrong classification occurs; a model may ask for clarification or resolve the reference from the brief. Evaluation must distinguish correct recovery, honest abstention and unsupported resolution.

### Minimal repair contract

Let the classification policy own its bounded raw-history selection once, or pass a typed source-selection descriptor through every upstream cut. If older context is intended to replace removed entries, build a classification-appropriate view explicitly; do not silently reuse the entire generating-model context and its unrelated cost. The request should carry compact provenance and a scoped retrieval/disambiguation disposition when needed. Any new instruction about missing context belongs in JIT atoms, not another ad hoc prompt suffix.

## 4. Gap H2: failed compressed-context construction still removes raw context

Articulation does attempt replacement, unlike perception. But it only assigns compressedCtx when GetContextString returns no error, then unconditionally chooses the raw suffix inside the active branch ([feature processInput 886–905][feature-articulation]; [main 809–827][main-articulation]).

The compressor can reject construction for real reasons: missing kernel or an already-exceeded context budget ([BuildContext 681–699][build-errors]). IsCompressionActive returns true when any summary segment exists or its tracked budget requests compression ([IsCompressionActive 109–125][active]). Thus active does not imply “a successfully built replacement is ready for this request.”

The resulting conversation object can contain the short raw suffix and an empty CompressedCtx. The articulation renderer includes compressed history only when that field is nonempty ([articulateWithConversation 233–242][compressed-render]). The constructor's error is not added to the warnings at this handoff. Other diagnostics or a later provider admission error may still occur; the narrower claim is that this particular missing-history condition is not represented to the answering call.

An over-budget failure is precisely the situation in which “just fall back to all raw history” is unsafe. A useful repair needs a bounded alternate view or an explicit refusal/clarification outcome, not a forced oversized request.

### Minimal repair contract

Construct a single immutable HistoryView result carrying status, source snapshot identity, coverage and admitted size. Decide raw replacement from successful construction/admission, not a separately sampled Boolean. On failure:

- preserve the current request and an explicitly bounded recent suffix;
- state that older conversation was not available to this call;
- permit only claims/actions whose required references and obligations are available;
- retrieve missing authorized material or ask a targeted clarification when necessary;
- never evade provider admission by flooding the full archive back into the prompt.

Optional absent background detail need not block harmless conversation. Required unresolved context should not be silently treated as known.

## 5. Gap H3: recorded turns and atom summaries do not certify conversation coverage

### Lane coverage

The direct perception-answer branch returns a responseMsg before the articulation compressor call ([feature processInput 335–339][fast-return]; [main 291–295][main-fast-return]). The responseMsg handler appends the assistant surface and schedules session persistence, without submitting a compressed turn ([Model.Update 195–201][response-handler]). The traced delegated assistantMsg path also returns before that feed; its handler stores shard results and history ([processInput 700–717][delegated-return]; [Model.Update 157–162][assistant-handler]).

Those surfaces are not necessarily lost from the application. JSON persistence copies the complete Message slice ([saveSessionState 64–76][raw-persistence]), and independent shard/kernel state may preserve some results. The gap is that the compressor's processed subset is not a coverage certificate for that full slice. A summary can exist and keep IsCompressionActive true while later perception-only exchanges occur exclusively in raw history; when those entries age out of the suffix, no linked compressed representation is guaranteed by this path.

This is a conditional source witness, not a claim that every conversation-only session activates compression. A session containing only fast answers may never feed enough data to activate it; the useful witness is a mixed session that already has a segment, then accumulates unrecorded fast-path exchanges.

### Representation coverage

Even when a turn reaches processTurn, its UserInput and SurfaceResponse are used to count original tokens; the constructed CompressedTurn stores intent/focus/result atoms, updates, memory operations and, on the feature, failed-control outcomes ([processTurn 127–158][feature-representation]). Its rolling-summary renderer subsequently serializes those atoms and outcomes ([generateObservationMaskedSummary 449–488][summary-render]). There is no semantic extraction of arbitrary surface prose at this step.

A controlled witness is two assistant answers with equal token counts and identical admitted controls but different substantive option lists. The prose differences do not become distinct conversational content in these compressed-turn fields. After raw eviction, an answer-only detail can therefore be absent from the compressed representation even though “one turn was compressed” is true. Token ratios cannot establish preservation of that detail.

This is an intentional atom-first representation, not an accusation that every summary must be lossless prose. What is missing is a truthful capability/coverage boundary and a recovery path for details the representation was never designed to preserve. The brief-item proposal owns requirements and accepted decisions; this note should reuse that representation rather than invent a second semantic decision ledger.

### Minimal repair contract

Observe completed user/assistant events at a shared host-owned boundary, independent of the response route. Keep control application separate from history observation: moving history recording must not replay memory effects or re-authorize model packets.

Assign stable session/message IDs, parent exchange identity, source digest and speaker/origin. Track explicit per-source disposition: raw in view, represented by a specific summary projection, recoverable from an authorized source reference, deliberately excluded with reason, pending recording, or unavailable. A highest recorded turn number alone is insufficient when routes create gaps.

Retain the existing raw archive as the source of recovery where possible; do not duplicate it into another unbounded store. A summary record should state the information classes it preserves and link its actual inputs. “Processed” is not “all meaning retained”; “available in storage” is not “delivered to this model.”

## 6. Why existing checks do not close this boundary

The [chat_history_parity_test.go contract][parity-test] recognizes GetContextString **and GetRecentTurnWindow** as compressed-context accessors. Its AST scan records whether a function mentions raw history, one accessor and IsCompressionActive. It does not perform control-flow/data-flow analysis, validate a successful return, compare source message coverage, or inspect the final provider request. Both perception and articulation selections live inside one large processInput function.

Consequently, the test can correctly guard a useful naming/architecture convention while still passing the missing-delivery and error branches above. Keep it as a structural tripwire; do not describe it as end-to-end history parity.

The [control round-trip fixture][control-test] waits for the articulated turn and checks sanitized memory/observation metadata and no replay. That is valuable but narrower than lane-complete dialogue coverage. The [long-session compressor fixture][long-test] constructs explicit diagnostic/result atoms and checks bounds/compression behavior. Such a fixture cannot establish preservation of assistant-only details absent from those atoms. None of these tests was run in this study.

## 7. Proposed design, ownership and failure boundaries

Use one request-local history view, not another independent planner.

**Host/Go owns:** immutable source capture; identities and digests; bounded storage/read operations; callback settlement and errors; exact request assembly and byte/token accounting; the list of source spans actually rendered.

**Mangle owns:** which history categories are required for the current route, relevance/retention and exclusion decisions, whether unresolved context requires retrieval/clarification or constrains completion. Rules operate over typed coverage facts, not fuzzy phrase banks.

**Models own:** interpretation and synthesis; proposed semantic memories with source references. A typed reference proves origin, not entailment. Never promote old assistant prose or retrieved document instructions into a new user authorization.

A proposed record can contain consumer, request/session ID, source snapshot revision, raw source IDs, summary IDs with input IDs and preservation class, recovery handles, explicit omissions, build/admission status and final rendered size. These are proposed fields, not shipped APIs. Reuse existing broker/request identity and archive interfaces after their actual contracts are checked.

Important failure cases:

- **Asynchronous recording:** the current feed is a goroutine. A request arriving before it settles must see pending coverage, not assume a contiguous recorded prefix. Serialize/acknowledge ingestion or merge an explicit bounded unrecorded suffix.
- **Session switch/restart:** bind views and pending ingestion to the originating session. Reconcile raw/archive and summary coverage after load. Do not infer coverage merely from a monotonic counter.
- **Duplicate callback/retry:** recording is idempotent by event identity; effect application stays exactly once under its existing contract.
- **Unavailable archive or corrupt reference:** show unavailable; never substitute a semantically similar message as the same source.
- **Oversized required evidence:** page or use a verified compact projection; if requirements cannot fit, negotiate scope or abstain rather than silently clipping.
- **Supersession and historical claims:** source-faithful recall can still be stale as an instruction. Retain timestamps and authority; apply the current brief/restriction contracts independently.
- **Privacy and deletion:** scope recall by session/workspace authorization, honor retention/deletion policy, and avoid unnecessary raw prompt copies or permanent receipts containing secret text.
- **Cancellation:** a canceled view build cannot publish a successful replacement; cancel pending reads, preserve failed/pending status and avoid late publication into another session.

## 8. Full cost accounting and fair alternatives

Optimize verified conversational/task outcome under a request budget, not compression ratio alone.

Measure the whole request lifecycle: raw capture/serialization and durable writes; source-index maintenance; Mangle derivation; summary generation or extraction if introduced; archive retrieval; prompt rendering; input/output/reasoning tokens; cached versus uncached input; provider retries; latency to a useful answer; human clarification burden; memory/disk footprint; and the cost of wrong-route or unsupported answers. Existing atom folding is deterministic; adding an LLM summarizer would be a new cost, not “free compression.”

Compare at least:

1. Current implementation.
2. Independent classifier window with honest omission metadata, plus an error-aware articulation handoff; no new semantic summarizer.
3. Coverage-aware views with targeted source recovery and reuse of the existing brief.
4. Larger raw-window reference where the same provider admission permits it.

Keep models, workloads, tools, evidence and authorization fixed. Separate cold from warm archive/index state and normal from failed reads. Report p50/p95 latency, all logical/provider sends, token totals, bytes retained, recovery frequency, referent accuracy, correct abstention and obligation violations. A cheaper treatment that drops difficult questions or asks the user to repeat everything is not an efficiency win.

Metadata costs are usually smaller than replaying prose, but not zero: per-message manifests and source indexes grow with session length. Compact interval descriptions are safe only for genuinely contiguous coverage; route holes need exceptions or explicit membership. Bound active receipts, retain source references under policy, and measure any compaction of the metadata itself.

## 9. Proposed validation and completion criteria

No tests below were executed.

| Fixture | Required evidence |
|---|---|
| Equal five-entry defaults before/after activation | No invented raw-count regression; report any coverage-notice difference separately. |
| Classifier window ten, compressor five | Final classification request either preserves the configured admissible view or explicitly declares the upstream policy and missing sources. |
| Old sentinel represented only in compressed context | Capture the actual perception request; presence in a compressor object alone is not a pass. |
| Active compressor with BuildContext budget error | Final articulation request has bounded fallback and explicit missing-history status, or an intentional refusal; no silent short-history answer. |
| Existing segment, then several perception-only answers | Every excluded source event has a recorded disposition or authorized recovery reference; fast-path outputs are not falsely claimed compressed. |
| Equal controls/counts, different assistant-only content | Later reference retrieves the actual correct source or honestly abstains; equal atom states must not imply equal source meaning. |
| Delayed recording and rapid next turn | Pending source coverage is retained; no accidental history gap or duplicate effect. |
| Session switch/restart during pending work | No cross-session summary/view publication; recovered state reconciles source identities. |
| Corrupt/missing/deleted source handle | Explicit unavailable result, no substituted or fabricated answer. |
| Required referent exceeds budget | Bound holds, source coverage remains truthful, and action does not proceed on an unacknowledged partial requirement. |

First use deterministic request-capture fixtures to establish transport and coverage. Then use normal production chat journeys with the world model/CodeDOM enabled, mixed lanes, old references, updates/corrections and deliberate unanswerable questions. Evaluate classification/routing separately from answer correctness: a later fluent answer does not repair an earlier wrong execution route.

Proposed build queue:

- **HISTORY-VIEW-01:** specify source identity, coverage units and consumer statuses; replace active-flag-as-readiness with one view result. Exit: error and mismatched-window fixtures inspect final requests.
- **HISTORY-VIEW-02:** route-independent, idempotent event recording without replaying controls. Exit: mixed fast/articulated/delegated paths yield exact source membership and pending dispositions.
- **HISTORY-VIEW-03:** connect omitted conversational detail to scoped recovery, reusing brief and archive capabilities. Exit: assistant-only detail and unavailable-source witnesses pass without widening authority.
- **HISTORY-VIEW-04:** compare end-to-end quality/cost under the same budgets. Exit: measured outcome/latency/token/storage report with failure subtypes, not a headline compression ratio.

Do not call the contract closed merely because a new field exists or a larger window helps one example. Closure needs actual consumer delivery, complete disposition of the source set in scope, bounded failure behavior, and a recovery/abstention outcome for omitted details.

## 10. Primary research foundations and limits

[LongMemEval, Wu et al., ICLR 2025](https://arxiv.org/abs/2410.10813) separates indexing, retrieval and reading and evaluates extraction, multi-session/temporal reasoning, updates and abstention. That supports measuring each stage and testing honest inability to answer, rather than treating stored memory as successful use. Its benchmark results do not measure codeNERD or prove this particular design improves coding outcomes.

[MemGPT, Packer et al.](https://arxiv.org/abs/2310.08560) demonstrates virtual-context management using tiers and data movement. It motivates recoverable history outside a finite request. Its model-managed control scheme is not a reason to move codeNERD's executive policy out of Mangle.

[Lost in the Middle, Liu et al.](https://arxiv.org/abs/2307.03172) finds position-sensitive use of evidence in the studied multi-document QA and key-value tasks. It motivates order/position ablations and argues against equating a large context window with reliable access. It does not establish that compression eliminates positional effects or that the same effect size holds for current coding models.

The recommendation is an engineering inference from these foundations and the source trace: make coverage and recovery explicit, prove final-request delivery, then optimize the representation with measured quality and total cost.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[root]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md
[perception-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/perception/agents.md
[prompt-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/agents.md
[standard]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/09-architecture-doc-standard.md
[ledger-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/context-ledger-prefix-costs.md
[retrieval-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/retrieval-freshness-coverage-and-context-value.md
[control-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/articulation-admission-and-control-fidelity.md
[restriction-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/perception-restriction-continuity.md
[tui-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/tui-evidence-turn-ownership.md
[context-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/context/WIRING-AND-NOT-BUILT.md#L20-L52
[context-gaps]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/context/03-GAP-ANALYSIS.md
[main-perception]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L144-L184
[feature-perception]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L141-L200
[main-prompt]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm.go#L261-L352
[feature-prompt]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/perception/transducer_llm.go#L261-L358
[main-articulation]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L809-L827
[feature-articulation]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L886-L905
[main-feed]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L923-L941
[feature-feed]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L998-L1012
[main-representation]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/compressor_turns.go#L114-L141
[feature-representation]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor_turns.go#L127-L158
[brief-context]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_hold.go#L100-L105
[brief-render]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/perception/transducer_llm.go#L348-L353
[classification-config]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/config/classification.go#L8-L46
[compressor-default]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/types.go#L42-L64
[production-default]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/config/memory.go#L151-L168
[compressor-boot]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/session_shared_boot.go#L345-L356
[recent-helper]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_follow_up.go#L15-L20
[firewall]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/shards/system/perception.go#L487-L513
[classification-selection]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/perception/transducer_llm.go#L370-L439
[build-errors]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor.go#L681-L699
[active]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor_metrics.go#L109-L125
[compressed-render]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/helpers_articulation.go#L233-L242
[fast-return]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L335-L339
[main-fast-return]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L291-L295
[response-handler]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L195-L201
[delegated-return]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L700-L717
[assistant-handler]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L157-L162
[raw-persistence]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/session_persistence.go#L64-L76
[summary-render]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor_turns.go#L449-L488
[parity-test]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/chat_history_parity_test.go#L13-L162
[control-test]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_control_roundtrip_test.go#L245-L277
[long-test]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/long_session_test.go#L25-L100


## As-built status

Verified and built 2026-10-28 on `main` after `c28a1212`. The perception selection, articulation handoff and compressor feed were unchanged from the audited pins.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| H1: the host cuts perception history to the compressor window, but perception renders no compressed history | Real, fixed | `processInput` now passes all of `m.history` (`cmd/nerd/chat/process.go:152`); `BuildPrompt` still renders no `CompressedHistory` (`internal/perception/transducer_llm.go:349`) |
| H1: the classification notice is computed on the cut slice and cannot count host-removed turns | Real, fixed by the above | `classificationHistoryBlock` now sees the whole conversation (`transducer_llm.go:458`) |
| H1 qualification: equal default windows (5) give the same raw count | Confirmed | The fix changes what the notice can count and what a larger configured classification window can admit |
| H2: articulation keeps the short window when `GetContextString` fails and says nothing | Real, fixed | `articulationHistoryView` returns the view, or a statement of how many earlier messages are missing and why (`cmd/nerd/chat/history_view.go`; call at `process.go:912`); the window still bounds the request |
| H3: fast-path and delegated answers return before the compressor feed | Confirmed, left open | `responseMsg`/`assistantMsg` paths in `processInput` |
| H3: compressed turns store atoms, not answer prose | Confirmed, intentional; left open | `internal/context/compressor` `processTurn` |
| The parity test checks presence of a gate, not delivery | Confirmed; its comment corrected | `internal/context/chat_history_parity_test.go` (perception is now raw-only; the test still guards articulation) |

### What was built

- Rulings: (1) the classification policy owns perception's history selection, once; no compressed view is built for perception. (2) A failed articulation view keeps the bounded window and puts a factual coverage statement in the compressed-history block (data, not an instruction, so no prompt atom).
- `cmd/nerd/chat/process.go` (two hunks), `cmd/nerd/chat/history_view.go`; spec `Docs/architecture/cli/03-GAP-ANALYSIS.md` `GAP-CLI-HISTORY-HANDOFF`.

### Tests

`cmd/nerd/chat/history_view_test.go`: with an active compressor (a loaded segment) and 20 prior messages, perception receives all 20 (it received 5 before); a failed view build yields the coverage statement with the count and reason, a built view is passed through, nothing omitted yields nothing. `internal/context` parity tests pass.

### Left open

- H3 route-independent, idempotent exchange recording with per-message coverage dispositions (HISTORY-VIEW-02/03): not built. It needs stable message identities through the fast, delegated and articulated routes and a decision on what a compressed turn preserves of answer prose.
- No source-identity manifest per request (HISTORY-VIEW-01 beyond the failure statement) and no cost comparison (HISTORY-VIEW-04).
