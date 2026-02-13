# internal/tactile/python - General Python Environment

This package provides a general-purpose Python project environment for containerized development and testing.

**Related Packages:**
- [internal/tactile](../CLAUDE.md) - Base execution infrastructure
- [internal/tactile/swebench](../swebench/CLAUDE.md) - SWE-bench harness consuming this

## Architecture

This is the foundation for all Python work in codeNERD - NOT benchmark-specific:
- Django, Flask, FastAPI web applications
- Data science / ML projects
- CLI tools and libraries
- Test suites of any complexity

SWE-bench, GitHub issues, and local debugging all use this foundation.

## File Index

| File | Description |
|------|-------------|
| `environment.go` | General-purpose containerized Python environment with state machine lifecycle. Exports `EnvironmentState` enum (Initializing→Ready→Testing→Complete), `EnvironmentConfig` (BaseImage/PythonVersion/Limits/Timeouts), `ProjectInfo` (Name/GitURL/Commit), `Environment` struct, and lifecycle methods (Initialize/Setup/Teardown/Reset/ApplyPatch/RunTests). |

## Key Types

### Environment
```go
type Environment struct {
    project   *ProjectInfo
    config    EnvironmentConfig
    executor  *PersistentDockerExecutor
    state     EnvironmentState
}
```

### EnvironmentConfig
```go
type EnvironmentConfig struct {
    BaseImage          string        // e.g., "python:3.10-slim"
    PythonVersion      string        // e.g., "3.10"
    MemoryLimit        int64         // Bytes (default 4GB)
    CPULimit           float64       // CPUs (default 2.0)
    NetworkEnabled     bool          // For pip install
    TestTimeout        time.Duration // Per-test timeout
    SetupTimeout       time.Duration // Total setup timeout
    WorkspaceDir       string        // Inside container
    EnableSnapshots    bool          // For state restore
    SnapshotAfterSetup bool          // Checkpoint ready state
}
```

### EnvironmentState
```go
const (
    StateInitializing EnvironmentState = "initializing"
    StateCloning      EnvironmentState = "cloning"
    StateSetup        EnvironmentState = "setup"
    StateReady        EnvironmentState = "ready"
    StatePatchApplied EnvironmentState = "patch_applied"
    StateTesting      EnvironmentState = "testing"
    StateComplete     EnvironmentState = "complete"
)
```

## Lifecycle

1. **Initialize**: Create persistent Docker container
2. **Setup**: Clone repo, install dependencies, run setup scripts
3. **Ready**: Environment ready for patches/tests
4. **ApplyPatch**: Apply model-generated patch
5. **RunTests**: Execute pytest with coverage
6. **Reset**: Restore to post-setup snapshot

## Dependencies

- `internal/tactile` - PersistentDockerExecutor, logging

## Testing

```bash
go test ./internal/tactile/python/...
```

---

**Remember: Push to GitHub regularly!**


> *[Archived & Reviewed by The Librarian on 2026-01-25]*