# Mangle External Predicates: Implementing Custom Logic in Go

## Overview

External predicates allow you to extend Mangle with custom Go logic that can be called from Datalog rules. This enables:

- Integration with external APIs
- Complex computations not expressible in Datalog
- Database queries
- File system operations
- Any Go functionality

## External Predicate Signature

```go
type ExternalPredicate func(query engine.Query, cb func(engine.Fact)) error
```

**Parameters**:
- `query engine.Query` - The query being evaluated (contains variable bindings)
- `cb func(engine.Fact)` - Callback to return facts (call once per result)

**Returns**:
- `error` - If the predicate fails or encounters an error

## Basic Example

### 1. Define the External Predicate Function

```go
// is_even(X) succeeds if X is an even number
func isEvenPredicate(query engine.Query, cb func(engine.Fact)) error {
    // Extract the argument from the query
    if len(query.Args) != 1 {
        return fmt.Errorf("is_even expects 1 argument, got %d", len(query.Args))
    }

    arg := query.Args[0]

    // Check if argument is bound to a number
    if constant, ok := arg.(ast.Constant); ok {
        if constant.Type == ast.NumberType {
            num := constant.NumValue

            // Check if even
            if num%2 == 0 {
                // Return the fact
                fact := ast.Atom{
                    Predicate: ast.PredicateSym{Symbol: "is_even", Arity: 1},
                    Args:      []ast.BaseTerm{constant},
                }
                cb(fact)
            }
            // If odd, don't call cb (predicate fails)
            return nil
        }
    }

    // If argument is unbound or wrong type, fail
    return nil
}
```

### 2. Register the External Predicate

```go
// During engine setup
externalPredicates := map[ast.PredicateSym]ExternalPredicate{
    {Symbol: "is_even", Arity: 1}: isEvenPredicate,
}

// Pass to engine (exact API may vary - check current Mangle version)
// This is conceptual - actual registration method depends on Mangle version
```

### 3. Use in Mangle Code

```mangle
Decl is_even(Number).

# Find even numbers
even_number(X) :- number(X), is_even(X).
```

---

## Pattern: Input-Only Predicate

External predicate that checks a condition:

```go
// file_exists(Path) succeeds if file exists
func fileExistsPredicate(query engine.Query, cb func(engine.Fact)) error {
    if len(query.Args) != 1 {
        return fmt.Errorf("file_exists expects 1 argument")
    }

    // Argument must be bound (input only)
    pathArg := query.Args[0]
    constant, ok := pathArg.(ast.Constant)
    if !ok {
        return fmt.Errorf("file_exists requires bound argument")
    }

    if constant.Type != ast.StringType {
        return fmt.Errorf("file_exists requires string argument")
    }

    path := constant.Symbol

    // Check file existence
    if _, err := os.Stat(path); err == nil {
        // File exists - return the fact
        fact := ast.Atom{
            Predicate: query.Predicate,
            Args:      []ast.BaseTerm{constant},
        }
        cb(fact)
    }
    // If file doesn't exist, don't call cb

    return nil
}
```

**Usage**:
```mangle
Decl file_exists(Path)
  descr [mode(+)].  # Input only

existing_config(Path) :-
    config_path(Path),
    file_exists(Path).
```

---

## Pattern: Generator Predicate

External predicate that generates multiple results:

```go
// list_files(Directory, File) generates all files in directory
func listFilesPredicate(query engine.Query, cb func(engine.Fact)) error {
    if len(query.Args) != 2 {
        return fmt.Errorf("list_files expects 2 arguments")
    }

    // First argument (directory) must be bound
    dirArg := query.Args[0]
    dirConst, ok := dirArg.(ast.Constant)
    if !ok || dirConst.Type != ast.StringType {
        return fmt.Errorf("list_files: directory must be bound string")
    }

    directory := dirConst.Symbol

    // Read directory
    entries, err := os.ReadDir(directory)
    if err != nil {
        return err  // Error reading directory
    }

    // Generate one fact per file
    for _, entry := range entries {
        if !entry.IsDir() {
            filename := ast.Constant{
                Type:   ast.StringType,
                Symbol: entry.Name(),
            }

            fact := ast.Atom{
                Predicate: query.Predicate,
                Args:      []ast.BaseTerm{dirConst, filename},
            }

            cb(fact)  // Call callback for each file
        }
    }

    return nil
}
```

**Usage**:
```mangle
Decl list_files(Directory, File)
  descr [mode(+, -)].  # Input, Output

all_config_files(File) :-
    list_files("/etc", File),
    :ends_with(File, ".conf").
```

---

## Pattern: Lookup Predicate

External predicate that looks up data from external source:

```go
// db_lookup(Table, Key, Value) looks up value in database
func dbLookupPredicate(db *sql.DB) ExternalPredicate {
    return func(query engine.Query, cb func(engine.Fact)) error {
        if len(query.Args) != 3 {
            return fmt.Errorf("db_lookup expects 3 arguments")
        }

        // Table and Key must be bound
        tableConst, ok1 := query.Args[0].(ast.Constant)
        keyConst, ok2 := query.Args[1].(ast.Constant)

        if !ok1 || !ok2 {
            return fmt.Errorf("db_lookup: table and key must be bound")
        }

        table := tableConst.Symbol
        key := keyConst.Symbol

        // Query database
        var value string
        err := db.QueryRow(
            "SELECT value FROM "+table+" WHERE key = ?",
            key,
        ).Scan(&value)

        if err == sql.ErrNoRows {
            // No result - predicate fails
            return nil
        }
        if err != nil {
            return err
        }

        // Return the fact
        valueConst := ast.Constant{
            Type:   ast.StringType,
            Symbol: value,
        }

        fact := ast.Atom{
            Predicate: query.Predicate,
            Args:      []ast.BaseTerm{tableConst, keyConst, valueConst},
        }
        cb(fact)

        return nil
    }
}

// Registration
externalPredicates[ast.PredicateSym{Symbol: "db_lookup", Arity: 3}] =
    dbLookupPredicate(database)
```

**Usage**:
```mangle
Decl db_lookup(Table, Key, Value)
  descr [mode(+, +, -)].  # Both inputs, one output

user_email(User, Email) :-
    db_lookup(/users, User, Email).
```

---

## Pattern: Computation Predicate

External predicate for complex computation:

```go
// sha256_hash(Input, Hash) computes SHA256 hash
func sha256Predicate(query engine.Query, cb func(engine.Fact)) error {
    if len(query.Args) != 2 {
        return fmt.Errorf("sha256_hash expects 2 arguments")
    }

    // Input must be bound
    inputConst, ok := query.Args[0].(ast.Constant)
    if !ok || inputConst.Type != ast.StringType {
        return fmt.Errorf("sha256_hash: input must be bound string")
    }

    input := inputConst.Symbol

    // Compute hash
    hash := sha256.Sum256([]byte(input))
    hashStr := hex.EncodeToString(hash[:])

    // Return result
    hashConst := ast.Constant{
        Type:   ast.StringType,
        Symbol: hashStr,
    }

    fact := ast.Atom{
        Predicate: query.Predicate,
        Args:      []ast.BaseTerm{inputConst, hashConst},
    }
    cb(fact)

    return nil
}
```

**Usage**:
```mangle
Decl sha256_hash(Input, Hash)
  descr [mode(+, -)].

password_hash(User, Hash) :-
    password(User, Pass),
    sha256_hash(Pass, Hash).
```

---

## Handling Variables

### Bound Variables

```go
// Check if argument is bound
if constant, ok := arg.(ast.Constant); ok {
    // Argument is a constant (bound)
    value := constant.Symbol
} else {
    // Argument is a variable (unbound)
    return fmt.Errorf("argument must be bound")
}
```

### Unbound Variables

If your external predicate can generate values for unbound variables:

```go
// enumerate_colors(Color) generates all colors
func enumerateColorsPredicate(query engine.Query, cb func(engine.Fact)) error {
    colors := []string{"red", "green", "blue", "yellow"}

    for _, color := range colors {
        colorConst := ast.Constant{
            Type:   ast.NameType,
            Symbol: color,
        }

        fact := ast.Atom{
            Predicate: query.Predicate,
            Args:      []ast.BaseTerm{colorConst},
        }
        cb(fact)
    }

    return nil
}
```

**Usage**:
```mangle
# Generate all colors
all_colors(C) :- enumerate_colors(C).
```

---

## Error Handling

### Return Errors for Invalid Input

```go
if len(query.Args) != expectedArity {
    return fmt.Errorf("predicate expects %d arguments, got %d",
        expectedArity, len(query.Args))
}

if wrongType {
    return fmt.Errorf("argument must be of type X")
}
```

### Return Errors for External Failures

```go
data, err := externalAPI.Fetch()
if err != nil {
    return fmt.Errorf("external API error: %w", err)
}
```

### Silent Failure (No Results)

If the predicate should simply fail (no results), don't call `cb` and return `nil`:

```go
if !conditionMet {
    // Predicate fails - no results
    return nil
}
```

---

## Type Conversion Helpers

### Extract Number

```go
func getNumber(arg ast.BaseTerm) (int64, error) {
    constant, ok := arg.(ast.Constant)
    if !ok {
        return 0, fmt.Errorf("argument is not a constant")
    }
    if constant.Type != ast.NumberType {
        return 0, fmt.Errorf("argument is not a number")
    }
    return constant.NumValue, nil
}
```

### Extract String

```go
func getString(arg ast.BaseTerm) (string, error) {
    constant, ok := arg.(ast.Constant)
    if !ok {
        return "", fmt.Errorf("argument is not a constant")
    }
    if constant.Type != ast.StringType {
        return "", fmt.Errorf("argument is not a string")
    }
    return constant.Symbol, nil
}
```

### Extract Name (Atom)

```go
func getName(arg ast.BaseTerm) (string, error) {
    constant, ok := arg.(ast.Constant)
    if !ok {
        return "", fmt.Errorf("argument is not a constant")
    }
    if constant.Type != ast.NameType {
        return "", fmt.Errorf("argument is not a name")
    }
    return constant.Symbol, nil
}
```

---

## Best Practices

### 1. Validate Inputs

Always check:
- Argument count
- Argument types
- Bound vs unbound

### 2. Document Modes

Declare modes in the Mangle declaration:

```mangle
Decl external_pred(Input, Output)
  descr [mode(+, -)].
```

### 3. Handle Errors Gracefully

Return descriptive errors:
```go
return fmt.Errorf("predicate_name: expected string, got %T", arg)
```

### 4. Be Careful with Side Effects

External predicates are called during evaluation, which may:
- Retry due to semi-naive algorithm
- Be called multiple times
- Be called in unexpected orders

**Avoid**:
- Modifying global state
- Non-idempotent operations (unless that's the goal)

### 5. Use Closures for State

If you need configuration or connections:

```go
func makeDBPredicate(db *sql.DB) ExternalPredicate {
    return func(query engine.Query, cb func(engine.Fact)) error {
        // Use db here
    }
}
```

---

## Complete Example

```go
package main

import (
    "fmt"
    "github.com/google/mangle/ast"
    "github.com/google/mangle/engine"
)

// External predicate: is_prime(X)
func isPrimePredicate(query engine.Query, cb func(engine.Fact)) error {
    if len(query.Args) != 1 {
        return fmt.Errorf("is_prime expects 1 argument")
    }

    numConst, ok := query.Args[0].(ast.Constant)
    if !ok || numConst.Type != ast.NumberType {
        return fmt.Errorf("is_prime requires a number")
    }

    n := numConst.NumValue
    if n < 2 {
        return nil  // Not prime
    }

    // Check primality
    for i := int64(2); i*i <= n; i++ {
        if n%i == 0 {
            return nil  // Not prime
        }
    }

    // Is prime - return fact
    fact := ast.Atom{
        Predicate: query.Predicate,
        Args:      []ast.BaseTerm{numConst},
    }
    cb(fact)

    return nil
}

func main() {
    // Register external predicate
    externalPreds := map[ast.PredicateSym]engine.ExternalPredicate{
        {Symbol: "is_prime", Arity: 1}: isPrimePredicate,
    }

    // Use in Mangle program
    source := `
    Decl is_prime(Number).
    Decl number(Number).

    number(2).
    number(3).
    number(4).
    number(5).
    number(6).

    prime_number(X) :- number(X), is_prime(X).
    `

    // ... parse, analyze, evaluate with externalPreds ...
}
```

---

## Advanced Topics

### Caching Results

```go
type cachedPredicate struct {
    cache map[string]bool
    fn    ExternalPredicate
}

func (c *cachedPredicate) Invoke(query engine.Query, cb func(engine.Fact)) error {
    key := fmt.Sprintf("%v", query.Args)

    if result, ok := c.cache[key]; ok {
        if result {
            // Cached success - regenerate fact
            fact := ast.Atom{Predicate: query.Predicate, Args: query.Args}
            cb(fact)
        }
        return nil
    }

    // Not cached - call underlying predicate
    called := false
    wrappedCb := func(fact engine.Fact) {
        called = true
        cb(fact)
    }

    err := c.fn(query, wrappedCb)
    c.cache[key] = called

    return err
}
```

### Rate Limiting

```go
func rateLimitedPredicate(pred ExternalPredicate, limiter *rate.Limiter) ExternalPredicate {
    return func(query engine.Query, cb func(engine.Fact)) error {
        if err := limiter.Wait(context.Background()); err != nil {
            return err
        }
        return pred(query, cb)
    }
}
```

---

## Integration with Virtual Store

In codeNERD, external predicates are often wrapped in the **VirtualStore** pattern:

```go
// In your VirtualStore implementation
func (vs *VirtualStore) Resolve(predSym ast.PredicateSym) (ExternalPredicate, bool) {
    switch predSym.Symbol {
    case "file_exists":
        return vs.fileExistsPredicate, true
    case "db_lookup":
        return vs.dbLookupPredicate, true
    default:
        return nil, false
    }
}
```

This allows clean separation between Mangle logic and external systems.
