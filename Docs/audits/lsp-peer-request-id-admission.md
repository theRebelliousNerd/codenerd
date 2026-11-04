---
doc-class: deep-dive
subsystem: session-critic-lsp-request-identity
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 6d3e469d9926adb39634b02d028bb4faca2fb7a8
verification-method: static-source-inspection
main-rechecked: 2026-11-04T18:47:07Z
supersedes: []
---

# Preserve server request IDs in critic LSP exchanges

## Finding and decision

**The external language-server client drops valid server requests whose JSON-RPC ID is a string.** Its shared inbound message type declares `ID *int64`. A string ID produces a JSON decoding error, and the read loop logs that error and continues before reaching the server-request response branch. The same request with an integer ID reaches that branch, which writes a response. [Inbound type][wire-type] [Decode and dispatch][dispatch]

This can prevent an otherwise valid diagnostic session from initializing: LSP expressly permits a server to send `window/showMessageRequest` while processing initialization, and that request may carry a string ID. A peer awaiting its response can leave codeNERD waiting for the initialization result until the existing diagnostic deadline or owner cancellation ends the exchange. The caller then names the files it did not diagnose. This is **a protocol interoperability defect with conditional grounding and latency consequences**, not an observed Pyright or TypeScript incident, an infinite production hang, or a bypass of mandatory verification. [Initialization allowance][lsp-initialize] [Request ID contract][lsp-request] [Initialization caller][diagnostic-init]

Preserve the peer's ID type and value at the wire boundary, classify request versus response independently, and answer supported server requests under an explicit method contract. Keep locally generated integer IDs, current cancellation ownership, optional-server behavior and the advisory role of the critic. A protocol response must not become consent to a server-requested action.

**Evidence boundary:** public main was pinned to [6d3e469d9926adb39634b02d028bb4faca2fb7a8][pin]. This review inspected root and scoped guidance, architecture, implementation, authored tests and neighboring audits. No repository program, test, build, language server, model call, inference, training, benchmark or proposed fixture was run. No implementation, credential or setting was changed. The witnesses and evaluations below are source-derived and unrun. Whether currently deployed versions of either configured language server emit this ID form remains unverified.

## Architectural role and current status

Root guidance assigns creative synthesis to the model and executive control and evidence selection to the deterministic harness. Session guidance keeps tool authority, post-edit obligations and result-bound evidence separate. The diagnostic client is part of the host's evidence acquisition: a protocol mismatch should not be left for a reviewer model to diagnose from missing observations. [Root guidance][root] [Session guidance][session-guidance]

The current session architecture explicitly preserves optional critic grounding with a positive diagnostic budget, owner cancellation, partial findings and unchecked-file notices. Source inspection confirms that design. Older world documentation still contains historical manager/fact-projection descriptions; the current manager itself says the unused external-server portion was removed and that external servers now ground the session critic directly. This report follows that live route rather than treating the old manager description as evidence of production wiring. [Session ownership contract][owner-spec] [Current manager scope][manager-scope]

| Capability | Status at the pin | Boundary |
|---|---|---|
| Generic framed LSP transport and asynchronous reading | Implemented | `NewClient` launches a reader over an `io.ReadWriteCloser`; `StartServer` supplies a subprocess transport. [Construction][construction] [Subprocess][subprocess] |
| Correlation of the client's own integer requests | Implemented | `call` allocates increasing integer IDs and keeps integer-keyed pending channels. [Calls][call] |
| Response to server-originated requests | Partial | Integer-ID requests reach the generic null-result reply; string-ID requests fail decoding before dispatch. This does not establish full method-specific response correctness. [Dispatch][dispatch] |
| Python and JS/TS diagnostic grounding | Wired in source | Session selects Pyright or TypeScript language server for matching written files and passes returned text into the critic's grounding. [Selection][selection] [Critic consumption][critic] |
| Diagnostic timeout and explicit stop | Implemented in source, not rerun here | Request cancellation closes the transport; cleanup joins its close callback. [Ownership][owner] [Cleanup][cleanup] |
| Typed peer IDs and the paired witness below | Proposed and unverified | No implementation or passing fixture is claimed. |

The client comment describes a reusable, language-agnostic stdio client, but the current production selection is narrower. Go grounding invokes `gopls check` separately; Rust is not in the inspected external-server selection. Do not infer that every language named in that generic comment uses this client in production. [Client intent][construction] [Go path][go-path] [Selection][selection]

## The protocol contract

The LSP 3.17 base specification permits integer or string request IDs and requires a response to a processed request. JSON-RPC requires the response to preserve the request's ID value. Using integers for codeNERD's own outbound requests does not restrict the IDs a server chooses for its independently originated requests. A string containing digits remains a string; it is not the corresponding integer. [LSP request and response shapes][lsp-request] [JSON-RPC correlation][jsonrpc]

`window/showMessageRequest` is a useful narrow witness because it is explicitly allowed during initialization. Its result permits null when no action was selected, and its optional capability concerns message-action properties. The witness does not depend on unadvertised configuration, dynamic registration, workspace editing or another optional capability. [Initialization allowance][lsp-initialize] [Show-message contract][lsp-show]

These primary specifications establish the allowed exchange. They do not establish which ID strategy a particular released server uses, that a real server waits in the exact schedule below, or that a null result proves a message was displayed. The latter is a separate UI/method behavior: the numeric control here establishes response progression only.

## Source trace from acquisition to critic grounding

### A valid ID is rejected before message classification

`rpcMessage` is used for every inbound body: responses to local calls, server notifications and server-originated requests. Its ID slot is a pointer to `int64`, not a union capable of representing either allowed request-ID type. [Inbound type][wire-type]

`readLoop` first reads one complete frame, then calls `json.Unmarshal` into that structure. On any decoding error it logs `LSP: undecodable message` and immediately continues. Go's documented decoding rules return a type error when a JSON value cannot populate its target type. Therefore a well-formed string-ID request never reaches either the pending-call map or the server-request responder. The fact that other fields might have been decoded before the error does not help; dispatch is skipped for the entire message. [Read and dispatch][dispatch] [Go decoding contract][go-json]

For an ordinary integer-ID server request with a nonempty method, the response-matching branch does not apply. The final request branch writes a null-result response carrying that integer. For the string-ID version, no correlated success or error response is produced. There is no fallback string-ID handler elsewhere in this loop. [Dispatch][dispatch]

The incoming frame has already been consumed, so this witness does not require malformed framing, byte-count confusion, transport closure or loss of alignment between frames. The reader remains available for later frames. Whether lost response progression blocks anything depends on what the peer does next. [Frame reader][frame]

### Initialization can retain the pending obligation

`Initialize` calls `call("initialize", ...)`, which records its own numeric pending request and waits for either the corresponding response or context termination. The read loop can receive a server request while that wait is active. Discarding the server request does not settle the pending initialization, notify its caller of the protocol mismatch, or cause the peer to receive a refusal. [Initialization][initialize] [Call lifecycle][call]

The session begins one request context before launching the server and initializes that client before opening any file. If the initialization wait ends with an expired or canceled request context, it returns a notice covering the language's entire file list. Thus an initialization witness can lose all optional diagnostic grounding for that language while preserving a visible incomplete result. [Session initialization][diagnostic-init] [Notice construction][notice]

### A later wait can lose remaining files while retaining earlier findings

After successful initialization, the session reads and opens files sequentially, waiting for each publication before opening the next. If a server-originated request is dropped at this stage and the peer withholds a needed publication pending its answer, the wait can consume the remaining request budget. The caller retains earlier diagnostic lines and names the current and remaining files as not diagnosed. [Per-file loop][diagnostic-files] [Publication wait][wait]

The downstream critic receives the collected diagnostic and omission text in `StaticDiagnostics` and its grounding prompt, subject to the existing owner checks and review admission. A missing optional observation is not converted here into a mandatory passing gate. A reviewer may still find the defect from source or other evidence, and independent tests may detect it. No particular reviewer mistake or retained bad edit follows inevitably. [Critic consumption][critic]

## Paired causal witness

This is a proposed protocol fixture, not captured runtime output. Use a plain supported workspace path, correctly framed UTF-8 messages, an otherwise live transport, a positive request budget and no URI aliases. Let the scripted peer return diagnostics only after initialization completes. Keep content, timing, capabilities and cleanup behavior identical between the two arms.

1. The production client sends its ordinary numeric-ID `initialize` request. The peer consumes that complete frame.
2. Before replying to initialization, the peer sends this permitted request body:

   ```json
   {"jsonrpc":"2.0","id":"7","method":"window/showMessageRequest","params":{"type":3,"message":"Analysis initialization notice"}}
   ```

3. The peer waits for the response to ID `"7"`, then intends to return its normal `InitializeResult`. No selected action is required; a valid no-selection response is enough for this fixture's continuation.
4. CodeNERD's decoder cannot put the string into `*int64`. The error branch consumes and drops the request. It produces no response to `"7"`.
5. Under the stated peer schedule, initialization remains pending. When the diagnostic budget expires, the session closes the transport, returns the unchecked-file notice and never opens the files in that language session. An earlier explicit owner stop ends the request through the existing cancellation path.

**Control:** change only `"id":"7"` to `"id":7`, adjusting the frame byte count. The current responder emits a null result with numeric ID 7. The scripted peer can then return the initialization result and publish a known diagnostic after `didOpen`. This contrast isolates peer-ID type; it does not rely on a different server delay, a different code change, optional configuration negotiation or a failure of the URI helper. [Type and dispatch][dispatch] [Pending initialization][call] [Session result][diagnostic-init]

Two negative controls constrain the claim:

- If the peer completes initialization independently of the unanswered request, progression need not fail. In the post-initialization variant, the same is true if it publishes the needed diagnostics without waiting for the response. The unanswered request remains a protocol defect.
- If the peer returns an explicit initialization error promptly while the request context remains live, the current caller logs and skips optional diagnostics instead of taking the timed-out notice branch. Do not describe every initialization failure as the same result. [Initialization branches][diagnostic-init]

A second placement can send the same request after the first file has produced a diagnostic and before a second file's publication. Require the existing first finding to survive and only the current and later files to appear in the incomplete notice. The decoder cause is unchanged; this placement checks the downstream evidence boundary, not a second independent finding.

## Existing audits and overlap

The current audit inventory, queue and relevant LSP/MCP reports were screened. Exact repository searches for `window/showMessageRequest`, `server-originated` and `string-ID` returned no hits; `rpcMessage` located this client and the separate MCP implementation/report. Search is a bounded duplication screen, not proof about every historical branch or private discussion. Pinned source determines the finding.

- **LSP document URI identity and diagnostic grounding** already covers malformed/equivalent URI spellings, publication matching, version/freshness limits and diagnostic-wait error attribution. Keep those findings credited. The present primary witness occurs before any document is opened and can use perfectly ordinary URI spelling. A URI codec or publication-generation repair would leave the inbound ID type unchanged. [URI audit][prior-uri]
- **Mangle LSP header boundary** concerns the inbound editor-facing Mangle server's header reader. This report concerns the external-server client's JSON body decoding after a correct frame. A shared protocol name does not make them the same transport endpoint. [Header audit][prior-header]
- **Mangle LSP workspace selection** concerns a discarded selected root at the CLI/manager route. The present workspace can be correct and stable throughout. [Workspace audit][prior-workspace]
- **Critic diagnostic owner stop** is implemented in current source and remains a required protection. This report does not revive detached cancellation, request longer diagnostic budgets, or claim permanent blocking through the session wrapper. [Ownership contract][owner-spec] [Ownership implementation][owner]
- **MCP versioned wire fidelity** covers another protocol and separate transport implementation, including message-kind and pending-response correlation. Its integer matching is for locally issued requests; that is not evidence that independently originated LSP requests are integer-only. Do not broaden the LSP repair into an MCP rewrite. [MCP audit][prior-mcp]
- **Recurse JS comment dependency admission**, the latest preceding publication, concerns importing non-executable source text into the planning graph. No dependency scanner, graph edge or focused work selection is involved here. [Preceding audit][prior-recurse]

## Proposed correction and acceptance contracts

### Preserve direction and typed identity

Decode the outer message with an ID representation that preserves missing, integer, string and invalid states. Validate the message form before dispatch. A tagged ID type or validated raw JSON field can support the allowed forms without coercing numeric-looking strings. It need not force the local pending map to accept strings: local requests can remain integer-identified, and responses must match their actual originating type and value.

Keep these invariants explicit:

1. A server-originated request is handled as a request even when its ID has the same numeric value as a locally pending request.
2. String `"1"` cannot settle local integer request `1`. An unknown or wrong-type response cannot satisfy initialization.
3. The response to an admitted peer request preserves its ID type and value, including an empty string if that is a valid request under the selected protocol profile. It must not silently replace a valid string ID with null or an integer.
4. Missing ID means notification only for a valid notification shape. Explicit null, boolean, object, array, fractional and out-of-profile numeric forms require the appropriate validation outcome, rather than accidental coercion.
5. Duplicate terminal responses and late responses cannot revive or complete a retired local request. A transport generation must not inherit pending state from an earlier client.

These are proposed boundary requirements, not claims that all listed malformed-input cases are current defects or that a new parser already exists.

### Define method responses without granting authority

Recognizing a string ID must lead to the same method-dispatch decision as its integer counterpart. For supported informational requests, define a truthful bounded response, including how a headless client reports no selection. For unsupported methods, return the applicable protocol error with the received ID. Do not extend the current generic null response to every newly decodable request and call that full LSP support. JSON-RPC defines a method-not-found error for an unavailable method. [JSON-RPC errors][jsonrpc]

Preserve capability honesty. Do not advertise configuration, registration, workspace-editing or interactive-action support merely to make a peer continue. Do not choose an offered action, install a component, change configuration, edit a file or launch another tool because a server asked. Any effectful capability must follow the existing typed permission and user-approval path. The narrow repair needs no new model call or model-mediated protocol decision. [Session authority][session-guidance]

### Keep failures and lifecycle ownership visible

Give unsupported/invalid peer traffic a bounded, attributable disposition. If a response cannot be written, preserve that failure rather than treating an attempted write as a delivered response. Avoid blocking the only reader indefinitely while sending an answer; any worker or queue introduced for replies needs bounded ownership and must join at cancellation. The existing request-owned transport closure remains load-bearing. [Write serialization][write] [Cleanup][cleanup]

Keep present positive behavior: absent optional binaries remain nonfatal, empty matching diagnostic publications remain observations, earlier findings survive a later incomplete file, explicit stop suppresses later language/review work, and the independent positive diagnostic budget remains unchanged. More accurate protocol handling should not be presented as a reason to weaken mandatory tests or make optional diagnostics mandatory.

## Costs and failure modes

The current string-ID branch spends frame reading, body allocation, JSON decoding and logging before discarding the request. Under the awaiting-peer witness it also consumes the remaining per-language budget and keeps the server session alive until cleanup. The configured default is 90 seconds per language diagnostic request, not per file or per whole turn. Actual delay is the remaining budget and can be shortened by owner stop or peer behavior; 90 seconds is not a measured regression. [Budget][budget] [Owner][owner]

Typed peer-ID support adds validation and storage proportional to ID length, plus a correlated response's encoding and bytes. Existing messages already have whole-body storage, but retaining raw and decoded IDs can add copies. If replies are queued, account for queue memory, scheduling, synchronization and teardown. The correction should not create an unbounded goroutine per request. Frame-size and peer-request-rate limits are related hardening choices; accepting string IDs alone does not establish bounded transport memory.

There is no inherent extra language-server launch or model call for the narrow correction. Successful continuation may cause **more** useful work: opening files that were previously skipped, running analysis, adding diagnostic text to the critic prompt and possibly triggering legitimate follow-on repair. Charge that work rather than describing all increased CPU, tokens or latency as regression. Conversely, claimed savings in retries, reviewer quality or task cost require measurement; none is established here.

Shorter timeouts, ignoring all peer requests or marking optional diagnostics disabled can reduce waiting but do not provide equivalent capability. Automatically converting strings to integers risks wrong-request completion. Extending only the Go type to `any` without message validation, typed matching and correct responses is an incomplete correction. A full protocol library may reduce bespoke maintenance, but adds a dependency, behavioral migration and compatibility work; adoption is a separate design decision, not required to prove this defect.

## Unrun evaluation and completion boundary

Start with a raw-wire fixture independent of the production `rpcMessage` type. Both existing fake-server decoders use `*int64` and primarily echo the client's numeric requests. Reusing that representation to generate and consume the new fixture would reproduce the implementation's blind spot. The authored tests cover missing binaries, in-flight disconnection, ordinary Python grounding, all-file iteration, partial output and timeout notices. Their existence does not prove server-originated string requests are handled, and none was executed in this review. [Client fixtures][client-tests] [Session peer][session-peer] [Grounding controls][grounding-tests] [Partial-result control][partial-test]

| Proposed case | Required observation |
|---|---|
| Paired numeric/string request during initialization | Both valid peer IDs receive the method's defined response with preserved type/value; initialization can progress under the same peer schedule. |
| Peer request ID equal to an outstanding local numeric ID | Answer the peer request without completing the local call. |
| String response `"1"` to local request `1` | No false local completion; mismatch remains attributable. |
| Peer request after an earlier successful file | Preserve earlier findings and continue later files when the request is handled. |
| No peer requests and normal numeric replies | Existing fast path, notifications and empty diagnostic observations remain correct. |
| Empty, digit-looking, Unicode and escaped string IDs | Defined typed round trip within explicit limits; no numeric coercion. |
| Malformed envelope, invalid ID, unknown method or oversized input | Defined bounded error/termination behavior; no accidental success or effect. |
| Peer never reads the reply or disconnects while it is written | Owner stop and deadline release the request and join any reply/cleanup work. |
| Stop during initialization, after a soft caller deadline and before later language dispatch | Preserve the existing cancellation contract and unchecked-file notices. |
| Real configured Pyright and TypeScript server versions | Record observed peer requests and ID types; report unaffected numeric-only sessions as unaffected. |

Then exercise the normal session post-edit critic path with an authorized Python or TypeScript change, the real LSP client and a scripted peer/critic. Capture the resulting grounding, unchecked-file list, review invocation and cleanup. Do not inject success directly into `StaticDiagnostics` or replace mandatory gates. Separately qualify real installed server versions and operating systems. A deterministic exchange proves the adapter contract; it cannot establish real-server incidence or improved coding outcomes.

Compare baseline and candidate with the same source revisions, server behavior, request budgets, tool/permission envelope and cold/warm setup. Measure correctly correlated replies, lost requests, false response matches, files opened/diagnosed, retained partial findings, cancellation-to-return delay, server/process lifetime, host CPU, memory, bytes and end-to-end wall time. Keep model tokens, calls and actual task outcomes as a separate later evaluation. Report timeout cases, unaffected controls and additional completed analysis, rather than selecting only the broken baseline arm to advertise speedup.

**Completion requires a legal peer-selected string ID to reach the same bounded method decision as an integer ID, with exact typed correlation, no new effect authority, preserved cancellation and honest diagnostic coverage.** Priority beyond that narrow interoperability repair should follow observed supported-server traffic and measured user impact, which remain open.

## Pinned references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/6d3e469d9926adb39634b02d028bb4faca2fb7a8
[root]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/agents.md#L3-L59
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/agents.md#L3-L26
[owner-spec]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/Docs/architecture/session/03-GAP-ANALYSIS.md#L164-L193
[manager-scope]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/manager.go#L12-L19
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client.go#L20-L90
[subprocess]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client.go#L93-L136
[wire-type]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client.go#L161-L168
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client.go#L193-L234
[write]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client.go#L175-L188
[frame]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client.go#L236-L263
[call]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client.go#L266-L309
[initialize]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client.go#L319-L352
[wait]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client.go#L363-L418
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/lsp_diagnostics.go#L217-L272
[diagnostic-init]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/lsp_diagnostics.go#L275-L300
[diagnostic-files]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/lsp_diagnostics.go#L301-L348
[notice]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/lsp_diagnostics.go#L163-L181
[go-path]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/lsp_diagnostics.go#L88-L160
[critic]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/build_verify.go#L1126-L1232
[owner]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/build_verify.go#L1482-L1533
[cleanup]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/build_verify.go#L1535-L1554
[budget]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/config/session.go#L77-L100
[client-tests]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/world/lsp/client_test.go#L12-L159
[session-peer]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/lsp_servers_test.go#L23-L105
[grounding-tests]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/lsp_servers_test.go#L108-L174
[partial-test]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/internal/session/lsp_servers_test.go#L176-L217
[prior-uri]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/Docs/audits/lsp-document-uri-identity-and-diagnostic-grounding.md
[prior-header]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/Docs/audits/mangle-lsp-header-boundary.md
[prior-workspace]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/Docs/audits/mangle-lsp-workspace-selection.md
[prior-mcp]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/Docs/audits/mcp-versioned-wire-fidelity.md
[prior-recurse]: https://github.com/theRebelliousNerd/codenerd/blob/6d3e469d9926adb39634b02d028bb4faca2fb7a8/Docs/audits/recurse-js-comment-dependency-admission.md
[lsp-request]: https://github.com/microsoft/language-server-protocol/blob/440d8b03d784cf0bd3769b1abb33aacc24b586a6/_specifications/lsp/3.17/specification.md#L149-L194
[lsp-initialize]: https://github.com/microsoft/language-server-protocol/blob/440d8b03d784cf0bd3769b1abb33aacc24b586a6/_specifications/lsp/3.17/general/initialize.md#L1-L10
[lsp-show]: https://github.com/microsoft/language-server-protocol/blob/440d8b03d784cf0bd3769b1abb33aacc24b586a6/_includes/messages/3.17/showMessageRequest.md#L1-L68
[jsonrpc]: https://www.jsonrpc.org/specification#request_object
[go-json]: https://pkg.go.dev/encoding/json#Unmarshal
