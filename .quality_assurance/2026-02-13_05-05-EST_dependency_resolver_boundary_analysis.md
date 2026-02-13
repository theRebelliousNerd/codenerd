# QA Journal: Boundary Value Analysis of Dependency Resolver
# Date: 2026-02-13 05:05 EST
# Author: QA Automation Engineer (Jules)
# Target Subsystem: internal/prompt/resolver.go

## 1. Executive Summary

This document presents a comprehensive Boundary Value Analysis (BVA) and Negative Testing strategy for the `DependencyResolver` component within the CodeNerd JIT Prompt Compiler subsystem. The analysis was conducted on 2026-02-13.

The `DependencyResolver` is a critical infrastructure component responsible for ensuring that prompt atoms are assembled in a topologically valid order, respecting `depends_on` constraints, and detecting cyclic dependencies that could lead to infinite recursion or logical paradoxes in the LLM's system prompt.

Our analysis has identified several significant edge cases and potential failure modes, ranging from robust handling of null inputs to performance bottlenecks with massive dependency graphs. The most critical finding is a non-deterministic sorting behavior for unknown atom categories, which violates the core requirement of reproducible prompt generation.

We recommend immediate remediation of the identified gaps, particularly the addition of defensive nil-checks and the implementation of a deterministic fallback sorting mechanism.

## 2. System Architecture & Context

The `DependencyResolver` operates within the `internal/prompt` package, serving as a pipeline stage in the JIT Prompt Compiler (`compiler.go`).

**Data Flow:**
1.  **Input**: A slice of `*ScoredAtom` objects. These atoms have already been selected by the `AtomSelector` based on semantic relevance and Mangle logic.
2.  **Processing**:
    -   **Topological Sort**: Using Kahn's Algorithm to order atoms such that dependencies appear before dependents.
    -   **Cycle Detection**: Using Depth-First Search (DFS) to identify and report circular dependencies.
    -   **Category Sorting**: Grouping atoms by `AtomCategory` to enforce high-level prompt structure (e.g., Identity before Methodology).
3.  **Output**: A slice of `*OrderedAtom` objects, ready for the `TokenBudgetManager`.

**Criticality**:
A failure in this component can lead to:
-   **Panics**: Crashing the entire `nerd` CLI or language server.
-   **Infinite Loops**: Freezing the application during cycle detection.
-   **Malformed Prompts**: Presenting instructions in the wrong order (e.g., using a tool before it is defined), confusing the LLM.
-   **Non-Determinism**: Generating different prompts for the same input, making debugging impossible.

## 3. Algorithmic Analysis

### 3.1. Topological Sort (Kahn's Algorithm)

The implementation uses Kahn's Algorithm, which is an iterative approach to topological sorting.

**Complexity**:
-   **Time**: O(V + E), where V is the number of atoms and E is the number of dependency edges.
-   **Space**: O(V) to store the in-degree map and the queue.

**Analysis**:
Kahn's algorithm is generally robust and efficient. However, the current implementation iterates over the input slice to build the adjacency list.
```go
	// Build in-degree map (count of incoming edges)
	inDegree := make(map[string]int, len(atoms))
    // ...
```
This is safe from recursion limits. However, the initial queue population iterates over the input slice. If the input slice is huge, this is still linear.

**Weakness**:
The cycle detection in Kahn's algorithm (checking `len(result) != len(atoms)`) is efficient but *opaque*. It tells us *that* a cycle exists, but not *where* it is. This forces the use of a separate `DetectCycles` method for error reporting.

### 3.2. Cycle Detection (DFS)

The `DetectCycles` method uses a recursive Depth-First Search.

**Complexity**:
-   **Time**: O(V + E).
-   **Space**: O(V) for the recursion stack in the worst case (a long line of dependencies).

**Analysis**:
Recursion is the Achilles' heel here. Go's stack is dynamic, starting small and growing. However, it is not infinite.
In a "User Extreme" scenario where a user (or an automated fuzzing tool) generates a dependency chain of 100,000 atoms (A->B->C...->Z), the recursion depth will reach 100,000.
While modern Go runtimes handle deep stacks well (GBs of stack), it is still a vector for:
1.  **Stack Overflow** (if limits are enforced).
2.  **Performance Degradation** due to stack growth/copying.
3.  **Memory Exhaustion** if each frame is large.

**Recommendation**:
Rewrite `DetectCycles` to use an iterative DFS with an explicit stack data structure on the heap. This moves the memory pressure from the call stack to the heap, which is much larger and safer.

### 3.3. Category Sorting (Map Iteration)

The `SortByCategory` method uses a map to group atoms.

```go
	// Group by category
	byCategory := make(map[AtomCategory][]*OrderedAtom)
    // ...
	// Append any remaining categories not in standard order
	for _, atoms := range byCategory {
		result = append(result, atoms...)
	}
```

**Analysis**:
Go's map iteration order is **randomized by design**. This is a well-known feature to prevent developers from relying on key order.
However, `SortByCategory` explicitly relies on iteration order for "remaining categories".
If an atom has a category that is *not* in the hardcoded `AllCategories()` list (e.g., a custom category injected by a plugin or a typo in a YAML file), it falls into this loop.
This means that `SortByCategory` is **non-deterministic** for unknown categories.
Run 1: `[Known, CustomA, CustomB]`
Run 2: `[Known, CustomB, CustomA]`

This is a **critical defect** for a system that prides itself on stability and reproducibility.

## 4. Detailed Boundary Value Analysis

We examined the following vectors in detail.

### 4.1. Vector A: Null / Undefined / Empty

#### Case A1: Nil ScoredAtom
**Scenario**: The input slice contains a `nil` pointer.
`atoms := []*ScoredAtom{nil}`
**Code Path**:
```go
	for _, sa := range atoms {
		atomMap[sa.Atom.ID] = sa // PANIC: runtime error: invalid memory address or nil pointer dereference
	}
```
**Impact**: Immediate crash.
**Mitigation**: Filter nil pointers at the start of `Resolve`.

#### Case A2: Nil Atom inside ScoredAtom
**Scenario**: `atoms := []*ScoredAtom{{Atom: nil}}`
**Code Path**:
```go
	for _, sa := range atoms {
		atomMap[sa.Atom.ID] = sa // PANIC
	}
```
**Impact**: Immediate crash.
**Mitigation**: Check `sa.Atom != nil`.

#### Case A3: Empty Dependency String
**Scenario**: `AtomA` depends on `""`.
`DependsOn: []string{""}`
**Code Path**:
```go
		for _, depID := range sa.Atom.DependsOn {
			if _, ok := atomMap[depID]; ok { // map lookup for ""
				inDegree[sa.Atom.ID]++
			}
		}
```
**Analysis**:
If there is no atom with ID `""`, `ok` is false, and it's ignored.
If there *is* an atom with ID `""` (unlikely but possible if validation fails), it establishes a dependency.
**Impact**: Mostly benign, but potentially confusing.
**Mitigation**: Validate that IDs are non-empty.

### 4.2. Vector B: Type Coercion
*Not applicable in Go (statically typed).*

### 4.3. Vector C: User Extremes

#### Case C1: Massive Dependency Chain (The "Snake")
**Scenario**: 10,000 atoms, each depending on the previous one. `A1 -> A2 -> ... -> A10000`.
**Behavior**:
-   `Resolve` (Kahn's): Works fine. Iterative.
-   `DetectCycles` (DFS): Recurses 10,000 times.
**Impact**: Potential stack overflow or high latency.
**Observation**: On a standard machine, Go can handle ~1M recursive calls before OOM, but it's risky.

#### Case C2: Massive Graph (The "Hairball")
**Scenario**: 100,000 atoms, dense dependencies (e.g., everyone depends on everyone).
**Behavior**:
-   `inDegree` map calculation becomes O(V*V) if dense.
-   Sorting becomes slow.
**Impact**: JIT compilation latency spikes, causing UI freeze.
**Mitigation**: Limit max dependencies per atom.

#### Case C3: Duplicate Dependencies
**Scenario**: `AtomA` depends on `["AtomB", "AtomB", "AtomB"]`.
**Code Path**:
```go
		for _, depID := range sa.Atom.DependsOn {
			if _, ok := atomMap[depID]; ok {
				inDegree[sa.Atom.ID]++ // Increments 3 times!
			}
		}
```
**Analysis**:
`inDegree` for `AtomA` becomes 3.
When `AtomB` is processed:
```go
		for _, dependent := range dependents[current.Atom.ID] { // AtomA appears 3 times in list?
			inDegree[dependent.Atom.ID]--
```
If `dependents` map is built using the same loop?
```go
	for _, sa := range atoms {
		for _, depID := range sa.Atom.DependsOn {
            // ...
				dependents[depID] = append(dependents[depID], sa) // Appends 3 times!
		}
	}
```
So `dependents["AtomB"]` contains `[AtomA, AtomA, AtomA]`.
When `AtomB` is popped, we iterate 3 times, decrementing `inDegree` of `AtomA` 3 times.
`3 - 1 - 1 - 1 = 0`.
**Result**: Logic actually holds! It works correctly by accident of symmetry.
**Risk**: Inefficient. Processing 3x more edges than necessary.

### 4.4. Vector D: State Conflicts (Concurrency)

#### Case D1: Non-Deterministic Sorting
**Scenario**: Two atoms, `AtomX` (Cat="Custom1") and `AtomY` (Cat="Custom2").
Neither "Custom1" nor "Custom2" are in `AllCategories()`.
**Code Path**:
```go
	// Append any remaining categories not in standard order
	for _, atoms := range byCategory {
		result = append(result, atoms...)
	}
```
**Behavior**:
Go runtime randomizes map iteration seed.
Run 1: `byCategory` yields "Custom1" then "Custom2". Output: `[X, Y]`.
Run 2: `byCategory` yields "Custom2" then "Custom1". Output: `[Y, X]`.
**Impact**:
If `AtomX` sets a variable that `AtomY` uses, and order matters (even without explicit dependency), the prompt will be flaky.
The LLM will receive different prompts, leading to non-reproducible behavior.
**Mitigation**: Collect keys, sort them, then iterate.

#### Case D2: Shared Mutable State
**Scenario**: `ScoredAtom` pointers are shared across goroutines.
**Analysis**: The `DependencyResolver` does not modify the *content* of atoms, only their order. However, if the caller modifies `DependsOn` while `Resolve` is running...
**Impact**: Data race. `Resolve` assumes exclusive access or immutability.
**Mitigation**: Document thread-safety requirements (Caller must ensure immutability).

## 5. Risk Assessment Matrix

| ID | Risk | Likelihood | Severity | Priority | Mitigation |
|----|------|------------|----------|----------|------------|
| R1 | Panic on Nil Atom | Low | High | P1 | Add nil check |
| R2 | Non-Deterministic Sort | Medium | High | P1 | Implement deterministic sort |
| R3 | Stack Overflow (DFS) | Low | Medium | P2 | Iterative DFS |
| R4 | Cycle Reporting Opaque | Medium | Low | P3 | Return cycle path in error |
| R5 | Duplicate Dep Performance | Low | Low | P4 | Deduplicate dependencies |

## 6. Proposed Mitigation Strategies

### 6.1. Deterministic Sort
```go
func (r *DependencyResolver) SortByCategory(atoms []*OrderedAtom) []*OrderedAtom {
    // ... group by category ...

    // Sort keys for deterministic iteration
    var unknownCats []string
    for cat := range byCategory {
        isKnown := false
        for _, known := range AllCategories() {
            if known == cat { isKnown = true; break }
        }
        if !isKnown {
            unknownCats = append(unknownCats, string(cat))
        }
    }
    sort.Strings(unknownCats)

    // ... standard categories ...

    // ... unknown categories in order ...
    for _, catStr := range unknownCats {
        cat := AtomCategory(catStr)
        if atoms, ok := byCategory[cat]; ok {
            result = append(result, atoms...)
        }
    }
    return result
}
```

### 6.2. Iterative Cycle Detection
Replace the recursive `dfs` closure with an explicit stack loop.
```go
    stack := []string{node}
    for len(stack) > 0 {
        curr := stack[len(stack)-1]
        // ...
    }
```

## 7. Expanded Test Plan

The following test cases should be added to `resolver_test.go`:

1.  **TestResolve_NilSafety**:
    -   Input: `[]*ScoredAtom{nil, {Atom: nil}}`
    -   Expected: Error or graceful skip (ignoring nil). Panic is unacceptable.

2.  **TestSortByCategory_Determinism**:
    -   Input: 100 atoms with random custom categories.
    -   Action: Run `SortByCategory` 100 times.
    -   Assert: Output slice is identical every time.

3.  **TestDetectCycles_DeepChain**:
    -   Input: Chain of 100,000 atoms.
    -   Action: Call `DetectCycles`.
    -   Assert: No panic, returns correct result.

4.  **TestResolve_ErrorFormat**:
    -   Input: Simple cycle A->B->A.
    -   Assert: `err.Error()` contains "A -> B -> A" or "B -> A -> B".

5.  **TestValidate_EmptyDepends**:
    -   Input: Atom with `DependsOn: []string{""}`.
    -   Assert: Warning or Error.

## 8. Conclusion

The `DependencyResolver` is logically sound for the "Happy Path" but fragile at the boundaries. The non-determinism in sorting is a subtle but dangerous flaw that could undermine the reliability of the entire JIT compiler. The potential for panics on nil inputs indicates a lack of defensive coding practices in this module.

By addressing the findings in this journal, we can significantly harden the subsystem against both malicious inputs and accidental misconfiguration, moving closer to the "PhD level" robustness required by CodeNerd.

## 9. Code Walkthrough & Trace Analysis

This section performs a simulated execution trace through critical paths to identify logic flaws not visible in high-level diagrams.

### 9.1. Trace: Panic on Nil Atom in `Resolve`

**File**: `internal/prompt/resolver.go:49`
**Function**: `Resolve(atoms []*ScoredAtom)`

```go
49:	for _, sa := range atoms {
50:		atomMap[sa.Atom.ID] = sa
51:	}
```

**Scenario**:
Caller passes `atoms = []*ScoredAtom{{Atom: nil}}`.
At line 49, `sa` is the struct `&ScoredAtom{Atom: nil}`.
At line 50, `sa.Atom` is `nil`.
Accessing `sa.Atom.ID` triggers a runtime panic: `panic: runtime error: invalid memory address or nil pointer dereference`.

**Implication**:
The JIT compiler does not validate `ScoredAtom` integrity before calling `Resolve`. While `AtomSelector` *should* produce valid atoms, defensive programming dictates that `Resolve` must not crash on invalid input. This is a classic "Garbage In, Crash Out" vulnerability.

### 9.2. Trace: Non-Deterministic Sorting in `SortByCategory`

**File**: `internal/prompt/resolver.go:216`
**Function**: `SortByCategory(atoms []*OrderedAtom)`

```go
216:	// Append any remaining categories not in standard order
217:	for _, atoms := range byCategory {
218:		result = append(result, atoms...)
219:	}
```

**Scenario**:
Input atoms have categories `CatA` and `CatB`, neither in `AllCategories()`.
`byCategory` map contains:
-   `"CatA": [Atom1]`
-   `"CatB": [Atom2]`

**Run 1**:
Go runtime randomizes map iteration.
Iterator starts at `CatA`.
Line 218 appends `Atom1` to `result`.
Iterator moves to `CatB`.
Line 218 appends `Atom2` to `result`.
Final Result: `[Atom1, Atom2]`.

**Run 2**:
Go runtime randomizes map iteration (new seed).
Iterator starts at `CatB`.
Line 218 appends `Atom2` to `result`.
Iterator moves to `CatA`.
Line 218 appends `Atom1` to `result`.
Final Result: `[Atom2, Atom1]`.

**Implication**:
If `Atom1` defines a variable `$X` and `Atom2` uses `$X`, Run 2 fails at LLM inference time because `$X` is used before definition.
Even if independent, the prompt hash changes, invalidating caches unnecessarily.

### 9.3. Trace: Stack Overflow in `DetectCycles`

**File**: `internal/prompt/resolver.go:170`
**Function**: `dfs(node string)`

```go
170:	dfs = func(node string) bool {
171:		color[node] = gray
172:
173:		for _, neighbor := range graph[node] {
            // ...
186:			if color[neighbor] == white {
187:				parent[neighbor] = node
188:				if dfs(neighbor) { // RECURSIVE CALL
189:					return true
190:				}
191:			}
192:		}
```

**Scenario**:
Input is a linear chain: `A->B->C...->Z` (Length N).
`dfs(A)` calls `dfs(B)` calls `dfs(C)` ... calls `dfs(Z)`.
Stack depth = N.

**Implication**:
Each stack frame consumes memory (registers, return address, locals).
If N is sufficiently large (e.g., 100k-1M), the goroutine stack will grow until it hits the runtime limit or OS limit.
While Go's segmented stacks are efficient, they are not infinite.
Furthermore, deeply recursive calls are slower due to stack resizing overhead.

## 10. Future Proofing

To ensure the `DependencyResolver` remains robust as the system scales, we recommend the following architectural improvements:

1.  **Strict Atom Validation Interface**: Introduce a `Validatable` interface that all pipeline stages must respect. Ensure `Resolve` accepts only `[]ValidatableAtom`.
2.  **Configuration-Driven Categories**: Move `AllCategories()` to a configuration file (YAML/JSON) loaded at runtime. This allows dynamic extension of categories without code changes, but requires a strict schema to prevent non-determinism.
3.  **Graph Library Integration**: Instead of ad-hoc graph algorithms, consider integrating a robust graph library (e.g., `gonum/graph`) for cycle detection and topological sorting. This offloads algorithmic complexity to a tested library.
4.  **Fuzz Testing**: Integrate `go-fuzz` or native Go 1.18+ fuzzing to automatically generate random dependency graphs and feed them to `Resolve`. This will catch edge cases (cycles, self-loops, deep chains) that manual testing might miss.
5.  **Telemetry for Cycle Detection**: When a cycle is detected, log the full path and the context (Shard ID, Intent) to a telemetry system. This helps developers identify and fix circular dependencies in the atom corpus quickly.

---
*End of Journal Entry*
