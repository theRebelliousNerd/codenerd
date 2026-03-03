# QA Journal: JIT Prompt Compiler - Negative Testing & Boundary Analysis
# Date: 2026-02-23 00:07 EST
# Author: QA Automation Engineer (Jules)
# Subsystem: internal/prompt (JITPromptCompiler & AtomSelector)
# Review Focus: Negative Testing, Boundary Value Analysis, Concurrency, State Conflicts

## 1. Executive Summary

This journal entry documents a comprehensive deep dive into the `internal/prompt` subsystem, specifically focusing on the `JITPromptCompiler` and its auxiliary components (`AtomSelector`, `DependencyResolver`, `TokenBudgetManager`). The primary objective of this analysis is to identify critical gaps in negative testing and boundary value handling. By simulating extreme conditions, malformed inputs, and concurrent state mutations, we aim to ensure the system remains robust and reliable in production environments.

The `JITPromptCompiler` is a cornerstone of the CodeNerd architecture, responsible for dynamically assembling system prompts from atomic fragments ("atoms"). It operates at the intersection of deterministic logic (Mangle) and probabilistic retrieval (Vector Search), making it a complex system with numerous failure modes. A failure in this subsystem directly degrades the AI's ability to understand context, follow protocol, and execute tasks safely.

While the existing test suite (`compiler_test.go`) provides solid coverage for "happy path" scenarios and some basic error conditions (e.g., nil context), it lacks rigorous testing for advanced failure vectors. Specifically, we have identified significant gaps in:
-   **Type Coercion**: Handling of non-string types returned by the kernel.
-   **Concurrency**: Safety during hot-swapping of databases via `RegisterDB`.
-   **Extremes**: Handling of massive inputs or token budgets.
-   **State Conflicts**: Ensuring data consistency when external resources (DBs) are modified during compilation.

This document details these findings, provides theoretical background, and outlines a concrete plan for improvement.

## 2. Theoretical Framework: Negative Testing & Boundary Value Analysis

Before diving into the specific findings, it is essential to establish the theoretical framework guiding this analysis.

### 2.1 Negative Testing
Negative testing ensures that the application can gracefully handle invalid input or unexpected user behavior. It is not about showing that the software works (Positive Testing), but rather showing that it doesn't fail in an uncontrolled manner when subjected to stress.
For the JIT Compiler, this means:
-   **Resilience**: The compiler should never panic, even if the database is corrupt or the kernel returns garbage.
-   **Degradation**: If a non-critical component (like Vector Search) fails, the system should fall back to a safe baseline (Skeleton Atoms) rather than aborting the request.
-   **Sanitization**: All inputs from the `CompilationContext` must be treated as untrusted until validated.
-   **Observability**: Failures in negative scenarios must be logged with sufficient context (context hash, shard ID) to allow for rapid diagnosis. Silent failures are often worse than crashes.

### 2.2 Boundary Value Analysis (BVA)
BVA focuses on values at the extreme ends of valid ranges. Errors most often occur at these boundaries.
For the JIT Compiler, key boundaries include:
-   **Token Budget**:
    -   *Minimum*: 0 or 1 token (should produce minimal/empty prompt).
    -   *Maximum*: `MaxInt64` (should not overflow).
    -   *Invalid*: Negative values (should be clamped to 0 or treated as error).
-   **String Length**:
    -   *Empty*: Empty shard IDs or Intent Verbs.
    -   *Massive*: 1MB+ strings (DoS vector).
    -   *Special*: Strings with only whitespace, or Mangle control characters.
-   **Collection Size**:
    -   *Empty*: 0 atoms in corpus.
    -   *Single*: 1 atom.
    -   *Massive*: 100,000+ atoms (performance boundary).
-   **Recursion Depth**:
    -   *Zero*: No dependencies.
    -   *Deep*: 100 levels of dependencies (stack overflow risk).
    -   *Cyclic*: A->B->A (infinite recursion risk).

### 2.3 Concurrency & State Mutation
Testing for race conditions involves identifying shared mutable state and ensuring atomic access.
-   **Shared State**: `projectDB`, `shardDBs`, `cache`.
-   **Mutators**: `RegisterDB`, `Compile` (cache writes).
-   **Readers**: `Compile` (DB reads, cache reads).
-   **Hazards**: "Check-then-Act" races, stale reads, resource exhaustion (connection pools).

## 3. Subsystem Deep Dive: Code Analysis

### 3.1 Component: `JITPromptCompiler` (compiler.go)

The `JITPromptCompiler` is the orchestrator. It manages the lifecycle of a prompt compilation request.

#### 3.1.1 `collectAtomsWithStats`
This method gathers atoms from multiple sources: Embedded Corpus, Project DB, Shard DBs, and Evolved Atoms.
**Code Analysis**:
```go
func (c *JITPromptCompiler) collectAtomsWithStats(...) {
    // ...
    if c.projectDB != nil {
        projectAtoms, err := c.loadAtomsFromDB(ctx, c.projectDB)
        // ...
    }
    // ...
}
```
**Risk**: The `loadAtomsFromDB` call performs a SQL query. If `c.projectDB` is closed concurrently by `RegisterDB`, this call will fail. The error handling here logs a warning but continues. This is good for resilience, but the concurrency race itself is unhandled.
**Performance Risk**: This method loads *all* atoms into memory before filtering. If the DB contains 100k atoms, this will cause a massive memory spike and GC pressure. A streaming approach or pushing filtering down to the SQL layer would be better.

#### 3.1.2 `collectKernelInjectedAtoms`
This method queries the kernel for `injectable_context` and `specialist_knowledge` facts.
**Code Analysis**:
```go
matchesShard := func(raw string) bool {
    raw = strings.TrimSpace(raw)
    if raw == "" { return false }
    // ...
    rawTrim := strings.TrimPrefix(raw, "/")
    if cc.ShardInstanceID != "" && rawTrim == cc.ShardInstanceID { return true }
    return rawTrim == cc.ShardID
}
```
**Risk**: As noted in the summary, if `cc.ShardID` is empty string `""`, and `raw` is just `"/"`, `rawTrim` becomes `""`. The comparison `"" == ""` evaluates to true. This allows an empty context to match potentially privileged or broad atoms.

### 3.2 Component: `AtomSelector` (selector.go)

The `AtomSelector` implements the "Skeleton/Flesh" bifurcation logic.

#### 3.2.1 `extractStringArg`
This helper converts interface values from the Kernel into strings.
**Code Analysis**:
```go
func extractStringArg(arg interface{}) string {
    switch v := arg.(type) {
    case string: return v
    case fmt.Stringer: return v.String()
    default: return fmt.Sprintf("%v", v)
    }
}
```
**Risk**: This functions "swallows" type errors.
-   If the Kernel returns `nil` (e.g., due to a logic bug in Mangle), `extractStringArg` returns `"<nil>"`.
-   If this result is used as an Atom ID, the system proceeds to look up an atom with ID `"<nil>"`, fails to find it, and silently drops it.
-   While "silently dropping" is better than panicking, it makes debugging extremely difficult. "Why is my atom missing?" -> "Because the ID was nil, but the logs say we looked for <nil>".
-   If the Kernel returns a complex type like `[]byte` (which might happen if Mangle interacts with binary data), `fmt.Sprintf("%v")` creates a string like `"[23 45 12]"`. This is definitely not a valid ID.

#### 3.2.2 `mangleQuoteString`
This helper escapes strings for use in Mangle facts.
**Code Analysis**:
It handles quotes, backslashes, newlines, and hex escapes.
**Risk**: It assumes the input string is UTF-8 or ASCII. If the input contains invalid UTF-8 sequences, `range` loop might behave unexpectedly (decoding error runes).
**Security Risk**: Mangle injection. If a malicious user can inject a string that breaks out of the quoted string context, they could inject arbitrary facts. The current implementation looks robust for standard escapes, but should be fuzz-tested against Mangle's lexer quirks.

## 4. Boundary Value Analysis (BVA) & Negative Testing Vectors

### 4.1 Vector A: Null, Undefined, and Empty Inputs

**Scenario A1: Empty `CompilationContext` Fields**
-   **Code Path**: `matchesShard` in `collectKernelInjectedAtoms` uses `cc.ShardID`.
-   **Vulnerability**: If `cc.ShardID` is empty string `""`, and `raw` is just `"/"` (which trims to `""`), `rawTrim == cc.ShardID` becomes `"" == ""`, returning `true`.
-   **Impact**: Unintended atoms injected into the prompt.
-   **Reproduction**:
    1.  Create `CompilationContext` with `ShardID: ""`.
    2.  Assert a fact `injectable_context("/", "secret_atom")` into the kernel.
    3.  Run `collectKernelInjectedAtoms`.
    4.  Verify if "secret_atom" is collected.

**Scenario A2: Nil or Empty Slices in `CompilationContext`**
-   **Code Path**: `buildContextFacts` iterates over `cc.Frameworks`, `cc.WorldStates()`.
-   **Analysis**: Safe. Go handles range over nil/empty slices correctly (0 iterations).
-   **Risk**: Low.

**Scenario A3: Empty Strings in `RegisterDB`**
-   **Code Path**: `RegisterDB(name, dbPath)`
-   **Vulnerability**: `sql.Open` might succeed with an empty path.
-   **Impact**: Silent failure or creation of useless DB connections.
-   **Recommendation**: Validate `dbPath` exists and is a file before calling `sql.Open`.

### 4.2 Vector B: Type Coercion & Data Integrity

**Scenario B1: Kernel Returns Non-String Types**
-   **Code Path**: `extractStringArg` in `internal/prompt/selector.go`.
-   **Vulnerability**:
    -   `nil` -> `"<nil>"`
    -   `[]byte` -> `"[...]"`
    -   `int` -> `"123"`
-   **Impact**: Logic errors, "Ghost Facts", difficult debugging.
-   **Reproduction**:
    1.  Mock the Kernel to return `Fact{Args: []interface{}{nil, 123}}`.
    2.  Run `SelectAtoms`.
    3.  Observe that `extractStringArg` returns `"<nil>"` and `"123"`.
    4.  Verify that `atomMap["<nil>"]` lookup fails silently.

**Scenario B2: Malformed JSON in `agents.json`**
-   **Code Path**: `InjectAvailableSpecialists` in `compiler.go`.
-   **Vulnerability**: JSON Unmarshal failure on type mismatch.
-   **Impact**: Missing specialists in context.
-   **Reproduction**:
    1.  Create `agents.json` with `{"agents": [{"name": 123}]}`.
    2.  Run `InjectAvailableSpecialists`.
    3.  Verify it handles the error gracefully but logs a warning.

### 4.3 Vector C: User Request Extremes (DoS & Performance)

**Scenario C1: Massive `CompilationContext` Inputs**
-   **Code Path**: `collectKnowledgeAtoms` in `compiler.go`.
-   **Logic**: `sb.WriteString(cc.IntentVerb) ...` repeated.
-   **Vulnerability**: Massive string construction.
-   **Impact**: OOM, Embedding API errors.
-   **Reproduction**:
    1.  Create `CompilationContext` with `IntentVerb` = 10MB string.
    2.  Run `collectKnowledgeAtoms`.
    3.  Measure memory usage and check for panic/error.

**Scenario C2: Massive Token Budget**
-   **Code Path**: `WithTokenBudget`.
-   **Vulnerability**: Integer overflow or precision loss in float conversion.
-   **Impact**: Potential infinite loop or crash if logic depends on budget decreasing.
-   **Reproduction**:
    1.  Set Budget to `math.MaxInt64`.
    2.  Run Compile.
    3.  Verify termination.

**Scenario C3: Negative Token Budget**
-   **Code Path**: `CompilationContext.AvailableTokens()`.
-   **Vulnerability**: `budget - reserved` becomes negative.
-   **Impact**: `Fit` method behavior with negative capacity.
-   **Reproduction**:
    1.  Set Budget = 100, Reserved = 200.
    2.  Verify `AvailableTokens` is -100.
    3.  Verify `Fit` handles it (should return 0 atoms or error).

### 4.4 Vector D: State Conflicts & Concurrency

**Scenario D1: Hot-Swapping Databases (`RegisterDB`) during Compilation**
-   **Code Path**: `RegisterDB` closes the old `projectDB`.
-   **Vulnerability**: Race condition between `Close()` and `QueryContext()`.
-   **Impact**: `sql: database is closed` error.
-   **Reproduction**:
    1.  Start a goroutine that runs `Compile` in a loop.
    2.  Start a goroutine that runs `RegisterDB` repeatedly.
    3.  Observe errors.
-   **Mitigation**: Use a `RWMutex` where `Compile` holds RLock for the duration of DB usage (not just acquisition), or implement a graceful swap.

**Scenario D2: Cache Invalidation Race**
-   **Code Path**: `RegisterDB` calls `clearPromptCache`.
-   **Vulnerability**: "Stale Write" to cache.
    1.  T1: Compile(Miss) -> Starts work.
    2.  T2: RegisterDB -> Clears cache.
    3.  T1: Finishes -> Writes result to cache.
-   **Result**: Cache now contains a result derived from the OLD database, effectively undoing the cache clear.
-   **Impact**: Users see old prompts for the TTL duration (5 minutes).
-   **Reproduction**:
    1.  Instrument `Compile` to sleep before writing to cache.
    2.  Trigger `Compile`.
    3.  Trigger `RegisterDB` during the sleep.
    4.  Verify cache content after T1 finishes.

## 5. Detailed Test Plan

To address these gaps, I propose adding the following tests to `compiler_test.go`.

### 5.1 `TestCompiler_ExtractStringArg_Robustness`
This test will verify that `extractStringArg` handles various types without panicking and produces predictable output (or we change the implementation to return error).

```go
func TestCompiler_ExtractStringArg_Robustness(t *testing.T) {
    // Inputs: nil, 123, []byte("test"), string("valid")
    // Expected: Safe string conversion or error handling.
    // Verify: Does not panic.
    // This is a direct unit test of the unexported helper.
    // We can expose it for testing or test indirectly via mocked Kernel queries.
}
```

### 5.2 `TestCompiler_ConcurrentRegisterDB`
This test simulates the "hot swap" scenario to identify reliability issues.

```go
func TestCompiler_ConcurrentRegisterDB(t *testing.T) {
    // 1. Setup compiler with a mock DB or real SQLite file.
    // 2. Start a long-running Compile (mock slow DB or Kernel).
    // 3. Call RegisterDB in another goroutine.
    // 4. Verify Compile does not crash with "database closed" or other panic.
    // Note: This might require `testing.Short()` check to skip in fast runs.
}
```

### 5.3 `TestCompiler_CacheInvalidationRace`
This test verifies the cache coherence protocol.

```go
func TestCompiler_CacheInvalidationRace(t *testing.T) {
    // 1. Compile (Miss) -> Start processing.
    // 2. Update DB (RegisterDB) -> Clears Cache.
    // 3. Compile finishes -> Writes to Cache.
    // 4. Verify: The entry written in step 3 should NOT persist or should be invalid.
    // Note: This requires white-box testing or hooks into the compiler to control timing.
}
```

### 5.4 `TestCompiler_InputSanitization`
This test verifies that the compiler protects itself from malicious or accidental massive inputs.

```go
func TestCompiler_InputSanitization(t *testing.T) {
    // 1. Create context with 1MB strings.
    // 2. Compile.
    // 3. Verify it finishes within reasonable time and doesn't OOM.
}
```

## 6. Recommendations for Improvement

Based on the analysis, I recommend the following code changes:

1.  **Concurrency Safety**:
    -   Modify `RegisterDB` to *not* close the old DB immediately. Instead, use a reference counting mechanism or a "grace period" before closing.
    -   Alternatively, enforce a `RWMutex` around the *entire* compilation process (heavy handed) or just the DB access phase.
    -   Fix the Cache Race by adding a `generation` counter. `RegisterDB` increments it. `Compile` captures it at start. `Compile` only writes to cache if captured generation == current generation.

2.  **Type Safety**:
    -   Update `extractStringArg` to return `(string, error)`.
    -   If the type is not `string` or `fmt.Stringer`, return an error.
    -   Log these errors as warnings in the caller.

3.  **Input Validation**:
    -   Add a `Validate()` method to `CompilationContext` that truncates strings to reasonable limits (e.g., 1KB for intent, 256B for shard ID).
    -   Sanitize `ShardID` to prevent directory traversal or Mangle injection (though `mangleQuoteString` handles injection).

4.  **Performance**:
    -   Refactor `collectAtomsWithStats` to filter atoms at the SQL query level (WHERE clause) instead of loading all and filtering in memory. This is a significant optimization for large project DBs.

## 7. Detailed Gap Analysis Table

| ID  | Vector             | Description                                          | Risk Level | Remediation Effort | Status      |
| :-- | :----------------- | :--------------------------------------------------- | :--------- | :----------------- | :---------- |
| A1  | Null/Empty         | Empty ShardID context matching wildcard atoms.       | Medium     | Low                | Identified  |
| A3  | Empty Input        | `RegisterDB` accepting invalid paths.                | Low        | Low                | Identified  |
| B1  | Type Coercion      | `extractStringArg` unsafe type handling.             | High       | Medium             | Identified  |
| B2  | Type Coercion      | `InjectAvailableSpecialists` malformed JSON.         | Low        | Low                | Identified  |
| C1  | Extreme Input      | Massive string construction in `collectKnowledge`.   | Low        | Low                | Identified  |
| C2  | Extreme Budget     | `MaxInt64` budget handling.                          | Low        | Medium             | Identified  |
| D1  | Concurrency        | `RegisterDB` vs `Compile` race condition.            | High       | High               | Identified  |
| D2  | State Conflict     | Cache invalidation race ("Stale Write").             | Critical   | High               | Identified  |

## 8. Code Review Checklist for Future Developers

To prevent regression and ensure ongoing quality, future developers working on the `internal/prompt` subsystem should adhere to the following checklist:

### 8.1 Input Validation
- [ ] Are all fields in `CompilationContext` validated for length and content?
- [ ] Are file paths (e.g., DB paths) checked for existence and permissions?
- [ ] Are JSON inputs (e.g., `agents.json`) parsed with error checking for type mismatches?

### 8.2 Database Interactions
- [ ] Is `sql.DB` access protected against concurrent closure?
- [ ] Are queries parameterized to prevent injection?
- [ ] Is the database connection pool configured correctly for the expected load?
- [ ] Are long-running queries avoided in the critical path?

### 8.3 Concurrency & State
- [ ] Are all shared maps (like `cache`) protected by mutexes?
- [ ] Is the "Check-then-Act" pattern used safely?
- [ ] Are long-running operations (like Vector Search) capable of being canceled via Context?
- [ ] Is the cache invalidation logic sound (handles race conditions)?

### 8.4 Mangle Integration
- [ ] Are all strings passed to Mangle properly quoted/escaped?
- [ ] Are return values from Mangle checked for type and nil-ness?
- [ ] Are logic rules (in `.mg` files) tested for stratification and termination?
- [ ] Are fallback mechanisms in place if Mangle evaluation fails?

## 9. Risk Assessment Matrix

The following matrix categorizes the identified risks based on Likelihood and Impact.

| Likelihood \ Impact | Low | Medium | High | Critical |
| :--- | :--- | :--- | :--- | :--- |
| **High** | A3 (Empty Input) | A1 (Empty Context) | D1 (Concurrency) | D2 (Stale Cache) |
| **Medium** | C1 (Extreme Input) | C2 (Extreme Budget) | B1 (Type Coercion) | |
| **Low** | B2 (Malformed JSON) | | | |

*   **D2 (Stale Cache)** is Critical/High because it leads to incorrect behavior that is persistent (until TTL) and hard to detect.
*   **D1 (Concurrency)** is High/High because it causes intermittent failures that degrade reliability.
*   **B1 (Type Coercion)** is High Impact because it can cause silent logic failures, but Medium Likelihood assuming kernel stability.

## 10. Historical Context of JIT Compilation

The evolution of Just-In-Time compilation in CodeNerd moved from a purely static template system to the current dynamic, atom-based architecture. This shift introduced significant complexity:
-   **Static Templates**: Deterministic, easy to test, no state mutation.
-   **Dynamic Atoms**: State-dependent, hard to test, relies on external DBs.

The move to dynamic compilation was driven by the need for context-aware prompts that adapt to the user's intent (e.g., "fix bug" vs "write feature"). However, the testing strategy has lagged behind this architectural shift, focusing primarily on the "happy path" of successful compilation rather than the chaotic reality of concurrent updates and malformed inputs.

## 11. Glossary

-   **Atom**: A fragment of a system prompt (e.g., a specific coding rule or persona definition).
-   **Skeleton**: The mandatory, deterministic core of a prompt (Identity, Safety).
-   **Flesh**: The optional, probabilistic parts of a prompt (Exemplars, Context).
-   **Mangle**: The logic programming engine used to select atoms based on rules.
-   **Vector Search**: The semantic search engine used to find relevant flesh atoms.
-   **CompilationContext**: The input object containing all request parameters (Shard ID, Intent, etc.).

## 12. Conclusion

The `JITPromptCompiler` is a sophisticated and critical component of CodeNerd. While its architecture allows for flexible and powerful prompt assembly, it currently exhibits classic vulnerabilities associated with distributed state management and dynamic typing.

The most critical findings are:
1.  **Cache Coherence Race**: The potential to serve stale prompts after a knowledge update is a significant correctness issue.
2.  **Database Concurrency**: The risk of `database closed` errors during updates reduces system reliability.
3.  **Type Coercion**: The silent swallowing of type errors from the Kernel masks potential bugs and makes diagnostics difficult.

By implementing the recommended tests and code fixes, we can significantly harden the subsystem against these failure modes, ensuring that CodeNerd remains reliable even under stress and during dynamic updates. The goal is to move from "it works when the stars align" to "it works even when the database is restarting and the user is typing in emojis".

## 13. References

-   `internal/prompt/compiler.go`
-   `internal/prompt/selector.go`
-   `internal/prompt/compiler_test.go`
-   Google Mangle Documentation (Internal)
-   Vector Search Integration Specs
-   Standard Go SQL/DB Documentation
-   CodeNerd Quality Assurance Handbook (2025 Edition)

*Signed,*
*Jules*
*QA Automation Engineer*
*Codenerd Quality Assurance Division*
