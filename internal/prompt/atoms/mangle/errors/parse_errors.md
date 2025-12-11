# Mangle Parse Errors Reference

Complete catalog of parse-time errors with exact messages, causes, and fixes.

---

## 1. Not a Base Term

**Error Pattern:**
```
not a base term: <term> <type>
```

**Exact Message:**
```
not a base term: predicate_call(X) *ast.Atom
```

**What Causes It:**
- Attempting to use a predicate call where a base term (variable, constant, or value) is expected
- Common in argument positions that expect values, not predicates

**Reproducing Example:**
```mangle
# WRONG - predicate call in argument position
result(X, some_pred(Y)).
```

**How to Fix:**
```mangle
# CORRECT - use variables or constants
result(X, Value) :- some_pred(Y), Value = Y.
```

**Related Errors:**
- "expected base term got..." (analysis phase)
- Type mismatch errors

---

## 2. Not an Atom

**Error Pattern:**
```
not an atom: <term> <type>
```

**Exact Message:**
```
not an atom: Variable *ast.Variable
```

**What Causes It:**
- Trying to parse something as an atom that isn't an atom
- Usually internal parser state issues
- Can occur when syntax is malformed

**Reproducing Example:**
```mangle
# WRONG - malformed predicate syntax
Variable(X).
```

**How to Fix:**
```mangle
# CORRECT - atoms start with lowercase
my_predicate(X).
```

**Related Errors:**
- Parse syntax errors
- Token recognition errors

---

## 3. Parse Error (Generic)

**Error Pattern:**
```
parse error at line <num>: <details>
```

**What Causes It:**
- Generic syntax violation
- Unrecognized token sequence
- Grammar rule violation

**Common Parse Errors:**

### 3.1 Missing Period
```mangle
# WRONG
result(X) :- source(X)

# Error: parse error at line 1: unexpected end of input
```

**Fix:** Add period
```mangle
result(X) :- source(X).
```

### 3.2 Unbalanced Parentheses
```mangle
# WRONG
result(X) :- source(X, Y.

# Error: parse error at line 1: mismatched input
```

**Fix:** Balance parentheses
```mangle
result(X) :- source(X, Y).
```

### 3.3 Invalid Characters
```mangle
# WRONG
result(X) :- source(X) && other(X).

# Error: token recognition error at: '&&'
```

**Fix:** Use comma for AND
```mangle
result(X) :- source(X), other(X).
```

### 3.4 Prolog Negation Syntax
```mangle
# WRONG
blocked(X) :- \+ permitted(X).

# Error: token recognition error at: '\'
```

**Fix:** Use `!` for negation
```mangle
blocked(X) :- !permitted(X).
```

---

## 4. Token Recognition Error

**Error Pattern:**
```
token recognition error at: '<token>'
```

**Common Cases:**

### 4.1 Backslash Characters
```mangle
# WRONG
\+ permitted(X)

# Error: token recognition error at: '\'
```

**Fix:** Use `!`
```mangle
!permitted(X)
```

### 4.2 Special Characters
```mangle
# WRONG - @ is not valid
result(@param)

# Error: token recognition error at: '@'
```

**Fix:** Use valid identifiers
```mangle
result(/param)
```

### 4.3 SQL/Prolog Operators
```mangle
# WRONG
X <> Y   # SQL not-equal
X =:= Y  # Prolog arithmetic equal

# Error: token recognition error
```

**Fix:** Use Mangle operators
```mangle
X != Y   # Mangle not-equal
X = Y    # Mangle unification
```

---

## 5. No Viable Alternative

**Error Pattern:**
```
no viable alternative at input '<text>'
```

**What Causes It:**
- Parser cannot find a valid grammar rule for the input
- Often due to keyword misuse or invalid syntax combinations

**Common Cases:**

### 5.1 Wrong Declaration Syntax
```mangle
# WRONG - Soufflé syntax
.decl parent(x:string, y:string).

# Error: no viable alternative at input '.decl'
```

**Fix:**
```mangle
Decl parent(X.Type<string>, Y.Type<string>).
```

### 5.2 SQL Keywords
```mangle
# WRONG
SELECT X FROM source(X).

# Error: no viable alternative at input 'SELECT'
```

**Fix:**
```mangle
# Use Mangle query syntax
result(X) :- source(X).
```

### 5.3 Case/When Statements
```mangle
# WRONG
result(X, case when X > 0 then /positive else /negative end).

# Error: no viable alternative at input 'case'
```

**Fix:**
```mangle
result(X, /positive) :- X > 0.
result(X, /negative) :- X <= 0.
```

---

## 6. Mismatched Input

**Error Pattern:**
```
mismatched input '<found>' expecting '<expected>'
```

**Common Cases:**

### 6.1 Missing Comma
```mangle
# WRONG
result(X) :- source(X) other(X).

# Error: mismatched input 'other' expecting ','
```

**Fix:**
```mangle
result(X) :- source(X), other(X).
```

### 6.2 Missing Colon-Dash
```mangle
# WRONG
result(X) source(X).

# Error: mismatched input 'source' expecting ':-'
```

**Fix:**
```mangle
result(X) :- source(X).
```

### 6.3 Unclosed List
```mangle
# WRONG
result([X, Y, Z) :- source(X, Y, Z).

# Error: mismatched input ')' expecting ']'
```

**Fix:**
```mangle
result([X, Y, Z]) :- source(X, Y, Z).
```

---

## 7. Empty Rule/Clause

**Error Pattern:**
```
parse error: unexpected end of input
```

**What Causes It:**
- Rule body is empty
- Incomplete clause

**Reproducing Example:**
```mangle
# WRONG
result(X) :- .

# Error: parse error: unexpected end of input
```

**How to Fix:**
```mangle
# Either make it a fact
result(/default).

# Or provide a body
result(X) :- source(X).
```

---

## 8. Invalid Variable Name

**Error Pattern:**
```
parse error at line <num>: invalid identifier
```

**What Causes It:**
- Variable names must be UPPERCASE
- Cannot start with numbers
- Cannot contain spaces or special chars

**Reproducing Example:**
```mangle
# WRONG
result(x) :- source(x).        # lowercase variable
result(2X) :- source(2X).      # starts with number
result(My Var) :- source(X).   # contains space
```

**How to Fix:**
```mangle
# CORRECT
result(X) :- source(X).
result(X2) :- source(X2).
result(MyVar) :- source(X).
```

---

## 9. Invalid Atom/Constant Name

**Error Pattern:**
```
parse error: invalid atom syntax
```

**What Causes It:**
- Atoms must start with `/`
- Cannot contain invalid characters

**Reproducing Example:**
```mangle
# WRONG
status(X, active).        # missing /
status(X, /my status).    # space in atom
status(X, /123).          # starts with number
```

**How to Fix:**
```mangle
# CORRECT
status(X, /active).
status(X, /my_status).
status(X, /a123).
```

---

## 10. Comment Syntax Errors

**Error Pattern:**
```
token recognition error at: '/'
```

**What Causes It:**
- Using wrong comment syntax
- Mangle uses `#` for comments, not `//` or `/* */`

**Reproducing Example:**
```mangle
# WRONG
// This is a comment
/* This is a block comment */
```

**How to Fix:**
```mangle
# CORRECT - Use # for comments
# This is a comment
# Multi-line comments need # on each line
```

---

## Diagnostic Patterns

### Pattern 1: Line and Column References
When you see:
```
error at line 42, column 15
```

- Line numbers are 1-indexed
- Column numbers point to the first character of the error
- Look at the exact character at that position

### Pattern 2: Context Clues
Parse errors often show context:
```
no viable alternative at input 'result(X) :- source(X) other'
                                                      ^^^^
```

The shown text indicates where the parser gave up.

### Pattern 3: Cascading Errors
One syntax error can cause multiple parse errors:
```mangle
# WRONG - missing period causes multiple errors
result(X) :- source(X)
other(Y) :- result(Y).
```

**Fix the first error first**, then re-parse.

---

## Quick Checklist for Parse Errors

When you get a parse error, check:

- [ ] Does every rule end with `.`?
- [ ] Are all parentheses balanced?
- [ ] Are all variables UPPERCASE?
- [ ] Are all atoms lowercase with `/` prefix?
- [ ] Are you using `,` for AND (not `&&` or space)?
- [ ] Are you using `!` for negation (not `\+` or `not`)?
- [ ] Are comments using `#` (not `//` or `/* */`)?
- [ ] Are there any special characters (`@`, `$`, `&`, etc.)?
- [ ] Is this Prolog/SQL/Soufflé syntax instead of Mangle?

---

## Error Recovery Strategy

1. **Isolate the error**: Comment out code until parse succeeds
2. **Add code back**: Un-comment line by line
3. **First error wins**: Fix the first reported error before others
4. **Syntax validation**: Use the pre-validator for common patterns
5. **Minimal example**: Reduce to smallest reproducing case

---

## Related Documentation

- [syntax_core.md](../syntax_core.md) - Complete Mangle syntax reference
- [analysis_errors.md](./analysis_errors.md) - Static analysis errors
- [type_errors.md](./type_errors.md) - Type system errors
