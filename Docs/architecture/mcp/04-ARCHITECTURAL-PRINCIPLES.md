# 04 — Architectural Principles: MCP

> Last verified against codebase: 2026-07-13  
> Binding for `internal/mcp` and its boot wiring

## P1 — Logic selects; models describe

Tool *metadata extraction* may use an LLM (`ToolAnalyzer`). Tool *selection* must prefer Mangle-derived `mcp_tool_selected` when the kernel and EDB allow it. Go `fallbackSelect` is a safety net, not the long-term executive.

## P2 — JIT, not dump

Never inject the full discovered catalog into LLM context by default. Use full / condensed / minimal tiers and `fitBudget` demotion. Mirror the prompt compiler’s budget discipline.

## P3 — Skeleton vs flesh

Always-on categories (filesystem read, search) are **skeleton**; task-specific tools are **flesh**. Policy section 50.7 encodes skeleton rules; Go thresholds approximate when policy is unavailable.

## P4 — Hybrid scoring with fixed weights

Default hybrid is **logic 0.7 × vector 0.3** (`ToolSelectionConfig`, policy combined score). Changing weights is a config/policy decision, not a one-off in call sites.

## P5 — Transport pluggability behind `MCPTransport`

All protocol details hide behind:

```go
Connect / Disconnect / ListTools / CallTool / GetCapabilities / Ping / IsConnected
```

Manager code must not special-case HTTP JSON shapes beyond transport implementations.

## P6 — Durable catalog per workspace

Tool identity, schemas, embeddings, and usage live under the workspace (`.nerd/mcp_tools.db`). Process restarts must not force full re-analysis when `AnalyzedAt` is set.

## P7 — Tool IDs are `serverID/toolName`

Canonical routing key is a single slash join (`processToolSchema`, `IntegrationAdapter.CallTool`). Parse from the **last** slash so server IDs may contain path-like prefixes carefully; reject `..` and extra separators in the tool name portion.

## P8 — Cycle-safe boundaries

`IntegrationClient` and `KernelInterface` / `LLMClient` are defined **in** `mcp` (or mirrored in core) to avoid import cycles with `core`/`config`. Do not import `internal/core` from this package.

## P9 — Fail open on selection, fail closed on safety

- Selection: empty tools / failed mangle → empty set or fallback, not panic.  
- Safety: unserializable args, traversal tool names, proxy non-primitives → **error**.  
- Connect failures: log + status error; do not abort entire agent boot.

## P10 — Background work must recover

DiscoverTools, RecordToolUsage, UpdateServerStatus run in goroutines with `recover` and elevated log levels on failure. Do not silently swallow telemetry that skews future scoring.

## P11 — Config maps, not enums of servers

Server IDs are arbitrary strings (`code_graph`, `scraper`, custom). VirtualStore stores `map[string]IntegrationClient`. Package code must not hardcode product server lists (call sites may name known servers).

## P12 — Wiring audit before deletion

Features may look unused (`CompileToolsForShard`, callbacks, policy file). Grep factory, VirtualStore, campaign, and e2e before removing. Prefer completing the wire over deleting the half-integrated path.
