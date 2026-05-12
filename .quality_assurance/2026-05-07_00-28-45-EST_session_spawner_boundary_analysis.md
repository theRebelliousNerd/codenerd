---
remediated: false
---
# Boundary Value Analysis and Negative Testing Journal
**Date/Time:** 2026-05-07 00:28:45 EST
**Module/Subsystem:** `internal/session/spawner.go` and related Spawner components.
**Author:** Jules (QA Automation Engineer)

## Overview

The `Spawner` subsystem within the `session` package is a critical component in Codenerd's neuro-symbolic architecture. Following the deprecation of domain-specific hardcoded shards (CoderShard, TesterShard, etc.) in December 2024, the `Spawner` orchestrates the Just-In-Time (JIT) creation of dynamically configured `SubAgent` instances. It acts as the gatekeeper and factory, routing user intents (extracted by the Transducer) into context-isolated, policy-bound execution loops (`SubAgent` containing an `Executor`).

This analysis evaluates the robustness of `internal/session/spawner.go` against boundary conditions, type coercion attempts, extreme user requests, and concurrent state modifications. Mangle, the declarative deductive logic engine at the heart of Codenerd, strongly influences the behavior of these dynamically spawned agents. Because agents rely on JIT configuration rules (often compiled from `intent_routing.mg` and persona atoms), a failure at the spawner level cascades into complete agent malfunction. Therefore, the spawner's resilience to invalid states, malformed configurations, and concurrency limits is paramount.

---

## 1. Null / Undefined / Empty Boundaries

The initialization, request parsing, and configuration loading phases are highly susceptible to null, empty, or undefined properties. The current test suite (`spawner_test.go`) covers the happy path and basic limits, but misses several edge cases where incomplete or blank data is passed.

### Missing Intent and Intent Properties
*   **Vector:** `SpawnRequest.IntentVerb` is an empty string `""` or `nil` (if treated as a pointer in upstream JSON payloads).
*   **Impact:** When `IntentVerb` is empty, the `generateConfig` method uses a fallback `"/general"`. However, if the `determineAgentName` or `determineAgentType` is called directly with an empty verb on a raw `perception.Intent`, it falls through to the `default` cases (returning `"executor"` and `SubAgentTypeEphemeral`).
*   **Gap:** We do not explicitly test that this fallback occurs correctly and doesn't trigger unexpected Mangle routing failures later when the subagent attempts to execute. The fallback compilation might succeed, but Mangle intent routing rules might expect a specific structured atom (e.g., `/fix`, `/review`). Passing `""` effectively bypasses specific persona constraints.
*   **Proposed Test:** Inject an intent with empty strings for `Verb`, `Category`, and `Target`. Ensure the spawner gracefully assigns default values, compiles a minimal baseline config, and does not panic or fail the spawn request.

### Empty String Injection in Names and IDs
*   **Vector:** Calling `Spawner.Spawn` with a `SpawnRequest.Name` consisting of an empty string `""` or just whitespace.
*   **Impact:** The `ID` generated would be something like `"-1683400000000000000"`. A blank or whitespace name might cause UI glitches (in the Nerd TUI) or logging confusion.
*   **Gap:** The code assumes well-formed names. There is no validation or sanitization of `req.Name`. If a caller passes `req.Name = ""`, it proceeds. Similarly, `GetByName("")` might return the first agent that accidentally got an empty name.
*   **Proposed Test:** Pass `Name: ""` and verify if it's accepted or rejected. Ideally, the system should reject invalid names or assign a generic ID (e.g., `anonymous-executor`).

### Null/Missing References in Dependencies
*   **Vector:** Although dependencies are injected via `NewSpawner`, certain dependencies might act conditionally (e.g., `s.configFactory == nil` or `s.jitCompiler == nil`). The code already handles `s.configFactory == nil` gracefully by returning an empty `&config.AgentConfig{}`.
*   **Impact:** If `s.virtualStore == nil` in `loadSpecialistConfig`, the fallback logic drops to `os.ReadFile`. This is partially safe, but what if the `SessionContext` is missing?
*   **Gap:** What if `req.SessionContext` is `nil`? The fallback code doesn't panic, but `generateConfig` assumes `req.SessionContext.DreamMode` access only after checking `req.SessionContext != nil`. What if `req.Type` is implicitly empty (a zero-value enum/string)?
*   **Proposed Test:** Construct a `SpawnRequest` with explicitly uninitialized references. Ensure no panics occur during deep property access.

### Empty or Malformed Paths in Specialist Loading
*   **Vector:** `loadSpecialistConfig` takes a `name` string to build a path: `filepath.Join(".nerd", "agents", name, "config.yaml")`.
*   **Impact:** If `name` is empty `""`, it attempts to load `.nerd/agents/config.yaml` rather than a specific directory. Worse, if `name` is `../`, it could lead to directory traversal outside the `.nerd` directory if `filepath.Join` isn't strictly cleaning bounds relative to the repository root.
*   **Gap:** The spawner implicitly trusts the `name` argument in `SpawnSpecialist`.
*   **Proposed Test:** Test `SpawnSpecialist` with `""`, `.`, `..`, and `../../../etc/passwd` to ensure the system rejects malformed specialist names before attempting file I/O.

---

## 2. Type Coercion and Invalid Enums

Go is statically typed, but interaction with external inputs (like JSON configurations, TUI commands, or Mangle Atom coercions) often leads to type boundary crossing where coercion flaws emerge.

### SubAgentType Invalid Values
*   **Vector:** `SubAgentType` is defined as a string alias (`"ephemeral"`, `"persistent"`, `"system"`). A malicious or malformed `SpawnRequest` might supply a completely invalid type, e.g., `SubAgentType("invalid_type")` or an integer coerced into a string.
*   **Impact:** The system will accept the invalid string because it doesn't strictly validate the enum enum on entry to `Spawn`. The `Metrics` system will just report this garbage string. It may cause unexpected behavior downstream if logic depends on matching exactly `"ephemeral"`.
*   **Gap:** No validation ensures the `req.Type` falls within the allowed predefined constants.
*   **Proposed Test:** Try spawning an agent with an invalid string cast as `SubAgentType`. Verify whether the system should reject it or coerce it to a default safe value like `SubAgentTypeEphemeral`.

### Mangle Atom vs. String Confusion in Configurations
*   **Vector:** The `determineAgentType` relies on `intent.Category` (a string that looks like a Mangle Atom, e.g., `"/system"`). If an incoming request supplies `intent.Category = "system"` (without the forward slash), it fails to match `case "/system":` and defaults to `SubAgentTypeEphemeral`.
*   **Impact:** System tasks (which require persistent or specific daemon handling) might be silently downgraded to ephemeral tasks that die after one execution loop, leading to missing background jobs.
*   **Gap:** The spawner's intent mapping rules are stringly typed and do not perform Mangle Atom normalization (ensuring the `/` prefix exists).
*   **Proposed Test:** Feed `intent.Category = "system"` (no slash) and verify it incorrectly falls back to ephemeral. Then, assert that proper normalization should be implemented or tested.

### Timeout Type Overflow / Negative Coercion
*   **Vector:** `req.Timeout` is a `time.Duration`. What if a value like `-1 * time.Second` or an extremely large duration (e.g., `math.MaxInt64`) is passed?
*   **Impact:** A negative timeout causes the underlying `context.WithTimeout` to instantly expire. The spawned agent will immediately transition to `SubAgentStateFailed` with a context deadline exceeded error. While mechanically correct, it represents wasted compute on spawn overhead.
*   **Gap:** The spawner accepts negative timeouts without defaulting or rejecting them.
*   **Proposed Test:** Spawn with a negative timeout and ensure it's either rejected early or handled gracefully as an instant-fail without crashing the spawner loop.

---

## 3. User Request Extremes (Load and Scale)

The `Spawner` acts as the concurrency control mechanism via `s.maxActiveSubagents`. We must stress test the limits of its scalability and its ability to handle extreme data constraints.

### Extreme Number of Concurrent Spawns
*   **Vector:** The user (or a runaway orchestrator loop) issues 10,000 `Spawn` requests concurrently.
*   **Impact:** The `countActive()` logic requires acquiring the global `s.mu` (which is a `sync.RWMutex` but acts as a full lock during spawn). Under extreme contention, the sequential nature of `s.mu.Lock()` in `Phase 1` and `Phase 5` of `Spawn()` could become a major bottleneck, leading to starvation or excessive latency.
*   **Gap:** The existing test `TestSpawner_Spawn_MaxLimit` only tests 2 agents. It doesn't test the performance degradation or locking starvation when thousands of goroutines hammer the `Spawn` method simultaneously.
*   **Proposed Test:** Write a benchmark/stress test that fires 10,000 goroutines calling `Spawn` against a spawner with a max limit of 100. Verify the rejection rate, the time it takes to reject, and ensure the active count strictly never exceeds 100 without a deadlock.

### Massive Specialist Configurations
*   **Vector:** A user edits `.nerd/agents/researcher/config.yaml` to be an extraordinarily large file (e.g., 50MB of tools, policies, or injected prompts).
*   **Impact:** `loadSpecialistConfig` reads the entire file into memory using `os.ReadFile` or `virtualStore.ReadRaw` and unmarshals it. A 50MB YAML file will spike memory consumption dramatically and tie up CPU during `yaml.Unmarshal`, potentially triggering OOM kills on laptops with 8GB of RAM.
*   **Gap:** There are no file size limits or streaming reads implemented when loading specialist configs. It assumes these files are small.
*   **Proposed Test:** Mock a virtual store that returns a 50MB valid YAML file for a specialist config. Test the memory allocation overhead and parsing latency to determine if a hard cap (e.g., 1MB) is required.

### JIT Compiler Extreme Latency
*   **Vector:** The `generateConfig` method calls `s.jitCompiler.Compile(ctx, ...)`. If the LLM call underpinning the JIT compilation experiences severe degradation (e.g., taking 60 seconds to respond), the `Spawn` method is blocked because it synchronously waits for the JIT config in `Phase 2` before creating the goroutine.
*   **Impact:** Calling `Spawn` blocks the caller. If the orchestrator is spawning multiple agents in a loop without its own goroutines, the entire system grinds to a halt.
*   **Gap:** The `Spawn` method does not enforce a timeout specifically for the JIT compilation phase independently of the overall spawn request (unless the caller provides a tight `ctx`, which they often don't).
*   **Proposed Test:** Mock a `JITCompiler` that blocks for 5 minutes. Verify that `Spawn` correctly respects `ctx` cancellation from the caller, or hangs indefinitely if no timeout is provided.

---

## 4. State Conflicts and Concurrency (Race Conditions)

The Spawner manages a shared map of subagents (`s.subagents`) protected by a `sync.RWMutex`. Concurrency flaws here can lead to lost agents, ghost agents, or panics.

### Time-Of-Check to Time-Of-Use (TOCTOU) in Capacity Limits
*   **Vector:** In `Spawn()`, `Phase 1` locks, checks `countActive()`, and unlocks. `Phase 2` takes time (JIT compilation, IO, LLM). `Phase 5` locks again, checks `countActive()`, and registers the agent.
*   **Impact:** The double-check logic (mitigating TOCTOU) is present and correct. However, if 100 threads pass `Phase 1` because the limit is 100 and current is 0, they all do `Phase 2` (costly JIT generation), and then only the first 100 get through `Phase 5`. The rest are rejected *after* wasting resources on config generation.
*   **Gap:** The system wastes significant LLM tokens and CPU cycles generating JIT configs for agents that will inevitably be rejected at `Phase 5`.
*   **Proposed Test:** Simulate 50 concurrent `Spawn` requests with a `MaxActiveSubagents` of 10. Mock `generateConfig` to track how many times it was invoked. The test should prove that `generateConfig` was invoked 50 times, but only 10 succeeded. This highlights a performance flaw requiring a token/reservation system.

### Concurrent Stop and Cleanup
*   **Vector:** `StopAll()` or `Stop()` is called simultaneously with `Cleanup()`.
*   **Impact:** `Cleanup()` iterates over `s.subagents` under a full lock, deleting items that are completed or failed. `StopAll()` gets a lock, copies the map to a slice, unlocks, and iterates over the slice calling `Stop()`.
*   **Gap:** Since `StopAll()` unlocks before calling `agent.Stop()`, an agent might complete and be cleaned up concurrently. This is mostly safe because the agent instance still exists in memory and `Stop()` handles its own state, but it could lead to redundant operations.
*   **Proposed Test:** Run `Cleanup()` on a ticker while continuously spawning and stopping agents from multiple goroutines to ensure the map manipulation doesn't panic and memory doesn't leak.

### Spawner.GetByName Predictability
*   **Vector:** The method `GetByName` iterates over the map (which is unordered in Go) and returns the *first* active subagent that matches the name.
*   **Impact:** If multiple agents are spawned with the same name (e.g., two "coder" agents), `GetByName`'s behavior is non-deterministic. It might return a different agent on successive calls.
*   **Gap:** The system allows duplicate names. If a caller relies on `GetByName` to retrieve a specific singleton agent (like a persistent specialist), it might accidentally fetch an ephemeral agent that happened to be named the same.
*   **Proposed Test:** Spawn three ephemeral agents with the exact same name. Call `GetByName` multiple times. Prove that it returns an agent, but highlight that it's fundamentally unreliable if uniqueness is not enforced at the Spawner level.

---

## 5. Performance and Architectural Robustness Evaluation

### Fallback Resilience
The Spawner implements a highly resilient fallback chain during JIT compilation:
1.  Try compiling with a specific intent and full context (8192 token budget).
2.  If failed, try compiling a baseline `/general` intent with reduced context (4096 tokens).
3.  If that fails, return an empty `&config.AgentConfig{}`.

This ensures the system **fails open** (in terms of availability) rather than failing closed. An agent with an empty config will still spawn and use the default persona and baseline tools. This is excellent for system uptime, though it might degrade the quality of the AI's output.

### The Cost of JIT Compilation
Currently, the spawner is performant enough to handle normal use cases. However, if the `ConfigFactory` and `JITCompiler` rely heavily on live LLM calls during the `Spawn` phase, creating ephemeral agents for every minor intent becomes a massive latency bottleneck. The system is reliant on caching within the `JITCompiler` to remain performant. If the cache misses, spawning takes seconds rather than milliseconds.

### Security via Virtual Store Integration
The switch to using `s.virtualStore.ReadRaw` instead of `os.ReadFile` in `loadSpecialistConfig` (as seen in the codebase) is a strong security improvement, isolating file reads. However, as noted in the Edge Case section, path traversal vulnerabilities must be explicitly tested.

### Conclusion
The Spawner is structurally sound and effectively mitigates TOCTOU conditions at the cost of resource utilization. Its major testing gaps lie in edge-case string manipulations, extreme concurrent loads, and null/empty request properties that might bypass Mangle routing constraints downstream. Implementing the proposed boundary tests will significantly harden the orchestrator's capability to manage chaotic or adversarial inputs.
### Extended Analysis: Null/Empty Handling Deep Dive

The null/empty handling in `Spawner` deserves further scrutiny. While Go's zero values provide a baseline safety net against raw null pointer dereferences (unlike Java or C++), the logical consequences of these zero values are profound in a neuro-symbolic system.

When `SpawnRequest.Type` is implicitly empty (i.e., `""`), it fails to match `SubAgentTypeEphemeral`, `SubAgentTypePersistent`, or `SubAgentTypeSystem`. The system might pass this empty string to the `Metrics` subsystem, which expects a known enum value. This could lead to downstream dashboard failures or data corruption in long-term metric storage. The test suite must actively assert that an empty `Type` string is either explicitly rejected at the `Spawn` boundary or aggressively coerced to a safe default like `SubAgentTypeEphemeral`.

Furthermore, the `SessionContext` pointer within `SpawnRequest` is critical. If `req.SessionContext` is `nil`, the `generateConfig` function attempts to check `req.SessionContext.DreamMode`. Currently, the code contains a protective `if req.SessionContext != nil` check, which prevents a panic. However, this is a fragile pattern. If future developers add new context-dependent features to the `Spawner` without mirroring this null check, a panic is guaranteed. A robust testing strategy must include a "Null Object Pattern" test, where a fully zeroed `SpawnRequest` is passed to every public method of the `Spawner` to guarantee that no hidden dereference bugs exist.

### Extended Analysis: Coercion and Mangle Typology

The impedance mismatch between Go strings and Mangle Atoms is the most significant source of subtle bugs in Codenerd. The `Spawner` attempts to bridge this gap in `determineAgentType` by switching on `intent.Category`.

Consider the intent category `/system`. In Mangle, this is an Atom. In the Go struct `perception.Intent`, it is stored as a string. If the Transducer accidentally outputs `"system"` (without the slash), the `switch` statement in `determineAgentType` will silently fall through to the `default` case, spawning an ephemeral agent instead of a system agent. This is a catastrophic silent failure. System agents are expected to run continuously and monitor critical background tasks. If they are spawned as ephemeral agents, they will execute once and die, leaving the system unmonitored.

Testing this coercion failure requires simulating a slightly broken Transducer that outputs un-prefixed strings. The tests must assert that the `Spawner` either automatically normalizes these strings (e.g., by prepending a `/` if it's missing) or strictly rejects them. Relying on string matching for neuro-symbolic routing is inherently dangerous without aggressive normalization.

### Extended Analysis: The Concurrency Bottleneck

The `maxActiveSubagents` limit is enforced using a `sync.RWMutex` (`s.mu`). The implementation uses a double-checked locking pattern to prevent Time-Of-Check to Time-Of-Use (TOCTOU) races.

However, this design creates a massive thundering herd problem. Imagine a scenario where `maxActiveSubagents` is set to 10, and 1,000 requests arrive simultaneously.
1. All 1,000 requests acquire the read lock (or a brief write lock if the implementation uses `s.mu.Lock()` for the initial check).
2. Assuming the count is 0, all 1,000 requests pass the initial check.
3. All 1,000 requests concurrently execute `Phase 2: Generate JIT config`. This involves calling the `JITCompiler`, which likely makes expensive LLM calls or complex Mangle queries.
4. After Phase 2, all 1,000 requests attempt to acquire the lock again in `Phase 5`.
5. Only the first 10 requests succeed in registering.
6. The remaining 990 requests are rejected with an error.

The result is that the system wastes 990 expensive JIT compilation cycles. This is a denial-of-service vulnerability triggered by legitimate high load. The test suite must simulate this exact scenario. A mock `JITCompiler` should track the number of times it is invoked. The test should assert that under heavy concurrent load, the number of JIT compilations does not wildly exceed the `maxActiveSubagents` limit. Solving this requires implementing a token bucket or semaphore system *before* Phase 2, reserving a slot before committing to the expensive compilation step.

### Extended Analysis: Specialist Configuration File Parsing

The `SpawnSpecialist` method introduces file I/O into the critical path of agent creation. It reads `config.yaml` from `.nerd/agents/{name}/`.

The current implementation has two severe risks:
1.  **Path Traversal:** As mentioned earlier, `filepath.Join(".nerd", "agents", name, "config.yaml")` is vulnerable if `name` is not sanitized. A name like `../../../../etc/passwd` could leak system files if the virtual store or OS file system allows it.
2.  **YAML Bombing:** The `yaml.Unmarshal` function is notoriously susceptible to "Billion Laughs" style attacks or simply massive, deeply nested files. If a user accidentally or maliciously creates a 500MB `config.yaml`, the `Spawner` will attempt to load the entire file into RAM and parse it. This will almost certainly trigger an Out-Of-Memory (OOM) panic, crashing the entire Codenerd process.

Testing must cover both of these vectors. We must pass malicious path strings and assert that the file path is strictly bounded within the `.nerd/agents` directory. Furthermore, we must mock a file read that returns a massive stream of YAML data and verify that the `Spawner` imposes a strict size limit (e.g., 1MB) before attempting to unmarshal the payload.

### Extended Analysis: Shutdown and Cleanup Races

The lifecycle management functions `Stop`, `StopAll`, and `Cleanup` interact with the same `s.subagents` map.

`StopAll` operates by taking a lock, copying the map values to a slice, releasing the lock, and then iterating over the slice to call `Stop()` on each agent. This is a common and generally safe pattern to avoid deadlocks. However, it creates a window where an agent might finish its task and trigger its own cleanup *while* `StopAll` is iterating.

If `Cleanup` is called on a ticker (as is common in session managers), it acquires the lock and deletes agents from the map. If `StopAll` is executing concurrently, it might call `Stop()` on an agent that `Cleanup` has just removed from the map.

While calling `Stop()` on a finished agent is usually a no-op (if implemented correctly), the test suite needs to guarantee this. We need a chaotic concurrency test:
1. Start 100 goroutines that continuously spawn and wait for agents.
2. Start 10 goroutines that continuously call `StopAll()`.
3. Start 10 goroutines that continuously call `Cleanup()`.
4. Let this run for several seconds.
5. The test succeeds if there are no panics, no map read/write concurrency errors, and the final state cleanly resolves to 0 active agents.

This type of chaotic testing is essential for a system designed to run as a long-lived daemon.

### Test Gap Matrix

To formalize these findings, here is a matrix of the specific test gaps that must be addressed in `spawner_test.go`:

| Category | Vector | Target Component | Expected Behavior |
| :--- | :--- | :--- | :--- |
| Null/Empty | Empty `SpawnRequest.Name` | `Spawn()` | Reject or assign default ID. |
| Null/Empty | Empty `SpawnRequest.Task` | `Spawn()` | Reject request; task is mandatory. |
| Null/Empty | Empty ID in `Stop()` | `Stop()` | Return clear error `subagent not found`. |
| Null/Empty | Empty name in `loadSpecialistConfig` | `loadSpecialistConfig` | Handle gracefully, do not read `config.yaml` at root. |
| Null/Empty | Nil `SessionContext` | `generateConfig` | Proceed safely without panic. |
| Type Coercion | Invalid `SubAgentType` | `Spawn()` | Coerce to Ephemeral or reject. |
| Type Coercion | Negative `Timeout` | `Spawn()` | Reject or instant fail without panic. |
| Type Coercion | Un-prefixed Mangle Atom (`intent.Category`) | `determineAgentType` | Normalize to `/category` or reject. |
| User Request Extremes | 50MB `config.yaml` | `SpawnSpecialist` | Reject file exceeding size limit (e.g., > 1MB). |
| User Request Extremes | 10,000 concurrent `Spawn` calls | `Spawn()` | Enforce limits quickly without locking starvation. |
| State Conflicts | TOCTOU Thundering Herd | `Spawn()` (Phase 1 vs 5) | Reserve slots *before* JIT compilation to save resources. |
| State Conflicts | Concurrent `StopAll` and `Cleanup` | `StopAll()`, `Cleanup()` | Safe map iteration without race conditions or deadlocks. |
| State Conflicts | Duplicate names in `GetByName` | `GetByName()` | Document non-deterministic behavior or enforce uniqueness. |

### Architectural Recommendations

Based on this boundary analysis, several architectural tweaks are recommended for the `Spawner`:

1.  **Slot Reservation:** Change the concurrency control mechanism from a post-compilation check to a pre-compilation semaphore. When a request arrives, it must acquire a token from a channel of size `maxActiveSubagents`. If the channel is full, it immediately rejects the request. If it acquires the token, it proceeds to JIT compilation. If JIT compilation fails, it releases the token. This eliminates the thundering herd TOCTOU problem completely.
2.  **Strict Sanitization:** Implement a `sanitizeSpawnRequest` function that runs at the very beginning of `Spawn`. This function should strip whitespace from names, enforce minimum string lengths, normalize Mangle Atoms, and clamp timeout values to sensible ranges (e.g., between 1 second and 24 hours).
3.  **File Size Limits:** Wrap the `virtualStore.ReadRaw` or `os.ReadFile` calls in `loadSpecialistConfig` with an `io.LimitReader` pattern to prevent YAML bombing.

By addressing these gaps, the `Spawner` will transform from a functional component into a robust, enterprise-grade orchestrator capable of handling the chaotic inputs expected in a production AI environment.

### Deep Dive: Mangle Deductive Routing Flaws

The core value proposition of Codenerd's new architecture is the elimination of hardcoded Go shards in favor of JIT compilation and Mangle-driven routing. The `Spawner` is the first point of contact where Go imperative code meets Mangle declarative logic. This boundary is fraught with subtle complexities.

When `SpawnForIntent` is called, it relies on `determineAgentType` and `determineAgentName`.

```go
func (s *Spawner) determineAgentName(intent perception.Intent) string {
	// Map common verbs to agent names
	switch intent.Verb {
	case "/fix", "/implement", "/refactor", "/create":
		return "coder"
	// ...
	default:
		return "executor"
	}
}
```

This implementation reveals a significant design flaw: **The Spawner is re-implementing intent routing in Go instead of querying Mangle.**

According to the architecture principles, Mangle should dictate behavior. The intent should be asserted as a fact (`user_intent(ID, Category, Verb, Target)`), and the system should query Mangle for the appropriate persona or agent type (e.g., `requires_persona(Intent, Persona)`).

Because the `Spawner` hardcodes the mapping from `/fix` to `"coder"`, it violates the neuro-symbolic design principle. If a user modifies the Mangle rules in `internal/prompt/atoms/identity/` or `intent_routing.mg` to create a new persona (e.g., `"architect"` for `/design`), the `Spawner` will completely ignore it and fall back to `"executor"` because the Go code hasn't been updated.

This is a critical integration boundary failure. The test suite must highlight this.

**Proposed Test:**
We must write a test that dynamically registers a new intent routing rule in the `Kernel` (e.g., `requires_persona(Intent, /architect) :- user_intent(_, _, /design, _, _)`). We then call `SpawnForIntent` with the `/design` verb. The test will currently fail (it will return `"executor"` instead of `"architect"`). This test serves as a failing proof-of-concept that the `Spawner` is ignoring Mangle's source of truth.

To fix this, the `Spawner` must be refactored to query the `Kernel`:

```go
// Proposed refactor
func (s *Spawner) determineAgentName(ctx context.Context, intent perception.Intent) string {
    // 1. Assert intent temporarily
    // 2. Query kernel: ?requires_persona(intent.ID, Persona)
    // 3. Return Persona or fallback to "executor"
}
```

Until this refactor is complete, the `Spawner` remains a fragile component that breaks the promises of the declarative architecture.

### Edge Case: The Runaway Spawner (Recursive Spawning)

Another theoretical boundary condition involves recursive spawning. Can a `SubAgent` spawn another `SubAgent`?

Currently, tools are routed through the `VirtualStore` or the `Executor`. If a tool is created (e.g., `spawn_agent`) that allows an agent to request a sub-task, it would call back into the `Spawner`.

If Agent A (coder) realizes it needs a test written, it might try to spawn Agent B (tester). What happens if Agent B fails and spawns Agent C (fixer), which then spawns Agent D (tester) again?

While the `maxActiveSubagents` limit prevents infinite resource exhaustion (the system will eventually hit the cap and block new spawns), it creates a deadlock scenario. If the global pool is full of waiting agents that are all trying to spawn child agents to complete their tasks, the system locks up. No agent can make progress because they are waiting on children, and no children can spawn because the pool is full.

**Proposed Test:**
We must construct a "deadly embrace" scenario.
1. Set `MaxActiveSubagents` to 2.
2. Spawn Agent A. Agent A's mock tool execution attempts to spawn Agent B and waits for it.
3. Spawn Agent C. Agent C's mock tool execution attempts to spawn Agent D and waits for it.
4. The system is now deadlocked.

This reveals that a single global limit (`maxActiveSubagents`) is insufficient for hierarchical or recursive agent orchestration. The Spawner needs a concept of "resource classes" or "depth limits" to prevent nested deadlocks.

### Evaluating the Semantic Compressor Integration

The `SubAgent` struct tightly integrates with the `SemanticCompressor`. When `CompressMemory` is called, it summarizes the conversation history to save LLM context window tokens.

However, the compression logic introduces a severe failure mode: **Hallucination Amplification**.

If the `SemanticCompressor` uses a cheap, fast LLM model (or if the main model simply hallucinates during the summarization prompt), critical facts might be distorted or invented. The new history becomes `[MEMORY SUMMARY] + [Recent Turns]`.

If the summary states: "User wants the database deleted" instead of "User wants the database backed up," the agent will operate on this hallucinated context for the remainder of its lifecycle.

While this is fundamentally an AI alignment problem, the system architecture must mitigate it. The `SubAgent` does not keep a backup of the uncompressed history. Once `CompressMemory` replaces the slice, the raw truth is gone from memory (though it might exist in the logging subsystem).

**Proposed Test:**
We cannot easily test LLM hallucinations in a unit test, but we can test the *recovery* mechanics.
1. What happens if the `Compressor` returns an error (e.g., API timeout)? The current code falls back to `simple trim to threshold`. We must verify this fallback drops the oldest turns correctly without corrupting the slice.
2. Does the system provide a way to inspect the compressed vs. raw history? We need tests verifying that the raw history is streamed to the `logging.CategorySession` before it is destroyed by compression.

### Final Verification Check

This document comprehensively covers the boundary vulnerabilities of the `Spawner` subsystem. By implementing the suggested tests (Null properties, Coercion failures, Thundering Herds, Mangle Routing Ignorance, and Recursive Deadlocks), the engineering team can bulletproof this critical infrastructure component. The next step is to insert `TODO: TEST_GAP` comments into the actual test files to track the implementation of these edge-case scenarios.

### Advanced Edge Cases: The JIT Compiler Context Propagation

The `Spawner` acts as a bridge between the imperative request and the declarative configuration. A critical phase in this process is `Phase 2: Generate JIT config`, which calls `s.generateConfig()`.

Inside `generateConfig`, the Spawner builds a `prompt.CompilationContext`:

```go
compilationCtx := &prompt.CompilationContext{
    IntentVerb:      intentVerb,
    OperationalMode: "/active",
    TokenBudget:     8192,
}
if req.SessionContext != nil && req.SessionContext.DreamMode {
    compilationCtx.OperationalMode = "/dream"
}
```

This code suffers from **Context Attrition**. The `SpawnRequest` contains a `SessionContext`, but only a single boolean (`DreamMode`) is extracted and propagated into the `CompilationContext`. If the `SessionContext` contains other vital constraints (e.g., specific user overrides, strict safety flags, or environmental variables like `MaxCost`), they are silently dropped here.

The test suite must aggressively assert that all relevant fields from the upstream `SessionContext` map correctly to the downstream `CompilationContext`. If a user initiates a session in a "strict sandbox" mode, and the Spawner drops that flag during JIT compilation, the resulting `SubAgent` might be spawned with destructive capabilities.

**Proposed Test:**
Create a highly populated `SessionContext` with custom metadata, specific token limits, and custom blackboard state. Call `Spawn` and mock the `JITCompiler`. Inside the mock, assert that the received `CompilationContext` faithfully represents the constraints defined in the original `SessionContext`. If it does not, it highlights a structural leak in data propagation.

### Advanced Edge Cases: Goroutine Leakage on Cancellation

In `Spawn()`, Phase 6 executes the subagent asynchronously:

```go
// Phase 6: Start execution
go agent.Run(ctx, req.Task)
```

The `agent.Run()` method creates a child context:

```go
ctx, cancel := context.WithCancel(ctx)
s.mu.Lock()
s.cancel = cancel
s.startTime = time.Now()
s.mu.Unlock()
```

If the parent `ctx` passed to `Spawn()` is cancelled *immediately* after `Phase 5` (registration) but *before* the goroutine starts executing `agent.Run()`, what happens?

The goroutine will start, `agent.Run` will create a child context (which is instantly cancelled because the parent is cancelled), and the executor will likely fail immediately. However, the `Spawner` has already registered this agent in `s.subagents` in Phase 5.

If the agent fails instantly, it transitions to `SubAgentStateFailed`. The `Cleanup()` method will eventually remove it. This seems safe.

But what if `agent.Run()` blocks on acquiring a lock that is held by a deadlocked tool? Even with cancellation, if the goroutine ignores the context inside a poorly written tool, the goroutine leaks. The agent remains in `s.subagents` forever (if it never reaches `SubAgentStateFailed` or `Completed`).

**Proposed Test:**
Simulate a "Zombie Agent" scenario.
1. Mock a `VirtualStore` or `ToolRegistry` that completely ignores `ctx.Done()` and blocks infinitely.
2. Call `Spawn()`.
3. Call `Stop()` immediately.
4. Verify that the agent transitions to `SubAgentStateFailed`.
5. If the mock tool is blocking infinitely and ignoring context, the agent will never transition state. It will consume a slot in `maxActiveSubagents` permanently.

This proves that the Spawner's capacity limit is vulnerable to poorly behaved tools. The Spawner itself needs a "reaper" mechanism that forcefuly evicts agents that exceed their absolute `Timeout`, rather than relying solely on the agent to respect context cancellation.

### Architectural Review: The Clean Loop vs. Mangle State

The documentation states: "SubAgents replace the old shard architecture... NEW: SubAgent with same ~50-line loop, all behavior from JIT."

This "clean loop" relies entirely on the state residing in the `Kernel` (Mangle) and the `VirtualStore`. However, the `SubAgent` struct retains its own local `conversationHistory`.

```go
type SubAgent struct {
    // ...
    conversationHistory []perception.ConversationTurn
    // ...
}
```

This local state violates the strict separation of concerns intended by the neuro-symbolic architecture. If the `Kernel` crashes or the session is serialized to disk for suspension/resumption (a likely future feature), the `conversationHistory` is trapped inside the volatile RAM of the Go struct.

The conversation history should be asserted into the `Kernel` as facts (e.g., `dialogue_turn(AgentID, TurnIndex, Role, Content)`), allowing Mangle to reason over past interactions (e.g., `detect_frustration(User) :- dialogue_turn(_, _, /user, Content), contains_profanity(Content).`).

By keeping `conversationHistory` trapped in a Go slice, the system prevents Mangle from accessing the most critical context of the execution.

**Proposed Architectural Shift:**
The `Spawner` and `SubAgent` should be refactored to push all history into the Fact Store. The test suite should anticipate this by verifying how the Spawner handles state reconstruction if an agent is "re-spawned" from a suspended state.

### Conclusion of Analysis

The `internal/session/spawner.go` module is a vast improvement over hardcoded shards, but it represents a dangerous boundary between imperative Go code and declarative Mangle logic. It suffers from string-to-atom coercion risks, context propagation leaks, and susceptibility to goroutine leakage if child tasks ignore cancellation.

The 400+ lines of analysis provided here outline exactly how the QA team must aggressively test this subsystem. The accompanying `TODO: TEST_GAP` comments in `spawner_test.go` and `subagent_test.go` will guide the implementation of these rigorous checks, ensuring Codenerd remains stable under extreme conditions.

### Summary of Testing Directives

To fully cover the vectors identified, the following actions must be taken by the test engineering team:

1.  **Refactor Mocks:** Ensure `MockJITCompiler`, `MockKernel`, and `MockVirtualStore` can reliably simulate extreme latencies, massive payload returns, and concurrent contention.
2.  **Implement Chaotic Loaders:** Create generic utilities to hammer the `Spawn` interface with varying invalid inputs (empty strings, nil pointers) at high concurrency to sniff out race conditions.
3.  **Validate Mangle Integrity:** Tests must ensure the Spawner respects Mangle assertions rather than overriding them with hardcoded Go logic (especially in `determineAgentName`).
4.  **Enforce Strict Sanitization:** Validate that all boundary input values (timeouts, strings, paths) are clamped, cleaned, or rejected prior to consuming system resources.

This deep dive concludes the boundary analysis for the Spawner subsystem.
