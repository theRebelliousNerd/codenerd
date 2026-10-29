---
title: Preserve request owned grounding sources through streaming articulation
date: 2026-10-29
repository: theRebelliousNerd/codenerd
target: main at 9dcb01b9736bf4763aa3ac391d29f133d73c6611
inspection_window: 2026-10-29 17:07–17:20 UTC
external_contract_review: 2026-10-29 17:15 UTC
verification: Source inspection only; no tests, benchmarks, or inference performed
status: Proposed repair and unrun evaluation
---

# Preserve request owned grounding sources through streaming articulation

Gemini streaming can receive provider grounding URLs without making them available to the host-generated Sources list. The stream reader decodes a response type containing grounding metadata but does not capture it. After accepting the streamed answer, articulation instead reads a client-global last-sources getter. This creates a primary omission defect and, under narrower conditions, stale attribution from an earlier request. The proposed repair is request-owned, bounded grounding metadata carried with the successful terminal result, not another lookup of shared client state. This finding concerns source attribution, not whether the model used grounding or answered correctly. [Stream reader][reader] [Articulation attribution][attribution]

## Scope and verification

The target is the pinned main commit above. The five inspected serving files—Gemini client, Gemini streaming adapter, articulation helper, chat process, and stream types—were byte-identical at active snapshots [a53cfc3][branch1], [80af186][branch2], [aa7988d][branch3], and [QA 51cfd42][qa]. This establishes parity only for those files, not entire branches. The latest inspected [research snapshot c89392f][research] adds reflection-observation-publication and why-query-scope-preservation work, unrelated to this handoff.

The existing [stream completion audit][prior-terminal] owns successful terminal semantics; [terminal knowledge synthesis grounding][prior-synthesis] owns knowledge context entering final synthesis; [research observation identity and admission][prior-research] owns browser acquisition. This audit preserves those boundaries and addresses metadata leaving the provider stream and reaching host attribution. A bounded search around `GetLastGroundingSources` found no prior exact finding; that is not an exhaustive novelty claim.

## The serving path loses an already decoded observation

This is a configured path, not a claim about every session. The factory applies Gemini search settings, and `buildBuiltInTools` enables search according to its flag, with optional URL context. Streaming requests include those built-in tools. Boot assigns the main model client, including the shared-session path that uses `cortex.LLMClient`. [Factory settings][factory] [Built in tools][builtin] [Streaming request][request] [Boot assignment][boot] [Shared session][shared]

Both Gemini streaming entry points use `runStreamingRequest`. Its reader unmarshals `geminiStreamChunk`, which embeds `GeminiResponse`; the latter includes `GroundingMetadata`. The reader forwards text and captures thought signatures, usage, and finish reasons, but neither collects grounding chunks nor clears the last-source field. By contrast, nonstream `captureGroundingSources` first clears that field and then extracts Web URIs from the first candidate. `CompleteWithSystem` and `Chat` invoke that capture. [Shared streaming entry][entries] [Stream reader][reader] [Response types][response-types] [Nonstream capture][capture] [Complete capture][complete-capture] [Chat capture][chat-capture]

The ordinary TUI articulation path passes a nonnil stream channel; the helper selects real streaming or thought streaming when supported. It requires a successful terminal and runs response admission before proceeding. It then obtains sources through `GetLastGroundingSources`, regardless of whether this answer streamed. The getter returns a defensive copy, but copying cannot establish which request produced its contents. The inspected passthrough, tracing, and scheduled-client wrappers forward that getter; on the stable, non-routed client path examined here, they do not reconstruct omitted metadata. [Process route][process-route] [Streaming selection][selection] [Terminal and envelope processing][gates] [Articulation attribution][attribution] [Getter][getter] [Passthrough][passthrough] [Tracing][tracing] [Scheduled client][scheduled]

This reaches user-facing output. The process appends a host Sources section to the response, submits that same response to the compressor when wired, and returns an assistant message that the update handler adds to history and schedules for saving. Losing ownership at articulation can therefore affect the displayed answer and retained representations; successful durable storage is a separate condition, not established by scheduling a save. [Source rendering][rendering] [Compression][compression] [Assistant message][assistant-message] [History update][history]

## Two precise unrun witnesses

**Primary omission.** Use one stable Gemini main client and credential, with grounding configured. Let normal TUI processing reach streaming articulation rather than an early perception answer, delegation, hold, or clarification. Begin with a nil getter—for example, the latest preceding nonstream capture returned no grounding. The provider stream supplies a valid, accepted answer envelope, at least one `GroundingChunks.Web.URI` across its events, and a successful `STOP`. There are no knowledge consultations, overlapping calls, or cancellations. The reader decodes but does not publish the metadata; the later getter remains empty, so the host Sources list omits those provider URLs. No timing race is required. This is a source-derived witness, not a reproduced execution. [Stream reader][reader] [Capture reset][capture] [Acceptance gates][gates] [Sources handoff][attribution] [Rendering][rendering]

**Conditional stale attribution.** The same client previously completed a nonstream request with sources A. One concrete producer is `synthesizeWithKnowledge`, which calls `m.client.CompleteWithSystem`; if that provider response is grounded, its capture can populate A. A later successful streaming request B supplies disjoint sources B, or no grounding, with no intervening same-client nonstream reset or client replacement. The getter remains A and the host can attach A to B. A may still be relevant or present in history; the defect is that host attribution does not reflect this request's provider metadata. Ordinary perception using that same client may reset A and eliminate this stale witness, but not the primary omission. Neither grounded synthesis nor separate-client classification is universal. [Knowledge synthesis][knowledge] [Complete capture][complete-capture] [Capture behavior][capture] [Stream reader][reader] [Sources handoff][attribution]

These witnesses do not establish absent inline citations, a wrong answer, authority bypass, or task-completion corruption. Existing terminal failures block completed output; whole-response capture already works, defensive copies prevent direct slice mutation, and a provider may legitimately return no citations. Model-authored inline references may preserve some user utility. The finding is Gemini-specific; potential analogous behavior in other providers requires a separate audit. [Acceptance gates][gates] [Nonstream capture][capture] [Getter][getter]

## Accumulate before publishing

The official API reference reviewed at 17:11 UTC and rechecked at 17:15 says streamed grounding chunks contain newly emitted entries rather than earlier entries repeated, and support indices refer across the accumulated response sequence. Consequently, replacing the source list on every event would discard earlier chunks. If retaining support indices, preserve ordered raw chunk positions and candidate scope separately from display deduplication. These are current external API semantics, not runtime observations or Google SDK behavior: this code uses its own HTTP streaming adapter. [Google grounding metadata][google-metadata] [Google grounding support][google-support] [Streaming adapter][request]

Extend the terminal `StreamResult`—currently stop reason, usage, and error—or an equivalent typed response object with an immutable, request-and-attempt-owned grounding observation. Represent no grounding distinctly from unavailable or partial metadata. Accumulate within the stream, retaining ordered chunks or URIs, candidate identity, explicit limits, and truncation reasons. Define handling for repeated final metadata rather than silently duplicating or replacing observations. A first repair can preserve the current URL-list feature; full claim-to-support spans can remain separate. [Current terminal result][stream-result]

Publish completed Sources only after successful terminal completion and envelope acceptance. Failed-stream evidence may remain diagnostic, never completed-answer attribution. Carry the observation through wrappers and credential-failover terminal attempts without consulting a global latest value. Preserve existing safety, control, and session gates. Label the resulting list as provider-supplied references, not claim-by-claim fact verification. No additional search or URL scraping is needed.

Clearing shared sources at stream start addresses part of the stale case but still loses current metadata. Per-event replacement is incompatible with incremental chunks. Disabling streaming or generating a second answer is not the preferred repair because it changes latency or request cost unnecessarily. A compatibility getter may remain for other callers, but it must not decide attribution for this answer. Keep the first implementation focused on serving-path parity rather than a broad provider rewrite.

## Costs and measurement

The current decoder already parses supported metadata. The added work is accumulation and projection over received metadata, with bounded retained memory, copying, and a single terminal payload—not necessarily another parse. The design intrinsically requires zero additional model or network requests. Sources retained in conversation history increase tokens by the actual retained list. If support indices are retained, limits and deduplication must preserve their raw mapping or explicitly report omissions; a URL-only repair need not store all support metadata. Measure allocations, first-token latency, completion latency, and throughput; no fixed budget or performance improvement is established here. API compatibility, wrapper propagation, and fixtures are real implementation costs. [History path][compression] [Terminal result][stream-result] [Google support indices][google-support]

## Evaluation before acceptance

Use a scripted provider HTTP stream through the actual boot and wrapper stack, normal process route, and `Update`. Control classification so the fixture reaches articulation; do not assume a model predicts the intended route. Compare the baseline, a clear-only patch, and request-owned accumulation with identical source payloads, chunk boundaries, model configuration, JIT context, and permissions. The expected distinction is omitted current sources on baseline, still-omitted sources with clear-only, and correctly owned sources with accumulation—not a claimed improvement in hallucination rate. [Boot][boot] [Process route][process-route] [Update][history]

Cover these fixture groups:

1. Empty state followed by grounded stream A; old nonstream A followed by disjoint stream B; old A followed by a stream with no grounding. Use distinct fixture URLs and assert provider metadata, host Sources, and stored history separately. The oracle is request provenance, not topic relevance or model truth.
2. Metadata spread across events, a metadata-only final event, absent metadata, repeated URLs, display deduplication, and explicit bounds. Where support indices are retained, preserve positions even when display entries merge. Unknown source types must remain unsupported rather than fabricated.
3. Both streaming variants, a nil-channel/nonstream control that already captures sources, and wrapper propagation. Separate candidate scope and repeated-metadata policies should have explicit expected outcomes.
4. Malformed JSON or SSE, EOF, truncation, filtering, and cancellation. Assert no accepted completed Sources or controls from failed output; do not require partial diagnostic evidence to disappear.
5. Follow-on ownership checks for overlap, failover, and late session receipts. These are regression defenses, not necessary assumptions of the primary witness.

The inspected Gemini uplift streaming tests exercise retry/backoff and thought separation; conformance and TUI terminal tests cover other stream behavior. The inspected nonstream uplift test checks copy-on-read. No grounding-metadata fixture was found in the inspected streaming tests; this is not a repository-wide coverage assertion. No tests, benchmarks, or model inference were performed for this audit. Acceptance requires serving-path evidence that successful answers retain only their own provider references while existing terminal and envelope gates remain intact. [Streaming uplift tests][stream-tests] [Conformance tests][conformance] [TUI terminal tests][terminal-tests] [Copy test][copy-test]

[branch1]: https://github.com/theRebelliousNerd/codenerd/tree/a53cfc308e8836c72370adaa46deba3f7b00180d
[branch2]: https://github.com/theRebelliousNerd/codenerd/tree/80af1863d3a3aa46a722ac8b0c8a0985582d4995
[branch3]: https://github.com/theRebelliousNerd/codenerd/tree/aa7988dec4c62116656ea722ff73b687a8509908
[qa]: https://github.com/theRebelliousNerd/codenerd/tree/51cfd42649552866d4aa54c53b68acf5740eb09b
[research]: https://github.com/theRebelliousNerd/codenerd/tree/c89392f38fe86f0cad4c82caab9d7d832b82ea43
[prior-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/stream-completion-contract.md
[prior-synthesis]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/terminal-knowledge-synthesis-grounding.md
[prior-research]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/research-observation-identity-and-admission.md
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_factory.go#L552-L577
[builtin]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini.go#L270-L292
[request]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini_streaming.go#L84-L138
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L750-L756
[shared]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session_shared_boot.go#L105-L109
[entries]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini_streaming.go#L21-L40
[reader]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini_streaming.go#L197-L303
[response-types]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_types.go#L740-L787
[capture]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini.go#L440-L452
[complete-capture]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini.go#L635-L636
[chat-capture]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini.go#L894
[process-route]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L955-L970
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/helpers_articulation.go#L303-L320
[gates]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/helpers_articulation.go#L357-L401
[attribution]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/helpers_articulation.go#L427-L428
[getter]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini.go#L342-L345
[passthrough]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/broker/passthrough.go#L86-L92
[tracing]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/tracing_client.go#L493-L500
[scheduled]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/scheduled_llm_client.go#L474-L481
[rendering]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L987-L993
[compression]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L1034-L1045
[assistant-message]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L1064-L1069
[history]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L138-L162
[knowledge]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_knowledge.go#L216-L254
[google-metadata]: https://ai.google.dev/api/generate-content#GroundingMetadata
[google-support]: https://ai.google.dev/api/generate-content#GroundingSupport
[stream-result]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/types/stream.go#L136-L150
[stream-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini_streaming_uplift_test.go#L35-L79
[conformance]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/stream_conformance_test.go#L274
[terminal-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/stream_terminal_test.go#L49-L83
[copy-test]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_gemini_uplift_test.go#L97-L108
