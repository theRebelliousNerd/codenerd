---
surface: "Session_Executor_VirtualStore_Autopoiesis"
mode: "pipeline"
subsystems_tested: ["Session", "Executor", "VirtualStore", "Autopoiesis"]
blast_radius: "critical"
remediated: false
---

## System Interaction Map
The integration surface between Session Executor, VirtualStore, and Autopoiesis forms a critical pipeline where facts are evaluated, actions are executed via VirtualStore, and learned rules/patterns are synthesized by Autopoiesis.
- `session.Executor.ExecuteToolCall` triggers `virtualStore.ExecuteTool`
- `VirtualStore` routes action and asserts `shell_exec_result` / `file_content` back to the kernel.
- The `kernel.Assert` calls triggers `rule_court` and `autopoiesis` pattern matching.
- `autopoiesis` uses `kernel.Query` to fetch learning facts.

## Contract Analysis
- **Executor & VirtualStore**: The executor expects VirtualStore to return synchronous, well-formed string outputs representing the result of a tool, and to internally assert the relevant Mangle facts into the kernel so that subsequent policy derivations (e.g. `turn_done`) hold.
- **VirtualStore & Kernel**: VirtualStore must successfully assert factual results into the Kernel. It assumes the Kernel will not reject these facts.
- **Autopoiesis & Kernel**: Autopoiesis assumes the kernel will contain structurally valid facts. It listens to the `fact_event_bus` or polls the kernel. If autopoiesis triggers rule synthesis, it relies on `RuleCourt` to validate the learned rules before promoting them to `learned.mg`.

## Failure Mode Enumeration
1. **Temporal**: Autopoiesis takes too long to analyze a turn, stalling the session executor's cleanup phase or blocking subsequent turns.
2. **Semantic**: VirtualStore executes a command but asserts malformed strings (e.g., control characters) that cause kernel parse errors.
3. **Ordering**: Autopoiesis attempts to read facts that the Session Executor has already retracted at the end of the turn.
4. **Partial**: VirtualStore executes a tool halfway, panics, and leaves the kernel in an inconsistent state (tool executed but no fact asserted).
5. **Corruption**: Autopoiesis synthesizes a structurally invalid rule and writes it to disk, corrupting the kernel on the next boot/clone.


## Adversarial Scenario 1: VirtualStore Hollow Success Malformation
**Context:** VirtualStore successfully completes a command but writes an invalid format in the facts due to a serialization edge case.
**Failure Mechanism:** VirtualStore uses an improperly escaped string for the `hollow_success` fact, passing initial struct boundaries but failing at Kernel syntax parsing.
**Expected Behavior:** The Kernel must reject the malformed fact. The Executor must intercept the error and fail the turn gracefully instead of propagating a panic. Autopoiesis must not process the turn.
**Cascading Blast Radius Analysis:** If unhandled, the panic will tear down the Executor goroutine, leaving the ShardManager waiting infinitely or the whole task pipeline corrupted.

## Adversarial Scenario 2: Autopoiesis Ouroboros Rule Synthesis
**Context:** Autopoiesis synthesizes a rule that creates an infinite loop `p(X) :- p(X+1)`.
**Failure Mechanism:** The RuleCourt evaluates the generated rule, but due to a missed stratification check in an edge case, the rule enters an evaluation cycle during validation.
**Expected Behavior:** RuleCourt must enforce a strict timeout during Sandbox evaluation and reject the rule.
**Cascading Blast Radius Analysis:** If the rule persists, the next Mangle evaluation in any shard will hang indefinitely, effectively deadlocking the system's inference capabilities.

## Adversarial Scenario 3: Executor Concurrent Context Cancellation
**Context:** A user cancels the session command exactly as VirtualStore asserts a large payload into the Kernel.
**Failure Mechanism:** Context cancellation hits VirtualStore mid-write. The VirtualStore leaves the transaction half-complete.
**Expected Behavior:** VirtualStore must handle context cancellations cleanly, ensuring no partial facts are left in the Kernel.
**Cascading Blast Radius Analysis:** Partial facts will corrupt future derivations. For example, a `file_content` fact without its corresponding `file_metadata` could cause nil pointer dereferences downstream in the Perception tier.

## Adversarial Scenario 4: Autopoiesis Stale Context Read
**Context:** Autopoiesis queries the Kernel for facts just after the Executor has retracted them at the end of a turn.
**Failure Mechanism:** Autopoiesis queries for `turn_done` but receives an empty result. It assumes an error rather than a completed turn.
**Expected Behavior:** Autopoiesis must handle transient or retracted facts by either caching the needed data earlier or failing gracefully with a log message rather than panicking.
**Cascading Blast Radius Analysis:** An unhandled empty state may trigger an Autopoiesis retry loop that consumes API scheduler slots, starving active Session Executors.

## Adversarial Scenario 5: VirtualStore Output Resource Exhaustion
**Context:** A requested `shell_exec` command produces 500MB of stdout.
**Failure Mechanism:** VirtualStore attempts to allocate the entire string into memory and assert it as a single Mangle Atom.
**Expected Behavior:** VirtualStore must enforce a hard truncation limit before passing the string to the Kernel or Executor.
**Cascading Blast Radius Analysis:** OOM kill of the entire codeNERD binary. All concurrent active sessions and shards are abruptly terminated.

## Adversarial Scenario 6: Autopoiesis Disk Full on Write
**Context:** Autopoiesis successfully validates a rule and attempts to append it to `learned.mg`, but the disk is full.
**Failure Mechanism:** The write operation returns an error, but Autopoiesis has already updated its internal cache.
**Expected Behavior:** Autopoiesis must recognize the write failure, rollback its internal cache, and alert the user without crashing the orchestrator.
**Cascading Blast Radius Analysis:** Cache mismatch with disk state means the current session behaves differently from future sessions, violating the determinism contract.

## Adversarial Scenario 7: VirtualStore Unregistered Tool Call
**Context:** The LLM hallucinates a tool call that is not registered in the VirtualStore but is syntactically valid.
**Failure Mechanism:** VirtualStore attempts to route the unknown tool and hits a default case panic or nil dereference.
**Expected Behavior:** VirtualStore must return a well-formatted `hollow_success` or specific error indicating the tool is unsupported.
**Cascading Blast Radius Analysis:** A panic in VirtualStore crashes the Executor, breaking the TDD loop and leaving the session unrecoverable.

## Adversarial Scenario 8: Autopoiesis Schema Contradiction Synthesis
**Context:** Autopoiesis creates a rule that explicitly contradicts a core policy loaded from `kernel_policy.go`.
**Failure Mechanism:** The RuleCourt fails to cross-reference the generated rule against built-in unmodifiable policies.
**Expected Behavior:** SchemaValidation within RuleCourt must detect the contradiction during the conflict-check phase and reject the rule.
**Cascading Blast Radius Analysis:** If written to `learned.mg`, the kernel will fail to boot on the next instantiation due to conflicting rules.

## Adversarial Scenario 9: Executor Hollow Success Fallback Truncation
**Context:** VirtualStore returns success, but the mutation count is zero. The Executor attempts to derive `hollow_success`.
**Failure Mechanism:** The derived `hollow_success` reason string is too long and gets truncated during serialization.
**Expected Behavior:** The Executor must format the reason properly and the downstream logs should capture the full context.
**Cascading Blast Radius Analysis:** Truncated error messages severely hinder debugging, hiding critical tool failure context from the operator.

## Adversarial Scenario 10: VirtualStore Permission Bypass Attempt
**Context:** An executor instance with limited AgentConfig attempts to call a restricted tool via VirtualStore.
**Failure Mechanism:** VirtualStore fails to validate the tool name against the AgentConfig allowlist before execution.
**Expected Behavior:** VirtualStore or the Executor boundary must explicitly reject the tool call with an ErrPermissionDenied before any execution begins.
**Cascading Blast Radius Analysis:** A malicious or rogue subagent could execute arbitrary shell commands, violating the strict boundaries of the shard.

## Adversarial Scenario 11: Autopoiesis Event Bus Starvation
**Context:** The kernel fact event bus is flooded by 10,000 rapid assertions from a noisy tool loop.
**Failure Mechanism:** Autopoiesis's event listener goroutine falls behind and its channel buffer overflows.
**Expected Behavior:** The event bus must implement backpressure or drop non-critical events safely without hanging the Kernel's `Assert` function.
**Cascading Blast Radius Analysis:** If the channel blocks, the Kernel `Assert` blocks, freezing the entire VirtualStore and Executor pipeline.

## Adversarial Scenario 12: VirtualStore GraphQuery Timeout
**Context:** VirtualStore initiates a complex `graph_query_result` that takes 30 seconds to return from the World Model.
**Failure Mechanism:** The VirtualStore blocking call exceeds the Executor's turn timeout.
**Expected Behavior:** VirtualStore must respect context cancellation and return a timeout error. Executor must handle this as a standard tool failure.
**Cascading Blast Radius Analysis:** Hanging goroutines in the Executor prevent task completion and hold API slots indefinitely.

## Adversarial Scenario 13: Autopoiesis Malformed Piggyback Parsing
**Context:** Autopoiesis analyzes a turn containing a malformed Piggyback JSON control packet.
**Failure Mechanism:** Autopoiesis attempts to unmarshal the raw string and panics.
**Expected Behavior:** Autopoiesis must validate JSON structures securely before accessing fields.
**Cascading Blast Radius Analysis:** Panic crashes the Autopoiesis analyzer, preventing any further rule learning for the lifespan of the service.

## Adversarial Scenario 14: Executor Multi-Turn State Leak
**Context:** Executor runs for 100 iterations, appending temporary contextual facts to the kernel.
**Failure Mechanism:** The Executor fails to retract all temporary facts at the end of each turn, leaking memory in the Kernel.
**Expected Behavior:** Executor must have a robust `defer` or cleanup mechanism guaranteeing complete retraction of turn-specific facts.
**Cascading Blast Radius Analysis:** Kernel memory balloons, eventually causing an OOM or massive query latency degradation during Mangle execution.

## Adversarial Scenario 15: VirtualStore Directory Traversal
**Context:** VirtualStore executes a `file_content` action with a path like `../../../etc/passwd`.
**Failure Mechanism:** VirtualStore does not sanitize the path against the workspace root.
**Expected Behavior:** VirtualStore must enforce a strict sandbox within the workspace root directory.
**Cascading Blast Radius Analysis:** Sensitive system files are exposed to the LLM context, representing a severe security vulnerability.
