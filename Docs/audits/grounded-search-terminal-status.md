---
doc-class: deep-dive
subsystem: perception-research
implementation-status: proposed
last-verified: 2026-10-30
verified-against: a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6
verification-method: static-source-inspection
---

# Preserve terminal status in explicit grounded search

## Finding and scope

The Meta implementation of `GroundedWebSearch` can return an incomplete Responses answer as successful structured research evidence. Its dedicated transport rejects HTTP failures and API error objects, but the conversion to `GroundedWebSearchResult` checks only that assistant text is nonempty. It does not admit the response's terminal status. The resulting tool JSON contains text, citations, retrieved pages and usage, but no indication that generation was incomplete.

This is a source-proven conditional acceptance defect on public `theRebelliousNerd/codenerd` branch `main`, pinned at `a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6`. Inspection occurred on 2026-10-30. No tests, application execution, inference, benchmarks or live provider probes were performed. No frequency, observed incorrect answer, final-task success or safety bypass is claimed. Repository sources were read through the GitHub connector; this note is not a code change.

The narrow repair is to classify and preserve terminal outcomes before explicit search publishes an ordinary result. Keep the dedicated search transport's sanitized errors and bounded payloads. Reuse existing truncation and refusal semantics rather than inventing a search-specific string convention.

## Reachable production path

Boot obtains the grounded-search interface from the actual configured main client and passes it to modular-tool hydration. Hydration conditionally registers the tool in both the VirtualStore and global registries through the catalog registrar. The registrar delegates to the research package's support check. This is a configured Meta capability, not a claim that every provider or task offers it. [Boot selection][boot], [hydration][hydrate], [catalog][catalog], [conditional registration][registration].

Given successful boot, a fresh appropriate registry, an offered and permitted `grounded_web_search` call, valid query, and admitted model request, the handler calls the injected searcher. The broker's grounded method forwards through its router or uses its existing metered compression wrapper. Neither can recover status already discarded by the provider adapter. The tool handler checks error and nil result, then serializes the result whole with no terminal field. [Broker][broker], [tool handler][tool].

The concrete provider sequence is:

1. Meta `GroundedWebSearch` builds its Responses request and calls `postGroundedSearch`.
2. The transport bounds the body, handles HTTP errors and rejects any nonnil API error object. After JSON decode it returns the reply without examining `Status` or `IncompleteDetails`.
3. The caller immediately invokes `groundedResultFromReply`. This extracts non-commentary assistant text and rejects only empty text.
4. A nonempty result is returned with nil error, usage is reported and sources are recorded. The normalized result retains no terminal status or incomplete reason. [Meta call and publication][meta-call], [transport completion][transport], [result conversion][result].

The broker already knows how to restate a truncated answer, but only when `types.AsOutputTruncated(err)` succeeds. A nil error takes the ordinary return branch. Thus this omission bypasses an existing recovery contract, not merely a log field. This does not imply that every typed failure should be retried. [Compression boundary][compression].

## Bounded witness and provider contract

A deterministic fixture can return HTTP 200 with a JSON response containing:

- `status: "incomplete"`;
- `error: null`;
- `incomplete_details.reason: "max_output_tokens"`;
- a non-commentary assistant message with nonempty `output_text`;
- optionally valid citation/search metadata and usage.

With the production conditions above, the current source returns that prefix as a successful grounded result and the tool emits ordinary JSON. The prefix can end at a grammatical sentence or form valid JSON; textual plausibility is not terminal completion evidence. No concurrent call, stale source getter, provider fallback, malformed body or tool-loop continuation is needed.

Meta's Responses documentation distinguishes terminal completion and incomplete outcomes and exposes incomplete details. Its computer-use guide explicitly documents an output-limit response as HTTP 200 with the incomplete status and output-limit reason. That guide is corroborating protocol documentation, not evidence of a grounded-search incident; applying the documented response shape to this search adapter is the source-derived witness. No live response was captured. [Responses protocol](https://dev.meta.ai/docs/protocols/responses), [Meta output-limit behavior](https://dev.meta.ai/docs/computer-use#handle-done-and-incomplete).

## Existing safeguards and exclusions

The gap is not universal. OpenAI's explicit grounded-search caller checks nonempty status and rejects anything other than `completed` before result conversion. Ordinary Meta completions and native tool-results calls use `responsesTerminalError`, which delegates to the shared terminal classifier. These are useful local precedents; neither sits on Meta's explicit search path. [OpenAI guard][openai], [ordinary Meta admission][meta-terminal], [shared classifier][terminal].

The transport still rejects HTTP errors, oversized responses, malformed JSON and nonnil API errors. Empty assistant text also fails. The witness deliberately avoids these controls rather than claiming they are broken. A `failed` response that carries an API error object is already rejected. This note does not claim all failed statuses survive, nor establish that queued or in-progress responses normally occur for this synchronous request.

A bounded audit search found no exact prior finding: `GroundedWebSearch` under `Docs/audits` matched only the old registration inventory and the request-visible tool-contract audit; `postGroundedSearch` had no documentation matches. The stream-completion audit owns streaming terminal evidence; the Responses refusal audit owns ordinary system/tool/SSE refusal identity; the grounding-source audit owns streaming attribution. This finding is the explicit-search acceptance boundary, not a replacement for those broader contracts. [Stream contract][prior-stream], [Responses refusal][prior-refusal], [Grounding attribution][prior-grounding].

## Proposed contract and alternatives

Before a provider reply becomes a successful `GroundedWebSearchResult`:

- Admit a supported completed outcome; reject explicit incomplete, failed and canceled outcomes. Specify and test compatibility for missing or unknown status rather than accidentally accepting it.
- Map output-limit incompleteness to the existing typed truncation contract, so permitted broker restatement is available. Other incomplete reasons, refusal and filtering need their own non-success dispositions, not availability failover.
- Preserve usage for every billed response even when no successful result is published. Meta currently reports usage after result conversion; moving admission earlier without moving accounting could hide failed-attempt cost.
- Keep raw provider messages, response bodies, reasoning and credentials out of model-facing errors. Calling the ordinary terminal helper blindly may violate this search-specific sanitization contract.
- Publish sources only with the corresponding admitted result, unless an explicit typed partial-evidence contract defines another safe use. Do not disguise partial answer text as a successful answer to preserve URLs.
- Do not change provider, model, output budget, permissions or server retention as an implicit repair.

A small sanitized guard in the Meta caller closes the demonstrated acceptance hole, but a generic status error would still prevent typed truncation recovery. A shared search terminal-admission helper can align Meta and OpenAI, retain a safe projection of shared terminal semantics and reduce future drift. Putting all admission in the generic transport is less attractive because callers own usage settlement and request-specific diagnostics. Evaluate these choices against existing contracts, not just code volume.

## Costs and acceptance evidence

The status check adds constant work over an already decoded reply; full refusal/content classification is linear in existing content blocks. It needs no extra model request for an intact answer. Correctly rejected truncations may trigger the broker's existing bounded restatements, increasing tokens and latency while preventing partial evidence from masquerading as completion. No net savings are established. Report failed-attempt usage, retry count, total wall time and accepted-result quality together; lower cost obtained by silently accepting a prefix is not an efficiency gain.

Proposed checks, all unrun:

1. Drive Meta's real explicit-search adapter with completed and incomplete HTTP-200 fixtures carrying identical nonempty text. Only the completed fixture may produce a successful result.
2. Cover output-limit and non-length incomplete reasons; empty output; API error object; malformed and oversized bodies; cancellation; completed refusal where supported; absent and unknown status under the chosen compatibility policy.
3. Through the broker and registered tool, assert that typed truncation follows permitted bounded recovery and an exhausted attempt never emits success JSON. Preserve the intact-answer baseline without extra calls.
4. Assert usage settlement on every billed non-success and success attempt, and no source publication from a rejected attempt.
5. Preserve sanitized-error fixtures with malicious provider messages, keys and reasoning; ensure no new terminal path leaks their contents.
6. Compare OpenAI and Meta explicit search with ordinary Responses completion admission, without assuming identical vendor capability support.

Acceptance is a provider-declared incomplete answer remaining non-success through the registered search tool, with typed recovery where authorized, complete accounting and unchanged sanitized boundaries. No implementation or test completion is implied.

[boot]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/system/factory.go#L1694-L1699
[hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/virtual_store_tools.go#L85-L104
[catalog]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/tools/catalog/catalog.go#L57-L66
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/tools/research/grounded_web_search.go#L115-L137
[broker]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/broker/optional.go#L135-L157
[tool]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/tools/research/grounded_web_search.go#L72-L111
[meta-call]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/perception/client_openai_compat_grounding.go#L211-L226
[transport]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/perception/client_openai_compat_grounding.go#L319-L338
[result]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/perception/client_openai_compat_grounding.go#L341-L355
[compression]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/broker/compression.go#L79-L112
[openai]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/perception/client_openai_grounding.go#L110-L124
[meta-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/perception/client_meta_responses.go#L392-L401
[terminal]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/perception/responses_reply.go#L323-L356
[prior-stream]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/Docs/audits/stream-completion-contract.md
[prior-refusal]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/Docs/audits/responses-refusal-wire-identity.md
[prior-grounding]: https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/Docs/audits/streaming-grounding-source-ownership.md

