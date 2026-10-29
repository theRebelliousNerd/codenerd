# testing — Current State Inventory

> Last verified: 2026-07-13  
> Source root: `internal/testing/`

## Package topology

```
internal/testing/                    # Go package "testing" — anchor only
├── doc.go                           # package comment
├── harness_subsystem.go             # integration-anchor comment
└── context_harness/                 # Go package "context_harness" — all logic
    ├── types.go
    ├── engine_interface.go
    ├── harness.go
    ├── simulator.go
    ├── mock_engine.go
    ├── real_engine.go
    ├── scenarios.go
    ├── scenarios_integration.go
    ├── metrics.go
    ├── reporter.go
    ├── fact_seeder.go
    ├── test_kernel_factory.go
    ├── file_logger.go
    ├── inspector.go
    ├── jit_tracer.go
    ├── activation_tracer.go
    ├── compression_viz.go
    ├── piggyback_tracer.go
    ├── feedback_tracer.go
    ├── README.md
    └── *_test.go (8 files)
```

### Counts (approximate, 2026-07-13)

| Kind | Count | Notes |
|------|------:|-------|
| Non-test `.go` under `context_harness/` | **21** | Includes scenarios + all engines/tracers |
| Test `.go` under `context_harness/` | **8** | Unit tests only |
| Parent `internal/testing/*.go` | **2** | Comments only; no exports |
| Mangle `.mg` in this tree | **0** | Facts are strings / `core.Fact`, not local rules |
| Reverse importers from `cmd/` | **1** | `cmd/nerd/cmd_test_context.go` |
| Other `internal/*` importers of this package | **0** (by design) | Production must not import test harness |

## File roles

### Orchestration & domain model

| File | Role | Hotspot notes |
|------|------|---------------|
| `types.go` | `Scenario`, `Turn`, `Checkpoint`, `Metrics`, `TestResult`, `SimulatorConfig`, modes/categories | Dual-mode flags; enrichment ratio docs |
| `harness.go` | `Harness` — scenario map, run one/all, observability wiring | Keys scenarios by `ScenarioID` |
| `simulator.go` | Turn loop, live LLM branch, checkpoint validation, fuzzy fact IDs | Largest behavioral surface |
| `engine_interface.go` | `ContextEngine` + `ActivationBreakdown` (9 components) + validation types | Interface is the dual-mode seam |
| `metrics.go` | Thread-safe `MetricsCollector` | Mutex; averages + F1 |
| `reporter.go` | Console + JSON output | Shared pass/fail marks with metrics |

### Engines

| File | Role | Hotspot notes |
|------|------|---------------|
| `mock_engine.go` | Fast fact enrichment + thresholded keyword/recency scoring | Activation threshold 100; back-ref boost +50 |
| `real_engine.go` | Wraps `ActivationEngine`, `Compressor`, kernel, store, LLM | `CompressTurn` uses metadata facts, not LLM compressor; `RetrieveContext` uses real scoring |

### Scenario libraries

| File | Role |
|------|------|
| `scenarios.go` | 8 mock scenarios + registry helpers (`GetScenario`, `AllScenarios`, `MockScenarios`, `ScenariosByCategory`) |
| `scenarios_integration.go` | 7 integration scenarios + message templates |

**Mock scenario IDs:**

1. `debugging-marathon`
2. `feature-implementation`
3. `refactoring-campaign`
4. `research-and-build`
5. `tdd-loop`
6. `campaign-execution`
7. `shard-collaboration`
8. `mangle-policy-debug`

**Integration scenario IDs:**

1. `campaign-phase-transition`
2. `swebench-issue-resolution`
3. `token-budget-overflow`
4. `dependency-spreading`
5. `verb-specific-boosting`
6. `ephemeral-filtering`
7. `context-feedback-learning` (in `IntegrationScenarios` / `AllScenarios`; **missing from `GetScenario` map**)

### Seeding & isolation

| File | Role | Honesty |
|------|------|---------|
| `fact_seeder.go` | Seed campaign/issue/graph/topology facts | `Clear()` is no-op |
| `test_kernel_factory.go` | Fresh kernel + parse Mangle fact strings | Schema load stubbed; `randomID()` always 0 |

### Observability

| File | Channel |
|------|---------|
| `file_logger.go` | Session dir + multi-writers + MANIFEST |
| `inspector.go` | Prompt + response + control packet inspection |
| `jit_tracer.go` | CompilationSnapshot atom traces |
| `activation_tracer.go` | Spreading activation snapshots |
| `compression_viz.go` | Before/after compression events |
| `piggyback_tracer.go` | Surface vs control packet |
| `feedback_tracer.go` | Context usefulness learning |

### Tests

| File | Focus |
|------|-------|
| `metrics_test.go` | Finalize averages / F1 |
| `reporter_test.go` | JSON + console |
| `simulator_test.go` | Scoring helpers, retrieval metrics, expectations, fuzzy match |
| `file_logger_test.go` | Log files + manifest |
| `seeder_logger_test.go` | FactSeeder + FileLogger writers |
| `feedback_test.go` | Feedback/piggyback tracers + store integration |
| `helpers_extra_test.go` | format/estimate helpers |
| `tracer_helpers_test.go` | formatNumber, truncate, groupBy, FactSeeder campaign |

## Hotspots (where behavior really lives)

1. **`simulator.go` `executeTurn`** — compression → optional tracers → retrieval-on-backref → metrics  
2. **`simulator.go` `validateCheckpoint`** — RetrieveContext + fuzzy ID match + P/R floors (advanced validators **not** called)  
3. **`mock_engine.go` `RetrieveContext`** — threshold filter + budget trim  
4. **`real_engine.go` `RetrieveContext`** — back-ref context assembly + `ScoreFacts` + `SelectWithinBudget`  
5. **`cmd/nerd/cmd_test_context.go`** — only production wiring surface  

## Parent package note

```go
// Package testing provides the robust Context Test Harness.
package testing
```

`internal/testing` itself exports nothing. Import path used by the CLI is:

```go
"codenerd/internal/testing/context_harness"
```

Any future shared test utilities for other domains should either grow under this parent carefully or stay in package-local `_test.go` helpers — do not drag production into this tree.

## CLI surface inventory

`cmd/nerd/cmd_test_context.go` registers `test-context` with flags:

| Flag | Default | Purpose |
|------|---------|---------|
| `--scenario` | "" | Run one by ID |
| `--all` | false | Run all registered |
| `--format` | console | console \| json |
| `--max-turns` | 0 | Override |
| `--token-budget` | 8000 | Retrieval budget |
| `--paging` | true | Config flag (simulator paging field) |
| `--mode` | mock | mock \| real |
| `--category` | "" | **Declared; not applied in run path** |
| `--live` | false | Live LLM assistant turns |
| `--verbose` / tracers / log-dir / console | various | Observability |

## Metrics semantics (as implemented)

`Metrics.CompressionRatio = original_tokens / compressed_tokens`.

- **&lt; 1.0** = semantic **enrichment** (short user text → many structured facts) — expected for harness mock turns  
- **&gt; 1.0** = true **compression**  
- Expected thresholds in scenarios are often ~0.3–0.5 enrichment, not 5:1 compression (README prose in `context_harness/README.md` is partially outdated relative to `types.go` comments)

## What is solid vs thin

| Area | State |
|------|-------|
| Scenario definitions | Solid, extensive |
| Mock retrieval + unit tests | Solid |
| Real activation retrieval path | Implemented and used |
| Real LLM compression path | Thin (compressor constructed, turn compress is fact-from-metadata) |
| Advanced checkpoint validators | Typed, not enforced in simulator |
| Kernel isolation for multi-scenario | Thin |
| Parent package API | Empty by design |
