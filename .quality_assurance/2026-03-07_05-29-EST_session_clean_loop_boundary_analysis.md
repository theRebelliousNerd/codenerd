# Session Clean Loop Subsystem Boundary Value & Negative Testing Analysis

**Date:** 2026-03-07 05:29 EST
**Subsystem Analyzed:** `internal/session` (`Executor` and `Spawner`)

## 1. Executive Summary

This journal entry details a deep dive into the `internal/session` package, specifically focusing on the `Executor` and `Spawner` components that form the core of the new JIT (Just-In-Time) clean loop architecture in codeNERD. The goal of this analysis is to identify edge cases, boundary conditions, and negative testing scenarios that are currently missing from the test suite (`executor_process_test.go`, `executor_test.go`, `spawner_test.go`, and `spawner_config_test.go`).

The clean loop replaces the legacy shard-based architecture, unifying execution into a single, JIT-configured pathway. Because this is the central nervous system of codeNERD, its robustness against extreme inputs, malformed state, and concurrency pressures is paramount.

We evaluated the system against four primary vectors:
1.  **Null/Undefined/Empty**
2.  **Type Coercion**
3.  **User Request Extremes**
4.  **State Conflicts**

## 2. Analysis of `Spawner` (`internal/session/spawner.go`)

The `Spawner` is responsible for dynamically creating isolated subagents. It relies on JIT compilation to determine the agent's identity, tools, and policies.

### 2.1 Null/Undefined/Empty Vectors

*   **Empty SpawnRequest Name:**
    *   **Scenario:** `Spawner.Spawn()` is called with an empty `req.Name`.
    *   **Current Behavior:** The spawner interpolates the name into an ID using `fmt.Sprintf("%s-%d", req.Name, time.Now().UnixNano())`. An empty name results in an ID like `-1678123456789`. While not inherently crashing the system, it leads to semantically meaningless log entries and could break downstream systems relying on named agents.
    *   **Improvement:** The test suite should verify that the system either rejects empty names or falls back to a deterministic default (e.g., `unnamed-agent`). A `TODO: TEST_GAP` has been added.
*   **Empty IntentVerb in SpawnRequest:**
    *   **Scenario:** `req.IntentVerb` is empty.
    *   **Current Behavior:** Handled correctly via a fallback: `if intentVerb == "" { intentVerb = "/general" }`.
    *   **Improvement:** While handled, the test suite currently lacks explicit coverage for this fallback path. We should assert that an empty intent verb correctly routes to the `/general` baseline configuration.
*   **Empty task in SpawnSpecialist:**
    *   **Scenario:** `SpawnSpecialist` is called with an empty task string.
    *   **Current Behavior:** The empty task string is passed to `agent.Run(ctx, task)`. The subagent loop will process an empty prompt.
    *   **Improvement:** We need to verify how `SubAgent` handles an empty initial prompt. Does it wait for input? Does it immediately terminate? Does it waste LLM tokens querying an empty prompt?
*   **Empty name in SpawnSpecialist (File Path Injection Risk):**
    *   **Scenario:** `name` is empty in `SpawnSpecialist(ctx, name, task)`.
    *   **Current Behavior:** The config path is built using `filepath.Join(".nerd", "agents", name, "config.yaml")`. An empty name evaluates to `.nerd/agents/config.yaml`.
    *   **Improvement:** A malicious or malformed name like `../../../etc/passwd` could lead to unintended file reads (path traversal). The test suite must verify that `SpawnSpecialist` sanitizes the `name` argument before joining it to a file path. A `TODO: TEST_GAP` has been added.

### 2.2 Type Coercion Vectors

*   **Invalid SubAgentType:**
    *   **Scenario:** A client passes an undefined `SubAgentType` integer to `SpawnRequest.Type`.
    *   **Current Behavior:** Go's strong typing prevents arbitrary types, but custom integer types can hold unmapped values.
    *   **Improvement:** Verify how the system logs or routes unknown `SubAgentType` values. Does it default to `SubAgentTypeEphemeral`?

### 2.3 User Request Extremes

*   **Massive Number of Concurrent Spawns:**
    *   **Scenario:** 10,000 parallel goroutines call `Spawner.Spawn()` simultaneously.
    *   **Current Behavior:** The `Spawner` enforces `maxActiveSubagents` via a two-phase lock check (`s.mu.Lock()`, check count, unlock, generate config, lock, check count again). This is a well-designed mitigation for Time-of-Check to Time-of-Use (TOCTOU) race conditions during the slow `generateConfig` phase.
    *   **Improvement:** We must write a high-concurrency test to explicitly verify this TOCTOU mitigation. The test should attempt to spawn agents slightly over the limit and ensure exactly `maxActiveSubagents` are spawned without panicking. A `TODO: TEST_GAP` has been added.
*   **Timeout Extremes (Negative or Massive):**
    *   **Scenario:** `req.Timeout` is set to `-1` or `1000000 * time.Hour`.
    *   **Current Behavior:** `if subCfg.Timeout == 0` defaults to 30 minutes. Negative timeouts are passed directly to the `SubAgent`. A negative timeout to a context will cause immediate cancellation.
    *   **Improvement:** The tests must verify that negative timeouts correctly and gracefully terminate the subagent immediately upon creation, rather than causing a panic loop. A massive timeout should not cause integer overflows in internal timers. A `TODO: TEST_GAP` has been added.
*   **Extreme JIT Compilation Delays:**
    *   **Scenario:** `s.jitCompiler.Compile` takes 29 minutes to complete.
    *   **Current Behavior:** The `Spawn` request blocks.
    *   **Improvement:** We need tests verifying that `ctx` cancellation during a long JIT compilation aborts the spawn process cleanly without leaking goroutines or partial agent state in the `subagents` map.

### 2.4 State Conflicts

*   **Concurrent StopAll() and Cleanup():**
    *   **Scenario:** `StopAll()` iterates over active agents, while `Cleanup()` iterates to delete them.
    *   **Current Behavior:** Both methods acquire `s.mu.Lock()` appropriately. However, `StopAll` copies the map to a slice before stopping to avoid holding the lock during stop operations.
    *   **Improvement:** Verify the interaction between `StopAll` and a concurrent `Spawn`. Does `StopAll` guarantee no new agents can spawn while it runs, or can a new agent slip in right after the map copy? A `TODO: TEST_GAP` has been added.
*   **GetByName Non-Determinism:**
    *   **Scenario:** Multiple ephemeral subagents are spawned with the same generic name (e.g., "coder"). A client calls `GetByName("coder")`.
    *   **Current Behavior:** The function iterates over the `s.subagents` map. Go map iteration is explicitly randomized. Therefore, `GetByName` will return a random active agent with that name.
    *   **Improvement:** This non-determinism can lead to flaky system behavior if clients expect to target a specific instance by name. The tests should verify this behavior, and architectural consideration should be given to whether `GetByName` should return a slice of agents or whether names must be strictly unique. A `TODO: TEST_GAP` has been added.
*   **JIT Fallback State:**
    *   **Scenario:** Both the primary JIT compilation AND the fallback baseline compilation fail in `generateConfig`.
    *   **Current Behavior:** It returns an empty `&config.AgentConfig{}, nil`.
    *   **Improvement:** The test suite must ensure the `SubAgent` can safely boot and execute (even if severely degraded) with a completely empty configuration struct. A `TODO: TEST_GAP` has been added.

## 3. Analysis of `Executor` (`internal/session/executor.go`)

The `Executor` is the heart of the clean loop. It processes a user's natural language input, invokes the JIT compiler, calls the LLM, and routes tool executions through the Virtual Store.

### 3.1 Null/Undefined/Empty Vectors

*   **Empty User Input:**
    *   **Scenario:** `Process(ctx, "")` is called.
    *   **Current Behavior:** Passed to transducer -> LLM.
    *   **Improvement:** The test suite must verify that an empty string does not cause out-of-bounds panics in prompt assembly or cause the LLM client to hang indefinitely. It should ideally result in a quick, sensible default response. A `TODO: TEST_GAP` was identified in previous test analysis.
*   **Nil Dependencies in Constructor:**
    *   **Scenario:** `NewExecutor` is called with a nil `JITCompiler`.
    *   **Current Behavior:** `Process` calls `e.jitCompiler.Compile(...)` directly. This will trigger a nil pointer panic.
    *   **Improvement:** The constructor should ideally validate dependencies, or the `Process` method must safely check for nil interfaces before invocation. A test must verify this panic or graceful degradation. A `TODO: TEST_GAP` has been added.
*   **Nil Tool Arguments (`call.Args`):**
    *   **Scenario:** The LLM hallucinates a tool call and omits the `input` schema entirely, resulting in a nil `map[string]interface{}`.
    *   **Current Behavior:** `executeToolCall` passes the nil map to `extractTarget` and `json.Marshal`. `json.Marshal(nil)` returns `"null", nil`. `extractTarget` loops over a nil map (which is a no-op in Go) and returns `"unknown"`.
    *   **Improvement:** The code appears resilient to this, but it relies on implicit Go behaviors. A test must explicitly pass a tool call with nil args to guarantee no regressions introduce a nil pointer dereference. A `TODO: TEST_GAP` has been added.

### 3.2 Type Coercion Vectors

*   **Un-marshalable Tool Arguments:**
    *   **Scenario:** `call.Args` contains a Go type that cannot be marshaled to JSON (e.g., a `chan int` or a function pointer), perhaps injected by a malformed modular tool.
    *   **Current Behavior:** `json.Marshal(call.Args)` inside `checkSafety` will return an error. The safety gate fails closed (`return false`).
    *   **Improvement:** This behavior is correct (fail closed), and a test for this (`TestExecutor_ArgsMarshalFailure`) was confirmed to exist.
*   **Mangle Argument Parsing Edge Cases:**
    *   **Scenario:** `parseMangleArgs` receives malformed quoted strings (e.g., `"unclosed quote, test`).
    *   **Current Behavior:** The manual rune parser will treat it as a single string.
    *   **Improvement:** Complex string parsing should be backed by fuzz testing to ensure catastrophic backtracking or panics do not occur.

### 3.3 User Request Extremes

*   **Extreme Number of Tool Calls (LLM Hallucination loop):**
    *   **Scenario:** A confused LLM enters an infinite loop of calling tools without yielding a final response.
    *   **Current Behavior:** The `Executor` has a `MaxToolCalls` config (default 50). The loop in `Process` explicitly checks `if i >= e.config.MaxToolCalls { break }`.
    *   **Improvement:** A test must explicitly verify that execution halts exactly at the boundary of `MaxToolCalls`, and that the intermediate results are properly surfaced or logged. A `TODO: TEST_GAP` was noted.
*   **Tool Execution Timeout:**
    *   **Scenario:** A modular tool (e.g., a bash command) hangs indefinitely.
    *   **Current Behavior:** `executeToolCall` wraps the context with a timeout: `context.WithTimeout(ctx, e.config.ToolTimeout)`.
    *   **Improvement:** The test suite must simulate a hanging tool and verify that the context timeout correctly aborts the execution and that `Process` continues gracefully (or returns the timeout error to the user). A `TODO: TEST_GAP` was noted.
*   **Extremely Large JIT Prompt Compilation:**
    *   **Scenario:** A brownfield monorepo requests triggers a massive inclusion of context, resulting in a 500k token prompt.
    *   **Current Behavior:** `TokenBudget` is hardcoded to `8192` in `buildCompilationContext`.
    *   **Improvement:** The test suite needs to verify how the executor responds when the JIT compiler exceeds the configured token budget.

### 3.4 State Conflicts

*   **Concurrent Process() Executions:**
    *   **Scenario:** A user rapidly mashes "send" in the CLI, causing multiple concurrent calls to `Executor.Process()` using the same session instance.
    *   **Current Behavior:** `Process` mutates the `conversationHistory` slice. It uses a write lock (`e.mu.Lock()`) during `appendToHistory`.
    *   **Improvement:** While `appendToHistory` is locked, the actual LLM generation and tool execution are outside the lock. The ordering of appended history turns might become interleaved randomly between the two concurrent requests. Tests must verify the determinism and safety of concurrent history modifications. A `TODO: TEST_GAP` has been added.
*   **Constitutional Gate Racing:**
    *   **Scenario:** A tool execution is blocked by the safety gate. The `pending_action` fact is asserted, but before it can be retracted in the `defer`, the kernel is reset or another thread executes conflicting Mangle logic.
    *   **Current Behavior:** The `defer` block catches the error and logs a warning: `"Failed to retract pending_action"`.
    *   **Improvement:** A test (`TestExecutor_RetractFactFailure`) verifies that the system does not crash when retraction fails, which is excellent.

## 4. Performance Implications

Is the system performant enough to handle these edge cases?

**Memory / CPU:**
*   The transition from the legacy `ShardManager` to the JIT `Spawner` significantly reduces baseline memory usage. Instead of pre-allocating heavy domain shards, subagents are ephemeral and garbage collected.
*   `Spawner.Cleanup()` operates in `O(N)` time relative to active agents, which is highly performant given `MaxActiveSubagents` is typically small (default 10).
*   However, the `Executor` caches `conversationHistory` up to 50 turns. For very large contexts, deep copies in `GetHistory()` could become a minor allocation bottleneck under high concurrency.

**Concurrency:**
*   The use of `sync.RWMutex` in `Executor` and `Spawner` is generally well-scoped (brief locks to check counts or update maps). The heavy operations (LLM generation, JIT compilation) occur outside the lock, preventing head-of-line blocking.
*   The TOCTOU double-check pattern in `Spawner.Spawn` is an excellent architectural choice for high-concurrency environments, ensuring limits are respected without locking the entire generation pipeline.

## 5. Conclusion

The `internal/session` clean loop subsystem is well-architected with defensive programming patterns (timeouts, safety gates, lock scoping). However, explicit boundary value analysis reveals critical gaps in test coverage, particularly around concurrent state mutations, missing dependencies (nil pointers), path traversal vulnerabilities in config loading, and extreme timeout/limit boundary enforcement. Resolving the identified `TEST_GAP` items will elevate the system's reliability to the high-assurance standards required by codeNERD.
