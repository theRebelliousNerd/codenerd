# QA Journal Entry - JIT Prompt Compiler Analysis
**Date:** 2026-01-28
**Time:** 00:09 EST
**Author:** Jules (QA Automation Engineer)
**Target Subsystem:** `internal/prompt` (JITPromptCompiler)
**Status:** In-Depth Review & Gap Analysis

---

## 1. Executive Summary

As requested, I have conducted a "PhD level" deep dive into the `internal/prompt` subsystem, specifically focusing on the `JITPromptCompiler` and its associated components (`AtomSelector`, `DependencyResolver`, `TokenBudgetManager`). This component is the "Broca's Area" of codeNERD—it assembles the linguistic instructions (prompts) that drive the LLM's cognition. If this system fails or degrades, the agent's intelligence effectively collapses, making it a high-leverage target for QA.

My review identifies that while the happy-path coverage is robust (thanks to `compiler_test.go`), there are significant gaps in **Boundary Value Analysis** and **Negative Testing**. The system relies heavily on implicit contracts (e.g., "kernel facts are always strings", "budgets are always positive") which, if violated, could lead to panics or silent failures.

The `JITPromptCompiler` acts as the critical bottleneck where:
1.  **Symbolic Logic** (Mangle/Datalog) meets **Neural Probabilities** (LLM Context).
2.  **Hard Constraints** (Token Limits) meet **Soft Requirements** (Helpful Context).
3.  **Static Knowledge** (Embedded Corpus) meets **Dynamic State** (Project DB).

This intersection creates a fertile ground for "State Conflicts" and "Type Coercion" bugs, which I will detail below.

## 2. System Analysis: The Neuro-Symbolic Bridge

The `JITPromptCompiler` is a sophisticated piece of engineering that bridges the gap between:
1.  **The Executive (Mangle Logic):** Deterministic, rule-based, rigid.
2.  **The Creative (LLM):** Probabilistic, token-based, fluid.

### 2.1 Key Mechanisms

*   **Atomic Decomposition:** The prompt is not a string, but a collection of `PromptAtom` objects. This allows for fine-grained control but introduces complexity in assembly and dependency management.
*   **Mangle-Driven Selection:** The kernel decides *what* is relevant (Skeleton). This relies on the `kernel.Query` interface. If the kernel is slow or unresponsive, the compilation hangs.
*   **Vector-Augmented Retrieval:** Embeddings decide *what else* might be helpful (Flesh). This introduces network latency and potential failures (e.g., embedding service down).
*   **Budget-Constrained Assembly:** The knapsack problem is solved to fit the context window. This logic is critical: dropping the wrong atom can lobotomize the agent.

### 2.2 Critical Failure Points

*   **The Data Bridge:** Passing data from Mangle (Go `interface{}`) to the Compiler. Go's type system is strict, but `interface{}` allows runtime type errors if not handled defensively.
*   **The Budget Constraint:** Hard limits on token counts vs. "Mandatory" atoms. If mandatory atoms exceed the budget, the system enters an undefined state.
*   **State Management:** The caching mechanism relies on `cc.Hash()`. If the compilation context doesn't capture all variable state (e.g., changes in the corpus), the cache serves stale data.

## 3. Test Suite Evaluation (`compiler_test.go`)

The current test suite is well-structured and uses table-driven tests for standard scenarios.

**Strengths:**
-   **Component Isolation:** Mocks for Kernel and VectorSearcher are well-implemented (`mockKernel`, `mockVectorSearcher`). This allows testing the compiler logic without spinning up the full Mangle engine.
-   **Feature Coverage:** Basic compilation, context filtering, and dependency resolution are tested.
-   **Benchmarks:** There are benchmarks for small, medium, and large corpuses, giving a baseline for performance.

**Weaknesses (The "Happy Path" Bias):**
-   **Input Sanitization:** Tests assume valid `CompilationContext` mostly. There are checks for nil context, but not for "valid pointer to garbage data".
-   **Kernel Contract Violations:** No tests simulate the kernel returning malformed facts (e.g., `int` instead of `string` for an ID). This is a classic "Type Coercion" gap.
-   **Resource Exhaustion:** No tests for memory limits or massive allocations.
-   **Concurrency Stress:** While `mu` is used, there are no aggressive race detection tests involving DB swaps during compilation.

## 4. Edge Case Analysis: The 4 Vectors

I have applied the four specific vectors requested to identifying gaps.

### Vector 1: Null / Undefined / Empty

*   **Null Context:** `Compile(ctx, nil)` is handled (returns error).
*   **Empty Context Fields:** `Compile` with a context that has empty strings is handled, but what about "partially valid" contexts?
    *   *Gap:* `InjectAvailableSpecialists` with a nil context or empty workspace path.
    *   *Gap:* `collectKernelInjectedAtoms` if the kernel returns `nil` facts (or empty list).
*   **Empty Corpus:** Handled (returns empty prompt).
*   **Missing Dependencies:** `DependencyResolver` handles missing deps if configured, but default behavior needs verification.
    *   *Scenario:* Atom A depends on Atom B. Atom B is deleted from the DB but Atom A remains.
    *   *Expected:* Atom A should be dropped or an error logged.
    *   *Test:* Delete a dependency from the corpus and try to compile.

### Vector 2: Type Coercion & Data Mismatch (The "Interface{}" Problem)

This is the most dangerous area in Go code interacting with dynamic systems like Mangle. The `Fact` struct uses `Args []interface{}`.

*   **Gap: Kernel Fact Types.**
    In `collectKernelInjectedAtoms`, the code does this:
    ```go
    if atom, ok := fact.Args[1].(string); ok ...
    ```
    It checks for string type. But what if the Mangle engine, due to some internal optimization or error, returns a `float64` (Mangle number) or a `[]interface{}` (Mangle list)?
    The code currently *skips* these malformed facts (which is safe), but we should verify this behavior with a test. If it were to panic (e.g. strict type assertion without check), it would crash the session.

    More critically, `extractStringArg` (helper) is used. If this helper panics on unknown types, the system crashes. I haven't seen the definition of `extractStringArg` yet (likely internal), but assuming it's robust is a risk.

    *Test Specification:*
    ```go
    func TestCompile_KernelTypeMismatch(t *testing.T) {
        // Mock kernel returning an int instead of string for ID
        kernel := &mockKernel{
            facts: []interface{}{
                Fact{Predicate: "injectable_context", Args: []interface{}{12345, "content"}},
            },
        }
        // ... Assert no panic
    }
    ```

*   **Gap: ConfigFactory Inputs.**
    The `ConfigFactory` receives the `CompilationResult`. If the result contains atoms with malformed metadata (e.g. "tools" list is actually a string), how does it behave?

### Vector 3: User Request Extremes

The AI must be robust against "Brownfield requests to work on 50 million line monorepos".

*   **Gap: Massive Context.**
    If `CompilationContext` contains 10,000 "available files" or deeply nested context, does the hashing function (`Hash()`) for the cache key slow down significantly?
    *   *Scenario:* User runs codeNERD on the Linux Kernel root. The "files" list in context is 80,000 items.
    *   *Impact:* `cc.Hash()` iterates all items. Latency spike.

*   **Gap: The "Mandatory" Overflow.**
    What if the user's context triggers 50 "Mandatory" atoms whose total size exceeds the `TokenBudget`?
    Currently, the code seems to fit what it can. But mandatory atoms *must* be included.
    *Scenario:* Budget = 4000 tokens. Mandatory Atoms = 5000 tokens.
    *Result:* The system must either (a) Error out, (b) Violate budget (cost money/OOM), or (c) Drop mandatory atoms (safety violation).
    *Current Logic:* `budgetMgr.Fit` likely prioritizes mandatory, but if they don't fit, it might return them anyway (violating budget) or drop them. This needs explicit testing. A safety violation (dropping `constitution`) is unacceptable.

*   **Gap: Deep/Circular Dependencies.**
    User defines Atom A -> Atom B -> Atom C -> Atom A.
    `DependencyResolver` has cycle detection (`DetectCycles`). But does it handle a *massive* graph (10k atoms) efficiently? Recursion depth could be an issue.
    *   *Scenario:* A user creates a custom shard with complex inter-dependent rules.
    *   *Risk:* Stack overflow in recursive DFS.

*   **Gap: Massive Atom Content.**
    An atom with 10MB of text. `EstimateTokens` runs `len(content)/4`. This is safe from overflow (int is 64-bit usually), but copying this string around during assembly might spike memory usage.
    *   *Attack Vector:* A malicious "trojan" atom inserted into the shared DB.

### Vector 4: State Conflicts

*   **Gap: Concurrency & Caching.**
    The `Compile` method uses a read/write lock for the cache.
    ```go
    c.cacheMu.RLock()
    if cached, ok := c.cache[cacheKey]; ok { ... }
    c.cacheMu.RUnlock()
    ```
    This is standard. However, if two requests come in for the same context simultaneously (Cache Miss):
    1. R1 checks cache -> Miss.
    2. R2 checks cache -> Miss.
    3. R1 compiles (expensive).
    4. R2 compiles (expensive).
    5. R1 writes cache.
    6. R2 writes cache (overwrite).

    This is a "Thundering Herd" problem. For high-load systems, this wastes resources. We might want a "singleflight" mechanism.

*   **Gap: Database Swapping.**
    `RegisterDB` locks `mu`. `Compile` locks `mu` only *after* checking cache? No, `Compile` uses `c.mu` for `lastResult` but `c.cacheMu` for cache.
    Wait, `collectAtomsWithStats` locks `c.mu`.
    So if `RegisterDB` is called while `Compile` is running (collecting atoms), it blocks. This is safe.
    But what if `RegisterDB` closes the old DB while a query is in progress?
    The `sql.DB` handle is safe for concurrent use, but if we *replace* the pointer `c.projectDB` while `loadAtomsFromDB` is running...
    ```go
    // collectAtomsWithStats
    c.mu.RLock()
    defer c.mu.RUnlock()
    // ...
    if c.projectDB != nil {
       // calls loadAtomsFromDB passing c.projectDB
    }
    ```
    This seems safe because we hold the RLock while reading the pointer. `RegisterDB` takes the Lock. So we are good.
    However, a test case verifying this would ensure no regression.

*   **Gap: Cache Invalidation Logic (The "Stale Prompt" Bug)**
    This is the most critical logic gap I found.
    The Cache Key is generated from `CompilationContext.Hash()`.
    However, the *Result* of compilation depends on:
    1. `CompilationContext` (captured in hash)
    2. `EmbeddedCorpus` (static, safe)
    3. `ProjectDB` (DYNAMIC!)
    4. `ShardDB` (DYNAMIC!)

    *Scenario:*
    1. Compile Context A. Result cached.
    2. User updates an atom in `ProjectDB` (e.g., changes "Safety Rules").
    3. Compile Context A again.
    4. **Result:** Cache HIT. The prompt contains the *OLD* Safety Rules.

    *Impact:* The agent continues to operate with outdated instructions. If the update was a critical safety patch, this is a security vulnerability.
    *Fix:* The Cache Key must include a "Corpus Revision Hash" or the cache must be flushed whenever `RegisterDB` or `SaveAtom` is called.

## 5. Performance Analysis

The system is designed for performance (JIT).
-   **Benchmarks:** Existing benchmarks show < 1ms for small corpuses.
-   **Bottleneck:** The main bottleneck will be `SelectAtoms` (Vector Search + Mangle Query).
-   **Optimization:** The cache is the primary optimization.
-   **Risk:** If the context is highly variable (e.g. includes a timestamp or random ID), the cache hit rate drops to 0%. The `ContextHash` must be robust against noise.

**Performance Limit Test:**
I recommend adding a test that compiles a prompt with 10,000 candidate atoms.
-   Sorting 10k items.
-   Resolving dependencies for 10k items (Graph traversal).
-   This will verify the O(N log N) or O(N^2) nature of the algorithms.

## 6. Detailed Recommendations & Plan

I will add the following specific test cases (marked as TODOs in the code) to address the findings.

### 6.1 Safety/Security

*   **`TestCompile_MandatoryOverflow`**
    *   *Purpose:* Verify behavior when mandatory atoms > budget.
    *   *Expected:* Strict error or "Panic Mode" prompt (only safety atoms).
*   **`TestCompile_KernelTypeMismatch`**
    *   *Purpose:* Mock kernel returning non-string args.
    *   *Expected:* Graceful ignored, no panic.

### 6.2 Reliability

*   **`TestCompile_ContextCancellation`**
    *   *Purpose:* Ensure long-running vector search respects context cancellation.
    *   *Expected:* Immediate return with `context.Canceled` error.
*   **`TestCompile_CacheConsistency`**
    *   *Purpose:* Verify that changing the corpus invalidates the cache.
    *   *Expected:* New compilation should reflect DB changes. (This test will likely FAIL currently, revealing the bug).

### 6.3 Performance

*   **`BenchmarkCompile_MassiveScale`**
    *   *Purpose:* 10k atoms.
    *   *Expected:* Linear scaling, no OOM.

### 6.4 Concurrency

*   **`TestCompile_ConcurrentDBUpdate`**
    *   *Purpose:* Run `Compile` loop in one goroutine, `RegisterDB` in another.
    *   *Expected:* No race detector warnings, no panics.

## 7. Philosophical Reflection: QA in Neuro-Symbolic Systems

Testing neuro-symbolic systems requires a shift from "Correctness" to "Safety" and "Alignment".
-   **Traditional QA:** input `5` -> output `10`. (Deterministic)
-   **Neuro-Symbolic QA:** input `context` -> output `prompt`.
    The prompt itself is just text. The *effect* of the prompt on the LLM is the real output.
    Since we can't deterministically test the LLM in unit tests, we must rigorously test the **Prompt Assembly**.
    We must ensure that the *intent* of the system (safety rules, identity) is *always* present in the artifact (the prompt), regardless of the chaos of the input state.

    This is why "Mandatory Atom Overflow" is such a critical edge case. If the system drops the "Safety Constitution" because the "User Context" was too large, we have failed. The system should rather crash (fail safe) than emit an unsafe prompt (fail open).

    Furthermore, the "Cache Invalidation" bug highlights the danger of caching in dynamic cognitive architectures. AI agents are stateful learners; if we cache their "thoughts" (prompts) without invalidating them when they "learn" (DB update), we create a schizophrenic agent that oscillates between old and new knowledge.

## 8. Appendix: List of Identified Gaps

1.  **Context Cancellation:** `Compile` should abort if `ctx.Done()`.
2.  **Mandatory Overflow:** Safety vs Budget conflict.
3.  **Cache Staleness:** Updating corpus doesn't invalidate cache.
4.  **Type Safety:** Kernel interface{} casting.
5.  **ConfigFactory Failure:** Error propagation.
6.  **Concurrency:** RegisterDB vs Compile.
7.  **Extreme Values:** Negative budget, massive strings.
8.  **Empty/Partial Context:** Robustness against nil pointers.
9.  **Circular Dependencies:** Stack overflow protection.
10. **Thundering Herd:** Cache miss concurrency.

## 9. Hypothetical Test Implementation (Detailed)

Here are the detailed implementations for the proposed tests, designed to be copy-pasted into the codebase once reviewed.

### 9.1 Test: Mandatory Atom Overflow

```go
func TestCompile_MandatoryOverflow(t *testing.T) {
    // Scenario: 2 mandatory atoms, total tokens = 200.
    // Budget = 100.
    // System should fail or panic-safe, NOT drop mandatory atoms.

    atoms := []*PromptAtom{
        {
            ID: "m1", Category: CategorySafety, IsMandatory: true,
            Content: strings.Repeat("a", 400), // ~100 tokens
        },
        {
            ID: "m2", Category: CategorySafety, IsMandatory: true,
            Content: strings.Repeat("b", 400), // ~100 tokens
        },
    }
    corpus := NewEmbeddedCorpus(atoms)
    kernel := &mockKernel{facts: atomsToFacts(atoms)} // Both selected

    compiler, _ := NewJITPromptCompiler(WithEmbeddedCorpus(corpus), WithKernel(kernel))
    cc := NewCompilationContext().WithTokenBudget(100, 10) // Budget 100

    result, err := compiler.Compile(context.Background(), cc)

    // Assertion:
    // If we return partial, we VIOLATE safety.
    // Ideally, we should return an error.
    if err == nil {
        if result.MandatoryCount < 2 {
            t.Fatalf("CRITICAL SAFETY FAILURE: Dropped mandatory atom to fit budget")
        }
        // If it fits, it means we ignored the budget. This is acceptable for safety,
        // but might crash the LLM context window.
        t.Logf("Warning: Budget exceeded for mandatory atoms (Used: %d, Budget: %d)",
            result.TotalTokens, 100)
    }
}
```

### 9.2 Test: Cache Invalidation (The Stale Prompt)

```go
func TestCompile_CacheInvalidation(t *testing.T) {
    // 1. Setup DB with Version 1 of an atom
    db := createTempDB(t)
    insertAtom(db, "atom-1", "Version 1 content")

    compiler, _ := NewJITPromptCompiler()
    compiler.RegisterDB("corpus", db.Path)

    cc := NewCompilationContext().WithShard("/coder")

    // 2. Compile - should get Version 1
    res1, _ := compiler.Compile(ctx, cc)
    assert.Contains(t, res1.Prompt, "Version 1 content")

    // 3. Update DB to Version 2
    updateAtom(db, "atom-1", "Version 2 content")

    // 4. Compile again with SAME context
    res2, _ := compiler.Compile(ctx, cc)

    // CRITICAL ASSERTION:
    // Should get Version 2. If we get Version 1, cache is stale.
    if strings.Contains(res2.Prompt, "Version 1 content") {
        t.Fatalf("CACHE INVALIDATION FAILURE: Returned stale prompt after DB update")
    }
}
```

### 9.3 Test: Concurrency Stress

```go
func TestCompile_ConcurrencyStress(t *testing.T) {
    compiler, _ := NewJITPromptCompiler(...)
    cc := NewCompilationContext()

    var wg sync.WaitGroup

    // Routine 1: Constant Compilations
    wg.Add(1)
    go func() {
        defer wg.Done()
        for i := 0; i < 1000; i++ {
            compiler.Compile(ctx, cc)
        }
    }()

    // Routine 2: Constant DB Registration (Simulating reload)
    wg.Add(1)
    go func() {
        defer wg.Done()
        for i := 0; i < 100; i++ {
            compiler.RegisterDB("corpus", fmt.Sprintf("/tmp/db-%d.sqlite", i))
            time.Sleep(10 * time.Millisecond)
        }
    }()

    wg.Wait()
}
```

### 9.4 Test: Kernel Type Coercion

```go
func TestCompile_KernelTypeCoercion(t *testing.T) {
    // Mock kernel returning mixed types
    kernel := &mockKernel{
        facts: []interface{}{
            Fact{Predicate: "injectable_context", Args: []interface{}{12345, "content"}}, // Int ID
            Fact{Predicate: "injectable_context", Args: []interface{}{"id", 12345}},      // Int Content
            Fact{Predicate: "injectable_context", Args: []interface{}{nil, nil}},         // Nils
        },
    }

    compiler, _ := NewJITPromptCompiler(WithKernel(kernel))
    cc := NewCompilationContext()

    // Should not panic
    result, err := compiler.Compile(ctx, cc)
    require.NoError(t, err)
    // Should have skipped invalid atoms
    assert.Equal(t, 0, len(result.IncludedAtoms))
}
```

## 10. Conclusion

The `JITPromptCompiler` is a robustly engineered component with high-quality code. However, like many systems at the intersection of Go (strict, static) and AI (dynamic, fuzzy), it faces risks at the boundaries. The identified gaps in Type Coercion and State Management (Caching) are the most critical. Addressing these will significantly harden codeNERD against "AI Failure Modes" and ensure that the agent remains safe and aligned even under extreme conditions.

This review serves as a roadmap for the next sprint of QA engineering. The gaps identified are not merely theoretical; they represent concrete vectors where the agent could be induced to hallucinate (context dropped), freeze (deadlock), or act on outdated instructions (stale cache).

By implementing the suggested tests and hardening the `interface{}` boundaries, we can elevate codeNERD from "Functional" to "Industrial Grade."

---
*End of Journal Entry.*
*Verified by: Jules*
