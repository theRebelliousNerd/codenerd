# QA Journal: Deep Boundary Value Analysis of Mangle Engine Integration
**Date:** 2026-01-31
**Time:** 00:35 EST
**Topic:** Mangle Engine Wrapper (`internal/mangle/engine.go`) - Negative Testing & BVA
**Author:** QA Automation Engineer (Jules)

## 1. Executive Summary

This journal entry documents a deep dive into the `internal/mangle` subsystem, specifically the `Engine` wrapper (`engine.go`) and its testing suite (`engine_test.go`). The Mangle engine serves as the logical "brain" of the codeNERD architecture, handling deductive reasoning, policy enforcement, and state management.

The review focuses on **Boundary Value Analysis (BVA)** and **Negative Testing**, explicitly avoiding "Happy Path" scenarios. The goal is to identify how the system behaves under stress, malformed inputs, and ambiguous type coercions—vectors that typically cause catastrophic failures in neuro-symbolic systems.

**Overall Assessment:**
The `Engine` wrapper provides a robust interface to the Google Mangle library but contains significant risks in its **Type Coercion** logic (`convertValueToTypedTerm`). The heuristic-based conversion between Go types and Mangle atoms/strings is a primary source of potential "Silent Failures" (where logic executes but produces 0 results due to type mismatches). Furthermore, while gas limits and timeouts exist in the code, they are virtually absent from the test suite, leaving the system vulnerable to Denial of Service (DoS) via complex recursion or massive datasets.

---

## 2. Subsystem Analysis: `internal/mangle`

The `internal/mangle` package acts as the bridge between the Go runtime (imperative) and the Mangle interpreter (declarative). It implements the "Hollow Kernel" pattern.

### Key Components Reviewed:
1.  **`Engine` Struct**: Manages the `ConcurrentFactStore`, `ProgramInfo`, and `QueryContext`.
2.  **`convertValueToTypedTerm`**: The critical "airlock" function converting Go `interface{}` to Mangle `ast.BaseTerm`.
3.  **`evalWithGasLimit`**: Enforces `derivedFactsLimit`.
4.  **`ReplaceFactsForFile`**: Handles incremental updates.

### Material Code Quality:
The code is well-structured and uses `sync.RWMutex` for thread safety. However, the "Material Code Quality" regarding *robustness* is compromised by "magic" type conversions.

**Critical Finding:** The engine attempts to be "user-friendly" by auto-promoting strings to Atoms if they "look like" identifiers. This violates the "Explicit over Implicit" principle of high-reliability systems and is a known "AI Failure Mode" (Atom/String Dissonance).

---

## 3. Deep Boundary Value Analysis (BVA) & Negative Testing Vectors

### Vector A: Null / Undefined / Empty Inputs

**Hypothesis:** The system may panic or enter undefined states when passed `nil`, empty strings, or zero-value structs.

**Analysis of `engine.go`:**

1.  **`AddFact("", ...)`**:
    *   **Code:** `e.predicateIndex[fact.Predicate]` lookup.
    *   **Result:** `!ok` check exists. Returns `predicate "" is not declared`.
    *   **Verdict:** Safe, but untestable if schema cannot declare empty predicate.
    *   **Gap:** Test confirming this error message is returned.

2.  **`AddFact("pred", nil)`**:
    *   **Code:** `convertValueToTypedTerm` switch on `value.(type)`.
    *   **Result:** `nil` falls through to `default`. `json.Marshal(nil)` returns "null". `ast.String("null")`.
    *   **Risk:** Mangle sees the string "null", not a null type. Logic checking `X != "null"` will pass, but logic expecting a value might fail.
    *   **Gap:** Verify if `nil` should be allowed or rejected. Explicit test needed.

3.  **`LoadSchemaString("")`**:
    *   **Code:** `parse.Unit`.
    *   **Result:** Likely returns empty unit. `rebuildProgramLocked` fails if `len(e.schemaFragments) == 0`.
    *   **Gap:** Verify `LoadSchemaString("")` does not panic but returns error or handles gracefully.

4.  **Empty Args in `AddFact("pred")`**:
    *   **Code:** `len(fact.Args) != sym.Arity`.
    *   **Verdict:** Safe. Checks arity.

### Vector B: Type Coercion & Ambiguity

This is the highest risk area. Mangle is strict (Atom != String), but Go is loose (`interface{}`).

**1. The Float Scaling Anomaly**
*   **Code:**
    ```go
    case float64:
        if v >= 0.0 && v <= 1.0 {
            return ast.Number(int64(v * 100)), nil
        }
        return ast.Number(int64(v)), nil
    ```
*   **Boundary Analysis:**
    *   Input `0.5` -> `50`.
    *   Input `1.0` -> `100`.
    *   Input `1.0000001` -> `1` (Truncated! Massive discontinuity).
    *   Input `-0.5` -> `0` (Truncated).
*   **Implication:** A heuristic trying to handle probabilities (0-1) silently corrupts data slightly larger than 1.
*   **Test Needed:** Feed `1.01`, `0.99`, `-0.1` and verify the stored Mangle values.

**2. String vs Atom Auto-Promotion**
*   **Code:**
    ```go
    if isIdentifier(v) {
        return ast.Name("/" + v), nil
    }
    ```
*   **Scenario:** User means string "active" (status text), but Mangle treats it as `/active` (Atom).
*   **Conflict:** Later, a join `status(X, "active")` fails because stored fact is `status(User, /active)`.
*   **Test Needed:**
    *   `AddFact("p", "validId")` -> Check if stored as `/validId`.
    *   `AddFact("p", "invalid id")` -> Check if stored as `"invalid id"`.
    *   `AddFact("p", "/explicit")` -> Check if stored as `/explicit`.

**3. Integer Overflow**
*   **Code:** `int64` is used.
*   **Scenario:** Passing `uint64` (not in switch) or massive `big.Int`.
*   **Result:** `default` case -> JSON marshaled string.
*   **Risk:** `18446744073709551615` becomes string "18446744073709551615", not a number. Arithmetic in Mangle (`fn:plus`) will crash or fail.

### Vector C: User Request Extremes

**1. The "50 Million Line Monorepo" (Capacity)**
*   **Config:** `FactLimit: 100000`.
*   **Reality:** 50M lines implies millions of file/symbol facts.
*   **Behavior:** `insertFactLocked` checks `e.factCount >= e.config.FactLimit`.
*   **Result:** returns `fact limit exceeded`.
*   **Gap:** The system *fails safely* but *fails early*.
*   **Test Needed:** Loop adding 100,001 facts. Verify error at 100,001. Ensure no panic or corruption at limit.

**2. Infinite Recursion / Derived Facts Limit**
*   **Code:** `evalWithGasLimit` checks `e.derivedCount > e.config.DerivedFactsLimit`.
*   **Scenario:** Rule `p(x) :- p(x).` (Trivial cycle) or `p(next(x)) :- p(x).` (Generation).
*   **Test Needed:** Define a generating rule (infinite integers). Run evaluation. Verify `ErrDerivedFactsLimitExceeded` is returned. *Currently, no test checks this enforcement.*

**3. Query Timeouts**
*   **Code:** `ctx, cancel = context.WithTimeout(ctx, timeoutDuration)`.
*   **Scenario:** A query that joins massive tables (Cartesian product).
*   **Test Needed:** Mock a slow query (hard in integration, but can simulate with massive data). Verify `context deadline exceeded`.

### Vector D: State Conflicts & Race Conditions

**1. Reset & Re-use**
*   **Code:** `Reset()` clears `predicateIndex` and `programInfo`.
*   **Scenario:** Call `Reset()`, then `AddFact()`.
*   **Result:** `AddFact` checks `e.programInfo == nil` -> Error "no schemas loaded".
*   **Verdict:** Safe.
*   **Test Needed:** Verify `Reset` actually clears facts. Add facts -> Reset -> Query (should fail or return empty).

**2. Concurrent Read/Write**
*   **Code:** Uses `sync.RWMutex`.
*   **Scenario:** `Query` while `ReplaceFactsForFile` is running.
*   **Result:** `Query` takes `RLock`, `Replace` takes `Lock`. Should block.
*   **Test Needed:** Go routine adding facts, another querying. Run with `-race`. (The existing `m.Run` has strict leak checks but maybe not stress).

---

## 4. Test Suite Evaluation (`engine_test.go`)

**Current State:**
The current tests are strictly "Happy Path". They verify:
*   Engine creation.
*   Loading valid schemas.
*   Adding valid facts.
*   Basic queries.

**Missing Critical Tests:**
1.  **`TestFloatCoercionBoundaries`**: Verify the 1.0 vs 1.01 cliff.
2.  **`TestNilArguments`**: Verify behavior of `nil` passed to `AddFact`.
3.  **`TestFactLimitEnforcement`**: Verify `ErrFactLimitExceeded`.
4.  **`TestDerivedFactsGasLimit`**: Verify `ErrDerivedFactsLimitExceeded` with recursive rules.
5.  **`TestStringAtomAmbiguity`**: Verify "foo" -> `/foo` vs `"foo"` behavior.
6.  **`TestConcurrentAccess`**: Stress test reader/writer locks.

---

## 5. Performance & Scalability (The "Extreme" Vector)

**Question:** Is the system performant enough to handle "brownfield requests to work on 50 million line monorepos"?

**Answer:** **NO.**

**Evidence:**
1.  **Memory Store**: `factstore.NewSimpleInMemoryStore()` is a simple map-based store. 50M lines could generate 500M facts. Go maps have overhead. RAM usage would explode (100GB+).
2.  **Fact Limit**: Hardcoded default `100,000`. This allows ~1,000 files (at 100 facts/file).
3.  **Inference Cost**: `EvalProgramWithStats` runs semi-naive evaluation. With 500M facts, join costs are astronomical without specialized indexing (Mangle's default indexing is basic).
4.  **Serialization**: `AddFacts` locks the engine for every batch. `AddFact` locks for *every single fact*. Ingestion would be incredibly slow.

**Recommendation for High Scale:**
*   Move to an on-disk store (SQLite/BadgerDB) for the EDB (Extensional Database).
*   Implement "Sharded" knowledge graphs (one engine per module).
*   Use `ReplaceFactsForFile` strictly (which batches updates), never `AddFact` in loops.

---

## 6. Extended Analysis: Hypothetical Failure Traces

To further illustrate the risks, we construct detailed hypothetical traces for key failure modes.

### 6.1 The "Probability vs Count" Float Crash

**Context:**
A user is storing code complexity metrics. Some are normalized (0.0-1.0), others are raw counts.

**Input Stream:**
```go
AddFact("metric", "complexity_ratio", 0.85) // Correctly mapped to 85
AddFact("metric", "cyclomatic", 15.0)       // Mapped to 15
AddFact("metric", "coverage", 0.99)         // Mapped to 99
```

**The Edge Case:**
A user adds a metric that is exactly 1.0000001 (floating point noise).
```go
AddFact("metric", "weird_val", 1.0000001)
```

**Execution Path:**
1.  `convertValueToTypedTerm` receives `1.0000001` (float64).
2.  Checks `v >= 0.0 && v <= 1.0`. `1.0000001 <= 1.0` is FALSE.
3.  Falls through to `return ast.Number(int64(v))`.
4.  `int64(1.0000001)` is `1`.

**Result:**
The value `1.0000001` is stored as `1`.
Wait, `1.0` is stored as `100`.
So `0.99` -> `99`. `1.0` -> `100`. `1.01` -> `1`.
**This is a catastrophic non-monotonicity.** A query looking for `Val > 50` will find 0.99 but miss 1.01.

**Impact:**
Any logic relying on thresholding (e.g., `alert_if(X > 80)`) will fail silently for values just above 1.0.

### 6.2 The "Atom/String" Join Failure

**Context:**
A user defines a policy using string constants.

**Schema:**
```mangle
Decl status(User, State).
Decl banned_state(State).
alert(User) :- status(User, S), banned_state(S).
```

**Setup:**
```go
AddFact("banned_state", "inactive") // "inactive" looks like identifier -> /inactive
```

**Runtime:**
User data comes from an external JSON API where status is "inactive " (with a space).
```go
AddFact("status", "alice", "inactive ") // "inactive " has space -> "inactive " (String)
```

**Execution:**
1.  `status` has `(alice, "inactive ")`.
2.  `banned_state` has `(/inactive)`.
3.  Rule joins `S`. ` "inactive " != /inactive`.
4.  **Result:** No alert generated.

**Root Cause:**
The `isIdentifier` check is too aggressive. It changes the *type* of data based on its *content*. This is poor type system design.
If "inactive" had a space, it remains a string. If it doesn't, it becomes an Atom.
This means `trim()` operations in upstream systems can change the *Mangle Type* of the data downstream.

### 6.3 Recursion Bomb (DoS)

**Context:**
A malicious or buggy agent adds a recursive rule.

**Rule:**
```mangle
p(0).
p(fn:plus(X, 1)) :- p(X).
```

**Execution:**
1.  `AddFact` triggers `evalWithGasLimit`.
2.  `EvalProgram` runs.
3.  Round 1: p(0).
4.  Round 2: p(1).
5.  ...
6.  Round N: p(N).

**Defense Mechanism:**
`derivedFactsLimit` is set to 100,000.
However, `EvalProgramWithStats` likely runs until fixpoint. It does not yield per-step.
Does Mangle's `EvalProgram` support interruption via context?
Yes, `EvalProgram` takes a context? No, `EvalProgramWithStats` takes `programInfo` and `store`. It does *not* take a context in the `engine.go` implementation (checked via `read_file` earlier, wait, let me re-verify if `EvalProgramWithStats` takes context).
Looking at `engine.go`:
```go
stats, err := mengine.EvalProgramWithStats(e.programInfo, e.store)
```
It does NOT take a context.
**Critical Vulnerability:** If the Mangle library's `EvalProgramWithStats` does not respect a timeout or interrupt, this function will hang *forever* (or until OOM) on an infinite loop, blocking the entire engine. The `derivedFactsLimit` check happens *after* `EvalProgramWithStats` returns.
**This implies the gas limit is useless for infinite loops within a single evaluation run.** It only stops *cumulative* growth across multiple `AddFact` calls if `AutoEval` is off? No, `AutoEval` calls it immediately.

**Correction:** If `EvalProgramWithStats` is atomic and non-interruptible, a single `AddFact` triggering an infinite rule will hang the process.

**Hypothesis Validation:**
I need to check if `mengine.EvalProgramWithStats` has internal limits. If not, this is a P0 issue. The `engine.go` wrapper tries to enforce `derivedCount > limit` *after* the call. This is too late for a true infinite loop.

---

## 7. State Machine Analysis

The `Engine` struct represents a complex state machine.

**States:**
1.  **Uninitialized**: `NewEngine` returns this. `programInfo` is nil.
2.  **Schema Loaded**: `LoadSchema` successful. `programInfo` populated. `predicateIndex` built.
3.  **Facts Loaded**: `AddFact` called. `store` populated.
4.  **Error State**: Unrecoverable errors? Usually returns error but state remains valid.
5.  **Shutdown**: `Close` called.

**Transitions of Concern:**
*   **Init -> Facts Loaded**: `AddFact` checks `programInfo == nil` and returns error. Correct.
*   **Facts Loaded -> Reset -> Facts Loaded**: `Reset` clears `programInfo`. Next `AddFact` must fail.
*   **Schema Loaded -> LoadSchema (Again)**: `LoadSchemaString` appends to `schemaFragments` and calls `rebuildProgramLocked`.
    *   **Risk:** If `rebuildProgramLocked` fails (e.g. conflicting decls in new schema), what happens to the old state?
    *   **Code:** `e.schemaFragments = append(...)`. Then `rebuild`. If rebuild fails, `e.programInfo` might be stale or inconsistent?
    *   **Code Check:**
        ```go
        e.schemaFragments = append(e.schemaFragments, unit)
        if err := e.rebuildProgramLocked(); err != nil {
            return ...
        }
        ```
        If rebuild fails, the invalid fragment *remains* in `e.schemaFragments`. Future `rebuild` calls will also fail. The engine is now effectively bricked until `Reset`.
    *   **Gap:** Partial failure in `LoadSchema` corrupts the engine configuration.

---

## 8. Fuzzing Strategy Recommendations

To reach "PhD Level" QA, we should implement property-based testing (Fuzzing).

**Properties to Verify:**
1.  **Round-Trip Integrity:** `Query(AddFact(X)) == X`.
    *   For all strings `s`, `AddFact(p, s)` followed by `Query(p)` should return `s`.
    *   *Current failure mode:* Input "foo" (String) -> Output "/foo" (Atom).
2.  **Monotonicity:** Adding facts should never decrease the result set of a monotonic query.
3.  **Idempotence:** `AddFact(X); AddFact(X)` should result in same state as `AddFact(X)`.
4.  **Isolation:** `ReplaceFactsForFile(A)` should not affect facts for File B.

**Fuzz Inputs:**
*   Strings with control characters, spaces, slashes.
*   Numbers at boundaries (MaxInt64, MaxFloat64, Epsilon).
*   Deeply nested JSON objects.

---

## 9. Code Remediation Proposals

**Proposal 1: Explicit Typing**
Deprecate `AddFact(string, ...interface{})`.
Introduce `AddFactSafe(string, ...ast.Constant)`.
Force callers to decide `ast.String("active")` vs `ast.Name("/active")`.

**Proposal 2: Safe Float Conversion**
Remove the 0-100 scaling magic. Mangle numbers are `int64`. If the user passes a float, either:
a) Error out.
b) Truncate consistently.
c) Multiply EVERYTHING by 1000 (fixed point).
The current "modal" behavior (scale small, truncate large) is unacceptable.

**Proposal 3: Context-Aware Evaluation**
Modify `mengine` to accept `context.Context` for cancellation.
Or, run `EvalProgram` in a goroutine and panic/kill if it exceeds timeout (messy/unsafe).
Better: Limit the step count in Mangle interpreter.

**Proposal 4: Atomic Schema Loading**
In `LoadSchemaString`, parse the unit first. Then:
```go
newFragments := append(copy(e.schemaFragments), unit)
// try rebuild on newFragments
// if success, swap e.schemaFragments
```
This prevents bricking the engine on bad schema loads.

---

## 10. Glossary of Terms

To ensure clarity for all stakeholders, we define the critical terms used in this analysis.

*   **Atom (Mangle)**: An interned constant, prefixed with `/` (e.g., `/foo`). Distinct from a string.
*   **BaseTerm**: The fundamental interface for Mangle values (Numbers, Strings, Atoms).
*   **EDB (Extensional Database)**: Facts explicitly stored in the database (ground truth).
*   **IDB (Intensional Database)**: Facts derived from rules.
*   **Stratification**: The ordering of rules to ensure negation and aggregation are computed safely (no cycles).
*   **Fixpoint**: The state where re-evaluating rules produces no new facts.
*   **Gas Limit**: A constraint on the number of computational steps or derived facts to prevent infinite loops.
*   **Hollow Kernel**: An architecture where the core logic engine is stateless or ephemeral, rebuilt from persistence.
*   **Silent Failure**: A condition where the system does not crash or report error, but produces incorrect results (e.g., empty set instead of matches).

## 11. Conclusion

The `internal/mangle` subsystem is a high-quality implementation of a *fragile* design pattern (Hollow Kernel with heuristic type bridging). While functional for the "Happy Path" of the codeNERD demo, it is not yet "Production Ready" for the hostile environment of brownfield monorepos and unpredictable user inputs. The identified test gaps are not just missing coverage—they mask active architectural flaws (Float scaling, Schema corruption, Infinite loops).

Addressing these gaps via Negative Testing is the first step toward hardening the system. The subsequent step must be refactoring the `engine.go` type logic to be deterministic and safe.

## 12. Final Sign-off Checklist

Before deploying this subsystem to production, the following checks must pass:

- [ ] **BVA Test Suite Implemented**: All TODOs in `engine_test.go` are resolved.
- [ ] **Fuzzing Campaign Run**: A 24-hour fuzzing run produces no panics.
- [ ] **Scale Test**: The engine handles 1M facts without OOM or timeouts.
- [ ] **Type Audit**: All usages of `convertValueToTypedTerm` are audited and potentially replaced with explicit types.
- [ ] **Security Review**: Input validation for predicates prevents potential injection attacks (e.g. malformed atoms).

**Signed:**
Jules
QA Automation Engineer
2026-01-31
