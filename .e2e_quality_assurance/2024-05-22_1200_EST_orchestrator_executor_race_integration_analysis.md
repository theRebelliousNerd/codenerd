---
surface: "Orchestrator-Executor Boundary"
mode: "boundary"
subsystems_tested: ["internal/campaign", "internal/session"]
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

## Scenario 1: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 1 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 1b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 1ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 1c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 1us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 2: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 2 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 2b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 2ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 2c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 2us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 3: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 3 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 3b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 3ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 3c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 3us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 4: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 4 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 4b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 4ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 4c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 4us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 5: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 5 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 5b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 5ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 5c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 5us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 6: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 6 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 6b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 6ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 6c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 6us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 7: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 7 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 7b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 7ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 7c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 7us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 8: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 8 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 8b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 8ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 8c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 8us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 9: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 9 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 9b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 9ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 9c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 9us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 10: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 10 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 10b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 10ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 10c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 10us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 11: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 11 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 11b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 11ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 11c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 11us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 12: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 12 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 12b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 12ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 12c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 12us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 13: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 13 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 13b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 13ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 13c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 13us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 14: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 14 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 14b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 14ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 14c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 14us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 15: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 15 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 15b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 15ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 15c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 15us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 16: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 16 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 16b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 16ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 16c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 16us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 17: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 17 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 17b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 17ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 17c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 17us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 18: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 18 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 18b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 18ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 18c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 18us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 19: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 19 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 19b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 19ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 19c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 19us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 20: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 20 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 20b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 20ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 20c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 20us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 21: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 21 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 21b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 21ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 21c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 21us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 22: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 22 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 22b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 22ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 22c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 22us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 23: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 23 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 23b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 23ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 23c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 23us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 24: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 24 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 24b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 24ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 24c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 24us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 25: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 25 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 25b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 25ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 25c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 25us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 26: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 26 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 26b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 26ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 26c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 26us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 27: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 27 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 27b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 27ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 27c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 27us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 28: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 28 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 28b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 28ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 28c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 28us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 29: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 29 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 29b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 29ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 29c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 29us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 30: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 30 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 30b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 30ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 30c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 30us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 31: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 31 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 31b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 31ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 31c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 31us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 32: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 32 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 32b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 32ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 32c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 32us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 33: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 33 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 33b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 33ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 33c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 33us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 34: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 34 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 34b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 34ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 34c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 34us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 35: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 35 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 35b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 35ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 35c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 35us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 36: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 36 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 36b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 36ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 36c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 36us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 37: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 37 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 37b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 37ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 37c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 37us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 38: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 38 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 38b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 38ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 38c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 38us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 39: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 39 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 39b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 39ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 39c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 39us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 40: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 40 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 40b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 40ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 40c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 40us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 41: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 41 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 41b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 41ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 41c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 41us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 42: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 42 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 42b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 42ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 42c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 42us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 43: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 43 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 43b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 43ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 43c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 43us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 44: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 44 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 44b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 44ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 44c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 44us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 45: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 45 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 45b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 45ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 45c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 45us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 46: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 46 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 46b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 46ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 46c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 46us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 47: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 47 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 47b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 47ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 47c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 47us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 48: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 48 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 48b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 48ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 48c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 48us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 49: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 49 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 49b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 49ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 49c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 49us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 50: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 50 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 50b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 50ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 50c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 50us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 51: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 51 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 51b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 51ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 51c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 51us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 52: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 52 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 52b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 52ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 52c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 52us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 53: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 53 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 53b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 53ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 53c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 53us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 54: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 54 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 54b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 54ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 54c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 54us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 55: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 55 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 55b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 55ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 55c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 55us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 56: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 56 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 56b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 56ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 56c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 56us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 57: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 57 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 57b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 57ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 57c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 57us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 58: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 58 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 58b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 58ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 58c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 58us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 59: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 59 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 59b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 59ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 59c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 59us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 60: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 60 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 60b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 60ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 60c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 60us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 61: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 61 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 61b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 61ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 61c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 61us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 62: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 62 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 62b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 62ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 62c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 62us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 63: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 63 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 63b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 63ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 63c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 63us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 64: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 64 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 64b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 64ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 64c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 64us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 65: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 65 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 65b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 65ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 65c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 65us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 66: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 66 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 66b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 66ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 66c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 66us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 67: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 67 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 67b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 67ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 67c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 67us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 68: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 68 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 68b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 68ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 68c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 68us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 69: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 69 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 69b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 69ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 69c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 69us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 70: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 70 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 70b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 70ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 70c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 70us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 71: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 71 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 71b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 71ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 71c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 71us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 72: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 72 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 72b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 72ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 72c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 72us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 73: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 73 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 73b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 73ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 73c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 73us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 74: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 74 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 74b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 74ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 74c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 74us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 75: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 75 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 75b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 75ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 75c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 75us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 76: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 76 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 76b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 76ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 76c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 76us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 77: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 77 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 77b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 77ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 77c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 77us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 78: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 78 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 78b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 78ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 78c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 78us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 79: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 79 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 79b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 79ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 79c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 79us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 80: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 80 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 80b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 80ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 80c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 80us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 81: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 81 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 81b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 81ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 81c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 81us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 82: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 82 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 82b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 82ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 82c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 82us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 83: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 83 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 83b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 83ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 83c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 83us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 84: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 84 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 84b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 84ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 84c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 84us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 85: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 85 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 85b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 85ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 85c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 85us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 86: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 86 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 86b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 86ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 86c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 86us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 87: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 87 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 87b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 87ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 87c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 87us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 88: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 88 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 88b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 88ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 88c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 88us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 89: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 89 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 89b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 89ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 89c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 89us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 90: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 90 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 90b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 90ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 90c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 90us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 91: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 91 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 91b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 91ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 91c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 91us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 92: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 92 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 92b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 92ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 92c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 92us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 93: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 93 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 93b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 93ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 93c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 93us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 94: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 94 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 94b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 94ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 94c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 94us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 95: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 95 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 95b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 95ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 95c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 95us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 96: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 96 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 96b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 96ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 96c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 96us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 97: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 97 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 97b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 97ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 97c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 97us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 98: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 98 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 98b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 98ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 98c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 98us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 99: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 99 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 99b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 99ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 99c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 99us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 100: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 100 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 100b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 100ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 100c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 100us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 101: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 101 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 101b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 101ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 101c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 101us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 102: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 102 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 102b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 102ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 102c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 102us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 103: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 103 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 103b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 103ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 103c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 103us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 104: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 104 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 104b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 104ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 104c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 104us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 105: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 105 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 105b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 105ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 105c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 105us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 106: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 106 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 106b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 106ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 106c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 106us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 107: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 107 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 107b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 107ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 107c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 107us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 108: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 108 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 108b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 108ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 108c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 108us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 109: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 109 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 109b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 109ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 109c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 109us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 110: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 110 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 110b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 110ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 110c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 110us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 111: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 111 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 111b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 111ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 111c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 111us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 112: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 112 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 112b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 112ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 112c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 112us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 113: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 113 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 113b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 113ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 113c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 113us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 114: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 114 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 114b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 114ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 114c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 114us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 115: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 115 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 115b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 115ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 115c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 115us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 116: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 116 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 116b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 116ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 116c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 116us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 117: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 117 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 117b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 117ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 117c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 117us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 118: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 118 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 118b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 118ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 118c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 118us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 119: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 119 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 119b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 119ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 119c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 119us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 120: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 120 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 120b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 120ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 120c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 120us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 121: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 121 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 121b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 121ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 121c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 121us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 122: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 122 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 122b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 122ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 122c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 122us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 123: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 123 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 123b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 123ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 123c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 123us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 124: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 124 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 124b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 124ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 124c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 124us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 125: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 125 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 125b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 125ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 125c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 125us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 126: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 126 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 126b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 126ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 126c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 126us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 127: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 127 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 127b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 127ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 127c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 127us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 128: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 128 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 128b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 128ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 128c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 128us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 129: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 129 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 129b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 129ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 129c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 129us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 130: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 130 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 130b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 130ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 130c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 130us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 131: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 131 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 131b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 131ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 131c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 131us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 132: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 132 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 132b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 132ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 132c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 132us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 133: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 133 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 133b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 133ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 133c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 133us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 134: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 134 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 134b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 134ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 134c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 134us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 135: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 135 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 135b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 135ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 135c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 135us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 136: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 136 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 136b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 136ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 136c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 136us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 137: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 137 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 137b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 137ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 137c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 137us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 138: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 138 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 138b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 138ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 138c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 138us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 139: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 139 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 139b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 139ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 139c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 139us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 140: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 140 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 140b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 140ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 140c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 140us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 141: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 141 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 141b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 141ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 141c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 141us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 142: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 142 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 142b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 142ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 142c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 142us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 143: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 143 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 143b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 143ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 143c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 143us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 144: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 144 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 144b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 144ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 144c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 144us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 145: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 145 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 145b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 145ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 145c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 145us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 146: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 146 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 146b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 146ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 146c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 146us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 147: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 147 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 147b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 147ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 147c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 147us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 148: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 148 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 148b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 148ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 148c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 148us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 149: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 149 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 149b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 149ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 149c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 149us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

## Scenario 150: Concurrent Inline State Bleed
* **Violated Contract:** Thread-Safe Context Isolation.
* **Injection Mechanism:** Spawn 150 concurrent generic tasks via `ExecuteWithContext` using different session contexts.
* **Expected Behavior:** Each task should observe its own context. In reality, state corruption occurs.
* **Severity:** P0.

## Scenario 150b: Async Execution Leak
* **Violated Contract:** Cancellation Propagation.
* **Injection Mechanism:** Spawn an async task with a 150ms delay and cancel the orchestrator context immediately.
* **Expected Behavior:** Subagent should be reaped. In reality, the goroutine leaks.
* **Severity:** P1.

## Scenario 150c: Spurious Result Caching
* **Violated Contract:** Ordering guarantees in TaskExecutor.
* **Injection Mechanism:** Delay the write lock in `ExecuteAsync` by 150us while a concurrent read polls `GetResult`.
* **Expected Behavior:** `GetResult` blocks or waits cleanly. In reality, it returns a false negative.
* **Severity:** P2.

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
