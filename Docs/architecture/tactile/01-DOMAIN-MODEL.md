# tactile — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/tactile/` (complete internal coverage)
> **Implementation: `internal/tactile/` — 16 non-test .go, 12 tests, 0 .mg**


## Package

`internal/tactile/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `Fact` | `internal/tactile/audit.go:17` |
| `AuditLogger` | `internal/tactile/audit.go:245` |
| `AuditFileLogger` | `internal/tactile/audit.go:351` |
| `ExecutionMetrics` | `internal/tactile/audit.go:439` |
| `ExecutionMetricsSnapshot` | `internal/tactile/audit.go:511` |
| `AuditedExecutorWrapper` | `internal/tactile/audit.go:583` |
| `OutputAnalyzer` | `internal/tactile/audit.go:622` |
| `TestAnalysis` | `internal/tactile/audit.go:673` |
| `BuildAnalysis` | `internal/tactile/audit.go:774` |
| `Diagnostic` | `internal/tactile/audit.go:783` |
| `DirectExecutor` | `internal/tactile/direct.go:20` |
| `DockerExecutor` | `internal/tactile/docker.go:18` |
| `Executor` | `internal/tactile/executor_interface.go:9` |
| `AuditedExecutorInterface` | `internal/tactile/executor_interface.go:23` |
| `LimitedExecutorInterface` | `internal/tactile/executor_interface.go:31` |
| `SandboxedExecutorInterface` | `internal/tactile/executor_interface.go:39` |
| `CompositeExecutorInterface` | `internal/tactile/executor_interface.go:50` |
| `CompositeExecutor` | `internal/tactile/factory.go:12` |
| `ExecutorFactory` | `internal/tactile/factory.go:169` |
| `PooledExecutor` | `internal/tactile/factory.go:239` |
| `RetryExecutor` | `internal/tactile/factory.go:328` |
| `FileOpType` | `internal/tactile/files.go:18` |
| `FileAuditEvent` | `internal/tactile/files.go:30` |
| `FileResult` | `internal/tactile/files.go:103` |
| `FileEditor` | `internal/tactile/files.go:117` |
| `FileInfo` | `internal/tactile/files.go:655` |
| `ContainerState` | `internal/tactile/persistent_docker.go:27` |
| `PersistentContainer` | `internal/tactile/persistent_docker.go:38` |
| `ContainerMount` | `internal/tactile/persistent_docker.go:55` |
| `ContainerSnapshot` | `internal/tactile/persistent_docker.go:63` |
| `ContainerPoolConfig` | `internal/tactile/persistent_docker.go:73` |
| `ContainerCreateOptions` | `internal/tactile/persistent_docker.go:101` |
| `ContainerExecOptions` | `internal/tactile/persistent_docker.go:116` |
| `PersistentDockerExecutor` | `internal/tactile/persistent_docker.go:129` |
| `NamespaceConfig` | `internal/tactile/platform_darwin.go:36` |
| `LimitedExecutorLinux` | `internal/tactile/platform_linux.go:51` |
| `CgroupManager` | `internal/tactile/platform_linux.go:301` |
| `NamespaceConfig` | `internal/tactile/platform_linux.go:595` |
| `NamespaceExecutor` | `internal/tactile/platform_linux.go:623` |
| `FirejailExecutor` | `internal/tactile/platform_linux_firejail.go:16` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `String` | `internal/tactile/audit.go:23` |
| `ToFacts` | `internal/tactile/audit.go:54` |
| `NewAuditLogger` | `internal/tactile/audit.go:262` |
| `AddCallback` | `internal/tactile/audit.go:270` |
| `SetFactCallback` | `internal/tactile/audit.go:277` |
| `EnableFileLogging` | `internal/tactile/audit.go:284` |
| `Close` | `internal/tactile/audit.go:297` |
| `Log` | `internal/tactile/audit.go:308` |
| `GetMetrics` | `internal/tactile/audit.go:340` |
| `NewAuditFileLogger` | `internal/tactile/audit.go:358` |
| `Write` | `internal/tactile/audit.go:378` |
| `Close` | `internal/tactile/audit.go:396` |
| `Rotate` | `internal/tactile/audit.go:409` |
| `NewExecutionMetrics` | `internal/tactile/audit.go:459` |
| `RecordEvent` | `internal/tactile/audit.go:467` |
| `Snapshot` | `internal/tactile/audit.go:528` |
| `Reset` | `internal/tactile/audit.go:565` |
| `NewAuditedExecutor` | `internal/tactile/audit.go:589` |
| `Execute` | `internal/tactile/audit.go:602` |
| `Capabilities` | `internal/tactile/audit.go:607` |
| `Validate` | `internal/tactile/audit.go:612` |
| `GetLogger` | `internal/tactile/audit.go:617` |
| `NewOutputAnalyzer` | `internal/tactile/audit.go:625` |
| `AnalyzeTestOutput` | `internal/tactile/audit.go:630` |
| `ToFacts` | `internal/tactile/audit.go:685` |
| `AnalyzeBuildOutput` | `internal/tactile/audit.go:723` |
| `ToFacts` | `internal/tactile/audit.go:792` |
| `NewDirectExecutor` | `internal/tactile/direct.go:29` |
| `NewDirectExecutorWithConfig` | `internal/tactile/direct.go:35` |
| `SetAuditCallback` | `internal/tactile/direct.go:44` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Tactile routing / action-to-tool surfaces**
