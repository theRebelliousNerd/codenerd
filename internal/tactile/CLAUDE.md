# internal/tactile - Motor Cortex Command Execution

This package is the motor cortex of the neuro-symbolic architecture, providing the lowest-level execution layer for physical world interaction - shell commands, process management, and sandboxed execution.

**Related Packages:**
- [internal/core](../core/CLAUDE.md) - VirtualStore consuming executor for action routing
- [internal/shards/system](../shards/system/CLAUDE.md) - TactileRouterShard delegating to executors

## Architecture

The tactile package follows the neuroscience metaphor:
- **Perception**: NL → Mangle atoms (sensory input)
- **Kernel**: Mangle reasoning (cognition)
- **Articulation**: Mangle atoms → NL (speech output)
- **Tactile**: Physical world interaction (motor output)

### Design Principles
- Minimal logic: Constitutional checks happen in VirtualStore, not here
- Sandboxing: Support for Docker, Linux namespaces, and direct execution
- Resource limits: CPU, memory, disk I/O constraints via rlimit/cgroups
- Structured output: Comprehensive execution results for kernel feedback
- Cross-platform: Windows and Unix support with platform-specific files
- Audit trail: Execution events as Mangle facts for kernel reasoning

## File Index

| File | Description |
|------|-------------|
| `types.go` | Core type definitions for command execution with sandbox and resource limit support. Exports Command, SandboxMode enum (none/docker/namespace/firejail), ResourceLimits, SandboxConfig, ExecutionResult, and ExecutorConfig with defaults. |
| `executor_interface.go` | Executor interface contract for all execution implementations. Exports Executor (Execute/Capabilities/Validate), AuditedExecutorInterface, LimitedExecutorInterface, SandboxedExecutorInterface, and CompositeExecutorInterface. |
| `executor.go` | Legacy SafeExecutor for VirtualStore backwards compatibility with binary allowlist. Exports ShellCommand (deprecated), SafeExecutor with AllowedBinaries map, NewSafeExecutor(), and Execute() for legacy callers. |
| `direct.go` | DirectExecutor for direct host execution via os/exec with no sandboxing. Exports NewDirectExecutor(), Capabilities() returning platform info, Execute() with timeout/stdin/audit support, and SetAuditCallback(). |
| `docker.go` | DockerExecutor for container-based isolation with ephemeral containers. Exports NewDockerExecutor(), detectDocker() checking availability, IsAvailable(), Execute() via `docker run --rm`, and volume mount configuration. |
| `persistent_docker.go` | PersistentDockerExecutor for long-running containers preserving state across commands. Exports ContainerState, PersistentContainer, ContainerMount, ContainerSnapshot, ContainerPool for SWE-bench workflows (clone → venv → patch → test). |
| `factory.go` | CompositeExecutor routing commands to appropriate executors based on sandbox mode. Exports NewCompositeExecutor() auto-registering Direct and Docker executors, RegisterExecutor() for custom modes, and SetAuditCallback() propagation. |
| `files.go` | File operations with full audit trail and Mangle fact generation. Exports FileOpType enum, FileAuditEvent with hash tracking, ToFacts() generating file_read/file_written/lines_edited predicates, and FileOperator for batched operations. |
| `audit.go` | Mangle fact generation from execution audit events for kernel injection. Exports Fact struct mirroring core.Fact, AuditEvent.ToFacts() generating execution_started/execution_completed/execution_failed predicates with resource usage. |
| `platform_windows.go` | Windows-specific job object handling for resource limits and process group control. Exports Windows API constants, JOBOBJECT structs, setupJobObject() for memory/CPU limits, and getProcessResourceUsage() via job accounting. |
| `platform_unix.go` | Unix-specific process group and resource usage extraction via syscall.Rusage. Exports getProcessResourceUsage(), setupProcessGroup() with Setpgid, killProcessGroup() using SIGKILL to pgid, and createRlimitsCommon(). |
| `platform_darwin.go` | macOS-specific rlimit handling and executor selection. Exports getMaxRSSBytes() (bytes on macOS), createRlimits() stub, GetPlatformExecutor() preferring Docker if available, and NamespaceConfig stub. |
| `platform_linux.go` | Linux-specific rlimits, namespace support, and cgroup isolation. Exports RLIMIT_NPROC, getMaxRSSBytes() (KB on Linux), createRlimits() with NPROC, NamespaceExecutor with PID/Net/Mount isolation, and cgroup resource limits. |
| `tactile_test.go` | Unit tests for DirectExecutor execution and timeout handling. Tests Execute() with echo command on Windows/Unix, timeout cancellation, and exit code propagation. |

## Key Types

### Command
```go
type Command struct {
    Binary           string            // Executable to run
    Arguments        []string          // Command-line args
    WorkingDirectory string            // Working directory
    Environment      []string          // KEY=VALUE pairs
    Stdin            string            // Input to stdin
    Limits           *ResourceLimits   // CPU, memory, etc.
    Sandbox          *SandboxConfig    // Isolation settings
    SessionID        string            // For audit linking
    RequestID        string            // Unique execution ID
    Tags             map[string]string // Categorization
}
```

### ExecutionResult
```go
type ExecutionResult struct {
    Success       bool
    ExitCode      int
    Stdout        string
    Stderr        string
    StartTime     time.Time
    EndTime       time.Time
    Duration      time.Duration
    ResourceUsage *ResourceUsage
    Cancelled     bool
    TimedOut      bool
    Error         string
    Platform      string
    Executor      string
}
```

### SandboxMode
```go
const (
    SandboxNone      SandboxMode = "none"       // Direct execution
    SandboxDocker    SandboxMode = "docker"     // Docker container
    SandboxNamespace SandboxMode = "namespace"  // Linux namespaces
    SandboxFirejail  SandboxMode = "firejail"   // Firejail sandbox
)
```

## Executor Hierarchy

```text
Executor (interface)
    |
    +-- DirectExecutor (no sandbox)
    |
    +-- DockerExecutor (ephemeral containers)
    |       |
    |       +-- PersistentDockerExecutor (stateful containers)
    |
    +-- NamespaceExecutor (Linux only)
    |
    +-- CompositeExecutor (routes by sandbox mode)
```

## Audit Trail Integration

All executors emit audit events as Mangle facts:

```datalog
execution_started("session-123", "req-456", "go", 1703001234).
execution_command("req-456", "go build ./...").
execution_completed("req-456", /success, 0, 2345).
resource_usage("req-456", 150, 78, 1048576, 23, 12).
```

File operations generate:
```datalog
file_read("/path/to/file.go", "session-123", 1703001234).
file_written("/path/to/new.go", "abc123", "session-123", 1703001235).
modified("/path/to/new.go").
```

## Dependencies

- `internal/logging` - Structured logging with CategoryTactile
- `os/exec` - Process execution
- `syscall` - Platform-specific process control
- Standard Docker CLI (optional)

## Testing

```bash
go test ./internal/tactile/...
```

## Subdirectories

- `python/` - Python environment management for SWE-bench
- `swebench/` - SWE-bench task orchestration

---

**Remember: Push to GitHub regularly!**


> *[Archived & Reviewed by The Librarian on 2026-01-25]*