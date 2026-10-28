# Anthropic native context admission: count what the provider will replay

**Status:** source-confirmed representation/admission gap; behavioral evaluation proposed, not executed.  
**Source pin:** [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871), inspected 2026-10-28.  
**Verification scope:** read-only source and existing-test inspection; no tests, benchmarks, or provider experiments run. No repository files changed.

## Decision

Make native server-tool content and provider continuations participate in context admission before optimizing their cost. Preserve grounded capability and native replay fidelity; do not solve the mismatch by silently dropping fetched evidence.

This is not another usage-accounting proposal. Accurate usage after completion cannot prevent an oversized request, and calibration cannot recover content a size function structurally ignores. `ConfidenceExact` is CodeNERD's internal label, not a guarantee of exact provider tokenization; Anthropic describes its count as an estimate even for supported complete inputs.

The originally flagged `NewAnthropicCounter` is now only a [test seam](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/broker/test_seams_test.go#L8-L14). Production reaches `NewAnthropicCounterWithWorkspace` through `Meter.CounterFor`. The meaningful issue is downstream: native server-tool results survive inference replay but disappear from every relevant preflight size representation. Provider-side continuations also bypass renewed admission.

## 1. What current source establishes

| Boundary | Current implementation | Consequence |
|---|---|---|
| Native response preservation | [`blocksFromAnthropicContent`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/blocks_anthropic.go#L112-L140) converts server-use/search/fetch results into `BlockServerTool`. | Grounding results enter native conversation content. |
| Actual replay | [`buildAnthropicMessagesFromHistory`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/blocks_anthropic.go#L78-L84) sends each server block’s raw JSON back. | These bytes are not merely diagnostic metadata. |
| Provider-count request | [`convertCountMessage`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/broker/counter_anthropic.go#L274-L311) handles text, thinking, tool-use and tool-result, but not `BlockServerTool`. | Server content vanishes before `/count_tokens`; successful responses still receive `ConfidenceExact`. |
| Fallback estimate | [`measure`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/broker/measure.go#L54-L95) likewise has no server-block case. | Counting-endpoint failure does not restore the omitted content. |
| Session/slot size estimate | [`EstimateRequestTokens`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/request_tokens.go#L19-L32) JSON-marshals `[]types.Message`. [`Message.blocks`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/types/interfaces.go#L50-L83) is unexported; [`NewMessage`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/types/content_block.go#L152-L174) intentionally excludes server blocks from the public projection. | Both upstream fitting and the slot backstop omit native server-result content. |
| Slot backstop | [`CompleteWithToolResults`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/client_anthropic.go#L323-L342) admits the neutral history before native conversion. | It does not independently measure the assembled Anthropic request. |
| Internal continuation | [`exchange`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/anthropic_web_tools.go#L52-L75) adds configured server tools, then appends paused assistant content and calls `postMessages` directly. | Up to four additional logical continuations occur without reentering broker or slot admission; transport retries can add HTTP sends. |

Go’s normal JSON encoder serializes exported fields, not private native storage; this is a representation omission, not an uncertain tokenizer ratio. [Go JSON documentation](https://pkg.go.dev/encoding/json#Marshal)

The session path is live: [`prepareWorkingRequest`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/working_context.go#L577-L642) preserves messages through block-aware transformations, but computes remaining space using the flat-field estimator. Its size-reduction loop selects public `ToolResults`; server results are not ordinary executor tool results.

### A decisive source-level witness

Take two valid native histories with identical roles, visible text, custom tool calls/results and tools, differing only in the text payload inside one valid `web_fetch_tool_result`:

- history A carries a small page;
- history B carries a substantially larger page.

Current code yields:

1. Identical `EstimateRequestTokens` input serialization.
2. Identical fallback `measure` values.
3. Identical Anthropic counting request bodies and therefore identical count-cache keys.
4. Different inference request bodies, because raw server results are replayed.

Thus increasing replayed fetched content cannot increase any of these preflight counts. This is a causal invariant derived from source, **not an executed benchmark**.

The slot contract explicitly defines its units using the session’s approximate ruler; do not misdescribe it as a provider-tokenizer guarantee. The defect is that this ruler excludes an entire content class actually sent.

## 2. Continuations are new request shapes, not transport retries

`pause_turn` continuation includes content that did not exist at initial admission. Anthropic documents resubmitting paused assistant content to continue server-tool execution. [Anthropic stop-reason guidance](https://platform.claude.com/docs/en/build-with-claude/handling-stop-reasons#pause_turn)

The current resumption limit bounds logical continuation count, not input volume or total HTTP sends: transport retries can produce additional sends. A small original prompt can be followed by a continuation containing a large retrieved document. Neither a successful initial count nor a four-resumption limit establishes that subsequent requests fit. Native server-tool declarations are also added by `exchange` after initial broker and slot admission, so the representation mismatch is not confined to existing result history.

No claim is made that every grounded request exceeds a limit, that this affects streamed Anthropic replies, or that the user’s active configuration enables these opt-in tools. The affected surface is non-streaming direct Anthropic with native server-web content or provider continuation.

## 3. Why “just include server tools in count_tokens” is insufficient

Current official documentation states that the token-counting endpoint rejects server tools such as web search and web fetch. Unsupported content must not be stripped while the resulting count is presented as exact for the complete request. [Anthropic token-counting documentation](https://platform.claude.com/docs/en/build-with-claude/token-counting)

A sound contract should distinguish:

- a provider count for a supported complete request;
- a supported-subset count plus explicitly bounded uncounted content;
- an estimate with declared uncertainty;
- content whose admission cost cannot currently be established.

The policy governing those states should remain explicit. Do not add an unrelated heuristic exception that silently bypasses the existing budget.

## 4. Proposed implementation contract

1. **One prepared request identity:** provider assembly determines content, model, native tool declarations and relevant options before admission. Counting must describe the same prepared request subsequently transmitted.
2. **Block-aware session fitting:** consume `Message.Content()`, not its public projection. Do not expose reasoning or raw server content in ordinary transcript logs merely to make JSON sizing convenient.
3. **Native-content cost policy:** do not charge raw ciphertext/base64 bytes as ordinary text tokens. Preserve opaque protocol objects; use documented semantics, provider evidence or a conservatively qualified bound.
4. **Readmit each continuation:** after a paused response changes the request, evaluate its new input against the same slot/window policy before dispatch.
5. **Bound content at the provider where supported:** web-fetch `max_content_tokens` is a possible configurable control; it is not currently represented in the repository’s web-tool config. Anthropic describes this limit and warns that fetched content consumes ordinary token budget; its text limit does not cover binary PDFs. [Web-fetch documentation](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-fetch-tool#content-limits)
6. **No silent destructive recovery:** if required native state cannot safely fit, surface a typed limit outcome or a policy-approved restart. Truncating opaque signed/native blocks is not valid compaction.

## 5. Evaluation: alternatives and closure gates

Compare the same tasks and scripted provider responses under:

- **A — current implementation**
- **B — block-aware estimates only**
- **C — complete-request admission plus continuation readmission**
- **D — C plus configurable provider content limits**

B is a useful ablation, not sufficient closure: it cannot fix a continuation that never reaches admission.

Required deterministic fixtures:

- Same visible history, increasing valid server-result payload: admission size or explicit uncertainty must change.
- Count endpoint available versus unavailable: neither path silently loses server content.
- Large returned page, small configured slot cap: continuation must be refused before the inference endpoint receives it.
- Boundary-fitting continuation: it must still succeed.
- Mixed web results and custom tool calls: preserve native block order, IDs, signatures and execution separation.
- Server tools disabled: ordinary requests retain their behavior.
- Cancellation during continuation admission: no extra provider send.
- Credential-pool route: selected key/workspace and continuation admission remain aligned.

Existing count-body tests inspect a static-marker request rather than comparing count and inference encodings; existing web-tool tests verify replay and bounded resumptions rather than size enforcement. Read tests as evidence of intended coverage, not proof this gap was exercised. [Count tests](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/broker/counter_anthropic_test.go#L91-L127), [web-tool tests](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/anthropic_web_tools_test.go)

Then run normal production-entry grounded coding/research tasks with CodeDOM and world-model context enabled. Report verified task completion, citation/evidence retention, unnecessary refusals, provider context failures and repeated work—not simply lower tokens.

Measure input/output/cache/reasoning tokens, search-use charges, all continuation attempts, counting calls, serialization CPU, peak bytes, p50/p95 latency and human recovery time. No universal savings percentage is justified yet.

## 6. Novelty and limits

This differs from the existing [detailed-usage study](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/detailed-usage-wiring-and-verified-efficiency.md), which repairs post-response report transport, and the [retrieval-freshness study](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/retrieval-freshness-coverage-and-context-value.md), which repairs evidence validity and recall. It also differs from the [September prompt-budget inventory](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/2026-09-09-system-hardening/02-PROMPT-BUDGET.md): the present failure crosses newly native server-tool replay, the flat session estimator and internal provider continuation.

Repository searches for `pause_turn`, `BlockServerTool`, `server_tool` and `max_content_tokens` found implementation/config/tests but no audit documenting this admission chain. This is bounded nonduplication evidence, not proof that no external discussion exists.

**Closure condition:** no outgoing affected request may claim a complete admission measurement after omitting native content, and every changed continuation must face the same admission contract. Demonstrate that without degrading native replay fidelity or grounded-task success.
