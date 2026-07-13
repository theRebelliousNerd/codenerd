# prompt — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/prompt/` (complete internal coverage)
> **Implementation: `internal/prompt/` — 25 non-test .go, 32 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/prompt/` (exists; 25 non-test Go files)
- 1:1 mapping: `Docs/architecture/prompt/` ↔ `internal/prompt/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/prompt/compiler.go` | 1185 | source |
| `internal/prompt/selector.go` | 1175 | source |
| `internal/prompt/loader.go` | 907 | source |
| `internal/prompt/budget.go` | 897 | source |
| `internal/prompt/predicate_selector.go` | 724 | source |
| `internal/prompt/context.go` | 701 | source |
| `internal/prompt/atoms.go` | 684 | source |
| `internal/prompt/assembler.go` | 634 | source |
| `internal/prompt/compiler_db.go` | 419 | source |
| `internal/prompt/resolver.go` | 410 | source |
| `internal/prompt/loader_embedding.go` | 355 | source |
| `internal/prompt/config_factory.go` | 352 | source |
| `internal/prompt/embedded.go` | 227 | source |
| `internal/prompt/evolved_atoms.go` | 209 | source |
| `internal/prompt/sync/synchronizer.go` | 193 | source |
| `internal/prompt/compiler_specialists.go` | 179 | source |
| `internal/prompt/vector_searcher.go` | 178 | source |
| `internal/prompt/query_expansion.go` | 154 | source |
| `internal/prompt/default_corpus.go` | 146 | source |
| `internal/prompt/baseline.go` | 114 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/prompt/assembler.go` | 634 |
| `internal/prompt/atoms.go` | 684 |
| `internal/prompt/baseline.go` | 114 |
| `internal/prompt/budget.go` | 897 |
| `internal/prompt/compiler.go` | 1185 |
| `internal/prompt/compiler_db.go` | 419 |
| `internal/prompt/compiler_options.go` | 72 |
| `internal/prompt/compiler_specialists.go` | 179 |
| `internal/prompt/config_defaults.go` | 88 |
| `internal/prompt/config_factory.go` | 352 |
| `internal/prompt/config_registry.go` | 31 |
| `internal/prompt/context.go` | 701 |
| `internal/prompt/default_corpus.go` | 146 |
| `internal/prompt/embedded.go` | 227 |
| `internal/prompt/evolved_atoms.go` | 209 |
| `internal/prompt/loader.go` | 907 |
| `internal/prompt/loader_embedding.go` | 355 |
| `internal/prompt/manifest.go` | 36 |
| `internal/prompt/output_mode.go` | 43 |
| `internal/prompt/predicate_selector.go` | 724 |
| `internal/prompt/query_expansion.go` | 154 |
| `internal/prompt/resolver.go` | 410 |
| `internal/prompt/selector.go` | 1175 |
| `internal/prompt/sync/synchronizer.go` | 193 |
| `internal/prompt/vector_searcher.go` | 178 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/prompt/atoms_test.go` | 1454 |
| `internal/prompt/compiler_test.go` | 1343 |
| `internal/prompt/budget_test.go` | 740 |
| `internal/prompt/assembler_test.go` | 738 |
| `internal/prompt/selector_test.go` | 729 |
| `internal/prompt/loader_test.go` | 622 |
| `internal/prompt/context_test.go` | 590 |
| `internal/prompt/resolver_test.go` | 587 |
| `internal/prompt/selector_gaps_test.go` | 507 |
| `internal/prompt/compiler_gaps_test.go` | 439 |

## 5. Behavior summary

Package **prompt** is a living codeNERD subsystem: JIT prompt compiler, atoms, selector, budget, resolver.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
