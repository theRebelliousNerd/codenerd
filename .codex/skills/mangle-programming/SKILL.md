---
name: mangle-programming
description: Master Google's Mangle declarative programming language for deductive database programming, constraint-like reasoning, and software analysis. This skill should be used when the user asks to write, debug, or review Mangle programs, schemas, predicates, policies, rules, queries, aggregations, or safety and stratification issues. Includes comprehensive AI failure mode prevention (atom/string confusion, aggregation syntax, safety violations, stratification errors).
license: Apache-2.0
version: 0.6.0
mangle_version: 0.4.0 (November 1, 2024)
last_updated: 2025-12-08
---

# Mangle Programming: The Complete Reference

**Google Mangle** is a declarative deductive database language extending Datalog with practical features for modern software analysis, security evaluation, and multi-source data integration. This skill provides comprehensive coverage from first principles to production deployment.

## Quick Start: Your First Program (5 Minutes)

```mangle
# Facts: What we know
parent(/oedipus, /antigone).
parent(/oedipus, /ismene).

# Rules: What we derive (Head :- Body means "Head is true if Body is true")
sibling(X, Y) :- parent(P, X), parent(P, Y), X != Y.

# Query: What we want to know
?sibling(/antigone, X)
# Result: X = /ismene
```

**Essential syntax**:
- `/name` - Named constants (atoms)
- `UPPERCASE` - Variables
- `:-` - Rule implication ("if")
- `,` - Conjunction (AND)
- `.` - Statement terminator (REQUIRED)

## CRITICAL: Before Writing Mangle Code

**Read [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md) first.** Mangle has unique semantics that conflict with AI training on Python/SQL/Prolog.

### The "DSL Trap"

**Mangle is a strict compiler, not a notebook.** Don't treat `.mg` files as general design documents mixing:

- **Taxonomy** (`/vehicle > /car`) → Route to Mangle facts via Go pre-processor
- **Intents** (`"I need help" -> /support`) → Route to Vector DB
- **Rules** (`permitted(X) :- safe(X).`) → Real Mangle code

If you have files mixing these, use a "Split-Brain Loader" (see Section 11 in 150-AI_FAILURE_MODES).

### Common Silent Failures

| Pattern | WRONG | CORRECT |
|---------|-------|---------|
| Constants | `"active"` | `/active` |
| Aggregation | `sum(X)` | `\|> let S = fn:sum(X)` |
| Grouping | `GROUP BY X` | `\|> do fn:group_by(X)` |
| Declaration | `.decl p(x:int)` | `Decl p(X.Type<int>).` |
| Negation | `not foo(X)` alone | `gen(X), not foo(X)` |
| Struct access | `R.field` | `:match_field(R, /field, V)` |

## When to Use Mangle

Use this skill when you need to write, debug, or review Mangle schemas, predicates, policies, rules, queries, aggregations, or resolve safety and stratification issues.

✅ **Perfect for**:
- Dependency analysis (transitive closures)
- Vulnerability detection (CVE propagation)
- Graph reachability and path finding
- Multi-source data integration
- Recursive relationship reasoning
- Security policy compliance
- Knowledge graph queries

❌ **Not suitable for** (See Section 11 in [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md)):
- **Fuzzy/semantic matching** → Use vector embeddings (neuro-symbolic architecture)
- **String manipulation** → No `fn:string_contains`, `fn:substring`, `fn:regex` exist!
- **HashMap-style exact lookup** → Mangle is for deduction, not key-value storage
- Optimization problems → Use MiniZinc, OR-Tools
- Numerical constraints → Use Z3, SMT solvers
- Distributed/parallel execution → Single-machine only
- Real-time streaming → Batch processing model

## The Reference Library

This skill uses **progressive disclosure**: Start with quick references below, then explore numbered references for depth.

### Navigation System

**000-Series: Orientation**
- [000-ORIENTATION](references/000-ORIENTATION.md) - How to navigate this library
  - Navigation patterns for different learning paths
  - Skill architecture and reference organization
  - From beginner to expert roadmaps

**100-Series: Fundamentals**
- [100-FUNDAMENTALS](references/100-FUNDAMENTALS.md) - Theory, concepts, and mental models
  - Logic programming foundations
  - Deductive database principles
  - Evaluation models (bottom-up, semi-naive)
  - When Mangle vs Prolog vs SQL vs Datalog

- [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md) - **CRITICAL: Read before writing Mangle**
  - Atom vs String confusion (silent failures)
  - Aggregation syntax (|> do fn:group_by pattern)
  - Declaration syntax (Decl vs .decl)
  - Safety violations (unbounded negation)
  - Stratification errors (negative cycles)
  - Infinite recursion pitfalls
  - Go integration anti-patterns
  - **Section 11: "Mangle as HashMap" anti-pattern** - Using Mangle for fuzzy matching when vectors are needed

**200-Series: Language Reference**
- [200-SYNTAX_REFERENCE](references/200-SYNTAX_REFERENCE.md) - Complete language specification
  - Every data type, operator, built-in function
  - Grammar, lexical rules, declarations
  - Safety constraints and stratification
  - REPL commands and file format

**300-Series: Pattern Catalog**
- [300-PATTERN_LIBRARY](references/300-PATTERN_LIBRARY.md) - Every common pattern
  - Selection, projection, join, union (SQL equivalents)
  - Recursive patterns (transitive closure, ancestors, paths)
  - Negation patterns (set difference, universal quantification)
  - Aggregation patterns (count, sum, avg, conditional)
  - Domain-specific (access control, temporal, provenance)

**400-Series: Recursion Mastery**
- [400-RECURSION_MASTERY](references/400-RECURSION_MASTERY.md) - Deep dive on recursive techniques
  - Linear vs non-linear recursion
  - Path construction and tracking
  - Cycle detection and prevention
  - Distance/cost accumulation
  - Mutual recursion patterns
  - Termination analysis

- [450-PROMPT_ATOM_PREDICATES](references/450-PROMPT_ATOM_PREDICATES.md) - JIT Prompt Compiler predicates
  - Section 45: Prompt atom selection and compilation
  - Multi-dimensional context matching (spreading activation)
  - Dependency resolution and conflict handling
  - Category ordering and budget allocation
  - Validation and learning signals
  - Integration with codeNERD architecture

**500-Series: Aggregation & Transforms**
- [500-AGGREGATION_TRANSFORMS](references/500-AGGREGATION_TRANSFORMS.md) - Complete aggregation guide
  - Transform pipeline architecture
  - Multi-stage aggregation
  - Conditional aggregation
  - Nested aggregation patterns
  - Window functions (simulated)
  - Complex analytics

**600-Series: Type System**
- [600-TYPE_SYSTEM](references/600-TYPE_SYSTEM.md) - Types, lattices, and gradual typing
  - Type declarations and inference
  - Structured types (maps, structs, lists)
  - Union types and generics
  - Lattice operations (experimental)
  - Type safety and runtime checks

**700-Series: Optimization**
- [700-OPTIMIZATION](references/700-OPTIMIZATION.md) - Performance engineering
  - Rule ordering strategies
  - Selectivity analysis
  - Semi-naive evaluation internals
  - Memory management
  - Profiling and benchmarking
  - Scaling limits

**800-Series: Theory**
- [800-THEORY](references/800-THEORY.md) - Mathematical foundations
  - First-order logic foundations
  - Fixed-point semantics
  - Stratification theory
  - Complexity analysis
  - Comparison with Datalog variants
  - Academic papers and research

**900-Series: Ecosystem**
- [900-ECOSYSTEM](references/900-ECOSYSTEM.md) - Tools, libraries, and integrations
  - Go integration patterns
  - Production deployment architectures
  - Testing and debugging
  - Custom fact stores
  - gRPC service integration
  - Monitoring and observability

### Go API Reference

- [GO_API_REFERENCE](references/GO_API_REFERENCE.md) - Complete Go package documentation
  - AST types (Constant, Atom, Variable, Clause)
  - Parse, Analysis, and Engine packages
  - FactStore implementations
  - json2struct and proto2struct converters
  - Mangle-service gRPC demo
  - Common integration patterns

### Up-to-Date API Reference

- [context7-mangle](references/context7-mangle.md) - Context7 comprehensive patterns (Mangle v0.4.0)
  - Installation (Go and Rust implementations)
  - Interpreter usage and REPL commands
  - Complete data types and structured data
  - Rule grammar and declarations
  - API integration (REST, gRPC)
  - Real-world examples (volunteer DB, security analysis)

### Legacy References (Being Migrated)

These will be consolidated into the numbered system:

- [SYNTAX.md](references/SYNTAX.md) - Basic syntax (-> 200-SYNTAX_REFERENCE)
- [EXAMPLES.md](references/EXAMPLES.md) - Working examples (-> 300-PATTERN_LIBRARY)
- [ADVANCED_PATTERNS.md](references/ADVANCED_PATTERNS.md) - Advanced patterns (-> 400, 500, 700)
- [PRODUCTION.md](references/PRODUCTION.md) - Deployment guide (-> 900-ECOSYSTEM)

## Essential Quick Reference

### Data Types

```mangle
/name               # Named constant (atom)
42, -17, 3.14       # Numbers
"text"              # Strings
[1, 2, 3]           # Lists
{/k: v}             # Structs
[/a: /foo, /b: /bar]  # Maps
fn:pair("a", "b")   # Pairs
fn:tuple(1, 2, 3)   # Tuples
```

### Operators

```mangle
:-                  # Rule implication (if)
<-                  # Alternative implication syntax
,                   # Conjunction (AND)
!                   # Negation (requires stratification)
=, !=, <, >, <=, >= # Comparisons
|>                  # Transform pipeline
```

### Built-in Functions

```mangle
fn:count()                        # Count elements
fn:sum(V), fn:max(V), fn:min(V)   # Aggregations
fn:group_by(V1, V2, ...)          # Group by
fn:plus(A, B), fn:minus(A, B)     # Arithmetic
fn:list:get(List, Index)          # List access (0-based)
:match_cons(List, Head, Tail)     # List destructure
:match_field(Struct, /field, Val) # Struct field access
:list:member(Elem, List)          # List membership
```

### Core Patterns

#### Transitive Closure

```mangle
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
```

#### Negation (Set Difference)

```mangle
safe(X) :- candidate(X), !excluded(X).
```

#### Aggregation

```mangle
count_by_category(Cat, N) :-
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:count().
```

#### Structured Data Query

```mangle
volunteer(Id) :-
  volunteer_record(R), :match_field(R, /id, Id).

volunteer_name(Id, Name) :-
  volunteer_record(R), :match_field(R, /id, Id), :match_field(R, /name, Name).
```

## Learning Paths

### Path 1: Beginner (0-2 hours)

1. Read SKILL.md (this file) - Quick Start and Critical section
2. Read [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md) - **Avoid common mistakes**
3. Read [000-ORIENTATION](references/000-ORIENTATION.md) - Understand the library
4. Read [100-FUNDAMENTALS](references/100-FUNDAMENTALS.md) - Sections 1-3 only
5. Try examples from [300-PATTERN_LIBRARY](references/300-PATTERN_LIBRARY.md) - Basic patterns
6. Install and run: `GOBIN=~/bin go install github.com/google/mangle/interpreter/mg@latest`

### Path 2: Intermediate (2-8 hours)

1. Complete beginner path
2. Read [200-SYNTAX_REFERENCE](references/200-SYNTAX_REFERENCE.md) - Full syntax
3. Read [context7-mangle](references/context7-mangle.md) - Up-to-date patterns
4. Study [400-RECURSION_MASTERY](references/400-RECURSION_MASTERY.md)
5. Study [500-AGGREGATION_TRANSFORMS](references/500-AGGREGATION_TRANSFORMS.md)
6. Build: Vulnerability scanner or dependency analyzer

### Path 3: Advanced (8-20 hours)

1. Complete intermediate path
2. Read [600-TYPE_SYSTEM](references/600-TYPE_SYSTEM.md)
3. Read [700-OPTIMIZATION](references/700-OPTIMIZATION.md)
4. Read [800-THEORY](references/800-THEORY.md)
5. Read [900-ECOSYSTEM](references/900-ECOSYSTEM.md)
6. Build: Production-grade analysis service

### Path 4: Expert (20+ hours)

1. Complete advanced path
2. Deep dive all 800-THEORY mathematical foundations
3. Contribute to <https://github.com/google/mangle>
4. Build custom fact stores and integrations
5. Optimize large-scale (millions of facts) programs

## Common Pitfalls (Avoid These)

**IMPORTANT**: These are the most frequent AI coding failures. See [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md) for comprehensive coverage.

### Pitfall 1: Atom vs String Confusion (CRITICAL - Silent Failure)

```mangle
# WRONG - "active" is a string, not an atom
status(User, "active").
active_users(U) :- status(U, "active").  # Matches, but wrong type

# CORRECT - /active is an atom (interned constant)
status(User, /active).
active_users(U) :- status(U, /active).
```

**Rule**: Use `/atom` for identifiers, enums, statuses. Use `"string"` only for human-readable text.

### Pitfall 2: Forgetting Periods

```mangle
# WRONG - Missing statement terminator
parent(/a, /b)

# CORRECT
parent(/a, /b).
```

### Pitfall 3: Wrong Aggregation Syntax

```mangle
# WRONG - SQL-style implicit grouping
region_sales(Region, Total) :-
    sales(Region, Amount),
    Total = sum(Amount).

# WRONG - Missing `do` keyword
region_sales(Region, Total) :-
    sales(Region, Amount) |>
    fn:group_by(Region),
    let Total = fn:sum(Amount).

# CORRECT - Full pipeline syntax
region_sales(Region, Total) :-
    sales(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:sum(Amount).
```

### Pitfall 4: Wrong Declaration Syntax

```mangle
# WRONG - Soufflé syntax
.decl dependency(app: string, lib: string)

# CORRECT - Mangle syntax
Decl dependency(App.Type<string>, Lib.Type<string>).
```

### Pitfall 5: Unbound Variables in Negation

```mangle
# WRONG - X not bound first (WILL CRASH)
bad(X) :- not foo(X).

# CORRECT - X bound by candidate first
good(X) :- candidate(X), not foo(X).
```

### Pitfall 6: Stratification Violations

```mangle
# WRONG - Negative cycle (winning -> losing -> not winning)
winning(X) :- move(X, Y), losing(Y).
losing(X) :- not winning(X).

# CORRECT - Break the cycle with base case
losing(X) :- position(X), not has_move(X).
has_move(X) :- move(X, _).
winning(X) :- move(X, Y), losing(Y).
```

### Pitfall 7: Infinite Recursion

```mangle
# WRONG - Unbounded counter generation (infinite loop)
next_id(ID) :- current_id(Old), ID = fn:plus(Old, 1).
current_id(ID) :- next_id(ID).

# CORRECT - Recursion bounded by finite domain
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
```

### Pitfall 8: Cartesian Products

```mangle
# INEFFICIENT (10K x 10K = 100M intermediate results)
slow(X, Y) :- table1(X), table2(Y), filter(X, Y).

# EFFICIENT (filter first, ~100 results)
fast(X, Y) :- filter(X, Y), table1(X), table2(Y).
```

### Pitfall 9: Missing Structured Data Accessors

```mangle
# WRONG - direct field access does not work
bad(Name) :- record({/name: Name}).

# CORRECT - use :match_field
good(Name) :- record(R), :match_field(R, /name, Name).
```

### Pitfall 10: Go Integration Type Errors

```go
// WRONG - String-based API doesn't exist
store.Add("parent", "alice", "bob")

// CORRECT - Must use engine.Value types
f, _ := factstore.MakeFact("/parent", []engine.Value{
    engine.Atom("alice"),
    engine.Atom("bob"),
})
store.Add(f)
```

## Installation & REPL

```bash
# Install Mangle interpreter
GOBIN=~/bin go install github.com/google/mangle/interpreter/mg@latest

# Start REPL
~/bin/mg

# REPL commands
::load file.mg      # Load program
?predicate(X, Y)    # Query
::show all          # Show all predicates
::help              # Help
Ctrl-D              # Exit
```

## Validation Scripts

This skill includes validation tools for Mangle programs and Go integration code.

### validate_mangle.py - Mangle Syntax Validator

Comprehensive validation for Mangle source files:

```bash
# Basic validation
python3 scripts/validate_mangle.py program.mg

# Strict mode (checks undeclared predicates)
python3 scripts/validate_mangle.py program.mg --strict

# Validate inline code
python3 scripts/validate_mangle.py --check-string "parent(/a, /b)."

# Verbose output (shows stratification info)
python3 scripts/validate_mangle.py program.mg --verbose
```

**Checks performed:**

- Syntax validation (periods, balanced brackets, arrow operators)
- Declaration syntax (Decl with .Type<> and modes)
- Aggregation pipelines (|> do fn: let)
- Safety constraints (head variables bound in body)
- Negation safety (variables bound before negation)
- Built-in function validation (fn:count, fn:sum, etc.)
- Basic stratification analysis

### diagnose_stratification.py - Stratification Diagnostic Tool

**Deep analysis for stratification issues** - the #1 cause of "unsafe" or "cannot compute fixpoint" errors:

```bash
# Analyze a file for stratification issues
python3 scripts/diagnose_stratification.py program.mg

# Detailed dependency analysis
python3 scripts/diagnose_stratification.py program.mg --verbose

# Output dependency graph in DOT format (for Graphviz)
python3 scripts/diagnose_stratification.py program.mg --graph > deps.dot

# JSON output for tooling integration
python3 scripts/diagnose_stratification.py program.mg --json

# Check inline code
python3 scripts/diagnose_stratification.py --check-string "bad(X) :- !bad(X)."
```

**What it detects:**

| Pattern | Detection | Fix Suggestion |
|---------|-----------|----------------|
| Direct self-negation | `bad(X) :- !bad(X)` | Use base case or split predicates |
| Mutual recursion | `winning -> losing -> !winning` | Add terminal conditions |
| Game theory cycles | Classic minimax patterns | Provides stratified rewrite |
| Complex negative cycles | Multi-predicate cycles | Suggests helper predicates |

**Output includes:**
- Stratum assignment for all predicates
- Cycle path for violations
- Line numbers where issues occur
- Targeted fix suggestions based on violation type
- DOT graph visualization for dependency analysis

**Exit codes:**
- `0` - Program is stratified (safe)
- `1` - Stratification violations found
- `2` - Parse/fatal error

### dead_code.py - Dead Code Detection Tool

**Finds rules that can never fire and unreachable/unused code**:

```bash
# Check policy and schema files
python3 scripts/dead_code.py policy.mg schemas.mg

# Full report (default)
python3 scripts/dead_code.py policy.mg schemas.mg --report

# JSON output for tooling
python3 scripts/dead_code.py *.mg --json > dead_code.json

# Only show unused predicates
python3 scripts/dead_code.py policy.mg --unused-only

# Only show undefined predicates
python3 scripts/dead_code.py policy.mg --undefined-only

# Ignore virtual/builtin predicates
python3 scripts/dead_code.py policy.mg --ignore query_learned --ignore recall_similar

# Verbose output with suggestions
python3 scripts/dead_code.py policy.mg --verbose
```

**What it detects:**

| Issue Type | Description | Severity |
|------------|-------------|----------|
| Unreachable rules | Rules with undefined body predicates | Error |
| Unused predicates | Predicates defined but never referenced | Warning |
| Undefined predicates | Predicates used but never defined | Error |
| Shadowed rules | Rules that may be unreachable due to earlier rules | Info |

**Output includes:**

- File and line numbers for all issues
- List of problematic predicates
- Fix suggestions for each issue type
- Summary statistics (rules, predicates, issues)

**Exit codes:**

- `0` - No dead code detected
- `1` - Dead code found
- `2` - Parse/fatal error

### trace_query.py - Query Evaluation Tracer

**Step-by-step query debugging** - answers "why doesn't my rule derive anything?":

```bash
# Basic query tracing
python3 scripts/trace_query.py policy.mg --query "next_action(X)"

# Query with seed facts
python3 scripts/trace_query.py policy.mg --query "next_action(X)" \
    --facts "user_intent(/id1, /query, /read, /foo, _). test_state(/failing)."

# Verbose tracing
python3 scripts/trace_query.py policy.mg --query "block_commit(X)" -v

# Inline code
python3 scripts/trace_query.py --check-string "a(X) :- b(X). b(1)." --query "a(X)"
```

**What it shows:**

| Output | Description |
|--------|-------------|
| Rules attempted | Which rules could potentially match |
| Variable bindings | How variables bind during evaluation |
| Body predicate status | Which body predicates succeed/fail |
| Final results | Derived facts or failure explanation |

**Exit codes:**

- `0` - Query succeeded with results
- `1` - Query failed (no results)
- `2` - Parse/usage error

### explain_derivation.py - Predicate Lineage/Provenance

**Proof tree visualization** - answers "how would X be derived?" and "why is X true?":

```bash
# Explain a fully ground fact
python3 scripts/explain_derivation.py policy.mg schemas.mg \
    --explain "delegate_task(/coder, \"fix bug\", /pending)"

# Explain with variables (all possible derivations)
python3 scripts/explain_derivation.py policy.mg --explain "next_action(X)"

# Show all derivation paths
python3 scripts/explain_derivation.py policy.mg \
    --explain "next_action(/run_tests)" --all-paths

# Export to JSON
python3 scripts/explain_derivation.py policy.mg \
    --explain "permitted(/fs_read)" --json
```

**Output includes:**

- Complete proof tree from goal to base facts
- Which rules contribute to the derivation
- Variable bindings at each step
- All alternative derivation paths (with `--all-paths`)

**Predicate format:**

```mangle
# Ground (fully specified)
delegate_task(/coder, "fix bug", /pending)

# Partially ground (with variables)
delegate_task(/coder, Task, /pending)
next_action(X)
```

### analyze_module.py - Cross-File Module Analyzer

**Multi-file coherence analysis** - detects cross-file issues in large Mangle codebases:

```bash
# Analyze all core policy files
python3 scripts/analyze_module.py internal/core/defaults/*.mg

# Check completeness (fail on undefined predicates)
python3 scripts/analyze_module.py *.mg --check-completeness

# With known virtual predicates (implemented in Go)
python3 scripts/analyze_module.py *.mg --check-completeness \
    --virtual "file_content,symbol_at,recall_similar"

# Generate dependency graph
python3 scripts/analyze_module.py *.mg --graph | dot -Tpng > deps.png

# JSON output for tooling
python3 scripts/analyze_module.py *.mg --json
```

**What it detects:**

| Issue Type | Description | Severity |
|------------|-------------|----------|
| Missing definitions | Predicate used but never defined | Error |
| Duplicate definitions | Same predicate defined in multiple files | Warning |
| Arity mismatches | Same predicate with different arities | Error |
| Unused exports | Defined but never referenced elsewhere | Info |
| Circular dependencies | File A depends on B depends on A | Warning |

**Exit codes:**

- `0` - No issues found
- `1` - Conflicts or missing definitions found
- `2` - Parse/fatal error

### generate_stubs.py - Go Virtual Predicate Stub Generator

**Generates idiomatic Go stubs** for Mangle virtual predicates, accelerating VirtualStore development:

```bash
# Generate stubs for all predicates
python3 scripts/generate_stubs.py internal/core/defaults/schemas.mg \
    --output stubs.go

# List all predicates with arities
python3 scripts/generate_stubs.py schemas.mg --list

# Generate only virtual predicates (Section 7B)
python3 scripts/generate_stubs.py schemas.mg --virtual-only \
    --output virtual_preds.go

# Generate specific predicates
python3 scripts/generate_stubs.py schemas.mg \
    --predicates "user_intent,recall_similar" --output user_preds.go

# Generate interface definitions only
python3 scripts/generate_stubs.py schemas.mg --interface-only

# Custom package name
python3 scripts/generate_stubs.py schemas.mg --package mangle --output gen.go
```

**Type mappings:**

| Mangle Type | Go Type |
|-------------|---------|
| `Type<n>`, `Type<name>` | `engine.Atom` |
| `Type<string>` | `engine.String` |
| `Type<int>` | `engine.Int64` |
| `Type<float>` | `engine.Float64` |
| `Type<[T]>` | `engine.List` |
| `Type<{/k: v}>` | `engine.Map` |
| `Type<Any>` | `engine.Value` |

**Generated code includes:**

- Struct type for each predicate
- `Name()` and `Arity()` methods
- Documented `Query()` stub with binding pattern examples
- `RegisterPredicates()` function for batch registration

### profile_rules.py - Performance Analyzer

**Static analysis for Cartesian explosion risks** and expensive rule patterns:

```bash
# Basic analysis
python3 scripts/profile_rules.py policy.mg

# CI/CD check (fail on high-risk rules)
python3 scripts/profile_rules.py policy.mg --warn-expensive

# Only show medium+ severity
python3 scripts/profile_rules.py policy.mg --threshold medium

# Show rewrite suggestions
python3 scripts/profile_rules.py policy.mg --suggest-rewrites

# With predicate size estimates
echo '{"big_table": 10000, "filter": 100}' > sizes.json
python3 scripts/profile_rules.py policy.mg --estimate-sizes sizes.json

# Full analysis
python3 scripts/profile_rules.py policy.mg -v --suggest-rewrites
```

**What it detects:**

| Pattern | Risk | Description |
|---------|------|-------------|
| Cartesian products | High | Multiple large predicates before filter |
| Unbounded joins | Medium | Predicates without shared variables |
| Nested aggregation | Medium | Complex `\|>` pipelines |
| Large recursive rules | Medium | Recursive predicates with wide bodies |
| Non-indexed lookups | Low | Missing selectivity optimizations |

**Example Cartesian explosion:**

```mangle
# BAD: 10K × 10K = 100M combinations before filter!
result(X, Y) :- big_table(X), big_table(Y), filter(X, Y).

# GOOD: Only 100 filter matches × 2 lookups = 200 operations
result(X, Y) :- filter(X, Y), big_table(X), big_table(Y).
```

**Exit codes:**

- `0` - No high-risk rules (or `--warn-expensive` not set)
- `1` - High-risk rules found and `--warn-expensive` set
- `2` - Parse/fatal error

### validate_go_mangle.py - Go Integration Validator

Validates Go code that uses the Mangle library:

```bash
# Validate single file
python3 scripts/validate_go_mangle.py internal/mangle/engine.go

# Validate directory
python3 scripts/validate_go_mangle.py internal/

# Validate entire codebase
python3 scripts/validate_go_mangle.py --codebase /path/to/project
```

**Checks performed:**

- Correct github.com/google/mangle/* imports
- Proper AST type handling (Constant.Type checks)
- Engine API usage patterns (EvalProgram, QueryContext)
- Error handling for parse/analysis operations
- Fact/Atom construction correctness
- codeNERD-specific patterns (VirtualStore, ToAtom)

### generate_template.py - Template Generator

Generate boilerplate Mangle files:

```bash
python3 scripts/generate_template.py schema > schemas.gl
python3 scripts/generate_template.py policy > policy.gl
```

## Asset Templates

Ready-to-use templates and examples in `assets/`:

### Starter Templates

| Asset | Purpose |
|-------|---------|
| [starter-schema.gl](assets/starter-schema.gl) | Schema template with EDB declarations |
| [starter-policy.gl](assets/starter-policy.gl) | Policy template with common IDB rules |
| [codenerd-schemas.gl](assets/codenerd-schemas.gl) | codeNERD neuro-symbolic architecture schemas |

### Example Programs

| Asset | Demonstrates |
|-------|--------------|
| [vulnerability-scanner.mg](assets/examples/vulnerability-scanner.mg) | Dependency analysis, CVE propagation |
| [access-control.mg](assets/examples/access-control.mg) | RBAC, role hierarchy, permissions |
| [aggregation-patterns.mg](assets/examples/aggregation-patterns.mg) | All aggregation patterns |

### Go Integration Boilerplate

| Asset | Purpose |
|-------|---------|
| [go-integration/main.go](assets/go-integration/main.go) | Complete Go embedding example |
| [go-integration/go.mod](assets/go-integration/go.mod) | Go module definition |

**Quick Start**:

```bash
# Copy Go boilerplate
cp -r assets/go-integration/ myproject/
cd myproject && go mod tidy && go run main.go

# Start new Mangle project
cp assets/starter-schema.gl schemas.gl
cp assets/starter-policy.gl policy.gl
```

## Comparison with Alternatives

| Feature | Mangle | Prolog | SQL | Datalog | Z3/SMT | MiniZinc |
|---------|--------|--------|-----|---------|--------|----------|
| **Logic programming** | ✅ Bottom-up | ✅ Top-down | ❌ | ✅ Bottom-up | ✅ | ❌ |
| **Recursion** | ✅ Native | ✅ Native | ⚠️ CTE only | ✅ Native | ⚠️ Limited | ❌ |
| **Aggregation** | ✅ Transforms | ⚠️ Bagof/setof | ✅ GROUP BY | ⚠️ Limited | ❌ | ⚠️ Limited |
| **Negation** | ✅ Stratified | ✅ NAF | ✅ NOT EXISTS | ✅ Stratified | ✅ | ❌ |
| **Optimization** | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| **Type system** | ⚠️ Optional | ❌ Untyped | ✅ Strong | ⚠️ Optional | ✅ | ✅ |
| **Best for** | Graph analysis | AI/logic | CRUD | Knowledge base | Constraints | Scheduling |

## Resources

- **GitHub**: <https://github.com/google/mangle>
- **Documentation**: <https://mangle.readthedocs.io>
- **Go Packages**: <https://pkg.go.dev/github.com/google/mangle>
- **Demo Service**: <https://github.com/burakemir/mangle-service>

## Support

For comprehensive answers:

1. Check the numbered references (000-900)
2. Search patterns in 300-PATTERN_LIBRARY
3. Review [context7-mangle](references/context7-mangle.md) for up-to-date API patterns
4. See GitHub issues for known problems

---

**Next step**: Read [000-ORIENTATION](references/000-ORIENTATION.md) to understand how to navigate this encyclopedic reference system.
