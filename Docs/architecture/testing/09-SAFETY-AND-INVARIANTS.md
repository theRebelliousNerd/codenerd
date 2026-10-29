# testing — Safety and Invariants

> Last verified: 2026-07-13

## Safety posture

This package is a **test harness**, not an agent executive. It deliberately avoids:

- Emitting `next_action`  
- Calling VirtualStore tool routes  
- Bypassing `permitted(...)` for user-visible side effects  
- Writing production code paths that skip policy  

CLI still boots Cortex (which loads policy), but the harness’s work product is logs + pass/fail metrics, not repository mutations via agent tools.

## Invariants

### I1 — ContextEngine implementations satisfy the interface

Compile-time:

```go
var _ ContextEngine = (*MockContextEngine)(nil)
var _ ContextEngine = (*RealIntegrationEngine)(nil)
```

### I2 — Scenario map keys are ScenarioID (kebab-case)

`NewHarness` indexes by `scenario.ScenarioID`, not `Name`. CLI `--scenario` must use IDs like `debugging-marathon`.

### I3 — Live implies real

CLI and config: `--live` without real mode upgrades to real and prints a warning. Live without LLM client fails at generate time.

### I4 — Kernel type honesty

CLI fails if `cortex.Kernel` is not `*core.RealKernel`. No silent nil kernel into engines.

### I5 — Metrics mutex integrity

All `MetricsCollector` mutators take `mu`. Finalize holds lock while computing averages.

### I6 — Real engine mutex integrity

`RealIntegrationEngine` guards fact lists, counters, and score maps under `mu` for Compress/Retrieve/Reset/stats.

### I7 — Activation threshold in mock

Mock retrieval applies `activationThreshold = 100` before budget selection so low-relevance facts do not flood results solely by budget packing.

### I8 — Enrichment-aware expectation checks

`meetsExpectations` must branch on expected ratio &lt; 1 vs ≥ 1. Changing this without updating all scenarios breaks mock suite meaning.

### I9 — Checkpoint fallback is lenient

If retrieval returns no IDs, validator falls back to `MustRetrieve` as retrieved — designed for mock/dev, can mask total retrieval failure. Treat as known softness (see failure modes).

### I10 — Test-only import boundary

Production libraries must not import `context_harness`. Violation would couple runtime to a stress harness and risk shipping mock scoring paths.

### I11 — FileLogger closes files

`Close` writes footers, closes all handles, writes MANIFEST. CLI defers Close; tests should too when creating loggers.

### I12 — No network in mock mode

Mock path uses pure Go scoring and local kernel fact load only. Network only via Cortex boot side effects and real/live LLM.

## Concurrency rules

| Object | Safe concurrent use? |
|--------|----------------------|
| Single `SessionSimulator` | No — sequential turns only |
| Single `Harness` | No concurrent RunScenario |
| `MetricsCollector` | Mutexed methods OK; still owned by one simulator |
| Multiple harnesses (different kernels) | OK in principle; not used by CLI |

## Data isolation rules (aspirational vs actual)

| Rule | Intended | Actual |
|------|----------|--------|
| Fresh facts per scenario | Yes | Shared Cortex kernel; engine Reset not always applied |
| Isolated workspace kernels | Factory path | CLI uses workspace Cortex |
| Seeder Clear | Retract facts | No-op |

Documented as gap; do not claim isolation guarantees for CLI multi-scenario runs today.

## Constitutional adjacency

| Concern | Harness behavior |
|---------|------------------|
| Default deny tools | N/A — no tools |
| Prompt injection in scenarios | Scenario text is trusted test data |
| Live LLM JSON | Parsed carefully (`extractJSON` / parse helpers); malformed responses error |
| Secrets | API key via env/flag; logs may contain prompts — treat log dirs as sensitive |
| Disk | Writes under `--log-dir` (default `.nerd/context-tests`) |

## Mangle Decl

Harness does not declare Mangle predicates. Invalid facts may fail at `LoadFacts` depending on kernel schema. Keep predicates aligned with core schemas used by Cortex boot.
