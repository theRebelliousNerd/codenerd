# Mangle Runtime Errors Reference

Complete catalog of evaluation-time errors that occur during program execution.

---

## 1. Fact Store Errors

### 1.1 Could Not Find Declaration

**Error Pattern:**
```
could not find declaration for <predicate>
```

**Exact Message:**
```
could not find declaration for my_predicate/2
```

**What Causes It:**
- Attempting to add fact for undeclared predicate
- Predicate not in loaded schemas
- Typo in predicate name

**Reproducing Example:**
```mangle
# Schema has this:
Decl parent(X.Type<string>, Y.Type<string>).

# But code tries to add:
# ERROR - typo or undeclared
parrent("alice", "bob").
```

**How to Fix:**

**Option 1:** Add declaration
```mangle
# CORRECT - declare before using
Decl my_predicate(X.Type<string>).

my_predicate("value").
```

**Option 2:** Fix typo
```mangle
# CORRECT - match existing declaration
Decl parent(X.Type<string>, Y.Type<string>).

parent("alice", "bob").
```

**Option 3:** Check schema load order
```go
// WRONG - adding facts before loading schema
engine.AddFact("parent", "alice", "bob")
engine.LoadSchema("schemas.mg")

// CORRECT - load schema first
engine.LoadSchema("schemas.mg")
engine.AddFact("parent", "alice", "bob")
```

**Related Errors:**
- "predicate not declared" (analysis phase)
- Arity mismatch errors

---

### 1.2 Could Not Unify Fact and Declaration

**Error Pattern:**
```
could not unify <fact> and <decl>: <reason>
```

**Exact Message:**
```
could not unify parent(1, 2) and Decl parent(X.Type<string>, Y.Type<string>): type mismatch
```

**What Causes It:**
- Fact arguments don't match declared types
- Type checking at insertion time

**Reproducing Example:**
```mangle
Decl parent(X.Type<string>, Y.Type<string>).

# WRONG - integers where strings expected
parent(1, 2).
```

**How to Fix:**
```mangle
Decl parent(X.Type<string>, Y.Type<string>).

# CORRECT - match declared types
parent("alice", "bob").
```

**Note:** See [type_errors.md](./type_errors.md) for comprehensive type mismatch patterns.

---

### 1.3 Fact Found Extra Variables

**Error Pattern:**
```
<fact> found extra variables <vars>
```

**What Causes It:**
- Fact contains unbound variables
- Facts must be ground (fully instantiated)

**Reproducing Example:**
```mangle
# WRONG - X is a variable in fact
parent("alice", X).
```

**How to Fix:**
```mangle
# CORRECT - facts are ground
parent("alice", "bob").
parent("alice", "charlie").
```

**Important:** Facts cannot contain variables. Only rules can have variables.

---

## 2. Evaluation Errors

### 2.1 Analysis Error

**Error Pattern:**
```
analysis: <underlying error>
```

**What Causes It:**
- Static analysis failed during evaluation
- Usually a safety or stratification issue
- Wrapper error for analysis-phase problems

**Example:**
```
analysis: variable X is not bound in result(X) :- other(Y)
```

**How to Fix:**
- Check underlying error details
- See [analysis_errors.md](./analysis_errors.md) for specific fixes

---

### 2.2 Stratification Error During Eval

**Error Pattern:**
```
stratification: <underlying error>
```

**Exact Message:**
```
stratification: program cannot be stratified
```

**What Causes It:**
- Stratification check failed at evaluation time
- Negation cycle detected

**How to Fix:**
- See [analysis_errors.md](./analysis_errors.md#1-stratification-violations)
- Break negation cycles
- Restructure rules into strata

---

### 2.3 Expected First Premise to Be Atom

**Error Pattern:**
```
expected first premise of clause: <clause> to be an atom <premise>
```

**What Causes It:**
- Rule's first body predicate is not an atom
- Transform or special form where atom expected

**Reproducing Example:**
```mangle
# WRONG - starts with transform
result(X) :- |> do fn:group_by(X), source(X).
```

**How to Fix:**
```mangle
# CORRECT - atom predicate first
result(X) :- source(X) |> do fn:group_by(X).
```

---

### 2.4 No Decl for Predicate

**Error Pattern:**
```
no decl for predicate <predicate>
```

**What Causes It:**
- Predicate used in evaluation but not declared
- Similar to "could not find declaration" but during rule evaluation

**Reproducing Example:**
```mangle
# Missing declaration
result(X) :- undefined_pred(X).
```

**How to Fix:**
```mangle
# CORRECT - declare all predicates
Decl undefined_pred(X.Type<string>).

result(X) :- undefined_pred(X).
```

---

## 3. External Predicate Errors

### 3.1 Ext Callback for Predicate Without Decl

**Error Pattern:**
```
ext callback for a predicate <pred> without decl
```

**What Causes It:**
- External predicate registered but not declared in schema
- Missing declaration for virtual/external predicate

**Reproducing Example:**
```go
// Go code registers external predicate
engine.RegisterExternal("my_external", callback)

// But schema doesn't declare it:
// Missing: Decl my_external(X) external().
```

**How to Fix:**
```mangle
# CORRECT - declare external predicate
Decl my_external(X.Type<string>) external().
```

**Important:** All predicates, including external ones, need `Decl` statements.

---

### 3.2 Ext Callback for Non-External Predicate

**Error Pattern:**
```
ext callback for predicate <pred> that is not marked as external()
```

**What Causes It:**
- Predicate declared but not marked with `external()` directive
- Mismatch between Go registration and schema

**Reproducing Example:**
```mangle
# WRONG - missing external() marker
Decl my_pred(X.Type<string>).
```

**How to Fix:**
```mangle
# CORRECT - mark as external
Decl my_pred(X.Type<string>) external().
```

---

## 4. Transform Evaluation Errors

### 4.1 Merging With Multiple Target Vars Not Implemented

**Error Pattern:**
```
merging with |target vars| != 1 not implemented: <fundep>
```

**What Causes It:**
- Complex functional dependency in transform
- Limitation in current implementation

**How to Fix:**
- Simplify transform
- Use single target variable
- Split into multiple rules

---

## 5. Inclusion Constraint Errors

### 5.1 Unexpected Inclusion Constraint

**Error Pattern:**
```
unexpected inclusion constraint <constraint>
```

**What Causes It:**
- Invalid inclusion constraint in declaration
- Constraint format not recognized

**How to Fix:**
- Review declaration syntax
- Check constraint specification
- Consult type system documentation

---

### 5.2 None of Inclusion Constraints Satisfied

**Error Pattern:**
```
none of the inclusion constraints are satisfied. reasons: <reasons>
```

**What Causes It:**
- Fact violates all declared inclusion constraints
- Type bounds not satisfied

**How to Fix:**
- Check fact matches at least one declared mode
- Verify types satisfy bounds
- Review inclusion constraint logic

---

## 6. Query Execution Errors

### 6.1 Query Timeout

**Error Pattern:**
```
query execution timed out after <duration>
```

**Exact Message (codeNERD):**
```
query execution timed out after 30s: context deadline exceeded
```

**What Causes It:**
- Query takes longer than configured timeout
- Infinite recursion or large result set
- Expensive computation

**Reproducing Example:**
```mangle
# WRONG - unbounded recursion
infinite(X) :- infinite(X).

# Query:
? infinite(X).
# ERROR: timeout after 30s
```

**How to Fix:**

**Option 1:** Add base case to stop recursion
```mangle
# CORRECT - bounded recursion
count(0).
count(N) :- count(M), M < 100, N = fn:plus(M, 1).
```

**Option 2:** Increase timeout (Go code)
```go
// Increase query timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
result, err := engine.Query(ctx, "expensive_query(X)")
```

**Option 3:** Optimize query
```mangle
# Add selectivity - filter early
result(X) :- X = /specific_id, large_table(X, Y).
# NOT: result(X) :- large_table(X, Y), X = /specific_id.
```

---

### 6.2 Predicate Not Declared (Query)

**Error Pattern:**
```
predicate <pred> is not declared
```

**What Causes It:**
- Querying undeclared predicate
- Typo in query

**Reproducing Example:**
```go
// WRONG - typo or undeclared
result, err := engine.Query(ctx, "undeclared_pred(X)")
```

**How to Fix:**
```go
// CORRECT - query declared predicate
result, err := engine.Query(ctx, "my_pred(X)")
```

---

### 6.3 Predicate Has No Modes

**Error Pattern:**
```
predicate <pred> has no modes declared
```

**What Causes It:**
- Predicate declared without modes
- Mode specification missing

**Reproducing Example:**
```mangle
# WRONG - no mode specification
Decl my_pred(X.Type<string>).
# Missing mode information
```

**How to Fix:**
```mangle
# CORRECT - include mode bounds
Decl my_pred(X.Type<string>) bound.
```

---

## 7. codeNERD-Specific Runtime Errors

### 7.1 No Schemas Loaded

**Error Pattern (codeNERD):**
```
no schemas loaded; call LoadSchema first
```

**What Causes It:**
- Attempting operations before loading schemas
- Schema initialization order wrong

**Reproducing Example:**
```go
engine := mangle.NewEngine(cfg, persistence)

// WRONG - adding facts before schema
engine.AddFact("parent", "alice", "bob")

// ERROR: no schemas loaded
```

**How to Fix:**
```go
engine := mangle.NewEngine(cfg, persistence)

// CORRECT - load schema first
engine.LoadSchema("schemas.mg")
engine.AddFact("parent", "alice", "bob")
```

---

### 7.2 No Schemas Loaded for Query

**Error Pattern (codeNERD):**
```
no schemas loaded; cannot execute query
```

**What Causes It:**
- Querying before schema initialization

**How to Fix:**
```go
// CORRECT order
engine.LoadSchema("schemas.mg")
// ... load facts ...
result, err := engine.Query(ctx, "my_pred(X)")
```

---

### 7.3 Failed to Parse Schema

**Error Pattern (codeNERD):**
```
failed to parse schema: <parse error>
```

**What Causes It:**
- Syntax error in schema file
- Schema file contains invalid Mangle code

**How to Fix:**
- Check parse error details
- See [parse_errors.md](./parse_errors.md)
- Validate schema syntax before loading

---

### 7.4 Failed to Analyze Schema

**Error Pattern (codeNERD):**
```
failed to analyze schema: <analysis error>
```

**What Causes It:**
- Schema passed parse but failed analysis
- Safety or stratification violation in schema

**How to Fix:**
- Check analysis error details
- See [analysis_errors.md](./analysis_errors.md)
- Fix safety/stratification issues

---

## 8. Fact Insertion Errors

### 8.1 Predicate Not Declared in Schemas (codeNERD)

**Error Pattern:**
```
predicate <pred> is not declared in schemas
```

**What Causes It:**
- Attempting to add fact for undeclared predicate
- SchemaValidator check failed

**Reproducing Example:**
```go
// Schema only has:
// Decl parent(X, Y).

// WRONG - typo or undeclared
engine.AddFact("parrent", "alice", "bob")
```

**How to Fix:**
```go
// CORRECT - match declared predicate
engine.AddFact("parent", "alice", "bob")
```

---

### 8.2 Predicate Arity Mismatch (codeNERD)

**Error Pattern:**
```
predicate <pred> expects <N> args, got <M>
```

**Exact Message:**
```
predicate parent expects 2 args, got 3
```

**What Causes It:**
- Fact arguments don't match declared arity

**Reproducing Example:**
```mangle
Decl parent(X.Type<string>, Y.Type<string>).
```

```go
// WRONG - 3 args when declared with 2
engine.AddFact("parent", "alice", "bob", "charlie")
```

**How to Fix:**
```go
// CORRECT - match arity
engine.AddFact("parent", "alice", "bob")
```

---

### 8.3 Unsupported Fact Argument Type (codeNERD)

**Error Pattern:**
```
unsupported fact argument type <type>
```

**What Causes It:**
- Passing Go type that can't convert to Mangle type
- Complex types without JSON serialization

**Reproducing Example:**
```go
// WRONG - complex Go struct without serialization
type MyStruct struct { unexported int }
engine.AddFact("data", MyStruct{42})
```

**How to Fix:**

**Option 1:** Use supported types
```go
// CORRECT - supported primitive types
engine.AddFact("data", "string", 42, 3.14, true)
```

**Option 2:** Serialize complex types
```go
// CORRECT - JSON serialize complex types
data := map[string]interface{}{"field": "value"}
jsonStr, _ := json.Marshal(data)
engine.AddFact("data", string(jsonStr))
```

---

## 9. Rule Evaluation Errors

### 9.1 Break Error (Internal)

**Error Pattern:**
```
break
```

**What Causes It:**
- Internal control flow error
- Should not be user-visible
- Indicates engine bug if seen

**How to Fix:**
- Report as engine bug
- Not user-fixable

---

## 10. Common Runtime Patterns

### Pattern 1: Order of Operations
```go
// CORRECT initialization order
engine := NewEngine(config, persistence)
engine.LoadSchema("schemas.mg")          // 1. Load schemas
engine.WarmFromPersistence(ctx)          // 2. Hydrate from DB
engine.AddFacts(initialFacts)            // 3. Add facts
result, err := engine.Query(ctx, query)  // 4. Query
```

### Pattern 2: Handling Evaluation Errors
```go
result, err := engine.Query(ctx, query)
if err != nil {
    if strings.Contains(err.Error(), "timeout") {
        // Handle timeout specifically
        log.Warn("Query timed out, trying with limit")
    } else if strings.Contains(err.Error(), "not declared") {
        // Predicate error
        log.Error("Undeclared predicate in query")
    } else {
        // Generic error
        log.Error("Query failed: %v", err)
    }
}
```

### Pattern 3: Fact Validation
```go
// Validate before insertion
if err := validator.ValidateRule(ruleText); err != nil {
    // Catch errors before engine evaluation
    return fmt.Errorf("invalid rule: %w", err)
}

// Then add
engine.AddFacts(facts)
```

---

## Debugging Runtime Errors

### Step 1: Check Error Message
- Read full error text
- Note the exact predicate/clause mentioned
- Check if it's a wrapper error (e.g., "analysis: ...")

### Step 2: Verify Initialization
- Schemas loaded?
- Correct load order?
- All predicates declared?

### Step 3: Check Fact Types
- Types match declaration?
- Arity correct?
- Ground facts (no variables)?

### Step 4: Review Rule Logic
- Variables bound before use?
- No infinite recursion?
- Stratification valid?

### Step 5: Enable Debug Logging
```go
// codeNERD logging
logging.SetLevel(logging.LevelDebug)

// View Mangle operations
engine.ToggleAutoEval(false)  // Manual control
engine.AddFacts(facts)
engine.RecomputeRules()       // Explicit evaluation
```

---

## Performance Issues

### Symptom: Slow Evaluation
**Possible Causes:**
- Cartesian product explosion
- Unbounded recursion
- Large intermediate results

**Diagnosis:**
```go
// Check fact counts
stats := engine.GetStats()
log.Printf("Total facts: %d", stats.TotalFacts)
log.Printf("Derived facts: %d", engine.GetDerivedFactCount())
```

**Fixes:**
- Add selectivity (filter early)
- Bound recursion
- Index key predicates
- Increase gas limit if needed

### Symptom: Memory Growth
**Possible Causes:**
- Fact leak (not removing old facts)
- Infinite derivation
- Too many derived facts

**Fixes:**
```go
// Set gas limit
config := mangle.DefaultConfig()
config.DerivedFactsLimit = 100000

// Monitor derived facts
derivedCount := engine.GetDerivedFactCount()
if derivedCount > threshold {
    engine.ResetDerivedFactCount()
}
```

---

## Related Documentation

- [parse_errors.md](./parse_errors.md) - Syntax errors
- [analysis_errors.md](./analysis_errors.md) - Static analysis errors
- [type_errors.md](./type_errors.md) - Type mismatches
- [gas_errors.md](./gas_errors.md) - Resource limit errors
