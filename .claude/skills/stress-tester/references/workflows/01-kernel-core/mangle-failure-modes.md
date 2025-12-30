# Mangle AI Failure Modes Stress Test

Comprehensive stress test for all 69 Mangle failure modes documented in AI_FAILURE_MODES.md.

## Overview

This test validates the Mangle kernel's ability to detect and reject invalid code patterns that AI agents commonly generate. Tests are organized by failure category.

**Expected Duration:** 30-60 minutes total

**Categories Covered:**
- Syntactic Hallucinations (8 tests)
- Semantic Safety & Logic (8 tests)
- Data Types & Functions (7 tests)
- Data Structure Access (4 tests)
- Go Integration (6 tests)
- Architectural Anti-Patterns (5 tests)
- Aggregation & Grouping (8 tests)
- Recursion & Termination (6 tests)
- Closed World Assumption (3 tests)
- Miscellaneous (14 tests)

## Prerequisites

```bash
# Build codeNERD
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd

# Clear logs
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue

# Verify kernel boots
./nerd.exe status

# Create test directory
New-Item -ItemType Directory -Path .nerd/test/failure_modes -Force
```

---

## Category 1: Syntactic Hallucinations (8 tests)

Tests for syntax errors from LLM training bias on SQL/Prolog/Soufflé.

### Test 1.1: Atom vs String Confusion

**Invalid Pattern:**
```mangle
# WRONG: Using string instead of atom
status(/user1, "active").
active_users(U) :- status(U, "active").
```

**Expected Behavior:** Empty results (silent failure - compiles but no unification)

**Verification:**
```bash
$test = @"
status(/user1, /active).
status(/user2, /inactive).
Decl active_users(U.Type<n>).
active_users(U) :- status(U, "active").
"@
$test | Out-File -FilePath .nerd/test/failure_modes/atom_string.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/atom_string.mg
```

**Pass Criteria:** Parser accepts but query returns zero results

---

### Test 1.2: Soufflé Declaration Syntax

**Invalid Pattern:**
```mangle
# WRONG: Soufflé syntax
.decl edge(x:number, y:number)
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = ".decl edge(x:number, y:number)"
$test | Out-File -FilePath .nerd/test/failure_modes/souffle_decl.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/souffle_decl.mg 2>&1
```

**Pass Criteria:** Error contains "syntax error" or "unexpected token"

---

### Test 1.3: Lowercase Variables

**Invalid Pattern:**
```mangle
# WRONG: Prolog-style lowercase variables
ancestor(x, y) :- parent(x, y).
```

**Expected Behavior:** Parser treats as constants, unexpected behavior

**Verification:**
```bash
$test = @"
Decl parent(A.Type<n>, B.Type<n>).
Decl ancestor(A.Type<n>, B.Type<n>).
parent(/alice, /bob).
ancestor(x, y) :- parent(x, y).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/lowercase_vars.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/lowercase_vars.mg
```

**Pass Criteria:** Parses but produces no results (x,y treated as atoms)

---

### Test 1.4: Inline Aggregation (SQL Style)

**Invalid Pattern:**
```mangle
# WRONG: SQL-style aggregation
total(Sum) :- item(X), Sum = sum(X).
```

**Expected Behavior:** Parse error or undefined function

**Verification:**
```bash
$test = @"
Decl item(X.Type<int>).
Decl total(Sum.Type<int>).
item(10).
item(20).
item(30).
total(Sum) :- item(X), Sum = sum(X).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/inline_agg.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/inline_agg.mg 2>&1
```

**Pass Criteria:** Error contains "undefined" or "unknown function"

---

### Test 1.5: Missing Periods

**Invalid Pattern:**
```mangle
# WRONG: Missing statement terminator
parent(/alice, /bob)
child(/bob, /alice)
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl parent(A.Type<n>, B.Type<n>).
parent(/alice, /bob)
parent(/bob, /charlie).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/missing_period.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/missing_period.mg 2>&1
```

**Pass Criteria:** Error contains "expected" or "syntax error"

---

### Test 1.6: Wrong Comment Syntax

**Invalid Pattern:**
```mangle
// This is a comment (C++ style)
/* Block comment */ (C style)
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
// This is a wrong comment
Decl test(X.Type<int>).
test(42).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/wrong_comments.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/wrong_comments.mg 2>&1
```

**Pass Criteria:** Error or unexpected behavior

---

### Test 1.7: Assignment vs Unification

**Invalid Pattern:**
```mangle
# WRONG: Using := or let outside transform
result(X) :- X := 5.
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl result(X.Type<int>).
result(X) :- X := 5.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/wrong_assignment.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/wrong_assignment.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 1.8: Implicit Grouping

**Invalid Pattern:**
```mangle
# WRONG: Assuming automatic GROUP BY like SQL
region_sales(Region, Total) :- sales(Region, Amount), Total = fn:Sum(Amount).
```

**Expected Behavior:** Error or incorrect results

**Verification:**
```bash
$test = @"
Decl sales(Region.Type<n>, Amount.Type<int>).
Decl region_sales(Region.Type<n>, Total.Type<int>).
sales(/north, 100).
sales(/north, 200).
sales(/south, 150).
region_sales(Region, Total) :- sales(Region, Amount), Total = fn:Sum(Amount).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/implicit_group.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/implicit_group.mg 2>&1
```

**Pass Criteria:** Error or empty results

---

## Category 2: Semantic Safety & Logic (8 tests)

Tests for logical validity violations.

### Test 2.1: Unsafe Head Variables

**Invalid Pattern:**
```mangle
# WRONG: X appears in head but not in body
result(X) :- other(Y).
```

**Expected Behavior:** Safety violation error

**Verification:**
```bash
$test = @"
Decl other(Y.Type<int>).
Decl result(X.Type<int>).
other(42).
result(X) :- other(Y).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/unsafe_head.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/unsafe_head.mg 2>&1
```

**Pass Criteria:** Error contains "unsafe" or "unbounded variable"

---

### Test 2.2: Unsafe Negation

**Invalid Pattern:**
```mangle
# WRONG: Variable in negation not bound
safe(X) :- not distinct(X).
```

**Expected Behavior:** Safety violation error

**Verification:**
```bash
$test = @"
Decl distinct(X.Type<int>).
Decl safe(X.Type<int>).
distinct(5).
safe(X) :- not distinct(X).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/unsafe_negation.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/unsafe_negation.mg 2>&1
```

**Pass Criteria:** Error contains "unsafe" or "unbound in negation"

---

### Test 2.3: Stratification Cycle

**Invalid Pattern:**
```mangle
# WRONG: Negative cycle
p(X) :- not q(X).
q(X) :- not p(X).
```

**Expected Behavior:** Stratification error

**Verification:**
```bash
$test = @"
Decl p(X.Type<int>).
Decl q(X.Type<int>).
p(X) :- not q(X).
q(X) :- not p(X).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/stratification.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/stratification.mg 2>&1
```

**Pass Criteria:** Error contains "stratification" or "cycle"

---

### Test 2.4: Infinite Recursion (Counter)

**Invalid Pattern:**
```mangle
# WRONG: Unbounded counter generation
count(N) :- count(M), N = fn:plus(M, 1).
```

**Expected Behavior:** Gas limit exceeded or timeout

**Verification:**
```bash
$test = @"
Decl count(N.Type<int>).
count(0).
count(N) :- count(M), N = fn:plus(M, 1).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/infinite_counter.mg -Encoding utf8
# This will timeout or hit gas limit
timeout 10 ./nerd.exe check-mangle .nerd/test/failure_modes/infinite_counter.mg 2>&1
```

**Pass Criteria:** Timeout, gas limit, or derivation explosion error

---

### Test 2.5: Cartesian Product Explosion

**Invalid Pattern:**
```mangle
# WRONG: Inefficient join order
slow(X, Y) :- user(X), user(Y), friends(X, Y).
```

**Expected Behavior:** Slow performance or resource exhaustion

**Verification:**
```bash
$test = @"
Decl user(U.Type<int>).
Decl friends(A.Type<int>, B.Type<int>).
Decl slow(X.Type<int>, Y.Type<int>).

# Generate many users
user(1). user(2). user(3). user(4). user(5).
user(6). user(7). user(8). user(9). user(10).

# Few friendships
friends(1, 2).
friends(3, 4).

# Bad join order
slow(X, Y) :- user(X), user(Y), friends(X, Y).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/cartesian.mg -Encoding utf8
timeout 5 ./nerd.exe check-mangle .nerd/test/failure_modes/cartesian.mg 2>&1
```

**Pass Criteria:** Completes but may be slow (10x10=100 checks for 2 results)

---

### Test 2.6: Null Checking (Open World)

**Invalid Pattern:**
```mangle
# WRONG: Checking for null (not a Mangle concept)
check(X) :- data(X), X != null.
```

**Expected Behavior:** Parse error (null not defined)

**Verification:**
```bash
$test = @"
Decl data(X.Type<int>).
Decl check(X.Type<int>).
data(42).
check(X) :- data(X), X != null.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/null_check.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/null_check.mg 2>&1
```

**Pass Criteria:** Error about undefined "null"

---

### Test 2.7: Duplicate Rule Definition Misunderstanding

**Invalid Pattern:**
```mangle
# AI thinks second rule overwrites first
p(1).
p(2).  # AI expects only p(2) to exist
```

**Expected Behavior:** Both facts exist (union)

**Verification:**
```bash
$test = @"
Decl p(X.Type<int>).
p(1).
p(2).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/duplicate_rules.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/duplicate_rules.mg
# Query should return both
```

**Pass Criteria:** Both facts present (not a failure, but tests understanding)

---

### Test 2.8: Anonymous Variable Misuse

**Invalid Pattern:**
```mangle
# WRONG: Using _ when value needed later
result(X) :- data(_, X), process(X, _).
```

**Expected Behavior:** Works correctly (tests understanding, not failure)

**Verification:**
```bash
$test = @"
Decl data(A.Type<int>, B.Type<int>).
Decl process(B.Type<int>, C.Type<int>).
Decl result(X.Type<int>).
data(1, 10).
data(2, 20).
process(10, 100).
result(X) :- data(_, X), process(X, _).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/anon_var.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/anon_var.mg
```

**Pass Criteria:** Works correctly (result(10) derived)

---

## Category 3: Data Types & Functions (7 tests)

Tests for type system and function hallucinations.

### Test 3.1: Map Dot Notation

**Invalid Pattern:**
```mangle
# WRONG: JavaScript-style field access
bad(Name) :- record(R), Name = R.name.
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl record(R.Type<{/name: string}>).
Decl bad(Name.Type<string>).
record({/name: "Alice"}).
bad(Name) :- record(R), Name = R.name.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/dot_notation.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/dot_notation.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 3.2: List Indexing

**Invalid Pattern:**
```mangle
# WRONG: Array bracket notation
head(H) :- list(L), H = L[0].
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl list(L.Type<[int]>).
Decl head(H.Type<int>).
list([1, 2, 3]).
head(H) :- list(L), H = L[0].
"@
$test | Out-File -FilePath .nerd/test/failure_modes/list_index.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/list_index.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 3.3: Type Mismatch (Int vs Float)

**Invalid Pattern:**
```mangle
# WRONG: Using int literal for float type
Decl value(X.Type<float>).
value(5).  # Should be 5.0
```

**Expected Behavior:** Type error

**Verification:**
```bash
$test = @"
Decl value(X.Type<float>).
value(5).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/type_mismatch.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/type_mismatch.mg 2>&1
```

**Pass Criteria:** Type error or no match

---

### Test 3.4: String Interpolation

**Invalid Pattern:**
```mangle
# WRONG: Template string syntax
msg("Error: $Code").
```

**Expected Behavior:** Literal string ($ not interpreted)

**Verification:**
```bash
$test = @"
Decl msg(M.Type<string>).
msg("Error: \$Code").
"@
$test | Out-File -FilePath .nerd/test/failure_modes/string_interp.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/string_interp.mg
```

**Pass Criteria:** Accepts as literal string

---

### Test 3.5: Hallucinated Functions (fn:split)

**Invalid Pattern:**
```mangle
# WRONG: Nonexistent function
parts(P) :- text(T), P = fn:split(T, ",").
```

**Expected Behavior:** Undefined function error

**Verification:**
```bash
$test = @"
Decl text(T.Type<string>).
Decl parts(P.Type<[string]>).
text("a,b,c").
parts(P) :- text(T), P = fn:split(T, ",").
"@
$test | Out-File -FilePath .nerd/test/failure_modes/fn_split.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/fn_split.mg 2>&1
```

**Pass Criteria:** Error about undefined function

---

### Test 3.6: Hallucinated Functions (fn:contains)

**Invalid Pattern:**
```mangle
# WRONG: Nonexistent string contains
match(T) :- text(T), fn:contains(T, "error").
```

**Expected Behavior:** Undefined function error

**Verification:**
```bash
$test = @"
Decl text(T.Type<string>).
Decl match(T.Type<string>).
text("this has error").
match(T) :- text(T), fn:contains(T, "error").
"@
$test | Out-File -FilePath .nerd/test/failure_modes/fn_contains.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/fn_contains.mg 2>&1
```

**Pass Criteria:** Error about undefined function

---

### Test 3.7: Wrong Struct Syntax

**Invalid Pattern:**
```mangle
# WRONG: JSON-style struct
data({"key": "value"}).
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl data(D.Type<Any>).
data({\"key\": \"value\"}).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/json_struct.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/json_struct.mg 2>&1
```

**Pass Criteria:** Parse error (keys must be atoms with /)

---

## Category 4: Data Structure Access (4 tests)

### Test 4.1: Direct Field Access on Struct

**Invalid Pattern:**
```mangle
# WRONG: Python-style attribute access
person_info(Name) :- person(P), Name = P.name.
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl person(P.Type<{/name: string}>).
Decl person_info(Name.Type<string>).
person({/name: "Bob"}).
person_info(Name) :- person(P), Name = P.name.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/direct_field.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/direct_field.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 4.2: Missing :match_field

**Invalid Pattern:**
```mangle
# WRONG: Direct unification with partial struct
good(Name) :- record(R), R = {/name: Name}.
```

**Expected Behavior:** May work but not idiomatic; test correct pattern

**Verification:**
```bash
$test = @"
Decl record(R.Type<{/name: string, /age: int}>).
Decl good(Name.Type<string>).
record({/name: "Charlie", /age: 30}).
# Correct way
good(Name) :- record(R), :match_field(R, /name, Name).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/match_field.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/match_field.mg
```

**Pass Criteria:** Correct pattern works

---

### Test 4.3: Hallucinated List Functions

**Invalid Pattern:**
```mangle
# WRONG: Python-style list methods
tail(T) :- list(L), T = L.tail().
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl list(L.Type<[int]>).
Decl tail(T.Type<[int]>).
list([1, 2, 3]).
tail(T) :- list(L), T = L.tail().
"@
$test | Out-File -FilePath .nerd/test/failure_modes/list_methods.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/list_methods.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 4.4: Correct List Pattern

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Works correctly

**Verification:**
```bash
$test = @"
Decl my_list(L.Type<[int]>).
Decl head_elem(H.Type<int>).
my_list([10, 20, 30]).
head_elem(H) :- my_list(L), :match_cons(L, H, _).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/correct_list.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/correct_list.mg
```

**Pass Criteria:** Successfully derives head_elem(10)

---

## Category 5: Go Integration (6 tests)

Tests for Go/Mangle boundary issues.

### Test 5.1: String-Based Fact API

**Invalid Pattern (Go):**
```go
// WRONG
store.Add("parent", "alice", "bob")
```

**Expected Behavior:** Type error

**Note:** This test requires Go code modification. Skip in CLI-only stress test.

**Pass Criteria:** Compile error in Go

---

### Test 5.2: Missing Parse Step

**Invalid Pattern (Go):**
```go
// WRONG
result := engine.Run("ancestor(X, Y) :- parent(X, Y).")
```

**Expected Behavior:** Compile error (no such method)

**Note:** This test requires Go code modification. Skip in CLI-only stress test.

**Pass Criteria:** Compile error in Go

---

### Test 5.3: External Predicate Binding

**Invalid Pattern:** Ignoring query binding patterns in external predicates

**Note:** This test requires implementing a custom external predicate. Skip in CLI-only stress test.

**Pass Criteria:** Proper handling of bound vs free variables

---

### Test 5.4: Undeclared Predicate Usage

**Invalid Pattern:**
```mangle
# WRONG: Using predicate without Decl
result(X) :- unknown_pred(X).
```

**Expected Behavior:** Undeclared predicate error

**Verification:**
```bash
$test = @"
Decl result(X.Type<int>).
result(X) :- unknown_pred(X).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/undeclared.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/undeclared.mg 2>&1
```

**Pass Criteria:** Error about undeclared predicate

---

### Test 5.5: Wrong Decl Capitalization

**Invalid Pattern:**
```mangle
# WRONG: lowercase decl
decl myPred(X.Type<int>).
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = "decl myPred(X.Type<int>)."
$test | Out-File -FilePath .nerd/test/failure_modes/lowercase_decl.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/lowercase_decl.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 5.6: Missing Type Wrapper

**Invalid Pattern:**
```mangle
# WRONG: Missing Type<>
Decl edge(X.int, Y.int).
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = "Decl edge(X.int, Y.int)."
$test | Out-File -FilePath .nerd/test/failure_modes/missing_type.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/missing_type.mg 2>&1
```

**Pass Criteria:** Parse error

---

## Category 6: Architectural Anti-Patterns (5 tests)

### Test 6.1: Hundreds of String Patterns (DSL Trap)

**Invalid Pattern:**
```mangle
# WRONG: Using Mangle as fuzzy matcher
intent_definition("review my code", /review).
intent_definition("check for bugs", /review).
intent_definition("look at my code", /review).
# ... 400 more
```

**Expected Behavior:** Compiles but only exact matches work

**Verification:**
```bash
$test = @"
Decl intent_definition(Text.Type<string>, Verb.Type<n>).
intent_definition("review my code", /review).
intent_definition("check for bugs", /debug).

Decl matched(V.Type<n>).
# This will NOT match "examine my code"
matched(V) :- intent_definition("examine my code", V).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/dsl_trap.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/dsl_trap.mg
```

**Pass Criteria:** No matches (demonstrates limitation)

---

### Test 6.2: Hallucinated String Functions

**Invalid Pattern:**
```mangle
# ALL OF THESE ARE WRONG
match(T) :- text(T), fn:substring(T, 0, 5) = "Error".
match(T) :- text(T), fn:lower(T) = "error".
match(T) :- text(T), fn:startswith(T, "ERR").
```

**Expected Behavior:** Undefined function errors

**Verification:**
```bash
$test = @"
Decl text(T.Type<string>).
Decl match(T.Type<string>).
text("Error123").
match(T) :- text(T), fn:substring(T, 0, 5) = "Error".
"@
$test | Out-File -FilePath .nerd/test/failure_modes/fn_substring.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/fn_substring.mg 2>&1
```

**Pass Criteria:** Error about undefined function

---

### Test 6.3: Valid Built-in Functions Test

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** All valid functions work

**Verification:**
```bash
$test = @"
Decl num(N.Type<int>).
Decl result(R.Type<int>).
num(5).
num(10).
num(15).

# Test arithmetic
result(R) :- num(A), num(B), A < B, R = fn:plus(A, B).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/valid_funcs.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/valid_funcs.mg
```

**Pass Criteria:** Successfully derives results

---

### Test 6.4: Taxonomy as Data (Wrong)

**Invalid Pattern:**
```mangle
# WRONG: Mixing data declaration with logic
TAXONOMY: /vehicle > /car > /sedan
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
TAXONOMY: /vehicle > /car
Decl test(X.Type<int>).
test(1).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/taxonomy.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/taxonomy.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 6.5: Correct Taxonomy Pattern

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Works with proper fact injection

**Verification:**
```bash
$test = @"
# CORRECT: Facts injected, transitive closure in logic
Decl subclass_of(Child.Type<n>, Parent.Type<n>).
Decl is_subtype(Child.Type<n>, Parent.Type<n>).

subclass_of(/sedan, /car).
subclass_of(/car, /vehicle).

is_subtype(X, Y) :- subclass_of(X, Y).
is_subtype(X, Z) :- subclass_of(X, Y), is_subtype(Y, Z).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/taxonomy_correct.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/taxonomy_correct.mg
```

**Pass Criteria:** Successfully derives is_subtype(/sedan, /vehicle)

---

## Category 7: Aggregation & Grouping (8 tests)

### Test 7.1: Missing `do` Keyword

**Invalid Pattern:**
```mangle
# WRONG: Missing do
region_sales(Region, Total) :-
    sales(Region, Amount) |>
    fn:group_by(Region),
    let Total = fn:Sum(Amount).
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl sales(Region.Type<n>, Amount.Type<int>).
Decl region_sales(Region.Type<n>, Total.Type<int>).
sales(/north, 100).
region_sales(Region, Total) :-
    sales(Region, Amount) |>
    fn:group_by(Region),
    let Total = fn:Sum(Amount).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/missing_do.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/missing_do.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 7.2: Wrong Aggregation Casing

**Invalid Pattern:**
```mangle
# WRONG: lowercase sum
total(T) :- item(X) |> do fn:group_by(), let T = fn:sum(X).
```

**Expected Behavior:** Undefined function error

**Verification:**
```bash
$test = @"
Decl item(X.Type<int>).
Decl total(T.Type<int>).
item(10).
total(T) :- item(X) |> do fn:group_by(), let T = fn:sum(X).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/wrong_casing.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/wrong_casing.mg 2>&1
```

**Pass Criteria:** Error about undefined function

---

### Test 7.3: Correct Aggregation Pattern

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Works correctly

**Verification:**
```bash
$test = @"
Decl sales(Region.Type<n>, Amount.Type<int>).
Decl region_sales(Region.Type<n>, Total.Type<int>).

sales(/north, 100).
sales(/north, 200).
sales(/south, 150).

region_sales(Region, Total) :-
    sales(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:Sum(Amount).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/agg_correct.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/agg_correct.mg
```

**Pass Criteria:** Successfully derives region_sales(/north, 300) and region_sales(/south, 150)

---

### Test 7.4: Prolog findall Hallucination

**Invalid Pattern:**
```mangle
# WRONG: Prolog syntax
total(T) :- findall(Amount, sales(_, Amount), Amounts), sum_list(Amounts, T).
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl sales(R.Type<n>, A.Type<int>).
Decl total(T.Type<int>).
sales(/a, 10).
total(T) :- findall(Amount, sales(_, Amount), Amounts), sum_list(Amounts, T).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/findall.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/findall.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 7.5: Aggregation Without Grouping

**Invalid Pattern:**
```mangle
# Ambiguous: aggregate without explicit grouping
count_items(N) :- item(_) |> let N = fn:Count().
```

**Expected Behavior:** May work (tests understanding)

**Verification:**
```bash
$test = @"
Decl item(X.Type<int>).
Decl count_items(N.Type<int>).
item(1).
item(2).
item(3).
count_items(N) :- item(_) |> let N = fn:Count().
"@
$test | Out-File -FilePath .nerd/test/failure_modes/count_no_group.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/count_no_group.mg
```

**Pass Criteria:** Should derive count_items(3)

---

### Test 7.6: Multiple Aggregations in One Rule

**Invalid Pattern:**
```mangle
# Multiple aggregations - complex pattern
stats(Count, Total) :-
    item(X) |>
    do fn:group_by(),
    let Count = fn:Count(),
    let Total = fn:Sum(X).
```

**Expected Behavior:** Should work (correct usage)

**Verification:**
```bash
$test = @"
Decl item(X.Type<int>).
Decl stats(Count.Type<int>, Total.Type<int>).
item(10).
item(20).
item(30).
stats(Count, Total) :-
    item(X) |>
    do fn:group_by(),
    let Count = fn:Count(),
    let Total = fn:Sum(X).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/multi_agg.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/multi_agg.mg
```

**Pass Criteria:** Successfully derives stats(3, 60)

---

### Test 7.7: fn:collect Usage

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Collects into list

**Verification:**
```bash
$test = @"
Decl tag(Item.Type<n>, Tag.Type<n>).
Decl all_tags(Item.Type<n>, Tags.Type<[n]>).

tag(/item1, /red).
tag(/item1, /large).
tag(/item2, /blue).

all_tags(Item, Tags) :-
    tag(Item, T) |>
    do fn:group_by(Item),
    let Tags = fn:collect(T).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/collect.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/collect.mg
```

**Pass Criteria:** Successfully collects tags per item

---

### Test 7.8: Unbound Variable in group_by

**Invalid Pattern:**
```mangle
# WRONG: Grouping by unbound variable
bad(X, Total) :-
    sales(_, Amount) |>
    do fn:group_by(X),
    let Total = fn:Sum(Amount).
```

**Expected Behavior:** Safety error

**Verification:**
```bash
$test = @"
Decl sales(R.Type<n>, A.Type<int>).
Decl bad(X.Type<n>, Total.Type<int>).
sales(/a, 10).
bad(X, Total) :-
    sales(_, Amount) |>
    do fn:group_by(X),
    let Total = fn:Sum(Amount).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/unbound_group.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/unbound_group.mg 2>&1
```

**Pass Criteria:** Error about unbound variable

---

## Category 8: Recursion & Termination (6 tests)

### Test 8.1: Safe Graph Traversal

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Terminates correctly

**Verification:**
```bash
$test = @"
Decl edge(X.Type<n>, Y.Type<n>).
Decl reachable(X.Type<n>, Y.Type<n>).

edge(/a, /b).
edge(/b, /c).
edge(/c, /d).

reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/safe_recursion.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/safe_recursion.mg
```

**Pass Criteria:** Successfully derives all reachable pairs

---

### Test 8.2: Bounded Depth Recursion

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Respects depth limit

**Verification:**
```bash
$test = @"
Decl edge(X.Type<n>, Y.Type<n>).
Decl path(X.Type<n>, Y.Type<n>, D.Type<int>).

edge(/a, /b).
edge(/b, /c).
edge(/c, /d).

path(X, Y, 1) :- edge(X, Y).
path(X, Z, D) :-
    edge(X, Y),
    path(Y, Z, D1),
    D = fn:plus(D1, 1),
    D < 10.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/bounded_depth.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/bounded_depth.mg
```

**Pass Criteria:** Successfully derives paths with depth limit

---

### Test 8.3: Unbounded Self-Reference

**Invalid Pattern:**
```mangle
# WRONG: No decreasing measure
grow(X) :- grow(Y), X = fn:plus(Y, 1).
```

**Expected Behavior:** Infinite loop or gas limit

**Verification:**
```bash
$test = @"
Decl grow(X.Type<int>).
grow(0).
grow(X) :- grow(Y), X = fn:plus(Y, 1).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/unbounded_self.mg -Encoding utf8
timeout 5 ./nerd.exe check-mangle .nerd/test/failure_modes/unbounded_self.mg 2>&1
```

**Pass Criteria:** Timeout or gas limit error

---

### Test 8.4: Cycle Detection Test

**Invalid Pattern:**
```mangle
# Cycle in graph
edge(/a, /b).
edge(/b, /c).
edge(/c, /a).  # Cycle!
```

**Expected Behavior:** Handles correctly (not an error)

**Verification:**
```bash
$test = @"
Decl edge(X.Type<n>, Y.Type<n>).
Decl reachable(X.Type<n>, Y.Type<n>).

edge(/a, /b).
edge(/b, /c).
edge(/c, /a).

reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/graph_cycle.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/graph_cycle.mg
```

**Pass Criteria:** Terminates (fixpoint reached)

---

### Test 8.5: Counter with Limit

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Generates limited sequence

**Verification:**
```bash
$test = @"
Decl counter(N.Type<int>).
counter(0).
counter(N) :- counter(M), N = fn:plus(M, 1), N < 5.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/counter_limit.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/counter_limit.mg
```

**Pass Criteria:** Generates 0,1,2,3,4 and stops

---

### Test 8.6: Mutual Recursion (Safe)

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Works correctly

**Verification:**
```bash
$test = @"
Decl even(N.Type<int>).
Decl odd(N.Type<int>).

even(0).
even(N) :- odd(M), N = fn:plus(M, 1), N < 10.
odd(N) :- even(M), N = fn:plus(M, 1), N < 10.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/mutual_recursion.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/mutual_recursion.mg
```

**Pass Criteria:** Generates even and odd numbers up to limit

---

## Category 9: Closed World Assumption (3 tests)

### Test 9.1: No NULL Concept

**Invalid Pattern:**
```mangle
# WRONG: Treating missing as null
handle(X, Status) :- item(X), Status = (known_status(X, S) ? S : /unknown).
```

**Expected Behavior:** Parse error (no ternary operator)

**Verification:**
```bash
$test = @"
Decl item(X.Type<int>).
Decl known_status(X.Type<int>, S.Type<n>).
Decl handle(X.Type<int>, Status.Type<n>).
item(1).
item(2).
known_status(1, /active).
handle(X, Status) :- item(X), Status = (known_status(X, S) ? S : /unknown).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/null_ternary.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/null_ternary.mg 2>&1
```

**Pass Criteria:** Parse error

---

### Test 9.2: Correct Closed World Pattern

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Unknown derived via negation

**Verification:**
```bash
$test = @"
Decl item(X.Type<int>).
Decl known_status(X.Type<int>, S.Type<n>).
Decl known(X.Type<int>).
Decl unknown(X.Type<int>).

item(1).
item(2).
known_status(1, /active).

known(X) :- known_status(X, _).
unknown(X) :- item(X), not known(X).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/cwa_correct.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/cwa_correct.mg
```

**Pass Criteria:** Derives unknown(2)

---

### Test 9.3: CASE/ELSE Hallucination

**Invalid Pattern:**
```mangle
# WRONG: SQL CASE syntax
result(X, Label) :-
    item(X),
    Label = CASE
        WHEN X > 100 THEN /high
        WHEN X > 50 THEN /medium
        ELSE /low
    END.
```

**Expected Behavior:** Parse error

**Verification:**
```bash
$test = @"
Decl item(X.Type<int>).
Decl result(X.Type<int>, Label.Type<n>).
item(75).
result(X, Label) :-
    item(X),
    Label = CASE
        WHEN X > 100 THEN /high
        ELSE /low
    END.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/case_else.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/case_else.mg 2>&1
```

**Pass Criteria:** Parse error

---

## Category 10: Miscellaneous (14 tests)

### Test 10.1: Package Import Hallucination

**Invalid Pattern:**
```mangle
# WRONG: Nonexistent import
use /std/date.
```

**Expected Behavior:** Parse error or ignored

**Verification:**
```bash
$test = @"
use /std/date.
Decl test(X.Type<int>).
test(1).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/fake_import.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/fake_import.mg 2>&1
```

**Pass Criteria:** Error or warning about unknown package

---

### Test 10.2: IO Operations Hallucination

**Invalid Pattern:**
```mangle
# WRONG: Mangle is pure, no IO
content(C) :- read_file("/path/to/file.txt", C).
```

**Expected Behavior:** Undeclared predicate error

**Verification:**
```bash
$test = @"
Decl content(C.Type<string>).
content(C) :- read_file("/path/to/file.txt", C).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/io_ops.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/io_ops.mg 2>&1
```

**Pass Criteria:** Error about undeclared predicate

---

### Test 10.3: Comparison Operators as Functions

**Invalid Pattern:**
```mangle
# WRONG: Treating operators as functions
greater(X, Y) :- value(X), value(Y), fn:greater_than(X, Y).
```

**Expected Behavior:** Undefined function error

**Verification:**
```bash
$test = @"
Decl value(V.Type<int>).
Decl greater(X.Type<int>, Y.Type<int>).
value(10).
value(5).
greater(X, Y) :- value(X), value(Y), fn:greater_than(X, Y).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/fn_comparison.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/fn_comparison.mg 2>&1
```

**Pass Criteria:** Error about undefined function

---

### Test 10.4: Correct Comparison Usage

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Works correctly

**Verification:**
```bash
$test = @"
Decl value(V.Type<int>).
Decl greater(X.Type<int>, Y.Type<int>).
value(10).
value(5).
greater(X, Y) :- value(X), value(Y), X > Y.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/comparison_correct.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/comparison_correct.mg
```

**Pass Criteria:** Derives greater(10, 5)

---

### Test 10.5: Boolean True/False Hallucination

**Invalid Pattern:**
```mangle
# WRONG: Boolean literals
flag(true).
flag(false).
```

**Expected Behavior:** Treated as atoms (need /)

**Verification:**
```bash
$test = @"
Decl flag(F.Type<n>).
flag(true).
flag(false).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/boolean.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/boolean.mg 2>&1
```

**Pass Criteria:** Error (should be /true or /false)

---

### Test 10.6: Correct Boolean Pattern

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Works with atoms

**Verification:**
```bash
$test = @"
Decl flag(F.Type<n>).
flag(/true).
flag(/false).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/boolean_correct.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/boolean_correct.mg
```

**Pass Criteria:** Both facts accepted

---

### Test 10.7: Date/Time Hallucinations

**Invalid Pattern:**
```mangle
# WRONG: Nonexistent date functions
recent(E) :- event(E, T), fn:days_since(T) < 7.
```

**Expected Behavior:** Undefined function error

**Verification:**
```bash
$test = @"
Decl event(E.Type<n>, T.Type<int>).
Decl recent(E.Type<n>).
event(/e1, 1000).
recent(E) :- event(E, T), fn:days_since(T) < 7.
"@
$test | Out-File -FilePath .nerd/test/failure_modes/date_funcs.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/date_funcs.mg 2>&1
```

**Pass Criteria:** Error about undefined function

---

### Test 10.8: Regex Hallucination

**Invalid Pattern:**
```mangle
# WRONG: Pattern matching
match(T) :- text(T), fn:regex(T, "^ERR-\\d+$").
```

**Expected Behavior:** Undefined function error

**Verification:**
```bash
$test = @"
Decl text(T.Type<string>).
Decl match(T.Type<string>).
text("ERR-123").
match(T) :- text(T), fn:regex(T, "^ERR-\\d+\$").
"@
$test | Out-File -FilePath .nerd/test/failure_modes/regex.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/regex.mg 2>&1
```

**Pass Criteria:** Error about undefined function

---

### Test 10.9: Variable Shadowing

**Invalid Pattern:**
```mangle
# Same variable name, different scope
p(X) :- q(X), r(X).
```

**Expected Behavior:** Works correctly (same X must unify)

**Verification:**
```bash
$test = @"
Decl q(X.Type<int>).
Decl r(X.Type<int>).
Decl p(X.Type<int>).
q(5).
q(10).
r(5).
p(X) :- q(X), r(X).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/var_shadow.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/var_shadow.mg
```

**Pass Criteria:** Derives p(5) only

---

### Test 10.10: Complex Atom Paths

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Nested atoms work

**Verification:**
```bash
$test = @"
Decl status(Entity.Type<n>, State.Type<n>).
status(/user/alice, /status/active).
status(/user/bob, /status/pending).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/nested_atoms.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/nested_atoms.mg
```

**Pass Criteria:** Both facts accepted

---

### Test 10.11: Negative Numbers

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Negative ints work

**Verification:**
```bash
$test = @"
Decl value(V.Type<int>).
value(-42).
value(0).
value(42).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/negative.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/negative.mg
```

**Pass Criteria:** All values accepted

---

### Test 10.12: Empty List

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Empty lists work

**Verification:**
```bash
$test = @"
Decl tags(Item.Type<n>, Tags.Type<[n]>).
tags(/item1, []).
tags(/item2, [/red, /large]).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/empty_list.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/empty_list.mg
```

**Pass Criteria:** Both facts accepted

---

### Test 10.13: Unicode in Strings

**Invalid Pattern:** N/A (correct usage test)

**Expected Behavior:** Unicode strings work

**Verification:**
```bash
$test = @"
Decl message(M.Type<string>).
message("Hello 世界 🌍").
"@
$test | Out-File -FilePath .nerd/test/failure_modes/unicode.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/unicode.mg
```

**Pass Criteria:** Fact accepted

---

### Test 10.14: Very Long Atom Names

**Invalid Pattern:** N/A (stress test)

**Expected Behavior:** Long names work

**Verification:**
```bash
$test = @"
Decl very_long_predicate_name_that_tests_parser_limits(X.Type<int>).
very_long_predicate_name_that_tests_parser_limits(42).
"@
$test | Out-File -FilePath .nerd/test/failure_modes/long_names.mg -Encoding utf8
./nerd.exe check-mangle .nerd/test/failure_modes/long_names.mg
```

**Pass Criteria:** Fact accepted

---

## Conservative Test Suite (10 min)

Run all syntactic tests (should fail fast).

```bash
# Run all syntax tests
Get-ChildItem .nerd/test/failure_modes/*.mg | ForEach-Object {
    Write-Host "`n=== Testing $($_.Name) ===" -ForegroundColor Cyan
    ./nerd.exe check-mangle $_.FullName 2>&1 | Select-Object -First 10
}
```

**Success Criteria:**
- [ ] All invalid patterns rejected
- [ ] All correct patterns accepted
- [ ] Clear error messages for failures

---

## Aggressive Test Suite (20 min)

Run all tests including recursion and aggregation.

```bash
# Run comprehensive test
$results = @()
Get-ChildItem .nerd/test/failure_modes/*.mg | ForEach-Object {
    Write-Host "`n=== Testing $($_.Name) ===" -ForegroundColor Cyan
    $start = Get-Date
    $output = timeout 10 ./nerd.exe check-mangle $_.FullName 2>&1
    $duration = (Get-Date) - $start

    $results += [PSCustomObject]@{
        Test = $_.Name
        Duration = $duration.TotalSeconds
        Output = $output -join "`n"
    }

    Write-Host "Duration: $($duration.TotalSeconds)s"
}

# Summary
$results | Format-Table Test, Duration -AutoSize
```

**Success Criteria:**
- [ ] All tests complete within timeout
- [ ] Infinite recursion tests timeout or hit gas limit
- [ ] No kernel panics
- [ ] Validation errors are informative

---

## Chaos Test Suite (30 min)

Load invalid patterns into kernel via task execution.

### Step 1: Attempt to Load Invalid Schema

```bash
# Copy invalid schema to .nerd/mangle
Copy-Item .nerd/test/failure_modes/unsafe_negation.mg .nerd/mangle/test_schema.mg

# Restart kernel (should detect invalid schema)
./nerd.exe restart
```

**Expected:** Kernel rejects invalid schema, falls back to known-good

### Step 2: Runtime Derivation Explosion

```bash
# Create a task that would trigger unbounded derivation
./nerd.exe run "analyze all possible paths in the dependency graph showing every transitive relationship at every depth"
```

Wait 5 minutes.

**Expected:** Gas limit prevents explosion, system recovers

### Step 3: Concurrent Invalid Queries

```bash
# Run multiple problematic queries
1..5 | ForEach-Object {
    Start-Job -ScriptBlock {
        ./nerd.exe query "undefined_predicate" 2>&1
    }
}

Get-Job | Wait-Job | Receive-Job
```

**Expected:** All queries fail gracefully, no crashes

**Success Criteria:**
- [ ] Kernel remains stable
- [ ] Invalid schemas rejected at startup
- [ ] Gas limit prevents runaway derivation
- [ ] System recovers from errors

---

## Hybrid Test (60 min)

Combine failure mode injection with system stress.

### Phase 1: Baseline (10 min)

```bash
# Normal operation
./nerd.exe status
./nerd.exe query "system_boot"
./nerd.exe scan
```

### Phase 2: Inject Invalid Patterns (20 min)

```bash
# Try to execute tasks that would generate invalid Mangle
./nerd.exe run "create a rule that finds all users where status is null"
./nerd.exe run "generate a recursive rule that counts from 1 to infinity"
./nerd.exe run "write a rule using SQL GROUP BY syntax"
```

**Expected:** LLM may generate invalid code, but kernel rejects it

### Phase 3: Stress Under Failure (20 min)

```bash
# Campaign with potential for invalid derivations
./nerd.exe campaign start "analyze the entire codebase, create recursive dependency graphs with unbounded depth, and check all possible paths"
```

Wait 15 minutes.

### Phase 4: Recovery (10 min)

```bash
./nerd.exe status
./nerd.exe query "system_boot"
./nerd.exe logic | Measure-Object -Line
```

**Success Criteria:**
- [ ] Invalid LLM output rejected
- [ ] Kernel remains stable throughout
- [ ] System recovers after campaign
- [ ] No corrupted state

---

## Post-Test Analysis

### Log Review

```bash
# Check for Mangle errors
Select-String -Path ".nerd/logs/*kernel*.log" -Pattern "parse|syntax|safety|stratification|gas|undeclared"

# Check for panics
Select-String -Path ".nerd/logs/*.log" -Pattern "panic|fatal|CRITICAL"
```

### Test Results Summary

Create a summary of all 69 tests:

```bash
$summary = @"
# Mangle Failure Modes Test Results

## Syntactic (8 tests)
- Atom/String: [PASS/FAIL]
- Soufflé Decl: [PASS/FAIL]
- Lowercase Vars: [PASS/FAIL]
- Inline Agg: [PASS/FAIL]
- Missing Period: [PASS/FAIL]
- Wrong Comments: [PASS/FAIL]
- Wrong Assignment: [PASS/FAIL]
- Implicit Group: [PASS/FAIL]

## Safety (8 tests)
- Unsafe Head: [PASS/FAIL]
- Unsafe Negation: [PASS/FAIL]
- Stratification: [PASS/FAIL]
- Infinite Counter: [PASS/FAIL]
- Cartesian: [PASS/FAIL]
- Null Check: [PASS/FAIL]
- Duplicate Rules: [PASS/FAIL]
- Anon Var: [PASS/FAIL]

## Types (7 tests)
- Dot Notation: [PASS/FAIL]
- List Index: [PASS/FAIL]
- Type Mismatch: [PASS/FAIL]
- String Interp: [PASS/FAIL]
- fn:split: [PASS/FAIL]
- fn:contains: [PASS/FAIL]
- JSON Struct: [PASS/FAIL]

## Data Structures (4 tests)
- Direct Field: [PASS/FAIL]
- match_field: [PASS/FAIL]
- List Methods: [PASS/FAIL]
- Correct List: [PASS/FAIL]

## Go Integration (6 tests)
- String API: [SKIP/PASS/FAIL]
- Missing Parse: [SKIP/PASS/FAIL]
- External Pred: [SKIP/PASS/FAIL]
- Undeclared: [PASS/FAIL]
- Lowercase Decl: [PASS/FAIL]
- Missing Type: [PASS/FAIL]

## Architecture (5 tests)
- DSL Trap: [PASS/FAIL]
- String Funcs: [PASS/FAIL]
- Valid Funcs: [PASS/FAIL]
- Taxonomy Data: [PASS/FAIL]
- Taxonomy Correct: [PASS/FAIL]

## Aggregation (8 tests)
- Missing do: [PASS/FAIL]
- Wrong Casing: [PASS/FAIL]
- Correct Agg: [PASS/FAIL]
- findall: [PASS/FAIL]
- Count No Group: [PASS/FAIL]
- Multi Agg: [PASS/FAIL]
- collect: [PASS/FAIL]
- Unbound Group: [PASS/FAIL]

## Recursion (6 tests)
- Safe Graph: [PASS/FAIL]
- Bounded Depth: [PASS/FAIL]
- Unbounded Self: [PASS/FAIL]
- Graph Cycle: [PASS/FAIL]
- Counter Limit: [PASS/FAIL]
- Mutual Recursion: [PASS/FAIL]

## Closed World (3 tests)
- No NULL: [PASS/FAIL]
- CWA Correct: [PASS/FAIL]
- CASE/ELSE: [PASS/FAIL]

## Miscellaneous (14 tests)
- Fake Import: [PASS/FAIL]
- IO Ops: [PASS/FAIL]
- fn:comparison: [PASS/FAIL]
- Comparison Correct: [PASS/FAIL]
- Boolean: [PASS/FAIL]
- Boolean Correct: [PASS/FAIL]
- Date Funcs: [PASS/FAIL]
- Regex: [PASS/FAIL]
- Var Shadow: [PASS/FAIL]
- Nested Atoms: [PASS/FAIL]
- Negative: [PASS/FAIL]
- Empty List: [PASS/FAIL]
- Unicode: [PASS/FAIL]
- Long Names: [PASS/FAIL]

## Overall Stats
- Total Tests: 69
- Passed: X
- Failed: Y
- Skipped: Z (Go integration)
"@

$summary | Out-File -FilePath .nerd/test/failure_modes_report.md
```

### Success Criteria

**Overall Test Pass:**
- [ ] All 69 failure modes behave as expected
- [ ] Invalid patterns properly rejected
- [ ] Correct patterns properly accepted
- [ ] Kernel remained stable throughout
- [ ] Clear, actionable error messages
- [ ] No false positives or negatives

### Known Issues to Watch For

- `syntax error` - Expected for wrong syntax patterns
- `undeclared predicate` - Expected for undefined functions
- `unsafe variable` - Expected for safety violations
- `stratification error` - Expected for negative cycles
- `gas limit exceeded` - Expected for infinite loops
- `type error` - Expected for type mismatches
- **NOT EXPECTED:** Kernel panic, silent failures, unclear errors

---

## Cleanup

```bash
# Remove test files
Remove-Item .nerd/test/failure_modes -Recurse -Force

# Clear logs
Remove-Item .nerd/logs/* -Force

# Restart kernel
./nerd.exe restart
```
