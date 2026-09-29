---
doc-class: governance
subsystem: browser
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: 6597099c
supersedes: []
---

# ADR-003: Execute Chrome Process Tree Reaper at Lifecycle Edges Only

## Context

Automating headless or windowed Chrome through Chrome DevTools Protocol (CDP) spawns multiple cooperating processes: the browser broker, network service, GPU process, and per-tab renderer utilities. When test runs are aborted, network connections drop, or the host application terminates abruptly, these Chrome processes often continue running in the background as orphans. Over time, orphaned instances accumulate, exhausting system memory, occupying network debugger ports, and retaining file locks on disk profiles.

To address this, upstream BrowserNERD designed an uncommitted process-reaper subsystem (`reaper.go`, `reaper_windows.go`, `reaper_unix.go`, `reaper_test.go`).

A key architectural question arises regarding the governance and exposure of this capability:
- Option A: Expose process reaping as an agent-facing tool (e.g. `browser_reap_processes` or `browser_kill`), allowing the model to detect and terminate stuck Chrome processes during an active session.
- Option B: Restrict process reaping strictly to infrastructure lifecycle edges (startup and shutdown), completely shielding process termination from LLM agency.

Furthermore, Rod's default launcher incorporates a companion helper (`leakless`) intended to monitor the parent process. On Windows systems, endpoint protection software (notably Microsoft Defender) frequently quarantines the `leakless` executable as a false positive, causing browser launches to fail intermittently.

## Decision

1. **Reaper Runs Exclusively at Lifecycle Edges**:
   - The process reaper is strictly an infrastructure lifecycle guard. It is **never** exposed as an LLM tool. The model is never granted the ability to terminate operating system processes.
   - **Startup Edge (`SessionManager.Start()`)**: Invokes `ReapOrphans(ctx)`. The scanner queries the operating system for running Chrome processes whose command-line parameters contain a BrowserNERD or Rod temporary user-data directory, inspects the parent process ID (PPID), and terminates the tree if the launcher process is dead.
   - **Shutdown Edge (`SessionManager.Shutdown()` / `CloseBrowser()`)**: Calls `KillProcessTree(pid)` on the tracked root PID and safely removes the associated temporary profile.
2. **Defensive Directory Deletion Guardrails**:
   - Directory cleanup via `removeUserDataDir` enforces strict safety validation (`isBrowserNERDUserDataDir`).
   - Only paths that explicitly contain designated temporary markers (`rod/user-data/`, `browsernerd/user-data/`) under system temporary directories are eligible for deletion.
   - Any path matching standard user profile directories (e.g. personal Chrome data under `AppData` or `~/.config`) is met with a hard safety refusal error.
3. **Hardened Launcher Configuration**:
   - Chrome is launched with `Leakless(false)` unconditionally across all platforms (`internal/browser/session_lifecycle.go:149-180`).
   - Lifecycle cleanup responsibility is assumed entirely by codeNERD's native reaper and process tree termination handles.

## Consequences

### Positive
- Completely eliminates orphaned Chrome processes and disk accumulation across test runs and development sessions.
- Preserves constitutional containment: the model cannot abuse process-killing tools to interfere with host processes.
- Eliminates launcher failures caused by antivirus quarantine of Rod's leakless binary.
- Protects personal user browser profiles from accidental modification or deletion.

### Negative
- Process cleanup occurs only on system restart or browser shutdown; intermediate orphaned tabs within an active, long-lived session must be managed via tab-level reaper routines rather than operating system process kills.

## Witness

**Witness:** Test `TestReapOrphans_ReapsOnlyDeadParentTrees` in `internal/browser/reaper_test.go` and method `ReapOrphans` in `internal/browser/reaper.go`.
