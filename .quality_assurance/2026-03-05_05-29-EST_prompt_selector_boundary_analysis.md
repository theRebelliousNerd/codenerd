# Boundary Value Analysis and Negative Testing: internal/prompt/selector.go

## 1. Executive Summary

This document outlines a comprehensive Boundary Value Analysis (BVA) and Negative Testing review for `internal/prompt/selector.go`, specifically focusing on the System 2 Architecture: Skeleton/Flesh Bifurcation for prompt compilation.

The `AtomSelector` component is a critical path for codeNERD's intelligence, responsible for JIT-compiling the exact prompt context sent to the LLM by combining deterministic rules (Skeleton) and semantic search (Flesh). A failure in this subsystem can either cause complete agent breakdown (Skeleton failure) or silent context degradation (Flesh failure).

The review evaluates the resilience of the selector against four major failure vectors:
1.  **Null/Undefined/Empty**: Handling missing contexts, nil slices, and empty properties.
2.  **Type Coercion/Fact Generation**: The bridge between Go types and Mangle facts.
3.  **User Request Extremes**: Massive numbers of atoms, extreme token counts, and huge IDs.
4.  **State Conflicts/Concurrency**: Timeouts, racing context updates, and searcher failures.

---

## 2. Subsystem Overview: AtomSelector

The `AtomSelector` implements a bifurcated approach to prompt atom selection:

*   **Skeleton (Deterministic)**: Categories like `Identity`, `Protocol`, `Safety`, and `Methodology`. These are evaluated strictly via Mangle rules and are *mandatory*. Failure to select these aborts the compilation process (Critical).
*   **Flesh (Probabilistic)**: Categories like `Exemplar`, `Domain`, `Context`, `Language`, `Framework`. These are selected via a combination of Vector Search (semantic similarity) and Mangle rule evaluation. Failure here is degradable; the system proceeds with Skeleton only.

The selection pipeline follows:
1.  **Preparation**: Filter atoms for structured output.
2.  **Phase 1 (Skeleton)**: Build facts, assert to kernel, query `selected_result`, map results.
3.  **Phase 2 (Flesh)**: Vector search (if semantic query exists), build facts + vector hit facts, assert to kernel, query `selected_result` (or fallback context matching), calculate combined scores.
4.  **Phase 3 (Merge)**: Merge Skeleton and Flesh, deduplicate by ID, sort by category/priority/score.

---

## 3. Analysis by Failure Vector

### 3.1. Null / Undefined / Empty Inputs

#### 3.1.1. `cc *CompilationContext` is `nil`
**Current Behavior:**
In `isMangleMandatoryContext`, `cc == nil` is checked safely.
In `mangleMandatoryLimits`, `cc == nil` is checked safely.
In `SelectAtoms`, `cc` is passed to `filterAtomsForStructuredOutput`, `selectMangleMandatoryIDs`, `loadSkeletonAtoms`, and `loadFleshAtoms`.
Inside `buildContextFacts`, `cc.OperationalMode`, `cc.CampaignPhase`, etc., are accessed directly without checking if `cc` is nil.
**Vulnerability (PANIC RISK):** If `cc` is nil, `buildContextFacts` will panic when trying to access `cc.OperationalMode` and calling `cc.WorldStates()`.
*Note: In `SelectAtomsWithTiming`, we see a check `if s.vectorSearcher != nil && cc != nil && cc.SemanticQuery != ""` but `cc` is passed to `loadSkeletonAtoms` unconditionally earlier.*

#### 3.1.2. `atoms []*PromptAtom` contains `nil` elements
**Current Behavior:**
`filterAtomsForStructuredOutput` handles `nil` safely.
`selectMangleMandatoryIDs` handles `nil` `atom` elements safely via `atomHasLanguage(atom, "mangle")` check.
In `loadSkeletonAtoms` and `loadFleshAtoms`, the loops checking `atom.Category` (`isSkeletonCategory(atom.Category)`) will **PANIC** if `atom` is `nil`.
```go
	for _, atom := range atoms {
		if isSkeletonCategory(atom.Category) { // PANIC IF atom == nil
			skeletonAtoms = append(skeletonAtoms, atom)
		}
	}
```
**Vulnerability (PANIC RISK):** A `nil` element in the candidate `atoms` slice will crash the JIT compilation process entirely.

#### 3.1.3. Empty `CompilationContext` Fields
**Current Behavior:**
Fields like `cc.OperationalMode`, `cc.IntentVerb`, etc., could be empty strings.
`buildContextFacts` defines `addContextFact(dim, val)` which checks `if val != ""` and avoids adding facts.
This is safe and prevents malformed Mangle facts like `current_context(/mode, //)`.

#### 3.1.4. Missing Kernel or VectorSearcher
**Current Behavior:**
`loadSkeletonAtoms` checks `s.kernel == nil` and returns an error immediately. This is correct as Skeleton is critical.
`loadFleshAtoms` checks `s.kernel == nil` and falls back to `fallbackFleshSelection`. This is graceful.
`getVectorScores` checks `s.vectorSearcher == nil` and returns `nil, nil`. Safe.

### 3.2. Type Coercion & Fact Formatting

#### 3.2.1. Atom ID String Escaping for Mangle
**Current Behavior:**
`mangleQuoteString(id)` is used to escape atom IDs when building facts. The function correctly handles standard escapes (`\"`, `\\`, `\n`, `\t`) and converts unprintable characters to `\xHH` or `\u{HHHH}`.
**Vulnerability (Low):** If an ID somehow contains a valid quote or backslash that bypasses normal flow, `mangleQuoteString` appears robust enough to wrap it cleanly.

#### 3.2.2. Category and Dimension Normalization
**Current Behavior:**
`mangleNormalizeNameConst` ensures values start with `/` and replaces invalid characters with `_`.
`extractStringArg` safely casts `string`, `int`, `int64`, `float64`, `bool`, and `fmt.Stringer` from Mangle `interface{}` returns back to Go strings.
**Robustness:** This handles the gap between Go types and Mangle types very effectively, preventing the "Atom/String Dissonance" issue seen in older codeNERD implementations. However, if a fact argument is fundamentally unexpected (e.g., a complex slice), the default `fmt.Sprintf("%v", v)` will still result in an unusable string, though it won't panic.

### 3.3. User Request Extremes

#### 3.3.1. Massive Number of Candidate Atoms (e.g., 1,000,000+)
**Current Behavior:**
`buildContextFacts` pre-allocates: `facts := make([]interface{}, 0, 15+len(atoms)*15)`.
For 1,000,000 atoms, this allocates a slice capacity of ~15,000,015. While large, this will consume ~120-240MB of RAM just for the interface slice.
The Mangle kernel is then hit with `AssertBatch(facts)`. If the kernel lacks strict memory bounds on fact ingestion, this could trigger an OOM.
The `AtomSelector` does not impose a candidate cap *before* fact generation (except for mandatory selection limits, which only apply to `mangle` language atoms).
**Vulnerability (OOM RISK):** Unbounded candidate atom slices can exhaust memory during fact generation and kernel assertion.

#### 3.3.2. Extreme Token Counts (`atom.TokenCount`)
**Current Behavior:**
`estimateAtomTokens` falls back to `EstimateTokens(atom.Content)` if `TokenCount` is 0.
The mandatory token limit (`mangleMandatoryTokenCap = 900000`) applies to Mangle mandatory selection.
However, during regular selection (`SelectAtoms`), token counts do *not* restrict the total number of atoms selected by the kernel. The final truncation/budgeting happens down the line in the `JITCompiler`.
**Robustness:** The selector delegates final budgeting. However, if an atom lies about its `TokenCount` (e.g., `-1` or `MaxInt`), `mangleMandatoryLimits` and `selectMangleMandatoryIDs` might behave unexpectedly due to integer overflows or logic gaps.

#### 3.3.3. Massive `SemanticQuery` Length
**Current Behavior:**
If `cc.SemanticQuery` is an entire 50MB log file dump, `s.getVectorScores` passes it directly to `vectorSearcher.Search()`.
**Vulnerability (Latency/Resource Exhaustion):** Depending on the `VectorSearcher` implementation, embedding a 50MB string could block the thread, consume massive GPU/CPU resources, or crash the embedding API. The timeout mechanism `context.WithTimeout(ctx, s.vectorSearchTimeout)` (default 10s) provides a safety net, meaning the query will simply timeout and `loadFleshAtoms` will proceed gracefully. This is a very resilient design choice.

### 3.4. State Conflicts & Race Conditions

#### 3.4.1. Shared Kernel Assertions
**Current Behavior:**
`loadSkeletonAtoms` and `loadFleshAtoms` both call `s.kernel.AssertBatch(facts)` and then `s.kernel.Query()`.
If the `KernelQuerier` implementation (typically a `virtualFactStore` wrapping `engine.Engine`) does not isolate facts per-request, concurrent `SelectAtoms` calls from different goroutines could cross-contaminate.
**Analysis:** The `Selector` uses a single `s.kernel`. Facts asserted in `loadSkeletonAtoms` are left in the kernel, and then `loadFleshAtoms` asserts more. If another chat session executes simultaneously, it will read the first session's context facts.
**Vulnerability (Critical Logical Race):** Mangle's standard engine evaluates all facts present. If two `SelectAtoms` run concurrently, they will mix `current_context` facts. One session might get atoms intended for the other's intent. `s.kernel` must be a scoped, per-compilation instance, or `AtomSelector` must use temporal facts/generation IDs to isolate queries.

#### 3.4.2. Vector Search Timeout Fallback
**Current Behavior:**
`getVectorScores` uses `context.WithTimeout`. If it times out, it returns an error. `loadFleshAtoms` logs a warning and proceeds without vector scores.
**Robustness:** Excellent. This prevents a slow embedding API from hanging the entire agent OODA loop. Flesh selection degrades gracefully to pure Mangle context matching.

#### 3.4.3. Context Cancellation
**Current Behavior:**
`ctx` is passed down but mostly used for `getVectorScores`. The Mangle engine evaluation (`s.kernel.Query`) does not explicitly receive `ctx`. If the kernel gets stuck in a recursive loop, canceling the `SelectAtoms` context will not halt the evaluation.
**Vulnerability (Goroutine Leak):** If Mangle evaluation hangs, the goroutine running `SelectAtoms` will block indefinitely, despite context cancellation.

---

## 4. Test Suite Gaps (`internal/prompt/selector_test.go`)

Based on the BVA above, the test suite is missing coverage for the following scenarios, which have been marked with `// TODO: TEST_GAP:` in the code.

1.  **Nil `CompilationContext` in `SelectAtoms`**: Tests must verify the panic behavior (or handle it gracefully) when `cc` is `nil`.
2.  **Nil `PromptAtom` elements in candidate slice**: Tests must verify the panic behavior when the `atoms` slice contains `nil` pointers.
3.  **Massive Number of Flesh Atoms**: A stress test asserting 10,000+ candidate atoms to verify slice capacity and kernel assertion performance/stability.
4.  **Extreme Token Counts**: Verify that negative or massive token counts don't crash integer calculations.
5.  **Massive ID Lengths**: Verify that `mangleQuoteString` and fact assembly handle extremely long atom IDs without truncation or stack overflows.

---

## 5. Recommendations for Improvement

### 5.1. Defensive Checks
*   **Fix `nil` checks:** In `SelectAtoms` and `SelectAtomsWithTiming`, immediately add:
    ```go
    if cc == nil {
        cc = NewCompilationContext() // or return an error
    }
    ```
*   **Filter `nil` atoms:** At the beginning of `SelectAtoms`, explicitly filter out `nil` elements from the candidate slice to prevent panics in category checks.

### 5.2. Memory Bounds
*   **Candidate Cap:** Implement a hard limit on the number of candidate atoms processed (e.g., max 5000). If the corpus exceeds this, apply a pre-filter or heuristic before fact building.
*   **Semantic Query Cap:** Truncate `cc.SemanticQuery` to a reasonable length (e.g., 8192 chars) before passing to `getVectorScores` to protect the embedding service.

### 5.3. Concurrency Safety
*   **Kernel Fact Isolation:** If `s.kernel` is shared across sessions, `SelectAtoms` is inherently unsafe due to cross-contamination of `current_context` facts. The architecture must ensure `s.kernel` is a scoped, ephemeral instance per JIT compilation, OR implement fact retraction (`RetractBatch`) inside a `defer` block to clean up the `current_context` and `atom` facts after selection.

### 5.4. Context Propagation
*   **Kernel Context:** Update `KernelQuerier` interface to accept `context.Context` for `Query` and `AssertBatch` methods, allowing early cancellation of long-running logic evaluations.

---

## 6. Conclusion

The `AtomSelector` demonstrates robust System 2 architecture with its clear Skeleton/Flesh bifurcation and graceful degradation for vector search failures. Its Mangle fact formatting logic is well-hardened against string dissonance.

However, its lack of defensive `nil` pointer checks on the Compilation Context and candidate slice poses immediate crash risks. Furthermore, its interaction with the shared Mangle kernel presents a potential concurrency flaw if not scoped properly at a higher level. Addressing these boundary conditions will make the JIT compiler exceptionally resilient.
---

## Appendix A: Hypothetical Stack Traces and Mitigation Diffs

This section details the exact crash paths identified during the boundary value analysis, demonstrating how the system fails under extreme conditions and providing the concrete code modifications required to secure the logic.

### A.1. Nil Compilation Context Panic

When `SelectAtoms` is called with a `nil` CompilationContext (e.g., during a malformed system initialization sequence), the failure occurs deep within the fact generation phase.

#### Stack Trace:
```text
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x28 pc=0x10d4a7c]

goroutine 42 [running]:
codenerd/internal/prompt.(*AtomSelector).buildContextFacts(0xc0001a2000, 0x0, 0xc0002b4000, 0x2, 0x2, 0xc0002b5000)
        /workspace/codenerd/internal/prompt/selector.go:785 +0x14c
codenerd/internal/prompt.(*AtomSelector).loadSkeletonAtoms(0xc0001a2000, 0xc00010a000, 0xc0002b4000, 0x2, 0x2, 0x0, 0xc0002b5000, ...)
        /workspace/codenerd/internal/prompt/selector.go:420 +0x1a8
codenerd/internal/prompt.(*AtomSelector).SelectAtoms(0xc0001a2000, 0xc00010a000, 0xc0002b4000, 0x2, 0x2, 0x0, 0xc0002b5000, ...)
        /workspace/codenerd/internal/prompt/selector.go:215 +0x1f0
```

#### Vulnerable Code:
```go
// internal/prompt/selector.go:785
func (s *AtomSelector) buildContextFacts(cc *CompilationContext, atoms []*PromptAtom, forcedMandatory map[string]struct{}) ([]interface{}, error) {
    // ...
	addContextFact("mode", cc.OperationalMode) // PANIC HERE if cc is nil
	addContextFact("phase", cc.CampaignPhase)
    // ...
}
```

#### Proposed Fix:
```go
func (s *AtomSelector) buildContextFacts(cc *CompilationContext, atoms []*PromptAtom, forcedMandatory map[string]struct{}) ([]interface{}, error) {
    if cc == nil {
        return nil, fmt.Errorf("CompilationContext is nil")
    }
    // ...
```
And similarly at the entry points (`SelectAtoms`, `SelectAtomsWithTiming`).

### A.2. Nil Prompt Atom Panic

If the `PromptAtom` corpus loader accidentally injects a `nil` pointer into the slice (e.g., due to a malformed YAML parsing error), the categorical bifurcation logic fails immediately.

#### Stack Trace:
```text
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x18 pc=0x10d3e40]

goroutine 45 [running]:
codenerd/internal/prompt.(*AtomSelector).loadSkeletonAtoms(0xc0001a2000, 0xc00010a000, 0xc0002b4000, 0x10, 0x10, 0xc000300000, ...)
        /workspace/codenerd/internal/prompt/selector.go:410 +0xb0
codenerd/internal/prompt.(*AtomSelector).SelectAtomsWithTiming(0xc0001a2000, 0xc00010a000, 0xc0002b4000, 0x10, 0x10, 0xc000300000, ...)
        /workspace/codenerd/internal/prompt/selector.go:275 +0x1b4
```

#### Vulnerable Code:
```go
// internal/prompt/selector.go:408
	for _, atom := range atoms {
		if isSkeletonCategory(atom.Category) { // PANIC HERE if atom is nil
			skeletonAtoms = append(skeletonAtoms, atom)
		}
	}
```

#### Proposed Fix:
At the start of `SelectAtoms`:
```go
    filtered := make([]*PromptAtom, 0, len(atoms))
    for _, atom := range atoms {
        if atom != nil {
            filtered = append(filtered, atom)
        }
    }
    atoms = filtered
```

### A.3. The Fact Contamination Race Condition (Theoretical)

This is a deeper architectural risk. The `AtomSelector` relies on asserting facts into a kernel to derive the `selected_result` set.

```go
    // Assert facts to kernel
	if err := s.kernel.AssertBatch(facts); err != nil {
		return nil, fmt.Errorf("CRITICAL: failed to assert skeleton facts: %w", err)
	}
```

If `s.kernel` points to a globally shared instance (like the primary codeNERD cortex), Session A might be evaluating a "Research" intent while Session B is evaluating a "Coding" intent.

**Timeline:**
1.  Session A asserts `current_context(/intent, /research)`.
2.  Session B asserts `current_context(/intent, /fix)`.
3.  Session A executes `s.kernel.Query("selected_result(...)")`.

**Result:**
The kernel contains *both* intents. The Mangle rules will likely trigger the selection of atoms related to both coding and research, flooding the LLM context with irrelevant information and potentially exceeding the token budget with high-priority but incorrect skeleton atoms.

**Architectural Mitigation:**
The `JITCompiler` must instantiate a fresh, ephemeral `factstore.SimpleInMemoryStore` for every compilation cycle, bind it to a new Mangle engine instance (or differential engine snapshot), and pass *that* specific isolated kernel down to the `AtomSelector`. This pattern ensures absolute isolation.

---

## Appendix B: Vector Search Threshold Tuning and Extremes

The `AtomSelector` allows dynamic weighting of the Vector Search results.

```go
// internal/prompt/selector.go:343
func (s *AtomSelector) SetVectorWeight(weight float64) {
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	s.vectorWeight = weight
}
```

While bounded safely between 0 and 1, edge cases emerge around how `Combined` scores are calculated when `logicScore` (from fallback match) and `vectorScore` interact.

```go
// internal/prompt/selector.go:610
combined := 0.5 + 0.5*vScore // Base 0.5 for context match, plus vector boost
```

If `vScore` is generated via a distance metric instead of a similarity metric (e.g., lower is better, standard in some L2 norm libraries), the logic would inversely penalize accurate matches. The system assumes the `VectorSearcher` normalizes outputs to `0.0 (Irrelevant) -> 1.0 (Exact Match)`.

**Test Strategy:** Add mock vector searchers that intentionally return `vScore = -1.0` or `vScore = 50.0` to ensure `combined` calculation doesn't push atoms arbitrarily to the top of the sort queue, risking context overflow.

---

## Appendix C: Mangle Identifier Escaping Edge Cases

The `mangleNormalizeNameConst` function attempts to clean strings to be valid Mangle identifiers.

```go
// internal/prompt/selector.go:107
func mangleNormalizeNameConst(s string) string {
    // ...
	parts := strings.Split(s, "/")
	var cleaned []string
	for _, p := range parts {
		if p == "" {
			continue
		}
        // ...
	}
	if len(cleaned) == 0 {
		return ""
	}
	return "/" + strings.Join(cleaned, "/")
}
```

If a user intent verb or category is entirely composed of unsupported unicode characters (e.g., `cc.IntentVerb = "🚀"`), the normalization strips all characters. `cleaned` becomes empty, and the function returns `""`.

```go
// internal/prompt/selector.go:792
	addContextFact := func(dim, val string) {
        // ...
			dim = mangleNormalizeNameConst(dim)
			val = mangleNormalizeNameConst(val)
			if dim == "" || val == "" {
				return
			}
			facts = append(facts, "current_context("+dim+", "+val+")")
		}
	}
```

If `val` becomes `""`, the `addContextFact` function silently drops the context. This is highly defensive and prevents syntax errors in the Mangle compilation loop. The agent will gracefully degrade to a more generic prompt compilation, fulfilling the requirement for robust System 2 operation.

## Appendix D: Expanding the BVA Horizon

To thoroughly secure the `AtomSelector` component against negative usage profiles, we need a multifaceted validation strategy that incorporates property-based testing and adversarial inputs alongside our deterministic unit checks.

### D.1 Fuzz Testing the Pipeline

A dedicated `go fuzz` suite should target the ingestion points of the prompt atom pipeline. Specifically, the conversion endpoints between external YAML representations, string-based LLM payloads, and Mangle facts.

1. `mangleQuoteString(s string)`: A fuzzer should feed random byte streams and ensure the resulting string is universally accepted by the Mangle parser without error.
2. `mangleNormalizeNameConst(s string)`: The normalizer must never return a string that violates the Mangle syntax grammar for constants (`/` followed by alphanumeric/underscore). Fuzzing can reveal edge cases around successive special characters.

### D.2 Load and Capacity Modeling

In extreme monorepo scenarios, the JIT compiler will need to process tens of thousands of contextual prompt fragments.

1. **Memory Profiling**: Implement benchmarks that ingest 50,000 synthesized `PromptAtom` structs to measure the footprint of the `AtomSelector`. If `facts := make([]interface{}, ...)` consumes more than 20% of the allocated slice capacity, pre-computation optimizations should be explored.
2. **Evaluation Budgets**: The integration between `AtomSelector` and `mangle.Eval` should enforce strict gas limits (`WithCreatedFactLimit`). A negative test must assert that evaluating a corpus with recursive or explosive rules correctly terminates instead of entering an infinite loop.

### D.3 Inter-Component Contracts

The `CompilationContext` represents a critical contract between the orchestrator layer and the logic layer.
Negative testing must ensure:
1. When `cc.ReservedTokens` is larger than `cc.TokenBudget`, the budget falls back to a safe baseline rather than underflowing to a negative integer.
2. Framework slice sizes (`cc.Frameworks`) and State slice sizes (`cc.WorldStates`) should be bounded. Passing 10,000 strings via the `cc` object should gracefully truncate rather than bloat the fact store to the point of performance degradation.

### D.4 Concurrency Stress Testing

Given the architectural risk identified in Appendix A.3 regarding fact contamination, a robust stress test is required:
```go
func TestAtomSelector_ConcurrentIsolation(t *testing.T) {
    // Spin up 100 concurrent goroutines executing SelectAtoms
    // Provide each with distinct, non-overlapping CompilationContext requirements
    // Assert that the resulting prompt atoms are strictly isolated to their requested contexts
}
```
This test will definitively prove whether the underlying Mangle kernel instance is correctly scoped per-compilation or unsafely shared across the global namespace.

### D.5. Boundary Conditions on Mangle Integration

One area that demands special attention in negative testing is the interaction between Go's typed domain models and Mangle's relatively permissive rule evaluation context.

1. **Schema Mismatches**: A test should deliberately assert facts where `Category` is not a known atom or is an unquoted string and observe how the selector gracefully recovers (or doesn't).
2. **Tokenization Limits**: Verify behavior when an atom's `Content` contains malformed unicode boundaries, to ensure truncation calculations do not split surrogate pairs, leading to invalid utf-8 sequences downstream in the JIT compiler pipeline.

### D.6. Contextual State Transition Fallbacks

Negative testing should validate that when invalid state transitions occur, the subsystem avoids compounding failure loops.

1. **Campaign Phase Extremes**: If `cc.CampaignPhase` contains a novel string like `/assault_sweep_stage_99`, the selector must verify that the lack of corresponding prompt atoms falls back safely to foundational coding prompts.
2. **Missing Operational Mode**: If `cc.OperationalMode` is uninitialized or explicitly an empty string, verify the system does not incorrectly match atoms requiring specific modes (like `/dream`).

### Final Thoughts

By addressing the panics caused by `nil` inputs, adding protective caps to candidate arrays, explicitly scoping the Mangle kernel against race conditions, and expanding our fuzz/load negative testing suite, the `AtomSelector` component will transition from a highly capable subsystem to a rigorously fortified intelligence component capable of supporting the most complex codeNERD deployment scenarios.

This boundary analysis uncovers several blind spots in how standard Go structures interact with logic programming engines under stress. The architectural remedy for these edge cases relies more on explicit contract enforcement (`nil` checking, capability bounds) than complex error handling, ensuring the execution path remains highly performant while maintaining safety boundaries.
