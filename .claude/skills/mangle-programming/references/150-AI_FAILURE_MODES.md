# 150: AI Failure Modes and Anti-Patterns

**Purpose**: Comprehensive guide to common mistakes when generating Mangle code. Essential reading before writing any Mangle logic.

## Critical Understanding: Why AI Agents Fail at Mangle

Mangle operates on fundamentally different principles than languages like Python, SQL, or Prolog:

| Paradigm | LLM Training Bias | Mangle Reality |
|----------|-------------------|----------------|
| **Evaluation** | Procedural/lazy | Bottom-up fixpoint (all facts computed) |
| **World Assumption** | Open (unknown ≠ false) | Closed (unknown = false) |
| **Negation** | Boolean NOT | Stratified, requires variable binding |
| **Aggregation** | Implicit GROUP BY | Explicit `\|>` transform pipeline |
| **Constants** | Strings everywhere | Atoms (`/name`) are distinct type |

---

## 1. Atom vs String Confusion (CRITICAL)

### The Problem

In Mangle, `/atom` and `"string"` are **completely different types**. They cannot unify.

### Failure Spectrum

| Concept | CORRECT Mangle | WRONG (AI Hallucination) | Training Bias |
|---------|----------------|--------------------------|---------------|
| **Constant** | `/active` | `'active'` or `"active"` | Python/SQL strings |
| **Enum Value** | `/status/pending` | `status.pending` or `:pending` | Java/Clojure |
| **Status Flag** | `/enabled` | `true` or `"enabled"` | Boolean or string |
| **Identifier** | `/log4j` | `"log4j"` | JSON dominance |

### Why This Matters

```mangle
# Facts stored with atoms
status(/user1, /active).
status(/user2, /inactive).

# WRONG - will return NOTHING (string doesn't match atom)
active_users(U) :- status(U, "active").

# CORRECT
active_users(U) :- status(U, /active).
```

### The Silent Killer

This error **compiles successfully** but returns empty results. The program runs, appears to work, but produces no output because `"active"` and `/active` never unify.

### Rule

**ALWAYS use `/atom` syntax for:**
- Identifiers (`/user_id`, `/project_name`)
- Enum values (`/critical`, `/warning`, `/info`)
- Status flags (`/active`, `/pending`, `/done`)
- Category labels (`/frontend`, `/backend`, `/database`)

**ONLY use `"string"` for:**
- Human-readable text (`"John Doe"`, `"Error message"`)
- External data that genuinely varies (`"CVE-2021-44228"`)
- Content that may contain spaces/special chars

---

## 2. Aggregation Syntax Errors (HIGH FREQUENCY)

### The Problem

Mangle requires explicit `|>` pipeline syntax for aggregation. SQL-style implicit grouping does NOT work.

### Common Hallucinations

```mangle
# WRONG - SQL/Soufflé mental model
region_sales(Region, Total) :-
    sales(Region, Amount),
    Total = sum(Amount).  # This is NOT how Mangle works!

# WRONG - Prolog findall mental model
region_sales(Region, Total) :-
    findall(Amount, sales(Region, Amount), Amounts),
    sum_list(Amounts, Total).

# WRONG - Missing `do` keyword
region_sales(Region, Total) :-
    sales(Region, Amount) |>
    fn:group_by(Region),  # Missing `do`!
    let Total = fn:Sum(Amount).
```

### Correct Syntax

```mangle
# CORRECT - Full pipeline syntax
region_sales(Region, Total) :-
    sales(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:Sum(Amount).
```

### Required Keywords

| Keyword | Purpose | Example |
|---------|---------|---------|
| `\|>` | Pipeline operator | `source() \|> ...` |
| `do` | Apply transform function | `do fn:group_by(X)` |
| `let` | Bind aggregation result | `let N = fn:Count()` |
| `fn:` | Function namespace | `fn:Sum`, `fn:Count`, `fn:group_by` |

### Function Casing

**CRITICAL**: Aggregation functions use specific casing:
- `fn:Count()` - Capital C
- `fn:Sum(X)` - Capital S
- `fn:Min(X)`, `fn:Max(X)` - Capital M
- `fn:group_by(X)` - lowercase

---

## 3. Type Declaration Syntax (HIGH FREQUENCY)

### The Problem

Mangle's `Decl` syntax is unique and AI frequently hallucinates Soufflé or other syntax.

### Hallucination Examples

```mangle
# WRONG - Soufflé syntax
.decl direct_dep(app: string, lib: string)

# WRONG - TypeScript style
type DirectDep = { app: string, lib: string }

# WRONG - SQL style
CREATE TABLE direct_dep (app VARCHAR, lib VARCHAR)

# WRONG - Missing Type<> wrapper
Decl direct_dep(App.string, Lib.string).
```

### Correct Syntax

```mangle
# CORRECT
Decl direct_dep(App.Type<string>, Lib.Type<string>).

# With name atoms
Decl status(Entity.Type<n>, State.Type<n>).

# With list type
Decl tags(ID.Type<int>, Tags.Type<[string]>).

# With map type
Decl config(ID.Type<int>, Data.Type<{/host: string, /port: int}>).
```

### Type Codes

| Type | Syntax | Examples |
|------|--------|----------|
| Integer | `Type<int>` | `42`, `-17` |
| Float | `Type<float>` | `3.14`, `-2.5` |
| String | `Type<string>` | `"hello"` |
| Name (atom) | `Type<n>` | `/active`, `/user1` |
| List | `Type<[T]>` | `[1, 2, 3]` |
| Map | `Type<{/k: v}>` | `{/x: 10}` |
| Any | `Type<Any>` | (matches anything) |

---

## 4. Safety Violations: Unbounded Variables

### The Problem

Every variable in a negated atom MUST be bound by a positive atom first. The AI often forgets the "generator" predicate.

### The AI Mental Model (Wrong)

```mangle
# Natural language: "Find users who are NOT admins"
# AI thinks: "Just negate admin"
non_admin(User) :- not admin(User).  # UNSAFE - will crash!
```

### Why This Fails

The Mangle engine asks: "What values should I test for `User`?" With bottom-up evaluation, variables represent potentially infinite domains. The engine cannot iterate over "all possible users in the universe."

### The Expert Fix

```mangle
# CORRECT - User is bound by user() first
non_admin(User) :- user(User), not admin(User).
#                  ^^^^^^^^^^^ Generator predicate
```

### Safety Checklist

Before any negation `not pred(X, Y, ...)`:
1. Is every variable already bound by a positive atom?
2. Does the positive atom appear BEFORE the negation?

```mangle
# SAFE patterns
safe(X) :- candidate(X), not excluded(X).
safe(X, Y) :- source(X, Y), not blocked(X).

# UNSAFE patterns - WILL FAIL
bad(X) :- not foo(X).              # X unbound
bad(X, Y) :- foo(X), not bar(Y).   # Y unbound
bad(X) :- not foo(X), source(X).   # X unbound when negation evaluated
```

---

## 5. Stratification Violations: Negative Cycles

### The Problem

Mangle rejects programs where negation creates circular dependencies.

### Classic Game Theory Failure

```mangle
# AI generating game logic (minimax-style)
winning(X) :- move(X, Y), losing(Y).
losing(X) :- not winning(X).  # STRATIFICATION ERROR!
```

### Why This Fails

The dependency graph:
1. `winning` depends on `losing`
2. `losing` depends on `not winning`
3. Cycle through negation = no stable truth value

### Detection Patterns

Watch for these dependency patterns:

```
A -> not B -> A     (negative cycle)
A -> B -> not A     (negative cycle)
A -> not A          (direct negative self-reference)
```

### Solutions

**Option 1: Add termination condition**
```mangle
# Terminal positions are losing (no moves available)
losing(X) :- position(X), not has_move(X).
has_move(X) :- move(X, _).

# Winning = can move to losing position
winning(X) :- move(X, Y), losing(Y).

# Everything else is drawn/unknown - NOT derived
```

**Option 2: Use explicit turn/depth counter**
```mangle
# Stratified by depth
losing_at_depth(X, 0) :- terminal_loss(X).
winning_at_depth(X, D) :-
    move(X, Y),
    losing_at_depth(Y, D1),
    D = fn:plus(D1, 1).
```

---

## 6. Infinite Recursion in Fixpoint

### The Problem

Mangle computes ALL derivable facts. Unbounded generation = infinite loop.

### The Counter Fallacy

```mangle
# AI attempting to generate incrementing IDs
next_id(ID) :- current_id(Old), ID = fn:plus(Old, 1).
current_id(ID) :- next_id(ID).

# RESULT: Infinite loop computing 1, 2, 3, 4, ... forever
```

### Why This Happens

The AI assumes lazy evaluation (compute on demand). Mangle uses **eager, exhaustive** evaluation:
- Generate ALL facts that can be derived
- No concept of "stopping when you have enough"
- No built-in limit on recursion depth

### Safe Recursion Patterns

**Graph traversal (finite domain)**
```mangle
# SAFE - limited by finite edge relation
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
```

**Bounded depth**
```mangle
# SAFE - explicit depth limit
path(X, Y, 1) :- edge(X, Y).
path(X, Z, D) :-
    edge(X, Y),
    path(Y, Z, D1),
    D = fn:plus(D1, 1),
    D < 10.  # Hard limit
```

### Red Flags

- Rules that increment counters without bound
- Self-referential rules without decreasing measure
- Generated values not constrained by existing facts

---

## 7. Cartesian Product Explosions

### The Problem

Poor clause ordering can create massive intermediate results.

### Inefficient Pattern

```mangle
# 10K users × 10K users = 100M pairs checked
slow(X, Y) :-
    user(X),           # 10,000 users
    user(Y),           # 10,000 users
    friends(X, Y).     # 1,000 friendships
```

### Optimized Pattern

```mangle
# 1,000 friendships checked, then validated
fast(X, Y) :-
    friends(X, Y),     # 1,000 friendships
    user(X),           # Verify existence
    user(Y).           # Verify existence
```

### Optimization Rules

1. **Filter early**: Most selective predicates first
2. **Join small to large**: Start with smallest relation
3. **Bound variables immediately**: Constrain before expanding

```mangle
# Pattern: restrictive → expansive
efficient(X, Y, Z) :-
    rare_condition(X),     # 100 results
    common_join(X, Y),     # 10,000 per X
    very_common(Y, Z).     # 100,000 per Y
```

---

## 8. Structured Data Access Errors

### The Problem

AI often treats Mangle structs like JSON objects with direct field access.

### Wrong Patterns

```mangle
# WRONG - Python/JS style
bad(Name) :- record({/name: Name}).

# WRONG - SQL dot notation
bad(Name) :- record.name = Name.

# WRONG - Direct unification with partial struct
bad(Name) :- record(R), R.name = Name.
```

### Correct Pattern

```mangle
# CORRECT - Use :match_field
good(Name) :-
    record(R),
    :match_field(R, /name, Name).

# Multiple field extraction
person_info(ID, Name, Age) :-
    person_record(R),
    :match_field(R, /id, ID),
    :match_field(R, /name, Name),
    :match_field(R, /age, Age).
```

### Map Functions

| Function | Purpose |
|----------|---------|
| `:match_field(Struct, /key, Value)` | Extract field |
| `:match_entry(Map, /key, Value)` | Same as match_field |
| `fn:map_get(Map, /key, Default)` | Get with default |

---

## 9. Go Integration Anti-Patterns

### Problem 1: Ignoring Type System

```go
// WRONG - AI assumes string-based API
store.Add("parent", "alice", "bob")

// CORRECT - Must use proper types
f, _ := factstore.MakeFact("/parent", []engine.Value{
    engine.Atom("alice"),
    engine.Atom("bob"),
})
store.Add(f)
```

### Problem 2: Missing Parse Step

```go
// WRONG - AI assumes direct string execution
result := engine.Run("ancestor(X, Y) :- parent(X, Y).")

// CORRECT - Must parse first
program, err := parse.Parse("program", ruleString)
if err != nil {
    // Handle parse error
}
engine.EvalProgramNaive(program, store)
```

### Problem 3: External Predicate Binding Patterns

```go
// AI often ignores query binding patterns
func myPredicate(query engine.Query, cb func(engine.Fact)) error {
    // MUST check which args are bound vs free
    for i, arg := range query.Args {
        switch arg.Type() {
        case engine.TypeVariable:
            // This argument is FREE - enumerate all values
        case engine.TypeConstant:
            // This argument is BOUND - filter to this value
        }
    }
    return nil
}
```

---

## 10. Closed World Assumption Errors

### The Problem

AI models think in "open world" (unknown ≠ false). Mangle uses "closed world" (unknown = false).

### Wrong Pattern

```mangle
# AI trying to handle "unknown" status
handle(X, Status) :-
    item(X),
    Status = case
        when known_status(X, S) then S
        else /unknown.  # WRONG - no case/else in Mangle!
```

### Mangle Reality

```mangle
# Known items
known(X) :- known_status(X, _).

# Unknown = not in known_status (closed world)
unknown(X) :- item(X), not known(X).
```

### Key Insight

There is no NULL, no UNKNOWN, no UNDEFINED in Mangle. A fact either:
- **Exists** (true)
- **Does not exist** (false, by closed world assumption)

---

## Validation Checklist

Before running Mangle code, verify:

| Check | How to Verify |
|-------|---------------|
| All constants use `/atom` syntax | Grep for quoted strings that should be atoms |
| Aggregations use `\|> do fn:group_by` | Check every aggregation pipeline |
| Declarations use `Decl ... Type<>` | No `.decl` or other syntax |
| Negated variables are bound | Every `not pred(X)` has X bound earlier |
| No negation cycles | Trace dependency graph |
| Recursion terminates | Check for decreasing measure |
| Selective clauses first | Most restrictive predicates early |
| Struct access uses `:match_field` | No direct field access |

---

## Quick Reference: Correct vs Incorrect

| Pattern | WRONG | CORRECT |
|---------|-------|---------|
| Atom | `"active"` | `/active` |
| Aggregation | `sum(X)` | `\|> let S = fn:Sum(X)` |
| Grouping | `group by X` | `\|> do fn:group_by(X)` |
| Declaration | `.decl p(x:int)` | `Decl p(X.Type<int>).` |
| Negation | `not foo(X)` alone | `gen(X), not foo(X)` |
| Struct access | `R.field` | `:match_field(R, /field, V)` |
| Comments | `/* block */` | `# line` |
| Period | `parent(a, b)` | `parent(/a, /b).` |

---

**Next**: Apply these patterns with [200-SYNTAX_REFERENCE](200-SYNTAX_REFERENCE.md) and test against [scripts/validate_mangle.py](../scripts/validate_mangle.py).
