# testing — TODO

> Last verified: 2026-09-25  
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

## Status — 2026-09-25 (lane B wave 3)

| ID | Status | Evidence |
|----|--------|----------|
| T-001 | **Done** | `validateCheckpoint` no longer substitutes `MustRetrieve` for an empty retrieval; no engine / a retrieval error fails the checkpoint. The mock-mode 0.5/0.5 floor and the 0.95/0.95 score for unmeasured turns in `calculateRetrievalMetrics` went with it. `TestValidateCheckpoint_WhenNothingIsRetrieved_ShouldFail` (was `TestValidateCheckpointFallback`, which pinned the soft pass), `TestCalculateRetrievalMetrics_ShouldNotScoreWhatWasNotMeasured`. |
| T-002 | **Done** | `validateActivation` / `validateCompression` / `validateFeedback` in `simulator.go`; what the engines cannot show (Compressor trigger, summary, feedback sample count) fails as unverified; `ActivationValidation.ValidateActivation(nil)` is an error, not a skip. `TestValidateCheckpoint_ShouldEnforceDeclaredValidators`. Also: `formatFloat` printed 20 as "D" (`TestFormatFloat` updated). |
| T-003 | **Done** | `Harness.RunAll` resets the engine before each scenario, runs in registry order (was map order). `TestHarnessRunAll_ShouldResetBetweenScenariosAndNameTheSkipped`. |
| — | **Done** (found while closing T-002) | Checkpoints were matched against the turn's slice index, not its TurnID, so in 7 of 8 mock scenarios no checkpoint ever ran; an unreached checkpoint now fails the scenario. `TestRunScenario_ShouldFireCheckpointsByTurnIDAndFailUnreachedOnes`. |
| T-010 | Declined | Driving the real `Compressor` needs an LLM client for summaries; not verifiable here. Its absence is now reported (compression validators fail as unverified) instead of passed. |
| T-011 | **Done** | `Scenario.InitialFacts` are parsed (`fact_seeder.go`) and asserted through `ContextEngine.SeedFacts` into the kernel and the retrieval pool before the first turn. `TestRunScenario_ShouldSeedTheScenariosInitialFacts`. |
| T-012 | Declined | Real JIT traces need the prompt compiler wired into the harness (`internal/prompt`, another lane's package); `JITTracer` stays synthetic and labelled so. |
| T-013 | **Done for mock mode** | `nerd test-context --mode=mock` boots no Cortex and runs on a fresh in-memory kernel, so scenario facts never land in the workspace's live kernel. Real mode keeps the Cortex kernel (the real engine needs its store and LLM client); isolation there is the engine's `Reset`. |
| T-020 | **Done** | `--category` narrows the harness (`Harness.SelectCategory`) and, without `--scenario`, runs the category. `TestSelectTestContextRun_ShouldApplyTheCategory`. Mock scenarios now declare `Category`/`Mode` (the filter found none). |
| T-021 | **Done** | `GetScenario` (and `MockScenarios`) removed: `AllScenarios` is the one registry and includes `context-feedback-learning`. |
| T-022 | **Done** | CLI help is generated from the registry. `TestTestContextHelp_ShouldListEveryRegisteredScenario`. |
| T-023 | **Done** | `context_harness/README.md` file tree and honesty rules updated. |
| T-024 | **Done** | `TestScenarioRegistry_ShouldBeCompleteAndConsistent` (unique IDs, category ↔ mode). |
| T-030 | **Done** | `FactSeeder` (typed seeders + no-op `Clear`) removed; isolation is `Reset` + a fresh kernel. |
| T-031 | **Done** | `TestKernelFactory` (and its unused schema list) removed. |
| T-032 | **Done** | `randomID` (always 0) removed with the factory. |
| T-033 | Declined | New scenario class (adversarial); vision backlog, not unfinished wiring. |
| T-034 | Declined | Replay from `.nerd/logs`; vision backlog. |
| T-035 | Declined | No CI config is edited from this lane. Mock mode now runs without a Cortex or API key, which is what a CI job needs; see the finding below before gating on it. |
| T-036 | **Done** (partly) | `PeakMemoryMB` sampled after each turn (`sampleMemory`); `TokenBudgetViolations` recorded when a checkpoint's retrieval exceeds the budget as the production `TokenCounter` counts it. `QualityDegradation` declined: nothing defines it. |
| T-037 | Declined | Only if needed. |

Finding (open): with the harness honest, 7 of 8 mock scenarios fail (they
failed before too, on aggregate metrics alone, with no checkpoint run). Their
scripts are sparse (2–7 key turns where the checkpoints sit at turns 35–95;
two checkpoints name turns the scripts do not have), and of the seven
checkpoints that now run, six retrieve none of their `MustRetrieve` facts on
the mock engine. Rewriting the scenarios is scenario authoring, not wiring;
`nerd test-context --category=mock` states each failure.

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
