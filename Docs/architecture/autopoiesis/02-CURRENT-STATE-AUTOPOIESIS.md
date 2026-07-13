# autopoiesis — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/autopoiesis/` (complete internal coverage)
> **Implementation: `internal/autopoiesis/` — 37 non-test .go, 30 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/autopoiesis/` (exists; 37 non-test Go files)
- 1:1 mapping: `Docs/architecture/autopoiesis/` ↔ `internal/autopoiesis/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/autopoiesis/ouroboros.go` | 1133 | source |
| `internal/autopoiesis/traces.go` | 994 | source |
| `internal/autopoiesis/prompt_evolution/evolver.go` | 770 | source |
| `internal/autopoiesis/quality.go` | 770 | source |
| `internal/autopoiesis/checker.go` | 722 | source |
| `internal/autopoiesis/tool_generation.go` | 656 | source |
| `internal/autopoiesis/thunderdome.go` | 626 | source |
| `internal/autopoiesis/feedback.go` | 599 | source |
| `internal/autopoiesis/tool_templates.go` | 592 | source |
| `internal/autopoiesis/prompt_evolution/strategy_store.go` | 573 | source |
| `internal/autopoiesis/persistence.go` | 554 | source |
| `internal/autopoiesis/prompt_evolution/atom_generator.go` | 504 | source |
| `internal/autopoiesis/prompt_evolution/feedback_collector.go` | 489 | source |
| `internal/autopoiesis/autopoiesis_feedback.go` | 459 | source |
| `internal/autopoiesis/autopoiesis_kernel.go` | 454 | source |
| `internal/autopoiesis/prompt_evolution/judge.go` | 404 | source |
| `internal/autopoiesis/profiles.go` | 372 | source |
| `internal/autopoiesis/autopoiesis_orchestrator.go` | 370 | source |
| `internal/autopoiesis/prompt_evolution/classifier.go` | 363 | source |
| `internal/autopoiesis/tool_compiler.go` | 359 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/autopoiesis/autopoiesis.go` | 32 |
| `internal/autopoiesis/autopoiesis_agents.go` | 214 |
| `internal/autopoiesis/autopoiesis_analysis.go` | 236 |
| `internal/autopoiesis/autopoiesis_delegation.go` | 187 |
| `internal/autopoiesis/autopoiesis_feedback.go` | 459 |
| `internal/autopoiesis/autopoiesis_helpers.go` | 194 |
| `internal/autopoiesis/autopoiesis_kernel.go` | 454 |
| `internal/autopoiesis/autopoiesis_orchestrator.go` | 370 |
| `internal/autopoiesis/autopoiesis_profiles.go` | 349 |
| `internal/autopoiesis/autopoiesis_tools.go` | 175 |
| `internal/autopoiesis/autopoiesis_types.go` | 304 |
| `internal/autopoiesis/checker.go` | 722 |
| `internal/autopoiesis/complexity.go` | 344 |
| `internal/autopoiesis/feedback.go` | 599 |
| `internal/autopoiesis/ouroboros.go` | 1133 |
| `internal/autopoiesis/panic_maker.go` | 326 |
| `internal/autopoiesis/patterns.go` | 163 |
| `internal/autopoiesis/persistence.go` | 554 |
| `internal/autopoiesis/profiles.go` | 372 |
| `internal/autopoiesis/prompt_evolution/atom_generator.go` | 504 |
| `internal/autopoiesis/prompt_evolution/classifier.go` | 363 |
| `internal/autopoiesis/prompt_evolution/evolver.go` | 770 |
| `internal/autopoiesis/prompt_evolution/feedback_collector.go` | 489 |
| `internal/autopoiesis/prompt_evolution/judge.go` | 404 |
| `internal/autopoiesis/prompt_evolution/strategy_store.go` | 573 |
| `internal/autopoiesis/prompt_evolution/types.go` | 314 |
| `internal/autopoiesis/quality.go` | 770 |
| `internal/autopoiesis/runtime_registry.go` | 217 |
| `internal/autopoiesis/thunderdome.go` | 626 |
| `internal/autopoiesis/tool_compiler.go` | 359 |
| `internal/autopoiesis/tool_detection.go` | 191 |
| `internal/autopoiesis/tool_generation.go` | 656 |
| `internal/autopoiesis/tool_templates.go` | 592 |
| `internal/autopoiesis/tool_validation.go` | 305 |
| `internal/autopoiesis/toolgen.go` | 154 |
| `internal/autopoiesis/traces.go` | 994 |
| `internal/autopoiesis/yaegi_executor.go` | 225 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/autopoiesis/prompt_evolution/prompt_evolution_test.go` | 1479 |
| `internal/autopoiesis/feedback_test.go` | 1059 |
| `internal/autopoiesis/persistence_test.go` | 887 |
| `internal/autopoiesis/ouroboros_test.go` | 865 |
| `internal/autopoiesis/toolgen_test.go` | 745 |
| `internal/autopoiesis/complexity_test.go` | 551 |
| `internal/autopoiesis/quality_test.go` | 508 |
| `internal/autopoiesis/thunderdome_harness_test.go` | 408 |
| `internal/autopoiesis/templates_coverage_test.go` | 252 |
| `internal/autopoiesis/helpers_coverage_test.go` | 220 |

## 5. Behavior summary

Package **autopoiesis** is a living codeNERD subsystem: Self-improvement: Ouroboros tool generation, SafetyChecker, Thunderdome.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (85%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
