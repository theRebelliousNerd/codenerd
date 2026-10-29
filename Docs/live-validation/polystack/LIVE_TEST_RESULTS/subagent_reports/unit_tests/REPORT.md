# Unit Tests Report — recent codeNERD fixes

**Date:** 2026-07-13  
**Workspace:** `C:\CodeProjects\codeNERD`  
**CGO:** `$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"`

## Summary

| Package | Result | Notes |
|---------|--------|-------|
| `internal/system` (focus: workspace root, Close, maintenance, close timeouts) | **PASS** | All focus tests green |
| `internal/system` (non-boot suite) | **PASS** | 44 tests, ~4.5s |
| `internal/system` (full package incl. Boot) | **HANG / FAIL** | Pre-existing: Boot hangs on Ollama embed |
| `internal/tools/core` (full package) | **PASS** | All tests green, ~0.13s |

**Overall focus status: PASS**

---

## Commands run

```powershell
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"

# Focus system (workspace + Close + maintenance + close-step)
go test ./internal/system/ -count=1 -timeout 90s `
  -run "TestResolveWorkspaceRoot|TestCortexClose|TestStartMaintenance|TestRunCloseStep|TestCortexKey" -v
# → PASS ok ~2.7s

# Non-boot system suite (excludes BootCortex / CodeDOM)
go test ./internal/system/ -count=1 -timeout 120s `
  -run "TestDiscover|TestSync|TestNewHolo|TestHolographic|TestCortexClose|TestRunClose|TestMCP|TestCortexKey|TestResolve|TestNormalize|TestCortex_Spawn|TestSessionVirtual|TestStartMaint|TestSessionKernel|TestVirtualStore" -v
# → PASS ok ~4.5s

# Full tools/core
go test ./internal/tools/core/ -count=1 -timeout 90s -v
# → PASS ok ~0.13s

# Full system package (includes Boot) — hangs
go test ./internal/system/ -count=1 -timeout 60s -run "TestBootCortexEndToEnd|TestBootCortexWithConfig"
# → FAIL timeout 1m; stuck in perception.InitSemanticClassifier → OllamaEngine.EmbedBatch
```

---

## Focus test results (`internal/system`)

| Test | Result |
|------|--------|
| `TestResolveWorkspaceRoot` | PASS (fixed for Windows `filepath.Abs`) |
| `TestResolveWorkspaceRoot_SetsCodenerdWorkspaceEnv` | PASS |
| `TestCortexClose_WhenNil_ShouldReturnNil` | PASS |
| `TestCortexClose_StopsMaintenanceBeforeLocalDB` | PASS |
| `TestCortexClose_CancelsMaintenance` | PASS (**new**) |
| `TestStartMaintenanceSchedule_NoImmediateRunAndFastCancel` | PASS |
| `TestRunCloseStep_Success` | PASS (**new**) |
| `TestRunCloseStep_Timeout` | PASS (**new**) |
| `TestRunCloseStep_PropagatesError` | PASS (**new**) |
| `TestCortexKey` | PASS |

---

## `internal/tools/core` results

Full package **PASS**, including:

| Area | Tests | Result |
|------|-------|--------|
| Workspace guard | `TestResolveWorkspacePath` (+ empty path case) | PASS |
| Workspace root env | `TestWorkspaceRoot_PrefersCodenerdEnv` (**new**) | PASS |
| Workspace root cwd | `TestWorkspaceRoot_FallsBackToCwd` (**new**) | PASS |
| Env root resolve | `TestResolveWorkspacePath_UsesEnvWhenRootEmpty` (**new**) | PASS |
| Write outside workspace | `TestWriteFileTool_Execute_OutsideWorkspace` (**new**) | PASS |
| File ops / search / register | existing suite | PASS |

---

## New tests added

### `internal/system`

1. **`TestCortexClose_CancelsMaintenance`** — `maintenance_schedule_test.go`  
   Starts maintenance with a long interval + hook, calls `Close()` without full Boot, asserts cancel/done fields cleared, hook never fired, second `Close()` safe.

2. **`TestRunCloseStep_Success`** — `cortex_close_test.go` (new file)  
   Step completes under timeout.

3. **`TestRunCloseStep_Timeout`** — `cortex_close_test.go`  
   Blocking step with 40ms timeout returns `timed out` promptly (<500ms), not after full sleep.

4. **`TestRunCloseStep_PropagatesError`** — `cortex_close_test.go`  
   Step errors propagate.

### `internal/tools/core`

5. **`TestWorkspaceRoot_PrefersCodenerdEnv`**
6. **`TestWorkspaceRoot_FallsBackToCwd`**
7. **`TestResolveWorkspacePath_UsesEnvWhenRootEmpty`**
8. **`TestWriteFileTool_Execute_OutsideWorkspace`** — write path rejected outside `CODENERD_WORKSPACE_ROOT`
9. Extra `empty path` case on `TestResolveWorkspacePath`

---

## Supporting fixes (not feature work)

| Change | Why |
|--------|-----|
| `TestResolveWorkspaceRoot` uses `t.TempDir()` + `filepath.Abs` | Was asserting Unix-style `"/explicit/path"`; on Windows Abs yields `C:\explicit\path` → FAIL |
| `missingLLMClient.CompleteWithStreaming` on `factory.go` | Keeps stub aligned with `types.LLMClient` after streaming method was added |
| `errors` import in `file_ops_test.go` | Required by outside-workspace assertion |

(`imageRouteStubLLM` already had `CompleteWithStreaming` in tree.)

---

## Known non-focus failures

### Full `go test ./internal/system/` hangs

- **Cause:** `TestBootCortexEndToEnd` / `TestBootCortexWithConfig_*` call `BootCortex` → `initKernel` → `InitPerceptionLayer` → `OllamaEngine.EmbedBatch` and block on HTTP to local Ollama.
- **Evidence:** stack shows `codenerd/internal/embedding.(*OllamaEngine).Embed` / `EmbedBatch` under `NewSemanticClassifierFromConfig`.
- **Not introduced by this unit-test pass.** Focus Close/workspace tests intentionally avoid full Boot.

### CodeDOM tests

- Not run in this pass (excluded with boot from focus suite).

---

## Files touched

| Path | Change |
|------|--------|
| `internal/system/factory.go` | `CompleteWithStreaming` on `missingLLMClient` |
| `internal/system/factory_helpers_test.go` | Windows-safe `TestResolveWorkspaceRoot` |
| `internal/system/maintenance_schedule_test.go` | `TestCortexClose_CancelsMaintenance` |
| `internal/system/cortex_close_test.go` | **new** — `runCloseStep` unit tests |
| `internal/tools/core/workspace_guard_test.go` | env/cwd/empty-path cases |
| `internal/tools/core/file_ops_test.go` | outside-workspace write test + `errors` import |

---

## Verdict

- **Focus packages / recent fixes: PASS**
- **New Close/maintenance/timeout/workspace-guard coverage: added and green**
- **Full `internal/system` package including Boot: FAIL (hang)** — environmental / Boot→Ollama dependency, out of focus scope
