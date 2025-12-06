# internal/core/

The Cortex - the decision-making center of codeNERD. Contains the Mangle kernel, fact management, and shard orchestration.

## Components

```
core/
├── kernel.go         # RealKernel - Mangle engine wrapper
├── virtual_store.go  # FFI gateway to external systems
├── shard_manager.go  # Shard lifecycle and orchestration
└── learning.go       # Autopoiesis pattern tracking
```

## Kernel

The kernel wraps Google Mangle with proper EDB/IDB separation.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        RealKernel                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  FactStore  │  │   Mangle    │  │   ProgramInfo       │  │
│  │    (EDB)    │◄─┤   Engine    │──┤   (compiled IDB)    │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│         ▲                                    ▲              │
│         │                                    │              │
│  ┌──────┴──────┐                    ┌────────┴────────┐     │
│  │   facts[]   │                    │  schemas.gl +   │     │
│  │  (Go slice) │                    │  policy.gl      │     │
│  └─────────────┘                    └─────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### Interface

```go
type Kernel interface {
    // LoadFacts adds facts to EDB and rebuilds the engine
    LoadFacts(facts []Fact) error

    // Query executes a Mangle query and returns matching facts
    Query(predicate string) ([]Fact, error)

    // QueryAll returns all derived facts grouped by predicate
    QueryAll() (map[string][]Fact, error)

    // Assert adds a single fact dynamically
    Assert(fact Fact) error

    // Retract removes all facts matching predicate
    Retract(predicate string) error
}
```

### Fact Type

```go
type Fact struct {
    Predicate string        // e.g., "user_intent"
    Args      []interface{} // Go types: string, int, float64, bool
}

// ToAtom converts to Mangle AST
func (f Fact) ToAtom() (ast.Atom, error) {
    // Handles:
    // - "/name" → Name constant
    // - "string" → String constant
    // - 42 → Number
    // - 3.14 → Float64
    // - true/false → Boolean constants
}

// String returns Datalog representation
func (f Fact) String() string {
    // e.g., user_intent("id1", /mutation, /fix, "auth.go", "").
}
```

### Usage

```go
// Create kernel
k := core.NewRealKernel()

// Load schemas and policy
k.LoadSchemas("schemas.gl content")
k.LoadPolicy("policy.gl content")

// Add facts
k.LoadFacts([]core.Fact{
    {Predicate: "user_intent", Args: []interface{}{"id1", "/mutation", "/fix", "auth.go", ""}},
    {Predicate: "file_topology", Args: []interface{}{"auth.go", "abc123", "/go", 1234, false}},
})

// Query derived facts
actions, _ := k.Query("next_action")
// Returns: [{Predicate: "next_action", Args: ["/spawn_coder"]}]

// Explain derivation
k.Query("why(next_action(/spawn_coder))")
```

## VirtualStore

The FFI gateway that abstracts external systems into logic predicates.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      VirtualStore                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  GetFacts(pred) ─────────────────────────────────────────▶  │
│       │                                                     │
│       ├── "mcp_tool_result" ──▶ Call MCP Server            │
│       ├── "file_content" ─────▶ Read Filesystem            │
│       ├── "shell_exec_result" ─▶ Execute Command           │
│       ├── "browser_dom" ──────▶ Query Rod Session          │
│       └── default ────────────▶ MemStore.GetFacts()        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Virtual Predicates

| Predicate | Source | Description |
|-----------|--------|-------------|
| `file_content(Path, Content)` | Filesystem | Read file contents |
| `shell_exec_result(Cmd, Output, Code)` | Shell | Execute command |
| `mcp_tool_result(Tool, Args, Result)` | MCP | Call MCP tool |
| `browser_dom(Selector, Element)` | Rod | Query DOM |
| `http_response(URL, Status, Body)` | Network | HTTP request |

### Implementation

```go
func (s *VirtualStore) GetFacts(pred ast.PredicateSym) []ast.Atom {
    switch pred.Symbol {
    case "file_content":
        path := extractPath(pred)
        content, _ := os.ReadFile(path)
        return []ast.Atom{
            ast.NewAtom("file_content", ast.String(path), ast.String(content)),
        }

    case "shell_exec_result":
        cmd := extractCmd(pred)
        output, code := execute(cmd)
        return []ast.Atom{
            ast.NewAtom("shell_exec_result",
                ast.String(cmd),
                ast.String(output),
                ast.Number(code)),
        }

    default:
        return s.MemStore.GetFacts(pred)
    }
}
```

## ShardManager

Orchestrates shard lifecycle and parallel execution.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      ShardManager                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  Registry   │  │  Pool       │  │   Orchestrator      │  │
│  │  (factory)  │  │  (active)   │  │   (scheduling)      │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│         │               │                    │              │
│         ▼               ▼                    ▼              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                   Shard Pool                        │    │
│  │  ┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐        │    │
│  │  │ Coder │  │ Test  │  │ Revw  │  │ Rsrch │        │    │
│  │  └───────┘  └───────┘  └───────┘  └───────┘        │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### Interface

```go
type ShardManager interface {
    // Spawn creates and registers a new shard
    Spawn(shardType string, config ShardConfig) (Shard, error)

    // Get retrieves an active shard by ID
    Get(id string) (Shard, bool)

    // Execute runs a task on the appropriate shard
    Execute(ctx context.Context, shardType, task string) (string, error)

    // Shutdown terminates a shard
    Shutdown(id string) error

    // ShutdownAll terminates all shards
    ShutdownAll() error

    // List returns all active shards
    List() []ShardInfo
}
```

### Shard Delegation

The kernel determines which shard to invoke:

```mangle
# Shard selection rules
delegate_to(/coder) :-
    user_intent(_, /mutation, _, _, _).

delegate_to(/tester) :-
    user_intent(_, /mutation, /test, _, _).

delegate_to(/reviewer) :-
    user_intent(_, /query, /review, _, _).

delegate_to(/researcher) :-
    user_intent(_, /query, /research, _, _),
    requires_deep_knowledge(_).
```

### Usage

```go
mgr := core.NewShardManager(deps)

// Spawn a coder shard
shard, _ := mgr.Spawn("coder", core.ShardConfig{
    Type: core.ShardTypeGeneralist,
})

// Execute a task
result, _ := mgr.Execute(ctx, "coder", `{
    "action": "fix",
    "target": "auth.go",
    "context": "Login fails for special chars"
}`)

// Shutdown when done
mgr.Shutdown(shard.ID())
```

## Learning (Autopoiesis)

Tracks patterns for runtime learning without retraining.

### Pattern Tracking

```go
type LearningTracker struct {
    rejections  map[string]int  // pattern → count
    acceptances map[string]int  // pattern → count
    preferences []Preference    // derived preferences
}

// Track rejection
func (t *LearningTracker) TrackRejection(pattern, reason string) {
    t.rejections[pattern]++
    if t.rejections[pattern] >= 3 {
        t.promoteToPreference(pattern, reason)
    }
}
```

### Mangle Integration

```mangle
# Detect preference signal
preference_signal(Pattern) :-
    rejection_count(Pattern, N), N >= 3.

# Promote to long-term memory
promote_to_long_term(FactType, FactValue) :-
    preference_signal(Pattern),
    derived_rule(Pattern, FactType, FactValue).
```

## Key Design Decisions

### 1. Mangle Engine Rebuild

The engine rebuilds on every fact change. This ensures:
- IDB rules always reflect current EDB
- No stale derivations
- Consistent query results

### 2. Fact Immutability

Facts are append-only with explicit retraction:
- `Assert()` adds facts
- `Retract()` removes by predicate
- No in-place updates

### 3. Shard Isolation

Each shard owns its kernel instance:
- Prevents cross-contamination
- Allows parallel reasoning
- Enables independent policy

### 4. Virtual Predicate Laziness

Virtual predicates evaluate on demand:
- No upfront I/O
- Results cached per query
- Enables infinite virtual facts

## Error Handling

```go
// Kernel errors
var (
    ErrInvalidFact    = errors.New("invalid fact structure")
    ErrParseFailure   = errors.New("failed to parse Mangle")
    ErrQueryFailure   = errors.New("query execution failed")
    ErrNotPermitted   = errors.New("action not permitted")
)

// Shard errors
var (
    ErrShardNotFound  = errors.New("shard not found")
    ErrShardBusy      = errors.New("shard is busy")
    ErrShardFailed    = errors.New("shard execution failed")
)
```

## Thread Safety

- `RealKernel` uses `sync.RWMutex` for concurrent access
- `ShardManager` uses mutex for pool operations
- Individual shards are single-threaded (one task at a time)
