## 2024-05-20 - Campaign Orchestrator & Session Executor Boundary Analysis
**Learning:** A known vulnerability exists at the Campaign Orchestrator and Session Executor boundary: concurrent inline JIT tasks managed by the Orchestrator mutate the shared `Executor` instance via `SetSessionContext` (which is not thread-safe), causing race conditions because `ExecuteAsync` is not utilized.
**Action:** The integration test must target this precise race condition and prove that `SetSessionContext` causes context bleed between concurrently executed tasks, validating the hypothesis that shared executor mutation during parallel execution is catastrophic.

## 2026-05-21 - MCP VirtualStore Boundary Data Race
**Learning:** The VirtualStore passes mutable maps (arguments) directly to the MCP Integration Client via `CallTool`. The `MCPClientManager` then attempts to serialize these arguments using `json.Marshal`. If a concurrent process modifies the map during marshaling, it triggers a data race (or panic) and corrupts the payload sent to the external MCP server. Furthermore, the `CallTool` function expects purely JSON-serializable primitives, but the VirtualStore may pass Mangle AST nodes.
**Action:** Always assert the type-safety and thread-safety of argument maps passed across the FFI boundary. Verify that data races on map mutation are caught during testing with `-race`.

## 2026-05-21 - IntegrationAdapter Nil Result Panic Potential
**Learning:** `IntegrationAdapter.CallTool` assumes that `manager.CallTool` returns a non-nil result when `err` is nil. If `manager.CallTool` returns `(nil, nil)`, `IntegrationAdapter` will panic when trying to access `result.Success` or `result.Output`.
**Action:** Add defensive nil checks for result pointers returned from the MCP Client Manager before dereferencing them in the adapter.

## 2026-05-21 - Stdio Transport Zombie Process Risk
**Learning:** `IntegrationClient.CallTool` receives a context. If the context is cancelled (e.g., via session timeout), the cancellation might not correctly propagate to Stdio transport child processes.
**Action:** When testing external tool integrations, assert that context cancellation immediately unblocks the caller and does not leave zombie processes consuming system resources.
