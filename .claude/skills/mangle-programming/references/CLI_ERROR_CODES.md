# Mangle Diagnostic Error Codes

Complete reference of all diagnostic codes produced by the Mangle CLI.

## Error Code Categories

| Range | Category | Source |
|-------|----------|--------|
| P001 | Parse/Syntax errors | mangle-parse, mangle-lexer |
| E000 | CLI/IO errors | mangle-cli |
| E001-E004 | Variable binding | mangle-semantic |
| E005-E009 | Built-in predicates/functions | mangle-semantic |
| E010-E014 | Transform and function | mangle-semantic |
| E015 | Stratification | mangle-stratification |
| E016-E023 | Performance warnings | mangle-stratification |
| E024-E035 | Declaration errors | mangle-semantic |
| E036-E046 | Advanced semantic | mangle-semantic |

---

## Parse Errors

### P001 - Parse Error

**Severity**: Error
**Source**: mangle-parse or mangle-lexer

Syntax error in the source file.

**Examples**:
- Missing period at end of clause
- Unbalanced parentheses
- Invalid token

---

## Variable Binding Errors

### E001 - Variable in Fact Head

**Severity**: Error
**Message**: Variable 'X' in fact head must be ground (facts cannot have variables)

Facts (clauses without a body) must have all ground terms.

**Bad**: `bad_fact(X).` - X is not ground

**Good**: `good_fact("alice").`

### E002 - Range Restriction Violation

**Severity**: Error
**Message**: Variable 'X' in head is not bound in the body (range restriction violation)

All variables in the head must appear in a positive atom in the body.

**Bad**: `bad_rule(X, Y) :- parent(X, _).` - Y is unbound

**Good**: `good_rule(X, Y) :- parent(X, Y).`

### E003 - Unbound Variable in Negation

**Severity**: Error
**Message**: Variable 'X' in negated atom must be bound before the negation

Variables in negated atoms must be bound by a positive atom first.

**Bad**: `bad_neg(R) :- !parent(X, "bob"), R = "orphan".` - X unbound before negation

**Good**: `good_neg(X) :- person(X), !parent(_, X).`

### E004 - Unbound Variable in Comparison

**Severity**: Error
**Message**: Variable 'X' must be bound before comparison

Comparison predicates require both arguments to be bound.

**Bad**: `bad_cmp(Name) :- :gt(Age, 18), person(Name, Age).` - Age unbound

**Good**: `good_cmp(Name) :- person(Name, Age), :gt(Age, 18).`

---

## Built-in Predicate/Function Errors

### E005 - Unknown Built-in Predicate

**Severity**: Error
**Message**: Unknown built-in predicate ':xyz'

The built-in predicate does not exist.

**Common mistakes**:
- :greater_than should be :gt
- :greaterThan should be :gt
- :string_concat should be fn:string_concat

### E006 - Built-in Arity Mismatch

**Severity**: Error
**Message**: Built-in ':lt' expects 2 arguments, got 1

### E007 - Built-in Mode Violation

**Severity**: Error
**Message**: Argument 1 of ':lt' must be bound (input mode)

### E008 - Unknown Built-in Function

**Severity**: Error
**Message**: Unknown function 'fn:xyz'

### E009 - Function Arity Mismatch

**Severity**: Error
**Message**: Function 'fn:add' expects 2 arguments, got 3

---

## Transform Errors

### E010 - Invalid Let Variable

**Severity**: Error
**Message**: Let variable must be a fresh variable

### E011 - Invalid Transform Structure

**Severity**: Error
**Message**: Transform must start with 'do fn:group_by(...)', found 'fn:collect'

Transforms must begin with `do fn:group_by(...)`.

**Bad**: `bad(Sum) :- numbers(N) |> do fn:collect(N), let Sum = fn:sum(N).`

**Good**: `good(Sum) :- numbers(N) |> do fn:group_by(), let Sum = fn:sum(N).`

### E012 - Unbound Variable in group_by

**Severity**: Error
**Message**: Variable 'X' in group_by must be bound in the body

### E013 - Invalid Function in Let

**Severity**: Error
**Message**: Invalid aggregation function in let

### E014 - Unbound Variable in Function Application

**Severity**: Error
**Message**: Variable 'Z' in function application must be bound

**Bad**: `bad(X, Y) :- numbers(X), Y = fn:add(X, Z).` - Z unbound

---

## Stratification Errors

### E015 - Negation Cycle

**Severity**: Error
**Message**: Stratification violation: negation cycle detected involving predicates: p/1 -> q/1

A predicate cannot depend negatively on itself (directly or transitively).

**Bad**:
```
p(X) :- numbers(X), !q(X).
q(X) :- numbers(X), !p(X).  # Cycle: p -> q -> p
```

### E016 - Missing Base Case

**Severity**: Warning
**Message**: Predicate 'p/2' has recursive rules but no base case - may not terminate

### E018 - Wrong Function Casing

**Severity**: Error
**Message**: Function 'fn:Sum' has wrong casing. Use 'fn:sum' instead

Functions must be lowercase after fn:

### E019 - Cartesian Explosion Warning

**Severity**: Warning
**Message**: Potential Cartesian explosion: predicates 'p' and 'q' have no shared variables

Reorder atoms to join on shared variables first.

### E020 - Hallucinated Function

**Severity**: Error
**Message**: Function 'fn:concat' does not exist. Did you mean 'fn:string_concat'?

### E021 - Late Filtering Warning

**Severity**: Warning
**Message**: Late filtering: comparison appears after 3 predicates. Consider moving filters earlier

### E023 - Massive Cartesian Product

**Severity**: Warning
**Message**: Massive Cartesian product: predicates 'a', 'b', and 'c' have no shared variables

---

## Declaration Errors

### E024 - Invalid Declaration Argument

**Severity**: Error
**Message**: Declaration argument must be a variable

### E025 - Declaration Bounds Mismatch

**Severity**: Error
**Message**: Expected 2 bounds, got 3

### E026 - External Predicate Mode Error

**Severity**: Error
**Message**: External predicate must have exactly one mode

### E030 - Invalid Pattern Argument

**Severity**: Error
**Message**: Pattern argument must be a variable or constant

### E031 - Package Name Case

**Severity**: Error
**Message**: Package name must be lowercase

---

## Advanced Semantic Errors

### E035 - Division by Zero

**Severity**: Error
**Message**: Division by zero

### E036 - Invalid group_by Argument Type

**Severity**: Error
**Message**: Arguments to fn:group_by must be variables, got Constant

### E037 - Duplicate Variable in group_by

**Severity**: Error
**Message**: Duplicate variable 'X' in fn:group_by - all arguments must be distinct

### E038 - Invalid String Escape

**Severity**: Error
**Message**: Invalid escape sequence in string

### E039 - Wildcard in Head

**Severity**: Warning
**Message**: Wildcard '_' in head is unusual - this argument will be unbound in derived facts

### E040 - Predicate Arity Mismatch

**Severity**: Error
**Message**: Predicate 'parent' used with arity 3, but declared with arity 2

### E041 - Private Predicate Access

**Severity**: Error
**Message**: Cannot access private predicate 'internal_helper' from outside its package

### E043 - Transform Redefines Variable

**Severity**: Error
**Message**: Transform let redefines body variable 'X'

### E044 - Duplicate Declaration

**Severity**: Error
**Message**: Duplicate declaration for predicate 'foo/2'

### E045 - Transform Without Body

**Severity**: Error
**Message**: Transform must have a body before the pipe

### E046 - Declaration Arity Mismatch

**Severity**: Error
**Message**: Declaration arity 2 does not match usage arity 3

---

## Valid Built-in Predicates

| Predicate | Arity | Description |
|-----------|-------|-------------|
| :lt | 2 | Less than |
| :le | 2 | Less than or equal |
| :gt | 2 | Greater than |
| :ge | 2 | Greater than or equal |
| :eq | 2 | Equal |
| :ne | 2 | Not equal |
| :match_prefix | 2 | Name prefix matching |
| :match_pair | 3 | Pair destructuring |
| :match_cons | 3 | List cons cell |
| :match_nil | 1 | Empty list check |
| :match_field | 3 | Struct field access |
| :list:member | 2 | List membership |
| :filter | 1 | Boolean filter |

## Valid Built-in Functions

| Function | Arity | Description |
|----------|-------|-------------|
| fn:add | 2 | Addition |
| fn:sub | 2 | Subtraction |
| fn:mult | 2 | Multiplication |
| fn:div | 2 | Division |
| fn:float_div | 2 | Float division |
| fn:mod | 2 | Modulo |
| fn:abs | 1 | Absolute value |
| fn:sum | 1 | Aggregation: sum |
| fn:count | 0 | Aggregation: count |
| fn:max | 1 | Aggregation: max |
| fn:min | 1 | Aggregation: min |
| fn:collect | 1 | Aggregation: collect to list |
| fn:group_by | 0+ | Group by variables |
| fn:string_concat | 2+ | String concatenation |
| fn:list | 0+ | Create list |
| fn:pair | 2 | Create pair |
| fn:cons | 2 | List cons |
| fn:append | 2 | List append |
| fn:len | 1 | List/string length |
