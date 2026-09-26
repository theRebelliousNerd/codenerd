---
surface: "InteractiveGate_Dreamer_Kernel"
mode: "boundary"
subsystems_tested: ["VirtualStore", "Dreamer", "Kernel"]
blast_radius: "critical"
remediated: false
---

# 🏰 SIEGE INTEGRATION ANALYSIS: InteractiveGate ↔ Dreamer ↔ Kernel

## 1. System Interaction Map

This analysis targets the boundary between `VirtualStore.PreflightDestructiveToolCall`, the `Dreamer.SimulateAction`, and the `Kernel` (specifically the cloned sandbox).

**Critical Path:**
1.  `VirtualStore.PreflightDestructiveToolCall(ctx, actionID, toolName, args)` (internal/core/virtual_store_interactive_gate.go)
2.  `VirtualStore` looks up `ActionType` via `actionTypeForToolName(toolName)`.
3.  `buildInteractiveActionRequest(actionID, at, args)` prepares the request.
4.  `VirtualStore.getDreamer()` fetches the dreamer.
5.  `Dreamer.SimulateAction(ctx, req)` (internal/core/dreamer.go) is invoked.
6.  `Dreamer` validates constraints (target length, missing kernel, missing critical_path_prefix facts).
7.  `Dreamer.projectEffects(kernel, actionID, req)` builds hypothetical facts.
8.  `Dreamer.evaluateProjection(kernel, actionID, projected)` clones the kernel (`kernel.Clone()`).
9.  `clone.Assert(fact)` injects projected facts.
10. `clone.Evaluate()` triggers the logic engine.
11. `clone.Query("panic_state(?)")` checks for violations.
12. Results return up to `VirtualStore`.
13. If blocked, `VirtualStore` injects `security_violation` and `dream_blocked_action` facts into the *main* kernel.

## 2. Contract Analysis

The integrity of the defensive system relies on the following implicit and explicit contracts:

*   **Contract A: Fail-Closed Mutability.** The VirtualStore *must* receive a blocked result (`Unsafe: true`) if the Dreamer fails, the Kernel is unavailable, or the simulation context expires. If the Dreamer panics or hangs, the gate must not fail-open.
*   **Contract B: Context/Cancellation Synchronicity.** If a timeout or cancellation (`ctx.Done()`) occurs during `SimulateAction`, the Dreamer must instantly abort and return `Unsafe: true`. It must not continue evaluating against a stale clone.
*   **Contract C: Sandbox Isolation.** `kernel.Clone()` must create a mathematically perfect isolation boundary. Asserting `projected_action` facts into the clone *must not* affect the parent kernel's fact base. If isolation leaks, speculative actions become real state.
*   **Contract D: Target Length Enforcement.** The Dreamer imposes a strict 4096-byte limit on the `req.Target` string. Payloads that exceed this limit are summarily rejected to prevent Mangle engine OOMs or regex DoS attacks.
*   **Contract E: Blocking Feedback Loop.** When the Dreamer blocks an action, the VirtualStore is responsible for injecting `security_violation` into the main Kernel. The main Kernel must accept this fact; if the Kernel rejects it (e.g., schema mismatch), the learning subsystems (Autopoiesis/TDD) will never know why the action failed.

## 3. Failure Mode Enumeration

*   **Temporal:** `SimulateAction` takes longer than the `VirtualStore` expects. A context timeout occurs, but the `VirtualStore` mistakenly proceeds (or panics) instead of treating the timeout as a block.
*   **Semantic:** The `VirtualStore` sends an `ActionRequest` with a seemingly benign target (e.g., `file.txt`), but the *payload* (which the cache might ignore or mishandle) contains a malicious shell injection.
*   **Ordering:** The main Kernel's schema updates (e.g., adding `panic_state` rules) arrive *after* `kernel.Clone()` is called, meaning the Dreamer evaluates the action against a stale safety policy.
*   **Partial:** `projectEffects` successfully generates 5 facts, but `clone.Assert()` fails on the 3rd fact. The clone evaluates a partial projection, potentially missing the `panic_state` trigger.
*   **Corruption:** Two concurrent `VirtualStore` threads call `SimulateAction` for the exact same target string but different payloads. If the Dreamer caches safety results by target alone, one payload inherits the safety verdict of the other.

## 4. Adversarial Scenario Design

### P0 - Critical State Leaks
*   **Scenario 1: Sandbox Isolation Leak via Deep Pointers.**
    *   *Contract:* Sandbox Isolation.
    *   *Mechanism:* The `kernel.Clone()` method performs a shallow copy of internal maps. When `evaluateProjection` adds a fact to the clone, it inadvertently appears in the parent kernel.
    *   *Expected Behavior:* Parent kernel queries for the projected fact return empty.
*   **Scenario 2: Concurrent Target Cache Collision Bypass.**
    *   *Contract:* Semantic evaluation integrity.
    *   *Mechanism:* Fire two concurrent `PreflightDestructiveToolCall` goroutines. Request A modifies `main.go` with harmless text. Request B modifies `main.go` with `os.Exit(1)`.
    *   *Expected Behavior:* The Dreamer evaluates both payloads independently; Request B is blocked by `panic_state` rules, even though they share the same target.

### P1 - Gate Bypasses and Fail-Open Flaws
*   **Scenario 3: Missing Critical Path Facts Bypass.**
    *   *Contract:* Fail-Closed on missing prerequisites.
    *   *Mechanism:* Instantiate a Dreamer without asserting `critical_path_prefix`. Send a highly destructive action (e.g., `delete_file /`).
    *   *Expected Behavior:* The Dreamer immediately returns `Unsafe: true` because it cannot guarantee the `panic_state` rules will fire without the path prefixes.
*   **Scenario 4: Context Cancellation Yields Fail-Open.**
    *   *Contract:* Fail-Closed Context Synchronicity.
    *   *Mechanism:* Cancel the context precisely during `clone.Evaluate()`. The `SimulateAction` `select { case <-ctx.Done(): ... }` block must intercept the cancellation.
    *   *Expected Behavior:* Dreamer returns `Unsafe: true` with a timeout reason; it must not return `Unsafe: false` by skipping the evaluation.
*   **Scenario 5: Nil Dreamer Interface Call.**
    *   *Contract:* Fail-Closed Mutability.
    *   *Mechanism:* `VirtualStore.getDreamer()` is rigged to return `nil`. `PreflightDestructiveToolCall` is invoked for an `ActionWriteFile`.
    *   *Expected Behavior:* `PreflightDestructiveToolCall` returns a non-nil `InteractiveGateError` stating the dreamer is unavailable.
*   **Scenario 6: Oversized Target Panic Exploitation.**
    *   *Contract:* Target Length Enforcement.
    *   *Mechanism:* Send an action with a `Target` string of 10,000 characters.
    *   *Expected Behavior:* The Dreamer catches the length violation at the validation stage and blocks the action *without* calling `kernel.Clone()` or evaluating rules.
*   **Scenario 7: Malformed Payload Fact Assertion Failure.**
    *   *Contract:* Full Projection Evaluation.
    *   *Mechanism:* Provide an `args` map containing deeply nested interfaces or un-stringifiable cyclic structs that `projectEffects` cannot convert to Mangle facts.
    *   *Expected Behavior:* If projection fails, the action is deemed unsafe. It must not evaluate an empty projection.

### P2 - Feedback Loop Breakages
*   **Scenario 8: Blocked Action Fact Injection Rejection.**
    *   *Contract:* Blocking Feedback Loop.
    *   *Mechanism:* The Dreamer correctly blocks an action. The VirtualStore attempts to inject `dream_blocked_action` and `security_violation`. The Kernel's schema is mocked to reject these predicates.
    *   *Expected Behavior:* The `VirtualStore` logs the injection failure but *still* returns an error to block the tool execution. The rejection must not cause the gate to succeed.
*   **Scenario 9: Unmapped Destructive Tool Name.**
    *   *Contract:* Default-Deny on unmapped effects.
    *   *Mechanism:* Invoke `PreflightDestructiveToolCall` with `toolName = "rm_rf_everything"`.
    *   *Expected Behavior:* The gate returns an error immediately because it cannot map the tool to an `ActionType`.
*   **Scenario 10: Multi-File Target Decomposition Failure.**
    *   *Contract:* Transactional tool (e.g., `apply_edits`) preflighting.
    *   *Mechanism:* Invoke a multi-file tool with an empty `paths` list or invalid JSON.
    *   *Expected Behavior:* The VirtualStore rejects the call before even reaching the Dreamer, because it cannot resolve the discrete targets.

### P3 - Edge Cases and Volume
*   **Scenario 11: Extreme Concurrent Simulation Load.**
    *   *Contract:* Resource management.
    *   *Mechanism:* Fire 500 concurrent goroutines calling `SimulateAction`.
    *   *Expected Behavior:* The system throttles or processes them without OOMing or triggering race conditions in the `RealKernel` cloning process.
*   **Scenario 12: Empty Action Type.**
    *   *Contract:* Validation layer robustness.
    *   *Mechanism:* Send an `ActionRequest` with `Type: ""`.
    *   *Expected Behavior:* Blocked at validation.
*   **Scenario 13: Non-Stringifiable Arguments in Payload.**
    *   *Contract:* Projection resilience.
    *   *Mechanism:* Pass a channel or function pointer in the `args` payload.
    *   *Expected Behavior:* Projection degrades gracefully or blocks; does not panic the Go runtime.
*   **Scenario 14: Nil Context into Preflight.**
    *   *Contract:* Basic API safety.
    *   *Mechanism:* Call `PreflightDestructiveToolCall(nil, ...)`
    *   *Expected Behavior:* Immediate `InteractiveGateError`.
*   **Scenario 15: Post-Action Validation on Missing Store.**
    *   *Contract:* Post-flight safety.
    *   *Mechanism:* Call `ValidateInteractiveToolResult` with a nil validator store.
    *   *Expected Behavior:* Fails open (returns nil error) because validators only add certainty, but logs a warning.

## 5. Cascading Failure Analysis

If the `InteractiveGate` fails open (e.g., a bug in `kernel.Clone()` causes a safety rule to be missed), the following cascade occurs:
1.  **Destructive Execution:** The VirtualStore executes the action (e.g., deleting a critical source file).
2.  **Kernel Blindness:** Because the Dreamer didn't block it, the VirtualStore injects `task_complete` instead of `security_violation`.
3.  **Learned Catastrophe:** The `TDDLoop` or `Ouroboros` loop sees the `task_complete` fact and associates the destructive action with success. It learns that deleting source files is a valid way to fix compiler errors.
4.  **Campaign Failure:** Subsequent phases of a Campaign orchestrator fail because the files they expect are gone.

If the `InteractiveGate` fails closed permanently (e.g., `critical_path_prefix` is never asserted):
1.  **Paralysis:** Every file write, edit, or shell command is blocked by the Dreamer.
2.  **Infinite Retry:** The `JITExecutor` sees continuous `security_violation` facts. It attempts to use alternative tools, which are also blocked.
3.  **Budget Exhaustion:** The session loop burns through its token budget in an infinite loop of failed actions until it terminates.

## 6. Architectural Implications and Traces

### The Clone Operation Deep Dive
When `kernel.Clone()` is called, it must meticulously duplicate the internal state of the `RealKernel`. The `RealKernel` is not a simple struct; it encapsulates the Mangle engine, a fact store, a rule base, and potentially an embedding connection.
1. **Fact Store:** The clone must have a completely independent fact store. If it merely references the original fact store's underlying map or slice, any `Assert` operation in the clone (such as adding the `projected_action` facts) will pollute the main session's working memory.
2. **Rule Base:** The rules (specifically the `panic_state` constitution rules) can be shared by reference because they are immutable during evaluation. However, the clone must be able to evaluate these rules against its isolated fact store.
3. **Evaluation Engine:** The Mangle engine itself must be re-initialized or safely cloned to ensure no internal fixpoint state is leaked between the parent and the clone.

If the clone operation is flawed (Scenario 1), the consequences are severe. A leaked `projected_action` fact into the main kernel might trick the `VirtualStore` into believing an action was already executed, or worse, it might trigger real `panic_state` rules in the main kernel during its next evaluation cycle, permanently paralyzing the session.

### The Caching Strategy and Collisions
The `Dreamer` might employ caching to avoid the expensive `kernel.Clone()` and `Evaluate()` operations for identical actions.
1. **Cache Key Construction:** If the cache key is solely `string(req.Type) + ":" + req.Target` (e.g., `write_file:/etc/passwd`), it ignores the `req.Payload` (the content to be written).
2. **The Exploit:** An attacker (or a malfunctioning subagent) could first send a benign payload to `/etc/passwd`. The `Dreamer` evaluates it, finds it safe, and caches the `Unsafe: false` result. Immediately after, the agent sends a malicious payload to `/etc/passwd`. The `Dreamer` hits the cache based on the target and type, skipping evaluation and incorrectly returning `Unsafe: false`.
3. **The Fix:** The cache key must deterministically hash the entire `req` object, including the `Payload`, to ensure distinct payloads are evaluated independently.

### The Feedback Loop Integrity
The `VirtualStore`'s responsibility doesn't end with blocking the action. It must inform the rest of the system *why* the action was blocked. It does this by asserting `security_violation` and `dream_blocked_action` facts into the main kernel.
1. **Schema Mismatch:** If the main kernel's schema doesn't define these predicates, the `Assert` operation will fail.
2. **Silent Failure:** If the `VirtualStore` ignores the `Assert` error and just returns the block to the Go caller, the Mangle-based reasoning loop (e.g., the `TDDLoop`) never sees the failure. It might think the action succeeded but had no effect, leading to infinite retry loops.
3. **The Contract:** The `VirtualStore` must guarantee that blocking feedback is durable in the kernel.

### The Temporal Dimension
The `context.Context` passed from the session orchestrator down to the `Dreamer` represents the user's patience and the system's resource budget.
1. **Cancellation Propagation:** If the user cancels the session (e.g., via Ctrl+C in the CLI), the context is canceled.
2. **The Race:** If the cancellation happens *while* the cloned kernel is running `Evaluate()`, the engine must halt immediately.
3. **The Result:** The `Dreamer` must intercept this cancellation and return `Unsafe: true` with a timeout/cancel reason. It must *never* return `Unsafe: false` simply because the evaluation didn't finish finding a violation. Fail-closed is absolute.

### Multi-File Transaction Preflighting
Tools like `apply_edits` or `repoint` are transactional; they modify multiple files. The `VirtualStore` handles this by decomposing the transaction and calling `PreflightDestructiveToolCall` recursively for each file (treating each as an `edit_file` operation).
1. **Atomic Preflight:** The entire transaction must fail if *any single file* fails the Dreamer preflight.
2. **State Consistency:** If file A passes, but file B fails, the transaction is blocked. This is correct. However, if the preflight for file A somehow leaked state into the kernel (see Clone deep dive), that leaked state must not persist.
3. **Performance:** Running a full kernel clone and evaluation for every file in a 50-file refactoring operation is computationally expensive. The system must handle this load without timing out, or it must optimize the preflight to run multiple projections in a single batch simulation.

## 7. Remediation Strategy

To harden this boundary, the following architectural adjustments should be verified or implemented:

1.  **Deep Clone Audit:** Rigorously review the `kernel.Clone()` implementation. Ensure that the fact store uses a copy-on-write or deep-copy mechanism. A simple map assignment is insufficient and constitutes a P0 vulnerability.
2.  **Payload-Aware Caching:** If the Dreamer caches results, the cache key generation must be updated. Use a cryptographic hash (e.g., SHA-256) of the serialized `ActionRequest` struct, ensuring all fields (Type, Target, Payload) contribute to the hash.
3.  **Strict Context Adherence:** Audit the `evaluateProjection` method. Ensure it listens to `ctx.Done()` during the evaluation loop. If the Mangle engine itself doesn't support context cancellation, it must be wrapped in a goroutine that allows the parent to abandon it and return `Unsafe: true` immediately.
4.  **Target Truncation:** Enforce the 4096-byte target limit consistently at the very edge of `PreflightDestructiveToolCall`, before any Mangle string atoms are constructed, to protect the engine from memory exhaustion.
5.  **Schema Verification Tests:** Add automated tests that explicitly verify the main kernel can accept `security_violation` and `dream_blocked_action` facts without error, ensuring the feedback loop remains intact across schema evolutions.
6.  **Load Testing the Gate:** Introduce integration tests that hammer the `VirtualStore` with hundreds of concurrent preflight requests to ensure the cloning and evaluation mechanisms don't cause goroutine leaks or deadlocks.

### Deep Dive into the InteractiveGate Boundary Flow Control
The `InteractiveGate` manages the flow between when an action is proposed by the LLM and when it is executed on the real filesystem.
This flow consists of three distinct phases: `Preflight`, `Execution`, and `Validation`.

**Phase 1: Preflight (The Dreamer Simulation)**
During `PreflightDestructiveToolCall`, the `VirtualStore` constructs an `ActionRequest`. The critical logic here revolves around the `ActionType`.
1. **Implicit Contracts on ActionType**: The system maps string `toolName` values (like `write_file`) to strongly typed `ActionType` enums (like `ActionWriteFile`). The Dreamer heavily relies on these enums. If the `VirtualStore` fails to map a tool correctly, it either defaults to a generic type or fails the request. A mismapped type could lead the Dreamer to simulate the wrong side-effects.
2. **Payload Serialization**: The `ActionRequest.Payload` is a `map[string]any`. When passing this to the Dreamer, the Dreamer's `projectEffects` must convert this dynamic map into concrete Mangle facts. If the tool arguments contain complex nested objects (e.g. JSON arrays or objects), the Mangle stringification logic might break.
3. **The Simulation Sandbox**: The simulation uses `kernel.Clone()`. As noted before, the depth of this clone is the single most critical security boundary in the codeNERD architecture. A shallow clone means the Dreamer's hypothetical `projected_action` facts become real facts in the agent's memory. This would poison the `ContextManager`, making the agent hallucinate that it has already edited files it has not touched.

**Phase 2: Execution (The Gap)**
After the Dreamer approves the action, the `InteractiveGate` returns control to the `TaskExecutor`, which actually runs the tool.
1. **The Time-Of-Check to Time-Of-Use (TOCTOU) Gap**: There is a race condition here. The Dreamer approves a payload based on the state of the filesystem at time T1. The tool executes at time T2. If an external process modifies the file between T1 and T2, the safety guarantee of the Dreamer is voided.
2. **Resource Exhaustion during Execution**: If the tool writes a 5GB file, the Dreamer's simulation (which only looked at the target path and action type) would have approved it, but the host system crashes.

**Phase 3: Validation (The Post-Mortem)**
After execution, `ValidateInteractiveToolResult` is called.
1. **Fact Assertions**: This function is responsible for closing the loop. It asserts `task_complete` or `tool_error` facts.
2. **Confidence Thresholds**: The validators return a confidence score. The VirtualStore only fails the action if the error confidence is >= 0.8. What happens if a validator is consistently returning 0.7 confidence of failure? The system fails open.
3. **Schema Evolution**: If the kernel schema is modified and the `task_complete` fact arity changes, this validation layer will panic or fail silently when trying to assert the fact, blinding the learning systems.

### Expanded Cascading Failure Modes

* **Failure Mode: The Infinite TDD Loop**
  If the `InteractiveGate` successfully blocks an action but fails to inject the `security_violation` fact, the `TDDLoop` is unaware of the blockage.
  - The `TDDLoop` executes a test. It fails.
  - The `TDDLoop` proposes a patch via `edit_file`.
  - The `InteractiveGate` (via Dreamer) blocks the edit because it violates a `panic_state` rule.
  - The `InteractiveGate` returns an error, but the fact injection fails.
  - The `TDDLoop` sees an execution error, but without the `security_violation` context, it thinks the tool simply failed (e.g., file locked).
  - The `TDDLoop` retries the exact same patch indefinitely, burning tokens until budget exhaustion.

* **Failure Mode: The False Positive Campaign Progression**
  If a multi-file `apply_edits` transaction fails halfway through (e.g. file 3 of 10 is blocked by the Dreamer).
  - The `VirtualStore` correctly blocks the transaction.
  - However, if the partial state changes were not rolled back by the `Codedom` layer, the filesystem is left in an inconsistent state.
  - The `CampaignOrchestrator` receives the error, but the agent observes the partial filesystem changes and assumes success, moving to the next campaign phase with broken code.

### P4 - Orchestrator Level Implications
* **Scenario 16: Campaign Orchestrator Timeout Propagation.**
    *   *Contract:* Timing guarantees across orchestrator layers.
    *   *Mechanism:* A campaign phase has a tight token budget and time limit. The Dreamer simulation takes 95% of the time limit.
    *   *Expected Behavior:* The `InteractiveGate` must proactively abort the simulation if it detects it will leave the orchestrator with insufficient time to articulate the response, rather than letting the orchestrator time out ungracefully.

* **Scenario 17: Multi-Agent Shared Kernel Contention.**
    *   *Contract:* Lock granularity on shared kernels.
    *   *Mechanism:* Two subagents (e.g., a Researcher and a Coder) share a kernel. Both call `PreflightDestructiveToolCall` simultaneously.
    *   *Expected Behavior:* The `VirtualStore` must manage locking such that one simulation does not block the other subagent from reading the core kernel state. If `kernel.Clone()` requires a global write lock, the architecture is bottlenecked.

* **Scenario 18: Feedback Fact Cardinality Explosion.**
    *   *Contract:* Memory bounds on fact ingestion.
    *   *Mechanism:* A rogue agent attempts 10,000 invalid actions, all blocked by the Dreamer.
    *   *Expected Behavior:* The `VirtualStore` injects 10,000 `security_violation` facts. The Kernel must either compress these, age them out, or throttle the agent, preventing a memory leak in the fact store.

* **Scenario 19: Payload Stringification Panic.**
    *   *Contract:* Resilience to untyped data.
    *   *Mechanism:* The `ActionRequest.Payload` contains a cyclic graph of Go maps (which `json.Marshal` handles but custom Mangle converters might not).
    *   *Expected Behavior:* The Dreamer's projection logic detects the cycle, logs an error, and blocks the action, rather than crashing the process with a stack overflow.

* **Scenario 20: Missing Validator Graceful Degradation.**
    *   *Contract:* Optionality of post-action validators.
    *   *Mechanism:* The `VirtualStore` is instantiated without a validator registry (e.g. during a fast-path TDD loop). `ValidateInteractiveToolResult` is called.
    *   *Expected Behavior:* The function immediately returns `nil` (success), recognizing that validation is an enhancement, not a strict requirement, ensuring the fast-path is not interrupted.

### System Integration Summary
The `InteractiveGate` is the fulcrum upon which codeNERD's safety relies. The Dreamer is a powerful mechanism, but its effectiveness is entirely dependent on the tight coupling and strict adherence to the contracts defined in this document. Any failure in context propagation, payload serialization, or sandbox isolation directly compromises the entire agent framework.


### P16 - Mangle Rule Evaluation Deep Dive
* **Scenario 41: `panic_state` Rule Interaction with `projected_action`.**
    *   *Contract:* Rule evaluation correctness.
    *   *Mechanism:* The Dreamer relies on rules structured like:
        `panic_state(ActionID, "critical file deletion") :- projected_action(ActionID, /delete_file, Path), critical_path_prefix(Prefix), fn:string_contains(Path, Prefix).`
        If a new `ActionType` is added (e.g., `ActionErase`), but the Mangle rules are not updated to check `projected_action(ActionID, /erase_file, Path)`, the gate will fail open.
    *   *Expected Behavior:* The system must have automated test coverage ensuring that every `ActionType` mapped in `interactiveToolActionType` has corresponding coverage in the `panic_state` constitution rules.

* **Scenario 42: `wants_direct_answer` Override Bypass.**
    *   *Contract:* Routing priority vs Safety priority.
    *   *Mechanism:* A user input triggers the `intent_signal(/is_question)` and `wants_direct_answer` logic. However, the user also includes a tool call block in their prompt to bypass perception.
    *   *Expected Behavior:* The `InteractiveGate` (which operates at the `VirtualStore` level) must remain completely ignorant of perception-level routing decisions. Safety is absolute. Even if the orchestrator is in a "direct answer" lane, if a tool execution is somehow triggered, the Dreamer preflight MUST run and MUST evaluate safety rules identically.

### Detailed Review of the ActionRequest Structure
The `ActionRequest` struct is the sole mechanism of communication between the Executor and the Dreamer.
```go
type ActionRequest struct {
	ActionID string
	Type     ActionType
	Target   string
	Payload  map[string]any
}
```
*   **ActionID:** Used to tie projected facts to a specific hypothetical event. Essential for correlating errors.
*   **Type:** The `ActionType` enum. Must map cleanly to Mangle atoms (e.g. `/write_file`).
*   **Target:** The extracted primary target. As analyzed in Scenario 26, this MUST be canonicalized before evaluation.
*   **Payload:** The raw arguments. This is incredibly dangerous if not handled correctly during projection (Scenario 19, 21).

### The Role of `critical_path_prefix`
The `assertCriticalPathFacts` method ensures the Dreamer refuses to run blind.
Why is this necessary?
Mangle is monotonic. If a rule relies on a fact, and that fact isn't present, the rule simply doesn't fire.
If `critical_path_prefix("/cmd")` is not in the kernel, the rule:
`panic_state(ID, Reason) :- projected_action(ID, /delete_file, Path), critical_path_prefix(Prefix), ...`
will evaluate to zero results, even if `Path` is `"/cmd/nerd/main.go"`.
By explicitly failing closed if these foundational facts are missing, the Dreamer guarantees a minimum level of safety posture.

### Summary of Architectural Findings
1.  **Clone is the Chokepoint:** `kernel.Clone()` is both the primary security mechanism (isolation) and the primary performance bottleneck (memory/GC overhead).
2.  **Types are Fragile:** The translation from Go `string` to Mangle `ast.Name` vs `ast.String` is prone to silent failures (empty join results).
3.  **Feedback is Imperative:** The loop is only closed when `security_violation` facts successfully land in the main kernel.

### P17 - Tool Output Parsing and VirtualStore Validation
* **Scenario 43: Validation Fact Truncation.**
    *   *Contract:* The `VirtualStore` passes tool execution results to the post-action validators.
    *   *Mechanism:* A tool executes successfully but generates 10MB of stdout (e.g., a massive compiler error log). The `ValidateInteractiveToolResult` function constructs an `ActionResult` with this 10MB output and passes it to the validators.
    *   *Expected Behavior:* If the validators attempt to assert this entire 10MB string as a fact into the Mangle kernel (e.g., `tool_output(ID, "10MB string")`), it will cause a massive memory spike and likely break the `TokenBudgetManager` during the next prompt compilation phase. The `VirtualStore` MUST truncate or summarize tool output before asserting it as a fact.

* **Scenario 44: Silent Validator Panics.**
    *   *Contract:* Post-action validation should not crash the main execution loop.
    *   *Mechanism:* A regex-based post-action validator encounters a maliciously crafted tool output designed to cause catastrophic backtracking (ReDoS). The validator hangs or panics.
    *   *Expected Behavior:* `ValidateInteractiveToolResult` must recover from panics within individual validators. A failing validator should result in a high-confidence failure return, or at least be logged and ignored, but it must never bring down the `VirtualStore` or the session `Executor`.

### P18 - The Dreamer Plan Manager State
* **Scenario 45: Orphaned Dream Plans.**
    *   *Contract:* Dream plans must be cleaned up after execution or cancellation.
    *   *Mechanism:* A complex multi-step campaign phase creates a comprehensive dream plan using the `planManager`. The phase is abruptly canceled by the user halfway through execution.
    *   *Expected Behavior:* The `planManager` must detect the context cancellation and aggressively prune the orphaned plan state from memory. If it fails to do so, long-running sessions will slowly leak memory as abandoned plans accumulate.

* **Scenario 46: Plan Manager State Bleed.**
    *   *Contract:* Sequential dream plans must not share state.
    *   *Mechanism:* Turn 1 creates a dream plan for refactoring. Turn 2 creates a new dream plan for testing.
    *   *Expected Behavior:* The `planManager` must guarantee strict isolation. If facts or projected states from Turn 1's plan leak into Turn 2's plan, the Dreamer might authorize Turn 2 based on the hypothetical success of Turn 1, leading to dangerous executions.

### Deep Architectural Review of Mangle Integration
The integration of a logic programming language (Mangle) into an imperative execution loop (Go) creates profound paradigm friction at the boundaries.

1. **The Monotonicity Problem:** Mangle is strictly monotonic. Facts can only be added, never removed, within a single evaluation context. The `VirtualStore` and `Session Executor` represent a highly stateful, mutating world. The bridge between them requires careful translation.
2. **The Fixpoint Illusion:** Go code expects a function call to return a definitive result in linear time. Mangle evaluates until it reaches a logical fixpoint. If the rules are not carefully stratified, or if a bug introduces a cyclical dependency, the Mangle engine will loop infinitely. The `Dreamer` is particularly vulnerable to this because it operates on *projected* (hypothetical) facts that might trigger untested rule paths.
3. **The "Everything is a String" Fallacy:** As noted in the `codenerd-builder` SKILL document, a common AI failure mode is assuming Mangle handles fuzzy string matching. The `InteractiveGate` must explicitly translate between Go strings (file paths, tool names) and Mangle Atoms (`/read_file`). A failure in this translation layer does not produce an error; it produces *zero results*, which the system often interprets as "safe" or "permitted". This fail-open characteristic of logic engines requires aggressive fail-closed wrappers in the Go layer.

### Conclusion and Path Forward
The `InteractiveGate` is robust in its conception but fragile in its execution due to the complexities of integrating an imperative Virtual Store with a declarative, neuro-symbolic Dreamer kernel.

To truly secure this boundary, the development team must:
1. **Prioritize the Differential Fact Store:** Implement scoped overlays in `kernel.Clone()` to eliminate the memory and performance overhead of deep cloning.
2. **Implement Strict Type Enforcement:** Create a dedicated translation layer that rigidly maps `ActionType` and `Target` to Mangle Atoms and Strings, respectively, logging critical errors upon any ambiguity.
3. **Audit Context Propagation:** Ensure that `ctx.Done()` is respected at the lowest levels of the Mangle evaluation engine, preventing malicious payloads from causing Denial of Service through infinite logic loops.
4. **Harden Target Canonicalization:** Force all file paths through `filepath.EvalSymlinks` and case-normalization before they ever reach the Dreamer's projection logic.

By addressing these core architectural friction points, codeNERD can achieve the high-assurance execution guarantees required for autonomous, destructive system administration tasks.

### P17 - Tool Output Parsing and VirtualStore Validation
* **Scenario 43: Validation Fact Truncation.**
    *   *Contract:* The `VirtualStore` passes tool execution results to the post-action validators.
    *   *Mechanism:* A tool executes successfully but generates 10MB of stdout (e.g., a massive compiler error log). The `ValidateInteractiveToolResult` function constructs an `ActionResult` with this 10MB output and passes it to the validators.
    *   *Expected Behavior:* If the validators attempt to assert this entire 10MB string as a fact into the Mangle kernel (e.g., `tool_output(ID, "10MB string")`), it will cause a massive memory spike and likely break the `TokenBudgetManager` during the next prompt compilation phase. The `VirtualStore` MUST truncate or summarize tool output before asserting it as a fact.

* **Scenario 44: Silent Validator Panics.**
    *   *Contract:* Post-action validation should not crash the main execution loop.
    *   *Mechanism:* A regex-based post-action validator encounters a maliciously crafted tool output designed to cause catastrophic backtracking (ReDoS). The validator hangs or panics.
    *   *Expected Behavior:* `ValidateInteractiveToolResult` must recover from panics within individual validators. A failing validator should result in a high-confidence failure return, or at least be logged and ignored, but it must never bring down the `VirtualStore` or the session `Executor`.

### P18 - The Dreamer Plan Manager State
* **Scenario 45: Orphaned Dream Plans.**
    *   *Contract:* Dream plans must be cleaned up after execution or cancellation.
    *   *Mechanism:* A complex multi-step campaign phase creates a comprehensive dream plan using the `planManager`. The phase is abruptly canceled by the user halfway through execution.
    *   *Expected Behavior:* The `planManager` must detect the context cancellation and aggressively prune the orphaned plan state from memory. If it fails to do so, long-running sessions will slowly leak memory as abandoned plans accumulate.

* **Scenario 46: Plan Manager State Bleed.**
    *   *Contract:* Sequential dream plans must not share state.
    *   *Mechanism:* Turn 1 creates a dream plan for refactoring. Turn 2 creates a new dream plan for testing.
    *   *Expected Behavior:* The `planManager` must guarantee strict isolation. If facts or projected states from Turn 1's plan leak into Turn 2's plan, the Dreamer might authorize Turn 2 based on the hypothetical success of Turn 1, leading to dangerous executions.

### Deep Architectural Review of Mangle Integration
The integration of a logic programming language (Mangle) into an imperative execution loop (Go) creates profound paradigm friction at the boundaries.

1. **The Monotonicity Problem:** Mangle is strictly monotonic. Facts can only be added, never removed, within a single evaluation context. The `VirtualStore` and `Session Executor` represent a highly stateful, mutating world. The bridge between them requires careful translation.
2. **The Fixpoint Illusion:** Go code expects a function call to return a definitive result in linear time. Mangle evaluates until it reaches a logical fixpoint. If the rules are not carefully stratified, or if a bug introduces a cyclical dependency, the Mangle engine will loop infinitely. The `Dreamer` is particularly vulnerable to this because it operates on *projected* (hypothetical) facts that might trigger untested rule paths.
3. **The "Everything is a String" Fallacy:** As noted in the `codenerd-builder` SKILL document, a common AI failure mode is assuming Mangle handles fuzzy string matching. The `InteractiveGate` must explicitly translate between Go strings (file paths, tool names) and Mangle Atoms (`/read_file`). A failure in this translation layer does not produce an error; it produces *zero results*, which the system often interprets as "safe" or "permitted". This fail-open characteristic of logic engines requires aggressive fail-closed wrappers in the Go layer.

### Conclusion and Path Forward
The `InteractiveGate` is robust in its conception but fragile in its execution due to the complexities of integrating an imperative Virtual Store with a declarative, neuro-symbolic Dreamer kernel.

To truly secure this boundary, the development team must:
1. **Prioritize the Differential Fact Store:** Implement scoped overlays in `kernel.Clone()` to eliminate the memory and performance overhead of deep cloning.
2. **Implement Strict Type Enforcement:** Create a dedicated translation layer that rigidly maps `ActionType` and `Target` to Mangle Atoms and Strings, respectively, logging critical errors upon any ambiguity.
3. **Audit Context Propagation:** Ensure that `ctx.Done()` is respected at the lowest levels of the Mangle evaluation engine, preventing malicious payloads from causing Denial of Service through infinite logic loops.
4. **Harden Target Canonicalization:** Force all file paths through `filepath.EvalSymlinks` and case-normalization before they ever reach the Dreamer's projection logic.

By addressing these core architectural friction points, codeNERD can achieve the high-assurance execution guarantees required for autonomous, destructive system administration tasks.

### Extended Contract Analysis: The Autopoiesis Loop
The `InteractiveGate` does not exist in a vacuum. It is the primary feedback mechanism for the `Autopoiesis` (self-learning) system.

* **Contract F: Learning from Rejection.** When the Dreamer blocks an action, it isn't just protecting the system; it is teaching the agent. The `VirtualStore` injects `security_violation(ActionID, Reason)`.
* **The Vulnerability:** If the `Reason` string is dynamically generated and contains unbounded variable data (like a 10MB file payload or a deeply nested JSON object), the Autopoiesis logic will attempt to analyze it.
* **Scenario 47: Autopoiesis OOM via Rejection Reason.**
    *   *Mechanism:* The Dreamer rejects an `edit_file` action because the target is oversized. The rejection reason includes the full target string.
    *   *Expected Behavior:* The `VirtualStore` must truncate the `Reason` string before asserting it as a fact. If a 10MB target string is embedded in the `security_violation` fact, the subsequent `Evaluate()` call in the main kernel will consume massive amounts of memory, potentially crashing the entire session. The feedback loop must be bounded.

### Extended Contract Analysis: Campaign Orchestration
Multi-phase campaigns rely on the `InteractiveGate` to maintain state consistency across phases.

* **Contract G: Transactional Integrity of Multi-File Edits.** As seen in Scenario 10 and 21, `apply_edits` is a multi-file transaction.
* **The Vulnerability:** The `VirtualStore` decompose `apply_edits` into multiple single-file `PreflightDestructiveToolCall` operations.
* **Scenario 48: Partial Preflight Success.**
    *   *Mechanism:* A tool requests edits to `file_A.go` and `file_B.go`. `file_A.go` passes the Dreamer preflight. `file_B.go` is blocked by a `panic_state` rule.
    *   *Expected Behavior:* The `VirtualStore` must implement an atomic rollback for the preflight phase itself. It cannot return a partial success. Furthermore, the `projected_action` facts generated during the successful preflight of `file_A.go` MUST be discarded. If they persist in the shared Dreamer cache or a leaked kernel clone, subsequent turns might falsely assume `file_A.go` was actually edited.

### The Role of `ActionType` Mapping
The `interactiveToolActionType` map in `virtual_store_interactive_gate.go` is the Rosetta Stone of this boundary.

```go
var interactiveToolActionType = map[string]ActionType{
	"read_file":   ActionReadFile,
	"write_file":  ActionWriteFile,
	"edit_file":   ActionEditFile,
	"delete_file": ActionDeleteFile,
    // ...
}
```

* **The Brittleness Problem:** This map is hardcoded in Go. However, the available tools are often dynamically loaded via MCP or generated via Autopoiesis.
* **Scenario 49: Dynamic Tool Type Inference.**
    *   *Mechanism:* The `Ouroboros` loop generates a new tool called `refactor_struct`. This tool is not in the hardcoded map.
    *   *Expected Behavior:* The `actionTypeForToolName` function falls back to `tools.LookupEffect(toolName)`. If `LookupEffect` relies on parsing the tool's description or LLM-generated metadata, an adversarial tool could disguise a destructive write operation as a harmless `EffectRead`. The `VirtualStore` would map it to `ActionReadFile`, skipping the Dreamer's destructive preflight entirely. The system must enforce strict, statically verifiable capability manifests for all dynamically loaded tools.

### Conclusion: The Illusion of Safety
The integration analysis reveals a critical theme: **The illusion of safety is worse than no safety at all.**
If the `InteractiveGate` fails open silently (due to schema mismatches, type confusions, or context timeouts), the orchestrator operates under the false assumption that all actions have been rigorously vetted. This leads to bolder, more destructive behavior from the SubAgents.
True integration resilience requires aggressive fail-closed defaults, strict type enforcement at the Go/Mangle boundary, and mathematically verifiable isolation of the Dreamer's speculative sandboxes.

### P19 - The Interaction of Perception and VirtualStore
The codeNERD architecture relies on a `Perception Transducer` to convert natural language into `user_intent` facts.

* **Scenario 50: Transducer to Gate Latency.**
    *   *Contract:* Real-time execution boundaries.
    *   *Mechanism:* A user requests a massive operation. The Transducer takes 30 seconds to parse the intent. By the time the `Session Executor` derives a `next_action` and calls the `VirtualStore`, the parent context is already near timeout.
    *   *Expected Behavior:* The `VirtualStore`'s `PreflightDestructiveToolCall` must check the remaining time on the context *before* invoking `kernel.Clone()`. If there are only 50ms left, cloning a massive kernel will definitely timeout, leading to an uncontrolled panic or an ungraceful fail-closed state. The system should proactively abort and yield a "timeout impending" error.

* **Scenario 51: Hallucinated Tool Targets.**
    *   *Contract:* Grounding of tool arguments.
    *   *Mechanism:* The LLM hallucinates a file path that does not exist, but matches the pattern of a critical system file (e.g., `/etc/shadow_backup`).
    *   *Expected Behavior:* The Dreamer evaluates the action. If the safety rules only protect exact paths (e.g., `/etc/shadow`), the Dreamer will allow the action. The `VirtualStore` execution layer will then fail because the file doesn't exist. This is a safe failure, but it highlights that the Dreamer is a *policy* engine, not a *validation* engine. It cannot verify the existence of files; it can only verify permissions.

### P20 - Cross-Boundary Data Integrity
The entire OODA loop relies on data integrity as it passes through multiple subsystems.

1. **User Input** (String) -> `Perception` ->
2. **Intent Fact** (Mangle Atom) -> `Kernel` ->
3. **Next Action Fact** (Mangle Atom) -> `VirtualStore` ->
4. **Tool Call** (Go Struct) -> `Dreamer` ->
5. **Action Request** (Go Struct) -> `Dreamer Projection` ->
6. **Projected Fact** (Mangle Atom) -> `Cloned Kernel` ->
7. **Verdict** (Bool) -> `VirtualStore` ->
8. **Feedback Fact** (Mangle Atom) -> `Main Kernel`

* **Scenario 52: Data Truncation in Translation.**
    *   *Mechanism:* At step 3, a highly complex nested JSON payload is extracted from the LLM response. The `VirtualStore` parses it into a `map[string]any`. During step 5, the Dreamer attempts to serialize this map back into a string for the `projected_action` fact.
    *   *Expected Behavior:* If the serialization uses a default Go `%v` formatter, maps will be printed as `map[key:value]`, which is invalid JSON. When the `Autopoiesis` loop later tries to read the `security_violation` feedback (Step 8), it will fail to parse the payload, breaking the learning loop. The translation layer must guarantee JSON-compatible serialization at all boundaries.

### Epilogue: The Destructive Engineer's Mindset
To truly test a neuro-symbolic system, one must abandon the concept of "unit testing". A unit test verifies that `PreflightDestructiveToolCall` returns an error when given a nil context. That is trivial.

An integration test—a *siege* test—verifies that when `PreflightDestructiveToolCall` receives a nil context, it fails closed, which causes the `VirtualStore` to inject a `security_violation`, which is correctly parsed by the `TDDLoop`, which then successfully formulates a new `next_action` that does not use a nil context, and that this entire sequence completes within the 10-second token budget without leaking goroutines or polluting the main kernel's fact store.

The cracks are not in the functions; they are in the assumptions the functions make about each other.

### P19 - The Interaction of Perception and VirtualStore
The codeNERD architecture relies on a `Perception Transducer` to convert natural language into `user_intent` facts.

* **Scenario 50: Transducer to Gate Latency.**
    *   *Contract:* Real-time execution boundaries.
    *   *Mechanism:* A user requests a massive operation. The Transducer takes 30 seconds to parse the intent. By the time the `Session Executor` derives a `next_action` and calls the `VirtualStore`, the parent context is already near timeout.
    *   *Expected Behavior:* The `VirtualStore`'s `PreflightDestructiveToolCall` must check the remaining time on the context *before* invoking `kernel.Clone()`. If there are only 50ms left, cloning a massive kernel will definitely timeout, leading to an uncontrolled panic or an ungraceful fail-closed state. The system should proactively abort and yield a "timeout impending" error.

* **Scenario 51: Hallucinated Tool Targets.**
    *   *Contract:* Grounding of tool arguments.
    *   *Mechanism:* The LLM hallucinates a file path that does not exist, but matches the pattern of a critical system file (e.g., `/etc/shadow_backup`).
    *   *Expected Behavior:* The Dreamer evaluates the action. If the safety rules only protect exact paths (e.g., `/etc/shadow`), the Dreamer will allow the action. The `VirtualStore` execution layer will then fail because the file doesn't exist. This is a safe failure, but it highlights that the Dreamer is a *policy* engine, not a *validation* engine. It cannot verify the existence of files; it can only verify permissions.

### P20 - Cross-Boundary Data Integrity
The entire OODA loop relies on data integrity as it passes through multiple subsystems.

1. **User Input** (String) -> `Perception` ->
2. **Intent Fact** (Mangle Atom) -> `Kernel` ->
3. **Next Action Fact** (Mangle Atom) -> `VirtualStore` ->
4. **Tool Call** (Go Struct) -> `Dreamer` ->
5. **Action Request** (Go Struct) -> `Dreamer Projection` ->
6. **Projected Fact** (Mangle Atom) -> `Cloned Kernel` ->
7. **Verdict** (Bool) -> `VirtualStore` ->
8. **Feedback Fact** (Mangle Atom) -> `Main Kernel`

* **Scenario 52: Data Truncation in Translation.**
    *   *Mechanism:* At step 3, a highly complex nested JSON payload is extracted from the LLM response. The `VirtualStore` parses it into a `map[string]any`. During step 5, the Dreamer attempts to serialize this map back into a string for the `projected_action` fact.
    *   *Expected Behavior:* If the serialization uses a default Go `%v` formatter, maps will be printed as `map[key:value]`, which is invalid JSON. When the `Autopoiesis` loop later tries to read the `security_violation` feedback (Step 8), it will fail to parse the payload, breaking the learning loop. The translation layer must guarantee JSON-compatible serialization at all boundaries.

### Epilogue: The Destructive Engineer's Mindset
To truly test a neuro-symbolic system, one must abandon the concept of "unit testing". A unit test verifies that `PreflightDestructiveToolCall` returns an error when given a nil context. That is trivial.

An integration test—a *siege* test—verifies that when `PreflightDestructiveToolCall` receives a nil context, it fails closed, which causes the `VirtualStore` to inject a `security_violation`, which is correctly parsed by the `TDDLoop`, which then successfully formulates a new `next_action` that does not use a nil context, and that this entire sequence completes within the 10-second token budget without leaking goroutines or polluting the main kernel's fact store.

The cracks are not in the functions; they are in the assumptions the functions make about each other.

### Appendix A: Required Architectural Refactors
Based on this analysis, the following refactors are mandatory for production stability:
1.  **Introduce `factstore.Overlay`:** Rewrite the Mangle engine's state management to support copy-on-write overlays. `kernel.Clone()` must be deprecated in favor of `kernel.PushScope()`.
2.  **Centralized Type Transducer:** Create a single package `internal/mangle/transducer` responsible for converting between Go types (`string`, `int`, `map`) and Mangle AST types (`ast.String`, `ast.Number`, `ast.Atom`). The `VirtualStore` and `Dreamer` must both use this centralized logic to eliminate "Atom/String Dissonance".
3.  **Strict Capability Manifests:** Delete the fallback logic in `actionTypeForToolName`. Tools without a statically defined, signed capability manifest must be globally denied.
4.  **Asynchronous Telemetry Sink:** Decouple the logging of simulated sandbox events from the main critical path. Simulated logs should be buffered and flushed asynchronously to prevent I/O blocking during the tight constraints of the Dreamer preflight loop.

### Appendix B: Operational Runbook for Gate Failures
When the `InteractiveGate` fails in production, operators must follow these steps:
1.  **Identify the Failure Mode:** Determine if the gate failed open (destructive action occurred) or failed closed (agent paralyzed).
2.  **Inspect the Fact Store:** If the agent is paralyzed, query the kernel for `security_violation` facts.
    `k.Query("security_violation(ID, Reason)")`
    If facts exist, the Dreamer is working, but the rules are too strict. If facts are missing, the VirtualStore feedback loop is broken.
3.  **Audit the Dreamer Cache:** If actions are mysteriously slipping through, inspect the Dreamer's cache keys. Look for hash collisions indicating that the payload was ignored during cache lookup.
4.  **Verify Schema Integrity:** Ensure the loaded Mangle schema matches the Go struct definitions for `ActionRequest` and `ActionResult`. Arity mismatches (e.g., passing 3 arguments to a 2-arity predicate) are the leading cause of silent fact assertion failures.
5.  **Restart the Session Executor:** If the `kernel.Clone()` operation has leaked memory, restarting the Session Executor is the only way to reclaim resources without bringing down the entire codeNERD daemon.

### Appendix C: Review of `apply_edits` Transactionality
The `apply_edits` tool is unique because it modifies multiple files simultaneously.
The `VirtualStore` handles this by iterating over the `paths` and calling `PreflightDestructiveToolCall` for each path as if it were an `edit_file` operation.
1.  **Atomicity Failure:** If the loop successfully preflights paths 1 through 4, but path 5 fails, the `PreflightDestructiveToolCall` for `apply_edits` returns an error. The operation is blocked. This is atomic.
2.  **Telemetry Amplification:** However, paths 1 through 4 still underwent full Dreamer simulations, generating logs, evaluating rules, and potentially allocating memory.
3.  **The Fix:** A true transactional preflight requires batch simulation. The `VirtualStore` should construct a single `ActionRequest` representing the entire transaction, allowing the Dreamer to evaluate all projected changes simultaneously against the `panic_state` rules. This reduces overhead and prevents partial evaluation leaks.
