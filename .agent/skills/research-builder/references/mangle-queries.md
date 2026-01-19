# Mangle Query Patterns and Causal Reasoning

Comprehensive guide to Google Mangle deductive database for BrowserNERD.

## Overview

Mangle is a Datalog-based deductive database that:
- Stores facts (predicates with arguments)
- Defines rules (logical implications)
- Evaluates queries (pattern matching with variables)
- Derives new facts through logical inference

## Schema Definition

### Predicate Declarations

```mangle
# Declare predicate signatures
Decl react_component(FiberId, ComponentName, ParentFiberId).
Decl dom_node(NodeId, Tag, Text, ParentId).
Decl net_request(RequestId, Method, Url, Initiator, Timestamp).
Decl caused_by(Effect, Cause).

# Predicates are typed by arity (number of arguments)
# All arguments are typed dynamically (string, number, or symbol)
```

### Type System

```mangle
# Strings
Decl person(string).
person("Alice").

# Numbers (int64)
Decl age(string, number).
age("Alice", 30).

# Floats (explicit)
Decl temperature(number).
temperature(fn:float64(98.6)).

# Symbols (atoms)
Decl status(/atom).
status(/ready).
```

## Fact Insertion

### From Go Code

```go
import (
    "github.com/google/mangle/ast"
    "github.com/google/mangle/engine"
    "github.com/google/mangle/factstore"
)

// Create fact store
store := factstore.NewSimpleInMemoryStore()

// Build atoms (terms)
requestId := ast.String("req_123")
method := ast.String("GET")
url := ast.String("https://api.example.com")
initiator := ast.String("script")
timestamp := ast.Number(int64(1700000000))

// Create predicate
predSym := ast.PredicateSym{Symbol: "net_request", Arity: 5}
clause := ast.NewClause(ast.NewAtom(predSym,
    []ast.BaseTerm{requestId, method, url, initiator, timestamp},
))

// Add to store
store.Add(clause)
```

### Type Conversion Helpers

```go
func toMangleTerm(val interface{}) ast.BaseTerm {
    switch v := val.(type) {
    case string:
        return ast.String(v)
    case int:
        return ast.Number(int64(v))
    case int64:
        return ast.Number(v)
    case float64:
        return ast.Float64(v)
    case bool:
        if v {
            return ast.String("true")
        }
        return ast.String("false")
    default:
        return ast.String(fmt.Sprintf("%v", v))
    }
}

// Usage
args := []interface{}{"req_123", "GET", "https://api.example.com", "script", 1700000000}
terms := make([]ast.BaseTerm, len(args))
for i, arg := range args {
    terms[i] = toMangleTerm(arg)
}
```

## Query Patterns

### Simple Queries

```mangle
# Find all GET requests
?- net_request(ReqId, "GET", Url, _, _).

# Result: bindings of ReqId and Url variables
# Example: {ReqId: "req_123", Url: "https://api.example.com"}
```

### Variable Bindings

```mangle
# Variables start with uppercase
?- net_request(ReqId, Method, Url, Initiator, Timestamp).

# Constants are lowercase or quoted strings
?- net_request("req_123", "GET", Url, _, _).

# Anonymous variable (don't care)
?- net_request(_, "POST", _, _, _).
```

### Conjunctive Queries

```mangle
# Find requests with responses
?- net_request(ReqId, _, Url, _, _),
   net_response(ReqId, Status, _, _).

# All conditions must be satisfied
# Variables unify across clauses
```

### Filtering

```mangle
# Numeric comparisons
?- net_response(ReqId, Status, _, Duration),
   Status >= 400,
   Duration > 1000.

# Built-in predicates
?- net_request(ReqId, _, Url, _, _),
   fn:startsWith(Url, "https://api.").
```

### Aggregation

```mangle
# Count requests
?- fn:count[ReqId](net_request(ReqId, _, _, _, _), Count).

# Sum durations
?- fn:sum[Duration](net_response(_, _, _, Duration), Total).

# Group by
?- fn:group_by[Method](
     net_request(ReqId, Method, _, _, _),
     fn:count[ReqId],
     MethodCount
   ).
```

## Rule Definition

### Basic Rules

```mangle
# Rule: HTTP errors are failures
failed_request(ReqId) :-
    net_response(ReqId, Status, _, _),
    Status >= 400.

# Now you can query:
?- failed_request(ReqId).
```

### Multiple Conditions

```mangle
# Rule: Slow failed requests
slow_failure(ReqId, Url, Duration) :-
    net_request(ReqId, _, Url, _, _),
    net_response(ReqId, Status, _, Duration),
    Status >= 400,
    Duration > 1000.
```

### Recursion

```mangle
# Rule: Transitive closure (all ancestors)
ancestor(Child, Ancestor) :-
    react_component(Child, _, Ancestor).

ancestor(Child, Ancestor) :-
    react_component(Child, _, Parent),
    ancestor(Parent, Ancestor).

# Query all ancestors of component
?- ancestor("fiber_10", Ancestor).
```

### Negation

```mangle
# Rule: Requests without responses
orphan_request(ReqId) :-
    net_request(ReqId, _, _, _, _),
    !net_response(ReqId, _, _, _).

# ! is negation-as-failure
```

## Causal Reasoning Patterns

### Temporal Causality

```mangle
# Effect happens shortly after cause
caused_by(Effect, Cause) :-
    error_event(Effect, TError),
    trigger_event(Cause, TTrigger),
    TTrigger < TError,
    fn:minus(TError, TTrigger) < 100.  # Within 100ms
```

**BrowserNERD Implementation**:
```mangle
caused_by(ConsoleErr, ReqId) :-
    console_event("error", ConsoleErr, TError),
    net_response(ReqId, Status, _, _),
    net_request(ReqId, _, _, _, TNet),
    Status >= 400,
    TNet < TError,
    fn:minus(TError, TNet) < 100.
```

### Failure Propagation

```mangle
# Cascading failures through dependency chain
cascading_failure(Child, Parent) :-
    net_request(Child, _, _, Parent, _),
    net_response(Child, ChildStatus, _, _),
    net_response(Parent, ParentStatus, _, _),
    ChildStatus >= 400,
    ParentStatus >= 400.

# Transitive cascades
cascading_chain(Child, Root) :-
    cascading_failure(Child, Root).

cascading_chain(Child, Root) :-
    cascading_failure(Child, Intermediate),
    cascading_chain(Intermediate, Root).
```

### Performance Anomalies

```mangle
# Slow API detection
slow_api(ReqId, Url, Duration) :-
    net_request(ReqId, _, Url, _, _),
    net_response(ReqId, _, _, Duration),
    Duration > 1000.

# Abnormally slow (compared to average)
abnormally_slow(ReqId, Url, Duration) :-
    net_request(ReqId, _, Url, _, _),
    net_response(ReqId, _, _, Duration),
    fn:avg[D](net_response(_, _, _, D), AvgDuration),
    Duration > fn:mult(AvgDuration, 2.0).
```

### State Race Conditions

```mangle
# Detect operations before ready
race_condition_detected() :-
    click_event(BtnId, TClick),
    dom_attr(BtnId, "id", "submit-btn"),
    state_change("isReady", "true", TReady),
    TClick < TReady.

# Detect competing writes
write_conflict(Key, Time1, Time2) :-
    state_change(Key, Val1, Time1),
    state_change(Key, Val2, Time2),
    Val1 != Val2,
    fn:minus(Time2, Time1) < 10.  # Within 10ms
```

### Declarative Testing

```mangle
# Test passes if conditions met
test_passed() :-
    navigation_event(_, "/dashboard", _),
    dom_text(_, "Welcome User").

# Test fails if error occurred
test_failed() :-
    console_event("error", _, _).

test_failed() :-
    net_response(_, Status, _, _),
    Status >= 500.

# Comprehensive test
full_test_result(Pass, Fail) :-
    fn:count[_](test_passed(), Pass),
    fn:count[_](test_failed(), Fail).
```

## Querying from Go

### Execute Query

```go
import (
    "github.com/google/mangle/analysis"
    "github.com/google/mangle/engine"
    "github.com/google/mangle/parse"
)

// Parse query
queryStr := `console_event("error", Message, Timestamp)`
unit, err := parse.Unit(strings.NewReader("query Q() " + queryStr + "."))
clause := unit.Clauses[0]

// Execute query
results, err := engine.EvalQuery(ctx, store, programInfo, clause.Head)

// Process results
for _, result := range results {
    // result is map[string]ast.BaseTerm
    message := result["Message"].(ast.String).Symbol
    timestamp := result["Timestamp"].(ast.Number).Value

    fmt.Printf("Error: %s at %d\n", message, timestamp)
}
```

### Query with Filtering

```go
// Query: failed_request(ReqId, Status) :- net_response(ReqId, Status, _, _), Status >= 400
queryStr := `net_response(ReqId, Status, _, _), Status >= 400`

// Parse and execute
// ...

for _, result := range results {
    reqId := result["ReqId"].(ast.String).Symbol
    status := result["Status"].(ast.Number).Value
    fmt.Printf("Failed: %s (status %d)\n", reqId, status)
}
```

### Temporal Queries

```go
// Query facts within time window
func QueryRecent(store factstore.FactStore, predicate string, window time.Duration) ([]ast.Clause, error) {
    cutoff := time.Now().Add(-window)

    // Filter fact buffer
    var recent []ast.Clause
    for _, fact := range facts {
        if fact.Timestamp.After(cutoff) && fact.Predicate == predicate {
            recent = append(recent, toClause(fact))
        }
    }

    return recent, nil
}
```

## Built-in Functions

### Arithmetic

```mangle
# Addition, subtraction, multiplication, division
fn:plus(10, 5)    # 15
fn:minus(10, 5)   # 5
fn:mult(10, 5)    # 50
fn:div(10, 5)     # 2

# Modulo
fn:mod(10, 3)     # 1
```

### String Operations

```mangle
# String concatenation
fn:concat("Hello", " ", "World")  # "Hello World"

# Substring check
fn:contains("hello world", "world")  # true

# Prefix/suffix
fn:startsWith("https://api.example.com", "https://")  # true
fn:endsWith("file.txt", ".txt")                       # true

# Length
fn:length("hello")  # 5
```

### Comparison

```mangle
# Numeric comparison
X > Y
X >= Y
X < Y
X <= Y
X = Y   # Equality
X != Y  # Inequality
```

### Aggregation

```mangle
# Count
fn:count[Var](Predicate, Count)

# Sum
fn:sum[Var](Predicate, Total)

# Average
fn:avg[Var](Predicate, Average)

# Min/Max
fn:min[Var](Predicate, Minimum)
fn:max[Var](Predicate, Maximum)
```

## Performance Considerations

### Indexing

Mangle automatically indexes facts by predicate for O(m) lookup instead of O(n):

```go
// Index predicates for fast lookup
index := make(map[string][]int)
for i, fact := range facts {
    index[fact.Predicate] = append(index[fact.Predicate], i)
}

// Query specific predicate
indices := index["net_request"]
for _, i := range indices {
    // Process facts[i]
}
```

### Stratification

Rules must be stratified (no cyclic negation):

```mangle
# VALID: Stratified (base facts, then derived)
failed(ReqId) :- net_response(ReqId, Status, _, _), Status >= 400.
critical(ReqId) :- failed(ReqId), net_request(ReqId, _, "/api/critical", _, _).

# INVALID: Cyclic negation
bad(X) :- good(X), !bad(X).
```

### Fact Buffer Management

Limit fact buffer size to prevent memory exhaustion:

```go
const maxFacts = 10000

func addFacts(newFacts []Fact) {
    if len(facts) + len(newFacts) > maxFacts {
        // Remove oldest facts (circular buffer)
        removeCount := len(facts) + len(newFacts) - maxFacts
        facts = facts[removeCount:]
    }
    facts = append(facts, newFacts...)
}
```

## BrowserNERD Integration Examples

### Network Analysis

```mangle
# Slow endpoints
slow_endpoint(Url, AvgDuration) :-
    fn:group_by[Url](
        net_request(_, _, Url, _, _),
        fn:avg[D](net_response(ReqId, _, _, D)),
        AvgDuration
    ),
    AvgDuration > 500.

# Failed by initiator
failures_by_initiator(Initiator, Count) :-
    fn:group_by[Initiator](
        net_request(ReqId, _, _, Initiator, _),
        fn:count[ReqId](net_response(ReqId, Status, _, _), Status >= 400),
        Count
    ).
```

### React Debugging

```mangle
# Find component by prop value
component_with_prop(FiberId, ComponentName) :-
    react_component(FiberId, ComponentName, _),
    react_prop(FiberId, "userId", "12345").

# Components without props
empty_component(FiberId, ComponentName) :-
    react_component(FiberId, ComponentName, _),
    !react_prop(FiberId, _, _).

# Deep component tree depth
component_depth(FiberId, Depth) :-
    react_component(FiberId, _, ParentId),
    ParentId = /null,
    Depth = 0.

component_depth(FiberId, Depth) :-
    react_component(FiberId, _, ParentId),
    ParentId != /null,
    component_depth(ParentId, ParentDepth),
    Depth = fn:plus(ParentDepth, 1).
```

### RCA Workflows

```mangle
# Full diagnosis: find root cause of error
diagnose_error(ErrorMsg, RootCause) :-
    console_event("error", ErrorMsg, TError),
    caused_by(ErrorMsg, ReqId),
    net_request(ReqId, _, RootCause, _, _).

# Alternative: cascading failure chain
diagnose_error(ErrorMsg, RootCause) :-
    console_event("error", ErrorMsg, _),
    caused_by(ErrorMsg, ChildReq),
    cascading_chain(ChildReq, RootReq),
    net_request(RootReq, _, RootCause, _, _).
```
