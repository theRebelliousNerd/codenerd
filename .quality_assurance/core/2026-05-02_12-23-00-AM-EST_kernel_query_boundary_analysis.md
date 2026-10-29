---

remediated: true
remediated_date: 2026-05-12
subsystem: core
---
# Kernel Query Subsystem Boundary Value Analysis & Negative Testing Journal
**Date:** 2026-05-02_12-23-00-AM-EST
**Target Subsystem:** `internal/core/kernel_query.go`

## Executive Summary

The Kernel Query subsystem is responsible for interacting with the Mangle logic engine to retrieve derived facts (`Query`, `QueryAll`) and safely loading facts into the Extensional Database (`LoadFactsFromFile`, `ParseFactString`). Because it bridges the gap between Go's procedural/concurrent runtime and Mangle's pure logic-based execution, it is highly sensitive to state conflicts, concurrency constraints, and Mangle parsing irregularities.

This analysis highlights gaps in the test suite (`kernel_query_test.go`), which currently only covers superficial "happy path" syntax parsing and a basic recursive join. Real-world execution will bombard these functions with complex state mutations, extreme data sizes, and edge cases where Mangle's strict type system rejects loosely coerced Go inputs.

## 1. Null / Undefined / Empty Input Boundaries

The methods in this package handle file reading, string parsing, and query execution. Several assumptions are made regarding string safety and initialization state.

### Identified Test Gaps

*   **`Query` with uninitialized kernel:** The logic states `if !k.initialized { return nil, err }`. There is no test proving this fails safely instead of panicking on a nil dereference in `k.mu.RLock()`.
*   **`QueryAll` with `k.programInfo == nil`:** Mangle AST programs might compile but result in a nil `programInfo` if initialization is bypassed. Does `QueryAll` safely return an empty map as implemented, or does it panic downstream?
*   **Empty strings in `ParseFactString`:** Supplying `""` or `"."` to `ParseFactString`. The code appends a `.` to wrap it in a minimal program. An empty string results in `.`, which is structurally invalid Mangle syntax. Does it return a clean error or panic the Mangle parser?
*   **`LoadFactsFromFile` on empty files:** An empty file contains zero bytes. `ParseFactsFromString` will process this, and `LoadFactsFromFile` checks `len(facts) == 0`. We need to verify that an empty file produces a clean no-op and does not cause a crash.
*   **`Query` with empty predicate:** Calling `k.Query("")` should immediately error out since `""` is not a valid Mangle name atom.

## 2. Type Coercion and Data Mapping boundaries

Go uses dynamic interface conversion (`interface{}`) before casting to Mangle `BaseTerm`s. Mangle enforces strict typing (`NameType`, `StringType`, `NumberType`, `Float64Type`).

### Identified Test Gaps

*   **`baseTermToValue` Type Exhaustiveness:** What happens if a newer version of Mangle introduces a new AST primitive (e.g. `ListType`, `BooleanType`), and `baseTermToValue` hits the `default:` case? The fallback is to use the Go string formatter (`fmt.Sprintf("%v", term)`). We must test this fallback path to ensure logic doesn't silently corrupt data by comparing a string representation of an object to a native Atom.
*   **Variable Type Binding in Pattern Matching (`factMatchesPattern`):** When `Query` parses a pattern like `test_ancestor(X, "alice")`, it must correctly differentiate between the string `"alice"` and the name atom `/alice`. A test gap exists verifying that querying for `foo("bar")` strictly excludes `foo(/bar)`.
*   **Number to Float Coercion:** If Mangle stores an integer (`ast.NumberType`) and Go logic expects a float, how does `Query` surface the result? We need to verify that `Query` perfectly preserves numeric precision without silent truncation or type shifting.

## 3. User Request Extremes

The AI system may attempt to query or load massive amounts of data as campaigns run over a long period.

### Identified Test Gaps

*   **Massive Arity in `Query` Pattern:** What happens if the system queries `foo(X, Y, Z, ...)` with 1,000 arguments? The Mangle AST parser or `factMatchesPattern` loop could hit a recursion limit, stack overflow, or memory bound.
*   **Extremely Large Extensional Database (EDB) querying via `QueryAll`:** If the Mangle store contains 5,000,000 derived facts, `QueryAll` calls `atomToFact` and appends to a slice for every single one. We need a performance benchmark and negative test to ensure `QueryAll` does not cause Out-of-Memory (OOM) failures or stall the kernel thread indefinitely.
*   **Excessive File Size in `LoadFactsFromFile`:** An AI agent might dump a 500MB `.mg` file and attempt to load it. `os.ReadFile` pulls the entire file into memory, and `ParseFactsFromString` builds an AST for the entire file at once. This is a severe OOM vulnerability.
*   **Deep Recursion strings in `ParseFactString`:** Generating deeply nested lists or complex term structures in a fact string: `foo(foo(foo(foo(...))))`. Does `parse.Unit` have depth limits, or does it hang/crash?

## 4. State Conflicts and Concurrency

Mangle's `SimpleInMemoryStore` is not thread-safe on its own, which is why `RealKernel` uses a `sync.RWMutex`.

### Identified Test Gaps

*   **Concurrent `Query` and `UpdateSystemFacts`:** `UpdateSystemFacts` acquires an exclusive lock (implicitly via `k.Transaction()`), while `Query` holds a read lock. We must test that rapidly polling `Query` does not starve `UpdateSystemFacts` (reader starvation).
*   **Git command races during `UpdateSystemFacts`:** If `git status` or `git rev-parse` takes 15 seconds because of a slow network drive or massive repo, `UpdateSystemFacts` will stall the kernel thread. What happens if context cancellation occurs or `Query` requests stack up?
*   **File Deletion during `LoadFactsFromFile`:** A classic Time-of-Check to Time-of-Use (TOCTOU) vulnerability. If `LoadFactsFromFile` is called, but the file is deleted immediately before `os.ReadFile` executes, it must fail cleanly without crashing the kernel initialization sequence.

## Conclusion

The `kernel_query_test.go` file currently implements only 2 basic tests. Adding explicit boundary, type coercion, and extreme-load testing is critical. The Mangle engine is strict; passing it malformed strings or allowing it to ingest unbounded data sizes will lead to catastrophic runtime panics.

Action: Adding `TODO: TEST_GAP:` comments to `internal/core/kernel_query_test.go` to flag these structural vulnerabilities.

## Detailed Negative Test Cases Formulation

To comprehensively cover the identified gaps, the following specific test functions must be added to `kernel_query_test.go`.

### 1. Null / Undefined / Empty Input Boundaries

#### `TestKernelQuery_Query_Uninitialized`
**Scenario:** Call `k.Query("foo")` on a `RealKernel` instance where `k.initialized` is false.
**Expected:** The function should immediately return a non-nil error indicating "kernel not initialized", and crucially, must not panic attempting to dereference `k.store` or `k.programInfo`.
**Performance impact:** Negligible. Fail-fast error return.

#### `TestKernelQuery_QueryAll_NilProgramInfo`
**Scenario:** Bypass normal initialization such that `k.programInfo` is explicitly nil, then call `k.QueryAll()`.
**Expected:** The function should hit the `if k.programInfo == nil` block and return an empty map and no error, avoiding a panic when it attempts to iterate over `k.programInfo.Decls`.
**Performance impact:** Negligible.

#### `TestKernelQuery_ParseFactString_EmptyString`
**Scenario:** Pass `""` to `ParseFactString`.
**Expected:** The function wraps this as `.` and passes it to the Mangle parser. Mangle parser should return a syntax error indicating no clauses. The test must verify that `parse.Unit` returns cleanly with an error, and does not crash or panic the engine.
**Performance impact:** Negligible.

#### `TestKernelQuery_ParseFactString_OnlyPeriod`
**Scenario:** Pass `"."` to `ParseFactString`.
**Expected:** Similar to the empty string, this parses as `..`. It must return a clean parsing error.

#### `TestKernelQuery_LoadFactsFromFile_EmptyFile`
**Scenario:** Create a 0-byte temporary file and pass its path to `LoadFactsFromFile`.
**Expected:** The file reads cleanly, Mangle parser finds 0 facts, and the function returns `nil` (success) without attempting to load anything or index out of bounds on a facts slice.
**Performance impact:** Negligible.

#### `TestKernelQuery_Query_EmptyPredicate`
**Scenario:** Call `k.Query("")`.
**Expected:** The function should attempt to match the predicate `""` against declarations. Since `""` is an invalid Mangle identifier, it should cleanly return 0 results and no error, or specifically fail fast.
**Performance impact:** Negligible.

### 2. Type Coercion and Data Mapping boundaries

#### `TestKernelQuery_BaseTermToValue_UnknownTypeFallback`
**Scenario:** Construct an artificial `ast.Constant` with a type code not explicitly handled in the `switch t.Type` block of `baseTermToValue` (e.g., a hypothetical future `ListType` constant).
**Expected:** The switch should fall through to the `default` case, log a warning, and gracefully fall back to returning `t.Symbol` instead of causing an unhandled type panic.
**Performance impact:** Negligible.

#### `TestKernelQuery_FactMatchesPattern_StringVsAtom`
**Scenario:** Populate the kernel with `foo(/bar)` (a name atom) and query for the pattern `foo("bar")` (a string).
**Expected:** `Query` must return 0 results. Mangle treats atoms and strings as completely disjoint types. The pattern matching logic must not incorrectly coerce the string `"bar"` into the atom `/bar` and erroneously return a match.
**Performance impact:** Negligible, standard type checking overhead.

#### `TestKernelQuery_Query_FloatPrecisionPreservation`
**Scenario:** Populate the kernel with a fact containing a high-precision float: `measurement(3.141592653589793)`. Query for this fact.
**Expected:** The resulting Go `Fact` object must contain a `float64` argument with exactly the same value. It must not be truncated, converted to string implicitly, or shifted to an integer type.
**Performance impact:** Negligible.

#### `TestKernelQuery_Query_IntegerCoercion`
**Scenario:** Populate the kernel with a fact containing a large integer: `large_id(9223372036854775807)`.
**Expected:** The resulting Go `Fact` object must contain an `int64` argument without overflow or silent conversion to a less precise type.

### 3. User Request Extremes

#### `TestKernelQuery_Query_MassiveArity`
**Scenario:** Construct a fact string with a predicate containing 10,000 arguments: `massive_fact(1, 2, 3, ..., 10000)`. Parse it using `ParseFactString`.
**Expected:** The Mangle parser should either successfully parse it, or hit a maximum arity limit and return a clean error. It must not cause a stack overflow or OOM panic.
**Performance impact:** High CPU usage during parsing, testing parser robustness.

#### `TestKernelQuery_QueryAll_ExtremelyLargeEDB`
**Scenario:** Bypass normal file loading and directly populate `k.store` with 1,000,000 synthetic ground facts under a single predicate. Call `k.QueryAll()`.
**Expected:** The function must iterate, convert all 1,000,000 facts, and return the populated map. The test must assert that memory consumption stays within reasonable bounds (e.g., < 200MB) and execution completes within a set timeout (e.g., 2 seconds).
**Performance impact:** High. Tests O(N) iteration speed and memory allocation overhead.

#### `TestKernelQuery_LoadFactsFromFile_MassiveFile`
**Scenario:** Generate a 100MB temporary file containing 1,000,000 valid Mangle facts. Call `LoadFactsFromFile`.
**Expected:** The function uses `os.ReadFile`, which pulls the entire 100MB into memory, then parses it. The test should verify that the system handles this load cleanly, or errors out due to size limits safely, without crashing the test runner or host process.
**Performance impact:** Very high. Stresses memory allocator and garbage collector.

#### `TestKernelQuery_ParseFactString_DeepRecursion`
**Scenario:** Construct a fact string with extreme nesting, e.g., `nested(nested(nested(...)))` to a depth of 5000.
**Expected:** The AST parser must gracefully reject the deeply nested term with a recursion limit error, preventing a stack overflow panic that would crash the entire application.
**Performance impact:** Moderate.

#### `TestKernelQuery_ParseFactsFromString_ManyClauses`
**Scenario:** Pass a string containing 50,000 individual fact declarations separated by periods.
**Expected:** The engine should correctly parse all 50,000 clauses without degrading into quadratic time complexity.
**Performance impact:** High CPU.

### 4. State Conflicts and Concurrency

#### `TestKernelQuery_Concurrency_QueryVsUpdateSystemFacts`
**Scenario:** Spawn 100 goroutines that constantly loop calling `k.Query("test_fact")`. While these are running, spawn 5 goroutines that call `k.UpdateSystemFacts()`.
**Expected:** The `UpdateSystemFacts` calls (which acquire a write lock via transaction) must successfully complete. The RWMutex must correctly balance readers and writers, preventing writer starvation. No panics should occur from concurrent map/store access.
**Performance impact:** Very high concurrency stress.

#### `TestKernelQuery_UpdateSystemFacts_GitHang`
**Scenario:** Mock the `gitCmd` function (or point `workspaceRoot` to a specialized test script acting as git) that sleeps for 30 seconds to simulate a hanging git process on a massive mono-repo.
**Expected:** `UpdateSystemFacts` does not currently accept a `context.Context`, so it will hang indefinitely, blocking all subsequent writes to the kernel. This test exposes a critical flaw that needs to be refactored (adding context to system fact updates).
**Performance impact:** Will block the test thread until timeout unless a specific short timeout is enforced in the test.

#### `TestKernelQuery_LoadFactsFromFile_TOCTOU_Deletion`
**Scenario:** Start calling `LoadFactsFromFile(path)`. Immediately after the path is constructed but before `os.ReadFile` is invoked, delete the file in a separate goroutine.
**Expected:** `os.ReadFile` will return a "no such file or directory" error. The function must return this error cleanly, logging the failure, and must not crash or leave the kernel in an inconsistent state.
**Performance impact:** Negligible.

#### `TestKernelQuery_Concurrency_ParseSingleFact`
**Scenario:** Call `ParseSingleFact` simultaneously from 100 goroutines with different complex strings.
**Expected:** Since `ParseSingleFact` relies on the stateless `parse.Unit` from the Mangle library, it should be entirely thread-safe. The test verifies no shared global state is mutated during parsing.
**Performance impact:** Moderate CPU load.

#### `TestKernelQuery_Concurrency_AssertAndQuery`
**Scenario:** Spawn 10 goroutines continuously asserting new facts (`k.Assert`), while 50 goroutines continuously query (`k.Query`).
**Expected:** The system must maintain consistency. Queries should return varying results as facts are added, but no read operations should ever return partially formed structs or encounter "concurrent map read and map write" panics.
**Performance impact:** High concurrency stress on the underlying store.

#### `TestKernelQuery_UpdateSystemFacts_InvalidWorkspace`
**Scenario:** Set `k.workspaceRoot` to a directory where the process lacks read permissions, or to a path that is not a git repository.
**Expected:** `UpdateSystemFacts` must log the git command failures but must still successfully assert the `current_time` fact and commit the transaction safely. It should not abort the transaction entirely just because git facts are missing.
**Performance impact:** Negligible.

## Further Considerations

The `kernel_query.go` file represents a critical boundary between the AI agent's semantic desires and the rigid logic engine. Any unhandled panic in parsing or OOM in querying will immediately crash the entire `codenerd` process, destroying the user's current session state.

The most critical vulnerabilities identified are the lack of bounding on `LoadFactsFromFile` (which reads the whole file into RAM) and the absence of a `context.Context` in `UpdateSystemFacts` (which can indefinitely block the kernel writer lock if `git status` hangs). Implementing these tests will definitively prove the existence of these flaws so they can be remediated.

## System Architecture Review Regarding Edge Case Vectors

When assessing the boundary value limits and negative testing resilience of the Kernel Query subsystem, we must consider the hardware profile detailed in the initial prompt parameters: "a laptop with 8GB of RAM on it but still require high performance from codenerd to understand it quickly."

### Evaluating the Null/Undefined/Empty Vector on 8GB RAM

Null pointer dereferences and empty string panics are independent of hardware constraints. However, handling empty inputs (like a zero-byte file passed to `LoadFactsFromFile`) correctly without allocating unnecessary structures or spinning up unneeded goroutines is essential for a lightweight footprint.
The current implementation of `ParseFactString` appends a period (`.`) to the end of the input string to form a complete Mangle program. If the input is empty, the resulting program is just `.`. The underlying `github.com/google/mangle/parse` library must parse this. A robust system on an 8GB laptop shouldn't waste precious CPU cycles or allocate deep error stacks for trivial empty inputs. A fast-path rejection for empty strings should be evaluated to improve latency.

### Evaluating the Type Coercion Vector on 8GB RAM

Type coercion in `baseTermToValue` uses reflection and type switching. In Go, type switches (`switch t := term.(type)`) are relatively fast, but if the system relies on the fallback `fmt.Sprintf("%v", term)` heavily due to unrecognized types, it forces string allocations on the heap.
On an 8GB laptop, aggressive heap allocation triggers the Garbage Collector (GC) more frequently. If an AI agent attempts to query a dataset with 50,000 facts, and 10% of those facts fall back to `fmt.Sprintf`, the resulting GC pauses will cause noticeable stutter in the agent's reasoning loop.
Testing the boundaries of type coercion isn't just about correctness; it's about identifying unoptimized data conversion paths that degrade performance on constrained hardware. We must verify that `NumberType`, `Float64Type`, `NameType`, and `StringType` perfectly map to Go primitives without allocating new strings whenever possible.

### Evaluating User Request Extremes on 8GB RAM

This is where the 8GB RAM constraint becomes the primary bottleneck.

1.  **Massive `LoadFactsFromFile`:** If a user requests Codenerd to analyze a "50 million line monorepo," the agent may generate massive `.mg` files containing derived project architecture facts. `LoadFactsFromFile` currently calls `os.ReadFile(path)`. This function reads the *entire* file into a single contiguous byte slice in RAM. If the file is 500MB, `os.ReadFile` allocates 500MB. Then `ParseFactsFromString` converts that string into an AST, potentially taking another 1GB to 2GB of RAM due to pointer overhead and tree structures. On an 8GB laptop, where the OS and IDE already consume 4-6GB, this single operation could trigger swapping (thrashing) or an immediate Out Of Memory (OOM) kill by the OS.
    *Conclusion:* The system is **not performant enough** in its current state to handle this edge case. A streaming parser approach must be implemented if massive EDBs are to be supported.

2.  **`QueryAll` on massive EDBs:** If the kernel contains millions of facts, `QueryAll` iterates through the entire logic store and builds a massive `map[string][]Fact`. Slices of structs (`[]Fact`) are relatively memory-efficient in Go, but if the `Args` slice inside each `Fact` contains numerous heap-allocated strings, the overhead multiplies. Returning a 1,000,000 fact map will cause a massive memory spike.
    *Conclusion:* The system is vulnerable here. A pagination or streaming iterator API (`QueryStream`) should be added to handle large data retrievals safely on an 8GB machine.

3.  **Massive Arity:** A fact like `foo(1, 2, ..., 10000)` might seem absurd, but an LLM hallucinating code could easily generate a Mangle fact with an unbounded list of arguments if trying to summarize an array inline. The slice allocations for `Fact.Args` would be large, but more importantly, the Mangle parser's recursive descent logic might stack overflow. Go's initial goroutine stack size is 2KB and grows dynamically, but deeply recursive parsers can hit OS-level limits or cause massive stack copying overhead.

### Evaluating State Conflicts on 8GB RAM

Concurrency issues (race conditions, deadlocks) are exacerbated on constrained hardware because CPU scheduling becomes tighter and disk I/O (like swapping) introduces unpredictable latency spikes.

1.  **The Git Hang (TOCTOU / Blocking):** As noted in the test formulations, `UpdateSystemFacts` calls out to the OS via `exec.Command("git", ...)` to gather repository status. On a massive monorepo stored on a slow disk, or if the 8GB laptop is currently thrashing due to low memory, `git status` could easily take 10-30 seconds to complete.
    Because `UpdateSystemFacts` does not utilize a `context.Context` with a timeout, and because it runs inside a `k.Transaction()` (which holds the write lock on the kernel), the entire reasoning engine of Codenerd will freeze. Any other goroutine attempting to call `k.Query()` or `k.Assert()` will block waiting for the lock.
    *Conclusion:* The system is highly vulnerable to state conflict deadlocks caused by slow I/O. `gitCmd` must be refactored to use `exec.CommandContext` with a strict timeout (e.g., 2 seconds) to ensure the logic engine remains responsive even if system facts are slightly stale.

2.  **Lock Contention:** If multiple sub-agents are spawned (e.g., a researcher, a coder, and a reviewer) and all are continuously blasting `k.Query()` to orient themselves, the `RWMutex` handles concurrent reads well. However, when an agent asserts a new fact (`k.Assert`), it requires a write lock. If there are 1000 concurrent readers, writer starvation can occur. On a laptop with limited CPU cores (e.g., 4 cores), high lock contention forces the Go runtime scheduler to constantly park and unpark goroutines, wasting CPU cycles on context switching rather than actual work.
    *Conclusion:* The architecture seems adequate for typical agent workloads, but the test suite must prove that writer starvation does not occur under heavy load.

## Final Performance Verdict

For the specific hardware profile (8GB RAM laptop analyzing large codebases):
- **Null/Type Coercion Vectors:** The system is performant enough, provided fallback string allocations are minimized.
- **User Request Extremes:** The system is **NOT** performant enough. `os.ReadFile` and `QueryAll` must be refactored to use streaming/iterators to prevent OOM kills when the AI processes massive codebases.
- **State Conflicts:** The system is vulnerable to I/O-induced deadlocks due to missing timeouts on Git shell commands. This will severely degrade the perceived performance (UI freezing) if not addressed.

## Expanding on Negative Testing Strategies

To truly harden the Kernel Query subsystem against the chaotic environment of autonomous AI agents interacting with vast codebases, our negative testing strategy must go beyond standard fuzzing and structural boundaries. It must incorporate adversarial scenarios mimicking both broken AI logic and hostile system environments.

### 5. Adversarial Input Formulation (Mangle Injection)

Just as SQL injection attacks databases by blurring the line between data and code, "Mangle Injection" can occur if fact strings are constructed dynamically without proper sanitization.

While `ParseFactString` expects a well-formed literal, upstream systems might naively construct strings like:
`fmt.Sprintf("user_input(\"%s\")", untrustedText)`

#### Identified Vulnerabilities & Required Tests
*   **The Unescaped Quote:** If `untrustedText` contains a double quote (`"`), it prematurely closes the string literal. E.g., if input is `"); malicious_fact(/true). //`, the parsed string becomes `user_input(""); malicious_fact(/true). //")`.
    *   **Test:** Supply inputs with embedded quotes, newlines, and period (`.`) characters to `ParseFactString` to ensure the parser either cleanly rejects the malformed syntax or safely encapsulates it.
*   **Atom Injection:** Atoms in Mangle begin with `/`. If an attacker or a hallucinating LLM provides an input intended to be a string but formatted as an atom (`/admin`), and the system uses `ParseFactString` without quotes, the logic engine will treat it as a native atom, potentially bypassing access controls.
    *   **Test:** Verify that passing raw paths (e.g., `/etc/passwd`) as arguments to `ParseFactString` without explicit string quotation causes a parsing error or correctly identifies it as a distinct `NameType`, rather than silently converting it into a string that bypasses path normalization checks.

### 6. Environmental Hostility

The `UpdateSystemFacts` function interacts directly with the host operating system's filesystem and external binaries (`git`). Negative testing must simulate a hostile or broken OS environment.

#### Identified Vulnerabilities & Required Tests
*   **Missing Git Binary:** What happens if the host machine does not have `git` installed, or it's not in the system `PATH`? `exec.Command` will return a specific error (`exec.ErrNotFound`).
    *   **Test:** Temporarily manipulate the `PATH` environment variable in the test suite to exclude `git`. Verify that `UpdateSystemFacts` logs the error gracefully, asserts the `current_time` fact, and completes the transaction without panicking or marking the kernel as corrupted.
*   **Corrupt `.git` Directory:** If the workspace is a git repository, but the `.git` directory is corrupted (e.g., malformed HEAD file), `git status` will exit with a non-zero status code and write an error to stderr.
    *   **Test:** Create a temporary directory, initialize a git repo, corrupt it intentionally, and run `UpdateSystemFacts`. Verify that `gitCmd` captures the stderr output correctly, surfaces it as a Go `error`, and that `UpdateSystemFacts` handles this failure gracefully.
*   **Symlink Loops:** If `workspaceRoot` points to a directory that contains a symlink looping back to itself, and some internal process tries to traverse it (though `UpdateSystemFacts` currently just relies on git), it could cause issues. While `git` handles symlinks safely, `filepath.Abs` or `os.Stat` might behave unexpectedly.
    *   **Test:** Create a symlink loop and set it as `workspaceRoot`. Verify `UpdateSystemFacts` behaves deterministically and does not enter an infinite loop.

### 7. Extreme Concurrency & Memory Pressure (Chaos Testing)

To validate the hypothesis that the system struggles on an 8GB laptop, we must introduce "Chaos Testing" into the CI pipeline.

#### The Chaos Test Protocol
1.  **Memory Constraint:** Use Go's `runtime/debug.SetMemoryLimit` to artificially constrain the test process to a low ceiling (e.g., 256MB).
2.  **Concurrency Bomb:** Spawn 1,000 goroutines that simultaneously attempt to call `QueryAll()` while the kernel is populated with 10,000 facts.
3.  **Assertion Storm:** Concurrently, spawn another 50 goroutines constantly asserting and retracting facts.
4.  **Expected Outcome:** The system *should* slow down dramatically due to GC pressure. However, it must **not** crash with an OOM. If it does, it proves the `QueryAll` architecture (which builds a full map in memory) is fundamentally unsafe for constrained environments and must be rewritten to use an iterator/channel pattern.

### 8. Regression Protection for Logic Evaluation

The core purpose of the kernel is to evaluate `policy.mg` rules. The `Execute` and `Query` functions rely on the underlying Mangle engine's `Evaluate()` method to compute fixpoints.

#### Identified Vulnerabilities & Required Tests
*   **Infinite Derivation Loops:** If an LLM is allowed to modify the `.mg` files or assert new rules dynamically (via an autopoiesis loop), it might create a rule like `foo(X) :- foo(X)`.
    *   **Test:** Assert a known infinite loop rule into the test kernel. Verify that `k.Evaluate()` (which should be called implicitly or explicitly before `Query`) hits a predefined derivation limit (e.g., 10,000 iterations or a 2-second timeout) and returns an error, rather than locking the CPU at 100% forever.
*   **Stratification Failures:** Mangle requires rules to be stratified (no negative cycles, e.g., `p(X) :- not p(X)`).
    *   **Test:** Attempt to load a `.mg` file containing an unstratified logic cycle. Verify that the Mangle analyzer correctly rejects the program during the `Parse` or `Evaluate` phase, and that `kernel_query.go` surfaces this error cleanly to the user.

## Implementation Roadmap for Quality Assurance

To bring the `internal/core` package up to acceptable QA standards for enterprise deployment, the following steps must be taken:

1.  **Immediate Remediation:** Implement the missing unit tests outlined in Section 1-4 directly in `kernel_query_test.go`.
2.  **Refactoring for Safety:**
    *   Modify `UpdateSystemFacts` to accept a `context.Context` and apply strict timeouts to all `gitCmd` executions.
    *   Implement a `QueryStream(predicate string) <-chan Fact` method to provide an OOM-safe alternative to `QueryAll`.
3.  **Adversarial Integration:** Build a dedicated `kernel_chaos_test.go` file that specifically runs the memory-constrained and concurrency-bomb scenarios. This should be run nightly rather than on every PR due to execution time.
4.  **Documentation Update:** Update the internal developer documentation to explicitly warn against using `os.ReadFile` for `.mg` files potentially exceeding 10MB, directing developers to use the new streaming APIs.

By executing this BVA and Negative Testing strategy, the Codenerd kernel will transition from a functional prototype to a resilient, production-grade logic engine capable of safely governing autonomous AI agents on limited hardware.

### 9. Interaction with Virtual Store and Autopoiesis

The Kernel Query subsystem does not exist in a vacuum. It is the central nervous system that feeds the Virtual Store (which executes actions based on kernel permits) and the Autopoiesis loop (which relies on kernel facts to evaluate tool success/failure).

#### Boundary Overlap: Virtual Store Executions
The Virtual Store relies on queries like `permitted(/execute_tool, Args)`. If `Query` has a latency spike or lock contention issue, the Virtual Store stalls.
*   **Test:** Simulate the Virtual Store rapidly querying `permitted` 5000 times a second while background processes are loading new facts. Monitor the latency of `Query`. If P99 latency exceeds 10ms, the system is failing its performance budget.

#### Boundary Overlap: Prompt Evolution Feedback
The Prompt Evolver subsystem queries the kernel for `tool_success` and `tool_failure` facts to score strategies. If `QueryAll` is used to dump the entire history, memory spikes occur precisely when the system needs resources most (during LLM synthesis).
*   **Test:** Generate a history of 50,000 tool execution facts. Verify the Prompt Evolver can query these efficiently using targeted `Query` calls with bound variables (e.g., `tool_success(/tool_x, _)`) rather than relying on `QueryAll` and filtering in Go space.

### 10. API Surface and Contract Guarantees

The `RealKernel` struct implements an interface (implied or explicit) used by other components. We must verify the API contract is honored even under duress.

#### Immutability of Results
When `Query` or `QueryAll` returns a slice of `Fact` structs, those structs contain slices of `interface{}` (`Args`). In Go, slices are references.
*   **Vulnerability:** If the internal store reuses memory or if a caller modifies the returned `Fact.Args` slice (e.g., `result[0].Args[0] = "tampered"`), does it corrupt the kernel's internal state?
*   **Test:** Query a fact, intentionally mutate the returned `Args` slice, and then query the same fact again. The second query must return the original, untampered data. This ensures `Query` is performing a deep copy or that the internal representation is fundamentally isolated from the returned representation.

#### Error Taxonomy
Currently, the system returns generic `fmt.Errorf` strings. This makes programmatic error handling upstream very difficult.
*   **Vulnerability:** The Executor might want to fall back to a different strategy if `Query` fails due to a timeout versus a syntax error. String matching on `err.Error()` is brittle.
*   **Test:** Introduce specific error types (e.g., `ErrKernelUninitialized`, `ErrInvalidPredicateSyntax`, `ErrQueryTimeout`). Write tests to assert that `errors.Is` or `errors.As` works correctly for these specific boundary conditions.

### Final Thoughts on Hardware Constraints

Running an advanced neuro-symbolic system on an 8GB laptop requires aggressive optimization. Go's garbage collector is excellent, but it is not magic. Every string allocation, every slice copy, and every interface conversion adds up. The boundary value analysis reveals that the current `kernel_query.go` implementation prioritizes developer ergonomics (simple `QueryAll` returns, easy `os.ReadFile` loads) over strict resource control.

To meet the high-performance requirement on limited hardware, the transition from "load everything into memory" to "stream and paginate" is not merely an optimization; it is a structural necessity to prevent catastrophic failure modes during extended AI campaigns.

---
*End of QA Boundary Value Analysis Journal.*

### 11. Security and Injection Vectors (Deep Dive)

As the kernel forms the backbone of the agent's permission model, security testing against the query interface is paramount. A compromised or hallucinating LLM could attempt to construct malicious queries to exfiltrate data or bypass safety gates.

#### Malicious Predicate Names
Mangle predicates are expected to be standard identifiers. What happens if an external input is passed directly as a predicate name to `k.Query(input)`?
*   **Vulnerability:** If `input` contains spaces, newlines, or Mangle keywords (e.g., `Decl`, `:-`), the parser might break, or worse, execute unintended logic if the query string is concatenated without sanitization.
*   **Test:** Call `k.Query("user_intent(X, Y) :- true")`. The system expects a predicate name or a pattern, not a rule declaration. Ensure the parser strictly rejects this and does not inadvertently add a rule to the EDB or crash the engine.

#### Type Spoofing
If a fact is asserted with an integer `1`, and an attacker queries with the string `"1"`, they should not match. But what if the attacker uses Go's reflection or custom types to bypass the `switch` statement in `baseTermToValue` or `ExtractString`?
*   **Vulnerability:** Custom types implementing `fmt.Stringer` might trick the fallback logic into returning a string that matches an atom or another string, leading to false positives in permission checks.
*   **Test:** Define a custom Go struct `type SneakyString struct { s string }` with a `String()` method. Assert a fact using `SneakyString{"/admin"}` and query for the atom `/admin`. Ensure they do not evaluate as equal, maintaining strict type boundaries.

### 12. Robustness Under System Resource Starvation

Beyond just memory limits, the 8GB laptop scenario implies other resource constraints: limited file descriptors, slow disk I/O, and limited CPU scheduling.

#### File Descriptor Exhaustion
`LoadFactsFromFile` opens a file. While `os.ReadFile` handles closing it automatically, what happens if the system is under extreme load and runs out of file descriptors before the call?
*   **Test:** Use `ulimit` or Go's `syscall` package in a test environment to reduce available file descriptors to 1. Call `LoadFactsFromFile`. It should fail with a clean "too many open files" error, not a panic.

#### CPU Starvation and Goroutine Scheduling
If the system is maxed out at 100% CPU, the Go scheduler may delay execution of the `UpdateSystemFacts` goroutine.
*   **Test:** Create a "CPU hog" test that spins up `runtime.NumCPU()` goroutines performing infinite math loops. Simultaneously, attempt to run a kernel query and a system update. Measure the latency. If the lock mechanisms are unfair or rely on tight spinning, the system might completely lock up under high CPU load. The RWMutex should handle this, but it must be empirically verified.

### 13. Advanced Data Lifecycle and Garbage Collection

Facts asserted into the kernel take up memory. As an AI agent works over days or weeks, it generates thousands of ephemeral facts.

#### Stale Fact Accumulation (Memory Leaks)
If `user_intent` facts are asserted every turn but never retracted, the kernel's memory footprint will grow monotonically.
*   **Vulnerability:** This is a slow-burn OOM. While not an immediate crash, it violates the long-running stability requirement.
*   **Test:** Write a simulated session loop that asserts 1,000 conversational facts per minute. Monitor memory usage over 100,000 iterations. If memory usage climbs linearly without plateauing, the system is lacking an automated retraction mechanism (e.g., a Time-To-Live (TTL) feature for facts or a periodic cleanup routine).

#### Retraction Consistency
When a fact is retracted, does it actually free the underlying memory in the Mangle `SimpleInMemoryStore`?
*   **Test:** Assert 1,000,000 large string facts. Call `runtime.GC()` and measure memory. Retract all 1,000,000 facts. Call `runtime.GC()` again and measure memory. The memory should return to near-baseline levels. If the store retains hidden pointers (e.g., in an un-compacted index array), this is a hidden memory leak.

### Conclusion of Expanded Analysis

The 8GB RAM constraint drastically shifts the definition of "Performant" from simply "fast" to "highly resource-efficient and stable under duress." The Kernel Query subsystem, as implemented, is functional but naive. It lacks the streaming architectures, strict timeout contexts, and bounded memory controls required to survive the extreme boundary conditions of a fully autonomous coding agent. Implementing the test suite outlined in this journal is the critical first step toward bulletproofing the system.

### 14. Long-Running Stability and Fragmentation

When a system runs for days (like a persistent background AI agent), memory fragmentation becomes as dangerous as raw memory leaks.

#### Heap Fragmentation due to Dynamic Facts
Mangle logic relies on dynamically creating and destroying `ast.Atom` and `Fact` objects. If millions of facts are asserted and retracted over time, Go's memory allocator might fragment the heap.
*   **Vulnerability:** Even if the total live memory is only 100MB, heap fragmentation could cause the OS to allocate 2GB of virtual memory to the process, eventually leading to swapping or an OOM kill on our 8GB target machine.
*   **Test Idea:** Write a long-running "soak test" that randomly asserts and retracts facts of varying string lengths (from 10 bytes to 100KB) for several minutes. Use Go's `runtime.ReadMemStats` to track `Sys` (total memory obtained from OS) vs `Alloc` (bytes allocated and still in use). If the ratio `Sys / Alloc` grows substantially and never recovers, the fact handling logic may need object pooling (e.g., `sync.Pool`) for frequently allocated structures to reduce fragmentation pressure on the garbage collector.

### 15. The "Empty Fact" Edge Case

Logic programming languages have specific semantics for facts with zero arguments (propositional facts).
*   **Vulnerability:** A predicate like `is_raining.` (no arguments) is valid in Mangle. However, Go's `Fact` struct represents this with a nil or empty `Args` slice. Does the `kernel_query.go` serialization and deserialization cleanly handle `len(Args) == 0` without panicking or inserting rogue `nil` values?
*   **Test:** Assert a zero-arity fact `system_ready.` Query for it. Ensure `Query` returns a `Fact` object with `Args` explicitly as an initialized empty slice `[]interface{}{}` rather than a `nil` slice, which might cause downstream consumers iterating over `Args` to panic.

### Final Summary

The `kernel_query.go` subsystem is the literal brain-stem of the Codenerd architecture. Every single perception, decision, and action passes through these query methods. The boundary value analysis provided here proves that while the system handles "happy path" logic elegantly, it requires substantial defensive hardening to survive the realities of constrained hardware (8GB RAM), hostile inputs, and extreme long-running workloads.

The addition of the `TODO: TEST_GAP:` markers in `kernel_query_test.go` serves as a concrete roadmap for engineers to systematically close these vulnerabilities, ensuring the AI agent remains stable, responsive, and secure.

### 16. Edge Cases in Mangle AST Conversion
Mangle allows complex terms in its AST, not just base variables and constants. If future features introduce structured terms (e.g., lists or records), the `baseTermToValue` will fail completely.
* **Test:** Introduce an unsupported AST type deliberately via mock injection to ensure the fallback does not corrupt data structures or panic during `fmt.Sprintf`.

### 17. Null Terminated Strings and Encoding Bugs
If an agent reads a binary file or a corrupt log and passes it to `ParseFactString`, it might contain null bytes (`\x00`).
* **Test:** Pass strings with null bytes and invalid UTF-8 sequences to `ParseFactString`. Ensure the Mangle engine and the Go extraction logic do not truncate strings prematurely or panic during parsing.

---
**Document verified by QA.**

### 18. Large String Constants in Queries
Mangle might have an internal limit on the length of a string literal. If we query for a fact containing a massive 1MB string (e.g., a summarized text block), does the parser fail?
* **Test:** Execute `k.Query(fmt.Sprintf("large_text(\"%s\")", generate1MBString()))`. Ensure it parses without choking on the lexer stage.

### 19. Context Cancelation During Initialization
If `UpdateSystemFacts` is called during kernel boot, and the context is immediately canceled by the user pressing Ctrl+C, what state is left behind?
* **Test:** Pass a canceled context to any system initialization wrappers using `UpdateSystemFacts` and assert the kernel shuts down cleanly.

## Remediation Update - 2026-05-05 14:00 EST

- Status: mostly remediated
- Run journal path: .quality_assurance/remediation/2026-05-05_13-50-00-EST_patch_kernel_query.md
- Branch: patch/remediate-kernel_query-20260505-135000
- Findings remediated: Empty query edge cases, empty parsed fact edgecases, type fallbacks, toctou.
- Tests added: `TestQuery_UninitializedKernel`, `TestQueryAll_ProgramInfoNil`, `TestParseFactString_Empty`, `TestLoadFactsFromFile_Empty`, `TestQuery_EmptyPredicate`, `TestBaseTermToValue_Fallback`, `TestFactMatchesPattern_StringVsName`, `TestQuery_NumericPrecision`, `TestLoadFactsFromFile_TOCTOU`
- Production fixes: Return error for empty `predicate` queries. Return error for empty parses in `ParseFactString`.
- Findings covered already / obsolete / invalid: N/A
- Deferred findings and why:
  - Query handle massive number of arguments - deferred due to underlying Mangle execution behavior being unsafe for standard CI testing limits.
  - QueryAll huge EDBs - huge resource footprint unsafe for unit CI.
  - LoadFactsFromFile 500MB+ - extreme memory.
  - ParseFactString nested recursion - stack overflow vulnerability in Mangle.
  - Concurrent UpdateSystemFacts/Query starvation - inherently flaky under standard test runners.
