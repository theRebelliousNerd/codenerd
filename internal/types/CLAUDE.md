# internal/types - Shared Type Definitions

This package provides shared type definitions used across codeNERD packages. It exists to break import cycles between core, articulation, and autopoiesis.

**Related Packages:**
- [internal/core](../core/CLAUDE.md) - Kernel implementation using these types
- [internal/shards](../shards/CLAUDE.md) - Shard implementations using ShardAgent
- [internal/store](../store/CLAUDE.md) - LearningStore implementation

## Architecture

Types in this package are foundational data structures with no complex dependencies:
- **Fact**: Core logical fact representation
- **ShardAgent**: Interface for all shard implementations
- **SessionContext**: Blackboard pattern for cross-shard state
- **Interfaces**: Kernel, LLMClient, LearningStore, LimitsEnforcer

## File Index

| File | Description |
|------|-------------|
| `types.go` | Core Mangle fact types and SessionContext for blackboard pattern. Exports `MangleAtom`, `Fact` with String()/ToAtom(), `SessionContext` containing compressed history, diagnostics, git context, knowledge atoms, allowed/blocked actions. |
| `shard.go` | Shard type definitions and configuration structures. Exports `ShardType` enum (ephemeral/persistent/user/system), `ShardState` enum, `ShardPermission` enum (8 permissions), `ModelCapability` enum, `ShardConfig`, `ShardResult`. |
| `interfaces.go` | Core interfaces breaking import cycles. Exports `Kernel` (LoadFacts/Query/Assert/Retract), `LLMClient` (Complete/CompleteWithSystem), `ShardAgent` (Execute/GetID/GetState), `ShardFactory`, `LearningStore`, `LimitsEnforcer`. |

## Key Types

### Fact
```go
type Fact struct {
    Predicate string
    Args      []interface{}
}
```

### SessionContext (Blackboard Pattern)
```go
type SessionContext struct {
    CompressedHistory   string
    CurrentDiagnostics  []Diagnostic
    TestState           *TestState
    ActiveFiles         []string
    ActiveSymbols       []string
    GitContext          *GitContext
    CampaignContext     *CampaignContext
    PriorShardOutputs   map[string]string
    KnowledgeAtoms      []string
    SpecialistHints     []string
    AllowedActions      []string
    BlockedActions      []string
    SafetyWarnings      []string
}
```

### Shard Types
```go
const (
    ShardTypeEphemeral  ShardType = "ephemeral"  // Type A
    ShardTypePersistent ShardType = "persistent" // Type B
    ShardTypeUser       ShardType = "user"       // Alias for persistent
    ShardTypeSystem     ShardType = "system"     // Type S
)
```

### Permissions
```go
const (
    PermissionReadFile  ShardPermission = "read_file"
    PermissionWriteFile ShardPermission = "write_file"
    PermissionExecCmd   ShardPermission = "exec_cmd"
    PermissionNetwork   ShardPermission = "network"
    PermissionBrowser   ShardPermission = "browser"
    PermissionCodeGraph ShardPermission = "code_graph"
    PermissionAskUser   ShardPermission = "ask_user"
    PermissionResearch  ShardPermission = "research"
)
```

## Dependencies

- `github.com/google/mangle/ast` - Mangle AST types

## Testing

```bash
go test ./internal/types/...
```
