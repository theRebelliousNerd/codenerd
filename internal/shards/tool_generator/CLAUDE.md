# internal/shards/tool_generator - Autopoiesis Tool Generation

This package implements the ToolGeneratorShard for autopoiesis operations, handling tool creation, refinement, and status queries using the Ouroboros Loop.

**Related Packages:**
- [internal/autopoiesis](../../autopoiesis/CLAUDE.md) - Orchestrator/Ouroboros providing generation
- [internal/shards](../CLAUDE.md) - Shard registration

## Architecture

The ToolGenerator shard is the user-facing interface to autopoiesis capabilities:
- Accepts natural language tool requests
- Routes to Ouroboros for spec→code→test→compile cycle
- Reports generation status and quality profiles
- Persists learnings for future generations

## File Index

| File | Description |
|------|-------------|
| `tool_generator.go` | `ToolGeneratorShard` implementing tool generation via Ouroboros. Exports `ToolGeneratorConfig` (ToolsDir/MaxRetries/CompileTimeout/SafetyMode), `ToolGeneratorResult` (Action/Success/ToolName/LoopResult/Profile), and Execute() parsing actions: generate, refine, list, status. |
| `tool_generator_test.go` | Unit tests for tool generation actions. Tests action parsing, Ouroboros integration, and result formatting. |

## Key Types

### ToolGeneratorConfig
```go
type ToolGeneratorConfig struct {
    ToolsDir       string        // .nerd/tools
    MaxRetries     int           // 3
    CompileTimeout time.Duration // 5 minutes
    SafetyMode     bool          // Extra safety checks
}
```

### ToolGeneratorResult
```go
type ToolGeneratorResult struct {
    Action     string                    // generate, refine, list, status
    Success    bool
    ToolName   string
    Message    string
    LoopResult *autopoiesis.LoopResult
    Profile    *autopoiesis.ToolQualityProfile
    Tools      []*autopoiesis.RuntimeTool
    Learnings  []*autopoiesis.ToolLearning
    Duration   time.Duration
}
```

## Actions

| Action | Description |
|--------|-------------|
| `generate` | Create new tool from natural language spec |
| `refine` | Improve existing tool based on feedback |
| `list` | List all generated tools |
| `status` | Get tool quality profile and metrics |

## Dependencies

- `internal/autopoiesis` - Orchestrator for generation
- `internal/core` - Kernel and VirtualStore
- `internal/types` - ShardConfig, LLMClient

## Testing

```bash
go test ./internal/shards/tool_generator/...
```
