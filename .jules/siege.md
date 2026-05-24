## 2024-05-20 - Campaign Orchestrator & Session Executor Boundary Analysis
**Learning:** A known vulnerability exists at the Campaign Orchestrator and Session Executor boundary: concurrent inline JIT tasks managed by the Orchestrator mutate the shared `Executor` instance via `SetSessionContext` (which is not thread-safe), causing race conditions because `ExecuteAsync` is not utilized.
**Action:** The integration test must target this precise race condition and prove that `SetSessionContext` causes context bleed between concurrently executed tasks, validating the hypothesis that shared executor mutation during parallel execution is catastrophic.

## 2024-05-24 - Mangle VirtualStore Mutability Boundary
**Learning:** At the VirtualStore and MCP boundary, passing mutable maps directly to the MCP Integration Client via CallTool can cause data races if mutated concurrently during JSON serialization. Additionally, CallTool expects JSON-serializable primitives, not raw Mangle AST nodes. Mocks at this boundary often hide the fact that Mangle nodes don't serialize cleanly to JSON, and that the orchestrator/kernel might mutate facts while I/O is pending.
**Action:** Always deep-copy arguments before passing them across the FFI boundary to MCP, and assert that values are converted to Go primitives before dispatch. Test concurrency explicitly at this seam.
