# 09 — Safety and Invariants: MCP

> Last verified against codebase: 2026-07-13

## 1. Position in constitutional model

MCP tool execution is an **effectful** path. Constitutional `permitted(...)` lives in core policy; this package must:

1. Not invent bypass routes around VirtualStore for production call paths.  
2. Sanitize at the VS proxy boundary.  
3. Reject dangerous tool naming / unserializable payloads.  
4. Bound memory (output truncation) and timeouts.

Default deny for *which* actions run remains a kernel/VirtualStore concern. This package ensures *how* MCP calls behave when allowed.

## 2. Invariants

### I1 — Tool ID shape

- Persistence and routing use `serverID/toolName`.  
- `parseToolID` splits on the **last** `/`.  
- Empty serverID after parse ⇒ invalid tool ID error.

### I2 — Tool name path safety

`CallTool` rejects tool names containing `..` or `/` or `\` (directory traversal class).

### I3 — Args must be JSON-serializable

Manager clones args and `json.Marshal`s before transport; failure returns error (no partial send).

### I4 — Proxy only allows primitive JSON trees

`mcpClientProxy.sanitizeVal` accepts string/number/bool and recursive maps/slices of same; other types ⇒ contract violation error (AST leak prevention).

### I5 — Results null-safe for Mangle

Proxy strips `\x00` from string/[]byte results; unknown types stringified rather than panicking.

### I6 — Output size bound

Manager caps MCP output at **500 KiB** with truncation marker.

### I7 — Timeouts

- Config timeout parsed; invalid/non-positive → **30s** default on connect.  
- HTTP client uses configured timeout.  
- Context deadline/cancel mapped to protocol errors on call.

### I8 — Connect is idempotent when already connected

`Connect` returns nil if transport already connected.

### I9 — Offline call does not panic

Missing/disconnected server returns `MCPCallResult{Success:false, Error:...}` (manager) rather than process crash.

### I10 — Analyzer never blocks discovery permanently

LLM failure → `analyzeWithoutLLM` heuristic path; analysis errors log Warn and continue with partial tool.

### I11 — Store path isolation

Bridge opens `{workspace}/.nerd/mcp_tools.db`; `MkdirAll` parent dirs; empty dbPath rejected.

### I12 — Background goroutines recover

DiscoverTools (post-connect), RecordToolUsage, UpdateServerStatus: `defer recover` + Error/Warn logs.

## 3. Concurrency invariants

| Resource | Guard |
|----------|-------|
| `MCPClientManager.servers` | RWMutex |
| `MCPToolStore` SQL | RWMutex |
| Bridge `adapters` | RWMutex |
| Transport connection flags | per-transport mutex |
| CallTool args | cloned before use |

## 4. Mangle safety

- Decl surface in `schemas_mcp.mg` before rules use predicates.  
- Temporary vector scores retracted after compile (best-effort).  
- Policy file uses bound variables and positive atoms for availability before boosts.  
- **Gap:** if policy not loaded, selection safety reduces to Go thresholds (still excludes low scores).

## 5. What this package does **not** enforce

| Concern | Owner |
|---------|-------|
| Whether invoking a tool is `permitted` | Core policy / Dreamer / VirtualStore action routing |
| Network egress allowlists | Host / policy / ops |
| Stdio subprocess sandboxing | OS / higher-level policy (stdio runs configured command) |
| Secret redaction in tool outputs | Not implemented here |
| Prompt injection via tool descriptions | Analyzer trust boundary; schemas trusted from server |

## 6. Stdio-specific caution

`NewStdioTransport` splits endpoint on whitespace and `exec.Command`s it. Config authors effectively choose a local process. Treat stdio MCP servers as **trusted workspace config**, not untrusted user chat input.

## 7. Testing of safety

- Client boundary tests: empty IDs, bad protocols, malformed tool IDs, extreme timeouts.  
- CallTool: unserializable args, disconnected server.  
- E2E VS: panic client, non-primitive args, concurrent access, client replacement.  
See `10-TESTING-ALIGNMENT.md`.
