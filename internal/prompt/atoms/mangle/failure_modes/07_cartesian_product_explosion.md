# Failure Mode 7: Cartesian Product Explosion

## Category
Performance Anti-Pattern (Query Optimization)

## Severity
MEDIUM - Severe performance degradation, potential out-of-memory

## Error Pattern
Poor predicate ordering that creates massive intermediate result sets. Placing large, unfiltered predicates before selective ones causes Mangle to compute huge Cartesian products.

## Wrong Code
```mangle
# WRONG - 10K users × 10K users = 100M pairs checked
slow(X, Y) :-
    user(X),           # 10,000 users
    user(Y),           # 10,000 users
    friends(X, Y).     # Only 1,000 actual friendships
# Generates 100M pairs, then filters to 1K!

# WRONG - Unfiltered join before constraint
inefficient(X, Y, Z) :-
    all_nodes(X),      # 100K nodes
    all_nodes(Y),      # 100K nodes
    all_nodes(Z),      # 100K nodes
    edge(X, Y),        # 500 edges
    edge(Y, Z).        # 500 edges
# Generates 100K³ = 1 TRILLION tuples before filtering!

# WRONG - Large table before specific filter
bad_query(Name) :-
    employee(ID, Name, Dept, Salary),  # 1M employees
    ID = /emp_12345.                   # Looking for one specific employee
# Iterates all 1M employees to find one!

# WRONG - Unconstrained variables in large cross product
bad_analysis(X, Y, Category) :-
    file(X),           # 50K files
    function(Y),       # 100K functions
    category(Category), # 20 categories
    related(X, Y, Category).  # Only 10K actual relations
# 50K × 100K × 20 = 100 BILLION combinations before filtering!
```

## Correct Code
```mangle
# CORRECT - Start with most selective predicate
fast(X, Y) :-
    friends(X, Y),     # 1,000 friendships
    user(X),           # Verify existence
    user(Y).           # Verify existence
# Only processes 1K pairs!

# CORRECT - Filter early, join late
efficient(X, Y, Z) :-
    edge(X, Y),        # 500 edges (most selective)
    edge(Y, Z),        # 500 more (still selective)
    all_nodes(X),      # Validate after filtering
    all_nodes(Y),
    all_nodes(Z).

# CORRECT - Bind variables early
good_query(Name) :-
    ID = /emp_12345,                   # Bind constant first
    employee(ID, Name, Dept, Salary).  # Direct lookup
# Processes only 1 row!

# CORRECT - Selective predicates first
good_analysis(X, Y, Category) :-
    related(X, Y, Category),  # 10K relations (most selective)
    file(X),                  # Validate
    function(Y),              # Validate
    category(Category).       # Validate
# Only 10K tuples processed!

# CORRECT - Use equality constraints early
optimized(X, Y) :-
    X = /specific_id,    # Bind X immediately
    edge(X, Y),          # Now selective
    node_info(Y, Data).  # Process only edges from specific_id
```

## Detection
- **Symptom**: Query runs for minutes/hours on small datasets
- **Symptom**: Memory usage spikes dramatically
- **Symptom**: Rules with many unbound variables early in clause
- **Pattern**: Large predicates (many facts) appear before filters
- **Test**: Check predicate sizes: `?predicate(X) |> let N = fn:Count()` for each
- **Profile**: Trace intermediate result sizes during evaluation

## Prevention

### Optimization Rules (In Priority Order)

1. **Filter first, join later**
   - Most selective predicates earliest
   - Equality constraints before joins

2. **Bind variables immediately**
   - Constants/filters before generators
   - Specific before general

3. **Selectivity ordering**
   - Small tables before large tables
   - Rare conditions before common conditions

4. **Index-friendly patterns**
   - Bound arguments before free arguments
   - Use equality over inequality when possible

### Selectivity Analysis Template

Before writing a rule, estimate selectivity:

```mangle
# For each predicate in a rule, count facts:
# ?user(X) |> let N = fn:Count()        → 10,000
# ?friends(X, Y) |> let N = fn:Count()  → 1,000

# Order by selectivity (smallest first):
# 1. friends(X, Y)     → 1,000 facts
# 2. user(X)           → 10,000 facts
# 3. user(Y)           → 10,000 facts

# Write rule in this order:
optimized(X, Y) :- friends(X, Y), user(X), user(Y).
```

## Optimization Patterns Reference

### Pattern 1: Constant Binding First
```mangle
# BAD - 100K iterations
bad(Name) :-
    person(ID, Name, Age),
    ID = /person_123.

# GOOD - 1 iteration (direct lookup)
good(Name) :-
    ID = /person_123,
    person(ID, Name, Age).
```

### Pattern 2: Selective Joins
```mangle
# BAD - 1M × 1M = 1T combinations
bad(X, Y) :-
    employee(X),
    employee(Y),
    manager_of(X, Y).  # Only 5K actual manager relationships

# GOOD - 5K combinations
good(X, Y) :-
    manager_of(X, Y),  # Selective join first
    employee(X),
    employee(Y).
```

### Pattern 3: Range Filters Early
```mangle
# BAD - Filter after generating all pairs
bad(X, Y) :-
    number(X),  # 0..1000
    number(Y),  # 0..1000
    X < 10,
    Y < 10,
    X < Y.

# GOOD - Filter before pairing
good(X, Y) :-
    number(X),
    X < 10,      # Filter X first
    number(Y),
    Y < 10,      # Filter Y first
    X < Y.       # Then compare
# Even better: use bounded generator if available
```

### Pattern 4: Existence Checks Last
```mangle
# BAD - Validate before filtering
bad(X, Y, Z) :-
    valid_x(X),        # Check validity
    valid_y(Y),        # Check validity
    valid_z(Z),        # Check validity
    actual_relation(X, Y, Z).  # Most selective!

# GOOD - Filter first, validate after
good(X, Y, Z) :-
    actual_relation(X, Y, Z),  # Get actual relations
    valid_x(X),                # Then validate
    valid_y(Y),
    valid_z(Z).
```

## Complex Example: Dependency Analysis

```mangle
# Schema
Decl file(Path.Type<string>).               # 50,000 files
Decl function(Name.Type<n>).                # 100,000 functions
Decl imports(File.Type<string>, Lib.Type<n>). # 10,000 imports
Decl calls(Func.Type<n>, Target.Type<n>).   # 200,000 calls
Decl defined_in(Func.Type<n>, File.Type<string>). # 100,000 definitions

# WRONG - Cartesian product explosion
bad_analysis(File, Func, Lib) :-
    file(File),              # 50K
    function(Func),          # 100K
    imports(File, Lib),      # 10K
    defined_in(Func, File).  # 100K
# 50K × 100K × 10K × 100K = 5 × 10^18 intermediate tuples!

# CORRECT - Selective predicates first
good_analysis(File, Func, Lib) :-
    defined_in(Func, File),  # 100K (binds both Func and File)
    imports(File, Lib),      # 10K (File already bound, just checks)
    file(File),              # Validation (File already bound)
    function(Func).          # Validation (Func already bound)
# ~100K tuples processed total!

# EVEN BETTER - If searching for specific file
best_analysis(Func, Lib) :-
    File = "src/main.go",      # Bind constant first!
    defined_in(Func, File),    # Get functions in this file
    imports(File, Lib),        # Get imports for this file
    function(Func).            # Validate
# Processes only functions in one file!
```

## Cardinality Estimation Table

| Pattern | Cardinality | Example |
|---------|-------------|---------|
| Single constant | 1 | `X = /specific_id` |
| Primary key lookup | 1 | `user_by_id(/id, X)` |
| Foreign key join | 10-100 | `orders_by_user(UserID, Order)` |
| Category filter | 100-1000 | `status(X, /active)` |
| Many-to-many join | 1K-100K | `related(X, Y)` |
| Full table scan | 10K-1M+ | `all_records(X)` |
| Cross product | N × M | `table1(X), table2(Y)` (NO filter) |

### Rule of Thumb
Order predicates by cardinality: **1 → 10 → 100 → 1K → 10K → ...**

## Why Ordering Matters

### Mangle's Join Strategy
Mangle evaluates predicates **left-to-right**:

```mangle
# Left-to-right evaluation:
result(X, Y, Z) :- pred1(X), pred2(Y), pred3(Z), filter(X, Y, Z).
#                  ^^^^^^^^  ^^^^^^^^  ^^^^^^^^  ^^^^^^^^^^^^^^^^
#                     1         2         3            4
# Step 1: Generate all X from pred1
# Step 2: For each X, generate all Y from pred2  → X × Y pairs
# Step 3: For each (X,Y), generate all Z from pred3  → X × Y × Z tuples
# Step 4: Filter → discards most tuples!
```

**Key insight**: Early predicates create the search space. Large early predicates = huge search space!

### Comparison: Good vs Bad Ordering

```mangle
# BAD: 10M × 10M × 100 = 10 trillion intermediate tuples
bad(X, Y, Z) :-
    all_items(X),      # 10M items
    all_items(Y),      # 10M items
    category(Z),       # 100 categories
    related(X, Y, Z).  # Only 50K actual relations

# GOOD: 50K × 1 × 1 × 1 = 50K intermediate tuples
good(X, Y, Z) :-
    related(X, Y, Z),  # 50K relations (most selective!)
    all_items(X),      # Validate X (now bound)
    all_items(Y),      # Validate Y (now bound)
    category(Z).       # Validate Z (now bound)
```

**Performance difference**: 10 trillion vs 50K = **200 million times faster!**

## Training Bias Origins
| Language | Pattern | Leads to Wrong Mangle |
|----------|---------|----------------------|
| SQL | Query optimizer reorders | Assuming automatic optimization |
| Prolog | Backtracking short-circuits | Early failure prevents full product |
| Python | Lazy generators | Evaluation on demand prevents explosion |

## Quick Check
Before writing a rule:
1. **Count facts** for each predicate: `?pred(X) |> let N = fn:Count()`
2. **Order by selectivity**: Smallest count first
3. **Bind constants early**: Specific values before generators
4. **Filters before joins**: Reduce search space ASAP
5. **Validate after filtering**: Existence checks last

## Debugging Aid
```mangle
# To profile cardinality, create diagnostic rules:
Decl cardinality(Predicate.Type<n>, Count.Type<int>).

# Count facts for each predicate
cardinality(/users, N) :- user(X) |> let N = fn:Count().
cardinality(/friends, N) :- friends(X, Y) |> let N = fn:Count().
cardinality(/posts, N) :- post(X) |> let N = fn:Count().

# Query to see selectivity:
# ?cardinality(P, N)
# Results:
# /friends → 1,000
# /users → 10,000
# /posts → 100,000
# Order rules: friends, users, posts
```
