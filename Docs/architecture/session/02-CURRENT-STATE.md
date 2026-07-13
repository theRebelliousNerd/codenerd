# 02 — Current State: session

> Last verified: 2026-07-13  
> Source root: `internal/session/`

## 1. Inventory summary

| Class | Count | Notes |
|-------|------:|-------|
| Non-test `.go` | 6 | executor, executor_tools, spawner, subagent, task_executor, semantic_compressor |
| Test `.go` | 14 | including mocks_test.go |
| Package `.md` | 1 | `internal/session/README.md` |
| Local `.mg` | 0 | uses global policy corpus |
| Approx non-test LOC | ~3149 | executor pair ~1644 |

## 2. File roles

| File | Role | Hotspots |
|------|------|----------|
| `executor.go` | Clean loop, interfaces, Piggyback, history, persist | `ProcessWithIntent`, `generateResponse*`, `processMangleUpdatesFromEnvelope`, `processPiggybackControlPacket` |
| `executor_tools.go` | Tool loop + safety | `runToolLoop`, `checkSafety`, `executeToolCall` |
| `spawner.go` | Registry + JIT config for workers | `Spawn` pending reservation, `loadSpecialistConfig` |
| `subagent.go` | Isolated runner | `Run`/`execute`, `CompressMemory` |
| `task_executor.go` | Unified task API | `ExecuteWithContext`, async TOCTOU fix |
| `semantic_compressor.go` | LLM summarizer | `Compress` |

## 3. Exported surface (summary)

See [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) for full tables.

Primary constructors:

- `NewExecutor`  
- `NewSpawner`  
- `NewSubAgent`  
- `NewJITExecutor`  
- `NewSemanticCompressor`  
- `DefaultExecutorConfig` / `DefaultSpawnerConfig` / `DefaultSubAgentConfig`

Primary interfaces:

- `TaskExecutor`  
- `JITCompiler`, `ConfigFactory`, `SessionPersister`, `InteractiveExecutiveGate`  
- `Compressor`

## 4. Runtime defaults (live values)

| Knob | Default | Defined |
|------|---------|---------|
| MaxToolCalls | 50 | `DefaultExecutorConfig` |
| MaxToolIterations | 8 | same |
| ToolTimeout | 5m | same |
| EnableSafetyGate | true | same |
| TokenBudget | 65536 | `DefaultTokenBudget` |
| Executor history max | 50 | `appendToHistory` |
| maxPayloadBytes | 100 KiB | safety |
| Tool result truncate | 16 KiB | `truncateToolResult` |
| MaxActiveSubagents | 10 | `DefaultSpawnerConfig` |
| SubAgent timeout | 30m (default cfg) / LLM timeout from appconfig when spawned | |
| Compress threshold | 10 | SubAgent execute |
| Specialist config max | 1 MiB | `maxSpecialistConfigSize` |

## 5. Consumer topology (living)

```
internal/system.factory  ──builds──► Executor, Spawner, JITExecutor
        │                              │
        └── Cortex.TaskExecutor ───────┤
                                       │
cmd/nerd/chat (delegation) ────────────┤
cmd/nerd/cmd_campaign.go ──rebuilds────┤
internal/campaign orchestrator ────────┤
internal/verification ─────────────────┘
tests/e2e/*  (integration)
```

## 6. Package README drift

`internal/session/README.md` still markets “No shards. No spawn. No factories” and December 2024 dating. **Code reality (2026-07-13):** Spawner, ConfigFactory, and TaskExecutor are first-class. Prefer this corpus for architecture truth; package README is historical orientation.

## 7. Mangle surface (external)

No `.mg` files in package. Runtime queries/asserts predicates listed in [IMPLEMENTED_SPEC.md §6.3](IMPLEMENTED_SPEC.md) and [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md).

## 8. Completeness heuristic

| Area | Heuristic |
|------|-----------|
| Core Process loop | ~95% |
| Native tool multi-turn | ~90% |
| Piggyback multi-turn | ~50% |
| Safety gate | ~98% (exact permission + payload/capability boundaries tested) |
| Spawn/lifecycle | ~90% |
| Persistence | ~40% |
| Overall package | **~90%** living production |

## 9. Verified 2026-07-13 boundary repairs

- `internal/session/executor_tools.go#Executor.checkSafety` authorizes only an
  exact `permitted(Action, Target, Payload)`; `safe_action/1` cannot authorize.
- Nil or empty `EffectiveAgentRuntimeConfig.AllowedTools` exposes zero tools, and
  Ouroboros registry membership does not grant a capability.
- `internal/session/spawner.go#loadSpecialistConfig` runs the runtime config
  validator after strict path, size, and YAML checks.
- Integration-tagged fallback/config tests now assert fail-closed behavior rather
  than documenting the former ambient-capability bug.

Focused package, race, vet, and integration-tagged gates passed; see
[_progress](_progress.md) for the receipt and [TODO](TODO.md) for authoritative
feature status.
