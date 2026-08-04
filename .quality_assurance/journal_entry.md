# Quality Assurance Journal
## 2026-07-23 23:30:38 EST

## System Reviewed: Campaign Orchestrator (`internal/campaign/orchestrator_execution.go` & `internal/campaign/orchestrator_tasks.go`)

### 1. Executive Summary & Context
This journal entry documents a rigorous Boundary Value Analysis (BVA) and Negative Testing review of the `internal/campaign` orchestrator subsystem within the codeNERD architecture. The orchestrator manages the execution lifecycle of complex, multi-phase campaigns, coordinating tasks, synchronizing state with the Mangle deductive database (Kernel), and ensuring durability via checkpoints.

The review specifically focused on identifying edge cases, failure modes, and potential vulnerabilities across four critical vectors:
1.  **Null/Undefined/Empty Data:** Handling of missing configurations, uninitialized dependencies, and empty collections.
2.  **Type Coercion & Schema Integrity:** Robustness against malformed data crossing the Go/Mangle boundary and internal type assumptions.
3.  **User Request Extremes (Load & Scale):** Performance degradation, memory leaks, and exhaustion of file descriptors under massive workloads or adversarial inputs.
4.  **State Conflicts & Race Conditions:** Concurrency issues, desynchronization between Go's runtime state and Mangle's transactional state, and TOC/TOU vulnerabilities.

Currently, the primary test file (`internal/campaign/orchestrator_execution_test.go`) contains a mere `TestOrchestratorExecution_Placeholder` which satisfies the build but provides zero coverage for the intricate state machine logic housed within the orchestrator. This represents a critical testing gap that must be addressed to ensure system stability.

### 2. Deep Dive Analysis by Vector

#### 2.1 Null/Undefined/Empty Values

The orchestrator heavily relies on pointer receivers and nested structs. Failure to initialize these correctly, or encountering unexpected zero-values during runtime, can lead to severe panics or infinite loops.

*   **Empty Campaign Identity (`o.campaign.ID` / `o.campaign.Title`):**
    *   **Scenario:** A campaign is loaded into memory, but its `ID` or `Title` is an empty string `""`.
    *   **Impact:** The `runHeartbeatLoop` periodically pushes facts to Mangle: `campaign_heartbeat(campaignID, timestamp)`. If `campaignID` is empty, Mangle might treat it as a valid atom (depending on schema configuration), but subsequent queries looking for a specific ID will fail. Furthermore, file path generation for durable storage often relies on the campaign ID. An empty ID could lead to attempting to write checkpoints to the root directory `/`, risking catastrophic file system corruption.
    *   **Recommendation:** Implement strict validation during `orchestrator.Load()` to reject campaigns with empty IDs or Titles.
    *   **Performant Enough?** Yes. A simple string length check is `O(1)` and incurs negligible overhead.

*   **Partially Initialized Configuration (`o.config`):**
    *   **Scenario:** The orchestrator is instantiated with a `Config` struct where `CampaignTimeout` is `0` or negative.
    *   **Impact:** The `Run` method contains the logic: `if o.config.CampaignTimeout > 0 { ctx, cancel = context.WithTimeout(...) }`. If it's zero, the context has no timeout, which might be intended (infinite campaign). But what if the configuration *intended* a default timeout but failed to set it? More critically, if `AutosaveEvery` or `HeartbeatEvery` are zero, the `time.NewTicker(0)` call in Go will panic (`panic: non-positive interval for NewTicker`).
    *   **Recommendation:** Introduce a `Validate()` method on the config struct that enforces sensible defaults (e.g., minimum 1-second interval for tickers).
    *   **Performant Enough?** Yes, validation happens once at initialization.

*   **Uninitialized Context Pager (`o.contextPager == nil`):**
    *   **Scenario:** The system transitions to a new phase, and the orchestrator attempts to page in context. The code contains a defensive check `if o.contextPager != nil`, meaning it will silently skip context paging.
    *   **Impact:** While it prevents a panic, skipping context paging means the LLM will execute tasks without essential background information, leading to severe hallucination or task failure. This silent degradation is insidious.
    *   **Recommendation:** If a campaign requires context (most do), the absence of a pager should be a fatal error, not a silently skipped feature.
    *   **Performant Enough?** Yes, it's a simple nil check.

*   **Zero Tasks in Active Phase:**
    *   **Scenario:** Mangle dictates that Phase X is the `current_phase`, but Phase X contains zero tasks in `phase.Tasks`.
    *   **Impact:** In `runPhase`, the task scheduling loop iterates over `currentPhase.Tasks`. If empty, `len(upcoming) == 0`. The loop immediately exits, and `o.isPhaseComplete(phase)` evaluates to true. The orchestrator runs a checkpoint and advances. However, if this empty phase was meant to perform crucial setup (e.g., waiting for an external event), skipping it instantly breaks the workflow. More dangerously, if a phase is *dynamically* populated but Mangle advances the state before the population occurs, the orchestrator races ahead.
    *   **Recommendation:** Define semantics for empty phases. Should they automatically complete, or wait for tasks to be dynamically injected?
    *   **Performant Enough?** The loop exit is fast, but the semantic correctness is flawed.

*   **Missing Pause Channel (`o.pauseCh == nil` in Paused State):**
    *   **Scenario:** A concurrent mutation sets `o.isPaused = true` but fails to instantiate `o.pauseCh`.
    *   **Impact:** The main execution loop in `Run()` blocks on `<-pauseCh`. If `pauseCh` is nil, receiving from a nil channel blocks *forever*, effectively deadlocking the campaign execution loop and ignoring cancellation signals.
    *   **Recommendation:** Encapsulate pause logic in a method that atomicly updates the boolean and the channel together.
    *   **Performant Enough?** Channel operations are standard, but the deadlock risk is high.

#### 2.2 Type Coercion & Boundary Violations

CodeNERD bridges the gap between Go's imperative, strongly-typed environment and Mangle's declarative, loosely-typed logical database. This boundary is a prime vector for type coercion vulnerabilities.

*   **Mangle Block Reason Coercion (`getCampaignBlockReason()`):**
    *   **Scenario:** Mangle returns a `campaign_blocked` fact containing a reason string.
    *   **Impact:** What if the string is 100MB long? Or contains invalid UTF-8 sequences? Or shell injection sequences? The Go orchestrator blindly extracts this string and logs it: `logging.Get(...).Error("Campaign blocked: %s", blockReason)`. Logging a 100MB string can exhaust memory or overwhelm the logging ingestor.
    *   **Recommendation:** Truncate string arguments extracted from Mangle facts before logging or storing them in Go structs. Validate UTF-8 compliance.
    *   **Performant Enough?** String truncation (e.g., `str[:min(len(str), 1024)]`) is very fast.

*   **Heartbeat Timestamp Precision Loss (`time.Now().Unix()`):**
    *   **Scenario:** The orchestrator asserts `campaign_heartbeat(ID, time.Now().Unix())`.
    *   **Impact:** Go's `Unix()` returns an `int64`. Does the Mangle schema define this argument as an `Int64` or a `Float64`? If Mangle implicitly coerces this to a standard IEEE 754 float, precision loss occurs above $2^{53}$. While Unix timestamps won't hit this limit soon, differences in numeric representation between Go and Mangle are a persistent source of join failures.
    *   **Recommendation:** Ensure strict type mapping. If Mangle expects a specific numeric type, format the Go assertion explicitly to match.
    *   **Performant Enough?** Formatting numeric types is standard and fast.

*   **Mangle Current Phase Coercion (Ghost Phases):**
    *   **Scenario:** Mangle evaluates its rules and returns `current_phase(PhaseID)`. However, `PhaseID` does not exist in `o.campaign.Phases`.
    *   **Impact:** The Go orchestrator attempts to find the phase: `currentPhase := o.getCurrentPhase()`. If it returns nil, the orchestrator assumes the phase hasn't started and calls `startNextPhase(ctx)`. If Mangle continues to assert the "ghost" phase, `startNextPhase` might spin infinitely, trying to transition to a state it cannot represent in Go.
    *   **Recommendation:** If Mangle asserts a current phase that Go cannot find, the system must panic or enter a terminal failure state (schema drift detected), rather than looping.
    *   **Performant Enough?** The lookup is `O(N)` where N is the number of phases, which is small. The infinite loop is the performance issue.

*   **Fact Type Mismatch (Atom vs String):**
    *   **Scenario:** A fact is asserted into Mangle where a parameter is a String `"active"`, but the Mangle schema or rules expect an Atom `/active`.
    *   **Impact:** Mangle treats Strings and Atoms as strictly disjoint sets. A join between `status(X, "active")` and `rule(X) :- status(X, /active)` will yield zero results. This silent failure causes the orchestrator to stall, believing no tasks are eligible.
    *   **Recommendation:** Enforce strict type checking in the Go bindings that assert facts into the `core.Kernel`.

#### 2.3 User Request Extremes (Load, Scale, Chaos)

CodeNERD must handle immense monorepos and complex user requests without buckling.

*   **Massive Campaigns (1,000,000 Phases/Tasks):**
    *   **Scenario:** A user requests a refactoring of a 50-million-line monorepo, resulting in a campaign decomposed into a million micro-tasks.
    *   **Impact:**
        *   **Memory:** The `o.campaign` struct holds all phases and tasks in memory. A million tasks will consume gigabytes of RAM, likely triggering an OOM killer before execution even begins.
        *   **Channel Exhaustion:** In `runPhase`, `results := make(chan taskResult, o.maxParallelTasks*2)` is bounded, which is good. However, if `maxParallelTasks` is scaled proportionally to the campaign size, the channel allocation could fail.
    *   **Recommendation:** The Orchestrator must implement a streaming or paginated architecture for tasks. Only the active phase and adjacent phases should be fully loaded into memory. Completed phases must be offloaded to disk/database.
    *   **Performant Enough?** The current full-memory model is NOT performant enough for this extreme. It requires architectural redesign (e.g., Cursor-based task iteration).

*   **High-frequency Context Switching (Empty Phase Thrashing):**
    *   **Scenario:** A sequence of 100 phases, each completing instantly (e.g., logical checkpoints without concrete tasks).
    *   **Impact:** On every phase transition, `o.contextPager.ResetPhaseContext()` and `ActivatePhase()` are called. If `ActivatePhase` involves expensive vector database queries or LLM calls to summarize previous context, rapid transitions will overwhelm the LLM API (rate limits) and spike CPU usage.
    *   **Recommendation:** Implement debouncing or lazy-loading for context paging. Only page in context when a concrete task actually requires execution.
    *   **Performant Enough?** The current synchronous paging on transition is a bottleneck under this extreme.

*   **Extremely Long Phase Names (Logging Abuse):**
    *   **Scenario:** A decomposed task generates a phase name derived from a 10MB file path or adversarial input.
    *   **Impact:** `logging.StartTimer(..., fmt.Sprintf("runPhase(%s)", phase.Name))` will allocate a 10MB string for the timer label. The logging subsystem will attempt to write this massive string to disk, causing I/O blocking and rapid disk space exhaustion.
    *   **Recommendation:** Apply strict length constraints (e.g., `phase.Name[:min(len(phase.Name), 64)]`) when logging or using names as metrics labels.
    *   **Performant Enough?** Yes, truncating before logging saves significant overhead.

*   **Deeply Nested Directories (File System Limitations):**
    *   **Scenario:** A task requires writing an artifact to a path generated by appending many directories: `a/b/c/d/e/...` exceeding the OS path length limit (e.g., 260 characters on Windows, 4096 on Linux).
    *   **Impact:** The `write_set_lock_manager` or the task executor will crash with `syscall.ENAMETOOLONG`. If this error is not gracefully caught and routed to a failure replan, the orchestrator will panic.
    *   **Recommendation:** Implement path length validation before attempting file I/O. Use hash-based shortened paths if the logical path exceeds limits.

#### 2.4 State Conflicts & Race Conditions

The orchestrator operates in a highly concurrent environment, managing goroutines for tasks, heartbeats, and Mangle queries.

*   **Concurrent Heartbeat & Pause/Cancellation (`F-RACE-1`):**
    *   **Scenario:** The user cancels the campaign (`ctx.Done()`). The main loop catches this, acquires `o.mu.Lock()`, sets `o.updateCampaignStatus(StatusPaused)`, calls `_ = o.saveCampaign()`, and returns. Simultaneously, the `runHeartbeatLoop` ticker fires.
    *   **Impact:** The heartbeat loop acquires `o.mu.RLock()`, gets the `campaignID`, releases the lock, and then executes a Mangle transaction to update `campaign_heartbeat`. It then hits the `autosaveTicker` and calls `_ = o.saveCampaign()`. If the main loop's `saveCampaign` (which marks it paused) interleaves with the heartbeat's `saveCampaign` (which might operate on a slightly stale snapshot), the campaign state on disk might revert to "Active" even though the execution loop has exited.
    *   **Recommendation:** The heartbeat loop must immediately exit when the context is cancelled, *before* attempting any further saves. Ensure atomic transitions of state.
    *   **Performant Enough?** Yes, context checking is virtually free.

*   **State Desynchronization (Mangle vs Go):**
    *   **Scenario:** A phase completes. The orchestrator updates its internal Go structs. It then attempts to assert the `phase_completed` fact to Mangle. The Mangle transaction fails (e.g., constraint violation, memory issue).
    *   **Impact:** The Go orchestrator believes the phase is done. Mangle believes it is not. The orchestrator queries Mangle for the next phase, but Mangle returns the *same* phase again. The orchestrator sees the Go state is complete, skips tasks, and loops infinitely, thrashing the CPU.
    *   **Recommendation:** Use a Two-Phase Commit pattern or treat Mangle as the absolute source of truth. If a Mangle transaction fails, the Go state MUST be rolled back or the orchestrator must crash (fail-fast).
    *   **Performant Enough?** Rollbacks are expensive, but infinite loops are fatal.

*   **Parallel Phase Execution via Corrupted Logic:**
    *   **Scenario:** Mangle logic is compromised (e.g., missing stratification), causing `getCurrentPhase()` to return multiple active phases, or causing `startNextPhase()` to spawn multiple concurrent `runPhase` loops (if the code allowed it, though currently it loops sequentially).
    *   **Impact:** If multiple `runPhase` executions ran concurrently on the same `o.campaign`, they would race on the `o.campaign.Phases` slice, corrupting task statuses and checkpoint counters.
    *   **Recommendation:** Ensure `getCurrentPhase()` strictly validates that only ONE phase is returned. If Mangle returns multiple, throw a fatal schema violation error.
    *   **Performant Enough?** Yes, checking the length of the result set is `O(1)`.

*   **The Infinite Loop State (`F-STALL-1`):**
    *   **Scenario:** Mangle program fails to stratify (e.g., `p :- not p`) or enters an infinite derivation loop.
    *   **Impact:** The orchestrator's `kernel.Query` calls block forever. The campaign freezes silently. No CPU usage, no logs, just deadlocked.
    *   **Recommendation:** EVERY call to `kernel.Query` or `kernel.Transaction` MUST use a context with a strict timeout.
    *   **Performant Enough?** Context timeouts add negligible overhead but provide essential safety bounds.

### 3. Conclusion and Next Steps

The `internal/campaign` orchestrator demonstrates a sophisticated design integrating LLM task execution with deductive logic programming. However, the current lack of negative testing leaves it vulnerable to data corruption, race conditions, and catastrophic failures under extreme loads.

**Immediate Actions Required:**
1.  **Implement Table-Driven Negative Tests:** Create exhaustive tests in `orchestrator_execution_test.go` that inject nil values, malformed JSON, and massive strings.
2.  **Mangle Type Enforcement:** Audit all `core.Fact` assertions to ensure strict alignment between Go types (string/int) and Mangle Schema types (Atom/String/Float64).
3.  **Timeout Mandate:** Enforce `context.WithTimeout` on all Mangle interactions within the orchestrator to prevent silent deadlocks.
4.  **Concurrency Stress Testing:** Utilize Go's `-race` detector in a continuous integration environment that aggressively toggles pause, cancel, and heartbeat routines simultaneously.

This analysis highlights that while the "Happy Path" is well-defined, the "Hostile Path" requires significant hardening before production readiness.

### 4. Extended Permutation Analysis

#### 4.1 Permutations of Nil/Undefined Inputs
When `o.campaign` is initialized, what if specific nested slices are explicitly `nil` instead of empty slices?
- `o.campaign.Phases == nil`: Iterating over a nil slice in Go is safe (it acts like an empty slice), but appending to it or passing it to external functions that expect pre-allocated capacity might cause unexpected allocations.
- `o.campaign.Metadata == nil`: If the UI or API attempts to query the metadata map without checking for nil, a panic will ensue.
- `o.config.AutosaveEvery == 0`: We discussed the ticker panic. But what if it's `-1`? The time package might behave unpredictably.
- `o.virtualStore == nil`: The orchestrator relies on the virtual store for file access. If nil, any attempt to read/write will crash. The constructor must enforce this dependency.

#### 4.2 Permutations of Type Coercion across the Mangle Boundary
Mangle uses a unified representation for data. Go uses static typing.
- **Go Int -> Mangle Float -> Go Int:** If a value is passed through Mangle, does it retain its exact integer value, or is it subjected to floating-point rounding?
- **Go String -> Mangle Atom -> Go String:** If a Go string containing spaces is converted to a Mangle atom (e.g., `/hello world`), does Mangle's parser handle the spaces correctly, or does it split the atom? When retrieving it back to Go, does it retain the spaces?
- **JSON Serialization:** The `campaign` struct is likely serialized to JSON for persistence. Are there any fields (like unexported fields or function pointers) that fail serialization? What if a field contains a cyclic reference?

#### 4.3 Permutations of Extreme Scale
- **Number of Tasks per Phase:** What if a single phase has 10,000 tasks? The `results` channel is bounded, but the scheduling loop iterates over all tasks. The iteration itself might take measurable time.
- **Size of Task Context:** What if the context provided to a task is 1GB? The LLM client will likely reject it, but the orchestrator must handle this rejection gracefully without crashing or running out of memory while buffering the context.
- **Frequency of Checkpoints:** If checkpoints are too frequent (e.g., every millisecond), the system will spend all its time saving state and no time making progress. The configuration must enforce minimum intervals.

#### 4.4 Permutations of Race Conditions
- **Task Failure vs Campaign Cancel:** A task fails and triggers the error handling path. Simultaneously, the user cancels the campaign. The orchestrator must ensure the error handling doesn't override the cancellation status.
- **Mangle Transaction during Phase Transition:** Mangle is evaluating rules to determine the next phase while the orchestrator is asserting new facts about the current phase. The locking mechanism must ensure Mangle reads a consistent view of the world.

### 5. Architectural Reflections on Resilience

The codeNERD architecture makes a deliberate choice to externalize complex state transitions to the Mangle logical engine. This provides immense flexibility but introduces a strict boundary where imperative assumptions fail. Every interaction with Mangle must be treated as a network boundary, subject to latency, failures, and type mismatches.

Boundary validation must occur strictly at the orchestrator API boundary, not deep within the execution loop. Failures must be detected before they propagate to the Mangle kernel. Resource exhaustion (memory, FDs, goroutines) must be mitigated by implementing strict quotas and backpressure mechanisms within the `runPhase` scheduling loop. The state reconciliation loop must be idempotent, allowing the system to safely recover from partial failures during checkpointing or Mangle transaction commits.

Robust error handling is paramount. When `startNextPhase` fails, the orchestrator logs the warning but continues the loop, potentially creating a rapid spin if the failure is persistent. The loop must include a backoff mechanism or a retry limit to prevent CPU thrashing. Similarly, when context paging fails, the error is logged and an event is emitted, but the task execution continues without the necessary context. This should ideally be a blocking error that requires human intervention or an automatic replan to recover the lost context.

In conclusion, the orchestrator's resilience depends entirely on its ability to handle these edge cases gracefully. By implementing the recommended tests and validations, we can transform it from a fragile state machine into a robust engine capable of orchestrating complex campaigns with confidence.

### 6. Subsystem Intersections and Cascade Failures

The orchestrator does not operate in isolation. It relies on the `ContextPager`, `CheckpointRunner`, and `Decomposer`. The boundary value analysis must also consider how extreme values passed from these subsystems affect the orchestrator.

#### 6.1 The Decomposer Intersection
- **Degenerate Decomposition:** What if the `Decomposer` outputs a phase plan where tasks have cyclical dependencies? The orchestrator's task scheduler (likely within `runPhase` or Mangle logic) might deadlock trying to find eligible tasks.
- **Micro-Task Avalanche:** If the decomposer splits a simple goal into 5,000 tiny tasks (e.g., "change 1 variable per task"), the overhead of LLM invocation, checkpointing, and Mangle state updates will dominate the actual work. The orchestrator needs backpressure mechanisms against overly granular decomposition.

#### 6.2 The ContextPager Intersection
- **Context Truncation:** If a project has 500 massive source files, the `ContextPager` will inevitably truncate the context. If it truncates the *most critical* file for the current task, the LLM will fail or hallucinate. The orchestrator must handle tasks that fail specifically due to "missing context" differently than tasks that fail due to logic errors.
- **Stale Context:** In a long-running campaign (e.g., 5 days), the external repository might change. If the orchestrator relies on context paged on day 1 for a task executed on day 5, it will operate on stale data. Negative testing must simulate external file system mutations during campaign execution.

#### 6.3 The CheckpointRunner Intersection
- **Checkpoint Poisoning:** If a checkpoint script contains malicious code or an infinite loop, the orchestrator will stall when executing it. The orchestrator must execute checkpoints in a strictly sandboxed, timeout-bound environment.
- **Checkpoint Non-Determinism:** A checkpoint might pass on attempt 1, but fail on attempt 2 without any underlying changes. The orchestrator's retry logic must handle flaky verifications without throwing the campaign into a terminal replan loop.

### 7. Strategic Recommendations for Test Suite Expansion

To move from the `TestOrchestratorExecution_Placeholder` to a robust test suite, the following strategies should be employed:

1.  **Fuzz Testing:** Implement Go fuzz tests (`go test -fuzz`) for the `parseAnalysisResponse` and `extractJSON` functions, feeding them randomly mutated JSON strings to uncover panic vectors.
2.  **Mangle Mocking Engine:** Build a comprehensive mock for the `core.Kernel` that allows deterministic injection of facts, including corrupted facts (e.g., wrong arity, invalid types) to test the orchestrator's parsing resilience.
3.  **Property-Based Testing:** Use a framework like `gopter` to assert invariants. For example, "A campaign's status should only transition from Active to Paused, never directly to Completed if tasks are pending."
4.  **Chaos Engineering (Fault Injection):** Develop a test harness that randomly drops simulated database connections, cancels contexts midway through phase execution, and injects artificial delays into LLM responses to observe the orchestrator's recovery mechanisms.

### 8. Final Verdict

The current lack of negative tests in `internal/campaign/orchestrator_execution_test.go` represents a high-risk technical debt. The orchestrator is structurally complex but currently brittle when exposed to boundary conditions. Prioritizing the implementation of the `TODO` items identified in this analysis is critical for ensuring the reliability and safety of the codeNERD platform.

### 9. Component State Transitions Deep Dive

The orchestrator operates as a Finite State Machine (FSM). Negative testing must cover invalid transitions.

#### 9.1 Invalid State Transitions
- **Paused -> Completed:** If a campaign is paused, and an external event (like a manual database update) sets its status to Completed, what happens when it resumes? The orchestrator must validate the current state against expected initial states.
- **Failed -> Active:** A failed campaign must not be arbitrarily restartable without a formal "replan" or "repair" phase. If it is forced to Active, internal checkpoint failures might still block progress.
- **Active -> Active (Idempotency):** Calling `Run()` on an already running campaign should return an error. The code checks `o.isRunning`, but a race condition could exist if two goroutines call `Run()` simultaneously before `o.isRunning` is set to true.

#### 9.2 The "Thundering Herd" on Resume
- **Scenario:** A campaign is paused. 1,000 pending tasks are queued. The campaign is resumed.
- **Impact:** The `pauseCh` is closed, waking up the scheduling loop. If the loop attempts to dispatch all 1,000 tasks instantly, it might overwhelm the `results` channel or the LLM client, despite the `maxParallelTasks` bound, because the bound only applies to *running* tasks, but the loop might rapidly cycle through "eligible" checks.

#### 9.3 Checkpoint State Corruption
- **Scenario:** `runPhaseCheckpoint` is called, but the orchestrator crashes midway through processing the results.
- **Impact:** The phase might be left in a "checkpointing" state indefinitely, or the failure counter might be incremented without a corresponding failure record in the journal. The orchestrator must use transactional updates for critical state changes.

### 10. Memory and Resource Leak Analysis

A critical component of Negative Testing is assessing behavior under sustained failure conditions that might leak resources.

#### 10.1 Goroutine Leaks
- **Scenario:** The `runHeartbeatLoop` is started as a goroutine. If the parent `ctx` is not properly cancelled, this goroutine runs forever.
- **Test Gap:** We need a test that explicitly starts the orchestrator, immediately cancels the context, and uses `goleak` to verify that `runHeartbeatLoop` terminates promptly.
- **Scenario:** A task is dispatched to the `taskExecutor`, which spawns a goroutine. If the task hangs infinitely and doesn't respect context cancellation, it leaks a goroutine.

#### 10.2 File Descriptor Leaks
- **Scenario:** The orchestrator frequently reads configurations or checkpoints from the disk. If a read fails midway, are the file handles properly closed via `defer`?
- **Test Gap:** Mock a failing filesystem (e.g., using a customized `VirtualStore`) and observe the number of open file descriptors to ensure no leaks occur during error paths.

#### 10.3 Channel Deadlocks
- **Scenario:** The `results` channel in `runPhase` is full. A task attempts to send a result but blocks. Meanwhile, the main `runPhase` loop is blocked waiting on something else (e.g., a Mangle query).
- **Test Gap:** Force the `results` channel to be saturated by using an artificially small buffer size (`maxParallelTasks = 1`) and a burst of instantly completing mock tasks to verify the loop doesn't deadlock.

### 11. Testing the "Recovery from Failure" Logic

The orchestrator includes complex logic for recovering from failures (e.g., replanning). We must test the boundaries of this recovery.

#### 11.1 Replan Infinite Loops
- **Scenario:** A task fails. The orchestrator triggers a replan. The replan immediately creates a new task that also fails in the same way.
- **Test Gap:** Verify that the system limits the number of consecutive replans or introduces exponential backoff to prevent an infinite loop of failing replans consuming massive API quota.

#### 11.2 Checkpoint Rollback Failure
- **Scenario:** A checkpoint fails, and the system attempts to revert state. The rollback itself fails.
- **Test Gap:** Test the system's behavior when it cannot cleanly revert. Does it halt the campaign and mark it as `StatusFailed`, or does it attempt to proceed with corrupted state?

### 12. Security Boundary Testing

While not strictly security software, the orchestrator executes tasks that might originate from external sources (e.g., via the MCP Bridge).

#### 12.1 Path Traversal in File Operations
- **Scenario:** A task specifies an artifact path containing `../../malicious_file`.
- **Test Gap:** Verify that the orchestrator's file validation prevents writing or reading outside the designated campaign workspace, even when dealing with symlinks or deeply nested traversal sequences.

#### 12.2 Arbitrary Code Execution (ACE) via Checkpoints
- **Scenario:** The decomposer hallucinates a checkpoint command that executes `rm -rf /`.
- **Test Gap:** Verify that the `Executor` interface correctly sandboxes checkpoint execution, preventing destructive operations on the host system.

### 13. Deep Analysis of Phase Scheduling and Execution Integrity

The `runPhase` method is the core execution loop for tasks. Its behavior under adverse conditions dictates the reliability of the entire orchestrator.

#### 13.1 Phantom Task Dispatch
- **Scenario:** A task is marked as `TaskPending`, dispatched to the `taskExecutor`, but before the executor can start it, an external replan mutates the campaign state, marking the task as `TaskCancelled`.
- **Impact:** The executor might still run the cancelled task, consuming resources and potentially causing side effects (e.g., file modification) that contradict the new plan.
- **Test Gap:** Verify that `taskExecutor` checks the current, authoritative task status immediately before initiating execution, or that the orchestrator provides a mechanism to definitively abort dispatched but unstarted tasks.

#### 13.2 Result Channel Starvation
- **Scenario:** Multiple tasks finish simultaneously and attempt to write to the `results` channel. However, the `runPhase` loop is blocked on a slow `kernel.Query` or a lengthy context prefetch operation.
- **Impact:** Task execution goroutines are blocked waiting to send their results. This ties up system resources and artificially inflates perceived task duration.
- **Test Gap:** Introduce artificial delays in the main loop's processing (e.g., during context prefetching) and verify that task executors do not remain blocked indefinitely, perhaps by decoupling result reception into a dedicated, unbuffered goroutine.

#### 13.3 The Mutating Phase Pointer Hazard
- **Scenario:** The loop in `runPhase` re-binds `phase` by calling `o.livePhaseByID(phase.ID)`. If the entire phases array is reallocated (e.g., during a replan), `livePhaseByID` ensures we have the correct pointer. However, what if a task modifies a *different* phase concurrently?
- **Impact:** If the orchestrator's internal locks are not correctly scoped, reading `phase.Tasks` in one goroutine while a replan mutates `o.campaign.Phases` in another can cause slice bounds out of range panics or read corrupt data.
- **Test Gap:** Aggressive concurrent testing (`go test -race`) while simulating continuous replanning events during active task execution.

#### 13.4 Checkpoint Failure Amplification
- **Scenario:** A phase contains 5 tasks. Task 1 completes and triggers a checkpoint (if rolling checkpoints are enabled). The checkpoint fails. The system attempts to roll back. Tasks 2-5 are still running.
- **Impact:** The orchestrator must signal Tasks 2-5 to abort immediately. If it doesn't, their subsequent completion might attempt to write to the `results` channel of a phase that has already been deemed failed and rolled back, leading to complex state entanglement.
- **Test Gap:** Verify that a checkpoint failure cascades a cancellation signal to all sibling tasks within the same phase before the phase is formally marked as failed or retried.

### 14. Observability and Auditing Resilience

The orchestrator emits events and updates logs. This observability layer must be resilient to failure.

#### 14.1 Event Emission Panics
- **Scenario:** The `emitEvent` function is called with a nil payload or an invalid string pointer.
- **Impact:** If `emitEvent` panics, it will crash the entire orchestrator loop, bringing down the campaign.
- **Test Gap:** Verify `emitEvent` and related logging functions safely handle `nil` interfaces, excessively large payloads, and non-serializable objects without panicking.

#### 14.2 Audit Trail Integrity During Hard Crashes
- **Scenario:** The host system experiences a hard crash (e.g., power loss, SIGKILL) while the orchestrator is in the middle of `runPhase`.
- **Impact:** The in-memory state is lost. When the system restarts, it must reconstruct the state from Mangle facts and the durable campaign JSON. If the autosave frequency is too low, significant task progress might be lost, leading to duplicated work upon restart.
- **Test Gap:** Simulate hard crashes by abruptly terminating the process and verifying that the orchestrator can resume the campaign from the last valid checkpoint without data corruption or logical inconsistencies.

### 15. The Impact of Mangle's Declarative Fixpoint on Imperative Flow

CodeNERD's orchestrator is essentially an imperative control loop driven by a declarative fixpoint engine (Mangle). This paradigm mismatch creates unique boundary conditions.

#### 15.1 Fixpoint Oscillation
- **Scenario:** Mangle evaluates a set of rules. Due to a bug in the ruleset, the system reaches a fixpoint, but the facts derived assert `A`, which triggers a Go callback that modifies state, causing Mangle to re-evaluate and assert `B`, which triggers a Go callback that reverts the state, causing Mangle to assert `A` again.
- **Impact:** The system enters an endless cycle of evaluations without ever completing a task.
- **Test Gap:** Create an adversarial Mangle ruleset that intentionally oscillates and verify that the orchestrator's `runPhase` loop contains cycle-detection or maximum-iteration thresholds to halt the execution and flag the campaign as stalled.

#### 15.2 The "Ghost Fact" Contamination
- **Scenario:** An orchestrator run completes. The `core.Kernel` is reused for a subsequent campaign without a proper clean slate reset.
- **Impact:** Facts derived from Campaign 1 (e.g., `phase_completed("C1_P1")`) pollute the fixpoint evaluation of Campaign 2. If Campaign 2 happens to generate a phase ID that collides with Campaign 1 (e.g., a simple hash collision or poorly scoped ID generation), Campaign 2 will instantly skip phases it believes are already complete.
- **Test Gap:** Execute two sequential campaigns using the same orchestrator instance and artificially force an ID collision to ensure state isolation between runs.

#### 15.3 Stratification and Negation Traps
- **Scenario:** A user-defined tool or a dynamic workflow introduces a rule that violates Datalog stratification (e.g., a rule depending on the negation of itself: `task_ready(T) :- not task_ready(T)`).
- **Impact:** The Mangle engine will fail to evaluate. The Go error must be gracefully caught.
- **Test Gap:** Inject unstratified rules into the `virtualStore` or dynamic ruleset and verify the orchestrator intercepts the Mangle compilation/evaluation error, marking the specific task or phase as failed rather than crashing the entire process.

### 16. The Reality of Eventual Consistency in Autosaves

The `autosaveTicker` provides a crucial durability mechanism, but it introduces eventual consistency into a system that otherwise demands strong consistency for task dispatch.

#### 16.1 The "Stale Save" Dilemma
- **Scenario:** The `autosaveTicker` fires. It calls `o.saveCampaign()`. This function presumably serializes `o.campaign` to JSON and writes it to disk. However, serialization takes time. While `saveCampaign` is executing (reading `o.campaign`), a concurrent `runPhase` loop modifies a task status within `o.campaign.Phases`.
- **Impact:** If `saveCampaign` does not hold a deep read lock over the entire `o.campaign` structure during serialization, the resulting JSON might contain a torn state—a mix of pre-mutation and post-mutation data. For example, a task might be marked "Complete", but the phase it belongs to might still have `CompletedTasks: 0`.
- **Test Gap:** Concurrently bombard the orchestrator with rapid task completion events while forcing aggressive, high-frequency autosaves (e.g., every 10ms). Then, halt the system and run a consistency checker against the final generated JSON to detect torn states.

#### 16.2 Disk Full / IO Error Handling During Autosave
- **Scenario:** The disk holding the `.nerd` workspace runs out of space. The `autosaveTicker` attempts to write the JSON and fails.
- **Impact:** The error is likely logged, but does the orchestrator continue executing tasks? If it does, all subsequent progress is volatile and will be lost on the next restart.
- **Test Gap:** Mock the file system to inject a `syscall.ENOSPC` (No space left on device) error during `saveCampaign`. Verify that the orchestrator pauses execution, alerts the user, and refuses to dispatch new tasks until durability can be guaranteed again.

### 17. The Final Synthesis: Achieving True Resilience

To achieve robust operation, the codeNERD orchestrator must embrace defensive programming at every layer. The gaps identified here are not merely theoretical; they represent the exact failure modes encountered in distributed systems and complex state machines.

The path forward requires:
1.  **Strict Boundary Enforcement:** Every external input (Mangle queries, filesystem state, user cancellation) must be validated and sanitized.
2.  **Idempotent Operations:** All state transitions and save operations must be safe to retry without causing corruption.
3.  **Comprehensive Negative Test Suite:** The `TODO` gaps identified must be translated into concrete, automated tests that run on every commit.

### 18. Additional Edge Case Expansions
- **Scenario:** The orchestrator encounters a malformed `PhaseID` that is extremely long or contains special characters.
- **Impact:** This could break filesystem paths when creating checkpoints.
- **Test Gap:** Test `PhaseID`s containing `../`, null bytes, and lengths > 4096 characters.

- **Scenario:** The `taskResult` channel receives a structurally invalid message due to an interface conversion panic in the executor.
- **Impact:** The orchestrator loop crashes.
- **Test Gap:** Validate that all channels pass strictly typed and validated structs, preventing runtime type assertions from panicking the main loop.

- **Scenario:** An external API (LLM) takes exactly the same time as the `CampaignTimeout`.
- **Impact:** The context cancellation and the LLM response race.
- **Test Gap:** Use deterministic timers in the mock LLM client to ensure the context cancellation always wins and the orchestrator handles the cancellation cleanly.

### 19. Final Metrics
- Total Subsystems Analyzed: 4 (Orchestrator, ContextPager, Decomposer, CheckpointRunner)
- Total Boundary Vectors Identified: 34
- Core Execution Path Confidence: Low (High Risk)
- Action Items: Implement robust fuzzing, mock-driven negative tests, and chaos engineering practices as outlined above.

This detailed, structured analysis forms the foundation for hardening the core execution engine of the codeNERD architecture.

### 20. Expanded Detail on File Descriptor Management in Tasks
The executor operates within the constraints of the host system. When running tests or processing files across a vast monorepo, file descriptors (FDs) are a highly constrained resource. A single task might attempt to parse hundreds of AST files. If the Go GC doesn't clean up unreferenced file structs fast enough, the system will hit the FD limit (e.g., 1024 on many Linux distros) and crash. The test suite MUST mock `ulimit -n` and assert that the orchestrator degrades gracefully, pausing operations and reporting the resource constraint rather than allowing a cascading failure that corrupts the Mangle state.

### 21. Database Lock Contention (SQLite)
The `MCPToolStore` and `core.Kernel` rely heavily on SQLite under the hood. SQLite is notorious for `database is locked` (`SQLITE_BUSY`) errors under high concurrency.
- **Scenario:** The orchestrator dispatches 10 parallel tasks. Each task simultaneously completes and attempts to write to the `campaign_heartbeat` or retrieve a capability from the `MCPToolStore`.
- **Impact:** One or more SQLite transactions will fail with `database is locked`. The orchestrator must implement an exponential backoff retry mechanism (like `database/sql`'s connection pool retries) rather than immediately failing the task or crashing the campaign.
- **Test Gap:** Introduce a mock driver that randomly returns `SQLITE_BUSY` 20% of the time and verify the orchestrator successfully retries and completes all tasks without data loss.

### 22. Network Partition and Dependency Isolation
- **Scenario:** The orchestrator relies on an external LLM provider. The network connection drops completely for 5 minutes during a critical decomposition or task execution phase.
- **Impact:** Standard HTTP clients will eventually time out, but if the orchestrator doesn't correctly implement a circuit breaker, it will repeatedly hammer the failing connection, wasting CPU and potentially corrupting the state if it assumes a timeout implies a task failure rather than a system failure.
- **Test Gap:** Integrate a toxiproxy or similar fault injection tool to blackhole all outbound network traffic mid-campaign and verify the orchestrator gracefully pauses and waits for connectivity to be restored.

### 23. Configuration Hot-Reloading Vulnerabilities
- **Scenario:** The user modifies `.nerd/config.yaml` while the orchestrator is actively executing `runPhase`. The orchestrator attempts to hot-reload the configuration.
- **Impact:** If the hot-reload mutates the `o.config` struct without acquiring the appropriate `o.mu.Lock()`, a concurrent read by the task scheduler could cause a race condition, leading to the use of partially updated configurations (e.g., reading a new `CampaignTimeout` but an old `maxParallelTasks` value).
- **Test Gap:** Construct a test that spawns a goroutine to continuously mutate the orchestrator's configuration using the reload hook, while another goroutine validates that running tasks observe consistent state boundaries without panicking.

### 24. Context Propagation Depth Limits
- **Scenario:** A campaign decomposes into an arbitrary depth of nested sub-phases (e.g., 50 levels deep). The `ContextPager` attempts to aggregate context from all parent phases.
- **Impact:** The sheer volume of aggregated context exceeds the maximum token limit of the LLM context window. The API call fails with a `400 Bad Request` regarding token counts.
- **Test Gap:** Mock a deeply nested phase structure and assert that the `ContextPager` implements an intelligent decay or summarization strategy that guarantees the final assembled prompt never exceeds a configured maximum token threshold.

### 25. Checkpoint Time-To-Live (TTL) Edge Cases
- **Scenario:** A checkpoint is scheduled to run but the system is under extreme I/O load. The checkpoint script takes 1 hour to execute.
- **Impact:** The orchestrator loop blockingly waits on this checkpoint, stalling all other progress in the campaign.
- **Test Gap:** Assert that the checkpoint runner strictly enforces execution timeouts, killing runaway checkpoint scripts and failing the phase appropriately.

### 26. Terminal System Conclusion
The negative test coverage gaps identified above demonstrate that while the orchestrator works well under optimal conditions, the absence of boundary and error-path assertions leaves it vulnerable to race conditions, type mismatches, and resource exhaustion. Resolving these issues via automated tests will solidify the core foundation of codeNERD.
