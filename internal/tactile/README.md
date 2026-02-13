# internal/tactile/

Motor Cortex - The lowest-level execution layer for physical world interaction.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Overview

The tactile package is the motor cortex of the neuro-symbolic architecture, providing shell command execution, process management, and sandboxed execution. It follows the neuroscience metaphor where Perception is sensory input, Kernel is cognition, Articulation is speech, and Tactile is motor output.

## Architecture

```text
VirtualStore → Executor Selection → Command Execution → Audit Trail
                     ↓
         ┌──────────┴──────────────────┐
         ↓           ↓                 ↓
    DirectExecutor   DockerExecutor    NamespaceExecutor
    (no sandbox)     (containers)      (Linux namespaces)
```

## Structure

```text
tactile/
├── types.go              # Command, ExecutionResult types
├── executor_interface.go # Executor interface contract
├── executor.go           # Legacy SafeExecutor
├── direct.go             # DirectExecutor (no sandbox)
├── docker.go             # DockerExecutor (containers)
├── persistent_docker.go  # Stateful containers
├── factory.go            # CompositeExecutor (routing)
├── files.go              # File operations with audit
├── audit.go              # Mangle fact generation
├── platform_*.go         # Platform-specific implementations
├── python/               # Python environment management
└── swebench/             # SWE-bench orchestration
```

## Key Types

### Command

```go
type Command struct {
    Binary           string
    Arguments        []string
    WorkingDirectory string
    Environment      []string
    Stdin            string
    Limits           *ResourceLimits
    Sandbox          *SandboxConfig
    SessionID        string
    RequestID        string
}
```

### SandboxMode

| Mode | Implementation | Use Case |
|------|----------------|----------|
| `none` | DirectExecutor | Trusted operations |
| `docker` | DockerExecutor | Isolated execution |
| `namespace` | NamespaceExecutor | Linux-only isolation |
| `firejail` | FirejailExecutor | Lightweight sandboxing |

## Executor Hierarchy

```text
Executor (interface)
    ├── DirectExecutor (no sandbox)
    ├── DockerExecutor (ephemeral containers)
    │       └── PersistentDockerExecutor (stateful)
    ├── NamespaceExecutor (Linux only)
    └── CompositeExecutor (routes by mode)
```

## Audit Trail

All executors emit Mangle facts for kernel reasoning:

```mangle
execution_started("session-123", "req-456", "go", 1703001234).
execution_completed("req-456", /success, 0, 2345).
file_written("/path/to/file.go", "abc123", "session-123", 1703001235).
```

## Platform Support

| Platform | Features |
|----------|----------|
| Windows | Job objects for resource limits |
| Linux | Namespaces, cgroups, rlimits |
| macOS | Docker preferred, rlimits |

## SWE-bench Integration

Persistent containers for the clone → venv → patch → test workflow:

```go
pool := NewContainerPool()
container, _ := pool.GetOrCreate("swebench-1")
container.Execute(ctx, Command{Binary: "pytest"})
```

## Testing

```bash
go test ./internal/tactile/...
```

---

**Last Updated:** December 2024


> *[Archived & Reviewed by The Librarian on 2026-01-25]*