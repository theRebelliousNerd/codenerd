# tactile — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/tactile/` (complete internal coverage)
> **Implementation: `internal/tactile/` — 16 non-test .go, 12 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/tactile/` (exists; 16 non-test Go files)
- 1:1 mapping: `Docs/architecture/tactile/` ↔ `internal/tactile/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/tactile/platform_linux.go` | 903 | source |
| `internal/tactile/persistent_docker.go` | 875 | source |
| `internal/tactile/audit.go` | 809 | source |
| `internal/tactile/python/environment.go` | 783 | source |
| `internal/tactile/platform_windows.go` | 706 | source |
| `internal/tactile/files.go` | 682 | source |
| `internal/tactile/docker.go` | 460 | source |
| `internal/tactile/factory.go` | 446 | source |
| `internal/tactile/types.go` | 417 | source |
| `internal/tactile/direct.go` | 325 | source |
| `internal/tactile/platform_linux_firejail.go` | 325 | source |
| `internal/tactile/swebench/instance.go` | 320 | source |
| `internal/tactile/swebench/harness.go` | 215 | source |
| `internal/tactile/platform_unix.go` | 116 | source |
| `internal/tactile/executor_interface.go` | 55 | source |
| `internal/tactile/platform_darwin.go` | 44 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/tactile/audit.go` | 809 |
| `internal/tactile/direct.go` | 325 |
| `internal/tactile/docker.go` | 460 |
| `internal/tactile/executor_interface.go` | 55 |
| `internal/tactile/factory.go` | 446 |
| `internal/tactile/files.go` | 682 |
| `internal/tactile/persistent_docker.go` | 875 |
| `internal/tactile/platform_darwin.go` | 44 |
| `internal/tactile/platform_linux.go` | 903 |
| `internal/tactile/platform_linux_firejail.go` | 325 |
| `internal/tactile/platform_unix.go` | 116 |
| `internal/tactile/platform_windows.go` | 706 |
| `internal/tactile/python/environment.go` | 783 |
| `internal/tactile/swebench/harness.go` | 215 |
| `internal/tactile/swebench/instance.go` | 320 |
| `internal/tactile/types.go` | 417 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/tactile/audit_test.go` | 1113 |
| `internal/tactile/coverage_boost_test.go` | 922 |
| `internal/tactile/tactile_test.go` | 726 |
| `internal/tactile/docker_platform_test.go` | 577 |
| `internal/tactile/types_coverage_test.go` | 300 |
| `internal/tactile/docker_platform_windows_test.go` | 246 |
| `internal/tactile/swebench/coverage_boost_test.go` | 213 |
| `internal/tactile/files_test.go` | 209 |
| `internal/tactile/swebench/instance_test.go` | 118 |
| `internal/tactile/python/environment_test.go` | 78 |

## 5. Behavior summary

Package **tactile** is a living codeNERD subsystem: Tactile routing / action-to-tool surfaces.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (85%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
