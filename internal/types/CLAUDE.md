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
| `interfaces.go` | Core interfaces breaking import cycles. Exports `Kernel` (LoadFacts/Query/Assert/Retract), `LLMClient` (Complete/CompleteWithSystem/CompleteWithTools), `LLMToolResponse` (with GroundingSources), `ShardAgent` (Execute/GetID/GetState), `ShardFactory`, `LearningStore`, `LimitsEnforcer`, `GroundingProvider` (read-only grounding status), `GroundingController` (control grounding features - Gemini only). |

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

### GroundingProvider (Optional Interface)

LLM clients that support grounding (like GeminiClient with Google Search) implement this interface:

```go
type GroundingProvider interface {
    GetLastGroundingSources() []string
    IsGoogleSearchEnabled() bool
    IsURLContextEnabled() bool
}

// Usage: Type assertion to check for grounding support
if gp, ok := client.(types.GroundingProvider); ok {
    sources := gp.GetLastGroundingSources()
}
```

### GroundingController (Optional Interface)

Extends GroundingProvider with control methods. Use this when you need to enable/disable grounding (only available for Gemini):

```go
type GroundingController interface {
    GroundingProvider
    SetEnableGoogleSearch(enable bool)
    SetEnableURLContext(enable bool)
    SetURLContextURLs(urls []string) // Max 20 URLs, 34MB each
}

// Usage: Type assertion to check for grounding control
if gc, ok := client.(types.GroundingController); ok {
    gc.SetEnableGoogleSearch(true)
    gc.SetURLContextURLs([]string{"https://docs.example.com"})
}
```

**Used by:** `internal/tools/research/grounding.go` (GroundingHelper), `internal/init/` (strategic knowledge generation)

### ThinkingProvider (Optional Interface)

LLM clients that support explicit thinking/reasoning mode (like GeminiClient with Thinking Mode) implement this interface. This metadata feeds into the System Prompt Learning (SPL) system:

```go
type ThinkingProvider interface {
    GetLastThoughtSummary() string  // Model's reasoning process
    GetLastThinkingTokens() int     // Tokens used for reasoning
    IsThinkingEnabled() bool
    GetThinkingLevel() string       // "minimal", "low", "medium", "high"
}

// Usage: Type assertion to check for thinking metadata
if tp, ok := client.(types.ThinkingProvider); ok {
    summary := tp.GetLastThoughtSummary()
    tokens := tp.GetLastThinkingTokens()
}
```

**SPL Integration:** The `recordShardExecution()` function in `cmd/nerd/chat/delegation.go` extracts thinking metadata via this interface and populates `ExecutionRecord.ThoughtSummary` and `ExecutionRecord.ThinkingTokens` for LLM-as-Judge evaluation.

## Dependencies

- `github.com/google/mangle/ast` - Mangle AST types

## Testing

```bash
go test ./internal/types/...
```

---

**Remember: Push to GitHub regularly!**
