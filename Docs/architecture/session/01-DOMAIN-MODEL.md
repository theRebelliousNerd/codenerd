# session — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/session/` (complete internal coverage)
> **Implementation: `internal/session/` — 6 non-test .go, 14 tests, 0 .mg**


## Package

`internal/session/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `String` | `internal/session/executor.go:76` |
| `DefaultExecutorConfig` | `internal/session/executor.go:159` |
| `NewExecutor` | `internal/session/executor.go:170` |
| `SetSessionContext` | `internal/session/executor.go:193` |
| `CloneForTask` | `internal/session/executor.go:208` |
| `SetConfig` | `internal/session/executor.go:222` |
| `SetAgentConfig` | `internal/session/executor.go:229` |
| `SetHistory` | `internal/session/executor.go:236` |
| `SetOuroborosRegistry` | `internal/session/executor.go:245` |
| `SetSessionPersister` | `internal/session/executor.go:254` |
| `SetSessionID` | `internal/session/executor.go:261` |
| `Process` | `internal/session/executor.go:294` |
| `ProcessWithIntent` | `internal/session/executor.go:314` |
| `ClearHistory` | `internal/session/executor.go:899` |
| `GetHistory` | `internal/session/executor.go:906` |
| `NewSemanticCompressor` | `internal/session/semantic_compressor.go:19` |
| `Compress` | `internal/session/semantic_compressor.go:26` |
| `DefaultSpawnerConfig` | `internal/session/spawner.go:62` |
| `NewSpawner` | `internal/session/spawner.go:70` |
| `Spawn` | `internal/session/spawner.go:121` |
| `SpawnForIntent` | `internal/session/spawner.go:198` |
| `SpawnSpecialist` | `internal/session/spawner.go:214` |
| `Get` | `internal/session/spawner.go:261` |
| `GetByName` | `internal/session/spawner.go:269` |
| `Stop` | `internal/session/spawner.go:286` |
| `StopAll` | `internal/session/spawner.go:299` |
| `Cleanup` | `internal/session/spawner.go:315` |
| `ListActive` | `internal/session/spawner.go:336` |
| `GetMetrics` | `internal/session/spawner.go:350` |
| `String` | `internal/session/subagent.go:27` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Session execution loop and clean executor**
