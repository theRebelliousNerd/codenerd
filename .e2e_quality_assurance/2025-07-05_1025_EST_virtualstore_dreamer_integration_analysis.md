---
surface: "VirtualStore ↔ Dreamer Gate"
mode: "boundary"
subsystems_tested: ["VirtualStore", "Dreamer", "InteractiveGate"]
blast_radius: "critical"
remediated: false
---

## System Interaction Map

*   `VirtualStore.PreflightDestructiveToolCall(ctx, actionID, toolName, args)` -> `interactiveToolActionType[toolName]`
*   `VirtualStore.PreflightDestructiveToolCall` -> `Dreamer.SimulateAction(ctx, req)`
*   `VirtualStore.ValidateInteractiveToolResult(ctx, actionID, toolName, args, output, success)` -> `ValidatorRegistry.Validate(ctx, req, res)`

## Contract Analysis

*   **VirtualStore to Dreamer:** VirtualStore assumes that Dreamer handles unexpected action types gracefully or fails closed. However, `PreflightDestructiveToolCall` explicitly states a **Fail-OPEN policy**: if Dreamer is unavailable or returns an unhandled error, it allows the action. If Dreamer returns `Unsafe: true`, VirtualStore blocks it.
*   **Dreamer to Cache:** Dreamer's cache relies on a deterministic string key `dreamCacheKey(req ActionRequest) string { return string(req.Type) + ":" + req.Target }`.
*   **VirtualStore to Validators:** VirtualStore relies on Validators returning `Confidence >= 0.8` to mark validation as failed.

## Failure Mode Enumeration

1.  **Temporal:** `SimulateAction` hangs indefinitely if context is not respected, starving VirtualStore execution.
2.  **Semantic:** `interactiveToolActionType` map is missing a destructive tool, bypassing the Dreamer completely (Fail-OPEN policy).
3.  **Corruption (Cache):** Two concurrent actions with identical Type and Target (e.g., `write_file` to `target.txt` with DIFFERENT payloads) overwrite each other's cache entries or reuse stale results.
4.  **Semantic (Fail-OPEN):** If Dreamer panics or returns a nil result (if possible), does it bypass safety?
5.  **Partial Pipeline:** Validation fails with confidence >= 0.8 AFTER a destructive tool has executed. Tool effects are applied, but validation failure returns `InteractiveGateError`, misleading the user about the actual system state.

## Adversarial Scenarios

1.  **Scenario: Cache Collision on Identical Target but Different Payload**
    *   **Contract:** DreamCache correctly isolates distinct actions.
    *   **Mechanism:** Concurrent `write_file` actions to the same file (`target.txt`) with different payloads (one safe, one malicious). Both generate the same cache key `write_file:target.txt`.
    *   **Behavior:** The malicious action might hit the cache entry of the safe action and bypass the Dreamer check.
    *   **Severity:** P0
2.  **Scenario: Fail-Open Bypass via Unknown Tool Name**
    *   **Contract:** Only safe actions bypass Dreamer.
    *   **Mechanism:** An LLM hallucinates a tool name like `rm_rf` or `destructive_write`.
    *   **Behavior:** `actionTypeForToolName` returns `false`, `PreflightDestructiveToolCall` returns `nil` (Fail-OPEN), allowing the action if the executor blindly runs it.
    *   **Severity:** P1
3.  **Scenario: Context Cancellation during Dreamer Simulation**
    *   **Contract:** Dreamer respects context cancellation and aborts simulation gracefully.
    *   **Mechanism:** Cancel the context passed to `PreflightDestructiveToolCall` immediately.
    *   **Behavior:** System shouldn't panic, but what is the return value? Does it fail open or closed upon cancellation error?
    *   **Severity:** P2
4.  **Scenario: Validator Panic on Malformed Output**
    *   **Contract:** Validators handle unexpected output formats robustly.
    *   **Mechanism:** Feed a huge/malformed string to `ValidateInteractiveToolResult`.
    *   **Behavior:** Validator panic might propagate up and crash VirtualStore or just fail validation.
    *   **Severity:** P2
5.  **Scenario: High Confidence Validation Failure Masks Real Success**
    *   **Contract:** `ValidateInteractiveToolResult` should accurately reflect state.
    *   **Mechanism:** A tool writes a file successfully, but a faulty validator returns failure with 0.9 confidence.
    *   **Behavior:** The function returns `InteractiveGateError`, but the file is actually modified, desyncing Mangle facts from reality.
    *   **Severity:** P1
6. **Scenario: Dreamer Sandbox OOM/Timeout Propagation**
   *   **Contract:** Dreamer safely terminates expensive simulations without crashing the process.
   *   **Mechanism:** Send an extremely complex rule evaluation that triggers a sandbox OOM.
   *   **Behavior:** Dreamer should return `Unsafe: true` with a reason.
   *   **Severity:** P2
7. **Scenario: Validation Fact Overwrite**
   *   **Contract:** Successive validation facts update the kernel accurately.
   *   **Mechanism:** Two concurrent validations for the same target.
   *   **Behavior:** Kernel facts might interleave incorrectly if not synchronized.
   *   **Severity:** P2
8. **Scenario: Missing Target Resolution**
   *   **Contract:** `extractActionTarget` finds the right file.
   *   **Mechanism:** Tool call uses an unusual parameter key not in the list.
   *   **Behavior:** Target becomes `"unknown"`. All such actions collapse into the same cache key `"write_file:unknown"`.
   *   **Severity:** P1
9. **Scenario: Dreamer Unavailability Fail-Open Cascade**
   *   **Contract:** If Dreamer is nil, Fail-Open applies.
   *   **Mechanism:** Initialize VirtualStore without a Dreamer, execute malicious action.
   *   **Behavior:** Action executes.
   *   **Severity:** P1
10. **Scenario: False Positive Validation Rollback**
    *   **Contract:** If validation fails, no rollback is done.
    *   **Mechanism:** Action writes, validator fails, VirtualStore returns error, but file remains modified.
    *   **Behavior:** Caller assumes failure, retries same action, causing potential duplication or state corruption.
    *   **Severity:** P1
11. **Scenario: Interactive Gate ActionType Mismatch**
    *   **Contract:** `interactiveToolActionType` matches actual modular tools exactly.
    *   **Mechanism:** A tool is registered as `"write_file_safe"` but not added to the map.
    *   **Behavior:** It's treated as non-destructive and bypasses Dreamer.
    *   **Severity:** P1
12. **Scenario: Payload Mutation During Preflight**
    *   **Contract:** Dreamer doesn't mutate `req.Payload`.
    *   **Mechanism:** Check if Dreamer alters payload map, affecting later execution.
    *   **Behavior:** Execution uses corrupted payload.
    *   **Severity:** P3
13. **Scenario: Validation with Missing Kernel**
    *   **Contract:** `processValidationResults` handles nil kernel gracefully.
    *   **Mechanism:** Kernel is nil, validation passes.
    *   **Behavior:** Should not panic.
    *   **Severity:** P3
14. **Scenario: Preflight on Non-Destructive Action returns error**
    *   **Contract:** Should return nil.
    *   **Mechanism:** Call with `read_file`.
    *   **Behavior:** Returns nil quickly.
    *   **Severity:** P3
15. **Scenario: Concurrent Dreamer and Validation**
    *   **Contract:** Independent read/write locks.
    *   **Mechanism:** Run preflight and validate concurrently on the same VS.
    *   **Behavior:** Should not deadlock.
    *   **Severity:** P2

## Cascading Failure Analysis

If DreamCache corruption (Scenario 1) occurs, unsafe actions are executed. This pollutes the file system. Mangle facts diverge from the actual file state. The Campaign Orchestrator might then proceed with a corrupted workspace, leading to a cascade of failing tests or broken builds, ultimately stalling the entire agent loop.

If Fail-Open Bypass (Scenario 2) occurs, an attacker bypasses the safety gate completely. This could lead to a reverse shell, unauthorized data exfiltration, or complete host compromise.

If High Confidence Validation Failure Masks Real Success (Scenario 5) occurs, the Session Executor enters a "Schrödinger's State". The tool modified the filesystem, but the LLM is told it failed. The LLM might retry the action, potentially duplicating data or breaking the file further, creating an unrecoverable state divergence.
