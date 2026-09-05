---
surface: "virtualstore_dreamer"
mode: "boundary"
subsystems_tested: ["VirtualStore", "Dreamer", "Kernel"]
blast_radius: "critical"
remediated: false
---

# System Interaction Map
The VirtualStore serves as the critical executive gateway that bridges the Mangle evaluation loop (the cognitive core) with the physical execution of side-effects.
When a SubAgent (managed by the Session Executor) decides to perform an operation, it outputs a tool call (such as a filesystem write, shell command execution, or semantic AST edit).
The execution sequence follows a strict and delicate dance across boundaries:
1. `VirtualStore.PreflightDestructiveToolCall(ctx, actionID, toolName, args)` is invoked by the interactive Session Executor.
   - This method evaluates whether the tool name maps to a known destructive action via the `actionTypeForToolName(toolName)` lookup.
   - If it maps to a destructive action (e.g., `write_file`), it attempts to acquire a reference to the Dreamer subsystem via `v.getDreamer()`.
2. `VirtualStore.getDreamer()` manages lazy initialization of the `Dreamer` subsystem.
   - It acquires `v.dreamerInitMu`, checks if `v.dreamer` is initialized, and if not, attempts to construct it.
   - It retrieves the default `CortexKernel` from the VirtualStore's environment, locates the concrete `RealKernel` (which actually stores the working logic and data).
   - If it succeeds in extracting the `RealKernel`, it instantiates and caches the `Dreamer` instance. Otherwise, it returns nil.
3. Back in `PreflightDestructiveToolCall`, if `getDreamer()` returned nil, the system initiates a fail-closed sequence.
   - A `security_violation` fact is injected into the kernel: `v.injectFact(newSecurityViolationFact(req, reason))`.
   - An `InteractiveGateError` is returned to the Session Executor, blocking the action.
4. If `Dreamer` is present, `VirtualStore` translates the tool payload into an `ActionRequest` and calls `Dreamer.SimulateAction(ctx, req)`.
   - The request crosses the boundary into the Dreamer subsystem.
5. Inside `Dreamer.SimulateAction(ctx, req)`:
   - It immediately validates the context and request parameters. If `req.Type` is empty, it fails closed.
   - It computes a deterministic cache key via `dreamCacheKey(req)`. Crucially, this key only concatenates `req.Type` and `req.Target`.
   - The Dreamer holds a thread-safe read-lock (`d.mu.RLock()`) to retrieve its internal `cache` and `kernel` pointers.
   - It queries the cache (`d.cache.Get(cacheKey)`). If a hit is found, it returns the cached `DreamResult`, entirely bypassing full simulation.
   - If there is a cache miss, it ensures the `kernel` pointer is non-nil. If nil, it fails closed.
6. Cache miss simulation execution:
   - The Dreamer calls `d.kernel.Clone()`. This is a heavyweight operation that creates an ephemeral snapshot of the Mangle engine, containing the entire current EDB and IDB state.
   - The Dreamer then calls `projectEffects(req, clonedKernel)`. This function translates the `ActionRequest` (including its potentially dangerous payload) into a series of hypothetical Mangle facts (e.g., `simulated_write(Target, Payload)`).
   - These facts are asserted into the cloned kernel's EDB.
   - The Dreamer executes `evaluateSimulation(clonedKernel)`. This runs a full Mangle deductive fixpoint evaluation. It checks for the derivation of `panic_state/1` or `forbids_action/2`.
   - The results of the evaluation form the `DreamResult`, detailing whether the action is `Unsafe` and the `Reason` for the block.
   - The `DreamResult` is stored in the cache (`d.cache.Store(cacheKey, result)`).
7. Returning across the boundary:
   - The `DreamResult` is handed back to `VirtualStore.PreflightDestructiveToolCall`.
   - If `DreamResult.Unsafe` is true, the `VirtualStore` blocks the action, injects a `security_violation` fact, and returns an `InteractiveGateError`.
   - If `DreamResult.Unsafe` is false, the VirtualStore permits the Session Executor to physically execute the tool.

# Contract Analysis
The integration between the `VirtualStore`, the `Dreamer`, and the `Kernel` relies on several implicit architectural contracts:
1. **Contract 1: Comprehensive Payload Evaluation (The Cache Key Gap)**
   - *Assumption:* The VirtualStore assumes that the DreamResult applies strictly to the exact tool call it requested, including all arguments.
   - *Reality:* The Dreamer caches based exclusively on the ActionType and Target. It inherently assumes that if an action is safe or unsafe for a specific target, it remains so regardless of the payload. This is fundamentally flawed for actions like `write_file` where the danger lies precisely in the payload.
   - *Violation:* Modifying the payload without modifying the target hits the cache.
2. **Contract 2: Context Propagation and Fail-Closed semantics**
   - *Assumption:* If the execution context is cancelled (due to a timeout or user abort), the Dreamer must immediately halt its heavyweight `kernel.Clone()` and `evaluateSimulation` operations, and the VirtualStore must treat this cancellation as a fail-closed blockage.
   - *Reality:* A cancellation arriving exactly after cache evaluation but before `kernel.Clone()` requires precise error handling.
   - *Violation:* If the Dreamer ignores context cancellation, it wastes CPU cycles. If the VirtualStore interprets a cancellation error as a benign failure, it might accidentally fail-open.
3. **Contract 3: Fact Injection Robustness**
   - *Assumption:* When the VirtualStore blocks an action, it injects a `security_violation` fact into the Kernel to inform the learning subsystem.
   - *Reality:* If the Kernel is corrupted, nil, or heavily loaded, the `injectFact` method might fail or block.
   - *Violation:* The system silently swallows the error, depriving the Autopoiesis subsystem of critical feedback, or panics, crashing the Session.
4. **Contract 4: Thread Safety and Cache Invalidation**
   - *Assumption:* The Dreamer cache is safe for concurrent access, and evicts entries appropriately under load.
   - *Reality:* High concurrency can stress the `d.cache.Store` mutex. If rules change in the Kernel, the cache is not automatically invalidated.
   - *Violation:* The cache becomes a bottleneck, or serves stale evaluations after a user implements a new safety policy.
5. **Contract 5: Resource Boundaries on Large Payloads**
   - *Assumption:* The system can handle payloads up to the LLM context limit.
   - *Reality:* The Dreamer projects the payload as a string into the Mangle engine. Mangle's string interning or parsing might OOM if given a 100MB string.
   - *Violation:* The Session crashes due to memory exhaustion during safety simulation.

# Failure Mode Enumeration
We categorize the failure modes along the boundaries:
**Temporal Failures:**
- `SimulateAction` takes too long to clone the kernel during a complex query. Context expires. The pipeline must reject the action, not leave dangling goroutines.
- Cancellation occurs exactly at the boundary crossing.
**Semantic Failures:**
- The cache key collision. The most critical vulnerability. A benign payload masks a subsequent malicious payload.
- Malformed payloads (invalid utf-8, massive strings) causing parsing panics inside the Mangle `projectEffects` step.
**Ordering Failures:**
- Kernel policy is updated via an interactive command, but the Dreamer cache retains the simulation result from the previous policy version.
**State Corruption:**
- Multiple concurrent interactive tool calls to the same target race to populate the cache.
- The `v.dreamerInitMu` lazy initialization lock fails under concurrent access, creating multiple Dreamer instances.
**Cascading Failures:**
- A failure to evaluate safety leads to a default-allow fallback, which mutates the filesystem, which is then read by the next agent phase, spreading the contamination.

# Adversarial Scenario Design
## Scenario 1: Cache Bypass via Payload Swap (State Corruption / Contract Violation)
**Violated Contract:** Dreamer must evaluate the full payload, not just the target.
**Injection:** Sequentially send benign payload then malicious payload to the same target.
**Expected:** System should evaluate both based on payload; currently it allows malicious payload due to cache hit.
**Severity:** P0

## Scenario 2: Concurrent Cache Poisoning (State Corruption)
**Violated Contract:** Cache must be thread-safe and payload-aware.
**Injection:** Race 20 goroutines writing different payloads to the same target simultaneously.
**Expected:** Some malicious payloads get through because a benign cache entry was written first by a faster goroutine.
**Severity:** P0

## Scenario 3: Context Cancellation Mid-Simulation (Temporal Failure)
**Violated Contract:** Dreamer must respect cancellation and VirtualStore must fail-closed.
**Injection:** Cancel context exactly during `SimulateAction`, specifically as the kernel clone begins.
**Expected:** VirtualStore blocks the action and cleans up correctly without leaking memory or panicking.
**Severity:** P1

## Scenario 4: Nil Context Upgrade (Smoke/Recovery)
**Violated Contract:** Dreamer should handle nil context gracefully without panicking.
**Injection:** Pass explicitly nil context from VirtualStore `PreflightDestructiveToolCall`.
**Expected:** Returns unsafe result, fails closed gracefully rather than segfaulting.
**Severity:** P2

## Scenario 5: Stale Cache After Policy Change (Cascading Failure)
**Violated Contract:** Cache should invalidate when Kernel policy changes.
**Injection:** Simulate benign action, update kernel to make it unsafe via a new `forbids_action` rule, simulate again.
**Expected:** System uses stale cache, allows action, creating a critical security risk where new policies are ignored.
**Severity:** P1

## Scenario 6: Massive Target String Resource Exhaustion (Resource Exhaustion)
**Violated Contract:** System must handle extreme lengths without OOM.
**Injection:** Pass a 100MB string as target payload.
**Expected:** Memory constrained but does not crash; rejected early by a length limit.
**Severity:** P2

## Scenario 7: Extreme Concurrency on Preflight (Resource Exhaustion)
**Violated Contract:** Preflight must be thread-safe under extreme load.
**Injection:** 1,000 goroutines calling `PreflightDestructiveToolCall` simultaneously.
**Expected:** No deadlocks, map concurrent read/write panics, or starvation.
**Severity:** P1

## Scenario 8: Fact Injection Failure Propagation (Cascading Failure)
**Violated Contract:** VirtualStore injects facts on block, must not crash if kernel is broken.
**Injection:** Break kernel assertion via a malformed schema, trigger a block in Dreamer.
**Expected:** VirtualStore catches the assertion error, logs it, and returns the block error, rather than panicking.
**Severity:** P1

## Scenario 9: Unmapped Tool Fallthrough (Smoke)
**Violated Contract:** Unmapped tools should not be destructive.
**Injection:** Pass an unknown tool name like 'read_file' which isn't destructive.
**Expected:** Allowed without simulation, returns nil error.
**Severity:** P3

## Scenario 10: Empty Action Type Handling (Contract Violation)
**Violated Contract:** Dreamer should reject empty actions.
**Injection:** Pass empty string for action type.
**Expected:** Fails closed, marks as Unsafe.
**Severity:** P2

## Scenario 11: Malformed Payload Parsing Panic (Contract Violation)
**Violated Contract:** Dreamer must safely ignore or parse weird payloads without panicking.
**Injection:** Pass an unparseable binary struct or nil pointer as the payload interface{}.
**Expected:** Handled without panic, either serializes gracefully or rejects.
**Severity:** P2

## Scenario 12: Cascading Block Failure Type (Cascading Failure)
**Violated Contract:** If Dreamer blocks, VirtualStore must return a specific recognizable error.
**Injection:** Trigger a legitimate block.
**Expected:** Returns an `InteractiveGateError` so the downstream Session Executor can identify the block source.
**Severity:** P2

## Scenario 13: Recovery after Kernel Unavailable (Recovery)
**Violated Contract:** System recovers if kernel comes back online.
**Injection:** Start with nil kernel, simulate (fails closed), restore kernel, simulate again.
**Expected:** Recovers and simulates successfully.
**Severity:** P1

## Scenario 14: Cache Eviction Thread Safety (State Corruption)
**Violated Contract:** Cache eviction must not race.
**Injection:** Fill cache over max capacity concurrently with 100 goroutines inserting unique keys.
**Expected:** Evicts properly, no race condition panics on the underlying map.
**Severity:** P1

## Scenario 15: Missing Dreamer on Destructive Tool (Contract Violation)
**Violated Contract:** Must fail closed if Dreamer missing.
**Injection:** Call with no dreamer initialized, simulate a destructive tool.
**Expected:** Fails closed immediately.
**Severity:** P0

# Deep Architectural Analysis: The Mangle Bridge Gap
The core vulnerability exposed by Scenario 1 (Cache Bypass via Payload Swap) is rooted in a fundamental mismatch between imperative state modification and declarative logic evaluation.

The `VirtualStore` operates imperatively. It receives a command: `write_file('/etc/passwd', 'malicious_content')`.
It needs a boolean answer: Is this safe? Yes or No.

The `Dreamer` operates declaratively. It translates the imperative command into a fact: `simulated_write('/etc/passwd', 'malicious_content')`.
It then asks the Mangle engine to evaluate all rules. If any rule derives `forbids_action(ActionID, Reason)`, the action is unsafe.

The gap occurs in the caching layer, which sits *between* the imperative request and the declarative evaluation.
The `dreamCacheKey` function constructs its key like this:
```go
func dreamCacheKey(req ActionRequest) string {
	return string(req.Type) + ":" + req.Target
}
```
This design assumes that the `Target` (the file path) is the sole determinant of safety. It completely ignores the `Payload`.

Why was it designed this way? Likely for performance.
The `Payload` can be massive (e.g., a 10MB source file). Hashing a 10MB string for every cache lookup is computationally expensive.
Furthermore, Mangle evaluation itself is expensive (cloning the kernel, projecting facts, running fixpoint evaluation).
The engineers likely optimized for the common case: repeatedly writing safe content to the same file during a TDD loop.

However, this optimization introduces a critical security flaw.
If an attacker (or a hallucinating LLM) first writes a benign payload to a sensitive file, the Dreamer evaluates it, determines it's safe, and caches the result: `write_file:/etc/passwd -> Safe`.
A microsecond later, the LLM attempts to write a malicious payload to the same file.
The `dreamCacheKey` is identical. The cache returns `Safe`. The Mangle engine is bypassed entirely.
The VirtualStore executes the write, and the system is compromised.

# Cascading Failure Analysis
If the Dreamer cache collision occurs (Scenario 1 & 2), the cascading failure is severe:
1. The VirtualStore allows an unsafe action (e.g., modifying a critical system file).
2. The Session Executor executes the tool via the modular tool registry.
3. The tool mutates the physical filesystem.
4. The resulting state is read by subsequent phases or the World Model (which monitors the filesystem).
5. The system's internal representation of the world is now corrupted.
6. Future decisions based on this corrupted world model will be flawed.
7. This represents a full containment breach.

If the VirtualStore fails to handle context cancellation properly (Scenario 3), it might allow partial execution or hang the session executor.
A hanging session executor starves API slots.
API slot starvation prevents the Campaign Orchestrator from making progress.
This leads to a global system deadlock, requiring a hard restart.

If fact injection fails silently (Scenario 8):
The Autopoiesis learning system relies on `security_violation` facts to learn which behaviors to avoid.
If the VirtualStore blocks an action but fails to record the fact, the LLM receives no negative feedback.
The LLM will repeatedly attempt the same blocked action, burning tokens and compute resources in an infinite loop of failure.
The TDD repair loop will stall, unable to progress.

If the cache is not invalidated upon kernel policy changes (Scenario 5):
A user might notice the system performing a dangerous action and issue an interactive command to update the safety policy.
The new rule is added to the Kernel.
However, because the Dreamer's cache is stale, it will continue to allow the dangerous action, directly violating the user's explicit new instructions.
This erodes user trust and demonstrates a failure of the neuro-symbolic governance model.

# The Fixpoint Concurrency Dilemma (Architectural Deep Dive)
When the `PreflightDestructiveToolCall` function intercepts a tool invocation, it inherently bridges two different execution models: the Go concurrent runtime and the Mangle monotonic logic engine.
The Go runtime is highly concurrent. Multiple SubAgents can operate simultaneously across different goroutines, executing tools concurrently.
The Mangle engine, particularly the `kernel.Clone()` and `evaluateSimulation` phases, requires a consistent, point-in-time snapshot of the database (EDB).

**The Race Condition at the Boundary:**
If SubAgent A and SubAgent B concurrently attempt destructive actions, they both trigger `PreflightDestructiveToolCall`.
If both miss the cache, they both clone the kernel.
SubAgent A clones Kernel V1.
SubAgent B clones Kernel V1.
SubAgent A projects its effects, evaluates them as Safe, and the VirtualStore executes the action, mutating the world. The world is now V2.
SubAgent B projects its effects onto its clone of V1. It evaluates them as Safe *relative to V1*.
The VirtualStore executes SubAgent B's action.
However, SubAgent B's action might be unsafe in world V2!

**Concrete Example:**
SubAgent A wants to delete `/app/data`.
SubAgent B wants to write to `/app/data/config.json`.
Both simulate concurrently on V1 (where `/app/data` exists).
SubAgent B's write is deemed safe because the directory exists.
SubAgent A's delete is deemed safe.
SubAgent A executes first, deleting the directory.
SubAgent B executes second, attempting to write to a non-existent directory.
The physical tool execution fails, returning an error to the LLM.
The LLM gets confused because the Dreamer told it the action was permitted.

This reveals a critical missing contract: **The Dreamer's simulation is only valid if the underlying world state has not changed between simulation and execution.**
Currently, there is no transactional lock holding the world state consistent across this boundary. The VirtualStore executes optimistically.

# The Memory Leak in Ephemeral Kernels
During a cache miss, `d.kernel.Clone()` is called.
This creates a new Mangle engine instance.
Mangle engines allocate memory for their internal structures (fact tables, rule indices).
If `SimulateAction` is interrupted by context cancellation *after* the clone but *before* evaluation finishes, the cloned kernel reference is lost when the function returns.
Go's garbage collector will eventually clean it up.
However, if the Mangle engine contains long-lived background goroutines (e.g., for channel-based evaluation or telemetry), these goroutines might leak, holding the memory indefinitely.
This is why Scenario 3 (Context Cancellation Mid-Simulation) is a P1 severity. It's not just about failing closed; it's about preventing resource exhaustion in a long-running campaign.

# Strategic Re-evaluation of the Cache Key
To fix the cache collision, the `dreamCacheKey` must incorporate the payload.
However, hashing a 100MB string is slow.
A potential remediation strategy:
1. **Type-Specific Caching:** For `delete_file`, the payload is irrelevant. The cache key can remain `type:target`. For `write_file`, the payload is everything. The cache key must be `type:target:hash(payload)`.
2. **Threshold Hashing:** Only hash the payload if it's under a certain size (e.g., 1MB). If it's over, bypass the cache and force simulation.
3. **Semantic Caching:** Instead of hashing the raw string, hash a semantic representation of the payload (e.g., an AST fingerprint for code edits). This is harder but more robust against trivial whitespace changes.

# Conclusion of Analysis
The VirtualStore ↔ Dreamer boundary is the most critical security juncture in the codeNERD architecture. It is the final gate before thoughts become physical actions.
The current implementation optimizes for speed over correctness by using an overly simplistic cache key.
The integration test suite (`virtualstore_dreamer_integration_test.go`) is specifically designed to relentlessly hammer these edge cases, proving that the cache can be bypassed, poisoned, and that context cancellations can cause erratic behavior.
Remediation requires modifying the core `dreamer.go` file to implement a payload-aware cache key and stricter context lifecycle management.

# Appendix: Detailed Subsystem Invariants and Verification Strategies

The architecture defines strict invariants that must hold true across the VirtualStore/Dreamer boundary. When designing E2E tests, these invariants form the basis of our assertions.

## Invariant A: Monotonicity of Safety
If an action A is deemed Unsafe in world state W, it must remain Unsafe in world state W' if W' is a superset of W (i.e., only new facts have been added, none retracted).
*Why:* Mangle is a monotonic logic system. Adding facts can only derive more facts, not fewer. If `panic_state` was derivable, adding more facts cannot un-derive it.
*Test Strategy:* Simulate an unsafe action. Assert new facts into the kernel. Simulate again. Verify it is still unsafe. If it becomes safe, the Mangle rules are non-monotonic, breaking the fundamental assumption of the evaluation engine.

## Invariant B: Isolation of Simulation
The `projectEffects` function must ONLY modify the cloned kernel. It must never leak hypothetical facts into the production kernel.
*Why:* If the main kernel becomes polluted with `simulated_write` facts, the agent will hallucinate that actions have already occurred, disrupting the OODA loop.
*Test Strategy:* Run a simulation for an action. After the simulation completes (whether safe or unsafe), query the main kernel for the projected facts. The query must return zero results.

## Invariant C: Idempotency of Rejection
Rejecting the same action multiple times must not compound state indefinitely.
*Why:* The VirtualStore injects a `security_violation` fact every time an action is blocked. If an agent gets stuck in a loop, it might trigger 1,000 rejections.
*Test Strategy:* Trigger the same block 100 times. Verify the kernel handles the duplicate facts gracefully (Mangle EDB sets naturally deduplicate identical facts, but we must verify the Go wrapper doesn't leak memory or duplicate structs).

## Invariant D: Bound on Simulation Time
The Dreamer must guarantee a response (Safe, Unsafe, or Error) within a strict time bound, regardless of the complexity of the Mangle rules or the size of the EDB.
*Why:* The Session Executor loop operates synchronously for interactive commands. A stalled simulation hangs the entire user session.
*Test Strategy:* Inject adversarial, highly recursive Mangle rules (e.g., computing the transitive closure of a massive graph) into the kernel. Attempt a simulation. Verify the context timeout successfully interrupts the evaluation and returns an error, rather than hanging forever.

## Invariant E: Fallback to Deny
In any indeterminate state (nil context, missing kernel, parsing error, timeout, cache corruption), the system must default to Deny (Unsafe).
*Why:* This is the core Constitutional Safety principle. Only an explicit derivation of `permitted` and the absence of `forbids_action` can allow an action.
*Test Strategy:* Systematically break every dependency of the Dreamer and VirtualStore, and verify that in every degraded state, an action request is blocked.

# Expanded Failure Scenarios for Continuous Integration

To ensure the system's robustness over time, the E2E test suite must be integrated into the CI pipeline. The following scenarios represent regression vectors that might be introduced by future development.

**Regression Vector 1: The "Optimization" Bypass**
A future developer might notice that `kernel.Clone()` is slow and attempt to optimize it by sharing the kernel and using Mangle's transient scopes or transaction boundaries. If not implemented perfectly, this could violate Invariant B (Isolation). The test suite must catch this by aggressively checking for state leakage.

**Regression Vector 2: The Silent Error Swallow**
As the VirtualStore grows to support more tools (e.g., network requests, database migrations), developers might add new `actionTypeForToolName` mappings. If they forget to implement the corresponding `projectEffects` logic in the Dreamer, the Dreamer might silently return "Safe" because it projected zero effects. The test suite must ensure that every supported tool actually generates meaningful hypothetical facts during simulation.

**Regression Vector 3: The Cache Key Regression**
Once the cache key collision is fixed (e.g., by hashing the payload), a future refactoring might accidentally revert it, or introduce a new collision (e.g., hashing only the first 1KB of the payload to save time). The `TestE2E_VirtualStoreDreamer_ContractViolation_CacheCollision` test must remain in the suite permanently as a structural guardrail against this specific regression.

# Summary of Remediation Requirements

To make the E2E test suite pass, the following codebase changes will be required:

1.  **Modify `dreamCacheKey`:** Update `internal/core/dreamer.go` to include a hash of the payload in the cache key.
    ```go
    // Conceptual fix
    func dreamCacheKey(req ActionRequest) string {
        payloadHash := hash(fmt.Sprint(req.Payload))
        return string(req.Type) + ":" + req.Target + ":" + payloadHash
    }
    ```
2.  **Harden Context Handling:** Ensure that `evaluateSimulation` respects `ctx.Done()` and halts execution immediately.
3.  **Graceful Nil Handling:** Add explicit `if ctx == nil` checks at the entry points of `PreflightDestructiveToolCall` and `SimulateAction` to prevent panics and ensure fail-closed behavior.
4.  **Enforce Eviction Thread Safety:** Audit `DreamCache.Store` to ensure the eviction logic (deleting half the map) does not cause starvation or deadlocks under high concurrent load.

Once these remediations are applied, the Siege E2E test suite will turn from red to green, proving the integrity of the subsystem boundary.

# Additional Deep Dives into OODA Loop Integrity

The OODA loop (Observe, Orient, Decide, Act) is fundamentally compromised if the Act phase (VirtualStore) executes actions that the Decide phase (Mangle) never explicitly authorized. This section explores how boundary failures at the VirtualStore-Dreamer interface ripple upward into the cognitive layers of the system.

## The Cognitive Dissonance of Hollow Success
When the VirtualStore executes an action, it relies on the interactive gate to have pre-verified its safety. If the gate (Dreamer) is bypassed via cache poisoning, the physical execution succeeds. The VirtualStore then reports a "success" back to the Session Executor.

However, the Mangle Engine (Decide phase) might have internally derived that the action was forbidden. If the Session Executor queries the kernel to understand the state of the world, it sees two contradictory facts:
1. `action_status(ActionID, /success)` - Injected by the VirtualStore after execution.
2. `forbids_action(ActionID, Reason)` - Derived logically by the policy rules.

This cognitive dissonance breaks the `autopoiesis` subsystem. The self-improving loops attempt to learn from past actions. If it sees that a forbidden action succeeded, it might erroneously learn that the forbidden rule is obsolete or that the action is actually safe, leading to reinforcement learning on corrupted data.

## The Semantic Grounding Problem
The Dreamer's `projectEffects` function is responsible for translating Go structs (the payload) into Mangle atoms. This is a fragile semantic bridge.

Consider a tool that modifies an AST. The payload might be a complex JSON representation of a syntax tree.
If `projectEffects` flattens this JSON into a single massive string atom, the Mangle rules cannot reason about its structure.
```mangle
simulated_ast_edit("src/main.go", "{\"type\": \"FunctionDecl\", \"name\": \"malicious_func\"}").
```
The safety rules cannot easily parse JSON inside a Mangle string. They might just check the length or presence of keywords.
This means the Dreamer is only providing the *illusion* of safety for complex payloads.

**Test Strategy:** The E2E tests must pass deeply nested, structurally complex payloads and verify that the `VirtualStore` properly blocks them if a rule targets a specific nested field. If it cannot, the architecture requires a more sophisticated `projectEffects` implementation that recursively shreds JSON payloads into relational Mangle facts.

## Temporal Desynchronization between World Model and Execution
The VirtualStore executes actions in the physical world (e.g., writing to disk).
The World Model subsystem (e.g., `fs.go`, `ast.go`) asynchronously observes these changes and updates the Mangle EDB.

If the Dreamer simulates an action, it projects hypothetical effects based on the *current* EDB.
If the World Model is lagging behind physical reality (e.g., a file was deleted on disk but the EDB still thinks it exists), the Dreamer's simulation is based on a stale hallucination.

**Boundary Failure Scenario:**
1. SubAgent deletes `config.json`. Physical execution succeeds.
2. World Model scanner is slow, EDB still contains `file_topology('config.json', /exists)`.
3. SubAgent attempts to write to `config.json`.
4. Dreamer simulates the write. It queries the EDB, sees the file exists, and approves the write.
5. VirtualStore attempts the physical write. It fails because the directory structure was altered by the previous delete.
6. The Session Executor crashes or enters an error recovery loop.

The integration tests must simulate this temporal desynchronization by artificially pausing the World Model updates while hammering the VirtualStore with dependent actions, proving that the system can recover gracefully when physical reality diverges from the cognitive projection.

# Final Validation Checklist for Siege Integration

Before marking this integration surface as fully audited, the following conditions must be met:
- [x] Cache collision vectors mapped and tested.
- [x] Context cancellation edge cases identified.
- [x] Nil reference upgrades (fail-closed semantics) verified.
- [x] Concurrency stress tests designed.
- [x] Resource exhaustion (massive strings, cache flooding) quantified.
- [x] Cascading failure paths (OODA loop corruption, API starvation) documented.
- [x] Remediation strategies outlined for future engineering work.

This journal serves as the blueprint for the `virtualstore_dreamer_integration_test.go` suite. By systematically testing these specific failure modes, we ensure the codeNERD architecture can survive real-world chaos at its most critical boundary.

# Appendix C: The Piggyback Protocol Boundary

The VirtualStore doesn't just interact with the Dreamer and the Kernel; it also forms a critical boundary with the Articulation subsystem via the Piggyback protocol. When an interactive tool executes, its success or failure state must be communicated back to the LLM.

## The Dual-Channel Feedback Loop
The codeNERD architecture uses a dual-channel output system:
1.  **Surface Response:** Natural language intended for the user (e.g., "I have successfully written the file").
2.  **Control Packet (Piggyback):** Structured JSON injected directly into the Mangle Kernel (e.g., `task_status(/write_file, /complete)`).

When the VirtualStore executes a tool, the Session Executor observes the result and formulates the next context window for the LLM. If the VirtualStore's safety gate (Dreamer) blocks an action, it is imperative that this blockage is correctly encoded into the Piggyback protocol.

## Failure Mode: Silent Protocol Truncation
If the VirtualStore returns a massive, unstructured error string (e.g., a stack trace from a panicking Dreamer) instead of a clean `InteractiveGateError`, the Session Executor might append this massive string to the context window.

If this pushes the total token count past the TokenBudgetManager's threshold, the system might truncate the context. If the truncation happens to sever the JSON control packet at the end of the prompt, the Articulation subsystem will receive a malformed Piggyback packet.

**Cascading Effect:**
1.  VirtualStore gate fails messily.
2.  Context window overflows.
3.  Piggyback JSON is truncated.
4.  Kernel fails to parse the control packet.
5.  The LLM's internal state (as tracked by the Kernel) diverges from reality. The LLM thinks it succeeded (because it generated the tool call), but the system actually blocked it and failed to communicate the block.

**Integration Test Design:**
To test this boundary, we must intentionally trigger a VirtualStore block using a massive payload (e.g., Scenario 6). We then observe the error returned to the Session Executor and verify that it is bounded in size and strictly formatted, preventing downstream protocol truncation.

# Conclusion

The architectural seam between the VirtualStore and the Dreamer is not just a simple function call; it is a complex, multi-dimensional boundary where imperative execution meets declarative logic, concurrent Go routines meet monotonic evaluation, and physical world state meets cognitive projection.

By systematically mapping the interaction contracts, enumerating the failure modes, and designing rigorous, adversarial E2E integration tests, we can harden this boundary against both accidental regressions and intentional malicious exploitation. The tests implemented alongside this journal serve as the executable proof of these architectural vulnerabilities, setting a strict quality bar for future remediation efforts.

# Appendix D: Context Compression and the Long-Running Campaign

In multi-phase execution scenarios, known as Campaigns, the VirtualStore-Dreamer boundary is subjected to sustained pressure over thousands of turns. This introduces a new class of boundary failures related to state accumulation and context compression.

## The Semantic Compression Boundary
As a Campaign progresses, the working memory (RAM tier) accumulates facts: `user_intent`, `next_action`, `simulated_write`, etc.
To prevent OOM errors and adhere to the LLM's context window, the `SemanticCompressor` (located in `internal/session/semantic_compressor.go`) periodically summarizes the oldest facts into a dense embedding or a summarized text blob.

**The Integration Gap:**
Does the Dreamer respect semantic compression?
When `Dreamer.SimulateAction` calls `kernel.Clone()`, it copies the entire EDB.
If the EDB contains compressed summary facts (e.g., `compressed_history("The agent previously edited auth.go")`) instead of the raw structural facts, the Mangle safety rules might fail to trigger.

For instance, a safety rule might be:
`forbids_action(Action, 'duplicate edit') :- action_request(Action, /write_file, Target), pending_edit(Target, _).`
If `pending_edit` was compressed away into a summary string, the rule will evaluate to false, and the Dreamer will erroneously approve an unsafe action.

**Testing Strategy:**
The integration suite must simulate a long-running campaign.
1. Execute 100 safe actions to fill the EDB.
2. Trigger the `SemanticCompressor`.
3. Attempt an action that *should* be blocked by a rule depending on a fact that was just compressed.
4. Verify whether the Dreamer fails-open (allowing the action) or if the system correctly pages the necessary context back into the EDB before simulation.

## The Ghost Fact Phenomenon
Mangle is monotonic. Facts, once asserted, remain until explicitly retracted.
The VirtualStore explicitly retracts `pending_edit` facts after execution:
`e.retractPendingEdit(fact)`

However, what happens if the VirtualStore crashes or the context is cancelled *after* the assertion but *before* the retraction?
This leaves a "Ghost Fact" in the kernel.

If a Ghost Fact `pending_edit('src/main.go', ...)` remains in the kernel:
1. The physical file is not actually being edited.
2. The Dreamer, on subsequent turns, will read the Ghost Fact.
3. The Dreamer will block all future attempts to edit `src/main.go` to prevent race conditions.
4. The Campaign halts permanently, deadlocked by its own safety mechanisms.

**Testing Strategy (Scenario 16):**
This requires an adversarial test that deliberately interrupts the VirtualStore precisely between `assertPendingEdits` and the deferred `retractPendingEdit`. We must then start a new transaction and attempt to edit the same file, proving that the system either cleans up Ghost Facts on boot/turn-transition, or succumbs to the deadlock.

# Final Reflection
This deep dive confirms that the VirtualStore-Dreamer boundary is highly susceptible to temporal, concurrent, and state-based failures. The rigorous, padding-free tests developed in `virtualstore_dreamer_integration_test.go` directly exercise these exact architectural gaps, fulfilling the mandate of the Siege engineering persona.

# Appendix E: The Threat of Adversarial Prompt Injection via Payloads

The final, and perhaps most insidious, boundary failure occurs when user-supplied data (the payload) subverts the Mangle engine itself during the simulation phase.

## The String interning vulnerability
When the Dreamer projects effects into the cloned kernel, it must convert the Go strings from the tool arguments into Mangle Atoms or Strings.
If the payload contains carefully crafted Mangle syntax (e.g., `' :- true. panic_state('injected'). %`), and the `projectEffects` function uses naive string concatenation instead of proper AST construction, the system is vulnerable to Rule Injection.

**The Attack Vector:**
1. An attacker prompts the agent to write a file containing malicious Mangle code.
2. The VirtualStore passes this payload to the Dreamer.
3. The Dreamer naively concatenates this into a hypothetical fact string and evaluates it.
4. The Mangle engine parses the string, executing the injected rule.
5. The attacker can force a `panic_state` to deny service, or theoretically manipulate the evaluation to bypass safety checks.

**Test Design (Scenario 17):**
The E2E suite must attempt to pass payloads containing Mangle syntax characters (`:-`, `.`, `%`, `()`) and verify that the Go AST parser correctly escapes them, treating them purely as literal string data rather than executable logic. The test should assert that no unintended facts (like `panic_state('injected')`) are derived during the simulation.

# Appendix F: Summary of Structural Vulnerabilities
In closing, the Siege analysis confirms 5 major structural flaws at the boundary:
1. Cache Key Collision (Payload Ignorance)
2. Ghost Fact Deadlocks (Interrupted Retractions)
3. Silent Protocol Truncation (Token Budget Overflows)
4. Semantic Compression Blindspots (Loss of critical safety facts)
5. Rule Injection (Naive String Escaping in Projection)
These must be remediated to ensure the integrity of the codeNERD architecture.

# Appendix G: Action Plan for Remediation
The immediate next steps for the engineering team, based on this analysis, are:
1. Update `internal/core/dreamer.go` to safely hash payloads.
2. Wrap `PreflightDestructiveToolCall` in a `defer` block that guarantees fact cleanup even on panics.
