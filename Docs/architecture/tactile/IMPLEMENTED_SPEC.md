# tactile — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — **code-grounded full corpus**  
> Language: Go  
> Primary sources: `internal/tactile/`, `internal/tactile/python/`, `internal/tactile/swebench/`  
> Scale (approx.): **16** non-test Go sources ≈ **7.5k+** lines; **~12** test files; **0** local `.mg`

---

## 1. Overview

`internal/tactile` is the **motor cortex** of codeNERD: the lowest-level execution layer that physically interacts with the outside world—shell commands, process lifecycle, sandboxed containers, and audited file edits.

It follows the neuroscience metaphor used across the runtime:

| Layer | Package | Metaphor |
|-------|---------|----------|
| Perception | `internal/perception` | Sensory input (NL → atoms) |
| Kernel | `internal/core` + Mangle | Cognition |
| Articulation | `internal/articulation` | Speech (atoms → NL) |
| **Tactile** | **`internal/tactile`** | **Motor output** |

### Design principles (from package docs)

1. **Minimal logic** — Constitutional checks happen in VirtualStore, not here.  
2. **Sandboxing** — Docker, namespaces, firejail, and direct execution.  
3. **Resource limits** — CPU, memory, output size, process count, network flags.  
4. **Structured output** — `ExecutionResult` rich enough for kernel feedback.  
5. **Cross-platform** — Windows / Linux / macOS via build-tagged files.  
6. **Audit trail** — Execution and file events as facts for the kernel.

### High-level control flow

```
Caller (VirtualStore / campaign / CLI / e2e)
   │
   ├─ Executor.Execute(ctx, Command)
   │     ├─ Validate
   │     ├─ config.Merge(cmd)
   │     ├─ AuditEventStart
   │     ├─ run (os/exec | docker | namespaces | firejail | job object)
   │     ├─ fill ExecutionResult
   │     └─ AuditEventComplete | Killed | Error
   │
   └─ FileEditor.Read/Write/Edit/...
         ├─ FS ops
         └─ FileAuditEvent → file_read / file_written / lines_* / modified
```

Fact-flow position:

```
user_intent → kernel next_action → VirtualStore (permitted?)
  → tactile motor → audit facts → kernel.Assert → further reasoning / articulation
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Core types (`Command`, `ExecutionResult`, limits, sandbox) | **Implemented** | `types.go` |
| `Executor` interface family | **Implemented** | `executor_interface.go` |
| `DirectExecutor` | **Implemented** | Host `os/exec`, env allowlist, output caps |
| `DockerExecutor` (ephemeral) | **Implemented** | `docker run --rm`, network default none |
| `PersistentDockerExecutor` | **Implemented** | create/start/exec/snapshot/health |
| `CompositeExecutor` + factory | **Implemented** | Mode routing; Docker auto-register if available |
| `PooledExecutor` / `RetryExecutor` | **Implemented** | Pooling; retry has simplistic delay loop |
| `FileEditor` | **Implemented** | Read/write/edit/insert/delete/replace/info |
| Audit → Fact pipeline | **Implemented** | `audit.go` |
| Execution metrics + JSONL file log | **Implemented** | `ExecutionMetrics`, `AuditFileLogger` |
| Output analyzers (Go test/build) | **Implemented** | Pattern-based |
| Windows Job Objects | **Implemented** | `platform_windows.go` |
| Linux cgroups + NamespaceExecutor | **Implemented** | `platform_linux.go` |
| Firejail executor | **Implemented** | Linux build tag |
| Unix rusage / rlimits / process groups | **Implemented** | `platform_unix.go` |
| Darwin platform selection | **Implemented** | Docker or direct |
| `python.Environment` | **Implemented** | Full lifecycle on persistent Docker |
| `swebench` harness | **Implemented** | Thin wrapper + instance schema |
| Local Mangle corpus | **N/A** | Decls live under `internal/core/defaults/` |
| Universal boot on Composite+audit | **Partial** | Chat boot often uses bare DirectExecutor |
| Namespace/Firejail in default Composite | **Partial** | Composite registers none + docker only |
| Docker live integration tests | **Partial** | Arg-building heavily tested; live Docker optional |

**Overall:** mature living motor package — **not** pre-implementation. Heuristic readiness **~85–90%** for day-to-day coding agent shell/file use; sandbox edge cases and python/swebench adoption lag.

---

## 3. Source inventory

### 3.1 Layout

```
internal/tactile/
  types.go
  executor_interface.go
  direct.go
  docker.go
  persistent_docker.go
  factory.go
  files.go
  audit.go
  platform_windows.go      # windows
  platform_linux.go        # linux
  platform_linux_firejail.go
  platform_unix.go         # !windows
  platform_darwin.go       # darwin
  README.md
  python/
    environment.go
    environment_test.go
  swebench/
    harness.go
    instance.go
    *_test.go
  *_test.go
```

### 3.2 Largest non-test sources (approx. lines)

| Path | Lines | Role |
|------|------:|------|
| `platform_linux.go` | ~903 | cgroups, NamespaceExecutor, GetPlatformExecutor |
| `persistent_docker.go` | ~875 | Stateful container pool |
| `audit.go` | ~809 | Facts, metrics, analyzers, audited wrapper |
| `python/environment.go` | ~783 | Python project lifecycle |
| `platform_windows.go` | ~706 | Job objects, limited Windows executor |
| `files.go` | ~682 | FileEditor + file facts |
| `docker.go` | ~460 | Ephemeral Docker run |
| `factory.go` | ~446 | Composite, factory, pool, retry |
| `types.go` | ~417 | Domain types + DefaultExecutorConfig |
| `direct.go` | ~325 | Direct host execution |
| `platform_linux_firejail.go` | ~325 | Firejail sandbox |
| `swebench/instance.go` | ~320 | Instance/prediction/eval types + load |
| `swebench/harness.go` | ~215 | Evaluate wrapper |
| `platform_unix.go` | ~116 | rusage, rlimits common, kill group |
| `executor_interface.go` | ~55 | Interfaces |
| `platform_darwin.go` | ~44 | Platform selection |

### 3.3 Test files

| Path | Focus |
|------|--------|
| `audit_test.go` | Fact formatting, event→facts, metrics, analyzers |
| `coverage_boost_test.go` | File facts, factory, pool, retry, limited writer |
| `docker_platform_test.go` | `buildDockerArgs`, validate, capabilities |
| `docker_platform_windows_test.go` | Job objects, limited Windows, namespace stub |
| `files_test.go` | Read/write/edit/insert/delete |
| `types_coverage_test.go` / `tactile_test.go` | Types and smoke |
| `platform_linux_executor_test.go` | Linux path (build-tagged) |
| `python/environment_test.go` | Environment unit coverage |
| `swebench/*_test.go` | Instance/harness extras |

---

## 4. Domain model

### 4.1 Sandbox modes

```go
// types.go
SandboxNone      = "none"
SandboxDocker    = "docker"
SandboxNamespace = "namespace"  // Linux
SandboxFirejail  = "firejail"   // Linux
```

| Mode | Implementation | Isolation strength |
|------|----------------|--------------------|
| `none` | `DirectExecutor` (+ optional limited wrappers) | Host process only |
| `docker` | `DockerExecutor` | Container, network default `none` |
| `namespace` | `NamespaceExecutor` | clone flags (PID/net/mount/UTS/IPC/user) |
| `firejail` | `FirejailExecutor` | Firejail profile wrapper |

### 4.2 Command

Primary input contract for all executors:

| Field | Purpose |
|-------|---------|
| `Binary`, `Arguments` | Executable + argv |
| `WorkingDirectory` | CWD (default from config) |
| `Environment` | Extra `KEY=VALUE` (merged after allowlist) |
| `Stdin` | Optional stdin string |
| `Limits` | Timeout, memory, output, processes, network |
| `Sandbox` | Mode + image/mounts/caps |
| `SessionID`, `RequestID` | Correlation for audit/facts |
| `Tags` | Free-form audit tags → `execution_tag` facts |

`CommandString()` builds a display string for logging/facts.

### 4.3 ResourceLimits

| Field | Semantics |
|-------|-----------|
| `TimeoutMs` | Wall-clock kill via context |
| `MaxCPUTimeMs` | Platform-dependent CPU budget |
| `MaxMemoryBytes` | Job object / rlimit / cgroup / docker `--memory` |
| `MaxOutputBytes` | Capture cap (default 10MB via config) |
| `MaxFileSize` | rlimit FSIZE where available |
| `MaxProcesses` | pids limit / job active process |
| `NetworkAllowed` | Influences Docker network and namespace NewNet |

### 4.4 ExecutionResult — success semantics (critical)

| Condition | `Success` | Meaning |
|-----------|-----------|---------|
| Process ran, exit 0 | `true` | Clean command success |
| Process ran, exit ≠ 0 | `true` | **Infrastructure OK**, command failed |
| Timeout / cancel | `true`, `Killed=true` | Executor worked; process stopped |
| Spawn/exec infra failure | `false`, `Error` set | Binary missing, docker dead, etc. |

Helpers:

- `IsError()` — infra failure  
- `IsNonZeroExit()` — ran but exit ≠ 0  
- `Output()` — Combined or Stdout+Stderr  

This split is intentional so policy can distinguish “tool broken” from “tests failed”.

### 4.5 ExecutorConfig defaults

`DefaultExecutorConfig()` (`types.go`):

- Working dir `.`  
- Default timeout **30s**, max **10m**  
- Max output **10MB**  
- Allowlist env: PATH, HOME, Go-related, TEMP/TMP, Windows profile/system, caches, LANG  
- Default limits timeout 30s / 10MB output  
- Docker default image `alpine:latest`  
- Resource usage collection enabled  

`Merge(cmd)` fills empty working dir, limits, sandbox, and **caps** timeout at `MaxTimeout`.

---

## 5. Executor hierarchy (deep dive)

### 5.1 Interface stack

```
Executor
  Execute / Capabilities / Validate

AuditedExecutorInterface   + SetAuditCallback
LimitedExecutorInterface   + SetDefaultLimits
SandboxedExecutorInterface + SetDefaultSandbox / AvailableSandboxModes
CompositeExecutorInterface + RegisterExecutor
```

Defined in `executor_interface.go`.

### 5.2 DirectExecutor (`direct.go`)

**Role:** default host executor.

Flow:

1. `Validate` — binary required; rejects non-`none` sandbox.  
2. `Merge` defaults.  
3. Emit `AuditEventStart`.  
4. `context.WithTimeout` from limits/default.  
5. `exec.CommandContext`, env allowlist + cmd env, optional stdin.  
6. stdout/stderr via `limitedWriter`.  
7. Classify error (timeout → killed, ExitError → non-zero, other → infra error).  
8. Optional `getProcessResourceUsage` (platform).  
9. Emit complete/killed/error audit.

**Env policy:** only `AllowedEnvironment` keys from host + explicit `Command.Environment`. Prevents casual secret leakage from full `os.Environ()`.

### 5.3 DockerExecutor (`docker.go`)

**Role:** ephemeral isolation: `docker run --rm`.

- Detects docker via `LookPath` + `docker version`.  
- Requires `Sandbox.Mode == docker`.  
- `buildDockerArgs`: network (default **none** unless allowed or explicit mode), read-only root, tmpfs, no-new-privileges, cap-drop, user, mounts, workdir, env, memory/cpu/pids, `-i` for stdin, then image + binary + args.  
- `PullImage` / `ImageExists` helpers.

**Not:** stateful multi-step (use PersistentDocker).

### 5.4 PersistentDockerExecutor (`persistent_docker.go`)

**Role:** long-lived containers for iterative workflows (SWE-bench, Python envs).

Lifecycle:

```
CreateContainer (docker create … sleep infinity)
  → StartContainer
  → ExecInContainer (docker exec) × N
  → optional CreateSnapshot / RestoreSnapshot
  → Stop / Remove / Cleanup
```

Features:

- In-memory container map + snapshots  
- Background health ticker (`Start`/`Stop`)  
- Labels `codenerd.managed=true`  
- Defaults: max 10 containers, 30m idle, python:3.11-slim, 2GB / 2 CPU  
- Copy to/from container  

Does **not** implement the plain `Executor` interface as primary path; higher layers call container methods or wrap via `python.Environment`.

### 5.5 CompositeExecutor + factory (`factory.go`)

`NewCompositeExecutorWithConfig`:

- Always registers `DirectExecutor` for `SandboxNone` as default.  
- Registers `DockerExecutor` for `SandboxDocker` **if** available.  
- Does **not** auto-register Firejail/Namespace (those need `RegisterExecutor` or platform factory).

`selectExecutor`: mode from `cmd.Sandbox.Mode`, else none. Only absent/none uses
the default Direct executor; an explicitly unavailable mode returns an error.

Factory methods:

| Method | Returns |
|--------|---------|
| `CreateDirect` | Direct |
| `CreateDocker` | Docker or error |
| `CreateComposite` | Composite |
| `CreateBest` | `GetPlatformExecutor` |
| `CreateAudited` | Wrapper + new AuditLogger |
| `CreateFromConfig(mode)` | Mode-specific; firejail/namespace return errors (Linux-only creation expected elsewhere) |

**PooledExecutor:** channel pool of Direct executors; create on empty, drop on full.  
**RetryExecutor:** retries only narrow infra-ish cases (`!Success && ExitCode==-1`); **does not** retry kill or executor `error`. Delay implementation is a busy-ish loop (gap).

### 5.6 Platform ladder

#### Windows (`platform_windows.go`)

- `getProcessResourceUsage` via process times + optional memory/IO.  
- `JobObject` create/set limits/assign/terminate/stats.  
- `LimitedExecutorWindows` wraps Direct with job objects when limits present.  
- `WindowsContainerExecutor` detects Windows-mode Docker / Hyper-V.  
- `GetPlatformExecutor` currently returns Direct (Docker presence does not change return to Composite — note vs Darwin).  
- `NamespaceConfig` stub only.

#### Linux (`platform_linux.go`, firejail)

- `LimitedExecutorLinux` + `CgroupManager` (v1/v2 detect, setup, add process, stats, cleanup).  
- Falls back to Direct if cgroup setup fails.  
- `NamespaceExecutor` sets `SysProcAttr.Cloneflags` from `NamespaceConfig`.  
- `GetPlatformExecutor` preference: Firejail → Namespace (root/userns) → Limited cgroup → Direct.  
- `FirejailExecutor` wraps binary with firejail flags when mode firejail.

#### Darwin (`platform_darwin.go`)

- MaxRSS units differ (bytes).  
- `GetPlatformExecutor`: Composite if Docker available, else Direct.

#### Unix shared (`platform_unix.go`)

- rusage extraction, process group kill, common rlimits (AS, CPU, FSIZE).

---

## 6. File motor — FileEditor (`files.go`)

### Operations

| Method | Behavior |
|--------|----------|
| `ReadFile` / `ReadLines` | Scanner, large buffer; audit read |
| `WriteFile` | MkdirAll, trailing newline, old/new hash |
| `EditLines` | Inclusive 1-indexed replace range |
| `InsertLines` | Insert after line |
| `DeleteLines` | Range delete |
| `ReplaceElement` | Convenience for content replace |
| `GetFileInfo` / `FileExists` / `CreateDirectory` | Metadata helpers |
| `Exec` | Always errors — file surface only |

### File facts

| Op | Predicates |
|----|------------|
| read | `file_read(Path, SessionID, Timestamp)` |
| write | `file_written(Path, Hash, SessionID, Timestamp)`, `modified(Path)` |
| edit | `lines_edited(...)`, `modified` |
| insert | `lines_inserted(...)`, `modified` |
| delete | `lines_deleted(...)`, `modified` |
| patch | no facts in switch |

Hashes: SHA-256 over lines + newlines (`computeHash`).

VirtualStore uses `core.NewTactileFileEditorAdapter` so core does not import cycles on concrete file types (`virtual_store_codedom.go`).

---

## 7. Audit, facts, metrics (`audit.go`)

### 7.1 Fact type

Local `tactile.Fact` mirrors core facts **without importing core** (cycle avoidance). `String()` delegates to the canonical `types.Fact` renderer so paths are not mistaken for Mangle names.

### 7.2 Execution fact catalog

| Event | Predicates (summary) |
|-------|----------------------|
| start | `execution_started`, `execution_command`, optional `execution_working_dir` |
| complete | `execution_completed`, `execution_output`, success/nonzero/failure, optional resource/io, `execution_sandbox` |
| killed | `execution_killed` |
| error | `execution_error` |
| blocked | `execution_blocked` |
| sandboxed | `execution_sandboxed` |
| tags | `execution_tag` per key |

### 7.3 AuditLogger

- Multiple event callbacks  
- Optional fact callback  
- Optional JSONL `AuditFileLogger` with nanosecond-unique rotate, owner-only file
  mode, environment/stdin redaction, and 64 KiB per-output-field bounds
- Embedded `ExecutionMetrics` → snapshot (success rate, avg duration, by
  binary/session, audit-sink write failures and last error)
- `AuditedExecutorWrapper` emits a fallback start/terminal lifecycle when the
  wrapped executor has no native audit callback

### 7.4 OutputAnalyzer

- Go `--- PASS/FAIL/SKIP` and coverage %, with exact summary matching
- Go compiler `file.go:line:col: msg` diagnostics, including Windows drive paths
- Emit `execution_test_summary`, `execution_test_state`, `execution_failed_test`,
  `execution_test_coverage`, `execution_build_summary`, and
  `execution_diagnostic`; coverage is a bounded integer percent
- Completed `go test` and `go build` audit events are classified by exact binary
  and subcommand and append the analyzer facts automatically; other commands do
  not enter this path. Per-item failed-test and diagnostic facts are capped at
  100 per execution while the aggregate summary retains the full counts.

Not a general multi-language parser — honest scope.

---

## 8. Subpackages

### 8.1 `python` — Environment

State machine:

```
initializing → cloning → checkout → setup → ready
  → patch_applied → testing → complete | error
```

Uses `PersistentDockerExecutor` for:

- Initialize container  
- Clone / checkout  
- venv + install deps (network on)  
- Apply/revert patch, get diff  
- pytest single/all/named tests  
- Snapshot after setup when configured  

General-purpose (not SWE-only): any Git URL or local mount style project.

### 8.2 `swebench`

- `Instance` mirrors HuggingFace SWE-bench schema (FAIL_TO_PASS, PASS_TO_PASS, gold patch, etc.)  
- Load JSON / JSONL  
- `Harness` converts instance → `python.ProjectInfo`, workspace `/testbed`  
- `Evaluate(prediction)` → apply patch → FTP/PTP tests → `Resolved` metrics  

Thin wrapper by design (comments in harness.go).

---

## 9. Integration map

### 9.1 VirtualStore (`internal/core`)

| Integration | Detail |
|-------------|--------|
| Constructor | `NewVirtualStore(executor tactile.Executor)` |
| Modern path | `initModernExecutor` → Composite + AuditLogger → `injectTactileFact` |
| File path | `SetFileEditor` / adapter over `tactile.FileEditor` |
| Metrics | `GetAuditMetrics()` |
| Actions | Shell-ish actions call executor (see `virtual_store_actions.go`) |

### 9.2 Chat boot (`cmd/nerd/chat/session_boot.go`)

```go
executor := tactile.NewDirectExecutor()
virtualStore := core.NewVirtualStoreWithConfig(executor, vsCfg)
fileEditor := tactile.NewFileEditor()
fileEditor.SetWorkingDir(workspace)
virtualStore.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))
```

Also registers shard name `tactile_router` (routing concept; not the tactile package itself).

### 9.3 CLI

- `cmd_campaign.go` — DirectExecutor for campaign runs  
- `dom_cmd.go` / `dom_replace_cmd.go` — DirectExecutor + FileEditor for CodeDOM surgical ops  

### 9.4 Campaign package

- Orchestrator config embeds `tactile.Executor`  
- Checkpoint runner, assault tasks, micro-checkpoint shell out via `tactile.Command`  
- Tests inject DirectExecutor or dummies  

### 9.5 E2E

- `tests/e2e/*` use Composite or Direct with kernel/VirtualStore  

### 9.6 Logging

- Category `tactile`  
- Helpers: `Tactile`, `TactileDebug`, `TactileWarn`, `TactileError`  
- Timers on execute / file / docker exec  

### 9.7 Mangle decls (elsewhere)

Examples:

- `schemas_shards.mg`: `execution_started`, `execution_completed`, …  
- `schemas_codedom.mg`: `file_read`, `file_written`  

Every predicate currently emitted by `AuditEvent.ToFacts`, `TestAnalysis.ToFacts`,
and `BuildAnalysis.ToFacts` has a matching Decl. Policy consumption remains
selective; unused facts are telemetry rather than executive state.

---

## 10. Concurrency and state

| Component | Concurrency notes |
|-----------|-------------------|
| Direct/Docker | Per-call; mutex on audit callback |
| Composite | RWMutex around executor map |
| PersistentDocker | RWMutex on container map; background health goroutine |
| FileEditor | RWMutex for callbacks/workdir |
| AuditLogger / Metrics | Mutexes on shared counters |
| PooledExecutor | Channel + stats mutex |

Not a process-global singleton; callers own instances.

---

## 11. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md). Headline gaps:

1. Boot often uses Direct without Composite audit path.  
2. Firejail/namespace not in default Composite registration.  
3. RetryExecutor delay not wall-clock sleep.  
4. Windows `GetPlatformExecutor` does not return Composite when Docker available (unlike Darwin).  
5. SWE-bench/python adoption outside package is limited.  
6. Policy consumption of every emitted fact is uneven.

---

## 12. Non-goals of this corpus revision

- Rewriting function bodies line-by-line  
- Implementing missing features  
- Inventing VirtualStore routes not in source  
- Vectryx product terminology  

---

## 13. Verify commands

```powershell
go test ./internal/tactile/...
go test ./internal/tactile/python/...
go test ./internal/tactile/swebench/...
```

---

## 14. Glossary

| Term | Meaning |
|------|---------|
| Motor cortex | Tactile package role metaphor |
| Infrastructure success | Process was spawned; exit code separate |
| Ephemeral Docker | One container per command (`--rm`) |
| Persistent Docker | Reused container via `docker exec` |
| Audit fact | Structured Mangle-bound atom from motor events |
| Allowlisted env | Subset of host env passed to processes |

---

*End of flagship IMPLEMENTED_SPEC for tactile.*
