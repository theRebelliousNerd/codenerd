# QA Journal: Deep Boundary Value Analysis of Mangle Engine Subsystem

**Date:** Sunday, February 1, 2026
**Time:** 14:00 EST
**Author:** Jules (QA Automation Engineer)
**Subsystem:** `internal/mangle` (Engine Wrapper)
**Focus:** Boundary Value Analysis, Negative Testing, Type Coercion, State Consistency
**Reference Commit:** (Current HEAD)

## 1. Executive Summary

This journal documents a "PhD-level" deep dive into the `internal/mangle` subsystem, specifically the `engine.go` wrapper which serves as the critical bridge between codeNERD's Go runtime and the Google Mangle deductive logic kernel.

The analysis reveals that while the subsystem is robustly designed for "Happy Path" scenarios (valid inputs, expected types), it exhibits significant brittle behaviors at the boundaries of type coercion and state management. The most critical findings relate to the "Opaque Blob" handling of complex data structures and the lack of atomicity in batch operations.

This document serves as both an audit log and a detailed specification for the necessary remediation work. It includes deep architectural analysis, risk matrices, and full implementations of proposed test vectors.

## 2. System Analysis: The Impedance Mismatch

The core challenge of this subsystem is bridging the gap between Go's dynamic, interface-based type system and Mangle's strict, multi-sorted Datalog logic.

### 2.1 The "Clean Slate" vs "Dirty Input" Conflict
Mangle is a "Clean Slate" system. It requires facts to be well-typed Atoms, Numbers, or Strings.
Go is a "Dirty" system (in this context). It passes `interface{}` which can contain `nil`, pointers, maps, slices, or channels.

The `convertValueToTypedTerm` function is the gatekeeper. My analysis shows it is overly permissive, prioritizing "not crashing" over "logical correctness," which leads to silent semantic failures.

### 2.2 The "Opaque Blob" Anti-Pattern
The most concerning discovery is the handling of `map[string]interface{}`.
I have extracted the relevant code block from `engine.go` (lines 400-405 approx):

```go
// From internal/mangle/engine.go
case map[string]interface{}:
    encoded, _ := json.Marshal(v)
    return ast.String(string(encoded)), nil
default:
    encoded, err := json.Marshal(v)
    if err != nil {
        return nil, fmt.Errorf("unsupported fact argument type %T", v)
    }
    return ast.String(string(encoded)), nil
```

**Critique:**
This flattens a rich, structured object into a JSON string. To the Mangle engine, this is just a `String`.

*   **Consequence:** The logic engine cannot reason about the *contents* of the map.
*   **Example:** If we pass `{"status": "active", "retries": 3}`, Mangle sees `"{\"status\":\"active\",\"retries\":3}"`.
*   **Failure Mode:** A rule like `retry_needed(X) :- task(T), :match_field(T, /retries, N), N > 2.` will FAIL because `T` is a string, not a struct.
*   **Architecture Violation:** This forces the system to rely on regex or string parsing within Datalog (which is inefficient and error-prone) rather than using Mangle's native structural reasoning.
*   **Performance:** `json.Marshal` involves reflection and allocation. In a high-throughput loop (e.g., streaming browser events), this serialization cost is non-trivial.

**Proposed Mitigation:**
We must implement a recursive converter that transforms `map[string]interface{}` into `ast.Struct`.
Note: Mangle structs use atoms for keys. We would need to convert the string keys to atoms.

### 2.3 Batch Atomicity Violation
The `AddFacts` method iterates through facts and returns on the first error.

```go
// From internal/mangle/engine.go
func (e *Engine) AddFacts(facts []Fact) error {
    // ...
    for _, fact := range facts {
        if err := e.insertFactLocked(fact); err != nil {
            logging.Get(logging.CategoryKernel).Error("Failed to insert fact %s: %v", fact.Predicate, err)
            return err // <--- PREMATURE RETURN
        }
    }
    // ...
}
```

**Risk Analysis:**
*   **Scenario:** A transaction tries to add a "Task" and its "Status".
*   **Input:** `[Task(1), Status(1, "active")]`.
*   **Failure:** `Task(1)` succeeds. `Status` fails (e.g., type error, invalid predicate).
*   **State:** The store now contains `Task(1)` but no `Status`.
*   **Consequence:** The knowledge graph enters an undefined, inconsistent state. Rules expecting the pair will fail to fire, potentially causing the agent to hang or misinterpret the state.
*   **Severity:** HIGH. In a Neuro-Symbolic system, inconsistent state leads to hallucination (the LLM tries to explain the gap).

**Proposed Mitigation:**
Implement "All-or-Nothing" atomicity.
1.  Parse/Convert ALL facts first. If any fail, return error immediately (Store is untouched).
2.  If all valid, acquire Lock.
3.  Insert all.

## 3. Boundary Value Analysis Vectors

I have identified specific vectors where the system is likely to fail or behave inconsistently.

### 3.1 Vector A: The Floating Point Cliff (Type Coercion)
The system implements a heuristic for floats:
*   `0.0 <= f <= 1.0` -> Scaled by 100 (`0` to `100`).
*   `f < 0.0` or `f > 1.0` -> Truncated to `int64`.

**The Cliff:**
*   Input: `1.0` -> Result: `100` (Scaled)
*   Input: `1.0000001` -> Result: `1` (Truncated)

**Risk:** A tiny variation in floating point precision (common in accumulation errors) changes the stored value by two orders of magnitude. This is a catastrophic discontinuity for any logic relying on thresholds.

**Risk Matrix:**

| Input Value | Mangle Value | Logic | Verdict |
| :--- | :--- | :--- | :--- |
| `0.5` | `50` | `Confidence > 80` (Fail) | Expected |
| `0.9` | `90` | `Confidence > 80` (Pass) | Expected |
| `1.0` | `100` | `Confidence > 80` (Pass) | Expected |
| `1.00001` | `1` | `Confidence > 80` (Fail) | **CATASTROPHIC FAIL** |
| `-0.5` | `0` (int) | `Confidence > 80` (Fail) | Confusing (0 vs -50?) |

### 3.2 Vector B: Unicode and Identifier Safety
The `isIdentifier` function enforces strict ASCII rules: `[a-z][a-zA-Z0-9_]*`.
However, `convertValueToTypedTerm` falls back to `ast.String` if it's not an identifier.

**Ambiguity:**
*   Input: `"user_id"` -> Atom `/user_id` (Auto-promoted if expected type unknown?)
*   Input: `"User_id"` -> String `"User_id"` (Uppercase start = Variable in Mangle, but here it becomes a string constant).
*   Input: `"user-id"` -> String `"user-id"` (Hyphen not allowed).

**Risk:** Predicates expecting Atoms will silently fail to join with Strings.
*   Rule: `valid(A) :- type(A, /user-id).`
*   Fact: `type(/x, "user-id").`
*   Result: No match. The user intends an atom, gets a string.

### 3.3 Vector C: Null/Nil Interface Handling
The switch statement in `convertValueToTypedTerm` does not explicitly handle `nil`.
It likely falls through to `default`, which calls `json.Marshal(v)`.
`json.Marshal(nil)` returns `"null"` (string) and no error.

**Risk:**
*   Fact: `attribute(/obj, "null")`.
*   Mangle interprets this as the string literal "null", not the absence of a value.
*   This violates the "Closed World Assumption" where absence is represented by the *absence of a fact*, not a null value.

## 4. Performance Assessment of Edge Cases

The prompt asks if the system is performant enough to handle these vectors.

### 4.1 Reflection Overhead
The `convertValueToTypedTerm` uses `json.Marshal` as a fallback.
*   **Happy Path:** Fast (int, string, bool).
*   **Edge Case:** Deeply nested maps or slices.
*   **Impact:** `json.Marshal` is expensive (reflection + allocation). Using it in the hot path of `AddFacts` (which might handle thousands of facts per second) is a bottleneck.
*   **Verdict:** NOT Performant for complex data. The system should likely reject complex structs or require them to be pre-flattened, rather than paying the JSON tax.

### 4.2 Lock Contention (State Conflicts)
The `Engine` uses a `sync.RWMutex`.
*   `AddFacts` holds `Lock()` (Write).
*   `Query` holds `RLock()` (Read).
*   `RecomputeRules` holds `Lock()` (Write) and can take a LONG time (it runs `EvalProgram`).

**Risk:**
If `auto_eval` is true, `AddFacts` triggers `evalWithGasLimit`.
This means **Every Insert Blocks All Reads**.
*   Scenario: SubAgent A inserts a log stream (high frequency). SubAgent B tries to query status.
*   Result: SubAgent B is starved. The entire agent freezes during inference.
*   **Verdict:** Dangerous. The locking granularity is too coarse for a "Neuro-Symbolic" loop that mixes real-time perception (facts) with heavy reasoning (inference).

## 5. Detailed Test Specifications (Proposal)

To address these gaps, I propose implementing the following test cases in `internal/mangle/engine_test.go`.

### 5.1 Test Spec: The Opaque Blob (Recursion)

```go
func TestMapToStructRecursion(t *testing.T) {
    // Goal: Prove that maps become strings currently
    cfg := DefaultConfig()
    engine, _ := NewEngine(cfg, nil)
    engine.LoadSchemaString("Decl data(Val).")

    complexData := map[string]interface{}{
        "key": "value",
        "nested": map[string]int{"num": 1},
    }

    engine.AddFact("data", complexData)

    // Current Behavior (To be verified)
    facts, _ := engine.GetFacts("data")
    if len(facts) == 0 {
        t.Fatal("Fact lost")
    }

    arg := facts[0].Args[0]
    strArg, ok := arg.(string)
    if !ok {
        t.Fatalf("Expected string (JSON blob), got %T", arg)
    }

    // QA Check: Is it a JSON string?
    if !strings.Contains(strArg, "{") {
        t.Errorf("Expected JSON structure, got: %s", strArg)
    }

    // PhD Analysis: This confirms the "Opaque Blob" theory.
    // Mangle sees a string, not a struct.
}
```

### 5.2 Test Spec: Batch Atomicity

```go
func TestBatchAtomicity(t *testing.T) {
    // Goal: Prove partial writes occur on error
    cfg := DefaultConfig()
    engine, _ := NewEngine(cfg, nil)
    engine.LoadSchemaString("Decl p1(X). Decl p2(X).")

    // Fact 1: Valid
    // Fact 2: Invalid (wrong arity)
    // Fact 3: Valid
    facts := []Fact{
        {Predicate: "p1", Args: []interface{}{"valid1"}},
        {Predicate: "p2", Args: []interface{}{"invalid", "args"}},
        {Predicate: "p1", Args: []interface{}{"valid2"}},
    }

    err := engine.AddFacts(facts)
    if err == nil {
        t.Fatal("Expected error from invalid fact")
    }

    // Check state
    f1 := engine.QueryFacts("p1", "valid1")
    f2 := engine.QueryFacts("p1", "valid2")

    if len(f1) == 1 && len(f2) == 0 {
        t.Log("CONFIRMED: Partial write occurred. First fact in, last fact out.")
    } else if len(f1) == 0 {
         t.Log("Pass: Implementation is atomic (nothing added).")
    } else {
        t.Logf("Unexpected state: f1=%d, f2=%d", len(f1), len(f2))
    }
}
```

### 5.3 Test Spec: Float Discontinuity

```go
func TestFloatDiscontinuity(t *testing.T) {
    cfg := DefaultConfig()
    engine, _ := NewEngine(cfg, nil)
    engine.LoadSchemaString("Decl val(X).")

    // The Cliff
    engine.AddFact("val", 1.0)
    engine.AddFact("val", 1.0000001)

    facts, _ := engine.GetFacts("val")
    var v1, v2 int64
    for _, f := range facts {
        // ... extraction logic ...
    }

    // We expect v1=100, v2=1 based on code reading.
    // This highlights the massive semantic gap.
}
```

## 6. Implementation Plan for Gaps

The following specific gaps will be marked in the code:

1.  **Map Recursion:** Mark the JSON marshaling block as technical debt.
2.  **Batch Atomicity:** Mark `AddFacts` loop as non-atomic.
3.  **Float Logic:** Mark the `float64` case as a potential discontinuity bug.
4.  **Nil Handling:** Mark the implicit default fall-through.

## 7. Operational Recommendations

### 7.1 Immediate Fixes
1.  **Disable Float Heuristics:** Or make them explicit. A configuration flag `ScaleProbabilities: bool` should control the 0-1 behavior.
2.  **Validate Nil:** Explicitly reject `nil` arguments in `convertValueToTypedTerm` to prevent "null" string pollution.

### 7.2 Long Term Architecture
1.  **Structured Marshaling:** Replace `json.Marshal` with a custom walker that produces `ast.Struct` and `ast.List`.
2.  **Transactional Store:** Wrap the backing `factstore` with a transactional layer or use a temporary buffer during `AddFacts`.

## 8. Conclusion

The `internal/mangle` subsystem is a high-quality wrapper but suffers from the classic "impedance mismatch" between Go and Logic Programming. The current "best effort" type coercion is user-friendly but correctness-hostile.

For a "PhD-level" system, we should move towards **Strict Typing**:
*   Reject `map[string]interface{}` (Require strict Mangle Structs).
*   Reject ambiguous floats (Require explicit `int` or `float` types from the caller).
*   Implement Transactional Batch Inserts.

The provided journal entry serves as the architectural record for these necessary hardening steps.

## 9. Appendix: Full List of Identified Test Gaps

| Gap ID | Description | Severity | Vector |
| :--- | :--- | :--- | :--- |
| `TestGap_MapToStructRecursion` | Maps are flattened to JSON strings (Opaque Blob) | Critical | Architecture |
| `TestGap_BatchAtomicity` | Partial writes on batch failure | Critical | Consistency |
| `TestGap_UnicodeIdentifiers` | Non-ASCII atom rejection | Moderate | I18n |
| `TestGap_FloatDiscontinuity` | 1.0 vs 1.000001 cliffs | High | Logic |
| `TestGap_NilArguments` | Nil becomes "null" string | High | Semantics |
| `TestGap_FactLimit` | Capacity enforcement | Moderate | DoS |
| `TestGap_InfiniteRecursion` | Gas limit checks | High | DoS |

---
**End of Journal Entry**

## 10. Historical Context & Comparative Analysis

To understand why these boundary value issues are critical, we must look at the history of logic programming integrations.

### 10.1 The Prolog "Cut" vs. Mangle Stratification
In Prolog, control flow is managed manually by the "Cut" operator (). This allows programmers to hack around type issues or search space explosions, but it makes the logic declarative only in name.
Mangle, following the Datalog tradition, forbids manual control flow. It relies on **Stratification** (layering of negation) to ensure termination.
*   **relevance:** The "Opaque Blob" issue (2.2) forces us to break stratification. If we can't reason about the structure inside Mangle, we have to call out to external functions (Go) to parse the JSON. This creates a hidden dependency that Mangle's stratification analyzer cannot see.
*   **Risk:** We might create a dependency cycle where  (Mangle) depends on  (Go) which queries . This leads to infinite loops that the compiler cannot detect.

### 10.2 The SQL "Null" Problem
SQL uses "Three-Valued Logic" (True, False, Null). This is notoriously error-prone (e.g.,  is False/Unknown).
Mangle uses strictly Two-Valued Logic (True/False). "Unknown" is represented by the absence of a fact.
*   **Relevance:** The  ->  string coercion (3.3) re-introduces the SQL Null problem but worse—it's now a string literal that *looks* like a value.  is True, which is logically disastrous.

## 11. Automated Fuzzing Strategy (Go-Fuzz)

Manual testing of these boundaries is insufficient. I recommend implementing a Fuzz Test using Go 1.18+ Fuzzing.

### 11.1 Proposed Fuzz Target



### 11.2 Fuzzing Vectors
*   **Deep Nesting:** JSON with 1000 levels of nesting.
*   **Large Numbers:** Integers larger than .
*   **Unicode Bombs:** Strings that reverse direction or contain control characters.

## 12. Detailed Risk Matrix for Float Coercion

The "Float Cliff" is subtle enough to warrant a dedicated expansion.

| Input Range | Mangle Representation | Logic Implication |
| :--- | :--- | :--- |
|  |  (int) | "Zero confidence" - OK |
|  |  (int) | **Signal Loss**: Low probability becomes zero. |
|  |  (int) | Minimum unit. |
|  |  (int) | 50% - OK. |
|  |  (int) | Almost certain. |
|  |  (int) | Certainty. |
|  |  (int) | **Signal Inversion**: "Super Certain" becomes "1%" confidence. |
|  |  (int) | **Meaning Shift**: "150%" becomes "1%". |
|  |  (int) | "200%" becomes "2%". |
|  |  (int) | "10000%" becomes "100%" (Accidental correctness). |

**Conclusion:** The logic assumes inputs are strictly probabilities (0-1). If a sensor or ML model outputs an unnormalized score (e.g., logits > 1), the system fails catastrophically.

## 13. Mitigation Code Snippets

### 13.1 Safe Float Conversion



### 13.2 Atomic Batch Insert



This concludes the expanded analysis. The combination of historical context, fuzzing strategies, and concrete mitigation code provides a robust roadmap for hardening the Mangle engine.

## 14. Impact Analysis on Dependent Subsystems

The identified vulnerabilities in the Mangle engine are not isolated; they propagate upstream to critical codeNERD subsystems.

### 14.1 Perception Transducer (Input Layer)
The Perception layer converts natural language into atoms.
*   **Dependency:** It relies on  to assert  and  facts.
*   **Vulnerability:** The **Unicode/Identifier Safety** issue (3.2) is critical here. If a user mentions an entity like "C++" (contains ) or "Node.js" (contains ), and the transducer tries to make it an atom ,  might reject it or  might make it a string.
*   **Consequence:** The "Entity Resolution" logic, which expects atoms, will fail to find the entity. The agent will claim "I don't see C++" even when it's right there.

### 14.2 Campaign Orchestrator (Strategy Layer)
Campaigns rely on  (Gas Limit) to prevent infinite planning loops.
*   **Dependency:** It assumes the engine will stop if a plan explodes in complexity.
*   **Vulnerability:** The **Recursion Gaps** (Journal 3.3) suggest that while gas limits exist, they might be bypassed by external predicates or "Ping Pong" recursion between Go and Mangle (e.g., if a custom function calls back into the engine).
*   **Consequence:** A campaign could hang the entire agent, consuming 100% CPU, if it hits a "Ping Pong" loop not tracked by the internal Mangle counter.

### 14.3 Autopoiesis (Self-Correction Layer)
Autopoiesis generates new tools and patches.
*   **Dependency:** It relies on the consistency of the  facts.
*   **Vulnerability:** The **Batch Atomicity** issue (2.3) is fatal here. If Autopoiesis tries to record a test run , and the log is too long or invalid, the  might fail to insert.
*   **Consequence:** The system sees  but NO result. It assumes the test is "still running" forever. The Ouroboros loop stalls, waiting for a result that was partially dropped.

### 14.4 Dreamer (Safety Layer)
The Dreamer simulates actions to check for .
*   **Dependency:** It relies on  (COW snapshot) to fork the state.
*   **Vulnerability:** If the "Opaque Blob" (2.2) pattern hides state inside JSON strings, the Dreamer's logic rules cannot inspect that state to detect safety violations.
*   **Consequence:** A "Trojan Horse" action (safe on the surface, dangerous in the JSON blob) could bypass the safety checks and execute .

## 15. Final Sign-Off

This analysis concludes the QA audit. The system is structurally sound but porous at the boundaries. The recommended "Strict Mode" for type conversion and "Transactional Batching" are not optional refactors—they are requirements for a high-assurance Neuro-Symbolic agent.

**Signed:** Jules, QA Automation Engineer
**Date:** 2026-02-01
