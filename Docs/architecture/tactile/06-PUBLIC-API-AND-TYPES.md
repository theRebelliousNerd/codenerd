# tactile — Public API and Types

> Last verified: **2026-07-13**  
> Package import path: `codenerd/internal/tactile`  
> Subpackages: `codenerd/internal/tactile/python`, `codenerd/internal/tactile/swebench`

## Interfaces (`executor_interface.go`)

| Interface | Methods |
|-----------|---------|
| `Executor` | `Execute`, `Capabilities`, `Validate` |
| `AuditedExecutorInterface` | + `SetAuditCallback` |
| `LimitedExecutorInterface` | + `SetDefaultLimits` |
| `SandboxedExecutorInterface` | + `SetDefaultSandbox`, `AvailableSandboxModes` |
| `CompositeExecutorInterface` | + `RegisterExecutor` |

## Core types (`types.go`)

| Type | Role |
|------|------|
| `SandboxMode` | `none` / `docker` / `namespace` / `firejail` |
| `Command` | Execution request |
| `ResourceLimits` | Caps |
| `SandboxConfig` | Isolation config |
| `ExecutionResult` | Outcome |
| `ResourceUsage` | rusage-like metrics |
| `ExecutorCapabilities` | Feature flags for an executor |
| `AuditEventType` / `AuditEvent` | Audit stream |
| `ExecutorConfig` | Defaults + merge |

### Functions

| Symbol | Role |
|--------|------|
| `DefaultExecutorConfig()` | Sensible defaults |
| `(Command).CommandString()` | Display string |
| `(ExecutionResult).IsError/IsNonZeroExit/Output` | Classification helpers |
| `(ResourceUsage).TotalCPUTimeMs` | CPU sum |
| `(ExecutorConfig).Merge` | Apply defaults to Command |

## Executors — constructors

| Constructor | File |
|-------------|------|
| `NewDirectExecutor` / `WithConfig` | `direct.go` |
| `NewDockerExecutor` / `WithConfig` | `docker.go` |
| `NewCompositeExecutor` / `WithConfig` | `factory.go` |
| `NewExecutorFactory` / `NewDefaultFactory` | `factory.go` |
| `NewPooledExecutor` | `factory.go` |
| `NewRetryExecutor` | `factory.go` |
| `NewPersistentDockerExecutor` | `persistent_docker.go` |
| `NewLimitedExecutorWindows` | `platform_windows.go` |
| `NewLimitedExecutorLinux` | `platform_linux.go` |
| `NewNamespaceExecutor` | `platform_linux.go` |
| `NewFirejailExecutor` | `platform_linux_firejail.go` |
| `NewWindowsContainerExecutor` | `platform_windows.go` |
| `GetPlatformExecutor` | platform_*.go |
| `GetLimitedExecutor` | windows |

## Docker helpers

| Symbol | Role |
|--------|------|
| `(*DockerExecutor).IsAvailable` | Detection result |
| `PullImage` / `ImageExists` | Image lifecycle |
| Persistent: `CreateContainer`, `Start/Stop/Remove`, `ExecInContainer`, `HealthCheck`, `Create/Restore/List/DeleteSnapshot`, `CopyTo/From`, `Cleanup`, `ListContainers` | Pool API |
| `DefaultContainerPoolConfig` | Pool defaults |

## Files (`files.go`)

| Symbol | Role |
|--------|------|
| `FileOpType` | read/write/edit/insert/delete/patch |
| `FileAuditEvent` / `ToFacts` | File audit |
| `FileResult` | Op outcome |
| `FileEditor` | Motor |
| `NewFileEditor` / `WithSession` | Ctors |
| `ReadFile`, `ReadLines`, `WriteFile`, `EditLines`, `InsertLines`, `DeleteLines`, `ReplaceElement` | Ops |
| `GetFileInfo`, `FileExists`, `CreateDirectory` | Meta |
| `SetAuditCallback`, `SetFactCallback`, `SetWorkingDir` | Wiring |
| `FileInfo` | Stat-like |

## Audit (`audit.go`)

| Symbol | Role |
|--------|------|
| `Fact` | Cycle-free fact type |
| `(Fact).String` | Datalog render |
| `(AuditEvent).ToFacts` | Event→facts |
| `AuditLogger` / `NewAuditLogger` | Hub |
| `AuditFileLogger` | JSONL |
| `ExecutionMetrics` / `Snapshot` / `Reset` | Counters |
| `NewAuditedExecutor` / `AuditedExecutorWrapper` | Decorator |
| `OutputAnalyzer` / `TestAnalysis` / `BuildAnalysis` / `Diagnostic` | Structured parse |

## python package

| Symbol | Role |
|--------|------|
| `EnvironmentConfig` / `DefaultConfig` | Config |
| `ProjectInfo` / `RepoName` | Project identity |
| `Environment` / `NewEnvironment` | Lifecycle owner |
| `Initialize`, `Setup`, `Teardown`, `Reset` | Lifecycle |
| `CloneRepo`, `Checkout*`, `SetupVirtualEnv`, `InstallDependencies` | Setup steps |
| `ApplyPatch`, `RevertChanges`, `GetDiff` | Patch loop |
| `RunPytest`, `RunTest(s)`, `RunAllTests` | Test |
| `Exec`, `ExecInVenv`, `ExecInRepo` | Raw exec |
| `EnvironmentState`, `TestResult` | State/results |

## swebench package

| Symbol | Role |
|--------|------|
| `Instance`, `Prediction`, `EvaluationResult`, `TestResult` | Schema |
| `LoadInstances`, `LoadInstance` | I/O |
| `Harness` / `NewHarness` | Wrapper |
| `Evaluate`, `RunFailToPassTests`, `RunPassToPassTests` | Metrics |
| Delegation: `Initialize/Setup/Teardown/Reset/ApplyPatch/...` | Env proxy |

## What callers typically need

**Minimum shell:**

```go
ex := tactile.NewDirectExecutor()
res, err := ex.Execute(ctx, tactile.Command{Binary: "go", Arguments: []string{"test", "./..."}})
```

**Audited composite (preferred for production VS):**

```go
ex := tactile.NewCompositeExecutor()
logger := tactile.NewAuditLogger()
logger.SetFactCallback(func(f tactile.Fact) { /* inject */ })
ex.SetAuditCallback(logger.Log)
```

**Files:**

```go
ed := tactile.NewFileEditorWithSession(sessionID)
ed.SetWorkingDir(workspace)
_, _ = ed.EditLines("foo.go", 10, 12, []string{"// replaced"})
```
