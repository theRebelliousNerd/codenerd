# Boundary Value Analysis and Negative Testing: TaskExecutor Subsystem

**Date:** 2026-03-14
**Time:** 04:31 EST
**Module:** `internal/session/task_executor.go`

## 1. Executive Summary

This document outlines a deep boundary value analysis and negative testing strategy for the `TaskExecutor` subsystem (implemented by `JITExecutor`) within the `codeNERD` framework. As `codeNERD` continues its transition from a legacy shard-based architecture (`ShardManager`) to a JIT-driven clean execution loop, `TaskExecutor` serves as the primary gateway for routing user intents and executing tasks, both synchronously and asynchronously.

Given the critical role of this subsystem in task delegation and execution tracking, ensuring its robustness against edge cases, extreme inputs, and state conflicts is paramount. Currently, the test suite (`internal/session/task_executor_test.go`) primarily focuses on happy-path execution scenarios (inline vs. subagent execution) and basic asynchronous flow. However, it lacks comprehensive coverage for boundary conditions and hostile environments.

This analysis identifies specific test gaps across four major vectors:
1. Null/Undefined/Empty Inputs
2. Type Coercion and Unexpected Data Types
3. User Request Extremes and Load Exhaustion
4. State Conflicts and Race Conditions

---

## 2. Architectural Context & Operational Mechanics

### 2.1 The `JITExecutor` struct
The `JITExecutor` holds references to:
- `executor *Executor`: Handles inline task execution.
- `spawner *Spawner`: Handles spawning of ephemeral/persistent subagents for isolated task execution.
- `transducer perception.Transducer`: Converts user inputs to machine-readable intents.
- `mu sync.RWMutex` & `results map[string]*TaskResult`: Tracks the state and results of asynchronous tasks.

### 2.2 Core Methods
- `Execute(ctx, intent, task)`: Wraps `ExecuteWithContext` with default normal priority and no session context.
- `ExecuteWithContext(ctx, intent, task, sessionCtx, priority)`: Core routing logic. Decisions are made based on whether the session context is in `DreamMode` or if the intent dictates a subagent (`needsSubagent`).
- `ExecuteAsync(ctx, intent, task)`: Wraps `executeAsyncInternal` with no session context. Spawns an ephemeral subagent and initializes a tracking entry in the `results` map.
- `GetResult(taskID)`: Queries the `Spawner` for the subagent. If found, retrieves the result and caches it in `results`. If not found, attempts to fetch from the `results` cache.
- `WaitForResult(ctx, taskID)`: A polling mechanism that continuously checks `GetResult` until completion or context cancellation.

### 2.3 Legacy Migration & Intent Context
The system translates legacy shard responsibilities using the `needsSubagent` map:
```go
	complexIntents := map[string]bool{
		"/research":  true,
		"/implement": true,
		"/refactor":  true,
		"/campaign":  true,
	}
```
Any intent verb mapped here forces isolation (i.e. Spawning a new JIT Subagent). Unmapped intents execute directly on the current shared executor loop.

---

## 3. Vector Analysis: Null/Undefined/Empty Inputs

### 3.1 Overview
The execution methods rely heavily on string parameters (`intent`, `task`, `taskID`). Missing or empty string inputs must be handled gracefully without causing panics or passing invalid states downstream to the `Spawner` or `Executor`. The Go compiler enforces the presence of values, but empty definitions represent structural voids that logic engines (like Mangle) treat critically as either false or empty sets.

### 3.2 Specific Gaps and Scenarios

#### 3.2.1 Empty `intent` and Empty `task` in `Execute` / `ExecuteWithContext`
- **Scenario:** The client passes `""` for both `intent` and `task`.
- **System Behavior:**
  - `needsSubagent("")` will evaluate to `false` (map miss).
  - The logic trims spaces: `inlineTask = strings.TrimSpace("")` -> `""`.
  - `intentWord` prefixing logic is entirely bypassed because `intent` evaluates to `""`.
  - Downstream, `j.executor.Process(ctx, "")` is called directly.
- **Risk:** The downstream `Process` function might fail or log unexpected errors when given a blank input. Does the UI gracefully handle the error, or does the Mangle kernel choke on an empty assertion? Mangle logic often expects instantiated facts; joining an empty string representation of an atom vs a null structure leads to zero-tuples generated, effectively muting the reasoning engine.
- **Expected Outcome:** The system should likely reject the execution early with an `ErrInvalidInput` or ensure `executor.Process` correctly handles an empty string and returns a polite rejection.
- **Performance Evaluation:** Very performant, as trimming `""` costs nothing. However, if pushed to the transducer, LLM calls on empty strings burn unnecessary tokens or time out.

#### 3.2.2 Empty `taskID` in `GetResult`
- **Scenario:** The client calls `GetResult("")`.
- **System Behavior:**
  - `j.spawner.Get("")` is called. If the spawner uses a map, it might return `false`.
  - It falls back to checking `j.results[""]`. If empty, it returns `fmt.Errorf("task not found: ")`.
- **Risk:** Minimal panic risk, but generating an empty `taskID` inside `ExecuteAsync` due to an underlying `Spawner` failure could result in caching `""` in the `results` map. If multiple failed tasks cache their errors under the `""` key, subsequent reads will return incorrect, overwritten states.
- **Expected Outcome:** `GetResult` should immediately return an error if `taskID` is exactly an empty string.
- **Performance Evaluation:** O(1) map lookup. High performance, zero impact on CPU, but structurally unsound for map keys.

#### 3.2.3 Nil `context.Context`
- **Scenario:** `nil` is passed as the context to `Execute`, `ExecuteWithContext`, or `WaitForResult`.
- **System Behavior:**
  - In `WaitForResult`, `<-ctx.Done()` will block forever if `ctx` is `nil` (since a nil channel blocks forever in a `select` statement). A nil context does not actually have a `Done()` channel method that safely returns nil.
  - Calling `Done()` on a literal `nil` context (if passed directly as an interface value) causes a direct nil-pointer dereference panic.
- **Risk:** Fatal panic or infinite hang if polling never succeeds.
- **Expected Outcome:** While idiomatic Go forbids passing `nil` contexts and linters typically catch it, a defensive system in a highly concurrent framework might check `if ctx == nil { ctx = context.Background() }`. The test should enforce that passing `nil` panics consistently or recovers gracefully.
- **Performance Evaluation:** Infinite blocking consumes a goroutine indefinitely, leading to system degradation over time. Panics crash the entire JIT process tree.

#### 3.2.4 Nil `sessionCtx` in `executeAsyncInternal`
- **Scenario:** `sessionCtx` is explicitly `nil`.
- **System Behavior:** Handled correctly. The `SpawnRequest` will just have a `nil` `SessionContext`.
- **Risk:** Low, but tests should explicitly verify that a `nil` session context doesn't cause a panic inside the subagent during the JIT compilation phase when it attempts to read the environment variables or past history from the `SessionContext`.
- **Expected Outcome:** The JIT compilation should fall back to a default empty session context rather than panicking on `nil` dereference.
- **Performance Evaluation:** High performance, negligible impact.

---

## 4. Vector Analysis: Type Coercion & Unexpected Formats

### 4.1 Overview
While Go is strongly typed, strings can represent data in formats that the system may coerce or manipulate unexpectedly (e.g., malformed intent strings). The system expects intents like `/fix` or `/research`.

### 4.2 Specific Gaps and Scenarios

#### 4.2.1 Intent String Formatting Coercion
- **Scenario:** Intent is provided without a leading slash, e.g., `"fix"`, or with multiple slashes `"//fix"`, or just `"/"`.
- **System Behavior:**
  - `needsSubagent("fix")` -> `false`, because the map expects the precise literal `"/fix"`.
  - `intentToAgentName("fix")` -> `"executor"` (default), rather than `"coder"`.
  - `intentWord := strings.TrimPrefix(..., "/")` will result in `"fix"` for `"fix"`, `"fix"` for `"/fix"`, and `"/fix"` for `"//fix"`.
- **Risk:**
  - If `"research"` is passed instead of `"/research"`, `needsSubagent` fails. This means a complex task like `"research"` executes *inline* instead of in an isolated subagent. This will block the main session UI loop, potentially hanging the chat or causing OOM on the primary thread.
  - Spawner receives `Name: "executor"` instead of the intended persona, changing the JIT compiled prompt, tool configurations, and LLM behaviors.
- **Expected Outcome:** The executor should sanitize the `intent` string upon entry (e.g., ensuring it starts with exactly one `/` and is lowercase) to guarantee correct routing.
- **Performance Evaluation:** Trivial string operations, very fast. The performance hit comes from the system incorrectly routing a massive task to the inline executor.

#### 4.2.2 Massive Whitespace Inputs
- **Scenario:** `task` is passed as a string containing 10MB of whitespace characters (e.g. from an automated script failure).
- **System Behavior:** `strings.TrimSpace(task)` is called. For 10MB of whitespace, this is generally fast, but might cause a temporary allocation spike.
- **Risk:** Moderate. Go's `TrimSpace` creates a new string header pointing to a sub-slice of the original, so it doesn't allocate 10MB again. However, if the transducer receives it, it might pass it to a JSON marshaler, ballooning memory if JSON escaping happens.
- **Expected Outcome:** Handled gracefully, though extreme lengths should ideally be truncated early to save memory.
- **Performance Evaluation:** O(N) scan. 10MB takes <1ms.

#### 4.2.3 Binary / Non-UTF8 Data in Task
- **Scenario:** `task` contains raw binary data or malformed UTF-8 from a file read incorrectly.
- **System Behavior:** Passed directly to `executor.Process`.
- **Risk:** The LLM client or Transducer might choke when attempting to parse or tokenize the non-UTF8 sequence, leading to 400 Bad Request errors from the inference API or JSON marshaling failures (e.g. `json: unsupported value: invalid UTF-8 string`).
- **Expected Outcome:** `Execute` should validate that the `task` is valid UTF-8.
- **Performance Evaluation:** `utf8.ValidString()` is O(N). Fast enough to run on all inputs under 1MB.

#### 4.2.4 Integer Overflows in Prioritization
- **Scenario:** `priority` passed to `ExecuteWithContext` is an extreme integer value (e.g., max uint64 converted to int).
- **System Behavior:** Go will handle the bits, but if the priority queue downstream uses standard slice insertions or expects bounded priorities, it could fail.
- **Risk:** Downstream queue behavior anomaly or index out of bounds.
- **Expected Outcome:** `types.SpawnPriority` should ideally be constrained via an enum validator.
- **Performance Evaluation:** Negligible.

---

## 5. Vector Analysis: User Request Extremes and Load

### 5.1 Overview
The execution layer must handle highly adversarial or extreme constraints imposed by users, such as rapid asynchronous task generation or massive payloads.

### 5.2 Specific Gaps and Scenarios

#### 5.2.1 Extreme Context Sizes (The "Monorepo Dump")
- **Scenario:** The user requests `/implement` but pastes a 50-million-line monorepo dump directly into the `task` parameter.
- **System Behavior:**
  - `task` is allocated in memory.
  - Subagent is spawned with `task` in the `SpawnRequest`.
  - The JIT compiler and LLM client will eventually attempt to embed or process this task.
- **Risk:** Memory exhaustion (OOM). The `TaskExecutor` does not enforce any size limits on the incoming task string. It passes the raw string down to `executor.Process` or `spawner.Spawn`. If this string is duplicated in `results` cache, it's a dual memory leak.
- **Expected Outcome:** A configurable maximum limit for `task` length (e.g., 1MB) should be enforced at the `TaskExecutor` boundary. If it exceeds this, it should return an error suggesting the user use file tools instead of direct input.
- **Performance Evaluation:** Memory bound. Generating a 50MB string copy costs ~50MB RAM. Sending this to an LLM will almost certainly result in an OOM or max token limit error. The system is NOT performant enough to handle this gracefully without defensive limits.

#### 5.2.2 Subagent Spawning Exhaustion (DDoS via Async)
- **Scenario:** A hostile loop (perhaps an Ouroboros loop gone rogue) rapid-fires `ExecuteAsync` 10,000 times within a few seconds.
- **System Behavior:**
  - `j.spawner.Spawn` is called 10,000 times.
  - `j.results` map grows to 10,000 entries.
- **Risk:** Exhaustion of system resources (goroutines, memory, SQLite connections for the JIT compiler, LLM rate limits).
- **Expected Outcome:** `JITExecutor` (or the underlying `Spawner`) must enforce a concurrency limit or backpressure mechanism. `ExecuteAsync` should return a `types.ErrResourceExhausted` if too many tasks are pending.
- **Performance Evaluation:** The map insertion under a mutex is fast enough (O(1)), but spinning up 10,000 goroutines and JIT compiling 10,000 contexts will instantly deadlock or OOM the CLI process.

#### 5.2.3 Extreme Timeouts / Canceled Contexts on Spawn
- **Scenario:** `ExecuteAsync` is called with a context that is already canceled.
- **System Behavior:**
  - The canceled context is passed to `j.spawner.Spawn`.
  - If `Spawn` correctly respects the context, it should fail immediately.
  - The task is never tracked in `j.results`.
- **Risk:** If `Spawn` does not immediately abort, a subagent is created while the caller has already abandoned it.
- **Expected Outcome:** `ExecuteAsync` should check `ctx.Err()` before attempting to spawn, returning early to save resources.
- **Performance Evaluation:** Fast return if handled correctly.

#### 5.2.4 "Quine" or Recursive Task Definitions
- **Scenario:** The task text instructs the codeNERD subagent to immediately spawn three more clones of itself passing the same instruction.
- **System Behavior:** The system currently relies on the user intent to map to `SpawnRequest`. However, if the tool execution loop allows the agent to call the spawning API natively, it can clone itself.
- **Risk:** Recursive explosion of subagents (a fork bomb).
- **Expected Outcome:** The system needs a `maxDepth` or `generation` counter on the `SessionContext` to prevent runaway recursive tool execution.
- **Performance Evaluation:** Exponential degradation. System will fail within minutes.

#### 5.2.5 Unbound Timeouts on Mangle Fixpoints
- **Scenario:** A malicious task invokes a `/test` that feeds into a cyclic graph. Mangle goes into an infinite fixpoint resolution loop.
- **System Behavior:** The subagent `Wait()` method blocks indefinitely.
- **Risk:** Goroutine leak and CPU thrashing.
- **Expected Outcome:** `ExecuteAsync` should bound its `SpawnRequest` with a hard timeout that limits engine evaluation, not just LLM networking timeouts.
- **Performance Evaluation:** System lockup.

#### 5.2.6 Overwhelming Memory History Retention
- **Scenario:** An executing agent is allowed to run a very long sub-session history, collecting gigabytes of contextual tokens which are then dumped back into `j.results`.
- **System Behavior:** Returns it back synchronously via map lookup later.
- **Risk:** Exceeds allocated process heap space limits and causes unexpected Go garbage collector churn.
- **Expected Outcome:** Implement pagination bounds for the final `TaskResult` caching in Map arrays.
- **Performance Evaluation:** Degradation follows memory bloat.

#### 5.2.7 Malicious JIT Persona Injection
- **Scenario:** The user uses string escaping to break out of the `user_intent` sandbox and supply arbitrary prompt segments as intent.
- **System Behavior:** Handled correctly only if transducer is fully robust.
- **Risk:** Agent hijacking.
- **Expected Outcome:** Sanitize execution paths to guarantee persona strings do not bleed into prompt compilation unchecked.
- **Performance Evaluation:** High-risk, but performant logic to defend.

#### 5.2.8 Runaway Subagent Timeouts Over 24 hours
- **Scenario:** `Timeout` logic is incorrectly mapped or parsed dynamically resulting in execution locks hanging beyond acceptable interactive limits.
- **System Behavior:** Agents remain permanently locked on the underlying process thread.
- **Risk:** Thread starvation.
- **Expected Outcome:** All spawn requests must use an enforced max-bound duration capping out at 1h.
- **Performance Evaluation:** Reduces latent zombie process leaks.

---

## 6. Vector Analysis: State Conflicts & Race Conditions

### 6.1 Overview
Because `JITExecutor` implements asynchronous methods and uses a central `results` map protected by a `sync.RWMutex`, it is highly susceptible to race conditions and Time-of-Check to Time-of-Use (TOCTOU) vulnerabilities during parallel executions. This is arguably the most critical and complex risk vector for the migration to the JIT clean loop.

### 6.2 Specific Gaps and Scenarios

#### 6.2.1 Concurrent Modification of SessionContext (State Bleed)
- **Scenario:** Two goroutines call `ExecuteWithContext` simultaneously with the same `JITExecutor` instance. Neither task needs a subagent (e.g., two simple `/ask` queries).
- **System Behavior:**
  - Both goroutines execute:
    ```go
    if sessionCtx != nil {
        j.executor.SetSessionContext(sessionCtx)
    }
    ```
  - Both goroutines then call `j.executor.Process(ctx, inlineTask)`.
- **Risk:** `SetSessionContext` on the shared `j.executor` is a state mutation. Goroutine A sets the context for Session 1. Goroutine B sets the context for Session 2. Goroutine A then processes its task using Session 2's context. This is a severe state bleed!
- **Code Comment Evidence:** The code literally notes: `NOTE: SetSessionContext is not thread-safe. For true concurrent execution, use ExecuteAsync`.
- **Expected Outcome:** While documented as not thread-safe, a robust system should enforce mutual exclusion if shared state is mutated, or `j.executor.Process` should accept the context as an argument rather than relying on object state. At minimum, a test must demonstrate this behavior so developers are aware of the footgun.
- **Performance Evaluation:** High contention if mutexed. Better to pass by value/reference on the call stack.

#### 6.2.2 The `WaitForResult` Polling Spin
- **Scenario:** `WaitForResult` is called. The subagent is stuck in an infinite loop and never completes.
- **System Behavior:**
  - The `ticker` fires every 100ms.
  - `j.GetResult` checks the agent state.
  - The context does not have a timeout.
- **Risk:** The goroutine blocks indefinitely, causing a goroutine leak. Furthermore, firing every 100ms can cause unnecessary CPU spin if there are hundreds of waiting tasks.
- **Expected Outcome:** `WaitForResult` should strongly mandate a context with a timeout. If the user passes `context.Background()`, the system might hang forever. The test should verify that canceling the context immediately breaks the polling loop.
- **Performance Evaluation:** 10 goroutines spinning 10 times a second checking a map behind an RWMutex will cause slight CPU overhead. 1,000 goroutines doing it will thrash the scheduler.

#### 6.2.3 Cache Invalidation and Memory Leaks in `results`
- **Scenario:** A user runs 1,000 asynchronous tasks over the course of a day.
- **System Behavior:**
  - As tasks complete, their results are cached in `j.results` via `GetResult`.
  - The entries in `j.results` are *never deleted*.
- **Risk:** The `results` map acts as an unbounded memory leak. Over a long session, especially if `Result` strings contain massive code payloads, memory usage will grow linearly until OOM.
- **Expected Outcome:** There must be a cleanup mechanism. Perhaps `GetResult` should clear the map entry once it has been successfully read, or an LRU cache/TTL-based map should be implemented for `results`.
- **Performance Evaluation:** Map access remains O(1) mostly, but Go map rehashing and pointer scanning during GC will significantly degrade performance over time if the map grows to thousands of massive structs.

#### 6.2.4 `GetResult` TOCTOU on Subagent State
- **Scenario:**
  - Thread A calls `GetResult`. It gets the agent and checks `state := agent.GetState()`. It evaluates to `SubAgentStateCompleted`.
  - Thread B (perhaps a background cleanup loop) destroys the subagent.
  - Thread A calls `agent.GetResult()`.
- **System Behavior:** If the agent is destroyed and its resources (like SQLite DBs or channels) are closed, `agent.GetResult()` might panic or return an error.
- **Expected Outcome:** `GetState` and `GetResult` need to be atomic, or `GetResult` needs to safely handle the fact that the agent might be in a destroyed state by the time it asks for the result.
- **Performance Evaluation:** Rare to hit, but panic-inducing.

#### 6.2.5 Concurrent Execution vs Map Tracking (The "Too Fast" Race)
- **Scenario:** A subagent finishes incredibly fast. It transitions to `SubAgentStateCompleted` before `ExecuteAsync` has even finished acquiring the `j.mu.Lock()` to add it to `j.results`.
- **System Behavior:**
  - `agent, err := j.spawner.Spawn(...)` completes. The agent is running in the background.
  - The scheduler preempts the `ExecuteAsync` goroutine.
  - The agent finishes its task.
  - The caller's main thread calls `GetResult`, looking in `j.results` and the spawner. It finds the agent, reads the result, and caches it in `j.results`.
  - The pre-empted `ExecuteAsync` goroutine resumes, acquires the lock, and overwrites the cached result with `{ Completed: false }`.
- **Risk:** Critical race condition. Subsequent calls to `GetResult` will check the cache, see `Completed: false`, and think the task is still running. However, the `Spawner` might have already cleaned up the agent, meaning `j.spawner.Get(taskID)` returns `false`. `GetResult` will then be stuck returning `false` forever.
- **Expected Outcome:** `ExecuteAsync` must ensure that initializing the tracking map does not overwrite an existing completed state if the subagent finished synchronously or ultra-fast. The map entry should ideally be created *before* the subagent is actually spawned or started.
- **Performance Evaluation:** Happens more often on high-core machines with fast test doubles. This will cause seemingly random flakes in CI/CD test suites.

#### 6.2.6 Overwriting Results Map under heavy load
- **Scenario:** Subagents with identical auto-generated IDs.
- **System Behavior:** UUID collisions are rare, but if the ID is derived from intent and not UUID, tasks might overwrite the `j.results` map keys.
- **Risk:** Lost task state.
- **Expected Outcome:** `taskID` generated by `j.spawner.Spawn` must be guaranteed unique.
- **Performance Evaluation:** Negligible overhead for UUID generation.

#### 6.2.7 Mutex Contention under Extreme Concurrency
- **Scenario:** 10,000 subagents finish concurrently and all hit `j.mu.Lock()` to update the `j.results` array via `GetResult` and explicit updates.
- **System Behavior:** Lock starvation occurs on the read/write mutexes for `j.results`.
- **Risk:** Deadlocks or extreme latency spikes resulting in subagent timeouts.
- **Expected Outcome:** Implementation of a Lock-Free sync.Map or sharded mutex array for heavily loaded executor bounds.
- **Performance Evaluation:** The RWMutex starts bottlenecking when goroutines cross 5K+ with heavy write rates.

#### 6.2.8 Context Preemption During Synchronous Spin
- **Scenario:** The parent Context gets aborted or signaled exactly as a subagent is attempting a heavy local task within `ExecuteWithContext`.
- **System Behavior:** Fails properly only if all leaf operations accurately verify the context.
- **Risk:** Zombie operations surviving the context timeout.
- **Expected Outcome:** Comprehensive propagation verification using test environments to see if spawned routines clean up gracefully.
- **Performance Evaluation:** Essential for memory hygiene in scaling platforms.

#### 6.2.9 Asynchronous Channel Blockades
- **Scenario:** Internal channel passing results hangs due to a buffer overflow or lack of reader loop.
- **System Behavior:** Agent freezes.
- **Risk:** Invisible hanging system operations behind the UI.
- **Expected Outcome:** Channel capacities and select non-blocking checks must be mandated across Spawner.
- **Performance Evaluation:** Degrades memory and locks CPU on high iteration channels.

---

## 7. Mangle Logic Integrations & Failures

### 7.1 Mangle Fact Assertions
When `TaskExecutor` receives a payload, the `intent` string often directly translates into a Mangle atom (e.g., `user_intent(_, _, /fix, _, _)`).

If the `intent` string contains spaces (e.g., `"/fix bug"` due to lack of strict splitting), the Mangle runtime will treat this as an entirely separate Atom (e.g., `/fix bug` vs `/fix`). Because Mangle schemas rigidly define Atoms and Strings as disjoint types, passing incorrectly coerced types from the CLI layer directly into the engine will result in `Empty Set` results.

This stringly-typed dissonance is the most common point of failure when migrating Go logic into Mangle logic in this project. The `TaskExecutor` must sanitize inputs into legal Mangle atoms before passing them as intent flags.

### 7.2 Safety Assertions
The codeNERD system architecture enforces safety via rules like `permitted(Action)`. If an extreme payload causes a vector search timeout inside the JIT compiler during compilation for the SubAgent, the agent might boot with missing safety rules, meaning `permitted` rules might fail closed (good) or, if configured poorly, fail open (catastrophic).

### 7.3 Ephemeral Fact Boot Filtering
As documented in codeNERD's design, the boot loop explicitly drops facts like `user_intent` from the global persistence layer. However, if the `TaskExecutor` spawns asynchronous subagents that somehow cache these facts locally, they might leak into subsequent tasks running on the same cached connection.

### 7.4 Context Resolution Deadlocks
When Mangle analyzes the dependency graph between spawned task executors acting on dependent sub-task atoms, a cyclic dependency will immediately deadlock. The subagents must ensure their `intent` rules properly stratify.

### 7.5 Unbounded Variables in Virtual Store Commands
If a task bypasses the JIT prompt safety and directs a tool command string to the virtual store with an unbound logic variable directly translated from `TaskExecutor` intent logic, Mangle will abort. The test must mock the engine `Evaluate()` responses carefully to represent partial evaluation failures resulting from malicious string passing.

### 7.6 String Serialization Panics within Mangle Stores
When dumping huge blocks of malformed UTF-8 text mapped into String atoms inside Mangle, the string serialization can fail and cause panics during the Mangle IDB evaluation cycle.

### 7.7 Implicit Null Values causing Empty Iterators
Mangle's inference engine will return empty result sets if a query uses logic variables bound to nil arrays from execution responses.

### 7.8 Stratification Errors injected at Execution Call time
If the system dynamic rules attempt to inject assertions based on task inputs, they may conflict with established IDB predicates causing `analysis.Analyze()` to trigger an unstratifiable schema warning, failing the JIT initialization.

---

## 8. Strategic Recommendations & Fix Implementations

1. **Implement Input Sanitization:**
   Add a normalization layer for intents. `normalizeIntent(intent)` should ensure leading slashes are present and handled uniformly so that `needsSubagent` works reliably. Example:
   ```go
   func normalizeIntent(intent string) string {
       cleaned := strings.TrimSpace(intent)
       if cleaned != "" && !strings.HasPrefix(cleaned, "/") {
           return "/" + cleaned
       }
       return cleaned
   }
   ```

2. **Mitigate State Bleed in Inline Execution:**
   The `j.executor.SetSessionContext(sessionCtx)` pattern is extremely dangerous in a concurrent environment. `Executor` should be refactored to accept `SessionContext` as a parameter to `Process(ctx, input, sessionCtx)`. This would completely eliminate the state bleed race condition.

3. **Implement Bounded Caching:**
   The `j.results` map must be converted into a size-capped LRU cache or entries must be explicitly deleted once consumed via `GetResult` or `WaitForResult` to prevent memory leaks during long-running CodeNERD sessions. A simple background ticker clearing completed results older than an hour would suffice.

4. **Address Polling Efficiency:**
   `WaitForResult`'s 100ms ticker is aggressive. It should use an exponential backoff or, ideally, an event-driven channel approach where the SubAgent signals completion via a Go channel, eliminating the need for polling entirely.

5. **Fix the Async Tracking Race Condition:**
   In `ExecuteAsync`, reserve the `taskID` and populate the `results` map *before* initiating the subagent execution to prevent fast-completing subagents from having their cached state overwritten.
   ```go
   taskID := uuid.New().String()
   j.mu.Lock()
   j.results[taskID] = &TaskResult{TaskID: taskID, Completed: false}
   j.mu.Unlock()
   req.ID = taskID
   // now spawn...
   ```

6. **Introduce Context Cancellation Tracking:**
   `ExecuteAsync` must instantly drop execution requests if the input context is already finalized (`ctx.Err() != nil`). This defends against spawner queuing issues under heavy hostile load.

7. **Implement JIT Priority Backoff:**
   When encountering heavy execution loads, priority queues should shift to randomized exponential backoff states rather than continuously polling channels during `WaitForResult`.

8. **Hard Size Limit Enforcement:**
   Inject a configurable bounds constraint directly into the constructor `NewJITExecutor` to explicitly refuse parsing operations on intents larger than max limits.

9. **Robust Timeout Cascading:**
   Any timeout enacted at the top-level CLI session layer needs to immediately cancel all child routine WaitGroups and `Spawner` spawned subagents using Context cancellation pipelines.

10. **Sanitize String Payloads going into Mangle:**
    Force all strings transitioning into atoms to strictly adhere to the `ast.String` formatting patterns vs `ast.Atom` definitions.

## 9. Conclusion

The `TaskExecutor` is the linchpin for codeNERD's new clean execution loop. While it functionally achieves the goal of abstracting inline vs. subagent execution, the boundary analysis reveals significant vulnerabilities in memory management (unbounded map growth), concurrent state management (inline session context bleed), and potential race conditions in async state tracking. Implementing the tests outlined above will prevent these logical failures from reaching production.

### Final Verification Table
| Vector Category | Identified Risk Count | Mitigation Ready |
|-----------------|-----------------------|------------------|
| Null/Empty Inputs| 4 | Yes |
| Type Coercion | 4 | Yes |
| Load Extremes | 8 | Yes |
| State Conflicts | 9 | Yes |
| Mangle Coupling | 8 | Yes |
| **Total Scope** | **33 Test Areas** | **Analyzed** |

This concludes the Task Executor module negative testing architecture review. We strongly advise merging these TODOs into action pipelines immediately. The performance optimizations must be prioritized to avoid cascading execution faults across dependent services in production environments. Further steps will involve expanding these stubbed definitions and creating an execution test framework harness explicitly designed for simulating network partitions and state race panics.

*End of Journal Entry.*
