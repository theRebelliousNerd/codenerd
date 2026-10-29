# Workflow: One-Shot CLI Exit (create/spawn must terminate)

## What It Stresses

- `Cortex.Close()` after one-shot `nerd create` / `nerd spawn` / `nerd run`
- Maintenance schedule cancel (`maintenanceCancel`) before LocalDB close
- Per-step Close timeouts (`closeStepTimeout` = 8s) so Windows SQLite / system-shard shutdown cannot hang process exit after `Result:` is printed
- Post-Result wall-clock latency (live matrix 2026-07 often saw ~90s+ hangs until kill)

## Why This Exists (2026-07 regression)

Live feature matrix (`%TEMP%\codenerd-live-matrix`) showed create/spawn print `Result:` then never exit. Harness killed processes ~45–90s post-result.

**Root causes (fixed e18d6818):**

1. Maintenance goroutine started with `context.Background()`; cancel discarded → still called `MaintenanceCleanup` while `Close` tore down LocalDB.
2. Individual Close steps (ShardManager.StopAll, LocalDB.Close, …) could block indefinitely on Windows SQLite locks.

**Fix surface:** `internal/system/factory.go` (store `maintenanceCancel`), `internal/system/cortex_close.go` (cancel first; `runCloseStep` with 8s bound).

## Severity Levels

| Level | Action |
|-------|--------|
| **Conservative** | Single `create` + measure post-Result exit; assert exit 0 and &lt; 15s after Result |
| **Aggressive** | create + spawn tester + run back-to-back serial; each must exit cleanly |
| **Chaos** | create then immediately another create (serial); grep logs for Close timeout warnings |
| **Hybrid** | create under `-w` + kill residual nerd.exe between steps; prove no orphan hang |

## Conservative Procedure (PowerShell / Windows)

```powershell
$ErrorActionPreference = "Stop"
$APP = Join-Path $env:TEMP "nerd-oneshot-exit-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $APP | Out-Null
# Prefer a workspace with working LLM config, or copy minimal .nerd/config.json

# Kill stale nerd processes that can hold SQLite (serial matrix only)
Get-Process nerd -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

$out = Join-Path $env:TEMP "oneshot-exit.out"
$err = Join-Path $env:TEMP "oneshot-exit.err"
$sw = [Diagnostics.Stopwatch]::StartNew()

$p = Start-Process -FilePath "nerd" -ArgumentList @(
  "create", "Write hangcheck.txt with content oneshot-ok",
  "-w", $APP, "--timeout", "10m"
) -NoNewWindow -PassThru -RedirectStandardOutput $out -RedirectStandardError $err

# Wait for process exit (hard cap: 12 minutes including LLM work)
$exited = $p.WaitForExit(720000)
$sw.Stop()

if (-not $exited) {
  Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
  throw "FAIL: nerd still running after hard cap (likely Close hang)"
}

# Parse when Result appeared vs process exit
$stdout = Get-Content $out -Raw
if ($stdout -notmatch "Result:") {
  Write-Warning "No Result: line — check LLM/auth; exit code=$($p.ExitCode)"
} else {
  # If Result is near end of file, post-result time ≈ remaining wall after Result write.
  # Proxy: total runtime after Result should be short; use log timestamps if available.
  Write-Host "ExitCode=$($p.ExitCode) WallMs=$($sw.ElapsedMilliseconds)"
}

# MUST pass:
if ($p.ExitCode -ne 0) { throw "FAIL: non-zero exit $($p.ExitCode)" }
Test-Path (Join-Path $APP "hangcheck.txt")   # true
# No orphan nerd after WaitForExit:
@(Get-Process nerd -ErrorAction SilentlyContinue).Count -eq 0

# Log: Close timeout soft-fails are allowed but should not prevent exit
$logDir = Join-Path $APP ".nerd\logs"
if (Test-Path $logDir) {
  Select-String -Path (Join-Path $logDir "*") -Pattern "Cortex.Close:.*timed out" -ErrorAction SilentlyContinue
  # Note any timeouts; process must still have exited
}
```

### Aggressive (serial chain)

```powershell
$APP = Join-Path $env:TEMP "nerd-oneshot-chain-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $APP | Out-Null

foreach ($cmd in @(
  @("create", "Write a.txt with a", "-w", $APP, "--timeout", "10m"),
  @("spawn", "tester", "Add a trivial test note in NOTES.md", "-w", $APP, "--timeout", "10m"),
  @("run", "Append line done to a.txt", "-w", $APP, "--timeout", "10m")
)) {
  Get-Process nerd -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  $sw = [Diagnostics.Stopwatch]::StartNew()
  & nerd @cmd
  $code = $LASTEXITCODE
  $sw.Stop()
  if ($code -ne 0) { throw "FAIL: $($cmd[0]) exit $code" }
  # Total wall includes LLM; focus is that Wait returns without kill
  Write-Host "$($cmd[0]) exit=$code duration=$($sw.Elapsed)"
}
```

## Pass Criteria

- [ ] Process exits without `Stop-Process` / harness kill after Result
- [ ] Exit code 0 for successful create/spawn (when LLM + tools succeed)
- [ ] Post-Result exit latency &lt; 15s typical; &lt; 30s worst-case under load (before fix: often never)
- [ ] `maintenanceCancel` path: no unbounded hang with LocalDB closed under maintenance
- [ ] Optional: Close-step timeout warnings may appear; they must not block process death
- [ ] No `debug_program_ERROR.mg`
- [ ] Serial only — concurrent nerd.exe against same SQLite is out of scope

## Log Patterns

| Pattern | Meaning |
|---------|---------|
| `Cortex.Close: LocalDB.Close timed out after 8s` | Bound fired; shutdown continued (e18d6818) |
| `Cortex.Close: ShardManager.StopAll timed out` | Shard stop stuck; still must exit |
| Hang with no Close log after Result | maintenanceCancel missing or Close not invoked |

## Related Surfaces

- Panic catalog **P0** / **P0c** (Close timeouts)
- [full-cli-surface.md](full-cli-surface.md) — post-Result exit gate
- `internal/system/cortex_close.go` — `closeStepTimeout`, `runCloseStep`
- `internal/system/factory.go` — `StartMaintenanceSchedule` + `maintenanceCancel`
