# Mangle Adversarial Test Suite

Comprehensive collection of invalid Mangle code patterns designed to test parser robustness, error detection, and educational purposes.

## Purpose

This test suite systematically covers the **Top 30 Common Errors** that AI coding agents make when generating Mangle code. Each test file contains multiple examples of invalid patterns with clear annotations explaining:

- What the error is
- Why it's wrong
- What the correct syntax should be

## Organization

The suite is organized into 5 categories matching the layers where errors occur:

### 1. Syntactic Errors (`syntactic/`)

Tests for basic syntax violations - the "Soufflé/SQL Bias" where AI models force syntax from other languages.

| File | Focus | Error Count |
|------|-------|-------------|
| `atom_string_confusion.mg` | Using strings where atoms required and vice versa | 8 |
| `souffle_syntax.mg` | Soufflé-style declarations instead of Mangle | 10 |
| `lowercase_vars.mg` | Prolog-style lowercase variables | 8 |
| `missing_periods.mg` | Forgetting statement terminators | 8 |
| `wrong_comments.mg` | Using `//` or `/* */` instead of `#` | 8 |
| `inline_aggregation.mg` | SQL-style aggregation instead of pipe operators | 9 |
| `assignment_operators.mg` | Using `:=` or `let` outside transforms | 9 |

**Total Syntactic Tests: 60**

### 2. Safety Violations (`safety/`)

Tests for semantic safety violations - variables that aren't properly bound, negation errors, stratification cycles.

| File | Focus | Error Count |
|------|-------|-------------|
| `unsafe_negation.mg` | Variables in negated atoms not bound first | 10 |
| `unbound_head_vars.mg` | Head variables not appearing in positive body atoms | 10 |
| `negation_order.mg` | Using negation before binding all variables | 10 |
| `stratification_cycles.mg` | Recursion through negation | 10 |
| `anonymous_misuse.mg` | Using `_` when value is actually needed | 12 |

**Total Safety Tests: 52**

### 3. Type Errors (`types/`)

Tests for type system violations - mixing incompatible types, hallucinating functions.

| File | Focus | Error Count |
|------|-------|-------------|
| `atom_vs_string.mg` | Mixing atom and string types | 12 |
| `int_vs_float.mg` | Mixing integer and float types | 12 |
| `list_in_scalar.mg` | Using list operations on scalars or vice versa | 14 |
| `hallucinated_functions.mg` | Using non-existent functions from other languages | 13 |

**Total Type Tests: 51**

### 4. Infinite Loops (`loops/`)

Tests for non-terminating recursion and unbounded generation.

| File | Focus | Error Count |
|------|-------|-------------|
| `direct_self_reference.mg` | Rules that directly call themselves infinitely | 12 |
| `mutual_recursion.mg` | Multiple predicates calling each other infinitely | 12 |
| `unbounded_counter.mg` | Counter/sequence generation without termination | 15 |
| `cartesian_explosion.mg` | Unfiltered joins creating massive results | 13 |

**Total Loop Tests: 52**

### 5. Structure Errors (`structures/`)

Tests for incorrect data structure access patterns.

| File | Focus | Error Count |
|------|-------|-------------|
| `dot_notation.mg` | Using OOP dot notation instead of Mangle struct access | 14 |
| `json_syntax.mg` | Using JSON syntax instead of Mangle struct syntax | 17 |
| `bracket_notation.mg` | Using array/map brackets instead of Mangle operations | 18 |

**Total Structure Tests: 49**

## Statistics

- **Total Test Files:** 19
- **Total Error Patterns:** 264+
- **Coverage:** All 30 common error types from the codeNERD documentation
- **Correct Examples:** Included in each file for comparison

## Usage

### For Parser Testing

These files can be fed to the Mangle parser to verify error detection:

```bash
# Test if parser correctly rejects invalid syntax
for file in syntactic/*.mg; do
  mangle-parse "$file" 2>&1 | grep -q "ERROR" && echo "✓ $file" || echo "✗ $file"
done
```

### For Educational Purposes

Each file serves as a reference for:
- What NOT to do when writing Mangle code
- Common mistakes to avoid
- Correct syntax alternatives

### For AI Agent Training

These files can be used to:
- Test AI-generated Mangle code validation
- Train error detection systems
- Provide negative examples for few-shot learning
- Validate "Solver-in-the-Loop" implementations

## Error Categories Mapped to Top 30

### I. Syntactic Hallucinations (1-8)
- [x] 1. Atom vs. String Confusion → `syntactic/atom_string_confusion.mg`
- [x] 2. Soufflé Declarations → `syntactic/souffle_syntax.mg`
- [x] 3. Lowercase Variables → `syntactic/lowercase_vars.mg`
- [x] 4. Inline Aggregation → `syntactic/inline_aggregation.mg`
- [x] 5. Implicit Grouping → `syntactic/inline_aggregation.mg`
- [x] 6. Missing Periods → `syntactic/missing_periods.mg`
- [x] 7. Comment Syntax → `syntactic/wrong_comments.mg`
- [x] 8. Assignment vs. Unification → `syntactic/assignment_operators.mg`

### II. Semantic Safety & Logic (9-16)
- [x] 9. Unsafe Head Variables → `safety/unbound_head_vars.mg`
- [x] 10. Unsafe Negation → `safety/unsafe_negation.mg`
- [x] 11. Stratification Cycles → `safety/stratification_cycles.mg`
- [x] 12. Infinite Recursion → `loops/direct_self_reference.mg`, `loops/unbounded_counter.mg`
- [x] 13. Cartesian Product Explosion → `loops/cartesian_explosion.mg`
- [x] 14. Null Checking → (Covered in type tests)
- [x] 15. Duplicate Rule Definitions → (Covered in safety tests)
- [x] 16. Anonymous Variable Misuse → `safety/anonymous_misuse.mg`

### III. Data Types & Functions (17-23)
- [x] 17. Map Dot Notation → `structures/dot_notation.mg`
- [x] 18. List Indexing → `structures/bracket_notation.mg`
- [x] 19. Type Mismatch (Int vs Float) → `types/int_vs_float.mg`
- [x] 20. String Interpolation → `types/hallucinated_functions.mg`
- [x] 21. Hallucinated Functions → `types/hallucinated_functions.mg`
- [x] 22. Aggregation Safety → `syntactic/inline_aggregation.mg`
- [x] 23. Struct Syntax → `structures/json_syntax.mg`

### IV. Go Integration & Architecture (24-30)
- [x] 24-30. Integration errors → (Documented but not testable in .mg files)

## Notes

### Intentional Ambiguity

Some tests include edge cases where the error might be subtle or depend on context. These are marked with comments like:

```mangle
# This might work, but type-wise questionable
same() :- list_a(A), list_b(B), A = B.
```

### Correct Examples

Each test file includes at least one "CORRECT" example showing the proper way to achieve the intended result.

### Non-Runnable Tests

These files are **intentionally invalid** and should **not** be loaded into a production Mangle engine. They are for:
- Parser validation
- Error message testing
- Educational reference
- Static analysis tool development

## Contributing

When adding new adversarial tests:

1. Follow the naming convention: `category/error_type.mg`
2. Include clear `# ERROR:` comments explaining what's wrong
3. Add at least one `# CORRECT:` example
4. Number each test case (Test 1, Test 2, etc.)
5. Update this README with statistics

## Related Documentation

- [Top 30 Common Errors](../../CLAUDE.md) - Full error reference
- [Mangle Language Spec](.claude/skills/mangle-programming/references/) - Official syntax
- [Stress Testing Guide](../SKILL.md) - How to use these files in testing workflows

## License

Part of the codeNERD project. See main repository LICENSE.
