# mcp — Open Questions

> Last verified: 2026-07-13  
> Real open design questions (not filler)

## Q1 — Where should Section 50 policy live?

**Options:** keep `internal/mcp/policy_mcp.mg` and add load path; or move under `internal/core/defaults/policy/` for consistency with other sections.

**Trade-off:** package locality vs single policy corpus. Kernel_init already loads schemas from defaults only.

## Q2 — Who owns fact emission: store, manager, or bridge?

Asserting Mangle facts on SaveTool couples persistence to kernel. Emitting in manager couples discovery to kernel. Bridge could own a “sync to kernel” pass.

**Need:** cycle-safe design (mcp must not import core types beyond interfaces).

## Q3 — Should disconnected tools remain selectable?

Policy marks tools available when server is `/disconnected` (cached offline). Is that desirable for LLM context (show tools you cannot call) vs only `/connected`?

## Q4 — When does compile run in the live agent loop?

API exists (`Compile`, `CompileToolsForShard`) but standard articulation/shard prompt assembly wiring is unclear. Is MCP tool text meant to be:

- Always injected for certain shards?  
- Only when config integrations present?  
- Only on explicit user/tool request?

## Q5 — Bridge lifecycle ownership after boot

Factory creates bridge, registers adapters, starts ConnectAll, then drops the bridge variable. Should Close run on shutdown? Should campaigns share the same store instance as boot?

## Q6 — Config key: `endpoint` vs `base_url` for stdio

`MCPServerConfig` has both `BaseURL` and `Endpoint`. HTTP/SSE use BaseURL; stdio uses Endpoint. Config conversion currently maps BaseURL from integration config — does stdio get a first-class field in `MCPServerIntegration`?

## Q7 — Relationship to `tool_routing.mg` section 40

Section 40 defines generic shard/capability affinities for static tools. Section 50 is MCP-specific. Should these converge, share `intent_requires_capability`, or remain dual?

## Q8 — Security model for stdio MCP

Is workspace config the sole trust boundary, or do we need allowlists of executable paths / no-shell-split parsing?

## Q9 — Vector dimensions change

If embedder model/dimensions change, old BLOB embeddings become invalid. Invalidation policy?

## Q10 — Multi-tenant / multi-workspace

Store path is per workspace. Shared MCP servers across workspaces — intentional isolation or missing global cache?
