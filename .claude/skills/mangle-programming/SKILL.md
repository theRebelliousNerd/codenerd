---
name: mangle-programming
description: >
  Master Google's Mangle declarative programming language for deductive database programming,
  constraint-like reasoning, and software analysis. From basic facts to production deployment,
  graph traversal to vulnerability detection, theoretical foundations to optimization.
  Includes comprehensive AI failure mode prevention (atom/string confusion, aggregation syntax,
  safety violations, stratification errors). Complete encyclopedic reference with progressive
  disclosure architecture. This skill should also be used when the user asks to "check mangle files",
  "lint .mg files", "get diagnostics for mangle", "find errors in mangle code", "analyze mangle syntax",
  "format mangle files", "get hover info", "find definition", "find references", or when programmatically
  working with Mangle language files (.mg). Includes the mangle-cli tool for parsing, semantic analysis,
  stratification checking, code navigation, batch queries, and CI/CD integration via SARIF output.
license: Apache-2.0
version: 0.8.0
mangle_version: 0.4.0 (November 1, 2024)
last_updated: 2025-12-30
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
| Aggregation | `sum(X)` | `\|> let S = fn:sum(X)` |
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
fn:count()                        # Count elements
fn:sum(V), fn:max(V), fn:min(V)   # Aggregations
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
    let N = fn:count().

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
let Total = fn:sum(Amount).
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
| [250-BUILTINS_COMPLETE](references/250-BUILTINS_COMPLETE.md) | Authoritative built-in functions & predicates |
| [300-PATTERN_LIBRARY](references/300-PATTERN_LIBRARY.md) | Every common pattern |
| [400-RECURSION_MASTERY](references/400-RECURSION_MASTERY.md) | Deep dive on recursive techniques |
| [450-PROMPT_ATOM_PREDICATES](references/450-PROMPT_ATOM_PREDICATES.md) | JIT Prompt Compiler predicates |
| [500-AGGREGATION_TRANSFORMS](references/500-AGGREGATION_TRANSFORMS.md) | Complete aggregation guide |
| [600-TYPE_SYSTEM](references/600-TYPE_SYSTEM.md) | Types, lattices, gradual typing |
| [700-OPTIMIZATION](references/700-OPTIMIZATION.md) | Performance engineering |
| [800-THEORY](references/800-THEORY.md) | Mathematical foundations |
| [900-ECOSYSTEM](references/900-ECOSYSTEM.md) | Tools, libraries, integrations |
| [950-ADVANCED_ARCHITECTURE](references/950-ADVANCED_ARCHITECTURE.md) | God-Tier patterns: ReBAC, taint analysis, bisimulation |
| [GO_API_REFERENCE](references/GO_API_REFERENCE.md) | Complete Go package documentation |
| [VALIDATION_TOOLS](references/VALIDATION_TOOLS.md) | Script documentation for validation tools |
| [context7-mangle](references/context7-mangle.md) | Up-to-date API patterns (v0.4.0) |

### Legacy References (Being Migrated)

| Reference | Contents | Migrating To |
|-----------|----------|--------------|
| [SYNTAX](references/SYNTAX.md) | Original syntax reference | 200-SYNTAX_REFERENCE |
| [EXAMPLES](references/EXAMPLES.md) | Original examples collection | 300-PATTERN_LIBRARY |
| [ADVANCED_PATTERNS](references/ADVANCED_PATTERNS.md) | Complex patterns | 400/500/700/950 |
| [PRODUCTION](references/PRODUCTION.md) | Deployment guide | 900-ECOSYSTEM |

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

## Mangle CLI Tool

The `mangle-cli` provides machine-readable access to Mangle language analysis capabilities for linting, code navigation, and CI/CD integration.

### CLI Location

```bash
node <skill-path>/scripts/mangle-cli.js <command> [options] <files...>
```

**Example**:

```bash
node .claude/skills/mangle-programming/scripts/mangle-cli.js check src/rules.mg
```

### Commands Quick Reference

| Command | Purpose | Key Options |
|---------|---------|-------------|
| `check` | Run all diagnostics | `--format`, `--severity`, `--fail-on` |
| `symbols` | List predicates/clauses | `--format` |
| `hover` | Info at position | `--line`, `--column` |
| `definition` | Find where defined | `--line`, `--column` |
| `references` | Find all uses | `--line`, `--column`, `--include-declaration` |
| `completion` | Get completions | `--line`, `--column` |
| `format` | Format files | `--write`, `--check`, `--diff` |
| `batch` | Multiple queries | JSON file or stdin |
| `file-info` | Complete analysis | (none) |

### Common Workflows

```bash
# Check files for errors (JSON output)
node mangle-cli.js check file.mg

# Human-readable output
node mangle-cli.js check --format text file.mg

# SARIF for GitHub Actions
node mangle-cli.js check --format sarif src/*.mg > results.sarif

# Code navigation
node mangle-cli.js hover file.mg --line 10 --column 5
node mangle-cli.js definition file.mg --line 10 --column 5

# Format files in place
node mangle-cli.js format --write file.mg
```

### CLI Reference Documentation

| Reference | Contents |
|-----------|----------|
| [CLI_COMMANDS](references/CLI_COMMANDS.md) | Detailed command documentation |
| [CLI_ERROR_CODES](references/CLI_ERROR_CODES.md) | All diagnostic codes and meanings |
| [CLI_OUTPUT_SCHEMAS](references/CLI_OUTPUT_SCHEMAS.md) | JSON output schemas |
| [CLI_BATCH_API](references/CLI_BATCH_API.md) | Batch query format and examples |

## Scripts & Tools

Python tools for validating, analyzing, and generating Mangle code. See [VALIDATION_TOOLS](references/VALIDATION_TOOLS.md) and [scripts/README.md](scripts/README.md) for complete documentation.

### Validation & Analysis

| Script | Purpose | Documentation |
|--------|---------|---------------|
| `validate_mangle.py` | Syntax validation, safety checks | [VALIDATION_TOOLS](references/VALIDATION_TOOLS.md) |
| `validate_go_mangle.py` | Go integration validation | [VALIDATION_TOOLS](references/VALIDATION_TOOLS.md) |
| `diagnose_stratification.py` | Stratification & negation cycle detection | [README.md](scripts/README.md) |
| `dead_code.py` | Unreachable/unused code detection | [README.md](scripts/README.md) |
| `profile_rules.py` | Cartesian explosion & performance analysis | [README.md](scripts/README.md) |

### Debugging & Tracing

| Script | Purpose | Documentation |
|--------|---------|---------------|
| `trace_query.py` | Query evaluation tracing | [README_trace_query.md](scripts/README_trace_query.md) |
| `explain_derivation.py` | Proof tree visualization | [README_explain_derivation.md](scripts/README_explain_derivation.md) |
| `analyze_module.py` | Cross-file coherence analysis | [README_ANALYZER.md](scripts/README_ANALYZER.md), [QUICKSTART.md](scripts/QUICKSTART.md) |

### Code Generation

| Script | Purpose | Documentation |
|--------|---------|---------------|
| `generate_stubs.py` | Go virtual predicate stubs | [STUB_GENERATOR_README.md](scripts/STUB_GENERATOR_README.md) |
| `generate_template.py` | Mangle file templates | See script docstring |

### Quick Usage

```bash
# Validate syntax
python3 scripts/validate_mangle.py program.mg --strict

# Detect stratification issues
python3 scripts/diagnose_stratification.py program.mg --verbose

# Profile performance
python3 scripts/profile_rules.py program.mg --warn-expensive

# Analyze multi-file module
python3 scripts/analyze_module.py *.mg --check-completeness

# Generate Go stubs for virtual predicates
python3 scripts/generate_stubs.py schemas.mg --virtual-only --output stubs.go
```

### CI/CD Integration

```bash
# Pre-commit validation
python3 scripts/validate_mangle.py *.mg --strict
python3 scripts/diagnose_stratification.py *.mg
python3 scripts/profile_rules.py *.mg --warn-expensive --threshold medium
```

## Asset Templates

Templates and examples for starting new Mangle projects. See [assets/README.md](assets/README.md) for complete documentation.

### Templates

| Asset | Purpose |
|-------|---------|
| [starter-schema.mg](assets/starter-schema.mg) | Schema template with EDB declarations (entities, attributes, relationships) |
| [starter-policy.mg](assets/starter-policy.mg) | Policy template with common IDB rules (transitive closure, negation, aggregation) |
| [codenerd-schemas.mg](assets/codenerd-schemas.mg) | Schema for codeNERD's neuro-symbolic architecture (intent, world model, TDD) |

### Examples

| Asset | Purpose |
|-------|---------|
| [examples/vulnerability-scanner.mg](assets/examples/vulnerability-scanner.mg) | Dependency tracking, CVE propagation, vulnerability paths |
| [examples/access-control.mg](assets/examples/access-control.mg) | RBAC with role hierarchy, permission inheritance, explicit denials |
| [examples/aggregation-patterns.mg](assets/examples/aggregation-patterns.mg) | All aggregation patterns: count, sum, min/max, grouping, nesting |

### Go Integration

| Asset | Purpose |
|-------|---------|
| [go-integration/](assets/go-integration/) | Complete Go embedding example with `main.go` and `go.mod` |

**Quick start**: Copy templates to start a new project:
```bash
cp assets/starter-schema.mg my-schema.mg
cp assets/starter-policy.mg my-policy.mg
```

## Resources

- **GitHub**: <https://github.com/google/mangle>
- **Documentation**: <https://mangle.readthedocs.io>
- **Go Packages**: <https://pkg.go.dev/github.com/google/mangle>

---

**Next step**: Read [000-ORIENTATION](references/000-ORIENTATION.md) to navigate this encyclopedic reference system.
