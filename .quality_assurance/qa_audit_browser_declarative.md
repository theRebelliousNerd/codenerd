# QA Audit Journal: Boundary Value & Negative Testing
# Date: 2026-09-19
# Time: 00:22:33 EST
# Target: internal/tools/research/browser_declarative.go and browser_test_test.go

## 1. Executive Summary

This journal entry documents a comprehensive Quality Assurance audit of the codeNERD `browser_test` subsystem (`browser_declarative.go`). As a QA Automation Engineer specializing in Boundary Value Analysis (BVA) and Negative Testing, the focus of this review is strictly on uncovering systemic vulnerabilities, edge cases, and unexpected behaviors that occur outside the "happy path".

The `browser_test` subsystem is responsible for creating, inspecting, generating, and running bounded declarative browser tests. It interfaces heavily with the live Cortex session, executing operations, and asserting states using Google's Mangle logic programming language. Due to its position as a bridge between the LLM's unstructured output and the deterministic, highly structured browser automation layer, it is a prime candidate for edge case exploitation.

The primary vectors evaluated in this audit include:
- Null, Undefined, and Empty input handling.
- Type Coercion and Schema Violations.
- User Request Extremes (e.g., massive payloads, frontier-level stress).
- State Conflicts and Race Conditions.

## 2. Architectural Context & Mangle Integration

Before diving into specific failure modes, it is crucial to understand the architectural constraints of the codeNERD framework, specifically the "JIT Clean Loop" and the strict adherence to typed Mangle facts.

The system uses `map[string]any` to receive arguments from the LLM. This lack of static typing at the boundary is a classic source of type coercion bugs. The system attempts to cast these values (e.g., `args["operation"].(string)`), which can panic if the LLM hallucinated a different type (like an array or object).

Furthermore, the integration with Mangle requires precise mapping of Go types to Mangle AST types. The `subtractBrowserFacts` function relies on a JSON-marshaled fingerprint of the fact to compute the set difference between the baseline and the fresh state. If the arguments contain non-deterministic types or unmarshalable structures, the fingerprinting will silently fail, leading to false positives or negatives in the test assertions.

## 3. Vector 1: Null / Undefined / Empty Inputs

The most fundamental negative tests involve missing data. The LLM might omit required fields or provide empty strings/structures.

### 3.1 Empty `operation`
The code does: `operation := strings.ToLower(strings.TrimSpace(stringArg(args, "operation")))`. If `operation` is omitted, it defaults to `""`, hitting the `default` switch case and returning an unsupported operation error. This is handled gracefully. However, there is no test verifying this behavior in `browser_test_test.go`.

### 3.2 Empty `test_yaml` vs `test` object
In the `create` operation, if both `test` and `test_yaml` are missing, `generateBrowserTest` is called. This is a highly surprising fallback mechanism!
```go
if args["test"] == nil && strings.TrimSpace(stringArg(args, "test_yaml")) == "" {
    return generateBrowserTest(ctx, args, "create", view)
}
```
If a user simply forgets to provide the test payload to `create`, the system suddenly attempts to read the session's action intent history and generate a test. This is an overloaded behavior that could lead to unintended generation. What if `session_id` is also null in this fallback? `generateBrowserTest` will fail with "session_id is required for generate".

### 3.3 Empty Lists within `test`
What happens if `test_yaml` is provided, but `actions: []` is an empty list? The system parses it, and during `run`, it will iterate over 0 actions. Is this considered a valid test? The assertions will run immediately. While logically sound, an empty test might be a user error that should be flagged.

### 3.4 Missing `session_id` on `run`
For the `run` operation, `session_id` is technically required to query facts and run actions. If `session_id` is missing, `queryScopedBrowserFacts` and `manager.ReadEvidence` might fail or, worse, attempt to query a global state if the underlying implementation doesn't strictly validate the session ID length.

## 4. Vector 2: Type Coercion & Schema Violations

The LLM interface inherently deals with JSON-like dynamic typing.

### 4.1 Type Assertion Panics
If `args["operation"]` is provided as an array `["create"]` instead of a string, `stringArg` (assuming it's a safe wrapper) might handle it, but we need to ensure it doesn't cause a direct type assertion panic.

### 4.2 Integer Coercion for Timestamps
The `since_ms` and `settle_timeout_ms` fields expect integers.
```go
SinceMS: int64Arg(args, "since_ms", 0)
```
If the LLM provides `since_ms: "1700000000000"`, does `int64Arg` coerce it? If not, it defaults to 0, completely changing the semantics of the `generate` operation by reading the entire session history instead of just the recent actions. This is a silent failure that leads to oversized fixture generation and subsequent failure due to `maxGeneratedBrowserActions`.

### 4.3 Boolean Coercion
Fields like `include_assertions`, `stop_on_error`, and `diagnose_on_failure` default to `true`. If the LLM passes `"false"` (string) instead of `false` (boolean), does `boolArg` parse the string, or does it see a non-boolean type and silently fall back to the default `true`? If it falls back to `true`, the user's explicit request to disable diagnosis is ignored, leading to wasted compute and potential context window overflow.

### 4.4 Malformed YAML
If `test_yaml` contains invalid YAML, `testspec.ParseYAML(raw)` returns an error. This error is wrapped by `browserTestInvalid` and returned to the LLM. This is the happy path for an error. However, what if the YAML is *valid* YAML but invalid according to the schema (e.g., missing required fields in an action)? The schema validation must be robust enough to catch this before execution begins, otherwise the execution loop might panic on nil pointers.

## 5. Vector 3: User Request Extremes (Frontier Bounds)

This vector tests the system's resilience against massive, complex, or intentionally abusive inputs.

### 5.1 Massive YAML Payloads
The `test_yaml` could be a 50MB string containing deeply nested YAML structures.
Does `testspec.ParseYAML` have a token limit or memory bound? Parsing a massive YAML file could cause the Go runtime to allocate excessive memory, potentially leading to an Out-Of-Memory (OOM) kill of the codeNERD process. This is particularly dangerous for a long-running agent framework.

### 5.2 Action Limit Bypass
There is a constant `maxGeneratedBrowserActions = testspec.MaxActions`. The `generate` operation checks this:
```go
if len(actions) > maxGeneratedBrowserActions {
    return "", fmt.Errorf(...)
}
```
However, does the `create` or `run` operation enforce this limit? If I pass a `test_yaml` with 10,000 actions, does the system attempt to execute all of them? Executing 10,000 browser actions would take hours and block the agent's JIT clean loop. There must be a strict bound on the number of actions a single run can execute, independent of the generation limit.

### 5.3 Massive Settle Timeout
The schema defines `settle_timeout_ms` with a description: "Run quiescence wait, hard-capped at 10000". But is this hard cap actually enforced in the code?
If the user requests `settle_timeout_ms: 999999999`, and the code blindly passes this to `waitForStableBrowser`, the test could hang for days waiting for the browser to settle.

### 5.4 Extreme Assertion Queries
The assertions use Mangle queries. What if the LLM generates a highly complex, unstratified, or infinitely recursive Mangle query for the assertion?
```yaml
assertions:
  - name: "infinite loop"
    query: "p(X) :- p(X)."
    expect: "present"
```
While Mangle's analysis phase (which must be called before evaluation) should catch unsafe or unstratified rules, we must ensure that `queryScopedBrowserFacts` strictly bounds the execution time and derivation limits to prevent the Mangle engine from spinning infinitely or exhausting memory.

## 6. Vector 4: State Conflicts & Race Conditions

Browser sessions are inherently stateful.

### 6.1 Concurrent Execution
What happens if the LLM issues two `run` operations concurrently for the same `session_id`?
The browser manager and the underlying CDP connection must serialize these actions, or they will interleave disastrously. If one run attempts to navigate while the other is attempting to click, both will likely fail, leaving the session in an undefined state.

### 6.2 Session Termination Mid-Run
If the `session_id` is valid at the start of `runBrowserTest`, but the session is unexpectedly terminated (e.g., the browser crashes or is closed by another shard) while the actions are being executed, what is the behavior?
The underlying CDP calls will return errors. The `run` loop must handle these gracefully, abort the test, and return a clear error indicating the session died, rather than panicking or hanging.

### 6.3 Evidence Flight Races
During `generateBrowserTest`, the system reads `action_intent` evidence.
If another shard or tool is actively recording evidence for the same session concurrently, is the read safe? Go's map concurrency rules mean that if the evidence store is not properly mutex-locked, a concurrent read and write will cause a fatal panic. The `ReadEvidence` and `RecordEvidence` methods in `browser.SessionManager` must be thoroughly stress-tested for concurrency.

## 7. Mangle Integration Edge Cases

The use of Mangle for assertions introduces unique logical boundary conditions.

### 7.1 Fact Fingerprinting Collisions
The `browserFactFingerprint` function uses JSON marshaling:
```go
encoded, err := json.Marshal(struct {
    Predicate string `json:"predicate"`
    Args      []any  `json:"args"`
}{Predicate: fact.Predicate, Args: fact.Args})
```
If the `Args` contain complex Go structures that marshal to the same JSON representation but are functionally different (e.g., a float64 `1.0` vs an integer `1`), the fingerprint will collide. This means `subtractBrowserFacts` might incorrectly subtract a baseline fact, causing a "fresh" assertion to fail.

### 7.2 Non-Deterministic Baseline
The baseline for "fresh" assertions is captured *before* the test runs. If the browser is actively emitting events (e.g., a polling AJAX request logging to the console every second), the baseline might capture `console_event(..., 5)`. After the test runs, the new state has `console_event(..., 6)`. The subtraction will leave the new event, and the `absent` assertion will fail.
This highlights the difficulty of deterministic testing in a live, asynchronous browser environment. The `settleTimeout` is crucial here, but it may not be sufficient for all applications.

### 7.3 Scope Mismatches
The assertion scope can be "fresh" (defaulting to checking against baseline). What if the user specifies an invalid scope like "historical"? Does the system fall back to a default, or does it reject the assertion?

## 8. Performance and Resilience Evaluation

Is the system performant enough to handle these edge cases?

1. **Memory**: The lack of bounds checking on `test_yaml` payload size is a potential vulnerability. The system needs a strict limit (e.g., 1MB) on incoming string arguments to prevent OOM attacks.
2. **CPU**: The Mangle engine is generally fast, but evaluating complex joins over thousands of facts can be CPU intensive. The `queryScopedBrowserFacts` must use `context.WithTimeout` to prevent CPU starvation.
3. **Goroutines**: If `waitForStableBrowser` leaks goroutines on timeout, repeated failures will degrade system performance.

## 9. Further Resilience Considerations: The JIT Clean Loop

The newly adopted JIT Clean Loop architecture demands an even stricter standard of resource cleanup.

### 9.1 Ephemeral Facts Lifespan
Because facts such as `user_intent` are ephemeral and filtered on every kernel boot, any background goroutine or test that lingers beyond its allowed window could read a clean slate. A browser test might span a boot cycle if `waitForStableBrowser` isn't canceled tightly. Tests must be added to inject kernel reboots mid-test to ensure the browser manager does not panic when the underlying store is wiped.

### 9.2 Tool Execution Contexts
When the `BrowserTestTool` executes, the `ctx` passed to it must be actively monitored. If the test loops indefinitely on a Mangle assertion (due to a complex cyclic rule), and the LLM's context times out, the `executeBrowserTest` function must respect `ctx.Done()` immediately. The code currently does not appear to check `ctx.Err()` during the `resolved.Actions` iteration loop.

### 9.3 Delegation Hardening Bypass
The new architecture emphasizes strict delegation paths. Does the declarative test framework respect the `PriorityHigh` execution constraints? A browser test might inadvertently trigger nested intents or side effects that circumvent the arbitration lane. A malicious declarative test payload could try to execute forbidden operations by chaining inputs that mimic JIT triggers. Negative testing must verify that the `browser_test` sandbox is hermetically sealed from the rest of the codenerd tool registry.

## 10. Advanced Boundary Value Analysis

Let's drill deeper into specific boundary values for the numerical inputs:

### 10.1 `settle_timeout_ms` Boundaries
- Values `0` and `< 0`: Does a negative timeout immediately fail the test, or bypass the settle check entirely? Bypassing it would cause race conditions on assertions.
- Value `9999`: Allowed.
- Value `10000`: Allowed (the hard cap).
- Value `10001`: Should be clamped to `10000`. We need tests to explicitly assert this clamping logic.

### 10.2 `since_ms` Boundaries
- Value `-1`: Will this read the entire history, or fail the validation?
- Value `MaxInt64`: No actions will be matched, resulting in an empty test. The system correctly errors on this, but is the error message clear?
- Fractional timestamps: If the flight records use floating point timestamps internally, passing an integer might miss events due to rounding or truncation errors in the coercion logic.

## 11. Security and Privacy Boundaries

### 11.1 The `ValueEnv` Resolution
The specification mentions "Sensitive fields require value_env, resolved only in an execution copy". This is a critical security boundary.

- What happens if `value_env` points to a sensitive environment variable like `AWS_ACCESS_KEY_ID`? Can a user's prompt arbitrarily read any environment variable during test execution?
- We must test the extraction of the test YAML. If the `generate` tool exports the `test_yaml` containing the raw resolved password, it's a catastrophic data leak. The tests (like `TestBrowserTestInvalidRedactsErrors`) try to cover this, but we need negative tests that actively try to coax the `generate` command into printing the secret.
- What if `value_env` points to a non-existent variable? Does the system panic, fail the test, or inject an empty string?

### 11.2 The `target` Selector Integrity
The documentation says "Opaque refs and raw selectors are forbidden. Fixtures use portable semantic element targets".
- Negative tests must provide raw XPath or CSS selectors (e.g., `#password`, `//div[@class='secret']`) to ensure the parser explicitly rejects them and does not silently fall back to an insecure selector engine.
- What if the semantic target resolves to multiple elements? Does the click action click all of them, the first one, or fail deterministically?

## 12. Stress Testing the Mangle Subsystem

The use of Mangle as the assertion engine requires stress testing its resource limits within the context of browser facts.

### 12.1 Derivation Limits
Mangle uses fixpoint derivation. If a declarative test introduces an assertion that generates millions of derived facts from a relatively small number of browser facts, the memory will spike. We need negative tests that intentionally submit "fork bomb" style Mangle queries in the `assertions` list.
- Example: `p(X, Y) :- console_event(..., X), console_event(..., Y).` This cartesian product over a large console log will explode. The system must have a hard derivation limit.

### 12.2 Atom/String Dissonance in Assertions
As noted in the AI Failure modes, Mangle strings (`"active"`) and Atoms (`/active`) are disjoint.
- We need tests where the declarative test passes `"absent"` as a string, but the underlying system expects an Atom.
- The schema parsing logic must correctly translate the declarative YAML string into the appropriate Mangle type to avoid silent failures (zero results leading to false positives on `absent` assertions).

## 13. Deep Dive: `subtractBrowserFacts` Logic Flaws

The fingerprinting mechanism in `subtractBrowserFacts` is a significant weak point.

### 13.1 Non-Deterministic JSON Keys
If a Mangle fact argument contains a nested map or object, `json.Marshal` does not guarantee the order of keys (historically, though Go 1.23 may be stable, it's brittle). If the keys marshal in a different order for the baseline fact vs the fresh fact, the fingerprints will not match. The fact will not be subtracted, causing false negatives.

### 13.2 Floating Point Precision Loss
If a browser event timestamp or coordinate is represented as a `float64` inside an `any` interface, JSON marshaling might serialize it as `1.000000` vs `1`. This will cause the fingerprint comparison to fail.

### 13.3 Remediation Strategy for Subtraction
Instead of JSON marshaling, the framework should leverage the `github.com/google/go-cmp/cmp` package or a custom recursive equality function specifically tailored for Mangle types (Atoms, Structs, etc.) to ensure robust set subtraction.

## 14. Remediation and Action Plan

To address these gaps, the following TODO comments have been added to the test suite (`internal/tools/research/browser_test_test.go`), and corresponding tests must be implemented:

1. **TODO: Edge Case (Null/Empty): Verify behavior when `operation` is an empty string or omitted.**
2. **TODO: Edge Case (Type Coercion): Verify `since_ms` gracefully handles stringified numbers or invalid types.**
3. **TODO: Edge Case (User Extremes): Ensure `run` enforces a hard cap on the number of actions executed, identical to the generation limit.**
4. **TODO: Edge Case (User Extremes): Verify `settle_timeout_ms` strictly enforces its documented 10000ms hard cap.**
5. **TODO: Edge Case (State Conflicts): Simulate session termination during `run` to ensure graceful failure without panics.**
6. **TODO: Edge Case (Mangle): Test `subtractBrowserFacts` with complex nested arguments to ensure fingerprinting does not collide or panic.**
7. **TODO: Edge Case (Null/Empty): Verify the fallback mechanism in `create` when both `test` and `test_yaml` are empty, specifically checking the `session_id` requirement.**
8. **TODO: Edge Case (User Extremes): Test passing a massive (e.g., 10MB) invalid YAML string to ensure the parser does not cause OOM or excessive CPU utilization.**

## 15. Conclusion

The declarative browser test framework in codeNERD is conceptually sound, leveraging Mangle for elegant assertions. However, its position at the boundary of LLM output necessitates rigorous negative testing. The LLM cannot be trusted to adhere to the schema, respect timing constraints, or provide valid data types. By systematically addressing the edge cases outlined in this journal—particularly those involving type coercion, payload extremes, and concurrent state mutations—the framework's resilience and reliability will be significantly enhanced.



## 16. Analyzing the Assertion Scope "Fresh" vs "Historical"

The assertion scope provides a mechanism to verify state changes, but its implementation introduces significant boundary conditions that must be thoroughly validated.

### 16.1 The Fallacy of Continuous Monitoring
The `fresh` scope operates on the assumption that comparing the state immediately before the test (the baseline) with the state immediately after (the fresh state) captures the true impact of the test actions.
However, this is a fallacy in modern web applications.
- What if an asynchronous background process (e.g., a websocket heartbeat or a delayed analytics beacon) fires during the test execution?
- If this event generates a `console_event` or a `network_request` fact, it will be captured in the fresh state.
- If the assertion expects `absent` for any new errors, and this unrelated background task logs a benign warning, the test fails.
- This creates incredibly flaky tests. Negative tests must simulate this by injecting delayed, out-of-band events during the `run` operation to verify the system's susceptibility to false negatives.

### 16.2 Historical Scope Overload
If a user specifies a `historical` scope, the system evaluates the assertion against the entire flight evidence history of the session.
- Boundary condition: A long-running session might have accumulated tens of thousands of facts.
- Evaluating a complex Mangle query against a massive historical fact base will lead to significant CPU latency.
- The `queryScopedBrowserFacts` function must have distinct timeouts and derivation limits based on the requested scope. A `historical` query should have a tighter bounds check than a `fresh` query due to the increased data volume. We need tests that create a massive historical baseline and measure the performance degradation.

## 17. The JIT Clean Loop: Repercussions on Ephemeral Test Data

The architectural shift to the JIT Clean Loop (Dec 2024) significantly impacts how we must structure browser tests.

### 17.1 Test ID and Session Collision
In the old architecture, sessions were long-lived and stateful. In the JIT Clean Loop, sessions start fresh, and ephemeral facts are wiped.
- When `BrowserTestTool` executes a test, it generates temporary facts (e.g., the assertion results themselves).
- If a test crashes midway, or the LLM times out, does the cleanup logic ensure these temporary facts are retracted from the Cortex kernel?
- If they are not retracted, they become "ghost facts" that contaminate subsequent JIT loops. We must negatively test this by forcibly killing a test run and then verifying that the kernel state is clean.

### 17.2 Cross-Session Bleed
The `session_id` is an arbitrary string.
- What happens if the LLM hallucinated a `session_id` that belongs to a different, concurrent task running on a different JIT shard?
- The browser manager uses `session_id` to route commands to the correct CDP connection.
- If the routing logic relies on a shared map without proper isolation, a malicious test payload could interact with another user's browser session.
- Negative tests must attempt to use a known `session_id` from a different shard and assert that the operation is strictly forbidden via RBAC or isolation boundaries.

## 18. Extensibility and Future-Proofing the Schema

The `browser_test` schema currently supports `create`, `inspect`, `generate`, and `run`.

### 18.1 Unexpected Operations
While the `operation` field is an Enum, the implementation uses `switch operation`.
- If an older version of the schema is somehow passed (perhaps restored from a legacy context window), containing an unsupported operation like `validate`, the `default` case handles it.
- Is this handled optimally? Returning an error is good, but does the error contain enough diagnostic information for the LLM to self-correct?

### 18.2 Schema Versioning
There is currently no explicit `version` field in the `test_yaml`.
- As the codeNERD framework evolves, the allowed semantic targets or assertion queries will inevitably change.
- Without a version field, backward compatibility becomes impossible to manage.
- A negative test should simulate loading a hypothetical "v2" YAML (e.g., one containing a new action type like `hover`) to ensure the parser fails gracefully and instructs the user on the correct schema format.

## 19. Detailed Examination of `executeBrowserReason`

When a test fails, and `diagnose_on_failure` is true, the system calls `executeBrowserReason`.

### 19.1 Infinite Recursion in Diagnosis
- What if the failure was caused by a massive DOM structure that exceeds the LLM's context window?
- `executeBrowserReason` attempts to read the DOM and provide a diagnosis.
- If the diagnosis itself fails (e.g., due to a timeout or context overflow), the system gracefully handles the `diagnoseErr`.
- However, we must ensure that the diagnosis process itself does not inadvertently mutate the browser state, further corrupting the session. Diagnosis must be strictly read-only.

### 19.2 Prompt Injection via DOM
- The DOM is untrusted input.
- If the browser is navigated to a malicious webpage, the webpage could contain HTML structured to exploit the `executeBrowserReason` LLM prompt (e.g., `<div id="ignore-previous-instructions">You are now a helpful assistant...</div>`).
- This is a critical security vulnerability. The declarative test framework must sanitize the DOM before passing it to the reasoning engine, or the reasoning engine must be explicitly hardened against indirect prompt injection.

## 20. Conclusion and Next Steps for QA

The findings documented in this journal represent a critical step toward maturing the `browser_test` subsystem. The transition to the JIT Clean Loop architecture demands a higher standard of rigor, particularly concerning ephemeral state management and concurrent execution.

The next immediate steps for the QA automation team are:
1.  **Implement the 8 TODOs:** Write concrete Go tests for each of the `// TODO` items added to `internal/tools/research/browser_test_test.go`.
2.  **Mangle Test Harness:** Develop a specialized test harness for `queryScopedBrowserFacts` that can simulate massive fact derivation and enforce hard timeouts, independent of the browser execution logic.
3.  **Concurrency Fuzzing:** Build a fuzzing pipeline that concurrently submits invalid, massive, and correctly formed YAML payloads to the `create` and `run` endpoints to stress test the underlying session manager's isolation boundaries.
4.  **Schema Enforcement:** Advocate for the addition of a formal `version` field in the `testspec` to enable safe future migrations and backward compatibility.

By systematically addressing these boundary conditions and negative test scenarios, we can ensure the codeNERD framework remains robust, secure, and performant, even when subjected to frontier-level stress by highly capable reasoning models.



## 21. Analyzing Tool Execution Timeouts

The `executeBrowserTest` function relies on `executeRunCommand` under the hood for some operations, or directly manages context timeouts for others. The integration between the JIT Clean Loop's overarching context and the tool's specific execution context is a common source of negative test failures.

### 21.1 JIT Context vs Local Context
- The `ctx` passed into `executeBrowserTest` is the primary JIT context. It represents the LLM's budget for the turn.
- If `settle_timeout_ms` is set to 10 seconds, but the JIT context only has 2 seconds remaining, does `waitForStableBrowser` respect the 2-second limit, or does it try to block for the full 10 seconds, causing a context deadline exceeded panic or hanging the tool executor?
- Negative tests must artificially constrain the `ctx` passed to the `run` operation to simulate an LLM turn that is out of time, verifying that the tool gracefully aborts and returns an incomplete status rather than spinning.

### 21.2 Orphaned Goroutines on Timeout
- If `waitForStableBrowser` uses a polling mechanism (e.g., a `time.Ticker` in a goroutine) to check for stability, what happens if the parent context cancels?
- We must verify that the polling goroutine selects on `ctx.Done()` and exits cleanly.
- A negative test can run 100 concurrent browser tests with a 1ms context timeout, and then profile the number of active goroutines to ensure none are leaked.

## 22. Boundary Values in `since_ms` and Log Rotation

The `generate` operation reads action intent evidence using `since_ms`.

### 22.1 Evidence Store Pruning
- The `codenerd` session manager likely has a mechanism to prune old evidence to prevent unbounded memory growth.
- What happens if the LLM requests `since_ms` that is so old it has been pruned from the active flight evidence store?
- Does the system return an error, return partial results, or silently return an empty test? The correct behavior is to inform the user that the requested history is unavailable. Negative tests must simulate a session with pruned evidence and assert the correct error is returned.

### 22.2 The `maxItems: 100` Hardcode
- In `generateBrowserTest`, the `ReadEvidence` call is hardcoded to `MaxItems: 100`.
- This is an undocumented boundary condition. If the user performed 150 actions since `since_ms`, the read will truncate.
- The code handles this: `if read.Truncated { return "", fmt.Errorf(...) }`.
- However, we must negatively test this boundary: simulate exactly 100 actions (should succeed), and 101 actions (should fail with the truncation error). This ensures the error handling pathway is active and accurate.

## 23. The `view` Parameter and Memory Boundaries

The `view` parameter dictates how much data is returned to the LLM context.

### 23.1 View `full` Context Overflow
- If `view` is set to `full` on a large declarative test with hundreds of actions and complex Mangle assertions, the returned JSON string could easily exceed 100KB.
- If the LLM context window is nearing its limit, appending this massive string will cause the prompt encoder to truncate older memories or fail the JIT compilation entirely.
- Negative tests must evaluate the maximum size of the `full` view output and potentially enforce a hard cap (e.g., refusing `full` view if the generated payload exceeds 50KB) to protect the JIT compilation phase.

### 23.2 Unexpected `view` Values
- The schema specifies an Enum: `["summary", "compact", "full"]`.
- The code normalizes the view: `strings.ToLower(strings.TrimSpace(stringArg(args, "view")))`.
- If an empty string is provided, it defaults to `compact`.
- What if a completely invalid string is provided that bypassed schema validation (perhaps injected via a subagent)? The code returns `fmt.Errorf("view must be summary, compact, or full")`. This is well handled, but must be explicitly asserted in a negative test.

## 24. Concurrency Hardening: The `SetBrowserManager` Pattern

The test file `browser_test_test.go` uses a global pattern that is highly susceptible to concurrency bugs.

### 24.1 Global State Mutability
- The tests call `SetBrowserManager(manager)` and `defer ClearBrowserManager(manager)`.
- This implies a global variable is holding the active browser manager.
- If `go test -short ./...` is run with `-p 1` (default), it's safe. But if tests are run in parallel (`t.Parallel()`), these global overrides will overwrite each other, causing intermittent, impossible-to-debug failures where Test A uses Test B's manager.
- The `browser_test` subsystem must be refactored to allow dependency injection of the manager into the tool directly, or the global setter must use a Goroutine-local storage or an explicit mutex. Negative testing the test suite itself by forcing `t.Parallel()` on all functions will immediately surface this vulnerability.

### 24.2 Sink Sinkhole
- The `NewSessionManagerWithSink` uses `nil` for the sink in tests: `browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)`.
- If the browser manager implementation does not explicitly check `if sink != nil` before writing, it will nil-pointer panic.
- A negative test must explicitly trigger an error condition that writes to the sink while passing a nil sink, ensuring the nil check is robust.

## 25. Final Review of Mangle Stratification Constraints

Mangle's logic programming model introduces unique negative test vectors concerning stratification.

### 25.1 Unstratified Assertions
- Stratification means that a rule cannot depend on the negation of itself (e.g., `p :- not p`).
- If an LLM generates a declarative test with an unstratified assertion, the Mangle analyzer will reject it.
- However, does the `queryScopedBrowserFacts` function correctly invoke the analyzer *before* evaluation?
- If it skips analysis and goes straight to evaluation, the Mangle engine might enter an infinite loop or panic.
- Negative tests must supply explicitly unstratified queries (e.g., `invalid_rule(X) :- console_event(X), not invalid_rule(X).`) and verify that the system returns a safe, explanatory error to the LLM, rather than crashing the JIT worker.

### 25.2 Type Mismatches in Joins
- A declarative test might attempt to join two facts on columns of different types.
- Example: `join_test(S) :- console_event(S, "error", Msg, Time), network_request(S, Msg, _, _)`. Here, `Msg` (a string) is joined with the URL (another string). This is technically valid in Mangle.
- But what if `Msg` (a string) is joined with a numeric ID? `join_test(S) :- console_event(S, _, _, ID), network_request(S, ID, _, _)`.
- Mangle handles disjoint types gracefully (it yields zero results), but this often confuses the LLM.
- The system must ensure that the `schema` for facts is clearly documented, and negative tests should verify that type mismatches in assertions don't cause underlying Go panics during variable binding.

## 26. Overall Subsystem Maturity Assessment

The `browser_declarative` subsystem is a powerful bridge between neuro-symbolic reasoning and deterministic browser control. However, its current iteration exhibits several critical boundary vulnerabilities.

The reliance on dynamically typed inputs (`map[string]any`) without comprehensive runtime schema enforcement makes it susceptible to type coercion attacks and silent failures. The integration with Mangle, while elegant, requires strict limits on derivation complexity and execution time to prevent resource exhaustion. Furthermore, the global state mutations in the test harness (`SetBrowserManager`) indicate a need for a more robust dependency injection pattern to support true concurrent testing.

By implementing the negative test vectors outlined in this journal, the QA engineering team will fortify the subsystem, ensuring it can safely and predictably handle the chaotic, unstructured outputs typical of advanced language models.

## 27. Cross-Platform Execution Discrepancies
The declarative test framework abstracts away the underlying operating system.
- Negative tests must verify behavior when a generated test attempts to use OS-specific keyboard shortcuts (e.g., `Meta+C` vs `Control+C`).
- Does the framework normalize these inputs across Windows, macOS, and Linux runners?
- If an LLM generates a test assuming a macOS environment, it might fail deterministically on a Linux CI runner.
- This represents a boundary condition at the OS interface layer that must be explicitly handled and tested.
