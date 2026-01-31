# QA Journal: JIT Prompt Compiler Boundary Value Analysis
**Date:** 2026-01-30 00:06 EST
**Author:** Jules (QA Automation Engineer)
**Subsystem:** Internal / Prompt / JITPromptCompiler
**File:** `internal/prompt/compiler.go`

## Executive Summary

This journal entry documents a deep-dive Boundary Value Analysis (BVA) and Negative Testing strategy for the `JITPromptCompiler` subsystem. This component is the "brain" of the codeNERD agent, responsible for dynamically assembling system prompts from atomic fragments based on context, Mangle rules, and token budgets.

Given its critical role in the "Clean Loop" architecture, failure here leads to "brain dead" agents, hallucinated instructions, or security bypasses. The analysis focuses on edge cases, type safety with the Mangle kernel, and resilience against extreme inputs.

## 1. System Overview & Criticality

The `JITPromptCompiler` operates at the intersection of:
1.  **Deterministic Logic (System 2)**: Mangle rules defining the "Skeleton" (identity, safety).
2.  **Probabilistic Search (System 1)**: Vector embeddings defining the "Flesh" (context, exemplars).
3.  **Resource Constraints**: Strict token budgets managed by `TokenBudgetManager`.

**Criticality Rating:** MAXIMUM.
If this fails, the LLM receives no instructions or malformed instructions.

## 2. Boundary Value Analysis Vectors

### Vector A: Null, Undefined, and Empty States

**Concept:** Go's type system handles `nil` for pointers, but logic errors often occur when fields *inside* structs are empty strings or nil slices.

**Identified Gaps:**

1.  **Empty CompilationContext Fields**:
    *   **Scenario:** `ShardID` is empty string "".
    *   **Impact:** `collectAtomsWithStats` skips shard-specific atoms. `matchesShard` in `collectKernelInjectedAtoms` might fail or match wildly if not careful.
    *   **Code Review:** `matchesShard` checks `rawTrim == cc.ShardID`. If both are empty, it might match universal atoms unintentionally.
    *   **Recommendation:** Explicitly test `ShardID=""` behavior.

2.  **Nil Options in Constructor**:
    *   **Scenario:** `NewJITPromptCompiler(WithEmbeddedCorpus(nil), WithKernel(nil))`.
    *   **Impact:** The compiler initializes with nil components. `Compile` checks `c.embeddedCorpus != nil`, but does it handle `c.kernel == nil` in all paths?
    *   **Code Review:** `Compile` checks `if c.kernel != nil` before asserting context facts. `collectKernelInjectedAtoms` returns `nil, nil` if kernel is nil. Seems safe, but requires verification that no NPEs occur in `selector`.

3.  **Empty Database Paths**:
    *   **Scenario:** `RegisterDB("corpus", "")`.
    *   **Impact:** `sql.Open("sqlite3", "")` might open a temp DB or fail.
    *   **Recommendation:** Verify behavior with invalid paths.

4.  **Nil External Dependencies**:
    *   **Scenario:** `InjectAvailableSpecialists(nil, ...)` or `InjectAvailableSpecialists(..., "")`.
    *   **Impact:** Function returns early `return nil`. Safe, but should be verified.

### Vector B: Type Coercion & Mangle Integration

**Concept:** The Mangle kernel communicates via `interface{}` facts. The Go code must bridge the gap between untyped Mangle atoms and typed Go strings.

**Identified Gaps:**

1.  **Kernel Fact Types**:
    *   **Scenario:** Mangle returns a fact `selected_result(123, "high", <nil>)`.
    *   **Impact:** `extractStringArg` converts `123` to `"123"` and `<nil>` to `"<nil>"`.
    *   **Analysis:**
        *   If AtomID matches "123" in the corpus, it works.
        *   If Source matches "<nil>", it might be logged as such.
        *   **Risk:** "Atom vs String" dissonance. If Mangle returns an atom `/foo`, does the Go driver receive it as string `"/foo"` or `"foo"`? The code assumes strings. If the driver returns a custom `mangle.Atom` type, `extractStringArg` falls back to `fmt.Sprintf("%v")`. This might produce structs like `{Value: foo}` instead of `"foo"`.
    *   **Recommendation:** Verify `extractStringArg` behavior with complex types (structs, pointers) if the underlying driver ever changes.

2.  **JSON Unmarshalling**:
    *   **Scenario:** `InjectAvailableSpecialists` reads `agents.json`.
    *   **Impact:** If JSON types don't match (e.g., "topics" is an array instead of string), `json.Unmarshal` fails. The code catches this and provides a fallback.
    *   **Verification:** Ensure the fallback is actually robust and informative.

### Vector C: User Request Extremes

**Concept:** Inputs that are valid types but have extreme values.

**Identified Gaps:**

1.  **Token Budget Extremes**:
    *   **Scenario:** `TokenBudget = 0` or `TokenBudget = -1`.
    *   **Impact:** `mangleMandatoryLimits` sets caps based on budget. If budget is <= 0, it defaults to constants.
    *   **Scenario:** `TokenBudget = MaxInt`.
    *   **Impact:** Logic should hold, but allocation arrays might bloat if sized by budget (unlikely here).

2.  **Massive Atom Content**:
    *   **Scenario:** An atom with 10MB of text.
    *   **Impact:** `EstimateTokens` might be slow. String concatenation in `Assembler` might spike memory.
    *   **Recommendation:** Benchmark with large atoms to detect OOM or timeout risks.

3.  **Dependency Graph Extremes**:
    *   **Scenario:** Circular dependencies (Atom A depends on B, B depends on A).
    *   **Impact:** `DependencyResolver` (not fully analyzed here, but assumed) must handle cycles without infinite recursion.
    *   **Scenario:** Deep chains (A->B->C->...->Z).
    *   **Impact:** Stack overflow if recursive.

4.  **Massive Corpus**:
    *   **Scenario:** 100,000 atoms in SQLite.
    *   **Impact:** `loadAtomsFromDB` loads *all* into memory slice.
    *   **Risk:** 100k * 1KB = 100MB. Per compilation? No, `collectAtomsWithStats` calls `loadAtomsFromDB` *every time*.
    *   **CRITICAL PERFORMANCE RISK:** If `projectDB` has many atoms, loading them all on every `Compile` call is a massive bottleneck.
    *   **Mitigation:** The `cache` helps, but a cache miss triggers full load.
    *   **Refactoring Needed:** `loadAtomsFromDB` should probably be cached or iterative.

### Vector D: State Conflicts & Concurrency

**Concept:** The compiler is a shared singleton in the `SessionExecutor`. Multiple goroutines might call `Compile` or `RegisterDB`.

**Identified Gaps:**

1.  **RegisterDB vs Compile Race**:
    *   **Scenario:** Goroutine 1 calls `Compile`. Goroutine 2 calls `RegisterDB` (closes old DB, assigns new).
    *   **Impact:** `collectAtomsWithStats` reads `c.projectDB` under `c.mu.RLock()`. `RegisterDB` acquires `c.mu.Lock()`.
    *   **Analysis:** This seems thread-safe due to RWMutex.
    *   **Edge Case:** `loadAtomsFromDB` takes `db` as argument. If `RegisterDB` closes the DB *after* `Compile` has retrieved the pointer but *before* it queries it?
        *   `Compile` gets `c.projectDB` (pointer).
        *   `RegisterDB` closes that `db` and assigns new one.
        *   `Compile` (holding old pointer) tries `db.QueryContext`.
        *   **Result:** "sql: database is closed" error.
        *   **Fix:** `loadAtomsFromDB` should probably be called while holding the lock, OR we accept that a mid-flight compilation fails if DB is swapped.

2.  **Cache Invalidation**:
    *   **Scenario:** DB is updated (atoms added).
    *   **Impact:** Cache is keyed by `CompilationContext` hash. It does NOT include DB state hash.
    *   **Result:** **Stale Data**. The compiler will serve old prompts even if DB changed, until TTL expires.
    *   **Recommendation:** `RegisterDB` should clear the cache? Or cache keys should include corpus version?

## 3. Detailed Code Analysis: `internal/prompt/compiler.go`

### 3.1 `Compile` Method Analysis

The `Compile` method is the heart of the system. Let's trace the execution path for boundary conditions.

```go
func (c *JITPromptCompiler) Compile(ctx context.Context, cc *CompilationContext) (*CompilationResult, error) {
    // ... validation ...

    // Cache Check
    // RACE: c.cacheMu.RLock() protects read.
    // POTENTIAL ISSUE: If cache is huge, this map lookup is fast, but eviction policy?
    // There is NO eviction policy in the provided code. Memory leak risk if many unique contexts.

    // Collect Atoms
    // CALLS: collectAtomsWithStats
    // RISK: DB Load on every miss.

    // Assert Context Facts
    // RISK: c.kernel.AssertBatch() might fail if kernel is remote/disconnected.
    // The code logs a warning but continues. This is GOOD (graceful degradation).

    // Kernel Injection
    // CALLS: collectKernelInjectedAtoms
    // RISK: String parsing of Mangle args.

    // Selection
    // CALLS: c.selector.SelectAtomsWithTiming
    // CRITICAL: This is where Mangle vs Vector weights are applied.

    // ... resolving, fitting, assembling ...

    // Cache Write
    // RACE: c.cacheMu.Lock()
    // ISSUE: Unbounded growth.
}
```

### 3.2 `collectAtomsWithStats` Analysis

```go
func (c *JITPromptCompiler) collectAtomsWithStats(...) {
    // ...
    // 2. Project Database
    if c.projectDB != nil {
        // CALLS: loadAtomsFromDB(ctx, c.projectDB)
        // CRITICAL PERFORMANCE ISSUE:
        // This function executes `SELECT * FROM prompt_atoms` on EVERY call.
        // Even if SQLite is fast, unmarshalling thousands of rows into structs is slow in Go.
        // It generates garbage (GC pressure).
    }
    // ...
}
```

**Observation**: The system assumes the corpus is small (hundreds of atoms). If users add thousands of knowledge atoms or rules, this will degrade rapidly.

### 3.3 `loadAtomsFromDB` Analysis

```go
func (c *JITPromptCompiler) loadAtomsFromDB(ctx context.Context, db *sql.DB) ([]*PromptAtom, error) {
    // ...
    // Query: SELECT ... FROM prompt_atoms
    // RISK: No LIMIT clause. Loads everything.
    // RISK: No WHERE clause filtering by shard/category (optimization opportunity).
    // ...
    // 2. Load Context Tags
    // Query: SELECT ... FROM atom_context_tags
    // RISK: N+1 query problem avoidance (good), but loads ALL tags for ALL atoms.
    // If there are 10k atoms and 10 tags each = 100k rows.
}
```

**Refactoring Recommendation**: We should move to a "Lazy Load" or "Cached Corpus" model. The corpus should be loaded once into memory (like `EmbeddedCorpus`) and refreshed only when signaled.

## 4. Mangle Logic & Type Coercion Deep Dive

Mangle is a logic language. Facts are tuples of terms. Terms can be:
- Atoms (`/foo`)
- Numbers (`123`, `123.456`)
- Strings (`"foo"`)
- Structures (`foo(1, 2)`)
- Lists (`[1, 2]`)

Go's `database/sql` driver and the `mangle` package integration act as the FFI (Foreign Function Interface).

### The `extractStringArg` Vulnerability

Located in `internal/prompt/selector.go`, this function is the bridge:

```go
func extractStringArg(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
```

**Scenario 1: Mangle returns an Atom `/foo`**
- If the Go driver wraps Mangle atoms in a struct `mangle.Constant`, and that struct implements `fmt.Stringer` returning `"/foo"`, then `extractStringArg` returns `"/foo"`.
- If our atom IDs in the DB are stored as `"identity/coder"` (no leading slash), and we compare `"/identity/coder"` == `"identity/coder"`, the match fails.
- **Verification**: Check if `collectKernelInjectedAtoms` strips leading slashes.
    - Code: `rawTrim := strings.TrimPrefix(raw, "/")`
    - It does! This suggests the developer anticipated this. Good.

**Scenario 2: Mangle returns a Number `123`**
- `extractStringArg` returns `"123"`.
- If Atom ID is `"123"`, it matches.
- This effectively coerces numbers to string IDs. This is acceptable behavior.

**Scenario 3: Mangle returns a List `[/a, /b]`**
- `extractStringArg` returns `"[/a /b]"` (standard Go fmt for slices).
- This will likely NOT match any Atom ID.
- Logic continues, effectively treating it as "no match". Safe.

**Conclusion**: The coercion logic is robust against crashes, but relies on string matching conventions (e.g., stripping slashes) to be semantically correct.

## 5. Detailed Test Plan

### 5.1 New Negative Tests

We will add the following tests to `internal/prompt/compiler_test.go`:

1.  **`TestCompile_WithZeroBudget`**: Verify behavior when budget is 0. Should default or return minimal skeleton.
2.  **`TestCompile_WithNilComponents`**: Explicitly construct compiler with nil kernel/corpus and call Compile.
3.  **`TestCompile_Concurrency`**: Run parallel compilations while toggling `RegisterDB`. Expect potential errors but no panics.
4.  **`TestCompile_TypeCoercion`**: Mock a kernel that returns `int` IDs. Verify `extractStringArg` handles it.

### 5.2 Performance Benchmarks

1.  **`BenchmarkCompile_LargeCorpus_Cold`**: Measure time with 10k atoms (cache miss).
2.  **`BenchmarkCompile_LargeCorpus_Hot`**: Measure time with 10k atoms (cache hit).

## 6. Proposed Improvements

1.  **Cache Invalidation Strategy**:
    *   Add `InvalidateCache()` method.
    *   Call it in `RegisterDB`.
2.  **Corpus Caching**:
    *   Don't reload all atoms from SQLite on every request. Cache the `[]*PromptAtom` from DB and invalidate only on change.
3.  **Robust Type Extraction**:
    *   Ensure `extractStringArg` logs a warning if it encounters unexpected types (like `nil`).

## 7. Hypothetical Crash Trace

If `cache` map is accessed without lock (e.g., a dev removes the mutex):

```text
fatal error: concurrent map read and map write

goroutine 18 [running]:
codenerd/internal/prompt.(*JITPromptCompiler).Compile(0xc000102000, 0xc000203000, 0xc000304000)
    /home/jules/codenerd/internal/prompt/compiler.go:215 +0x45
...
```

The current implementation uses `c.cacheMu` correctly, so this is avoided.

If `TokenBudgetManager` panics on 0 budget:

```text
panic: runtime error: integer divide by zero

goroutine 18 [running]:
codenerd/internal/prompt.(*TokenBudgetManager).Fit(...)
    /home/jules/codenerd/internal/prompt/budget.go:55
codenerd/internal/prompt.(*JITPromptCompiler).Compile(...)
    /home/jules/codenerd/internal/prompt/compiler.go:320
```

We need to verify `budget.go` handles 0. The compiler code sets `budget = c.config.DefaultTokenBudget` if `budget <= 0`. This prevents the zero division at the top level, but what if `DefaultTokenBudget` is also 0?

Code check in `compiler.go`:
```go
	budget := cc.AvailableTokens()
	if budget <= 0 {
		budget = c.config.DefaultTokenBudget
	}
    // ...
	fitted, err := c.budgetMgr.Fit(ordered, budget)
```

If `DefaultTokenBudget` is 0 (set via `WithDefaultTokenBudget(0)`), then `budget` is 0.
Does `budgetMgr.Fit` handle 0? We need to test this.

## 8. Specific Test Implementations

Here is the exact code logic we will add to `compiler_test.go`:

### 8.1 Test: Extreme Token Budgets

```go
func TestCompile_ZeroTotalBudget(t *testing.T) {
    // Setup
    compiler := New(...)
    compiler.config.DefaultTokenBudget = 0 // Force zero
    cc := NewCompilationContext().WithTokenBudget(0, 0)

    // Execute
    result, err := compiler.Compile(ctx, cc)

    // Assert
    // Should probably error or return only mandatory atoms that fit (which is none if budget is strict)
    // or panic if budgetMgr divides by zero.
}
```

### 8.2 Test: Concurrency

```go
func TestCompile_ConcurrentDBRegistry(t *testing.T) {
    compiler := New(...)
    var wg sync.WaitGroup

    // Reader routine
    wg.Add(1)
    go func() {
        defer wg.Done()
        for i := 0; i < 100; i++ {
            compiler.Compile(ctx, cc)
        }
    }()

    // Writer routine
    wg.Add(1)
    go func() {
        defer wg.Done()
        for i := 0; i < 10; i++ {
            compiler.RegisterDB("project", "test.db")
            time.Sleep(1 * time.Millisecond)
        }
    }()

    wg.Wait()
}
```

### 8.3 Test: Nil Safety

```go
func TestCompile_AllNil(t *testing.T) {
    // NewJITPromptCompiler uses functional options.
    // If we pass nil as options or set internal fields to nil manually (via unsafe or if exported)
    // we want to ensure no panic.
    // Since fields are private, we rely on public API.

    c, _ := NewJITPromptCompiler()
    // c.embeddedCorpus is nil by default if not set.
    // c.kernel is nil by default.
    // c.projectDB is nil.

    // This is the default state test.
    res, err := c.Compile(ctx, cc)
    assert.NoError(t, err)
    assert.Empty(t, res.Prompt)
}
```

## 9. Reference Traceability

The following table maps the identified vectors to lines in `internal/prompt/compiler.go` that need testing.

| Vector | File | Line (Approx) | Vulnerability |
| :--- | :--- | :--- | :--- |
| **Null/Empty Context** | `compiler.go` | 136 | `if cc == nil` (Validation present but limited) |
| **Cache Race** | `compiler.go` | 148 | `c.cacheMu.RLock()` (Needs high-concurrency verification) |
| **DB Performance** | `compiler.go` | 175 | `loadAtomsFromDB` call on every miss |
| **Mangle Type Coercion** | `compiler.go` | 268 | `extractStringArg(fact.Args[0])` |
| **Budget Logic** | `compiler.go` | 315 | `if budget <= 0` fallback logic |
| **Kernel Injection** | `compiler.go` | 370 | `matchesShard` with empty shard IDs |
| **Specialist Injection** | `compiler.go` | 830 | `os.ReadFile` failure handling |

## 10. Conclusion

The `JITPromptCompiler` is well-structured but optimized for small-scale use. The most significant risks are **Performance** (loading full DB on every request) and **Memory Leaks** (unbounded cache). The logic errors (Type Coercion, Nulls) are mostly handled but deserve explicit regression tests.

The addition of the proposed tests and `// TODO: TEST_GAP` comments will significantly harden the system against regression as it scales.

---
*End of Journal Entry*
