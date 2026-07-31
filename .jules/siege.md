## 2024-07-21 - MCPIntegrationBridge Database Lifecycle and Null Store Handing
**Learning:** The `MCPIntegrationBridge` internally manages multiple interdependent subsystems (compiler, store, manager). If `NewMCPIntegrationBridge` receives an invalid database path that fails `NewMCPToolStore`, it correctly aborts and returns the error. However, if a failure were to occur *after* the store is created (e.g. failing to initialize a compiler or an analyzer), the store could leak if not explicitly closed. Additionally, testing integration components deeply relies on properly mocking or instantiating these concrete dependencies to avoid nil dereference panics when methods like `CompileToolsForShard` are called with an empty or non-existent store.
**Action:** When adding tests for composite managers like `MCPIntegrationBridge`, always verify both valid integration (using temporary file systems or in-memory DBs) and explicit failure pathways that gracefully handle sub-component breakdown without cascading panics. Ensure `bridge.Close()` pathways verify proper state teardown.

## 2025-07-19 - VirtualStore Synchronous Mangle Block
**Learning:** The `VirtualStore` boundary explicitly discards the execution `context.Context` when querying the World Model via `query_graph/3`. Mangle evaluations (which are CPU-bound and synchronous) rely on this virtual predicate. If the underlying SQLite graph blocks on a write lock, the entire Mangle derivation loop hangs indefinitely, causing a thread starvation cascade in the Session Executor that cannot be cancelled by the user.
**Action:** When mocking virtual predicates or adding external integrations to Mangle, always require an explicit context parameter to prevent execution deadlocks.

## 2024-12-28 - VirtualStore Dreamer Cache Collision
**Learning:** The `DreamCache` key design uses only `ActionType + Target` (e.g., `write_file:config.json`), completely ignoring the `Payload`. This creates a catastrophic semantic bypass where a benign write caches a 'Safe' verdict, allowing a subsequent malicious write to the same target to bypass the kernel safety gate entirely.
**Action:** Always include a hash of the payload in the cache key for stateful safety gates, or explicitly separate caching mechanisms from semantic policy evaluations.

## 2024-12-27 - Prompt Compiler Γò¼├┤Γö£├æΓö£Γòó LLM Client Boundary
**Learning:** The `TokenBudgetManager` acts as the critical throttle between the JIT-assembled atoms and the LLM's physical context window limit. An oversight here (such as truncating a string halfway through a UTF-8 character or misallocating resources in concurrent calls) directly causes the downstream `LLMClient` to panic or receive a `400 Bad Request`.
**Action:** Enforce strict UTF-8 validity checks and bounds truncation at the very edge of the prompt assembler, treating the TokenBudgetManager as a defensive firewall rather than a simple string trimmer.

## 2024-12-27 - Stale Facts in Cross-Boundary Pipelines
**Learning:** If an LLM call fails mid-stream in the pipeline, the facts asserted into the kernel during the earlier JIT intent classification may linger, causing "ghost context" in subsequent interactive turns.
**Action:** The pipeline MUST include an explicit rollback or retracting deferred function that clears transient `next_action` or `current_intent` facts if the downstream LLM boundary encounters an error.

## 2024-05-26 - VirtualStore FFI and Mangle Type Mismatches
**Learning:** When the `Session Executor` requests tool execution via the `VirtualStore`, the `VirtualStore` must translate tool arguments into Mangle Atoms to assert facts in the kernel. If a tool argument is passed as a generic string instead of a strict Mangle `ast.String` or `ast.Name`, the kernel join silently fails (returns 0 results) rather than erroring out, leading the Executor to believe an action is permitted when it isn't (or vice versa).
**Action:** Write boundary tests that explicitly inject Mangle type confusion (e.g., passing `/string` instead of `"string"`) at the VirtualStore FFI layer to ensure the type-checker catches the error before policy evaluation.

## 2026-06-17 - Prompt Compiler and LLM Client Streaming Lifecycles
**Learning:** The boundary between the `Prompt Compiler` and `LLM Client` involves passing heavily constructed, massive strings (`CompiledContext`). However, if the network stream breaks or the context times out during the `LLMClient.Stream` phase, goroutines can leak if they don't explicitly listen to `ctx.Done()` alongside the slow network channel. A single streaming failure can leave the `Compiler` state hanging or leak resources across multiple turns.
**Action:** When testing the `Compiler` -> `LLMClient` pipeline, always inject simulated network latency and test `context.Cancel()` exactly mid-stream to ensure the boundary guarantees fast teardown.

## 2026-06-15 - Prompt Compiler and LLM Client Integration
**Learning:** The boundary between the Prompt Compiler and the LLM Client hides a critical structural constraint: token budget enforcement dictates not just cost, but protocol survival. If the prompt compiler estimates tokens incorrectly or over-allocates to the prompt, the LLM client might abruptly truncate the piggyback JSON structure. This results in the articulation system receiving fragmented control packets, leading to a cascading failure where the entire pipeline crashes out because the intended `next_action` state was lost in truncation.
**Action:** Always allocate distinct overhead tokens for protocol metadata (like Piggyback packets) separate from the general output budget. Tests should deliberately push the total token count exactly to the threshold limit to ensure the truncating logic inside `TokenBudgetManager` prioritizes essential system instructions and JSON structure over variable length context.

## 2023-10-27 - [Session to Kernel Fact Isolation]
**Learning:** The new architecture removes Shard-specific kernels and relies on a shared `core.Kernel`. Concurrent SubAgents using the same Kernel must use session-isolated facts or transient scopes, otherwise `user_intent` from one request can bleed into the routing of another request.
**Action:** Always write tests that run concurrent Executors asserting conflicting facts to verify cross-talk doesn't occur.

## 2024-07-02 - Implicit Fail-Closed Contract in Dreamer
**Learning:** The VirtualStore relies on the Dreamer's fail-closed behavior (returning `Unsafe: true` on bad inputs like nil context or oversized targets) to prevent execution. If `SimulateAction` were to panic or hang instead of returning a valid `DreamResult`, `RouteAction` might fail to block the action gracefully, potentially crashing the entire action routing pipeline.
**Action:** Always test the extreme boundary cases (nil contexts, massive strings) in `SimulateAction` to ensure the fail-closed contract holds and doesn't cascade into a panic.

## 2024-07-02 - Fact Injection Assumption
**Learning:** When the Dreamer blocks an action, the VirtualStore unconditionally injects `security_violation` and `dream_blocked_action` facts. It assumes the underlying kernel will accept these without issues. If the kernel's schema is strict or state is corrupted, this injection could fail silently or panic, breaking the feedback loop for the learning subsystems.
**Action:** Integration tests must verify that the facts are not just "sent" but are actually retrievable from the kernel after a blocked action.

## 2026-06-23 - APIScheduler Γò¼├┤Γö£├æΓö£Γòó ScheduledLLMCall Context Fallback Contract
**Learning:** In `core/api_scheduler.go`, the `AcquireAPISlot` method implements a defensive fallback to catch TOCTOU (Time-Of-Check to Time-Of-Use) race conditions when context cancellation perfectly overlaps with slot acquisition. If the `select` statement arbitrarily chooses the `<-waitCtx.Done()` path, the system could leak a slot because the releaser already incremented `currentlyExecuting`. The secondary nested `select { case <-w: ... }` forces the system to acknowledge the acquired slot and ignore the cancellation.
**Action:** When testing cross-boundary cancellation, you must simulate this exact microsecond overlap. Standard `context.Cancel()` testing will completely miss this because the Go scheduler won't reliably hit the race. You must use mock blockers (channels) to guarantee the releaser and the canceller fire simultaneously to prove the fallback prevents permanent API slot starvation.

## 2024-12-27 - Prompt Compiler ╬ô├Ñ├╢ LLM Client Boundary
**Learning:** The `TokenBudgetManager` acts as the critical throttle between the JIT-assembled atoms and the LLM's physical context window limit. An oversight here (such as truncating a string halfway through a UTF-8 character or misallocating resources in concurrent calls) directly causes the downstream `LLMClient` to panic or receive a `400 Bad Request`.
**Action:** Enforce strict UTF-8 validity checks and bounds truncation at the very edge of the prompt assembler, treating the TokenBudgetManager as a defensive firewall rather than a simple string trimmer.

## 2024-12-28 - TDDLoop and VirtualStore Boundary
**Learning:** If the TDDLoop generates a `next_action` fact using `ast.String` instead of `ast.Name` for the tool name (e.g. `"/edit_file"` vs `"/edit_file"`), the VirtualStore might silently reject it, and the `TDDLoop` transitions to a next state without actually mutating the file system, causing an infinite testing loop.
**Action:** When testing the `TDDLoop` -> `VirtualStore` pipeline, write boundary tests that enforce the correct Mangle types, or simulate empty LLM patch generations to ensure the TDD loop correctly escalates.

## 2024-12-28 - TDDLoop Thread Safety and Resets
**Learning:** The session orchestrator might `Reset()` the `TDDLoop` asynchronously (e.g., if a user aborts via chat). If `RunToCompletion()` is modifying slice states (like `patches` or `diagnostics`) without holding `t.mu`, it will data race and crash the engine.
**Action:** Always test `Reset()` concurrently with `RunToCompletion()` in E2E tests, verifying that internal mutexes hold and prevent state corruption.

## 2024-07-05 - VirtualStore Interactive Gate vs Dreamer Cache Collision
**Learning:** The Dreamer's cache implementation in `internal/core/dreamer.go` uses `string(req.Type) + ":" + req.Target` as the cache key. This completely ignores the `req.Payload`. Two concurrent interactive tool calls (e.g. `write_file`) modifying the same file with different content will collide, potentially allowing a malicious payload to bypass safety checks by reusing the cache entry of a benign payload.
**Action:** When testing the VirtualStore Γåö Dreamer boundary, always construct concurrent races that exploit cache key collisions (same type + target, different payload).

## 2024-07-06 - Campaign Phase Boundary Context Paging Overflow
**Learning:** If a SubAgent spawned for a task generates a massive output (e.g., a huge log or data dump), and the Orchestrator passes that output forward as context to the next phase without bounded semantic compression or truncation, the resulting combined state will overflow the TokenBudgetManager for the next phase. This causes the downstream LLM context compilation to panic or fail unexpectedly.
**Action:** When orchestrating multi-phase execution, the pipeline must strictly bound the phase output context state before transitioning and passing it forward as input, never assuming arbitrary text fits.

## 2026-07-31 - [Spawner ↔ APIScheduler Integration Assumptions]
**Learning:** The Spawner's `maxActiveSubagents` configuration does not map 1:1 to the APIScheduler's `MaxConcurrentAPICalls`. The Spawner assumes the APIScheduler's wait queues are unbounded and will properly honor context cancellation for queued agents. If the APIScheduler fails to evict timed-out waiters from its queues, it triggers a system-wide deadlock, as the Spawner's subagents count against its capacity even while waiting.
**Action:** When stress-testing resource orchestration boundaries, always test queue eviction under heavy concurrency and timeout conditions, not just max capacity limits.
