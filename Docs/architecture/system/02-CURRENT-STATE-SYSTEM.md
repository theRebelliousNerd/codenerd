# system — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/system/` (complete internal coverage)
> **Implementation: `internal/system/` — 5 non-test .go, 11 tests, 1 .mg**


## 1. Source location

- Primary package: `internal/system/` (exists; 5 non-test Go files)
- 1:1 mapping: `Docs/architecture/system/` ↔ `internal/system/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/system/factory.go` | 1136 | source |
| `internal/system/factory_adapters.go` | 433 | source |
| `internal/system/agent_registry.go` | 284 | source |
| `internal/system/holographic_code_scope.go` | 172 | source |
| `internal/system/cortex_close.go` | 62 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/system/agent_registry.go` | 284 |
| `internal/system/cortex_close.go` | 62 |
| `internal/system/factory.go` | 1136 |
| `internal/system/factory_adapters.go` | 433 |
| `internal/system/holographic_code_scope.go` | 172 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/system/agent_registry_coverage_test.go` | 334 |
| `internal/system/dom_demo_test.go` | 205 |
| `internal/system/factory_boot_test.go` | 162 |
| `internal/system/factory_test.go` | 120 |
| `internal/system/dom_mangle_test.go` | 107 |
| `internal/system/tool_compilation_test.go` | 95 |
| `internal/system/mocks_test.go` | 87 |
| `internal/system/session_kernel_adapter_test.go` | 71 |
| `internal/system/factory_helpers_test.go` | 63 |
| `internal/system/factory_adapters_test.go` | 57 |

## 5. Behavior summary

Package **system** is a living codeNERD subsystem: System factory / boot wiring helpers.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
