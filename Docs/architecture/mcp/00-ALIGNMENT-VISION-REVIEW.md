# 00 — Alignment & Vision Review: MCP (`internal/mcp`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/mcp/` (10 non-test Go, 16 tests, 1 local policy `.mg`)

## 1. North-star statement

codeNERD separates **creative** work (LLM) from **executive** control (Mangle kernel). MCP tools are external effect surfaces: they must not be dumped wholesale into every prompt, and they must not bypass constitutional routing.

`internal/mcp` implements that split for tools:

1. **Discovery & analysis** enrich tools with structured metadata (categories, capabilities, shard affinities) — LLM optional, heuristic fallback always available (`analyzer.go`).
2. **Selection** prefers hybrid **logic × 0.7 + vector × 0.3**, with Mangle query `mcp_tool_selected(...)` when the kernel is present (`compiler.go`, `policy_mcp.mg`).
3. **Execution** goes through `IntegrationAdapter` → `MCPClientManager` → transport, and at the VirtualStore edge through `mcpClientProxy` sanitization (`integration.go`, `internal/core/virtual_store_mcp_proxy.go`).
4. **Rendering** is three-tier (full / condensed / minimal) so token budgets stay under control (`renderer.go`, `fitBudget` in `compiler.go`).

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **4** | Analyzer uses LLM for metadata; compiler asserts vector scores and queries `mcp_tool_selected`; Go `fallbackSelect` when Mangle fails (`compiler.go`) |
| Fact-flow fidelity | **3** | Boot wires adapters into VirtualStore (`system/factory.go`); schemas loaded via `schemas_mcp.mg`; **tool registration facts are not systematically asserted on discover** (compiler only asserts temporary `mcp_tool_vector_score`) |
| JIT discipline | **5** | Explicit JIT Tool Compiler mirrors prompt compiler skeleton/flesh + budget demotion (`compiler.go`, package README) |
| Transport completeness | **4** | HTTP, stdio, SSE implemented; protocol maturity varies (stdio/SSE more complex) |
| Persistence + vectors | **4** | SQLite store + optional sqlite-vec; brute-force cosine fallback (`store.go`) |
| Safety at edge | **4** | Arg clone/JSON check, path traversal reject on tool name, output size cap, proxy sanitization (`client.go`, `virtual_store_mcp_proxy.go`) |
| Boot wiring | **3** | Bridge created when integrations configured; adapters registered; **bridge not retained on boot context** for later compile-from-kernel paths; ConnectAll is fire-and-forget goroutine |
| Policy loading | **2** | `policy_mcp.mg` lives in package but is **not** listed in kernel policy load paths (only Decl surface in `internal/core/defaults/schemas_mcp.mg` is boot-loaded) |
| Test grounding | **4** | Dense unit/coverage tests on client, store, compiler, renderer, transports; e2e VS proxy suite under `tests/e2e/` |
| Observability | **3** | `logging.CategoryTools` throughout; compilation stats logged; no first-class metrics exporter |

**Overall alignment: 3.6 / 5** — package is a **real, non-trivial implementation** of MCP + JIT tool serving. Residual risk is **policy/schema wiring partials** (Mangle selection may always fall through to Go) and **lifecycle ownership** of the integration bridge after boot.

## 3. What “good” looks like (MCP-specific)

| Good | Bad |
|------|-----|
| Tools selected by shard + task + budget | Entire MCP catalog dumped into every prompt |
| Offline-analyzed tools cached in SQLite | Re-LLM-analyze every connect |
| VirtualStore proxy sanitizes args/results | Raw map races and AST leakage into JSON-RPC |
| Skeleton tools always available when policy says so | Ad-hoc “always include filesystem” hardcoding only in Go |
| Failed server connect degrades with status | Silent hang of the whole agent boot |
| Mangle Decl + rules loaded together | Decls without rules (or rules never loaded) |

## 4. Related corpora

- `Docs/architecture/core/` — VirtualStore, `SetMCPClient`, schemas  
- `Docs/architecture/system/` — factory boot wiring  
- `Docs/architecture/config/` — `IntegrationsConfig`  
- `Docs/architecture/embedding/` — engines used by analyzer/compiler  
- `Docs/architecture/campaign/` — `MCPToolStore` consumers  
- `Docs/architecture/prompt/` — JIT pattern sibling  
- `Docs/architecture/tools/` — static/non-MCP tools  

## 5. Verdict

Treat `internal/mcp` as **production-capable client infrastructure** with a **designed** JIT compiler and **partially wired** Mangle executive path. Do not document it as pre-implementation. Do not claim full Mangle-driven selection is always live until `policy_mcp.mg` is proven loaded and tool EDB facts are asserted on discovery.
