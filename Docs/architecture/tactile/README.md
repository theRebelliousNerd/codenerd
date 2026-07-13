# tactile — Architecture Corpus (`internal/tactile`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — code-grounded full corpus  
> Language: Go (module `codenerd`)  
> Primary package: `internal/tactile/` (+ `python/`, `swebench/`)  
> Role in stack: **Motor cortex** — lowest-level physical-world effectors (shell, files, sandbox)

## Scope

This corpus documents the **tactile motor layer**: command execution, resource limits, sandbox backends, file line edits with audit facts, persistent Docker pools, Python environments, and SWE-bench harness wrappers.

It is **not**:

- Constitutional policy (lives in kernel / VirtualStore / Mangle policy)
- Prompt JIT or articulation
- A product Spec template set under `Docs/Spec/`

Tactile intentionally does **minimal logic**. Permission is decided upstream; tactile executes, captures results, and emits structured audit facts for the kernel.

## North-star placement

```
user input → perception → user_intent → kernel next_action
  → VirtualStore (permitted?) → tactile Executor / FileEditor
  → AuditEvent.ToFacts / FileAuditEvent.ToFacts → kernel.Assert
  → articulation → TUI/stdout
```

| Role | Owner |
|------|--------|
| Creative center (what to try) | LLM via session / shards |
| Executive (may it run?) | Mangle `permitted(...)` via VirtualStore |
| Motor (actually run / write) | **`internal/tactile`** |
| Memory of effects | Audit facts + metrics → kernel |

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory + deep flows |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision for tactile |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory, hotspots, metrics |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types/funcs that matter |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, VirtualStore, CLI, campaign, e2e |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, fact contracts |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging category, metrics, timers |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open questions |
| [_progress.md](_progress.md) | Rebuild journal |

> Older thin auto-stubs (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-TACTILE.md`, …) if still present are **superseded** by this map.

## Package tree (source)

```
internal/tactile/
├── types.go                 # Command, Result, limits, sandbox, config
├── executor_interface.go    # Executor + specialized interfaces
├── direct.go                # DirectExecutor (host os/exec)
├── docker.go                # DockerExecutor (ephemeral docker run --rm)
├── persistent_docker.go     # Stateful docker create/exec pool
├── factory.go               # Composite, factory, pool, retry
├── files.go                 # FileEditor + file audit facts
├── audit.go                 # AuditEvent → Fact, metrics, analyzers
├── platform_windows.go      # Job objects, LimitedExecutorWindows
├── platform_linux.go        # cgroups, NamespaceExecutor, GetPlatformExecutor
├── platform_linux_firejail.go
├── platform_unix.go         # rusage, rlimits, process groups
├── platform_darwin.go       # macOS platform executor selection
├── python/environment.go    # Containerized Python project lifecycle
└── swebench/                # Thin SWE-bench harness over python.Environment
```

## Verify

```powershell
# Unit + package tests
go test ./internal/tactile/...

# With verbose
go test -v ./internal/tactile/...

# Broader consumers (optional)
go test ./internal/core/ -count=1 -run VirtualStore
go test ./internal/campaign/ -count=1
```

No local `.mg` files live in this package. Execution/file predicates are declared in `internal/core/defaults/` (e.g. `schemas_shards.mg`, `schemas_codedom.mg`).

## Quick mental model

```
                    ┌─────────────────────┐
 Command / file op  │  CompositeExecutor  │── mode ──► Direct | Docker | NS | Firejail
                    │  FileEditor         │──► host FS (audited)
                    └──────────┬──────────┘
                               │ AuditEvent / FileAuditEvent
                               ▼
                    AuditLogger.Log → Fact → VirtualStore.injectTactileFact → kernel
```

## Related packages

| Package | Relationship |
|---------|----------------|
| `internal/core` | VirtualStore holds Executor + FileEditor; injects tactile facts |
| `internal/campaign` | Checkpoint / assault / task handlers call `tactile.Executor` |
| `cmd/nerd` | Boot constructs DirectExecutor + FileEditor; DOM cmds use FileEditor |
| `internal/logging` | `CategoryTactile` + convenience helpers |
| `internal/session` | Task/campaign paths consume executor results indirectly |

---

**Architecture Version (package README):** 2.0.0 motor-cortex framing — corpus rebuilt 2026-07-13.
