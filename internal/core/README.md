# internal/core/

The Cortex - the decision-making center of codeNERD. Contains the Mangle kernel, fact management, and VirtualStore (FFI gateway).

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

---

## ⚠️ Major Change: ShardManager Removed

As of **December 2024**, the `ShardManager` has been **removed** and replaced by the **Session Executor** in `internal/session/`.

### What Changed

- **Removed:** `internal/core/shard_manager.go` (~12,000 lines across 5 modularized files)
- **Replaced By:** `internal/session/executor.go` + `spawner.go` + `subagent.go` (~1,115 lines)

**Shard orchestration is now handled by:**
- **Intent Routing** (`internal/mangle/intent_routing.mg`) - Declarative persona selection
- **ConfigFactory** (`internal/prompt/config_factory.go`) - AgentConfig generation
- **Session Executor** (`internal/session/executor.go`) - Universal execution loop
- **Spawner** (`internal/session/spawner.go`) - Dynamic subagent creation

---

## Current Components

```
core/
├── kernel_*.go          # RealKernel - Mangle engine wrapper (modularized into 8 files)
├── virtual_store*.go    # FFI gateway to external systems (modularized)
├── fact_categories.go   # Fact categorization for context management
├── consistency_test.go  # Kernel consistency tests
├── defaults/            # Default schemas and policies
│   ├── schema/         # Mangle schema definitions
│   └── policy/         # Mangle policy rules
└── shards/             # DEPRECATED - minimal compatibility only
    └── CLAUDE.md       # Legacy documentation
```

**Modularization:**
- `kernel.go` split into 8 files: `kernel_types.go`, `kernel_init.go`, `kernel_facts.go`, `kernel_query.go`, `kernel_eval.go`, `kernel_validation.go`, `kernel_policy.go`, `kernel_virtual.go`
- `virtual_store.go` modularized with specialized files like `virtual_store_graph.go`

---

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
│  │   facts[]   │                    │  schemas.mg +   │     │
│  │  (Go slice) │                    │  policy.mg +    │     │
│  │             │                    │  intent_routing │     │
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
    // - "/name" → Name constant (atom)
    // - "string" → String constant
    // - 42 → Number
    // - 3.14 → Float64
    // - true/false → Boolean constants
}

// String returns Datalog representation
func (f Fact) String() string {
    // e.g., user_intent("id1", /command, /fix, "auth.go", /none).
}
```

### Usage

```go
// Create kernel
k := core.NewRealKernel()

// Load schemas and policy
k.LoadSchemas("schemas.mg content")
k.LoadPolicy("policy.mg content")

// Add facts
k.LoadFacts([]core.Fact{
    {Predicate: "user_intent", Args: []interface{}{"id1", "/command", "/fix", "auth.go", ""}},
    {Predicate: "file_topology", Args: []interface{}{"auth.go", "abc123", "/go", 1234, false}},
})

// Query derived facts (Intent Routing)
personas, _ := k.Query("persona")
// Returns: [{Predicate: "persona", Args: ["/coder"]}]

// Explain derivation
k.Query("why(persona(/coder))")
```

### Intent Routing Integration

The kernel now loads `internal/mangle/intent_routing.mg` which contains declarative persona selection rules:

```mangle
# Loaded automatically by kernel initialization
persona(/coder) :- user_intent(_, _, /fix, _, _).
persona(/tester) :- user_intent(_, _, /test, _, _).
persona(/reviewer) :- user_intent(_, _, /review, _, _).
persona(/researcher) :- user_intent(_, _, /research, _, _).
```

---

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
│       ├── "graph_query_result"─▶ Query World Model Graph   │ (NEW)
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
| `graph_query_result(QueryType, Params, Result)` | **NEW** World Model | Query code graph |

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

    case "graph_query_result":
        // NEW: Query World Model graph
        queryType := extractQueryType(pred)
        params := extractParams(pred)
        result := s.worldModel.QueryGraph(queryType, params)
        return result.ToAtoms()

    default:
        return s.MemStore.GetFacts(pred)
    }
}
```

### Integration with Session Executor

The VirtualStore is used by the Session Executor to route tool calls:

```go
// internal/session/executor.go
func (e *Executor) ExecuteToolCall(ctx context.Context, toolCall ToolCall) (string, error) {
    // Check if tool is allowed by AgentConfig
    if !e.isToolAllowed(toolCall.Name, e.currentConfig.Tools) {
        return "", fmt.Errorf("tool %s not allowed", toolCall.Name)
    }

    // Route through VirtualStore
    return e.virtualStore.ExecuteTool(ctx, toolCall)
}
```

---

## Fact Categories (NEW)

**File:** `fact_categories.go`

Categorizes facts for context management and spreading activation:

```go
type FactCategory string

const (
    FactCategoryIntent      FactCategory = "intent"       // user_intent, goal
    FactCategoryWorld       FactCategory = "world"        // file_topology, symbol_graph
    FactCategoryDiagnostic  FactCategory = "diagnostic"   // test_state, build_error
    FactCategoryAction      FactCategory = "action"       // next_action, permitted
    FactCategoryContext     FactCategory = "context"      // context_atom, priority
    FactCategoryLearning    FactCategory = "learning"     // learned_pattern, preference
    FactCategorySession     FactCategory = "session"      // session_state, phase
)

func CategorizeFactByPredicate(predicate string) FactCategory {
    // Automatic categorization based on predicate name
}
```

Used by:
- **Context Paging** (`internal/campaign/context_pager.go`)
- **Spreading Activation** (`internal/context/activation.go`)
- **JIT Compiler** (`internal/prompt/compiler.go`)

---

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

### 3. Removed: Shard Isolation

**Old Architecture:**
Each shard owned its kernel instance for isolation.

**New Architecture:**
SubAgents share the parent kernel but have isolated conversation histories. The Session Executor manages tool permissions via AgentConfig rather than kernel-level isolation.

**Rationale:**
- Simpler architecture (one kernel, not N kernels)
- Permissions enforced by AgentConfig.Tools rather than kernel separation
- Conversation isolation sufficient for context management

### 4. Virtual Predicate Laziness

Virtual predicates evaluate on demand:
- No upfront I/O
- Results cached per query
- Enables infinite virtual facts

---

## Integration with JIT-Driven Architecture

### Kernel Role in New Architecture

```
User Input → Perception Transducer → user_intent atoms
                                          ↓
                                    Kernel.Assert(user_intent)
                                          ↓
                           Kernel.Query("persona(P)") ← Intent Routing (.mg)
                                          ↓
                                     persona(/coder)
                                          ↓
                           ConfigFactory.Generate(...) + JIT Compiler
                                          ↓
                                   Session Executor
                                          ↓
                         VirtualStore.ExecuteTool(...) ← Tool routing
```

### VirtualStore Role in New Architecture

```
Session Executor receives LLM response with tool calls
     ↓
For each tool call:
     ↓
Check AgentConfig.Tools (permission)
     ↓
VirtualStore.ExecuteTool(toolName, params)
     ↓
Route to appropriate system (filesystem, shell, MCP, graph)
     ↓
Return result to Session Executor
     ↓
Inject result into next LLM turn
```

---

## Error Handling

```go
// Kernel errors
var (
    ErrInvalidFact    = errors.New("invalid fact structure")
    ErrParseFailure   = errors.New("failed to parse Mangle")
    ErrQueryFailure   = errors.New("query execution failed")
    ErrNotPermitted   = errors.New("action not permitted")
)

// VirtualStore errors
var (
    ErrToolNotFound   = errors.New("tool not found")
    ErrToolFailed     = errors.New("tool execution failed")
    ErrPermissionDenied = errors.New("tool permission denied")
)
```

---

## Thread Safety

- `RealKernel` uses `sync.RWMutex` for concurrent access
- `VirtualStore` is thread-safe for concurrent tool execution
- **Removed:** ShardManager mutex (no longer exists)
- **New:** Session Executor manages concurrency per subagent

---

## Modularization

The `internal/core/` package has been modularized for better maintainability:

### Kernel Modularization (8 files)

| File | Purpose |
|------|---------|
| `kernel_types.go` | Core type definitions (RealKernel, Fact) |
| `kernel_init.go` | Constructor, Mangle engine boot |
| `kernel_facts.go` | LoadFacts, Assert, Retract |
| `kernel_query.go` | Query execution, pattern matching |
| `kernel_eval.go` | Policy evaluation, rule execution |
| `kernel_validation.go` | Schema validation, safety checks |
| `kernel_policy.go` | Policy/schema loading |
| `kernel_virtual.go` | Virtual predicate handling |

### VirtualStore Modularization

| File | Purpose |
|------|---------|
| `virtual_store.go` | Main VirtualStore implementation |
| `virtual_store_graph.go` | **NEW** GraphQuery integration |

---

## Migration Notes

### For Developers Using `internal/core/`

**What Still Works:**
- `RealKernel` interface (unchanged)
- `VirtualStore` interface (unchanged)
- Fact management (Assert, Retract, Query)
- Virtual predicates

**What Changed:**
- **Removed:** `ShardManager` - use `internal/session/Executor` and `Spawner` instead
- **Removed:** Shard-specific kernel instances - SubAgents share parent kernel
- **Added:** Intent routing logic in kernel initialization
- **Added:** GraphQuery virtual predicate

**Migration Example:**

```go
// OLD (DELETED)
shardMgr := core.NewShardManager()
shardMgr.Spawn(ctx, "coder", task)

// NEW
executor := session.NewExecutor(kernel, virtualStore, llmClient, jitCompiler, configFactory, transducer)
executor.Execute(ctx, task)

// Or for subagents
spawner := session.NewSpawner(kernel, virtualStore, llmClient, jitCompiler, configFactory, transducer, cfg)
subagent, _ := spawner.Spawn(ctx, session.SpawnRequest{
    Name: "coder",
    Task: task,
    Type: session.Ephemeral,
    IntentVerb: "fix",
})
```

---

## See Also

- [Session Executor](../session/executor.go) - Universal execution loop (replaces ShardManager)
- [Spawner](../session/spawner.go) - Dynamic subagent creation
- [Intent Routing](../mangle/intent_routing.mg) - Declarative persona selection
- [JIT-Driven Execution Model](../../.claude/skills/codenerd-builder/references/jit-execution-model.md) - Complete architecture guide
- [VirtualStore Graph Integration](virtual_store_graph.go) - GraphQuery implementation

---

**Last Updated:** December 27, 2024
**Architecture Version:** 2.0.0 (JIT-Driven)
