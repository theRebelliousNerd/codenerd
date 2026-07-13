# testing — TODO

> Last verified: 2026-07-13  
> Priorities: P0 blocking correctness → P3 polish

## P0 — Correctness / false confidence

| ID | Item | Why |
|----|------|-----|
| T-001 | Stop empty-retrieval soft-pass in real mode (`validateCheckpoint` fallback to MustRetrieve) | Masks total retrieval failure |
| T-002 | Enforce `ValidateActivation` / `ValidateCompression` / `ValidateFeedback` when non-nil | Integration scenarios currently under-assert |
| T-003 | Call `ContextEngine.Reset()` (and ideally reseed) between scenarios in `RunAll` | Cross-scenario contamination |

## P1 — Fidelity

| ID | Item | Why |
|----|------|-----|
| T-010 | Drive real `Compressor` (or documented substitute) from `RealIntegrationEngine.CompressTurn` | Real mode overclaims compression fidelity |
| T-011 | Seed `InitialFacts` in harness run path via `FactSeeder` | Integration world setup may never load |
| T-012 | Wire real JIT compiler traces when available | JITTracer currently synthetic |
| T-013 | Isolated kernel per scenario in CLI optional flag | Isolation without full Cortex reuse |

## P2 — Operator UX / registry

| ID | Item | Why |
|----|------|-----|
| T-020 | Apply `--category` filter in `runTestContext` | Flag is dead |
| T-021 | Add `context-feedback-learning` to `GetScenario` map | Registry incompleteness |
| T-022 | Sync CLI Long help with 8 mock + 7 integration IDs | Operator docs drift |
| T-023 | Update `context_harness/README.md` enrichment ratios + file tree | Local README stale |
| T-024 | Registry completeness unit test (`AllScenarios` ⊆ `GetScenario`) | Prevent regressions |

## P3 — Polish

| ID | Item | Why |
|----|------|-----|
| T-030 | Implement `FactSeeder.Clear` or delete with wiring audit | Honest API |
| T-031 | Load minimal schemas in `TestKernelFactory` or remove list | Dead config |
| T-032 | Fix `randomID()` for isolated workspaces | Collision risk |
| T-033 | Adversarial scenarios (context bombing, thrash) | Vision backlog |
| T-034 | Replay scenarios from `.nerd/logs` | Vision backlog |
| T-035 | Optional CI job: mock scenarios without full interactive UX | Automate spine |
| T-036 | Populate `PeakMemoryMB` / quality degradation meaningfully | Metrics completeness |
| T-037 | Parallel-safe harness docs + optional worker pool | Only if needed |

## Done / non-goals (do not reopen without reason)

- Dual-mode interface exists  
- CLI command exists  
- File multi-channel logging exists  
- Turning harness into full OODA / tool simulator — out of scope  
- Vectryx product surfaces — out of scope  

## Suggested sequence

1. T-001 + T-002 + T-003 (stop lying about passes)  
2. T-020 + T-021 + T-024 (registry/CLI honesty)  
3. T-010 + T-011 (real fidelity)  
4. Docs sync T-022/T-023  
5. Future scenario classes T-033/T-034  
