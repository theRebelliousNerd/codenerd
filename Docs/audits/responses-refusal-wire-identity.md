# Responses refusal identity and terminal outcome parity

## Conclusion and scope

**The Responses adapter can lose an explicit provider refusal before existing terminal-outcome policy sees it.** A refusal-only completed response becomes a generic empty-answer error in `CompleteWithSystem`, an empty `end_turn` result with no error at the tool-results adapter boundary, or `StreamUnsupported` in an ordinary streaming context. Conditional consumers can consequently admit secondary generation or a no-tool nudge. This does not establish final task success or permission bypass.

This addendum reviews public CodeNERD sources at [`9d26d9c3af600a5d01f904b138974e02ec7838c4`][pin], reconfirmed on 2026-10-29 at 20:16 UTC. Static source inspection only; no application execution, tests, model calls or benchmarks were run. The [stream completion contract audit][prior-audit] already owns typed refusal, non-failover of refused/filtered/truncated outcomes, and its N1–N3 follow-ups. This contribution is upstream Responses wire identity and regeneration eligibility, not a replacement refusal policy.

The [predecessor OpenAI client][predecessor] already inspected message content and length without preserving typed refusal: the omission is not wholly new. [Current guidance][guidance] points toward Responses-only/shared-helper handling; the [architecture note][architecture] promises incomplete errors and stream completion, not specifically typed nonstream filtering.

## Provider identity and current decoding

OpenAI documents a response with status `completed`, null error and incomplete details, but message content containing a `refusal` block and `refusal` field. Completed status therefore does not establish a non-refusal answer. Its streaming reference documents `response.refusal.delta` and `response.refusal.done`: explicit metadata, not answer-text heuristics. These mutable provider sources were consulted on 2026-10-29. [Structured-output guide][provider-json]; [streaming events][provider-sse].

Local [`responsesContent`][wire-content] has `Type`, `Text`, and `Annotations`, but no refusal field. [Ordinary JSON unmarshalling][transport] does not reject the documented error-null refusal. Despite the [envelope's terminal metadata][wire-reply], [answer extraction][answer] selects non-commentary `output_text`; the [tool parser][tool-parser] initializes `end_turn` and reads message blocks through `c.Text`. Neither recovers explicit refusal identity.

## Outcomes by call path

The following outcomes assume a refusal-only completed response, no function call, and otherwise valid transport and terminal data unless stated otherwise.

| Path | Current outcome | Relevant distinction |
| --- | --- | --- |
| `CompleteWithSystem` | Generic empty-answer error when empty completion is not allowed | `responsesTerminal` returns nil for completed status; trimming then finds no answer. [Source][system-call] |
| `CompleteWithToolResults` | Empty text, `end_turn`, nil error at the adapter boundary | The same terminal check passes, then the tool parser returns its result. Downstream task acceptance is not established. [Source][tool-call] |
| Responses SSE, ordinary context | `StreamUnsupported` | Refusal events/content are not recognized; completed status reaches empty-stream handling. [Reader][stream-reader], [classifier][stream-classifier], [terminal][stream-terminal] |
| Empty-completion opt-in | Empty success can be permitted | `WithAllowEmptyCompletion` changes empty-result handling; no production stream caller enabling it was established. [Sources][system-call], [terminal][stream-terminal] |
| Nonstream `incomplete/content_filter` | Generic error, not typed `StreamFiltered` | Only the length case becomes typed truncation. [Source][reply-terminal] |
| SSE `incomplete/content_filter` | Typed `StreamFiltered` | This separate classification already exists. [Source][stream-classifier] |

The SSE switch omits refusal delta/done events and terminal refusal content. Completed output maps to ordinary completion unless function calls change it: refusal-to-unsupported here is not a missing completion marker. [Reader and finish mapping][stream-reader].

## Conditional classification witness

The [system factory][classification-factory] supports a configured OpenAI classification worker with a valid key/model, constructs the primary/main-secondary pair wrapped in `ScheduledLLMCall`, and builds the understanding transducer from the resolved client. The [client factory][classification-client] selects OpenAI and [broker/pool wrappers][client-wrappers]; the [transducer method plan][transducer-call] reaches `CompleteWithSystem`.

If that primary returns the documented completed refusal and context/budget admit a secondary attempt, its generic empty-answer error qualifies for [fallback][fallback]. The existing terminal exclusions recognize typed `StreamRefused`, `StreamFiltered`, and `StreamTruncated`, not this generic error. Optional schema support is deliberately not forwarded, so the witness follows ordinary completion rather than assumed schema dispatch.

This is conditional secondary eligibility, not proof that a secondary answers. The HTTP/schema retry loop sees no execution error for a valid completed refusal; usage is already reported before the empty-answer check. [Execution handling][execution]. Existing classification admission and [`classificationStop`][classification-stop] remain, including its held-work precedence qualification. A secondary result must still satisfy those gates; known failed classification is not assumed to continue unrestricted. [Typed call-failure handling][classification-failure]. The [classification outcomes audit][classification-audit] retains ownership of broader typed-outcome/recovery work.

Wrappers do not repair identity: broker [compression][compression] settles/returns provider errors and retries compression only for typed truncation; [routing][router] forwards. The [key pool][key-pool] returns this status-zero generic error without rotating keys and preserves typed causes. The [scheduled call][scheduled] propagates errors. No extra key-pool rotation is claimed.

## Conditional native tool consequence

The native OpenAI route in [`generateResponse`][native-generation] returns its tool-capable result; the active [working-context bridge][working-context] also returns `CompleteWithToolResults` without a post-reply refusal guard. In [`executor_tools.go`][no-tool-branch], a nil error with no tool calls can enter `retryWithNoToolNudge` when the current Mangle intent requires a tool and the tool catalog is nonempty; otherwise that branch can return the empty response without error. The [nudge][nudge] additionally requires JIT and compilation context, successful recompilation with a nonempty prompt, and applicable admission before another generation. Erasing refusal can therefore make it eligible for an ordinary no-tool nudge, not guarantee another model call.

The [request-visible tool audit][tool-audit] owns the broader tool contract; the [retry callability audit][retry-audit] owns grant/catalog continuity, not refusal eligibility. The existing [hollow-success verdict][hollow-success] and [effective JIT allowlist check][jit-allowlist] remain. Neither final task success nor unauthorized tool execution is established.

## Safeguards and limits

The [TUI articulation path][tui] collects/waits for the stream and returns terminal errors before its processor. An ordinary refusal-only Responses stream is therefore not accepted as a piggyback control prefix. Its demonstrated cost is lost refusal identity and misleading capability diagnosis, not unsafe control processing.

The [streaming fallback][fallback] protects emission and cancellation; no production streaming-fallback route was established, so TUI secondary regeneration is not claimed. The [Responses-reader caller][stream-caller] is OpenAI; [Meta's grounded-stream route][meta-ground-stream] uses `StreamWhole` with `completeWithSystemGrounded`, not this reader. Shared helpers do not establish provider-wire parity.

No incident frequency or measured savings/quality effect is established. Bad input/configuration, HTTP errors, and schema rejection are distinct from explicit provider safety refusal.

## Proposed contract and remedy choices

Preserve the existing audit's terminal policy by fixing classification at the boundary:

1. **Decode explicit metadata.** Preserve JSON refusal content and SSE refusal observations, including terminal-only content, as existing `StreamRefused`, never answer-text heuristics.
2. **Unify outcomes.** System/tool-result JSON and SSE should distinguish refusal, filtering, truncation, normal completion, and unsupported tool behavior. Give nonstream incomplete filtering typed parity.
3. **Separate diagnostics.** Keep bounded, redactable refusal diagnostics outside answer/control payloads. Rendering refusal as a normal answer hides the distinction. Allow-empty must not erase known refusal.
4. **Preserve accounting and termination.** Do not throw at the first refusal delta and lose final usage. Deduplicate delta/done/terminal observations without inventing required event ordering. Preserve cancellation and malformed/protocol checks. Specify precedence for observed refusal followed by a broken stream so it cannot silently become availability retry, without weakening cancellation.

A narrow OpenAI fix limits provider impact but risks divergent JSON/SSE logic. A shared decoder reduces duplication but expands compatibility review. Separate provider-wire extraction from semantic classification, and verify Meta's wire contract before extending OpenAI assumptions.

## Costs and failure modes

Recognition is linear in events/content already processed and requires no additional model calls. A boolean marker is cheap/bounded but loses diagnostics. Retained text needs a cap, duplicate reconciliation, and truncation indicators to avoid unbounded/repeated content. Sensitive context in refusal diagnostics requires deliberate logging, UI exposure, and redaction choices.

Changing identity affects callers treating empty tool results as success or unsupported streams as capability failures. Review API/UI compatibility, not just error strings. Preserve billed usage on refusal/terminal failure. Correct typing can prevent the conditional secondary call, not justify suppressing genuine availability fallback.

Failure modes include prose-based false positives, allow-empty erasure, duplicate diagnostics, lost usage, relaxed terminal checks, and incorrect cross-provider assumptions. Maintenance requires a provider/call-mode fixture matrix, not live harmful-prompt testing.

Any later cost evaluation should use a paired matrix matching configured provider/model, context, caps, and permissions. Report attempted generations/retries, input/output/reasoning/cache tokens, scheduler occupancy, time to terminal, and end-to-end latency, including failed attempts. These are proposed measurements, not results or quality claims; no live harmful prompt is needed to validate refusal identity and retry eligibility.

## Proposed checks and acceptance

**These checks are proposed, not run.** The [read conformance fixture][conformance] includes incomplete filtering, not completed refusal content or refusal delta/done. The [API rejection test][effort-test] is pre-wire effort configuration. This does not claim all relevant tests are absent or that the suite passes.

- Add deterministic completed-refusal fixtures for system JSON, tool-result JSON, and SSE. Exercise delta/done, terminal-only content, and repeated full final content; assert one refusal identity and retained final usage.
- Cover ordinary completion, incomplete filtering and truncation, tool calls, cancellation, EOF, and malformed streams. Include partial refusal followed by a transport/protocol failure and record the chosen precedence.
- Repeat explicit-refusal cases with allow-empty enabled. Empty completion may remain allowed, but known refusal must not become success.
- Pass refusal through broker, pool, scheduler, and fallback wrappers. Assert preserved typed identity and zero secondary calls; retain a genuine availability-outage baseline that makes one secondary attempt.
- Exercise the actual classification pair, native-tool branch, and TUI consumer with fakes. Typed refusal must trigger neither secondary generation, no-tool nudge, nor TUI processing; an ordinary non-refusal planning reply may still qualify for the nudge. Do not infer final task semantics from adapter results.

Acceptance is explicit refusal identity across supported Responses call modes, typed filter parity, preserved accounting and terminal/cancellation safeguards, and unchanged availability fallback for its legitimate cases. Keep the existing stream completion audit as owner of the broader refusal/failover contract.

[pin]: https://github.com/theRebelliousNerd/codenerd/tree/9d26d9c3af600a5d01f904b138974e02ec7838c4
[prior-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/Docs/audits/stream-completion-contract.md#L182-L231
[predecessor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_openai.go#L195-L201
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/Docs/architecture/perception/14-OPENAI-RESPONSES.md#L53-L60
[provider-json]: https://developers.openai.com/api/docs/guides/structured-outputs
[provider-sse]: https://developers.openai.com/api/reference/resources/responses/streaming-events
[wire-content]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_wire.go#L339-L343
[wire-reply]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_wire.go#L153-L166
[answer]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_reply.go#L26-L48
[tool-parser]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_reply.go#L99-L149
[system-call]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai.go#L310-L342
[tool-call]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai.go#L353-L379
[reply-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_reply.go#L290-L316
[stream-reader]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_stream.go#L69-L126
[stream-classifier]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_stream.go#L133-L155
[stream-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/stream_terminal.go#L49-L81
[classification-factory]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/system/factory.go#L1206-L1304
[classification-client]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_factory.go#L404-L422
[client-wrappers]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_factory.go#L840-L859
[transducer-call]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/transducer_llm.go#L253-L282
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_fallback.go#L92-L200
[execution]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai.go#L269-L293
[compression]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/broker/compression.go#L49-L104
[router]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/broker/router.go#L36-L45
[key-pool]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/key_pool.go#L220-L246
[scheduled]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/core/scheduled_llm_client.go#L141-L192
[tui]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/cmd/nerd/chat/helpers_articulation.go#L309-L380
[stream-caller]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai_stream.go#L30-L64
[conformance]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/stream_conformance_test.go#L81-L120
[effort-test]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai_test.go#L188-L206
[transport]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/responses_transport.go#L113-L120
[classification-stop]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/cmd/nerd/chat/process.go#L330-L337
[classification-failure]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/transducer_llm.go#L146-L164
[native-generation]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/executor.go#L1792-L1846
[working-context]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/working_context.go#L1005-L1025
[no-tool-branch]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/executor_tools.go#L108-L169
[nudge]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/executor_tools.go#L2365-L2401
[tool-audit]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/Docs/audits/request-visible-tool-contract-and-investigation-affordances.md
[retry-audit]: https://github.com/theRebelliousNerd/codenerd/blob/75240b6649e9e7fce0c117ac5b03c6f4f0a95840/Docs/audits/retry-jit-callability-continuity.md#L11-L38
[hollow-success]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/executor.go#L2935-L2976
[jit-allowlist]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/session/executor_tools.go#L2445-L2452
[classification-audit]: https://github.com/theRebelliousNerd/codenerd/blob/75240b6649e9e7fce0c117ac5b03c6f4f0a95840/Docs/audits/perception-classification-outcomes-and-recovery.md#L320-L337
[meta-ground-stream]: https://github.com/theRebelliousNerd/codenerd/blob/9d26d9c3af600a5d01f904b138974e02ec7838c4/internal/perception/client_openai_compat.go#L815-L826
