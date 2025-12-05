# internal/autopoiesis - Self-Modification Capabilities

This package implements autopoiesis (self-creation) - the ability for codeNERD to modify itself by detecting needs and generating new capabilities.

## Architecture

Autopoiesis provides three core capabilities:
1. **Complexity Analysis** - Detect when campaigns are needed
2. **Tool Generation** - Create new tools when capabilities are missing
3. **Persistence Analysis** - Identify when persistent agents are needed

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `autopoiesis.go` | ~400 | Main orchestrator |
| `complexity.go` | ~300 | Task complexity analysis |
| `toolgen.go` | ~450 | LLM-based tool generation |
| `persistence.go` | ~400 | Persistent agent detection |

## Key Types

### Orchestrator
```go
type Orchestrator struct {
    complexity  *ComplexityAnalyzer
    toolGen     *ToolGenerator
    persistence *PersistenceAnalyzer
}

func (o *Orchestrator) QuickAnalyze(ctx, input, target) QuickResult
func (o *Orchestrator) Analyze(ctx, input, target) (*AnalysisResult, error)
```

### ComplexityResult
```go
type ComplexityResult struct {
    Level           ComplexityLevel  // Simple → Epic
    Score           float64          // 0.0-1.0
    NeedsCampaign   bool
    NeedsPersistent bool
    SuggestedPhases []string
    EstimatedFiles  int
}
```

### ComplexityLevel
| Level | Description |
|-------|-------------|
| ComplexitySimple | Single action, single file |
| ComplexityModerate | Multiple files, one phase |
| ComplexityComplex | Multiple phases, dependencies |
| ComplexityEpic | Full feature, multiple components |

### ToolNeed
```go
type ToolNeed struct {
    Name       string
    Purpose    string
    InputType  string
    OutputType string
    Confidence float64
}
```

### PersistenceNeed
```go
type PersistenceNeed struct {
    AgentType       string
    Purpose         string
    LearningGoals   []string
    MonitoringScope string
    Schedule        string
    Confidence      float64
}
```

## Detection Patterns

### Complexity Indicators
- Epic: "implement full system", "build complete feature"
- Complex: "refactor entire", "multi-phase migration"
- Moderate: "update all files", "rename across"

### Persistence Indicators
- Learning: "remember my preferences", "always use"
- Monitoring: "watch for changes", "alert me when"
- Triggers: "on every commit", "whenever I push"

### Tool Need Indicators
- "can't you do X", "is there a way to"
- "I need a tool for", "how can I"

## Tool Generation

The ToolGenerator can:
1. Detect missing capabilities from user input
2. Generate Go code using LLM
3. Validate generated code
4. Write to `.nerd/tools/`
5. Register for runtime use

## Integration Points

- `processInput` in chat.go calls `QuickAnalyze`
- Warns user about complex tasks needing campaigns
- Recommends persistent agents when detected
- Triggers tool generation on `/generate_tool` action

## Dependencies

- `internal/perception` - LLMClient for analysis

## Testing

```bash
go test ./internal/autopoiesis/...
```
