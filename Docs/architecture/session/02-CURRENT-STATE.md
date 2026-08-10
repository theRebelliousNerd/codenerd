# 02 — Current State: session

> Last verified: 2026-08-09
> Source root: `internal/session/`

## 1. Inventory summary

| Class | Count | Notes |
|-------|------:|-------|
| Non-test `.go` | 14 | executor/tool loop, verification gates, spawner/subagent/task runtime, summaries |
| Test `.go` | 33 | including mocks and live-defect regression suites |
| Package `.md` | 2 | `internal/session/README.md`, scoped `agents.md` guidance |
| Local `.mg` | 0 | uses global policy corpus |
| Approx non-test LOC | ~5700 | package inventory measured 2026-08-09 |

## 2. File roles

| File | Role | Hotspots |
|------|------|----------|
| `executor.go` | Clean loop, interfaces, Piggyback, history, persist | `ProcessWithIntent`, `generateResponse*`, `processMangleUpdatesFromEnvelope`, `processPiggybackControlPacket` |
| `executor_tools.go` | Tool loop + safety | `runToolLoop`, `checkSafety`, `executeToolCall` |
| `build_verify.go`, `test_verify.go`, `coverage_profile.go` | Post-edit build/test/coverage gates | verify, repair, bounded subprocesses |
| `critic.go`, `lsp_diagnostics.go`, `severity.go` | Advisory semantic and diagnostic gates | critic uplift, gopls findings |
| `gate_names.go` | Stable verification gate names | gate identity |
| `spawner.go` | Registry + JIT config for workers | `Spawn` pending reservation, `loadSpecialistConfig` |
| `subagent.go` | Isolated runner | `Run`/`execute`, `CompressMemory` |
| `task_executor.go` | Unified task API | `ExecuteWithContext`, async TOCTOU fix |
| `semantic_compressor.go` | LLM summarizer | `Compress` |
| `turn_summary.go` | Compact execution summaries | result evidence |

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
| MaxToolIterations | 8 base | same |
| AdaptiveToolBudget | true | same |
| ToolIterationExtensionSize | 8 | same |
| MaxToolIterationExtensions | 2 | same |
| ToolLoopRepeatThreshold | 2 identical trace cycles | same |
| ToolTimeout | 5m | same |
| FinalAnswerReserve | 5m (half of remainder for short turns) | same |
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

## 10. Verified 2026-08-09 live-review repairs

- Deadline-bound tool loops reserve a conclusion window and preserve provider
  tool-use/result pairing without replaying effects.
- Native and Piggyback batches share one execution, budget, cancellation, and
  accounting implementation. Both transports also run the build, test, and
  advisory critic gates after durable Go writes; Piggyback hard-gate failures
  fail explicitly because that protocol has no native repair round.
- `SetConfig` and every production config read use the executor mutex and a
  coherent snapshot; the session race suite covers concurrent replacement.
- `write_oriented_intent/1` is policy-owned. A real-kernel parity test pins the
  conservative Go fallback used during partial initialization.
- Write protection fails closed on missing targets, query failures, and a
  missing policy authority at executor, VirtualStore, and registry layers.
