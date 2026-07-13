# 06 — Public API and Types: session

> Last verified: 2026-07-13  
> Package: `codenerd/internal/session`

## 1. Interfaces

### JITCompiler (`executor.go`)

```go
type JITCompiler interface {
    Compile(ctx context.Context, compilationCtx *prompt.CompilationContext) (*prompt.CompilationResult, error)
}
```

Implemented by prompt package compilers; mocked in tests.

### ConfigFactory (`executor.go`)

```go
type ConfigFactory interface {
    Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error)
}
```

Production: `prompt.NewDefaultConfigFactory()` in Cortex boot.

### SessionPersister (`executor.go`)

```go
type SessionPersister interface {
    StoreSessionTurn(sessionID string, turnNumber int, userInput, intentJSON, response, atomsJSON string) error
    StoreCompressedState(sessionID string, turnNumber int, stateJSON string, ratio float64) error
}
```

Cortex may pass local DB when available.

### InteractiveExecutiveGate (`executor.go`)

```go
type InteractiveExecutiveGate interface {
    PreflightDestructiveToolCall(ctx context.Context, actionID, toolName string, args map[string]any) error
    ValidateInteractiveToolResult(ctx context.Context, actionID, toolName string, args map[string]any, output string, success bool) error
}
```

Implemented by `*core.VirtualStore`.

### Compressor (`subagent.go`)

```go
type Compressor interface {
    Compress(ctx context.Context, turns []perception.ConversationTurn) (string, error)
}
```

### TaskExecutor (`task_executor.go`)

```go
type TaskExecutor interface {
    Execute(ctx context.Context, req TaskRequest) (string, error)
    ExecuteWithContext(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error)
    ExecuteAsync(ctx context.Context, req TaskRequest) (taskID string, err error)
    GetResult(taskID string) (result string, done bool, err error)
    WaitForResult(ctx context.Context, taskID string) (string, error)
}
```

`JITExecutor` also exposes `SpawnConsultation` for consultation manager.

## 2. Core structs

| Type | File | Purpose |
|------|------|---------|
| `Executor` | executor.go | Clean loop runner |
| `ExecutorConfig` | executor.go | Budgets, timeouts, gate, token budget |
| `ExecutionResult` | executor.go | Process output |
| `MangleAtom` | executor.go | Thin string wrapper (legacy helper) |
| `ToolCall` | executor_tools.go | Local tool invocation |
| `Spawner` | spawner.go | Subagent registry |
| `SpawnerConfig` | spawner.go | Max active, token budget |
| `SpawnRequest` | spawner.go | Spawn parameters |
| `SubAgent` | subagent.go | Isolated worker |
| `SubAgentConfig` | subagent.go | ID, name, type, intent, timeout |
| `SubAgentMetrics` | subagent.go | Duration, turns, state |
| `SubAgentState` | subagent.go | idle/running/completed/failed |
| `SubAgentType` | subagent.go | ephemeral/persistent/system |
| `TaskRequest` | task_executor.go | IntentVerb, Persona, Task, ConfigRef |
| `TaskResult` | task_executor.go | Async result cache entry |
| `JITExecutor` | task_executor.go | TaskExecutor impl |
| `SemanticCompressor` | semantic_compressor.go | LLM compressor |

## 3. Constants / defaults

| Symbol | Value |
|--------|-------|
| `DefaultTokenBudget` | 65536 |
| `SubAgentTypeEphemeral` | `"ephemeral"` |
| `SubAgentTypePersistent` | `"persistent"` |
| `SubAgentTypeSystem` | `"system"` |
| maxPayloadBytes | 100 * 1024 (unexported) |
| maxSpecialistConfigSize | 1 << 20 (unexported) |

## 4. Constructors and mutators

### Executor

| Func | Notes |
|------|-------|
| `NewExecutor(kernel, vs, llm, jit, factory, transducer)` | Core wire-up |
| `DefaultExecutorConfig()` | Defaults |
| `SetSessionContext` / `SetConfig` / `SetAgentConfig` | Config |
| `SetHistory` / `GetHistory` / `ClearHistory` | Conversation |
| `SetOuroborosRegistry` | Dual tools |
| `SetSessionPersister` / `SetSessionID` | Persistence |
| `CloneForTask()` | Isolated copy |
| `Process` / `ProcessWithIntent` | Main loop |

### Spawner

| Func | Notes |
|------|-------|
| `NewSpawner(..., SpawnerConfig)` | |
| `DefaultSpawnerConfig()` | max 10 |
| `Spawn` / `SpawnForIntent` / `SpawnSpecialist` | Create |
| `Get` / `GetByName` / `ListActive` | Query |
| `Stop` / `StopAll` / `Cleanup` | Lifecycle |
| `GetMetrics` | Observability |

### SubAgent

| Func | Notes |
|------|-------|
| `NewSubAgent(cfg, deps...)` | Builds inner Executor |
| `DefaultSubAgentConfig(name)` | ID generation |
| `Run` / `Stop` / `Wait` / `WaitWithContext` | Lifecycle |
| `GetID` / `GetName` / `GetState` / `GetResult` / `GetMetrics` | Status |
| `CompressMemory` / `SetCompressor` | Memory |

### Task / compress

| Func | Notes |
|------|-------|
| `NewJITExecutor(executor, spawner, transducer)` | |
| `NewSemanticCompressor(client)` | |

## 5. Important unexported helpers (behavior contracts)

| Helper | Contract |
|--------|----------|
| `presetIntentForTask` | nil if no slash verb; Confidence 1.0 |
| `normalizeTaskIntentVerb` | shard aliases → canonical verbs |
| `categoryForIntentVerb` | mutation vs query default |
| `checkSafety` | constitutional algorithm |
| `intentRequiresToolCall` | kernel query; false if unavailable |
| `capabilityHintForAgentName` | model capability multiplexing |

## 6. Import expectations for consumers

Consumers should depend on:

- `TaskExecutor` for orchestration  
- `Executor.Process` only for the primary interactive session  
- Not construct SubAgents bypassing Spawner capacity unless tests require it
