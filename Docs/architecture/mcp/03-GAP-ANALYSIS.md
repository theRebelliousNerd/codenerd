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
