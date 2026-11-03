---
doc-class: deep-dive
subsystem: cli-articulation-context
implementation-status: proposed-continuation
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
supersedes: []
---

# Carry the answering turn's shard evidence into knowledge synthesis

## Finding and decision

The repaired knowledge-synthesis handoff preserves recent conversation, acquired workspace evidence and compressed history, but omits the separate shard-result evidence that the initial answering call receives. `ConversationContext.LastShardResult` can supply a prior review's findings to initial articulation after the original reply is outside the raw-history suffix. `knowledgeParentOf` copies only `RecentTurns`, rendered `Evidence` and `CompressedCtx`. The terminal call renders those fields and consultation results, not the last shard result or shard-history block. [Initial renderer][first-render], [parent projection][parent], [terminal renderer][terminal]

**Decision:** extend the existing immutable parent-grounding contract to conserve the bounded shard-evidence view actually admitted to the first answering request. Do not rerun the review, replay all history, reacquire the newest global shard result, or treat a specialist's generic advice as a guaranteed replacement for the omitted finding.

This is a **follow-through on `terminal-knowledge-synthesis-grounding.md`**, whose broad requirement already covers answer-critical parent evidence. Its as-built section accurately credits the evidence/recent-history/compressed-history repair; this report does not claim those fields remain missing. The newly isolated residual is the unprojected blackboard field after that repair. It is not another final-answer ownership defect, a review-parser defect, or the forty-finding admission limit. [Prior audit and implemented scope][prior], [answer-identity repair][identity]

**Evidence:** public `main` remained `388205138ba6d47c1cff0c5e97c896dfa642a391` at the approximately 16:46 UTC recheck on 2026-10-30. Guidance, main's audit inventory and additions through `research-main-20261030-1619` (`3ef34bc061b497b9cfc471f4cc46f5df595b299d`) were reviewed. Independent source challenge confirmed the narrow field omission and the conditional aged-suffix witness. No tests, application execution, inference, benchmarks or failure injection were performed. This is a research proposal, not a reproduced incident or an implemented fix.

All five inspected active code/QA heads retain the same three-field `knowledgeParent` and projection function: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `767a3291d6287b238417f2466592a9002810d05a`. Their complete `process_knowledge.go` files differ from main; no whole-branch equivalence is claimed.

## Existing protections and intended behavior

The source deliberately supports follow-ups through the normal perception/routing/articulation pipeline. It removed a substring-based interceptor that incorrectly redirected unrelated questions to the last shard. The remaining explicit blackboard carrier is therefore a legitimate context source, not a proposed new shortcut around routing. [Follow-up route contract][follow-up]

The current implementation also provides useful protections:

- Successful review results retain parsed findings and metrics in `lastShardResult`, with a sliding history of ten **results**, not ten ordinary conversation turns. A normal assistant reply replaces that result only when its message carries a `ShardResult` payload. [Storage][storage], [Update handling][update]
- Initial articulation renders the result's type, task, turn, up to forty findings and metrics. It renders finding-map keys deterministically and discloses the number beyond forty. The witness below uses one finding, so this admission cap does not cause the loss. [First-call shard evidence][first-render]
- Knowledge gathering freezes the parent's recent messages, rendered workspace evidence and compressed context rather than consulting a later unrelated chat state. Failed, empty and capped consultations are explicitly described to synthesis, with query and purpose. [Frozen parent][parent], [consultation coverage][terminal]
- Controls settle before consultation; the compressor records sanitized controls without replay. Synthesis performs one follow-up completion rather than restarting perception, routing and the initial turn. The result carries the owning request, including terminal errors. [Control ordering][handoff], [owned synthesis][synthesis]
- A failed compressed-context build already produces a coverage notice while retaining the bounded raw suffix. This report concerns a successful projection that has no shard-evidence field, not that already-fixed failure notice. [History selection][selection]

These are controls to preserve, not evidence that every parent field reaches the terminal request.

## Production trace

### 1. A review can outlive its raw-history placement

`storeShardResult` extracts findings from the returned text, stores the raw result and sets `m.lastShardResult`. Its normal assistant-message consumer invokes it before adding the displayed reply. The multi-shard review route has an additional ordinary producer: Update formats a successful aggregate and calls `storeAggregatedReviewResult`, which stores its deduplicated findings and metrics in the same blackboard. [Ordinary producer][storage], [ordinary consumer][update], [aggregate consumer][aggregate-update], [aggregate storage][aggregate-storage]

The ordinary extractor retains a recognized finding's raw line, severity, file, message and optional line number. This provides a concrete small finding for the witness; it does not depend on populating a field that current review parsing never writes. In particular, this report does not rely on hypothetical nonempty aggregate `Category` or `Recommendation` values. [Finding extractor][extract]

On later full-articulation turns, active compression selects only the configured recent raw suffix. `getRecentTurns` slices the `Message` array: its unit is individual messages, not user/assistant pairs. But `ConversationContext` independently receives `m.lastShardResult` and `m.shardResultHistory`. Consequently, aging the original review reply out of the raw suffix does not itself remove the finding from the first answering request. Ordinary intervening articulation replies have no new shard payload to replace that blackboard entry. [Selection and independent fields][selection], [suffix units][suffix], [ordinary articulation result][ordinary-result]

### 2. The initial request receives the shard evidence

`articulateWithConversation` emits the “Last Shard Execution Result” section directly from `LastShardResult`; its findings and metrics are not merely kernel facts or unconsumed struct fields. A separate block summarizes earlier shard executions. This data is appended to the request body consumed by the completion call. [Shard blocks][first-render], [completion call][complete]

No special legacy prompt-assembler fallback is necessary for this path. It is the direct main-chat answering renderer. This distinction matters because other session-context fields in the repository have more conditional prompt consumers.

### 3. Consultation projects away that source

If the admitted articulation response contains knowledge requests and the consultation guard is clear, `processInput` passes `knowledgeParentOf(convCtx)` to `handleKnowledgeRequests`. The helper's struct and return literal contain only:

```
RecentTurns
Evidence
CompressedCtx
```

Neither `LastShardResult` nor `ShardHistory` is represented. The gathered message transports this reduced parent to Update; Update invokes synthesis with that parent and the consultation results. Synthesis renders precisely those three parent channels before the original input and each consultation's query, purpose and outcome. It does not reconstruct the omitted blackboard section. [Branch and handoff][handoff], [projection][parent], [Update-to-synthesis][gathered-update], [terminal rendering][terminal]

The initial interim response is displayed and added to UI history, but is not a synthesis argument or an added entry in the already-frozen `Parent.RecentTurns`. Therefore a finding mentioned only in that interim display is not a host-enforced repair either. A consultation that repeats the finding in its query, purpose or returned prose **can** repair this particular information loss; the witness expressly excludes that recovery. [Interim handling and synthesis arguments][gathered-update]

## Bounded source-derived witness

The relevant conditions are conjunctive:

1. A normal completed review leaves a small parsed finding in `lastShardResult`. No review parser error, large finding list, location deduplication collision or oversized field is required.
2. Compression is active; enough subsequent ordinary messages put the original review reply outside the configured raw suffix. No newer result or session reset replaces the retained blackboard entry.
3. The finding's answer-relevant detail is absent from the actual selected compressed context, other acquired evidence and remaining recent messages. This must be observed in a future fixture, not assumed from the word “compressed”: kernel observations or other memory may supply a recovering copy.
4. A follow-up reaches full articulation rather than ending as a perception answer or delegation. Its original sentence refers to the retained finding without spelling out the decisive detail.
5. Initial articulation requests domain advice. The consultation's query, purpose and successful response do not restate that detail; for example, they discuss general remediation priorities rather than the particular file and observed condition.

Under these conditions, the first request contains the retained finding and the terminal request does not. The source proves the omitted input channel. It does not prove how often models choose such consultations or what answer the second model produces.

A sharp evaluation witness uses two matched histories with different retained finding texts, A and B. Keep the later raw suffix, compressed context, current input and generic consultation data identical. The first answering prompts differ in their blackboard evidence; `knowledgeParentOf` produces the same fields and the terminal prompt builder produces the same body. A model cannot use the omitted distinction from that body alone. This is a source-level observational-equivalence argument, not a claim that real provider outputs or timing will be identical.

The potential consequence is loss of the basis for a finding-specific explanation or recommendation. The final answer may ask the user to repeat information, answer generically, or make an unsupported selection. **No inevitable wrong answer, false completion, write-authority escalation or destruction of stored evidence is established.**

## Recovery and scope limits

- Before the original reply leaves the raw suffix, copied `RecentTurns` may already contain everything needed. Without active compression, the initial history selection is the full history, another important recovering control.
- A selected compressed/kernel observation, explicit user restatement, workspace read, or specialist output can carry the same fact. Those are successful controls, not failed reproductions to conceal.
- The original report remains visible in chat history and normal persistence copies that history. Aggregate review persistence/export can provide another record. Retention outside the final request is not automatic delivery to that request, but it rules out global-loss claims. [Raw persistence][persistence], [aggregate persistence][aggregate-persistence]
- A direct answer without consultation retains the first-call evidence path; this report concerns the additional terminal synthesis boundary.
- Findings are reports from an earlier review, not guaranteed current workspace truth. Preserving them must preserve provenance and age rather than silently certify them as newly verified defects.
- The previous terminal-grounding audit already leaves a synthesis-specific JIT role, complete-request admission and broader receipt work open. This continuation should update that workstream, not create a parallel synthesis architecture. The recent compressed-control-outcome audit concerns ratio fallback inside history compression; this finding loses a separate first-call evidence field even when the compressed-history string is copied perfectly. [Prior status][prior], [compression continuation][compression-audit]

## Proposed bounded shared snapshot

1. **Define one shard-evidence projection for both answering phases.** Extract the existing last-result and shard-history rendering into a shared bounded projection. At minimum, carry the same admitted finding/metric data, result type, task and turn provenance, together with the existing finding-count coverage notice. A typed immutable view or a rendered immutable string can implement this; these are proposal choices, not existing APIs.
2. **Freeze the view owned by this answer.** Capture it from the first answering context, not from `m.lastShardResult` when synthesis later runs. If using typed maps/slices, copy their contents rather than sharing mutable backing objects. A rendered string is a simple initial option because it avoids mutable map aliases and matches exactly what the first call saw.
3. **Preserve, do not broaden, evidence.** Do not inject the full raw shard transcript merely because it is retained. Do not restore omitted forty-first findings without a separate admission/continuation decision. The minimum repair carries the already-admitted view; a broader selection policy can later make relevant findings recoverable with explicit coverage.
4. **Budget the complete terminal request.** Charge the additional view alongside recent history, workspace evidence, specialist replies, persona and output reserve. If required evidence cannot be admitted, keep an explicit omission or narrower-answer disposition. A handle is a valid alternative only if the terminal phase can actually resolve it; the current plain synthesis call must not promise a nonexistent tool loop.
5. **Keep controls and ownership unchanged.** This is historical data, not instructions to apply the old packet, rerun the review or authorize a repair. Preserve one-call synthesis, request attribution, failed-consultation disclosure, ordinary permission gates and the existing no-replay compressor path.

This small projection repair does not require the full deferred JIT/receipt redesign. Conversely, adding a struct field without rendering it, or rendering a newly read global result rather than the frozen one, would not meet the contract.

## Costs and unrun evaluation

The minimal projection needs no additional provider call, but its bytes are real cost. It increases terminal input tokens, may displace other useful context, and can affect latency and cached-prefix reuse. Snapshot construction/copying adds CPU and retained memory for the lifetime of consultations. Whole finding values can be large even when the count is small; preserve existing limits and evaluate explicit byte/token admission rather than appending an unlimited block. A new archive-recall design would additionally require identity, authorization, freshness, lookup and retention work. Do not count today's missing evidence as a token-efficiency win.

Suggested acceptance cases, **not run**:

| Case | Required evidence |
|---|---|
| Normal review result → Update → subsequent messages → full articulation → consultation → Update → synthesis | Capture both actual completion requests. A small retained finding appears in both, or the terminal request explicitly accounts for unavailable required evidence. Do not hand-inject the final `knowledgeParent`. |
| Matched A/B findings with otherwise identical terminal inputs | The repaired terminal evidence distinguishes A from B while preserving historical provenance; baseline projection is expected to collapse them. |
| Recent-history overlap, compressed-context recovery, specialist restatement | Acknowledge these baseline successes; measure redundant bytes and avoid claiming every consultation loses the finding. |
| No consultation; no last result; unrelated current question | Preserve existing behavior, no fabricated finding, no new routing interceptor and no extra review/provider call. |
| New shard result or session change while gathering | Terminal snapshot remains bound to the original answer; no replacement with the newest blackboard. |
| More than forty findings or oversized fields; tight final budget | Preserve explicit coverage and bounded admission; do not silently expand the first-call selection or lose required provenance. |
| Failed/empty/capped consultations; admitted memory controls | Existing outcome disclosure and request ownership remain; controls apply once, not during evidence reconstruction. |

Extend the existing composed grounding test: it already drives a real `processInput` read, consultation and synthesis request and proves workspace sentinels and PARTIAL coverage survive. It does not exercise `LastShardResult`. Keep that test and the answer-identity regressions; add a production-produced blackboard finding, normal history selection and the actual Update/synthesis handoff. Do not manufacture absence by clearing a compressed context that would naturally contain the finding. [Current composed test][grounding-test], [identity repair][identity]

For later capability trials, use matched requests, provider/model, workspace state and authority. Measure supported finding-specific answers, correct uncertainty, unnecessary user repeats and recovery work alongside total calls, input/output tokens, latency, snapshot bytes, serialization cost and displaced useful evidence. Count initial articulation and consultations as well as the terminal call. Better evidence continuity is source-supported as a design objective; improved answer accuracy or net cost is a measurement still to perform.

[first-render]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_articulation.go#L165-L230
[parent]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_knowledge.go#L38-L58
[terminal]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_knowledge.go#L274-L329
[prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/terminal-knowledge-synthesis-grounding.md#L149-L185
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-synthesis-answer-identity.md
[follow-up]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L161-L169
[storage]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L264-L296
[update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L144-L173
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L1070-L1109
[synthesis]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_knowledge.go#L228-L270
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L975-L999
[aggregate-update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L237-L259
[aggregate-storage]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L299-L360
[extract]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L76-L108
[suffix]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_follow_up.go#L15-L21
[ordinary-result]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L1112-L1122
[complete]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_articulation.go#L308-L380
[gathered-update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L980-L1028
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_persistence.go#L75-L90
[aggregate-persistence]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L603-L634
[compression-audit]: https://github.com/theRebelliousNerd/codenerd/blob/3ef34bc061b497b9cfc471f4cc46f5df595b299d/Docs/audits/compressed-control-outcome-fallback.md
[grounding-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/knowledge_grounding_test.go#L16-L61
