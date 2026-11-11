---
surface: "ShardManager_JITExecutor"
mode: "boundary"
subsystems_tested: ["ShardManager", "JITExecutor"]
blast_radius: "critical"
remediated: false
---

# ShardManager to JITExecutor Delegation Seam Analysis

## System Interaction Map
The core boundary investigated is `ShardManager.SpawnAsyncWithContext` (in `internal/core/shards/manager_spawn.go:262-289` / `:295`) interacting with `JITExecutor.Execute` (via `TaskDelegator` in `internal/session/task_executor.go`).
*   `Cortex.SpawnTask` (or `SpawnTaskWithTarget`) calls `ShardManager.Spawn`.
*   If `ShardManager` cannot find a factory for the requested `ShardType` (e.g., domain personas like `coder` or user-defined agents), it delegates to the "clean loop" (the `TaskDelegator` interface).
*   The `TaskDelegator` is wired at boot via `ShardManager.SetTaskDelegator`.
*   The actual interface `TaskDelegator` expects an `Execute(ctx context.Context, intentVerb, task string) (string, error)` signature.
*   The adapter `newDelegatedShardAgent` (`internal/core/shards/manager_spawn.go:304`) bridges `types.ShardAgent`'s `Execute` to `TaskDelegator.Execute`.

## Contract Analysis
**Implicit contracts:**
1.  **State Management:** The delegated agent (`newDelegatedShardAgent`) must manage its own state (`Running`, `Completed`, `Failed`) correctly even though it executes in a separate executor. If the delegated agent doesn't transition to `Completed` on success or `Failed` on error, the `ShardManager` might track it as a zombie.
2.  **Cancellation:** Context cancellation passed into `ShardManager.Spawn` must propagate correctly through `newDelegatedShardAgent` down into `TaskDelegator.Execute` and abort the JIT loop without leaving orphaned state.
3.  **Hollow Output Fallback:** As noted in the instructions, an old bug caused empty/fake execution strings to be returned. `TaskDelegator.Execute` must return the *actual* result string or a real error, never a fake success like `"BaseShardAgent execution"`.
4.  **Error Propagation:** Panic or typed errors in `JITExecutor` must cross back across the `TaskDelegator` seam and return cleanly to `ShardManager`.
5.  **Concurrency / Starvation:** If many non-system shards are spawned, `ShardManager` delegates them. The `JITExecutor` (which internally handles task execution) must not deadlock if it tries to spawn something else or if resources (like LLM slots) are constrained.

## Failure Mode Enumeration
*   **Temporal:** `JITExecutor` takes a long time (simulating a slow LLM call); context is cancelled at the `ShardManager` layer. Does `JITExecutor` stop? Does `ShardManager` correctly un-track the delegated agent?
*   **Semantic:** The `TaskDelegator` is asked to execute a task for an invalid persona that doesn't exist in JIT atoms. How does the failure bubble up?
*   **Ordering:** A task is queued, started, but the session shuts down before it completes.
*   **Corruption:** `ShardManager` state map gets corrupted because the delegated agent updates its state without locks (or incorrectly).
*   **Resource:** Spawning 100 delegated shards simultaneously.

## Adversarial Scenario Design
1.  **Scenario: Delegate Non-Existent Persona**
    *   *Violated Contract:* Semantic contract (persona exists).
    *   *Injection:* Ask `ShardManager` to spawn a `ShardTypeUser` with `Name="GhostAgent"` where `GhostAgent` has no JIT DB.
    *   *Expected Behavior:* The `JITExecutor` fails to compile the prompt or fails early; error bubbles back up to `ShardManager`, and the delegated agent goes to `Failed` state.
    *   *Severity:* P2
2.  **Scenario: Context Cancellation Mid-Flight**
    *   *Violated Contract:* Temporal/Cancellation contract.
    *   *Injection:* Cancel context exactly 10ms after spawning the delegated task.
    *   *Expected Behavior:* The `JITExecutor` correctly aborts. The returned error is `context.Canceled`. The agent transitions to `Failed`.
    *   *Severity:* P1
3.  **Scenario: Hollow Output Re-Emergence Check**
    *   *Violated Contract:* Semantic contract (no fake success).
    *   *Injection:* Bypass normal factory (ensure it delegates) and run a task that the `JITExecutor` trivially fails on.
    *   *Expected Behavior:* Should return an error, NOT `"BaseShardAgent execution"` or any fake success.
    *   *Severity:* P0
4.  **Scenario: Panic in JIT Executor**
    *   *Violated Contract:* Semantic/Safety contract.
    *   *Injection:* Inject a mock `TaskDelegator` that panics.
    *   *Expected Behavior:* The panic is either caught by the delegated agent adapter and turned into an error, or caught by the `ShardManager` recovery middleware. If not caught, blast radius analysis applies.
    *   *Severity:* P1
5.  **Scenario: Concurrent Spawn Flood (Resource Exhaustion)**
    *   *Violated Contract:* Resource/Concurrency contract.
    *   *Injection:* Spawn 100 delegated agents concurrently.
    *   *Expected Behavior:* No deadlock, no state corruption in `ShardManager`'s agent tracking.
    *   *Severity:* P1
6.  **Scenario: Delegated Agent State Corruption**
    *   *Violated Contract:* State machine invariants.
    *   *Injection:* Observe the `ShardState` of the returned `types.ShardAgent` while it's executing, ensure it transitions `Pending` -> `Running` -> `Completed`/`Failed` appropriately, without race conditions.
    *   *Severity:* P2
7.  **Scenario: Image Shard Delegation Refusal**
    *   *Violated Contract:* Semantic routing contract (Image shards MUST NOT delegate).
    *   *Injection:* Attempt to spawn an `image_generator` shard with no factory registered.
    *   *Expected Behavior:* `ShardManager.SpawnAsyncWithContext` should immediately return a specific error refusing to delegate image shards to the worker LLM.
    *   *Severity:* P1
8.  **Scenario: Nil Task Delegator**
    *   *Violated Contract:* Initialization contract.
    *   *Injection:* Do not call `SetTaskDelegator`, then attempt to spawn a user persona.
    *   *Expected Behavior:* Specific error indicating "no factory registered and no task delegator wired".
    *   *Severity:* P2
9.  **Scenario: Cross-Boundary Context Leak**
    *   *Violated Contract:* Context isolation contract.
    *   *Injection:* Inject a value into the context at the `ShardManager` layer; verify it is correctly received by the `TaskDelegator`.
    *   *Severity:* P3
10. **Scenario: Cascading Failure from LLM client in JIT Executor**
    *   *Violated Contract:* Error propagation contract.
    *   *Injection:* `TaskDelegator` returns an LLM timeout error.
    *   *Expected Behavior:* The error crosses the `delegatedShardAgent` boundary intact.
    *   *Severity:* P2
11. **Scenario: Delegated Agent Stop Method**
    *   *Violated Contract:* Cancellation via explicit `.Stop()` method instead of context cancellation.
    *   *Injection:* Call `agent.Stop()` on the returned delegated agent while it's running.
    *   *Expected Behavior:* It should cancel the execution context internally and cause the `Execute` call to return an error.
    *   *Severity:* P1
12. **Scenario: Delegated Agent Permission Check**
    *   *Violated Contract:* Permission propagation.
    *   *Injection:* Check if the delegated agent correctly reports permissions (often inherited from its `BaseShardAgent` wrapper).
    *   *Severity:* P3
13. **Scenario: Success State Transition Verification**
    *   *Violated Contract:* Success state machine invariant.
    *   *Injection:* The `TaskDelegator` completes successfully with a string result.
    *   *Expected Behavior:* The returned string exactly matches; the delegated agent's state is strictly `Completed`.
    *   *Severity:* P0
14. **Scenario: State Change Race Condition**
    *   *Violated Contract:* Memory safety invariant.
    *   *Injection:* Read `agent.GetState()` repeatedly in a separate goroutine while the `TaskDelegator` is executing and finishing.
    *   *Expected Behavior:* No data race (verify with `-race`).
    *   *Severity:* P1
15. **Scenario: Slow Task Delegator with Timeout**
    *   *Violated Contract:* Temporal contract.
    *   *Injection:* Use a context with a timeout; `TaskDelegator` sleeps past the timeout.
    *   *Expected Behavior:* `Execute` should return a timeout error; state should be `Failed`.
    *   *Severity:* P1

## Cascading Failure Analysis
If `ShardManager` successfully delegates a task to `JITExecutor`, but the `JITExecutor` returns a completely hollow output (like `"BaseShardAgent execution"`) or drops errors, the system cascade is catastrophic:
- The system thinks the persona (e.g. `coder`) has completed the task successfully.
- The `CampaignOrchestrator` marks the phase as "Done".
- No actual work was committed.
- The `Articulation` layer tells the user "I'm done" while presenting no diffs.
If the `delegatedShardAgent` fails to update its state to `Completed` or `Failed` internally because it crashes or hangs, the `ShardManager`'s internal maps will bloat (`s.subagents` in `manager.go`?), leading to memory leaks over time during long sessions or large campaigns.

## Detailed Blast Radius Analysis for Scenarios

### Temporal Failure Blast Radius
If a temporal failure occurs (Scenario 2, Scenario 15) during delegation, the ShardManager has successfully tracked the request internally. If the JITExecutor does not respect context cancellation, it might hold the Goroutine or API slot indefinitely. The downstream blast radius involves Campaigns stalling forever because a specific persona task is marked 'Pending' instead of 'Failed'.

### Semantic Boundary Contract Cascade
When `GhostAgent` is requested but doesn't exist (Scenario 1), the immediate consequence is a JIT compilation failure. However, if the error routing logic in `newDelegatedShardAgent` drops this error, `ShardManager` logs it as completed but with 0 facts derived. Downstream, the `CampaignOrchestrator` consumes the "completion", but finds no state changes. The orchestrator may immediately transition to the next phase, assuming work is done, executing subsequent operations on empty context.

### Hollow Output Cascade (P0)
If the JIT Executor accidentally returns the old `BaseShardAgent` success string (Scenario 3) rather than executing:
1. `ShardManager` records `types.ShardStateCompleted`.
2. The Session's World Model receives a completion notification.
3. The Articulation layer says "I'm done" to the user.
4. No rules are learned, no files are patched.
5. The system behaves like a complete success, meaning the TDD repair loop (which runs on `test_state(/failing)`) is bypassed because the patch step "succeeded".

### Resource Exhaustion Cascade
In Scenario 5, if `ShardManager` accepts 100 spawns and queues them all for delegation, the `JITExecutor`'s worker pool can become saturated. If the API rate limits (e.g., Anthropic or Gemini) are hit, they return 429s. If the delegator doesn't have robust backoff strategies, the 429s bubble back to `ShardManager` and immediately fail the entire fleet of agents. This breaks the Campaign Orchestrator which expects retries on transient API errors, not instant fleet-wide failure.

### Error Propagation Cascade
If an error from the JIT LLM generation is returned as a raw un-typed string instead of a structured error (Scenario 10), the `ShardManager` cannot differentiate between an API timeout and a context window exhaustion. Thus, it cannot escalate properly. The `Session` relies on precise typed errors to determine if it should retry the turn or inform the user. A generic error halts the loop entirely.

### Concurrency Corruption Cascade
In Scenario 14, if `ShardManager`'s state map is mutated without proper mutex protection while the JIT Executor updates the status asynchronously:
1. The pointer to the `types.ShardAgent` can become nil or corrupted.
2. An orchestrator querying `sm.GetActiveShards()` panics on dereference.
3. The entire background service loop crashes, taking down the entire `codeNERD` server process, requiring a manual restart and resulting in total session state loss.

### Additional Contract: Budget Management
The JITExecutor is responsible for managing the token budget. If `ShardManager` delegates a task with an extremely large context (e.g., 50 files), the `JITExecutor` might fail budget enforcement.
- If it fails open, the LLM rejects the request (400 Bad Request).
- This 400 bubbles back to ShardManager, failing the agent.

### Mitigation & Architectural Remediation Thoughts
To fix the exposed integration cracks, we need:
1. A strongly typed error translation layer at the `TaskDelegator` boundary.
2. A strict prohibition on `ShardStateCompleted` unless specific success criteria (like actual derived facts) are met.
3. Hard context propagation bounds (with time budgets) explicitly passed from `ShardManager` down through the JIT Executor.

### Deep Dive: Delegation Seam State Machine
The state machine for `types.ShardAgent` under `TaskDelegator` is uniquely fragile. A standard system shard handles its own state updates via `Execute()`. A delegated shard wraps `Execute()` around an external call. This means there is a moment between `JITExecutor` returning and `newDelegatedShardAgent` updating its struct where the state is inconsistent with reality.

### Deep Dive: Logging Ambiguity
When delegation occurs, logs are split. `ShardManager` logs to `logging.CategoryShards`, while the JITExecutor logs to `logging.CategorySession`. If an integration failure occurs mid-flight, debugging requires cross-correlating timestamps between these two streams. There is no shared correlation ID injected by `ShardManager` that the `JITExecutor` respects.

### Expanded Scenario Details
#### Scenario 16: Zero Tool Availability
*Violated Contract:* Tool Injection Contract.
*Injection:* ShardManager passes a task that absolutely requires filesystem tools, but the Delegator provides a config with zero tools allowed.
*Expected Behavior:* The task fails gracefully, reporting "I cannot complete this because I lack required tools", rather than hallucinating paths.

#### Scenario 17: Out of Order Completion
*Violated Contract:* Ordering.
*Injection:* Spawn A then B. B finishes first.
*Expected Behavior:* ShardManager must not assume FIFO processing for delegated tasks. Results map must handle arbitrary completion orders.

#### Scenario 18: Orphaned Delegation Result
*Violated Contract:* Memory lifecycle.
*Injection:* Spawn delegated task, wait for completion, but NEVER call `sm.GetResult()`.
*Expected Behavior:* Result should eventually expire or be cleaned up, not leak memory indefinitely.

#### Scenario 19: Double Cancellation
*Violated Contract:* Reentrancy.
*Injection:* Cancel context twice rapidly.
*Expected Behavior:* No panic. Subsequent cancellations are no-ops.

#### Scenario 20: Panic during Result Marshalling
*Violated Contract:* Safe Boundary Transition.
*Injection:* JITExecutor succeeds but panics while serializing the result string.
*Expected Behavior:* ShardManager's `newDelegatedShardAgent` must catch the panic via `defer recover()` and return it as a typed error.

### Extended Scenario Breakdown

#### Scenario 21: JIT Prompt Compilation Timeouts
*Violated Contract:* The boundary's tolerance for slow generation phases prior to the LLM call.
*Injection:* Delay the JIT Compilation mock by exactly the context timeout duration minus 1ms.
*Expected Behavior:* The operation should either complete barely in time or cleanly cancel. ShardManager shouldn't throw arbitrary "unknown state" errors.

#### Scenario 22: High Token Budget Overflow
*Violated Contract:* Token budget management across boundaries.
*Injection:* Provide a user request that compiles to an extremely large prompt, larger than the token budget allows, but which JITExecutor truncates poorly.
*Expected Behavior:* JITExecutor correctly handles truncation without causing a silent drop of critical control structures. If it cannot, it returns an explicit error to ShardManager indicating budget violation.

#### Scenario 23: Incomplete JSON Parsing in Delegated Action
*Violated Contract:* Valid structural return formats for internal state.
*Injection:* JITExecutor receives a malformed JSON from the LLM (simulate via mock) and fails to parse it.
*Expected Behavior:* The parse error is gracefully wrapped as a Typed Error and returned to ShardManager. It must not panic the JIT loop.

#### Scenario 24: Missing VirtualStore Instance
*Violated Contract:* Execution prerequisites.
*Injection:* ShardManager initializes but the environment lacks a configured VirtualStore, and a delegated task requires executing an effectful tool.
*Expected Behavior:* The tool execution fails at the pre-flight check inside the JIT Executor. The failure propagates gracefully across the boundary, putting the agent into `ShardStateFailed` rather than hanging indefinitely.

#### Scenario 25: Ouroboros Loop Triggers Within JIT Executor
*Violated Contract:* Bounded execution time and recursive limits.
*Injection:* A task delegated from ShardManager triggers the Autopoiesis (self-modification) loop, which loops repeatedly attempting to fix code.
*Expected Behavior:* The recursive loop within the JITExecutor respects its execution boundaries (e.g. max retries). Once limits are hit, it surfaces the error to ShardManager rather than starving the background goroutine pool indefinitely.

#### Scenario 26: Concurrent Read/Write on Shared State
*Violated Contract:* Thread safety of shared session context.
*Injection:* Two delegated tasks simultaneously try to update the shared SessionContext's Blackboard map.
*Expected Behavior:* The session context should either employ a mutex or be strictly immutable per turn. Any race condition caught by the Go race detector fails this test.

#### Scenario 27: Stale Knowledge Base Synchronization
*Violated Contract:* JIT synchronization limits.
*Injection:* ShardManager invokes a user-defined agent right after its underlying `prompts.yaml` is modified, but before `promptSyncer` has completed.
*Expected Behavior:* The `promptSyncer` fails or delays. ShardManager gracefully falls back to the existing Knowledge Base (KB) atoms rather than failing the spawn entirely, ensuring high availability at the cost of slight eventual consistency.

#### Scenario 28: Agent Re-initialization under Load
*Violated Contract:* Safe lifecycle teardown.
*Injection:* Terminate and re-initialize the same user agent rapidly 100 times via `ShardManager.SpawnAsyncWithContext`.
*Expected Behavior:* Previous instances correctly detach and garbage collect. Their context cancellation signals are respected, and no goroutine leaks occur.

#### Scenario 29: Cross-Turn Dependency Failure
*Violated Contract:* Temporal continuity of multi-step processes.
*Injection:* A Campaign phase requires two sequential tasks. Task 1 is delegated and succeeds, writing to the VirtualStore. Task 2 is delegated but fails to read the expected output due to synchronization lag.
*Expected Behavior:* The JITExecutor implements retry or verification steps natively, or surfaces a descriptive missing-dependency error to the ShardManager so the Campaign Orchestrator can retry phase 2.

#### Scenario 30: Excessive Diagnostic Error Feed
*Violated Contract:* Boundary payload size limits.
*Injection:* A delegated compiler task generates a 50MB log file of errors. The JITExecutor attempts to parse and return this as the result string.
*Expected Behavior:* The JITExecutor truncates the output before crossing the boundary back to ShardManager to prevent OOM errors and excessive string copy operations.

### Conclusion
The ShardManager and JITExecutor boundary is a critical juncture where long-lived background orchestration meets ephemeral, JIT-compiled LLM interactions. The legacy architectural decision to use `BaseShardAgent` as a hollow fallback created a severe blind spot for un-implemented personas, resulting in false positives for task completions. By strictly enforcing the `TaskDelegator` contract, explicitly routing errors, and verifying correct state machine transitions (Pending -> Running -> Completed/Failed), the system's robustness is significantly enhanced. The adversarial scenarios designed herein specifically target the asynchronous nature of this boundary, ensuring that cancellation, panics, and resource constraints do not corrupt the orchestrator's state or lead to silent failures.




### Cross-System Data Transfer and Persistence

When `ShardManager` delegates a task to `JITExecutor`, the state and result must traverse several serialization boundaries:
1. **Request Formulation:** The `task` string and `sessionCtx` are passed into `ShardManager.SpawnAsyncWithContext`. These parameters must be preserved unaltered across the `TaskDelegator` boundary.
2. **JIT Compilation:** The `JITExecutor` converts the request into Prompt Atoms based on the `ShardType` (Persona). If `ShardManager` allows a `ShardType` that doesn't have corresponding atoms in the Knowledge Base (KB), the `JITExecutor` falls back to empty or default prompts, meaning the LLM lacks identity context.
3. **Execution Output:** The LLM's raw response, which could contain arbitrary tokens, code blocks, or embedded JSON (Piggyback Protocol), is parsed by the `JITExecutor`. If the `JITExecutor` returns the raw string rather than structuring errors out of it, `ShardManager` assumes success regardless of the payload's content.

### Deep Dive: `context.Context` Propagation and Goroutine Leaks
The use of `context.Context` across the `ShardManager` / `JITExecutor` boundary is a classic concurrency pitfall.
* `ShardManager` creates the context, possibly wrapping it in a timeout or cancellation handle.
* It passes this context to `newDelegatedShardAgent`, which passes it to `TaskDelegator.Execute`.
* The `JITExecutor` must explicitly respect this context. If it ignores `ctx.Done()`, and blocking operations (like LLM network calls or database writes) take longer than the timeout, the `ShardManager` might detach and consider the agent failed, but the underlying Goroutine continues running, leaking memory and network connections.
* If 100 delegated shards are spawned and then cancelled, 100 Goroutines will leak if the boundary doesn't aggressively poll `ctx.Err()`.

### State Machine Transition Weaknesses
The `ShardManager` tracks states: `Pending`, `Running`, `Completed`, `Failed`.
For a standard system shard, `sm` updates these via `recordResult` after the execution returns.
For a delegated shard:
* Transition to `Running` occurs when `TaskDelegator.Execute` is called.
* Transition to `Completed` or `Failed` depends ENTIRELY on the return values of `Execute`.
* If `Execute` panics, and the adapter does not `recover()`, the state machine remains permanently in `Running`, creating zombie agents that block cleanup and appear indefinitely active in the `/sessions` tracking.

### Campaign Orchestrator Deadlocks
The Campaign Orchestrator issues tasks sequentially per phase. If a phase delegates a task to the `JITExecutor`, it blocks waiting for completion.
* If `ShardManager` loses track of the delegated agent because of state machine corruption, the orchestrator hangs forever.
* If the `JITExecutor` takes hours, the orchestrator stalls.
* Therefore, the boundary MUST enforce internal timeouts for delegated tasks independently of the user's overarching request context to ensure the orchestrator can make forward progress, even if progress means retrying a failed LLM call.

### Memory Lifecycle and Result Acknowledgment
`ShardManager` uses `sm.GetResult(id)` to retrieve results and implicitly garbage collect the result state (via `delete(sm.results, id)`).
If an orchestrator spawns a delegated task but never calls `GetResult`, the result map in `ShardManager` grows unbounded. The `JITExecutor` itself does not hold onto results, so the memory pressure accumulates entirely on the ShardManager boundary.

### Security and Permission Inversion
A user-defined agent executes via `JITExecutor`. The `JITExecutor` respects `permitted(Action)` rules derived by the Kernel.
If `ShardManager` bypasses standard permission checks when spawning a delegated task, an attacker could potentially invoke high-privilege tools (e.g., `shell.run`) by crafting a specific Persona that JIT compiles into a privileged context, escaping the sandbox.

### Conclusion on Remediation
This boundary is not just a function call; it is a bridge between two fundamentally different concurrency and lifecycle models. To fortify it:
1. Wrap all `TaskDelegator.Execute` calls in a `defer recover()` block that forces a transition to `Failed` and returns a Typed Error.
2. Implement strict timeout enforcement within `newDelegatedShardAgent`.
3. Introduce structured logging that carries a correlation ID across the boundary to trace requests accurately from `ShardManager` through the JIT compilation phase and LLM inference.
4. Add robust unit/integration tests that aggressively assert `context.Canceled` is respected.

### Adversarial Input Scenarios Breakdown
When treating the `JITExecutor` as an untrusted or flaky sub-component, we must test how `ShardManager` handles diverse adversarial inputs:

1. **Massive Payload Injection:** A delegated task returns a 100MB string. Does `ShardManager` OOM while trying to marshal it into `types.ShardResult`?
2. **Empty String Returns:** The LLM responds with literally nothing (`""`). `ShardManager` should treat this as a semantic failure, not a success.
3. **Invalid UTF-8:** A malformed payload containing raw bytes or invalid UTF-8 sequences causes JSON serialization panics later in the pipeline.
4. **Control Character Flood:** Strings filled with `\0` (null bytes) or thousands of newlines that break the terminal articulation layer.

### ShardManager Queue Saturation
If `SpawnQueue` is enabled on `ShardManager`, delegated tasks occupy queue slots.
* If `JITExecutor` tasks are generally slower than system shards, the queue will fill entirely with delegated tasks, starving critical system shards (like `perception_firewall` or `executive_policy`) of execution resources.
* Priority inversions must be tested: can a low-priority user persona block a high-priority system shard because the JIT Executor is stalled?

### Context Isolation
Every spawn request has a `types.SessionContext`.
* Does the `TaskDelegator` accidentally leak variables or Blackboard facts from one user's session context into the global JIT context for the next request?
* If `ShardManager` spawns two tasks concurrently, does the `JITExecutor` share mutable state between them?

### The "BaseShardAgent" Bug (Regression Analysis)
Historically, when a factory was missing, `ShardManager` returned an empty `BaseShardAgent` that reported success.
The fix was to delegate. However, if the `TaskDelegator` itself wraps its execution in a way that swallows real errors (e.g., returning `("BaseShardAgent execution", nil)` as a mock placeholder), the bug resurfaces.
Our E2E test `TestE2E_Boundary_Delegation_HollowOutputPrevention` explicitly asserts against this magic string to prevent regressions.

### LLM Token Budget Exhaustion
The JIT compiler enforces a strict token budget.
* If `ShardManager` requests a task that requires massive context, the JIT compiler will truncate context.
* If truncation fails, the LLM will return a `context_length_exceeded` error.
* This error must be explicitly mapped back to a typed error by the `TaskDelegator` so the Orchestrator knows it must reduce context (paging), rather than just retrying the identical failed prompt indefinitely.

### Autopoiesis (Self-Modification) Feedback Loop
Delegated agents can generate code and tools.
* If the `JITExecutor` executes a tool that modifies the repository, but the execution fails midway, the state is inconsistent.
* `ShardManager` only knows `Completed` or `Failed`. It doesn't have a `Partial` state.
* The orchestrator must handle `Failed` by triggering the TDD loop to revert or fix the changes.

### Final Assessment
The integration seam between `ShardManager` and `JITExecutor` is the central nervous system of codeNERD, bridging the imperative Go orchestration logic with the declarative, neuro-symbolic JIT execution layer. The analysis highlights that most failure modes stem from a mismatch in concurrency expectations, improper error wrapping, and lifecycle mismanagement. The provided test suite directly mitigates these risks by asserting explicit state transitions, temporal boundaries, and error fidelity.

### System-Wide Impact of Delegation Failures
When `ShardManager` delegates to the `JITExecutor`, it assumes the executor behaves as a reliable worker node. However, the `JITExecutor` relies on external systems (LLMs, filesystems, virtual stores). A failure here has massive upstream implications:
1. **Perception Transducer Desync:** The Perception Transducer derives a `next_action` fact. If the `JITExecutor` fails silently, the Kernel believes the action was completed, creating a ghost fact. Future turns will assume this work is done.
2. **VirtualStore State Corruption:** If the `JITExecutor` successfully executes an interactive tool on the `VirtualStore` but fails to return the output string due to a boundary serialization error, the `VirtualStore` is mutated, but the orchestrator believes the operation failed and might retry it, leading to duplicate mutations.
3. **Session State Desync:** The session history (recorded by `SessionPersister`) relies on accurate outputs from the `JITExecutor`. If the boundary truncates outputs incorrectly, the history loses fidelity, degrading future context windows.

### Defensive Mechanisms in ShardManager
To protect itself from `JITExecutor` flakiness, `ShardManager` employs several defensive mechanisms:
*   **Detachment:** The `spawnDetached` mechanism allows `ShardManager` to abandon a stalled task, preventing goroutine leaks at the manager level (though the underlying worker might still leak).
*   **Timeouts:** Explicit timeouts on the `SpawnQueue` prevent indefinite blocking.
*   **Error Wrapping:** The `TaskDelegator` adapter (`newDelegatedShardAgent`) must wrap raw string errors into strongly typed Go errors.

### The Missing Telemetry Contract
A critical implicit contract is telemetry. `ShardManager` tracks `spawnCounter`, active shard counts, and emits `ShardResult` metrics. When a task is delegated, the `JITExecutor` generates its own telemetry (LLM token usage, tool invocation counts).
Currently, these metrics are disjoint. The boundary lacks a unified context to correlate `ShardManager` spawn requests with `JITExecutor` token usage.
*   **Vulnerability:** A rogue delegated task could consume excessive API budget without triggering `ShardManager` limits, as the manager only tracks "active tasks", not "active tokens".

### Scenario 31: Nested Delegation Deadlock
*   **Violated Contract:** Non-reentrancy of the delegation loop.
*   **Injection:** A delegated `JITExecutor` task uses an MCP tool that internally spawns another Shard via `Cortex.SpawnTask`, which then delegates back to `JITExecutor`.
*   **Expected Behavior:** The system must detect the recursion and break the loop with an explicit error, preventing stack overflow or worker pool exhaustion.

### Scenario 32: Ephemeral Fact Leakage
*   **Violated Contract:** Kernel state cleanliness across boundary calls.
*   **Injection:** A delegated task asserts ephemeral facts during execution but crashes before cleanup.
*   **Expected Behavior:** The `ShardManager` or `JITExecutor` must guarantee a cleanup deferred function that retracts ephemeral facts asserted during the failed execution, maintaining Kernel integrity for the next turn.

### Summary
The boundary between imperative shard management and declarative JIT execution is the most complex seam in the codeNERD architecture. Our analysis and tests demonstrate that while the "hollow output" bug has been mitigated, temporal, concurrent, and semantic failure modes remain active threats. Continuous adversarial testing (Siege methodology) is required to ensure this boundary remains resilient against real-world chaos.

## Cross-Boundary Failure Path Trace:
This section specifically addresses the requirement to trace a failure path that crosses 3+ subsystems.

**The Path:** `Campaign Orchestrator` -> `ShardManager` -> `JITExecutor` -> `LLM Client`

1.  **Origin:** The `Campaign Orchestrator` determines the next logical step in a multi-phase plan is to generate code. It issues a `SpawnTask` request to `ShardManager` for a user-defined agent (e.g., `RustExpert`).
2.  **Boundary 1 (Orchestrator -> ShardManager):** The Orchestrator expects a synchronous-like return (awaiting completion) and relies on the state transition to `Completed`.
3.  **Boundary 2 (ShardManager -> JITExecutor):** The `ShardManager` receives the request. Finding no Go implementation for `RustExpert`, it invokes the `TaskDelegator` to hand the execution over to the `JITExecutor`. It tracks the agent state as `Running`.
4.  **Boundary 3 (JITExecutor -> LLM Client):** The `JITExecutor` compiles the prompt, assembles the context, and makes a streaming request to the `LLM Client`.
5.  **The Failure Injection:** The `LLM Client` encounters a transient network partition midway through receiving the stream. It returns a generic, untyped error string (e.g., `"connection reset by peer"`).
6.  **The Cascade:**
    *   The `JITExecutor` receives the error. Because it's an un-typed error, the executor's retry logic (which looks for specific HTTP status codes like 429 or 503) fails to trigger. It aborts the execution.
    *   The `TaskDelegator` adapter receives the error. It transitions the agent state to `Failed` and returns the error to `ShardManager`.
    *   `ShardManager` records the failure in its `results` map and unblocks the waiting Orchestrator.
    *   The `Campaign Orchestrator` receives the failure. Because the error is still a raw, un-typed string, the Orchestrator doesn't know this is a transient network issue. Instead of retrying the task, it assumes the `RustExpert` failed the semantic task (e.g., "I couldn't write the code").
    *   **Catastrophic Result:** The Orchestrator halts the entire campaign, discarding all progress, because a recoverable network error was not properly typed and propagated across three subsystem boundaries.

### Remediation Strategy for Multi-Boundary Cascades
To prevent this specific cascade, the system must enforce strict error typing at each boundary:
*   The `LLM Client` must return explicit `NetworkError`, `RateLimitError`, or `SemanticError` types.
*   The `JITExecutor` must wrap these errors in its own context-aware types (e.g., `ExecutionFailed{Reason: NetworkError}`).
*   The `TaskDelegator` boundary must explicitly marshal these structured errors back to `ShardManager`.
*   The `Campaign Orchestrator` must switch on the error type to determine if the failure warrants a retry, a fallback strategy, or a campaign abort.

## Mock Adequacy Analysis
The initial implementation of the E2E tests relied heavily on a simple `MockTaskDelegator`. This mock was **inadequate** because it hid real integration bugs.
*   **What it hid:** The mock simply returned a string or an error based on an injected function. It completely bypassed the `JITExecutor`'s complex internal state machine, context compilation, and token budget enforcement.
*   **The specific bug hidden:** If the `JITExecutor` successfully generated code but panicked during the parsing of the Piggyback control packet (the JSON payload), the mock would never trigger this. The system would assume the integration is sound, while in reality, a valid LLM response would crash the background worker pool.
*   **The Action Taken:** The tests must simulate the actual failure modes of the `JITExecutor` (panics during marshalling, context timeouts, empty returns) to prove that `ShardManager` handles them robustly, even if we mock the underlying LLM call to avoid flakiness.

## Architectural Invariants requiring Production Data
One critical architectural invariant is the **Token Budget Degradation** strategy.
*   **The Invariant:** When a context exceeds the token budget, the system should gracefully degrade by dropping older history or less relevant files, rather than failing the prompt compilation.
*   **The Verification Problem:** This invariant cannot be reliably tested without "production-sized" data (massive files, long histories). Small unit tests will never trigger the degradation logic.
*   **The Required E2E Approach:** E2E tests must intentionally construct scenarios that vastly exceed the nominal token limits (e.g., Scenario 22) to force the boundary logic into its degradation paths, proving that the system does not silently fail or OOM.

## The Role of the VirtualStore in Delegation
The `VirtualStore` is the ultimate arbiter of side effects in codeNERD. When a task is delegated across the `ShardManager`/`JITExecutor` boundary, the `VirtualStore` becomes the third critical subsystem in the triad.

### Implicit Contract: Executing Effects
1.  **The Assumption:** `ShardManager` assumes that if a delegated task succeeds, any necessary side effects (file writes, test executions) have been committed to the `VirtualStore` and, consequently, to the real filesystem or kernel.
2.  **The Crack:** If the `JITExecutor` compiles a task that generates tools (Ouroboros loop) but executes them in a sandboxed, ephemeral `VirtualStore` clone (e.g., during a Dreamer simulation) instead of the main `VirtualStore`, the changes are discarded when the JIT loop finishes.
3.  **The Resulting Cascade:** `JITExecutor` returns success to `ShardManager`. `ShardManager` updates the state to `Completed`. The Orchestrator proceeds to the next phase, assuming a file exists. The next phase immediately fails with a `FileNotFound` error.

### Adversarial Scenarios for the VirtualStore Boundary
To truly stress this triad, the E2E suite must inject failures at the `VirtualStore` level during a delegated execution:

#### Scenario 33: VirtualStore Write Permission Denial
*   **Violated Contract:** Effect propagation and authorization.
*   **Injection:** A delegated task attempts to write to a file, but the `VirtualStore` rejects the operation due to missing interactive gate permissions (the Kernel denied `permitted(/write_file)`).
*   **Expected Behavior:** The `JITExecutor` must catch the permission error, recognize it as a fatal constraint violation, and fail the task. It must NOT return a generic "LLM failed" error. The exact permission denial must cross back over the `TaskDelegator` boundary to `ShardManager`.

#### Scenario 34: VirtualStore Mid-Flight Transaction Rollback
*   **Violated Contract:** Atomicity of delegated tasks.
*   **Injection:** A delegated task executes three tool calls. The first two succeed in the `VirtualStore`. The third triggers a system-level panic or a catastrophic constraint violation, causing the `VirtualStore` to rollback the transaction.
*   **Expected Behavior:** The `JITExecutor` must reflect the rolled-back state. It cannot return partial success. `ShardManager` must receive a clear failure indicating the transaction was aborted, ensuring no partial state is assumed by upstream orchestrators.

#### Scenario 35: Asynchronous VirtualStore Event Consumption
*   **Violated Contract:** Eventual consistency synchronization.
*   **Injection:** A delegated task finishes and returns success, but the `VirtualStore` is processing background events (like updating the symbol graph or syncing to SQLite) asynchronously.
*   **Expected Behavior:** `ShardManager` must not consider the task fully `Completed` (and thus unblock upstream dependencies) until the `VirtualStore` signals that all associated background effects are durably committed. If this synchronization barrier is missing, race conditions will plague multi-phase campaigns.

## Conclusion on the Siege Mentality
The core philosophy of Siege is that "A system that survives unit tests but dies in integration is a liar."
The `ShardManager` to `JITExecutor` boundary is the most likely place for this system to lie. It uses interfaces (`TaskDelegator`) to hide complexity, which is good for modularity but dangerous for reliability.
By treating the `JITExecutor` as a hostile environment—capable of returning garbage, panicking, hanging forever, or dropping context—we force the `ShardManager` to become robust. The scenarios detailed in this journal are not theoretical; they represent the exact failure modes observed in production LLM integrations. Implementing tests for these scenarios ensures the codeNERD architecture can survive the chaos of real-world operation.

## Cross-Boundary Resource Contention

The `ShardManager` and `JITExecutor` share underlying physical and virtual resources. When analyzing the boundary, we must consider how resource contention in one subsystem affects the other.

### The API Slot Starvation Problem
The `JITExecutor` relies on an `APIScheduler` to acquire slots for outbound LLM calls. The `ShardManager` manages the lifecycle of the workers requesting those slots.

1.  **The Scenario:** A Campaign Orchestrator spawns 10 user-defined agents (delegated to `JITExecutor`) and 2 system agents (managed natively by `ShardManager`).
2.  **The Contention:** The 10 JIT agents immediately request API slots. If the scheduler only allows 5 concurrent slots, 5 JIT agents block.
3.  **The Integration Crack:** If the 2 system agents *also* need API slots (e.g., the `perception_firewall` needing to validate an incoming message), they enter the same queue. If the `JITExecutor` tasks are long-running (e.g., generating complex code), the system agents starve.
4.  **The Consequence:** The entire system becomes unresponsive because critical system-level perception and routing functions are blocked by background generation tasks.
5.  **Remediation:** The boundary must enforce strict priority routing. Tasks delegated to `JITExecutor` must run at a lower priority than native system shards, ensuring the `APIScheduler` prioritizes core loops over background generation.

### Database Lock Contention (SQLite)
Both the `JITExecutor` (compiling prompts from the Knowledge Base) and the `ShardManager` (managing state and telemetry) interact with the underlying SQLite databases.

1.  **The Scenario:** `ShardManager` rapidly spawns and tears down dozens of short-lived delegated agents.
2.  **The Contention:** Each spawn might trigger a read from the `.nerd/shards/<name>_knowledge.db` to compile the JIT prompt. Simultaneously, the `SessionPersister` is writing turn results to the session DB.
3.  **The Integration Crack:** SQLite handles concurrent reads well, but writes (or WAL checkpoints) lock the database. If a rogue delegated agent attempts to modify its own KB (via a learned tool) while another agent is booting, `database is locked` errors occur.
4.  **The Consequence:** A `database is locked` error during JIT compilation crashes the executor. If this error isn't handled gracefully across the boundary, the `ShardManager` fails the agent, and the orchestrator panics.
5.  **Remediation:** The `TaskDelegator` must implement robust retry logic with exponential backoff for `SQLITE_BUSY` errors, preventing transient database locks from surfacing as hard execution failures to the `ShardManager`.

### Memory Fragmentation and GC Pressure
The `JITExecutor` allocates massive strings for prompts and LLM responses. `ShardManager` holds maps of thousands of `ShardResult` objects.

1.  **The Scenario:** A long-running campaign generates gigabytes of text across hundreds of delegated turns.
2.  **The Contention:** Go's Garbage Collector (GC) must work overtime to clean up the ephemeral strings generated by the `JITExecutor`.
3.  **The Integration Crack:** If `ShardManager` holds onto `ShardAgent` references (because `GetResult` was never called, as detailed in Scenario 18), the massive strings attached to those results are never freed.
4.  **The Consequence:** The system eventually OOMs. The GC pause times skyrocket, causing temporal contracts (like context timeouts) to fire spuriously, failing tasks that were actually proceeding normally.
5.  **Remediation:** Implement strict size bounds on `ShardResult.Result` strings stored by `ShardManager`, and enforce a TTL (Time To Live) on unretrieved results to ensure the `JITExecutor`'s allocations can be reclaimed.

## Summary of Architectural Vulnerabilities
This Siege analysis has identified several critical architectural vulnerabilities at the `ShardManager` / `JITExecutor` boundary:
1.  **State Machine Desynchronization:** `Running` states that never terminate due to unhandled panics or ignored context cancellations.
2.  **Error Typification Loss:** Meaningful errors (Rate Limits, DB Locks) degraded to raw strings, breaking downstream orchestrator logic.
3.  **Resource Inversion:** Background delegated tasks starving foreground system shards of API slots or DB connections.
4.  **Silent Failures (Hollow Outputs):** The historical bug where a missing factory resulted in a fake success string, bypassing the entire TDD and repair loop.

By systematically addressing these vulnerabilities through the adversarial test scenarios defined herein, the codeNERD architecture can achieve the resilience required for autonomous, multi-agent operations.

## Final Verification Strategies

To ensure that the remediations for the vulnerabilities identified above are effective, specific verification strategies must be employed beyond unit testing.

### 1. Chaos Engineering Injection Points
Instead of relying solely on mocks, we must introduce chaos injection points directly into the production code paths during testing.
*   **JIT Compiler Fault Injection:** Introduce a flag that randomly drops 1% of prompt atoms during compilation to verify how the LLM and the subsequent validation layers handle degraded context.
*   **Network Partition Simulation:** Use a custom `http.RoundTripper` in the LLM client to randomly drop connections or introduce extreme latency (e.g., 60 seconds) to ensure the `TaskDelegator`'s context timeouts fire accurately and the `ShardManager` cleans up resources without leaking goroutines.

### 2. Telemetry and Observability Assertions
Integration tests must assert not just the final outcome, but the intermediate telemetry.
*   **Goroutine Leak Detection:** Every integration test must capture the goroutine count before and after execution, failing immediately if the baseline count increases.
*   **Metric Validation:** When a delegated task fails due to a simulated rate limit, the test must query the internal metrics registry to verify that a `rate_limit_encountered` counter was incremented, ensuring observability contracts are met.

### 3. Long-Duration Soak Tests
The memory fragmentation and GC pressure issues (detailed above) can only be verified through soak testing.
*   **The Setup:** Run a simulated campaign that continuously delegates tasks to the `JITExecutor` for 24 hours.
*   **The Assertion:** Monitor the RSS (Resident Set Size) memory. The test fails if memory usage grows unbounded, proving that the `ShardManager` is successfully garbage collecting completed `ShardResult` objects and the `JITExecutor` isn't leaking prompt allocations.

By implementing these advanced verification strategies, we move from merely identifying cracks in the architecture to proving mathematically that those cracks have been sealed against real-world chaos.

## The Role of the Spreading Activation Network
In the neuro-symbolic architecture of codeNERD, the Transducer converts user input into `user_intent` facts, which then trigger a Spreading Activation network to pull relevant context.

### Interaction with Delegation
When `ShardManager` delegates to `JITExecutor`, the execution is largely isolated. However, the `JITExecutor` relies on the Spreading Activation network to populate its working memory before making the LLM call.
1. **The Scenario:** A task is delegated requiring context about a deeply nested file `src/utils/math.go`.
2. **The Contention:** If the Spreading Activation network is slow or configured with an activation threshold that is too high, the required context won't be paged into memory.
3. **The Result:** The `JITExecutor` compiles a prompt missing critical information. The LLM hallucinates a response.
4. **The Boundary Failure:** `JITExecutor` returns the hallucinated response as a "success". `ShardManager` logs it as completed. The user receives broken code.
5. **Remediation:** The `JITExecutor` must implement a "context verification" step prior to calling the LLM. If the required symbols (extracted from the task intent) are not present in the activated context, it must fail early and explicitly notify `ShardManager` of a `ContextStarvationError`.

### Spreading Activation Poisoning
Conversely, what happens if the context is too large?
1. **The Scenario:** A delegated task triggers a massive spreading activation wave, pulling in 500 files.
2. **The Consequence:** The `JITExecutor`'s Token Budget Manager must aggressively prune the context. If the pruning algorithm is naive, it might drop the *most relevant* files simply because they are large, while keeping hundreds of small, irrelevant files.
3. **Remediation:** The boundary tests must assert that the Token Budget Manager prioritizes files based on their activation energy (relevance), not just size, ensuring the `JITExecutor` has the highest quality context possible before executing the delegated task.

## Final Thought on Defensive Architecture
The ultimate goal of this Siege analysis is to drive the architecture towards "Fail-Safe Default" behaviors.
If a component in the `JITExecutor` panics, the default should be failure propagation.
If an API slot is unavailable, the default should be backpressure, not starvation.
If context is missing, the default should be refusal, not hallucination.
By hardening the `ShardManager` / `JITExecutor` boundary, we enforce these fail-safe defaults at the most critical juncture of the system.
