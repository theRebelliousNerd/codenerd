---
surface: "SessionExecutor_VirtualStore_Kernel"
mode: "boundary"
subsystems_tested: ["SessionExecutor", "VirtualStore", "CortexKernel"]
blast_radius: "critical"
remediated: false
---

# Interaction Map

- SessionExecutor (universal loop) routes execution via `virtualStore.Execute(ctx, call)`.
- The VirtualStore evaluates actions against the Kernel using `kernel.Query("permitted(?)", action.Name)` or equivalent policy rules.
- Fact flow: Executor → Action Context → VirtualStore FFI → Kernel derivation of consequences.
- JIT config configures the Executor, and these properties must be enforced by the VirtualStore.
- The `Executor.Process` loop invokes LLM for tool calls, routing them through `virtualStore.Execute`. The VirtualStore then asserts `active_tool` facts into the `Kernel`, runs the tool via `TaskExecutor` or `ToolRegistry`, and asserts `tool_result` facts back into the `Kernel`.

# Contract Analysis

1.  **VirtualStore Isolation:** The VirtualStore must handle concurrent calls cleanly, protecting internal kernel state. Implicit contract: VirtualStore execution must be serialized or thread-safe relative to the Kernel's fact database.
2.  **Mangle Policy Validation:** The VirtualStore relies on the kernel for safety bounds. Implicit contract: VirtualStore must not bypass kernel policy checks; if the kernel is unresponsive, it defaults to deny.
3.  **Executor Error Handling:** The SessionExecutor loop relies on precise error propagation from the VirtualStore to maintain its state machine and provide feedback. Implicit contract: VirtualStore must return typed, checkable errors, not raw string panics, so the Executor can retry or fail gracefully.
4.  **Resource Limits:** High-volume parallel executions on the VirtualStore should hit a rate limit, not cause an OOM or deadlock in the kernel.
5.  **Fact Lifecycle:** Ephemeral facts asserted by the VirtualStore during tool execution must be retracted upon completion or failure. Implicit contract: No ghost facts leak across execution boundaries.
6.  **Type Safety:** VirtualStore must marshal Go values to strict Mangle AST nodes (e.g. `ast.String` vs `ast.Name`) correctly, otherwise the kernel's joins will fail silently.

# Failure Mode Enumeration

-   **Temporal:** Kernel is slow validating an action, causing context timeout during `virtualStore.Execute`. Does the SessionExecutor clean up orphaned facts?
-   **Semantic:** Mangle policy query returns syntactically valid but illogical results (e.g., action is both permitted and explicitly denied).
-   **Ordering:** Executor schedules Task A and B. B relies on A's side effects. If run concurrently via async task executor, does the VirtualStore serialize them correctly?
-   **Partial:** VirtualStore begins a multi-step action (e.g., read file + write graph fact) and fails midway. Are kernel facts rolled back?
-   **Corruption:** Concurrent goroutines asserting facts via VirtualStore FFI while Kernel is doing evaluation.
-   **Type Mismatch:** Passing `/active` vs `"active"` into VirtualStore fact generation leads to silent join failures in the kernel, making the system think the action is not permitted.

# Adversarial Scenarios

1.  *Contract:* Context cancellation stops execution immediately without leaking state.
    *Mechanism:* Cancel context mid-way through a long-running VirtualStore action (e.g., deep code scan).
    *Expectation:* Fails cleanly with ContextCanceled; kernel state does not reflect partial success. Severity: P1.
2.  *Contract:* Thread-safe VirtualStore tool execution.
    *Mechanism:* 50 goroutines call `Execute` on VirtualStore for the same kernel instance simultaneously.
    *Expectation:* No race conditions, deadlocks, or panicked goroutines. Severity: P0.
3.  *Contract:* Invalid Mangle schema inputs from VirtualStore are rejected.
    *Mechanism:* VirtualStore attempts to assert a fact with raw bytes or invalid AST strings.
    *Expectation:* Kernel returns explicit type mismatch error; VirtualStore propagates this back to Executor. Severity: P1.
4.  *Contract:* VirtualStore handles panic recovery from poorly formed external tools.
    *Mechanism:* Tool execution (via FFI) panics internally.
    *Expectation:* VirtualStore catches the panic, converts to an error, and returns it to the Executor; system stays stable. Severity: P0.
5.  *Contract:* Strict fallback for unavailable tools.
    *Mechanism:* Executor requests a tool execution that is restricted by JIT config but available in the registry.
    *Expectation:* VirtualStore enforces the restriction derived from the kernel or config, returning an explicit denial error. Severity: P2.
6.  *Contract:* Fact cleanup after execution failure.
    *Mechanism:* Inject a fault in the VirtualStore execution path after it asserts `active_tool`.
    *Expectation:* VirtualStore `defer` logic retracts `active_tool` even on failure. Severity: P1.
7.  *Contract:* Bounded context expansion.
    *Mechanism:* Executor passes a 10MB string as an argument to a tool.
    *Expectation:* VirtualStore truncates or rejects the argument before passing it to the kernel to prevent OOM. Severity: P1.
8.  *Contract:* Mangle type safety (Atom vs String).
    *Mechanism:* Pass a Go string that looks like an atom (`"/my_atom"`) to the VirtualStore.
    *Expectation:* It must be marshalled as `ast.String`, not `ast.Name`, preventing malicious type coercion. Severity: P0.
9.  *Contract:* Serialized access to mutable FFI state.
    *Mechanism:* Mutate a shared map passed as an argument to the VirtualStore while it is executing.
    *Expectation:* VirtualStore copies arguments or uses synchronization; no concurrent map read/write panics. Severity: P0.
10. *Contract:* Executor halts on irrecoverable VirtualStore error.
    *Mechanism:* VirtualStore returns a fatal database corruption error.
    *Expectation:* Executor aborts the loop rather than retrying blindly and exacerbating the corruption. Severity: P1.
11. *Contract:* Handling of zero-result queries.
    *Mechanism:* Kernel policy evaluates to an empty result due to disjoint types.
    *Expectation:* VirtualStore treats this as a denial, not a success. Severity: P1.
12. *Contract:* Prevention of goroutine leaks.
    *Mechanism:* Executor calls VirtualStore, VirtualStore spins up a streaming goroutine, Executor cancels context.
    *Expectation:* Streaming goroutine respects context and exits, no leaks. Severity: P0.
13. *Contract:* Isolation of ephemeral facts between sessions.
    *Mechanism:* Executor runs Session A and Session B concurrently using the same VirtualStore but different subagents.
    *Expectation:* Facts from Session A do not influence Session B's policy evaluation. Severity: P0.
14. *Contract:* Handling of cyclical dependencies in tool execution.
    *Mechanism:* Executor calls a tool that recursively calls the VirtualStore to execute another tool.
    *Expectation:* VirtualStore detects the cycle or enforces a depth limit to prevent stack overflow. Severity: P1.
15. *Contract:* Correct serialization of Piggyback control packets.
    *Mechanism:* Executor receives a Piggyback packet with malformed JSON and passes it to the VirtualStore for processing.
    *Expectation:* VirtualStore rejects it gracefully without corrupting kernel state. Severity: P2.

# Cascading Failure Analysis

If the VirtualStore fails to safely serialize actions or handle panics, the CortexKernel state becomes corrupted. This corrupted state leads to incorrect future policy evaluations. For instance, if a tool execution panics and the panic is caught but the associated cleanup logic is skipped, ephemeral facts (like `active_tool`) might become orphaned in the kernel. This ghost fact tells the Executor that the tool is still running, leading to fixpoint deadlocks where the session hangs indefinitely waiting for a result that will never come, or the TDD loop enters an infinite retry cycle. The Executor relies heavily on deterministic VirtualStore boundaries to decide whether to advance the phase, replan, or fail the session. A failure here cascades up to the orchestrator, stalling the entire campaign.
