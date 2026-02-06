# QA Journal: Dreamer (Precog Safety) Boundary Value Analysis
Date: 2026-02-05 05:05 EST
Author: Jules (QA Automation Engineer)
Component: internal/core/dreamer.go (Dreamer)

## 1. Executive Summary

The `Dreamer` subsystem serves as the "Precog" safety layer for the codeNERD agent, tasked with simulating actions before they are executed to detect unsafe states (specifically `panic_state`). This analysis focuses on boundary value vectors, negative testing scenarios, and performance limitations.

**Verdict**: The subsystem contains **Critical Architectural Vulnerabilities** regarding performance at scale and concurrency safety. While it handles basic logic correctly, it is liable to cause system-wide lockups on large repositories and potential safety bypasses under high concurrency or malicious input conditions.

## 2. System Analysis

The `Dreamer` relies on:
1.  `SimulateAction`: The entry point for simulation.
2.  `projectEffects`: Deterministic projection of action consequences.
3.  `codeGraphProjections`: Graph-based impact analysis.
4.  `Mangle Kernel`: Logic engine for evaluation.

### Critical Failure Vectors

#### A. Vector: User Request Extremes (Massive Scale)
**Severity: Critical**
**Location**: `codeGraphProjections` (lines 182-259)

The current implementation of `codeGraphProjections` performs a full-table scan of the code knowledge base for *every* file-related action simulation.

```go
// Line 193
defs, err := d.kernel.Query("code_defines")
```

**Analysis**:
*   The `Query("code_defines")` call retrieves *all* `code_defines` facts from the kernel.
*   In a "50 million line monorepo" (User Extreme), this could easily represent 5-10 million facts.
*   The system loads these into a Go slice `[]Fact`, causing massive heap allocation.
*   It then iterates linearly O(N) to find symbols belonging to `req.Target`.
*   If `ActionRequest` is for a file with 0 symbols, we still pay the O(N) cost.

**Secondary Impact**:
```go
// Line 232
callFacts, err := d.kernel.Query("code_calls")
```
*   It performs a *second* full-table scan of the call graph.
*   Call graphs are typically 10x-100x larger than definition tables.
*   In a large repo, this will cause an **Out of Memory (OOM)** crash or a timeout (if `d.config.ToolTimeout` catches it, but the allocation happens in Go runtime which might panic first).

**Mitigation**:
*   The Mangle Kernel MUST support bound queries (e.g., `Query("code_defines", path)`).
*   If the kernel does not support this, the schema design is fundamentally flawed for this use case and requires an index or auxiliary map.

#### B. Vector: Concurrency & State Conflicts
**Severity: High**
**Location**: `SimulateAction` vs `SetKernel`

The `Dreamer` struct is not thread-safe.

```go
type Dreamer struct {
    kernel *RealKernel
}

func (d *Dreamer) SetKernel(kernel *RealKernel) {
    d.kernel = kernel
}
```

**Scenario**:
1.  Goroutine A calls `SimulateAction`.
2.  It checks `d.kernel == nil` (Line 72). Pass.
3.  Goroutine B calls `SetKernel(nil)` (or swaps to a new kernel).
4.  Goroutine A proceeds to `d.kernel.Clone()` (Line 113).
    *   If `d.kernel` became nil, this panics.
    *   If `d.kernel` changed, we might clone Kernel B but then use logic predicated on Kernel A.

**Race Condition**:
*   `SimulateAction` does not hold a read lock on `d.kernel`.
*   `SetKernel` does not hold a write lock.

**Mitigation**:
*   Add `sync.RWMutex` to `Dreamer`.
*   `SimulateAction` must `RLock()`.
*   `SetKernel` must `Lock()`.

#### C. Vector: Null/Undefined/Empty Inputs
**Severity: Medium**
**Location**: `projectEffects` and `criticalPrefix`

**Scenario 1: Empty Target**
*   `req.Target` is `""`.
*   `path` becomes `""` (Line 146).
*   `projected_action` asserted with empty path.
*   `codeGraphProjections` called with `""`.
*   `filepath.Clean("")` returns `"."`.
*   If `code_defines` has definitions for `"."` (unlikely but possible in malformed inputs), it matches.

**Scenario 2: Malformed Strings**
*   `toString` helper:
    ```go
    func toString(arg interface{}) string {
        switch v := arg.(type) {
        case string: return v
        case MangleAtom: return string(v)
        default: return fmt.Sprintf("%v", v)
        }
    }
    ```
*   If the kernel returns a `float64` (e.g., `1.0`), `toString` returns `"1"`.
*   If the kernel returns `nil` (from a bug in Mangle), `toString` returns `"<nil>"`.
*   Comparison `file == path` fails silently or matches incorrectly if path is `"<nil>"`.

#### D. Vector: Type Coercion (Mangle Dissonance)
**Severity: Medium**
**Location**: `SimulateAction` -> `d.evaluateProjection`

Mangle distinguishes between `/atom` and `"string"`.
*   `ActionDeleteFile` projects `/file_missing` (Atom).
*   The policy rule might expect:
    ```mangle
    panic_state(ID, Reason) :- projected_fact(ID, /file_missing, Path).
    ```
*   If the projection code accidentally uses `"file_missing"` (String), the join fails silently.
*   Current code uses `MangleAtom("/file_missing")`. This appears correct, BUT:
    *   `projected_action` uses `string(req.Type)`.
    *   If Mangle policy expects an Atom for action type (e.g., `/delete_file`), but receives `"delete_file"`, the safety check fails open (no panic detected).
    *   `req.Type` is `ActionType` (string alias).
    *   Code: `Args: []interface{}{ actionID, string(req.Type), path }`.
    *   **Risk**: If schema defines `projected_action(ID, ActionType: Atom, Target)`, this injection of a String will cause type mismatch and the safety rule will **NEVER FIRE**.

## 3. Detailed Boundary Analysis

### 3.1 Massive Input Simulation
The system is untested against large corpora.

**Hypothesis**: Feeding 100k `code_defines` facts will cause `SimulateAction` to exceed `ToolTimeout` or crash.

**Test Gap**:
*   Need a test that populates the kernel with 100k dummy facts.
*   Measure execution time of `codeGraphProjections`.
*   Assert it is under 100ms. (It will likely fail).

### 3.2 Thread Safety Simulation
**Hypothesis**: Concurrent `SimulateAction` and `SetKernel` will panic.

**Test Gap**:
*   Spawn 100 goroutines calling `SimulateAction`.
*   Spawn 1 goroutine calling `SetKernel` randomly.
*   Expect: No Panic.

### 3.3 Path Traversal / Injection
**Hypothesis**: `criticalPrefix` checks simple string containment.
*   Path: `/home/user/my_project/internal/mangle_stuff.go`.
*   `criticalPrefix` checks `strings.Contains(path, "internal/mangle")`.
*   Result: MATCH.
*   False Positive: User cannot edit their own file if it contains "internal/mangle" in the name.
*   False Negative: `../../internal/mangle` (if not cleaned perfectly).
*   `filepath.Clean` is used in `codeGraphProjections` but `criticalPrefix` uses raw `strings.Contains`.

## 4. Risk Matrix

| Risk ID | Scenario | Likelihood | Impact | Severity |
|---------|----------|------------|--------|----------|
| R-001 | OOM on Large Repo | High | System Crash | Critical |
| R-002 | Race Condition Panic | Medium | Service Outage | High |
| R-003 | False Positive Safety Block | High | User Frustration | Medium |
| R-004 | Type Mismatch Bypass | Medium | Safety Bypass | Critical |

## 5. Improvement Recommendations

### 5.1 Optimization
Refactor `codeGraphProjections` to avoid full table scans.
*   **Option A**: Add an inverted index in Go memory (Symbol -> File).
*   **Option B**: Extend `RealKernel` to support indexed queries.
*   **Option C**: Use Mangle's `Query` with bound arguments if available.

### 5.2 Concurrency
Implement `sync.RWMutex` on the `Dreamer` struct.

### 5.3 Type Safety
Standardize on Atoms for all categorical data.
*   `ActionType` should be projected as `MangleAtom`.
*   Current: `string(req.Type)`.
*   Proposed: `MangleAtom("/" + string(req.Type))`.

### 5.4 Path Logic
Use strict prefix checking for `criticalPrefix`.
*   Instead of `strings.Contains`, use `filepath.Rel` and check for `..`.

## 6. Required Test Coverage (Gaps)

The following tests are missing and must be added:

1.  **Test_Dreamer_Performance_LargeGraph**:
    *   Setup: 50k `code_defines`, 100k `code_calls`.
    *   Action: `SimulateAction` on a file with 50 symbols.
    *   Assert: Duration < 500ms.

2.  **Test_Dreamer_Concurrency_Race**:
    *   Parallel execution of Simulate and SetKernel.

3.  **Test_Dreamer_TypeMismatch_Bypass**:
    *   Setup: Policy expecting Atom for ActionType.
    *   Action: Simulate with current String injection.
    *   Assert: Policy fails to trigger (confirming the bug) or triggers (refuting it).

4.  **Test_Dreamer_Input_Extremes**:
    *   Action: Target = 1MB string.
    *   Action: Target = Empty string.

## 7. Conclusion

The `Dreamer` is functionally sound for small, single-threaded demos but architecturally unfit for production use in large repositories or high-concurrency environments. The O(N) dependency on code graph size is a blocking issue for scaling.

Signed,
Jules
QA Automation Engineer
