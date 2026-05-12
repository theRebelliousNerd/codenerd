---

remediated: false
subsystem: session
---
# Quality Assurance Journal: Session Clean Loop Boundary Analysis

**Date:** March 23, 2026
**Time:** 04:12 EST
**Subsystem:** Session Clean Loop (`internal/session/executor.go`, `internal/session/spawner.go`)

## 1. Introduction: The JIT-Driven Universal Execution Loop

codeNERD's architecture was updated in December 2024 to delete the legacy domain shards (coder, tester, reviewer, researcher). In their place, a streamlined "Universal Execution Loop" was implemented in the `session` package. This JIT-driven architecture significantly reduced boilerplate code, condensing roughly 35,000 lines of rigid implementations into about 1,115 lines of highly flexible infrastructure.

The core of this new architecture lies in two primary files:
- `executor.go`: Implements the clean OODA (Observe, Orient, Decide, Act) loop. It processes user input via the transducer, compiles the JIT prompt and configuration, communicates with the LLM, and dispatches actions to the Virtual Store.
- `spawner.go`: Manages the lifecycle and instantiation of these JIT-driven SubAgents. It dynamically determines the persona and capabilities required based on the parsed intent, provisioning SubAgents as ephemeral, persistent, or system-level processes.

Because this subsystem acts as the central nervous system for all codeNERD actions, its resilience to boundary conditions and edge cases is paramount. A failure here does not just break a single tool; it halts the entire cognitive cycle of the agent. This analysis focuses strictly on non-happy-path scenarios, specifically identifying gaps in the current test suite concerning Null/Undefined/Empty inputs, Type Coercion, User Request Extremes, and State Conflicts.

---

## 2. Boundary Value Analysis and Edge Case Vectors

The current test suite (`executor_process_test.go` and `spawner_test.go`) covers the basic happy path: a user says "Hello," the LLM replies "Hello user," and the correct intent is parsed. However, it severely lacks robustness testing. Below is a detailed analysis of missing coverage across critical failure vectors.

### 2.1 Null / Undefined / Empty Inputs

**Vector 1: Empty User Input (`executor.Process`)**
- *Scenario:* A user submits an empty string (`""`) or entirely whitespace payload.
- *System Behavior:* The transducer attempts to parse the intent of `""`. Depending on the transducer implementation (e.g., regex vs. ML classifier), this might return an empty intent, throw an error, or hallucinate a default action.
- *Performance Implication:* If it reaches the LLM, it wastes API tokens on a blank prompt. The executor should ideally short-circuit the OODA loop if the input is trivial, responding with a rapid default articulation (e.g., "How can I help you?").
- *Gap:* There is no test verifying that `Process(ctx, "")` halts cleanly without panic or unnecessary network calls.

**Vector 2: Nil Compilation Context (`executor.buildCompilationContext`)**
- *Scenario:* The `intent` struct returned by the transducer has empty or nil fields (e.g., `Verb: ""`, `Target: nil`).
- *System Behavior:* `buildCompilationContext` sets `IntentVerb: ""` and `IntentTarget: nil`. When this is passed to the `JITCompiler`, the compiler might fail to locate a matching persona, defaulting to a bare-bones baseline.
- *Performance Implication:* Minimal performance hit, but massive quality degradation. The agent will act lobotomized.
- *Gap:* The test suite does not verify the behavior of the Spawner or Executor when the intent is semantically void.

**Vector 3: Nil Tool Arguments (`executor.executeToolCall`)**
- *Scenario:* The LLM decides to call a tool but hallucinates the structure, providing a `nil` `call.Args` map instead of `{}` or omitting required keys.
- *System Behavior:* `executeToolCall` marshals the nil map to JSON for Ouroboros tools or passes it directly to modular tools.
- *Performance Implication:* If a tool implementation does not nil-check its arguments, this will cause a nil pointer dereference panic, crashing the `SubAgent` goroutine.
- *Gap:* No test injects a `ToolCall{Args: nil}` into the execution pipeline to ensure the system rejects it gracefully rather than panicking.

### 2.2 Type Coercion

**Vector 1: Mangle Argument Coercion (`executor.parseMangleArg`)**
- *Scenario:* The `parseMangleArg` function uses a heuristic to determine if a string is a Mangle Atom or a String literal. It checks for a leading `/`.
- *System Behavior:* If a user or tool generates a legitimate string that happens to start with `/` (e.g., an absolute file path `"/etc/passwd"`), `parseMangleArg` coerces it into a Mangle Atom instead of a string.
- *Performance Implication:* This causes a "String/Atom Dissonance." A Mangle query looking for the string `"/etc/passwd"` will fail to join with the Atom `//etc/passwd`, resulting in silent logical failures (empty Datalog sets) rather than explicit errors.
- *Gap:* Tests must explicitly verify that `parseMangleArgs` correctly differentiates between paths, atoms, and quoted literals, particularly around edge cases like path representations.

**Vector 2: Tool Argument Extraction (`executor.extractTarget`)**
- *Scenario:* The safety gate attempts to extract the primary target of a tool call to assert `pending_action(..., Target, ...)`. It loops through common keys like `"path"`, `"url"`.
- *System Behavior:* If the LLM passes an array or a complex JSON object as the value for `"path"` instead of a scalar string, `types.ExtractString(val)` might fail or return a serialized JSON string.
- *Performance Implication:* If the target is incorrectly extracted, the `permitted` query will fail to match the target, triggering a false positive in the Constitutional Gate and blocking a valid action.
- *Gap:* There is no negative test passing nested objects to `extractTarget` to ensure the safety gate degrades gracefully or rejects malformed types predictably.

### 2.3 User Request Extremes

**Vector 1: Infinite Tool Call Loops (`executor.Process`)**
- *Scenario:* The LLM gets stuck in a loop, continually generating tool calls that fail, then retrying the exact same tool call (e.g., trying to read a non-existent file).
- *System Behavior:* The loop `for i, call := range llmResponse.ToolCalls` executes sequentially. The `ExecutorConfig` defines a `MaxToolCalls` limit (default 50).
- *Performance Implication:* If the limit is not strictly enforced, the agent will burn massive amounts of tokens and compute cycles. The current loop checks `if i >= e.config.MaxToolCalls`, but this only limits calls *within a single LLM response*. If the LLM returns 1 call per response over 100 turns, the limit is bypassed.
- *Gap:* The test suite does not verify that multi-turn execution halts appropriately. A test must mock an LLM that recursively calls tools to ensure the `MaxTurns` (defined in `SubAgentConfig`) is respected by the overarching run loop.

**Vector 2: Massive Context History Exhaustion**
- *Scenario:* A user engages in an extremely long campaign, generating tens of thousands of conversation turns.
- *System Behavior:* `appendToHistory` appends turns to `e.conversationHistory`. It currently has logic: `if len(e.conversationHistory) > maxHistory { ... }`.
- *Performance Implication:* Because `renderedCache` in the UI uses a slice for O(1) performance, continuous appending without semantic compression will eventually hit memory limits or cause the UI to stutter during full re-renders. Furthermore, passing an uncompressed 10k turn history to the transducer will cause massive latency and likely token limit exhaustion in the LLM.
- *Gap:* There are no load tests pushing 10,000+ mock turns into `appendToHistory` to verify that the slice truncation (or potential semantic compression handoff) occurs without index out-of-bounds panics.

**Vector 3: Tool Execution Timeouts (`executor.executeToolCall`)**
- *Scenario:* A tool (e.g., a web scraper or a complex build command) hangs indefinitely and ignores the `ctx.Done()` signal.
- *System Behavior:* `executeToolCall` wraps the context with `e.config.ToolTimeout` (default 5 mins). However, if the underlying modular tool is poorly written and blocks on a non-select channel or synchronous I/O without checking `ctx`, the goroutine will leak.
- *Performance Implication:* Over time, spawned ephemeral agents that hit blocking tools will accumulate, eventually hitting the `MaxActiveSubagents` limit in the Spawner, paralyzing the system (Denial of Service).
- *Gap:* A test must supply a mock modular tool that purposefully blocks for longer than `ToolTimeout` to verify that the executor returns a timeout error and cleans up its state, even if the tool itself leaks.

### 2.4 State Conflicts

**Vector 1: Race Conditions in Spawner Capacity Check (`spawner.Spawn`)**
- *Scenario:* Two concurrent requests attempt to spawn an agent when `activeCount == maxActiveSubagents - 1`.
- *System Behavior:* The `Spawn` method implements a "Phase 1" check with a brief lock, then releases the lock to perform expensive JIT compilation (Phase 2), and then acquires the lock again (Phase 5) to register the agent.
- *Performance Implication:* This is a classic Time-of-Check to Time-of-Use (TOCTOU) vulnerability. While the code *does* re-check capacity in Phase 5 (`if activeCount >= s.maxActiveSubagents`), the expensive JIT compilation (Phase 2) has already been performed unnecessarily by the second request. Under heavy load, this causes massive CPU/API spike contention.
- *Gap:* There is no concurrent benchmark or test verifying the TOCTOU mitigation. A test should use `sync.WaitGroup` to launch 100 concurrent spawn requests against a spawner with a max limit of 10 to ensure exactly 10 succeed and the rest fail gracefully without state corruption.

**Vector 2: Fail Closed Vulnerability (`executor.checkSafety`)**
- *Scenario:* The `kernel` fails to initialize properly, leaving `e.kernel == nil`. The configuration `EnableSafetyGate` is set to `true`.
- *System Behavior:* The current logic states:
  ```go
  if e.kernel == nil {
      if e.config.EnableSafetyGate {
          // Log error
          return false
      }
      return true
  }
  ```
  This is the correct "Fail Closed" behavior. However, prior to December 2024, legacy systems sometimes returned `true` here, resulting in "God Mode" agents.
- *Performance Implication:* Denying all actions is correct for security, but terrible for user experience if the kernel is only temporarily unavailable.
- *Gap:* The test suite explicitly lacks a test verifying this exact logic branch. A test must instantiate an Executor with `nil` kernel and `EnableSafetyGate=true`, issue a tool call, and assert it is blocked.

**Vector 3: Mangle Updates Batching Panic (`executor.processMangleUpdatesFromEnvelope`)**
- *Scenario:* Piggyback++ protocol receives 150 `mangle_updates` in a single control packet.
- *System Behavior:* The `MangleUpdatePolicy` defines `MaxUpdates: 100`. The `FilterMangleUpdates` function truncates or blocks the excess.
- *Performance Implication:* If the underlying `AssertBatch` function in the kernel interface assumes the transaction size is unbounded, it might hit SQLite parameter limits (e.g., `SQLITE_MAX_VARIABLE_NUMBER`) and panic with `database is closed` or similar CGO errors.
- *Gap:* There is no test sending an extreme payload of `mangle_updates` to ensure the policy truncates it before it overwhelms the underlying `VirtualStore` or `Kernel` transaction scope.

---

## 3. Evaluation of System Performance vs Edge Cases

The JIT-driven architecture is highly performant in standard scenarios due to its lean `SubAgent` instantiation (avoiding the massive memory overhead of booting hardcoded domain shards). However, its performance degrades unpredictably under edge case stress:

1. **Memory Efficiency:** The `Spawner`'s map-based registry (`s.subagents`) combined with pointers ensures memory overhead per agent is minimal. However, because SubAgents are managed via Go channels and contexts, the "Forgotten Sender" anti-pattern (goroutine leaks) is a significant risk if the JIT compiler or LLM client hangs without respecting `ctx.Done()`.
2. **Execution Latency:** The TOCTOU issue in the Spawner means under sudden bursts of intent generation, latency will spike as multiple goroutines attempt expensive LLM compilations, only to be rejected at Phase 5. A semaphoring approach (e.g., using a buffered channel of size `maxActiveSubagents` to gate Phase 2) would be drastically more performant.
3. **Mangle Sync Overhead:** The safety gate (`checkSafety`) requires 2 synchronous interactions with the Mangle kernel per tool call: `Assert(pending_action)` and `Query(permitted)`. While in-memory Mangle is fast, if this is backed by SQLite, it creates a massive serialization bottleneck. 50 tool calls = 100 synchronous disk I/O operations.

---

## 4. Bridging Imperative Go Tests and Declarative Mangle Logic

To truly fortify this system, the Go test suite must be modernized to handle Mangle's declarative nature. The current mocks in `mocks_test.go` are purely imperative and often fail to catch neuro-symbolic logic bugs.

### Recommended Test Architecture Improvements:

1. **The Clean Slate Fact Store:**
   Never use a global or reused `MockKernel` across parallel tests. Mangle evaluation is monotonic. If Test A asserts `permitted(/read_file, ...)`, and Test B checks if `/delete_file` is blocked but uses the same mock instance, ghost facts will contaminate the result. Every test *must* instantiate a fresh `factstore.NewSimpleInMemoryStore()`.

2. **Type-Strict AST Assertions:**
   The `extractTarget` and Mangle coercion gaps identified above stem from "Stringly Typed" logic. Tests must stop asserting `res.String() == "p(/a)"`. Instead, they must construct explicit Mangle AST elements:
   ```go
   ast.NewAtom("current_intent") // Enforces /current_intent
   ast.NewString("current_intent") // Enforces "current_intent"
   ```
   Testing the boundary between these two types will catch 90% of the silent join failures in the intent routing logic.

3. **Termination Verification for Tool Loops:**
   Because the executor drives a recursive action loop, tests must enforce termination. Use `context.WithTimeout` aggressively in tests, not just to prevent CI hangs, but to explicitly prove that an adversarial LLM input (e.g., recursive tool calls) causes the `Executor` to reach a fixpoint or halt predictably, proving the logic is safe and finite.

4. **Piggyback Injection Assertions:**
   The `generateResponseWithPiggybackTools` function injects a massive JSON catalog into the system prompt. Tests must verify that this injection does not push the total prompt size over the `TokenBudgetManager`'s allowed limit, which would cause an immediate 400 Bad Request error from the upstream LLM provider.

---
[End of Journal]

## 5. Detailed Test Plan and Implementation Strategy

To effectively close the identified gaps, the engineering team must implement a structured testing campaign across both `executor_process_test.go` and `spawner_test.go`. The following sections outline the specific test cases required, including pseudocode and assertions necessary to prove the robustness of the system against each edge case vector.

### 5.1 Implementing Null / Undefined / Empty Input Tests

**Test Case 1: Graceful Degradation on Empty Input**
- *File:* `executor_process_test.go`
- *Objective:* Prove that `Process(ctx, "")` and `Process(ctx, "   ")` return a swift, default response without triggering backend LLM calls or panicking the transducer.
- *Implementation Details:*
  1. Instantiate the `Executor` with a `MockTransducer` configured to return an empty intent (e.g., `Intent{Verb: "", Category: ""}`) when receiving empty input.
  2. The `MockLLMClient` should be configured to panic or fail the test if invoked.
  3. Assert that the `ExecutionResult.Response` contains a default, helpful message (e.g., "I didn't catch that.") or that the `Error` field gracefully handles the empty intent.
  4. Assert `ToolCallsExecuted == 0`.

**Test Case 2: Nil Compilation Context Handling**
- *File:* `executor_process_test.go`
- *Objective:* Ensure `buildCompilationContext` and `JITCompiler.Compile` do not panic when the intent is malformed or lacks critical fields.
- *Implementation Details:*
  1. Inject an `Intent` with `Target: nil` and `Verb: ""`.
  2. Verify the returned `CompilationContext` defaults to safe values (e.g., `IntentVerb: "/general"`, `OperationalMode: "/active"`).
  3. Ensure the mock `JITCompiler` returns the baseline prompt without error.

**Test Case 3: Rejection of Nil Tool Arguments**
- *File:* `executor_process_test.go`
- *Objective:* Guarantee that `executeToolCall` safely rejects a tool call with a `nil` argument map, preventing a downstream panic in the tool registry.
- *Implementation Details:*
  1. Construct a `types.LLMToolResponse` containing a `ToolCall` where `Input: nil`.
  2. Mock the `LLMClient` to return this response.
  3. Assert that `executor.Process` logs the failure but does not crash the goroutine.
  4. Verify the `Error` field of the `ExecutionResult` or the specific tool execution error indicates "invalid arguments" or similar.

### 5.2 Implementing Type Coercion Tests

**Test Case 4: Mangle Argument Resolution (String vs. Atom)**
- *File:* `executor_test.go` (or wherever `parseMangleArg` is tested)
- *Objective:* Verify `parseMangleArg` accurately distinguishes between file paths (`/path/to/file`) and Mangle Atoms (`/active`).
- *Implementation Details:*
  1. Test inputs: `"/etc/passwd"`, `"/active"`, `"\"/quoted/path\""`, `"regular_string"`.
  2. Assert that `"/etc/passwd"` returns the literal string `"/etc/passwd"` (or requires explicit quoting depending on the intended design, though the current heuristic assumes leading `/` is an atom. *Note: If the heuristic is flawed, this test should drive a fix to the heuristic itself.*).
  3. Assert that `"\"/quoted/path\""` returns the string `"/quoted/path"`.
  4. Assert that `"/active"` returns `types.MangleAtom("/active")`.

**Test Case 5: Target Extraction Robustness**
- *File:* `executor_test.go`
- *Objective:* Ensure `extractTarget` does not panic when presented with complex, nested JSON structures for target keys (e.g., `"path": {"nested": "value"}`).
- *Implementation Details:*
  1. Pass a map containing `"path": map[string]interface{}{"nested": "value"}`.
  2. Verify that `types.ExtractString` safely stringifies the object or returns a fallback like `"unknown"`, preventing the Constitutional Gate from panicking during assertion.

### 5.3 Implementing User Request Extremes Tests

**Test Case 6: Halting Infinite Tool Loops**
- *File:* `executor_process_test.go`
- *Objective:* Prove that the `Executor` strictly enforces `MaxToolCalls`, even across multiple simulated LLM turns if the loop logic is ever modified to handle multi-turn execution internally.
- *Implementation Details:*
  1. Configure `ExecutorConfig.MaxToolCalls = 3`.
  2. Mock the `LLMClient` to continually return a response with 5 tool calls.
  3. Assert that the loop breaks exactly after 3 tool executions.
  4. Verify that the execution duration is reasonable and no goroutines leak.

**Test Case 7: Context History Exhaustion Mitigation**
- *File:* `executor_process_test.go`
- *Objective:* Ensure `appendToHistory` correctly truncates the history slice without index-out-of-bounds errors when adding the 10,001st turn.
- *Implementation Details:*
  1. Pre-populate `e.conversationHistory` with 10,000 dummy turns.
  2. Call `Process` with a new input.
  3. Assert that the length of `e.conversationHistory` remains at or below `maxHistory` (e.g., 100 or whatever the configured limit is).
  4. Verify the oldest turns were correctly evicted.

**Test Case 8: Tool Execution Timeout Enforcement**
- *File:* `executor_process_test.go`
- *Objective:* Verify that a malfunctioning, blocking tool does not hang the `Process` loop indefinitely.
- *Implementation Details:*
  1. Register a mock tool in `tools.Global()` that sleeps for 10 seconds, ignoring `ctx.Done()`.
  2. Configure `ExecutorConfig.ToolTimeout = 100 * time.Millisecond`.
  3. Mock the LLM to call this tool.
  4. Assert that `Process` returns within ~100ms with a timeout error for that specific tool call, allowing the agent to continue or report the failure.

### 5.4 Implementing State Conflict Tests

**Test Case 9: Spawner TOCTOU Mitigation Verification**
- *File:* `spawner_test.go`
- *Objective:* Prove that concurrent spawn requests accurately respect `maxActiveSubagents` without executing expensive Phase 2 compilation unnecessarily.
- *Implementation Details:*
  1. Configure `SpawnerConfig.MaxActiveSubagents = 5`.
  2. Use a `sync.WaitGroup` to launch 50 concurrent `SpawnForIntent` requests.
  3. The mock `JITCompiler` should increment an atomic counter each time it is called.
  4. Assert that exactly 5 agents are returned successfully.
  5. *Crucially:* Assert that the `JITCompiler` atomic counter is ideally close to 5 (or minimally higher), proving the lock strategy prevents massive redundant compilation.

**Test Case 10: Fail Closed on Nil Kernel**
- *File:* `executor_process_test.go`
- *Objective:* Explicitly verify the "Fail Closed" security posture when the kernel is unavailable but safety is mandated.
- *Implementation Details:*
  1. Instantiate the `Executor` with `kernel: nil` and `config.EnableSafetyGate = true`.
  2. Mock the LLM to call a sensitive tool (e.g., `delete_file`).
  3. Assert that `executeToolCall` returns an error indicating the action was blocked by the safety gate.
  4. Verify the tool was *not* executed.

**Test Case 11: Mangle Updates Batch Truncation**
- *File:* `executor_process_test.go`
- *Objective:* Ensure the Piggyback protocol safely truncates massive `mangle_updates` arrays to prevent database transaction limits.
- *Implementation Details:*
  1. Construct a `PiggybackEnvelope` containing 200 valid `mangle_updates` (e.g., `observation(...)`).
  2. Pass it to `processMangleUpdatesFromEnvelope`.
  3. Assert that the `MockKernel` receives exactly `MaxUpdates` (e.g., 100) assertions, and the rest are safely discarded or blocked.

---

## 6. Strategic Recommendations for the QA Automation Pipeline

To prevent regressions of this magnitude in the future, the QA pipeline should incorporate the following strategies:

1. **Fuzz Testing the Transducer:** The transducer is the entry point for all natural language. Implement Go's native fuzzing (`testing.F`) to bombard the `ParseIntentWithContext` function with random byte slices, massive strings, and malformed unicode to ensure it never panics.
2. **Chaos Monkey for Tool Registries:** Periodically inject failures into the modular and Ouroboros tool registries during end-to-end integration tests. Tools should randomly timeout, return garbled JSON, or panic. The `Executor` must gracefully handle all these scenarios, encapsulating the failure and reporting it back to the LLM for self-correction (the TDD Repair Loop).
3. **Mangle Static Analysis:** Integrate the `analysis.Analyze(program)` step into the CI pipeline for all `.mg` files. This ensures that any new logic rules added to the intent routing or constitutional policies are stratified and safe *before* they are loaded into the kernel, preventing runtime evaluation errors.

By rigorously implementing these boundary tests and adopting a defensive testing posture against the non-deterministic nature of LLMs, the Session Clean Loop will provide a stable, resilient foundation for all codeNERD cognitive processes.

[End of Extended Journal]

## 7. Extended Analysis: Edge Cases in SubAgent Lifecycle Management

The `SubAgent` orchestrates the execution loop for specific intents. While `executor.go` handles the single-turn loop, `subagent.go` manages state transitions (Idle -> Running -> Completed/Failed). This lifecycle introduces another layer of complexity that requires rigorous testing, especially around concurrency and state manipulation.

### 7.1 SubAgent State Transitions Under Load

**Vector 1: Concurrent Stop Requests (`subagent.Stop`)**
- *Scenario:* The UI or Spawner calls `Stop()` on a single `SubAgent` multiple times concurrently (e.g., rapid user clicks on a "Cancel" button).
- *System Behavior:* `Stop()` should cancel the subagent's internal context, signaling the running goroutine to halt. If the state transition is not atomic or correctly locked, it could lead to multiple context cancellations or race conditions updating the state to `SubAgentStateFailed` or `SubAgentStateCompleted`.
- *Performance Implication:* Generally low, but race conditions could leave the subagent in an inconsistent state, preventing cleanup by the Spawner.
- *Gap:* Tests must verify that `Stop()` is idempotent and safe to call concurrently from multiple goroutines.

**Vector 2: MaxTurns Enforcement Loop**
- *Scenario:* A complex task causes the LLM to engage in a prolonged back-and-forth, repeatedly calling tools that fail or provide insufficient information. The subagent reaches `MaxTurns` (e.g., 100).
- *System Behavior:* The `Run` loop in `subagent.go` should check `turnCount >= cfg.MaxTurns` and terminate the loop, transitioning to `SubAgentStateCompleted` or `SubAgentStateFailed`.
- *Performance Implication:* If `MaxTurns` is not enforced correctly, the subagent could run indefinitely, consuming resources and potentially hanging the Spawner if `maxActiveSubagents` is reached.
- *Gap:* The test suite must simulate an LLM that perpetually returns tool calls without arriving at a final answer, verifying that the subagent halts exactly at `MaxTurns`.

### 7.2 Memory Compression and Context Paging

**Vector 3: Memory Compressor Failure (`subagent.Compressor`)**
- *Scenario:* A `Persistent` subagent reaches its memory limit and triggers the `Compressor` to summarize its history. The compression LLM call fails (e.g., API timeout or rate limit).
- *System Behavior:* The subagent needs to handle this gracefully. Should it drop the oldest turns? Should it pause execution and retry? If it continues without compressing, the context window will eventually explode.
- *Performance Implication:* A failed compression step followed by continued execution will rapidly degrade performance and increase API costs.
- *Gap:* A test must inject a failing `Compressor` mock and verify the subagent's fallback behavior (e.g., truncating history as a last resort or returning an error).

### 7.3 Integration with the Spawner's Cleanup Loop

**Vector 4: Zombie SubAgents (`spawner.Cleanup`)**
- *Scenario:* The `Cleanup` method is responsible for removing `Completed` or `Failed` subagents from the `s.subagents` map. If a subagent's goroutine panics unexpectedly without updating its state, it becomes a "zombie."
- *System Behavior:* The `Cleanup` loop will ignore the zombie subagent because its state is still `Running`.
- *Performance Implication:* Over time, zombie subagents will accumulate, eventually filling the `maxActiveSubagents` quota and permanently paralyzing the system.
- *Gap:* Tests must ensure that the `Run` method in `subagent.go` uses `defer` to guarantee a state transition to `Failed` in the event of a panic. A test should mock a tool that panics and verify the Spawner correctly cleans up the resulting failed subagent.

## 8. Conclusion and Next Steps

The JIT-driven architecture of the Session package is a massive leap forward in maintainability and flexibility compared to the legacy domain shards. However, the consolidation of complexity into the `Executor` and `Spawner` means these components must be incredibly robust.

The boundary value analysis above reveals critical gaps in handling null inputs, type coercion around Mangle boundaries, user extremes (especially runaway LLM loops), and concurrent state conflicts.

The immediate next steps are to implement the test cases outlined in sections 5 and 7, starting with the highest-risk areas:

1.  **Fail-Closed Verification:** Ensure the Constitutional Gate fails securely when the kernel is unavailable.
2.  **Infinite Loop Prevention:** Verify `MaxToolCalls` and `MaxTurns` are strictly enforced to prevent resource exhaustion.
3.  **TOCTOU Mitigation:** Implement concurrent spawn testing to guarantee capacity limits are respected without massive performance penalties.

By systematically addressing these gaps, the codeNERD team can ensure the Universal Execution Loop remains resilient, efficient, and safe under all operating conditions.

## 9. Appendix: Traceability Matrix

The following table maps the identified gaps to the specific files and functions that need testing.

| Gap Area | File Under Test | Function Under Test | Vector | Priority |
|---|---|---|---|---|
| Null/Empty Inputs | `executor.go` | `Process` | Empty User Input | High |
| Null/Empty Inputs | `executor.go` | `buildCompilationContext` | Nil Target/Verb | Medium |
| Null/Empty Inputs | `executor.go` | `executeToolCall` | Nil Tool Args | High |
| Type Coercion | `executor.go` | `parseMangleArg` | Path vs Atom | Critical |
| Type Coercion | `executor.go` | `extractTarget` | Nested JSON Target | Medium |
| User Extremes | `executor.go` | `Process` | Infinite Tool Loop | Critical |
| User Extremes | `executor.go` | `appendToHistory` | Context Exhaustion | High |
| User Extremes | `executor.go` | `executeToolCall` | Tool Timeout Leak | High |
| State Conflicts | `spawner.go` | `Spawn` | TOCTOU Limit Check | Critical |
| State Conflicts | `executor.go` | `checkSafety` | Fail Closed (Nil Kernel) | Critical |
| State Conflicts | `executor.go` | `processMangleUpdates` | Update Batch Truncation | Medium |
| SubAgent Lifecycle | `subagent.go` | `Stop` | Concurrent Stop Calls | Medium |
| SubAgent Lifecycle | `subagent.go` | `Run` | MaxTurns Enforcement | High |
| SubAgent Lifecycle | `subagent.go` | `Compressor` | Compressor Failure Fallback | Medium |
| SubAgent Lifecycle | `subagent.go` | `Run` / `spawner.Cleanup` | Panic Recovery (Zombie Agents) | Critical |

## 10. Architectural Reflection on JIT vs Shards

While the primary focus of this analysis is identifying testing gaps, it is worth reflecting on the architectural shift from static Shards (e.g., Coder, Tester) to the dynamic JIT compiler.

The legacy Shard system provided explicit isolation at the cost of massive code duplication. A Coder shard could only run coding tools. The JIT system unifies the execution path (`executor.go`), meaning a bug here affects *all* capabilities.

This single point of failure makes the edge cases identified in this document even more critical. If a malformed Mangle update (Type Coercion Vector 1) crashes the executor, the agent loses its ability to think, act, and repair itself simultaneously. In the old system, a crash in the Tester shard might still leave the Coder intact to generate a fix.

Therefore, the tests implemented based on this analysis must be treated as the foundation of codeNERD's stability.


## 11. Final Thoughts on "Ghost Facts" and Test Dissonance

As highlighted in the codebase context ("Mangle as HashMap" Anti-Pattern), Go strings and Mangle Atoms represent disjoint types. `ast.Name("active")` corresponds to `/active`, while `ast.String("active")` corresponds to `"active"`.

This distinction is frequently lost in imperative Go testing frameworks where strings are the universal currency. When writing the tests proposed in Section 5 (especially Test Case 4: Mangle Argument Resolution), developers must forcefully reject "stringly typed" assertions.

If a test converts a Mangle Datalog set result to a string (`res.String() == "p(/a)"`), it introduces a brittle dependency on the unordered nature of sets. The correct approach is to assert set membership using `store.Read(...)`.

This means that while the `executor.Process` function is a Go function returning a Go struct, its internal logic relies on the Mangle Engine. To write high-leverage Mangle tests in Go for this subsystem, the "Clean Slate" Fact Store (instantiating `factstore.NewSimpleInMemoryStore()` inside the test loop) is non-negotiable.

If `executor_test.go` or `spawner_test.go` reuse a global mock store, "ghost facts" from a successful test will bleed into a negative test. A test designed to verify that the Constitutional Gate blocks an action might fail (i.e., the action is incorrectly permitted) simply because a `permitted(...)` fact remained in the mock store from the previous test case.

The imperative Go tests must bridge the gap to Mangle's declarative, fixpoint-based logic by ensuring isolation, type safety (Atoms vs. Strings), and termination verification for recursive rules. Only then can the Universal Execution Loop be considered truly robust.

## 12. Checklist for QA Engineer Review

Before these tests are considered fully implemented and the gaps closed, the following criteria must be met:

*   **Test Isolation Check:** Confirm that no test in `internal/session/executor_process_test.go` or `internal/session/spawner_test.go` reuses a global `MockKernel` across parallel runs. Each test must instantiate a fresh store or cleanly retract all facts it asserts.
*   **Mangle Type Enforcement Check:** Confirm that `parseMangleArg` tests use `ast.NewAtom()` and `ast.NewString()` to explicitly verify the distinction, rather than checking if a Go string starts with a slash.
*   **Target Extraction Safety Check:** Confirm that a deeply nested JSON object (e.g., `{"key": {"nested": "value"}}`) passed as the `path` argument to `executeToolCall` does not crash the safety gate's JSON unmarshaling or `types.ExtractString` functions.
*   **Timeout & Cancellation Check:** Confirm that the test suite includes a "hanging tool" mock and verifies that `executeToolCall` times out gracefully without leaking the underlying goroutine in the Spawner's subagent map.
*   **Fail-Closed Validation:** Confirm the explicit test exists for `kernel == nil` and `config.EnableSafetyGate == true`, asserting that the tool is blocked and not executed.
*   **Infinite Loop Validation:** Confirm the test exists for an LLM that perpetually returns tool calls without answering the user, verifying that `MaxToolCalls` (executor) and `MaxTurns` (subagent) are enforced.
*   **TOCTOU Mitigation Validation:** Confirm the concurrent spawn benchmark/test uses a waitgroup to spawn 100 agents against a limit of 10, asserting the JIT compilation phase was not executed 100 times.

By rigorously following this checklist and implementing the test cases detailed in sections 5 and 7, codeNERD will solidify its JIT-driven architecture against edge cases, maximizing system stability and preventing regressions.

## 13. Additional Considerations for Future Enhancements

The current implementation of the Session Clean Loop focuses on essential OODA loop functionality. However, there are several architectural improvements that could mitigate some of the identified edge cases entirely.

*   **Replacing the Synchronous Mangle Safety Check:** The `checkSafety` function relies on synchronous Mangle engine interactions (asserting `pending_action` and querying `permitted`). This could be optimized using a more specialized local constraint solver or pre-compiled rule sets.
*   **Improving `parseMangleArg` Heuristics:** The current logic assumes strings starting with `/` are Mangle Atoms. While true in most codeNERD domain schemas, it introduces the type coercion vulnerabilities described earlier (e.g., absolute file paths). A more robust parser that distinguishes based on the expected predicate schema would be beneficial.
*   **Semantic Compression Thresholds:** The `appendToHistory` logic currently truncates strictly on length. Implementing a heuristic threshold for semantic compression based on token count or context relevance would be more memory efficient than simple truncation.

These considerations highlight that robust testing often reveals opportunities for fundamental architectural improvements beyond simply patching bugs.

[Final End of Document]

## 14. Addendum: Performance Implications of the Clean Loop

The JIT-driven architecture is a massive architectural shift for codeNERD, fundamentally altering the performance characteristics of the cognitive cycle.

The previous Shard-based system suffered from high memory overhead and startup latency due to the need to pre-allocate domain-specific shards (Coder, Tester, Reviewer) and their associated dependencies. The new Universal Execution Loop significantly mitigates these issues by dynamically compiling the necessary capabilities just-in-time (`JITCompiler`).

However, this transition introduces new performance bottlenecks:

1.  **JIT Compilation Overhead:** Every user interaction now triggers a `JITCompiler.Compile` call. This process must be extremely fast and efficient, as it occurs *before* the LLM begins generating a response. If the JIT compilation fails or times out, the system defaults to a baseline prompt, severely degrading the quality of the LLM's output. The performance of this step is critical.
2.  **Config Factory Overhead:** Similar to JIT compilation, the `ConfigFactory.Generate` step happens on every turn. This dynamically provisions the `AgentConfig` (allowed tools, policies). This step must also be highly performant to avoid adding noticeable latency to the overall response time.
3.  **Mangle Safety Checks:** As mentioned previously, the `checkSafety` function requires synchronous interactions with the Mangle kernel (asserting `pending_action` and querying `permitted`). This adds latency to every tool execution. If the Mangle kernel is backed by a slow storage mechanism (e.g., SQLite on a slow disk), this will become a significant bottleneck.

To ensure the JIT-driven architecture remains performant, rigorous benchmarking is necessary. The testing strategy should include benchmarks for `JITCompiler.Compile`, `ConfigFactory.Generate`, and `checkSafety` to ensure these critical path functions meet stringent latency requirements.

This concludes the comprehensive boundary analysis and testing strategy for the Session Clean Loop. By addressing these gaps and considerations, the codeNERD team can ensure the JIT-driven architecture is not only flexible and maintainable but also robust, secure, and performant.
