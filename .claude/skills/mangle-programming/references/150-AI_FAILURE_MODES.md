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
    let Total = fn:sum(Amount).
```

### Correct Syntax

```mangle
# CORRECT - Full pipeline syntax
Decl sales(Region, Amount).
region_sales(Region, Total) :-
    sales(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:sum(Amount).
```

### Required Keywords

| Keyword | Purpose | Example |
|---------|---------|---------|
| `\|>` | Pipeline operator | `source() \|> ...` |
| `do` | Apply transform function | `do fn:group_by(X)` |
| `let` | Bind aggregation result | `let N = fn:count()` |
| `fn:` | Function namespace | `fn:sum`, `fn:count`, `fn:group_by` |

### Function Casing

**CRITICAL**: All built-in functions use **lowercase** after the `fn:` prefix:
- `fn:count()` - lowercase c
- `fn:sum(X)` - lowercase s
- `fn:min(X)`, `fn:max(X)` - lowercase m
- `fn:group_by(X)` - lowercase
- `fn:collect(X)` - lowercase

**Common AI mistake**: Capitalizing function names (`fn:Sum`, `fn:Count`) will cause errors.

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

# WRONG - typed variables (0.4.0 wrote `App.Type<string>`; the pinned engine accepts neither)
Decl direct_dep(App.string, Lib.string).
```

### Correct Syntax

```mangle
# CORRECT
Decl direct_dep(App, Lib) bound [/string, /string].

# With name atoms
Decl status(Entity, State) bound [/name, /name].

# With list type (type functions are capitalised; one bound per argument)
Decl tags(ID, Tags) bound [/number, fn:List(/string)].

# With map type
Decl config(ID, Data) bound [/number, fn:Map(/string, /any)].
```

### Type Codes

| Type | Syntax | Examples |
|------|--------|----------|
| Integer | `/number` | `42`, `-17` |
| Float | `/float64` | `3.14`, `-2.5` |
| String | `/string` | `"hello"` |
| Name (atom) | `/name` | `/active`, `/user1` |
| List/Map/Any | (leave arg unbound) | `[1, 2, 3]`, `{/x: 10}` |

---

## 4. Safety Violations: Unbounded Variables

### The Problem

Every variable in a negated atom MUST be bound by a positive atom first. The AI often forgets the "generator" predicate.

### The AI Mental Model (Wrong)

```mangle
# Natural language: "Find users who are NOT admins"
# AI thinks: "Just negate admin"
Decl admin(User).
Decl non_admin(User).
non_admin(User) :- !admin(User).  # UNSAFE: analysis rejects it ("variable User is not bound")
```

### Why This Fails

The Mangle engine asks: "What values should I test for `User`?" With bottom-up evaluation, variables represent potentially infinite domains. The engine cannot iterate over "all possible users in the universe."

### The Expert Fix

```mangle
# CORRECT - User is bound by user() first
Decl user(User).
Decl admin(User).
Decl non_admin(User).
non_admin(User) :- user(User), !admin(User).
#                  ^^^^^^^^^^ Generator predicate
```

### Safety Checklist

Before any negation `!pred(X, Y, ...)`:
1. Is every variable bound by a positive atom in the same body?
2. Order does not matter for negation (the engine moves a negated atom after the atoms that bind
   it); it does matter for comparisons: `X > 0` before `X` is bound fails analysis.

```mangle
Decl candidate(X).
Decl excluded(X).
Decl source(X, Y).
Decl blocked(X).
Decl foo(X).
Decl safe(X).
Decl safe_pair(X, Y).
Decl also_safe(X).

# SAFE patterns
safe(X) :- candidate(X), !excluded(X).
safe_pair(X, Y) :- source(X, Y), !blocked(X).
also_safe(X) :- !foo(X), candidate(X).   # accepted: the engine moves the negation after candidate(X)
```

```mangle
# UNSAFE patterns - WRONG: analysis rejects both ("variable X is not bound")
bad(X) :- !foo(X).              # X unbound
bad2(X, Y) :- foo(X), !bar(Y).  # Y unbound
```

---

## 5. Stratification Violations: Negative Cycles

### The Problem

Mangle rejects programs where negation creates circular dependencies.

### Classic Game Theory Failure

```mangle
# WRONG: AI generating game logic (minimax-style). The engine reports
# "stratification failed: program cannot be stratified".
Decl move(From, To).
Decl winning(X).
Decl losing(X).
winning(X) :- move(X, Y), losing(Y).
losing(X) :- move(X, _), !winning(X).  # negation through the recursion
```

### Why This Fails

The dependency graph:
1. `winning` depends on `losing`
2. `losing` depends on `!winning`
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
Decl position(X).
Decl move(From, To).
Decl has_move(X).
Decl losing(X).
Decl winning(X).
losing(X) :- position(X), !has_move(X).
has_move(X) :- move(X, _).

# Winning = can move to losing position
winning(X) :- move(X, Y), losing(Y).

# Everything else is drawn/unknown - NOT derived
```

**Option 2: Use explicit turn/depth counter**
```mangle
# Stratified by depth
Decl terminal_loss(X).
Decl move(X, Y).
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
Decl edge(X, Y).
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
```

**Bounded depth**
```mangle
# SAFE - explicit depth limit
Decl edge(X, Y).
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
Decl user(X).
Decl friends(X, Y).
slow(X, Y) :-
    user(X),           # 10,000 users
    user(Y),           # 10,000 users
    friends(X, Y).     # 1,000 friendships
```

### Optimized Pattern

```mangle
# 1,000 friendships checked, then validated
Decl friends(X, Y).
Decl user(X).
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
Decl rare_condition(X).
Decl common_join(X, Y).
Decl very_common(Y, Z).
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
Decl record(R).
Decl person_record(R).
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
| `:match_field(Struct, /key, Value)` | Read a struct field (`{/key: v}`); Struct bound, Value free |
| `:match_entry(Map, Key, Value)` | Read a map entry (`["key": v]`) with the key as written |
| `fn:struct:get(Struct, /key)` | A struct field as a function |

There is no get-with-default: `fn:map:get` is declared and not wired. Derive the default with a
second rule and a projection (`has_key(M) :- ...`, then `!has_key(M)`).

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
Decl item(X).
Decl known_status(X, Status).
Decl known(X).
Decl unknown(X).

# Known items
known(X) :- known_status(X, _).

# Unknown = not in known_status (closed world)
unknown(X) :- item(X), !known(X).
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
| Declarations use `Decl ... bound [...]` | No `.decl` or other syntax |
| Negated variables are bound | Every `!pred(X)` has X bound by a positive atom in the same body |
| No negation cycles | Trace dependency graph |
| Recursion terminates | Check for decreasing measure |
| Selective clauses first | Most restrictive predicates early |
| Struct access uses `:match_field` | No direct field access |

---

## Quick Reference: Correct vs Incorrect

| Pattern | WRONG | CORRECT |
|---------|-------|---------|
| Atom | `"active"` | `/active` |
| Aggregation | `sum(X)` | `\|> let S = fn:sum(X)` |
| Grouping | `group by X` | `\|> do fn:group_by(X)` |
| Declaration | `.decl p(x:int)` | `Decl p(X) bound [/number].` |
| Negation | `!foo(X)` alone | `gen(X), !foo(X)` |
| Struct access | `R.field` | `:match_field(R, /field, V)` |
| Comments | `/* block */` | `# line` |
| Period | `parent(a, b)` | `parent(/a, /b).` |

---

## 11. Architectural Anti-Pattern: "Mangle as HashMap"

### The Problem

AI agents often misuse Mangle as a pattern-matching database when Mangle can only do **exact unification**. This manifests as storing hundreds of "canonical sentences" as facts, expecting fuzzy matching.

### Root Cause: The "DSL Trap"

This is a classic case of the **"DSL (Domain Specific Language) Trap."** Developers treat `.mg` files as general design documents—mixing **Taxonomy** (Data), **Intents** (Configuration), and **Rules** (Logic).

**Mangle is a strict compiler, not a notebook.** It will panic if it encounters lines like `Taxonomy: Vehicle > Car` or `Intent: "refund"`.

```mangle
# THE DSL TRAP IN ACTION - mixing data and logic in one file:

# This is DATA (taxonomies, hierarchies) - NOT valid Mangle:
# TAXONOMY: /vehicle > /car > /sedan  ← PARSE ERROR!
# INTENT: "my car won't start" -> /breakdown_support  ← PARSE ERROR!

# This is LOGIC (real Mangle rules) - valid:
Decl user_intent(X, Y).
Decl has_active_warranty(User).
eligible_for_support(User) :-
    user_intent(User, /breakdown_support),
    has_active_warranty(User).
```

**The Three Categories Being Confused:**

| Category | Example | Correct Home |
|----------|---------|--------------|
| **Taxonomy** | `/vehicle > /car > /sedan` | Mangle facts via Go pre-processor |
| **Intents** | `"I need help" -> /support` | Vector DB (fuzzy matching) |
| **Rules** | `permitted(X) :- safe(X).` | Mangle engine (real logic) |

### The Anti-Pattern in Practice

```mangle
# WRONG ARCHITECTURE: Storing 400+ patterns expecting fuzzy matching
Decl user_input(Input).
intent_definition("review my code", /review, "file_types").
intent_definition("check for bugs", /debug, "codebase").
intent_definition("look at my code", /review, "file_types").
intent_definition("Review my code", /review, "file_types").  # Case variation!
# ... 400 more variations

# WRONG: Trying to match user input against these facts
matched_intent(Verb) :-
    user_input(Input),
    intent_definition(Input, Verb, _).  # Only works for EXACT matches!
```

### Why This Fails

When a user says "examine my code", there's no `intent_definition` with that exact string. Mangle cannot:
- Match "examine" to "review" (semantic similarity)
- Ignore case differences
- Handle typos or synonyms
- Do substring matching

### Invalid String Functions (DO NOT EXIST)

AI agents frequently hallucinate these functions that **do not exist in Mangle**:

```text
# NONE OF THESE EXIST (analysis: "unknown function"):
fn:string_contains(Input, Keyword)   # DOES NOT EXIST
fn:contains(Input, Keyword)          # DOES NOT EXIST
fn:substring(S, Start, End)          # DOES NOT EXIST
fn:match(S, Pattern)                 # DOES NOT EXIST
fn:regex(S, Pattern)                 # DOES NOT EXIST
fn:lower(S)                          # DOES NOT EXIST
fn:upper(S)                          # DOES NOT EXIST
fn:trim(S)                           # DOES NOT EXIST
fn:split(S, Delim)                   # DOES NOT EXIST
fn:startswith(S, Prefix)             # DOES NOT EXIST
fn:endswith(S, Suffix)               # DOES NOT EXIST
```

### Valid Built-in Functions (Complete List)

Generated from the pinned engine's `symbols/symbols.go` (arity after the slash; `n` is variadic).
Anything not here does not exist: there is no modulo, and `fn:len`, `fn:concat`, `fn:append` and
`fn:map_get` are `fn:list:len`, `fn:string:concat`, `fn:list:append` and nothing.

**Arithmetic**: `fn:plus/n`, `fn:minus/n`, `fn:mult/n`, `fn:div/n` (integer), `fn:sqrt/1`
(float), `fn:float:plus/n`, `fn:float:mult/n`, `fn:float:div/n`.

**Comparison** (in the body, integers only): `X = Y`, `X != Y`, `X < Y`, `X <= Y`, `X > Y`,
`X >= Y`, or `:lt/2`, `:le/2`, `:gt/2`, `:ge/2`.

**Reducers** (after `|> do fn:group_by(...)`): `fn:count/0`, `fn:sum/1`, `fn:min/1`, `fn:max/1`,
`fn:avg/1` (float), `fn:float:sum/1`, `fn:float:min/1`, `fn:float:max/1`, `fn:collect/n`,
`fn:collect_distinct/n`. Declared but not usable: `fn:count_distinct`, `fn:pick_any`,
`fn:map:get`.

**Lists**: `fn:list/n`, `fn:list:cons/2`, `fn:list:append/2` (one element), `fn:list:get/2`,
`fn:list:len/1`, `fn:list:contains/2` (`/true` or `/false`), `:list:member/2`, `:match_cons/3`,
`:match_nil/1`.

**Structs, maps, pairs, tuples**: `fn:struct/n`, `fn:struct:get/2`, `:match_field/3`,
`fn:map/n`, `:match_entry/3`, `fn:pair/2`, `:match_pair/3`, `fn:tuple/n`.

**Strings and names**: `fn:string:concat/n`, `fn:string:replace/4`, `:string:contains/2`,
`:string:starts_with/2`, `:string:ends_with/2`, `fn:number:to_string/1`,
`fn:float64:to_string/1`, `fn:name:to_string/1`, `fn:name:root/1`, `fn:name:tip/1`,
`fn:name:list/1`, `:match_prefix/2`.

**Time and durations**: `fn:time:now/0`, `fn:time:add/2`, `fn:time:sub/2`, `fn:time:trunc/2`,
`fn:time:format/2`, `fn:time:parse_rfc3339/1`, `fn:time:year/1` ... `fn:time:second/1`,
`:time:lt/2` (and `le`, `gt`, `ge`), `fn:duration:parse/1`, `fn:duration:add/2`,
`:duration:lt/2` (and `le`, `gt`, `ge`), plus the interval forms, which need a temporal store
codeNERD does not have.

### The Correct Architecture: Neuro-Symbolic

When you need to match natural language input to structured actions:

```text
User Input: "check my code for security issues"
                    |
    EMBEDDING LAYER (Go/Python + Vector DB)
    - Encode user input as embedding
    - Semantic search against pre-embedded patterns
    - Returns: [(canonical_text, verb, similarity_score), ...]
                    |
    ASSERT AS MANGLE FACTS
    - semantic_match("check my...", "review code", /review, 85).
    - semantic_match("check my...", "security scan", /security, 72).
                    |
    MANGLE KERNEL (Deductive Reasoning)
    - Apply scoring rules
    - Handle verb composition
    - Enforce safety constraints
    - Derive final action
```

### Mangle's Role in Neuro-Symbolic Architecture

**Use Mangle for** (what it excels at):
```mangle
# Deductive scoring based on semantic_match facts
Decl semantic_match(A1, A2, A3, A4).
Decl verb_composition(A1, A2, A3, A4).
Decl blocked_action(Action).
Decl requires_approval(Action).
selected_verb(Verb) :-
    semantic_match(_, _, Verb, Score),
    Score >= 85.

# Verb composition (multi-step intents)
compound_action(V1, V2) :-
    semantic_match(_, _, V1, S1),
    semantic_match(_, _, V2, S2),
    S1 >= 70, S2 >= 70,
    verb_composition(V1, V2, _, _).

# Safety constraints
permitted(Action) :-
    selected_verb(Action),
    !blocked_action(Action),
    !requires_approval(Action).
```

**Do NOT use Mangle for**:
- Storing 400+ exact-match patterns
- String/substring matching
- Fuzzy text similarity
- Natural language understanding

### Summary: Data vs Rules

| Type | Belongs In | Example |
|------|------------|---------|
| **Exact-match patterns** | Vector DB | `intent_definition("review code", /review)` |
| **Fuzzy matches** | Embedding search | "check my code" -> similarity to "review code" |
| **Composition rules** | Mangle | `compound_action(V1, V2) :- ...` |
| **Safety constraints** | Mangle | `permitted(A) :- ..., !blocked(A).` |
| **Transitive relations** | Mangle | `reachable(X, Z) :- reachable(X, Y), edge(Y, Z).` |

### Red Flags: Signs of Mangle Misuse

1. **Hundreds of ground facts with string literals** - Data masquerading as logic
2. **Duplicate facts with case/punctuation variations** - Compensating for lack of fuzzy matching
3. **Predicates named `*_definition` or `*_pattern`** - Likely storing lookup data
4. **Rules with `fn:string_*` or `fn:contains`** - Will fail with parse errors
5. **Using Mangle to match user input directly** - Should use embeddings first

### Salvage Strategy: The "Split-Brain" Loader

If you have existing files mixing data and logic, **don't throw them away**. Use a Go pre-processor that routes content to the correct system.

**The Concept:**
- **Pseudo-Code (Taxonomy/Intents):** Parse in Go, inject as **Facts** or **Embeddings**
- **Real Code (Rules):** Pass to Mangle Engine

**Example Hybrid File (`policy.mg`):**

```text
# --- DATA SECTION (Go pre-processor intercepts) ---
TAXONOMY: /vehicle > /car > /sedan
TAXONOMY: /vehicle > /truck
INTENT: "my car won't start" -> /breakdown_support

# --- LOGIC SECTION (Real Mangle) ---
eligible_for_support(User) :-
    user_intent(User, /breakdown_support),
    has_active_warranty(User).
```

**Go Pre-Processor Pattern:**

```go
func LoadHybridFile(path string, vectorDB VectorStore, store factstore.FactStore) (string, error) {
    file, _ := os.Open(path)
    scanner := bufio.NewScanner(file)
    var mangleCode strings.Builder

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())

        // 1. Route INTENTs to Vector DB (Fuzzy Matching)
        if strings.HasPrefix(line, "INTENT:") {
            phrase, intentAtom := parseIntentLine(line)
            vectorDB.Add(phrase, intentAtom)
            continue // Don't send to Mangle Parser!
        }

        // 2. Route TAXONOMY to Mangle Store (Graph Structure)
        if strings.HasPrefix(line, "TAXONOMY:") {
            child, parent := parseTaxonomyLine(line)
            // Inject fact: subclass_of(/child, /parent) directly
            atom := ast.NewAtom("subclass_of", ast.Name(child), ast.Name(parent))
            store.Add(atom)
            continue
        }

        // 3. Keep Real Logic for the Compiler
        mangleCode.WriteString(line + "\n")
    }

    return mangleCode.String(), nil
}
```

**Taxonomy as Mangle Logic (after injection):**

```mangle
# Facts injected by Go loader:
# subclass_of(/car, /vehicle).
# subclass_of(/sedan, /car).

# Transitive closure rule (add to .mg file):
Decl subclass_of(X, Y).
is_subtype(X, Y) :- subclass_of(X, Y).
is_subtype(X, Z) :- subclass_of(X, Y), is_subtype(Y, Z).

# Now you can query: ?is_subtype(/sedan, /vehicle) → True
```

**Debug Your Taxonomy with Mangle:**

```mangle
# Find circular dependencies (A > B > A)
Decl is_subtype(X, Y).
Decl subclass_of(X, Y).
taxonomy_error(A, "Cycle Detected") :- is_subtype(A, A).

# Find orphan nodes (no path to root)
taxonomy_error(A, "Orphan Node") :-
    subclass_of(A, _),
    !is_subtype(A, /root).
```

### Migration Checklist

1. **Don't Manual Rewrite** - Use Go loader for existing data
2. **Strict Separator** - Use prefixes (`TAXONOMY:`, `INTENT:`) for routing
3. **Sanitize Atoms** - Ensure terms are valid (`/sedan`, not `Sedan`)
4. **One Source of Truth** - Keep taxonomy and logic in same file for readability

---

## 12. Application: JIT Prompt Compiler

The neuro-symbolic pattern from Section 11 extends to **dynamic prompt engineering**. Instead of monolithic 20,000-character prompts, decompose into atomic units and let Mangle act as the "linker".

### The Compilation Pipeline

```text
Task Context
    ↓
VECTOR DB (Search Engine) → Find relevant atomic prompts
    ↓
MANGLE KERNEL (Linker) → Resolve dependencies, conflicts, phase gating
    ↓
GO RUNTIME (Assembler) → Concatenate and output final string
```

### Hybrid Prompt File Format

Use `PROMPT:` prefix for atomic prompts (routed to Vector DB by Go loader):

```text
# --- DATA SECTION (Parsed by Go Loader) ---
PROMPT: /role_coder [role] -> "You are a Senior Go Engineer..."
PROMPT: /cap_sql [tool] -> "You can access a PostgreSQL database..."
PROMPT: /safe_no_delete [safety] -> "CRITICAL: Do NOT generate DROP..."
PROMPT: /phase_coding [phase] -> "During CODING: Write clean code..."

# --- LOGIC SECTION (Mangle Rules) ---
# Dependency: SQL tool requires safety constraint
requires(/cap_sql, /safe_no_delete).

# Conflict: verbose and concise are mutually exclusive
conflicts(/fmt_verbose, /fmt_concise).
```

### Prompt Compiler Rules (Mangle as Linker)

```mangle
# =============================================================================
# PROMPT COMPILER LOGIC
# =============================================================================

# Schema declarations
Decl vector_hit(AtomID, Score) bound [/name, /number].   # integer 0..100: comparisons are integer-only
Decl current_phase(Phase) bound [/name].
Decl category(AtomID, Category) bound [/name, /name].
Decl requires(AtomID, Dependency) bound [/name, /name].
Decl conflicts(AtomID, OtherID) bound [/name, /name].

Decl selected(AtomID) bound [/name].
Decl excluded(AtomID) bound [/name].
Decl suppressed(AtomID) bound [/name].
Decl final_atom(AtomID) bound [/name].
Decl ordered_result(AtomID, Rank) bound [/name, /number].

# 1. SELECTION: High-confidence vector hits
selected(P) :- vector_hit(P, Score), Score > 85.

# 2. DEPENDENCY RESOLUTION: Auto-include required atoms
selected(Dep) :- selected(P), requires(P, Dep).

# 3. PHASE GATING: Force appropriate role for current phase
selected(/role_coder) :- current_phase(/coding).
selected(/role_architect) :- current_phase(/planning).
selected(/role_tester) :- current_phase(/testing).
selected(/role_reviewer) :- current_phase(/review).

# Exclude formatting atoms during planning
excluded(P) :- current_phase(/planning), category(P, /fmt).

# 4. CONFLICT RESOLUTION: If conflicting atoms selected, suppress loser
# During coding phase, coder role wins over architect
# (Variables are letters and digits only: P_Loser does not lex.)
suppressed(Loser) :-
    current_phase(/coding),
    selected(/role_coder), selected(Loser),
    conflicts(/role_coder, Loser).

# 5. FINAL ASSEMBLY: Combine selected, minus excluded and suppressed
final_atom(P) :- selected(P), !excluded(P), !suppressed(P).

# 6. ORDERING: Safety first, then role, tool, format
# (Uses priority facts injected by Go loader)
Decl priority(Category, Rank) bound [/name, /number].

# Base priorities
priority(/safety, 1).
priority(/role, 2).
priority(/tool, 3).
priority(/phase, 4).
priority(/fmt, 5).

# Derive ordered results
ordered_result(P, Rank) :-
    final_atom(P),
    category(P, Cat),
    priority(Cat, Rank).
```

### Why Mangle is Perfect Here

| Feature | Benefit |
|---------|---------|
| **Dependency Resolution** | Recursive `requires/2` auto-includes safety constraints |
| **Conflict Detection** | `conflicts/2` prevents contradictory instructions |
| **Phase Gating** | Rules change behavior based on `current_phase/1` |
| **Priority Ordering** | Aggregation derives assembly order |
| **Negation** | `!excluded(P)` cleanly filters unwanted atoms |

### What NOT to Do

```mangle
# WRONG - Storing prompt text in Mangle (belongs in Go map/Vector DB)
prompt_text(/role_coder, "You are a Senior Go Engineer...").  # NO!

# WRONG - String matching for relevance (use Vector DB)
relevant(P) :- prompt_text(P, Text), fn:contains(Text, UserQuery).  # NO!

# CORRECT - Mangle holds metadata, Vector DB holds text
# category(/role_coder, /role).
# requires(/cap_sql, /safe_no_delete).
```

### Benefits Over Static Prompts

| Aspect | Static Prompt | JIT Compiled |
|--------|---------------|--------------|
| **Maintenance** | Edit 20K char file | Edit single atom |
| **Safety** | Manual inclusion | Auto-injected with tools |
| **Context-Awareness** | One-size-fits-all | Phase-gated behavior |
| **Token Efficiency** | Full prompt always | Only relevant atoms |
| **Conflict Prevention** | Hope you noticed | Mangle enforces |

For complete implementation, see [prompt-architect skill](../../prompt-architect/SKILL.md) and [cli-engine-integration/references/prompt-management.md](../../cli-engine-integration/references/prompt-management.md).

---

**Next**: Apply these patterns with [200-SYNTAX_REFERENCE](200-SYNTAX_REFERENCE.md) and run them on the engine: `nerd check-mangle --standalone --eval <predicate> file.mg`.
