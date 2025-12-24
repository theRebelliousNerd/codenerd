---
name: mangle-programming
description: Master Google's Mangle declarative programming language for deductive database programming, constraint-like reasoning, and software analysis. From basic facts to production deployment, graph traversal to vulnerability detection, theoretical foundations to optimization. Includes comprehensive AI failure mode prevention (atom/string confusion, aggregation syntax, safety violations, stratification errors). Complete encyclopedic reference with progressive disclosure architecture.
license: Apache-2.0
version: 0.6.0
mangle_version: 0.4.0 (November 1, 2024)
last_updated: 2025-12-23
---

# Mangle Programming: The Complete Reference

**Google Mangle** is a declarative deductive database language extending Datalog with practical features for modern software analysis, security evaluation, and multi-source data integration.

## Quick Start: Your First Program

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

### Common Silent Failures

| Pattern | WRONG | CORRECT |
|---------|-------|---------|
| Constants | `"active"` | `/active` |
| Aggregation | `sum(X)` | `\|> let S = fn:Sum(X)` |
| Grouping | `GROUP BY X` | `\|> do fn:group_by(X)` |
| Declaration | `.decl p(x:int)` | `Decl p(X.Type<int>).` |
| Negation | `not foo(X)` alone | `gen(X), not foo(X)` |
| Struct access | `R.field` | `:match_field(R, /field, V)` |

## When to Use Mangle

✅ **Perfect for**: Dependency analysis, vulnerability detection, graph reachability, multi-source data integration, recursive relationships, security policy compliance

❌ **Not suitable for** (See Section 11 in [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md)):
- **Fuzzy/semantic matching** → Use vector embeddings
- **String manipulation** → No `fn:string_contains`, `fn:substring`, `fn:regex` exist!
- **HashMap-style lookup** → Mangle is for deduction, not key-value storage

## Essential Quick Reference

### Data Types

```mangle
/name               # Named constant (atom)
42, -17, 3.14       # Numbers
"text"              # Strings
[1, 2, 3]           # Lists
{/k: v}             # Structs
[/a: /foo, /b: /bar]  # Maps
```

### Built-in Functions

```mangle
fn:Count()                        # Count elements
fn:Sum(V), fn:Max(V), fn:Min(V)   # Aggregations
fn:group_by(V1, V2, ...)          # Group by
fn:plus(A, B), fn:minus(A, B)     # Arithmetic
fn:list:get(List, Index)          # List access (0-based)
:match_cons(List, Head, Tail)     # List destructure
:match_field(Struct, /field, Val) # Struct field access
```

### Core Patterns

```mangle
# Transitive Closure
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).

# Negation (Set Difference)
safe(X) :- candidate(X), !excluded(X).

# Aggregation
count_by_category(Cat, N) :-
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:Count().

# Structured Data Query
volunteer_name(Id, Name) :-
    volunteer_record(R), :match_field(R, /id, Id), :match_field(R, /name, Name).
```

## Common Pitfalls

### Atom vs String Confusion (CRITICAL)

```mangle
# WRONG - "active" is a string, not an atom
status(User, "active").

# CORRECT - /active is an atom (interned constant)
status(User, /active).
```

### Wrong Aggregation Syntax

```mangle
# WRONG - SQL-style
Total = sum(Amount).

# CORRECT - Full pipeline syntax
sales(Region, Amount) |>
do fn:group_by(Region),
let Total = fn:Sum(Amount).
```

### Unbound Variables in Negation

```mangle
# WRONG - X not bound first (WILL CRASH)
bad(X) :- not foo(X).

# CORRECT - X bound by candidate first
good(X) :- candidate(X), not foo(X).
```

See [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md) for all 30+ failure patterns.

## The Reference Library

This skill uses **progressive disclosure**: Start with quick references, explore numbered references for depth.

### Core References

| Reference | Contents |
|-----------|----------|
| [000-ORIENTATION](references/000-ORIENTATION.md) | Navigation and learning paths |
| [100-FUNDAMENTALS](references/100-FUNDAMENTALS.md) | Theory, concepts, mental models |
| [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md) | **CRITICAL: Read before writing Mangle** |
| [200-SYNTAX_REFERENCE](references/200-SYNTAX_REFERENCE.md) | Complete language specification |
| [300-PATTERN_LIBRARY](references/300-PATTERN_LIBRARY.md) | Every common pattern |
| [400-RECURSION_MASTERY](references/400-RECURSION_MASTERY.md) | Deep dive on recursive techniques |
| [450-PROMPT_ATOM_PREDICATES](references/450-PROMPT_ATOM_PREDICATES.md) | JIT Prompt Compiler predicates |
| [500-AGGREGATION_TRANSFORMS](references/500-AGGREGATION_TRANSFORMS.md) | Complete aggregation guide |
| [600-TYPE_SYSTEM](references/600-TYPE_SYSTEM.md) | Types, lattices, gradual typing |
| [700-OPTIMIZATION](references/700-OPTIMIZATION.md) | Performance engineering |
| [800-THEORY](references/800-THEORY.md) | Mathematical foundations |
| [900-ECOSYSTEM](references/900-ECOSYSTEM.md) | Tools, libraries, integrations |
| [GO_API_REFERENCE](references/GO_API_REFERENCE.md) | Complete Go package documentation |
| [context7-mangle](references/context7-mangle.md) | Up-to-date API patterns (v0.4.0) |

## Learning Paths

### Beginner (0-2 hours)
1. Read this file - Quick Start and Critical section
2. Read [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md)
3. Read [000-ORIENTATION](references/000-ORIENTATION.md)
4. Try examples from [300-PATTERN_LIBRARY](references/300-PATTERN_LIBRARY.md)

### Intermediate (2-8 hours)
1. Complete beginner path
2. Read [200-SYNTAX_REFERENCE](references/200-SYNTAX_REFERENCE.md)
3. Study [400-RECURSION_MASTERY](references/400-RECURSION_MASTERY.md) and [500-AGGREGATION_TRANSFORMS](references/500-AGGREGATION_TRANSFORMS.md)

### Advanced (8-20 hours)
1. Complete intermediate path
2. Read [600-TYPE_SYSTEM](references/600-TYPE_SYSTEM.md), [700-OPTIMIZATION](references/700-OPTIMIZATION.md), [800-THEORY](references/800-THEORY.md)

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
```

## Validation Tools

Scripts for validating Mangle programs. See [VALIDATION_TOOLS](references/VALIDATION_TOOLS.md) for complete documentation.

| Tool | Purpose |
|------|---------|
| `validate_mangle.py` | Syntax validation, safety checks |
| `diagnose_stratification.py` | Stratification issue detection |
| `dead_code.py` | Unreachable/unused code detection |
| `trace_query.py` | Query evaluation tracing |
| `explain_derivation.py` | Proof tree visualization |
| `analyze_module.py` | Cross-file coherence analysis |
| `generate_stubs.py` | Go virtual predicate stubs |
| `profile_rules.py` | Cartesian explosion detection |
| `validate_go_mangle.py` | Go integration validation |

Quick usage:
```bash
python3 scripts/validate_mangle.py program.mg --strict
python3 scripts/diagnose_stratification.py program.mg --verbose
```

## Asset Templates

| Asset | Purpose |
|-------|---------|
| [starter-schema.gl](assets/starter-schema.gl) | Schema template with EDB declarations |
| [starter-policy.gl](assets/starter-policy.gl) | Policy template with common IDB rules |
| [examples/](assets/examples/) | Vulnerability scanner, access control, aggregation |
| [go-integration/](assets/go-integration/) | Complete Go embedding example |

## Resources

- **GitHub**: <https://github.com/google/mangle>
- **Documentation**: <https://mangle.readthedocs.io>
- **Go Packages**: <https://pkg.go.dev/github.com/google/mangle>

---

**Next step**: Read [000-ORIENTATION](references/000-ORIENTATION.md) to navigate this encyclopedic reference system.
