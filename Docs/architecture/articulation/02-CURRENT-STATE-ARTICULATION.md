# articulation — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/articulation/` (complete internal coverage)
> **Implementation: `internal/articulation/` — 8 non-test .go, 7 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/articulation/` (exists; 8 non-test Go files)
- 1:1 mapping: `Docs/architecture/articulation/` ↔ `internal/articulation/`

## 2. Largest source files

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

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/articulation/emitter.go` | 1103 |
| `internal/articulation/json_scanner.go` | 105 |
| `internal/articulation/kernel_context.go` | 128 |
| `internal/articulation/prompt_assembler.go` | 1164 |
| `internal/articulation/prompt_assembler_adapter.go` | 108 |
| `internal/articulation/protocol_types.go` | 239 |
| `internal/articulation/schema.go` | 240 |
| `internal/articulation/stream_parser.go` | 109 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/articulation/prompt_assembler_test.go` | 941 |
| `internal/articulation/emitter_test.go` | 523 |
| `internal/articulation/emitter_boundary_test.go` | 316 |
| `internal/articulation/json_scanner_test.go` | 239 |
| `internal/articulation/emitter_extra_test.go` | 86 |
| `internal/articulation/emitter_helpers_test.go` | 86 |
| `internal/articulation/stream_parser_test.go` | 46 |

## 5. Behavior summary

Package **articulation** is a living codeNERD subsystem: Atoms→NL emission, Piggyback protocol, prompt assembly bridge.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (85%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
