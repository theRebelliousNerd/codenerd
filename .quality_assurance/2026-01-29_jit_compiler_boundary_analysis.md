# QA Journal: Boundary Value Analysis of JIT Prompt Compiler
**Date:** 2026-01-29 05:00 EST
**Author:** Jules (QA Automation Engineer)
**Target System:** `internal/prompt` (JITPromptCompiler)
**Focus:** Negative Testing, Edge Cases, Security Vulnerabilities

## 1. Executive Summary

This journal entry documents a comprehensive boundary value analysis and negative testing review of the **JIT Prompt Compiler** subsystem within the codeNERD architecture. As the central orchestration engine responsible for assembling the "mind" of the agent from atomic components, the JIT Compiler is a high-criticality system. A failure here results in a "lobotomized" agent (empty context) or a "schizophrenic" agent (hallucinated/corrupted context).

Our analysis reveals that while the "Happy Path" is well-architected with System 2 bifurcation (Skeleton/Flesh) and robust caching, the system exhibits significant fragility at the boundaries. Specifically, the interface between the Go runtime and the Mangle logic kernel is vulnerable to injection attacks, type coercion failures, and unhandled edge cases in string processing.

The review identified **5 Critical Gaps** in the current test suite that expose the system to stability and security risks.

## 2. Theoretical Foundations: The Imperative-Declarative Impedance Mismatch

To understand the severity of these gaps, we must first analyze the theoretical underpinnings of the JIT architecture. The system operates by bridging two distinct computational models:

1.  **The Imperative World (Go)**:
    -   State is mutable.
    -   Strings are sequences of bytes.
    -   Control flow is explicit (loops, conditionals).
    -   Types are static but coercible (interface{}).

2.  **The Declarative World (Mangle/Datalog)**:
    -   State is a monotonic fixpoint of deductions.
    -   Atoms (`/foo`) are disjoint from Strings (`"foo"`).
    -   Control flow is implicit (resolution of dependencies).
    -   Logic must be stratified (no negation cycles).

The vulnerabilities identified below primarily arise at the **Transduction Interface**—the point where Go data structures are serialized into Mangle facts. This interface implicitly assumes a "well-behaved" universe where identifiers are clean, strings are safe, and relationships are acyclic. In the real world of brownfield repos and adversarial inputs, this assumption is dangerous.

### Neuro-Symbolic Safety Implications
The "Neuro-Symbolic" promise of codeNERD relies on the Logic Kernel acting as a safety interlock for the LLM. If the Transduction Interface is compromised, the safety interlock fails, allowing the LLM to operate without constitutional constraints.
For example, if an attacker can inject a fact that disables the `/safety` policy via a SQL-injection-like attack in an Atom ID, the agent might be tricked into executing malicious code. This is not just a bug; it is a **Privilege Escalation** vulnerability where the "Creative" LLM bypasses the "Executive" Logic Kernel.

## 3. System Architecture Review

The JIT Prompt Compiler (`internal/prompt/compiler.go`) operates as a pipeline:
1.  **Collection**: Aggregates atoms from multiple sources (Embedded, DB, SPL, Kernel).
2.  **Selection**: Uses `AtomSelector` to filter atoms via Mangle rules and Vector Search.
3.  **Resolution**: Orders atoms and resolves `DependsOn` dependencies.
4.  **Budgeting**: Fits atoms into the token window (`TokenBudgetManager`).
5.  **Assembly**: Concatenates the final string.

The critical interface is **Step 2 (Selection)**, where Go structs are converted into Mangle facts to drive the logic engine. This "Transduction Layer" is the primary source of identified vulnerabilities.

## 4. Gap Analysis: The "Mangle Injection" Vulnerability

**Severity:** Critical
**Vector:** Input Validation / SQL-Injection variant

### The Mechanism
The `AtomSelector.buildContextFacts` method constructs Mangle code using string interpolation (`fmt.Sprintf`).

```go
// internal/prompt/selector.go
facts = append(facts, fmt.Sprintf("atom('%s')", id))
```

This pattern is fundamentally unsafe when `id` is derived from external input or untrusted database content. Mangle's parser expects atoms to be quoted strings or valid identifiers. If an `id` contains a single quote (`'`), it breaks out of the literal.

### Proof of Concept (Hypothetical)
Imagine a malicious or malformed atom ID: `malicious_id') :- dangerous_rule(X). --`

If the ID is `test' )`, the generated fact becomes:
```mangle
atom('test' )')
```
This causes a syntax error in the Mangle parser, aborting the entire compilation process. The agent crashes or falls back to a degraded state.

If the ID is crafted to inject valid Mangle syntax:
`id` = `test'), valid_fact('pwned`
Result:
```mangle
atom('test'), valid_fact('pwned')
```
This allows an attacker (or a bug in the atom DB) to assert arbitrary facts into the kernel, potentially altering the selection logic to exclude safety prompts or include malicious instructions.

### Suggested Test Case (Go)
To verify this, we recommend adding the following test case:

```go
func TestVulnerability_MangleInjection(t *testing.T) {
    // Malicious atom ID designed to break out of single quotes
    maliciousID := "id_with_quote'OR'1'='1"

    atoms := []*PromptAtom{
        {ID: maliciousID, Category: CategoryIdentity, Content: "Pwned"},
    }

    corpus := NewEmbeddedCorpus(atoms)
    // Use a real kernel or a mock that parses the facts
    // For this test, we need to verify the string sent to AssertBatch
    capturingKernel := &CapturingKernel{}

    compiler, _ := NewJITPromptCompiler(WithEmbeddedCorpus(corpus), WithKernel(capturingKernel))
    compiler.Compile(context.Background(), NewCompilationContext())

    // Assert that the generated fact is escaped
    // BAD: atom('id_with_quote'OR'1'='1')
    // GOOD: atom('id_with_quote\'OR\'1\'=\'1')
    for _, fact := range capturingKernel.CapturedFacts {
        if strings.Contains(fact, "atom('id_with_quote'OR'") {
            t.Errorf("VULNERABILITY: Atom ID was not escaped: %s", fact)
        }
    }
}
```

### Recommendation
1.  **Sanitization**: Implement a `EscapeMangleString` function that escapes single quotes and validates input.
2.  **Parameterized Queries**: If the Mangle Go adapter supports binding variables, use that instead of string interpolation.
3.  **Strict Validation**: Reject atom IDs that do not match `^[a-zA-Z0-9_\-\/]+$`.

## 5. Gap Analysis: The "Atom Syntax Breach" (Type Coercion)

**Severity:** High
**Vector:** Type Coercion / Syntax Violation

### The Mechanism
Mangle distinguishes strictly between **Atoms** (interned constants starting with `/`) and **Strings** (quoted text). The code attempts to enforce this via helper functions, but falls short in `buildContextFacts`:

```go
// internal/prompt/selector.go
addContextFact("shard", cc.ShardType)
// ...
if !strings.HasPrefix(val, "/") {
    val = "/" + val
}
facts = append(facts, fmt.Sprintf("current_context(%s, %s)", dim, val))
```

If `cc.ShardType` is `"coder shard"`, the code converts it to `"/coder shard"`.
The generated Mangle code is:
```mangle
current_context(/shard, /coder shard)
```
**This is invalid Mangle syntax.** Atoms cannot contain spaces unless they are quoted or follow specific escaping rules (which vary by implementation). Even if valid, `atom(/coder shard)` is likely interpreted as two tokens or a syntax error.

### The "Babel Fish" Problem
Furthermore, `extractStringArg` blindly converts any type to string:
```go
func extractStringArg(arg interface{}) string {
    // ...
    return fmt.Sprintf("%v", v)
}
```
If the kernel returns a list `[1, 2]`, this returns `"[1 2]"`. If the downstream logic expects a specific format (e.g. splitting by comma), this breaks.
More dangerously, if the kernel returns a `float64` (e.g., `1e10`), string formatting might produce scientific notation, which might not match expected string patterns in downstream logic.

### Suggested Test Case (Go)
```go
func TestVulnerability_AtomSyntaxSpaces(t *testing.T) {
    // Context with spaces
    cc := NewCompilationContext().
        WithShard("/coder", "", "").
        WithIntent("/fix", "complex target with spaces")

    // This should NOT panic or error during compilation
    // Ideally, spaces should be replaced by underscores or quoted
    _, err := compiler.Compile(context.Background(), cc)

    if err != nil && strings.Contains(err.Error(), "syntax error") {
        t.Errorf("VULNERABILITY: Context with spaces caused syntax error: %v", err)
    }
}
```

### Recommendation
1.  **Quote Atoms**: Always quote atoms if they contain non-alphanumeric characters, or reject them.
2.  **Type Assertion**: `extractStringArg` should probably fail or log error if the type is not a string, rather than doing a "best effort" `sprintf` which hides logic errors.

## 6. Gap Analysis: The "Recursive Death Spiral" (Circular Dependencies)

**Severity:** Medium (Potential Hang/Crash)
**Vector:** State Conflict / Graph Cycle

### The Mechanism
The `DependencyResolver` (implied in `internal/prompt/resolver.go`) is responsible for ordering atoms based on `DependsOn`.
If the atom graph contains a cycle:
`A` depends on `B`
`B` depends on `A`

A naive topological sort will either:
1.  Panic.
2.  Loop infinitely.
3.  Fail silently (drop both).

Given the JIT nature of the system, atoms can be added dynamically from the DB. A content editor might accidentally create a loop.

### Theoretical Impact
In a Directed Acyclic Graph (DAG), topological sort is linear time `O(V+E)`. In a cyclic graph, the algorithm is undefined. If the implementation uses recursion without a visited/recursion-stack check, it will trigger a stack overflow. If it uses an iterative Kahn's algorithm, it will terminate but produce an incomplete list (nodes in cycle are never processed). The latter is "safer" but leads to silent dropping of critical context.

### Suggested Test Case (Go)
```go
func TestVulnerability_CircularDependencies(t *testing.T) {
    atoms := []*PromptAtom{
        {ID: "A", DependsOn: []string{"B"}},
        {ID: "B", DependsOn: []string{"A"}},
    }
    corpus := NewEmbeddedCorpus(atoms)
    compiler, _ := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))

    // Should return error or break cycle, NOT hang/panic
    done := make(chan bool)
    go func() {
        compiler.Compile(context.Background(), NewCompilationContext())
        done <- true
    }()

    select {
    case <-done:
        // Success (or safe failure)
    case <-time.After(1 * time.Second):
        t.Fatal("VULNERABILITY: Compiler hung on circular dependency")
    }
}
```

### Recommendation
1.  **Cycle Detection**: Ensure the resolver implements Tarjan's algorithm or DFS with visited set to detect cycles.
2.  **Graceful degradation**: Break the cycle deterministically (e.g., drop the lowest priority atom) rather than failing the whole compilation.

## 7. Gap Analysis: The "Budget Black Hole" (Integer Extremes)

**Severity:** Medium
**Vector:** User Request Extremes / Integer Overflow

### The Mechanism
`TokenBudgetManager` performs arithmetic on `cc.TokenBudget`.
Inputs can be:
- Negative (Logic error in caller).
- Zero.
- `MaxInt`.

The current code:
```go
if budget <= 0 {
    budget = c.config.DefaultTokenBudget
}
```
This handles simple "missing budget" cases. But what if `DefaultTokenBudget` is also 0? Or negative due to bad config?
And what about `EstimateTokens`? If a string is massive (1GB), `EstimateTokens` might take seconds or overflow an `int` counter if not careful (though unlikely to overflow `int` with 1GB, it's 10^9 bytes, int is 2*10^9).

However, `mangleMandatoryLimits` logic is suspicious:
```go
budgetCap := int(float64(budget) * mangleMandatoryBudgetRatio)
```
If `budget` is massive (near `MaxInt`), `float64` conversion might lose precision, but `int` cast back is usually fine.
But if `budget` is negative (and not caught), `budgetCap` becomes negative.
`if budgetCap > 0 && budgetCap < tokenCap` checks might fail unexpectedly.

### Integer Overflow Scenarios
1.  **Accumulator Overflow**: `tokensUsed += tokens`. If `tokensUsed` is near MaxInt and `tokens` is large, it wraps around to negative.
    `if tokensUsed + tokens > tokenCap` -> `negative > positive` is FALSE.
    The check PASSES, allowing the atom.
    We effectively add infinite atoms until we crash memory.

2.  **Negative Budget**: If `budget` is somehow negative (e.g. `MaxInt` + 1 wrap around in caller), `budgetCap` becomes negative.
    `tokenCap` becomes 0.
    The Mandatory limit effectively disables all mandatory atoms.

### Suggested Test Case (Go)
```go
func TestVulnerability_BudgetOverflow(t *testing.T) {
    // Force a negative budget via overflow or bad config
    cc := NewCompilationContext().WithTokenBudget(math.MaxInt64, 0)
    // Add massive atoms
    atoms := []*PromptAtom{
        {ID: "massive", Content: strings.Repeat("x", 1000000)}, // 1MB
    }
    // Verify it doesn't accept infinite massive atoms
    // ...
}
```

### Recommendation
- Explicit validation in `CompilationContext.Validate()` to enforce `TokenBudget > 0` (after defaults).
- Cap max budget to reasonable semantic limits (e.g. 1M tokens) to prevent DoS via massive allocation.
- Use `int64` for token counters internally to prevent 32-bit overflow on 32-bit systems (though 64-bit is standard now).

## 8. Gap Analysis: The "Schrödinger's DB" (Concurrency)

**Severity:** Low (Flaky tests/Production race)
**Vector:** State Conflict / Race Condition

### The Mechanism
The compiler uses `c.mu` (RWMutex) to protect `shardDBs`.
However, `Compile` releases the lock during long operations (like `SelectAtoms` or `getVectorScores` which has a 10s timeout).
If `RegisterDB` or `UnregisterShardDB` is called *while* `Compile` is waiting on vector search, the state of `shardDBs` might change?
Actually, `collectAtomsWithStats` holds the Read Lock:
```go
c.mu.RLock()
defer c.mu.RUnlock()
```
This protects the map read.
But `collectAtomsWithStats` is just *one step*.
`Compile` does not hold the lock across the entire process.
Step 1: Collect (Read Lock held) -> Returns atoms.
Step 2: Select (No Lock on `c.mu`, but uses atoms).
Step 3: Resolve...

This seems safe for `shardDBs` map access.
However, `InjectAvailableSpecialists` reads from the filesystem (`workspace/.nerd/agents.json`).
File I/O is not thread-safe if another process is writing that file.
Also, `c.lastResult` update:
```go
c.mu.Lock()
c.lastResult = result
c.mu.Unlock()
```
This is safe.

But `cache` access:
```go
c.cacheMu.RLock()
// ...
c.cacheMu.RUnlock()
```
This is safe.

The potential race is in **Database Connection Management**.
`RegisterDB` closes the old DB.
```go
if c.projectDB != nil {
    c.projectDB.Close()
}
c.projectDB = db
```
If `collectAtomsWithStats` is running:
1. It enters RLock.
2. It accesses `c.projectDB`.
3. It calls `loadAtomsFromDB(ctx, c.projectDB)`.
   Inside `loadAtomsFromDB`:
   `rows, err := db.QueryContext(...)`
   This uses the DB connection.

If `RegisterDB` runs on another goroutine:
1. It wants `c.mu.Lock()`.
2. It blocks until `collectAtomsWithStats` releases RLock.

So this seems correctly synchronized! **Good job, developers.**

However, `loadAtomsFromDB` might take time. The lock is held for the duration of the query.
If the query hangs, the compiler locks up (RWLock).
`RegisterDB` will block indefinitely.

### Missing Test Coverage
- Concurrent `Compile` and `RegisterDB`.
- Slow DB query + RegisterDB.

## 9. Appendix: Sanitization Reference Implementation

To address the Mangle Injection vulnerability (Gap #4), we propose the following robust escaping mechanism. This should be implemented in `internal/prompt/selector.go` or a new `mangle_utils.go` file.

```go
// escapeMangleString sanitizes a string for use in a Mangle single-quoted string literal.
// It escapes single quotes and ensures the string is safe to embed.
func escapeMangleString(s string) string {
    if s == "" {
        return "''"
    }

    var sb strings.Builder
    sb.WriteRune('\'')

    for _, r := range s {
        switch r {
        case '\'':
            // Escape single quote: ' -> '' (Mangle/SQL style) or \' depending on parser
            // Assuming Mangle follows Datalog conventions which often use backslash
            sb.WriteString("\\'")
        case '\\':
            sb.WriteString("\\\\")
        case '\n':
            sb.WriteString("\\n")
        case '\r':
            sb.WriteString("\\r")
        case '\t':
            sb.WriteString("\\t")
        default:
            if r < 32 || r > 126 {
                // Escape non-printable chars
                sb.WriteString(fmt.Sprintf("\\u%04x", r))
            } else {
                sb.WriteRune(r)
            }
        }
    }

    sb.WriteRune('\'')
    return sb.String()
}
```

This implementation ensures that any malicious payload like `') :- dangerous(X).` is harmlessly treated as a string literal: `'\'\) :- dangerous\(X\).'`.

## 10. Conclusion & Action Plan

The JIT Prompt Compiler is a sophisticated piece of engineering, but it assumes "well-behaved" inputs. The lack of sanitization in Mangle fact generation is the most critical issue.

**Immediate Actions:**
1.  **Add Negative Tests**: Modify `internal/prompt/compiler_test.go` to expose the identified gaps.
    - Test case for `ShardType` with spaces.
    - Test case for Atom ID with single quotes.
    - Test case for Circular Dependencies.
    - Test case for Negative Budget.
2.  **Fix Mangle Generation**: (Future Task) Rewrite `buildContextFacts` to be injection-safe.
3.  **Harden Type Coercion**: (Future Task) Improve `extractStringArg`.

This journal serves as the formal record of these findings.

**Signed:** Jules, QA Automation Engineer
**System:** codeNERD/internal/prompt
**Version:** v0.4.0 (Analysis based on)
