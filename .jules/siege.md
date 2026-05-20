# Siege Journal

## 2025-05-17 - [Initial Recon]
**Learning:** Found architecture documentation detailing the migration from legacy Shards to the JIT-driven Session Executor loop.
**Action:** Always focus on `internal/session` and how it interacts with `internal/core` (Kernel/VirtualStore), `internal/prompt` (JIT Config), and `internal/campaign` (Orchestration).

## 2025-05-17 - [Adversarial E2E Integration Suite Design]
**Learning:** Found existing tests in `tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go` and `session_clean_loop_integration_test.go` and `session_executor_kernel_integration_test.go`. I need to choose an untargeted boundary. Since `Campaign Orchestrator <-> Session Executor` or `Autopoiesis <-> Kernel` or `Prompt Compiler <-> LLM Client` are all juicy. The prompt states "Create a journal entry at `.e2e_quality_assurance/{date}_{time}_EST_{target_name}_integration_analysis.md`" and the file `tests/e2e/session_executor_kernel_integration_test.go` exists, which means the boundary `Session ↔ Kernel` is already heavily tested. I'll focus on `Campaign ↔ Session Executor` or `Campaign ↔ ShardManager`. But wait, ShardManager was deleted. So I will focus on `Campaign Orchestrator ↔ Session Executor`.
**Action:** Let's trace `internal/campaign/orchestrator.go` calling `internal/session/executor.go` and `internal/session/spawner.go`.
**Decision:** I will design the adversarial E2E integration test suite around the `Campaign Orchestrator ↔ Session Executor` boundary. This is a complex boundary because the Orchestrator (in `internal/campaign/`) invokes tasks via `session.TaskExecutor` (which is implemented by `internal/session/executor.go` and `internal/session/spawner.go`).
**Learning:** `setupTestExecutor` in `tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go` sets up a test executor with mocks. However, my goal is to create an adversarial test suite for `Campaign Orchestrator <-> Session Executor`. I will create a new test file: `tests/e2e/campaign_session_integration_test.go`. Let's create an environment for Campaign Orchestrator connected to Session Executor. The Orchestrator takes a `TaskExecutor` which can be `session.NewJITExecutor(session.NewExecutor(...), session.NewSpawner(...), ...)`.
**Action:** Let's look at `internal/campaign/orchestrator_di_test.go` to see how Orchestrator dependencies are set up for testing.
**Learning:** `OrchestratorConfig` accepts a `TaskExecutor`. I need to instantiate a real `JITExecutor`, which needs a real `Executor` and `Spawner` from `session`. The `Session` components need `Kernel`, `VirtualStore`, `LLMClient`, `JITCompiler`, `ConfigFactory`, `Transducer`. Let's mock the boundaries *outside* of Orchestrator and Session, so `MockKernel`, `MockLLMClient`, etc., but use the real `internal/campaign` and `internal/session` components to test their interaction boundary.

Let's read `tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go` to steal the mock objects and create a robust setup.
**Learning:** `OrchestratorConfig` has `TaskExecutor`. We'll write the journal as requested, then write the test. The test will be `tests/e2e/campaign_session_integration_test.go`.

**System Interaction Map:**
1. Campaign Orchestrator runs a Phase (`runPhase` in `internal/campaign/orchestrator_tasks.go`)
2. For each task, Orchestrator calls `o.spawnTask` (`internal/campaign/orchestrator_task_handlers.go`).
3. `spawnTask` calls `taskExecutor.ExecuteAsync` or `Execute`.
4. `TaskExecutor` (specifically `JITExecutor` in `internal/session/task_executor.go`) calls `executor.Process` or `spawner.Spawn`.
5. The `Executor` does a JIT compilation and interacts with the mock LLM.
6. The `Executor` finishes and the result flows back to `TaskExecutor`.
7. `Orchestrator` receives the task result via a channel or callback and updates state.
**Finding:** Orchestrator calls `te.Execute(ctx, intent, task)`. `JITExecutor.Execute` ends up running tasks sequentially (since it delegates to `executeWithSubagent` or `executor.Process`). But Orchestrator has bounded parallelism: `results := make(chan taskResult, o.maxParallelTasks*2)`.
Is Orchestrator calling `spawnTask` inside goroutines?
**Learning:** `Orchestrator` schedules tasks in goroutines (`go o.runSingleTask`). This is bounded by `maxParallelTasks`.
`runSingleTask` calls `executeTask`, which calls `spawnTask`, which calls `taskExecutor.Execute(ctx, intent, task)`.

Let's look at `JITExecutor.Execute`. Wait, I want to create the journal first.
**Finding:** Orchestrator sets `MaxParallelTasks` and loops in goroutines. `JITExecutor.ExecuteWithContext` sees `needsSubagent` returns `false` for `/fix`, `/test`. For inline execution: `j.executor.SetSessionContext(sessionCtx)`. Wait! `SetSessionContext is not thread-safe. For true concurrent execution, use ExecuteAsync`. BUT the Orchestrator does NOT use `ExecuteAsync`. It calls `te.Execute(ctx, intent, task)` inside the `runPhase` goroutines (`runSingleTask`).
This is a HUGE state corruption vulnerability. Concurrent task executions from the Campaign Orchestrator will overwrite `j.executor.SetSessionContext` leading to cross-talk between tasks in the same phase!

Also, `Executor.Process` runs the JIT compilation and updates `e.conversationHistory` without locking!

This means `Campaign Orchestrator ↔ Session Executor` has a critical thread-safety violation when `JITExecutor` falls back to inline execution for non-complex intents.

I will write the `.e2e_quality_assurance` journal based on this.

**Subsystems Tested:** Campaign Orchestrator, Session Executor, Spawner.

## 2026-05-19 - [Context Bleed via JITExecutor Inline Execution]
**Learning:** The Campaign Orchestrator executes phase tasks in parallel goroutines, invoking the `TaskExecutor`. However, when `JITExecutor` opts for "inline" execution instead of spawning a subagent (e.g., for simple intents like `/fix`), it calls `executor.SetSessionContext()`. This function mutates the shared `Executor` instance state and is explicitly documented as not thread-safe. Consequently, concurrent inline tasks overwrite each other's context, leading to catastrophic context bleed and data races in `conversationHistory`.
**Action:** Always write adversarial scenarios that exploit JIT/Optimization fallbacks. When an architecture provides a "fast path" (like inline execution) and a "safe path" (like subagent spawning), force massive parallel execution through the fast path to break implicit shared-state contracts.
