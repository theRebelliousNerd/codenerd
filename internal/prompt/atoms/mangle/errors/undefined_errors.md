# Mangle Undefined Predicate/Function Errors Reference

Complete catalog of errors for undefined predicates, functions, and symbols.

---

## 1. Undeclared Predicate Errors

### 1.1 Predicate Not Declared in Schemas

**Error Pattern:**
```
predicate <pred> is not declared in schemas
predicate <pred> not declared
undeclared predicate <pred>
```

**Exact Message (codeNERD):**
```
predicate server_health is not declared in schemas
```

**What Causes It:**
- Using predicate without `Decl` statement
- Typo in predicate name
- Schema not loaded
- SchemaValidator caught hallucinated predicate

**Reproducing Example:**
```mangle
# schemas.mg has:
Decl user_action(Action.Type<name>).
Decl permitted(Action.Type<name>).

# Rule uses undefined predicate:
candidate_action(/monitor_server) :- server_health(/degraded).
# ERROR: server_health not declared!
```

**Why It's Critical:**
- Predicate with no data source will NEVER fire
- Rule is dead code
- Logic error in reasoning chain

**How to Fix:**

**Option 1:** Add declaration
```mangle
# CORRECT - declare the predicate
Decl server_health(Status.Type<name>).

candidate_action(/monitor_server) :- server_health(/degraded).
```

**Option 2:** Fix typo
```mangle
# If it's a typo, use correct name
candidate_action(/monitor_server) :- system_status(/degraded).
```

**Option 3:** Use existing predicate
```mangle
# CORRECT - use declared predicate
candidate_action(/monitor_server) :- user_action(/check_health).
```

---

### 1.2 Predicate Not Declared (Query)

**Error Pattern:**
```
predicate <pred> is not declared
```

**Context:** Occurs during query execution

**Reproducing Example:**
```go
// WRONG - querying undeclared predicate
result, err := engine.Query(ctx, "undefined_pred(X)")
// ERROR: predicate undefined_pred is not declared
```

**How to Fix:**
```go
// CORRECT - query declared predicate
result, err := engine.Query(ctx, "my_pred(X)")
```

---

### 1.3 Could Not Find Declaration

**Error Pattern:**
```
could not find declaration for <predicate>
```

**Context:** Occurs during fact insertion

**Reproducing Example:**
```go
// WRONG - adding fact for undeclared predicate
engine.AddFact("undeclared_pred", "value")
// ERROR: could not find declaration for undeclared_pred/1
```

**How to Fix:**
```mangle
# 1. Add declaration
Decl undeclared_pred(X.Type<string>).
```

```go
// 2. Add fact
engine.AddFact("undeclared_pred", "value")
```

---

### 1.4 No Decl for Predicate (Evaluation)

**Error Pattern:**
```
no decl for predicate <predicate>
```

**Context:** Occurs during rule evaluation

**What Causes It:**
- Rule uses predicate that wasn't declared
- Predicate appears in rule body but not in schema

**Reproducing Example:**
```mangle
# Schema missing declaration for helper predicate
result(X) :- undeclared_helper(X).
# ERROR during evaluation: no decl for predicate undeclared_helper
```

**How to Fix:**
```mangle
# Add missing declaration
Decl undeclared_helper(X.Type<string>).

result(X) :- undeclared_helper(X).
```

---

## 2. Undefined Function Errors

### 2.1 Hallucinated Functions (Common AI Error)

**Pattern:**
Functions that don't exist in Mangle but AI models hallucinate from other languages.

**Common Hallucinations:**

#### 2.1.1 String Functions
```mangle
# WRONG - These don't exist in Mangle
fn:split(String, Delimiter)      # Python/JS
fn:substring(String, Start, End)  # Java
fn:replace(String, Old, New)      # Common
fn:trim(String)                   # Common
fn:lowercase(String)              # Common
fn:contains(String, Substring)    # SQL
```

**What Exists:**
```mangle
# Mangle has limited string support
fn:string_concat(S1, S2)  # Concatenation (if available)
# Most string ops must be done in Go via external predicates
```

**Fix:** Use external predicates for complex string operations.

---

#### 2.1.2 List Functions
```mangle
# WRONG - These don't exist
fn:append(List, Element)     # Python
fn:filter(List, Predicate)   # Functional languages
fn:map(List, Function)       # Functional languages
fn:reduce(List, Function)    # Functional languages
fn:get(List, Index)          # Array indexing
```

**What Exists:**
```mangle
# Mangle list operations (if available)
:match_cons(List, Head, Tail)  # List destructuring
fn:Collect(Var)                # Aggregation to list
```

---

#### 2.1.3 Date/Time Functions
```mangle
# WRONG - These don't exist
fn:now()                    # Current time
fn:date(Year, Month, Day)   # Date construction
fn:format_date(Date, Fmt)   # Date formatting
fn:add_days(Date, N)        # Date arithmetic
```

**Fix:** Use external predicates or pass timestamps as integers.

---

#### 2.1.4 Math Functions (Extended)
```mangle
# WRONG - These don't exist in basic Mangle
fn:sqrt(X)      # Square root
fn:pow(X, Y)    # Power
fn:abs(X)       # Absolute value
fn:floor(X)     # Floor
fn:ceil(X)      # Ceiling
fn:round(X)     # Rounding
```

**What Exists:**
```mangle
# Basic arithmetic only
fn:plus(X, Y)   # Addition
fn:minus(X, Y)  # Subtraction
fn:mult(X, Y)   # Multiplication (if available)
fn:div(X, Y)    # Division (if available)
```

---

#### 2.1.5 Aggregation Casing Errors
```mangle
# WRONG - lowercase
fn:count()
fn:sum(X)
fn:min(X)
fn:max(X)
fn:avg(X)

# CORRECT - Capital first letter
fn:Count()
fn:Sum(X)
fn:Min(X)
fn:Max(X)
fn:Avg(X)  # If available
```

---

### 2.2 Function Not Found (Runtime)

**Error Pattern:**
```
function <func> not found
undefined function <func>
```

**What Causes It:**
- Calling function that doesn't exist in builtin package
- Typo in function name
- Function not imported

**Reproducing Example:**
```mangle
# WRONG - function doesn't exist
result(Y) :- source(X), Y = fn:sqrt(X).
# ERROR: function sqrt not found
```

**How to Fix:**

**Option 1:** Use external predicate
```mangle
# CORRECT - implement in Go
Decl sqrt_ext(X.Type<float>, Y.Type<float>) external().

result(Y) :- source(X), sqrt_ext(X, Y).
```

**Option 2:** Use available function
```mangle
# CORRECT - use existing function
result(Y) :- source(X), Y = fn:plus(X, 1).
```

---

## 3. Schema Validator Errors (codeNERD)

### 3.1 Rule Uses Undefined Predicates

**Error Pattern (codeNERD):**
```
rule uses undefined predicates: [<pred1>, <pred2>, ...] (available: [<available_preds>])
```

**Exact Message:**
```
rule uses undefined predicates: [server_health, system_load] (available: [user_action, permitted, task_status, ...])
```

**What Causes It:**
- SchemaValidator pre-validation
- Catches AI hallucinations before expensive Mangle compilation
- Predicates in rule body not in schema or learned.mg

**Reproducing Example:**
```mangle
# schemas.mg has:
Decl user_action(A.Type<name>).
Decl permitted(A.Type<name>).

# AI generates rule with undefined predicates:
candidate_action(/fix_issue) :-
    user_intent(/debug, Target, _),
    system_load(Load),           # Undefined!
    server_health(/degraded),    # Undefined!
    Load > 80.

# ERROR: rule uses undefined predicates: [system_load, server_health]
```

**How to Fix:**

**Option 1:** Declare missing predicates
```mangle
# Add declarations
Decl system_load(Load.Type<int>).
Decl server_health(Status.Type<name>).

# Now rule is valid
candidate_action(/fix_issue) :-
    user_intent(/debug, Target, _),
    system_load(Load),
    server_health(/degraded),
    Load > 80.
```

**Option 2:** Use available predicates
```mangle
# CORRECT - use only declared predicates
candidate_action(/fix_issue) :-
    user_intent(/debug, Target, _),
    permitted(/fix_issue).
```

**Option 3:** Add to learned.mg
```mangle
# If predicates are derived, add to learned.mg
system_load(Load) :- /* definition */.
server_health(Status) :- /* definition */.
```

---

### 3.2 Validation Errors List

**Error Pattern (codeNERD):**
```
validation errors:
line <N>: <error>
```

**Example:**
```
validation errors:
line 1: rule uses undefined predicates: [undefined_pred]
line 5: variable X is not bound
```

**How to Fix:**
- Address each error individually
- See specific error type documentation
- SchemaValidator shows all errors at once

---

## 4. Built-in Functions Reference

### 4.1 Confirmed Available Functions

**Arithmetic:**
```mangle
fn:plus(X, Y)      # X + Y
fn:minus(X, Y)     # X - Y
fn:mult(X, Y)      # X * Y (availability varies)
fn:div(X, Y)       # X / Y (availability varies)
```

**Aggregation (in transforms):**
```mangle
fn:Count()         # Count facts
fn:Sum(X)          # Sum values
fn:Min(X)          # Minimum value
fn:Max(X)          # Maximum value
fn:Collect(X)      # Collect into list
fn:Pick(X)         # Pick one value (first)
```

**Comparison (operators, not functions):**
```mangle
X = Y              # Equal
X != Y             # Not equal
X < Y              # Less than
X <= Y             # Less or equal
X > Y              # Greater than
X >= Y             # Greater or equal
```

**List Operations:**
```mangle
:match_cons(List, Head, Tail)      # [H|T] destructuring
:match_entry(Map, Key, Value)      # Map access (if available)
:match_field(Struct, Field, Value) # Struct field access
```

---

### 4.2 Function Availability Check

**To verify if a function exists:**

1. Check Mangle builtin package documentation
2. Test in minimal example:
   ```mangle
   Decl test(X.Type<int>, Y.Type<int>).
   test(X, Y) :- X = 1, Y = fn:the_function(X).
   ```
3. Review codeNERD's `builtin` imports

**If function not available:**
- Implement as external predicate in Go
- Use alternative logic
- Request function addition to Mangle

---

## 5. Symbol Resolution Errors

### 5.1 Unknown Symbol in Expression

**Error Pattern:**
```
unknown symbol <symbol>
symbol <symbol> not bound
```

**What Causes It:**
- Using symbol before definition
- Typo in symbol name

**Reproducing Example:**
```mangle
# WRONG - typo in variable name
result(X) :- source(X), Y = fn:plus(Xx, 1).
              # Xx is typo for X!
```

**How to Fix:**
```mangle
# CORRECT - match variable names
result(X) :- source(X), Y = fn:plus(X, 1).
```

---

## 6. External Predicate Issues

### 6.1 External Predicate Not Registered

**Error Pattern:**
```
external predicate <pred> not registered
no callback for external predicate <pred>
```

**What Causes It:**
- Predicate declared as `external()` but no Go callback registered
- Mismatch between schema and Go code

**Reproducing Example:**
```mangle
# Schema declares external:
Decl my_external(X.Type<string>) external().
```

```go
// WRONG - no registration
engine.LoadSchema("schemas.mg")
// Missing: RegisterExternal call!
```

**How to Fix:**
```go
// CORRECT - register callback
engine.RegisterExternal("my_external", func(query engine.Query, cb func(engine.Fact)) error {
    // Implementation
    return nil
})

engine.LoadSchema("schemas.mg")
```

---

### 6.2 Ext Callback for Non-External Predicate

**Error Pattern:**
```
ext callback for predicate <pred> that is not marked as external()
```

**What Causes It:**
- Go code registers callback but schema doesn't mark as external
- Predicate declared but missing `external()` directive

**Reproducing Example:**
```mangle
# WRONG - missing external() marker
Decl my_pred(X.Type<string>).
```

```go
// Go code tries to register
engine.RegisterExternal("my_pred", callback)
// ERROR: not marked external!
```

**How to Fix:**
```mangle
# CORRECT - add external() marker
Decl my_pred(X.Type<string>) external().
```

---

## 7. Diagnostic Strategies

### Strategy 1: List All Declared Predicates

```go
// codeNERD SchemaValidator
validator := mangle.NewSchemaValidator(schemasText, learnedText)
validator.LoadDeclaredPredicates()

available := validator.GetDeclaredPredicates()
log.Printf("Available predicates: %v", available)
```

### Strategy 2: Check Before Use

```go
// Validate rule before adding
if err := validator.ValidateRule(ruleText); err != nil {
    log.Error("Rule validation failed: %v", err)
    // Shows which predicates are undefined
}
```

### Strategy 3: Grep for Decls

```bash
# Find all declared predicates in schemas
grep "^Decl " .nerd/mangle/*.mg internal/mangle/*.mg

# Output:
# Decl user_action(Action.Type<name>).
# Decl permitted(Action.Type<name>).
# ...
```

---

## 8. Common Patterns

### Pattern 1: Virtual Predicates (External)

```mangle
# Declare as external
Decl file_exists(Path.Type<string>) external().
Decl file_content(Path.Type<string>, Content.Type<string>) external().
Decl ast_node(File.Type<string>, Node.Type<string>) external().
```

```go
// Register in Go
func registerVirtualPredicates(engine *mangle.Engine) {
    engine.RegisterExternal("file_exists", fileExistsCallback)
    engine.RegisterExternal("file_content", fileContentCallback)
    engine.RegisterExternal("ast_node", astNodeCallback)
}
```

---

### Pattern 2: Derived Predicates (learned.mg)

```mangle
# Base facts in schemas
Decl user_action(Action.Type<name>).
Decl permitted(Action.Type<name>).

# Derived predicates in learned.mg (automatically available)
candidate_action(Action) :- user_action(Action), permitted(Action).

# Can use candidate_action in other rules
next_action(Action) :- candidate_action(Action), priority(Action, /high).
```

---

### Pattern 3: Progressive Schema Loading

```go
// Load in layers
engine.LoadSchema("base_schemas.mg")        // Core predicates
engine.LoadSchema("extensions.mg")          // Extended predicates
engine.LoadSchema(".nerd/mangle/learned.mg") // Derived predicates

// Now all predicates are available
```

---

## Quick Diagnosis Table

| Error Message | Likely Cause | Fix |
|--------------|--------------|-----|
| "predicate X not declared" | Missing Decl | Add declaration |
| "function Y not found" | Hallucinated function | Use external predicate |
| "rule uses undefined predicates" | AI hallucination | Add Decl or use available |
| "ext callback ... without decl" | Missing external Decl | Add `external()` marker |
| "ext callback ... not marked" | Missing external() | Add to Decl |
| "fn:count" error | Wrong casing | Use `fn:Count()` |
| "fn:split" error | Function doesn't exist | Implement externally |

---

## Prevention Checklist

Before writing Mangle rules:

- [ ] Review available predicates (`grep "^Decl"`)
- [ ] Check SchemaValidator output
- [ ] Verify all predicates in rule are declared
- [ ] Confirm function names match available builtins
- [ ] Use correct casing for aggregation functions
- [ ] Mark external predicates with `external()`
- [ ] Test with minimal example before complex rule

---

## Related Documentation

- [parse_errors.md](./parse_errors.md) - Syntax errors
- [analysis_errors.md](./analysis_errors.md) - Safety errors
- [builtins_complete.md](../builtins_complete.md) - Available functions
- [engine/README.md](../engine/README.md) - External predicate registration
