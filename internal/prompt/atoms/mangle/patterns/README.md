# Mangle Pattern Library

Comprehensive collection of idiomatic Mangle patterns for common programming tasks. Each pattern includes problem descriptions, solution templates, complete working examples, and variations.

## Pattern Categories

### 1. Graph Operations
**File:** [graph_traversal.md](graph_traversal.md)

- Transitive closure (reachability)
- Shortest paths and distance calculations
- Cycle detection
- Bounded depth traversal
- Connected components

**Use when:** Working with dependency graphs, call graphs, file hierarchies, or any graph-based program analysis.

### 2. Hierarchies
**File:** [hierarchy.md](hierarchy.md)

- Ancestor-descendant relationships
- Depth/level calculations
- Subtree extraction
- Leaf node detection
- Sibling relationships
- Lowest Common Ancestor (LCA)
- Path to root

**Use when:** Modeling organizational structures, file systems, class inheritance, AST trees, or any tree-like data.

### 3. Temporal Logic
**File:** [temporal.md](temporal.md)

- Event ordering and sequencing
- Time range filtering
- Duration calculations
- Overlapping intervals
- Sliding windows
- Most recent events
- Gap detection

**Use when:** Working with timestamps, event logs, session tracking, or time-based analysis.

### 4. Filtering
**File:** [filtering.md](filtering.md)

- Range checks
- Multi-attribute filtering (AND)
- Multi-predicate filtering (OR)
- Exclusion filtering (NOT)
- Pattern matching on atoms
- Conditional filtering
- Threshold filtering with aggregation

**Use when:** Selecting subsets of data, validating inputs, or applying complex selection criteria.

### 5. Joining
**File:** [joining.md](joining.md)

- Inner joins
- Multi-way joins (3+ predicates)
- Left joins (outer joins)
- Self-joins
- Join with aggregation
- Conditional joins
- Star joins
- Anti-joins

**Use when:** Combining data from multiple predicates, relational queries, or cross-referencing facts.

### 6. Exclusion (Safe Negation)
**File:** [exclusion.md](exclusion.md)

- Set difference (A - B)
- Missing items / gap detection
- Orphaned records
- Never-occurred events
- Exclusive OR (XOR)
- Stratified negation
- Negation with aggregation

**Use when:** Finding what's missing, detecting gaps, or implementing "not in" logic safely.

### 7. Counting
**File:** [counting.md](counting.md)

- Basic count
- Count distinct
- Conditional count
- Count with zero (include empty groups)
- Running totals
- Percentage/ratio calculations
- Threshold filtering (HAVING)
- Multi-level count

**Use when:** Calculating metrics, statistics, or thresholds.

### 8. Grouping
**File:** [grouping.md](grouping.md)

- Basic grouping
- Multi-attribute grouping
- Multiple aggregations per group
- Conditional aggregation
- Hierarchical grouping (rollup)
- Grouping with filtering (HAVING)
- Distinct values per group
- Window functions

**Use when:** Aggregating data, calculating group statistics, or rollup reports.

### 9. Existence Checks
**File:** [existence.md](existence.md)

- Basic existence check
- Conditional existence
- Find first match
- All items have property (universal quantification)
- At least N items exist
- Unique existence (exactly one)
- Missing relationship detection
- Any vs. All

**Use when:** Validating data presence, checking invariants, or finding unique items.

### 10. Set Operations
**File:** [set_operations.md](set_operations.md)

- Union (A ∪ B)
- Intersection (A ∩ B)
- Difference (A - B)
- Symmetric difference (A Δ B)
- Subset/superset checks
- Disjoint sets
- Set equality
- Cardinality comparison

**Use when:** Combining or comparing collections, set algebra, or Venn diagram operations.

### 11. Default Values
**File:** [default_values.md](default_values.md)

- Basic defaults
- Cascading defaults (coalesce)
- Numeric defaults (zero-fill)
- Inherited defaults (hierarchy)
- Conditional defaults
- Range defaults (fill missing numbers)
- Interpolation (forward fill)
- Aggregate-based defaults

**Use when:** Handling missing data, NULL coalescing, or providing fallback values.

### 12. Conditional Logic
**File:** [conditional_logic.md](conditional_logic.md)

- Basic if-then-else
- Multiple conditions (if-elif-else)
- Guard clauses
- Nested conditions
- Case/switch statements
- Boolean expressions (AND, OR, NOT)
- Range conditions
- State machines

**Use when:** Implementing decision logic, branching behavior, or state transitions.

## Pattern Structure

Each pattern file follows this structure:

1. **Problem Description** - What problem does this solve?
2. **Core Pattern** - The fundamental template
3. **Complete Working Example** - Full schema + facts + rules + results
4. **Variations** (10+) - Alternative approaches and extensions
5. **Anti-Patterns** - Common mistakes and how to fix them
6. **Performance Tips** - Optimization guidance
7. **Common Use Cases in codeNERD** - Real-world applications

## How to Use These Patterns

### 1. Problem-First Approach
Start with your problem, find the matching pattern category, then adapt the template.

### 2. Template Adaptation
```mangle
# 1. Copy the template from the pattern file
# 2. Replace placeholder predicates with your schema
# 3. Adjust types and constraints
# 4. Test with sample data
```

### 3. Composition
Combine multiple patterns for complex queries:

```mangle
# Example: Counting + Filtering + Grouping
# Count high-value orders per customer

# Step 1: Filter (filtering.md)
high_value_order(OrderId, CustomerId, Amount) :-
  order(OrderId, CustomerId, Amount),
  Amount > 1000.

# Step 2: Group and Count (counting.md + grouping.md)
customer_high_value_count(CustomerId, Count) :-
  high_value_order(OrderId, CustomerId, Amount)
  |> do fn:group_by(CustomerId),
     let Count = fn:Count().

# Step 3: Filter on count (existence.md)
vip_customer(CustomerId) :-
  customer_high_value_count(CustomerId, Count),
  Count >= 5.
```

## Quick Reference by Use Case

### codeNERD-Specific Patterns

| Task | Pattern(s) | Example |
|------|------------|---------|
| File dependency analysis | Graph Traversal | Transitive imports, circular dependencies |
| Test coverage gaps | Exclusion + Set Operations | Untested files, missing test cases |
| Shard execution stats | Counting + Grouping | Success rate per shard, avg duration |
| Code complexity metrics | Grouping + Filtering | High-complexity functions per module |
| Symbol resolution | Joining + Existence | Definition-usage matching |
| Error pattern detection | Temporal + Counting | Recurring errors, error spikes |
| Configuration precedence | Default Values | User > Project > System defaults |
| Access control | Conditional Logic | Permission checks, role-based access |

### General Programming Patterns

| Task | Pattern(s) | Example |
|------|------------|---------|
| Finding duplicates | Counting + Filtering | Items with count > 1 |
| Top N items | Grouping + Existence | Highest sales per region |
| Missing sequence numbers | Exclusion | Gaps in order IDs |
| Parent-child queries | Hierarchy | All descendants of node X |
| Session analysis | Temporal + Grouping | User activity by time window |
| Data validation | Existence + Conditional | All required fields present |
| Merging datasets | Set Operations | Union of A and B |
| Null handling | Default Values | Coalesce multiple sources |

## Safety Guidelines

### 1. Negation Safety
Always ground variables before negation:
```mangle
# ✓ SAFE
result(X) :- candidate(X), not excluded(X).

# ✗ UNSAFE
result(X) :- not excluded(X).  # X is unbound!
```

### 2. Stratification
No recursion through negation:
```mangle
# ✗ WRONG - cycle through negation
p(X) :- q(X), not p(Y), related(X, Y).

# ✓ CORRECT - break into strata
temp(X) :- q(X).
p(X) :- temp(X), not excluded(X).
```

### 3. Type Safety
Match types from schema declarations:
```mangle
# Schema: Decl value(X.Type<int>)

# ✓ CORRECT
result(X) :- value(X), X > 100.  # int > int

# ✗ WRONG
result(X) :- value(X), X > "100".  # int > string
```

## Performance Best Practices

1. **Filter Early**: Most restrictive predicates first
2. **Avoid Cartesian Products**: Always join on shared variables
3. **Materialize Intermediate Results**: Store derived facts if reused
4. **Index Join Keys**: Ensure foreign keys are indexed
5. **Use Selectivity**: Small predicates before large ones
6. **Stratify Complex Queries**: Break into stages
7. **Limit Recursion Depth**: Use bounded traversal for deep graphs

## Contributing New Patterns

When adding patterns to this library:

1. **Problem Description** - Clear statement of the problem
2. **Template** - Generic, reusable solution
3. **Complete Example** - Schema + facts + rules + expected results
4. **3+ Variations** - Alternative approaches
5. **Anti-Patterns** - Common mistakes
6. **Performance Tips** - Optimization guidance
7. **Use Cases** - Real-world applications

## References

- [Mangle Language Reference](../../mangle-programming/references/)
- [Core Syntax](../syntax_core.md)
- [Type System](../type_system.md)
- [Aggregation Syntax](../aggregation_syntax.md)
- [Negation Safety](../negation_safety.md)
- [Stratification](../stratification.md)

## Version History

- **v1.0** (2024-12) - Initial pattern library with 12 core categories
  - 120+ pattern variations
  - codeNERD-specific use cases
  - Performance optimization guidance
