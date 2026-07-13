# config — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/config/` (17 non-test .go, 4 tests, 0 .mg)**


## 1. Source location

- Primary package: `internal/config/` (**exists** with 17 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

| Path | Lines | Kind |
|------|------:|------|
| `internal/config/user_config.go` | 1322 | source |
| `internal/config/config.go` | 446 | source |
| `internal/config/llm.go` | 211 | source |
| `internal/config/llm_timeouts.go` | 179 | source |
| `internal/config/ux.go` | 155 | source |
| `internal/config/memory.go` | 130 | source |
| `internal/config/limits.go` | 100 | source |
| `internal/config/integrations.go` | 87 | source |
| `internal/config/jit.go` | 76 | source |
| `internal/config/reflection.go` | 53 | source |
| `internal/config/shard.go` | 53 | source |
| `internal/config/world.go` | 41 | source |

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/config/config_comprehensive_test.go` | 885 |
| `internal/config/config_test.go` | 386 |
| `internal/config/env_override_test.go` | 289 |
| `internal/config/config_defaults_test.go` | 170 |

## 4. Current behavior (summary)

Package **config** is a living codeNERD subsystem: Config loading, engines, limits, user config.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (70%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
