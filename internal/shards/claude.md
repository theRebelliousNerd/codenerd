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

## File Index

| File | Description |
|------|-------------|
| `registration.go` | Provides `RegisterAllShards()` and `RegistryContext` for shard dependency injection. Solves the "hollow shard" problem by ensuring factories have kernel and LLM at instantiation. |
| `requirements_interrogator.go` | Socratic shard for early-phase clarification before campaigns. Exports `RequirementsInterrogatorShard` that generates clarifying questions for ambiguous tasks. |

### Subdirectory Shards

| Subdirectory | Purpose |
|--------------|---------|
| `coder/` | CoderShard - Code generation, refactoring, bug fixing with transaction rollback |
| `nemesis/` | NemesisShard - Adversarial patch analysis, attack generation, armory persistence |
| `researcher/` | ResearcherShard - Documentation research, web fetching, knowledge atom extraction |
| `reviewer/` | ReviewerShard - Code review, security scanning, hypothesis verification |
| `system/` | System shards - Constitution gate, executive policy, router, legislator, world model |
| `tester/` | TesterShard - Test generation, execution, coverage analysis, TDD loop support |
| `tool_generator/` | ToolGenerator - Ouroboros-driven tool generation from capability gaps |

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
