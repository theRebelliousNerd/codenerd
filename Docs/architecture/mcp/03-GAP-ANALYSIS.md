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
