# 01 — Vision: MCP Tool Integration

> Last verified against codebase: 2026-07-13  
> Package: `internal/mcp`

## 1. Product intent

External tools should feel like **first-class capabilities** of the agent without becoming an unbounded prompt tax or an ungoverned side-channel.

The vision for MCP in codeNERD:

1. **Connect** to any configured MCP server (language-server style processes, HTTP services, SSE bridges) without hardcoding server IDs in the package core.
2. **Discover** tools once, **analyze** them into structured atoms (categories, capabilities, domain, shard affinities), and **embed** them for semantic retrieval.
3. **Compile** a per-task tool set the way the prompt system compiles prompt atoms: skeleton (always needed) + flesh (context-dependent) + token budget demotion.
4. **Execute** tools only through VirtualStore / policy-facing paths so effectful calls remain under constitutional control.
5. **Learn** from usage (counts, success, latency) so later selection can prefer tools that actually work.

## 2. Architectural target shape

```
┌──────────────────────────────────────────────────────────────┐
│                    JIT TOOL COMPILER                         │
│  TaskContext + ShardType + TokenBudget                       │
│         │                                                    │
│         ├─ VectorSearch(embeddings) ──► mcp_tool_vector_score│
│         ├─ Mangle Query mcp_tool_selected(Shard, Tool, Mode) │
│         └─ FallbackSelect (affinity × vector hybrid)         │
│         │                                                    │
│         ▼                                                    │
│  CompiledToolSet { Full, Condensed, Minimal } + Stats        │
│         │                                                    │
│         ▼                                                    │
│  ToolRenderer → markdown / JSON / invocation text for LLM    │
└──────────────────────────────────────────────────────────────┘
         ▲ persistence                         ▲ invoke
         │                                     │
   MCPToolStore                         MCPClientManager
   (.nerd/mcp_tools.db)                 transports: http|stdio|sse
```

## 3. Design goals (normative)

| Goal | Meaning |
|------|---------|
| G1 Dynamic servers | Config map of server IDs; no package recompile for new MCP servers |
| G2 JIT serving | Never send all schemas; tiered render by relevance |
| G3 Logic-first selection | Mangle rules decide modes when EDB is populated; Go is fallback |
| G4 Durable memory | Tools and embeddings survive process restarts in workspace DB |
| G5 Degrade gracefully | No LLM, no vec, no kernel, offline server → still usable paths |
| G6 Safe integration | IntegrationClient boundary mirrors VirtualStore contract |
| G7 Campaign awareness | Long-horizon planners can query MCP catalog before generating tools |

## 4. Non-goals

- Implementing a full **MCP server** (codeNERD is the **client**).
- Replacing `internal/tools` static tool definitions wholesale.
- Embedding product-specific Vectryx/NeuroLog domain features into the MCP package.
- Fuzzy NL matching inside Mangle (embeddings first, then facts).

## 5. Success criteria

- A workspace with `integrations.servers` can auto-connect, discover tools, and expose `CallTool` via VirtualStore without code changes outside config.
- A shard spawn path can call compile+render and receive a budgeted tool block.
- Kernel queries over `mcp_tool_*` predicates are meaningful when facts are asserted.
- Disconnects and bad servers do not crash the agent process.
