# config — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/config/` (complete internal coverage)
> **Implementation: `internal/config/` — 17 non-test .go, 5 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/config/` (exists; 17 non-test Go files)
- 1:1 mapping: `Docs/architecture/config/` ↔ `internal/config/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/config/user_config.go` | 1408 | source |
| `internal/config/config.go` | 446 | source |
| `internal/config/llm.go` | 211 | source |
| `internal/config/llm_timeouts.go` | 179 | source |
| `internal/config/ux.go` | 155 | source |
| `internal/config/memory.go` | 130 | source |
| `internal/config/limits.go` | 100 | source |
| `internal/config/integrations.go` | 87 | source |
| `internal/config/jit.go` | 76 | source |
| `internal/config/shard.go` | 56 | source |
| `internal/config/reflection.go` | 53 | source |
| `internal/config/world.go` | 41 | source |
| `internal/config/logging.go` | 32 | source |
| `internal/config/build.go` | 25 | source |
| `internal/config/execution.go` | 16 | source |
| `internal/config/tool_generation.go` | 15 | source |
| `internal/config/mangle.go` | 13 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/config/build.go` | 25 |
| `internal/config/config.go` | 446 |
| `internal/config/execution.go` | 16 |
| `internal/config/integrations.go` | 87 |
| `internal/config/jit.go` | 76 |
| `internal/config/limits.go` | 100 |
| `internal/config/llm.go` | 211 |
| `internal/config/llm_timeouts.go` | 179 |
| `internal/config/logging.go` | 32 |
| `internal/config/mangle.go` | 13 |
| `internal/config/memory.go` | 130 |
| `internal/config/reflection.go` | 53 |
| `internal/config/shard.go` | 56 |
| `internal/config/tool_generation.go` | 15 |
| `internal/config/user_config.go` | 1408 |
| `internal/config/ux.go` | 155 |
| `internal/config/world.go` | 41 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/config/config_comprehensive_test.go` | 885 |
| `internal/config/config_test.go` | 386 |
| `internal/config/env_override_test.go` | 289 |
| `internal/config/config_defaults_test.go` | 170 |
| `internal/config/ollama_worker_config_test.go` | 33 |

## 5. Behavior summary

Package **config** is a living codeNERD subsystem: Configuration loading, engines, limits, user and memory config.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (70%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
