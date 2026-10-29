---
surface: "Campaign_Decomposer"
mode: "pipeline"
subsystems_tested: ["campaign.Orchestrator", "campaign.Decomposer", "session.Executor", "core.VirtualStore"]
blast_radius: "critical"
remediated: false
---

## System Interaction Map

The codeNERD campaign system relies heavily on the `campaign.Orchestrator` to manage multi-phase execution. The primary interaction map between the orchestrator, the decomposer, and the execution layer involves several deep cross-boundary calls:

1.  **Initialization Boundary (`campaign.Start`):**
    The user invokes the campaign command. This triggers the campaign initialization phase. The system takes the raw user intent and feeds it into the campaign lifecycle manager. At this stage, the orchestrator sets up the basic workspace and kernel session scope.
    *Function calls:* `campaign.NewOrchestrator(config)`, `orchestrator.Start(ctx, goal)`.

2.  **Decomposition Boundary (`campaign.Decomposer`):**
    The orchestrator needs a plan. It delegates to the `Decomposer`. The `Decomposer` takes the high-level goal and uses an LLM (typically a highly capable model) to break the goal into a sequence of `Phases`. Each `Phase` contains multiple parallelizable `Tasks`.
    *Function calls:* `decomposer.Decompose(ctx, goal)`, `llm.Complete(ctx, prompt)`. The boundary here is between the deterministic campaign state machine and the non-deterministic LLM output, which must be strictly parsed into JSON conforming to the `campaign.Plan` schema.

3.  **Scheduling Boundary (`campaign.Orchestrator` -> `session.Spawner`):**
    Once a plan is established, the Orchestrator enters a state machine loop. For each phase, it iterates through the tasks. For each task, it must create an isolated execution environment. It uses the `session.Spawner` to create a `SubAgent`.
    *Function calls:* `spawner.Spawn(ctx, config)`. This is a critical boundary because the `Spawner` allocates resources (kernel isolation scopes, memory boundaries) based on the task description.

4.  **Execution Boundary (`session.Executor`):**
    With a SubAgent ready, the orchestrator dispatches the task. The `session.Executor` takes over the JIT execution loop. It compiles the prompt, calls the LLM, parses tools, and executes them.
    *Function calls:* `executor.Execute(ctx, prompt)`. The orchestrator typically wraps this in a goroutine to allow parallel task execution within a phase, constrained by `MaxParallelTasks`.

5.  **VirtualStore Boundary (`core.VirtualStore`):**
    During task execution, the `session.Executor` will invoke tools via the `VirtualStore` (e.g., `write_file`, `bash`). The `VirtualStore` interacts with the underlying OS and the Mangle kernel (for safety via the Dreamer).
    *Function calls:* `vStore.ExecuteTool(ctx, call)`.

6.  **State Transition Boundary (`campaign.Orchestrator`):**
    As tasks complete, they signal the Orchestrator. The orchestrator must synchronize these signals. A phase ONLY transitions when ALL its constituent tasks have successfully completed. The output of the completed tasks must be collected, summarized, and paged into the context for the next phase.
    *Function calls:* `orchestrator.monitorTasks()`, `orchestrator.transitionPhase()`.

## Contract Analysis

Deep architectural contracts exist at these boundaries:

1.  **Acyclic Execution Graph Contract:**
    The `Decomposer` is entrusted with generating the task graph. The implicit contract is that the LLM will generate a Directed Acyclic Graph (DAG) of dependencies. If the LLM generates a cyclical dependency (e.g., Task A depends on Task B, which depends on Task A), the Orchestrator MUST detect this and reject the plan before execution begins. Failure to do so results in a deadlock where neither task can ever be scheduled.

2.  **Phase Gating and Synchronization Contract:**
    A Phase acts as a hard synchronization barrier. The Orchestrator contract guarantees that no task from Phase N+1 will begin execution until every single task in Phase N has returned a success status. If this contract is violated, tasks in Phase N+1 might operate on incomplete or missing state, leading to unpredictable and cascading failures.

3.  **SubAgent State Isolation Contract:**
    When the Orchestrator uses the Spawner to create a SubAgent for a task, the contract guarantees that the SubAgent's state (its Mangle facts, its temporary files, its execution history) is isolated from other concurrently running SubAgents in the same phase. If Task 1 and Task 2 run simultaneously, Task 1 must not be able to inadvertently read or overwrite Task 2's specific kernel facts unless explicitly shared via global state.

4.  **Fail-Fast and Escalation Contract:**
    If a critical task within a phase fails (e.g., panics, times out, or exhausts its retry budget), the Orchestrator must immediately halt the scheduling of any new tasks in that phase, gracefully cancel any currently running tasks, and escalate the failure. The campaign should enter a recovery/replanning loop or abort entirely. Silent failures or continuing despite a failed critical dependency breaks the integrity of the entire campaign.

5.  **Context Paging and Budget Limit Contract:**
    As a campaign progresses, the Orchestrator accumulates state (outputs from previous phases). The contract with the downstream `session.Executor` is that the Orchestrator will never feed it a context string that exceeds the LLM's physical token limits. The Orchestrator must employ semantic compression or truncation. If it naively concatenates 10MB of logs, the downstream prompt compiler will fail or the LLM client will return a 400 Bad Request.

6.  **Resource Cleanup Contract:**
    The Orchestrator must clean up resources associated with a task once it completes. This includes canceling context trees, freeing API slots acquired via the `APIScheduler`, and releasing any isolated kernel resources. Goroutine leaks at the task execution boundary will quickly exhaust system resources during long-running campaigns.

## Failure Mode Enumeration

1.  **Temporal:**
    *   **Decomposer Timeout:** The initial LLM call to decompose the goal takes too long, stalling the entire campaign initialization.
    *   **Task Hang:** A `session.Executor` gets stuck in an infinite TDD loop or a hanging bash command. If the Orchestrator doesn't enforce a strict task timeout, the phase never completes.
    *   **Phase Gating Timeout:** All tasks complete, but the synchronization channel drops a message, causing the orchestrator to wait forever.

2.  **Semantic:**
    *   **Cyclical Dependencies:** The Decomposer returns syntactically valid JSON, but the dependency logic is flawed (cycles).
    *   **Hallucinated Dependencies:** A task claims to depend on a task ID that doesn't exist in the phase.
    *   **JSON Truncation:** The LLM runs out of output tokens while generating the plan JSON, resulting in a structurally invalid string that crashes the parser if not handled safely.

3.  **Ordering:**
    *   **Premature Transition:** A race condition in the `monitorTasks` loop causes the orchestrator to increment the phase counter before the last background task actually finishes its cleanup defer block.
    *   **Out-of-Order Execution:** The scheduler ignores dependency lists and starts Task B before Task A finishes.

4.  **Partial:**
    *   **Shard Panic:** A SubAgent experiences a nil pointer dereference deep in the VirtualStore during execution. The panic bubbles up. If the Orchestrator doesn't recover it, the entire campaign process crashes.
    *   **Mid-Campaign Abort:** The user hits Ctrl+C. The context is canceled. If the orchestrator doesn't handle this gracefully, the campaign state is corrupted and cannot be resumed.

5.  **Corruption:**
    *   **Context Bleed:** The Orchestrator reuses a struct or a map between two parallel tasks without proper mutexes, causing one task to overwrite the prompt of another.
    *   **Global State Contamination:** A task asserts a fact into the shared Mangle kernel without scoping it to its SubAgent ID, polluting the global intent routing for subsequent tasks.

## Adversarial Scenario Design

1.  **Scenario: Cyclical Dependency Injection (P0)**
    *   **Violated Contract:** Acyclic Execution Graph.
    *   **Injection Mechanism:** We mock the `Decomposer` LLM client to return a hardcoded JSON plan where Task A depends on Task B, and Task B depends on Task A.
    *   **Expected System Behavior:** The `campaign.Start` or plan validation phase MUST return an error indicating a cycle was detected. It must not enter an infinite scheduling loop or deadlock.
    *   **Severity:** P0 (Complete system halt).

2.  **Scenario: Orchestrator Resilience Against Task Panic (P1)**
    *   **Violated Contract:** Fail-Fast and Escalation / Orchestrator stability.
    *   **Injection Mechanism:** We inject a custom `TaskExecutor` mock that deliberately panics when its `Execute` method is called.
    *   **Expected System Behavior:** The Orchestrator's execution goroutine must use a `defer recover()` block to catch the panic. The task should be marked as failed, the phase should be halted, and the error should be propagated up cleanly. The main application must not crash.
    *   **Severity:** P1 (Process crash).

3.  **Scenario: Silent Task Hang and Timeout Enforcement (P1)**
    *   **Violated Contract:** Progress monitoring and task timeouts.
    *   **Injection Mechanism:** We mock the `TaskExecutor` to block indefinitely (e.g., reading from an empty channel). We configure the Orchestrator with a short `TaskTimeout`.
    *   **Expected System Behavior:** The Orchestrator must enforce the timeout, cancel the context passed to the hanging task, mark the task as failed (timeout), and proceed with error handling for the phase.
    *   **Severity:** P1 (Resource starvation).

4.  **Scenario: Extreme Phase Scaling and Concurrency Limits (P2)**
    *   **Violated Contract:** Resource limits.
    *   **Injection Mechanism:** We mock the `Decomposer` to return a phase containing 10,000 independent tasks. We set `MaxParallelTasks` to 5.
    *   **Expected System Behavior:** The Orchestrator must not attempt to spawn 10,000 goroutines simultaneously. It must queue them and execute them in batches of 5, without causing OOM or descriptor exhaustion.
    *   **Severity:** P2 (Performance degradation).

5.  **Scenario: Context Paging Overflow at Phase Boundary (P1)**
    *   **Violated Contract:** Token budget enforcement across phases.
    *   **Injection Mechanism:** Task A in Phase 1 completes and returns a 50MB string as its result. The Orchestrator attempts to transition to Phase 2.
    *   **Expected System Behavior:** The Orchestrator must truncate, summarize, or gracefully reject the massive state before passing it to Phase 2. It must not pass the 50MB string to the downstream LLM client, which would cause an immediate hard failure.
    *   **Severity:** P1 (Downstream system crash).

6.  **Scenario: Synchronized Phase Transition Race (P1)**
    *   **Violated Contract:** Phase Gating and Synchronization.
    *   **Injection Mechanism:** We create a phase with 100 parallel tasks. We use a `sync.WaitGroup` inside our mock `TaskExecutor` to block all tasks until a signal is given, then release them all simultaneously so they complete at the exact same microsecond.
    *   **Expected System Behavior:** The Orchestrator's state machine must handle the thunderous herd of completion signals safely without race conditions. It must transition to the next phase exactly once.
    *   **Severity:** P1 (State machine corruption).

7.  **Scenario: Safe JSON Parsing on Truncation (P1)**
    *   **Violated Contract:** Safe boundaries with non-deterministic LLMs.
    *   **Injection Mechanism:** We mock the `Decomposer` LLM to return a half-finished, syntactically invalid JSON string (simulating hitting max tokens).
    *   **Expected System Behavior:** The `json.Unmarshal` will fail. The Orchestrator must catch this error, trigger a retry on the decomposer, or fail gracefully. It must not panic.
    *   **Severity:** P1 (Panic).

8.  **Scenario: Mid-Campaign Context Cancellation (P1)**
    *   **Violated Contract:** Clean abort and resource cleanup.
    *   **Injection Mechanism:** We start a campaign. While Phase 2 tasks are running, we trigger `cancel()` on the parent context.
    *   **Expected System Behavior:** The Orchestrator must immediately signal all running tasks to stop. It must wait for them to acknowledge the cancellation (via their own context checks) and return. No goroutines should be left running after `Start()` returns.
    *   **Severity:** P1 (Goroutine leak).

9.  **Scenario: Missing Task Dependencies Validation (P2)**
    *   **Violated Contract:** Valid execution graph.
    *   **Injection Mechanism:** We mock a plan where Task A depends on "Task X", but "Task X" does not exist in the phase list.
    *   **Expected System Behavior:** The Orchestrator validates the dependency graph before execution and returns an error about the missing dependency.
    *   **Severity:** P2 (Execution stall).

10. **Scenario: Re-Planning Loop Exhaustion (P2)**
    *   **Violated Contract:** Bounded retries.
    *   **Injection Mechanism:** We mock a task to always fail. We set the Orchestrator's `MaxRetries` to 3.
    *   **Expected System Behavior:** The Orchestrator attempts the task 3 times, fails on the 4th, and then hard-fails the campaign, avoiding an infinite loop.
    *   **Severity:** P2 (Infinite loop).

11. **Scenario: SubAgent ID Collision (P1)**
    *   **Violated Contract:** Unique tracking.
    *   **Injection Mechanism:** We inject a mock `Spawner` that returns the same SubAgent ID for two different parallel tasks.
    *   **Expected System Behavior:** If the Orchestrator uses the ID as a map key, it must detect the collision and error out, rather than silently overwriting task state.
    *   **Severity:** P1 (State corruption).

12. **Scenario: Mismatched Phase Inputs/Outputs (P2)**
    *   **Violated Contract:** Data pipeline integrity.
    *   **Injection Mechanism:** Phase 2 explicitly requires an output variable from Phase 1. We mock Phase 1 to return success but without providing that output.
    *   **Expected System Behavior:** The Orchestrator or the Task execution validation should catch the missing input and fail the task with a clear missing dependency error.
    *   **Severity:** P2 (Logic error).

13. **Scenario: Global Tool Rate Limiting / API Contention (P2)**
    *   **Violated Contract:** Resource scheduling.
    *   **Injection Mechanism:** We mock 50 parallel tasks that all immediately request the LLM API. We restrict the mock `APIScheduler` to 3 concurrent slots.
    *   **Expected System Behavior:** The tasks must queue up and execute 3 at a time without timing out prematurely (assuming overall phase timeout is sufficient).
    *   **Severity:** P2 (False positive failures).

14. **Scenario: Campaign State Serialization Integrity (P1)**
    *   **Violated Contract:** Durable resumption.
    *   **Injection Mechanism:** We run a campaign halfway through. We cancel it. We serialize the Orchestrator state to JSON. We deserialize it into a new Orchestrator instance and call resume.
    *   **Expected System Behavior:** The campaign must resume from the exact phase and task it left off at, without repeating completed tasks.
    *   **Severity:** P1 (Data loss).

15. **Scenario: SubAgent Kernel Isolation (P1)**
    *   **Violated Contract:** Strict memory bounds.
    *   **Injection Mechanism:** Task A asserts a specific fact into its kernel instance. Task B (running concurrently) queries for that fact.
    *   **Expected System Behavior:** Task B must return 0 results. The orchestrator's spawning logic must ensure isolated kernel namespaces or transaction scopes.
    *   **Severity:** P1 (Cross-talk).

## Cascading Failure Analysis

**The Anatomy of a Context Paging Overflow Cascade (Scenario 5):**

The boundary between phase output and the subsequent phase input is a highly vulnerable seam. Consider the scenario where Task A (Phase 1) is instructed to "read the log file and summarize errors." Due to a flawed prompt or a massive log file, the SubAgent's LLM fails to summarize and instead outputs the entire 50MB log file as its result.

1.  **The Trigger:** Task A completes successfully from its own perspective and returns the 50MB string to the `Orchestrator`.
2.  **The Flaw:** The `Orchestrator` aggregates phase results. Without semantic compression, it naively concatenates this 50MB string into the "Previous Phase Context" field for Phase 2 tasks.
3.  **The First Cascade:** The Orchestrator spawns Task B for Phase 2. The `session.JITExecutor` receives the 50MB context.
4.  **The Deep Cascade:** The `JITExecutor` passes this to the `PromptCompiler`. The `TokenBudgetManager` attempts to allocate tokens. Because the input context vastly exceeds the LLM's maximum context window (e.g., 200k tokens), the budget manager forcefully truncates the prompt.
5.  **The Catastrophe:** If the budget manager prioritizes the massive context over the system instructions or tool schemas (a common failure mode if weights are misconfigured), the LLM receives the 50MB log but *loses* the `Piggyback Protocol` instructions and tool schemas.
6.  **The Impact:** The downstream LLM, stripped of its instructions, hallucinated a generic response instead of a structured control packet. The `Articulation` system fails to parse the JSON. The `VirtualStore` rejects the action. Task B fails.
7.  **The Result:** The entire campaign crashes out, not because of a logic error in Phase 2, but because an uncompressed artifact from Phase 1 destroyed the protocol boundary for the subsequent phase.

**The Anatomy of a Missing Dependency Cascade (Scenario 12):**
If the orchestrator allows execution of a DAG with missing internal dependencies:
1.  **The Flaw:** Task C depends on Task B, but Task B was typoed in the LLM response as Task Z. The Orchestrator accepts the DAG.
2.  **The Cascade:** Task C is placed in a waiting queue. Since Task B exists but doesn't resolve "Task Z", Task C waits indefinitely.
3.  **The Deadlock:** The entire phase cannot complete because Task C is part of it.
4.  **The Result:** The campaign hangs until the overarching timeout kills it.

**The Anatomy of SubAgent State Bleed Cascade (Scenario 3 & 15):**
1.  **The Trigger:** Concurrent tasks execute.
2.  **The Flaw:** The `spawner.Spawn` uses the root `core.Kernel` without proper namespace isolation or `KernelTx` scopes.
3.  **The Cascade:** Task 1 asserts `permitted_action(/delete_file, "foo.txt")`. Task 2, which was not supposed to delete anything, happens to hallucinate a `delete_file` call for `foo.txt`.
4.  **The Catastrophe:** The `VirtualStore` queries the kernel. Because Task 1's fact is globally visible, it incorrectly authorizes Task 2's dangerous action.
5.  **The Result:** Unauthorized file deletion due to cross-agent state contamination.

**The Anatomy of Orchestrator Panic Cascade (Scenario 2):**
1.  **The Flaw:** A deeply nested function in the VirtualStore during task execution hits a nil pointer deref.
2.  **The Cascade:** The goroutine panics. The `Orchestrator` did not wrap the `Executor.Execute` call in a `defer recover()`.
3.  **The Result:** The panic escapes the goroutine, bringing down the entire `nerd` process. All other running campaigns and the entire CLI session die abruptly.

## Actionable Remediation Strategies for Discovered Seams

To mitigate the issues discovered in these adversarial scenarios, the following architectural invariants must be enforced:

1.  **Strict DAG Validation:** Implement Tarjan's strongly connected components algorithm inside the `Decomposer` output parser to categorically reject cyclic dependencies before the state machine initializes.
2.  **Safe Concurrency Caps:** The `MaxParallelTasks` configuration must be strictly enforced via a worker pool pattern or a semaphore, guaranteeing that the Orchestrator will never spawn unbounded goroutines regardless of the LLM's hallucinated task counts.
3.  **Mandatory Resource Timeouts:** Every context passed across the Orchestrator -> Executor boundary MUST have a definitive deadline. `context.Background()` must never cross this boundary.
4.  **Semantic Paging Thresholds:** Implement a hard byte limit (e.g., 8192 bytes) on the context string that can be forwarded from Phase N to Phase N+1. If exceeded, the Orchestrator must enforce a summarization sub-task or truncate with a warning marker.
5.  **Kernel Transaction Scopes:** The `session.Spawner` must wrap the `core.Kernel` in a `MemoryTier` or a specific `KernelTx` that automatically rolls back all asserted EDB facts when the task context is canceled.
