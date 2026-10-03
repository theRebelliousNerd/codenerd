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

## Positive per-call executive authority

**GAP-MCP-REMOTE-EFFECT-AUTHORITY:** every remote execution route, including
ControlPlane.Call, MCPClientManager.CallTool, and IntegrationAdapter.CallTool,
requires positive current kernel authority for the resolved server/tool identity,
host-resolved effect, and canonical argument payload. Risk metadata and a model's
`confirm_risk` flag are not authority. Neither absence of `mcp_tool_gated` facts,
nil kernel, query failure, missing classification, nor a risk override may admit
a remote effect. Confirmation requirements remain a separate policy obligation.

Reuse the existing pending-action/permitted-action constitutional semantics with
scoped request ownership; exact call identity, target, and canonical arguments
must remain bound through dispatch. Resolve effect from reviewed host metadata
and policy, not model prose or tool-name heuristics. Unknown effect fails closed.
Read-only classified requests may derive existing read authority; write, delete,
and execution effects must obtain their corresponding actual permissions, not
the outer control-plane verb's blanket read classification. Keep requests isolated
and retract transient facts on every exit. Do not issue a second executable action.

Acceptance uses a real kernel and counted transport: positive exact authorization
executes once through each entry route; nil/empty/error authority, unknown effect,
missing metadata, mismatched server/tool/args, revoked permission, and model-supplied
confirmation/override controls execute zero times. Preserve schema validation,
catalog/discovery, escaping, output retention, and telemetry assertions. Update
fixtures to establish actual positive authority, never relax the production gate.

The authored boundary is shared in `internal/mcp/remote_authority.go:172` and
joined to the constitutional permission in
`internal/core/defaults/policy/policy_mcp.mg:111`. Production acceptance remains
open: configured discovery must install schema-pinned host reviews, and the
session must propagate its scope/call identity into remote dispatch. Until that
wiring exists, unreviewed configured calls intentionally refuse; do not infer a
review from server annotations, model confirmation, or discovery success. The
real-kernel route tests are component evidence, not proof of configured boot or
model-facing end-to-end delivery.
