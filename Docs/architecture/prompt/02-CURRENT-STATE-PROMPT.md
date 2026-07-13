# prompt — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/prompt/` (25 non-test .go, 32 tests, 0 .mg)**


## 1. Source location

- Primary package: `internal/prompt/` (**exists** with 25 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

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

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/prompt/atoms_test.go` | 1454 |
| `internal/prompt/compiler_test.go` | 1343 |
| `internal/prompt/budget_test.go` | 740 |
| `internal/prompt/assembler_test.go` | 738 |
| `internal/prompt/selector_test.go` | 729 |
| `internal/prompt/loader_test.go` | 622 |

## 4. Current behavior (summary)

Package **prompt** is a living codeNERD subsystem: JIT prompt compiler, atoms, selector, budget.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (90%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
