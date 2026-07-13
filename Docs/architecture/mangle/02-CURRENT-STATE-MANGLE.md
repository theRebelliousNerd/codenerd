# mangle — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/mangle/` (21 non-test .go, 39 tests, 1 .mg)**


## 1. Source location

- Primary package: `internal/mangle/` (**exists** with 21 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

| Path | Lines | Kind |
|------|------:|------|
| `internal/mangle/engine.go` | 1100 | source |
| `internal/mangle/lsp.go` | 1055 | source |
| `internal/mangle/differential.go` | 866 | source |
| `internal/mangle/grammar.go` | 787 | source |
| `internal/mangle/proof_tree.go` | 482 | source |
| `internal/mangle/feedback/loop.go` | 476 | source |
| `internal/mangle/feedback/prompt_builder.go` | 446 | source |
| `internal/mangle/synth/compile.go` | 424 | source |
| `internal/mangle/schema_validator.go` | 412 | source |
| `internal/mangle/feedback/pre_validator.go` | 402 | source |
| `internal/mangle/transpiler/sanitizer.go` | 379 | source |
| `internal/mangle/synth/validate.go` | 330 | source |

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/mangle/torture_test.go` | 2825 |
| `internal/mangle/mangle_validation_test.go` | 1394 |
| `internal/mangle/engine_test.go` | 885 |
| `internal/mangle/synth/validate_test.go` | 867 |
| `internal/mangle/feedback/feedback_test.go` | 711 |
| `internal/mangle/synth/compile_test.go` | 691 |

## 4. Current behavior (summary)

Package **mangle** is a living codeNERD subsystem: Mangle engine bindings, differential evaluation, feedback loops.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (90%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
