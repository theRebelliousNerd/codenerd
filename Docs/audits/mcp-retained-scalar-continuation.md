---
title: Retained MCP string tails need a bounded path to disclosure
subsystem: mcp-control-plane-retained-results
status: source-supported; not reproduced; proposed contract
source_commit: 715b4c931d352e98f3a9a536ffa4ccfee25f3d29
review_date: 2026-11-03
---

# Retained MCP string tails need a bounded path to disclosure

## Finding

At **`715b4c931d352e98f3a9a536ffa4ccfee25f3d29`**, `mcp_call` can retain an entire successful remote result and advertise `mcp_expand` for its withheld content, while the expansion interface cannot disclose the tail of a long string. Every available view returns a prefix; the largest view caps a string at 4,000 source bytes. Selecting `/content/0/text` removes the surrounding structure but applies the same cap. A deeper pointer is rejected because the selected value is a scalar, and no offset, range, cursor or search parameter is exposed. [Call surface][call-tool] [Expansion surface][expand-tool] [Budgets][budget] [Scalar shaping][scalar] [Pointer resolution][pointer]

This is **retention without complete addressability**, not missing storage, silent truncation, or an argument for unbounded `full` responses. The output honestly reports truncation. The defect is that a promised continuation can stop making progress while the original bytes remain available. A model following the next-step advice can repeatedly receive the same prefix or a scalar-traversal error; actual repeated model behavior was not observed. [Expansion envelope and advice][expand]

**Status and scope.** Static public-source review only. No repository application, server, model call, build, test, benchmark or reproduction was executed. Existing test definitions were read, not run. All codeNERD references pin the commit above; only this standalone Markdown draft was written. No claim is made that every long result is needed in full, that a particular user lost evidence, or that the issue bypasses remote permissions.

## Required outcome and architectural fit

A finite context budget and recoverable evidence are compatible requirements. The repository says evicted context must remain recoverable and that models use typed, policy-mediated operations. Its broker documentation already separates general retention from representation-specific projection and forbids hydration by rerunning the producer. The repair belongs at that projection boundary: bounded access to an already acquired immutable value, not new remote authority or a model-generated substitute for missing text. [Root guidance][guidance] [Retention/projection ownership][broker]

The existing MCP architecture explicitly documents bounded full views and five shaping axes, including string width. Those limits are useful controls to preserve. Its stronger claim that an original-payload pointer makes every cut actionable is not satisfied for a terminal string longer than the widest view. Object and array navigation can locate a leaf; it cannot divide that leaf. [Current architecture][architecture]

## Actual production chain

1. **Boot and registration.** The factory calls `initMCPBridge`; configured servers produce a bridge whose control plane is bound to `mcpctl` before asynchronous connection. The bridge constructs the plane and installs its kernel risk gate; the plane constructs a default handle store. The modular catalog includes the MCP family, whose registration installs the five verbs. VirtualStore hydration installs the catalog into both modular registries. [Factory entry][boot-entry] [Binding][boot-plane] [Bridge][bridge] [Store construction][plane-store] [Catalog][catalog] [Verbs][verbs] [Registry hydration][registration]
2. **Admitted remote execution.** `executeCall` forwards tool, arguments, view and item ceiling to `ControlPlane.Call`. It resolves the tool, validates arguments, checks risk, and calls the manager. The manager dispatches through current remote authority, not a raw transport escape hatch. [Facade][call-tool] [Control plane][call] [Manager][manager-call]
3. **Full successful result arrives.** For example, stdio's CallTool feeds the wire result decoder, which returns the complete JSON-RPC result object in `MCPCallResult.Output` when successful. The old manager-side 500 KiB cut is explicitly gone. The witness uses a small ordinary result, not a wire-limit exception or malformed envelope. [Stdio result][stdio-result] [Result decoder][wire-result] [Manager's full-output boundary][manager-call]
4. **Shape then retain.** The plane calls `DigestJSON(raw.Output, ...)`. When the digest is truncated, it mints a handle from the original `raw.Output`, includes elision metadata and suggests expansion. Retention copies the original bytes; it does not store merely the digest. [Call result construction][call] [Retention admission][retain-mint]
5. **Expand then reshape.** `executeExpand` accepts handle, pointer, view and max_items. The plane chooses a fixed view budget. HandleStore.Get returns a copy of the retained bytes; HandleStore resolves the pointer, marshals the selected value, and calls DigestJSON again. Nothing moves a starting offset forward. [Expansion tool][expand-tool] [Plane expansion][expand] [Handle projection][handles] [Retained lookup][retain-get]

Reachability is conditional on a connected, currently registered and host-reviewed remote capability, valid arguments, exact positive permission and any required human approval. Those controls still apply. Current remote authority generates a request identity when none was supplied, so the older admission audit's missing propagation baseline must not be copied into a claim that all current calls are unreachable. Conversely, a configured server alone does not establish permission. [Current identity/review checks][remote-check] [Final permission and dispatch][remote-permit]

## Static witness: a live handle whose scalar tail cannot be read

Assume an allowed read-only tool returns a successful result with this structure:

```json
{"content":[{"type":"text","text":"S"}],"isError":false}
```

Here `S` is notation for an **8,192-byte ASCII string**, not the one-character literal shown above. It contains a unique task-relevant marker after byte 4,000, no embedded JSON object, and no alternate field containing the same marker. The whole result fits comfortably within the default retention budget and even below the full view's 16,000-byte overall payload budget. No eviction, expiry or competing workspace is needed.

| Requested observation | Source-derived result |
|---|---|
| Initial `mcp_call` with compact view | The text is clipped to its first 400 bytes plus an ellipsis; a string-tail elision points at `/content/0/text`; original output is retained if admitted to the store. |
| `mcp_expand(handle=H, pointer="/content/0/text", view="full")` | The selected string is clipped to its first 4,000 bytes plus an ellipsis; 4,192 source bytes remain omitted. |
| Repeat that expansion | Same source prefix, not the next page. Success describes an obtained shaped view, not complete recovery. |
| Use `/content/0/text/4000` or another deeper pointer | Scalar-traversal error; string characters are not array elements. |
| Omit pointer, or select `/content` or `/content/0` | The same string is encountered and capped again. |
| Raise `max_items`, or ask for an unknown view such as raw | Item count cannot enlarge string width or exceed the tier ceiling; unsupported views are errors. |

The simple selected string requires no adaptive shrink: its prefix and JSON serialization fit the full byte budget. For more complicated or escape-heavy payloads, adaptive shaping can shrink further; it never supplies an offset into the omitted tail. The 4,000 figure is the maximum source-prefix length for this view, not a universal exact result size or token count. UTF-8 strings can be cut slightly earlier to preserve a code-point boundary, and the ellipsis/JSON encoding add representation bytes. [Budget and normalization][budget] [Scalar clipping][scalar] [Adaptive shaping][shrink] [Elision metadata][elision]

This witness is stronger than “full is still bounded”: with all exposed selectors exhausted, **no sequence of successful expansions of this retained value can produce the marker**. The raw bytes remain in the handle store. A debugger, a new Go API caller or an unrelated server feature could expose them, but those are not the model-facing continuation it was given.

The official MCP tool-result schema includes text content items, so `/content/0/text` is an ordinary envelope shape, not an invented malformed server response. RFC 6901 locates JSON values by object membership and array indexing; it is not a substring-range language. Rejecting traversal below a string is therefore not the operation to “fix.” The interface needs a distinct way to select within the resolved string. [MCP text results][mcp-standard] [JSON Pointer evaluation][rfc6901]

## Why the surrounding mechanisms do not close the gap

### Working-context recall retains the facade's output

The modular execution path returns `result.Result`. For these tools that string is the marshaled control-plane envelope, already containing the shaped data, handle and elision report. The common accounting path passes that output into `recordWorkingResult`, whose WorkingRecord.Body stores the supplied string. Consequently archiving and recalling this tool observation can recover the **complete envelope**, not string bytes the facade never returned. The manager's comment about whole-result working-context storage is not proof of a raw-output side channel on this route. [Envelope marshaling][marshal] [Modular return][tool-return] [Record call][record-call] [Recorded body][record-body]

### Other control-plane modes are not substring access

`mcp_map` and `mcp_probe` describe capabilities rather than retained result bytes. `mcp_context` ranks server resources and prompt templates; its query selects resource metadata, not matches within H. A server may independently expose a range-reading tool, a shorter summary, or a separately addressable resource, but that is neither guaranteed nor necessarily the same historical result.

There is also a direct sibling case: `readTopResources` concatenates resource text, clips its excerpt to the view's string budget, and retains the complete text as a **root JSON string**. Its handle consequently meets the same scalar barrier even without the `/content/0/text` wrapper. Binary content is deliberately represented by a descriptive placeholder in this path; this audit does not promise reconstruction of binary resources that were never retained as text. [Context surface][context-tool] [Resource retention][context-resource]

### Store lifetime and bounds remain real

The default store has 64 entries, an 8 MiB payload-byte ceiling and a 30-minute TTL. Mint refuses a single payload exceeding the whole byte budget. Get copies the bytes, updates LRU recency and rejects an expired entry; a read does not reset its creation-based TTL. These controls mean not every truncated call receives a usable handle forever. The witness assumes successful mint and expansion before any such loss. It isolates addressability **while retention works**, rather than disguising expiry as the new defect. [Defaults][retain-default] [Mint][retain-mint] [Get][retain-get]

Internal `HandleStore.Expand` accepts a DigestBudget, and internal Go callers can choose different budgets. The production control-plane method reconstructs one from the advertised view; no arbitrary budget is exposed through its tool schema. Adding fields to a model's input cannot create an offset that `executeExpand` never reads. [Internal API][handles] [Production budget][expand] [Exposed parameters][expand-tool]

## Impact and limits

A relevant final paragraph, diagnostic conclusion, table row encoded inside text, or document constraint can be retained yet unavailable through the prescribed continuation. Potential consequences include a correctly reported incomplete answer, extra fruitless expansion turns, a fresh remote call, or reasoning without the missing evidence. None was measured, and relevance is task-dependent: a task whose answer is in the prefix need not fail.

A fresh call is not a read of H. It can cost network/service work, return changed data, or—if the original operation was effectful—repeat an effect. The existing expansion path correctly avoids reinvocation; it does not itself duplicate any side effect. Do not weaken authorization or automatically rerun the producer to make tail recovery appear successful. Expiry advice is an existing adjacent concern, not evidence of an actual duplicate effect in this witness.

This is not a confidentiality disclosure: the proposed read must stay within the same retained artifact and applicable ownership. Nor is `Success:true, Truncated:true` inherently dishonest. The actionable mismatch is the claimed ability to read the rest, followed by next-step advice that is impossible at a full-view scalar.

## Prior work and coverage

| Existing work | What it establishes; what remains distinct |
|---|---|
| MCP architecture | Intentionally bounded full views, string caps and handle-backed elisions. It documents the mechanism but does not supply scalar ranges or identify the terminal no-progress case. [Shaping/handles][architecture] |
| Broker retention generalization | Already owns shared retention and never-rerun hydration. Reuse it; do not propose a second cache. This report fills the MCP projection's missing sub-scalar selector. [Broker contract][broker] |
| S15 truncation work | Distinguishes redeemable retained observations, process-retained history and irrecoverable cuts. Honest marking is necessary but does not itself establish redemption through this specific API. [S15 distinctions][s15] |
| MCP wire-fidelity audit | Covers protocol versions, catalog pagination and bounded transport handling, including preservation of acquired results. A complete successful wire result can still meet this later projection dead end. [Wire contract][wire-audit] |
| MCP capability-admission audit | Covers admission, authority and task identity. This witness begins after permitted success; it does not propose a weaker route around those checks. [Admission study][admission-audit] |
| Journey N20 and VirtualStore QA | N20 concerns offering MCP verbs without configured servers. The QA case concerns giant remote payload/time limits. Neither is a live, modest-size handle with an unreadable scalar suffix. [Journey][journey] [QA][qa] |

Tests already check enormous-string budget enforcement and truncation reporting; hostile expansion tests check bounded output; a retention test reaches a later small array element through a pointer. Those are useful controls, not end-to-end scalar coverage. The inspected assertions do not require every byte of a long retained string to become reachable. [String test][digest-test] [Expansion budget test][expand-test] [Small-element test][pointer-test]

Targeted searches included mcp_expand, handles.go, string_tail, scalar, offset, range and relevant audits/QA/journeys, followed by pinned reads. No exact existing scalar-tail report was located. This is a bounded novelty screen, not an exhaustive assertion about all historical discussions.

## Proposed continuation contract — not implemented

**While a handle is live and access is admitted, every retained text scalar must be recoverable through finite bounded local reads, with explicit progress and completion, without invoking its producer again.**

A minimal extension can keep the five-verb surface:

1. **Separate value location from within-value selection.** Keep RFC 6901 pointer semantics. Add optional range/cursor parameters to `mcp_expand` for a resolved text scalar; absent parameters preserve current structural expansion. Reject range selection on unsupported node types explicitly.
2. **Define the coordinate system.** One suitable design uses offsets into the decoded string's UTF-8 bytes, not offsets into escaped JSON wire syntax. Return the selected pointer, actual start/end, total source length, a continuation and an end-of-value flag. Use a clearly stated interval convention; do not mix inclusive and half-open boundaries.
3. **Guarantee progress.** A nonterminal successful page must advance its continuation. Split only at valid UTF-8 boundaries; reject or explicitly normalize a start inside a code point, and handle an impossibly small page budget without a zero-progress success. Distinguish payload content from display ellipses so concatenated source pages reconstruct the original value.
4. **Bind continuations to evidence.** A cursor identifies the retained handle, original pointer, representation/version and next position. It cannot select a different handle's data, drift to a freshly fetched value, or outlive retention silently. Duplicate reads return the same slice, supporting safe retries.
5. **Preserve budgets and truthful status.** Bound source-page bytes and the serialized response, including metadata. Escaping can make a small decoded slice large on the wire; shrink the current slice with an advancing actual boundary, not the whole result back to its first prefix. Emit explicit invalid-range, expired, unsupported-type or unavailable outcomes.
6. **Teach only possible next steps.** At a truncated scalar, return the actual continuation, not “use a deeper pointer” or “full” when already full. If retention failed, do not promise redemption. Keep schema wording, envelope fields and architecture contracts aligned.
7. **Preserve policy and ownership.** Local expansion performs no remote call and grants no new remote effect. Reuse the existing retain layer, lifecycle and owner controls. New schema fields must survive the normal registry/native/Piggyback tool path; no arbitrary shell or out-of-band file export is required.

A bounded within-string search can help locate evidence but is not a substitute for complete pageability: searches can miss unknown constraints and do not prove complete coverage. Raising the string limit, appending only a tail preview, or stripping the ellipsis changes which middle bytes are hidden without making all bytes reachable.

## Costs and fair evaluation — all unrun

Current expansion copies the entire retained payload, decodes it, resolves a value, re-encodes that value and digests it again. Repeating a no-progress expansion repeats host work and returns repeated model context. A straightforward paging implementation that reparses a B-byte payload for each of P pages can still cost O(PB), despite bounded response size. Caching decoded values or offsets can reduce repeated work but adds memory, indexing and lifetime costs; the store's payload-byte ceiling is not a bound on all transient/decoded allocations. [Lookup copy][retain-get] [Projection][handles] [Digest][digest]

Measure separate ledgers: unique source bytes disclosed, repeated bytes, serialized bytes, model tokens, local CPU/allocations, peak retained/decoded memory, elapsed latency, remote requests and remote billable work where observable. There is no measured token or reliability saving here. A repair may intentionally spend more tokens when previously unreachable evidence is actually needed; compare cost per correctly evidenced task, not smallest response.

Use identical frozen payloads and policy across baseline and proposed implementations. Start with deterministic local retention tests, then run the real registered tool and admitted remote path with a counted test server. Keep all source bytes, task questions and output budgets matched.

| Unrun case | Required oracle |
|---|---|
| 8,192-byte ASCII scalar; evidence beyond byte 4,000 | Bounded pages recover the marker and reconstruct the exact decoded string; current full expansion is the negative control. |
| Evidence at start, middle, end; no relevant evidence | Correct answers or explicit absence only after declared coverage; avoid a tail-only benchmark. |
| 119/120/121, 399/400/401 and 3,999/4,000/4,001-byte strings | Boundary behavior, small-result compatibility and meaningful truncation/continuation metadata. |
| Multibyte UTF-8, combining marks, escaped quotes/newlines, long single line | Valid source reconstruction without gaps or duplicated bytes; serialized budgets also hold. |
| Root resource string and nested text leaf; escaped pointer keys | Same range contract at both production producers; correct absolute value identity. |
| Long arrays and objects containing short strings | Existing structural navigation remains useful; range feature does not regress it. |
| Repeated, overlapping, invalid or cross-handle cursors | Idempotent valid reads; explicit validation failure; no unrelated bytes disclosed. |
| Expiry, eviction, over-budget mint, restart | Honest unavailable outcome, never automatic producer rerun. |
| Counted effectful original call, then many expansions | Exactly one original remote dispatch; every expansion local. |
| Working-context archive/recall of the facade envelope | Recall alone cannot be mistaken for recovery of the original scalar; actual continuation remains usable while live. |
| Large retained payload and many small pages | Report O(PB)-style amplification, allocation peaks and latency; compare indexed/cached alternatives fairly. |
| Denied/unreviewed remote tool | No remote dispatch; improved disclosure never weakens the original admission gate. |

A clean implementation exit requires full finite reconstruction and a tail-dependent task through the production facade, with unchanged no-reinvoke and bounded-response controls—not merely a test that the handle exists or the returned prefix fits.

## Primary references

- [RFC 6901, JSON Pointer §4][rfc6901]: identifies object/array values; it does not define substrings of JSON strings. This establishes why another selector is necessary, not permission to change pointer semantics.
- [MCP 2024-11-05 tool results][mcp-standard]: defines ordinary text content blocks and distinguishes tool execution errors. It does not require a server to provide a substring tool or guarantee that a second call reproduces the first.
- [RFC 9110 §14][range-reference]: useful design precedent for explicit range units, selected representation and complete-length metadata. The proposed local interface is not HTTP and need not copy its inclusive syntax.
- Packer et al., [*MemGPT: Towards LLMs as Operating Systems*, v2][memgpt]: studies movement between memory tiers for bounded-context models. It motivates separating storage capacity from current-window selection; it does not prove that a retained artifact is reachable or that this particular repair improves task quality.

The design inference is narrow: bounded storage plus bounded views needs a progress-capable read operation at every representation level where information is withheld. A pointer to an indivisible scalar is not that operation.

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/agents.md#L17-L24
[boot-entry]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/system/factory.go#L1872-L1887
[boot-plane]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/system/factory_mcp.go#L18-L56
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/integration.go#L108-L141
[plane-store]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/controlplane.go#L74-L99
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/core/virtual_store_tools.go#L57-L83
[catalog]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/tools/catalog/catalog.go#L33-L48
[verbs]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/tools/mcpctl/register.go#L5-L29
[call-tool]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/tools/mcpctl/tools.go#L110-L157
[expand-tool]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/tools/mcpctl/tools.go#L160-L199
[marshal]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/tools/mcpctl/mcpctl.go#L51-L62
[call]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/controlplane_invoke.go#L84-L171
[expand]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/controlplane_invoke.go#L263-L321
[budget]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/digest.go#L38-L101
[elision]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/digest.go#L103-L137
[shrink]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/digest.go#L251-L300
[digest]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/digest.go#L309-L345
[scalar]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/digest.go#L348-L384
[pointer]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/digest.go#L469-L507
[handles]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/handles.go#L87-L150
[retain-default]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/retain/retain.go#L50-L61
[retain-mint]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/retain/retain.go#L135-L193
[retain-get]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/retain/retain.go#L196-L240
[remote-check]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/remote_authority.go#L285-L332
[remote-permit]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/remote_authority.go#L490-L563
[manager-call]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/client.go#L668-L714
[stdio-result]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/transport_stdio.go#L371-L392
[wire-result]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/wire.go#L319-L350
[tool-return]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/session/executor_tools.go#L2757-L2813
[record-call]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/session/executor_tools.go#L1110-L1136
[record-body]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/session/working_context.go#L371-L404
[context-resource]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/controlplane_invoke.go#L511-L553
[context-tool]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/tools/mcpctl/tools.go#L202-L238
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/Docs/architecture/mcp/IMPLEMENTED_SPEC.md#L454-L500
[broker]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/Docs/architecture/broker/TODO.md#L224-L232
[s15]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/Docs/journeys/impl/S15-never-silently-truncate.md#L106-L118
[wire-audit]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/Docs/audits/mcp-versioned-wire-fidelity.md#L90-L114
[admission-audit]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/Docs/audits/mcp-capability-admission.md#L35-L49
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/Docs/journeys/07-external-audit-2026-09-19.md#L93-L99
[qa]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/.quality_assurance/core/2026-04-28_04-16-19-EST_virtual_store_boundary_analysis.md#L232-L235
[digest-test]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/digest_test.go#L84-L105
[expand-test]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/expand_brutal_test.go#L141-L171
[pointer-test]: https://github.com/theRebelliousNerd/codenerd/blob/715b4c931d352e98f3a9a536ffa4ccfee25f3d29/internal/mcp/handles_test.go#L11-L32
[rfc6901]: https://www.rfc-editor.org/rfc/rfc6901.html#section-4
[mcp-standard]: https://modelcontextprotocol.io/specification/2024-11-05/server/tools#tool-result
[range-reference]: https://www.rfc-editor.org/rfc/rfc9110.html#section-14
[memgpt]: https://arxiv.org/abs/2310.08560v2

