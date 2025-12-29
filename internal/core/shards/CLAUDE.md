# internal/core/shards - Legacy Shard Infrastructure

**Status:** LEGACY - Will be phased out (December 2024)
**Current Architecture:** See [internal/session/](../../session/CLAUDE.md) for Session Executor

---

## ⚠️ Major Architectural Change

As of **December 2024**, the `ShardManager` (`internal/core/shard_manager.go`) has been **removed** and replaced by the **Session Executor** in `internal/session/`.

This directory contains legacy shard infrastructure that may still be used for backward compatibility. New code should use:
- **Session Executor** (`internal/session/executor.go`) - Universal execution loop
- **Spawner** (`internal/session/spawner.go`) - Dynamic subagent creation
- **SubAgent** (`internal/session/subagent.go`) - Execution context

**Related Packages:**
- [internal/session](../../session/CLAUDE.md) - **NEW:** Session-based execution (replaces ShardManager)
- [internal/core](../CLAUDE.md) - Kernel and VirtualStore integration
- [internal/shards](../../shards/CLAUDE.md) - **DEPRECATED:** Legacy shard implementations
- [internal/types](../../types/CLAUDE.md) - ShardAgent interface (legacy)

## Architecture (LEGACY)

This subpackage contains the core shard lifecycle management:
- **ShardManager**: Orchestrates all shard agents
- **SpawnQueue**: Backpressure-aware prioritized spawning
- **BaseShardAgent**: Common functionality for all shards
- **Config Helpers**: Default configurations by shard type

## File Index

| File | Description |
|------|-------------|
| `manager.go` | Core `ShardManager` struct orchestrating all shard agents. Exports `ShardManager` with factories, profiles, disabled set, kernel/llmClient/virtualStore dependencies, SpawnQueue for backpressure, and JIT registrar callbacks. |
| `spawn_queue.go` | Prioritized backpressure-aware shard spawning. Exports `SpawnQueue`, `SpawnQueueConfig` with max queue sizes and worker count, `BackpressureStatus`, and errors (ErrQueueFull, ErrQueueTimeout, ErrQueueStopped). |
| `agents.go` | `BaseShardAgent` providing common shard functionality. Exports `BaseShardAgent` with id/config/state management, kernel/llmClient dependencies, GetID/GetState/SetState/GetConfig/Stop methods. |
| `config.go` | Default shard configurations by type. Exports `DefaultGeneralistConfig()` for Type A ephemeral, `DefaultSpecialistConfig()` for Type B persistent with knowledge path, and `DefaultSystemConfig()` for Type S system shards. |

## Key Types

### ShardManager
```go
type ShardManager struct {
    shards        map[string]types.ShardAgent
    results       map[string]types.ShardResult
    profiles      map[string]types.ShardConfig
    factories     map[string]types.ShardFactory
    kernel        types.Kernel
    llmClient     types.LLMClient
    virtualStore  any
    learningStore types.LearningStore
    spawnQueue    *SpawnQueue
    jitRegistrar  types.JITDBRegistrar
}
```

### SpawnQueueConfig
```go
type SpawnQueueConfig struct {
    MaxQueueSize        int           // 100 default
    MaxQueuePerPriority int           // 30 default
    DefaultTimeout      time.Duration // 5 minutes
    HighWaterMark       float64       // 0.7
    WorkerCount         int           // 2
    DrainTimeout        time.Duration // 30 seconds
}
```

## Shard Type Configurations

| Type | Timeout | Permissions |
|------|---------|-------------|
| Generalist (A) | 15 min | read_file, write_file, network |
| Specialist (B) | 30 min | read_file, write_file, network, browser, research |
| System (S) | 24 hours | read_file, write_file, exec_cmd, network |

## Dependencies

- `internal/types` - ShardAgent, ShardConfig, ShardFactory
- `internal/logging` - Structured logging
- `internal/usage` - Usage tracking

## Testing

```bash
go test ./internal/core/shards/...
```

---

**Remember: Push to GitHub regularly!**
