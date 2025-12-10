# Mangle Integration: Go↔Logic Bridge

This reference covers the complete integration between Go applications and Google Mangle, with special attention to patterns used in the codeNERD neuro-symbolic architecture.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Go Application                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │
│   │  Perception │───▶│   Kernel    │───▶│Articulation │        │
│   │  Transducer │    │  (Mangle)   │    │   Emitter   │        │
│   └─────────────┘    └──────┬──────┘    └─────────────┘        │
│          │                  │                  │                │
│          ▼                  ▼                  ▼                │
│   ┌─────────────────────────────────────────────────────┐      │
│   │                   Virtual Store                      │      │
│   │  (File System, AST, Network, LLM Peripherals)       │      │
│   └─────────────────────────────────────────────────────┘      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Import Structure

AI agents frequently hallucinate import paths. Use these exact imports:

```go
import (
    // Core AST types
    "github.com/google/mangle/ast"

    // Parsing rules and facts
    "github.com/google/mangle/parse"

    // Safety and stratification analysis
    "github.com/google/mangle/analysis"

    // Evaluation engine
    "github.com/google/mangle/engine"

    // Fact storage
    "github.com/google/mangle/factstore"

    // Builtins (optional)
    "github.com/google/mangle/builtin"
)
```

**Common AI Hallucination**: `import "mangle"` or `import "google/mangle"` - these do NOT exist.

## Fact Creation

### The Type System

Mangle has distinct value types that MUST be used correctly:

| Go Type | Mangle Type | Usage |
|---------|-------------|-------|
| `ast.Atom` | Named constant | Enums, statuses, identifiers: `/active`, `/us_east` |
| `ast.String` | String literal | Human-readable text, paths: `"hello"`, `"/path/to/file"` |
| `ast.Number` | Numeric literal | Integers, floats: `42`, `3.14` |
| `ast.Variable` | Logic variable | Query placeholders: `X`, `Project` |

### Creating Atoms vs Strings

**CRITICAL**: The most common silent failure is Atom/String confusion.

```go
// For enumerated values, statuses, identifiers - use Atom
status := ast.Atom("active")       // Represents /active in Mangle
lang := ast.Atom("go")             // Represents /go in Mangle

// For human-readable text, file paths - use String
name := ast.String("John Smith")   // Represents "John Smith"
path := ast.String("/path/to/file") // Represents "/path/to/file"
```

**Why This Matters**: If facts are stored with atoms but queried with strings (or vice versa), the query returns empty results with NO ERROR.

```mangle
# Facts stored with atoms
status(/user123, /active).

# Query with string - RETURNS NOTHING
?status(/user123, "active").  # Empty result!

# Query with atom - WORKS
?status(/user123, /active).   # Returns the fact
```

### Creating Facts Programmatically

```go
// Method 1: Using factstore.MakeFact
fact, err := factstore.MakeFact("user_status", []engine.Value{
    engine.Atom("user123"),
    engine.Atom("active"),
})
if err != nil {
    return fmt.Errorf("make fact: %w", err)
}
store.Add(fact)

// Method 2: Using AST directly
atom := &ast.Atom{
    Predicate: ast.PredicateSym{Symbol: "user_status"},
    Args: []ast.BaseTerm{
        ast.Atom("user123"),
        ast.Atom("active"),
    },
}
```

### Converting Go Structs to Facts

Pattern used in codeNERD:

```go
// Define ToAtom method on domain types
type FileTopology struct {
    Path     string
    Package  string
    Language string
    Size     int64
    Modified time.Time
}

func (f *FileTopology) ToAtom() (*ast.Atom, error) {
    return &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "file_topology"},
        Args: []ast.BaseTerm{
            ast.String(f.Path),           // Path is a string
            ast.String(f.Package),        // Package name
            ast.Atom(f.Language),         // Language is an enum/atom
            ast.Number(float64(f.Size)),  // Size as number
            ast.Number(float64(f.Modified.Unix())), // Timestamp
        },
    }, nil
}

// Batch conversion
func (store *FactStore) AddFileTopologies(files []FileTopology) error {
    for _, f := range files {
        atom, err := f.ToAtom()
        if err != nil {
            return fmt.Errorf("convert file %s: %w", f.Path, err)
        }
        store.AddAtom(atom)
    }
    return nil
}
```

## Engine Initialization

### Complete Initialization Pattern

```go
func InitializeEngine(rules string) (*engine.Engine, *factstore.SimpleInMemoryStore, error) {
    // Step 1: Parse rules
    units, err := parse.Unit(strings.NewReader(rules))
    if err != nil {
        return nil, nil, fmt.Errorf("parse rules: %w", err)
    }

    // Step 2: Analyze for safety and stratification
    // This catches:
    // - Unbound variables in negation
    // - Stratification violations (negative cycles)
    // - Undefined predicates (in strict mode)
    programInfo, err := analysis.Analyze(units, nil)
    if err != nil {
        return nil, nil, fmt.Errorf("analyze rules: %w", err)
    }

    // Step 3: Create fact store
    store := factstore.NewSimpleInMemoryStore()

    // Step 4: Create engine with program and store
    eng, err := engine.New(programInfo, store)
    if err != nil {
        return nil, nil, fmt.Errorf("create engine: %w", err)
    }

    return eng, store, nil
}
```

### Loading Multiple Rule Files

```go
func LoadRuleFiles(paths []string) (*analysis.ProgramInfo, error) {
    var allUnits []*ast.Unit

    for _, path := range paths {
        data, err := os.ReadFile(path)
        if err != nil {
            return nil, fmt.Errorf("read %s: %w", path, err)
        }

        units, err := parse.Unit(strings.NewReader(string(data)))
        if err != nil {
            return nil, fmt.Errorf("parse %s: %w", path, err)
        }

        allUnits = append(allUnits, units...)
    }

    return analysis.Analyze(allUnits, nil)
}
```

## Querying

### Basic Query

```go
func QueryNextAction(ctx context.Context, eng *engine.Engine) ([]string, error) {
    // Build query with unbound variable
    query := &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "next_action"},
        Args: []ast.BaseTerm{
            ast.Variable("Action"),
        },
    }

    // Execute query
    results, err := eng.Query(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("query: %w", err)
    }

    // Extract results
    var actions []string
    for _, result := range results {
        if atom, ok := result.Args[0].(ast.Atom); ok {
            actions = append(actions, string(atom))
        }
    }

    return actions, nil
}
```

### Parameterized Query

```go
func QueryUserPermissions(ctx context.Context, eng *engine.Engine, userID string) ([]Permission, error) {
    query := &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "permitted"},
        Args: []ast.BaseTerm{
            ast.Atom(userID),      // Bound: specific user
            ast.Variable("Action"), // Unbound: find all actions
            ast.Variable("Resource"), // Unbound: find all resources
        },
    }

    results, err := eng.Query(ctx, query)
    if err != nil {
        return nil, err
    }

    var perms []Permission
    for _, r := range results {
        perms = append(perms, Permission{
            Action:   string(r.Args[1].(ast.Atom)),
            Resource: string(r.Args[2].(ast.Atom)),
        })
    }
    return perms, nil
}
```

## External Predicates (Virtual Store)

External predicates allow Mangle to query Go functions as if they were facts. This is the foundation of codeNERD's VirtualStore.

### Defining an External Predicate

```go
// ExternalPredicateHandler processes queries against external systems
type ExternalPredicateHandler func(query engine.Query, emit func(engine.Fact)) error

// File existence check
func fileExistsHandler(query engine.Query, emit func(engine.Fact)) error {
    // Check binding pattern
    if !query.Args[0].IsBound() {
        // Cannot enumerate all files - require bound path
        return errors.New("file_exists requires bound path argument")
    }

    path := query.Args[0].AsString()
    if _, err := os.Stat(path); err == nil {
        // File exists - emit the fact
        emit(engine.Fact{
            Predicate: "file_exists",
            Args:      query.Args,
        })
    }
    return nil
}
```

### Registering External Predicates

```go
type VirtualStore struct {
    handlers map[string]ExternalPredicateHandler
}

func NewVirtualStore() *VirtualStore {
    return &VirtualStore{
        handlers: make(map[string]ExternalPredicateHandler),
    }
}

func (vs *VirtualStore) Register(predicate string, handler ExternalPredicateHandler) {
    vs.handlers[predicate] = handler
}

// Use with engine
func (vs *VirtualStore) AsExternalPredicates() engine.WithExternalPredicates {
    return engine.WithExternalPredicates(func(pred string, query engine.Query, emit func(engine.Fact)) error {
        if handler, ok := vs.handlers[pred]; ok {
            return handler(query, emit)
        }
        return nil // Unknown predicate - no results
    })
}
```

### Common Virtual Predicates for codeNERD

```go
// AST Query - symbol definitions
func symbolDefinedHandler(query engine.Query, emit func(engine.Fact)) error {
    // Query: symbol_defined(File, Symbol, Line)
    // Returns symbols defined in Go files

    file := query.Args[0].AsString()
    fset := token.NewFileSet()
    node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
    if err != nil {
        return err
    }

    ast.Inspect(node, func(n ast.Node) bool {
        switch d := n.(type) {
        case *ast.FuncDecl:
            emit(engine.Fact{
                Predicate: "symbol_defined",
                Args: []engine.Value{
                    engine.String(file),
                    engine.Atom(d.Name.Name),
                    engine.Number(float64(fset.Position(d.Pos()).Line)),
                },
            })
        case *ast.TypeSpec:
            emit(engine.Fact{
                Predicate: "symbol_defined",
                Args: []engine.Value{
                    engine.String(file),
                    engine.Atom(d.Name.Name),
                    engine.Number(float64(fset.Position(d.Pos()).Line)),
                },
            })
        }
        return true
    })
    return nil
}

// HTTP status check - network peripheral
func httpStatusHandler(query engine.Query, emit func(engine.Fact)) error {
    url := query.Args[0].AsString()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    req, _ := http.NewRequestWithContext(ctx, "HEAD", url, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        emit(engine.Fact{
            Predicate: "http_status",
            Args: []engine.Value{
                engine.String(url),
                engine.Atom("error"),
                engine.String(err.Error()),
            },
        })
        return nil
    }
    defer resp.Body.Close()

    emit(engine.Fact{
        Predicate: "http_status",
        Args: []engine.Value{
            engine.String(url),
            engine.Number(float64(resp.StatusCode)),
            engine.Atom("ok"),
        },
    })
    return nil
}
```

## codeNERD Integration Patterns

### The OODA Loop

```go
// Observe → Orient → Decide → Act

type OODALoop struct {
    engine *engine.Engine
    store  *factstore.SimpleInMemoryStore
    vs     *VirtualStore
}

// Observe: Transducer converts NL to atoms
func (o *OODALoop) Observe(ctx context.Context, input string) error {
    // Parse user intent
    intent, err := o.transducer.Parse(input)
    if err != nil {
        return err
    }

    // Add user_intent fact
    atom := &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "user_intent"},
        Args: []ast.BaseTerm{
            ast.Atom(intent.ID),
            ast.Atom(intent.Category),
            ast.Atom(intent.Verb),
            ast.String(intent.Target),
            ast.String(intent.Constraint),
        },
    }
    o.store.AddAtom(atom)
    return nil
}

// Orient: Spreading activation selects relevant context
func (o *OODALoop) Orient(ctx context.Context) error {
    // Query context_atom to find relevant facts
    results, err := o.engine.Query(ctx, &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "context_atom"},
        Args: []ast.BaseTerm{ast.Variable("Atom")},
    })
    if err != nil {
        return err
    }

    // Activate context...
    return nil
}

// Decide: Mangle derives next_action
func (o *OODALoop) Decide(ctx context.Context) (string, error) {
    results, err := o.engine.Query(ctx, &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "next_action"},
        Args: []ast.BaseTerm{ast.Variable("Action")},
    })
    if err != nil {
        return "", err
    }

    if len(results) == 0 {
        return "", errors.New("no action derived")
    }

    return string(results[0].Args[0].(ast.Atom)), nil
}

// Act: Virtual Store executes action
func (o *OODALoop) Act(ctx context.Context, action string) (string, error) {
    // Check permission first
    permitted, err := o.checkPermitted(ctx, action)
    if err != nil {
        return "", err
    }
    if !permitted {
        return "", fmt.Errorf("action %s not permitted", action)
    }

    // Execute via appropriate peripheral
    return o.vs.Execute(ctx, action)
}
```

### Piggyback Protocol

The Articulation Emitter uses dual-channel output:

```go
type PiggybackResponse struct {
    // Surface channel: Human-readable for user
    Surface string

    // Control channel: Atoms for kernel
    Control []*ast.Atom
}

func (e *Emitter) Emit(ctx context.Context, result ActionResult) (*PiggybackResponse, error) {
    resp := &PiggybackResponse{}

    // Generate human-readable summary
    resp.Surface = e.formatForHuman(result)

    // Generate control atoms for kernel
    resp.Control = append(resp.Control, &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "action_result"},
        Args: []ast.BaseTerm{
            ast.Atom(result.ActionID),
            ast.Atom(result.Status),
            ast.String(result.Output),
        },
    })

    // Add learned facts
    for _, fact := range result.DerivedFacts {
        resp.Control = append(resp.Control, fact)
    }

    return resp, nil
}
```

### Shard Execution Pattern

```go
type Shard interface {
    Execute(ctx context.Context, task Task) (string, error)
}

type ShardManager struct {
    engine *engine.Engine
    shards map[string]Shard
}

// Execute shard and record result
func (sm *ShardManager) ExecuteShard(ctx context.Context, shardName string, task Task) error {
    shard, ok := sm.shards[shardName]
    if !ok {
        return fmt.Errorf("unknown shard: %s", shardName)
    }

    result, err := shard.Execute(ctx, task)

    // Record execution in Mangle
    status := "success"
    if err != nil {
        status = "failed"
        result = err.Error()
    }

    fact := &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "shard_executed"},
        Args: []ast.BaseTerm{
            ast.Atom(shardName),
            ast.String(task.ID),
            ast.Atom(status),
            ast.String(result),
        },
    }
    sm.engine.Store().AddAtom(fact)

    return err
}
```

## Error Handling

### Parse Errors

```go
units, err := parse.Unit(strings.NewReader(rules))
if err != nil {
    // Parse error - syntax problem
    // Common causes:
    // - Missing period at end of rule
    // - Wrong atom syntax ("foo" instead of /foo)
    // - Unbalanced brackets
    return fmt.Errorf("mangle parse error: %w", err)
}
```

### Analysis Errors

```go
programInfo, err := analysis.Analyze(units, nil)
if err != nil {
    // Analysis error - semantic problem
    // Common causes:
    // - Unbound variable in negation
    // - Stratification violation (negative cycle)
    // - Type mismatch in declarations
    return fmt.Errorf("mangle analysis error: %w", err)
}
```

### Query Errors

```go
results, err := eng.Query(ctx, query)
if err != nil {
    // Query error - runtime problem
    // Common causes:
    // - Context cancelled
    // - External predicate failure
    // - Resource exhaustion
    return fmt.Errorf("mangle query error: %w", err)
}
```

## Testing

### Unit Testing Mangle Rules

```go
func TestPermissionRule(t *testing.T) {
    rules := `
        permitted(User, /read, Resource) :-
            user_role(User, /admin).
        permitted(User, /read, Resource) :-
            owner(User, Resource).
    `

    eng, store, err := InitializeEngine(rules)
    if err != nil {
        t.Fatalf("init: %v", err)
    }

    // Add test facts
    store.AddAtom(&ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "user_role"},
        Args: []ast.BaseTerm{ast.Atom("alice"), ast.Atom("admin")},
    })
    store.AddAtom(&ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "owner"},
        Args: []ast.BaseTerm{ast.Atom("bob"), ast.Atom("doc1")},
    })

    // Test admin permission
    results, _ := eng.Query(context.Background(), &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "permitted"},
        Args: []ast.BaseTerm{
            ast.Atom("alice"),
            ast.Atom("read"),
            ast.Variable("Resource"),
        },
    })

    if len(results) == 0 {
        t.Error("expected alice to have read permission")
    }

    // Test owner permission
    results, _ = eng.Query(context.Background(), &ast.Atom{
        Predicate: ast.PredicateSym{Symbol: "permitted"},
        Args: []ast.BaseTerm{
            ast.Atom("bob"),
            ast.Atom("read"),
            ast.Atom("doc1"),
        },
    })

    if len(results) == 0 {
        t.Error("expected bob to have read permission on doc1")
    }
}
```

### Integration Testing

```go
func TestOODALoop(t *testing.T) {
    loop := NewOODALoop()

    ctx := context.Background()

    // Simulate user input
    err := loop.Observe(ctx, "list all go files")
    if err != nil {
        t.Fatalf("observe: %v", err)
    }

    // Orient
    err = loop.Orient(ctx)
    if err != nil {
        t.Fatalf("orient: %v", err)
    }

    // Decide
    action, err := loop.Decide(ctx)
    if err != nil {
        t.Fatalf("decide: %v", err)
    }

    if action != "/list_files" {
        t.Errorf("expected /list_files, got %s", action)
    }
}
```

## Performance Considerations

### Fact Store Selection

| Store Type | Use Case | Characteristics |
|------------|----------|-----------------|
| `SimpleInMemoryStore` | Small datasets (<100K facts) | Fast, no persistence |
| Custom SQLite store | Large datasets, persistence | Slower queries, durable |
| External predicate | Huge/external data | Query on demand |

### Rule Optimization

```mangle
# BAD - Large Cartesian product first
slow(X, Y) :- big_table(X), another_big_table(Y), filter(X, Y).

# GOOD - Filter early
fast(X, Y) :- filter(X, Y), big_table(X), another_big_table(Y).
```

### Incremental Updates

```go
// Don't rebuild entire store - add incrementally
func (s *Store) UpdateFile(file FileTopology) error {
    // Remove old facts for this file
    s.RetractMatching("file_topology", file.Path)

    // Add new facts
    return s.AddFileTopology(file)
}
```

## Common AI Integration Errors

1. **String API hallucination**: `store.Add("pred", "arg")` doesn't exist
2. **Missing analysis step**: Skipping `analysis.Analyze()` leads to runtime failures
3. **Atom/String confusion**: Using wrong type causes silent query failures
4. **Context severance**: Creating `context.Background()` instead of deriving from parent
5. **Error suppression**: Ignoring parse/analysis errors

Always follow the patterns in this reference for reliable Mangle integration.
