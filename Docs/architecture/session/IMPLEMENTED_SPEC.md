# session — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/session/` (6 non-test .go, 14 tests, 0 .mg)**


## 1. Purpose

Clean execution loop / session executor

## 2. Source paths

| Path | Role |
|------|------|
| `internal/session/` | Primary implementation |
| `Docs/architecture/session/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global | **n/a** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 90% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

| Path | Lines |
|------|------:|
| `internal/session/executor.go` | 1057 | source |
| `internal/session/executor_tools.go` | 587 | source |
| `internal/session/spawner.go` | 516 | source |
| `internal/session/subagent.go` | 470 | source |
| `internal/session/task_executor.go` | 415 | source |
| `internal/session/semantic_compressor.go` | 104 | source |

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
