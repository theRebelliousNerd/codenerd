# Tool Registration Integration

This document describes the integration of tool registration with the kernel and shards.

## Overview

The tool registration system bridges three components:
1. **Ouroboros Loop** - Generates and compiles tools
2. **Mangle Kernel** - Tracks tool availability via logic facts
3. **Shards** - Query and execute available tools

## Architecture

```
┌─────────────────┐
│ Ouroboros Loop  │ Generates tools → compiles → creates binaries
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  ToolRegistry   │ Registers tools → injects facts into kernel
└────────┬────────┘
         │
         ├──────────────────┐
         ▼                  ▼
┌─────────────┐      ┌──────────┐
│ Mangle      │      │  Shards  │ Query tools → execute via VirtualStore
│ Kernel      │      │          │
└─────────────┘      └──────────┘
```

## Components

### 1. Schema Declarations (schemas.gl)

```datalog
# Tool registration details
Decl registered_tool(ToolName, Command, ShardAffinity).

# Tool is available for use
Decl tool_available(ToolName).

# Tool exists in registry
Decl tool_exists(ToolName).

# Tool is compiled and ready
Decl tool_ready(ToolName).

# Tool hash for change detection
Decl tool_hash(ToolName, Hash).

# Capabilities provided by tools
Decl tool_capability(ToolName, Capability).
```

### 2. Policy Rules (policy.gl)

```datalog
# Tool is available (registered and ready)
tool_available(ToolName) :-
    registered_tool(ToolName, _, _).

# Tool exists in registry
tool_exists(ToolName) :-
    tool_registered(ToolName, _).

# Tool is ready for execution
tool_ready(ToolName) :-
    tool_exists(ToolName),
    tool_hash(ToolName, _).

# Capability is available if any tool provides it
capability_available(Cap) :-
    tool_capability(_, Cap).
```

### 3. ToolRegistry (tool_registry.go)

The registry manages tool metadata and kernel fact injection:

```go
type Tool struct {
    Name          string
    Command       string    // Path to binary
    ShardAffinity string    // /coder, /tester, /reviewer, /all
    Description   string
    Capabilities  []string
    Hash          string
    RegisteredAt  time.Time
    ExecuteCount  int64
}

type ToolRegistry struct {
    tools   map[string]*Tool
    kernel  Kernel
    workDir string
}
```

**Key Methods:**

- `RegisterTool(name, command, shardAffinity)` - Register a tool and inject facts
- `GetTool(name)` - Retrieve a registered tool
- `GetToolsForShard(shardType)` - Get tools for a specific shard
- `SyncFromOuroboros(executor)` - Sync from Ouroboros registry
- `UnregisterTool(name)` - Remove tool and retract facts

### 4. VirtualStore Integration

The VirtualStore coordinates tool execution:

```go
// Register a tool
vs.RegisterTool("context7_docs", "/path/to/binary", "/researcher")

// Query tools for a shard
tools := vs.GetToolsForShard("/researcher")

// Execute via exec_tool action
action := Fact{
    Predicate: "next_action",
    Args: []interface{}{"/exec_tool", "context7_docs", map[string]interface{}{
        "input": "query string",
    }},
}
```

## Usage Flow

### During /init (Tool Generation)

1. **Ouroboros generates tools:**
   ```go
   loop := NewOuroborosLoop(client, config)
   result := loop.Execute(ctx, need)
   ```

2. **VirtualStore syncs to registry:**
   ```go
   vs.SetToolExecutor(loop) // Automatically syncs tools
   ```

3. **Facts injected into kernel:**
   ```datalog
   registered_tool("context7_docs", "/path/to/binary", /researcher).
   tool_registered("context7_docs", "2025-12-06T10:30:00Z").
   tool_hash("context7_docs", "abc123...").
   tool_capability("context7_docs", "search").
   tool_capability("context7_docs", "fetch").
   ```

### During Shard Execution

1. **Shard queries available tools:**
   ```go
   tools := vs.GetToolsForShard("/researcher")
   // Returns: [context7_docs, github_search, ...]
   ```

2. **Kernel derives tool availability:**
   ```datalog
   tool_available("context7_docs") :-
       registered_tool("context7_docs", _, _).
   ```

3. **Shard executes tool via VirtualStore:**
   ```go
   output, err := vs.RouteAction(ctx, Fact{
       Predicate: "next_action",
       Args: []interface{}{"/exec_tool", "context7_docs", ...},
   })
   ```

### Tool Affinity

Tools can be restricted to specific shard types:

| ShardAffinity | Available To |
|---------------|--------------|
| `/coder` | CoderShard only |
| `/tester` | TesterShard only |
| `/reviewer` | ReviewerShard only |
| `/researcher` | ResearcherShard only |
| `/generalist` | Ephemeral shards |
| `/all` | All shards |

Example:
```go
// Only ResearcherShard can use this tool
registry.RegisterTool("deep_research", "/path/to/binary", "/researcher")

// All shards can use this tool
registry.RegisterTool("file_reader", "/path/to/binary", "/all")
```

## Example: Complete Flow

```go
// 1. Create kernel and VirtualStore
kernel := NewRealKernel()
vs := NewVirtualStore(executor)
vs.SetKernel(kernel)

// 2. Setup Ouroboros and register tools
loop := NewOuroborosLoop(client, DefaultOuroborosConfig(workDir))
vs.SetToolExecutor(loop) // Auto-syncs to registry

// 3. Generate a tool
need := &ToolNeed{
    Name:        "context7_docs",
    Capability:  "fetch_context7_documentation",
    Description: "Fetch documentation from Context7 API",
}
result := loop.Execute(ctx, need)

// 4. Tool automatically registered and facts injected
kernel.Evaluate()

// 5. Query kernel to verify
facts, _ := kernel.Query("tool_available")
// Returns: [tool_available("context7_docs")]

// 6. Shard queries available tools
tools := vs.GetToolsForShard("/researcher")
// Returns: [{Name: "context7_docs", ...}]

// 7. Shard executes tool
output, _ := vs.RouteAction(ctx, Fact{
    Predicate: "next_action",
    Args: []interface{}{"/exec_tool", "context7_docs", map[string]interface{}{
        "input": "search query",
    }},
})
```

## Testing

```bash
# Run registry tests (no kernel dependency)
go test ./internal/core -run TestToolRegistry_GetToolsForShard -v

# Test tool registration
go test ./internal/core -run TestToolRegistry_RegisterTool -v

# Test shard affinity
go test ./internal/core -run TestToolRegistry_GetToolsForShard -v
```

## File Locations

| Component | File | Purpose |
|-----------|------|---------|
| Registry | `internal/core/tool_registry.go` | Tool management & fact injection |
| Tests | `internal/core/tool_registry_test.go` | Registry unit tests |
| Schemas | `internal/mangle/schemas.gl` | Fact declarations (Section 28) |
| Policy | `internal/mangle/policy.gl` | Derived rules (Section 12B) |
| Integration | `internal/core/virtual_store.go` | VirtualStore methods |

## Facts Generated

When a tool is registered, these facts are injected into the kernel:

```datalog
# Core registration
registered_tool(ToolName, Command, ShardAffinity).
tool_registered(ToolName, RegisteredAt).

# Optional metadata
tool_hash(ToolName, Hash).               # If hash available
tool_capability(ToolName, Capability).   # For each capability

# Derived by policy rules
tool_available(ToolName).                # Derived from registered_tool
tool_exists(ToolName).                   # Derived from tool_registered
tool_ready(ToolName).                    # Derived from exists + hash
capability_available(Capability).        # Derived from any tool_capability
```

## Future Enhancements

1. **Tool Dependencies**: Track dependencies between tools
2. **Tool Versions**: Support multiple versions of the same tool
3. **Tool Metrics**: Track execution time, success rate, quality scores
4. **Tool Learning**: Integrate with autopoiesis feedback loop
5. **Tool Discovery**: Scan directories for pre-existing tools
6. **Tool Validation**: Verify tool binaries before registration
7. **Tool Cleanup**: Remove orphaned binaries
8. **Shard Preferences**: Learn which tools work best for which shards

## Summary

The tool registration integration creates a complete feedback loop:

1. **Ouroboros** generates tools
2. **Registry** tracks them and injects facts
3. **Kernel** reasons about availability
4. **Shards** query and execute tools
5. **Feedback** flows back to improve future generations

This enables the system to grow its capabilities organically while maintaining formal correctness through Mangle logic.
