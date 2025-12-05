# internal/core - System Kernel & Shard Management

This package implements the core runtime components of codeNERD: the Mangle Kernel and ShardManager.

## Architecture

The core package provides:
1. **RealKernel** - The Mangle/Datalog inference engine
2. **ShardManager** - Lifecycle management for ShardAgents
3. **ShadowMode** - Simulation before execution
4. **TDD Loop** - Test-driven development automation

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `virtual_store.go` | ~1100 | In-memory fact store for kernel |
| `shard_manager.go` | ~950 | Shard spawning, lifecycle, profiles |
| `tdd_loop.go` | ~750 | TDD automation (Red→Green→Refactor) |
| `kernel.go` | ~500 | RealKernel implementation |
| `shadow_mode.go` | ~300 | Action simulation |

## Key Types

### RealKernel
```go
type RealKernel struct {
    engine *mangle.Engine
    store  *VirtualStore
}

func (k *RealKernel) LoadFacts(facts []Fact) error
func (k *RealKernel) Query(predicate string) ([]Fact, error)
```

### ShardManager
```go
type ShardManager struct {
    profiles map[string]ShardConfig
    active   map[string]ShardInterface
}

func (sm *ShardManager) Spawn(ctx context.Context, shardType, task string) (string, error)
func (sm *ShardManager) DefineProfile(name string, config ShardConfig) error
```

### ShardConfig
```go
type ShardConfig struct {
    Name          string
    KnowledgePath string
    SystemPrompt  string
    Tools         []string
}
```

### Fact
```go
type Fact struct {
    Predicate string
    Args      []interface{}
}
```

## Shard Types

| Constant | Value | Description |
|----------|-------|-------------|
| ShardTypeSystem | 1 | Always-on core shards |
| ShardTypeEphemeral | 2 | Spawn-execute-die |
| ShardTypePersistent | 3 | LLM-created, survives sessions |
| ShardTypeUser | 4 | User-defined specialists |

## Shadow Mode

Simulates actions before execution:
```go
shadowMode.Simulate(action, target) → SimulationResult
```

Returns predicted:
- Files affected
- Risk level
- Side effects
- Rollback capability

## TDD Loop

Automated test-driven development:
1. **Red**: Generate failing test
2. **Green**: Implement to pass
3. **Refactor**: Clean up implementation
4. Loop until all tests pass

## Dependencies

- `internal/mangle` - Datalog engine
- `internal/shards` - Shard implementations
- `internal/store` - Persistence

## Testing

```bash
go test ./internal/core/...
```
