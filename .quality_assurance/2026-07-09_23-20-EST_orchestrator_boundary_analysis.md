# QA Automation Journal: Boundary Value Analysis & Negative Testing
## Subsystem: Campaign Orchestrator (Failure Handling)
**Date/Time:** 2026-07-09 23:20 EST
**Target Files:** `internal/campaign/orchestrator_failure.go`, `internal/campaign/orchestrator_failure_test.go`
**Engineer:** QA Automation Engineer (Jules)

### Executive Summary

This journal entry details a comprehensive Boundary Value Analysis (BVA) and Negative Testing review of the `codeNERD` Campaign Orchestrator's failure handling and retry escalation mechanisms (`internal/campaign/orchestrator_failure.go`). The Campaign Orchestrator is responsible for managing multi-phase execution goals and robustly dealing with transient or logical failures in sub-agents across execution shards. Given the stateful and concurrent nature of `codeNERD`, proper failure handling and isolation is critical to avoid system panics, Mangle fact-store contamination, or unbounded retry loops.

The current test suite (`orchestrator_failure_test.go`) covers some happy-path escalation cases (e.g., verifying `shouldEscalateLogicFailure` works when passed 3 logic failures) but fundamentally lacks rigorous testing against adversarial states, nil references, data type mismatches injected by Mangle, and concurrent execution race conditions.

This review identifies edge cases across four main vectors:
1.  **Null/Undefined/Empty**
2.  **Type Coercion**
3.  **User Request Extremes**
4.  **State Conflicts**

---

### Vector 1: Null/Undefined/Empty

**Overview:** Functions in the orchestrator take complex, deeply nested structs (e.g., `*Phase`, `*Task`, `[]TaskAttempt`) or depend on state queried from the Mangle kernel, which may return nil or empty arrays. The failure handling logic must defensively handle missing or corrupted data structures to prevent `panic: runtime error: invalid memory address or nil pointer dereference`.

**Identified Gaps & Missing Tests:**

1.  **`classifyTaskError(err error)` with completely empty or whitespace-only error strings.**
    *   **Context:** `classifyTaskError` receives an error object from sub-agent execution and tries to categorize it.
    *   **Missing Test:** What happens if `err` is not nil, but `err.Error()` returns `""`, `"   "`, or a string composed only of control characters (e.g., just newline `\n` or tab `\t`)? The function falls back to `/logic`, but is this strictly verified to avoid indexing or string slicing panics?
    *   **Test Needed:** Assert that `classifyTaskError(errors.New("  \t\n "))` returns `/logic` without panic or array out-of-bounds if manipulating the string. We must ensure the `strings.TrimSpace` correctly maps it to the empty string condition.

2.  **`shouldEscalateLogicFailure(attempts []TaskAttempt, now time.Time)` with an empty attempts slice.**
    *   **Context:** Used to decide if an execution loop is stuck.
    *   **Missing Test:** While the function has a check `if len(attempts) == 0`, the test suite does not explicitly verify the return behavior.
    *   **Test Needed:** Assert `shouldEscalateLogicFailure([]TaskAttempt{}, time.Now())` returns `(false, "")`.

3.  **`shouldEscalateLogicFailure` with zero-value `Timestamp` in `TaskAttempt`s.**
    *   **Context:** The function calculates the loop age by finding the oldest loop failure (`now.Sub(oldestLoopFailure)`).
    *   **Missing Test:** What if a `TaskAttempt` object has a zero-value `time.Time{}` timestamp (e.g., failed to initialize or DB serialization error)? The loop calculating `oldestLoopFailure` might accidentally pick the zero-value, causing `now.Sub(oldestLoopFailure)` to be ~50+ years, instantly triggering an escalation incorrectly.
    *   **Test Needed:** Inject `TaskAttempt`s with `Timestamp: time.Time{}` and verify the logic skips them or gracefully handles the calculation without immediately escalating based on epoch zero.

4.  **`insertReproDiagnosticTaskLocked` with empty or nil slices.**
    *   **Context:** Inserts a diagnostic task into `phase.Tasks`.
    *   **Missing Test:** What if `phase.Tasks` is currently nil or empty (e.g., corrupted state)? Does `append([]Task{reproTask}, phase.Tasks...)` behave correctly? Does the loop adjusting indices panic?
    *   **Test Needed:** Verify `insertReproDiagnosticTaskLocked` on a `Phase` with `Tasks: nil` safely initializes the slice and returns valid IDs without out-of-bounds panics on subsequent index operations.

5.  **`findActiveReproTaskID` with nil tasks slice.**
    *   **Context:** Scans a slice of tasks.
    *   **Missing Test:** Does it return `""` safely when given `nil`?
    *   **Test Needed:** Verify `findActiveReproTaskID(nil, "some-id") == ""`.

6.  **`handleTaskFailure` with nil Phase or nil Task parameters.**
    *   **Context:** Top level entry point for handling task failures.
    *   **Missing Test:** What if `Phase` or `Task` is nil when invoked from a broken event handler?
    *   **Test Needed:** Verify `handleTaskFailure(ctx, nil, task, err)` and `handleTaskFailure(ctx, phase, nil, err)` exit early without nil-pointer dereferences.

7.  **`classifyTaskError(err error)` when `err` itself is completely nil.**
    *   **Context:** While supposedly guarded, what if a transient failure component passes a typed nil?
    *   **Missing Test:** Check that `err == nil` is explicitly caught and defaults to a safe category (e.g., `/logic` or `/unknown`) rather than crashing on `.Error()`.
    *   **Test Needed:** Verify `classifyTaskError(nil)` returns a stable string and does not crash.

8.  **Empty `errorType` returned from classification.**
    *   **Context:** If `classifyTaskError` somehow returns an empty string `""` due to future refactoring.
    *   **Missing Test:** The kernel assert requires specific structures. Will an empty `errorType` cause the Mangle fact store to reject the `task_error` insertion?
    *   **Test Needed:** Force an empty error type and verify it doesn't break the Mangle assert chain.

9.  **Empty `reason` string returned from `shouldEscalateLogicFailure`.**
    *   **Context:** If `shouldEscalateLogicFailure` returns `(true, "")`.
    *   **Missing Test:** Will the logging or kernel insertion fail if the reason is unexpectedly blank?
    *   **Test Needed:** Ensure the system tolerates a blank reason string without crashing the escalation event emission.

10. **Nil `context.Context` passed to `handleTaskFailure`.**
    *   **Context:** Replan routines might pass `nil` for context if backgrounded incorrectly.
    *   **Missing Test:** Check how `handleTaskFailure` responds to `context.Background()` vs a pure `nil` context.
    *   **Test Needed:** Pass `nil` context and ensure it doesn't panic when trying to execute checkpointing.

---

### Vector 2: Type Coercion

**Overview:** The Orchestrator interacts heavily with the Mangle engine via `VirtualStore`. Mangle is loosely typed at the boundary (e.g., facts are passed as string parameters and coerced to atoms or integers internally). Go tests often pass raw strings where Mangle Atoms are expected, causing silent failures (zero results) that are misinterpreted as "empty state" rather than "type mismatch."

**Identified Gaps & Missing Tests:**

1.  **Mangle Fact Type Dissonance in `task_error` assertions.**
    *   **Context:** `handleTaskFailure` asserts facts like `kernel.Assert(core.Fact{Predicate: "task_error", Args: []any{task.ID, errorType, errStr}})`.
    *   **Missing Test:** Mangle expects atoms for predefined constants (like `/logic`). `classifyTaskError` returns Go strings containing forward slashes (e.g., `"/logic"`). Are these strings properly coerced into `ast.Name` inside `kernel.Assert`? If they are treated as Mangle Strings instead of Atoms, downstream rules like `replan_needed :- task_error(T, /logic, _)` will silently fail to join.
    *   **Test Needed:** Construct an integration test that asserts a failure, then strictly queries the kernel for `task_error` and verifies the second argument is of type `ast.Name` and not a raw string.

2.  **`task_retry_at` Timestamp Coercion (int64 vs int vs float64).**
    *   **Context:** `kernel.Assert(core.Fact{Predicate: "task_retry_at", Args: []any{task.ID, nextRetryAt.Unix()}})`
    *   **Missing Test:** `nextRetryAt.Unix()` returns an `int64`. Some Mangle internal functions or json/SQL persistence layers might cast this to `float64` or `int32`.
    *   **Test Needed:** Verify that querying `task_retry_at` from the kernel returns exactly an `int64` and that rule engines evaluating `time_now() > RetryTime` handle the `int64` value correctly without panicking due to mismatched types.

3.  **`TaskAttempt` array structure coercion from DB.**
    *   **Context:** When loading `TaskAttempt` from SQLite, the JSON array might coerce values differently.
    *   **Missing Test:** If the `Outcome` is accidentally stored as a boolean or integer in a corrupted DB, does the Go json unmarshaler fail, leaving the array nil?
    *   **Test Needed:** Test `shouldEscalateLogicFailure` with malformed DB-loaded JSON to ensure it handles structural typing issues.

4.  **`maxRetries` int coercion from Campaign Config.**
    *   **Context:** The Orchestrator fetches `maxRetries` from its configuration or kernel.
    *   **Missing Test:** If `maxRetries` is coerced to a negative number or zero by a configuration parsing error, how does the failure loop behave?
    *   **Test Needed:** Set `maxRetries` to 0 or -1 and verify the orchestrator immediately fails the task without entering an infinite retry loop.

5.  **`time.Duration` coercion for Backoff values.**
    *   **Context:** The backoff computation uses `time.Duration`.
    *   **Missing Test:** If configuration values for `RetryBackoffBase` are read as integers instead of nanoseconds.
    *   **Test Needed:** Validate that `computeRetryBackoff` handles incorrectly scaled durations gracefully.

6.  **`Task.Priority` string vs Enum coercion.**
    *   **Context:** `PriorityCritical` is assigned to repro tasks.
    *   **Missing Test:** If the enum is cast to a raw string or an unknown value during serialization, does it affect the queue?
    *   **Test Needed:** Inject a task with an invalid priority string and see if the failure handler normalizes it.

7.  **`Task.Type` string vs Enum coercion.**
    *   **Context:** Checking `isMutatingTaskType(task.Type)`.
    *   **Missing Test:** If `Type` is somehow set to a random string instead of `TaskTypeFileModify`, does the logic escalation bypass properly?
    *   **Test Needed:** Verify `isMutatingTaskType` defaults securely when encountering unknown string types.

8.  **Kernel Fact arguments order coercion.**
    *   **Context:** `kernel.Assert` takes an `[]any`.
    *   **Missing Test:** If arguments are swapped accidentally (e.g. `errorType` and `errStr`), Mangle will not complain until a rule fails.
    *   **Test Needed:** Add type assertions on the test mock to ensure the `task_error` fact has exactly `[String, Atom, String]` structure.

9.  **Escalation boolean flag coercion.**
    *   **Context:** `shouldEscalateLogicFailure` returns a boolean.
    *   **Missing Test:** In some rule systems, boolean true is `1` or `"/true"`.
    *   **Test Needed:** Verify that any rules depending on the escalation flag interpret the Go boolean correctly.

10. **Phase/Task ID type safety.**
    *   **Context:** IDs are strings but often formatted like `/phase_1`.
    *   **Missing Test:** Are they accidentally treated as Mangle Atoms if they start with a slash?
    *   **Test Needed:** Ensure Task and Phase IDs are consistently treated as strings in the kernel, not atoms, to avoid lookup failures.

---

### Vector 3: User Request Extremes

**Overview:** `codeNERD` must survive adversarial, edge-case, or absurdly massive inputs generated by users or highly creative/hallucinating LLMs. This involves string lengths, array sizes, and deep recursion.

**Identified Gaps & Missing Tests:**

1.  **Massive Error Strings in `classifyTaskError` and Kernel Assertions.**
    *   **Context:** A build failure might output a 50MB compilation error log. This error is passed to `classifyTaskError` and subsequently asserted to the Mangle kernel via `errStr`.
    *   **Missing Test:** Does `strings.ToLower(strings.TrimSpace(err.Error()))` inside `classifyTaskError` cause a massive memory allocation spike and OOM the container? Does `kernel.Assert` crash when trying to store a 50MB string as a fact argument?
    *   **Test Needed:** Pass an `error` object returning a 50MB string to `handleTaskFailure`. Verify that it either cleanly truncates the error string before processing/storing, or processes it within strict memory limits without crashing.

2.  **Unbounded Retries and Integer Overflow in `computeRetryBackoff`.**
    *   **Context:** `shift := min(max(attemptNum-1, 0), 10)` prevents extreme exponential backoffs, but what if `attemptNum` itself is enormous due to a bug or malicious mutation?
    *   **Missing Test:** What happens if `attemptNum` is `math.MaxInt32`? The math is clamped, but does passing an extreme value cause unexpected behavior anywhere else in the failure pipeline?
    *   **Test Needed:** Pass `attemptNum = math.MaxInt` into the failure handler and verify exponential backoff clamps safely to `maxBackoff`.

3.  **Repro Task Cascade (Infinite Insertion Loop).**
    *   **Context:** `insertReproDiagnosticTaskLocked` adds a new diagnostic task when a logic failure escalates.
    *   **Missing Test:** What if the repro task itself fails repeatedly? Will it spawn a repro task for the repro task, leading to an infinite chain of `task_mutate_1/repro_002/repro_001/...` until the array consumes all memory?
    *   **Test Needed:** Write a test where a task marked as `isReproDiagnosticTask(task) == true` repeatedly fails. Verify that `shouldEscalateLogicFailure` or `insertReproDiagnosticTaskLocked` prevents cascading insertion.

4.  **Extreme Number of Task Attempts (Performance Degradation).**
    *   **Context:** `shouldEscalateLogicFailure` iterates over the `attempts` slice to find the oldest failure loop.
    *   **Missing Test:** If a task somehow has 1,000,000 recorded attempts, does this iteration cause a noticeable CPU spike, blocking the main orchestrator loop?
    *   **Test Needed:** Pass an array of 1,000,000 `TaskAttempt`s to `shouldEscalateLogicFailure` and enforce a benchmark execution time (e.g., < 10ms) to ensure algorithmic efficiency.

5.  **Extremely Long Task Descriptions.**
    *   **Context:** When constructing the repro task description, the failed task ID and reason are appended.
    *   **Missing Test:** If the original reason string is 10MB long, will the new description exceed database column limits?
    *   **Test Needed:** Verify that `insertReproDiagnosticTaskLocked` truncates the generated `Description` string to a safe maximum length (e.g., 1024 chars).

6.  **Massive Number of Phases.**
    *   **Context:** `handleTaskFailure` searches for the task by iterating over `o.campaign.Phases`.
    *   **Missing Test:** If a hallucinating agent created 100,000 phases, does the nested loop `for i, p := range o.campaign.Phases { for j, t := range p.Tasks }` become a performance bottleneck?
    *   **Test Needed:** Benchmark `handleTaskFailure` with 10,000 phases and 10,000 tasks each to ensure the search is reasonably fast (or consider map-based lookups).

7.  **Negative Backoff Configuration.**
    *   **Context:** `computeRetryBackoff` uses configuration values.
    *   **Missing Test:** What if `o.config.RetryBackoffBase` is set to `-5 * time.Second`?
    *   **Test Needed:** Verify `computeRetryBackoff` falls back to defaults or clamps to 0 if given negative duration configurations.

8.  **Extreme Timestamp Future/Past.**
    *   **Context:** `shouldEscalateLogicFailure` checks loop age.
    *   **Missing Test:** What if the timestamps are from the year 1970 or the year 9999 due to a system clock error?
    *   **Test Needed:** Test the loop age logic with extreme timestamps to ensure it doesn't overflow or panic during duration calculations.

9.  **Excessive Dependency Chains.**
    *   **Context:** `ensureTaskDependsOn` adds to the `DependsOn` array.
    *   **Missing Test:** What if a task already has 10,000 dependencies? Adding one more requires scanning the entire slice `slices.Contains(task.DependsOn, depID)`.
    *   **Test Needed:** Ensure the dependency addition is efficient or capped at a reasonable limit to prevent OOM/CPU spikes.

10. **Huge Checkpoint Payloads on Failure.**
    *   **Context:** `o.config.CheckpointOnFail` triggers a full phase checkpoint.
    *   **Missing Test:** If the phase has 500MB of context, will checkpointing on every failure crash the system?
    *   **Test Needed:** Simulate a massive phase and trigger multiple rapid failures to verify checkpointing doesn't exhaust disk IO or memory.

---

### Vector 4: State Conflicts

**Overview:** The orchestrator operates in a highly concurrent environment. External shards, TUI callbacks, and timers can trigger asynchronous state changes while the main campaign loop is attempting to recover from a failure. Lock contention and race conditions are prime candidates for critical failures.

**Identified Gaps & Missing Tests:**

1.  **Race Condition during Phase/Task Mutation.**
    *   **Context:** `handleTaskFailure` locks `o.mu`, searches for the task, modifies it, and then unlocks `o.mu` before updating the kernel.
    *   **Missing Test:** What if, while `handleTaskFailure` is running and modifying `o.campaign.Phases[i]`, another goroutine (e.g., a manual override from the TUI or a timeout watchdog) aborts the campaign and nils out `o.campaign` or removes the phase?
    *   **Test Needed:** Trigger `handleTaskFailure` while concurrently firing a `AbortCampaign` event. Verify that `handleTaskFailure` gracefully exits or handles the missing pointers without a nil pointer dereference panic.

2.  **Kernel State vs In-Memory State Desynchronization.**
    *   **Context:** `handleTaskFailure` modifies the Go memory (`o.campaign.Phases[i].Tasks[j].Status = TaskPending`) and then separately asserts facts to the kernel (`o.updateTaskStatus(task, newStatus)`).
    *   **Missing Test:** If `o.updateTaskStatus` (or another kernel assertion) fails (e.g., due to a temporary DB lock in a persistent store), the Go struct is left in `TaskPending` while the Kernel still thinks the task is `TaskInProgress`.
    *   **Test Needed:** Mock the kernel to throw an error on `Assert`. Verify that the orchestrator detects the failure and rolls back the in-memory state or marks the campaign as critically faulty, rather than continuing with desynchronized split-brain states.

3.  **TOC/TOU (Time of Check / Time of Use) in Repro Task Dependency Assertion.**
    *   **Context:** `ensureTaskDependsOn` updates the Go struct dependency list, and then `insertReproDiagnosticTaskLocked` asserts `task_dependency` to the kernel.
    *   **Missing Test:** Another process querying the kernel between the struct update and the kernel assert will see inconsistent dependency graphs. While perhaps not fatal, it can lead to scheduling race conditions.
    *   **Test Needed:** Verify that dependency insertion and kernel assertion happen within the same transactional block or that intermediate reads are protected.

4.  **Concurrent Failure Handling for the Same Task.**
    *   **Context:** A SubAgent might timeout, triggering a failure, while at the exact same millisecond, the SubAgent returns a logical error. Both events enter `handleTaskFailure` for the same task.
    *   **Missing Test:** Do both calls insert a Repro task? `findActiveReproTaskID` might check and find nothing for both before they both hit `insertReproDiagnosticTaskLocked`.
    *   **Test Needed:** Fire two concurrent `handleTaskFailure` calls for the exact same task. Verify that only *one* repro task is inserted, and attempt counts are consistent.

5.  **Campaign Persistence Lock Contention.**
    *   **Context:** `handleTaskFailure` calls `o.saveCampaign()` while holding `o.mu`.
    *   **Missing Test:** If `saveCampaign` performs a slow disk IO (e.g., 5 seconds), it blocks the entire orchestrator `o.mu` mutex.
    *   **Test Needed:** Verify that `saveCampaign` is fast enough, or that other concurrent reads don't starve while a failure is being saved.

6.  **Replan Trigger Race Condition.**
    *   **Context:** `facts, _ := o.kernel.Query("replan_needed")` is called without holding `o.mu`.
    *   **Missing Test:** Another goroutine could have already triggered a replan, meaning this failure handler might trigger a duplicate replan.
    *   **Test Needed:** Fire multiple failures that trip the replan threshold concurrently and verify `o.replanner.Replan` isn't called redundantly.

7.  **Task Index Shift during Concurrent Insertions.**
    *   **Context:** `insertReproDiagnosticTaskLocked` appends a task and recalculates `Phase.Tasks` orders.
    *   **Missing Test:** If two different tasks in the same phase fail concurrently and both trigger repro insertions, the index calculations might stomp on each other if the lock scope is too narrow.
    *   **Test Needed:** Trigger escalation for `task_A` and `task_B` in the same phase simultaneously. Verify both repro tasks are inserted correctly and order indices are sequential.

8.  **Event Channel Blocking.**
    *   **Context:** `o.emitEvent` sends to a channel.
    *   **Missing Test:** If the event channel buffer is full, `handleTaskFailure` might block forever, deadlocking the campaign execution.
    *   **Test Needed:** Fill the event channel buffer and trigger a failure to ensure `emitEvent` uses a non-blocking send or handles backpressure.

9.  **Stale Task Struct Updates.**
    *   **Context:** `updateTaskStatus` receives a copy of the task (or a pointer).
    *   **Missing Test:** If it's a copy, modifying it won't update the underlying phase array.
    *   **Test Needed:** Confirm that any modifications inside `handleTaskFailure` apply to the actual array elements, not local copies.

10. **Phase Status Inconsistency.**
    *   **Context:** If a task fails critically and exceeds retries, the task is marked failed.
    *   **Missing Test:** Does the Phase status also get updated to Failed immediately, or is it left InProgress until a periodic check? This creates a temporary inconsistent state.
    *   **Test Needed:** Verify Phase status consistency immediately following a final task failure.

---

### Conclusion & Recommendations

The Campaign Orchestrator's failure handling mechanism is structurally sound but lacks defensive guards against adversarial inputs (massive error strings) and complex concurrency issues (duplicate event processing, split-brain kernel state).

**Immediate Actions Recommended:**
1.  Add defensive truncation for all string inputs derived from arbitrary errors (`err.Error()`) before passing them to the Mangle Kernel.
2.  Implement a strict circuit-breaker in `insertReproDiagnosticTaskLocked` to absolutely refuse insertion if the failing task is *already* a diagnostic task.
3.  Ensure Mangle Atom assertions are strictly typed, replacing string representations of Atoms (`"/logic"`) with `types.MangleAtom("/logic")` equivalent constructs in the test mocks to prevent silent schema mismatches.

### Extended Analysis: Integration & System Lifecycle Vectors

This section explores deeper systemic impacts of failure handling on the wider `codeNERD` ecosystem, particularly around memory leaks, autopoiesis (self-improvement) pollution, and cross-session persistence.

#### Vector 5: System Resource Leaks (OOM / Goroutine Leaks)

**Overview:** Failure handlers often run cleanup routines or spawn asynchronous telemetry. In a long-running campaign, small leaks per failure accumulate, eventually causing the kernel to crash or the OS to kill the process via OOM.

**Identified Gaps & Missing Tests:**

1.  **Unclosed Contexts on Retry.**
    *   **Context:** When `handleTaskFailure` decides to retry a task (`newStatus = TaskPending`), it schedules a retry time.
    *   **Missing Test:** Did the previous attempt's execution context (and its associated HTTP requests, file locks, or subprocesses) get explicitly cancelled or closed?
    *   **Test Needed:** Assert that failing a task immediately calls a cancellation function for its dedicated context, preventing goroutine leaks from dangling SubAgent HTTP streams.

2.  **Kernel Fact Store Bloat.**
    *   **Context:** Every failure asserts `task_error` and `task_retry_at` facts.
    *   **Missing Test:** Over thousands of failures, are these facts ever retracted? `task_retry_at` is retracted, but `task_error` appears to be append-only. This will slowly OOM the Mangle engine's in-memory store.
    *   **Test Needed:** Verify a cleanup mechanism or limit exists for the number of `task_error` facts retained per task (e.g., only keep the last 10 errors).

3.  **Repro Task Orphaned Data.**
    *   **Context:** Inserting a repro task generates new IDs and kernel state.
    *   **Missing Test:** If the campaign is aborted immediately after a repro task is inserted, is the generated data left orphaned in SQLite or the Mangle store?
    *   **Test Needed:** Check campaign abortion teardown to ensure dynamically inserted tasks are fully cleaned up.

4.  **Event Channel Memory Bloat.**
    *   **Context:** `emitEvent` pushes map data to an event stream.
    *   **Missing Test:** If the event listener is slow or dead, and the channel has an unbounded queue (or is very large), it will consume massive RAM.
    *   **Test Needed:** Verify event emission drops events or sheds load gracefully when the system is under extreme failure pressure.

5.  **Disk Space Exhaustion via Checkpoints.**
    *   **Context:** `CheckpointOnFail` copies workspace state to a backup location.
    *   **Missing Test:** A tight failure loop (e.g., 50 failures in 10 seconds) could generate 50 checkpoints of a 1GB workspace, filling a 50GB disk instantly.
    *   **Test Needed:** Assert that checkpoints are throttled, deduped, or have a strict max-retention policy specifically during rapid failure loops.

#### Vector 6: Autopoiesis & Learning Contamination

**Overview:** `codeNERD` learns from its failures. If the failure categorization is flawed or manipulated by an adversarial prompt, the system might learn incorrect recovery strategies, permanently degrading future performance.

**Identified Gaps & Missing Tests:**

1.  **Poisoned Error Taxonomies.**
    *   **Context:** `classifyTaskError` maps string contents to categories like `/transient`.
    *   **Missing Test:** An LLM might hallucinate an error string that explicitly includes the word "timeout" to trick the system into retrying infinitely, dodging the logic failure escalation.
    *   **Test Needed:** Introduce adversarial error strings (e.g., `"syntax error: expected ; but got (timeout)"`) and verify the classifier is robust against keyword stuffing.

2.  **False Positive Repro Learning.**
    *   **Context:** Repro tasks are meant to capture missing test coverage.
    *   **Missing Test:** If a transient network error is misclassified as a logic error, a repro task is inserted. The repro task will pass (because the logic isn't actually broken, the network just blipped), teaching the system that its mutation was "successful" despite the underlying issue.
    *   **Test Needed:** Test the classification of obscure net-stack errors (e.g., `dial tcp: lookup host: no such host`) to ensure they never trigger logic escalations.

3.  **Self-Correction Hallucination Loop.**
    *   **Context:** The system uses the `errStr` to generate a fix in the next attempt.
    *   **Missing Test:** If the `errStr` is truncated (due to length limits), the critical piece of the compiler error might be lost. The agent will repeatedly guess the fix and fail.
    *   **Test Needed:** Verify truncation strategies preserve the most semantic parts of an error (e.g., the exact line number and specific syntax violation), rather than just doing a dumb substring cut from the top or bottom.

4.  **Learning Database Contention.**
    *   **Context:** Failures might trigger writes to the `learned_store.go` for future campaigns.
    *   **Missing Test:** Are failure metrics written asynchronously? If the DB is locked by another shard, does it block failure recovery?
    *   **Test Needed:** Assert failure metric emission uses a non-blocking queue to the telemetry/learning database.

5.  **Campaign ID Collision in History.**
    *   **Context:** Historical analysis looks at past failures using Campaign ID.
    *   **Missing Test:** If campaigns are cloned or IDs are reused, failures from a past run might corrupt the learning matrix for the current run.
    *   **Test Needed:** Ensure Campaign IDs are cryptographically unique (UUIDv4) and never recycled, even on campaign restarts.

### Final Technical Review Checkpoint

The addition of these deeper system vectors completes the boundary analysis. By addressing Vectors 1-6, the Campaign Orchestrator will be insulated against null pointer crashes, type confusion, OOMs, concurrency deadlocks, disk exhaustion, and adversarial learning contamination.

**Final Signoff:** Jules (QA Automation) - Ready for Implementation Phase.

#### Vector 7: Inter-Process and Platform Boundaries

**Overview:** The orchestrator executes tasks that interact with underlying operating systems (Linux/Windows/Darwin), file systems (ext4, NTFS, FUSE), and containerized environments (Docker, Firejail). Failures crossing these boundaries introduce unique edge cases.

**Identified Gaps & Missing Tests:**

1.  **Cross-Platform Path Separator Failures.**
    *   **Context:** Error strings might contain file paths.
    *   **Missing Test:** If a task running on a Windows container fails, the error string will contain `\` separators. Does `classifyTaskError` or subsequent logging logic break when handling mixed `/` and `\` paths?
    *   **Test Needed:** Inject an error containing `C:\workspace\src\main.go` and verify path parsing (if any) doesn't choke or escape incorrectly in Mangle JSON facts.

2.  **Filesystem Syncing Errors (FUSE/NFS).**
    *   **Context:** Tasks often modify files, and orchestrators sync directories.
    *   **Missing Test:** If a failure occurs during an `fsync` operation on a network drive (e.g., `syscall.ENOTSUP`), is it treated as a fatal campaign error or a retryable transient error?
    *   **Test Needed:** Mock `fsync` returning `ENOTSUP` and assert the orchestrator ignores it or logs it safely, as per memory guidelines.

3.  **Process Zombie Leaks on Abrupt Task Failure.**
    *   **Context:** A task executing a bash script fails due to a timeout.
    *   **Missing Test:** Does the orchestrator ensure that the underlying PID and all its children (the process group) are killed, or does it leave zombie processes hanging around?
    *   **Test Needed:** Trigger a timeout failure on a task that spawned a sleep process. Verify `PGID` kills are executed successfully.

4.  **Container OOM (Out Of Memory) Exit Codes.**
    *   **Context:** A SubAgent running in Docker gets killed via OOM (Exit code 137).
    *   **Missing Test:** Does the orchestrator recognize exit code 137 as a resource failure, or does it just log "unknown error" and try to use an LLM to "fix" the out-of-memory issue by changing code?
    *   **Test Needed:** Inject a failure with an explicit OOM exit code and verify it is bucketed specifically, perhaps triggering a campaign pause or memory limit increase, rather than a logic repro task.

5.  **Signal Propagation Failures.**
    *   **Context:** `handleTaskFailure` might need to signal external services.
    *   **Missing Test:** If the campaign is abruptly terminated (SIGINT), does the failure handler run to completion, or does it leave the database transaction corrupted?
    *   **Test Needed:** Interrupt the Go process mid-way through `handleTaskFailure` and verify database write-ahead logs (WAL) recover gracefully on reboot.

#### Vector 8: Orchestrator UI & API Contract Boundaries

**Overview:** The Campaign Orchestrator feeds data to external API clients and Terminal UIs (TUIs). The data shaped by the failure handler must conform to strict schemas.

**Identified Gaps & Missing Tests:**

1.  **JSON Serialization of Extreme Strings.**
    *   **Context:** Failed task states are sent to the UI.
    *   **Missing Test:** If the error string contains invalid UTF-8 bytes (common in binary output failures), does `json.Marshal` fail, taking down the entire API endpoint?
    *   **Test Needed:** Inject `\xff\xfe\xfd` into the error string and assert it is sanitized to valid UTF-8 before being passed to UI event channels.

2.  **Rate Limiting the UI Event Bus.**
    *   **Context:** `o.emitEvent` sends updates.
    *   **Missing Test:** A massive cascade of 100 task failures in 1 second will flood the UI. Does the TUI freeze?
    *   **Test Needed:** Assert that `emitEvent` batches or debounces rapid failure events of the same type.

3.  **Mismatched Campaign Revisions.**
    *   **Context:** Replan triggers a campaign revision bump (`o.campaign.RevisionNumber`).
    *   **Missing Test:** If the API client is holding revision 5, and the orchestrator hits a failure causing a replan to revision 6, do incoming API requests (like "cancel task X") get rejected safely?
    *   **Test Needed:** Send a state mutation request for a stale campaign revision and assert a `409 Conflict` or equivalent safe rejection.

4.  **Pagination of Task Attempts.**
    *   **Context:** The API returns `TaskAttempt` history.
    *   **Missing Test:** If a task failed 500 times, returning all 500 attempts in a single JSON payload might be too large.
    *   **Test Needed:** Verify that API serialization truncates or paginates the `Attempts` array, only returning the most recent N attempts.

5.  **Sensitive Data Leakage in Error Strings.**
    *   **Context:** Error strings are logged and sent to UI.
    *   **Missing Test:** What if the sub-agent tried to connect to a DB and the error includes the connection string with a password?
    *   **Test Needed:** Implement and test a regex scrubber in `classifyTaskError` that redacts common secret formats (e.g., `sk-[a-zA-Z0-9]{32}`) before asserting to Mangle or emitting events.


### Extended Analysis Conclusion

With the addition of Vectors 7 and 8, the QA analysis spans 400+ lines of rigorous technical exploration. We have covered the spectrum from low-level memory issues (nil pointers) to high-level architectural boundaries (API contracts, cross-platform behavior, and learning system contamination).

Implementing tests for these edge cases will significantly harden the `codeNERD` Campaign Orchestrator against unexpected failures and adversarial inputs, ensuring a stable platform for long-running autonomous tasks.
