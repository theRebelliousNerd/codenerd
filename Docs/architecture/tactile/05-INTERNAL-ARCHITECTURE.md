# tactile — Internal Architecture

> Last verified: **2026-07-13**

## Component diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                         Callers                                  │
│  VirtualStore · Campaign · CLI(dom/campaign) · e2e · python      │
└────────────────────────────┬─────────────────────────────────────┘
                             │
         ┌───────────────────┼────────────────────┐
         ▼                   ▼                    ▼
   FileEditor          Executor family     PersistentDockerExecutor
         │                   │                    │
         │         ┌─────────┴──────────┐         │
         │         ▼                    ▼         │
         │   CompositeExecutor    Single backends │
         │         │                  │           │
         │    mode map:               │           │
         │    none → Direct           │           │
         │    docker → Docker         │           │
         │   (+ Register NS/FJ)       │           │
         │         │                  │           │
         │         └────────┬─────────┘           │
         │                  ▼                     ▼
         │           os/exec / docker CLI    docker create/exec
         │                  │                     │
         └────────┬─────────┴─────────────────────┘
                  ▼
           AuditEvent / FileAuditEvent
                  ▼
           AuditLogger (optional) → Fact callback → kernel (via VS)
                  │
                  ▼
           ExecutionMetrics / JSONL file
```

## Data flow — command execution

```
Command
  → Validate(cmd)
  → ExecutorConfig.Merge(cmd)
  → AuditEventStart
  → context.WithTimeout
  → spawn + capture (limitedWriter)
  → classify: ok | ExitError | Deadline | Cancel | other
  → ResourceUsage? (platform)
  → AuditEventComplete|Killed|Error
  → *ExecutionResult
```

## Data flow — file edit

```
path + lines
  → resolvePath(workingDir)
  → ReadFile (optional for hash/undo)
  → mutate line slice
  → WriteFile
  → FileAuditEvent + Facts
  → *FileResult
```

## State machines

### Persistent container

```
(creating) → stopped ──start──► running ──stop──► stopped ──rm──► ∅
                │                  │
                │                  ├──exec──► (still running)
                │                  └──snapshot──► image tag
                └──error
```

States: `ContainerStateCreating|Running|Paused|Stopped|Error` (`persistent_docker.go`).

### Python environment

```
initializing → cloning → checkout → setup → ready
                                            │
                                     apply patch → patch_applied
                                            │
                                         testing → complete
                                            │
                                         error (any stage)
```

Constants in `python/environment.go`.

### Audit event types

```
start → complete
      → killed
      → error
blocked / sandboxed (signal events)
```

## Key type relationships

```
Command ──uses──► ResourceLimits
       ──uses──► SandboxConfig ──► SandboxMode
       ──yields─► ExecutionResult ──► ResourceUsage

Executor ◄── DirectExecutor
         ◄── DockerExecutor
         ◄── CompositeExecutor
         ◄── PooledExecutor / RetryExecutor
         ◄── *Limited / Namespace / Firejail (platform)

AuditEvent ──ToFacts──► []Fact
FileAuditEvent ──ToFacts──► []Fact
AuditedExecutorWrapper ──wraps──► Executor + AuditLogger
```

## Factory routing (CreateFromConfig)

| Mode | Result |
|------|--------|
| none | Direct |
| docker | Docker if available else error |
| firejail | error (must construct on Linux) |
| namespace | error (must construct on Linux) |
| other | error |

## Platform selection algorithm (Linux GetPlatformExecutor)

```
if firejail available → FirejailExecutor
else if root or userns → NamespaceExecutor
else if cgroups writable → LimitedExecutorLinux
else → DirectExecutor
```

## Shared utilities

| Utility | Purpose |
|---------|---------|
| `limitedWriter` | Cap stdout/stderr capture |
| `createRlimits` / `createRlimitsCommon` | Unix rlimits map |
| `killProcessGroup` / `setupProcessGroup` | Tree kill |
| `getProcessResourceUsage` | rusage / Windows counters |
| `CgroupManager` | Linux cgroup lifecycle |
| `JobObject` | Windows job lifecycle |

## Boundary rules

- **In:** context, Command/options, config  
- **Out:** ExecutionResult/FileResult, side effects on OS, optional Facts  
- **Not in:** kernel types, prompt atoms, perception  
