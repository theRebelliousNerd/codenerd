# QA Journal: Boundary Value Analysis of JIT Prompt Compiler
**Date:** 2026-02-01 05:03:00 EST
**Subject:** `internal/prompt/compiler.go` (JITPromptCompiler)
**QA Engineer:** Jules (AI Automation Specialist)
**Scope:** Deep Boundary Value Analysis & Negative Testing

## Executive Summary

The `JITPromptCompiler` is the "Executive Function" of the codeNERD architecture. It is responsible for assembling the prompt context that drives the LLM's cognition. A failure here is catastrophic: it can lead to hallucination (missing context), safety bypasses (missing constitutional atoms), or total system failure (crashes/deadlocks).

This analysis focuses on **Negative Testing**, **Boundary Value Analysis**, and **Destructive Testing**. We are not interested in the happy path; we assume the happy path works. We are interested in what happens when the world is imperfect.

This document serves as the primary reference for the "hardening" phase of the JIT subsystem. It identifies 4 major vectors of failure and details 12 specific gaps in the current test suite.

## 1. System Overview & Risk Profile

The `JITPromptCompiler` is a complex orchestration engine that combines:
1.  **Determinism:** Mangle logic rules (Skeleton).
2.  **Probabilism:** Vector search semantics (Flesh).
3.  **Resource Constraints:** Token budgets (Knapsack-like problem).
4.  **State:** Caching, Database connections, Mutexes.

**Risk Profile:** HIGH.
-   **Safety Critical:** Yes (Constitutional Guardrails are injected here).
-   **Performance Critical:** Yes (Blocking path for every user turn).
-   **Complexity:** High (Polyglot: Go, SQL, Mangle Logic, Vector Math).
-   **Dependencies:** High (Kernel, SQLite, Vector Store).

## 2. Boundary Value Analysis Vectors

### Vector A: Null, Undefined, and Empty States

The "Nothingness" vector. Go is statically typed, so we don't have "undefined", but we have `nil` pointers and "zero values" which are often more dangerous because they are silent.

#### A1. The `nil` Options Pattern
The constructor `NewJITPromptCompiler` accepts variadic options.
-   **Scenario:** `NewJITPromptCompiler(nil)` or `NewJITPromptCompiler(WithKernel(nil))`.
-   **Current Behavior:** `NewJITPromptCompiler` iterates options. If an option function is nil, it panics? No, the slice contains functions. But if `WithKernel(nil)` is called, it sets `c.kernel = nil`.
-   **Risk:** `Compile` methods check `if c.kernel != nil`, but what about `selector.SetKernel(nil)`? If `selector` assumes a non-nil kernel after initialization, it will panic.
-   **Test Gap:** Explicitly pass `nil` to all `With*` functions and assert no panics.
-   **Reproduction Snippet:**
    ```go
    func TestCrash_NilOption(t *testing.T) {
        // Should not panic
        c, err := NewJITPromptCompiler(WithKernel(nil), WithVectorSearcher(nil))
        assert.NoError(t, err)
        // Should not panic when used
        _, err = c.Compile(context.Background(), NewCompilationContext())
    }
    ```

#### A2. Empty Context Fields
-   **Scenario:** `CompilationContext` with empty `ShardID`, empty `IntentVerb`, or empty strings in `Languages` slice.
-   **Risk:**
    -   `collectKernelInjectedAtoms` relies on `cc.ShardID`. If empty, does it match nothing? Or everything (wildcard accident)?
    -   `collectKnowledgeAtoms` constructs a query string. `strings.Join` on empty parts might result in an empty query, triggering a vector search for "" which might return garbage or error.
-   **Test Gap:** Fuzz `CompilationContext` fields with empty strings and observe atom selection.
-   **Reproduction Snippet:**
    ```go
    func TestCrash_EmptyContext(t *testing.T) {
        cc := &CompilationContext{
            ShardID: "", // Empty
            IntentVerb: "",
            Languages: []string{"", "", ""}, // Empty strings
        }
        // Should handle gracefully
        c.Compile(context.Background(), cc)
    }
    ```

#### A3. The "Ghost" Database
-   **Scenario:** `RegisterDB("corpus", "")` or `RegisterDB("corpus", "/path/to/nowhere")`.
-   **Risk:** `sql.Open` often succeeds even if the file doesn't exist (lazy open). The error happens at `Ping` or first `Query`.
-   **Current Code:** `RegisterDB` does call `Ping`. Good.
-   **But:** What if the file exists but is 0 bytes? Or is a directory? Or is a locked file?
-   **Test Gap:** Test `RegisterDB` with: a directory path, a non-existent path, a 0-byte file, a currupt SQLite file.
-   **Expected Behavior:** Should return explicit error "invalid database file".

### Vector B: Type Coercion and Data Fidelity

The "Square Peg in Round Hole" vector. The interface between Go and Mangle/SQL is a translation layer prone to fidelity loss.

#### B1. Kernel Type Dissonance
The Mangle kernel returns `Fact` objects where `Args` are `[]interface{}`.
-   **Scenario:** Mangle rule returns a number `42` or a complex structure `p(/a)` instead of a string atom ID.
-   **Current Code:** `collectKernelInjectedAtoms` uses `extractStringArg` (unexported).
-   **Risk:** If `extractStringArg` is naive (e.g., just type asserts `string`), it will panic or return empty on non-strings. Mangle `Atom` types are often custom structs or `int64`.
-   **Test Gap:** Mock the Kernel to return `int`, `float64`, `bool`, `nil`, and custom structs in `Args`. Verify `Compile` degrades gracefully (logs warning) rather than crashing.
-   **Reproduction Snippet:**
    ```go
    type mockBadKernel struct {}
    func (m *mockBadKernel) Query(pred string) ([]Fact, error) {
        return []Fact{{Predicate: "test", Args: []interface{}{123, nil, false}}}, nil
    }
    ```

#### B2. JSON Unmarshalling of `agents.json`
-   **Scenario:** `InjectAvailableSpecialists` parses `.nerd/agents.json`.
-   **Risk:** If `agents.json` contains numbers where strings are expected (e.g., `"name": 123`), `json.Unmarshal` will fail or leave fields empty.
-   **Test Gap:** Create a temporary `agents.json` with malformed types and verify `InjectAvailableSpecialists` handles it (likely by returning a fallback string).

### Vector C: User Request Extremes

The "Kaiju" vector. What happens when inputs are aggressively large, small, or weird?

#### C1. The Token Budget Singularity
-   **Scenario:** `TokenBudget = 0` or `TokenBudget = -1`.
-   **Current Code:** `budgetMgr.Fit` likely assumes positive integers.
-   **Risk:**
    -   Infinite loop if logic tries to "reduce" content to fit < 0.
    -   Divide by zero in `BudgetUtilization` calculation (`float64(used) / float64(budget)`).
-   **Test Gap:** Compile with `WithTokenBudget(0)` and `WithTokenBudget(-100)`. Assert `BudgetUtilization` does not result in `NaN` or `+Inf` (or panics).

#### C2. The Million Atom March
-   **Scenario:** The database contains 100,000 atoms. `Compile` is called.
-   **Risk:**
    -   `collectAtomsWithStats` loads all into a slice `[]*PromptAtom`. 100k structs might be 100MB+ RAM.
    -   `selector.SelectAtomsWithTiming` iterates all 100k. $O(N)$.
    -   `resolver.Resolve` might be $O(N^2)$ or $O(N \log N)$.
    -   Performance cliff: Latency exceeds user patience (e.g., >2s).
-   **Test Gap:** Performance test with 100k mocked atoms. Verify latency stays < 200ms. If not, we need pagination or pre-filtering.
-   **Mitigation:** Add `LIMIT` clause to SQL load or filter by category in SQL.

#### C3. The Dependency Ouroboros (Cycles)
-   **Scenario:** Atom A depends on B. Atom B depends on A.
-   **Risk:** `resolver.Resolve` enters infinite recursion or stack overflow.
-   **Current Code:** `Resolve` likely has visited sets.
-   **Test Gap:** Create a tight loop A->B->A and a long loop A->B->C->...->A. Verify topological sort returns an error or breaks the cycle deterministically.

#### C4. Deeply Nested Dependencies
-   **Scenario:** A->B->C->... (depth 1000).
-   **Risk:** Stack overflow if using recursive DFS for resolution.
-   **Test Gap:** Construct a chain of 1000 atoms. Verify `Resolve` completes without stack overflow.

### Vector D: State Conflicts and Concurrency

The "Heisenbug" vector.

#### D1. The Thundering Herd
-   **Scenario:** 50 concurrent requests for the same context (e.g., a popular agent starts up).
-   **Risk:** Cache miss -> 50 simultaneous DB reads + 50 Mangle evaluations + 50 Vector searches.
-   **Current Code:** `c.cacheMu` protects the map read/write. But does it protect the *generation*?
    -   Likely: Check cache (Lock/Unlock) -> Miss -> Compile (No Lock) -> Store (Lock/Unlock).
    -   Result: 50 redundant compilations.
-   **Optimization Gap:** Singleflight pattern (suppress duplicate in-flight requests).
-   **Test Gap:** Spawn 50 goroutines requesting the same context. Measure how many actually call `Compile` internals (mock counters).

#### D2. The Hot-Swap Race
-   **Scenario:** `RegisterDB` is called (reloading corpus) while `Compile` is running.
-   **Risk:** `c.projectDB` is closed while `collectAtomsWithStats` is reading from it.
-   **Result:** `sql: database is closed` error in the middle of a request.
-   **Current Code:** `RegisterDB` locks `c.mu`. `collectAtomsWithStats` locks `c.mu.RLock`.
-   **Verdict:** This seems safe (RWMutex).
-   **Verification:** Confirm that `collectAtomsWithStats` holds the RLock for the *entire* duration of `loadAtomsFromDB`. (Looking at code: Yes, `defer c.mu.RUnlock()` is used).

#### D3. The Stale Cache
-   **Scenario:** `RegisterDB` updates the corpus. Cache contains prompts compiled from old corpus.
-   **Risk:** Users get old prompts after an update.
-   **Test Gap:** Verify `RegisterDB` invalidates or clears `c.cache`. (Checking code: `RegisterDB` does NOT seem to clear `c.cache`. **BUG SUSPICION**).

## 3. Detailed Improvement Recommendations

### Improvement 1: Singleflight for Compilation
**Problem:** Thundering herd on cache miss.
**Solution:** Implement `golang.org/x/sync/singleflight`.
**Code:**
```go
// In JITPromptCompiler struct
requestGroup singleflight.Group

// In Compile
key := cc.Hash()
val, err, _ := c.requestGroup.Do(key, func() (interface{}, error) {
    return c.compileInternal(ctx, cc)
})
```

### Improvement 2: Cache Invalidation Strategy
**Problem:** `RegisterDB` updates source truth but cache remains stale.
**Solution:** `RegisterDB` should call `c.ClearCache()`.
**Code:**
```go
func (c *JITPromptCompiler) RegisterDB(...) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    // ... swap DB ...
    c.cache = make(map[string]*CompilationResult) // Wipe cache
    return nil
}
```

### Improvement 3: Robust Type Coercion Helper
**Problem:** Mangle interactions are brittle.
**Solution:**
```go
func extractStringArg(arg interface{}) string {
    switch v := arg.(type) {
    case string:
        return v
    case fmt.Stringer:
        return v.String()
    case int, int64, float64:
        return fmt.Sprintf("%v", v)
    case nil:
        return ""
    default:
        return "" // specific fallback
    }
}
```

### Improvement 4: Budget Math Safety
**Problem:** Division by zero risk.
**Solution:**
```go
if budget > 0 {
    result.BudgetUsed = float64(result.TotalTokens) / float64(budget)
} else {
    result.BudgetUsed = 1.0 // Or 0.0, define semantics for infinite budget
}
```

## 4. Performance Assessment

### Latency Budget
-   **Target:** < 50ms (p99) for cached, < 500ms (p99) for uncached.
-   **Bottlenecks:**
    1.  **Vector Search:** `c.vectorSearcher.Search` is an external I/O call (or heavy computation). This is the biggest unknown.
    2.  **Mangle Assert:** `c.kernel.AssertBatch`. If context has many facts, this bridges Go->Mangle.
    3.  **Atom Loading:** `loadAtomsFromDB`. SQL query latency.

### Scalability
-   **Atoms:** $N$.
-   **Complexity:** Linear $O(N)$ for selection.
-   **Limit:** ~10,000 atoms seems fine. ~100,000 atoms will degrade to >1s latency without indexing.
-   **Recommendation:** If atom count grows > 10k, implement pre-filtering by Category in SQL ("WHERE category IN (...)") before loading into Go memory. Current `loadAtomsFromDB` loads *everything*.

## 5. Test Gaps Checklist (Detailed)

The following gaps must be addressed in `internal/prompt/compiler_test.go`.

### Gap 1: Empty/Invalid ShardID
-   **Description:** When `ShardID` is empty string, does the compiler accidentally match wildcard rules `/_all` or skip everything?
-   **Priority:** High (Security risk - accidental access).
-   **Implementation:** `t.Run("empty_shard_id_logic", ...)`

### Gap 2: Massive Atom Corpus
-   **Description:** Load 100k atoms into memory. Measure `Compile` latency.
-   **Priority:** Medium (Scalability).
-   **Implementation:** Benchmark with N=100000.

### Gap 3: Invalid DB Paths
-   **Description:** Pass a directory or non-existent path to `RegisterDB`.
-   **Priority:** Medium (Robustness).
-   **Implementation:** `RegisterDB("corpus", "/tmp/nonexistent")` -> should error.

### Gap 4: Malformed `agents.json`
-   **Description:** `InjectAvailableSpecialists` reads a file with invalid JSON types (int instead of string).
-   **Priority:** Low (Config error).
-   **Implementation:** Write temp file `agents.json` with `{ "agents": [{"name": 123}] }`.

### Gap 5: Kernel Type Coercion
-   **Description:** Kernel returns `int` or `nil` in `Fact.Args`.
-   **Priority:** High (Crash risk).
-   **Implementation:** Mock kernel returning mixed types.

### Gap 6: Extreme Token Budgets
-   **Description:** `TokenBudget` is -1 or 0.
-   **Priority:** High (Math safety).
-   **Implementation:** `WithTokenBudget(0)`. Check for panic or NaN.

### Gap 7: Cache Staleness
-   **Description:** Compile -> RegisterDB(new_db) -> Compile. Result should match new DB.
-   **Priority:** High (Correctness).
-   **Implementation:** Mock changing DB and verify output changes.

### Gap 8: Massive Content Strings
-   **Description:** Atom content is 10MB string.
-   **Priority:** Low (DoS).
-   **Implementation:** Allocate huge string, try to compile.

### Gap 9: Deep Dependency Chains
-   **Description:** A->B->C... (1000 deep).
-   **Priority:** Medium (Stack overflow).
-   **Implementation:** Generate chain, call `Resolve`.

### Gap 10: Empty/Partial Context Pointers
-   **Description:** `CompilationContext` fields are nil/empty.
-   **Priority:** Medium (Robustness).
-   **Implementation:** Fuzz context.

### Gap 11: Thundering Herd
-   **Description:** Concurrent calls for same key.
-   **Priority:** Medium (Performance).
-   **Implementation:** `sync.WaitGroup` with 50 goroutines.

### Gap 12: Nil Options
-   **Description:** `NewJITPromptCompiler(nil)`.
-   **Priority:** Low (API safety).
-   **Implementation:** Constructor test.

## 6. Conclusion

The `JITPromptCompiler` is well-structured but trusts its inputs and environment too much. It assumes:
1.  The Database is always there and valid.
2.  The Mangle Kernel always speaks "String".
3.  The Token Budget is always reasonable.
4.  The Cache is always consistent with the DB.

These assumptions are vulnerabilities in a production neuro-symbolic system where "dreaming" agents (generating their own context) might feed garbage into the compiler. Strengthening the boundary checks and adding fallback mechanisms is critical for achieving "High Assurance" status.

## 7. Action Items

1.  Add `// TODO: TEST_GAP` comments to `internal/prompt/compiler_test.go` for each of the 12 gaps above.
2.  Implement `extractStringArg` helper immediately to prevent crashes.
3.  Implement `c.ClearCache()` in `RegisterDB`.
4.  Add unit tests for the 12 gaps.

---
*End of Journal Entry*
