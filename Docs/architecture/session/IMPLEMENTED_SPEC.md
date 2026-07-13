# session — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/session/` (complete internal coverage)
> **Implementation: `internal/session/` — 6 non-test .go, 14 tests, 0 .mg**


## 1. Purpose

Session execution loop and clean executor

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/session/` | Primary implementation |
| `Docs/architecture/session/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (6 src / 14 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/session/executor.go` | 1057 | source |
| `internal/session/executor_tools.go` | 587 | source |
| `internal/session/spawner.go` | 516 | source |
| `internal/session/subagent.go` | 470 | source |
| `internal/session/task_executor.go` | 415 | source |
| `internal/session/semantic_compressor.go` | 104 | source |

### Types (sampled)

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

### Functions (sampled)

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

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
