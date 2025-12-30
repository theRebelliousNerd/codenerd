# Mangle Development Tools

Python tools for analyzing and diagnosing Google Mangle (Datalog) programs.

## Tools

### 1. `diagnose_stratification.py`
Analyzes Mangle programs for stratification issues - the #1 cause of "unsafe" or "cannot compute fixpoint" errors in Datalog.

**Usage:**
```bash
python diagnose_stratification.py policy.mg
python diagnose_stratification.py policy.mg --verbose --graph > deps.dot
```

**What it detects:**
- Negative cycles (predicates that depend negatively on themselves)
- Stratification violations (unsafe use of negation)
- Mutual recursion through negation

**Output:**
- Text report with cycle detection and fix suggestions
- DOT graph for visualization
- JSON output for tooling integration

### 2. `profile_rules.py`
Static performance analysis tool that estimates Cartesian explosion risks and identifies expensive rules.

**Usage:**
```bash
# Basic analysis
python profile_rules.py policy.mg

# CI/CD integration
python profile_rules.py policy.mg --warn-expensive --threshold medium

# With size estimates
echo '{"big_table": 10000, "filter": 100}' > sizes.json
python profile_rules.py policy.mg --estimate-sizes sizes.json

# Full analysis with rewrites
python profile_rules.py policy.mg --suggest-rewrites
```

**What it detects:**
1. **Cartesian Products (HIGH RISK)**: Predicates with disjoint variables creating exponential blowup
   ```mangle
   # BAD: A and B independent until filter
   bad(A, B) :- table_a(A), table_b(B), filter(A, B).
   # Cost: |table_a| × |table_b| combinations

   # GOOD: Filter first
   good(A, B) :- filter(A, B), table_a(A), table_b(B).
   # Cost: |filter| × 2 lookups
   ```

2. **Late Filtering (MEDIUM RISK)**: Comparisons after expensive joins
   ```mangle
   # BAD: Comparison after joins
   slow(X, Y) :- big(X), big(Y), related(X, Y), X < Y.

   # BETTER: Comparison earlier
   fast(X, Y) :- big(X), big(Y), X < Y, related(X, Y).
   ```

3. **Unbounded Recursion (HIGH RISK)**: Recursive rules without base cases or depth limits

4. **Late Negation (MEDIUM RISK)**: Negation after expensive operations

5. **Suboptimal Ordering (LOW RISK)**: Large predicates before small ones

**Options:**
- `--warn-expensive`: Exit with error code if high-risk rules found (for CI/CD)
- `--threshold LEVEL`: Only show issues at or above level (low/medium/high)
- `--json`: JSON output for tooling
- `--suggest-rewrites`: Show optimized versions of problematic rules
- `--estimate-sizes FILE`: Load predicate size estimates from JSON
- `--verbose`: Show detailed analysis for all rules

**Output:**
```
======================================================================
MANGLE PERFORMANCE ANALYSIS
Source: policy.mg
======================================================================

Rules analyzed: 373

ISSUES FOUND:
  High risk:   29
  Medium risk: 20
  Low risk:    0

[RISK: HIGH] Line 329-334: left_of(A, B)
  left_of(A, B) :-
      interactable(A, _),
      interactable(B, _),
      geometry(A, Ax, _, _, _),
      geometry(B, Bx, _, _, _),
      Ax < Bx.

  ISSUE: Cartesian product between interactable(A) and interactable(B)
  ESTIMATED COST: O(N²) where N = |interactable| ~ 1,000,000 combinations
  SUGGESTION: Add constraint or reorder predicates
```

**Exit Codes:**
- `0`: No high-risk rules (or `--warn-expensive` not set)
- `1`: High-risk rules found and `--warn-expensive` set
- `2`: Parse error or fatal error

## CI/CD Integration

Add to your build pipeline:

```bash
# Check for performance issues
python profile_rules.py internal/mangle/*.mg --warn-expensive --threshold medium

# Check for stratification issues
python diagnose_stratification.py internal/mangle/policy.mg
```

## Size Estimates File

Create a JSON file with predicate cardinality estimates:

```json
{
  "user": 1000,
  "file": 50000,
  "dependency_link": 100000,
  "interactable": 500
}
```

This helps the analyzer provide more accurate cost estimates.

## Requirements

- Python 3.7+
- No external dependencies (uses only stdlib)

## Examples

See `test_performance.mg` for examples of common performance anti-patterns.
