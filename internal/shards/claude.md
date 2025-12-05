# internal/shards - Specialist ShardAgents

This package implements the specialist ShardAgents that perform domain-specific tasks in the codeNERD system.

## Architecture

ShardAgents are specialized LLM-based agents that can be spawned to handle specific tasks. They follow the Piggyback Protocol for communication and can learn from interactions.

### Shard Types

| Type | Lifecycle | Description |
|------|-----------|-------------|
| Type 1 | System/Always-On | Core system shards (kernel, etc.) |
| Type 2 | Ephemeral | Spawn, execute task, die |
| Type 3 | Persistent | LLM-created, survives sessions |
| Type 4 | User-Defined | Custom specialists |

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `researcher.go` | ~2000 | ResearcherShard - Documentation research, codebase exploration |
| `reviewer.go` | ~1500 | ReviewerShard - Code review, security scanning |
| `research_tools.go` | ~1400 | Research tooling, web fetching, knowledge extraction |
| `tester.go` | ~1300 | TesterShard - Test generation and execution |
| `coder.go` | ~1000 | CoderShard - Code generation, refactoring |

## Key Types

### ShardInterface
All shards implement this interface:
```go
type ShardInterface interface {
    Execute(ctx context.Context, task string) (string, error)
    GetType() ShardType
    GetName() string
}
```

### ResearcherShard
Handles research tasks:
- Documentation lookup
- Codebase exploration
- Knowledge atom extraction
- Web research integration

### ReviewerShard
Handles code review:
- Full code review
- Security vulnerability scanning
- Complexity analysis
- Style/lint checking

### TesterShard
Handles testing:
- Test generation
- Test execution
- Coverage analysis
- TDD loop support

### CoderShard
Handles code changes:
- Bug fixing
- Refactoring
- Feature implementation
- Code generation

## Dependencies

- `internal/perception` - LLMClient for shard execution
- `internal/core` - ShardManager for lifecycle management
- `internal/store` - LearningStore for shard memory

## Adding New Shards

1. Create `newshard.go` implementing ShardInterface
2. Add shard type constant
3. Register in ShardManager factory
4. Add to VerbCorpus in perception/transducer.go for NL routing

## Testing

```bash
go test ./internal/shards/...
```
