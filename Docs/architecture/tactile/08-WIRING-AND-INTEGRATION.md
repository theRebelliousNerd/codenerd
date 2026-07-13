# tactile — Wiring and Integration

> Last verified: **2026-07-13**

## Principle

Tactile is a **library motor**. Nothing auto-registers it as a singleton at process start. Callers construct executors/editors and attach them to VirtualStore, campaign orchestrators, or CLI tools.

## Boot path — interactive chat

**File:** `cmd/nerd/chat/session_boot.go`

```
kernel.Evaluate
  → tactile.NewDirectExecutor()
  → core.NewVirtualStoreWithConfig(executor, vsCfg)
  → virtualStore.SetKernel(kernel)
  → tactile.NewFileEditor(); SetWorkingDir(workspace)
  → virtualStore.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))
```

**Shard name:** `tactile_router` is registered on the shard manager as an action-routing concept (chat-specific). This is **not** the tactile package exporting a shard agent type from `internal/tactile`.

**Implication:** primary UX path historically favors **Direct** (no sandbox routing) unless VirtualStore upgrades to modern composite internally.

## VirtualStore modern executor

**File:** `internal/core/virtual_store.go` — `initModernExecutor`

```
DefaultExecutorConfig
  + WorkingDir / AllowedEnvironment from VS
  → NewCompositeExecutorWithConfig
  → NewAuditLogger
  → SetFactCallback → injectTactileFact
  → composite.SetAuditCallback(logger.Log)
  → modernExecutor + useModernExecutor flag
```

`injectTactileFact` maps `tactile.Fact` → `core.Fact`, normalizes some status strings to Mangle atoms, `kernel.Assert`.

When modern path is active, shell actions should hit composite + audit. Confirm call sites honor `useModernExecutor` vs legacy `executor` field (both exist on VirtualStore).

## File editor adapter

**File:** `internal/core/virtual_store_codedom.go`

```
tactile.FileEditor
  → NewTactileFileEditorAdapter
  → core.FileEditor interface methods
  → convertResult(*tactile.FileResult) → *FileEditResult
```

Breaks import cycle: core defines interface; tactile implements concrete; adapter in core package.

## Campaign wiring

**Types:** `internal/campaign/orchestrator_types.go` — `Executor tactile.Executor` in config.

**Construction examples:**

- Tests: `tactile.NewDirectExecutor()`  
- Assault: `newAssaultExecutor` builds `NewDirectExecutorWithConfig` with workspace/output/timeout  
- Checkpoint: `NewCheckpointRunner(executor, …)`  

**Execution:** handlers build `tactile.Command{Binary, Arguments, WorkingDirectory, …}` and call `Execute`.

## CLI wiring

| Command surface | Wiring |
|-----------------|--------|
| Campaign Cobra | DirectExecutor |
| DOM commands | DirectExecutor + FileEditor |
| DOM replace | FileEditor for surgical replace |

## E2E wiring

`tests/e2e/*` construct Composite or Direct and pass into VirtualStore / session fixtures to exercise cross-boundary fact flow.

## Logging wiring

No registration required. First log call uses `logging.Get(CategoryTactile)` via helpers. Category constant in `internal/logging/logger.go`.

## Mangle wiring

Tactile **does not load `.mg` files**. Declared predicates live under `internal/core/defaults/`:

| Area | Example file |
|------|----------------|
| Execution | `schemas_shards.mg` (`execution_started`, `execution_completed`, …) |
| Files | `schemas_codedom.mg` (`file_read`, `file_written`) |
| Notes | `schemas_execution.mg` comments on overlapping decls |

If audit emits a predicate without Decl, kernel Assert may fail (logged as inject error).

## Registration checklist (for new integrations)

1. Construct Executor via factory or Direct.  
2. If facts needed: AuditLogger + callback into kernel.Assert (or use VS modern init).  
3. Set FileEditor on VS for line ops.  
4. Ensure sessionID/requestID on Command for correlatable facts.  
5. Prefer VirtualStore action entry so `permitted` runs first.  
6. Grep for dormant hooks before claiming unused.

## Wiring gaps (honest)

| Gap | Detail |
|-----|--------|
| Dual executor fields on VS | legacy `executor` vs `modernExecutor` |
| Chat boot Direct | may skip composite docker routing |
| Firejail/NS | not registered in Composite by default |
| python/swebench | library-only; sparse outer CLI |

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).
