# Edge Case Detector Boundary Value Analysis and Negative Testing

Date: 2026-03-01 00:16 EST
Component: `internal/campaign/edge_case_detector.go`
Test File: `internal/campaign/edge_case_detector_test.go`

## Overview
The `EdgeCaseDetector` component is a critical path subsystem in the Campaign orchestration architecture. It is responsible for analyzing target files in a campaign and deciding whether they should be created, extended, modularized, or refactored prior to execution. It achieves this by utilizing code metrics like lines of code, churn rate, cyclomatic complexity, and dependency topology (via Mangle rules) to make these deterministic decisions.

This analysis focuses strictly on boundary value analysis and negative testing. It identifies missing edge cases in the current test suite and assesses the component's performance and robustness in extreme scenarios. The goal of this document is not to find happy path scenarios, but to find where the system fails, acts non-deterministically, or exhibits massive performance degradation.

---

## 1. Null/Undefined/Empty Vectors

### Empty/Nil Contexts
**The Issue:**
The `AnalyzeFiles` method accepts a `context.Context`. The test suite currently does not explicitly check what happens when a canceled context, an expired context (deadline exceeded), or a `context.TODO()` is passed.
While `analyzeFile` does check `ctx.Err()`, it is crucial to verify that the loop breaks or gracefully handles the remaining files.

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_AnalyzeFiles_CanceledContext` that passes a context canceled before the function is even invoked. Assert that the returned decisions list is empty and the error is `context.Canceled`.
- Create a test `TestEdgeCaseDetector_AnalyzeFiles_TimeoutContext` that mimics a heavy payload but with a tight timeout (e.g., 1ms), proving that the function stops analyzing mid-stream without leaking goroutines.
- Currently, `AnalyzeFiles` has `ctx, cancel := context.WithTimeout(ctx, d.config.AnalysisTimeout*time.Duration(len(paths)))`. What happens if `len(paths)` is zero? The duration is 0, which immediately cancels the context. The test `TestEdgeCaseDetector_AnalyzeFiles_NilDependencies` passes because it doesn't enter the loop, but it's an edge case behavior that should be explicitly documented and tested for correctness.

**Performance Assessment:**
The system handles canceled contexts adequately for the outer loop (`case <-ctx.Done():`), but `analyzeFile` can still execute heavy Mangle queries (`kernel.Query`) if the context is canceled *after* the initial `if err := ctx.Err(); err != nil` check. The Mangle queries are synchronous and do not accept a context. Therefore, if a user requests a cancellation during a 10,000-file campaign, the system is NOT performant enough; it will block on the current Mangle query until it finishes, leading to zombie processes and unresponsiveness.

### Nil/Empty Intelligence Reports
**The Issue:**
The test `TestEdgeCaseDetector_AnalyzeFiles_NilDependencies` passes `nil` for `IntelligenceReport`, verifying that the code does not panic. However, it does not verify what happens if `IntelligenceReport` is completely empty (all fields are zero-value, empty maps/slices, e.g., `&IntelligenceReport{}`).

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_AnalyzeFiles_EmptyIntelligence` passing an initialized but empty report.
- The system should seamlessly fall back to default values. If `intel.FileTopology` is empty, the file is assumed to not exist (`Exists: false`), defaulting to `ActionCreate`. The tests need to verify this explicit fallback logic.

**Performance Assessment:**
An empty intelligence report bypasses most of the heavy iteration in `gatherMetrics`, making it extremely fast. The system is performant enough for this specific edge case.

### Empty File Paths
**The Issue:**
What happens if the `paths` slice contains empty strings `""`? The function `detectLanguage` might panic or return "unknown". `filepath.Ext("")` returns `""`. `filepath.Base("")` returns `"."`.

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_EmptyPathString`.
- Send `[]string{""}` into `AnalyzeFiles`.
- Assert how `matchesPath("", "...")` behaves. `strings.HasSuffix("", filepath.Base(""))` translates to `strings.HasSuffix("", ".")` which is false. This prevents panics, but the code still attempts to analyze an empty path, querying the kernel for dependencies on `"."`.

**Performance Assessment:**
While not a performance bottleneck, querying the kernel for dependencies of an empty path is wasted I/O and CPU time. The system should fast-fail or filter out empty paths before attempting any analysis. It is currently inefficient in handling malformed input arrays.

---

## 2. Type Coercion and Data Anomaly Vectors

### Extreme Numerical Limits
**The Issue:**
The system parses numerical limits in `queryComplexity` via `parseNumber`. It handles `int`, `int64`, `float32`, `float64`. Mangle might return facts with unexpected numerical representations, especially if the complexity analysis script goes haywire.

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_TypeCoercion_NaN` simulating a Mangle return value of `math.NaN()`. `parseNumber` will return the `NaN` float. `maxComplexity < NaN` will behave weirdly (always false). `decision.Complexity` becomes `NaN`. Later, `decision.Complexity >= d.config.HighComplexity` will evaluate to false.
- Create a test `TestEdgeCaseDetector_TypeCoercion_Inf` simulating `math.Inf(1)`. `decision.Complexity` becomes `+Inf`. This will permanently trigger `ActionRefactorFirst` for the file, regardless of its actual size.
- What if `parseNumber` receives a massive integer (e.g., `uint64` max)? The `float64` conversion might lose precision, though for complexity metrics, this is acceptable.

**Performance Assessment:**
Type coercion overhead is minimal. The `parseNumber` function uses a type switch, which is highly optimized in Go. The system is performant enough to handle millions of type coercions per second. The issue is logic correctness (NaN poisoning), not raw performance.

### String Coercion and Unrecognized Types
**The Issue:**
`parseArg` extracts arguments from `core.MangleAtom` or falls back to `fmt.Sprintf("%v", v)`. If an unexpected type is returned, it creates a string representation.

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_TypeCoercion_Struct` where Mangle returns a complex nested struct instead of a string or atom. `fmt.Sprintf("%v", v)` will serialize it. If this string is matched against a filepath, it will fail harmlessly, but allocating strings for massive structs is dangerous.

**Performance Assessment:**
`fmt.Sprintf("%v", v)` uses reflection under the hood and allocates a new string. If Mangle erroneously returns 10,000 large structs instead of atoms for dependency links, the system will allocate massive strings for each, causing memory bloat and GC pressure. The system is NOT performant enough to handle type anomalies at scale. It should strict-type check or bound the length of the formatted string.

---

## 3. User Request Extremes

### Extreme File Sizes and Campaign Lengths
**The Issue:**
Consider a brownfield request asking the agent to work on a 50 million line monorepo. The campaign orchestrator might target 10,000 files in a single pass.

**Test Improvement Plan:**
- Create a benchmark/test `BenchmarkEdgeCaseDetector_10000Files`.
- Simulate an intelligence report with 10,000 entries and a kernel mock returning 50,000 facts.
- Evaluate the time taken to process.
- Also test `LineCount = math.MaxInt32`. Will `decision.Complexity = float64(decision.LineCount) / 50.0` overflow float64? No, float64 can hold up to 10^308. But we must test that `DetermineAction` correctly identifies it as `ActionModularize` without integer overflows in other heuristic calculations.

**Performance Assessment:**
The system is **NOT performant enough** for extreme length campaigns.
1. **O(N) Sequential Iteration:** `AnalyzeFiles` iterates over 10,000 files serially.
2. **N+1 Query Problem:** Inside the loop, `analyzeFile` calls `queryDependencies` and `queryComplexity`. Each of these executes a bare `d.kernel.Query("dependency_link")`.
3. **Mangle Query Inefficiency:** If there are 100,000 dependency facts in the world state, `queryDependencies` fetches *all 100,000 facts* and then iterates through them in Go to find matches for the current file (`d.matchesPath(file, path)`). For 10,000 files, this means 10,000 * 100,000 = 1,000,000,000 (1 Billion) loop iterations and string comparisons (`matchesPath`).
4. **Memory Allocation:** The slice `facts` returned by `kernel.Query` is allocated 10,000 times.
This will cause the Edge Case Detector to hang for minutes or hours, exceeding `AnalysisTimeout` and crashing the campaign.

**Architectural Fix:**
The kernel queries must be parameterized, or the logic reversed: query Mangle *once* for all dependencies and all complexities, build an in-memory hash map (`map[string][]string`), and then loop through the files in O(1) lookup time.

### New/Unknown Coding Languages
**The Issue:**
If the user invents a new coding language or uses an obscure one (e.g., `.zig`, `.nim`, `.mojo`, or `.bas` for QBasic), `detectLanguage` defaults to `"unknown"`.

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_UnknownLanguage`. Pass a file `test.mojo`.
- Verify that `suggestSplits` still provides sane suggestions. Currently, `suggestSplits` blindly appends `_types`, `_helpers` before the extension. `test_types.mojo` is generated.
- But what if the language does not allow underscores in filenames? What if the language strictly enforces one class per file matching the filename (like Java)? The suggestions are hardcoded to Golang/TypeScript patterns.

**Performance Assessment:**
The string manipulation for unknown languages is performant. The performance is not degraded. However, the system's *intelligence* degrades to a rigid heuristic that might be actively harmful to the user's extreme request.

### Extreme Number of Dependents (The "God File" Vector)
**The Issue:**
What if the campaign targets a "God File" (e.g., `common/utils.go` or `types/index.ts`) that has 50,000 dependents across the monorepo?

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_GodFile`. Inject a mock kernel that returns 50,000 facts where `file` depends on `path`.
- `decision.Dependents` will append 50,000 strings. `decision.ImpactScore` becomes 50,000.
- Ensure that the logging logic (`logDecisionSummary`) does not attempt to print all 50,000 dependents. The current implementation only logs the count (`highImpactCount`), which is safe.

**Performance Assessment:**
Appending 50,000 strings to a slice causes multiple slice growth reallocations. While Go's append is fast, doing this for multiple "God Files" sequentially will strain the garbage collector. The system is marginally performant, but would benefit from pre-allocating slices if the fact count is known, or just calculating the impact score without storing all 50,000 dependent paths if they are never actually used by the LLM (they are serialized to JSON in `EdgeCaseAnalysis`, so a 50,000-item array will generate a massive JSON context payload, potentially blowing out the LLM token context window).
This is a critical failure vector: The `EdgeCaseAnalysis.FormatForContext` does not print the dependents array, but if the full struct is serialized elsewhere, it's a token limit bomb.

---

## 4. State Conflicts and Race Conditions

### Concurrency and Kernel Re-entrancy
**The Issue:**
`EdgeCaseDetector` holds a reference to `core.RealKernel`. `AnalyzeFiles` currently runs sequentially. If a developer attempts to optimize it by wrapping `analyzeFile` in a goroutine pool (to solve the N+1 performance issue mentioned above), is `d.kernel.Query` thread-safe?

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_Concurrency`. Spawn 100 goroutines calling `AnalyzeFiles` simultaneously with the same EdgeCaseDetector instance.
- Run with `-race`.
- Verify that reading `d.config` and calling `kernel.Query` does not trigger race condition panics.

**Performance Assessment:**
Currently, by forcing serial execution, the system avoids concurrency bugs at the cost of massive performance degradation on large campaigns. If parallelized without verifying Mangle's internal thread safety (Mangle maps and slices are notoriously not thread-safe without mutexes), the system will crash.

### File System vs. Intelligence Mismatch (Stale State)
**The Issue:**
The intelligence report claims a file exists (`intel.FileTopology[path]`), but the file was deleted in the meantime by a background process, the user, or another subagent.

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_StaleIntelligence`.
- Mock the file as existing in the intel report, but ensure it does not exist on disk.
- Currently, `analyzeFile` relies purely on `intel` and does not actually check the filesystem using `os.Stat` or `world.Scanner`. This means it operates on *stale state*.
- If it decides `ActionExtend` based on the stale report, the subsequent orchestrator action will fail (or hallucinate) when it tries to read/write to a deleted file.

**Performance Assessment:**
Relying on cached `intel` is highly performant (O(1) map lookup). However, it sacrifices correctness. Adding an `os.Stat` check would add I/O overhead. For 10,000 files, `os.Stat` takes around 10-50ms total on modern NVMe drives, which is perfectly performant and a necessary trade-off for correctness. The system currently trades crucial safety for negligible performance gains.

### Race Condition in Context Cancellation during Kernel Query
**The Issue:**
`AnalyzeFiles` loops over paths and checks `ctx.Done()`. If the context is canceled exactly as a heavy `kernel.Query` is invoked inside `queryDependencies`, the Mangle engine does not currently respect Go's `context.Context` (as noted in system memories: `The KernelQuerier interface... does not support context.Context`).

**Test Improvement Plan:**
- This is a known architectural gap. The test should mock a blocking kernel query and verify that the system hangs, documenting the failure.
- The fix requires modifying `core.RealKernel` to accept contexts, which is outside the scope of `EdgeCaseDetector`, but the detector must be tested to show how it handles this downstream limitation.

**Performance Assessment:**
The inability to cancel Mangle queries means the system is severely unperformant when dealing with user cancellations or timeouts. A 30-second timeout might trigger, but the active kernel query will continue chewing CPU cycles in the background, leading to resource exhaustion.

---

## 5. Algorithmic and Heuristic Boundary Cases

### Chesterton's Fence Boundary
**The Issue:**
The warning trigger for Chesterton's Fence is `decision.ChurnRate >= d.config.HighChurnRate`. If `HighChurnRate` is set to 0, every file triggers it. If it is negative, it still triggers.

**Test Improvement Plan:**
- Test with `EdgeCaseConfig.HighChurnRate = -1`. The system should ideally sanitize configuration values to prevent zero or negative thresholds from breaking the logic.
- Test `ChurnRate` exactly at the boundary (e.g., `ChurnRate = 10` when `HighChurnRate = 10`). Ensure the `<` vs `<=` operators behave exactly as expected.

### Cyclomatic Complexity Fallback
**The Issue:**
If no complexity data is found, it falls back to `decision.Complexity = float64(decision.LineCount) / 50.0`.
If `LineCount` is 49, complexity is 0.98. If `LineCount` is 0, complexity is 0.
What if `LineCount` is negative? (Impossible via physical lines, but what if symbol density estimation fails and `symbolCount` is somehow corrupted?)

**Test Improvement Plan:**
- Test `decision.LineCount = -100`. Complexity becomes -2.0. This bypasses `ActionRefactorFirst` (which checks `>= HighComplexity`). It is harmless but logically unsound.

**Performance Assessment:**
The fallback heuristic is extremely fast (basic division). Performant enough for all edge cases.

---

## Conclusion & Actionable Directives

The `EdgeCaseDetector` is a logically sound subsystem for small-to-medium campaigns, but its current architecture harbors **O(N*M) algorithmic complexity cliffs** that render it completely non-performant for large monorepos (User Request Extremes).

1. **Immediate Refactor:** Reverse the dependency querying. Do not fetch all facts inside a loop. Fetch facts once, map them, and evaluate in O(1).
2. **Context Propagation:** `analyzeFile` must do everything in its power to check `ctx.Err()` repeatedly.
3. **Stale State Protection:** Introduce `os.Stat` to verify `intel.FileTopology` hasn't been invalidated by external forces.
4. **Token Limits:** Ensure that massive slices (like `Dependents` on God Files) are not serialized into the LLM context, or cap them to a safe limit (e.g., top 10).

## 6. Extended Negative Edge Case Considerations

### Input Mutation During Analysis
**The Issue:**
The `targetPaths` slice or `IntelligenceReport` could be mutated concurrently by the campaign orchestrator while `AnalyzeFiles` is running. Slices and maps in Go are not thread-safe.

**Test Improvement Plan:**
- Create a test `TestEdgeCaseDetector_InputMutation`. Launch a goroutine that modifies `intel.FileTopology` while `AnalyzeFiles` is processing a large list of paths.
- Expected behavior: Go's race detector should catch this if tested properly, but structurally, the detector should perhaps deep-copy necessary data or orchestrators should enforce immutability guarantees.

### Massive Output Structs and Memory Leaks
**The Issue:**
`EdgeCaseAnalysis` holds arrays of file paths (`ModularizeFiles`, `RefactorFiles`, etc.) and the original `Decisions`. For a 100,000 file campaign, this struct will easily consume hundreds of megabytes. More importantly, when it is serialized to JSON (`FormatForContext`), it can cause the host process to OOM (Out of Memory).

**Test Improvement Plan:**
- Write a benchmark `BenchmarkEdgeCaseAnalysis_FormatForContext_100k`. Ensure string builder pre-allocation handles massive string concatenation efficiently without excessive reallocations.

### Extreme Date/Time Churn Hotspots
**The Issue:**
If `GitChurnHotspots` contains future dates or negative epochs, does it affect churn calculation? Currently, the detector only looks at `ChurnRate`.

**Test Improvement Plan:**
- Test with `GitChurnHotspots` holding `ChurnRate = math.MaxInt32`.
- Verify the `CHESTERTON'S FENCE` logging output does not produce a massively negative integer or panic formatting logic.

### Self-Referential Dependencies
**The Issue:**
What if Mangle reports that a file depends on itself (`A depends on A`) or there is a cyclic dependency (`A depends on B, B depends on A`)?

**Test Improvement Plan:**
- Test `queryDependencies` with `fact.Args[0] == fact.Args[2]`.
- The current logic adds `decision.Dependencies = append(decision.Dependencies, imported)`. A file depending on itself will have its own name in `Dependencies` and `Dependents`. This increases `ImpactScore`. An infinite loop won't happen here because it just iterates facts, but `ImpactScore` will be falsely inflated.
- The system should filter out self-dependencies (`file != imported`).

### Unicode/Emoji in File Paths
**The Issue:**
What happens if the file path contains complex Unicode characters or emojis (e.g., `src/🚀/main.go`)?

**Test Improvement Plan:**
- Test `detectLanguage` and `matchesPath` with `src/🚀/main.go`. `filepath.Base()` handles unicode fine. Mangle Strings support unicode, but if atom syntax limits character sets (e.g., only `[a-zA-Z0-9_]`), Mangle might crash when asserting the facts.
- The boundary between Go string capabilities and Mangle atom syntax is a common failure point.

### Symlinks and Abstract Paths
**The Issue:**
Does `EdgeCaseDetector` support symlinks or paths with `..`?

**Test Improvement Plan:**
- `matchesPath` simply checks `candidate == path` or `strings.HasSuffix`. If `path` is `../foo/bar.go` and `candidate` is `src/foo/bar.go`, they won't match.
- Paths should be cleaned/normalized (`filepath.Clean`) before analysis.

## Summary of Extreme Weaknesses
1. **O(N) Fact Iteration:** The system pulls all facts from Mangle to Go to filter them. This is the biggest performance risk for large campaigns.
2. **Missing Input Validation:** Empty strings, zero-length arrays, and nil pointers in nested fields are not sanitized.
3. **Stale State Discrepancy:** The system blindly trusts `IntelligenceReport` for file existence instead of the actual file system, causing a race condition against disk changes.
4. **Context Ignorance:** Mangle queries ignore the execution context, meaning massive operations cannot be cleanly aborted.

## Final Reflection
The `EdgeCaseDetector` requires a substantial architectural shift to be considered performant enough for frontier-level tasks. The current sequential `analyzeFile` loop with O(N) full-database pulls from Mangle will break under the load of real-world large-scale repositories.

Negative testing and boundary value analysis have exposed that the system is only tuned for "Happy Path" small-scale operations (10-100 files). To handle 50M line monorepos, the Mangle queries must be parameterized, the analysis must run concurrently, and string concatenations must be batched or capped.

Furthermore, state conflicts (stale intelligence vs. disk) and context-ignorant blocking queries must be resolved to ensure the campaign orchestrator remains responsive and deterministic under stress.

## 7. Deep Dive: The Mangle / Go Boundary Values
### The "Atom/String Dissonance"
As documented in system memories, Mangle treats `/active` (Atom) and `"active"` (String) as disjoint types.
In `EdgeCaseDetector`, `parseArg` attempts to handle both:
```go
func (d *EdgeCaseDetector) parseArg(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case core.MangleAtom:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
```
**The Issue:**
If a file path in Mangle is stored as an Atom (e.g., `/src/main.go`), it is disjoint from `"src/main.go"`. `string(v)` converts the type, but it preserves the prefix if it was `/`.
When `matchesPath` compares `candidate` (the parsed Mangle string) with `path` (the Go string), it checks `candidate == path` or `strings.HasSuffix`. If `candidate` is `/src/main.go` and `path` is `src/main.go`, `candidate == path` is false, but `strings.HasSuffix("/src/main.go", "main.go")` is true.

**Test Improvement Plan:**
- What if the file is `/main.go`? `strings.HasSuffix("/main.go", "main.go")` is true. But what if it's `foo/main.go` vs `bar/main.go`? The system must ensure that Mangle paths and Go paths are normalized before comparison.
- Test `parseArg` with Mangle Atoms containing `/`. Does `string(v)` output `/src/main.go`? If so, the `matchesPath` logic is brittle and relies on `HasSuffix` to paper over the Atom/String dissonance.

### The "Forgotten Sender" (Goroutine Leaks)
**The Issue:**
If the test or production code uses Mangle streaming APIs and a failure occurs, the un-drained channels will block the engine's goroutines forever. While `kernel.Query` is synchronous and returns `[]core.MangleFact`, if an orchestrator attempts to cancel the context during a long `Query` call (which doesn't accept a context), the orchestrator moves on, but the `Query` continues executing in the background, consuming memory.

**Test Improvement Plan:**
- Write a boundary test proving that `AnalyzeFiles` with a massive timeout and a massive Mangle fact base cannot be cleanly canceled without leaking the background `Query` process.
- The architectural fix is not in `EdgeCaseDetector` but in `core.RealKernel`'s implementation, yet the test must exist in `EdgeCaseDetector` to prove its vulnerability to upstream design flaws.

### Ignoring "Empty" Results
**The Issue:**
`kernel.Query` might return an empty slice and `err == nil`. In logic programming, this means zero tuples satisfied the query.
If `dependency_link` returns zero tuples, `decision.ImpactScore` becomes 0.
However, what if the query failed silently due to a type mismatch in Mangle (e.g., joining an Atom with a String)?
Mangle returns empty results for type mismatches, not errors.

**Test Improvement Plan:**
- The detector assumes an empty result means "no dependencies". But for a 50M line monorepo, a core file having 0 dependencies is highly suspicious.
- Introduce an anomaly detection heuristic: If a known large file has 0 dependencies, flag a warning that the Mangle knowledge graph might be incomplete or suffering from Atom/String dissonance.

## 8. Extreme Input Data Validation

### Malicious File Paths (Directory Traversal)
**The Issue:**
If `targetPaths` contains `../../../etc/passwd` or `C:\Windows\System32\cmd.exe`, `filepath.Ext()` and `filepath.Base()` will process them.

**Test Improvement Plan:**
- Test `analyzeFile` with directory traversal payloads. The detector itself doesn't read or write the files, it just analyzes strings and asks Mangle. But it outputs these paths in `EdgeCaseAnalysis.FormatForContext`.
- If the LLM sees `../../../etc/passwd` as a target, it might generate malicious tool calls. The detector must sanitize or reject paths outside the campaign workspace.

### Massive Complexity Floats
**The Issue:**
If `cyclomatic_complexity` returns `math.MaxFloat64`, `decision.Complexity >= d.config.HighComplexity` will be true. But what if it returns `-math.MaxFloat64`?
A malicious or buggy Mangle rule might derive negative complexity.

**Test Improvement Plan:**
- Test `queryComplexity` with negative floats. The logic handles it mathematically (`-1.0 >= 10.0` is false), but negative complexity is logically impossible.
- The detector should clamp `Complexity` to a minimum of `0.0` to maintain domain integrity.

## 9. Structural System Performance Boundaries

### The 200,000 Token Total Budget Conflict
**The Issue:**
System memory states: "The ContextPager... defaults to a 200,000 token total budget".
If `EdgeCaseAnalysis.FormatForContext()` is appended to the prompt, and it contains 10,000 file names in `NoTestFiles` (the loop breaks at 10, but the `ModularizeFiles`, `RefactorFiles`, and `HighImpactFiles` loops do not have a hard cap).

**Test Improvement Plan:**
- Test `FormatForContext` with 5,000 items in `ModularizeFiles`. The resulting string will be massive, blowing out the 200,000 token budget before the LLM even sees the code.
- Add boundary tests asserting that `FormatForContext` truncates all lists to a maximum of 10-20 items, similar to `NoTestFiles`, to guarantee it never exceeds a safe token budget limit.

### Memory Allocation in Batch Analysis
**The Issue:**
`AnalyzeForCampaign` initializes slices `ModularizeFiles: []string{}`, etc.
For 10,000 files, these slices will grow dynamically, reallocating memory multiple times.

**Test Improvement Plan:**
- In `AnalyzeForCampaign`, the total capacity is known (`len(decisions)`). It is more performant to pre-allocate these slices with a capacity of `len(decisions)` to avoid GC thrashing on extreme campaigns.
- `analysis.ModularizeFiles = make([]string, 0, len(decisions))`
- Add a benchmark to prove the GC reduction for massive arrays.

## Conclusion to Extended Analysis
The `EdgeCaseDetector` is conceptually elegant but practically fragile against extreme inputs, asynchronous state changes, and the inherent quirks of the Mangle engine (Atom vs String, silent empty results). To harden this component for enterprise-scale monorepos, rigorous input validation, context awareness, query parameterization, and strict string-builder capping must be implemented.

## 10. The 2026-02-26_00-06-EST Context Pager Connection

### Extreme Compression vs Edge Cases
**The Issue:**
According to memory, `Campaign Context Pager` defaults to a 200,000 token total budget if initialized with a zero or negative value.
If `EdgeCaseAnalysis` identifies 5,000 files as `ActionModularize`, the string output by `FormatForContext()` will contain 5,000 file names. This single section might consume 10,000 - 30,000 tokens of the budget, leaving little room for the actual source code. The Context Pager's massive state compression will then indiscriminately truncate the context, potentially cutting off the actual target file's content.

**Test Improvement Plan:**
- Introduce a hard cap (e.g., 20) on the number of items printed for *all* categories in `FormatForContext()`, similar to the existing cap on `NoTestFiles`.
- Write a boundary test that generates 5,000 `ModularizeFiles` and asserts that the formatted string length is bounded and contains a "and X more" summary rather than the full list.

### `ResolvePrioritizedCallers` Intersection
**The Issue:**
The system memory notes that `ResolvePrioritizedCallers` in `internal/world/holographic.go` optimizes context gathering by sorting and limiting candidates.
The `EdgeCaseDetector` currently sorts its entire 10,000 decision list:
```go
sort.Slice(decisions, func(i, j int) bool {
    return d.actionPriority(decisions[i].RecommendedAction) > d.actionPriority(decisions[j].RecommendedAction)
})
```
However, `d.actionPriority` does not prioritize within the same action type (e.g., ImpactScore or Complexity). So, 5,000 `ActionModularize` decisions will be sorted randomly relative to each other.

**Test Improvement Plan:**
- Write a test that supplies 100 `ActionModularize` decisions with different `ImpactScore` values. The test should prove that the highest-impact files are sorted to the top.
- Modify the `sort.Slice` logic to break ties using `ImpactScore`, `Complexity`, or `LineCount` to ensure the most critical files are evaluated first by the LLM, aligning with the "Wiring Over Deletion" and `ResolvePrioritizedCallers` philosophy.

### Missing Test Gap: LLM Malformed JSON
**The Issue:**
The memory notes explicit test gaps for the Campaign Decomposer (LLM Total Failure, LLM Malformed JSON).
While `EdgeCaseDetector` does not directly parse LLM JSON (it relies on Mangle facts and `IntelligenceReport`), it *generates* JSON via the `json` tags on its structs.

**Test Improvement Plan:**
- If an orchestrator relies on `json.Marshal(analysis)` to persist campaign state or pass it to another subagent, write a boundary test that injects malformed unicode strings or extreme numeric boundaries (like `math.NaN()`) into `FileDecision` and attempts to `json.Marshal` it. Go's `json` encoder handles `NaN` by either panicking or encoding as an invalid string depending on the version/config. We must test that `EdgeCaseAnalysis` serializes cleanly under all edge-case data scenarios.

## 11. Final Assessment on System Performance for Edge Cases

The `EdgeCaseDetector` is robust for well-formed, medium-scale campaigns but possesses critical O(N) database-query loops, unchecked string formatting loops, and potential NaN/Inf logic poisoning. Its tests lack explicit verification of empty structures, extreme inputs (massive monorepos), and the vital context cancellation propagation. Addressing these gaps, particularly the N+1 Mangle query problem and strict string budgeting in `FormatForContext`, is mandatory before deployment to production environments handling 50-million-line codebases.

## 12. "Chesterton's Fence" Churn Rate Anomalies
### Extreme Commits Over Short Time
**The Issue:**
The `EdgeCaseDetector` triggers Chesterton's Fence warnings when `ChurnRate >= d.config.HighChurnRate`.
If a file has 50 commits, but they were all made in the last 10 minutes by an automated refactoring script (e.g., `gofmt`), does it warrant a high-risk warning?
The system currently treats a `ChurnRate` of 10 over 10 years the same as a `ChurnRate` of 10 over 10 minutes.

**Test Improvement Plan:**
- This is an algorithmic boundary limit. The test `TestEdgeCaseDetector_DetermineAction_Logic` asserts that high churn leads to `ActionModularize` or `ActionRefactorFirst`. However, it should be recognized in tests that churn without a temporal boundary can lead to false positives. A future iteration of `GitChurn` might include a timestamp, and tests should be ready to evaluate the velocity of churn.

### Negative Impact Score
**The Issue:**
Can `ImpactScore` be negative? It is calculated as `len(decision.Dependents)`. Since lengths are `int` but mathematically `>= 0`, it cannot be negative naturally. However, if external logic ever manipulates it:

**Test Improvement Plan:**
- Write an explicit unit test `TestEdgeCaseDetector_ImpactScoreBounds`. If `len` is zero, it's 0. It can never be negative. It's a positive boundary case. Verify that `d.parseNumber` and `parseArg` do not somehow inject negative arrays or panic on massive arrays.
