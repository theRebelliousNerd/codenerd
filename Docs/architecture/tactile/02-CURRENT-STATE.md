# tactile — Current State

> Last verified: **2026-07-13**  
> Source root: `C:\CodeProjects\codeNERD\internal\tactile\`

## Snapshot

| Metric | Value |
|--------|------:|
| Non-test `.go` files (root + python + swebench) | **16** |
| Test `.go` files | **~12** |
| Local `.mg` | **0** |
| Package README | yes (`internal/tactile/README.md`) |
| Architecture version claim in package README | 2.0.0 (Dec 2024 JIT-driven framing) |
| Primary dependency out | `internal/logging` |
| Subpackage deps | `python` → `tactile`; `swebench` → `tactile` + `python` |

## File roles (precise)

### Root package

| File | Build tags | Role / hotspot |
|------|------------|----------------|
| `types.go` | all | Domain types, defaults, merge |
| `executor_interface.go` | all | Executor contracts |
| `direct.go` | all | Host execution; limitedWriter |
| `docker.go` | all | Ephemeral docker run |
| `persistent_docker.go` | all | Stateful container pool |
| `factory.go` | all | Composite, factory, pool, retry |
| `files.go` | all | FileEditor motor |
| `audit.go` | all | Facts, metrics, analyzers |
| `platform_windows.go` | `windows` | Job objects, limited Windows, Windows containers |
| `platform_linux.go` | `linux` | cgroups, namespaces, platform ladder |
| `platform_linux_firejail.go` | `linux` | FirejailExecutor |
| `platform_unix.go` | `!windows` | rusage, rlimits common, kill pg |
| `platform_darwin.go` | `darwin` | Darwin GetPlatformExecutor |
| `README.md` | — | Package overview |

### Subpackages

| File | Role |
|------|------|
| `python/environment.go` | Containerized Python project lifecycle |
| `swebench/instance.go` | Dataset types + loaders |
| `swebench/harness.go` | Evaluate / FTP-PTP orchestration |

## Hotspots (complexity / risk)

1. **`platform_linux.go`** — cgroup v1/v2, namespace clone flags, platform selection; privilege-sensitive.  
2. **`persistent_docker.go`** — lifecycle + background health; orphaned containers if process dies without Cleanup.  
3. **`audit.go`** — fact schema surface area for kernel Decl alignment.  
4. **`docker.go` `buildDockerArgs`** — security-relevant defaults (network none).  
5. **`factory.go` RetryExecutor** — retry policy + delay loop quality.  
6. **`files.go`** — line-index math (1-based inclusive) for CodeDOM.

## What is production-hot

| Path | Typical production use |
|------|------------------------|
| DirectExecutor | Chat boot, campaign, DOM CLI |
| FileEditor | VirtualStore CodeDOM adapter, DOM replace |
| Audit facts | When modern executor / callbacks wired |
| Docker / Persistent / python / swebench | Optional / lab / benchmark paths |

### Docker selection receipt

**VERIFIED CURRENT:** `internal/tactile/docker.go#detectDocker` caches a
bounded Docker probe for 30 seconds, including negative results, so repeated
`VirtualStore` construction does not launch one five-second `docker version`
probe per instance. The probe is single-flight under a mutex.
`internal/tactile/factory.go#NewCompositeExecutor` passes its caller-supplied
`ExecutorConfig` to `NewDockerExecutorWithConfig`; availability checks therefore
measure the requested binary/configuration instead of silently using defaults.
Focused evidence is recorded in [_progress.md](_progress.md).

## What is dormant or sparse

| Surface | Note |
|---------|------|
| Namespace / Firejail via Composite | Not auto-registered |
| PooledExecutor | Available; few callers observed |
| RetryExecutor | Available; delay implementation weak |
| WindowsContainerExecutor | Detect + caps; not primary boot path |
| swebench Evaluate | Library ready; limited outer wiring |

## Status vs north star

| Expectation | Reality |
|-------------|---------|
| Motor only | Yes — no intent classification |
| Sandbox options | Implemented with platform variance |
| Facts for kernel | Implemented; consumption uneven |
| Policy inside tactile | Correctly **not** implemented |

## Inventory counts by area

| Area | Src files | Dominant concern |
|------|----------:|------------------|
| Core execute | 5 | Direct, docker, factory, types, interface |
| Audit/files | 2 | Facts + FS |
| Platform | 5 | OS isolation/limits |
| Python | 1 | Env lifecycle |
| SWE-bench | 2 | Benchmark wrapper |
