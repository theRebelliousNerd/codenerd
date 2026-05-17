---
surface: "Campaign Orchestrator ↔ Session Executor"
mode: "boundary"
subsystems_tested: ["internal/campaign", "internal/session"]
blast_radius: "critical"
remediated: false
---

# Siege Journal: Campaign Orchestrator ↔ Session Executor Integration Analysis

## 1. System Interaction Map

When the `Campaign Orchestrator` executes a `Phase`, it processes `Tasks` by dispatching them to the `Session Executor`. The specific integration boundary is as follows:

1. `Orchestrator.runPhase(ctx, phase)` (in `internal/campaign/orchestrator_tasks.go`) pulls eligible tasks and limits concurrency using `o.maxParallelTasks` and `o.determineConcurrencyLimit()`.
2. For each eligible task, it spawns a goroutine: `go o.runSingleTask(ctx, phase, task, lease, results)`.
3. `runSingleTask` invokes task-specific handlers like `o.executeGenericTask`, which eventually calls `o.spawnTask(ctx, intent, task.Description)`.
4. `spawnTask` invokes the interface method: `te.Execute(ctx, intent, task)` where `te` is the `session.TaskExecutor`.
5. The `TaskExecutor` implementation (`internal/session/task_executor.go: JITExecutor.Execute`) calls `j.ExecuteWithContext`.
6. Inside `ExecuteWithContext`, if `needsSubagent(intent)` returns false (e.g., for `/fix`, `/test`, which are not in the `complexIntents` map), it falls back to **inline execution**.
7. In inline execution, `j.executor.SetSessionContext(sessionCtx)` is called, which mutates the shared `Executor` state.
8. `j.executor.Process(ctx, inlineTask)` is then called to interact with the LLM via `internal/session/executor.go`.
9. The `Executor` updates its `conversationHistory` and performs observations (`e.observe(ctx, input)`).

## 2. Contract Analysis

The implicit contract here is:
- **Concurrency Support:** `Campaign Orchestrator` assumes `TaskExecutor.Execute` is thread-safe and isolated per-task, because it explicitly spawns goroutines to run multiple tasks in parallel during a phase.
- **Task Isolation:** Each task's context, prompt, and execution history must not bleed into other tasks running concurrently in the same phase.
- **Resource Management:** `TaskExecutor` is expected to manage underlying resources (like LLM clients and VirtualStore) safely under concurrent load.
- **Failure Propagation:** If a task fails or blocks indefinitely, the Orchestrator expects context cancellation or a timeout error to gracefully fail the task, without bringing down the orchestrator or hanging the entire phase.

**Reality:**
The `JITExecutor` documentation states: `// NOTE: SetSessionContext is not thread-safe. For true concurrent execution, use ExecuteAsync which spawns isolated subagents.` However, the Orchestrator uses `Execute` for intents like `/fix` and `/test`, leading to concurrent mutations of the single shared `Executor` instance.

## 3. Failure Mode Enumeration

1. **State Corruption (Cross-Talk):** Multiple goroutines call `SetSessionContext` on the same `Executor`. Task A's LLM prompt might accidentally receive Task B's context.
2. **Data Race in Conversation History:** The `Executor` appends to `e.conversationHistory` during `Process`. Concurrent `Process` calls from multiple tasks will trigger data races and slice corruption.
3. **Temporal Failure (Deadlock/Hang):** If one inline task gets blocked on an LLM call or a VirtualStore tool, other tasks might be starved if they share synchronized resources inside the `Executor` (though the Executor lock scope is small, resource locks might block).
4. **Semantic Corruption (Ghost Facts):** If the shared `Kernel` asserts facts without task isolation, Task A might trigger rules based on Task B's asserted facts.
5. **Partial Pipeline Failure:** A context cancellation for one task might inadvertently cancel shared resources or stop the phase prematurely if error handling isn't robust across the boundary.

## 4. Adversarial Scenario Design

1. **Scenario 1: Concurrent Inline Execution Data Race (P0)**
   - **Violated Contract:** Thread safety of `TaskExecutor.Execute`.
   - **Mechanism:** Schedule 10 tasks in a phase using the `/fix` intent. The orchestrator spawns 10 goroutines. The `JITExecutor` falls back to inline execution.
   - **Expected Behavior:** Race detector flags slice corruption in `e.conversationHistory` and `e.sessionContext`.
   - **Severity:** P0. State corruption causes random LLM hallucination and crashes.

2. **Scenario 2: Context Bleed Between Tasks (P1)**
   - **Violated Contract:** Task Isolation.
   - **Mechanism:** Task A has context `file:A`, Task B has context `file:B`. Both execute concurrently.
   - **Expected Behavior:** The LLM client for Task A receives the prompt intended for Task B due to the overwritten `SessionContext`.
   - **Severity:** P1.

3. **Scenario 3: Orchestrator Cancellation Does Not Leak Goroutines (P2)**
   - **Violated Contract:** Resource cleanup on timeout.
   - **Mechanism:** Start a task that calls a blocking tool via the LLM. Cancel the orchestrator context.
   - **Expected Behavior:** The task execution terminates immediately and the goroutine exits.
   - **Severity:** P2.

4. **Scenario 4: Task Spam Exhausts Concurrency Limits (P2)**
   - **Violated Contract:** Bounded parallelism.
   - **Mechanism:** Feed a phase with 1,000 tasks.
   - **Expected Behavior:** The Orchestrator limits active tasks to `maxParallelTasks`. Memory usage remains stable.
   - **Severity:** P2.

5. **Scenario 5: Async Subagent Execution Completes Correctly (P1)**
   - **Violated Contract:** Correct asynchronous routing.
   - **Mechanism:** Execute a task with `/research` (complex intent).
   - **Expected Behavior:** `JITExecutor` spawns an isolated subagent instead of inline execution. Concurrency works safely.
   - **Severity:** P1.

6. **Scenario 6: Tool Error Propagation (P2)**
   - **Violated Contract:** Error reporting.
   - **Mechanism:** A tool called by the LLM fails.
   - **Expected Behavior:** The error bubbles up from `Executor` through `JITExecutor` to `Orchestrator` and triggers task failure (and potentially replan).
   - **Severity:** P2.

7. **Scenario 7: Malformed Intent Handling (P3)**
   - **Violated Contract:** Graceful degradation.
   - **Mechanism:** Orchestrator requests execution of an unknown intent.
   - **Expected Behavior:** `TaskExecutor` rejects it or falls back cleanly, returning a clear error to Orchestrator.
   - **Severity:** P3.

8. **Scenario 8: JIT Compiler Failure (P1)**
   - **Violated Contract:** Pipeline reliability.
   - **Mechanism:** `JITCompiler` returns an error during compilation.
   - **Expected Behavior:** The specific task fails, but the orchestrator handles the failure without crashing other tasks.
   - **Severity:** P1.

9. **Scenario 9: Massive Task Result Payload (P2)**
   - **Violated Contract:** Memory limits on results.
   - **Mechanism:** The LLM returns a 10MB result string.
   - **Expected Behavior:** Orchestrator truncates or safely stores the result without OOMing the main process.
   - **Severity:** P2.

10. **Scenario 10: Mixed Inline and Async Tasks (P1)**
    - **Violated Contract:** Pipeline uniformity.
    - **Mechanism:** Phase contains tasks with `/fix` and `/research`.
    - **Expected Behavior:** Both execute correctly. The async ones get isolated, the inline ones race (if not fixed).
    - **Severity:** P1.

11. **Scenario 11: Task Retry Logic on Timeout (P2)**
    - **Violated Contract:** Recovery mechanism.
    - **Mechanism:** Task times out because of a slow LLM.
    - **Expected Behavior:** Orchestrator registers task failure and follows its retry threshold before triggering replan.
    - **Severity:** P2.

12. **Scenario 12: Context Paging Limits Exceeded by Task Output (P2)**
    - **Violated Contract:** Context budgeting.
    - **Mechanism:** Task result is larger than context budget.
    - **Expected Behavior:** Orchestrator pages context appropriately and compresses safely.
    - **Severity:** P2.

13. **Scenario 13: Spawner Exhaustion (P1)**
    - **Violated Contract:** Subagent resource limits.
    - **Mechanism:** Trigger 100 concurrent `/research` tasks. Max subagents is 50.
    - **Expected Behavior:** 50 spawn, the rest fail with capacity error, Orchestrator catches and retries.
    - **Severity:** P1.

14. **Scenario 14: Replan Triggered by Checkpoint Failure (P2)**
    - **Violated Contract:** Replan coordination.
    - **Mechanism:** Tasks complete but checkpoint verification fails.
    - **Expected Behavior:** Phase is not marked complete, replanner is triggered.
    - **Severity:** P2.

15. **Scenario 15: Heartbeat Maintained During Heavy LLM Load (P2)**
    - **Violated Contract:** Orchestrator control loops.
    - **Mechanism:** Long-running task blocks `Executor`.
    - **Expected Behavior:** Orchestrator heartbeat loop continues uninterrupted.
    - **Severity:** P2.

## 5. Cascading Failure Analysis

If the data race in `JITExecutor` corrupts the `SessionContext`, a `/fix` task might receive the context of a `/test` task. The LLM will then output assertions for the wrong target file. The `VirtualStore` will modify the wrong file. The `Campaign Orchestrator` will receive a success signal, but the codebase will be corrupted. The checkpoint will then fail (if verification is robust), triggering an unnecessary `Replan`. The replanner will be confused because the requested change was applied to the wrong file, leading to an endless loop of failing modifications until the `CampaignTimeout` is hit. This proves that a simple missing mutex at the integration boundary leads to total system failure and code destruction.
