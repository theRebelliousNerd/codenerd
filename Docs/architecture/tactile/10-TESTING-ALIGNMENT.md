# tactile — Testing Alignment

> Last verified: **2026-08-09**

## Commands

```powershell
# Full package tree
go test ./internal/tactile/...

# Subpackages
go test ./internal/tactile/python/...
go test ./internal/tactile/swebench/...

# Verbose single area
go test -v ./internal/tactile/ -run Audit
go test -v ./internal/tactile/ -run Docker
go test -v ./internal/tactile/ -run FileEditor
```

Platform-tagged tests only compile on their OS (e.g. `docker_platform_windows_test.go`, linux executor tests).

## Existing coverage map

| Area | Tests | Quality |
|------|-------|---------|
| AuditEvent.ToFacts (all event types) | `audit_test.go` | Strong |
| Fact.String formatting | `audit_test.go` | Strong |
| ExecutionMetrics | `audit_test.go` | Strong |
| OutputAnalyzer test/build | `audit_test.go` | Strong; exact summaries, integer coverage, Windows paths |
| Completed go test/build analyzer facts through live Mangle schema | `internal/core/virtual_store_tactile_audit_test.go` | Strong |
| Audit sink lifecycle/redaction/errors | `audit_test.go` | Strong |
| FileAuditEvent.ToFacts | `coverage_boost_test.go` | Strong |
| FileEditor read/write/edit/insert/delete | `files_test.go`, boost | Strong |
| limitedWriter | boost | Strong |
| Docker buildDockerArgs matrix | `docker_platform_test.go` | Strong |
| Docker Validate / Capabilities | docker_platform_test | Good |
| Composite / Factory / Pool / Retry | boost | Good |
| Windows JobObject / Limited | `docker_platform_windows_test.go` | Good (Windows CI) |
| Types helpers | types/tactile tests | Basic |
| python Environment | environment_test | Present |
| swebench | harness/instance tests | Present |

## What is under-tested

| Gap | Risk |
|-----|------|
| Live `docker run` / daemon absent paths beyond detect | Integration surprise |
| Live cgroup Setup/AddProcess | Needs privileged Linux |
| Live Namespace clone | Privileged / kernel config |
| PersistentDocker full lifecycle against real daemon | Orphan containers |
| Full process execution through VirtualStore plus fact query | Schema boundary is real-kernel tested; live process route remains broader integration |
| RetryExecutor timing correctness | Busy-loop may hide bugs |
| Concurrent Execute stress | Race detector optional |

## Alignment to architecture principles

| Principle | Test evidence |
|-----------|---------------|
| Success vs non-zero | Audit complete facts tests |
| Docker network default | buildDockerArgs tests |
| Env allowlist | DirectExecutor_BuildEnvironment tests |
| File modified facts | FileAuditEvent write/edit tests |
| Factory mode errors for firejail/ns | CreateFromConfig tests |

## Recommended test additions (backlog)

1. Full live process path: AuditLogger callback receives execution_started then
   completed for Direct and the queried kernel facts retain correlation. The
   current real-kernel test proves the production `AuditEvent.ToFacts` analyzer
   bridge without launching a subprocess.
2. PersistentDocker unit tests with fake dockerPath / stub runner if introduced.
3. python state transitions without real docker (interface seams).

## External tests that depend on tactile

| Location | Role |
|----------|------|
| `internal/core/*` mocks | Executor interface compliance |
| `internal/campaign/*` | Orchestrator with DirectExecutor |
| `tests/e2e/*` | Cross-boundary |

Run after tactile semantics change:

```powershell
go test ./internal/core/ -count=1 -run 'VirtualStore|FileEditor|Tactile'
go test ./internal/campaign/ -count=1
```

## Coverage philosophy

Prefer pure unit tests for arg building, fact shapes, and line math. Treat live Docker/cgroup as optional integration (tag `integration` if added) so default `go test ./...` stays hermetic on developer desktops without Docker.
