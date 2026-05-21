---
surface: "mcp_virtualstore"
mode: "boundary"
subsystems_tested: ["mcp", "core.VirtualStore", "core.Kernel"]
blast_radius: "critical"
remediated: false
---

# MCP to VirtualStore Integration Analysis

## 1. System Interaction Map

The boundary between the Model Context Protocol (MCP) subsystem and the VirtualStore is a critical interaction zone in the codeNERD architecture. The VirtualStore acts as the FFI gateway for the Mangle kernel, mapping virtual predicates to external tool executions. The MCP IntegrationBridge provides the adaptation layer.

Key interaction points and function signatures:

- **Registration & Binding:**
  - `core.VirtualStore.SetMCPClient(serverID string, client core.IntegrationClient)`
  - `core.VirtualStore.GetMCPClient(serverID string) core.IntegrationClient`
  - `mcp.NewIntegrationAdapter(manager *MCPClientManager, serverID string) *IntegrationAdapter`

- **Execution Flow:**
  1. Mangle Kernel evaluates a rule requiring an external action (e.g., `impact_radius`).
  2. VirtualStore intercepts the virtual predicate and identifies it maps to an MCP tool.
  3. `VirtualStore` calls `mcpClient.CallTool(ctx context.Context, tool string, args map[string]interface{}) (interface{}, error)`.
  4. `mcp.IntegrationAdapter` prepends the `serverID` to the tool name (`serverID/toolName`).
  5. `mcp.MCPClientManager.CallTool(ctx, toolID, args)` is invoked.
  6. Argument serialization checks occur: `json.Marshal(args)`.
  7. Mutex locks acquire the server connection `m.servers[serverID]`.
  8. The underlying MCP transport (HTTP, SSE, Stdio) executes `Transport.CallTool(ctx, toolName, args)`.
  9. Result is unmarshaled into `MCPCallResult`.
  10. Background goroutine updates `MCPToolStore.RecordToolUsage`.
  11. `IntegrationAdapter` extracts the `result.Output` and returns it to `VirtualStore`.
  12. `VirtualStore` maps the output back into Mangle `Fact` instances and asserts them in the Kernel.

## 2. Contract Analysis

The implicit contracts between the VirtualStore and the MCP subsystem are numerous and fragile:

**Contract A: Type Fidelity & Serialization**
- *Assumption:* The `VirtualStore` assumes that arguments passed to `CallTool` are purely JSON-serializable primitives (strings, numbers, booleans, slices, maps).
- *Reality:* The Mangle Kernel can produce complex nested types or AST nodes. If VirtualStore passes an `ast.Atom` or `ast.Variable` without unpacking it, `json.Marshal` in `MCPClientManager` will fail or serialize it incorrectly.

**Contract B: Lifecycle & Connectivity**
- *Assumption:* The `VirtualStore` assumes that if `GetMCPClient(serverID)` returns a non-nil client, the server is connected and ready to execute tools.
- *Reality:* The underlying transport may have disconnected asynchronously. `CallTool` will fail fast, but VirtualStore may not have a retry mechanism for transport-level failures.

**Contract C: Result Formatting**
- *Assumption:* The `IntegrationAdapter` returns `result.Output` as `interface{}`. The `VirtualStore` expects this output to be in a specific schema (e.g., a map, a specific string format) to map it back into Mangle facts.
- *Reality:* MCP servers are external and can return arbitrary JSON. If a server updates its tool schema, `VirtualStore` may panic during type assertion (e.g., `output.(map[string]interface{})["radius"].(float64)`).

**Contract D: Concurrency & Thread Safety**
- *Assumption:* `CallTool` is thread-safe and can be invoked concurrently by multiple parallel kernel evaluations or multiple subagents.
- *Reality:* `MCPClientManager` uses an `RWMutex` to retrieve the connection, but the underlying transport might serialize requests or drop them under high load. `VirtualStore`'s `SetMCPClient` is locked, but changing the client while calls are in flight could cause race conditions.

**Contract E: Context Cancellation & Leaks**
- *Assumption:* When the `Session Executor` cancels a context (due to timeout or user abort), the `VirtualStore` passes this to `CallTool`, and the MCP transport immediately aborts the external process.
- *Reality:* Stdio transports might orphan child processes if the context cancellation isn't handled via SIGKILL. The `VirtualStore` continues, but system resources leak.

## 3. Failure Mode Enumeration

For each contract, we anticipate the following failure modes:

### Temporal Failures
1. **The Stalled Transport:** MCP server accepts the request but never responds. Context timeout in `VirtualStore` must trigger. Does `VirtualStore` leave partial state?
2. **The Zombie Connection:** Transport disconnects abruptly during `CallTool`. `MCPClientManager` fails, returns an error. `VirtualStore` must handle this gracefully without crashing the session.
3. **The Thundering Herd:** 50 simultaneous Mangle evaluations try to call `CallTool` on the same MCP server. Transport queue overflows.

### Semantic Failures
4. **The Ghost Types:** VirtualStore passes Go `ast.Constant` values instead of primitive strings. `json.Marshal` succeeds but creates `{}` or unexpected JSON. MCP server fails validation.
5. **The Schema Drift:** MCP server returns an array instead of an object. `VirtualStore` type assertion panics, crashing the executor.
6. **The Silent Truncation:** MCP server returns a 10MB JSON response. `VirtualStore` tries to map this to Mangle facts. The Kernel's spreading activation blows up the token budget.

### Ordering Failures
7. **The Late Arrival:** `SetMCPClient` is called *after* the Mangle rule is evaluated but *before* the retry loop. Does VirtualStore dynamically re-evaluate, or is the fact cached as a failure?
8. **The Disconnect Race:** Server disconnects exactly when `CallTool` acquires the `RLock`.

### Partial Failures
9. **The Usage Stats Panic:** `CallTool` succeeds, but the background goroutine updating `MCPToolStore.RecordToolUsage` panics due to a closed database connection. Does it crash the manager?
10. **The Half-Asserted Facts:** `VirtualStore` receives a complex result, asserts 3 facts successfully, panics on the 4th. The Kernel is now in an inconsistent logical state.

### Corruption Failures
11. **The Shared Map Mutation:** `VirtualStore` passes a map to `CallTool`. The `VirtualStore` mutates the map while `MCPClientManager` is marshaling it. Data race detected.
12. **The Connection Overwrite:** Two agents try to register different MCP servers with the same `serverID`.

## 4. Adversarial Scenario Design

**Scenario 1: The Infinite Null Output (Temporal/Semantic)**
- *Violated Contract:* Result Formatting
- *Mechanism:* Mock MCP server returns a `nil` output with `Success=true`.
- *Expected Behavior:* `IntegrationAdapter` returns `nil, nil`. `VirtualStore` must gracefully reject the nil map without panicking.
- *Severity:* P1

**Scenario 2: The Context Cancellation Race (Temporal)**
- *Violated Contract:* Context Cancellation
- *Mechanism:* Cancel the context exactly as `CallTool` enters the `MCPClientManager`.
- *Expected Behavior:* Function returns `context.Canceled` immediately. No zombie goroutines.
- *Severity:* P2

**Scenario 3: The Map Mutation Data Race (Corruption)**
- *Violated Contract:* Concurrency & Thread Safety
- *Mechanism:* Pass an args map to `CallTool` from `VirtualStore`, and concurrently mutate that map in a separate goroutine.
- *Expected Behavior:* The system should either clone the map before passing or `MCPClientManager` should clone before marshaling (run with `-race`).
- *Severity:* P1

**Scenario 4: The 100MB Output Bomb (Resource Exhaustion)**
- *Violated Contract:* Result Formatting
- *Mechanism:* MCP server returns an artificially massive 100MB string output.
- *Expected Behavior:* `VirtualStore` truncates or rejects the output before converting to Mangle facts, preventing OOM.
- *Severity:* P0

**Scenario 5: The AST Type Leak (Semantic)**
- *Violated Contract:* Type Fidelity
- *Mechanism:* Pass an `ast.String("test")` directly into the args map instead of a primitive Go string.
- *Expected Behavior:* MCP Client Manager marshals it correctly or explicitly rejects it. If it marshals as `{}` the server will fail.
- *Severity:* P2

**Scenario 6: The Rapid Connect/Disconnect (Temporal/Corruption)**
- *Violated Contract:* Lifecycle & Connectivity
- *Mechanism:* Concurrently call `Connect`, `Disconnect`, and `CallTool` in a tight loop.
- *Expected Behavior:* No deadlocks on `mu.RLock()`. `CallTool` safely returns "not connected" errors.
- *Severity:* P1

**Scenario 7: The Background Stat Panic (Partial)**
- *Violated Contract:* Error Isolation
- *Mechanism:* Mock `MCPToolStore` to panic on `RecordToolUsage`.
- *Expected Behavior:* The background goroutine panic is recovered, or it crashes the system? If it crashes the system, it's a P0.
- *Severity:* P0

**Scenario 8: The Type Assertion Panic (Semantic)**
- *Violated Contract:* Result Formatting
- *Mechanism:* Return `[]interface{}{"unexpected array"}` when `VirtualStore` expects a `map[string]interface{}`.
- *Expected Behavior:* `VirtualStore` handles the error gracefully and asserts an error fact in the Kernel, rather than panicking.
- *Severity:* P0

**Scenario 9: The Empty Server ID (Semantic)**
- *Violated Contract:* Registration
- *Mechanism:* Call `SetMCPClient` with an empty string as `serverID`.
- *Expected Behavior:* `VirtualStore` rejects the registration.
- *Severity:* P3

**Scenario 10: The Concurrent BootGuard Bypass (State Corruption)**
- *Violated Contract:* Concurrency
- *Mechanism:* Two goroutines attempt to trigger `CallTool` while `bootGuardActive` is transitioning.
- *Expected Behavior:* Thread-safe access to `bootGuardActive`.
- *Severity:* P2

**Scenario 11: The Tool Name Injection (Semantic)**
- *Violated Contract:* Security/Routing
- *Mechanism:* `VirtualStore` passes `../admin/tool` as the tool name.
- *Expected Behavior:* `IntegrationAdapter` constructs `serverID/../admin/tool`. Does it bypass routing?
- *Severity:* P1

**Scenario 12: The Zombie Stdio Transport (Temporal)**
- *Violated Contract:* Context Cancellation
- *Mechanism:* Send a massive payload to a Stdio transport, then cancel context.
- *Expected Behavior:* The transport closes stdin/stdout and kills the process.
- *Severity:* P2

**Scenario 13: The Missing Code Graph Fallback (Semantic)**
- *Violated Contract:* Graceful Degradation
- *Mechanism:* `VirtualStore` attempts to use `code_graph` MCP client which is not configured.
- *Expected Behavior:* `VirtualStore` falls back to "Deep impact analysis skipped".
- *Severity:* P3

**Scenario 14: The Ouroboros Call (Cascading)**
- *Violated Contract:* Call depth
- *Mechanism:* MCP server tool execution somehow triggers another MCP server tool execution recursively.
- *Expected Behavior:* Maximum depth exceeded or timeout.
- *Severity:* P2

**Scenario 15: The Null Result Struct (Semantic)**
- *Violated Contract:* Result Formatting
- *Mechanism:* `IntegrationAdapter` receives a nil `*MCPCallResult` from `Manager` despite no error.
- *Expected Behavior:* Adapter checks `result == nil` and returns an error without panicking on `result.Success`.
- *Severity:* P1

## 5. Cascading Failure Analysis

**If the MCP Transport Panics:**
The `CallTool` function is executed synchronously from the perspective of the `VirtualStore`. If `json.Marshal` panics or the underlying transport panics, the `VirtualStore` goroutine will panic. Since `VirtualStore` is often invoked during the Mangle Kernel's evaluation loop (`Eval()`), a panic here will crash the entire `RealKernel`. This cascades to the `Session Executor`, causing the session to terminate abruptly.

**If the Background Stat Goroutine Leaks:**
`MCPClientManager` spawns `go func() { m.store.RecordToolUsage(...) }`. If `RecordToolUsage` blocks indefinitely (e.g., SQLite lock), these goroutines will leak on every tool call. After thousands of calls, the system will OOM or hit thread limits, stalling the entire codeNERD process.

**If VirtualStore Panics on Type Assertion:**
A very common failure. The LLM or MCP server hallucinates a response format. `VirtualStore` expects `output.(map[string]interface{})["files"].([]interface{})`. If it's a string, Go panics. This crashes the `VirtualStore`, the `Kernel`, and the `Session`. No recovery loop (Autopoiesis) is triggered because the process dies.

**If Context Cancellation is Ignored:**
If `MCPClientManager` ignores context cancellation, a run-away MCP tool (e.g., a massive `grep`) will continue consuming CPU. The `Session Executor` might move on to the next phase, but the system load remains high. Eventually, concurrent zombie tools will starve the CPU.

## Conclusion

The MCP ↔ VirtualStore boundary is highly susceptible to semantic failures (type panics) and temporal failures (goroutine leaks). The lack of explicit schemas for tool outputs means the `VirtualStore` is defensively programming against an infinitely variable external state.
