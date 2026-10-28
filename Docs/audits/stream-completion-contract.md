---
doc-class: deep-dive
subsystem: perception-broker-chat
implementation-status: partial
last-verified: 2026-10-28
verified-against: 66ccd529ba906c01a518a99efd143708b56686c5
verification-method: static-source-inspection
supersedes: []
---

# A closed stream is not a completed answer

**Decision:** make provider completion evidence a required part of the streaming contract. Preserve fast incremental display, but distinguish provisional content from a terminally accepted response. A clean transport EOF, a parseable prefix, and a provider's successful generation are three different observations.

**Evidence status:** source inspection and design only. No tests, inference, failure injection, benchmarks, implementation, or live provider requests were executed. Conditional examples are proposed witnesses, not reproduced incidents. Source statements are pinned to the revision above; they do not assert that later main is unchanged.

## 1. Why this is the next useful seam

CodeNERD's root guidance separates the model's creativity from the harness's executive truth and demands typed operations and evidence-grounded behavior (`agents.md`, **North Star** and **How This Shapes Runtime Changes**, [source](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/agents.md#north-star)). That split is weakened if a provider adapter reports successful completion merely because it stopped receiving bytes.

The repository already identifies streaming as unfinished work in `Docs/architecture/perception/TODO.md`, **P0: Normalize provider failure semantics**, including the September 25 progress note ([source](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/Docs/architecture/perception/TODO.md#p0-normalize-provider-failure-semantics)). This document is a concrete refinement of `perception-provider-failure-contract-v1`, not a competing roadmap or a claim of novel discovery.

It also completes a specific edge of the existing no-silent-truncation ambition (`Docs/journeys/impl/S15-never-silently-truncate.md`, **Status**, lines 10–19, [source](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/Docs/journeys/impl/S15-never-silently-truncate.md#L10-L19)). The existing [detailed-usage audit](detailed-usage-wiring-and-verified-efficiency.md), §§1–2, addresses accounting metadata and settlement. This document addresses whether generated content is complete and usable, independently of whether its cost is known. Receipt settlement remains that document's responsibility.

## 2. Current behavior: the matrix is asymmetric

| Path | Inspected source and symbol | Current conclusion |
|---|---|---|
| OpenAI streaming | `CompleteWithStreaming`, [`internal/perception/client_openai.go:359–422`](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/perception/client_openai.go#L359-L422) | Malformed JSON events are skipped. `[DONE]` returns, but an ordinary scanner end with no scanner error also reaches the successful branch. `finish_reason` is not checked in this stream path. |
| OpenRouter streaming | `CompleteWithStreaming`, [`internal/perception/client_openrouter.go:358–421`](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/perception/client_openrouter.go#L358-L421) | The same relevant pattern: skip malformed event JSON, no required terminal evidence, no finish-reason validation before success. This is separately inspected, not inferred from its API shape. |
| Gemini streaming | `runStreamingRequest`, [`internal/perception/client_gemini_streaming.go:221–305,318–361`](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/perception/client_gemini_streaming.go#L221-L361) | Records `lastFinishReason`, but non-`STOP` reasons produce a warning in the successful scanner branch, not a terminal error. A missing finish reason can also reach that branch. Malformed event JSON is skipped. |
| OpenAI-compatible streaming | `consumeStream`, [`internal/perception/client_openai_compat.go:990–1065`](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/perception/client_openai_compat.go#L990-L1065) | Already rejects an empty content stream and emits `OutputTruncated` for length stops. Preserve these improvements. It still skips malformed JSON and does not require terminal evidence or reject every non-success finish reason. Ordinary EOF after nonempty content with no recorded length stop can be accepted. |
| Anthropic streaming | `readAnthropicStream`, [`internal/perception/anthropic_stream.go:87–118,165–229`](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/perception/anthropic_stream.go#L87-L229) | Already requires `message_stop` and a stop reason; rejects malformed JSON, unexpected EOF, length termination, refusal, unexpected terminal reasons and empty text. It also bounds event and response accumulation. This is a stronger local precedent, not a proof every protocol-order case is covered. |
| Typed length failure | `OutputTruncated` and `LengthStop`, [`internal/types/truncation.go:24–38,70–79`](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/types/truncation.go#L24-L79) | The repository already has a shared diagnostic type and provider spelling normalization. Do not invent a competing string convention. |

These are production-selectable clients, not only test doubles: `newRawClientFromConfig` selects the respective providers (`internal/perception/client_factory.go:596–655,681–693`), and `NewClientFromConfig` installs a broker or chooses a credential pool (`:898–906`, [source](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/perception/client_factory.go#L596-L906)). Provider selection determines which behavior is relevant. This review does not claim that every provider uses these parsers, that Steve's active model is affected, or that all coding execution is streamed.

### What wrappers already do correctly

`FallbackClient.CompleteWithStreaming` does not splice a secondary answer after primary content has been forwarded, and a clean empty primary stream currently returns without consulting the secondary (`internal/perception/client_fallback.go:163–197`, [source](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/perception/client_fallback.go#L163-L197)). The credential pool's `Stream` separately tracks both content and thought emission and only retries before emission (`internal/perception/key_pool.go:249–344`, [source](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/perception/key_pool.go#L249-L344)). Preserve these boundaries. A terminal-evidence fix must not turn a half-visible answer into a transparent concatenation of two models' outputs.

The broker's streaming entry point admits and proxies the stream, or routes through the credential pool; it does not itself parse provider terminal events (`internal/broker/broker.go:344–369`, `CompleteWithStreaming`, [source](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/internal/broker/broker.go#L344-L369)). Consequently, it cannot reconstruct a finish reason the adapter discarded. Do not assume the non-streaming restatement mechanism automatically repairs an unreported streaming failure.

## 3. Failure mechanisms and their limits

### S1. Correct HTTP framing can still contain an incomplete model response

A server or proxy can end an HTTP body cleanly after delivering some complete SSE frames but before the provider's successful terminal indication. Go's scanner need not report a read error for that condition. The inspected OpenAI/OpenRouter paths therefore permit a no-error end without establishing model completion. This is not a claim that a broken TCP connection always looks clean: network truncation can yield a transport error, which these paths already propagate.

**Proposed witness:** return a valid partial content event and cleanly terminate the HTTP body, omitting finish evidence. Compare with the same prefix followed by the correct provider terminal sequence. The first must be incomplete and the second eligible for downstream validation. Add an immediate empty EOF control; do not silently treat that as a useful response unless the operation contract explicitly permits empty output.

### S2. Provider truncation can be downgraded to a log message or ignored

A length-limited completion is not an intact answer even if its visible prefix ends on a sentence or closes valid JSON. OpenAI/OpenRouter streaming do not inspect the field; Gemini records a non-STOP warning but leaves the returned error channel free of a corresponding terminal failure in that branch. OpenAICompat and Anthropic show that the shared typed failure can already be used at this boundary.

**Proposed witness:** supply a syntactically valid answer and then a length finish. Require typed truncation regardless of whether the answer happens to parse. Refusal, content filtering, safety termination and unsupported continuation must remain distinct from both success and truncation; they should not be turned into a generic retry that attempts to evade provider safety behavior.

### S3. Skipping malformed data can produce a plausible but altered answer

The skip branches remove evidence without telling the consumer which content was lost. A malformed event between two valid deltas could leave a fluent yet incomplete result. More subtly, an SSE event is delimited by a blank line and can contain multiple `data:` lines; treating each line as independent JSON is not the complete SSE framing rule. This is a compatibility risk to test, not a claim that current vendors routinely split JSON over multiple data lines.

**Proposed witness:** compare one event encoded as a single data line with the equivalent valid multiline SSE frame. The decoded event must match. A genuinely malformed JSON event must produce a protocol failure rather than silently disappearing. Unknown well-formed event types can be ignored only under a declared forward-compatibility rule; malformed known content cannot.

### S4. Downstream syntax checks cannot supply missing transport semantics

The actual chat articulation consumer forwards surface fragments while collecting the full response, waits for its content/thought/error channels to close, and returns an error when it receives a stream error (`articulateWithConversation`, `cmd/nerd/chat/helpers_articulation.go:286–361`, [source](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/cmd/nerd/chat/helpers_articulation.go#L286-L361)). This existing error path is useful. On no error, it passes the accumulated bytes to the response processor and builds control-bearing output (`:364–399`, [source](https://github.com/theRebelliousNerd/codenerd/blob/66ccd529ba906c01a518a99efd143708b56686c5/cmd/nerd/chat/helpers_articulation.go#L364-L399)).

Thus a malformed final envelope may still be rejected downstream, but a parseable prefix is not proof the provider finished. The narrow finding is missing completion evidence at this boundary. It is **not** proof of unauthorized execution, false `/done` in every task, or bypass of independent kernel policy and verification gates.

## 4. Proposed contract: provisional chunks plus one terminal result

The following names are proposed design notation, not existing declarations.

```text
StreamAttempt:
  logical_call_id, attempt_id
  provider, model, endpoint_identity, protocol_profile_version
  operation_kind, requested_candidate_set
  visible_content_emitted, auxiliary_content_emitted

StreamTerminal:
  attempt_id
  outcome: complete | truncated | refused | filtered | canceled |
           transport_failed | protocol_failed | incomplete | unsupported
  provider_finish_reason, terminal_evidence_kind
  content_digest, content_bytes, accepted_candidate_set
  usage_completeness, diagnostic_code
```

Completion of generation, validity of the returned schema, authorization of any proposed effects, and verification of the coding task remain separate gates. `complete` means only that the declared provider protocol completed this operation acceptably. It is not proof that the answer is true or the task is done.

### 4.1 Provider-specific terminal state machine

1. Parse SSE framing with bounded line, event and total-response resources. Assemble multiple data lines at the event boundary; honor comments and the selected protocol's event vocabulary.
2. Decode each relevant event once. Validate candidate/message identity and legal transitions for the requested operation. Unexpected identity changes or unsupported terminal shapes cannot silently become success.
3. Emit allowed surface chunks provisionally. Never mix hidden reasoning or provider-private signature material into that channel. Preserve existing provider-specific separation.
4. Record each required candidate's finish state. For text-only operations, a tool-call finish is not an ordinary text success; it requires an explicitly supported operation contract. Reject incomplete tool arguments rather than execute them as a prefix.
5. Validate the provider's required terminal evidence. Anthropic's message stop is not interchangeable with an OpenAI sentinel; Gemini must follow its own candidate finish contract. A compatibility endpoint's omissions must be an explicit, tested profile, not a global “EOF means done” escape hatch.
6. Publish exactly one terminal result. EOF before sufficient evidence becomes incomplete; malformed events become protocol failure; known length stops reuse `OutputTruncated`. A usage-only final chunk must not overwrite candidate completion or create a false empty answer.
7. Join owned parser work and close transport resources. Cancellation must interrupt blocked reads and blocked forwarding, and must not be converted into success by a simultaneous channel close.

Do not universally require both a content finish and an arbitrary sentinel when a provider's protocol specifies otherwise. Conversely, do not waive validation for all compatible providers because one endpoint omits a field. Version the negotiated profile and carry the reason for any weaker guarantee.

### 4.2 Incremental migration without inventing a replacement stack

The immediate correctness fix can preserve the existing channel API: adapters track terminal evidence and send a typed error for every non-complete result before closure. This closes the demonstrated no-error holes with less integration work than a new public stream interface.

A richer internal event/terminal representation becomes worthwhile where consumers need identity, replay-safe attempt transitions and distinct terminal classes. Bridge legacy channels at one owned boundary. A consumer must observe terminal completion, not just content-channel closure; separate channel scheduling cannot guarantee which closure it sees first. If an error arrives after some visible text, the UI should mark that attempt incomplete and avoid committing its content as a complete durable answer. Diagnostic retention should be bounded and redacted.

Go validates protocol observations. Mangle derives whether an attempt can advance the task, whether a permitted retry has a useful purpose, and which verification obligations remain. New model-facing recovery guidance belongs in JIT prompt atoms, not one-off provider prose. No model assertion that “the answer is complete” substitutes for the terminal observation.

### 4.3 Retry and failover rules

- Before any output is externally exposed, a retryable transport/provider failure can permit another attempt under the existing deadline, budget, capability and authorized-provider constraints. Record a new attempt identity; do not overwrite the failed one.
- After output exposure, never append a secondary response as though it were continuation of the first. End and mark the old attempt; any re-generation is a distinct attempt with an explicit UI replacement/restart contract.
- Truncation repair may restate a response within a feasible budget, but must not claim to reconstruct missing content. A new answer still needs terminal and schema validation. Bound repair by evidence of progress and resource constraints, not an unbounded hidden retry loop.
- A refused/filtered response is not an availability failure to route around. Authentication/configuration and invalid-request classes require their appropriate resolution, not blind repeated generation.
- Account for paid failed attempts even when no text escaped. “Safe to retry” is a correctness property, not a claim the first provider did no work or charged nothing.

## 5. Full costs and fair alternatives

Let B be received bytes, E decoded events, Q the maximum queued bytes, and A attempts. A framed state machine is O(B + E) parsing work with bounded event storage; semantic validation adds its own cost. If a full answer must be retained for the existing response processor, that remains O(B) memory or bounded spool storage. Do not advertise constant-memory streaming while a downstream consumer still accumulates the whole envelope.

The main benefit sought is fewer silently incomplete answers and less downstream rework. It may cost more tokens when a formerly accepted prefix now triggers a legitimate regeneration. Measure that tradeoff rather than declaring every retry wasteful.

Record provider input and output token totals for all attempts, with cached-input and reasoning-output subcounts explicitly marked non-additive where the provider includes them in those totals. Normalize to disjoint uncached-input, cache-read, cache-write, visible-output and reasoning-output buckets only when the provider's documented semantics support that partition; otherwise retain totals and overlapping subcounts without summing them. Also record network bytes; parser CPU and allocations; buffered and retained bytes; queue/lease duration; time to first visible chunk; time to accepted terminal result; downstream validation and repair calls; human recovery; and final verified-task rate. Unknown usage stays unknown, with cost lower bounds, using the existing accounting proposal rather than a duplicate logger.

Compare at identical models, task snapshots, authority and acceptance requirements:

1. **Current implementation:** include its current downstream checks, not a straw-man parser.
2. **Adapter-only terminal validation:** retain legacy channels and existing UI; smallest correctness change.
3. **Typed terminal result + provisional UI/history state:** add explicit attempt lifecycle where needed.
4. **Buffered-before-display reference:** simple correctness baseline with worse first-token latency and greater retained-content pressure. It need not be the final product.

Keep successful intact streams byte-equivalent. Compare each provider to its own baseline before drawing whole-harness conclusions. Do not confound a cheaper model, different output budget, broader permission or weaker tests with a harness gain. Count correct actionable failure separately from verified completion so refusing all streams cannot win the metric.

## 6. Proposed conformance and capability evaluation

No tests below were executed. Begin with deterministic protocol fixtures, then exercise factory → broker/pool/scheduler → actual articulation consumer. Unit-level EOF checks alone do not prove wiring.

| Fixture | Required result |
|---|---|
| Valid deltas followed by clean EOF, no terminal evidence | Incomplete result, not success; partial display/history labelled accordingly. |
| Valid answer followed by length finish | Typed truncation even when JSON/sentences look complete. |
| Complete provider terminal sequence | Successful terminal result, unchanged content and no unnecessary retry. |
| Malformed known-content event between valid deltas | Protocol failure, no silent content deletion. |
| Valid multiline data frame, comments and heartbeat | Same decoded event as its single-line equivalent; no false failure for valid framing. |
| Refusal/filter/safety stop; unsupported tool-call finish in text operation | Distinct terminal classification, no availability failover that evades refusal. |
| Usage-only tail and missing usage tail | Content completion and accounting completeness remain separate. |
| Empty successful response where disallowed; where explicitly allowed | Correct operation-specific behavior, not a global blanket rule. |
| Cancel while read/forward is blocked | Owned work terminates, resources released, canceled outcome preserved. |
| Error before versus after first visible chunk | Permitted pre-output retry only; no mixed-attempt answer after exposure. |
| Content closes before delayed terminal error | Consumer does not finalize prematurely. |
| Oversize event/response and slow consumer | Bounded resources and explicit resource failure, no silent cut. |
| Unknown well-formed event or new terminal reason | Declared compatibility behavior; unsupported reason cannot become success by default. |

For coding capability, use matched tasks requiring long explanations, multi-file plans, structured envelopes and repair reports. Inject failures at several semantic boundaries, including a parseable but incomplete prefix. Independently score whether the harness reports the actual failure, preserves task state, avoids control-bearing prefix acceptance, and eventually reaches verified behavior under the same resource allowance. Include intact-stream controls so a more defensive parser does not merely improve fault cases by degrading normal usability. Report paired differences and uncertainty, including tail latency and human recovery, not only average token counts.

## 7. Primary foundations and bounded implications

- [WHATWG HTML, Server-sent events, parsing and interpretation](https://html.spec.whatwg.org/multipage/server-sent-events.html#parsing-an-event-stream): SSE has event framing and multiline data semantics. This supports a framing parser; it does not define when an LLM answer is semantically complete. Do not copy browser auto-reconnection behavior into paid generation blindly.
- [OpenAI, Chat Completions streaming events](https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events): finish reasons distinguish ordinary stopping from length limits and other outcomes; interrupted streams may lack final usage. This supports separate generation and accounting states, not an assertion that every OpenAI-compatible endpoint conforms.
- [Google, GenerateContent response and FinishReason](https://ai.google.dev/api/generate-content): candidate finish reasons distinguish normal stopping, token limits and other termination. A warning log is not equivalent to communicating that distinction to the caller.
- [Anthropic, Streaming messages](https://platform.claude.com/docs/en/build-with-claude/streaming): message lifecycle and stream error events support explicit completion validation. Existing CodeNERD Anthropic logic already follows important parts of that pattern.

These are protocol foundations adapted to source-observed seams. No novelty, measured accuracy increase, speedup or comparative superiority is claimed.

## 8. Buildable refinement queue

These local research IDs refine the existing provider-failure contract; they do not replace owning specifications or claim adoption.

| ID | Deliverable | Exit criterion |
|---|---|---|
| STREAM-TERM-01 | Provider-specific terminal conformance | EOF-without-completion, length, refusal and valid-finish fixtures have distinct asserted outcomes for each supported stream adapter. |
| STREAM-TERM-02 | Framing and malformed-event discipline | Equivalent valid SSE encodings decode identically; malformed known events cannot be silently omitted. |
| STREAM-TERM-03 | Terminal result through real consumers | Registered articulation path rejects a syntactically valid prefix with failed terminal status; intact responses remain unchanged. |
| STREAM-TERM-04 | Attempt-aware UI and failover | Visible output is never silently merged across attempts; late errors retain incomplete state; cancellation resources are bounded. |
| STREAM-TERM-05 | Verified total-system value | Paired normal/faulted task study reports completion, false acceptance, full cost and recovery without weaker authority or acceptance checks. |

The narrow first step is to make every adapter tell the truth about how generation ended. Only then should higher-level retry, history and task policy decide what happens next.

## As built (2026-10-27)

**Implemented**

- Contract (STREAM-TERM-01/03): `types.LLMClient.CompleteWithStreaming` and `types.ThoughtStreamer.CompleteWithStreamingAndThoughts` now return provisional chunk channels plus `<-chan types.StreamResult` carrying exactly one terminal `{StopReason, Usage, Err}` (`internal/types/stream.go`). `StreamOutcomeOf` classifies complete / truncated (`*OutputTruncated`) / refused / filtered / canceled / unsupported / protocol_failed / incomplete / failed via `*types.StreamError`. A terminal channel closed without a value is `ErrStreamIncomplete`. Producers publish through `StreamWriter` (Finish exactly once, after the last chunk; `Close` without Finish = incomplete); wrappers use `RelayStream`/`CaptureStream`; whole-text engines use `StreamWhole`.
- Framing (STREAM-TERM-02): `internal/perception/stream_sse.go:sseReader` implements WHATWG framing (multi-line data, comments, `event:`, bounded 1 MiB events). Malformed known events are protocol failures, never skipped.
- Provider terminal state machines: `stream_chat.go:readChatStream` (OpenAI, OpenRouter, Z.AI, Meta/DashScope/Moonshot via OpenAICompat): finish_reason is the evidence, `[DONE]` alone is not; `anthropic_stream.go:readAnthropicStream` (message_stop after stop_reason; refusal → refused); `client_gemini_streaming.go:readGeminiStream` (finishReason; non-STOP no longer a log warning; promptFeedback block → filtered). Shared classification in `stream_terminal.go` (empty stream rejected unless `WithAllowEmptyCompletion`; usage partial when the usage block never arrived). xAI, xAI OAuth, Claude CLI, Codex CLI and Meta web-search stream their validated whole completion (`StreamWhole`); the Claude CLI's line adapter, which forwarded nothing from real stream-json output, is deleted with `CompleteStreaming`/`StreamChunk`.
- Consumers: `cmd/nerd/chat/helpers_articulation.go:articulateWithConversation` requires a complete terminal before the response processor sees the text; `model_update.go` clears an incomplete streamed prefix on error. Wrappers forward the terminal after their own work: broker (settles first), `core/scheduled_llm_client.go` (now also routes streams via `clientFor`, releases the slot before publishing), `perception/tracing_client.go`, `key_pool.go:Stream` (retry only before emission), `client_fallback.go` (no failover after output, and never on refused/filtered/truncated).
- Conformance (§6): `internal/perception/stream_conformance_test.go` runs one scenario table (complete, EOF-without-terminal, length, malformed, multiline, filter/refusal, tool finish, missing usage tail, heartbeat/comments, provider error, cancel-while-blocked, multiline ≡ single-line) against OpenAI, OpenRouter, Meta, DashScope, Z.AI, Anthropic and Gemini fake HTTP servers; consumer witnesses in `cmd/nerd/chat/stream_terminal_test.go`.

**Ruled differently**

- Changed the channel API now instead of the doc's two-step migration (§4.2): an error-only channel cannot express "closed without terminal evidence", which is the defect.
- No attempt/logical-call IDs or content digest on the terminal: no consumer needs them yet; retry identity is carried by broker `Receipt.Attempts`.
- Tool-call finish on the text surface is `unsupported`, not a separate tool contract; text streams never carry tool calls.

**Deferred**

- STREAM-TERM-04 UI replace/restart contract for regenerated attempts (no regeneration path streams today) and STREAM-TERM-05 paired fault-injection study (needs live models).
- Thought streaming for Anthropic (`thinking_delta` → thoughts) would enable thinking on chat turns and change cost; not done without a decision.
