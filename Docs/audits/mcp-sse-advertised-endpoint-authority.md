---
doc-class: deep-dive
subsystem: mcp
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 4f762251d3db6363c354006ace011931cd42c2dc
verification-method: static-source-inspection
supersedes: []
---

# SSE advertised endpoints escape configured destination authority

## Finding and decision

**The legacy SSE transport can send configured headers to a server-advertised destination outside the configured origin before connection initialization succeeds.** The first `endpoint` event becomes `postURL` without a destination check. `post` resolves that value, attaches the configured headers, and sends directly to the resulting URL. An absolute URL supplies its own destination. The existing cross-host redirect guard only runs on HTTP redirects, so it does not mediate this first POST. [Endpoint acceptance][endpoint] [POST construction][post] [URL resolution][resolve] [Redirect guard][redirect]

This path is reachable during enabled auto-connect at ordinary system boot. A remote tool does not need to be selected, reviewed or permitted: initialization itself produces the first POST. A subsequent handshake error does not reverse a request already received at the advertised destination. The source establishes the dispatch path; actual delivery still depends on routing, TLS, cancellation and the recipient. No live transmission or exploitation was attempted. [Boot][factory] [Auto-connect][auto] [Transport selection][select] [Handshake][initialize]

**Recommended contract:** resolve and admit the advertised endpoint before publishing it as ready or attaching headers. Default to the configured origin. If a deployment intentionally splits its SSE and POST services, require an explicit destination grant and a separate decision about which configured credentials may be sent there. Use the same admitted authority for subsequent redirects. A remote endpoint event must not grant itself access to another destination.

This is a concrete residual of [MCP wire fidelity][prior], whose proposal already requests credential-forwarding policy for endpoint changes and redirects. Its implementation account records the redirect guard on both SSE HTTP clients. The missing endpoint-event path is distinct from that implemented redirect protection; the general requirement is not new. This report supplies the current causal witness, production boundary and acceptance contract for closing it.

## Evidence scope and current status

Public main was inspected at [`4f762251d3db6363c354006ace011931cd42c2dc`][pin], observed November 4, 2026 at 14:38 UTC and rechecked unchanged at 14:48 UTC. Every CodeNERD reference below pins that revision. Root guidance, architecture guidance, MCP architecture, current transport and configuration code, relevant authored tests, existing audits and targeted repository search results were read. No repository program, test, build, benchmark, model inference, server fixture or credential operation was run. Evaluations below are proposed and unrun.

[Root guidance][root] puts executive authority in deterministic machinery and requires production-path evidence. [Architecture guidance][doc-guidance] separates the intended contract from implemented capability. The older MCP corpus contains dated inventory and status claims, so current implementation claims here come from pinned source. Its safety division still usefully places global egress policy with the host/policy/operator and actual transport behavior with the MCP adapter. This proposal preserves that division: configured destination authority is enforced at the adapter, while deployment-wide IP/network restrictions remain additional controls. [Safety ownership][safety]

| Capability | Status at the pin | Evidence and limit |
|---|---|---|
| Configured headers reach HTTP and SSE | Implemented | Config conversion preserves `Headers`; construction expands environment references. This audit used no real values. [Fields][config-fields] [Conversion][config-map] [Expansion][headers] |
| Cross-host HTTP redirect refusal | Implemented | Both SSE clients install the guard. It compares a redirect target with the first request in that redirect chain. [Constructor][constructor] [Guard][redirect] |
| SSE endpoint generation ownership | Implemented | Only the first endpoint for the active generation is installed; reconnect resets endpoint state. This prevents a later event from replacing the same generation's endpoint. [Reset][reset] [Acceptance][endpoint] |
| Advertised destination admission | Missing on the inspected path | Resolution performs no origin/credential check before the POST. [Resolve][resolve] [Post][post] |
| Reviewed remote tool authority | Implemented on its own boundary | Tool effects require current review, schema and positive kernel permission. That does not interpose on connection initialization. [Admission][authority-entry] [Final dispatch][authority-final] |
| Proposed destination and credential grants | Not implemented here | Names and shapes in this report are design requirements, not existing configuration keys or Mangle predicates. |
| Incident frequency, concrete compromise and repair performance | Unverified | No runtime experiment or deployment inspection was performed. |

## How the destination changes

### Configuration and ordinary boot

`MCPServerIntegration` accepts `protocol`, `base_url`, enabled/auto-connect flags and a header map. `ToMCPServerConfigs` keeps enabled entries and carries `BaseURL`, `Headers` and those flags to the manager. In the normal system factory, `initMCPBridge` builds that bridge and starts `ConnectAll` in a background goroutine. The bridge delegates to the manager, whose enabled auto-connect entries invoke `Connect`. The `sse` branch constructs `NewSSETransportWithHeaders(cfg.BaseURL, timeout, cfg.Headers)` and immediately connects. These are current production calls, not a hypothetical direct constructor route. [Config][config-fields] [Conversion][config-map] [Factory entry][factory-entry] [Factory bridge][factory] [Bridge][bridge] [Manager][auto] [Selection][select]

The constructor expands header values once and retains the resulting map. Headers whose expanded, trimmed values are empty are dropped; a nonempty configured API-key or authorization header remains available for both the stream GET and message POSTs. The stream GET targets `baseURL` and attaches the map. Configuring no credentials reduces the credential aspect, but does not establish permission for an arbitrary destination to receive the JSON-RPC body. [Expansion][headers] [Constructor][constructor] [Stream request][get]

### Server data becomes the first POST target

The implemented wire profile is the 2024-11-05 initialization lifecycle. Its official SSE specification requires a server-advertised POST endpoint, which explains this two-channel design. It does not make a remote event an operator grant to disclose configured headers to another origin. Legitimate split-host deployments therefore need an explicit compatibility path rather than an assertion that absolute endpoint URIs violate MCP. [Implemented profile][wire-limit] [Legacy transport specification][legacy-spec]

After the stream starts, `handleEvent` receives an `endpoint` event. Under the generation lock it stores the event's data, sets `endpointReady`, and closes `initSignal`. Neither URL parsing nor destination comparison occurs there. `Connect` wakes and calls `initialize`; that calls `call("initialize", initializeParams())`. The request snapshots `postURL` and reaches `post` before waiting for the response on the SSE stream. [Event][endpoint] [Connect ordering][connect-init] [Initialize][initialize] [Call][call]

`resolveURL` parses the configured base and advertised reference, then returns `base.ResolveReference(ref).String()`. Go documents that an absolute reference ignores the base. Scheme-relative references can also replace the authority while retaining the scheme. This behavior is ordinary URI resolution, not evidence of authorization. RFC 3986 defines resolution; it does not decide who may receive credentials. [Resolver][resolve] [Go URL contract][go-url] [URI resolution][rfc3986]

The resulting URL is passed into `http.NewRequestWithContext`. The code then applies every retained configured header and calls `t.client.Do`. This direct request is not an HTTP redirect from the stream GET. It has its own request chain, and Go's `CheckRedirect` hook is invoked before following a redirect, not before the first request. Go's redirect-sensitive handling of headers such as `Authorization` is likewise not an origin check on this freshly constructed POST. [POST][post] [Go client contract][go-http]

The redirect guard remains useful: it rejects a later hop whose `URL.Host` differs from the first request in that chain. For the advertised POST chain, however, that first request already targets the advertised destination. Installing the hook on both clients cannot connect these two independent chains. The current guard compares host, not the complete scheme/host/port origin; the proposed invariant should not inherit that narrower comparison as a claim of HTTPS downgrade protection. [Guard][redirect]

### Per-tool permission does not repair initialization

The minimal witness ends at the initialization POST. It does not call `tools/call`, and needs no tool catalog, schema review, model decision or permission bypass.

For later calls, a successfully connected server record retains `Endpoint: cfg.BaseURL`. The remote-authority subject and final registration check use that record, while SSE sends through its private advertised `postURL`. A later permitted operation can therefore carry the same destination discrepancy, provided the normal handshake, discovery and authority requirements have all succeeded. Those requirements must remain; this report does not establish that an unreviewed tool executes or that a denied call reaches a server. [Server record][server-record] [Approval subject][authority-subject] [Final checks][authority-final]

## Minimal causal witness

Use two isolated, counted test services, A and B. These are proposed local fixtures with synthetic headers, not instructions to contact a real third-party or internal service.

Preconditions:

1. A is the configured SSE origin. The transport has a nonempty synthetic header such as `X-Audit-Canary: synthetic-only`.
2. B has a different origin, is reachable by the fixture client, and is not an explicitly admitted POST destination.
3. A responds to the GET with HTTP 200 and a valid SSE `endpoint` event containing B's absolute URL. A keeps the stream open until the client finishes.
4. The test caller's context remains live through the first POST. For a TLS fixture, ordinary certificate validation succeeds using fixture-only trust.

Source-predicted sequence:

| Step | Observation to collect | Why it follows |
|---|---|---|
| GET A | A sees the synthetic header | The configured header map is attached to the stream request. |
| Endpoint event | B's URL becomes `postURL` | First active-generation event is installed without destination admission. |
| Initialize | A JSON-RPC initialization body is constructed | Connection performs the handshake before reporting connected. |
| First POST B | B sees the synthetic header and initialization body | Absolute resolution preserves B; headers are attached directly before `Do`. |
| B returns an error | `Connect` fails, but B's request count remains one | A later status/handshake error cannot undo received bytes. |

Have B return a prompt non-2xx response for the minimal case. Then no matching JSON-RPC response, valid server capability, or completed handshake is needed to establish the outbound event. Capture the request at B before asserting the expected connection error. The decisive oracle is the recipient count/header, not whether `Connect` eventually succeeds. Source predicts attempted dispatch; the controlled fixture would establish delivery. It has not been executed. [POST status ordering][post] [Handshake failure cleanup][connect-init]

Negative controls should change one variable at a time: advertise a relative same-origin path; advertise an absolute same-origin path; remove the header; or block B at the fixture network boundary. The no-header case must still record destination and body separately. A failed dial or canceled context must not be reported as credential delivery.

### Consequences and limits

A already receives the same configured header on the initial GET. A malicious A therefore does not learn a previously unknown credential by choosing B. The additional boundary failure is that the client itself can transmit that credential and its protocol body to B using the client's network position, without a separate destination decision. Misconfigured split-host or plaintext endpoints can also cause unwanted forwarding without a malicious server. [Stream headers][get] [POST headers][post]

An HTTPS-to-HTTP advertised URL can move the POST onto plaintext if the network permits it. A loopback or private-network advertised address can cause an HTTP POST from the client's environment if reachable. The method and body remain those constructed by the MCP client; this is not arbitrary socket access or arbitrary-body execution. No internal response exfiltration, metadata credential retrieval, privileged service effect, actual compromise, or unlimited network reach is established. Standard TLS checks, egress controls, unsupported schemes, unavailable recipients and cancellation can stop transmission. Same-origin enforcement alone would not solve DNS rebinding or a compromised permitted service.

## Relationship to earlier work

The predecessor's [forwarding requirement][prior-policy] already covers the desired high-level principle. Its [shipped account][prior-shipped] records cross-host redirects as refused on both SSE clients. This report narrows the remaining distance to **the server-advertised first POST**, with a witness that never receives an HTTP redirect. It does not reopen working response-ID validation, protocol negotiation, incoming wire bounds, or stream-disconnect handling.

An older QA discussion considers a configuration explicitly pointing at an internal address. That is a different source of authority: here the operator configures A, and A's response introduces B. The proposed contract must continue to support intentionally configured local MCP servers. A blanket private-address ban would hide this distinction and break supported deployments. [Older endpoint concern][old-qa]

The [capability admission audit][capability] concerns review and effect authority after connection; the [stdio cancellation audit][stdio] concerns a blocked outbound pipe writer; the [retained scalar audit][scalar] concerns local continuation through a bounded result view. None is the cause of the advertised-destination witness.

Targeted searches covered `resolveURL`, `postURL`, `cross-host`, `SSRF`, endpoint/redirect and credential/SSE terms, alongside the audit inventory and current MCP tests. No existing concrete report of this endpoint-event path was found in that bounded screen. A separate recursion screen found only overlap or closed candidates in settlement, recovery and mission finalization; no additional recursion report is bundled here.

## Proposed contract and implementation boundaries

### Admit a destination before disclosing anything to it

For each connection generation, retain a validated configured origin, the admitted effective POST endpoint, and the applicable credential-scope identity. Origin should include scheme, canonical host and effective port; do not authorize by URL string prefix, suffix, or hostname alone. RFC 6454 provides the origin comparison model. This use is a proposed client policy, not a claim that the legacy MCP specification mandates same-origin POST endpoints. [Origin model][origin]

Resolve relative references with the standard URL library, preserving legitimate paths and query parameters. Then validate the resolved result before setting endpoint readiness or building a credential-bearing request. Refuse malformed, unsupported, opaque or userinfo-bearing endpoints and HTTPS downgrade under the default policy. Same-origin relative and absolute URLs should both remain usable. An invalid first event must terminate that generation with an explicit reason, cancel its stream and release waiters; do not mark it ready, make a probe request, or silently fall back to another destination.

Use a parsed, validated endpoint value tied to the connection generation. Do not discard the admitted value and reconstruct an unchecked destination from raw server text later. Apply the policy to the initial stream request, the advertised POST and every followed redirect against the original admitted authority. Retain existing generation fences and cancellation behavior. Any same-origin path redirection must also have a documented URI-resolution base; changing from configured URI to effective redirected URI is a separate compatibility decision to test.

### Keep destinations and credential grants explicit

A split-host deployment may legitimately need B. Support it only through an operator-authorized destination set, if that capability is chosen. Permission to send a protocol body to B and permission to forward A's credential to B must be distinguishable. Assign credentials by admitted destination/scope, rather than copying the entire original map automatically. Removing only `Authorization` leaves custom API-key and tenant headers; removing all headers still sends a potentially sensitive tool body to B. Rejecting the unadmitted destination is the safe default.

The host adapter owns canonicalization, transport enforcement and publication of an immutable admitted identity. Existing Mangle policy continues to own tool-effect authorization, with the effective endpoint/scope available where needed for exact approval binding. Remote text and model prose must not create an exception. If new facts are introduced, specify their declarations, generation ownership, retraction and fail-closed consumers before claiming integration.

Keep secrets and full query tokens out of logs and facts. A refusal can carry a bounded reason and sanitized origin identity. A configured local HTTP origin remains a legitimate explicit configuration; do not turn this focused repair into a universal network allowlist or disable ordinary TLS checks to make a fixture pass. Deployment egress controls remain useful defense in depth.

### Preserve failure and recovery semantics

A destination rejection happens before the first disallowed request, so it can truthfully report zero dispatch to that destination. A timeout or lost response after an allowed dispatch has a different, possibly uncertain outcome and must not be relabeled as a pre-dispatch refusal. Do not automatically retry a tool effect at another host or transport to compensate for rejection.

After a rejected generation is closed, a later deliberate reconnect may read a fresh endpoint and must revalidate it. Old-generation endpoint events cannot restore authority, and a valid same-origin reconnect must not inherit a permanent failed-readiness latch. Retain the existing reset, first-endpoint and stream cleanup protections. [Current reset][reset] [Current finish][finish]

Saltzer and Schroeder's complete-mediation principle motivates placing enforcement at every destination-bearing request boundary rather than relying on one redirect callback. The paper supplies a design principle, not evidence that CodeNERD has been exploited. [Protection principles][protection]

## Costs and alternatives

Let `u` be the endpoint length, `h` total configured header bytes, `k` explicit destination grants and `r` permitted redirects. URL parsing/canonicalization costs scale with `u`; a simple grant scan adds up to `O(k)` comparisons. The current POST already parses both base and reference and copies header values, so caching an immutable admitted endpoint can avoid some repeated parsing. Per-request header selection still processes relevant headers; no measured speedup follows from these bounds.

Storage adds parsed endpoint/origin and scope metadata per live generation, plus any `k` grants. No model call, embedding, training step or extra network round trip is necessary for the default same-origin decision. Redirects need the same check on each hop, up to the existing bound. Preserve limits on raw endpoint/event size; a validation step should not create unbounded copies or diagnostic strings from the wire's large event allowance. [Existing redirect bound][redirect] [Wire limit][wire-limit]

The main cost is compatibility and configuration complexity. Deployments that currently split GET/POST origins would fail until intentionally authorized; users need a clear refusal and migration guidance. Host canonicalization, IDNA, effective ports, IPv6 and proxy behavior require precise tests. Adding DNS/IP screening would introduce separate resolution latency, caching, rebinding and local-development policy costs and should not be hidden in a claim that origin comparison solves all SSRF.

Alternatives:

- **Only strengthen the redirect hook:** preserves existing behavior but leaves the first advertised POST unmediated.
- **Strip recognized secret headers on cross-origin POST:** incomplete for custom headers and bodies; also silently breaks authentication.
- **Require relative endpoint syntax only:** narrower, but rejects safe absolute same-origin deployments and can mishandle scheme-relative references if checked superficially.
- **Reject cross-origin after successful initialization:** too late for initialization headers and bytes.
- **Disable legacy SSE entirely:** removes this route but discards a supported protocol rather than fixing its destination contract. It needs a separate compatibility decision.
- **Enforce only an external egress proxy:** useful for network reach, but it may not know this connection's per-origin credential and body-disclosure contract.

No token savings or end-to-end task-quality gain is claimed. The primary outcome is preventing unauthorized destination dispatch while preserving legitimate connection behavior. Operational costs to measure include startup success, rejected configurations, recovery latency, resource cleanup and false-positive refusal rates.

## Unrun evaluation plan

The existing `TestSSEResolveURL` expressly expects an absolute other-host URL to remain absolute. That is a correct generic resolver assertion, but not an admission test. `TestHTTPRefusesCrossHostRedirectThatWouldForwardCredentials` uses an actual HTTP redirect and verifies no request reaches its target. The SSE fixture advertises a relative `/rpc/<session>` path and exercises lifecycle behavior. Preserve these useful controls while adding destination admission at the actual sender boundary. All these test bodies were read, not executed here. [Resolver fixture][resolver-test] [Redirect fixture][redirect-test] [SSE fixture][sse-test]

| Designed case | Required observation after the proposed repair |
|---|---|
| Absolute different-origin endpoint, nonempty synthetic headers | B receives zero requests; connection fails before initialization dispatch to B. |
| Same hostname, different effective port | Refused without an explicit grant; host text alone cannot authorize it. |
| Scheme-relative different-origin endpoint | Resolved first, then refused before B is contacted. |
| HTTPS stream advertises HTTP POST | Refused under the default policy, including otherwise matching host/port. |
| Relative path and absolute same-origin URL | Handshake and notifications complete normally; query/session tokens are preserved without logging them. |
| Different-origin endpoint with no configured headers | Destination policy still applies; body-only leakage is not accepted as success. |
| Explicit split-host destination without credential sharing | Only destination-scoped credentials reach B; A's canary does not. This case requires the proposed configuration capability. |
| Explicit split-host destination with a bounded credential grant | B receives only approved headers and protocol body; an additional destination C remains blocked. |
| Redirect from an admitted request to an unadmitted destination | Zero requests at the redirect target, with policy anchored to the admitted connection authority. |
| Malformed, opaque, userinfo-bearing or unsupported-scheme endpoint | Clear bounded refusal, no target request, clean stream/pending-state teardown. |
| Rejected endpoint then valid reconnect | Fresh endpoint is admitted; stale-generation events and state cannot resurrect the old destination. |
| Cancellation or failed dial before POST delivery | Honest error and zero delivery observed; never count an attempted send as a leak. |
| B receives POST then rejects initialization | Baseline exposes the original ordering; repair must stop the request rather than merely preserve the later error. |

Run the first witness through the real `sse` configuration conversion, manager selection and system boot path with local counted services and synthetic headers. Enable and auto-connect only the fixture server. For a model-free whole-system fixture, use counted fail-on-call model stubs and isolate or disable unrelated background producers through supported test seams. Set `AutoDiscoverTools` false for the initial case so the test neither depends on tool-analysis/model work nor confuses later discovery traffic with initialization. This controls MCP tool analysis, not unrelated whole-system boot model activity. [Discovery gate][discovery-gate] Keep bridge prerequisites valid and record an error if boot stops before reaching the transport; that is not a passing destination test. A direct transport fixture supplies a smaller diagnostic case, but cannot replace production wiring acceptance.

Use paired configurations and the same response script, context deadlines and network conditions when comparing current and repaired implementations. Record GET/POST destinations, methods, synthetic header presence, dispatch count, handshake outcome, time to failure/recovery, goroutine/connection cleanup and allocations. Synchronize on event delivery and recipient receipt instead of relying on sleeps. A meaningful security pass requires zero requests at unadmitted targets, while positive controls still connect.

A separate integration case can carry a harmless reviewed tool call through the real kernel and verify effective endpoint identity in its approval subject. Denied/unreviewed tools must still execute zero times. Preserve existing response-ID, protocol-version, wire-size, cancellation, reconnect and concurrent-disconnect coverage. Full package, race and production-path results remain to be established; a resolver-only assertion or a passing redirect test does not close this contract.

## Completion condition

The gap closes when ordinary configured SSE boot cannot send headers or protocol bodies to an unadmitted advertised endpoint, legitimate same-origin connections still work, any chosen split-host exception is explicit and scope-bound, redirects cannot widen that grant, and failure/reconnect paths preserve their lifecycle guarantees. Update architecture status only with those source changes and appropriately scoped execution receipts. This documentation establishes the current source-level gap and proposed acceptance contract, not a shipped repair.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/4f762251d3db6363c354006ace011931cd42c2dc
[root]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/agents.md
[doc-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/Docs/architecture/AGENTS.md
[safety]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/Docs/architecture/mcp/09-SAFETY-AND-INVARIANTS.md#L86-L101
[config-fields]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/config/integrations.go#L27-L51
[config-map]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/config/integrations.go#L168-L209
[headers]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/headers.go#L8-L36
[factory-entry]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/system/factory.go#L1880-L1889
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/system/factory_mcp.go#L18-L55
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/integration.go#L192-L211
[auto]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/client.go#L159-L177
[select]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/client.go#L260-L287
[server-record]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/client.go#L298-L307
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L46-L74
[reset]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L94-L108
[get]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L110-L125
[connect-init]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L160-L195
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L217-L250
[endpoint]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L278-L289
[call]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L333-L378
[post]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L397-L418
[resolve]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L448-L457
[initialize]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse.go#L471-L485
[redirect]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/wire.go#L213-L226
[wire-limit]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/wire.go#L23-L37
[authority-entry]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/remote_authority.go#L280-L317
[authority-subject]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/remote_authority.go#L370-L386
[authority-final]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/remote_authority.go#L550-L574
[prior]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/Docs/audits/mcp-versioned-wire-fidelity.md
[prior-policy]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/Docs/audits/mcp-versioned-wire-fidelity.md#L108-L114
[prior-shipped]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/Docs/audits/mcp-versioned-wire-fidelity.md#L174-L187
[old-qa]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/.quality_assurance/mcp/2026-05-14_05-00-00-AM-EST_mcp_client_manager_boundary_analysis.md#L348-L360
[capability]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/Docs/audits/mcp-capability-admission.md
[stdio]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/Docs/audits/mcp-stdio-outbound-cancellation.md
[scalar]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/Docs/audits/mcp-retained-scalar-continuation.md
[resolver-test]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/compiler_select_test.go#L56-L68
[redirect-test]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/wire_fidelity_test.go#L416-L435
[sse-test]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/transport_sse_test.go#L32-L80
[go-url]: https://pkg.go.dev/net/url@go1.27.3#URL.ResolveReference
[go-http]: https://pkg.go.dev/net/http@go1.27.3#Client
[rfc3986]: https://www.rfc-editor.org/rfc/rfc3986.html#section-5.2
[origin]: https://www.rfc-editor.org/rfc/rfc6454.html#section-4
[protection]: https://web.mit.edu/Saltzer/www/publications/protection/Basic.html
[legacy-spec]: https://modelcontextprotocol.io/specification/2024-11-05/basic/transports
[discovery-gate]: https://github.com/theRebelliousNerd/codenerd/blob/4f762251d3db6363c354006ace011931cd42c2dc/internal/mcp/client.go#L388-L403
