# Mangle Adversarial Test Index

Quick reference for finding specific error patterns.

## By Error Type

### Atom/String Confusion
- `syntactic/atom_string_confusion.mg` - Lines 5-40
- `types/atom_vs_string.mg` - Lines 5-60

### Aggregation Errors
- `syntactic/inline_aggregation.mg` - Lines 5-90
- `safety/unsafe_negation.mg:Test 5` - Lines 35-40

### Anonymous Variable `_` Misuse
- `safety/anonymous_misuse.mg` - Lines 5-120

### Assignment Operators
- `syntactic/assignment_operators.mg` - Lines 5-70

### Bracket Notation
- `structures/bracket_notation.mg` - Lines 5-140

### Cartesian Products
- `loops/cartesian_explosion.mg` - Lines 5-200

### Comments
- `syntactic/wrong_comments.mg` - Lines 5-50

### Dot Notation
- `structures/dot_notation.mg` - Lines 5-100

### Function Hallucinations
- `types/hallucinated_functions.mg` - Lines 5-110

### Infinite Loops
- `loops/direct_self_reference.mg` - Lines 5-90
- `loops/mutual_recursion.mg` - Lines 5-110
- `loops/unbounded_counter.mg` - Lines 5-120

### JSON Syntax
- `structures/json_syntax.mg` - Lines 5-120

### List/Scalar Confusion
- `types/list_in_scalar.mg` - Lines 5-90

### Lowercase Variables
- `syntactic/lowercase_vars.mg` - Lines 5-55

### Missing Periods
- `syntactic/missing_periods.mg` - Lines 5-50

### Negation Safety
- `safety/unsafe_negation.mg` - Lines 5-85
- `safety/negation_order.mg` - Lines 5-90

### Number Type Confusion
- `types/int_vs_float.mg` - Lines 5-80

### Soufflé Syntax
- `syntactic/souffle_syntax.mg` - Lines 5-60

### Stratification Violations
- `safety/stratification_cycles.mg` - Lines 5-100

### Unbound Variables
- `safety/unbound_head_vars.mg` - Lines 5-85

## By Severity

### Critical (Will Crash/Hang)
1. `loops/direct_self_reference.mg` - Infinite recursion
2. `loops/mutual_recursion.mg` - Mutual infinite recursion
3. `loops/unbounded_counter.mg` - Memory exhaustion
4. `loops/cartesian_explosion.mg` - Exponential blowup
5. `safety/stratification_cycles.mg` - Non-terminating logic

### High (Parse/Safety Errors)
1. `safety/unsafe_negation.mg` - Unsafe logic
2. `safety/unbound_head_vars.mg` - Invalid derivation
3. `syntactic/souffle_syntax.mg` - Wrong language syntax
4. `syntactic/missing_periods.mg` - Malformed programs

### Medium (Type Errors)
1. `types/atom_vs_string.mg` - Type mismatches
2. `types/int_vs_float.mg` - Numeric type errors
3. `types/list_in_scalar.mg` - Container type errors
4. `types/hallucinated_functions.mg` - Undefined functions

### Low (Syntax Variations)
1. `syntactic/lowercase_vars.mg` - Variable naming
2. `syntactic/wrong_comments.mg` - Comment syntax
3. `structures/dot_notation.mg` - Access patterns
4. `structures/bracket_notation.mg` - Indexing syntax
5. `structures/json_syntax.mg` - Literal syntax

## By Language Source (AI Bias)

### SQL Bias
- `syntactic/inline_aggregation.mg`
- `syntactic/souffle_syntax.mg:Test 3`
- `syntactic/wrong_comments.mg:Test 3`
- `loops/cartesian_explosion.mg:Test 2, 8, 9`

### Prolog Bias
- `syntactic/lowercase_vars.mg`
- `safety/stratification_cycles.mg`

### Soufflé Bias
- `syntactic/souffle_syntax.mg`

### Python Bias
- `types/hallucinated_functions.mg:Test 1, 4`
- `structures/bracket_notation.mg:Test 2, 5, 8`
- `syntactic/wrong_comments.mg:Test 4`

### JavaScript Bias
- `types/hallucinated_functions.mg:Test 2, 9`
- `structures/dot_notation.mg`
- `structures/json_syntax.mg`
- `structures/bracket_notation.mg:Test 3, 13`

### Java/C# Bias
- `structures/dot_notation.mg:Test 5`

### Imperative Programming Bias
- `syntactic/assignment_operators.mg`
- `structures/bracket_notation.mg:Test 7, 14`

## Search Tags

### #atoms
- `syntactic/atom_string_confusion.mg`
- `types/atom_vs_string.mg`

### #aggregation
- `syntactic/inline_aggregation.mg`

### #recursion
- `loops/direct_self_reference.mg`
- `loops/mutual_recursion.mg`
- `loops/unbounded_counter.mg`

### #negation
- `safety/unsafe_negation.mg`
- `safety/negation_order.mg`
- `safety/stratification_cycles.mg`

### #types
- `types/atom_vs_string.mg`
- `types/int_vs_float.mg`
- `types/list_in_scalar.mg`

### #structures
- `structures/dot_notation.mg`
- `structures/json_syntax.mg`
- `structures/bracket_notation.mg`

### #safety
- All files in `safety/`

### #performance
- `loops/cartesian_explosion.mg`
- `loops/unbounded_counter.mg`

## Quick Start Examples

### Test Parser Error Detection
```bash
# Should all return parse errors
mangle-parse syntactic/*.mg
```

### Test Safety Analyzer
```bash
# Should detect safety violations
mangle-analyze safety/*.mg
```

### Test Type Checker
```bash
# Should detect type errors
mangle-typecheck types/*.mg
```

### Test Termination Analysis
```bash
# Should detect non-terminating programs
mangle-terminates loops/*.mg
```

## File Size Summary

| Category | Files | Lines | Avg Lines/File |
|----------|-------|-------|----------------|
| Syntactic | 7 | ~420 | 60 |
| Safety | 5 | ~400 | 80 |
| Types | 4 | ~360 | 90 |
| Loops | 4 | ~480 | 120 |
| Structures | 3 | ~360 | 120 |
| **Total** | **23** | **~2020** | **88** |

## Cross-References

### Error Type → Multiple Files

**Negation errors appear in:**
- `safety/unsafe_negation.mg`
- `safety/negation_order.mg`
- `safety/stratification_cycles.mg`

**Type confusion appears in:**
- `syntactic/atom_string_confusion.mg`
- `types/atom_vs_string.mg`
- `types/int_vs_float.mg`
- `types/list_in_scalar.mg`

**Recursion errors appear in:**
- `loops/direct_self_reference.mg`
- `loops/mutual_recursion.mg`
- `loops/unbounded_counter.mg`
- `safety/stratification_cycles.mg`

**Structure access appears in:**
- `structures/dot_notation.mg`
- `structures/bracket_notation.mg`
- `structures/json_syntax.mg`

## Testing Checklist

Use this checklist when implementing a Mangle parser/analyzer:

- [ ] Rejects all files in `syntactic/` with appropriate parse errors
- [ ] Detects all safety violations in `safety/`
- [ ] Catches all type errors in `types/`
- [ ] Identifies non-terminating programs in `loops/`
- [ ] Rejects invalid structure access in `structures/`
- [ ] Provides helpful error messages referencing correct syntax
- [ ] Doesn't false-positive on "CORRECT" examples in each file
