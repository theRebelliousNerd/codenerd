# Anthropic native context admission: count what the provider will replay

**Status:** representation gap closed for the broker, the session fitter and the slot backstop (see As-built status at the end); continuation readmission and web-fetch content limits remain open and need `internal/perception`. Behavioral evaluation proposed, not executed.  
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


## As-built status

Verified 2026-10-28 against the `feat/conversation-brief` worktree, whose Anthropic client, broker, prompt and session sources match the pin for every boundary in section 1. Only the changes below were made; `internal/perception` was not touched (another session owns it).

### Claim ledger

| Claim | Verdict | Evidence in the worktree |
|---|---|---|
| `NewAnthropicCounter` is only a test seam; production uses `NewAnthropicCounterWithWorkspace` | Confirmed, nothing to build | `internal/broker/test_seams_test.go:11`; production caller `internal/broker/default.go:237` |
| Server blocks are preserved and replayed raw | Confirmed, preserved | `internal/perception/blocks_anthropic.go:78-87` (replay), `:129-132` (capture) |
| `convertCountMessage` omits `BlockServerTool` and the success is labeled exact | Real, fixed | `internal/broker/counter_anthropic.go` (`convertCountMessage`, `withServerContent`) |
| `measure` has no server-block case | Real, fixed | `internal/broker/measure.go:95-100`, `serverContentChars` |
| `EstimateRequestTokens` marshals the public projection and omits native blocks | Real, fixed; the same omission also hid replayed reasoning (signed or redacted thinking), fixed with it | `internal/prompt/request_tokens.go` (`nativeBlockTokens`) |
| The slot backstop admits the neutral history before native conversion | Confirmed, and now block-aware through the one estimator | `internal/perception/client_anthropic.go:327`, `internal/perception/input_window.go:30` |
| The session fitter uses the flat estimator and cannot archive server results | Confirmed; now sees server content. Server blocks are opaque and are never archived, so a history whose native content alone overruns the window is refused with `ErrInputBudgetExceeded` | `internal/session/working_context.go:898-905`, `:577-642` |
| `exchange` readmits nothing across up to four continuations and the declarations it adds | Real, open | `internal/perception/anthropic_web_tools.go:52-75`; needs `internal/perception` (patch below) |
| The count endpoint rejects server tools | Taken from the doc's cited provider documentation, not re-verified. The count body deliberately never carries server content | `counter_anthropic.go` (`convertCountMessage`) |
| `max_content_tokens` is not represented in config | Confirmed absent (`grep` over `internal`); not built, because the only consumer is the tool declaration `exchange` builds in `internal/perception` and a config field with no reader would be dead | |

The doc's causal witness (two histories differing only in the fetched page get identical preflight sizes) is now false for every size representation the repository owns; the tests below are that witness inverted.

### What was built

- `types.ServerBlockChars` (`internal/types/server_block_size.go`): the one definition of a replayed server block's size, in characters. The block's JSON is counted as the text it is; opaque encoded payloads (`encrypted_content`, `encrypted_index`, a `base64` source's `data`) are charged at their decoded upper bound (three quarters of the encoded length), not as prose. Unparseable raw is charged whole, never as zero.
- Broker (`internal/broker/measure.go`): `measure` counts server blocks, so the fallback estimate, calibration and attribution all see them. Calibration trains on the same size the bill reflects.
- Counter (`internal/broker/counter_anthropic.go`, `types.go`): the provider count covers the supported subset; replayed server content is added as a bounded estimate, the result takes the estimator's confidence (never `ConfidenceExact`), `Count.EstimatedTokens` declares the estimated share, and `Source` names it. The provider count is cached unchanged; the estimate is rebuilt from the current ratio. A request with no server content is a pure provider count as before.
- Prompt estimator (`internal/prompt/request_tokens.go`): `EstimateRequestTokens` adds `nativeBlockTokens` (server blocks, signed reasoning; redacted reasoning at the tokens the provider counted; unsigned reasoning, which is never replayed, at zero). The session fitter and the Anthropic slot backstop both go through it, so both are now block-aware without a second code path. `ServerContentTokens` is exported for the continuation patch below.

### Tests

- `internal/types`: `TestServerBlockChars`, `TestServerBlockCharsInvalidJSONIsChargedWhole`.
- `internal/broker/server_content_test.go`: `TestMeasureCountsReplayedServerToolBlocks`, `TestAnthropicCounterDeclaresServerContentAsEstimate` (same visible history, larger payload: count grows; server content absent from the count body; never exact; estimated share declared), `TestAnthropicCounterFallbackKeepsServerContent` (endpoint down: no silent loss), `TestAnthropicCounterWithoutServerContentStaysExact` (disabled path unchanged).
- `internal/prompt/request_tokens_test.go`: `TestEstimateRequestTokens_CountsReplayedServerContent`, `TestServerContentTokens_DoesNotChargeBase64AsProse`, `TestEstimateRequestTokens_CountsReplayedReasoning`.
- `internal/session/working_context_test.go`: `TestPrepareWorkingRequest_FitsToReplayedServerContent` (a fitting block is replayed byte for byte; a block that cannot fit is a typed `ErrInputBudgetExceeded` before any provider call). Confirmed failing with `nativeBlockTokens` disabled.

### Deliberately left, with reasons

1. **Continuation readmission (section 2, contract item 4).** Lives in `internal/perception/anthropic_web_tools.go`. Patch: thread the neutral `history` and `defs` that `CompleteWithToolResults`, `CompleteWithTools` and `Complete` already hold into `exchange` (today it receives only the `AnthropicRequest`); before each `c.postMessages(ctx, next, plan)` in the `pause_turn` loop, build `grown := append(history[:len(history):len(history)], types.NewAssistantMessage(blocksFromAnthropicContent(resp.Content)...))` (`resp.Content` is already cumulative across resumes) and call `c.admitInput(system, grown, defs)`; on `ErrInputWindowExceeded` return an error that says the paused turn's continuation was not sent (the first request was). Check `ctx.Err()` first so a cancellation sends nothing. Tests to add there: a scripted `pause_turn` response carrying a large fetch result under a small `MaxInputTokens` must leave the endpoint at one inference hit; a boundary-fitting continuation must still be sent; a cancelled context must add no send; with web tools off, requests are unchanged.
2. **Server-tool declaration cost (section 2).** The declarations `exchange` appends (`web_search`, `web_fetch`) and the provider-side prompt they imply are not in `Request.Tools`, so no client-side measure sees them. Their token cost is provider-defined and was not established here, so no constant was invented. The same perception patch should add the declarations' JSON to the admitted tool list, and a count for a grounded slot should be read as a lower bound until the provider's `usage` figures calibrate it (the calibrator already learns from them through `measure`).
3. **`web_fetch` `max_content_tokens` (item 5).** Needs a config field and the tool declaration, both consumed in perception; add together with item 1.
4. **Count-endpoint behavior with server blocks in messages** was not probed (no provider calls). The conservative path (never send them, estimate them, say so) is correct whether or not the endpoint would have accepted them. If a probe shows it does, a later change can send them and shrink `EstimatedTokens`.
5. **Credential-pool alignment** needs no change here: counting and admission take their key and workspace from the same `Meter.CounterFor` route as before.
6. **Behavioral evaluation (section 5, arms A-D, token and latency ledgers)** was not run. The opaque-payload bound (three quarters of the encoded length) is an upper bound on plaintext, not a measured ratio; whether it tracks the provider's charge for encrypted search results is for the first grounded-task run to show, through the broker's existing estimate-versus-bill reconciliation.
