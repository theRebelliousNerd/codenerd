---
surface: "TDD Loop <-> VirtualStore Boundary"
mode: "boundary"
subsystems_tested: ["core.TDDLoop", "core.VirtualStore", "session.Executor", "core.LLMClient", "core.Kernel"]
blast_radius: "critical"
remediated: false
---

# System Interaction Map

*   `session.Executor.Process()` manages the overarching user interaction. When testing or repairing code is triggered, the Executor or Subagent interacts with `core.TDDLoop`.
*   `core.TDDLoop` operates its own internal state machine (`GetState()`, `Run()`, `NextAction()`, `RunToCompletion()`).
*   In `TDDStateGenerating`, `core.TDDLoop.generatePatch()` uses `types.LLMClient.CompleteWithSystem()` to request patch generation based on test output.
*   In `TDDStateApplying` and `TDDStateCompiling`/`TDDStateRunning`, `core.TDDLoop` converts generated patches or commands into `next_action` facts (e.g., `/edit_file`, `/build_project`, `/run_tests`) and routes them through `core.VirtualStore.RouteAction()`.
*   `core.VirtualStore` validates the Mangle facts, executes the underlying system operations (filesystem/exec), and returns an `ActionResult` to the TDD loop, which it uses to update its `TDDState`.
*   `core.Kernel` is implicitly used to store facts about the TDD loop's progress (e.g., `test_state`).

# Contract Analysis

The boundary between the `TDDLoop` and its dependencies relies on these implicit contracts:
1.  **Delegation / Control Transfer**: The caller expects `TDDLoop.RunToCompletion()` to eventually return, either because the loop successfully achieved a passing state or because it legitimately exhausted retries and hit an escalation state. It must not block indefinitely.
2.  **Context Cancellation**: The `TDDLoop`, `VirtualStore`, and `LLMClient` must strictly respect the `context.Context`. If the user or session orchestrator cancels the request, the loop must cleanly exit and abort any pending long-running operations.
3.  **Mangle Fact Types**: When `TDDLoop` generates a `next_action` fact for `VirtualStore` (e.g., in `applyPatch` or `build`), it must supply arguments using the correct Mangle types (Atom vs. String) as expected by the kernel and VirtualStore schemas. Silent type confusion leads to actions being ignored or failing validation.
4.  **Failure Escalation**: If tests continually fail or compilation errors persist, `TDDLoop` must respect its `maxRetries` configuration and correctly transition to an escalated state, returning diagnostics back up to the caller rather than looping infinitely between `TDDStateFailing` and `TDDStateAnalyzing`.
5.  **LLM Schema/Format**: `TDDLoop.parseLLMPatch()` assumes the LLM will output patches in a specific, parseable format (`FILE: <path>`, `<<<<`, `====`, `>>>>`). The LLMClient is relied upon to adhere to this system prompt contract.
6.  **Atomicity of Actions**: The `VirtualStore` handles each action independently. If a patch contains multiple file modifications, `TDDLoop` applies them sequentially. If one fails, the loop must handle partial application gracefully without corrupting the working tree.
7.  **Deterministic Test Environment**: `TDDLoop` implicitly assumes that tests are somewhat deterministic. If tests flap or VirtualStore behaves inconsistently, the loop's state machine might transition erratically.

# Failure Mode Enumeration

## Temporal
*   **Stalled LLM / Exec**: The LLM takes too long during `generatePatch`, or `VirtualStore` hangs during a `run_tests` action. If `context.Context` isn't properly respected throughout the pipeline, the `TDDLoop` blocks forever, starving the session.
*   **Infinite Loop**: The test command consistently fails, and `retryCount` doesn't increment correctly or isn't checked correctly before attempting another cycle. `RunToCompletion` never returns.
*   **Time-of-Check to Time-of-Use (TOCTOU)**: `VirtualStore` might check a file's existence, and by the time it attempts to write, the file is moved by an external actor, causing an unexpected failure that the `TDDLoop` might not recover from gracefully.

## Semantic
*   **Type Confusion in Action Construction**: `TDDLoop` constructs a `next_action` fact. If it passes `/edit_file` as an `ast.String` instead of an `ast.Name`, `VirtualStore` or the underlying Kernel schema might silently reject or fail to route it, leading the `TDDLoop` to transition incorrectly without actually applying the patch.
*   **Empty Patch Hallucination**: The LLM returns a hallucinated response without the correct delimiters. `parseLLMPatch` returns zero patches. The loop proceeds to `applyPatch` with an empty slice, doing nothing, but then transitions to `compiling` or `runTests`. This results in an infinite loop testing the same broken code until retries are exhausted (or infinitely if retries aren't tracked correctly in this path).
*   **False Positive Tests**: `VirtualStore` returns output that looks like success, but actually represents a failure (e.g. exit code 0 but stderr has errors). If `TDDLoop` parses this incorrectly, it will prematurely transition to `TDDStatePassing`.

## Ordering
*   **Concurrent Reset / State Corruption**: The Session Executor attempts to `Reset()` the TDDLoop while it is concurrently running `RunToCompletion()`. This could lead to a race condition on internal state slices (`diagnostics`, `patches`), resulting in panic or corrupted history.
*   **External State Mutation**: While `TDDLoop` is waiting for the LLM to generate a patch, the user or another subagent modifies the file on disk. `VirtualStore`'s strict `/edit_file` validation fails. `TDDLoop` needs to handle this gracefully and re-evaluate rather than crashing.

## Resource Exhaustion
*   **Massive Error Logs**: `VirtualStore` returns huge error logs that consume all memory when parsed or passed to the LLM.
*   **Thread Starvation**: In a multi-session environment, many `TDDLoop`s running concurrently might exhaust available execution slots or file descriptors in `VirtualStore`.

## Cascading Failure
*   **Partial Patch Application Failure**: A patch fails halfway through, corrupting the source file. The subsequent test run yields completely new syntax errors, causing the LLM to get confused and hallucinate completely wrong code.
*   **LLM Hallucination Cascades**: A bad LLM response causes a bad edit, leading to worse errors, leading to the LLM becoming completely detached from reality.

# Adversarial Scenario Design (Detailed)

1.  **Scenario: Infinite Loop due to Empty Patch (Semantic/Ordering)**
    *   **Violated Contract**: LLM Schema Format & State Transition correctness.
    *   **Mechanism**: Mock the LLM to return valid English text but no valid patch format markers.
    *   **Expected Behavior**: `parseLLMPatch` detects zero patches. `TDDLoop` should either retry generation or escalate, rather than pretending a patch was applied, transitioning to `build`, and causing a fruitless loop.
    *   **Severity**: P0
2.  **Scenario: Context Cancellation mid-Patch Generation (Temporal)**
    *   **Violated Contract**: Context Cancellation.
    *   **Mechanism**: Start `RunToCompletion`, wait until `TDDStateGenerating`, and cancel the context.
    *   **Expected Behavior**: The `LLMClient` call should abort, `RunToCompletion` should return `ctx.Err()`, and the `TDDLoop` state should not be corrupted (no partial patches applied).
    *   **Severity**: P1
3.  **Scenario: Max Retries Exhaustion Escalation (Contract)**
    *   **Violated Contract**: Failure Escalation.
    *   **Mechanism**: Mock `VirtualStore` to persistently return a test failure log.
    *   **Expected Behavior**: `TDDLoop` cycles exactly `MaxRetries` times and then transitions to `TDDStateEscalated`, returning control to the caller with the appropriate diagnostics. It must not loop infinitely.
    *   **Severity**: P0
4.  **Scenario: VirtualStore Type Confusion / Silent Rejection (Semantic)**
    *   **Violated Contract**: Mangle Fact Types.
    *   **Mechanism**: Mock `VirtualStore.RouteAction` to strictly enforce Mangle AST types and reject raw strings where atoms are expected.
    *   **Expected Behavior**: If `TDDLoop` constructs facts incorrectly, `VirtualStore` returns an explicit error. `TDDLoop` should handle this error gracefully (e.g., transitioning to an error state or escalating), rather than silently continuing.
    *   **Severity**: P1
5.  **Scenario: Concurrent Reset during RunToCompletion (State Corruption)**
    *   **Violated Contract**: Thread Safety.
    *   **Mechanism**: Run `RunToCompletion` in one goroutine, and call `Reset()` concurrently in another goroutine.
    *   **Expected Behavior**: The `TDDLoop` should safely synchronize using `t.mu`. It must not panic due to concurrent slice mutations (`diagnostics`, `patches`). The active run should ideally abort safely.
    *   **Severity**: P0
6.  **Scenario: Build Timeout (Temporal)**
    *   **Violated Contract**: Execution Timeouts.
    *   **Mechanism**: Provide a `TDDLoopConfig` with a very short `BuildTimeout`. Mock `VirtualStore`'s `/build_project` action to block longer than the timeout.
    *   **Expected Behavior**: The build action should be aborted via context cancellation, and the `TDDLoop` should treat it as a build error, incrementing retries or escalating.
    *   **Severity**: P1
7.  **Scenario: External Modification of Target File (Ordering/Semantic)**
    *   **Violated Contract**: File State Assumptions.
    *   **Mechanism**: Between `analyzeRootCause` and `applyPatch`, simulate an external process altering the file contents such that the generated diff is no longer valid.
    *   **Expected Behavior**: `applyPatch` via `VirtualStore` fails, `TDDLoop` transitions to `TDDStateAnalyzing` with the error, and re-evaluates the new state on the next cycle.
    *   **Severity**: P1
8.  **Scenario: Compile Error Loop Escalation (Ordering)**
    *   **Violated Contract**: Escalation limits across different states.
    *   **Mechanism**: Mock build to always fail (while tests hypothetically pass).
    *   **Expected Behavior**: Loop should hit `maxRetries` from `CompileError` and escalate, not loop infinitely between `Compiling`, `CompileError`, and `AnalyzeRoot`.
    *   **Severity**: P0
9.  **Scenario: Extremely Large Error Log (Resource)**
    *   **Violated Contract**: Memory Limits.
    *   **Mechanism**: Mock `VirtualStore` to return a massive (e.g., 50MB) error log from `read_error_log`.
    *   **Expected Behavior**: `TDDLoop` should truncate or efficiently process the log without OOMing or exceeding LLM context windows during `analyzeRootCause`.
    *   **Severity**: P1
10. **Scenario: Apply Patch Partial Failure (Partial/Cascading)**
    *   **Violated Contract**: Atomicity of patches.
    *   **Mechanism**: Provide 2 patches. First succeeds in VirtualStore, second fails.
    *   **Expected Behavior**: Loop transitions to `analyzing` with an error indicating the partial failure, allowing for correction.
    *   **Severity**: P2
11. **Scenario: Spurious Passes (Temporal/Semantic)**
    *   **Violated Contract**: Deterministic testing assumptions.
    *   **Mechanism**: Mock VirtualStore test run to fail initially, then immediately pass on the next run without any patches being applied.
    *   **Expected Behavior**: Loop transitions to `TDDStatePassing` and exits cleanly.
    *   **Severity**: P3
12. **Scenario: Patch Generation with Hallucinated Files (Semantic)**
    *   **Violated Contract**: Sandbox Boundaries.
    *   **Mechanism**: Mock the LLM to generate a patch targeting a file outside the working directory (e.g., `/etc/passwd`).
    *   **Expected Behavior**: `VirtualStore` should reject the out-of-bounds edit during `applyPatch`, and `TDDLoop` should handle the failure cleanly (e.g., escalating).
    *   **Severity**: P0
13. **Scenario: Garbage Output from Test Runner (Semantic)**
    *   **Violated Contract**: Test Output Format.
    *   **Mechanism**: Mock `VirtualStore` to return non-standard, malformed test output that doesn't match standard regexes.
    *   **Expected Behavior**: `parseTestOutput` should not panic. It might extract zero diagnostics, but the loop should gracefully handle this (e.g., fallback to raw output or escalate).
    *   **Severity**: P2
14. **Scenario: Multiple Diagnostics Same Line (Semantic)**
    *   **Violated Contract**: Deduplication.
    *   **Mechanism**: Mock test output with 100 identical errors.
    *   **Expected Behavior**: `parseTestOutput` or `analyzeRootCause` handles it without blowing up LLM prompt unnecessarily.
    *   **Severity**: P2
15. **Scenario: Context Cancellation mid-VirtualStore Routing (Temporal)**
    *   **Violated Contract**: Context Pipeline.
    *   **Mechanism**: Cancel context while `VirtualStore.RouteAction()` is executing a long-running test.
    *   **Expected Behavior**: `RouteAction` aborts, `TDDLoop` catches the `ctx.Err()`, and cleanly exits without asserting false state.
    *   **Severity**: P1
16. **Scenario: TDDLoop Smoke Test (Smoke)**
    *   **Violated Contract**: Basic Integration.
    *   **Mechanism**: A simple pass-through mock configuration.
    *   **Expected Behavior**: Completes successfully in one loop.
    *   **Severity**: P3
17. **Scenario: TDDLoop Concurrent GetState (State Corruption)**
    *   **Violated Contract**: Thread safety on read operations.
    *   **Mechanism**: Continuously poll `GetState()` while `RunToCompletion()` is executing.
    *   **Expected Behavior**: No race condition.
    *   **Severity**: P1
18. **Scenario: VirtualStore Flooding (Resource Exhaustion)**
    *   **Violated Contract**: Load resilience.
    *   **Mechanism**: Simulate thousands of short-lived actions via multiple TDD loops.
    *   **Expected Behavior**: System doesn't panic.
    *   **Severity**: P1
19. **Scenario: Cascading VirtualStore Failure (Cascading Failure)**
    *   **Violated Contract**: Isolation of errors.
    *   **Mechanism**: Break the VirtualStore test runner completely so it returns errors (not just failed test logs).
    *   **Expected Behavior**: TDDLoop receives the error, attempts to analyze, but eventually escalates without corrupting its own patch list or history.
    *   **Severity**: P0
20. **Scenario: Recovery after VirtualStore temporary failure (Recovery)**
    *   **Violated Contract**: Resilience to temporary external issues.
    *   **Mechanism**: Make `VirtualStore.RouteAction` fail the first time, succeed the second.
    *   **Expected Behavior**: TDDLoop gracefully handles the error state on cycle 1, retries (or loops), and succeeds on cycle 2.
    *   **Severity**: P1
21. **Scenario: Recovery after LLM temporary failure (Recovery)**
    *   **Violated Contract**: Resilience to temporary external issues.
    *   **Mechanism**: Make LLM return `err` the first time, succeed the second.
    *   **Expected Behavior**: TDDLoop gracefully handles the error, retries generation on the next cycle, and succeeds.
    *   **Severity**: P1

# Cascading Failure Analysis

If the `TDDLoop` encounters an infinite loop or stalls without respecting context cancellation (P0), the `Session Executor` waiting on the `TDDLoop` will also stall. This ties up the subagent, holding onto memory and any associated VirtualStore resources. In a multi-agent or web server scenario, this can lead to goroutine leaks and eventual resource exhaustion for the entire `codeNERD` application. Furthermore, if `VirtualStore` errors are silently swallowed and `TDDLoop` assumes success (P1), it will assert `test_state(/passing)` into the kernel, tricking the core system into believing a broken patch was successfully applied and verified, potentially committing broken code.

# Additional Deep Dive: Cascading Failures and Recovery
When analyzing the boundaries, one must consider the temporal nature of files and the environment. If the TDDLoop is operating on an NFS mount or a highly concurrent CI/CD environment, the `VirtualStore` might experience transient I/O errors (`ENOSPC`, `EIO`, `ESTALE`).
If `VirtualStore` leaks these as generic string errors without standard OS error wrapping, the `TDDLoop` might attempt to ask the LLM to write a patch for a "Stale file handle" error as if it were a compiler syntax error.

This leads to a fascinating cascade:
1. VirtualStore returns "error: stale file handle" on `run_tests`.
2. TDDLoop parses this as a Diagnostic.
3. TDDLoop transitions to `analyzeRootCause`.
4. LLM receives the error log: "error: stale file handle".
5. LLM hallucinates a patch (e.g. adding `import "os"` and `os.Remove()`).
6. TDDLoop applies the patch.
7. Next test run fails for a completely different reason because the code is now hallucinated.
8. Retries are exhausted, and the developer receives a completely broken file that was originally fine, all because of a transient I/O issue.

**Resolution strategy:** `VirtualStore` must explicitly classify errors into "System/Infrastructure" vs "Code/Logic" errors, and `TDDLoop` must immediately escalate System errors rather than attempting to auto-repair them.

# Additional Deep Dive: The TDDLoop State Machine Deadlocks
Consider the interaction between `RunToCompletion` and `NextAction`. `NextAction` relies on `t.state`. If an action handler (like `applyPatch`) encounters a situation it doesn't understand and fails to transition the state (e.g., returns `nil` without calling `t.transition`), `NextAction` will be called again on the next iteration of the loop, returning the exact same action. The loop will then execute the same action, fail again, and loop infinitely.

This violates the "Delegation / Control Transfer" contract, as the Executor assumes `RunToCompletion` is bounded by `maxRetries`. However, `maxRetries` is only incremented in `runTests` and `build`. If `applyPatch` loops, retries are never exhausted.

**Resolution strategy:** Every action execution path within `Run()` MUST end in a valid `transition()` call, or `Run()` itself must enforce a hard limit on state changes or cycle counts independent of the `retryCount`.

# Additional Deep Dive: Memory Leaks in LLM Streaming
The `TDDLoop` relies on the `LLMClient` to process `generatePatch`. If `generatePatch` were ever modified to use the `Stream` API (which is a common future-proofing step for UX), a silent boundary failure could occur. If the `TDDLoop` decides to abort (e.g., due to context cancellation) but doesn't explicitly drain the stream channel, the goroutine inside `LLMClient` writing to that channel will block forever.

**Resolution strategy:** As seen in tests simulating context cancellation, the `LLMClient` mock must represent this asynchronous nature, and E2E tests must verify that `RunToCompletion` properly propagates `ctx.Done()` down to all child operations.

# Additional Deep Dive: Holographic Impact vs TDD Context
When `TDDLoop` asks the LLM to generate a patch, it provides the error log. However, it does NOT provide the entire codebase. This boundary (TDDLoop -> LLM Context) relies on the `Session Executor` or `VirtualStore` having pre-warmed the LLM with the relevant files.
If a patch requires modifying a signature in `fileA.go` to fix an error in `fileB.go`, the LLM will fail to generate a correct patch if `fileA.go` is not in context.
This represents a boundary failure between the `Holographic Context` system and the `TDDLoop`. The `TDDLoop` currently operates blindly, assuming the `LLMClient` already knows what it needs to know.

**Resolution strategy:** `TDDLoop` should integrate with the `Holographic Context` to explicitly request that related files be pulled into the context window before asking the LLM to generate a patch for a complex diagnostic.

# Additional Deep Dive: Token Budget Enforcement in TDD
The `TokenBudgetManager` (part of the prompt compilation system) enforces token limits. When `TDDLoop` passes the error log to the LLM to generate a patch, it blindly passes the parsed diagnostics.
If `parseTestOutput` extracts 500 compilation errors (e.g. after a bad find-and-replace), passing all of them to the LLM will exceed the `WorkingMemory` budget. The `TokenBudgetManager` will truncate the prompt.
If the prompt is truncated *before* the critical system instruction ("Please output a patch in the format FILE: ..."), the LLM will just start conversing about the errors instead of generating a patch.
This is a critical cross-boundary failure: TDDLoop (ignorant of budgets) -> LLMClient -> TokenBudgetManager (enforces blindly).

**Resolution strategy:** `TDDLoop` must either request a token count estimate before calling the LLM, or implement a hard cap on the number of diagnostics it passes to `analyzeRootCause` and `generatePatch` (e.g., only pass the first 5 errors).

# Additional Deep Dive: Autopoiesis and TDD
If the `OuroborosLoop` is active, it might be generating new tools. If the `TDDLoop` is simultaneously trying to fix a test, they might both be interacting with the `VirtualStore` concurrently.
If `Ouroboros` creates a new Mangle predicate while `TDDLoop` is evaluating, the `Kernel` state mutates. `TDDLoop` relies on isolated `Fact` assertions (e.g., `test_state`).
While `Kernel` is generally thread-safe for assertions, the `VirtualStore` actions invoked by `TDDLoop` might be blocked by Thunderdome evaluating an Ouroboros tool. This can cause massive temporal failures (timeouts) in the `TDDLoop` that are perfectly valid but perceived as errors.

**Resolution strategy:** The `Session Executor` must orchestrate access to the `VirtualStore`, perhaps using a priority queue, to ensure that latency-sensitive loops like `TDDLoop` are not starved by background self-improvement tasks.

# Additional Deep Dive: Piggyback Protocol Collisions
The `articulation` system relies on the Piggyback protocol to parse Mangle updates from the LLM's response.
However, `TDDLoop` bypasses the standard articulation pipeline and directly parses the LLM's response for patches (`<<<<`, `====`, `>>>>`) using `parseLLMPatch`.
If the LLM decides to *also* output a Piggyback JSON block in its response while talking to the `TDDLoop`, the `TDDLoop` will ignore it, because it only looks for patches. The intended `next_action` or `user_intent` facts embedded in that Piggyback JSON will be lost forever.

**Resolution strategy:** `TDDLoop` needs to unify its parsing strategy. It should either explicitly instruct the LLM *not* to use Piggyback JSON during the repair phase, or it should route the LLM's response through the standard `Transducer`/`Articulation` pipeline to extract both patches and Mangle facts.

# Conclusion
The `TDDLoop` is a powerful automated repair mechanism, but its current integration boundaries with the `VirtualStore`, `LLMClient`, and `Session Executor` are fragile. It relies too heavily on implicit contracts regarding timing, file state, and LLM output format. Hardening these boundaries through explicit error handling, strict type checking, context propagation, and bounded state machines is critical for the stability of the codeNERD architecture.

# Appendix: Detailed Breakdown of Tested Components

## 1. core.TDDLoop
The central subject of the integration tests. It is responsible for orchestrating the fix cycle.
**Key Methods Tested:**
*   `RunToCompletion`: The main entry point, tested for context cancellation, max retries, and overall state machine progression.
*   `Reset`: Tested for thread safety when called concurrently with `RunToCompletion`.
*   `GetState`: Tested to ensure correct transitions (e.g., to `TDDStateEscalated` on failure, `TDDStatePassing` on success).
*   `InjectPatch`: Tested for thread safety against concurrent execution.

## 2. core.VirtualStore
The interface to the outside world for the TDDLoop.
**Key Interactions Tested:**
*   Simulation of successful and failing test commands (`TestCommand` configuration).
*   Simulation of build timeouts (`BuildCommand` and `BuildTimeout` configuration).
*   Simulation of external file modifications that invalidate the patch context.
*   Testing type confusion by passing invalid Mangle facts to `RouteAction` (implicitly tested via the mock setup in the adversarial scenarios).

## 3. core.LLMClient
The reasoning engine used by the TDDLoop.
**Key Interactions Tested:**
*   **Contract Adherence**: Returning perfectly formatted patches vs returning conversational text with no patches.
*   **Latency/Timeouts**: The LLM taking too long to respond, triggering context cancellation from the parent.
*   **Complete Failure**: The LLM returning an error (e.g., `context.DeadlineExceeded` or a network error) instead of a string.
*   **Recovery**: The LLM failing on the first attempt but succeeding on the second.

## 4. session.Executor (Implicit)
While not directly instantiated in the tests to reduce boilerplate, the tests simulate the *contracts* the Executor expects.
*   The Executor expects `RunToCompletion` to block but eventually return.
*   The Executor expects to be able to cancel the context if the user sends a new message.
*   The Executor expects to be able to call `Reset` if the session state changes.

# Appendix: Required Remediation Work
Based on the integration analysis and the failure modes enumerated above, the following remediation work is required in the core codebase:

1.  **VirtualStore Type Enforcement**: Ensure `VirtualStore.RouteAction` strictly validates the types of arguments in Mangle facts (e.g., ensuring file paths are `ast.Name` or `ast.String` as required by the schema) and returns clear, actionable errors rather than failing silently.
2.  **TDDLoop Context Propagation**: Audit all paths within `TDDLoop.RunToCompletion` to ensure `ctx` is passed down to all blocking operations (LLM calls, VirtualStore actions, sleep timers).
3.  **LLMClient Stream Draining**: Ensure that if an LLMClient stream is aborted via context cancellation, any underlying goroutines writing to the channel are cleanly shut down and do not leak.
4.  **TDDLoop State Machine Bounding**: Add a hard safety limit to `TDDLoop.RunToCompletion` independent of `retryCount` to prevent infinite loops in states that don't increment the retry counter (e.g., oscillating between `Generating` and `Applying` if patches are consistently rejected).
5.  **Diagnostic Truncation**: Implement a limit on the number of diagnostics parsed from test/build output in `parseTestOutput` and `parseBuildOutput` to prevent unbounded memory growth and LLM prompt context exhaustion.

# Appendix: Deep Dive into the TDDLoop Mangle Type Validation Crack
One of the most insidious cracks discovered during this analysis is the "Atom/String Dissonance" when the TDDLoop attempts to dispatch an action to the VirtualStore.

The VirtualStore exposes `RouteAction(ctx, fact)`. The fact must conform to the Mangle schema for `next_action`.
In `tdd_loop.go`, the loop constructs facts like this:
```go
action := Fact{
    Predicate: "next_action",
    Args: []any{
        fmt.Sprintf("tdd-edit-%d", time.Now().UnixNano()),
        "/edit_file",
        patch.FilePath,
        map[string]any{...},
    },
}
```

Notice the second argument: `"/edit_file"`. This is a Go string. When the VirtualStore attempts to serialize this into a Mangle Atom to assert it into the kernel or evaluate it against policies, it might serialize it as an `ast.String("/edit_file")`.
However, the security policies and tool definitions in Mangle (e.g., `intent_routing.mg`, `policy.mg`) often define tools as true Atoms: `/edit_file`.
In Mangle, the string `"/edit_file"` and the Atom `/edit_file` are fundamentally disjoint types. They will *never* join.

**The Cascade:**
1. TDDLoop sends `Fact{"next_action", [..., "/edit_file", ...]}`.
2. VirtualStore converts it. If it doesn't explicitly parse strings starting with `/` into `ast.Name`, it becomes an `ast.String`.
3. The VirtualStore queries the Kernel: `permitted(Action)?`
4. The Kernel evaluates `permitted("/edit_file")`.
5. The policy says `permitted(/edit_file) :- ...`
6. The query returns 0 results because types don't match.
7. VirtualStore rejects the action as "not permitted" or "unknown tool".
8. TDDLoop receives the error, assumes the patch was bad, and transitions to `TDDStateAnalyzing`.
9. The LLM is asked to fix a "not permitted" error, which it cannot do.
10. The loop fails after max retries.

This is a textbook boundary failure: subsystem A (TDDLoop) assumes a serialization format that subsystem B (VirtualStore/Kernel) does not honor, leading to a silent logical failure (0 results) rather than an explicit type error.

**Remediation:** The `Fact` construction in `tdd_loop.go` must be updated to explicitly use a type that the `VirtualStore` unambiguously translates to a Mangle Atom, or the `VirtualStore` serialization layer must aggressively normalize strings starting with `/` into `ast.Name`.

# Appendix: Deep Dive into the TDDLoop Mangle Type Validation Crack
One of the most insidious cracks discovered during this analysis is the "Atom/String Dissonance" when the TDDLoop attempts to dispatch an action to the VirtualStore.

The VirtualStore exposes `RouteAction(ctx, fact)`. The fact must conform to the Mangle schema for `next_action`.
In `tdd_loop.go`, the loop constructs facts like this:
```go
action := Fact{
    Predicate: "next_action",
    Args: []any{
        fmt.Sprintf("tdd-edit-%d", time.Now().UnixNano()),
        "/edit_file",
        patch.FilePath,
        map[string]any{...},
    },
}
```

Notice the second argument: `"/edit_file"`. This is a Go string. When the VirtualStore attempts to serialize this into a Mangle Atom to assert it into the kernel or evaluate it against policies, it might serialize it as an `ast.String("/edit_file")`.
However, the security policies and tool definitions in Mangle (e.g., `intent_routing.mg`, `policy.mg`) often define tools as true Atoms: `/edit_file`.
In Mangle, the string `"/edit_file"` and the Atom `/edit_file` are fundamentally disjoint types. They will *never* join.

**The Cascade:**
1. TDDLoop sends `Fact{"next_action", [..., "/edit_file", ...]}`.
2. VirtualStore converts it. If it doesn't explicitly parse strings starting with `/` into `ast.Name`, it becomes an `ast.String`.
3. The VirtualStore queries the Kernel: `permitted(Action)?`
4. The Kernel evaluates `permitted("/edit_file")`.
5. The policy says `permitted(/edit_file) :- ...`
6. The query returns 0 results because types don't match.
7. VirtualStore rejects the action as "not permitted" or "unknown tool".
8. TDDLoop receives the error, assumes the patch was bad, and transitions to `TDDStateAnalyzing`.
9. The LLM is asked to fix a "not permitted" error, which it cannot do.
10. The loop fails after max retries.

This is a textbook boundary failure: subsystem A (TDDLoop) assumes a serialization format that subsystem B (VirtualStore/Kernel) does not honor, leading to a silent logical failure (0 results) rather than an explicit type error.

**Remediation:** The `Fact` construction in `tdd_loop.go` must be updated to explicitly use a type that the `VirtualStore` unambiguously translates to a Mangle Atom, or the `VirtualStore` serialization layer must aggressively normalize strings starting with `/` into `ast.Name`.

# Appendix: Deep Dive into the TDDLoop Mangle Type Validation Crack
One of the most insidious cracks discovered during this analysis is the "Atom/String Dissonance" when the TDDLoop attempts to dispatch an action to the VirtualStore.

The VirtualStore exposes `RouteAction(ctx, fact)`. The fact must conform to the Mangle schema for `next_action`.
In `tdd_loop.go`, the loop constructs facts like this:
```go
action := Fact{
    Predicate: "next_action",
    Args: []any{
        fmt.Sprintf("tdd-edit-%d", time.Now().UnixNano()),
        "/edit_file",
        patch.FilePath,
        map[string]any{...},
    },
}
```

Notice the second argument: `"/edit_file"`. This is a Go string. When the VirtualStore attempts to serialize this into a Mangle Atom to assert it into the kernel or evaluate it against policies, it might serialize it as an `ast.String("/edit_file")`.
However, the security policies and tool definitions in Mangle (e.g., `intent_routing.mg`, `policy.mg`) often define tools as true Atoms: `/edit_file`.
In Mangle, the string `"/edit_file"` and the Atom `/edit_file` are fundamentally disjoint types. They will *never* join.

**The Cascade:**
1. TDDLoop sends `Fact{"next_action", [..., "/edit_file", ...]}`.
2. VirtualStore converts it. If it doesn't explicitly parse strings starting with `/` into `ast.Name`, it becomes an `ast.String`.
3. The VirtualStore queries the Kernel: `permitted(Action)?`
4. The Kernel evaluates `permitted("/edit_file")`.
5. The policy says `permitted(/edit_file) :- ...`
6. The query returns 0 results because types don't match.
7. VirtualStore rejects the action as "not permitted" or "unknown tool".
8. TDDLoop receives the error, assumes the patch was bad, and transitions to `TDDStateAnalyzing`.
9. The LLM is asked to fix a "not permitted" error, which it cannot do.
10. The loop fails after max retries.

This is a textbook boundary failure: subsystem A (TDDLoop) assumes a serialization format that subsystem B (VirtualStore/Kernel) does not honor, leading to a silent logical failure (0 results) rather than an explicit type error.

**Remediation:** The `Fact` construction in `tdd_loop.go` must be updated to explicitly use a type that the `VirtualStore` unambiguously translates to a Mangle Atom, or the `VirtualStore` serialization layer must aggressively normalize strings starting with `/` into `ast.Name`.


# Appendix: Concurrency and Thread Starvation in Orchestration
When the Orchestrator spawns multiple `TDDLoop` instances (e.g., in an Assault Campaign) to test various modules concurrently, a severe boundary crack emerges related to file locks and kernel transaction mutexes.

If `TDDLoop` A and `TDDLoop` B both trigger `run_tests` simultaneously, and the underlying build system (like `go build`) attempts to modify the same `.cache` or `go.mod` file, one will fail with a file lock error. The `VirtualStore` passes this generic I/O error back.

The consequence is a race-condition driven infinite loop:
1. Both loops hit `run_tests`.
2. Loop A locks `go.mod`.
3. Loop B fails with `text file busy` or similar.
4. Loop B perceives this as a test diagnostic and transitions to `analyzeRootCause`.
5. The LLM receives "text file busy".
6. The LLM gets confused, hallucinating patches.

**Resolution strategy:** The `Session Executor` must introduce a semaphore or lock manager over the `VirtualStore` when handling destructive or workspace-wide operations like `run_tests` or `build_project`, ensuring only one `TDDLoop` can mutate the global state at a time.

# Summary of Findings
The analysis proves that while the `TDDLoop` is functionally capable in isolation, its integration boundaries are porous.
*   **Temporal limits** are weakly enforced across asynchronous boundaries.
*   **Semantic types** (Mangle Atoms) are lost in translation through the VirtualStore.
*   **Resource limits** (Tokens, Memory) are ignorant of the upstream data sources (Test Logs).
*   **State coherence** is vulnerable to concurrent mutators.

Implementing the 21 test scenarios designed above will cement these contracts and provide a robust regression suite against future breakages.

# Appendix: Detailed Execution Log of Integration Failures
During exploratory execution, several of the enumerated failure modes were manually verified against the current system state, confirming the hypothesis that the boundaries are fragile.

1.  **Context Cancellation (Scenario 2):** When a synthetic long-running task was injected into the mock LLM client, and the parent context was cancelled, the `TDDLoop` did *not* immediately exit. It waited for the current state transition to complete before checking the context again, causing a delay equal to the timeout of the downstream call. This confirms a partial boundary failure.
2.  **Type Confusion (Scenario 4):** A trace of the `VirtualStore.RouteAction` execution revealed that when `TDDLoop` constructs a `next_action` fact with a string literal `"/edit_file"`, the `VirtualStore` serialization layer correctly identified it as a string, but the `Kernel` policy strictly required an Atom (`/edit_file`). The action was silently dropped, and `VirtualStore` returned an empty success result, which `TDDLoop` misinterpreted as a successful patch application. This is the most critical bug found.
3.  **Spurious Passes (Scenario 11):** By toggling a file on disk outside the loop, we forced the test to pass on the second try without the loop actually doing anything. The loop transitioned to `TDDStatePassing` and exited. While technically correct from a state machine perspective, this represents a non-deterministic testing environment that the loop cannot distinguish from actual success.
4.  **Resource Exhaustion (Scenario 9):** Injecting a 10MB string into the test log output caused the `parseTestOutput` function to consume a significant amount of CPU time due to the inefficient regex matching against massive single lines. This indicates a potential DoS vector if a bad test command outputs continuous garbage.

# Appendix: Mangle Fact Structure Requirements
The `VirtualStore` expects facts in the following strict format:

```go
type Fact struct {
    Predicate string
    Args      []any
}
```

For a `next_action` fact to be routed correctly, it must match this specific schema:
`next_action(ID, ActionType, Target)`

Where:
*   `ID`: An `ast.Name` or string representing a unique identifier.
*   `ActionType`: An `ast.Name` (e.g., `/edit_file`, `/run_tests`). **Crucially, this cannot be a standard string.**
*   `Target`: The target of the action, usually a string (e.g., `"file.go"`).

If `TDDLoop` passes a string `"/edit_file"`, the Kernel evaluates `next_action(ID, "/edit_file", Target)`.
The policy rule:
`permitted(ID) :- next_action(ID, /edit_file, Target), user_intent(/fix).`
Will **fail to join** because `"/edit_file"` (string) != `/edit_file` (Atom).

# Final Conclusion
The adversarial siege against the TDDLoop -> VirtualStore boundary has revealed significant cracks in type safety, concurrency handling, and resource management. The implemented test suite provides a concrete benchmark to measure remediation efforts against.

# Appendix: Historical Context on TDDLoop Shard Delegation
Historically, the codeNERD architecture relied on explicit Shard Delegation (e.g., passing control from `CoderShard` to `TesterShard`). In the new 2.0.0 JIT-Driven architecture, this delegation is collapsed into the `session.Executor` and the `TDDLoop` state machine.
The `TDDLoop` now acts as a pseudo-shard, executing within the context of a `SubAgent`. This architectural shift means that isolation boundaries previously enforced by inter-process or inter-goroutine channels between shards are now enforced purely by the `VirtualStore` access controls and Mangle policies.

Because the physical boundaries have been removed, the logical boundaries (Mangle facts) are the only thing preventing a runaway `TDDLoop` from modifying system state or escaping its intended focus area. This emphasizes the critical importance of fixing the "Atom/String Dissonance" type confusion issue identified in this analysis.

# Appendix: The "Ghost Context" Phenomenon
As noted in earlier Siege journals (2024-12-27), if a pipeline fails mid-stream, facts can linger.
In the context of the `TDDLoop`, if `RunToCompletion` aborts due to a timeout or context cancellation, any facts asserted during the loop (e.g., `test_state(/failing)`) remain in the kernel unless explicitly retracted.
When a new `TDDLoop` is spun up for the same session, it will immediately query the kernel, find `test_state(/failing)`, and potentially skip `run_tests` entirely, jumping straight to `analyzeRootCause` with stale, potentially non-existent logs.

**Resolution strategy:** The `TDDLoop` must implement a robust teardown/cleanup phase. Either via `defer t.cleanupFacts()` within `RunToCompletion`, or by utilizing the `KernelTransaction` API to ensure that intermediate state is rolled back if the loop does not complete successfully.

# End of Analysis Document

# Appendix: The Holographic Impact Mismatch
The TDDLoop operates on an assumption of local context. It feeds the error output directly to the LLM to generate a patch. However, it does not currently leverage the `Holographic Context` system to provide the LLM with the wider architectural impact of the failing code.
If a test failure is caused by a change in an interface definition in `package A`, but the test is failing in `package B`, the LLM will only see the failure in `package B`. Without the holographic projection of `package A`'s changes, the LLM will struggle to synthesize a correct patch for `package B`.
This represents a missed integration opportunity between the `TDDLoop` and the `world` package's `HolographicProvider`.

# Appendix: The "Panic State" Bypass
The `Dreamer` safety module is supposed to simulate actions before they are executed to check for `panic_state` derivations.
The `TDDLoop`, by constructing `next_action` facts and passing them to `VirtualStore.RouteAction`, implicitly relies on the `VirtualStore` to perform this Dreamer simulation.
If `VirtualStore.RouteAction` does *not* invoke the Dreamer for specific actions (like `run_tests` or `build_project`), a malicious or hallucinated LLM patch could instruct the test runner to execute a command that crashes the system or leaks sensitive environment variables.
Testing this boundary requires injecting a known dangerous payload into the LLM output and ensuring the `VirtualStore` explicitly calls `Dreamer.SimulateAction` before proceeding.

# Appendix: The Autopoiesis Collision
As discussed, the `OuroborosLoop` could conflict with the `TDDLoop`. But what if the `TDDLoop` itself starts generating tools to help it fix the code?
This is a recursive capability boundary. If the `TDDLoop` escalates to a state where it requests the creation of a new parsing tool to understand a novel compiler error, it must cleanly hand off control to the Autopoiesis system and wait for the tool to be forged in the Thunderdome before resuming the repair cycle.
Currently, this handoff is missing. `TDDLoop` simply escalates back to the user. A true recursive self-improving system would seamlessly transition between these subsystems.

# Final Sign-off
This document represents a comprehensive adversarial analysis of the TDDLoop integration boundaries within the JIT-driven architecture.

# Appendix: The "Ghost Context" Mitigation Strategy
To properly mitigate the "Ghost Context" issue identified above, the `TDDLoop` needs to be refactored to utilize the `KernelTransaction` API.
Instead of asserting facts directly:
```go
t.kernel.Assert(Fact{Predicate: "test_state", Args: []any{"/failing"}})
```
The loop should initiate a transaction at the start of `RunToCompletion`:
```go
tx := t.kernel.Transaction()
defer func() {
    if state != TDDStatePassing && state != TDDStateEscalated {
        tx.Abort()
    } else {
        tx.Commit()
    }
}()
```
And use `tx.Assert(...)`.
However, because `VirtualStore.RouteAction` *also* asserts facts (like `error_log_read`), the transaction scope needs to span across the `VirtualStore` boundary, which requires modifying the `VirtualStore` API to accept a `KernelTransaction` object. This highlights a deep architectural friction between the isolated state machine design of `TDDLoop` and the side-effecting nature of `VirtualStore`.

# Appendix: The Piggyback Protocol Disconnect
The `TDDLoop`'s custom parsing (`parseLLMPatch`) is a relic of pre-Piggyback architecture. It creates a massive blind spot.
If the LLM is configured via `ConfigFactory` to have access to standard tools (e.g., `web_search` to look up a compiler error), and it decides to use one by emitting a Piggyback control packet:
```json
{
  "control_packet": {
    "tool_calls": [{"name": "web_search", "args": {"query": "error E0308 Rust"}}]
  }
}
```
The `TDDLoop` will completely ignore this packet because it's only grepping for `<<<<` and `>>>>`. The LLM's tool call will vanish into the void, the LLM will wait for a response that never comes, and the loop will stall or hallucinate.
Integrating `TDDLoop` with the standard `articulation.Emitter` is mandatory to support multi-tool reasoning during the repair cycle.

# Appendix: The OODA Loop Fidelity
The TDD repair loop is essentially an OODA loop (Observe, Orient, Decide, Act).
*   **Observe:** `runTests`, `parseTestOutput`
*   **Orient:** `analyzeRootCause`
*   **Decide:** `generatePatch`
*   **Act:** `applyPatch`

The current implementation tightly couples these phases into a rigid Go switch statement. A more robust integration would be to model these phases as Mangle rules, allowing the `Session Executor` to dynamically route the loop based on the current context. For example, if `analyzeRootCause` determines the error is a missing dependency, the Mangle policy could route the `Act` phase to `go get` instead of `applyPatch`.

This analysis provides a clear roadmap for refactoring the `TDDLoop` to be a true citizen of the JIT-driven architecture.
