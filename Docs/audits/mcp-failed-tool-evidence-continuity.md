---
title: Failed MCP tools need recoverable diagnostic evidence
subsystem: mcp-control-plane
status: source-supported residual; proposed contract; evaluations unrun
source_commit: 7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9
review_date: 2026-11-05
---

# Failed MCP tools need recoverable diagnostic evidence

## Finding and decision

**The ordinary `mcp_call` path can correctly report a remote tool failure while discarding the diagnostic content returned with that failure.** The current wire decoder preserves an `isError:true` result in `MCPCallResult.Output`. The manager returns that result. But `ControlPlane.Call` returns early whenever `raw.Success` is false, carrying only failure metadata and the extracted error string. It never reaches the payload digester or handle store. A valid embedded diagnostic resource therefore disappears from the model-facing result, even when the entire response is tiny and the caller requested `view:"full"`. [Wire result][wire] [Manager][manager] [Failure projection][projection]

This is a residual of the existing error-content requirement in [MCP versioned wire fidelity][prior-wire], not a new claim that `isError` is ignored. That earlier false-success defect is repaired: this path returns `success:false`, and the manager passes false to asynchronous usage recording when a store is present. The remaining gap is **evidence delivery after correct outcome classification**, not a claim that every accounting write succeeds.

**Proposed decision:** separate the invocation's outcome from the disclosure of its evidence. Keep a tool failure failed, but pass its valid result payload through bounded presentation and, when necessary, local retention and expansion. A failed operation may supply exactly the information needed to repair the request or understand partial effects. Its status must not decide whether that evidence remains accessible.

This report is a static source deduction at public main [`7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9`][pin]. Root and scoped guidance, MCP architecture, relevant implementation and test definitions, the current audit inventory, and targeted repository searches were inspected. No CodeNERD program, server fixture, test, build, benchmark, model inference, training or runtime reproduction was executed. All evaluation proposals below are unrun. No production incident, frequency, automatic duplicate effect, or measured task-quality gain is claimed.

## Intended behavior and current status

The root contract requires bounded, revision-aware and recoverable context. The model-facing tool description promises that withheld result material is retained for expansion without rerunning the call. MCP's implementation specification describes failure classification and full payload preservation at the transport boundary. Those are compatible requirements, but their composition is incomplete. [Root guidance][root] [Architecture guidance][architecture-guidance] [Tool contract][tool] [Implemented wire contract][implemented]

| Status | Evidence and scope |
| --- | --- |
| Implemented | The supported wire decoder identifies `isError:true`, sets `Success:false` and `ToolError:true`, preserves the full result in `Output`, and derives a text summary. |
| Implemented | The manager preserves the output and uses the false success value for usage recording. Current remote review, schema, permission and approval controls remain before dispatch. |
| Implemented | Successful control-plane results are shaped by view, with omission metadata and a retained handle when shaping withholds content and retention admits it. |
| Partial | A valid failed-tool result reaches the manager intact but loses its non-summary payload at the control-plane return. The new `ToolError` distinction is not represented in that `CallResult` envelope either. |
| Proposed | A shared bounded evidence path for successful and failed valid tool results, outcome-preserving expansion, and end-to-end diagnostic-disclosure tests. |
| Unverified | Runtime incidence, every alternative diagnostic sink, model recovery behavior, the proposed implementation and all cost or outcome measurements. |

The status table does not treat dated architecture prose or historical test receipts as proof that those tests ran in this review. Current behavior below is traced from pinned implementation.

## Production trace

### A configured bridge exposes the same control plane to the model

The normal system factory calls `initMCPBridge`. That builds the integration bridge, binds its control plane through `mcpctl.SetControlPlane`, and starts configured connections. The bridge constructs `NewControlPlane` and installs the kernel risk gate. The modular catalog registers the MCP family. Thus the affected method is on the production model-facing route, not only a directly callable helper. [Factory entry][factory-entry] [Binding][binding] [Bridge][bridge] [Catalog][catalog]

The `mcp_call` handler validates the requested view, obtains arguments and the bound plane, calls `plane.Call`, and serializes that returned envelope. Its serializer has no separate parameter containing the manager's raw payload. [Facade][facade] [Serializer][serialize]

Reachability still requires an available configured server, a resolved tool, valid arguments, current host review and schema identity, exact positive kernel permission, and any applicable human approval. A test fixture must establish those conditions; merely providing a server or setting `confirm_risk` is not sufficient. The witness below assumes ordinary admitted read-only work and does not attempt to defeat those boundaries. [Admission and dispatch][authority]

### The wire boundary correctly keeps both failure and evidence

The decoder parses the tool result, checks the inspected envelope fields, and handles `isError:true` by returning:

- `Success:false`;
- `ToolError:true`;
- the full original result in `Output`;
- an `Error` string produced by `toolErrorText`.

The summary helper collects only nonempty top-level content blocks whose `type` is `"text"`. It does not extract `resource.text` from embedded resource blocks, preserve their URIs, or encode images. When no top-level text exists it uses a fallback message. That is reasonable behavior for a short summary **if the original content remains separately available**. The source comment explicitly says preservation is intended to support shaping and retrieval. [Decoder and summary][wire]

All three transport implementations use this decoder. The manager calls the authorized transport, preserves `Output`, and schedules usage accounting using `result.Success` when a store is configured; the manager's old 500 KiB output cut is explicitly removed. This finding requires neither an oversized wire message nor a transport error. [HTTP][transport-http] [SSE][transport-sse] [Stdio][transport-stdio] [Manager return and usage][manager]

### The control-plane branch drops evidence before it can be retained

After manager execution, `ControlPlane.Call` handles errors, nil results, and `!raw.Success`. The last branch constructs a fresh `CallResult` containing tool identity, facet/risk, requested view, latency, `raw.Error`, `success:false` and a failure summary, then returns. It assigns none of `Data`, `Shape`, `Elided`, `Handle`, `Bytes`, `FullBytes` or `Truncated`. It also does not carry the raw `ToolError` field. [Result type and branch][projection]

Only the following success path calls `BudgetFor`, `DigestJSON` and `handles.Mint`. Consequently, changing the view or item ceiling cannot make the failure branch disclose the lost payload. No handle is minted and then evicted; the failed payload never enters this control-plane retention path. [Shaping and retention][projection]

The omission metadata is absent/defaulted rather than a truthful description of the discarded resource. That does not establish that an explicit “complete” claim was made. It does establish that the caller receives neither the resource nor the ordinary omission-and-continuation mechanism promised for withheld evidence.

### The outer working archive cannot restore an input it never received

The facade serializes only `plane.Call`'s envelope. The session executor takes the modular tool's returned string, settles its effects and passes that string to `recordWorkingResult`. The working record saves the supplied body. This is a useful archive of what the tool facade actually returned, but it has no reference to the discarded `MCPCallResult.Output`. [Facade][facade] [Modular return][modular-return] [Archive handoff][archive-handoff] [Stored body][working]

This distinction matters because the manager comment points to the working context as the reason not to truncate results there. The argument works only if later adapters preserve the content on its way to that archive. A full archive of a shortened envelope is not a full archive of the remote result.

## Concrete witness: a small failed result with an attached diagnostic

Use an isolated, conforming MCP 2024-11-05 server fixture. Its ordinary read-only validation tool has a valid input schema and is admitted through the normal review and permission route. The tool receives one validly encoded request whose business-level validation fails. It responds with a normal matching JSON-RPC response whose `result` is:

```json
{
  "isError": true,
  "content": [
    {
      "type": "text",
      "text": "Request validation failed; details attached."
    },
    {
      "type": "resource",
      "resource": {
        "uri": "diagnostic://validation/r-42",
        "mimeType": "application/json",
        "text": "{\"field\":\"customer_id\",\"reason\":\"required\",\"request\":\"r-42\"}"
      }
    }
  ]
}
```

The sample identifiers are synthetic. The URI names the returned diagnostic; it is not an instruction to contact any real service.

This uses the retained 2024 profile's embedded-resource content, not a newer `structuredContent` extension. The official schema allows text, image and embedded-resource items in `CallToolResult.content`, with an independent `isError` flag. Its error guidance places tool-originated failures in the result so callers can see and act on them. The specification leaves rendering choices to clients; the stronger bounded-recovery requirement here comes from CodeNERD's own interface contract. [Legacy tools specification][mcp-tools] [Legacy schema][mcp-schema]

Request `view:"full"`. Keep the fixture response comfortably below the wire, full-view and retention limits, with no cancellation, corruption, concurrent reconnect, eviction or storage failure. Do not add the diagnostic details to the top-level summary; that would remove the condition being tested.

| Boundary | Source-predicted observation |
| --- | --- |
| Transport decode | Failure is correctly classified; complete `Output` still contains `customer_id`, `required`, `r-42` and the diagnostic URI. |
| Manager return | The same failed result and output reach the control plane. Usage recording receives false success. |
| Control-plane return | `success:false` and the short validation message remain. The embedded resource, URI and field-specific explanation are absent. |
| `mcp_call` serialization | Only that shortened envelope is returned to the tool caller. |
| Local expansion | No handle from this call exists for `mcp_expand` to redeem. |
| Outer result archive and recall | The stored body is the shortened envelope; retrieving it again cannot recover the omitted marker. |

The decisive evidence is the diagnostic's presence at the transport/manager boundary and absence from the actual serialized tool result and its local continuation surfaces. The normal remote call count remains one.

A paired control can return exactly the same content with `isError:false`. That control reaches shaping, and its tiny full-view payload fits the current budgets. It is a fixture for showing status-dependent loss, **not a recommendation to mislabel a failed operation as successful**. Another control can put all diagnostic text in a short ordinary text block; the current summary then carries those text bytes, although not the original content structure. [View budgets][budgets] [Summary extraction][wire]

The practical consequence is an incomplete explanation precisely when repair needs one. A model might guess the missing field, repeat a request, or ask for information the server already supplied. Those are plausible downstream costs to evaluate, not observed model behavior or proof that the whole task necessarily fails.

## Boundaries and alternative recovery paths

**The remote failure remains a failure.** Do not reopen the repaired false-success finding or change statistics to count a successful disclosure as a successful remote operation. Reading the diagnostic successfully is a different event from the tool succeeding.

**Direct manager consumers have the payload.** This report is about the progressive `mcp_call`/`mcp_expand` surface and its ordinary working-result handoff. A caller using `MCPClientManager.CallTool` directly can inspect `Output`; this is not evidence that every MCP consumer discards errors.

**Top-level text is not wholly lost.** `toolErrorText` joins ordinary text blocks into the returned error string. The minimal witness intentionally puts the specific evidence in a supported embedded resource. An image-only failure would take the generic fallback, but the text-resource case already establishes the defect without requiring multimodal rendering.

**A resource URI is not a local recovery handle.** Even when a server can independently serve that URI, the failure envelope above omits it. A later resource search/read is another remote operation with its own authority, availability, cost and version. It is not proof that the historical bytes already supplied by this invocation were retained. No assertion is needed that the server is forbidden to make the resource available elsewhere.

**Logs and external backups are outside the proved path.** A remote server log, transport trace or other separately configured record might preserve the result. The inspected control plane does not return such a reference. This audit does not claim universal irrecoverable data loss.

**No replay is established.** The current failed branch itself does not automatically repeat the tool. Restoring evidence must not introduce that behavior, particularly because a remote failure can follow partial effects. Permission to invoke once and failure of the invocation do not establish that repetition is safe.

**Long text errors expose the same missing shared presentation boundary.** `toolErrorText` does not apply a view budget, and the failure branch copies that string directly into `CallResult.Error`. A large text-only error therefore bypasses the control plane's `DigestBudget` path. Wire bounds and later working-context presentation still exist; this is not a claim of an unbounded process or unlimited model input. Keep this as a related design constraint: simply appending all raw error content to `Error` would exchange omission for unbudgeted disclosure. [Summary extraction][wire] [Failure projection][projection] [Budget scope][budgets]

## Relationship to existing audits and primary literature

The closest predecessor is [MCP versioned wire fidelity][prior-wire]. Its result-normalization section already requires useful error content and original-result preservation for shaping. Its as-built account accurately describes the decoder's `ToolError` flag and retained `Output`. The new contribution is the subsequent control-plane branch that prevents those retained bytes from reaching the promised user of them. This is a concrete remaining implementation boundary, not a replacement specification or a claim to have discovered the general need for error feedback.

[Retained MCP scalar continuation][prior-scalar] starts after a successful result has been stored and concerns a long scalar that available selectors cannot fully address. The present witness is small, uses `view:"full"`, and fails **before any handle is minted**. Fixing substring paging does not recover the discarded failed result; retaining failed results does not by itself fix long-scalar paging. Preserve both acceptance boundaries.

[MCP capability admission][prior-admission] concerns authority before remote dispatch. Here that authority succeeds, and the complete remote result arrives. [The shared retention contract][broker-contract] likewise supplies a reusable distinction between retained payloads and their views; this report does not propose another unrelated cache.

The inspected wire test already checks that both text and image tool-error results retain their original output at the transport API. The large-result control-plane test checks shaping, local expansion and unchanged remote call count for a successful result. Those are useful but separate endpoints. Their composition with a failed embedded-resource result is the missing assertion here. These test bodies were read, not run. [Wire fixture][wire-test] [Successful control-plane fixture][plane-test]

Targeted searches included `ToolError`, `tool reported failure` and `error content`, alongside the audit inventory and relevant source/test bodies. No existing audit of this exact failure-projection witness was found in that bounded screen. Recursion lifecycle and mission candidates examined in the same review were already covered or lacked a complete production witness; they are not bundled into this report.

Saltzer, Reed and Clark's *End-to-End Arguments in System Design* (1984) supplies the relevant architectural lesson: a lower layer's successful preservation is not sufficient evidence of the application-level property at the final consumer. Here, checking the manager's `Output` alone cannot establish that a diagnostic is available through the model-facing facade. This is an analogy guiding test placement, not experimental evidence about CodeNERD or a mandate for a particular storage design. [Primary paper][end-to-end]

## Proposed evidence-disclosure contract

Everything in this section is proposed design, not an existing API or permission grant.

### Preserve outcome and payload independently

For a valid terminal tool result, keep the remote outcome, invocation identity and acquired payload distinct from the status of presenting or expanding that payload.

- A remote `isError:true` result remains unsuccessful in the original call envelope, usage learning and any invocation receipt.
- A successful local disclosure operation means evidence was obtained, not that the historical invocation became successful.
- Full-result retention preserves the original `isError` and content types. When expansion selects only a subtree, its envelope should still make the original invocation's failed outcome and provenance available without requiring inference from the selected text.
- Keep pre-dispatch refusals, transport uncertainty, malformed responses and valid tool execution failures distinguishable. Merely finding nonempty raw bytes is not enough to claim a valid tool-result payload.

The host adapter supplies truthful typed observations. Mangle remains responsible for authority and any continuation or completion decision. The repair is not permission to retry, fetch an embedded URI or treat remote diagnostic instructions as trusted control.

### Route valid failed results through bounded disclosure

Use the existing shaping and retention machinery for valid tool-result content regardless of success. For a small full-view failure, include the diagnostic content directly. For a bounded view that withholds evidence, return omission metadata and a valid local handle when retention succeeds. Keep the human-readable error summary short and useful.

Do not copy the success constructor wholesale: it currently writes `Success:true` and an “ok” summary. Shared payload processing needs outcome-aware envelope construction. Likewise, a new handle must refer to the exact acquired result, not a reconstructed object that silently drops resource fields or annotations.

Bound the serialized error envelope as well as its shaped data. Current `MaxBytes` is documented as a cap on the shaped payload, not a proof that every surrounding field fits the same number. Account for escaping, summary, identity, omission metadata and any schema/refusal detail when defining a total response ceiling. Preserve the separate limits for wire acquisition, retained bytes, string width, arrays, depth and object keys. [Current budgets][budgets]

Reuse the current handle lifecycle rather than promising indefinite recovery. The MCP store applies its own entry, byte and TTL limits over the shared retention engine. If retention refuses, fails, expires or is evicted, expose that state honestly and preserve the original failure classification. Do not publish a usable handle for missing bytes or instruct automatic replay as a substitute for local expansion. [MCP handle storage][handles]

### Bind recovery to the actual observation

Carry the existing tool/server and request association through the result's record. A diagnostic's embedded URI and text are source data, not a grant to access the URI or proof that its content remains current later. Reconnect, catalog changes, repeated identical requests and changed responses must not relabel an earlier error as the latest tool state.

The normal outer archive should retain a complete supported diagnostic view or a live reference to the exact original payload, with a defined lifetime relationship. Avoid two unrelated references whose independent eviction silently breaks the promised continuation. If broader working-context retention is chosen instead of the MCP handle store, document that ownership explicitly and verify the actual tool-facing path.

A missing diagnostic must become an evidence-availability outcome, not a fabricated “no details supplied” claim. Conversely, empty error content can truthfully remain empty. Do not invent details, enlarge permission, suppress the failure, or let a remote error decide policy.

## Proposed evaluation, not execution

Begin with deterministic protocol fixtures; no model is needed to establish the loss. Use hand-authored valid 2024 wire JSON independent of the client's result structs. Then exercise the configured bridge, registered `mcp_call`, actual manager/transport and serialized facade output under normal review and permission setup.

For the minimal case:

1. Instrument fixture request counts, exact response bytes and wire/manager observations without making a separate direct invocation.
2. Invoke the model-facing facade once with full view, capturing all observations from that same invocation.
3. Assert that its wire/manager result is unsuccessful and contains the attached diagnostic, then inspect the serialized facade result rather than stopping at the manager return.
4. Under the proposed implementation, require the diagnostic URI and text to be present directly, or available through an explicitly declared local continuation when a narrower view withholds them.
5. Inspect the executor's recorded working result and its supported recall path.
6. Require the remote invocation count to remain one throughout local expansion/recall. Keep failure statistics and original outcome unchanged.

| Case | Required oracle |
| --- | --- |
| Tiny failed text plus embedded resource, full view | Specific diagnostic and URI reach the facade without changing the failed outcome. |
| Same content marked successful as a fixture control | Shared disclosure behavior is comparable; the fixture is not an admissible production “fix.” |
| Short text-only failure | Existing visible error text remains useful and the original call remains unsuccessful. |
| Resource-only or image-only valid failure | Honest typed evidence/availability, rather than losing all content behind a generic summary. Image rendering support must be stated separately from byte retention. |
| Narrow view with multiple or deep resources | Explicit elisions; local handles refer to the acquired failure payload; expansion makes bounded progress. |
| Large error text, Unicode, escaping and many content blocks | Every declared envelope/payload budget is measured after serialization; omitted bytes remain recoverable within the supported retention contract. |
| Retention refusal, eviction, expiry and process restart | Truthful unavailable state; no fabricated handle and no automatic remote rerun. |
| Protocol error, malformed result and no result | Remain distinct from a valid failed tool result; diagnostic handling does not create false success. |
| Denied permission, missing review, stale schema or refused approval | Zero remote dispatch remains required; error-evidence work never bypasses admission. |
| Cancellation or lost response after dispatch | No claim that a diagnostic was received when it was not; preserve uncertainty and do not replay automatically. |
| Mutable resource or reconnect after the result | Local expansion returns the captured observation; a fresh remote read is not silently substituted. |
| Effectful synthetic fixture returning failure after a counted change | Local inspection does not repeat the effect, and original outcome/effect uncertainty stays visible. |
| Successful large-result regression suite | Existing shaping, original-payload pointers and no-reinvocation behavior remain intact. |

The small full-view witness is the primary acceptance test. It must not require an artificial tiny budget, unavailable server, unusually long string, model mistake or altered permission gate. Large and failure-injection cases validate the broader contract but should not obscure that simple result.

For any later explicitly authorized end-to-end model evaluation, use tasks whose diagnostic identifies the needed repair and tasks where the error is already self-contained. Hold tools, model, prompts, authority, server responses and initial task states equal. Include unrecoverable remote errors and correctly refused retries. Count task correctness and recovery evidence separately from a model's willingness to continue.

## Full costs and fair comparisons

Let `N` be the acquired result bytes, `T` the top-level text extracted into the error summary, `P` the shaped payload bytes and `R` the original bytes admitted to retention.

The current failed route already pays the remote operation, network transfer, wire parsing, raw-output storage and text-summary extraction. The small witness's missing resource does not save that upstream work. It avoids later shaping and retained-copy work by withholding evidence, which is not an equivalent successful outcome.

A common payload path adds parsing/walking/serialization proportional to the content examined, plus retained storage and copying when a view omits material. The current retention engine copies admitted payloads, and local expansion parses and shapes the selected content. These CPU, memory and storage costs remain even without another model or remote call. Avoid claiming a universal peak-memory reduction from shorter prompts: retaining the original and constructing a view can temporarily increase host memory. [Digest implementation][digest] [Handle expansion][handles]

For small full-view failures, direct inclusion can avoid a handle and extra local expansion call. For larger failures, a bounded summary plus selective local access can be preferable to sending every diagnostic byte on every turn. Whether that reduces total task cost depends on which evidence is needed and how many disclosures follow. No token saving follows automatically from this audit.

Compare at least:

- the current failure-summary-only path;
- bounded outcome-preserving payload disclosure with existing retention;
- an explicitly scoped alternative that retains the raw result through the outer working archive.

An unbounded raw error dump is useful only as a diagnostic reference, not a fair production baseline. Reinvoking the remote tool is neither equivalent retrieval nor a safe cost-saving alternative.

Report separately:

- diagnostic availability, exact payload identity, original failure classification, omission accuracy and local-expansion success;
- remote invocation counts, duplicate-effect counts, blocked dispatch and unchanged usage-success accounting;
- bytes acquired, parsed, copied, returned and retained; cache eviction pressure and retention failures;
- host CPU, allocation/peak memory, serialization, local read and expansion latency, and task critical-path latency;
- all model input/output/reasoning and cache-token classes actually observed, failed/retried attempts, tool calls and remote charges when known;
- repair success, false diagnosis, unnecessary repeat calls and user/operator recovery time.

Keep no-error overhead separate from error-conditioned benefit. Include all attempted cases and retained-evidence failures in denominators. Do not add overlapping token categories or concurrent stage timings, and do not translate unavailable remote costs into zero. A monetary aggregate requires explicit prices and disjoint units.

## Acceptance boundary

Close this gap when a valid failed remote tool result remains failed **and** its supported diagnostic content reaches the model-facing result or a truthful bounded local continuation through the ordinary production path. Verify the serialized facade and working-context handoff, not only the transport's `Output`.

Preserve the repaired wire outcome, exact remote authority, response limits, local no-reinvocation design and existing success-path behavior. The narrow source-supported conclusion is: **keeping the error payload in the transport result does not preserve it for the caller when the next adapter returns before disclosure and retention.**

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9
[root]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/agents.md#north-star
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/Docs/architecture/AGENTS.md
[implemented]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/Docs/architecture/mcp/IMPLEMENTED_SPEC.md
[wire]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/wire.go#L314-L375
[manager]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/client.go#L667-L742
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/controlplane_invoke.go#L54-L170
[factory-entry]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/system/factory.go#L1880-L1890
[binding]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/system/factory_mcp.go#L18-L55
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/integration.go#L103-L138
[catalog]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/tools/catalog/catalog.go#L26-L44
[tool]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/tools/mcpctl/tools.go#L109-L129
[facade]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/tools/mcpctl/tools.go#L132-L157
[serialize]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/tools/mcpctl/mcpctl.go#L51-L62
[authority]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/remote_authority.go#L435-L580
[transport-http]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/transport_http.go#L126-L153
[transport-sse]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/transport_sse.go#L485-L514
[transport-stdio]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/transport_stdio.go#L366-L392
[modular-return]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/session/executor_tools.go#L2801-L2857
[archive-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/session/executor_tools.go#L1105-L1155
[working]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/session/working_context.go#L391-L434
[budgets]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/digest.go#L55-L101
[digest]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/digest.go
[handles]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/handles.go#L33-L152
[wire-test]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/wire_fidelity_test.go#L439-L477
[plane-test]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/mcp/controlplane_test.go#L357-L402
[prior-wire]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/Docs/audits/mcp-versioned-wire-fidelity.md
[prior-scalar]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/Docs/audits/mcp-retained-scalar-continuation.md
[prior-admission]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/Docs/audits/mcp-capability-admission.md
[broker-contract]: https://github.com/theRebelliousNerd/codenerd/blob/7a62ee763d0a8a70cf7d007a7b6310a4a40ee8d9/internal/retain/retain.go#L1-L22
[mcp-tools]: https://modelcontextprotocol.io/specification/2024-11-05/server/tools#tool-result
[mcp-schema]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/schema/2024-11-05/schema.ts#L610-L664
[end-to-end]: https://web.mit.edu/Saltzer/www/publications/endtoend/endtoend.pdf

