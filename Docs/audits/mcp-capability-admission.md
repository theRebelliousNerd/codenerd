---
doc-class: deep-dive
subsystem: mcp-session-core
implementation-status: planned
last-verified: 2026-10-28
verified-against: 8cd6e1f8804709996b46e43641a3ffe464d2088a
verification-method: static-source-inspection
supersedes: []
---

# Finish MCP capability admission, not just its unreachable identity helper

Status: source-grounded integration proposal, not implemented or runtime-validated results.
Source baseline: main at `8cd6e1f8804709996b46e43641a3ffe464d2088a`, rechecked identical to main on 2026-10-28 at approximately 04:07 UTC. No tests, inference, training, or remote operations were run.

## 1. Intended capability

CodeNERD should discover external capabilities, expose only the information a task needs, and invoke a selected operation only under current, exact kernel authority. A remote operation should be attributable to the task that requested it, use a reviewed effect model rather than trust a server's description, and return evidence that distinguishes refusal, transport failure, remote failure, and successful completion.

The unfinished innovation is **policy-governed capability admission with task-correlated execution**. It is not “add a caller to every unreachable function.” In particular, making `WithRemoteCallIdentity` reachable is necessary for a chosen propagation design but is not sufficient to complete configured MCP delivery.

The existing [MCP safety specification](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/Docs/architecture/mcp/09-SAFETY-AND-INVARIANTS.md#L110-L143) already identifies two open production obligations: install schema-pinned host reviews during configured discovery and propagate session scope/call identity. This document refines that acknowledged gap; it does not claim to have discovered a previously undocumented one.

## 2. What is implemented and what remains unconnected

| Slice | Source-grounded state | Meaning |
|---|---|---|
| Control plane registration and discovery | Implemented wiring in [factory](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/system/factory.go#L1747-L1774) | A configured bridge is constructed, registered with the five-verb control plane, and connected. |
| Exact per-call authorization | Implemented shared [dispatch boundary](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/mcp/remote_authority.go#L186-L420) | It validates current registration, reviewed effect, schema, frozen arguments, confirmation and exact kernel permission before transport invocation. |
| Identity when caller supplies none | Implemented [fallback](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/mcp/remote_authority.go#L225-L232) | A new random request ID also serves as scope and call ID. Absence of the helper does not remove identity or exact authorization. |
| Session-owned correlation | Unconnected in inspected production routes | The helper accepts an upstream scope/call identity, but production symbol search found no caller. The safety specification explicitly keeps this wiring open. |
| Trusted review installation | Unconnected in inspected configured boot | The review setter exists, but inspected callers were tests, not configured boot. No installer was found in factory or bridge construction/discovery. The safety specification confirms this open obligation. |
| Discovery readiness | Implemented, narrower than capability readiness | [Ready](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/mcp/integration.go#L189-L245) means discovery has finished as far as possible, even if some servers failed. It does not assert that effects have host reviews or that any operation is currently permitted. |

These findings support a narrower conclusion than “MCP is disconnected.” Discovery and safe refusal are meaningful behavior. They also support a stronger conclusion than “only an old wrapper is dead”: without a review installation path, the inspected configured route cannot satisfy the host-review prerequisite merely by discovering a tool.

The authority boundary rejects a call without a current review, before generating its call identity. A missing identity caller is therefore not the first obstacle on an unreviewed configured route. [Review validation and refusal](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/mcp/remote_authority.go#L140-L224).

## 3. Actual production path and semantic gaps

The model-facing path runs through [mcpctl.executeCall](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/tools/mcpctl/tools.go#L109-L157), then [ControlPlane.Call](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/mcp/controlplane_invoke.go#L114-L132), then manager dispatch. The control plane adds risk-confirmation context, not remote-call identity. The integration adapter also forwards its supplied context to the manager. These are distinct routes that must retain the same authority boundary; neither should gain a weaker direct-transport escape hatch.

The kernel's [mcp_remote_permitted rule](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/core/defaults/policy/policy_mcp.mg#L111-L122) joins the exact request, review, connected server, operation, current subject, pending action, constitutional permission and confirmation. Scope and call ID participate in exact matching, but this inspected rule does **not** independently establish that a scope belongs to an active session or authenticated user. Correlation is not authentication. Wiring upstream strings must not be advertised as implementing session ownership, per-user ACLs or revocation by itself.

The dispatch boundary also freezes canonical arguments and checks for changed connection/catalog state before invocation. Transient asserted facts are retracted on exit, and cleanup failure poisons the fact emitter rather than pretending the state is clean. These guarantees must survive integration; they are not overhead to remove casually.

### Why blindly wiring the helper once would be wrong

[WithRemoteCallIdentity](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/mcp/remote_authority.go#L46-L64) stores a pointer to a state containing an atomic used flag. Dispatch consumes it exactly once. Consumption occurs before reviewed target extraction and later policy checks, so a failed attempt can still consume its supplied identity.

Meanwhile [readTopResources](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/mcp/controlplane_invoke.go#L509-L524) can issue several resource reads using the same outer context. Adding one identity to that outer context would make the second leaf operation encounter an already-consumed state. A retry that reuses the same context would have the same problem.

The missing contract is therefore not simply “context has a call ID.” It is:

- stable identity for the parent logical task or invocation;
- a fresh child identity for each remote leaf attempt;
- explicit parent/child association;
- a fresh attempt identity after denial or retry;
- no inference that a fresh attempt authorizes repeating a side effect.

## 4. Proposed completion contracts

### A. Trusted admission, separate from discovery

Keep discovery facts as descriptive data. Add a host-owned review process that binds the current server registration, operation kind, subject ID and schema digest to an effect and target interpretation. The current review type contains operation, tool ID, schema hash, effect, target argument and optional risk override; persistence, reviewer provenance and lifecycle remain design work, not present fields. [Current review type](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/mcp/remote_authority.go#L37-L44).

A proposed manifest should additionally identify its trusted issuer, review revision, server identity/configuration binding and invalidation conditions. The host validates these before installing reviews; the model must not install its own review to unblock a task. Schema changes and server identity changes invalidate the relevant admission until reviewed again. Disconnect behavior and rediscovery must not silently resurrect obsolete reviews.

Do not infer effect authority from an MCP server's read-only or destructive annotations. The [MCP schema specification](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations) describes these annotations as hints and warns clients against using untrusted-server annotations for tool-use decisions. This supports the existing host-review boundary, not bypassing it.

Keep resource reads and prompt retrieval in this contract too. Their operation kind matters, even when the reviewed effect is read-only. “Not a tool call” must not become a policy bypass.

### B. Logical identity versus attempt identity

At a trusted session boundary, define a logical operation identifier and its owner/scope. At the final leaf-dispatch boundary, allocate a unique attempt ID and bind it to that logical operation. Preserve the existing one-shot semantics for the attempt.

Conceptual algorithm, not implementation:

1. Resolve the trusted parent operation and cancellation context.
2. Enumerate the selected remote leaf operations.
3. For each leaf, allocate a fresh attempt identity and record its parent relation.
4. Resolve current reviewed metadata and canonicalize arguments.
5. Ask the kernel for exact permission under the current review and policy state.
6. Invoke the frozen operation only if permitted.
7. Record an appropriately redacted terminal receipt; retract transient request facts.
8. On retry, classify whether the previous attempt reached transport and whether repeating it is safe. A new ID prevents accidental identity reuse, not duplicate side effects.

If per-session revocation or ownership enforcement is desired, add an explicit policy relation between the scope, active session, owner and revision. That is a separate proposed rule contract, not a property already supplied by the current string fields.

### C. Typed refusals and bounded recovery

Currently a manager error is summarized as “transport error” by [ControlPlane.Call](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/mcp/controlplane_invoke.go#L123-L132), even though dispatch may reject before transport. Preserve the underlying error but add a stable typed outcome such as:

- not admitted: trusted review missing or stale;
- invalid request: arguments or target contract failed;
- policy denied: exact permission absent or ambiguous;
- confirmation missing;
- attempt consumed;
- canceled before dispatch;
- transport outcome uncertain;
- remote failure;
- successful response.

Only transport-state evidence should justify a transport retry. A missing review should create one bounded repair/admission obligation and stop repeated model attempts; neither confirmation nor switching to a shell is an acceptable substitute.

A receipt should link logical operation, leaf attempt, schema/review revision and outcome without publishing raw arguments, secrets or target contents. Existing authorization logging intentionally omits sensitive payloads; preserve that restraint. Retention and cardinality limits belong in the receipt contract.

### D. Honest readiness

Expose discovery-ready separately from admission state. Present “discovered but not reviewed” as actionable status instead of a confusing failed tool. Do not promise “all MCP ready” merely because a catalog exists, and do not precompute a durable “permitted” bit: exact call permission can depend on current arguments, target, confirmation and constitutional policy.

Progressive disclosure should preserve the current small control surface. Show a compact unavailable reason or review revision only when relevant; do not flood every prompt with every tool schema or all review records.

## 5. Cost model and optimization boundaries

For one task, maintain separate resource ledgers; this is not a sum of unlike units:

```text
Resources(task) = {
  model_input_tokens, model_output_tokens, cached_input_tokens,
  host_CPU_seconds, remote_compute_units_if_observable,
  critical_path_wall_seconds, human_review_minutes,
  transport_requests, transferred_bytes, retained_bytes
}
```

Attribute each dimension to discovery, review loading/refresh, schema validation, canonicalization, authority queries, transport, result shaping, receipts and cleanup. Count retry attempts in those same stages rather than adding a second retry total. Cached and uncached input tokens must be disjoint categories if summed for billing. Do not sum concurrent stage latencies as task wall time; measure the actual critical path. A monetary total is permissible only with explicit unit prices for disjoint billable resources and stated assumptions about human time and shared infrastructure. Report unavailable remote-compute data as unavailable, not zero.

Also measure peak catalog/review/receipt memory, retained result bytes and lock-wait latency. Denied attempts still incur lookup, validation, policy or model costs. Human review and manifest maintenance are not free infrastructure.

The authority mutex currently spans dispatch, including the invoked operation. This suggests a concurrency measurement, not an immediate lock-removal recommendation. Any shortened critical section must preserve registration consistency, review validity, exact authorization and cleanup failure behavior. Do not trade a smaller latency number for a check/use race.

A schema-pinned review cache can avoid repeated human review of unchanged subjects, but should not cache exact permission across changed arguments or policy state. Parent/child receipts add bookkeeping; their token savings are hypothetical until they reduce redundant retries or investigation in a measured end-to-end task.

Compare at equal authority. A system that “succeeds” by trusting server annotations or permitting more actions is not a fair efficiency baseline. The current safely blocked configured route is a correctness baseline for refusal, not a capable execution baseline for all workloads.

## 6. Proposed acceptance and evaluation

No evaluations below were executed in this documentation pass.

First establish configured boot acceptance using a harmless counted remote endpoint and the actual session/model-facing path, not a unit fixture that manually installs a review:

1. Start from configured discovery and show the subject is visible but not admitted.
2. Demonstrate zero transport invocations when review is absent.
3. Install an authorized host review through the intended production configuration flow.
4. Drive an allowed request through the ordinary session route and verify exactly one remote invocation and a task-correlated receipt.
5. Verify repeated attempts, multi-resource fanout and legacy adapter routes preserve fresh leaf identities and exact policy checks.
6. Change schema, server binding, review revision, target, arguments, connection and policy independently; ensure stale authority never grants an invocation.
7. Simulate denial, cancellation, cleanup failure and uncertain transport outcome; verify correct state classification and no automatic unsafe replay.

Measure authorization violations, unexplained denials, correlation completeness, duplicate side effects, repeated refusal turns, task success, total model tokens, wall time, transport count, review burden, lock wait and memory retention. Include small and large catalogs and concurrent tasks; report tail latency as well as averages.

Run three fair comparisons: current guarded path, completed admission/correlation path with identical policy, and completed path with typed bounded recovery. Keep model, task, tool contracts, available information and permission sets matched. Attribute gains separately to “previously unavailable reviewed capability” and “less wasted execution”; do not conflate capability restoration with better reasoning.

## 7. Relationship to other unfinished work

The existing [request-bound human approval audit](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/Docs/audits/request-bound-human-approval.md) addresses a different authorization loop. `confirm_risk` is model deliberateness, not evidence of human consent. This admission proposal should consume that authority when required, not manufacture it.

The right next milestone is not deleting `WithRemoteCallIdentity`, nor changing a dead-code baseline. It is one configured end-to-end remote capability whose trusted review, task lineage, exact permission, bounded result and truthful outcome all survive the real production route. Only after that contract is chosen can unused interfaces be judged as retained extension points, superseded APIs or actual dead code.

### Lifecycle qualification: configured reviews do not yet survive ordinary boot

Static follow-up against main at `356f65e0e6bb639adaa90ab36212daa714114871`, rechecked on 2026-10-28 at approximately 07:11 UTC. No tests, inference, or remote operations were run. This qualifies the **Trusted admission** and **Honest readiness** bullets in main's As built section; it does not replace the original proposal or its historical source baseline.

The configuration plumbing exists, but normal configured startup removes its installed reviews before a tool can use them:

1. The factory converts integration configuration, constructs the bridge, binds the model-facing control plane, and then starts `ConnectAll`. The bridge constructor calls `SetFactEmitter`, which installs configured reviews. [Factory](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L1730-L1763), [bridge constructor](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/integration.go#L97-L108), [emitter installation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L120-L133).
2. Every newly established connection then deletes installed reviews belonging to its server. Discovery follows this deletion; it does not reinstall configured reviews. Consequently, this affects the first successful connection, not just reconnects. [Connection publication and discovery](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L346-L389).
3. A call with otherwise valid current metadata reaches the authority boundary with no installed review and is refused before request identity allocation, exact permission evaluation, or remote tool invocation. The model-facing control plane and legacy integration adapter both use that manager boundary. [Review prerequisite](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/remote_authority.go#L280-L315), [control-plane dispatch](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/controlplane_invoke.go#L114-L132), [adapter](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/integration.go#L34-L50).

This is a safely blocked capability-delivery gap, not an authorization bypass. “Zero remote tool invocations” does not mean zero network activity: connection setup and discovery can already have contacted the server.

Disconnect also removes the server's reviews, and a subsequent successful Connect repeats the removal. An already-connected Connect returns early, but cannot restore reviews previously erased. Installing a trusted review after connection through `SetReviewedToolEffect`, or rebuilding configured reviews with another emitter installation, are API-level possibilities—not recovery steps present in the inspected production startup. Moving review installation later only in the factory would therefore leave manual connection and reconnect behavior unresolved. [Lifecycle behavior](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L238-L255), [disconnect](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/client.go#L399-L435), [review APIs](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/remote_authority.go#L141-L204).

#### Configured review validity is not live admission

`nerd mcp tools` compares reviews loaded from config with persisted tool rows; it does not consult the manager's installed review state. It can therefore print `reviewed: read` and `0 not admitted` while ordinary startup has already removed the executable review. Its present output establishes a configured schema match, not current connection, catalog, admission, or per-call permission. Label those states separately; an offline inspection must not imply live admission. [CLI comparison](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/cmd_systems.go#L110-L135), [config and persisted-store sources](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/cmd_systems.go#L237-L266).

The current configured-review test verifies installation and emitter replacement, but never connects. The model-facing authority test connects and discovers first, then manually installs its review. Those are useful component checks, not coverage of the failing configured lifecycle. [Configured-review test](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mcp/configured_reviews_test.go#L5-L29), [facade fixture ordering](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/mcpctl/authority_test.go#L78-L99).

#### Scoped completion contract

Keep operator configuration as the trusted review source, distinct from executable admission for a current connection/catalog generation. Establish a lifecycle-owned reconciliation point that validates the configured review against the current subject and server binding before making it executable. Cover first connection, manual connection, reconnect, catalog refresh and emitter replacement—not merely constructor ordering.

Preserve invalidation when a subject disappears, its schema changes, its registration is replaced, or authority becomes unhealthy. Define whether and how an unchanged trusted review may be revalidated after reconnect, and define explicit revocation precedence so reconciliation cannot silently undo a revocation. Do not blindly retain old executable reviews, infer authority from discovery, or cache exact permission across changed calls. Reconnection must not itself replay a previously attempted effect.

Acceptance should extend section 6 with the following lifecycle evidence:

- Load reviews through real configuration and traverse factory/bridge startup to the ordinary `mcp_call` surface; do not repair the fixture with a post-connect review setter. With current review and exact positive permission, a counted harmless endpoint receives exactly one tool invocation.
- Missing, malformed, stale, revoked or incorrectly bound reviews receive zero tool invocations. Check both the model-facing surface and legacy adapter, preserving the shared authority boundary.
- Exercise first connection, already-connected Connect, disconnect/reconnect, failed connection, unchanged and changed rediscovery, removed tools, emitter replacement and independent servers. Verify legitimate unchanged authority can be re-established according to the chosen contract, while stale authority cannot survive or resurrect.
- Check connection/catalog changes around in-flight calls and human-approval waits: obsolete authority never dispatches, and no lifecycle event automatically retries an uncertain effect.
- Verify CLI output distinguishes configured review validity, offline/cache-only knowledge and live admission; per-call permission remains a separate decision.

These are proposed checks, not executed results. Resource/prompt configuration reviews, parent-operation correlation and typed refusal outcomes remain the separate deferred work listed above.

## Branch-status addendum: configured tool-review lifetime repaired

**Branch qualification, 2026-10-28 (source checked at 08:57–09:04 UTC).** The original analysis remains a statement about its pinned baseline, not the whole repository today. Main remains [`356f65e`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871). Implementation landed on `feat/conversation-brief` in [`e033fc4`](https://github.com/theRebelliousNerd/codenerd/commit/e033fc454e20482c5d008212991b09e5b997b9d7); this addendum inspects [`462153c`](https://github.com/theRebelliousNerd/codenerd/commit/462153c50a504ee5830ba3aa4667d73c6db9d9a2). The [four intervening commits](https://github.com/theRebelliousNerd/codenerd/compare/e033fc454e20482c5d008212991b09e5b997b9d7...462153c50a504ee5830ba3aa4667d73c6db9d9a2) do not change these implementation files. This is static source and test-definition inspection: no tests, builds, provider calls, or live browser runs were executed for this addendum. “Implemented” below means present on that feature pin, not merged to main or independently runtime-qualified.

**Narrow implementation update.** The earlier configured tool-review and CLI-readiness work must be separated from the reconnect lifetime defect. At the inspected feature pin, `Connect` clears previous connection reviews and then reinstalls only that server's configured reviews through the existing normalization/install path. A fixture explicitly checks that the configured review survives while a runtime-only review does not. This closes the source-visible ordering problem where emitter setup installed reviews that the first connection immediately deleted. [Connect ordering](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/internal/mcp/client.go#L350-L364), [configured installation](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/internal/mcp/remote_authority.go#L176-L193), [witness](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/internal/mcp/wire_fidelity_test.go#L757-L784).

**Not broader authority.** The reinstall retains schema-bound host review; it does not make a connected server or catalog into exact-call permission. Parent logical-operation correlation, resource/prompt configuration reviews, typed control-plane refusal outcomes, reviewer provenance and expiry remain outside this narrow repair. The feature's existing [admission as-built account](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/Docs/audits/mcp-capability-admission.md#L162) records those deferred capabilities.

**Unverified here.** The fixture establishes intended review-map lifetime coverage, not independently observed session-level end-to-end authorized execution. This addendum ran no test or remote effect and does not claim the full acceptance sequence or measured reduction in refusal/retry cost is complete.

