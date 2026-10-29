# Mangle Anti-Patterns Reference

> What NOT to do when programming in Mangle

This directory contains comprehensive documentation of common mistakes developers make when learning Mangle, organized by the type of incorrect mental model being applied.

---

## Overview

Mangle is a **Datalog-based logic programming language**. Developers coming from other paradigms often try to apply familiar patterns that don't work in Mangle's declarative, immutable, pure logic environment.

These anti-patterns are categorized by the "stochastic gap" - the cognitive distance between what LLMs (and humans) expect based on training data and what Mangle actually supports.

---

## Anti-Pattern Categories

### 1. Syntax Translation Errors

Attempting to use syntax from similar-looking languages:

- **[SQL Thinking](sql_thinking.md)** - SELECT/FROM/WHERE, aggregation, NULL handling
- **[Prolog Thinking](prolog_thinking.md)** - Lowercase variables, cut operator, assert/retract
- **[Soufflé Thinking](souffle_thinking.md)** - `.decl` syntax, pragmas, subsumption

**Core Issue:** Mangle has its own syntax that resembles but differs from SQL, Prolog, and Soufflé.

---

### 2. Paradigm Mismatches

Trying to use patterns from incompatible programming paradigms:

- **[Imperative Thinking](imperative_thinking.md)** - Sequential steps, loops, mutation, control flow
- **[OOP Thinking](oop_thinking.md)** - Objects, methods, inheritance, encapsulation
- **[Ordering Assumptions](ordering.md)** - Relying on rule/atom execution order

**Core Issue:** Mangle is declarative (describes facts and relationships) not imperative (describes steps and procedures).

---

### 3. Semantic Mismatches

Misunderstanding Mangle's logical semantics:

- **[Null Handling](null_handling.md)** - Trying to use null, None, undefined
- **[Mutation](mutation.md)** - Trying to update facts, increment counters, modify state
- **[Side Effects](side_effects.md)** - Expecting I/O, logging, randomness, time

**Core Issue:** Mangle follows the Closed World Assumption with immutable facts and pure logic.

---

### 4. Limited Built-in Support

Expecting rich standard library features that don't exist:

- **[String Manipulation](string_manipulation.md)** - Regex, split, formatting, interpolation

**Core Issue:** Mangle has minimal string support - complex string operations belong in Go.

---

### 5. Meta-Programming Limitations

Expecting runtime code generation and reflection:

- **[Dynamic Predicates](dynamic_predicates.md)** - Creating predicates at runtime, computed names
- **[Meta-Programming](meta_programming.md)** - Macros, templates, reflection, self-modification

**Core Issue:** Mangle predicates are static - use Go for code generation.

---

## Quick Reference: Top 30 Errors

Based on the codeNERD project documentation, here are the most common mistakes:

### Syntax (10 errors)

1. Using `"active"` instead of `/active` (atoms)
2. `.decl` instead of `Decl` (declaration syntax)
3. Lowercase variables `x` instead of `X`
4. Inline aggregation without pipe `|>`
5. Implicit grouping (no `fn:group_by()`)
6. Missing period `.` at end of rule
7. Wrong comment syntax (`//` instead of `#`)
8. Assignment `:=` instead of unification `=`
9. Map dot notation `Map.key` instead of `:match_entry()`
10. List indexing `List[0]` instead of `:match_cons()`

### Logic/Safety (8 errors)

11. Unsafe head variables (unbound in body)
12. Unsafe negation (variables not bound first)
13. Stratification cycles (recursion through negation)
14. Infinite recursion (no base case or limit)
15. Cartesian product (large tables before filters)
16. Null checking (`X != null`)
17. Duplicate rule names expecting overwrite (actually union)
18. Anonymous variable `_` misuse (each is independent)

### Data Structures (6 errors)

19. Type mismatch (int vs float)
20. String interpolation (`"Error: $Code"`)
21. Hallucinated functions (`fn:split`, `fn:date`)
22. Aggregation safety (unbounded grouping vars)
23. Struct syntax (JSON `{"key": "value"}` instead of `{/key: "value"}`)
24. Fact store type errors (passing raw strings instead of `engine.Value`)

### Integration (6 errors)

25. Incorrect entry point (`engine.Run()` doesn't exist)
26. Ignoring imports (missing package references)
27. Wrong external predicate signature
28. Parsing vs execution confusion (passing raw strings)
29. Assuming I/O access in rules (`read_file()`)
30. Package hallucination (importing non-existent libraries)

---

## Mental Model Shift

### From Imperative to Declarative

| Imperative | Declarative (Mangle) |
|------------|----------------------|
| "Do this, then that" | "These facts are true" |
| Steps execute in order | All rules apply simultaneously |
| Variables store changing values | Variables are immutable bindings |
| Functions have side effects | Predicates are pure relations |
| State mutates over time | Facts derive from facts |
| Control flow (if/while/for) | Logical conditions (guards) |

### From Object-Oriented to Relational

| OOP | Logic Programming |
|-----|-------------------|
| Objects with methods | Predicates with arguments |
| Inheritance | Multiple rules (union) |
| Encapsulation | Naming conventions |
| Mutation | Retract-and-replace (in Go) |
| `object.property` | Separate predicates or `:match_field()` |

### From Open World to Closed World

| Open World (SQL) | Closed World (Mangle) |
|------------------|----------------------|
| NULL = unknown | No null - absent = false |
| Three-valued logic | Two-valued logic |
| `IS NULL` checks | Negation: `not pred(_)` |
| Nullable fields | Separate optional predicates |

---

## The Golden Rules

1. **Facts are immutable** - Create new facts, don't modify existing ones
2. **Predicates are pure** - No side effects, I/O, or randomness
3. **Variables are bindings** - Once bound, they never change
4. **Rules are simultaneous** - Don't rely on execution order
5. **Atoms need slashes** - `/active` not `"active"`
6. **Variables need uppercase** - `X` not `x`
7. **Negation needs safety** - Bind variables before `not`
8. **Recursion needs base cases** - Always include termination
9. **Aggregation needs pipes** - Use `|> do fn:group_by()`
10. **Complex logic needs Go** - String parsing, I/O, code generation

---

## How to Use These References

### For Learning

1. Read the anti-pattern guide for your background:
   - SQL developer? Start with [sql_thinking.md](sql_thinking.md)
   - Python/Java developer? Start with [imperative_thinking.md](imperative_thinking.md) and [oop_thinking.md](oop_thinking.md)
   - Prolog developer? Start with [prolog_thinking.md](prolog_thinking.md)

2. Read common semantic issues:
   - [null_handling.md](null_handling.md)
   - [mutation.md](mutation.md)
   - [side_effects.md](side_effects.md)

3. Understand limitations:
   - [string_manipulation.md](string_manipulation.md)
   - [dynamic_predicates.md](dynamic_predicates.md)

### For Debugging

When your Mangle code doesn't work:

1. Check syntax errors first:
   - Are atoms prefixed with `/`?
   - Are variables UPPERCASE?
   - Does every statement end with `.`?
   - Is aggregation using `|>` syntax?

2. Check safety errors:
   - Are all head variables bound in the body?
   - Are variables in `not` bound before negation?
   - Is recursion bounded?

3. Check semantic errors:
   - Are you trying to mutate facts?
   - Are you expecting side effects?
   - Are you checking for null?

4. Check integration errors:
   - Are you using the correct Go API?
   - Are facts loaded with proper types?
   - Are external predicates registered?

### For Code Review

When reviewing Mangle code, watch for:

- [ ] Atoms without `/` prefix
- [ ] Lowercase variables
- [ ] Missing periods `.`
- [ ] Unsafe negation
- [ ] Unbounded recursion
- [ ] Null checks (`!= null`)
- [ ] Mutation attempts
- [ ] Side effects in rules
- [ ] Order-dependent logic
- [ ] Complex string manipulation

---

## Additional Resources

From the codeNERD project:

- **Mangle Programming Skill**: `/skill:mangle-programming`
  - Complete language reference from basics to production optimization
  - Located in `.claude/skills/mangle-programming/references/`

- **codeNERD Builder Skill**: `/skill:codenerd-builder`
  - Integration patterns for Mangle + Go
  - Located in `.claude/skills/codenerd-builder/references/`

- **Project Documentation**:
  - `internal/mangle/schemas.gl` - Schema examples
  - `internal/mangle/policy.gl` - Rule examples
  - `CLAUDE.md` - Top 30 common errors list

---

## Contributing

When adding new anti-patterns:

1. Follow the existing format:
   - Category header
   - Description
   - Wrong Approach (with example)
   - Why It Fails (explanation)
   - Correct Mangle Way (with example)

2. Include:
   - Code examples for both wrong and right approaches
   - Clear explanations of why the anti-pattern fails
   - Multiple correct alternatives when applicable
   - Key insights or design principles

3. Cross-reference:
   - Link to related anti-patterns
   - Update this README's index
   - Add to Quick Reference if in top 30

---

## License

Part of the codeNERD project. See project root for license information.

---

## Feedback

These anti-patterns were developed through iterative debugging of LLM-generated Mangle code. If you encounter new anti-patterns or have corrections, please contribute!

**Remember:** The best way to learn Mangle is to understand what NOT to do from your previous programming experience.
