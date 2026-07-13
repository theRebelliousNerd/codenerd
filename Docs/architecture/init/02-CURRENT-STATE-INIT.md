# init — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/init/` (complete internal coverage)
> **Implementation: `internal/init/` — 16 non-test .go, 7 tests, 1 .mg**


## 1. Source location

- Primary package: `internal/init/` (exists; 16 non-test Go files)
- 1:1 mapping: `Docs/architecture/init/` ↔ `internal/init/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/init/initializer.go` | 1128 | source |
| `internal/init/scanner.go` | 1034 | source |
| `internal/init/profile.go` | 956 | source |
| `internal/init/strategic_knowledge.go` | 742 | source |
| `internal/init/agents.go` | 728 | source |
| `internal/init/strategic_documents.go` | 600 | source |
| `internal/init/tools.go` | 566 | source |
| `internal/init/agents_registration.go` | 521 | source |
| `internal/init/scanner_dependencies.go` | 504 | source |
| `internal/init/interactive.go` | 446 | source |
| `internal/init/validation.go` | 376 | source |
| `internal/init/agents_knowledge.go` | 369 | source |
| `internal/init/jit_integration.go` | 261 | source |
| `internal/init/shared_kb.go` | 195 | source |
| `internal/init/typeu_agents.go` | 178 | source |
| `internal/init/eta_tracker.go` | 158 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/init/agents.go` | 728 |
| `internal/init/agents_knowledge.go` | 369 |
| `internal/init/agents_registration.go` | 521 |
| `internal/init/eta_tracker.go` | 158 |
| `internal/init/initializer.go` | 1128 |
| `internal/init/interactive.go` | 446 |
| `internal/init/jit_integration.go` | 261 |
| `internal/init/profile.go` | 956 |
| `internal/init/scanner.go` | 1034 |
| `internal/init/scanner_dependencies.go` | 504 |
| `internal/init/shared_kb.go` | 195 |
| `internal/init/strategic_documents.go` | 600 |
| `internal/init/strategic_knowledge.go` | 742 |
| `internal/init/tools.go` | 566 |
| `internal/init/typeu_agents.go` | 178 |
| `internal/init/validation.go` | 376 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/init/init_coverage_test.go` | 1360 |
| `internal/init/typeu_coverage_test.go` | 563 |
| `internal/init/agents_knowledge_helpers_test.go` | 147 |
| `internal/init/init_test.go` | 101 |
| `internal/init/interactive_display_test.go` | 81 |
| `internal/init/scanner_dependencies_test.go` | 69 |
| `internal/init/scanner_test.go` | 61 |

## 5. Behavior summary

Package **init** is a living codeNERD subsystem: Workspace/project initialization and scanning.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (70%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
