---
doc-class: shipped-with-future
subsystem: browser
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472
supersedes: []
---

# B3 process lifecycle contract

This supplements the legacy corpus and capability group 2 of
`14-BROWSERNERD-1.2-PORT-SPEC.md`. The executive remains the live kernel;
process cleanup is a private driver responsibility, never a model tool.

## Required behavior

- Every local launch disables leakless and records the Chrome PID, profile
  path, and whether codeNERD allocated that temporary profile. Explicit
  profiles and debugger attachments are never eligible for directory deletion.
- Startup scans once, before launch. Only direct profiles beneath resolved
  system-temp `codenerd/user-data`, `rod/user-data`, or `browsernerd/user-data`
  containers qualify. Live launchers, ambiguous ownership, and symlink escapes
  fail closed. Process/query/cleanup failures must remain visible.
- Failed launches/connections, stale connections, browser close, and shutdown
  kill tracked process trees and remove allocated profiles after confirming
  exit. Shutdown visits tabs before browser processes.
- CreateTab registers an about:blank session and starts its stream before
  navigation. Stream subscription readiness belongs to B2. Navigation waits
  for DOM parsing, and navigating to the current URL explicitly reloads.
- Detached metadata does not consume tab capacity.
- Cleanup timeout and retry interval are per-operation settings in
  `browser.reaper`; runtime configuration adapters must carry that section.

## Current state / implementation record (source review only)

| Responsibility | Authored source witness | Verification status |
|---|---|---|
| Launch ownership and leakless hardening | `internal/browser/session_lifecycle.go:166` (`launchControlURL`); `internal/browser/session_manager.go:55` (`browserRecord`) | Formatted; not compiled or exercised. |
| Temporary-path guard and cleanup retry | `internal/browser/reaper.go:120` (`temporaryProfilePath`), `internal/browser/reaper.go:181` (`removeUserDataDir`) | Unit regressions authored at `internal/browser/reaper_test.go:98` (`TestRemoveUserDataDir_SafetyRefusal`); unrun. |
| Startup orphan scan and count | `internal/browser/reaper.go:237` (`ReapOrphans`), `internal/browser/reaper.go:221` (`ReapedOrphans`); `internal/browser/session_lifecycle.go:35` (`startDefault`) | Boot log contains `reaped_orphans`; `internal/browser/reaper_test.go:145` (`TestReapOrphans_ReapsOnlyDeadParentTrees`) is unrun. |
| Native process tree termination | `internal/browser/reaper_windows.go:105` (`KillProcessTree`); `internal/browser/reaper_unix.go:131` (`KillProcessTree`) | Windows and Unix implementations and interface checks authored; platform builds unrun. |
| Close/shutdown cleanup | `internal/browser/session_lifecycle.go:558` (`CloseBrowser`), `internal/browser/session_lifecycle.go:623` (`shutdown`) | `internal/browser/reaper_live_test.go:83` (`TestBrowserProcessLifecycle_LiveShutdown`) and `internal/browser/reaper_live_test.go:268` (`TestBrowserProcessLifecycle_LiveStartupReapsPlantedOrphan`) are opt-in and unrun. |
| Initial stream ordering and parsed-document wait | `internal/browser/session_lifecycle.go:351` (`CreateTab`), `internal/browser/session_lifecycle.go:450` (`waitDocumentParsed`) | Call order authored; B2 subscription readiness remains required. `internal/browser/reaper_live_test.go:180` (`TestCreateTab_LiveReturnsBeforeBackgroundResourceLoads`) is unrun. |
| Reload and intentional-navigation sensor | `internal/browser/session_manager.go:881` (`Navigate`); `internal/browser/session_lifecycle.go:456` (`markAttendedNavigation`) | `internal/browser/reaper_live_test.go:127` (`TestNavigate_SameURL_Reloads`) is unrun. Mangle owns unattended classification. |
| Detached tab capacity | `internal/browser/session_lifecycle.go:322` (`liveTabCountLocked`); `internal/browser/session_lifecycle_test.go:105` (`TestSessionManagerTabLimitIgnoresDetachedMetadataAndReservesAtomically`) | Existing source/test preserved; unrun. |

## Wiring and remaining work

`startDefault` invokes the scan before launch and logs the count
for that scan (`internal/browser/session_lifecycle.go:35`). `CloseBrowser` and
`shutdown` call the tracked-process cleanup through `closeBrowserResources`
(`internal/browser/session_lifecycle.go:590`). Config fields are authored at
`internal/browser/session_manager.go:98` (`Config`); operator overrides still
need copying in `internal/system/factory.go:1756` (`initAutopoiesisAndBrowser`) and
`cmd/nerd/cmd_browser.go:153` (`getBrowserConfig`). The separate headless research
manager also needs that section in `internal/tools/research/browser_read.go:94`
(`researchHeadlessConfig`). These files are outside B3 ownership.

`startEventStream` still creates its full subscriptions in a goroutine
(`internal/browser/session_manager_dom.go:22`); B2 must make subscription/hook
installation synchronous. B3 deliberately keeps that method signature intact.

## Gap rows and acceptance

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-BROWSER-05 | Launcher hardening | OPEN: authored in `internal/browser/session_lifecycle.go:166` (`launchControlURL`) | Required behavior above | High | B3 verification | Chrome host | All launches disable leakless; `TestBrowserProcessLifecycle_LiveShutdown` passes. |
| GAP-BROWSER-06 | Orphan reaper and cleanup | OPEN: authored in `internal/browser/reaper.go:237` (`ReapOrphans`) | Required behavior above | Critical | B3 verification | GAP-BROWSER-05; config adapters | `go test -count=1 ./internal/browser/...` passes; `CODENERD_BROWSER_LIVE_CHROME` names Chrome and live shutdown/orphan tests pass; Windows/Linux build checks pass. |
| GAP-BROWSER-07 | Reload and unattended navigation | OPEN: reload/sensor authored in `internal/browser/session_manager.go:881` (`Navigate`) | Required behavior above | Medium | B3/B2/B1 verification | GAP-BROWSER-02; live kernel derivation and output | `TestNavigate_SameURL_Reloads` passes; B2/B1 live proof separately observes derived unattended navigation. |

No build, test, or live result has been obtained by this author-only lane.

## Decisions

- Native Windows handles replace upstream `taskkill`: inspected process handles
  survive PID reuse, termination errors remain visible, and no shell kill path is
  introduced (`internal/browser/reaper_windows.go:105`, `KillProcessTree`).
- Profile allocation uses a resolved codeNERD container and writes a launcher
  ownership marker. Unmarked native profiles fail closed; legacy Rod/BrowserNERD
  temporary containers retain the dead-parent check
  (`internal/browser/reaper.go:167`, `profileLauncherAlive`).
- Configured launch flags are never silently dropped on a fallback attempt
  (`internal/browser/session_lifecycle.go:166`, `launchControlURL`).
