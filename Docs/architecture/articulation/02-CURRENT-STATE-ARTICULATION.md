# articulation — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/articulation/` (8 non-test .go, 7 tests, 0 .mg)**


## 1. Source location

- Primary package: `internal/articulation/` (**exists** with 8 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

| Path | Lines | Kind |
|------|------:|------|
| `internal/articulation/prompt_assembler.go` | 1164 | source |
| `internal/articulation/emitter.go` | 1103 | source |
| `internal/articulation/schema.go` | 240 | source |
| `internal/articulation/protocol_types.go` | 239 | source |
| `internal/articulation/kernel_context.go` | 128 | source |
| `internal/articulation/stream_parser.go` | 109 | source |
| `internal/articulation/prompt_assembler_adapter.go` | 108 | source |
| `internal/articulation/json_scanner.go` | 105 | source |

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/articulation/prompt_assembler_test.go` | 941 |
| `internal/articulation/emitter_test.go` | 523 |
| `internal/articulation/emitter_boundary_test.go` | 316 |
| `internal/articulation/json_scanner_test.go` | 239 |
| `internal/articulation/emitter_extra_test.go` | 86 |
| `internal/articulation/emitter_helpers_test.go` | 86 |

## 4. Current behavior (summary)

Package **articulation** is a living codeNERD subsystem: Atoms→NL, Piggyback emitter, prompt assembly bridge.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (85%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
