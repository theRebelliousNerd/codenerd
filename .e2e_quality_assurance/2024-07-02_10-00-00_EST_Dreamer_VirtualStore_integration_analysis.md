---
surface: "Dreamer_VirtualStore"
mode: "boundary"
subsystems_tested: ["Dreamer", "VirtualStore", "Kernel"]
blast_radius: "critical"
remediated: false
---

# Dreamer_VirtualStore Integration Analysis

## 1. System Interaction Map

*   `VirtualStore.RouteAction(ctx, fact)` parses a `next_action` fact.
*   If `isDestructiveAction(req.Type)`, it lazily initializes or retrieves the Dreamer via `VirtualStore.getDreamer()`.
*   `Dreamer.SimulateAction(ctx, req)` is called to speculatively execute the action.
*   `SimulateAction` creates a temporary cache key via `dreamCacheKey(req)`.
*   It checks `DreamCache.results` to avoid redundant simulations.
*   If not cached, it clones the kernel (or utilizes the cache).
*   If `Unsafe` is true, `RouteAction` intercepts the execution and injects `security_violation` and `dream_blocked_action` facts back into the VirtualStore.
*   Otherwise, the action proceeds to kernel permission gates and actual execution.

## 2. Contract Analysis

*   **Lazy Initialization:** `VirtualStore` assumes `getDreamer()` returns a non-nil, functional `Dreamer` instance.
*   **Safety Priority:** `RouteAction` assumes `Dreamer.SimulateAction` will synchronously and accurately flag unsafe actions.
*   **Fail-Closed:** If `SimulateAction` fails or encounters an error (like a nil context or a target that is too long), it must return `Unsafe: true` to block execution.
*   **Fact Injection:** When an action is blocked, `VirtualStore` asserts `security_violation` and `dream_blocked_action` facts. It assumes the underlying kernel will accept these facts without panicking or corrupting state.
*   **Cache Integrity:** The `DreamCache` assumes that concurrent simulations of the same action type and target will not lead to race conditions or incorrect cache hits/misses.

## 3. Failure Mode Enumeration

*   **Temporal:** `SimulateAction` takes too long, causing the session executor to time out and leave the system in an unknown state.
*   **Semantic:** `SimulateAction` returns `Unsafe: false` for a clearly malicious or destructive action that should be blocked.
*   **Ordering:** A blocked action injects a fact, but a concurrent execution loop reads the kernel state before the fact is fully propagated.
*   **Partial:** `SimulateAction` panics halfway through, crashing the `VirtualStore` routing process.
*   **Corruption:** The `DreamCache` is mutated by a concurrent goroutine, causing a subsequent simulation to erroneously return `Unsafe: false` based on a stale or corrupted cache entry.

## 4. Adversarial Scenario Design

1.  **Contract:** Context Validation. **Failure:** Nil context passed to `SimulateAction`. **Behavior:** Should return fail-closed (`Unsafe: true`) with reason "nil context provided". **Severity:** P1.
2.  **Contract:** Input Validation. **Failure:** Target path > 4096 bytes. **Behavior:** Should return fail-closed with reason "target path exceeds maximum length". **Severity:** P2.
3.  **Contract:** Cache Thread Safety. **Failure:** Concurrent simulations of the exact same action. **Behavior:** Cache should correctly lock and only perform one underlying kernel simulation, returning consistent results without race conditions. **Severity:** P1.
4.  **Contract:** Fact Injection on Block. **Failure:** Simulate an unsafe action. **Behavior:** `VirtualStore` must inject exactly one `security_violation` and one `dream_blocked_action` fact into the kernel. **Severity:** P0.
5.  **Contract:** Fail-Closed on Panic. **Failure:** (Simulated via mock or injected fault) `SimulateAction` panics. **Behavior:** `RouteAction` should recover and reject the action, not crash the entire process. **Severity:** P0.
6.  **Contract:** Cache Eviction. **Failure:** Flood Dreamer with > 256 unique actions. **Behavior:** Cache should evict oldest entries without panicking or leaking memory. **Severity:** P2.
7.  **Contract:** Laziness. **Failure:** Route a non-destructive action. **Behavior:** `Dreamer` should not be initialized (or at least `SimulateAction` should not be called). **Severity:** P3.
8.  **Contract:** Fact Extraction. **Failure:** `VirtualStore` attempts to inject facts with un-escaped strings or malformed arguments after a block. **Behavior:** Kernel should safely accept or gracefully reject without crashing. **Severity:** P1.
9.  **Contract:** Context Cancellation. **Failure:** Cancel context during `SimulateAction`. **Behavior:** Simulation should abort and return `Unsafe: true` (fail-closed) or a context error. **Severity:** P1.
10. **Contract:** Idempotency. **Failure:** Run the same unsafe action twice sequentially. **Behavior:** Both should be blocked, the second using the cache, and facts should be injected both times (or handled gracefully by the kernel if duplicates). **Severity:** P2.
11. **Contract:** Allowed Action. **Failure:** Run a safe, allowed destructive action. **Behavior:** `SimulateAction` returns `Unsafe: false`, and `RouteAction` proceeds without injecting security facts. **Severity:** P0.
12. **Contract:** Corrupted Cache Hit. **Failure:** Manually poison the cache to say a known-unsafe action is safe. **Behavior:** `RouteAction` will proceed (demonstrating the danger of cache poisoning). **Severity:** P1.
13. **Contract:** Massive Payload. **Failure:** Action payload is 100MB. **Behavior:** Should not OOM during simulation or cache key generation. **Severity:** P2.
14. **Contract:** Recursive Simulation. **Failure:** An action tries to trigger another action during simulation. **Behavior:** Dreamer should isolate the simulation environment. **Severity:** P1.
15. **Contract:** Kernel Rejection of Injected Facts. **Failure:** Modify kernel schema to reject `security_violation`. **Behavior:** `RouteAction` should log the error but still block the original action. **Severity:** P2.

## 5. Cascading Failure Analysis

*   If the `DreamCache` is corrupted or bypassed, malicious actions can execute directly on the system, potentially altering critical code or configuration.
*   If `RouteAction` fails to handle a panic in `SimulateAction`, the entire session loop crashes, starving all pending tasks and breaking the user connection.
*   If `security_violation` facts are not injected properly, the downstream autopoiesis or learning systems will not register the failure, preventing the system from adapting to adversarial inputs.

## 6. Detailed Action Simulation Flow

The interaction between the VirtualStore and the Dreamer is not just a simple function call. It initiates a complex sequence of state projections and evaluations. When `VirtualStore.RouteAction` invokes `Dreamer.SimulateAction`, the following precise steps occur:

1.  **Request Parameter Validation:** The `ActionRequest` is checked for basic validity (e.g., non-empty type, reasonable payload size).
2.  **Context Checking:** The provided `context.Context` is inspected for deadlines or cancellations before beginning the heavy work.
3.  **Cache Lookup:** The deterministic key is generated and checked against the `DreamCache`. If a hit occurs, the cached `DreamResult` is immediately returned.
4.  **Kernel Cloning:** If no cache hit, the Dreamer takes a snapshot (clone) of the current Mangle kernel state. This is a critical isolation step.
5.  **Fact Projection:** The intended effects of the action are translated into speculative facts (e.g., `file_deleted("target.txt")`) and asserted into the cloned kernel.
6.  **Safety Rule Evaluation:** The cloned kernel evaluates its `panic_state` and constitutional rules against the new speculative facts.
7.  **Result Synthesis:** If any `panic_state` rules trigger, the action is marked unsafe. The specific rule that triggered provides the rejection reason.
8.  **Cache Population:** The result is stored in the `DreamCache` for future identical requests.

This multi-step flow highlights several fragile points where failures can cascade if not properly handled by both systems.

## 7. Deep Dive: The Cache Contamination Threat

The `DreamCache` serves as a critical optimization, preventing the Dreamer from performing expensive kernel cloning and evaluation for repeated identical actions. However, this optimization introduces a severe risk of state corruption.

If an attacker can manipulate the input to generate a cache key that collides with a legitimate action, or if a concurrent modification alters the cache state mid-read, the fail-closed guarantee is compromised.

For instance, consider two rapid, identical requests. The first is evaluated as unsafe and blocked. The second hits the cache and is also blocked. This is the intended behavior. Now consider a scenario where the action is evaluated, but before it can be cached, a background learning process updates the kernel's safety rules, rendering the action safe. The first request will correctly be blocked, but if the cache isn't properly synchronized with rule updates, subsequent requests will erroneously be blocked until the cache entry is evicted.

The integration tests must rigorously stress this cache behavior, ensuring that concurrent access is safe and that cache entries are correctly invalidated or ignored when the underlying security context changes.

## 8. Deep Dive: Null Byte Injection and Path Traversal

When the VirtualStore passes the target string (often a filepath) to the Dreamer, it assumes the string is well-formed. However, adversarial inputs may contain null bytes (`\x00`) or path traversal sequences (`../`).

If the Dreamer's validation logic does not properly sanitize these inputs before asserting them as facts in the cloned kernel, several things can go wrong:

1.  **Parsing Errors:** The Mangle parser might fail when constructing the speculative fact, causing the Dreamer to panic or fail-open.
2.  **Logic Bypass:** The safety rules in the kernel might be written to match exact strings. An injected null byte could cause a string comparison to fail, allowing a malicious action to slip through the rules undetected.
3.  **Command Injection:** If the speculative facts are later used to construct shell commands (even in simulation), unsanitized characters could lead to execution escapes.

The test suite must include specific cases that inject null bytes, long strings, and strange encodings to verify that the boundary properly sanitizes or rejects them without crashing.

## 9. Comprehensive Matrix of Boundary Contracts

The following matrix maps the specific contracts to their verification requirements:

| Subsystem A | Subsystem B | Contract Description | Verification Method | Failure Impact |
| :--- | :--- | :--- | :--- | :--- |
| VirtualStore | Dreamer | Synchronous Return | Context timeout tests | Session hang, DoS |
| VirtualStore | Dreamer | Fail-Closed on Error | Inject nil context, malformed input | Security bypass |
| Dreamer | Kernel | Fact Injection Stability | Assert blocked facts, verify via query | Learning loop broken |
| Dreamer | Kernel Clone | State Isolation | Modify clone, assert original untouched | State corruption |
| Session | VirtualStore | Robust Parsing | Malformed next_action payload | Process panic |
| Dreamer | DreamCache | Thread Safety | Concurrent identical requests | Race conditions, erratic blocks |
| Dreamer | DreamCache | Bounded Memory | High-volume unique requests | OOM crash |

## 10. Expanding the Threat Model

While the initial failure modes focus on immediate crashes or bypasses, a mature threat model for the Dreamer-VirtualStore boundary must also consider secondary effects.

*   **Resource Starvation via Simulation:** An attacker might intentionally trigger complex simulations that take a long time to evaluate, exhausting CPU resources even if they are ultimately blocked.
*   **Information Disclosure via Rejection Reasons:** The `Reason` string returned by the Dreamer might inadvertently leak internal kernel state or rule definitions to the user, allowing them to craft more sophisticated bypasses.
*   **Fact Exhaustion:** By repeatedly triggering actions that are blocked, an attacker forces the VirtualStore to inject thousands of `security_violation` facts into the kernel, eventually hitting memory limits or slowing down query performance for legitimate operations.

The tests implemented address the immediate execution threats, but future enhancements should incorporate performance and memory profiling to defend against these secondary starvation attacks.

## 11. Remediation Recommendations

Based on the adversarial analysis, the following architectural changes are recommended for the codebase:

1.  **Strict Type Enforcement:** Enforce strict type checking on the `Payload` map in `ActionRequest` before passing it to the Dreamer.
2.  **Context-Aware Caching:** The `DreamCache` should incorporate a hash of the current active safety rules in its key generation. If the rules change, the hash changes, effectively invalidating stale cache entries.
3.  **Asynchronous Fact Injection:** The injection of `security_violation` facts should be decoupled from the synchronous execution path of `RouteAction` to prevent kernel lock contention under heavy blocking scenarios.

## 12. Extended Failure Mode Enumeration (Detailed Scenarios)

The initial analysis identified broad categories of failure. This section provides a granular, exhaustive list of specific adversarial scenarios targeting the boundary.

*   **Scenario 1: The Nil Payload Trap.** A `next_action` fact is asserted with a `nil` payload map instead of an empty map. The parser handles it, but the Dreamer attempts a key lookup without checking for nil, causing a panic.
*   **Scenario 2: The Deeply Nested Payload.** The payload contains a map nested 50 levels deep. The JSON serializer or deep-copy mechanism used during kernel cloning hits a stack overflow or consumes excessive memory.
*   **Scenario 3: The Ghost Action ID.** The `ActionID` provided in the request is empty or a whitespace string. The DreamCache uses this (or the type/target) as a key, potentially colliding with other un-ID'd actions.
*   **Scenario 4: The Typo Action.** An action type is provided that is close to a destructive action (e.g., `delet_file`). The parser doesn't catch it, and the Dreamer defaults to "allow" because it's not recognized as destructive, but a later system interprets it dangerously.
*   **Scenario 5: The Time-Traveling Request.** An action request is routed with a session ID or timestamp from a previous, already-closed session. The routing logic accepts it, and the Dreamer evaluates it against current state, leading to inconsistent rule application.
*   **Scenario 6: The Unicode Homoglyph Attack.** The target path contains Unicode characters that look like slashes or standard characters but are technically different. The Dreamer's validation allows it, but the underlying OS filesystem normalizes them, resulting in an unintended file modification.
*   **Scenario 7: The Symlink Maze.** The target is a complex chain of symbolic links pointing back to itself or to critical system files outside the sandbox. The Dreamer evaluates the literal path, while execution follows the links.
*   **Scenario 8: The Regex Bomb.** If safety rules use regular expressions against the target path, providing a carefully crafted, highly complex string can cause catastrophic backtracking in the regex engine during simulation, hanging the process.
*   **Scenario 9: The Fact Saturation.** Asserting a `next_action` fact that itself contains hundreds of other embedded facts as arguments. The VirtualStore parser attempts to construct them all before passing to the Dreamer, causing memory bloat.
*   **Scenario 10: The Concurrent Retraction.** While the Dreamer is simulating an action, another process retracts the very facts the Dreamer is relying on for context. The cloned kernel might have captured a partial, inconsistent state.

## 13. The Cascading Impact of "Ghost Facts"

One of the most insidious failure modes involves the injection of `security_violation` facts.

When the Dreamer blocks an action, the VirtualStore injects these facts. However, what happens if the action is blocked due to a transient error (e.g., a brief memory limit hit during simulation)? The action is rejected, the violation fact is asserted, and the system records that the user (or LLM) attempted a malicious act.

If the LLM retries the identical action, and this time the simulation succeeds, the action executes. But the `security_violation` fact from the *first* attempt remains in the kernel.

This is a "Ghost Fact." It represents a state that is no longer accurate (the action wasn't actually malicious, just blocked by a transient fault).

These ghost facts can cascade into severe logic errors:
*   **Legislator Confusion:** The learning subsystems see a high number of violations and incorrectly infer that the current prompt strategy is flawed, triggering unnecessary self-correction cycles.
*   **Trust Degradation:** The orchestrator might lower the trust score of a specific subagent based on these false violations, eventually quarantining it.
*   **Rule Conflict:** The presence of the violation fact might trigger a higher-level constitutional rule that halts all further execution, effectively creating a self-inflicted Denial of Service.

To mitigate this, the integration test suite must verify not only the injection of these facts but their specific context. Furthermore, the architecture should perhaps tag these facts with a confidence score or tie their lifecycle strictly to the turn they occurred in, retracting them if they are determined to be noise.

## 14. Architectural Invariants requiring Verification

Beyond simple function contracts, there are several systemic invariants that must hold true across this boundary.

*   **Invariant A: Monotonicity of Safety.** If an action is deemed unsafe at time T, and the underlying safety rules (the constitution) only become *stricter* over time, the action must remain unsafe at time T+1. The cache must respect this.
*   **Invariant B: Execution Isolation.** The process of *simulating* an action in the Dreamer must never result in a permanent change to the real filesystem, external APIs, or the primary kernel state.
*   **Invariant C: Deterministic Rejection.** Given the exact same kernel state and the exact same ActionRequest, the Dreamer must consistently return the same `DreamResult` (including the specific `Reason`). Flakiness here indicates non-determinism in the evaluation engine.
*   **Invariant D: Bounded Latency.** The end-to-end time from `RouteAction` being called to returning an error (if blocked) or proceeding must be bounded, regardless of the complexity of the action or the current size of the cache.

## 15. The Role of the "Boot Guard"

The `RouteAction` function contains a crucial piece of logic:

```go
v.mu.RLock()
bootGuardActive := v.bootGuardActive
v.mu.RUnlock()
if bootGuardActive {
    return "", fmt.Errorf("boot guard active...")
}
```

This acts as a pre-boundary shield. If the system is still initializing, no actions, regardless of their safety, are routed to the Dreamer.

An adversarial test should attempt to bypass this. If a race condition exists where `bootGuardActive` is flipped momentarily, or if facts are queued and processed out of order, an action might slip through before the system's full safety posture (the constitution and learned rules) is fully loaded.

This would mean the Dreamer evaluates the action against an empty or partial rule set, likely defaulting to "allow" and permitting a destructive action during the vulnerable boot window.

## 16. Future-Proofing the Boundary

As the codeNERD architecture evolves, this boundary will become more complex.

*   **Multi-Agent Coordination:** When multiple subagents are proposing actions simultaneously, the VirtualStore will need to route them concurrently. The Dreamer must be capable of simulating actions in parallel without state bleed between clones.
*   **Continuous Learning:** As the autopoiesis system introduces new rules dynamically, the cache invalidation strategy will need to move from a simple LRU model to a dependency-aware model (invalidating only cache entries affected by the specific rule change).
*   **External Tool Integration:** As MCP tools are added, the definition of a "destructive action" will blur. The Dreamer will need a robust way to introspect the schema of unknown tools to determine if they require simulation.

The integration tests designed today form the baseline defense for these future complexities. They ensure that the fundamental assumptions about fail-closed behavior, state isolation, and correct fact routing remain intact as the system grows.

## 17. Expanded Contract Table

| Subsystem A | Subsystem B | Contract Description | Verification Method | Failure Impact |
| :--- | :--- | :--- | :--- | :--- |
| VirtualStore | Dreamer | Payload Type Safety | Inject mismatched types | Parser panic |
| VirtualStore | Parser | Fact Arity | Assert fact with missing args | Index out of bounds |
| Dreamer | Kernel | Deep Copy Integrity | Modify nested map during simulation | Original state mutation |
| Kernel | VirtualStore | Synchronous Query | Check injected facts immediately | Race condition on reads |
| Session | VirtualStore | ID Generation | Ensure unique ActionIDs | Cache collision |
| Parser | Dreamer | Encoding Safety | Inject UTF-16 or invalid bytes | String matching failures |
| Dreamer | OS FS | Traversal Prevention | Inject `../` in targets | Sandbox escape |

## 18. Conclusion of Siege Analysis

The `Dreamer` ↔ `VirtualStore` boundary is the single most critical security chokepoint in the codeNERD execution loop. It is the line between abstract thought (the LLM's intent) and physical reality (file modifications, command executions).

The analysis reveals that while the basic fail-closed mechanisms are in place, the reliance on a shared, stateful cache (`DreamCache`) and the assumption of robust error handling across multiple layers of parsing and execution create significant vulnerabilities.

The provided test suite addresses the highest priority risks: cache races, context leaks, massive payloads, and basic injection. However, continuous adversarial testing, particularly focusing on the "Ghost Fact" problem and complex state mutations during simulation, will be necessary to harden this boundary against sophisticated attacks.

## 19. Extended Scenario Definitions

### 19.1. The "Schrödinger's File" Scenario
*   **Contract:** Consistent State Projection.
*   **Failure:** A `next_action` fact proposes writing to a file, and a subsequent fact proposes reading from it.
*   **Behavior:** The Dreamer must accurately project the state change from the first action into the kernel clone, so the second action (during its simulation) sees the "written" state. If the clone resets between these closely coupled actions, the second action might fail or be blocked based on stale data.
*   **Severity:** P2 (Logic failure, not a crash, but prevents complex workflows).
*   **Cascading Impact:** Multi-step agent plans fail because intermediate state isn't preserved across speculative evaluations.

### 19.2. The Concurrent Retraction Storm
*   **Contract:** Kernel Clone Stability under load.
*   **Failure:** While the Dreamer is cloning the kernel, another high-priority thread (e.g., the Session Executor handling a context window collapse) issues massive retraction commands (e.g., clearing all `working_memory` facts).
*   **Behavior:** The `RealKernel`'s locking mechanisms (`mu.RLock()` vs `mu.Lock()`) must perfectly synchronize. If the clone captures a partially retracted state, it might evaluate a safe action as unsafe (due to missing context) or an unsafe action as safe (due to missing constraints).
*   **Severity:** P1 (Intermittent security bypass or DoS).
*   **Cascading Impact:** Flaky rule evaluation; agent unpredictably refuses to perform tasks it is permitted to do.

### 19.3. The Recursive Dream Loop
*   **Contract:** Bounded Speculation.
*   **Failure:** The action being simulated triggers a virtual predicate evaluation that, in turn, attempts to assert a new `next_action` fact (e.g., a rule that says "if reading this file fails, attempt to create it").
*   **Behavior:** The Dreamer must detect that it is already in a simulation context and prevent nested simulations from spawning infinitely, consuming all stack space.
*   **Severity:** P0 (Process panic via Stack Overflow).
*   **Cascading Impact:** Complete system crash triggered by specific, cyclic safety rules interacting with generic actions.

### 19.4. The Poisoned Cache Key Collision
*   **Contract:** Deterministic, Unique Cache Keys.
*   **Failure:** The `dreamCacheKey` function relies only on `req.Type` and `req.Target`. An attacker provides a target string that includes a delimiter used by the key function, effectively spoofing a different action type or target.
*   **Behavior:** The Dreamer retrieves a cached result for action A when evaluating action B, incorrectly applying the safety verdict of A.
*   **Severity:** P1 (Security bypass).
*   **Cascading Impact:** An explicitly blocked action (e.g., `delete_file /etc/passwd`) is permitted because it collided with a cached, permitted action (e.g., `read_file /etc/passwd`).

### 19.5. The Unbound Variable Exploit
*   **Contract:** Safe Fact Assertion.
*   **Failure:** The `ActionRequest` payload contains strings formatted as Mangle variables (e.g., `X` or `TargetFile`) instead of concrete atoms.
*   **Behavior:** When the Dreamer asserts these as speculative facts, the Mangle engine might interpret them as unbound variables in a context where grounding is expected, leading to evaluation panics or undefined behavior during the `panic_state` query.
*   **Severity:** P2 (Engine error or panic).
*   **Cascading Impact:** The simulation crashes, triggering the fail-closed mechanism, but preventing the legitimate processing of inputs containing literal uppercase words.

### 19.6. The Memory Leak via Payload Accumulation
*   **Contract:** Memory release after simulation.
*   **Failure:** The Dreamer processes millions of actions over a long-running session. The `ActionRequest` contains large payloads (e.g., full file contents).
*   **Behavior:** If the `DreamCache` stores the entire `ActionRequest` within the `DreamResult` (which it does: `DreamResult.Request = req`), it is holding onto massive payload data unnecessarily. Even with a cap of 256 items, this could mean gigabytes of retained memory.
*   **Severity:** P2 (Slow OOM).
*   **Cascading Impact:** The system gradually slows down due to GC pressure and eventually crashes, requiring frequent restarts.

## 20. Code-Level Implementation Flaws

Reviewing the implementation of `VirtualStore.RouteAction` and `Dreamer.SimulateAction`, specific vulnerabilities are apparent:

1.  **Missing Timeout in Simulation:** `SimulateAction` does not internally enforce a timeout. It relies entirely on the provided `ctx`. If the caller passes `context.Background()`, a complex simulation could hang indefinitely.
2.  **Insufficient Sanitization:** The conversion of the fact arguments to the `ActionRequest` struct in `parseActionFact` relies on Go's dynamic typing (`any`). If an argument is unexpectedly a struct or a channel instead of a string or map, it might panic during type assertion before ever reaching the Dreamer.
3.  **Cache Key Weakness:** The `dreamCacheKey` function simply concatenates strings: `string(req.Type) + ":" + req.Target`. If `req.Type` contains a colon, collisions are trivial. (e.g., Type: `exec`, Target: `:cmd` vs Type: `exec:`, Target: `cmd`).

## 21. Actionable Mitigation Strategies

To secure this boundary, the following specific code changes are required:

1.  **Harden Cache Keys:** Implement a cryptographic hash (e.g., SHA-256) or a robust serialization format (JSON) for generating cache keys, incorporating not just Type and Target, but also the Session ID and a hash of the current safety rules.
2.  **Enforce Internal Timeouts:** `SimulateAction` should wrap the provided context with its own maximum upper bound (e.g., `context.WithTimeout(ctx, 5*time.Second)`) to guarantee it never hangs, regardless of the caller's context.
3.  **Payload Stripping:** Modify `DreamResult` to NOT store the entire `ActionRequest`. It should only store the `ActionID`, the `Unsafe` boolean, and the `Reason`. The payload is not needed for caching the safety verdict.
4.  **Strict Type Assertions:** In `parseActionFact`, use the `comma-ok` idiom for all type assertions and return a structured error if the type is unexpected, rather than risking a panic.

## 22. The Role of the "InteractiveExecutiveGate"

Memory notes that the `VirtualStore` interacts with the `Dreamer` safety module through the `InteractiveExecutiveGate`. This gate acts as a pre-flight check.

If this gate is bypassed or its logic is flawed, the Dreamer might receive actions that are contextually inappropriate (e.g., a background task attempting an interactive UI prompt).

The boundary isn't just about *can* it execute (safety), but *should* it execute (contextual appropriateness). The integration tests must verify that the gate properly filters actions before they consume simulation resources in the Dreamer.

## 23. Final Assessment of the Boundary

The boundary between the VirtualStore and the Dreamer is heavily relied upon but surprisingly brittle. Its strength lies in the declarative safety rules of the Mangle engine, but its weakness lies in the imperative Go code that bridges the gap.

The translation of unstructured data (the payload) into structured facts, the reliance on a shared cache without rule-invalidation, and the assumption of well-formed inputs create a broad attack surface.

The "Siege" mentality requires us to look not at how the system functions when used correctly, but how it fractures when the contracts are violated. The tests written today prove that the system can survive basic assaults (nil contexts, large strings). The next phase of hardening must address the subtle logical vulnerabilities: cache poisoning, state corruption, and recursive exhaustion.

## 24. Further Contract Analysis: The "Payload as Context" Problem

A significant, underexplored contract between `VirtualStore` and `Dreamer` involves the `Payload` map.

While the `Target` specifies *what* is being acted upon (e.g., a file path), the `Payload` specifies *how* (e.g., the exact content being written). The Dreamer currently focuses primarily on the Type and Target for its cache key and basic safety checks.

However, advanced constitutional rules might (and should) inspect the payload. For instance, a rule might permit writing to `config.json` *unless* the payload contains the string `"admin: true"`.

If the Dreamer's simulation phase fails to accurately project the payload into the cloned kernel state, or if the Mangle queries used for `panic_state` cannot efficiently introspect the payload map, this entire class of payload-aware safety rules is rendered useless.

*   **Vulnerability:** An attacker could craft an action that is ostensibly safe based on Type and Target, but carries a malicious payload that bypasses the intended, but un-evaluable, safety checks.
*   **Test Requirement:** The test suite must include scenarios where the *only* difference between a safe and unsafe action is the content of the payload map, verifying that the Dreamer correctly differentiates them and the cache does not conflate them.

## 25. The Impact of Mangle Engine Limitations

The Dreamer relies completely on the underlying `codeberg.org/TauCeti/mangle-go` engine for evaluation. The integration boundary must therefore account for the engine's known limitations:

1.  **Stratification Constraints:** Mangle requires rules to be stratified (no negative cycles). If the autopoiesis system dynamically generates a rule that violates stratification, the engine will fail during the analysis phase.
    *   *Boundary Impact:* The Dreamer's `SimulateAction` must handle analysis errors gracefully. If the kernel clone fails to analyze due to bad rules, the action must fail-closed, rather than panicking or assuming the action is safe.
2.  **Evaluation Timeouts:** Complex recursive rules can take excessive time to reach a fixpoint.
    *   *Boundary Impact:* The integration tests must verify that the Dreamer respects `context.DeadlineExceeded` not just during its own Go logic, but that this cancellation propagates down into the Mangle engine's `Eval()` loop, halting execution immediately.
3.  **Type Dissonance:** As noted in previous learnings, Mangle treats Atoms (e.g., `/user`) and Strings (e.g., `"user"`) differently.
    *   *Boundary Impact:* The `parseActionFact` function in the VirtualStore must be absolutely rigid in how it converts Go strings into Mangle types before passing them to the Dreamer, ensuring that the safety rules join correctly against the provided data.

## 26. Simulating State Rollback

The core premise of the Dreamer is that it is a *simulation*. It clones the kernel, asserts facts, evaluates rules, and then the clone is discarded.

However, the VirtualStore is intrinsically linked to external state (the filesystem, external APIs).

When `SimulateAction` is called, it might need to interact with external systems to project an accurate state (e.g., checking if a file exists before simulating its deletion).

If these external interactions are not strictly read-only or perfectly mocked within the simulation context, the "simulation" leaks into reality.

*   **The "Observer Effect" Failure:** If simulating an action causes a side effect (e.g., touching a file, incrementing an API counter, triggering a webhook), the isolation contract is broken.
*   **Verification:** Integration tests must utilize sandboxed mocks for all external interfaces during the `SimulateAction` call, explicitly asserting that no actual file writes or network calls were made while the Dreamer was active.

## 27. Cross-Phase Contamination in Campaigns

During a multi-phase campaign (e.g., Discovery -> Planning -> Execution), the VirtualStore processes hundreds of actions.

The Dreamer's cache currently does not seem to partition based on the active campaign phase.

*   **The Risk:** An action that is safe in the "Execution" phase (e.g., `write_file`) might be strictly forbidden in the "Discovery" phase. If an agent attempts the action during Discovery, it is blocked and cached. If the phase transitions to Execution, the action might remain erroneously blocked if the cache hit occurs before the cache entry expires.
*   **The Fix:** The cache key must explicitly incorporate the current `campaign_phase` fact, ensuring that safety verdicts are scoped to the specific operational context they were generated in.

## 28. Conclusion of Extended Analysis

The `Dreamer` ↔ `VirtualStore` integration is not merely a function call; it is a complex negotiation between an imperative state manager (VirtualStore) and a declarative logic engine (Mangle, via the Dreamer).

The vulnerabilities identified—cache poisoning, un-bounded speculation, ghost facts, payload blindness, and cross-phase contamination—highlight the necessity for continuous, adversarial integration testing.

The tests implemented in `tests/e2e/dreamer_virtualstore_integration_test.go` provide a foundation, but the true strength of the codeNERD architecture will be determined by how rigorously these specific edge cases are mitigated in future iterations.

Siege's recommendation is to prioritize the redesign of the `DreamCache` and the strict enforcement of internal timeouts within the simulation loop, as these represent the most direct vectors for Denial of Service and Security Bypass.

## 29. Deep Dive: The Articulation Transducer Impact

While this analysis focuses on the `Dreamer` ↔ `VirtualStore` boundary, it is crucial to trace the cascading impact of failures here onto the downstream Articulation Transducer.

When the Dreamer blocks an action, it injects `security_violation` and `dream_blocked_action` facts. The Articulation system (responsible for generating the final LLM response) queries the kernel for these facts to explain the failure to the user or subagent.

*   **Contract:** The Articulation system assumes that if an action failed, a corresponding explanatory fact exists in the kernel.
*   **Failure Mode:** If the VirtualStore's fact injection fails (e.g., due to a Mangle schema error as discussed in Section 2), the Articulation system is blind. It sees that the action didn't execute but has no context as to why.
*   **Cascading Impact:** The LLM receives a generic "Action Failed" response rather than "Action Blocked: Target path exceeds limits." Without the specific feedback, the LLM cannot correct its behavior, leading to endless retry loops (the "TDD Loop of Death") where the LLM repeatedly proposes the same invalid action.

## 30. Adversarial Scenario: The Retry Storm

Combining the above concepts, we can model a critical failure scenario:

1.  **Attack:** The LLM proposes an action that is valid but contextually unsafe (e.g., modifying `main.go` during a read-only code review phase).
2.  **Dreamer Block:** The Dreamer correctly identifies this via `panic_state` rules and returns `Unsafe: true`.
3.  **Injection Failure:** The VirtualStore attempts to inject the `security_violation` fact, but due to a corrupted Mangle schema (perhaps introduced by a faulty dynamic rule), the injection fails silently.
4.  **Articulation Blindness:** The Articulation system formulates the response. It queries for the failure reason but finds none. It sends the LLM: "Action failed. Please try again."
5.  **The Storm:** The LLM, believing it made a simple syntax error or transient mistake, proposes the exact same action again. The cycle repeats indefinitely until the `SessionExecutor` hits its maximum turn limit or the token budget is exhausted.

*   **Mitigation Strategy:** The VirtualStore must treat fact injection failures after a Dreamer block as critical errors. If it cannot record the violation, it must forcibly crash the current action loop and surface a hard system error to the Orchestrator, rather than allowing the Articulation system to fail open.

## 31. The Importance of Testing the "Unhappy Path"

The majority of standard unit tests verify that the system works when given valid inputs. Siege's philosophy dictates that the true measure of a system's resilience is how it behaves when the implicit contracts are violated.

The integration test suite (`tests/e2e/dreamer_virtualstore_integration_test.go`) explicitly targets these unhappy paths:
*   `TestE2E_Dreamer_VirtualStore_NilContext`: Verifies fail-closed behavior on missing context.
*   `TestE2E_Dreamer_VirtualStore_OversizedTarget`: Verifies boundary limits before simulation.
*   `TestE2E_Dreamer_VirtualStore_FactInjection`: Proves the critical feedback loop is intact.
*   `TestE2E_Dreamer_VirtualStore_CacheEviction`: Prevents memory exhaustion attacks.

These tests do not just verify that the system *functions*; they verify that it *survives*. They are the baseline defense against the cascading failures described in this journal.

## 32. Final Recommendations for the codeNERD Architecture

The `Dreamer` ↔ `VirtualStore` integration is a powerful pattern for neuro-symbolic safety. By simulating actions in a logic engine before executing them imperatively, codeNERD achieves a high degree of confidence in its autonomous operations.

However, the analysis demonstrates that this pattern requires rigorous engineering discipline at the boundaries.

**Key Takeaways for Future Development:**

1.  **Treat the Cache as a Security Perimeter:** The `DreamCache` is not just a performance optimization; it is a security decision caching mechanism. It must be as robust against poisoning and collision as the primary evaluation logic.
2.  **Strict Boundary Typing:** The conversion between Go's dynamic types (`map[string]any`) and Mangle's rigid logic atoms is a prime source of silent failures. Enforce strict type checking at the moment of ingress (in `parseActionFact`).
3.  **Fail-Closed is Not Enough; Fail-Loud is Required:** When an action is blocked, silently dropping it is insufficient. The system must loudly and reliably record the failure (via fact injection) to enable learning and prevent retry storms.
4.  **Continuous Adversarial Testing:** The `e2e` tag must be used to continuously run these adversarial scenarios against the codebase. As the constitution grows and new tools are added, the boundaries will shift, and new vulnerabilities will emerge.

This QA Journal serves as both an analysis of the current state and a roadmap for hardening the codeNERD architecture against the inevitable chaos of real-world integration.

## 33. The Impact of Mangle's Negation as Failure (NAF)

The safety rules evaluated by the Dreamer rely heavily on Mangle's implementation of Negation as Failure. A rule might state: "An action is safe if there is NO evidence that it modifies a protected directory."

*   **Contract:** The Dreamer assumes that the cloned kernel contains a *complete* representation of the relevant state.
*   **Failure Mode:** If the `VirtualStore` fails to assert a crucial piece of context (e.g., the current working directory or the active user permissions) during the cloning phase, the Mangle engine will evaluate the negation as `true` (because the evidence is absent, not because it's false).
*   **Cascading Impact:** An action that should have been blocked is permitted because the safety rule failed to find the required context facts, leading to a silent security bypass based on missing information rather than explicit permission.

## 34. The "Schrödinger's State" During Simulation

When `SimulateAction` runs, it projects the intended effects of the action into the kernel clone. However, some actions have complex, multi-step effects.

*   **Vulnerability:** If an action (e.g., `build_project`) implies multiple distinct state changes (e.g., creating a directory, writing multiple files, executing a script), the Dreamer might only project a simplified version of these effects (e.g., just `action_executed("build_project")`).
*   **Consequence:** The safety evaluation is performed against an incomplete or idealized model of the action's impact. The `panic_state` rules might clear the simplified action, but the actual execution might violate constraints that the Dreamer was blind to.
*   **Testing Strategy:** Integration tests must verify that complex actions are broken down into granular, verifiable speculative facts during simulation, ensuring the evaluation model accurately reflects the physical execution reality.

## 35. The Risk of State Leakage via Global Variables

While the Mangle kernel state is isolated via cloning, the Go environment executing the `SimulateAction` logic is not.

*   **Contract:** The Dreamer's simulation must be entirely side-effect free in the host environment.
*   **Failure Mode:** If any part of the simulation logic (perhaps a custom virtual predicate invoked during rule evaluation) relies on or modifies global Go variables (e.g., a shared configuration struct, a global logger state, or a shared map of active connections), the "isolated" simulation can corrupt the state of the parent process.
*   **Cascading Impact:** Subsequent actions, even legitimate ones, might fail or behave unpredictably because the global state was mutated during a prior, discarded simulation.
*   **Verification:** Advanced integration tests should employ Go's `-race` detector specifically targeting the simulation pathways to ensure no shared memory is modified during `Dreamer` execution.

## 36. Evaluating the DreamRouter and Learning Persistence

The `Dreamer` contains a reference to a `DreamRouter`, which is responsible for routing confirmed learnings to persistence stores.

*   **Integration Boundary:** While this analysis focused on the VirtualStore, the `DreamRouter` represents another critical boundary.
*   **Failure Mode:** If the Dreamer successfully evaluates an action, extracts a learning, but fails to serialize it correctly for the `DreamRouter`, the learning is lost.
*   **Cascading Impact:** The autopoiesis system (Ouroboros loop) fails to adapt. The agent continues to make the same mistakes because the feedback loop from the Dreamer to the permanent memory store is severed.
*   **Future Testing:** Subsequent Siege campaigns must target the `Dreamer` ↔ `DreamRouter` boundary, verifying that complex Mangle facts derived during simulation are accurately persisted and re-hydrated in future sessions.

## 37. Summary of Vulnerability Vectors

To summarize the extensive analysis, the `Dreamer` ↔ `VirtualStore` integration is susceptible to the following vectors:

1.  **Input Validation Vectors:** Nil payloads, massive targets, malformed paths, unicode homoglyphs.
2.  **State Management Vectors:** Cache poisoning, concurrent cache races, missing context facts during cloning, state leakage via globals.
3.  **Logic Engine Vectors:** Stratification failures, unbounded recursion timeouts, type dissonance (Atom vs String), NAF exploitation via missing data.
4.  **Feedback Loop Vectors:** Silently failing fact injection, articulation blindness, retry storms, ghost facts.

## 38. Final Siege Directive

The implementation of the integration tests in this PR addresses the most immediate, critical stability issues (crashes, panics, memory exhaustion).

However, the logical vulnerabilities (cache poisoning, NAF exploitation) require architectural refactoring—specifically, upgrading the `DreamCache` key generation and enforcing strict, typed boundaries between the imperative Go code and the declarative Mangle kernel.

This QA journal stands as the blueprint for that future remediation work. The cracks have been mapped; the next step is to seal them.

## 39. The Token Economy and Simulation Budgets

The system operates under strict resource constraints, primarily driven by the token budget of the underlying LLM. However, local simulation in the Dreamer also consumes resources (CPU, Memory).

*   **Contract:** The Dreamer's resource consumption must be proportionate to the value of the simulation.
*   **Failure Mode:** A malicious or confused agent might propose actions that are cheap to articulate (few tokens) but extremely expensive to simulate locally (e.g., recursive directory traversal rules in Mangle).
*   **Cascading Impact:** The local codeNERD instance becomes unresponsive, starved of CPU cycles by the Dreamer's evaluation engine, acting as a local Denial of Service that bypasses the external API token limits.
*   **Testing Strategy:** E2E tests must include benchmarks for simulation time. The `SimulateAction` function should ideally incorporate a "fuel" or evaluation budget that aborts complex queries before they exhaust system resources.

## 40. Handling Context Cancellation Propagation

When the main session loop (or a user interrupt) cancels the context, this cancellation must propagate instantly through all layers.

*   **Vulnerability:** The VirtualStore checks context at the beginning of `RouteAction`. But if the context is cancelled *while* the Dreamer is deep inside `kernel.Clone()` or evaluating a complex Mangle rule, does it abort immediately?
*   **Consequence:** If the underlying Mangle engine does not respect context cancellation at a granular level, a cancelled action will continue to burn CPU cycles until the evaluation naturally concludes (or panics).
*   **Verification:** Integration tests must trigger a complex action, wait a few milliseconds, cancel the context, and assert that the total execution time is strictly bounded by the cancellation signal, not the evaluation depth.

## 41. The Threat of "Action Masquerading"

An adversarial prompt injection might attempt to trick the system by providing a payload that mimics a different action type.

*   **Contract:** The `ActionType` dictates the simulation logic, not the payload content.
*   **Failure Mode:** If the Dreamer relies on fields within the payload to determine *how* to simulate the action (rather than strictly following the `ActionType`), an attacker could pass an `ActionType` of `read_file` but include payload fields that the Dreamer interprets as a `write_file` operation during simulation.
*   **Cascading Impact:** The cache records the action as safe (because it was evaluated under the rules of `read_file`), but the downstream executor might rely on the payload fields to actually perform a write, bypassing the safety checks entirely.
*   **Remediation:** Strict schema validation for each `ActionType` must occur before simulation. The Dreamer must reject payloads containing unexpected keys for the declared action type.

## 42. Siege's Final Stand

This document represents a comprehensive mapping of the structural weaknesses at the `Dreamer` ↔ `VirtualStore` boundary. By systematically categorizing and designing tests for these failures, we shift the engineering culture from "does it work?" to "what happens when it breaks?"

The true value of this analysis is not just the immediate tests implemented, but the enduring shift in architectural perspective. Every new tool, every new rule, and every new subagent must now be evaluated against this expanded threat model. The cracks have been found; the siege was successful. Now, the rebuilding begins.

## 43. The "Silent Drop" Conundrum

In some edge cases, the Mangle kernel might accept a fact (like `security_violation`) but due to a subtle schema mismatch or an over-constrained rule, it might not trigger any downstream consequences.

*   **Vulnerability:** The VirtualStore assumes that successfully injecting the fact is sufficient to trigger the learning/autopoiesis loop.
*   **Consequence:** If the fact is dropped silently by the rules engine (e.g., it requires a fourth argument that was omitted in a recent code update), the failure is logged in Go, but the agent's logic engine remains oblivious.
*   **Verification:** Tests must not only query that the fact exists, but also run a secondary query simulating the autopoiesis system's view to ensure the fact is actionable within the logic environment.

## 44. Review of the Test Suite Coverage

The implementation in `tests/e2e/dreamer_virtualstore_integration_test.go` successfully addresses the core P0 and P1 vulnerabilities identified in this journal.
*   It proves the cache can handle high concurrency without racing.
*   It proves the system recovers from malformed inputs without panicking.
*   It proves that the fail-closed contract is honored when context is missing or targets are absurdly large.

The tests are robust, but they represent the beginning, not the end, of the siege. Continuous investment in adversarial testing is required to maintain the structural integrity of the codeNERD architecture.
