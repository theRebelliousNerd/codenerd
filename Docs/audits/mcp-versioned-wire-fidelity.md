---
doc-class: deep-dive
subsystem: mcp-transport
implementation-status: partially-implemented (legacy 2024-11-05 profile built; modern profiles and SDK choice open)
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Complete versioned MCP interoperability without weakening authority

## Decision and scope

Complete **version-bound remote capability fidelity**: a configured MCP server's wire messages must become faithful capability, catalog, and operation-result observations before they influence CodeNERD's executive or learning. Transport connectivity, protocol compatibility, catalog completeness, host admission, and successful remote execution are different claims. None should stand in for another.

This section of the document was a documentation-only proposal grounded in main at `356f65e0e6bb639adaa90ab36212daa714114871`, confirmed on 2026-10-28 at approximately 06:38 UTC. No code, tests, builds, inference, remote tool executions, or benchmarks were run. Runtime consequences below are static inferences, not reproduced incidents.

Do **not** restore `transportFor`. Its [removal commit](https://github.com/theRebelliousNerd/codenerd/commit/b0f04d78bf62f1d986271e646d0edf6555a5adb5) deleted a disconnected transport lookup helper, not the configured transport path. The current manager selects HTTP, stdio, or SSE from configuration, connects, discovers, and routes calls through exact remote authority. [Configured path](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L238-L398), [guarded call](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L634-L707).

The existing [MCP capability-admission audit](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/mcp-capability-admission.md) already owns trusted review installation, parent/leaf correlation, typed authority refusals, review provenance, and honest admission status. Its new as-built section records configured tool reviews and CLI admission status as implemented. This proposal consumes that authority unchanged; it addresses the independent question: **does the external protocol observation mean what CodeNERD says it means?**

The root `Docs/audits/*.md` corpus was checked for overlapping wire/version/capability-decoding work. The MCP architecture gap document already acknowledges general protocol edge cases and records SSE lifecycle work. No inspected audit specifies the version-bound projection contract developed here. This is a bounded repository overlap check, not a claim of research novelty.

## 1. Current path and distinct unfinished behavior

The result-path consequences below are conditional on a call satisfying the existing authority boundary and reaching transport. This inspection did not establish successful configured boot execution. A separate admission-lifetime issue is visible in the inspected construction/connect sequence: the bridge installs the fact emitter and configured reviews before connection, while `Connect` subsequently clears reviews for that server. That issue belongs in the existing admission audit, not a claim that wire fixes alone restore configured execution. [Bridge installation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/integration.go#L97-L108), [review installation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L123-L132), [connection invalidation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L347-L356), [admission work](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/mcp-capability-admission.md).

### 1.1 Capability objects can become an apparently valid empty capability set

`MCPCapabilities` uses boolean fields for `tools`, `resources`, `prompts`, and `logging`. All three transports decode the `initialize` result into that type. If decoding fails, each attempts to decode the **whole result envelope** directly into the same boolean type. [Type](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/types.go#L186-L192), [HTTP](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_http.go#L178-L207), [SSE](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_sse.go#L447-L485), [stdio](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_stdio.go#L397-L463).

Even the requested `2024-11-05` revision defines server capabilities as objects, including sub-capabilities. It also requires the initialization-complete notification and agreement on a supported version. These are not obligations invented by a newer revision. [2024 lifecycle](https://modelcontextprotocol.io/specification/2024-11-05/basic/lifecycle).

For a normal envelope containing object-valued capabilities, the nested decode fails. The fallback can then ignore the outer envelope's unknown fields and succeed with all booleans false. The inferred defect is **successful empty interpretation of advertised capabilities**, not necessarily a failed connection. Indeed, configured tool discovery is controlled by `AutoDiscoverTools`; it does not require the decoded tools bit, so discovery and some calls can still work against permissive servers. A universal “MCP cannot execute” claim would be wrong. [Manager capability handling and discovery](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L285-L395).

The stdio fallback returns before sending `notifications/initialized`. HTTP and SSE have no corresponding notification in their inspected initialization path. The returned protocol version is not represented in those decoded envelopes. The HTTP fixture instead advertises booleans, as do the inspected SSE lifecycle fixtures. They establish useful local behavior but do not establish compatibility with the standard capability envelope. [HTTP fixture](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_http_test.go#L13-L59), [SSE fixtures](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_sse_test.go#L123-L169).

### 1.2 The HTTP label does not establish a Streamable HTTP contract

The HTTP implementation posts to one URL, assigns request ID `1`, sets JSON content type, and decodes one JSON response. It does not branch on response content type or verify that the response ID matches the outstanding request. Its capability request advertises `2024-11-05`. The separate SSE transport is the older GET/endpoint-event/POST pattern, not a JSON-or-SSE response implementation on that HTTP request. [HTTP request/response boundary](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_http.go#L246-L295), [SSE connection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_sse.go#L106-L190).

Existing SSE generation fencing, stream lifetime separation, and pending-call release are real implemented mechanisms. Preserve them; this proposal does not reopen the already-addressed “stream dies at the POST timeout” problem. [SSE lifecycle contract and witnesses](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/mcp/03-GAP-ANALYSIS.md).

### 1.3 Catalog and result semantics are flattened too early

The three tool-list methods parse a tools slice without following `nextCursor`; shared resource/prompt list helpers likewise parse one page. First-page success is not complete discovery. [HTTP list](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_http.go#L96-L117), [shared lists](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/resources.go#L76-L118).

All three tool-call methods mark a JSON-RPC result successful without decoding the tool result's `isError`. The control plane accepts that success bit and the manager records it in tool usage. Consequently an ordinary MCP tool execution error can be counted as success; the payload may retain the error flag, but the host summary/counter disagrees with it. [HTTP result](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_http.go#L121-L159), [SSE result](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_sse.go#L505-L537), [stdio result](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/transport_stdio.go#L370-L393), [consumer](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/controlplane_invoke.go#L123-L168), [usage](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L682-L703).

Pagination and the distinction between JSON-RPC errors and tool-level `isError` results already exist in the requested 2024 revision. [2024 tools specification](https://modelcontextprotocol.io/specification/2024-11-05/server/tools). Fixing their projection does not require adopting every new MCP feature.

## 2. A version contract, not a date-string upgrade

The official specification inspected on 2026-10-28 includes two materially different eras. Revisions through `2025-11-25` use initialization. The `2026-07-28` revision instead carries version and client capabilities per request; version incompatibility is an explicit error. A dual-era client must determine the supported behavior family rather than infer it from “HTTP.” [Versioning and compatibility](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning).

Streamable HTTP itself changed: the 2026 revision removes protocol-level sessions and the GET stream endpoint; each request can receive JSON or request-scoped SSE. A blanket plan to add `MCP-Session-Id` and always send `initialize` would implement an older compatibility path, not the current core protocol. Conversely, removing those older mechanisms indiscriminately would break legacy servers. [2026 transport](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http), [2025 transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports).

Proposed supported profiles should be explicit, for example:

- a named legacy initialization profile with its exact supported versions;
- a legacy HTTP+SSE binding where intentionally retained;
- a modern Streamable HTTP profile with only the modern features actually implemented;
- an explicitly named custom adapter if existing Boolean-capability HTTP servers must remain supported.

Do not silently preserve a nonstandard envelope by treating every decode error as an alternative protocol. Unsupported profiles should produce one actionable compatibility status, not repeated model repair attempts. A profile change changes the wire epoch and invalidates incompatible cached observations. It does not authorize additional effects.

The production choice between maintaining custom codecs and adopting an official SDK remains open. Compare supported version matrices, cancellation semantics, authentication hooks, dependency footprint, result fidelity, and migration cost before choosing. An SDK can reduce protocol maintenance but cannot replace CodeNERD's host reviews, exact Mangle permission, or catalog publication fences.

## 3. Proposed completion contracts

### A. Decode evidence first; derive behavior second

Decode each supported version into a typed wire model, validate required shape, and only then project into internal observations. Preserve object presence separately from optional sub-capabilities. Distinguish a legitimately empty capability set from unknown, malformed, and unsupported data. Unknown optional fields may remain forward-compatible; malformed required fields may not become a zero-value success.

Carry a compact observation identity: configured endpoint identity, connection/transport generation, protocol profile and version, credential-scope identity or revision without secret material, and observation time. Self-reported server software metadata is descriptive, not proof of identity; the modern discovery specification explicitly warns against using it for security decisions. [Server discovery](https://modelcontextprotocol.io/specification/2026-07-28/server/discover).

Go owns transport and deterministic parsing. Mangle should derive planning/recovery eligibility from truthful host observations; the model must not certify protocol readiness. Any new fact surface needs declared schemas, ownership, consumer routing, replacement/retraction behavior, and tests. Do not manufacture a durable `permitted` bit from a protocol-ready bit.

### B. Keep readiness dimensions separate

The proposed status is a product, not one Boolean:

`wire compatibility × capability observation × catalog coverage × host admission × exact-call outcome`.

A connected socket with malformed capabilities is not a validated capability observation. A valid first catalog page is not complete coverage. A complete catalog is not admission. An admitted tool is not permission for arbitrary arguments. A received response is not successful execution.

For legacy profiles, normal requests follow successful supported initialization and the required completion notification. For modern profiles, requests carry the selected metadata contract instead. Only capabilities actually supported by the client are advertised. Features requiring sampling, elicitation, or roots must remain explicitly unavailable unless their policy-mediated handling is implemented.

### C. Preserve catalog coverage through pagination and publication

Consume opaque continuation cursors until exhaustion or a declared resource/time limit. Track cursor repetition, page/byte counts, cancellation, and incomplete coverage. If collection stops early, preserve its known subset with an explicit incomplete state; never derive absence from it or remove the undiscovered tail as though the server withdrew those tools.

Partial and older catalogs remain coverage-labelled **descriptive evidence**, not current execution authority. Preserve their provenance, age, generation, and incomplete/offline status for reasoning or repair, but do not republish old schemas as current registration or retain old permission merely to keep a useful catalog visible. Executable metadata must independently satisfy the current connection/schema/review contract; historical visibility must never imply current invocability.

Commit a complete new snapshot through the existing connection/catalog generation fence. A reconnect, changed authorization scope, incompatible profile, or newer refresh prevents an older page sequence from publishing over current state. Resource and prompt publication already have generation mechanisms; extend or reuse them rather than invent a parallel cache authority. Staging a full catalog trades memory for coherent publication; a streaming implementation needs an equally explicit snapshot/coverage contract.

Discovery completeness is independent of prompt size. Retain the full acquired catalog where authorized, while JIT selection and progressive disclosure choose what enters a model request. Solving pagination by dumping every schema into every prompt defeats the project's intent.

### D. Normalize remote results before shaping or learning

The wire layer must distinguish a validated terminal tool result from a JSON-RPC error, malformed response, and incomplete/unsupported continuation. `isError: true` is a remote tool failure, not host success. Preserve useful error content and full original result for existing retrieval/shaping, but publish the correct outcome to summaries and usage learning.

Modern results can explicitly request additional input rather than finish the operation. Such a result is not a successful final tool result. Supporting it requires a separate, authorized continuation contract; an implementation without that feature should report it as unsupported/incomplete instead of pretending completion. [Modern tool result forms](https://modelcontextprotocol.io/specification/2026-07-28/server/tools).

Use request identities with the non-reuse scope required by the selected profile, not merely uniqueness among concurrently outstanding requests. In `2024-11-05`, a requestor cannot reuse an ID within the same session, even for sequential completed calls, and a response must carry the corresponding request ID. HTTP's constant `1` therefore violates that retained profile's identity contract independently of concurrency. [2024 message rules](https://modelcontextprotocol.io/specification/2024-11-05/basic/messages). Define the identity lifetime explicitly for modern profiles rather than importing a removed protocol-session concept. Validate version, message kind, correlation, and terminal result structure; do not accept a wrong-ID response merely because it arrived on the expected HTTP request. This wire identity complements, rather than replaces, the existing one-shot authority identity.

### E. Cancellation, recovery, and budgets are profile-specific

Do not conflate losing a transport response with proving that the remote effect did not happen. Reconnecting or probing a supported version must not automatically replay a tool effect. Use harmless discovery/protocol probes for compatibility decisions. Any subsequent effect attempt still crosses the existing exact authorization boundary.

Cancellation semantics differ by revision: the 2026 HTTP binding uses closure of the response SSE stream as its cancellation signal, unlike the older protocol's explicit cancellation/resumption behavior. Implement the chosen profile's semantics, and still report uncertain remote completion honestly. [2026 cancellation](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http#cancellation), [2025 disconnection rules](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports).

Bound response bytes, SSE event size, buffered result memory, pending requests, discovery staging, and retained diagnostics. Bounds must cause explicit incomplete/error outcomes, not cut valid JSON and report success. Preserve the existing full-result retention goal through bounded storage/spooling where appropriate rather than restoring an unmarked truncation helper. Redact credentials and sensitive remote payloads from compatibility diagnostics. Bind endpoint changes and redirects to an explicit credential-forwarding policy.

## 4. Costs and efficiency hypotheses

The expected gain is fewer false capability absences, missing tools, false successes, and model retries—not intrinsically cheaper individual HTTP requests. Full pagination and correct lifecycle handling can increase network work while restoring capability. Measure that separately from efficiency on tasks both versions can already complete.

Use separate ledgers for model tokens, host CPU, peak memory, retained storage, network requests/bytes, critical-path wall time, and remote billable units when observable. Track cold startup, warm reuse, refresh, decoding, policy dispatch, result shaping, and recovery. Do not add concurrent stage durations as task wall time or count one retry twice. Remote cost is unknown when the server does not expose it, not zero.

For a catalog of `N` items with server page capacity `p`, complete acquisition requires approximately `ceil(N/p)` list requests rather than one, plus any profile establishment. Staging costs scale with the acquired catalog bytes; prompt cost should scale with the selected disclosure, not all `N` items. Cache savings require a valid endpoint/profile/authorization-scope binding and a defined freshness rule.

Compare custom-adapter repair and SDK integration using the same supported profiles and safety policy. Include dependency maintenance, migration, protocol-fixture upkeep, and operator configuration burden. Do not claim performance savings until measured. A higher success rate obtained by weakening review, relaxing schema validation, or widening permissions is not an efficiency gain.

## 5. Proposed acceptance and evaluation

No evaluations in this section were executed. Start with standard-shaped protocol fixtures independent of the implementation's internal Boolean DTOs, then use at least one maintained independent server implementation per advertised profile. Pin server and SDK versions. Exercise the real configured bridge, review loading, manager, control plane, kernel, and model-facing result path—not only direct transport calls.

| Witness family | Required observation |
|---|---|
| Legacy capability envelopes | Object capabilities, empty objects, omitted optional fields, malformed required fields, unsupported returned version, and failed initialized notification stay distinct. No silent empty fallback. |
| Modern/legacy compatibility | Correct metadata or handshake for the selected profile; recognized modern version errors do not accidentally trigger a legacy interpretation; compatibility refusal is actionable. |
| HTTP message delivery | Both JSON and SSE response forms work where advertised; notifications do not terminate calls; sequential and concurrent requests obey profile-scoped ID non-reuse; wrong IDs, duplicate terminal responses, malformed envelopes, and unsupported result forms never become success. |
| Catalog coverage | A second-page-only target is discoverable; cursor loops, page failures, limits, and cancellation are incomplete rather than empty/complete. Old reconnect/refresh pages cannot replace a newer catalog; retained historical/partial descriptions do not confer current execution authority. |
| Result fidelity | JSON-RPC error, `isError: true`, successful result, malformed result, and input-required continuation yield matching control-plane summaries and usage counts. |
| Recovery and authority | Disconnect before response, cancellation, and compatibility retry do not duplicate side effects. Every actual invocation still requires current schema-bound review and exact permission; the counted endpoint sees zero effects on denial. |
| Resource discipline | Large JSON bodies, multi-line SSE events, concurrent calls, repeated reconnect, and slow servers remain within explicit bounds with truthful outcomes and no retained stale pending requests. |

Report compatibility success by profile, catalog recall/coverage, false-success count, false capability absence, duplicate effects, unauthorized invocations, task completion, extra model turns, network requests/bytes, CPU, memory/storage, and p50/p95 end-to-end latency. Use matched tasks, model, tool universe, credentials, permission sets, and fault schedules.

Separate three claims: (1) corrected wire behavior under controlled fixtures, (2) interoperability with independent implementations, and (3) improved task-level capability or efficiency. Passing one does not establish the others. The initial release gate should require zero falsely complete catalogs, wrong-request successes, and unauthorized/duplicate effects in the declared adversarial matrix; any unimplemented profile remains visibly unsupported.

## 6. Completion sequence

1. Specify the supported protocol/profile matrix and wire-to-observation schemas in the MCP architecture corpus.
2. Correct capability, catalog, and terminal-result interpretation for the retained legacy profile; remove the permissive fallback or confine it to an explicitly selected custom adapter.
3. Choose an SDK or equivalent codec/binding implementation for additional profiles, preserving existing authority and lifecycle fences.
4. Add configured end-to-end witnesses and independent-server interoperability evidence.
5. Update current-state claims, public configuration guidance, and readiness reporting only for verified profiles.

The milestone is not “all MCP versions supported.” It is a declared set of remote capabilities whose availability, completeness, execution outcome, and resource cost survive the wire without changing the user's authority.


---

## As-built status (2026-10-28, branch `feat/conversation-brief`)

Every claim in section 1 was re-checked against the working tree before building. The legacy `2024-11-05` profile of sections 3.A-3.E is built; the modern profiles (section 2) and the SDK decision (section 6, steps 3-4) are deliberately left open.

### Claim verification

| Claim | Verdict | Evidence (pre-change source) |
|---|---|---|
| Capabilities decode falls back to the whole envelope and yields all-false | **Real, all three transports** | `transport_http.go:200`, `transport_sse.go:478`, `transport_stdio.go:435`: `var simple MCPCapabilities` decoded from the whole result. Object-valued `{"tools":{}}` fails the first decode (`types.go:187` is four `bool`s) and the second silently succeeds. |
| `notifications/initialized` missing | **Real** | Absent from HTTP and SSE entirely; stdio sent it only on its happy path (`transport_stdio.go:449-454`), after the fallback return at `:435`. |
| HTTP request ID is constant `1`, response ID unchecked, content type unbranched | **Real** | `transport_http.go:256` (`ID: 1`); the decode at `callLocked` never compared `resp.ID`; it always `json.Decode`d the body. SSE's counter never reused IDs but also never separated responses from server requests that carried the same ID (`handleEvent` decoded every message as `mcpResponse`). |
| Pagination ignored | **Real** | `ListTools` (three transports) and `resources.go:82,113` decoded one page; `nextCursor` was never read. |
| `isError` ignored, counted as success | **Real, three transports** | `transport_http.go:156`, `transport_sse.go:534`, `transport_stdio.go:390` set `Success: true` for any JSON-RPC result; `client.go` then passed `result.Success` to `RecordToolUsage`. |
| Configured reviews cleared by `Connect` | **Real, and trivially fixable, so fixed here** | `integration.go:108` installs the emitter (and configured reviews) before connect; `client.go:349-355` then deleted every review bound to the server and nothing reinstalled the configured ones. The doc assigned this to the admission audit; the fix is four lines and shares the code path, so it is built. |
| Restore `transportFor` | **Correctly rejected** (not built) | The configured path in `client.go` `Connect` is the only transport selection. |
| SSE lifecycle (generation fencing, stream lifetime) already implemented | **Confirmed, preserved** | The generation/`connectionDone` fencing in `transport_sse.go` is untouched. |

Two defects the doc did not list were found while building and fixed: the old SSE parser dropped the first event when it had no `event:` line (it carried the empty type) and required `data: ` with a space; and the stdio reader could end (EOF, or a line over the 64 KiB `bufio.Scanner` default) without waking pending calls, which then hung until their context expired.

### What was built

- **`internal/mcp/wire.go`** (new): typed wire boundary.
  - `parseInitializeResult`: requires a `protocolVersion` in the supported set, a `capabilities` object, and object values for `tools`/`resources`/`prompts`/`logging` (an empty object still declares the capability; unknown capabilities are forward-compatible). No fallback. `UnsupportedProtocolError` for another version. The legacy boolean form was **removed**, not adapted: no in-repo server or fixture needs it, and the doc's "named custom adapter" option has no consumer.
  - `rpcMessage` classification (response / request / notification / invalid), `responseFor` (matching integer ID, `jsonrpc: "2.0"`, no batches), `intID`.
  - `callResultFromRPC`: JSON-RPC error, `isError`, malformed result, non-array `content` and non-boolean `isError` projected into `MCPCallResult` (new `ToolError` flag; full remote result kept in `Output`). All three transports use it.
  - `listPaged`: cursor-following collection with repeated-cursor, page (1000), item (200000) and cancellation handling; `IncompleteCatalogError` carries the subset.
  - `scanSSE`: SSE field grammar shared by the SSE transport and the HTTP event-stream response form.
  - Bounds: `maxWireMessageBytes` (64 MiB) on HTTP bodies, SSE events and stdio lines; diagnostics quoting remote bodies are redacted and capped at 512 bytes.
- **`transport_http.go`**: `initialize` -> validate -> `notifications/initialized`, then connected; atomic non-reused request IDs; response ID validation; `application/json` or request-scoped `text/event-stream` responses, anything else refused; `Accept` header; cross-host redirect refused (`refuseCrossHostRedirect`) so static credential headers stay bound to the configured host (also on both SSE clients).
- **`transport_sse.go`**: handshake with the notification; server requests/notifications never delivered to pending calls; a completed call's pending entry is removed so a duplicate terminal response is dropped; `closeCause` surfaces why a stream died; any 2xx acknowledges a POST.
- **`transport_stdio.go`**: Connect now performs the handshake (previously deferred to `GetCapabilities`); reader exit tears the session down (`teardownLocked`/`abort`) and wakes pending calls with the cause; `done` is recreated per connection (the old code panicked on a second `Disconnect` after reconnect).
- **`resources.go`**: list helpers delegate to `listPaged`; `resources/read` and `prompts/get` require their `contents`/`messages` arrays.
- **`client.go`, `remote_authority.go`**: `Connect` fails on missing capabilities instead of warning; configured reviews reinstalled after the connect-time clear (`installConfiguredReviewsForServerLocked`); `DiscoverTools` / `DiscoverResources` / `DiscoverPrompts` publish a partial subset, record coverage and return the `IncompleteCatalogError`; `beginRemoteToolRefresh(..., complete bool)` leaves unlisted tools registered when the listing was partial (a listed tool whose schema changed is still revoked).
- **`catalog_coverage.go`** (new): `CatalogCoverage` record and `MCPClientManager.IncompleteCatalogs`. Production consumer: `ControlPlane.Atlas` adds `incomplete_catalogs` to each server row and a "counts are a floor" note to its summary, and `catalogFor` keeps a partial resource/prompt subset with its error.
- Specs updated: `Docs/architecture/mcp/IMPLEMENTED_SPEC.md` (new 4.1a, 4.2-4.4), `06-PUBLIC-API-AND-TYPES.md`.

### Tests

New standard-shaped fixtures (hand-written JSON from the specification, independent of the package DTOs): `wire_fixtures_test.go`, `wire_fidelity_test.go`, `wire_transports_test.go`, plus one case in the external `mcp_client_integration_test.go` suite. Existing fixtures (`internal/tools/mcpctl/authority_test.go`, `fake_server_test.go`, `transport_http_test.go`, `transport_sse_test.go`, `catalog_generation_test.go`, `controlplane_test.go`, `mcp_client_integration_test.go`) were moved from boolean capabilities and string `content` to conforming envelopes, and now assert the initialized notification's position.

| Defect | Tests |
|---|---|
| Object capabilities read as empty | `TestHTTPInitialize_ObjectCapabilitiesAreObserved`, `..._EmptyObjectCapabilityIsDeclared`, `..._ServerWithNoCapabilitiesIsLegitimatelyEmpty`, `TestSSEInitialize_ObjectCapabilitiesAndInitializedNotification`, `TestStdioHandshakeObservesObjectCapabilitiesAndNotifies` |
| Malformed / legacy envelopes accepted | `TestHTTPInitialize_MalformedEnvelopesAreRefusedNotEmptied` (8 shapes), `TestSSEInitialize_BooleanCapabilityFormIsRefused`, `TestStdioBooleanCapabilityFormIsRefusedAtConnect`, `TestManagerConnectRefusesAMalformedHandshake` |
| Unsupported version | `TestHTTPInitialize_UnsupportedVersionIsOneActionableError` |
| Initialized notification | `TestHTTPInitialize_SendsInitializedNotificationBeforeAnyRequest`, `..._FailedInitializedNotificationFailsConnect`, SSE and stdio handshake tests |
| Constant / reused IDs; wrong-ID success | `TestHTTPRequestIDsAreNeverReused`, `TestSSERequestIDsAreNeverReusedAcrossReconnect`, `TestHTTPResponseWithWrongIDIsNeverSuccess`, `TestHTTPEnvelopeFaultsAreNotSuccess` (8 malformed bodies) |
| Response forms | `TestHTTPResponseMayBeARequestScopedEventStream`, `TestHTTPEventStreamFaultsAreNotSuccess`, `TestHTTPUnsupportedContentTypeIsRefused` |
| Server request delivered as response; duplicate terminal | `TestSSEServerRequestWithCollidingIDIsNotDeliveredAsAResponse`, `TestSSEDuplicateTerminalResponseIsIgnored`, `TestStdioIsErrorResultIsAToolFailureAndServerRequestsAreNotAnswers` |
| SSE parsing | `TestSSEEventWithoutTypeLineIsAMessage`, `TestScanSSEFollowsTheFieldGrammar`, `TestScanSSERefusesOversizedInput`, `TestMessageKindsAreClassifiedByShape` |
| Pagination | `TestToolListFollowsNextCursorToExhaustion` (target on last page), `TestResourceAndPromptListsFollowNextCursor`, `TestRepeatedCursorIsIncompleteNotComplete`, `TestLaterPageFailureKeepsEarlierPagesAsIncomplete`, `TestFirstPageFailureIsAnOrdinaryError`, `TestMalformedCatalogPagesAreRefusedNotEmptied`, `TestCancelledCollectionIsIncomplete` |
| Partial listing withdrew tools | `TestPartialDiscoveryDoesNotWithdrawUnlistedTools` (also asserts the Atlas gap and recovery), `TestPartialDiscoveryStillRevokesChangedListedTools` |
| `isError` counted as success | `TestToolResultOutcomeFollowsTheWire` (9 shapes), `TestJSONRPCErrorIsAFailureNotAToolError`, `TestSSEIsErrorResultIsAToolFailure`, suite test `TestCallTool_RemoteIsErrorIsFailureInResultAndUsage` (persisted usage: 1 use, 0 successes) |
| Bounds | `TestHTTPOversizedResponseIsAnErrorNotACut`, `TestSSEOversizedEventEndsTheStreamWithItsCause`, `TestStdioOversizedMessageFailsPendingCallsWithItsCause`, `TestStdioServerExitWakesPendingCalls`, `TestHTTPErrorBodyQuotedIntoErrorsIsBounded` |
| Credential forwarding | `TestHTTPRefusesCrossHostRedirectThatWouldForwardCredentials` |
| Reviews cleared by Connect | `TestConfiguredReviewsSurviveConnect` (also asserts a runtime review does not survive a reconnect) |
| Stdio reconnect | `TestStdioReconnectAfterDisconnectIsClean` |

### Deliberately left open

- **Modern profiles (2025 session lifecycle, 2026 per-request metadata and Streamable HTTP cancellation, input-required tool results).** Each is a different lifecycle and binding; implementing one correctly needs an independent server to test against, which this change did not have. The refusal is explicit instead: a server that selects another version fails Connect with `UnsupportedProtocolError` naming the supported version.
- **SDK versus custom codecs (section 6, step 3)** and **independent-server interoperability evidence (step 4).** Neither can be settled by fixtures written by the same author; both remain a decision and an evaluation.
- **Observation identity** (endpoint, generation, profile, credential-scope revision, observation time) beyond `ProtocolVersion`/`ServerName`/`ServerVersion` on `MCPCapabilities`, and the Mangle surface for it. A new fact surface needs declarations, ownership and retraction semantics; no consumer exists yet, so none was added.
- **Cancellation semantics per profile and replay safety** are unchanged: the legacy profile sends no `notifications/cancelled`; a lost response is still reported as an error, never as proof the effect did not happen, and nothing retries a tool call.
- **Server-to-client requests** (`ping`, sampling, roots) are logged and not answered on stdio/SSE; the client declares no capabilities, so a conforming server should not send them. Replying `-32601` is a possible follow-up.
- **Streaming spooling of oversized results.** The 64 MiB bound produces an explicit error; spooling larger results to bounded storage was not built.
- **Efficiency/interop measurements** (section 5 reports) were not run.

Verification: `go test -race -count=1 ./internal/mcp/` and `go test ./internal/tools/mcpctl/` pass; `go vet` clean for `internal/mcp`, `mcpctl`, `campaign`, `config`; `internal/system` and `cmd/nerd` build. Runs were done in a scratch copy of HEAD plus these files because `internal/store` in the worktree was mid-edit by another lane and did not compile at the time.
