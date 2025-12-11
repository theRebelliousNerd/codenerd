# Mangle Error Messages: Complete Catalog

## Parse Errors

### Syntax Errors

**Unexpected Token**
```
Parse error at line X, column Y: unexpected token 'TOKEN'
```

**Cause**: Invalid syntax, missing punctuation, wrong keywords

**Common Cases**:
- Missing period at end of rule
- Using `,` instead of `.` to end statement
- SQL/Soufflé syntax instead of Mangle

**Fix**: Check syntax, ensure proper Mangle grammar

---

**Unclosed String**
```
Parse error: unclosed string literal
```

**Cause**: Missing closing quote on string

**Example**:
```mangle
# ERROR
person("Alice).  # Missing closing quote

# FIX
person("Alice").
```

---

**Invalid Character**
```
Parse error: invalid character 'CHAR'
```

**Cause**: Using unsupported characters

**Fix**: Check allowed characters for identifiers, atoms, strings

---

## Analysis Errors

### Safety Violations

**Unbound Head Variable**
```
Safety error: Variable X appears in head but is not bound in body
```

**Cause**: Variable in rule head doesn't appear in any positive atom in body

**Example**:
```mangle
# ERROR
result(X) :- other(Y).  # X never bound

# FIX
result(X) :- other(X).
```

---

**Unbound Variable in Negation**
```
Safety error: Variable X in negated atom is not bound
```

**Cause**: Variable appears in `not atom` but isn't bound before the negation

**Example**:
```mangle
# ERROR
safe(X) :- not dangerous(X).  # X unbound

# FIX
safe(X) :- thing(X), not dangerous(X).
```

---

**Unbound Variable in Inequality**
```
Safety error: Variable X in inequality is not bound
```

**Cause**: Variable in `!=`, `<`, or `<=` isn't bound elsewhere

**Example**:
```mangle
# ERROR
different(X, Y) :- X != Y.  # Both unbound

# FIX
different(X, Y) :- person(X), person(Y), X != Y.
```

---

### Arity Errors

**Predicate Arity Mismatch**
```
Arity error: Predicate 'pred' expects N arguments, got M
```

**Cause**: Using predicate with wrong number of arguments

**Example**:
```mangle
# Declaration
Decl parent(Person, Child).  # Arity 2

# ERROR
result(X) :- parent(X).  # Only 1 argument

# FIX
result(X) :- parent(X, Y).
```

---

**Function Arity Mismatch**
```
Arity error: Function 'fn:name' expects N arguments, got M
```

**Cause**: Calling function with wrong number of arguments

**Example**:
```mangle
# ERROR
sum(S) :- a(A), S = fn:plus(A).  # fn:plus needs 2 args

# FIX
sum(S) :- a(A), b(B), S = fn:plus(A, B).
```

---

### Declaration Errors

**Undeclared Predicate**
```
Declaration error: Predicate 'pred' is not declared
```

**Cause**: Using predicate without declaration (if declarations required)

**Fix**: Add `Decl pred(Args).` before use

---

**Duplicate Declaration**
```
Declaration error: Predicate 'pred' already declared
```

**Cause**: Multiple `Decl` statements for same predicate

**Fix**: Keep only one declaration per predicate

---

### Mode Violations

**Mode Violation: Input Not Bound**
```
Mode error: Argument N of 'pred' must be bound (mode: +)
```

**Cause**: Mode declared as input (`+`) but variable not bound before use

**Example**:
```mangle
Decl lookup(Key, Value)
  descr [mode(+, -)].  # Key must be input

# ERROR
result(V) :- lookup(K, V).  # K not bound

# FIX
result(V) :- key_data(K), lookup(K, V).
```

---

## Stratification Errors

**Cannot Stratify: Cycle Through Negation**
```
Stratification error: Cannot stratify program - cycle through negation detected
```

**Cause**: Predicates recursively depend on their own negation (directly or indirectly)

**Example**:
```mangle
# ERROR: p depends on not q, q depends on not p
p(X) :- not q(X).
q(X) :- not p(X).
```

**Fix**: Redesign logic to eliminate mutual negation

---

**Predicate Recursively Depends on Own Negation**
```
Stratification error: Predicate 'pred' recursively depends on its own negation
```

**Cause**: Self-negation in recursive rule

**Example**:
```mangle
# ERROR
reach(X, Y) :- edge(X, Y).
reach(X, Z) :- reach(X, Y), not reach(Y, Z).  # Negates itself!
```

**Fix**: Remove self-negation, redesign logic

---

## Type Errors

**Type Bounds Violated**
```
Type error: Expected TYPE for argument N, got ACTUAL_TYPE
```

**Cause**: Fact violates declared type bounds

**Example**:
```mangle
Decl person(Name, Age)
  bounds [ /string, /number ].

# ERROR
person("Alice", "thirty").  # Age is string, not number

# FIX
person("Alice", 30).
```

---

**Type Mismatch in Function**
```
Type error: Function 'fn:name' expects TYPE, got ACTUAL_TYPE
```

**Cause**: Wrong type passed to function

**Example**:
```mangle
# ERROR
result(R) :- name(N), R = fn:plus(N, 1).  # N is string, plus needs number

# FIX - ensure N is a number
result(R) :- age(A), R = fn:plus(A, 1).
```

---

**Cannot Mix Integer and Float**
```
Type error: Cannot mix integer and float in operation
```

**Cause**: Using int with float function or vice versa

**Example**:
```mangle
# ERROR
result(R) :- R = fn:plus(3, 3.14).  # int + float

# FIX - use consistent types
result(R) :- R = fn:float_plus(3.0, 3.14).
```

---

## Evaluation Errors

**Fact Limit Exceeded**
```
Evaluation error: Created fact limit (N) exceeded
```

**Cause**: Evaluation created more facts than allowed limit

**Common Cause**: Infinite recursion, unbounded fact generation

**Example**:
```mangle
# ERROR - infinite counter
count(N) :- count(M), N = fn:plus(M, 1).
count(0).
```

**Fix**: Add bounds to recursion, increase limit, or redesign logic

---

**Division by Zero**
```
Evaluation error: Division by zero
```

**Cause**: Calling `fn:div` or `fn:float_div` with zero denominator

**Fix**: Add guard condition

```mangle
# ERROR
result(R) :- numerator(N), denominator(D), R = fn:div(N, D).  # D could be 0

# FIX
result(R) :- numerator(N), denominator(D), D != 0, R = fn:div(N, D).
```

---

**Builtin Function Error**
```
Evaluation error: Function 'fn:name' failed: REASON
```

**Cause**: Builtin function encountered error (e.g., invalid arguments)

**Fix**: Check function documentation, validate inputs

---

**External Predicate Error**
```
Evaluation error: External predicate 'pred' failed: ERROR_MESSAGE
```

**Cause**: Custom external predicate returned error

**Fix**: Check external predicate implementation, handle edge cases

---

## Common Error Patterns

### 1. Lowercase Variable

```mangle
# ERROR - interpreted as constant
result(x) :- data(y).

# FIX - uppercase for variables
result(X) :- data(Y).
```

**Error**: Likely "constant x not found" or arity mismatch

---

### 2. Using String for Atom

```mangle
# ERROR - if declaration expects name type
person("alice", 30).  # When /alice expected

# FIX
person(/alice, 30).
```

**Error**: "Type bounds violated: expected /name, got /string"

---

### 3. Missing Grouping in Aggregation

```mangle
# ERROR - Cat not bound before transform
total(Cat, T) :-
    item(Value)
    |> do fn:group_by(Cat),  # Cat never bound!
       let T = fn:Sum(Value).

# FIX - bind Cat first
total(Cat, T) :-
    item(Cat, Value)  # Cat now bound
    |> do fn:group_by(Cat),
       let T = fn:Sum(Value).
```

**Error**: "Variable Cat in group_by is not bound"

---

### 4. Infinite Recursion

```mangle
# ERROR - no bounds
count(N) :- count(M), N = fn:plus(M, 1).
count(0).

# FIX - add bound
count(N) :- count(M), M < 100, N = fn:plus(M, 1).
count(0).
```

**Error**: "Created fact limit exceeded"

---

### 5. Cartesian Product Explosion

```mangle
# INEFFICIENT - huge intermediate result
result(X) :- huge_table(X), X = /specific_id.

# BETTER - filter first
result(X) :- X = /specific_id, huge_table(X).
```

**Error**: May hit fact limit or be very slow

---

## Debugging Strategies

### 1. Read Error Messages Carefully

Mangle errors include:
- Line and column numbers
- Specific variable or predicate names
- Clear description of problem

### 2. Check Safety

For "variable not bound" errors:
- Ensure all head variables appear in positive body atoms
- Ensure variables in negation are bound before negation
- Ensure variables in comparison are bound

### 3. Verify Arity

Count arguments:
- In declaration: `Decl pred(A1, A2, A3).` → arity 3
- In use: `pred(X, Y, Z)` → must also be arity 3

### 4. Trace Recursion

For "fact limit exceeded":
- Find recursive rules
- Check for base cases
- Verify termination conditions
- Add explicit bounds

### 5. Use Type Declarations

Type errors caught early:
```mangle
Decl value(Val)
  bounds [ /number ].

# This will error at analysis time, not evaluation
value("not a number").  # Immediate type error
```

### 6. Test Incrementally

- Start with facts only
- Add simple rules one at a time
- Test after each addition
- Isolate problematic rules

---

## Error Prevention Checklist

Before running Mangle code:

- [ ] All predicates declared
- [ ] All variables are UPPERCASE
- [ ] All rules end with period `.`
- [ ] All head variables appear in positive body atoms
- [ ] Variables in negation are bound before negation
- [ ] Recursive rules have base cases and bounds
- [ ] Function applications have correct arity
- [ ] Type bounds match actual data
- [ ] Aggregations use transform blocks (`|>`)
- [ ] Grouping variables are bound before transform

---

## Getting More Information

### Parse Errors

- Check line and column number
- Look for missing or extra punctuation
- Verify syntax matches Mangle grammar

### Analysis Errors

- Review safety rules
- Check variable binding flow
- Verify arity throughout

### Evaluation Errors

- Add logging to see intermediate facts
- Use `EvalProgramWithStats` to monitor growth
- Check for unexpected fact explosion

### Type Errors

- Review type declarations
- Ensure consistency between declaration and usage
- Use type checker during analysis
