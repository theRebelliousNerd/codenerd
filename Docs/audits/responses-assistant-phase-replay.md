# Responses assistant phase preservation during native tool replay

## Finding and scope

CodeNERD loses assistant message `phase` when a Responses reply becomes native tool history. The decoder reads the field, but the shared reply adapter converts message text into phase-free content blocks. The next request reconstructs those blocks as untagged assistant messages. This is a source-proven protocol-fidelity defect in the shared OpenAI and Meta Responses path at main commit `9d26d9c3af600a5d01f904b138974e02ec7838c4`. It does not establish that a model actually stopped early or that a particular request was rejected.

The recommended change is to preserve each retained assistant message item's original phase and content boundaries through normalization, working history, and provider serialization. Keep the existing display filtering, reasoning preservation, item order, and tool-call pairing. Do not manufacture phases for messages that did not have one.

This is newly exposed through OpenAI's Responses integration, not a newly introduced Meta regression. The preceding Meta implementation already decoded `Phase` and converted message contents to metadata-free text blocks. The shared extraction carries that omission forward. Ollama remains on Chat Completions, so this finding does not establish a defect in its new transport arrangement. [Earlier Meta decoder](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_meta_responses.go#L293-L296), [earlier conversion](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_meta_responses.go#L859-L865), [Ollama transport](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_ollama.go#L77-L104).

## Why the distinction matters

OpenAI's reasoning guidance recommends retaining the original phase when manually replaying assistant messages. It explains that missing phase information can make a preamble be treated as a final answer. The field is optional; this is not evidence that every phase-free message is invalid, that every model behaves identically, or that omission universally produces HTTP 400. Its conversation-state guidance also favors replaying the complete output rather than an incomplete projection. [OpenAI phase guidance](https://developers.openai.com/api/docs/guides/reasoning?api-mode=chat#phase-parameter), [conversation state](https://developers.openai.com/api/docs/guides/conversation-state).

Meta documents a more specific validation consequence: an intermediate assistant message immediately before a function call requires the commentary phase on manual replay; omitting it can produce the documented HTTP 400. Meta's returned final messages have no phase, while `final_answer` is input-only. That distinction rules out a blanket repair that marks every assistant message `final_answer`. The provider consequences are documented and conditional, not observed outcomes from this audit. [Meta message phase contract](https://dev.meta.ai/docs/protocols/responses#message-phase).

## Where information is lost

The wire representation is not the problem. `responses_wire.go` includes `Phase` on the decoded output item. The separate `responsesAnswerText` projection intentionally excludes commentary when assembling flat answer text. That is useful presentation behavior and should remain: an intermediate status message need not appear as the final user-facing answer. [Decoded phase](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_wire.go#L284-L294), [answer projection](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_reply.go#L26-L48).

The loss occurs in `responsesToolResponseFromReply`. For a message item, it walks the text contents and appends `types.TextBlock(c.Text)` for each one. This includes commentary, despite its exclusion from flat `Text`. `ContentBlock` has no phase representation, and `TextBlock` records only kind and text. The conversion also discards the relationship between multiple text contents and the single assistant message item that contained them. [Reply normalization](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_reply.go#L99-L150), [block representation](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/types/content_block.go#L42-L97).

`AssistantMessageFrom` prefers the response's ordered `Blocks` over its flat text. Consequently, commentary survives in native history as text, but without its original phase. This is not merely an unused parser field or a diagnostic logging discrepancy. The same representation retains reasoning and tool-use blocks, which explains why fixing only flat answer text would not fix replay. [Assistant message construction](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/types/content_block.go#L247-L260).

On the outbound side, `responsesInputFromHistory` reconstructs each `BlockText` using an assistant `output_text` message with no phase. It does preserve important neighboring structure, including ordered reasoning and function calls. Nevertheless, the original message's phase and grouping are unavailable by this point. An input serializer change alone cannot recover information already discarded upstream. [Assistant input construction](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_input.go#L21-L31), [history reconstruction](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_input.go#L97-L178).

## Reachable native continuation

For an admitted nonisolated task, the ordinary TUI delegation route reaches this executor. Normal task delegation invokes the observed task executor, which chooses a nonisolated clone and calls `ProcessWithIntent`; that route reaches `runToolLoop`. This establishes a live entry path without claiming that every TUI task uses native tool continuation. [TUI delegation](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/cmd/nerd/chat/process.go#L628-L640), [observed executor invocation](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/cmd/nerd/chat/delegation.go#L64-L73), [nonisolated execution](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/task_executor.go#L395-L441), [tool loop entry](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/executor.go#L1439-L1448).

Both providers deliberately use stateless requests. OpenAI sets `store:false`; its tool-results request builds input from history and decodes through the shared reply adapter. Meta also sets `store:false` and uses the same history builder and reply conversion. The omission therefore affects the actual manual-replay mechanism, not a dormant alternative to server-managed conversation state. [OpenAI request settings](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai.go#L193-L210), [OpenAI continuation](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai.go#L353-L379), [Meta request settings and continuation](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_meta_responses.go#L100-L185).

The native executor appends `AssistantMessageFrom` to history, then appends tool results and requests another completion through the working-context route. Later native responses are appended through the same constructor. Native function calls also trigger an early return from piggyback promotion, so that path does not rewrite this response into a phase-preserving form. [Initial native history](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/executor_tools.go#L180-L194), [results and subsequent rounds](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/executor_tools.go#L342-L398), [promotion boundary](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/piggyback_promotion.go#L33-L35).

Working-context preparation assembles history and updates its ledger. Assembly applies `WithToolResults` and trailing text; `WithToolResults` retains non-tool-result blocks unchanged. These operations cannot restore missing phase metadata. After the guard, `completeWithWorkingContext` invokes `CompleteWithToolResults`. Healthy, under-budget history is enough; compaction is unnecessary. [Preparation](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/working_context.go#L591-L616), [assembly](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/working_context.go#L659-L677), [block retention](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/types/content_block.go#L349-L374), [provider handoff](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/working_context.go#L1005-L1025).

Existing controls still matter. Continuation policy or cancellation can prevent another request; a follow-up error or nil response returns failure, and a no-tool response proceeds to terminal verification. Ordinary nonretryable 4xx responses surface as errors. Nothing here proves an authorization bypass or false overall completion. The witness assumes an admitted continuation, not disabled safeguards. [Continuation and error handling](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/executor_tools.go#L293-L402), [transport errors](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_transport.go#L43-L100).

## Bounded witness

The smallest useful witness is an ordinary admitted native tool round, not an adversarial payload:

1. The provider returns a successful completed Responses reply containing a nonempty assistant message with `phase: commentary`, such as “I will inspect the file,” followed by a valid offered `read_file` function call with a valid call identifier. An optional encrypted reasoning item may precede them.
2. The native call passes host admission and executes. No unsupported tool, malformed arguments, restart, failed response, or compaction is required.
3. The executor retains the response's ordered blocks and appends the matching tool result. Working history remains healthy and within budget.
4. The next provider request contains the same assistant text before the function call and matching result, but the reconstructed message has no phase.

Steps one through three specify the conditions for the source trace. Step four is the representation defect established by that trace. A model's later answer is not part of the proof. For Meta, compare the serialized intermediate message with the documented validation rule. For OpenAI, treat possible answer-quality or continuation effects as hypotheses requiring separately authorized evaluation.

## Required preservation contract

The durable contract should operate at the assistant message item, not at the whole executor message. One `types.Message` can contain multiple assistant output items together with reasoning and function-call blocks. A single phase on that outer message cannot express their original boundaries or different phases. Adding a phase to individual text blocks alone also leaves ambiguous which contents belonged to one original message.

Two designs deserve evaluation: provider-neutral typed assistant-item metadata, or a protocol-owned lossless sidecar with explicit version and provenance. This audit does not select a raw sidecar implementation. Either design needs one authoritative mapping between semantic blocks and retained provider items, so a later serializer does not duplicate content or resurrect stale text.

The acceptance contract should require:

- Preserve a retained item's known phase, original absence of phase, and ordered content membership. Establish an explicit policy for unknown phase values rather than silently changing them into a known phase.
- Emit phase only where the destination protocol supports it. Do not add it to user messages, invent it for legacy history or host-appended text, or assign Meta's untagged final output a phase it never carried.
- Preserve existing reasoning identifiers and signatures, function-call identifiers, result pairing, and relative order. Phase repair must not regress the already-supported structured replay.
- Keep final-answer display filtering separate from transport preservation. Hidden commentary can still be necessary protocol history.
- Define which layer owns metadata after rewriting, promotion, result replacement, persistence, and compaction. A retained item keeps valid metadata; a deliberately dropped or transformed item must not leave a contradictory replay sidecar.

The shared Responses layer is the natural place to centralize the wire contract, with an explicit representation agreement with shared types and session history. This fits the current transport ownership guidance while keeping Ollama's Chat Completions contract separate. [Perception ownership guidance](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/agents.md#L74-L79).

## Costs and alternatives

Preserving metadata adds work proportional to retained items, plus storage and request bytes. Phase values are small beside normal content, but latency and token costs remain unmeasured. A raw replay sidecar could duplicate substantial text. Account for that duplication in persistence, context budgeting, and deduplication.

Compatibility work includes legacy transcripts, one-time cache-prefix changes, and provider transitions that cannot blindly forward provider-specific metadata. Rewriting and compaction must not preserve stale protocol items. Versioning and provenance should make unsupported replay explicit without disabling otherwise valid history.

Switching to `store:true` or introducing `previous_response_id` is not a substitute. Those choices change retention, privacy, operational state, and potentially cost policy, requiring separate authorization and design review. Preserve current stateless behavior. No automatic model calls are required to establish the serialization invariant.

## Evaluation before a fix is accepted

First use deterministic, provider-free fixtures that compare retained assistant items with their reconstructed request equivalents. These should establish exact phase presence, value, item boundaries, order, and call-result pairing rather than merely finding the expected text somewhere in a request.

The unrun evaluation matrix should cover:

- Commentary followed by a native call; multiple assistant message items; and one message containing multiple text contents.
- `final_answer`, absent phase, and an unknown phase value under the chosen compatibility policy, including Meta's untagged final-message behavior.
- Reasoning plus multiple calls and results, with existing identifier and signature preservation checked alongside phase.
- Compaction that retains an item and compaction that intentionally drops it; rewriting, promotion, and tool-result replacement without stale metadata.
- Broker forwarding, persistence and legacy-history loading, and provider transitions with incompatible metadata policies.
- Streaming and nonstreaming display, ensuring commentary remains excluded from flat final-answer text while required replay metadata survives.

Only after those checks should authorized live evaluation test Meta's documented validation case and OpenAI's possible continuation-quality effects. Keep these separate: HTTP acceptance does not establish answer quality, and one plausible answer does not establish protocol fidelity. Record model, provider, request shape, and phase-support assumptions before drawing conclusions. No such calls or tests were run for this audit.

## Existing coverage and priority

Existing tests deserve credit. OpenAI tests cover reasoning replay without message phase, and a streaming commentary fixture checks display hiding. The block round-trip test checks ordered message and tool reconstruction with phase absent. These are useful safeguards, but none establishes the missing phase-preservation invariant. The as-built Responses document's reported verification likewise should not be reinterpreted as a successful test of phase-preserving native history. [Reasoning replay coverage](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai_test.go#L235-L286), [commentary fixture](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai_test.go#L353-L356), [block round trip](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/blocks_roundtrip_test.go#L315-L362), [documented verification](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/Docs/architecture/perception/14-OPENAI-RESPONSES.md#L71-L77).

This is a native-continuation correctness fix, distinct from host admission of unsettled response items, general reasoning loss, or universal commentary leakage. The adjacent settlement audit addresses that different admission boundary. Prior screening did not surface an exact phase-continuity finding; this is a bounded search result, not proof that no overlapping discussion exists. [Native response control settlement](https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/Docs/audits/native-response-control-settlement.md).

The source evidence supports addressing the shared representation before adding provider-specific workarounds. The success condition is straightforward: when a returned assistant message is retained for replay, its original phase semantics and item boundaries survive to the next compatible provider request.
