---
surface: "VirtualStore_Dreamer_InteractiveGate"
mode: "boundary"
subsystems_tested: ["VirtualStore", "Dreamer", "Kernel", "SessionExecutor", "ValidatorRegistry"]
blast_radius: "critical"
remediated: false
---

# Siege Journal: VirtualStore <-> Dreamer <-> Kernel Boundary Analysis

## 1. System Interaction Map
### A. The Executor-to-Gate Handshake
The boundary begins when `SessionExecutor` (in `internal/session/executor.go`) initiates a tool call. Because interactive tools execute via `tools.Global()` instead of `RouteAction`, the executor must manually wrap the call with two explicit gates on `VirtualStore`: `PreflightDestructiveToolCall` and `ValidateInteractiveToolResult`.
1. **Call:** `executor.go` -> `VirtualStore.PreflightDestructiveToolCall(ctx, actionID, toolName, args)`
2. **Mapping:** `VirtualStore` cross-references `interactiveToolActionType` to map `toolName` (e.g., `write_file`) to `ActionType` (`ActionWriteFile`). If the tool is unmapped, the gate silently opens (fail-open for unmapped tools).
3. **Request Construction:** `buildInteractiveActionRequest` is called. It uses `extractActionTarget` to guess the target from `args` (e.g., `path`, `filename`, `url`). It copies all `args` into the `Payload` field of the `ActionRequest`.

### B. The Dreamer Cache Path
4. **Retrieval:** `VirtualStore.getDreamer()` fetches the `Dreamer` singleton. If `nil`, the system injects a `security_violation` fact and blocks the action (fail-closed).
5. **Simulation:** `Dreamer.SimulateAction` receives the request. It extracts a `cacheKey` using `string(req.Type) + ":" + req.Target`.
6. **Cache Hit:** If the key exists in `DreamCache`, `SimulateAction` returns immediately with the cached `DreamResult` (updating `ActionID` and `Request`). The kernel is completely bypassed.
7. **Cache Miss:** The `Dreamer` clones the `RealKernel` (a heavy ~277KB memory copy).
8. **Projection:** `projectEffects(kernel, actionID, req)` translates the `ActionRequest` into projected facts (e.g., `projected_file_write(Target)`).
9. **Evaluation:** `evaluateProjection` asserts these facts into the sandboxed kernel and queries for `panic_state/2`. The result dictates the final `Unsafe` boolean.
10. **Storage:** The verdict is written back to `DreamCache` and returned to `VirtualStore`.

### C. The Post-Execution Validator Pipeline
11. **Execution:** Assuming approval, the executor runs the tool logic natively.
12. **Validation:** `VirtualStore.ValidateInteractiveToolResult` is called with the output.
13. **Dispatch:** The request is rebuilt and sent to `ValidatorRegistry.Validate(ctx, req, res)`.
14. **Fact Injection:** Results are asserted to the kernel via `processValidationResults` to inform subsequent logic (like `task_complete/1`).
15. **Threshold:** If validation fails with a confidence >= 0.8, the executor considers the action structurally failed.

## 2. Contract Analysis

### A. The Cache Completeness Contract (Violated)
- **Implicit Contract:** The `DreamCache` assumes that an `ActionType` and a `Target` (e.g., `write_file` on `config.json`) fully define the security footprint of an action.
- **Breach:** The footprint heavily depends on the `Payload` (the content being written). By omitting the payload hash from the cache key, the Dreamer treats writing `{}` and writing `rm -rf /` as the exact same action. If the benign write happens first, the malicious write gets a free pass.

### B. The Payload Immutability Contract (Violated)
- **Implicit Contract:** The `args` map passed from the `SessionExecutor` to the `VirtualStore` to the `Dreamer` remains immutable during the simulation lifecycle.
- **Breach:** `buildInteractiveActionRequest` does a shallow copy of `args` into `Payload`. However, since `args` is an `any` map, nested structures remain references. If the executor or another goroutine mutates the map while `SimulateAction` is blocking on a kernel clone, a Time-Of-Check to Time-Of-Use (TOCTOU) vulnerability occurs when `projectEffects` finally reads it.

### C. The Target Extraction Contract
- **Implicit Contract:** The heuristic in `extractActionTarget` (checking `path`, `filename`, `url`, etc.) aligns perfectly with the underlying tool's target resolution logic.
- **Breach:** If a tool accepts an unconventional key (like `dest_path`) that `extractActionTarget` misses, the target defaults to `'unknown'`. The Dreamer simulates an action against `'unknown'`, potentially allowing an action that should be blocked on the actual destination path.

### D. The Thread-Safety Contract
- **Implicit Contract:** The `DreamCache` map operations are fully synchronized and will not deadlock with the `Dreamer`'s own internal states.
- **Breach:** While `c.mu` protects the map, the eviction logic (triggering when size reaches 256) is an O(N) iteration over the map to delete half the entries. Under high concurrent load of unique targets, this eviction stall causes massive lock contention, slowing down all parallel SubAgents attempting to use interactive tools.

### E. The Validation Confidence Contract
- **Implicit Contract:** Validators provide a normalized confidence score (0.0 to 1.0) and the system strictly respects the >= 0.8 threshold for rejection.
- **Breach:** If a validator returns a failure string but a confidence of 0.79, `ValidateInteractiveToolResult` will silently swallow the error and return `nil`, causing the LLM to believe the action succeeded completely.

### F. The Fail-Closed Contract on Nil Parameters
- **Implicit Contract:** If the system is handed a `nil` context or a `nil` payload map, it will fail closed and not panic.
- **Breach:** `buildInteractiveActionRequest` handles `nil` arguments cleanly by instantiating an empty map, but nested logic in tool implementations or validators might panic. The boundary itself successfully defends against `nil` crashes.

## 3. Failure Mode Enumeration

### 1. Semantic Bypass via Cache Collision
- **Vector:** Agent executes `write_file(target: 'config.js', content: 'var a = 1')`. Cached as Safe. Agent subsequently executes `write_file(target: 'config.js', content: 'malicious payload')`.
- **Impact:** Malicious payload circumvents kernel `panic_state` rules because the Dreamer hits the cache and returns immediately.

### 2. Context Leak in Streaming Scenarios (Temporal)
- **Vector:** A user aborts a command while `SimulateAction` is busy cloning the kernel. `ctx.Done()` fires, but `clone.assertWithoutEvalChecked` doesn't listen to context.
- **Impact:** The goroutine continues consuming CPU and memory until the simulation finishes, even though the user session is already gone.

### 3. Missing Fact Avalanche (Semantic)
- **Vector:** The target string contains characters that are invalid in Mangle Atoms (e.g., unmatched quotes).
- **Impact:** `VirtualStore` tries to inject `security_violation` fact, but `k.Assert` fails. The rejection reason is lost, preventing the `learningCollector` from picking up the pattern.

### 4. Interactive Tool Blindspot (Ordering)
- **Vector:** An adversarial LLM decides to use an unmapped tool name like `sys_write` instead of `write_file`.
- **Impact:** `actionTypeForToolName` returns false. `PreflightDestructiveToolCall` returns `nil`. The action executes unhindered because `VirtualStore` assumes only explicitly mapped tools are destructive.

### 5. TOCTOU Map Mutation (State Corruption)
- **Vector:** A concurrent shard logic loop mutates the `args` map while waiting for the Dreamer gate to return.
- **Impact:** The Dreamer approves `args['content'] = 'safe'`, but the tool executes with `args['content'] = 'unsafe'`.

### 6. Eviction Lock Stutter (Resource Exhaustion)
- **Vector:** 10 shards simultaneously spawn and write to 30 unique files each. The cache fills to 256. Every subsequent write triggers a half-cache deletion.
- **Impact:** Shard execution time spikes from 100ms to 2000ms as they all pile up on `c.mu.Lock()`.

### 7. Invalidation Race (Temporal)
- **Vector:** A background process updates a Mangle policy, calling `InvalidateCache`. At the exact same nanosecond, a tool call passes the `cache != nil` check in `SimulateAction` but hasn't called `cache.Store`.
- **Impact:** The cache is wiped, but then the tool call finishes and writes its potentially stale verdict back into the fresh cache, poisoning it for future calls.

### 8. Null Payload Panic (Semantic)
- **Vector:** `args` is strictly `nil` rather than an empty map.
- **Impact:** Handled gracefully by `buildInteractiveActionRequest`, but if `Payload` access assumes keys exist without checking, a nil map read could panic down the line in validators.

### 9. Validation Fact Duplication (Ordering)
- **Vector:** The tool executes twice due to a retry logic. The validator runs twice and asserts the same validation facts.
- **Impact:** The kernel fills with duplicate facts if they contain timestamps, degrading query performance over long sessions.

### 10. The 0.79 Confidence Silent Failure (Semantic)
- **Vector:** A validator detects an issue but assigns 0.79 confidence.
- **Impact:** The `InteractiveGate` returns success. The agent thinks it fixed the bug, but the validation failed, leading to infinite loops in autonomous repair cycles.

## 4. Adversarial Scenario Design

### SCENARIO-01: Cache Payload Collision
- **Violated Contract:** Semantic Bypass
- **Injection Mechanism:** Submit benign `write_file`, wait for cache. Submit malicious `write_file` to same target. Assert that the second call skips simulation and executes.
- **Severity:** Critical

### SCENARIO-02: TOCTOU Argument Mutation
- **Violated Contract:** State Corruption
- **Injection Mechanism:** Launch goroutine to continually mutate `args['content']` while `PreflightDestructiveToolCall` is running. Verify race conditions.
- **Severity:** Critical

### SCENARIO-03: Context Cancellation Mid-Clone
- **Violated Contract:** Temporal Delay
- **Injection Mechanism:** Pass context with 1ms timeout. Assert that the simulation returns immediately and doesn't leak.
- **Severity:** Operational

### SCENARIO-04: Cache Eviction Storm
- **Violated Contract:** Resource Exhaustion
- **Injection Mechanism:** Rapidly invoke 1000 unique tool calls concurrently. Monitor lock contention and verify no panics occur during massive map iteration.
- **Severity:** Operational

### SCENARIO-05: Missing Mapped Tool Bypass
- **Violated Contract:** Contract Violation
- **Injection Mechanism:** Invoke a destructive action using a tool name not present in `interactiveToolActionType`. Assert it bypasses safety.
- **Severity:** Enhancement

### SCENARIO-06: Invalidation Race Poisoning
- **Violated Contract:** Temporal State Corruption
- **Injection Mechanism:** Run `InvalidateCache` in parallel with `SimulateAction`. Verify that verdicts calculated before invalidation do not re-enter the cache.
- **Severity:** Operational

### SCENARIO-07: The 0.79 Confidence Trap
- **Violated Contract:** Validation Contract
- **Injection Mechanism:** Mock a validator to return a severe error with confidence 0.79. Assert that the executor proceeds as if successful.
- **Severity:** Enhancement

### SCENARIO-08: Fact Injection Rejection
- **Violated Contract:** Cascading Error
- **Injection Mechanism:** Trigger a security violation with an unparseable target string (`test(ing`). Verify the system degrades gracefully without crashing the kernel.
- **Severity:** Operational

### SCENARIO-09: Nil Arguments Map
- **Violated Contract:** Panic Check
- **Injection Mechanism:** Pass a literal `nil` for arguments to the gate. Assert graceful handling and conversion to empty map.
- **Severity:** Enhancement

### SCENARIO-10: Massive Target Path
- **Violated Contract:** Resource Exhaustion
- **Injection Mechanism:** Pass a 5MB string as the target path. Verify the cache key generation limits length or handles memory without OOM.
- **Severity:** Operational

### SCENARIO-11: Silent Target Misalignment
- **Violated Contract:** Target Contract
- **Injection Mechanism:** Pass arguments without standard target keys (e.g., `destination_file`). Assert target becomes 'unknown' and is still processed safely.
- **Severity:** Operational

### SCENARIO-12: Empty Payload Map Simulation
- **Violated Contract:** Semantic Boundary
- **Injection Mechanism:** Pass arguments with target but no content. Verify the projection handles empty maps safely.
- **Severity:** Enhancement

### SCENARIO-13: Concurrent Validation Assault
- **Violated Contract:** Resource Exhaustion
- **Injection Mechanism:** Simulate 500 successful tool executions rapidly calling `ValidateInteractiveToolResult`. Monitor kernel assertion speeds.
- **Severity:** Operational

### SCENARIO-14: The Unsafe Cache Latch
- **Violated Contract:** Semantic State
- **Injection Mechanism:** Trigger an action that fails simulation. Assert that the `Unsafe: true` verdict IS cached, preventing future clones for the same bad action.
- **Severity:** Operational

### SCENARIO-15: Validation Threshold Override
- **Violated Contract:** Validation Contract
- **Injection Mechanism:** Mock a validator to return 0.81 confidence on failure. Assert that the executor halts.
- **Severity:** Operational

## 5. Cascading Failure Analysis

### Cascade Path 1: The Cache Collision Domino Effect
1. **Initial Flaw:** `DreamCache` key ignores payload.
2. **Subsystem A (Dreamer):** Returns cached 'Safe' verdict for malicious payload because it shares a target with a previous benign action.
3. **Subsystem B (VirtualStore):** Trusts the Dreamer and allows the `InteractiveGate` to open.
4. **Subsystem C (SessionExecutor):** Executes the malicious tool payload (e.g., overwriting a critical policy file) because it assumes safety checks passed.
5. **Subsystem D (Kernel):** The altered policy file is loaded on the next cycle, fundamentally changing the agent's behavior.
6. **Final Impact:** Complete agent hijacking due to a missing string concatenation.

### Cascade Path 2: Eviction Lock Starvation
1. **Initial Flaw:** `DreamCache.Store` performs O(N) map deletion while holding `c.mu.Lock()`.
2. **Subsystem A (DreamCache):** Reaches 256 entries. Next call triggers eviction block.
3. **Subsystem B (Dreamer):** `SimulateAction` goroutines pile up waiting for `c.mu.Lock()` to store their results.
4. **Subsystem C (SessionExecutor):** Interactive tool calls take seconds instead of milliseconds.
5. **Subsystem D (Orchestrator):** The overarching campaign assumes shards are stuck and starts killing them via timeouts.
6. **Final Impact:** System grinds to a halt under moderate concurrent load, resulting in failed tasks and phantom timeouts.

### Cascade Path 3: The 0.79 Phantom Success
1. **Initial Flaw:** `ValidateInteractiveToolResult` requires >= 0.8 confidence to flag an error.
2. **Subsystem A (ValidatorRegistry):** Detects that a file edit failed to apply properly, but assigns 0.79 confidence due to partial matches.
3. **Subsystem B (VirtualStore):** Swallows the error and returns `nil` to the executor.
4. **Subsystem C (SessionExecutor):** Feeds a 'Tool execution successful' string back to the LLM.
5. **Subsystem D (TDD Loop):** The LLM runs tests, which fail because the edit didn't happen. It tries the exact same edit again.
6. **Final Impact:** The agent gets trapped in an infinite loop of attempting an action, thinking it succeeded, and failing the subsequent test.

### Cascade Path 4: Invalid Fact Injection Death Spiral
1. **Initial Flaw:** Agent outputs a malformed string containing reserved Mangle tokens.
2. **Subsystem A (Transducer):** Fails to fully sanitize the target string.
3. **Subsystem B (VirtualStore):** Dreamer blocks it, attempts to inject `security_violation(req, reason)`.
4. **Subsystem C (Kernel):** The assertion crashes or errors silently due to schema violations from the unescaped string.
5. **Subsystem D (Learning):** The `autopoiesis` loop never receives the failure event.
6. **Final Impact:** The model is not penalized for hallucinating bad strings and repeats the mistake indefinitely.

### Cascade Path 5: Unmapped Tool Exploit
1. **Initial Flaw:** An advanced model infers an unregistered shell tool from the environment.
2. **Subsystem A (VirtualStore):** Fails to map the tool name in `interactiveToolActionType`. Silently allows it through `PreflightDestructiveToolCall`.
3. **Subsystem B (SessionExecutor):** Executes a potentially destructive environment command (e.g. `rm`).
4. **Subsystem C (ValidatorRegistry):** Never runs because the tool isn't mapped, so no verification occurs.
5. **Final Impact:** Total bypass of the safety gating mechanism, leading to uncontrolled system state.

### Cascade Path 6: TOCTOU State Drift
1. **Initial Flaw:** The `SessionExecutor` utilizes a single map for arguments across the entire subagent lifetime.
2. **Subsystem A (VirtualStore):** Passes the map by reference into `buildInteractiveActionRequest`.
3. **Subsystem B (Dreamer):** Clones the kernel (takes ~270ms).
4. **Subsystem C (SessionExecutor):** A concurrent callback modifies the map content.
5. **Subsystem D (VirtualStore):** Executes the tool with the modified payload which was never simulated.
6. **Final Impact:** A benign command gets mutated into a malicious one mid-flight, destroying system integrity.

## 6. Deep Dive into VirtualStore Contract Extensibility

### Edge Case: The State Isolation Boundary
In multi-agent systems, isolation between parallel executions is paramount. Consider a scenario where Agent A and Agent B operate on the same logical target (`config.json`) but with distinct session parameters (e.g. Agent A uses role 'admin', Agent B uses role 'auditor').

The `VirtualStore` contract assumes that simply locking `Dreamer` logic behind the target path is sufficient. However, if Agent A is blocked due to policy restricting admin writes, the cache stores `Unsafe: true`. Agent B, attempting a purely auditable, non-destructive write (which policy allows), will hit the cache key `write_file:config.json` and be incorrectly blocked.

This reveals a fundamental architectural oversight: **Security contexts must be part of the cache key.** The Dreamer cache currently treats the action target as the sole dimension of security, ignoring the actor's identity and intent payload.

### The Problem with Fail-Open Unmapped Tools
The `interactiveToolActionType` map in `internal/core/virtual_store_interactive_gate.go` dictates whether an action undergoes `Dreamer` simulation. If a tool is not present in this map, it defaults to non-destructive and skips the gate entirely.
While this design allows modular tools to be injected without modifying the core VirtualStore switch, it creates a massive fail-open vulnerability. If a new capability like `execute_sql` is registered in `internal/tools` but the developer forgets to add it to `interactiveToolActionType`, that tool will execute destructively without ANY simulation checks.
The integration boundary assumes synchronization between tool registries and VirtualStore mappings, an implicit contract that is fragile and prone to human error.

### Validation Asymmetry
Post-action validation introduces an asymmetry in how the system interprets success. When a tool succeeds natively (e.g. `File written successfully`), the VirtualStore translates this into a standard output. However, validators run *after* this output is generated. If a validator fails, it injects a failure fact, but the tool executor is already holding a success string.
This race between truth (the filesystem state observed by validators) and perceived truth (the tool's standard output) means the LLM might be told "Write successful" by the articulation layer, while the kernel simultaneously registers `validation_failed(Action)`. This split-brain scenario breaks the single-source-of-truth contract that Mangle relies on.

### The Memory Footprint of the Clone Array
When cache misses spike (e.g., during a project-wide search and replace operation), the Dreamer instantiates dozens of `RealKernel` clones. Each clone involves copying the underlying SQLite memory maps and Go struct pointers.
While the Go runtime garbage collector is efficient, a surge of 100 concurrent clones during an automated refactor will spike memory usage by nearly 30MB instantly. The contract assumes that clones are transient and cheap, but under adversarial or heavy load, this boundary becomes the primary bottleneck for the entire system, shifting the failure mode from logic errors to OOM crashes.

### Semantic Compression in the Interactive Gate
The `VirtualStore` passes all tool arguments into the `ActionRequest.Payload`. For massive tools (like `edit_lines` passing a 1MB file string), this payload moves through the Dreamer, the validators, and eventually into the kernel facts.
Mangle strings are not optimized for MB-scale payloads. Asserting a fact with a 1MB string argument forces the engine to hash and store that massive string. The contract here assumes that tool arguments are small, metadata-like parameters (e.g., line numbers, small diffs). When the LLM decides to dump a whole file into a tool call, the `VirtualStore` blindly passes it, causing severe degradation in the kernel's unification performance.

### Transitive Permissions
When the Dreamer simulates an action, it does so in isolation. It checks `panic_state` based on the exact facts provided. However, some actions trigger secondary actions natively within the tool (e.g., a build tool that also cleans directories).
The `VirtualStore` gate only simulates the primary action. If the secondary action violates policy (e.g., deleting a protected directory during the build), the Dreamer won't catch it because the `ActionRequest` only represents the top-level command. This transitive permission bypass fundamentally undermines the constitutional safety model, proving that integrating dynamic shell execution with declarative policy verification is inherently leaky.

### Conclusion on the Interactive Gate Boundary
The decision to bypass `RouteAction` for interactive tools and instead use wrap-around capability gates (`PreflightDestructiveToolCall` and `ValidateInteractiveToolResult`) was pragmatic but architecturally dangerous. It fractures the execution path, requiring developers to maintain parity between two parallel routing systems. The cache collisions, TOCTOU vulnerabilities, and unmapped tool bypasses are all symptoms of this fractured boundary.
