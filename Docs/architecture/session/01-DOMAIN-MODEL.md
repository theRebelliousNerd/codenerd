# session — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/session/` (6 non-test .go, 14 tests, 0 .mg)**


## Source package

`internal/session/`

## Exported / primary types (sampled)

| Type | Location |
|------|----------|
| `JITCompiler` | `internal/session/executor.go:36` |
| `ConfigFactory` | `internal/session/executor.go:41` |
| `SessionPersister` | `internal/session/executor.go:46` |
| `InteractiveExecutiveGate` | `internal/session/executor.go:62` |
| `MangleAtom` | `internal/session/executor.go:74` |
| `Executor` | `internal/session/executor.go:89` |
| `ExecutorConfig` | `internal/session/executor.go:125` |
| `ExecutionResult` | `internal/session/executor.go:268` |
| `ToolCall` | `internal/session/executor_tools.go:20` |
| `SemanticCompressor` | `internal/session/semantic_compressor.go:14` |
| `Spawner` | `internal/session/spawner.go:28` |
| `SpawnerConfig` | `internal/session/spawner.go:51` |
| `SpawnRequest` | `internal/session/spawner.go:99` |
| `SubAgentState` | `internal/session/subagent.go:18` |
| `SubAgentType` | `internal/session/subagent.go:43` |
| `Compressor` | `internal/session/subagent.go:47` |
| `SubAgentConfig` | `internal/session/subagent.go:63` |
| `SubAgent` | `internal/session/subagent.go:116` |
| `SubAgentMetrics` | `internal/session/subagent.go:368` |
| `TaskRequest` | `internal/session/task_executor.go:17` |
| `TaskExecutor` | `internal/session/task_executor.go:33` |
| `TaskResult` | `internal/session/task_executor.go:54` |
| `JITExecutor` | `internal/session/task_executor.go:137` |

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 0 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| — | 0 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **Clean execution loop / session executor**

## Data & control concepts

- Primary language surface: Go under `internal/session/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
