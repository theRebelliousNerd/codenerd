---
doc-class: deep-dive
subsystem: deliberation-mcp
implementation-status: proposed-refinement
audit-status: source-grounded-conditional
last-verified: 2026-11-03
verified-against: 81977967f7decaad6f89e0e9bd7f967c188bd85b
verification-method: static-source-inspection
---

# Keep deliberation read-only at the resolved MCP operation

## Finding and decision

The deliberation roles are described and prompted as read-only, but their executable catalog includes the general `mcp_call` facade. The facade can resolve to a host-reviewed remote write. Its existing authorization boundary checks the review, current registration, exact arguments and constitutional permission, but does not join a deliberation-specific effect ceiling. An otherwise permitted remote write therefore has no additional barrier merely because it was requested while proposing, challenging or probing a design. [Role contract][proposer] [Read broker][reads] [Inherited catalog][catalog] [Remote authority][remote]

**Classification: a conditional phase-authority contract gap, not an observed unauthorized mutation or a bypass of MCP's existing permission system.** A successful dispatch still depends on all ordinary gates, a configured reviewed operation, a connected server and valid arguments. No such operation was executed in this review. The narrower missing guarantee is that an operation permitted for the application as a whole is not necessarily permitted inside its read-only deliberation phase.

The recommended refinement is to bind a host-owned read-only phase constraint to deliberation and carry it through every nested operation. At the final MCP boundary, intersect that constraint with the already resolved host-reviewed effect and existing permission. Preserve genuinely read-only MCP operations. Do not replace exact authorization with a tool-name blacklist, trust server annotations as authority, or globally disable all external lookups.

This is a specific continuation of two acknowledged directions, not a claim to have invented them. The perception-restriction audit explicitly records that generic `/external` effects are not blocked by its workspace-only restriction. The portable-crucible proposal separates read-only design from separately authorized experiments. This audit identifies where those contracts meet in the live deliberation route and supplies a bounded acceptance plan. [Known external limitation][restriction-audit] [Existing crucible][portable]

### Evidence boundary

The public main branch resolved to **81977967f7decaad6f89e0e9bd7f967c188bd85b**, committed November 3, 2026 at 11:22:29 UTC, and was rechecked unchanged at 11:47 UTC before freezing this report. Repository links below use that immutable source pin. [Commit][pin]

Root and scoped guidance, session and mission architecture, related audits, implementation and relevant authored tests were read. No repository program, test, build, model inference, benchmark, gate, external MCP call or failure injection was run. No implementation, credential, configuration, permission or setting was changed. Every proposed API, invariant and evaluation below is unimplemented and unrun.

## 1. The requirement is a phase boundary

Socratic ideation works because the proposer and critic can inspect evidence and challenge requirements before committing to a change. “Inspect the available storage designs” must remain different from “write a storage configuration,” even if the application normally has authority to perform both. A successful requirements review is not execution authority.

The current proposer atom forbids edits and commands, says workspace content is evidence rather than instructions, and treats mission success prose as a proposed acceptance condition rather than permission to change the gate. The interrogator likewise forbids edits, command execution and authority grants. The session specification calls the empirical operations concrete read-only probes and leaves implementation verification to downstream execution. These are stronger promises than “this catalog has no tool literally named write_file.” [Proposer][proposer] [Interrogator][interrogator] [Session contract][session-spec]

The useful target contract is:

> During deliberation, model-requested evidence operations cannot mutate the target repository or external application state, or run arbitrary target commands, even when an enclosing task or configured integration ordinarily permits those effects.

This is not a claim of zero process effects. The conductor persists thread snapshots, emits logs and invokes models; retrieval may maintain host-owned caches. Those operational activities need their existing ownership and privacy rules. The proposed ceiling concerns the target operations offered to the deliberating model and its delegated probes or consultations, not a fiction that the whole process never writes a byte. [Persistence][persistence]

Root guidance assigns safety and orchestration to Mangle, not model discretion, and says safety-gate or permission changes are not work for codeNERD to widen for itself. Accordingly, this report proposes a separately reviewed host/policy refinement; it does not authorize an autonomous mission to implement or relax the restriction. [Root guidance][guidance] [Architecture guidance][architecture]

## 2. Current source chain

### 2.1 The catalog is an envelope of names

The policy derives `deliberation_read_tool(T)` from the shared `/core` and `/codedom_read` envelopes. The shared core explicitly includes `mcp_map`, `mcp_probe`, `mcp_call`, `mcp_expand` and `mcp_context`. Its comment acknowledges the MCP plane and relies on MCP policy for risk. [Deliberation catalog][deliberation-policy] [Core envelope][catalog]

`ReadTools.Definitions` offers registered names present in this relation. `ReadTools.Execute` checks the positive context-byte bound, current name membership, argument parse status and registry availability, then calls the registry. It does not resolve the selected MCP operation's reviewed effect or enforce a phase-specific read ceiling. The argument, allowlist, workspace and preflight guards it invokes remain real protections. [Read broker][reads] [Registry guards][registry] [Preflight installation][preflight-install]

The built-in effect manifest explicitly classifies `mcp_call` and `mcp_context` as `EffectExternal`, not `EffectRead`. That is honest: one outer facade can expose several kinds of leaf operation. The flaw is not this classification; it is treating facade membership as a sufficient read-only phase boundary. [Effect manifest][effects]

### 2.2 Both participants and empirical probes reach the broker

The registered deliberation shard invokes the dedicated role agent, which invokes `ReadTools.Execute` for offered calls. The shard configuration lists read-file and code-graph permissions, but the dedicated role path obtains its definitions from the deliberation broker rather than converting those permission labels into a resolved-MCP-effect check. [Shard][shard] [Agent execution][agent]

An empirical challenge is another route. The conductor identifies critical data items for the chosen candidate, calls the configured probe executor and persists output or error. Both the chat and mission constructors set this executor to the same broker. Consequently, fixing only the participant's tool-list presentation would leave the empirical-probe route uncovered. [Probe runner][probes] [Chat wiring][chat] [Mission wiring][mission-wiring]

The production mission constructor uses the normal Cortex, shard manager and modular registry. Mission preparation constructs a `DesignOnly: true` thread in `propose` mode before adopting its result. Neither field is a resolved remote-effect ceiling in the inspected MCP boundary. They govern design handling and disposition, not an independent per-operation read-only predicate. [Mission preparation][mission-prepare] [Disposition policy][deliberation-policy]

The chat and mission consultation closures explicitly merge a `RestrictReadOnly` restriction before using the consultation manager. This is important positive behavior and should remain. It is not a proof that remote writes are excluded: the generic restriction policy deliberately exempts `/external`, since that broad classification also includes lookups. [Chat consultation][chat] [Mission consultation][mission-wiring] [Restriction semantics][restriction-policy]

### 2.3 MCP performs real authorization, but for a different question

The facade parses arguments and forwards the context to `ControlPlane.Call`. The control plane resolves the operation, validates its schema, checks risk and calls the manager. [Facade][facade] [Control plane][control]

The manager requires current registration and a schema-pinned host review. It freezes arguments, resolves a reviewed target, creates an exact request and joins the existing `mcp_remote_permitted` rule before invoking transport. The rule requires the reviewed effect's corresponding action, connected server, current subject, pending action, exact constitutional permission and required confirmation. Metadata conflicts, stale reviews and missing authority fail closed. These safeguards must not be removed or characterized as absent. [Remote boundary][remote] [MCP policy][mcp-policy]

For a reviewed `RemoteWrite`, the action is `/write_file`. The constitution includes that action among ordinary safe actions, with dangerous-content, restricted-tuple, consent and coder-protection conditions still applied. The remote request facts do not bind the operation to a deliberation phase or an allowed-effect set. Therefore, if those ordinary conditions hold, the existing rules have no additional reason to deny the write because its caller is deliberating. [Effect/action mapping][remote-types] [Request facts][remote-facts] [Constitution][constitution]

This remains conditional on the outer registry preflight. External tools take its generic effectful path; a missing speculative safety engine or an unsafe simulation can stop them. A safe preflight is not a statement that the operation is appropriate in a read-only phase, and the inspected preflight does not derive that phase restriction. [Preflight][preflight] [Simulation gate][preflight-simulation]

Configured host reviews are not hypothetical infrastructure. Current boot builds and binds the configured MCP bridge; the manager installs configured reviews with its emitter and reinstalls them on connection. The older MCP audit's initial missing-review-installer finding has a later as-built qualification, which current source supports. An installation without matching reviews still refuses remote calls and does not exhibit the positive-dispatch witness. [Boot][mcp-boot] [Review installation][review-install] [Reconnect][review-reconnect] [MCP architecture][mcp-architecture]

### 2.4 Existing generic restrictions do not silently solve this

The session restriction implementation ties a call to its owning turn and declared tool effect. Its policy bars write, execute and unknown effects for read-only restrictions, but explicitly excludes external effects. Simply adding `WithRestriction(...RestrictReadOnly)` to one more constructor is therefore insufficient for `mcp_call`. It would improve local effect propagation without establishing the resolved remote operation's ceiling. [Restriction bridge][restriction-bridge] [Restriction policy][restriction-policy]

Nor should the fix blindly mark all external operations as writes. That would turn legitimate MCP lookups into false denials and undercut the ideation partner's ability to gather evidence. The host already has a more precise classification at the remote leaf; use that admitted classification under a phase-specific contract.

## 3. Conditional witness, not an executed reproduction

Use a disposable, counted fake transport in a future test. Do not use a real service or user file to establish this behavior.

Preconditions:

1. A normal booted deliberation route exposes `mcp_call`.
2. A connected fixture server advertises a valid operation, with a current matching host review declaring `RemoteWrite`.
3. Its target and arguments satisfy the ordinary constitutional and risk rules. Any required risk acknowledgment is present. The registry's installed preflight admits the operation under its normal contract.
4. The model fixture requests that operation during proposer or challenger evidence gathering, or the selected critical data item names the same facade and arguments.

Source-derived schedule:

- The broker accepts the facade name as a deliberation read tool.
- The registry applies its existing guards.
- The facade resolves the leaf operation and forwards to the normal MCP boundary.
- The host review resolves to a write action.
- Existing positive permission is derived without a deliberation-phase premise.
- The transport invocation is reachable under these preconditions.

Expected current-source distinction: there is no phase-specific refusal, not a claim that every configuration dispatches. The future regression should count dispatches and attribute the actual refusal stage. If the fixture is stopped only by missing review or an unrelated preflight failure, it has not tested the missing phase boundary.

The matching positive control is a host-reviewed read with the same catalog, source, schema freshness and valid authority. It should remain usable. The matching non-deliberation control is the reviewed write under an ordinary authorized execution phase; the proposed refinement should not disable it.

A malicious server, forged schema or prompt injection is unnecessary for this witness. Conversely, a server that lies about an admitted read effect is a different trust problem. This report does not claim the host can prove remote implementation purity from metadata.

## 4. Proposed contract and ownership

### 4.1 Host-owned phase identity and ceiling

On entering the conductor, establish a scoped phase descriptor containing:

- owning task/thread and invocation identity;
- phase kind, such as design evidence;
- allowed target-effect set;
- parent restriction identity or generation;
- lifecycle state sufficient to distinguish a current phase from a finished or superseded one.

These are proposed semantics, not current fields. The trusted host establishes the descriptor from the actual execution path, not a `role`, `mode` or `DesignOnly` string supplied by a model.

A child can narrow the parent's ceiling, never widen it. Carry the descriptor through both participant calls, empirical probes and consultations. An ordinary mutation task can enter a read-only deliberation child; the parent's broader authority remains available only to a separately admitted execution stage.

Avoid one global “read-only now” fact. The kernel is shared and unrelated work can coexist. Each leaf request must bind to its own owner and phase. Cleanup must retract only that invocation's facts, with failures surfaced rather than treated as unrestricted success.

### 4.2 Intersect at the actual effect boundary

Before remote transport dispatch, require both:

1. all current exact MCP authority and confirmation conditions; and
2. the host-reviewed leaf effect allowed by the owning phase.

Go should measure and carry the existing reviewed effect and identity; Mangle should derive admissibility. Use the shared remote admission boundary so the control plane, direct manager and adapter routes cannot disagree. No second executable request or direct-transport shortcut is needed.

The no-mutation phase must reject write, delete and execute leaves regardless of a low risk label or a model-supplied confirmation flag. Existing approval of an operation outside this phase should not silently change the phase. If a useful experiment needs effects, return that need to the normal authorized execution workflow rather than asking an evidence-gathering model to waive its own ceiling.

Unknown, missing, stale or conflicting effect identity must not count as a read. Preserve current schema-change and connection-change revalidation. A cached eligibility decision must be invalidated with the review/phase identity it depends on.

### 4.3 Keep discovery and dispatch separate

Discovery may describe broader capabilities, but a read-only phase should not advertise a known write as currently callable. Options include a read-filtered MCP discovery view or an explicit unavailable-in-this-phase disposition. This presentation is a usability improvement, not the security boundary. A call made anyway must be refused at final admission.

Preserve `mcp_expand` for retained results because it does not re-invoke the server. Resource and prompt retrieval also need phase propagation to their leaf operations, while preserving their existing independent host-review requirements. A stale handle should not encourage an implicit mutating reinvocation.

The model should receive a bounded typed refusal explaining that the operation is outside the current phase, whether anything reached transport, and what authorized stage would be needed. Do not turn it into an automatic fallback to another operation, a request for broader credentials or permission to repeat an uncertain effect.

### 4.4 Lifecycle and evidence

A phase begins before model evidence collection and ends only after its children are joined. Cancellation cannot erase the classification of an operation already dispatched. An uncertain remote outcome remains uncertain; a new phase or request ID is not grounds for blind replay.

A refusal receipt needs phase/owner identity, tool/review identity, resolved effect, decision and dispatch state. Retain a digest or redacted identifier rather than sensitive arguments. Keep it distinguishable from “server unavailable,” “schema invalid” and “ordinary policy denied.”

For old saved deliberation threads, the resumed host route should create a current read-only phase from its own semantics. Do not trust an absent legacy field as unrestricted authority, or restore a historical descriptor as a new grant.

## 5. Relation to existing audits

| Existing work | Preserved contribution | Increment here |
|---|---|---|
| Perception restriction continuity | Typed restrictions and known external-effect limitation | The dedicated deliberation route and its stronger no-edit/no-command promise; resolve the MCP leaf instead of changing the broad external category |
| MCP capability admission | Exact host review, positive permission, operation identity and configured wiring | Intersect existing authority with the narrower design phase |
| Portable recurse deliberation crucible | Separate design evidence from authorized experiments | A concrete live facade/leaf boundary that must enforce that separation |
| Deliberation read invocation identity | Retain arguments and observation identity | No new claim about missing arguments; this concerns whether the operation may execute |
| Citation, progress, critical-item retirement and selected-review handoff | Integrity and continuity of deliberation evidence and critique | Those records cannot prevent an effect that was admitted during evidence gathering |

Sources: [restriction audit][restriction-audit], [MCP audit][mcp-audit], [crucible][portable], [read identity][read-audit], [citation][citation-audit], [critical retirement][critical-audit], [selected handoff][handoff-audit].

The generic external exception is already documented. This report must not be used to relabel that acknowledged limitation as a wholly new vulnerability. Its distinct deliverable is the source-pinned phase-specific mechanism and acceptance contract.

## 6. Verification plan, all unrun

### Deterministic contract tests

Use the real policy kernel, current production predicate ownership and a counted fake transport. Hold all ordinary admission conditions constant while varying only the phase ceiling or reviewed effect.

| Case | Required result |
|---|---|
| Deliberation + reviewed read | One permitted dispatch, bounded evidence returned |
| Deliberation + reviewed write | Zero dispatches; explicit phase refusal |
| Deliberation + reviewed delete/execute | Zero dispatches; no approval prompt that silently widens the phase |
| Ordinary authorized execution + reviewed write | Existing permitted behavior retained |
| Missing/stale/conflicting host review | Existing refusal retained regardless of phase |
| Model risk flag or server read-only hint contradicts host review | No expansion of the host-admitted effect |
| Proposer, challenger and empirical probe | Same phase ceiling at the leaf |
| Nested consultation | Parent ceiling survives the delegated executor |
| Concurrent restricted and ordinary work | No sibling authority borrowing or global suppression |
| Cancellation or review replacement before dispatch | No stale-authority dispatch |
| Reloaded pending values thread | Host reinstates the phase before any new evidence operation |
| Context/resource fanout and retained expansion | Each remote leaf has correct ownership; local expansion does not re-execute |
| Kernel query/assertion/retraction failure | Visible refusal or containment, not unrestricted continuation |

Existing authored tests already cover ordinary MCP positive/negative authority, including a reviewed-write case, configured review installation, and deliberation budget/flow behavior. They are useful controls, not evidence that this cross-boundary phase contract is tested or passes. This review read them and did not run them. [Remote tests][remote-tests] [Configured review test][review-test] [Deliberation tests][deliberation-tests] [Agent tests][agent-tests]

### Production-route acceptance

After a reviewed implementation, exercise both the normal chat design command and recurse mission construction, with the ordinary world model, CodeDOM context, role registration and registry gates enabled. Supply deterministic participant outputs first, so model compliance cannot hide a missing boundary. Ensure a reviewed write that would succeed outside deliberation is refused inside it. Separately establish that a genuine read remains accessible.

A test that uses an unreviewed server, omits the preflight engine, or offers no MCP facade can pass by accidental unavailability. Such a result is a negative control, not acceptance.

Only after deterministic admission tests pass should an explicitly authorized live-model study assess usability: does the proposer find appropriate reads, leave a blocked experiment unresolved, or repeatedly ask for disallowed effects? Use synthetic services without meaningful external state changes. A model that politely avoids writes is not a substitute for counted host refusal.

### Fair comparison and full costs

Compare current behavior, a blunt “remove all MCP calls from deliberation” baseline, and the proposed resolved-effect ceiling. Keep tasks, models, prompts apart from required disclosure changes, ordinary permissions, source snapshots and budgets matched. Include tasks needing MCP reads so the crude baseline cannot appear optimal by refusing all useful work.

Report:

- unauthorized-for-phase dispatch count and false-denial rate;
- read-dependent design success, correctly unresolved requirements and incorrect readiness;
- repeated disallowed attempts, repair turns and human interruptions;
- complete model input/output/reasoning/cached tokens where available, all consultations and retries, plus unknown usage rather than fabricated zero;
- tool/transport calls, policy facts and queries, serialization/hashing CPU, retained bytes, peak memory and p50/p95 end-to-end latency;
- host-review maintenance and migration effort, operator diagnosis time and any independently observed recovery cost.

The phase check should not intrinsically require another model inference, but that is a design expectation, not a measured saving. Clear refusals could reduce wasted turns; overly broad refusal could increase them. Measure both. Include unsuccessful and blocked tasks in denominators. Avoid double-counting a retry's provider cost as both a model-call cost and a separate monetary repair fee.

No performance, correctness-rate or token-saving claim is established here.

## 7. Primary references and limits

Saltzer and Schroeder's original protection paper motivates limiting privileges to the work being performed and checking authority at every access. Applied here, configured application authority and a narrower design-phase authority should both matter at the actual operation boundary. This is an architectural inference, not empirical validation of a particular implementation. [Saltzer and Schroeder, 1975](https://web.mit.edu/saltzer/www/publications/protection/Basic.html)

The MCP specification says tool annotations are advisory and may not faithfully describe behavior; untrusted annotations should not decide tool use. Therefore a server's `readOnlyHint` cannot replace the current host review or the proposed phase constraint. [MCP ToolAnnotations](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations)

These references support the contract, not a claim that remote behavior can be proven safe merely by tagging it read-only. Host-review quality, server trust and ordinary data-sharing authorization remain separate obligations.

## Acceptance decision

A trustworthy design-only deliberation may use admitted external reads, but a current host-reviewed write that is permitted elsewhere must execute zero times while that phase is active. The result must hold through participants, empirical probes and nested consultations without weakening ordinary MCP authority or disabling authorized execution outside the phase.

Until that is demonstrated, describe the current guarantee precisely: local edit/command names are excluded from the deliberation envelope and remote operations retain normal MCP safeguards, but the inspected implementation does not establish a separate read-only ceiling over resolved MCP effects.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/81977967f7decaad6f89e0e9bd7f967c188bd85b
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/agents.md#L1-L110
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/architecture/AGENTS.md
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/prompt/atoms/system/deliberation_proposer.yaml#L14-L69
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/prompt/atoms/system/requirements_interrogator.yaml#L14-L51
[session-spec]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L896
[reads]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/deliberation/read_tools.go#L14-L82
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/deliberation/read_tools.go#L84-L212
[shard]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/shards/deliberation.go#L14-L65
[catalog]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/core/defaults/policy/intent_routing_rules.mg#L258-L271
[deliberation-policy]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/core/defaults/policy/deliberation.mg#L53-L74
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/tools/registry.go#L329-L432
[effects]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/tools/effects.go#L5-L61
[probes]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/deliberation/outcome.go#L125-L172
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/cmd/nerd/chat/deliberation.go#L19-L55
[mission-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/cmd/nerd/cmd_campaign_mission.go#L41-L70
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/campaign/recurse_mission.go#L173-L260
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/deliberation/persistence.go#L24-L74
[facade]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/tools/mcpctl/tools.go#L109-L158
[control]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/mcp/controlplane_invoke.go#L77-L145
[remote-types]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/mcp/remote_authority.go#L30-L99
[remote]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/mcp/remote_authority.go#L259-L582
[remote-facts]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/mcp/remote_authority.go#L309-L407
[mcp-policy]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/core/defaults/policy/policy_mcp.mg#L38-L124
[constitution]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/core/defaults/policy/constitution.mg#L11-L150
[restriction-policy]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/core/defaults/policy/constitution.mg#L740-L849
[restriction-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/session/turn_restriction.go#L12-L125
[preflight]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/core/virtual_store_interactive_gate.go#L100-L216
[mcp-boot]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/system/factory_mcp.go#L12-L57
[review-install]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/mcp/client.go#L123-L136
[review-reconnect]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/mcp/client.go#L350-L369
[mcp-architecture]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/architecture/mcp/09-SAFETY-AND-INVARIANTS.md#L108-L153
[remote-tests]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/mcp/remote_authority_test.go#L242-L285
[review-test]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/mcp/configured_reviews_test.go
[deliberation-tests]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/deliberation/deliberation_test.go
[agent-tests]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/deliberation/agent_test.go
[restriction-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/audits/perception-restriction-continuity.md#L242-L270
[mcp-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/audits/mcp-capability-admission.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/audits/portable-recurse-deliberation-crucible.md
[read-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/audits/deliberation-read-invocation-identity.md
[citation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/audits/deliberation-citation-admission.md
[critical-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/audits/deliberation-critical-item-retirement.md
[handoff-audit]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/Docs/audits/deliberation-selected-review-handoff.md


[preflight-install]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/core/virtual_store_write_guard.go#L28-L57
[preflight-simulation]: https://github.com/theRebelliousNerd/codenerd/blob/81977967f7decaad6f89e0e9bd7f967c188bd85b/internal/core/virtual_store_interactive_gate.go#L270-L306
