# Advanced Mangle Programming Atoms

This directory contains comprehensive atoms covering advanced Mangle topics, edge cases, and deep dives into complex scenarios.

## When to Use Advanced Atoms

Use these atoms when you need:
- Deep understanding of Mangle semantics
- Solutions to edge cases and gotchas
- Performance optimization strategies
- Migration from other logic languages
- Debugging complex logic programs
- Understanding of stratification theory

## Atom Reference

### 1. performance.md
**Use when**: Optimizing slow Mangle programs, understanding query performance, or dealing with large datasets.

**Key Topics**:
- Selectivity ordering (most critical optimization)
- Constant binding strategies
- Join optimization
- Negation and recursion performance
- Aggregation optimization
- Index-friendly patterns
- Gas limits and computational budgets

**Example Scenarios**:
- Query takes too long to execute
- Memory usage is too high
- Need to understand why one query is faster than another
- Implementing production systems with performance SLAs

### 2. termination.md
**Use when**: Dealing with recursive predicates, infinite loops, or proving program termination.

**Key Topics**:
- Well-founded ordering
- Structural recursion on lists
- Bounded depth recursion
- Non-terminating patterns (infinite generators)
- Termination analysis techniques
- Gas limits and timeouts
- Ranking functions

**Example Scenarios**:
- Program never completes
- Need to prove recursion terminates
- Implementing bounded search algorithms
- Protecting against runaway queries

### 3. stratification_deep.md
**Use when**: Understanding or debugging stratification errors, dealing with negation dependencies.

**Key Topics**:
- Complete stratification theory
- Dependency graph analysis
- Cyclic negation detection
- Stratification algorithm (Tarjan's)
- Fixing non-stratified programs
- Aggregation stratification constraints

**Example Scenarios**:
- "Not stratified" error from compiler
- Understanding why negation fails
- Designing complex rule hierarchies
- Migrating from well-founded semantics systems

### 4. negation_semantics.md
**Use when**: Working with negation-as-failure, understanding Closed World Assumption.

**Key Topics**:
- Closed World Assumption (CWA) vs Open World Assumption (OWA)
- Negation-as-failure semantics
- Safety rules for negation
- Set difference, absence checks
- Universal quantification via negation
- Performance considerations

**Example Scenarios**:
- Using `not` in rules
- Computing "all except" queries
- Integrity constraints and violations
- Understanding why negation produces unexpected results

### 5. aggregation_edge_cases.md
**Use when**: Working with aggregations, especially in corner cases.

**Key Topics**:
- Empty group handling
- Null handling (CWA semantics)
- Multiple aggregations in one rule
- Filtering before vs after aggregation
- Nested aggregations (multi-strata)
- Count vs count_distinct
- Aggregation safety rules

**Example Scenarios**:
- Aggregation returns no results (expected 0)
- Need to compute multiple aggregates efficiently
- Aggregating over filtered data
- Handling departments with no employees

### 6. list_operations.md
**Use when**: Working with lists, recursive list processing, or functional patterns.

**Key Topics**:
- List destructuring (`:match_cons`)
- List construction (`:cons`)
- Built-in list functions (`fn:length`, `fn:list:get`, etc.)
- Recursive list patterns (sum, map, filter, reverse)
- Nested lists and matrices
- Performance considerations (tail recursion)

**Example Scenarios**:
- Processing collections of data
- Implementing functional algorithms
- Building or transforming lists
- Pattern matching on list structure

### 7. struct_operations.md
**Use when**: Working with structured data, JSON-like objects, or nested data.

**Key Topics**:
- Struct field access (`:match_field`)
- Nested struct navigation
- Struct construction and updates (immutable)
- Validation and partial matching
- Struct vs multiple facts trade-offs
- Map-like operations

**Example Scenarios**:
- Parsing JSON data from external APIs
- Working with hierarchical configuration
- Modeling entities with many attributes
- Deciding between structs and normalized facts

### 8. external_predicates_advanced.md
**Use when**: Implementing Go-side external predicates, interfacing with external systems.

**Key Topics**:
- External predicate signatures
- Caching strategies
- Lazy evaluation
- Error handling patterns
- Side effects management
- Streaming large results
- Context-aware predicates
- Performance optimization

**Example Scenarios**:
- Fetching data from databases or APIs
- Implementing complex computations in Go
- Interfacing with file systems or networks
- Optimizing external calls with caching

### 9. debugging.md
**Use when**: Debugging Mangle programs that don't work as expected.

**Key Topics**:
- Common debugging scenarios
- Tracing and logging techniques
- Incremental rule building
- Negation debugging
- Stratification error diagnosis
- Variable binding checks
- Visualization and minimization

**Example Scenarios**:
- Rule doesn't fire (expected facts not derived)
- Too many or too few results
- Logic errors in complex rules
- Understanding execution flow
- Finding the source of incorrect results

### 10. migration.md
**Use when**: Migrating code from Soufflé, Prolog, or SQL to Mangle.

**Key Topics**:
- Soufflé → Mangle syntax translation
- Prolog → Mangle conversion
- SQL → Mangle equivalents
- Key differences summary
- Migration checklists
- Full migration examples

**Example Scenarios**:
- Converting existing Soufflé programs
- Porting Prolog knowledge bases
- Translating SQL queries to logic rules
- Understanding syntax differences

## Usage Patterns

### For Beginners
Start with the basic atoms in `../` directory. Come here when you encounter:
1. Performance issues → `performance.md`
2. Infinite loops → `termination.md`
3. Stratification errors → `stratification_deep.md`
4. Debugging needs → `debugging.md`

### For Intermediate Users
Deepen your understanding with:
1. `negation_semantics.md` - Master negation
2. `aggregation_edge_cases.md` - Handle corner cases
3. `list_operations.md` - Functional programming patterns
4. `struct_operations.md` - Complex data structures

### For Advanced Users
Optimize and extend with:
1. `performance.md` - Production optimization
2. `external_predicates_advanced.md` - Go integration
3. `migration.md` - Port existing systems

### For System Architects
Understand the theory:
1. `stratification_deep.md` - Complete stratification theory
2. `termination.md` - Formal termination proofs
3. `negation_semantics.md` - CWA semantics

## Quick Reference

| Problem | Recommended Atom |
|---------|-----------------|
| Query is slow | `performance.md` |
| Program never finishes | `termination.md` |
| "Not stratified" error | `stratification_deep.md` |
| Negation not working | `negation_semantics.md` |
| Aggregation returns nothing | `aggregation_edge_cases.md` |
| Working with lists | `list_operations.md` |
| Parsing JSON/structs | `struct_operations.md` |
| External Go predicates | `external_predicates_advanced.md` |
| Rule doesn't fire | `debugging.md` |
| Porting from SQL/Prolog | `migration.md` |

## Cross-References

These atoms reference concepts from the basic atoms directory:
- `../syntax.md` - Core syntax rules
- `../rules.md` - Basic rule structure
- `../functions.md` - Built-in functions
- `../types.md` - Type system

## Examples Index

### Performance Optimization
- Selectivity ordering: `performance.md` § 1
- Join optimization: `performance.md` § 3
- Avoiding Cartesian products: `performance.md` § Anti-patterns 2

### Recursion
- Well-founded recursion: `termination.md` § 1
- Transitive closure: `termination.md` § 1, Example 1
- List recursion: `list_operations.md` § Recursive Patterns

### Negation
- Set difference: `negation_semantics.md` § Pattern 1
- Universal quantification: `negation_semantics.md` § Pattern 3
- Integrity constraints: `negation_semantics.md` § Pattern 5

### Aggregation
- Empty groups: `aggregation_edge_cases.md` § Edge Case 1
- Multiple aggregates: `aggregation_edge_cases.md` § Edge Case 3
- Nested aggregation: `aggregation_edge_cases.md` § Edge Case 5

### Data Structures
- List operations: `list_operations.md` § Built-in Functions
- Struct field access: `struct_operations.md` § Accessing Fields
- Nested structures: `struct_operations.md` § Nested Struct Access

### External Integration
- Caching pattern: `external_predicates_advanced.md` § Pattern 1
- Error handling: `external_predicates_advanced.md` § Pattern 3
- Streaming: `external_predicates_advanced.md` § Pattern 5

### Debugging
- Tracing: `debugging.md` § Technique 1
- Incremental testing: `debugging.md` § Technique 2
- Common bugs: `debugging.md` § Common Bugs

### Migration
- From SQL: `migration.md` § From SQL
- From Prolog: `migration.md` § From Prolog
- From Soufflé: `migration.md` § From Soufflé

## Contributing

When adding new advanced atoms:
1. Cover edge cases comprehensively
2. Include multiple examples (good and bad)
3. Provide checklists for verification
4. Cross-reference related atoms
5. Update this README with the new atom

## File Size Summary

| File | Lines | Size | Coverage |
|------|-------|------|----------|
| performance.md | ~450 | 10KB | Query optimization, selectivity |
| termination.md | ~420 | 9KB | Recursion termination, gas limits |
| stratification_deep.md | ~450 | 10KB | Complete stratification theory |
| negation_semantics.md | ~440 | 10KB | CWA, negation-as-failure |
| aggregation_edge_cases.md | ~470 | 11KB | Empty groups, null handling |
| list_operations.md | ~500 | 12KB | All list functions, patterns |
| struct_operations.md | ~480 | 11KB | Field access, nested structures |
| external_predicates_advanced.md | ~600 | 14KB | Go integration, caching, errors |
| debugging.md | ~500 | 11KB | Tracing, common bugs, tools |
| migration.md | ~500 | 12KB | Soufflé/Prolog/SQL → Mangle |

**Total**: ~4,800 lines of comprehensive advanced documentation.
