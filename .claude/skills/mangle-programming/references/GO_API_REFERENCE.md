# Mangle Go API Reference

**Version**: Mangle v0.4.0 (November 2024)
**Repository**: https://github.com/google/mangle

Comprehensive reference for integrating Mangle into Go applications.

---

## Package Overview

| Package | Import Path | Purpose |
|---------|-------------|---------|
| `ast` | `github.com/google/mangle/ast` | AST types (Atom, Constant, Variable) |
| `parse` | `github.com/google/mangle/parse` | Parsing Mangle source code |
| `analysis` | `github.com/google/mangle/analysis` | Program analysis and validation |
| `engine` | `github.com/google/mangle/engine` | Semi-naive evaluation engine |
| `factstore` | `github.com/google/mangle/factstore` | Fact storage implementations |
| `builtin` | `github.com/google/mangle/builtin` | Built-in predicates and functions |
| `symbols` | `github.com/google/mangle/symbols` | Built-in symbols registry |
| `unionfind` | `github.com/google/mangle/unionfind` | Unification implementation |
| `json2struct` | `github.com/google/mangle/json2struct` | JSON to Mangle struct conversion |
| `proto2struct` | `github.com/google/mangle/proto2struct` | Protobuf to Mangle struct conversion |

---

## AST Package (`ast`)

The core types for representing Mangle programs.

### Constants (Type Bounds)

```go
// Type expressions for built-in types
var AnyBound     // Universal type (contains all values)
var BotBound     // Empty type (no elements)
var Float64Bound // All float64 values
var NameBound    // All name constants (/...)
var StringBound  // All string constants ("...")
var BytesBound   // All byte strings
var NumberBound  // All integers
```

### Boolean Constants

```go
var TrueConstant  // The /true name constant
var FalseConstant // The /false name constant
var TruePredicate  // Unconditionally true proposition
var FalsePredicate // Unconditionally false proposition
```

### Constant Types

```go
type ConstantType int

const (
    NameType    ConstantType = iota // /name constants
    StringType                       // "string" constants
    BytesType                        // b"bytes" constants
    NumberType                       // Integer constants
    Float64Type                      // Float64 constants
    PairType                         // fn:pair(a, b)
    ListType                         // [a, b, c]
    MapType                          // [/key: value]
    StructType                       // {/field: value}
)
```

### Constant Constructors

```go
// Primitive constructors
func Name(symbol string) (Constant, error)  // Create /name constant
func String(str string) Constant            // Create "string" constant
func Bytes(bytes []byte) Constant           // Create byte string
func Number(num int64) Constant             // Create integer
func Float64(floatNum float64) Constant     // Create float64

// Structured data constructors
func Pair(fst, snd *Constant) Constant                  // Create pair
func List(constants []Constant) Constant                // Create list
func Map(kvMap map[*Constant]*Constant) Constant        // Create map
func Struct(kvMap map[*Constant]*Constant) Constant     // Create struct
func ListCons(fst, snd *Constant) Constant              // Cons cell [H|T]
func MapCons(key, val, rest *Constant) Constant         // Map entry
func StructCons(label, val, rest *Constant) Constant    // Struct field
```

### Constant Accessors

```go
// Type-specific value extraction
func (c Constant) NameValue() string         // Extract name
func (c Constant) StringValue() string       // Extract string
func (c Constant) NumberValue() int64        // Extract int64
func (c Constant) Float64Value() float64     // Extract float64
func (c Constant) PairValue() (*Constant, *Constant)  // Extract pair
func (c Constant) ConsValue() (*Constant, *Constant)  // Extract cons cell

// Iteration
func (c Constant) ListValues() func(yield func(*Constant) bool)   // Iterate list
func (c Constant) MapValues() func(yield func(k, v *Constant) bool)
func (c Constant) StructValues() func(yield func(k, v *Constant) bool)

// Empty checks
func (c Constant) IsListNil() bool
func (c Constant) IsMapNil() bool
func (c Constant) IsStructNil() bool
```

### Core Interfaces

```go
// Term is the base interface for all logical terms
type Term interface {
    Equals(Term) bool
    Hash() uint64
    String() string
    ApplySubst(Subst) Term
}

// BaseTerm is the subset used as arguments (constants, variables, ApplyFn)
type BaseTerm interface {
    Term
    ApplySubstBase(Subst) BaseTerm
}

// Subst maps variables to base terms
type Subst interface {
    Get(Variable) (BaseTerm, bool)
}
```

### Predicate and Function Symbols

```go
// PredicateSym represents a predicate with its arity
type PredicateSym struct {
    Symbol string
    Arity  int
}

// FunctionSym represents a function (fn:name)
type FunctionSym struct {
    Symbol string
    Arity  int
}
```

### Logic Terms

```go
// Variable represents a logic variable (uppercase)
type Variable struct {
    Symbol string
}

// Atom represents predicate(args...)
type Atom struct {
    Predicate PredicateSym
    Args      []BaseTerm
}

// NegAtom represents !predicate(args...)
type NegAtom struct {
    Atom Atom
}

// Eq represents X = Y
type Eq struct {
    Left, Right BaseTerm
}

// Ineq represents X != Y
type Ineq struct {
    Left, Right BaseTerm
}
```

### Constructors

```go
// Create atoms
func NewAtom(predicateSym string, args ...BaseTerm) Atom
func NewQuery(pred PredicateSym) Atom  // Query pattern with variables
func NewNegAtom(predicateSym string, args ...BaseTerm) NegAtom

// Create clauses (rules)
func NewClause(head Atom, premises []Term) Clause
```

### Utility Functions

```go
// Variable management
func FreshVariable(used map[Variable]bool) Variable
func AddVars(term Term, m map[Variable]bool)
func AddVarsFromClause(clause Clause, m map[Variable]bool)
func ReplaceWildcards(used map[Variable]bool, term Term) Term

// Formatting
func FormatNumber(num int64) string
func FormatFloat64(floatNum float64) string

// Comparison
func EqualsConstants(left, right []Constant) bool
func HashConstants(constants []Constant) uint64
```

---

## Parse Package (`parse`)

Parse Mangle source code into AST.

### Functions

```go
// Parse a complete source unit (file)
func Unit(r io.Reader) (SourceUnit, error)

// Parse individual components
func Atom(s string) (ast.Atom, error)
func Clause(s string) (ast.Clause, error)
func Term(s string) (ast.Term, error)
func Decl(s string) (ast.Decl, error)
```

### Usage Example

```go
import (
    "strings"
    "github.com/google/mangle/parse"
)

program := `
    parent(/oedipus, /antigone).
    sibling(X, Y) :- parent(P, X), parent(P, Y), X != Y.
`

unit, err := parse.Unit(strings.NewReader(program))
if err != nil {
    return err
}
// unit.Clauses contains parsed facts and rules
// unit.Decls contains declarations
```

---

## Analysis Package (`analysis`)

Validate and analyze Mangle programs.

### Functions

```go
// Analyze a parsed source unit
func AnalyzeOneUnit(unit parse.SourceUnit, knownPredicates map[ast.PredicateSym]*ast.Decl) (*ProgramInfo, error)

// Analyze with imports
func AnalyzeUnit(unit parse.SourceUnit, packages map[string]*ProgramInfo) (*ProgramInfo, error)
```

### ProgramInfo

```go
type ProgramInfo struct {
    Decls     map[ast.PredicateSym]*ast.Decl  // Predicate declarations
    Rules     []ast.Clause                     // IDB rules
    InitialFacts []ast.Atom                    // EDB facts
    // ... internal fields
}
```

---

## Engine Package (`engine`)

Evaluate Mangle programs using semi-naive evaluation.

### Evaluation Functions

```go
// Evaluate program to fixed point
func EvalProgram(program *analysis.ProgramInfo, store factstore.FactStore) error

// Evaluate with statistics
func EvalProgramWithStats(program *analysis.ProgramInfo, store factstore.FactStore) (Stats, error)

// Stratified evaluation (for negation)
func EvalStratifiedProgram(program *analysis.ProgramInfo, initialFacts map[ast.PredicateSym][]ast.Atom, store factstore.FactStore) error
func EvalStratifiedProgramWithStats(program *analysis.ProgramInfo, initialFactProviders, handlers map[ast.PredicateSym]func() ([]ast.Atom, error), store factstore.FactStore) (Stats, error)
```

### QueryContext

```go
type QueryContext struct {
    PredToRules map[ast.PredicateSym][]ast.Clause
    PredToDecl  map[ast.PredicateSym]*ast.Decl
    Store       factstore.FactStore
}

// Execute a query
func (ctx *QueryContext) EvalQuery(goal ast.Atom, mode ast.Mode, subst unionfind.UnionFind, callback func(ast.Atom) error) error
```

---

## FactStore Package (`factstore`)

Store and retrieve facts.

### Interfaces

```go
type FactStore interface {
    Add(atom ast.Atom) bool          // Add fact, returns true if new
    GetFacts(query ast.Atom, callback func(ast.Atom) error) error
    ListPredicates() []ast.PredicateSym
    EstimateFactCount() int
}

type FactStoreWithRemove interface {
    FactStore
    Remove(atom ast.Atom) bool
}

type ConcurrentFactStore interface {
    FactStore
    // Thread-safe operations
}
```

### Implementations

```go
// In-memory store
func NewSimpleInMemoryStore() *SimpleInMemoryStore

// Concurrent wrapper
func NewConcurrentFactStore(store FactStoreWithRemove) ConcurrentFactStore
```

---

## JSON to Struct Conversion (`json2struct`)

Convert JSON data to Mangle structs.

### Functions

```go
// Convert JSON blob to Mangle struct
func JSONtoStruct(jsonBlob []byte) (ast.Constant, error)

// Convert individual value
func ConvertValue(value any) (ast.Constant, error)
```

### Type Mappings

| JSON Type | Mangle Type |
|-----------|-------------|
| string | `ast.String` |
| number | `ast.Float64` |
| boolean | `ast.TrueConstant` / `ast.FalseConstant` |
| array | `ast.List` |
| object | `ast.Struct` |

### Usage Example

```go
import "github.com/google/mangle/json2struct"

jsonData := []byte(`{"name": "Alice", "age": 30, "active": true}`)
mangleStruct, err := json2struct.JSONtoStruct(jsonData)
// Result: {/name: "Alice", /age: 30.0, /active: /true}
```

---

## Protobuf to Struct Conversion (`proto2struct`)

Convert Protocol Buffer messages to Mangle structs.

### Functions

```go
// Convert proto message to Mangle struct
func ProtoToStruct(msg protoreflect.Message) (ast.Constant, error)

// Convert individual field value
func ProtoValueToConstant(value protoreflect.Value, fd protoreflect.FieldDescriptor) (ast.Constant, error)

// Convert enum to Mangle name
func ProtoEnumToConstant(enum protoreflect.EnumValueDescriptor) (ast.Constant, error)
```

### Notes

- Only populated fields appear in output struct
- Fields with default values are omitted
- Nested messages converted recursively
- Enums become `/EnumType/Value` names

---

## Mangle Service (Demo)

A gRPC service for experimenting with Mangle.

**Repository**: https://github.com/burakemir/mangle-service

### API Endpoints

| Service | Method | Description |
|---------|--------|-------------|
| `mangle.Mangle.Query` | RPC | Execute queries |
| `mangle.Mangle.Update` | RPC | Add facts/rules |

### Query Request

```json
{"query": "reachable(/a, X)"}
```

### Update Request

```json
{"program": "edge(/d, /e). edge(/e, /f)."}
```

### Running the Service

```bash
# Build
go get ./... && go build ./...

# Run server
go run ./server --source=example/demo.mg

# Run client
go run ./client --query="reachable(X, /d)"

# With persistence
go run server/main.go --db=/tmp/foo.mangle.db.gz --source=example/demo.mg --persist=true
```

**Warning**: This is a demo service, not for production use.

---

## Official Examples

The repository includes example programs in `examples/`:

| File | Pattern Demonstrated |
|------|---------------------|
| `aggregation.mg` | Aggregation operations |
| `ancestor.mg` | Ancestor relationship (recursion) |
| `dataflow_analysis.mg` | Dataflow analysis |
| `dataflow_liveness.mg` | Liveness analysis |
| `example.mg` | Basic introductory example |
| `example_type_error.mg` | Type error handling |
| `flow_checking.mg` | Flow-checking patterns |
| `map_aggregation.mg` | Aggregation with maps |
| `one_or_two_leg_trip.mg` | Path/trip logic |
| `project_aggregation.mg` | Project-level aggregation |
| `reversesamegen.mg` | Reverse and same generation |
| `shortest_path.mg` | Shortest path algorithms |

---

## Common Integration Patterns

### Basic Program Execution

```go
import (
    "strings"
    "github.com/google/mangle/analysis"
    "github.com/google/mangle/engine"
    "github.com/google/mangle/factstore"
    "github.com/google/mangle/parse"
)

func ExecuteProgram(source string) error {
    // 1. Parse
    unit, err := parse.Unit(strings.NewReader(source))
    if err != nil {
        return fmt.Errorf("parse: %w", err)
    }

    // 2. Analyze
    programInfo, err := analysis.AnalyzeOneUnit(unit, nil)
    if err != nil {
        return fmt.Errorf("analyze: %w", err)
    }

    // 3. Create fact store
    store := factstore.NewSimpleInMemoryStore()

    // 4. Evaluate to fixed point
    if _, err := engine.EvalProgramWithStats(programInfo, store); err != nil {
        return fmt.Errorf("eval: %w", err)
    }

    return nil
}
```

### Querying Results

```go
func QueryPredicate(store factstore.FactStore, predName string, arity int) ([]ast.Atom, error) {
    pred := ast.PredicateSym{Symbol: predName, Arity: arity}
    query := ast.NewQuery(pred)

    var results []ast.Atom
    err := store.GetFacts(query, func(atom ast.Atom) error {
        results = append(results, atom)
        return nil
    })
    return results, err
}
```

### Adding Facts Dynamically

```go
func AddFact(store factstore.FactStore, predicate string, args ...ast.BaseTerm) {
    atom := ast.NewAtom(predicate, args...)
    store.Add(atom)
}

// Usage:
name, _ := ast.Name("/alice")
AddFact(store, "person", name)
AddFact(store, "age", name, ast.Number(30))
```

### Type-Safe Constant Handling

```go
func ConstantToGo(c ast.Constant) interface{} {
    switch c.Type {
    case ast.NameType:
        return c.Symbol
    case ast.StringType:
        return c.Symbol
    case ast.NumberType:
        return c.NumValue
    case ast.Float64Type:
        return c.Float64Value
    case ast.ListType:
        var items []interface{}
        for item := range c.ListValues() {
            items = append(items, ConstantToGo(*item))
        }
        return items
    default:
        return c.String()
    }
}
```

---

## Resources

- **GitHub**: https://github.com/google/mangle
- **Go Packages**: https://pkg.go.dev/github.com/google/mangle
- **Documentation**: https://mangle.readthedocs.io
- **Demo Service**: https://github.com/burakemir/mangle-service
