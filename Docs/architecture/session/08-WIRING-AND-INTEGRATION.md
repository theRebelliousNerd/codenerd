# 08 — Wiring and Integration: session

> Last verified: 2026-07-13

## 1. Cortex boot path (primary)

Location: `internal/system/factory.go` → `initFinalExecutors`.

```
BootCortexWithConfig
  → … kernel, VS, perception, JIT …
  → initFinalExecutors
       sessionKernel  := sessionKernelAdapter{kernel}
       sessionVS      := sessionVirtualStoreAdapter{vs}
       sessionLLM     := sessionLLMAdapter{workerOrMainLLM}
       configFactory  := prompt.NewDefaultConfigFactory()
       sessionExecutor := session.NewExecutor(...)
       if localDB != nil { SetSessionPersister(localDB) }
       sessionSpawner  := session.NewSpawner(..., DefaultSpawnerConfig())
       taskExecutor    := session.NewJITExecutor(executor, spawner, transducer)
       virtualStore.SetTaskExecutor(&taskDelegatorAdapter{executor: taskExecutor})
  → Cortex{ TaskExecutor, SessionExecutor, SessionSpawner, ... }
```

Notes:

- Task LLM may be `shardLLMClient` (worker) when configured, not the TUI main client.  
- Log: `JITExecutor wired in BootCortex` on CategorySession.  
- Token budget currently DefaultSpawnerConfig unless higher layer mutates SetConfig (verify callers when changing).

## 2. Cortex task API

`Cortex` methods (same file) route non-special cases through `TaskExecutor.Execute` / `ExecuteWithContext` with `session.TaskRequest{IntentVerb, Task}`.

Nil TaskExecutor → error path (must be wired).

## 3. VirtualStore task delegation

`SetTaskExecutor` on VirtualStore receives `taskDelegatorAdapter` so RouteAction / internal delegation can call session tasks without importing session cycles incorrectly.

## 4. Chat TUI wiring

| File | Role |
|------|------|
| `cmd/nerd/chat/session_boot.go` | Boots system; session types available on model |
| `cmd/nerd/chat/session_boot_helpers.go` | Helpers around session components |
| `cmd/nerd/chat/session_adapters.go` | Adapters |
| `cmd/nerd/chat/delegation.go` | Migration notes ShardManager → TaskExecutor |
| `cmd/nerd/chat/delegation_routing.go` | `shardTypeToTaskRequest` → `session.TaskRequest` |
| `cmd/nerd/chat/model_types.go` | Holds session-related fields |

Interactive user turns may still use broader chat process pipelines; delegated coding work funnels TaskExecutor.

## 5. Campaign wiring (secondary assembly)

`cmd/nerd/cmd_campaign.go` constructs a **local** session stack:

```
session.NewExecutor(...)
session.NewSpawner(..., DefaultSpawnerConfig())
session.NewJITExecutor(...)
virtualStore.SetTaskExecutor(campaignTaskDelegatorAdapter)
shards.NewConsultationManager(campaignTaskExecutorConsultationSpawner)
```

Also used when building orchestrator configs with `TaskExecutor` field.

**Risk:** campaign path may drift from Cortex `initFinalExecutors` (persister, ouroboros registry, token budgets). Audit both when changing constructor signatures.

## 6. Consultation spawn

`JITExecutor.SpawnConsultation(ctx, specialistName, task)` builds:

```go
TaskRequest{ IntentVerb: "/consult/" + specialistName, Task: task }
```

and async-spawns via internal helper. Consultation manager depends on this seam.

## 7. Interactive executive gate wiring

Session does **not** register the gate. It type-asserts:

```go
if gate, ok := e.virtualStore.(InteractiveExecutiveGate); ok && gate != nil { ... }
```

Production requires `*core.VirtualStore` (or adapter exposing methods) on the executor’s `virtualStore` field. If chat/system adapters wrap VS without forwarding Preflight/Validate, the gate silently no-ops — **wiring gap to audit** when changing adapters.

## 8. Ouroboros registry

`SetOuroborosRegistry` must be called by a higher layer after tools are generated/registered. Session only stores and uses it. If unset, modular tools still work; generated tools absent from catalog.

## 9. Perception learning hook

When `perception.SharedTaxonomy != nil`, each Process queues a `ReasoningTrace`. Wiring of SharedTaxonomy is outside session.

## 10. Verification package

`internal/verification/verifier.go` imports session — treat as secondary consumer; confirm expectations when changing ExecutionResult or TaskExecutor.

## 11. E2E wiring proofs

| Test area | Path pattern |
|-----------|--------------|
| Clean loop | `tests/e2e/session_clean_loop_integration_test.go` |
| Kernel | `session_executor_kernel_integration_test.go` |
| Context isolation | `session_context_isolation_test.go` |
| VS+Kernel | `SessionExecutor_VirtualStore_Kernel_integration_test.go` |
| Orchestrator | `orchestrator_executor_*` |
| Campaign | `campaign_session_integration_test.go` |
| Piggyback | `piggyback_executor_full_boundary_test.go` |
| Safety fallback | `tool_safety_fallback_config_test.go` |

## 12. Integration checklist for new features

1. Who constructs Executor? (Cortex / campaign / test)  
2. Is TaskExecutor required on Cortex?  
3. Does VS adapter implement InteractiveExecutiveGate?  
4. Is Ouroboros registry set when generation is on?  
5. Are e2e tests updated for new spawn/process semantics?  
