# Calling Mangle from Go: Complete Integration Guide

## Overview

Mangle is a Go library that can be embedded in Go applications. This guide covers the complete integration workflow.

## Basic Imports

```go
import (
    "github.com/google/mangle/ast"
    "github.com/google/mangle/parse"
    "github.com/google/mangle/analysis"
    "github.com/google/mangle/engine"
    "github.com/google/mangle/functional"
    "github.com/google/mangle/builtin"
)
```

---

## Complete Evaluation Pipeline

### Step 1: Parse Source Code

```go
import "github.com/google/mangle/parse"

// Parse a single source unit
sourceCode := `
Decl parent(Person, Child).

parent(/alice, /bob).
parent(/bob, /charlie).

ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).
`

parsedUnit, err := parse.Unit(sourceCode)
if err != nil {
    log.Fatalf("Parse error: %v", err)
}
```

**parse.Unit** returns:
- `*ast.SourceUnit` - Parsed declarations, clauses, and facts
- `error` - Syntax errors if any

---

### Step 2: Analyze the Program

```go
import "github.com/google/mangle/analysis"

// Analyze for safety, arity, etc.
analyzedUnit, err := analysis.AnalyzeOneUnit(parsedUnit)
if err != nil {
    log.Fatalf("Analysis error: %v", err)
}
```

**analysis.AnalyzeOneUnit** checks:
- Variable safety (all head variables bound)
- Arity correctness
- Mode declarations
- Returns analyzed structure ready for evaluation

---

### Step 3: Check Stratification (if using negation)

```go
// Only needed if program uses negation
strata, predToStratum, err := analysis.Stratify(analyzedUnit)
if err != nil {
    log.Fatalf("Stratification error: %v", err)
}

fmt.Printf("Program has %d strata\n", len(strata))
```

**analysis.Stratify** returns:
- `[][]ast.PredicateSym` - List of strata (topologically sorted SCCs)
- `map[ast.PredicateSym]int` - Predicate to stratum mapping
- `error` - If program cannot be stratified (cycle through negation)

---

### Step 4: Prepare Fact Store

```go
import "github.com/google/mangle/engine"

// Create empty fact store
store := engine.NewStore()

// The parsed unit already contains facts
// Evaluation will use these initial facts
```

---

### Step 5: Evaluate the Program

```go
// Set evaluation options (optional)
opts := []engine.EvalOption{
    engine.WithCreatedFactLimit(1_000_000),  // Prevent runaway
}

// Evaluate without stratification (pure Datalog)
err = engine.EvalProgram(analyzedUnit, store, opts...)
if err != nil {
    log.Fatalf("Evaluation error: %v", err)
}

// OR: Evaluate with statistics
stats, err := engine.EvalProgramWithStats(analyzedUnit, store, opts...)
if err != nil {
    log.Fatalf("Evaluation error: %v", err)
}
fmt.Printf("Created %d facts in %d iterations\n",
    stats.FactsCreated, stats.Iterations)

// OR: Evaluate stratified program (if using negation)
stats, err = engine.EvalStratifiedProgramWithStats(analyzedUnit, store, strata, opts...)
if err != nil {
    log.Fatalf("Evaluation error: %v", err)
}
```

---

### Step 6: Query Results

```go
// Query all facts for a predicate
predSym := ast.PredicateSym{Symbol: "ancestor", Arity: 2}

facts := store.GetFacts(predSym)
for _, fact := range facts {
    // fact is of type engine.Fact (which is an ast.Atom)
    fmt.Printf("Fact: %s\n", fact.String())

    // Access arguments
    if len(fact.Args) == 2 {
        arg1 := fact.Args[0]  // engine.Value (wraps ast.BaseTerm)
        arg2 := fact.Args[1]
        fmt.Printf("  Ancestor: %s, Descendant: %s\n", arg1, arg2)
    }
}
```

---

## Working with Facts

### Creating Facts Programmatically

```go
// Create a fact: parent(/alice, /bob)
predSym := ast.PredicateSym{Symbol: "parent", Arity: 2}

// Create constant arguments
alice := ast.Constant{Type: ast.NameType, Symbol: "alice"}
bob := ast.Constant{Type: ast.NameType, Symbol: "bob"}

// Wrap in engine.Value
aliceVal := engine.NewValue(alice)
bobVal := engine.NewValue(bob)

// Create atom (fact)
fact := ast.Atom{
    Predicate: predSym,
    Args: []ast.BaseTerm{alice, bob},
}

// Add to store
store.Add(fact)
```

### Creating Different Constant Types

```go
// Name constant
nameConst := ast.Constant{Type: ast.NameType, Symbol: "alice"}

// String constant
stringConst := ast.Constant{Type: ast.StringType, Symbol: "Alice Smith"}

// Number constant
numConst := ast.Constant{Type: ast.NumberType, NumValue: 42}

// Float constant
floatConst := ast.Constant{Type: ast.Float64Type, FloatValue: 3.14}

// List constant
list := ast.Constant{
    Type: ast.ListShape,
    fst: &engine.Value{/* first element */},
    snd: &engine.Value{/* rest of list */},
}

// Map constant
mapConst := ast.Constant{
    Type: ast.MapShape,
    // ... constructed with key-value pairs
}
```

---

## Complete Example

```go
package main

import (
    "fmt"
    "log"

    "github.com/google/mangle/parse"
    "github.com/google/mangle/analysis"
    "github.com/google/mangle/engine"
)

func main() {
    // 1. Parse
    source := `
    Decl parent(Person, Child).

    parent(/alice, /bob).
    parent(/bob, /charlie).

    ancestor(X, Y) :- parent(X, Y).
    ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).
    `

    parsed, err := parse.Unit(source)
    if err != nil {
        log.Fatalf("Parse: %v", err)
    }

    // 2. Analyze
    analyzed, err := analysis.AnalyzeOneUnit(parsed)
    if err != nil {
        log.Fatalf("Analysis: %v", err)
    }

    // 3. Evaluate
    store := engine.NewStore()
    opts := []engine.EvalOption{
        engine.WithCreatedFactLimit(10_000),
    }

    stats, err := engine.EvalProgramWithStats(analyzed, store, opts...)
    if err != nil {
        log.Fatalf("Evaluation: %v", err)
    }

    fmt.Printf("Evaluation: %d facts in %d iterations\n",
        stats.FactsCreated, stats.Iterations)

    // 4. Query results
    ancestorSym := ast.PredicateSym{Symbol: "ancestor", Arity: 2}
    facts := store.GetFacts(ancestorSym)

    fmt.Println("\nAncestor facts:")
    for _, fact := range facts {
        fmt.Printf("  %s\n", fact.String())
    }
}
```

---

## Evaluation Options

### WithCreatedFactLimit

```go
// Limit maximum facts created
opts := []engine.EvalOption{
    engine.WithCreatedFactLimit(1_000_000),
}
```

This prevents:
- Infinite recursion
- Memory exhaustion
- Runaway computation

### Future Options

The `EvalOption` type is extensible. Future versions may add:
- `WithTimeout(duration)` - Maximum evaluation time
- `WithMemoryLimit(bytes)` - Maximum memory usage
- `WithStackDepth(depth)` - Maximum recursion depth

---

## Error Handling

### Parse Errors

```go
_, err := parse.Unit(source)
if err != nil {
    // Syntax error: line X, column Y: unexpected token
    // Detailed error message with position
}
```

### Analysis Errors

```go
_, err := analysis.AnalyzeOneUnit(parsed)
if err != nil {
    // Safety violation
    // Arity mismatch
    // Undeclared predicate
}
```

### Stratification Errors

```go
_, _, err := analysis.Stratify(analyzed)
if err != nil {
    // Cannot stratify: cycle through negation
    // Details about which predicates form the cycle
}
```

### Evaluation Errors

```go
err := engine.EvalProgram(analyzed, store, opts...)
if err != nil {
    // Fact limit exceeded
    // Type error (if using type checking)
    // Function application error
}
```

---

## Querying Patterns

### Get All Facts for Predicate

```go
predSym := ast.PredicateSym{Symbol: "person", Arity: 2}
facts := store.GetFacts(predSym)
```

### Check if Specific Fact Exists

```go
// Create the fact you're looking for
targetFact := ast.Atom{
    Predicate: ast.PredicateSym{Symbol: "parent", Arity: 2},
    Args: []ast.BaseTerm{
        ast.Constant{Type: ast.NameType, Symbol: "alice"},
        ast.Constant{Type: ast.NameType, Symbol: "bob"},
    },
}

// Query
allFacts := store.GetFacts(targetFact.Predicate)
found := false
for _, fact := range allFacts {
    if fact.Equals(targetFact) {
        found = true
        break
    }
}
```

### Pattern Matching Query

```go
// Get all parent facts, extract children of alice
parentSym := ast.PredicateSym{Symbol: "parent", Arity: 2}
facts := store.GetFacts(parentSym)

aliceConst := ast.Constant{Type: ast.NameType, Symbol: "alice"}

for _, fact := range facts {
    if len(fact.Args) == 2 {
        if fact.Args[0].Equals(aliceConst) {
            fmt.Printf("Alice's child: %s\n", fact.Args[1])
        }
    }
}
```

---

## Performance Tips

### 1. Use Fact Limits

Always set `WithCreatedFactLimit` in production:
```go
opts := []engine.EvalOption{
    engine.WithCreatedFactLimit(10_000_000),
}
```

### 2. Check Statistics

Monitor evaluation performance:
```go
stats, _ := engine.EvalProgramWithStats(analyzed, store, opts...)
fmt.Printf("Efficiency: %d facts / %d iterations = %.2f facts/iter\n",
    stats.FactsCreated, stats.Iterations,
    float64(stats.FactsCreated) / float64(stats.Iterations))
```

### 3. Pre-analyze Once

If evaluating multiple times with different facts:
```go
// Parse and analyze once
analyzed, _ := analysis.AnalyzeOneUnit(parsed)

// Evaluate multiple times with different initial facts
for _, initialFacts := range dataSets {
    store := engine.NewStore()
    // Add initial facts
    for _, fact := range initialFacts {
        store.Add(fact)
    }
    // Evaluate
    engine.EvalProgram(analyzed, store, opts...)
}
```

### 4. Reuse Stores Carefully

`engine.NewStore()` creates fresh state. Don't reuse stores unless you want cumulative facts.

---

## Type Checking Integration

### Create Type Checker

```go
import "github.com/google/mangle/builtin"

// Extract declarations from analyzed unit
decls := make(map[ast.PredicateSym]ast.Decl)
// ... populate from analyzed.Declarations

// Create type checker
typeChecker, err := builtin.NewTypeChecker(decls)
if err != nil {
    log.Fatalf("Type checker error: %v", err)
}
```

### Check Facts

```go
// Check a fact against type bounds
err := typeChecker.CheckTypeBounds(fact)
if err != nil {
    log.Printf("Type error: %v", err)
    // Fact violates declared type bounds
}
```

---

## Next Steps

- See `external_predicates.md` for implementing custom predicates in Go
- See `ast_types.md` for detailed AST structure
- See `fact_store.md` for advanced store operations
