# mangle — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/mangle/` (complete internal coverage)
> **Implementation: `internal/mangle/` — 21 non-test .go, 39 tests, 1 .mg**


## 1. Source location

- Primary package: `internal/mangle/` (exists; 21 non-test Go files)
- 1:1 mapping: `Docs/architecture/mangle/` ↔ `internal/mangle/`

## 2. Largest source files

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
| `internal/mangle/feedback/types.go` | 253 | source |
| `internal/mangle/feedback/error_classifier.go` | 252 | source |
| `internal/mangle/synth/schema.go` | 213 | source |
| `internal/mangle/synth/decoder.go` | 169 | source |
| `internal/mangle/synth/spec.go` | 122 | source |
| `internal/mangle/feedback/normalize.go` | 76 | source |
| `internal/mangle/simd_intersect_amd64.go` | 51 | source |
| `internal/mangle/parse_lock.go` | 44 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/mangle/differential.go` | 866 |
| `internal/mangle/engine.go` | 1100 |
| `internal/mangle/feedback/error_classifier.go` | 252 |
| `internal/mangle/feedback/loop.go` | 476 |
| `internal/mangle/feedback/normalize.go` | 76 |
| `internal/mangle/feedback/pre_validator.go` | 402 |
| `internal/mangle/feedback/prompt_builder.go` | 446 |
| `internal/mangle/feedback/types.go` | 253 |
| `internal/mangle/grammar.go` | 787 |
| `internal/mangle/lsp.go` | 1055 |
| `internal/mangle/parse_lock.go` | 44 |
| `internal/mangle/proof_tree.go` | 482 |
| `internal/mangle/schema_validator.go` | 412 |
| `internal/mangle/simd_intersect_amd64.go` | 51 |
| `internal/mangle/simd_intersect_generic.go` | 22 |
| `internal/mangle/synth/compile.go` | 424 |
| `internal/mangle/synth/decoder.go` | 169 |
| `internal/mangle/synth/schema.go` | 213 |
| `internal/mangle/synth/spec.go` | 122 |
| `internal/mangle/synth/validate.go` | 330 |
| `internal/mangle/transpiler/sanitizer.go` | 379 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/mangle/torture_test.go` | 2825 |
| `internal/mangle/mangle_validation_test.go` | 1394 |
| `internal/mangle/engine_test.go` | 885 |
| `internal/mangle/synth/validate_test.go` | 867 |
| `internal/mangle/feedback/feedback_test.go` | 711 |
| `internal/mangle/synth/compile_test.go` | 691 |
| `internal/mangle/feedback/pre_validator_test.go` | 393 |
| `internal/mangle/synth/schema_test.go` | 353 |
| `internal/mangle/schema_validator_test.go` | 316 |
| `internal/mangle/feedback/types_test.go` | 293 |

## 5. Behavior summary

Package **mangle** is a living codeNERD subsystem: Mangle engine bindings, differential evaluation, generation feedback.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
