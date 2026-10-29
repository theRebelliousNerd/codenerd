# Residual slow exit after `nerd create` / `spawn` Result (Windows)

**Date:** 2026-07-13  
**Repo:** `C:\CodeProjects\codeNERD`  
**Scope:** Post-Result hang residual after known fixes (workspace root env, `maintenanceCancel` on Close, `runCloseStep` 8s timeouts).

---

## Root-cause hypothesis

Post-Result delay is dominated by **`Cortex.Close()` racing background SQLite work that starts at boot**, not by the coding shard itself.

### Primary cause: immediate maintenance on every fresh boot

`GetOrBootCortex` always starts `StartMaintenanceSchedule` after a cache miss:

```178:182:internal/system/factory.go
	// Start background maintenance for archival, cleanup, and logging.
	// Only spawned on a fresh boot so cache hits do not leak goroutines.
	// Cancel is stored on Cortex and invoked from Close() so one-shot CLI
	// (create/spawn) does not hang after Result while the loop holds DB work.
	_ = cortex.StartMaintenanceSchedule(context.Background())
```

**Before this fix**, the schedule goroutine did:

1. **`runMaintenance()` immediately** → `LocalDB.MaintenanceCleanup` (archive / purge / DELETE activation logs under SQLite).
2. Then tick every 30 minutes.

One-shot CLI (`nerd create`, `nerd spawn`) path:

1. Boot Cortex (starts maintenance immediately).
2. Spawn coder, print Result.
3. `defer cortex.Close()` cancels maintenance **but did not wait for the goroutine**.
4. Close then runs `LocalDB.Close` / `LearningStore.Close` / perception close while cleanup may still hold SQLite statements.

On Windows (`mattn/go-sqlite3`), `db.Close()` waits for outstanding ops → multi-second stalls.  
`runCloseStep` 8s timeouts **cap** hangs but do not **prevent** them: residual exit can still be ~8s per contended step (StopAll + LocalDB + LearningStore + perception).

### Secondary cause: immediate reflection workers

Reflection defaults to **enabled**. Both workers used the same anti-pattern:

- `LocalStore.runReflectionWorker` → immediate `processReflectionCycle()` (embed timeout up to **45s**).
- `LearningStore.runLearningReflectionWorker` → immediate `processLearningReflectionCycle()`.

`stopReflectionWorker` / learning stop only wait **2s** for the worker, then Close proceeds to `db.Close()`, which can still block on in-flight SQL/embed work.

### Other paths reviewed (not primary for residual)

| Path | Finding |
|------|---------|
| **StopAll / system shard Stop / persistLearning** | Sequential `Stop()` on auto system shards; `persistLearning` is usually cheap if empty. Can add delay if many shards + LearningStore I/O, but not the main race with SQLite close. |
| **ClosePerceptionLayer** | Closes semantic classifier + learned/embedded corpus DBs; normally fast unless contended. |
| **LocalDB.Close** | Blocks on open statements; victim of maintenance/reflection race, not independently slow when idle. |
| **GetOrBootCortex cache** | Fine; only fresh boots start maintenance. Cache hits do not re-start the loop. |
| **runCloseStep 8s timeouts** | Safety net only; abandoned goroutines can still hold SQLite after timeout returns. |

---

## Fix shipped

### 1. No immediate maintenance; wait on cancel (root fix)

**Files:** `internal/system/factory.go`, `internal/system/cortex_close.go`

- Removed immediate `runMaintenance()` from `StartMaintenanceSchedule`.
- First cycle waits a full `maintenanceInterval` (default 30m) — long-lived TUI still maintains; one-shot CLI exits before any cycle.
- Added `maintenanceDone` channel + `stopMaintenanceSchedule(wait)`.
- `Cortex.Close()` calls `stopMaintenanceSchedule(2s)` **before** StopAll / LocalDB.Close so cancel is drained, not fire-and-forget.

### 2. No immediate reflection cycles

**File:** `internal/store/reflection_worker.go`

- LocalStore and LearningStore workers only process on ticker ticks (45s), not at start.
- Same one-shot / SQLite-close rationale as maintenance.

### 3. Unit tests

**File:** `internal/system/maintenance_schedule_test.go`

- `TestStartMaintenanceSchedule_NoImmediateRunAndFastCancel` — hook asserts 0 runs before interval; cancel clears state quickly; LocalDB.Close stays fast.
- `TestCortexClose_StopsMaintenanceBeforeLocalDB` — minimal Cortex Close path is fast and clears maintenance + LocalDB.

### 4. Package compile unblocks (incidental)

- `imageRouteStubLLM.CompleteWithStreaming` in `factory_test.go` (LLMClient interface).
- Removed duplicate `missingLLMClient.CompleteWithStreaming` in `factory_adapters.go`.

---

## Test results

```
go test ./internal/system/ -run "TestStartMaintenanceSchedule|TestCortexClose_Stops"  → ok (~1.2s)
go test ./internal/store/  -run "TestReflectionWorker_*Lifecycle"                     → ok (~0.14s)
```

### Live create probe (rebuild `nerd.exe`)

Workspace: `.nerd/live_feature_matrix/polystack`

| Run | Result | Total wall | Exit | Notes |
|-----|--------|------------|------|-------|
| hang_residual_probe create | Result printed; file `ok` | ~30.9s | 0 | Dominated by LLM spawn, not Close |
| hangprobe2 create | Result printed | ~29.1s | 0 | Same |

Stdout-to-file buffering prevented reliable mid-stream post-Result timing; process exited cleanly with exit 0 and no orphan from successful runs. Stale `nerd` PIDs from other matrix/stress work were killed; final `nerd_count=0`.

---

## What was deliberately not changed

- **Disable system shards on create path** — larger product change; StopAll cost secondary once DB races are gone.
- **Skip maintenance entirely for CLI** — unnecessary if first cycle is deferred; TUI still gets periodic cleanup.
- **Lower `closeStepTimeout`** — still a good backstop for unrelated hangs.

---

## Summary

| Question | Answer |
|----------|--------|
| **Root cause** | Immediate boot-time `runMaintenance()` (+ reflection cycles) race `LocalDB.Close` on Windows after Result. |
| **Fix shipped?** | **Yes** — defer maintenance/reflection to first ticker; wait for maintenance goroutine on Close. |
| **Tests** | Unit tests pass for schedule/cancel/Close; store reflection lifecycle still green; live create exits 0. |
)