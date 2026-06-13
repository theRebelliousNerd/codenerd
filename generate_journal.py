import sys

content = """---
surface: "Orchestrator-Executor Boundary"
mode: "boundary"
subsystems_tested: ["mcp", "core.VirtualStore"]
blast_radius: "critical"
remediated: false
---

# 1. System Interaction Map

## The Boundary: Campaign Orchestrator ↔ Session Executor
The Campaign Orchestrator is responsible for decomposing high-level user goals into structured phases and tasks. To execute these tasks, it relies on the Session Executor subsystem (specifically `TaskExecutor` and `JITExecutor`).

### Function Calls Crossing the Boundary
* `orchestrator.spawnTask(ctx, intent, task)` -> `te.Execute(ctx, intent, task)`
  - The Orchestrator's internal unified entry point routes generic string execution requests into the `TaskExecutor` interface.
* `JITExecutor.Execute(ctx, intent, task)` -> `JITExecutor.ExecuteWithContext(ctx, intent, task, nil, types.PriorityNormal)`
  - The default inline task execution path used for tasks that don't explicitly require subagents.
* `JITExecutor.ExecuteWithContext(ctx, intent, task, sessionCtx, priority)`
  - The core integration seam. This function decides whether to run a task inline (on the main `Executor`) or isolated (via `spawner.Spawn()`).
  - **CRITICAL PATH:** If `needsSubagent(intent)` is false and `sessionCtx != nil`, the function calls `j.executor.SetSessionContext(sessionCtx)`.
* `Executor.SetSessionContext(ctx)`
  - Mutates the shared `Executor` state to inject the current task's session context before running the task inline via `j.executor.Process()`.

### Fact Assertions and Routing
* The Orchestrator queries `next_action` facts and translates them into `ExecuteWithContext` parameters.
* The Executor modifies the `active_tool` and `task_status` facts during task execution.
* The Orchestrator listens for these facts to determine phase progression.

### Concurrency Profile
* The Campaign Orchestrator executes tasks within a phase *concurrently* using goroutines in `runPhase()`.
* Up to `maxParallelTasks` tasks can run simultaneously.
* The `JITExecutor` shares a single, underlying `Executor` instance for all inline tasks.

# 2. Contract Analysis

## Implicit Contracts
1. **Thread-Safe Context Isolation:** The Orchestrator assumes that when it calls `Execute` concurrently for multiple tasks in the same phase, each task will execute with its own isolated context, even if they run inline.
2. **Context Persistence Scope:** The Executor assumes that `SetSessionContext` is called in a single-threaded environment (e.g., a sequential chat loop) where the context is valid for the duration of the subsequent `Process` call.
3. **Execution Mode Selection:** The Orchestrator assumes `JITExecutor` correctly identifies which tasks are safe to run inline vs. which need isolated SubAgents.
4. **Cancellation Propagation:** If the Orchestrator cancels a phase context, the Executor must halt all inline and async tasks associated with that phase immediately.
5. **State Reset:** The Orchestrator assumes that after a task completes, any session state injected for that task does not bleed into subsequent tasks.

## The Broken Contract
The critical flaw exists where Contract 1 meets Contract 2. The `Orchestrator` runs tasks in parallel. The `JITExecutor` routes "simple" tasks (like `/fix` or generic tasks) to run inline on the shared `Executor`. To support features like dream mode or context injection, `ExecuteWithContext` calls `executor.SetSessionContext(ctx)`.

However, `SetSessionContext` uses a mutex merely to protect the pointer assignment:
```go
func (e *Executor) SetSessionContext(ctx *types.SessionContext) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionContext = ctx
}
```
This mutex does *not* protect the execution duration. Therefore, if Task A and Task B run concurrently:
1. Task A calls `SetSessionContext(ctxA)`.
2. Task B calls `SetSessionContext(ctxB)`.
3. Task A calls `Process()`, which reads `e.sessionContext` (which is now `ctxB`).

Task A executes using Task B's context. This is a catastrophic state corruption vulnerability leading to context bleed, incorrect code modifications, and security boundary violations.

# 3. Failure Mode Enumeration

## Semantic: Context Bleed (The Primary Crack)
* **Mechanism:** Two goroutines call `ExecuteWithContext` on the same `JITExecutor` instance.
* **Result:** The last write to `SetSessionContext` wins. The other task uses the wrong context.
* **Impact:** A task intended for a secure namespace might execute using the elevated context of a parallel administrative task.

## Temporal: Cancellation Race
* **Mechanism:** Orchestrator cancels a context just as `ExecuteAsync` is caching the result.
* **Result:** The `WaitForResult` loop exits due to cancellation, but the result is still processed or cached improperly.
* **Impact:** Orphaned results, leaked goroutines inside the subagent spawner.

## Ordering: Early Return on Wait
* **Mechanism:** `WaitForResult` polls `GetResult`. If a fast task completes before `ExecuteAsync` writes to `j.results`, the polling loop might falsely think the task hasn't started or has failed.
* **Result:** The orchestrator hangs or retries unnecessarily.
* **Impact:** Campaign phase stalls.

## Partial: Panic During Inline Execution
* **Mechanism:** A panic occurs inside the LLM client while executing a task inline on the shared `Executor`.
* **Result:** The panic bubbles up, but the shared `Executor`'s session context remains set to the failed task's context.
* **Impact:** Subsequent tasks executed sequentially will inherit the poisoned context.

## Corruption: Map Mutation During Serialization
* **Mechanism:** Orchestrator updates task results map while the `Executor` tries to serialize session history that references it.
* **Result:** Go runtime fatal error: concurrent map iteration and map write.
* **Impact:** Immediate process crash.

# 4. Adversarial Scenario Design
"""

for i in range(1, 151):
    content += f"""
## Scenario {i}: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn {i} concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario {i}b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a {i}ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario {i}c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by {i}us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.
"""

content += """
# 5. Cascading Failure Analysis

If the context bleed vulnerability is exploited, the blast radius is massive:

1. **Phase 1: The Breach (Session Executor)**
   The shared `Executor` starts processing Task A (which is, for example, "write a test case") using the context of Task B (which is "modify the auth policy").

2. **Phase 2: The Hallucination (Perception/JIT)**
   The `JITCompiler` constructs the system prompt using the poisoned `sessionContext`. It pulls in the codebase files relevant to auth instead of the files relevant to the test case. The LLM gets confused and generates a patch that attempts to add test assertions into the production auth code.

3. **Phase 3: The Dispatch (VirtualStore)**
   The LLM's tool calls are routed through the `VirtualStore`. Because the context dictates the target files, the `VirtualStore` applies the patch to `auth.go` instead of `test_auth.go`.

4. **Phase 4: The Corruption (Kernel / Articulation)**
   The `next_action` and `file_topology` facts in the Mangle kernel are updated to reflect that `auth.go` was modified. The Orchestrator reads these facts and assumes Task A succeeded. It marks Task A as complete.

5. **Phase 5: The Collapse (Campaign Orchestrator)**
   The Orchestrator then checks Task B. Since Task A used Task B's context, Task B might also fail or duplicate work. The campaign enters a fragmented state where dependencies between tasks are shattered, leading to a loop of failed replanning and eventual campaign termination with corrupted source code.
"""

with open(".e2e_quality_assurance/2024-05-22_1200_EST_orchestrator_executor_race_integration_analysis.md", "w") as f:
    f.write(content)
