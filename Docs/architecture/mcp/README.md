# mcp — Architecture Corpus (`internal/mcp`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/mcp/`  
> Scale: **10** non-test `.go` + **1** local `.mg` + package README; **16** test files

## Scope

This corpus documents codeNERD’s **Model Context Protocol (MCP) client stack** and **JIT Tool Compiler**: connect to external MCP servers (HTTP / stdio / SSE), discover and analyze tools, persist them with embeddings, select a context-aware tool set (Mangle + vector hybrid, with Go fallback), and render that set for LLM context. It also documents how VirtualStore and system boot wire MCP adapters into the OODA fact-flow.

It is **not** the static tool runner (`internal/tools/`), not the Mangle engine (`internal/mangle/`), and not the prompt JIT (`internal/prompt/`) — though it deliberately mirrors the prompt compiler’s skeleton/flesh + budget patterns.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision for MCP |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types/funcs that matter |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, config, VirtualStore, campaign |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Concurrency, sanitization, default-deny edge |
| [09-MANGLE-SURFACE.md](09-MANGLE-SURFACE.md) | Predicates, `policy_mcp.mg`, schema split |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, e2e, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories, stats, debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Role in fact-flow

```
config.integrations.servers
        │
        ▼
system factory boot ──► MCPIntegrationBridge
        │                     │
        │                     ├─ MCPClientManager (transports)
        │                     ├─ MCPToolStore (.nerd/mcp_tools.db)
        │                     ├─ ToolAnalyzer + embeddings
        │                     ├─ JITToolCompiler (+ optional kernel)
        │                     └─ ToolRenderer → LLM context string
        │
        ▼
VirtualStore.SetMCPClient(serverID, IntegrationAdapter)
        │
        ▼
next_action / workflows call GetMCPClient(serverID).CallTool(...)
        │
        ▼
MCP server (HTTP | stdio | SSE) ──► MCPCallResult
```

## Verify

```powershell
go test ./internal/mcp/...
go test ./tests/e2e/ -run MCP -count=1   # VirtualStore MCP proxy e2e (if present)
```

Build note (sqlite-vec / store): use CGO flags from root `AGENTS.md` when exercising vector index paths against a real DB.

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring honesty, and explicit partials — **not** inventory stubs.  
All path citations under `internal/mcp/`, `internal/system/factory.go`, `internal/config/integrations.go`, `internal/core/defaults/schemas_mcp.mg`, and campaign consumers were verified against the tree on **2026-07-13**.
