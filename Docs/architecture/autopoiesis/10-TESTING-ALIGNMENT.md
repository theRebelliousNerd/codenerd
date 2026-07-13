# 10 — Testing Alignment: Autopoiesis

> Last verified against codebase: **2026-07-13**

## 1. Commands

```powershell
go test ./internal/autopoiesis/...
go test ./internal/autopoiesis/prompt_evolution/...
go test ./tests/e2e/ -run Autopoiesis
go test ./cmd/nerd/ui/ -run Autopoiesis
go test ./cmd/nerd/chat/ -run Autopoiesis
```

Optional benches: `traces_bench_test.go`, `traces_benchmark_test.go`, `prompt_evolution/classifier_bench_test.go`.

## 2. Package test inventory (representative)

| File | Focus |
|------|-------|
| `complexity_test.go` | levels, patterns, LLM vs heuristic |
| `analysis_heuristics_test.go` | campaign/persistence triggers |
| `checker_test.go` | SafetyChecker |
| `ouroboros_test.go` | loop construction, stages, happy path |
| `ouroboros_panic_test.go` | panic persistence |
| `ouroboros_tool_test.go` / `ouroboros_wrapper_test.go` | tool paths |
| `delegation_test.go` | kernel delegations success/fail |
| `orchestrator_test.go` | NewOrchestrator, sync, should generate, learnings refresh |
| `feedback_test.go` | LearningStore, races, NaN, refine |
| `autopoiesis_agents_test.go` | agent lifecycle |
| `autopoiesis_profiles_test.go` | profiles + parse |
| `toolgen_test.go` / `toolgen_gaps_test.go` | generation gaps |
| `tool_compiler_test.go` | compile forge |
| `thunderdome_*_test.go` | harness + normalize |
| `should_generate_tool_test.go` | throttle/confidence |
| `*_coverage_test.go` | helpers, patterns, types, utils, templates |
| `mocks_test.go` | MockKernel, MockLLM, MockToolSynthesizer |
| `prompt_evolution/prompt_evolution_test.go` | SPL unit coverage |

## 3. E2E (`tests/e2e/autopoiesis_kernel_ouroboros_integration_test.go`)

Emphasizes **kernel bridge contracts**, not full LLM generation:

- Valid kernel binding smoke  
- NaN float injection  
- Overlapping retracts  
- Rapid assertions  
- Temporal retract interruption  
- Cascading registration rejection / nil kernel resilience  
- Recovery rebind  
- Atom coercion / partial failure  

Also: `tests/e2e/rulecourt_feedback_integration_test.go` imports package.

## 4. Coverage strengths

- Heuristic detectors well tested  
- Learning store concurrency tested  
- Delegation processing with mocks  
- Stage string / config defaults  
- UI page model with autopoiesis types  

## 5. Coverage gaps

| Gap | Risk |
|-----|------|
| Full Ouroboros + real `go build` + Thunderdome kill multi-retry under CI time budgets | Integration fragility undetected |
| Chat `process.go` light generate_tool vs Ouroboros parity | Safety depth regression |
| `go_safety.mg` empty-policy fail-closed | Policy load regression |
| SPL auto-promote end-to-end with JIT compiler | Atom pollution |
| VirtualStore SetToolGenerator/Executor roundtrip | Boot wiring drift |
| Yaegi vs binary path selection policy | Dual-runtime confusion |

## 6. Recommended test additions (backlog)

1. Golden SafetyChecker cases for each `ViolationType`.  
2. Fake LLM scripted Ouroboros: safety fail → regen → pass.  
3. Thunderdome fatal attack → MaxPanicRetries reject.  
4. Boot restore: binary+source → `tool_registered` batch equals registry size.  
5. ProcessKernelDelegations ignores non-pending statuses.  

## 7. Alignment with principles

Tests currently protect **learning integrity**, **delegation shape**, and **local loop construction** better than **end-to-end constitutional generate_tool**. Prefer new tests on the dual-path gap (P0 in gap analysis).
