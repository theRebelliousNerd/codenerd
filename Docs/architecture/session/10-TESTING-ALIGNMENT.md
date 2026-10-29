# 10 — Testing Alignment: session

> Last verified: 2026-07-13

## 1. Commands

```powershell
go test ./internal/session/...
go test ./internal/session/ -count=1 -timeout 120s
go test ./tests/e2e/ -run "Session|Executor|Piggyback|Orchestrator|Campaign" -count=1
```

## 2. Unit / package tests by concern

| Concern | Tests (representative) |
|---------|------------------------|
| Constitutional gate | `TestExecutor_CheckSafety_*`, `TestExecutor_EmptyToolCallName`, real-kernel write/glob |
| Payload / boundary | massive payload reject, boundary size, nil args |
| Process loop | SimpleInput, ToolExecution, MaxToolCalls, ToolTimeout |
| Fallbacks | Nil JIT baseline, ConfigFactory fallback, Transducer error |
| Fail closed | SafetyGateFailClosed, kernel assert/query failures |
| Piggyback parse helpers | `TestParseMangleArg(s)` |
| Spawner capacity | MaxLimit, TOCTOU, concurrent max race |
| Spawner lifecycle | Lifecycle, StopAll concurrent, Shutdown concurrent spawn |
| Config fallback | GenerateConfig fallbacks, nil JIT |
| Specialist | `TestLoadSpecialistConfig` |
| SubAgent | Run success, cancel, double kill, compress |
| TaskExecutor | Inline isolation, preset skips perception, async, aliases |
| Compressor | empty, extremes, unprintable, timeout |

Mocks: `mocks_test.go` (+ extras in spawner_gaps_test).

## 3. E2E coverage (downstream)

| Suite | Proves |
|-------|--------|
| session_clean_loop_integration | Full loop wiring |
| session_executor_kernel_integration | Real kernel safety/process |
| session_context_isolation | Isolation properties |
| SessionExecutor_VirtualStore_Kernel | VS + kernel |
| orchestrator_executor_integration | Campaign-style orchestration |
| orchestrator_executor_race_integration | Concurrency |
| campaign_session_integration | Campaign + session |
| piggyback_executor_full_boundary | Piggyback boundary |
| tool_safety_fallback_config | safe_action / config |
| intent_mangle_routing_integration | Intent routing |
| specialist_config_boundary | Specialist path |
| task_executor_async_lifecycle | Async lifecycle |
| cross_boundary_integration | Cross-package |

## 4. Coverage strengths

- Safety gate is heavily unit-tested including adversarial arity / marshal / retract failures.  
- Isolation regressions (history contamination) have explicit tests after CloneForTask introduction.  
- Spawn races explicitly targeted (improvements + gaps tests).

## 5. Coverage gaps

| Gap | Severity |
|-----|----------|
| Piggyback multi-iter tool feedback (feature incomplete) | N/A until feature |
| InteractiveExecutiveGate real Dreamer block e2e | Medium |
| SessionPersister success path integration | Low |
| Ouroboros execute path with real registry | Medium |
| Campaign vs Cortex assembly parity tests | Medium |
| Memory compression quality (semantic) | Low (heuristic) |

## 6. Testing principles for new code

1. Safety changes: unit + at least one real-kernel test when policy matching is involved.  
2. Concurrency changes: race-oriented tests (`-race` when feasible on CI).  
3. Prefer interface mocks for LLM; do not require live API keys in unit tests.  
4. When adding spawn entry points, test max-capacity and cancel-stop.  
5. Process path tests should cover empty input, cancelled ctx, and nil optional deps.

## 7. Alignment with north star

Tests that assert **fail-closed**, **preset intent skips perception**, and **isolation** are the highest-value north-star tests. Prefer extending those over snapshot-style text matching of LLM baselines.
