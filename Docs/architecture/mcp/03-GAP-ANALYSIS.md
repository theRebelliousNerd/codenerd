# 03 — Gap Analysis: MCP

> Last verified against codebase: 2026-07-13  
> Sources: `internal/mcp/*`, `internal/system/factory.go`, `internal/core/defaults/schemas_mcp.mg`

## 1. Spec vs reality matrix

| Capability | Vision | Reality | Gap |
|------------|--------|---------|-----|
| Multi-protocol client | HTTP/stdio/SSE | Implemented | Low |
| Multi-server manager | Dynamic map | Implemented | Low |
| Discover + analyze | Auto on connect | Implemented with cache | Low |
| Durable store + vec | sqlite + vec0 | Implemented + brute fallback | Low |
| JIT compile | Hybrid select + budget | Implemented | Medium (Mangle path weak) |
| Mangle policy rules | Section 50 IDB | File exists `policy_mcp.mg` | **High** — not in kernel policy load |
| Schema Decls | Boot-loaded | `schemas_mcp.mg` in defaults | Low |
| Assert EDB on discover | Register tools as facts | Not found in Go | **High** |
| Render for LLM | Tiers | Implemented | Low |
| VirtualStore invoke | IntegrationClient | Adapter + proxy | Low |
| Boot wiring | Full lifecycle | Adapters + ConnectAll; bridge not stored | Medium |
| Compile on shard spawn | Always available | Method exists; callers sparse | Medium |
| Usage learning → selection | Feed affinities | Stats recorded; not closed loop | Medium |
| Resources/prompts MCP | Full MCP features | Caps recorded; tools-focused | Low (non-goal for now) |
| Server mode | Host tools | Out of scope | N/A |
| Package README accuracy | Matches tree | Lists missing schemas path | Low doc debt |

## 2. Prioritized gaps

### P0 — Selection truthfulness

1. **`policy_mcp.mg` not loaded into kernel program.**  
   Compiler `mangleSelect` queries `mcp_tool_selected(%q, ToolID, RenderMode)` and falls back on empty/error. Without policy IDB + EDB, **fallback is the live path**.

2. **No systematic assertion of `mcp_tool_registered` / capability / affinity facts on discover.**  
   Even if policy loaded, base relevance rules need EDB. Compiler only asserts ephemeral `mcp_tool_vector_score`.

### P1 — Lifecycle & discoverability

3. **Bridge not retained** on boot context after factory block — compile/render for shards may not reach the same store/manager instance unless reconstructed.

4. **ConnectAll async** — first tool calls may race “not connected” before discover finishes.

5. **`cmd_mangle_check` path** references `internal/mcp/schemas_mcp.mg` which is not on disk (actual: `internal/core/defaults/schemas_mcp.mg`).

### P2 — Product completeness

6. Usage stats (`RecordToolUsage`) do not yet adjust selection scores.

7. Intent/domain boosts in policy depend on `current_intent` / `file_topology` — need cross-system fact population (not MCP-local).

8. Stdio/SSE protocol edge cases vs evolving MCP specs.

## 3. Non-gaps (do not “fix”)

| Item | Why not a gap |
|------|----------------|
| No MCP server implementation | Explicit client-only design |
| Heuristic analyzer without LLM | Intentional degrade path |
| Brute-force semantic search | Documented fallback when sqlite-vec absent |
| IntegrationClient defined in both mcp and core | Deliberate cycle break; shapes match |
| Campaign only optionally uses store | Consumer choice, not missing package API |

## 4. Recommended closure order

1. Load `policy_mcp.mg` (or move rules under `internal/core/defaults/policy/`) with kernel_init.  
2. On `SaveTool` / discover success, assert registration + metadata facts; retract/update on disconnect.  
3. Keep bridge on system/boot context; expose compile for articulation/shard paths.  
4. Gate first CallTool or expose readiness on connect+discover.  
5. Close usage→affinity feedback loop later.

## 5. Alignment to north star

Gaps 1–2 are the difference between **“LLM-described tool lists with Go scoring”** and **“logic-determined tool reality.”** Closing them is the primary north-star work for this package.

## 2026-10-02 accepted verification obligation

The historical backlog above requires separate reconciliation against the live implementation; it is not current verification evidence. The following obligation is independently reproduced on db1d4b7e and accepted before implementation.

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-MCP-06-001 | Executable MCP lifecycle and policy verification | **VERIFIED CURRENT:** external parser fixture `internal/mcp/facts_parse_external_test.go:12` retains serialized mangle.ParseAtom; external golden policy controls are at `internal/mcp/policy_golden_test.go:145`, and lifecycle/cache/vector retraction fixtures remain internal at `internal/mcp/facts_lifecycle_test.go:63`. Root full sqlite_vec MCP package gate passes on the modified db1d4b7e working tree: `artifact:.corpus-build/runs/all-features-20261002/round2-mcp.log`. The original import-cycle failure remains in baseline-mcp.log as baseline evidence, not current state. | **PROPOSED UPLIFT:** parser and policy-engine witnesses run as external MCP tests, while internal discovery/cache/status/retraction fixtures retain their assertions. Minimal test-only accessors may expose the production fact serializers. Keep the process-wide serialized parser in `internal/mangle/parse_lock.go#ParseAtom` (`internal/mangle/parse_lock.go:45`). | High | Gate-verified test-only repair (working tree) | None for this bounded obligation; race/live-authority/portfolio acceptance remains separate | No in-package MCP test imports internal/mangle. Preserved lifecycle, golden-policy selections, escaping, vector-retraction, and external real-kernel witnesses pass, followed by the full MCP package. No assertion is removed, skipped, or weakened to obtain compiler success. |

The initial implementation boundary is four test files: internal/mcp/facts_lifecycle_test.go, internal/mcp/policy_golden_test.go, planned:internal/mcp/export_test.go, and planned:internal/mcp/facts_parse_external_test.go. Runtime, config, and permission changes are outside this packet. The root closes the gap and reconciles current-state prose only after behavioral evidence exists.


## Required contract: SSE connection lifecycle

The long-lived SSE GET must survive the per-request POST timeout. Disconnect must cancel pending calls without a send-on-closed panic. Reconnect must reset handshake state, wait for the fresh endpoint, and perform correlated initialize/list/call on that connection.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-MCP-SSE-LIFECYCLE | SSE connection lifecycle | `SSETransport.Connect`, `finishConnection`, and `CallTool` now use per-connection stream state and release pending calls safely (`internal/mcp/transport_sse.go:76`, `:213`, `:505`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | In-process SSE server proves stream outlives request timeout, disconnected pending calls terminate, repeated reconnect uses fresh endpoints, and race-instrumented concurrent responses/disconnect do not panic. |

**Verified current:** `TestSSETransport_LongLivedStreamOutlastsRequestTimeout`, `TestSSETransport_ReconnectUsesFreshEndpointAndRoundTrips`, `TestSSETransport_DisconnectReleasesPendingCall`, and `TestSSETransport_ResponseAndDisconnectAreSafeConcurrently` (`internal/mcp/transport_sse_test.go:123`, `:156`, `:275`, `:317`) pass in the full MCP package and focused race suite.

## Required contract: Generation-fenced MCP resource and prompt catalogs

Keep offline catalogs available for reasoning after `Disconnect`: preserve the
existing tool EDB/manager/control-plane cache and the resource/prompt
manager/EDB/control-plane caches. Resource and prompt catalogs are separate
control-plane data with a five-minute cache. Only after a new connection has
successfully become current, invalidate its resource/prompt manager maps,
retract the exact corresponding EDB groups (`mcp_remote_resource`,
`mcp_resource_registered`, `mcp_remote_prompt`, and `mcp_prompt_registered`),
and invalidate their control-plane cache before marking the connection
connected. Keep the offline tool catalog intact.

Each resource/prompt `Discover` captures the exact current connection and its
generation before issuing its network list call. After the response returns,
an authority fence and exact-current-connection check must precede both fact
publication and manager/control-plane cache publication. A late list response
from a prior connection must not repopulate a catalog after a same-server
reconnect. Successful-connect invalidation must advance/use the same generation
fence so an old in-flight discovery or cache fill cannot erase or republish over
a newer catalog. Return publication, invalidation, and retraction errors to the
caller; do not silently convert them to an empty catalog.
Existing host-side remote execution authorization remains fail-closed and is
not bypassed or relaxed by catalog caching.

Current wiring exposes the race boundary: both `Connect` and `Disconnect`
currently delete the manager's resource/prompt slices
(`internal/mcp/client.go:259-269`, `:318-340`), which loses useful offline
catalogs. The helper used before list calls clears map/facts
(`internal/mcp/remote_authority.go:420-451`) without a connection-generation
fence or surfaced emitter error. `DiscoverResources` and `DiscoverPrompts`
publish facts before checking the exact connection pointer
(`internal/mcp/client.go:623-653`, `:685-715`), allowing an old blocked list to
write after reconnect. The control plane keeps a five-minute catalog cache
(`internal/mcp/controlplane.go:45-76`, `:494-543`), and `ConnectServer` in
integration invalidates it at `internal/mcp/integration.go:181-193`.
`FactEmitter` stores exact resource/prompt groups under per-server keys
(`internal/mcp/facts.go:163-218`) and retracts those keys during full server
retraction (`:262-290`). These paths need one per-server generation fence
spanning successful-connect invalidation, discovery publication, and
control-plane cache fills, while preserving all cached catalogs on disconnect.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-MCP-CATALOG-GENERATION | Resource/prompt catalog reconnect freshness | `Connect` and `Disconnect` currently clear resource/prompt manager maps, and lifecycle invalidation does not retract the exact EDB groups; discovery publishes before exact connection checks (`internal/mcp/client.go:259-269`, `:318-340`, `:623-653`, `:685-715`; `internal/mcp/remote_authority.go:420-451`). The separate five-minute control-plane catalog cache is not fenced against old fills (`internal/mcp/controlplane.go:45-76`, `:494-543`). | Required contract above. | P1 | Accepted; implementation pending | Manager lifecycle, exact FactEmitter retraction, generation fencing, and control-plane cache invalidation | A same-server reconnect test proves disconnect preserves offline tool/resource/prompt caches, then successful reconnect invalidates only resource/prompt manager, EDB and control-plane catalogs before marking connected. Fresh discovery publishes only the new catalog; blocked old list and cache fills cannot overwrite it. Publication/retraction errors surface, and remote execution remains fail-closed. |

## Required contract: Co-locate MCP remote authority evidence

The Mangle rules that decide remote MCP authority execute in the policy
consumer. Preserve each predicate's canonical owner, and copy only this bounded
base-evidence set from its owner into that consumer: `mcp_tool_registered/3`,
`mcp_tool_risk/2`, `mcp_tool_risk_source/2`, `mcp_server_status/2`,
`mcp_tool_name/2`, `mcp_tool_schema_hash/2`, `mcp_server_registered/4`,
`mcp_remote_request/13`, `mcp_remote_reviewed/5`,
`mcp_remote_operation/2`, `mcp_remote_resource/4`,
`mcp_resource_registered/2`, `mcp_remote_prompt/4`, and
`mcp_prompt_registered/2`. This is the complete mirror allowlist, not a
blanket sharing rule. The per-call request/review evidence and metadata must be
retracted symmetrically on replacement, failure, and removal. `pending_action/5`
and `permitted/3` remain policy-owned. Derived remote authorization, including
`mcp_remote_permitted`, is recomputed from the co-located base facts; it is never
copied, synthesized in Go, or treated as permission when evidence is absent,
stale, or conflicting.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-MCP-AUTHORITY-COLOCATION | Sharded remote MCP authority evidence | The shard manifest now carries the fourteen bounded MCP consumer predicates while retaining existing owners (`internal/shards/registration.go:265-275`); `PredicateConsumerRoutes` feeds the runtime and derivation analyzer (`:418-437`; `internal/system/factory.go:1311-1314,1545`; `internal/core/derivation_map.go:843-884`). Cortex sends each routed mutation to the canonical owner and policy consumer, and the router mirrors each mutation (`internal/core/cortex_kernel.go:207-235`; `internal/core/shard_fact_router.go:59-103,169-237`). | Required contract above; the fixed fourteen-predicate set remains the complete bounded mirror allowlist. | P1 | Implemented; focused tests passed; aggregate verification pending | Shard routing/registration and system config | `TestRegisterAllShardFactories` pins the exact fourteen predicates and their canonical owners (`internal/shards/registration_manifest_test.go:59-100`). `TestCortexPredicateConsumerRoutes_PlaceRuleAndMirrorEveryMutationPath` verifies owner/consumer copies across Assert, LoadFacts, AssertBatch, exact and predicate-set retracts, and transaction writes/retractions (`internal/core/predicate_consumers_test.go:89-197`). `TestRemoteAuthority_ProductionShardsAuthorizeEffectAndRevokeRemovedMetadata` proves a production sharded authorization/effect, verifies the policy copy, then removes metadata and proves a subsequent call is denied without a second effect (`internal/mcp/remote_authority_test.go:145-192`); `TestRemoteAuthority_AllProductionRoutesRequireExactPositivePermission` covers missing, conflicting, and denied evidence (`:238-430`). `TestShardJoin_EveryRuleCanFireOnTheShardedKernel` verifies MCP authority conclusions fire only on policy (`internal/shards/shard_join_audit_test.go:128-170`). These focused routing, integration, and join gates passed; fleet aggregate verification remains pending. |

The policy's declared input and derived-predicate boundary is summarized in
[`09-MANGLE-SURFACE.md`](09-MANGLE-SURFACE.md); shard ownership and the safety
invariant are specified in
[`../shards/08-WIRING-AND-INTEGRATION.md`](../shards/08-WIRING-AND-INTEGRATION.md)
and [`../shards/09-SAFETY-AND-INVARIANTS.md`](../shards/09-SAFETY-AND-INVARIANTS.md).
