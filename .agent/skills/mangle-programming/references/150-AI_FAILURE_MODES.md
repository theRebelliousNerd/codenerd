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
| Aggregation | `sum(X)` | `\|> let S = fn:sum(X)` |
| Grouping | `group by X` | `\|> do fn:group_by(X)` |
| Declaration | `.decl p(x:int)` | `Decl p(X.Type<int>).` |
| Negation | `not foo(X)` alone | `gen(X), not foo(X)` |
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

```mangle
# THESE WILL ALL FAIL WITH PARSE ERRORS:
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

**Arithmetic**:
- `fn:plus(X, Y)` - Addition
- `fn:minus(X, Y)` - Subtraction
- `fn:mult(X, Y)` - Multiplication
- `fn:div(X, Y)` - Division
- `fn:mod(X, Y)` - Modulo

**Comparison** (use directly, not as functions):
- `X = Y`, `X != Y`, `X < Y`, `X > Y`, `X <= Y`, `X >= Y`

**Aggregation** (must use `|> do`/`let` pipeline):
- `fn:count()` - Count items
- `fn:sum(X)` - Sum values
- `fn:min(X)` - Minimum value
- `fn:max(X)` - Maximum value
- `fn:group_by(X)` - Group by value
- `fn:collect(X)` - Collect into list

**List/Data**:
- `fn:list(A, B, C)` - Create list
- `fn:len(L)` - List length
- `fn:concat(A, B)` - Concatenate
- `fn:append(L, X)` - Append to list
- `fn:pair(K, V)` - Create pair
- `fn:map_get(M, K, Default)` - Get from map with default

**Struct Access** (use colon prefix, not `fn:`):
- `:match_field(Struct, /key, Value)` - Extract field
- `:match_entry(Map, /key, Value)` - Same as match_field

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
is_subtype(X, Y) :- subclass_of(X, Y).
is_subtype(X, Z) :- subclass_of(X, Y), is_subtype(Y, Z).

# Now you can query: ?is_subtype(/sedan, /vehicle) → True
```

**Debug Your Taxonomy with Mangle:**

```mangle
# Find circular dependencies (A > B > A)
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
Decl vector_hit(AtomID.Type<n>, Score.Type<float>).
Decl current_phase(Phase.Type<n>).
Decl category(AtomID.Type<n>, Category.Type<n>).
Decl requires(AtomID.Type<n>, Dependency.Type<n>).
Decl conflicts(AtomID.Type<n>, OtherID.Type<n>).

Decl selected(AtomID.Type<n>).
Decl excluded(AtomID.Type<n>).
Decl suppressed(AtomID.Type<n>).
Decl final_atom(AtomID.Type<n>).
Decl ordered_result(AtomID.Type<n>, Rank.Type<int>).

# 1. SELECTION: High-confidence vector hits
selected(P) :- vector_hit(P, Score), Score > 0.85.

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
suppressed(P_Loser) :-
    selected(P_Winner), selected(P_Loser),
    conflicts(P_Winner, P_Loser),
    current_phase(/coding),
    P_Winner = /role_coder.

# 5. FINAL ASSEMBLY: Combine selected, minus excluded and suppressed
final_atom(P) :- selected(P), !excluded(P), !suppressed(P).

# 6. ORDERING: Safety first, then role, tool, format
# (Uses priority facts injected by Go loader)
Decl priority(Category.Type<n>, Rank.Type<int>).

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

**Next**: Apply these patterns with [200-SYNTAX_REFERENCE](200-SYNTAX_REFERENCE.md) and test against [scripts/validate_mangle.py](../scripts/validate_mangle.py).
