---
doc-class: shipped-with-future
subsystem: config
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472
supersedes: []
---

# Native browser cleanup request settings

This is a standard-labelled supplement beside the legacy config corpus. The
target is one operator-authorized `browser.reaper` section shared by every native
browser entry path. Defaults and checks live in config; these values bound one
OS operation and never decide when an agent run ends.

## Current state / implementation record (source review only)

- `BrowserReaperConfig` declares `timeout_ms` and `poll_interval_ms`
  (`internal/config/browser_reaper.go:6`). `DefaultBrowserReaperConfig` supplies
  2000/50 milliseconds (`internal/config/browser_reaper.go:11`).
- `Check` defaults omitted zero fields, rejects negative or overflowing
  durations, and rejects a poll interval above the request timeout
  (`internal/config/browser_reaper.go:26`). `UserConfig.Check` calls it under the
  `browser.reaper` prefix (`internal/config/check.go:112`, call at line 341).
- `BrowserAutomationConfig` owns the JSON section and `GetBrowserConfig`
  defaults omitted fields without replacing explicit values
  (`internal/config/user_config.go:1474`, `internal/config/user_config.go:1521`).
- `TestBrowserReaperConfigDefaultsAndCheck` covers the real defaults, getter,
  section check, and root check (`internal/config/browser_reaper_test.go:5`). It
  has not been run; no shipped/runtime claim is made.

## Wiring and gap

The browser package can consume this typed section through `Config.Reaper`
(`internal/browser/session_manager.go:98`). The existing adapters copy fields
individually: `initAutopoiesisAndBrowser` (`internal/system/factory.go:1756`),
`getBrowserConfig` (`cmd/nerd/cmd_browser.go:153`), and `researchHeadlessConfig`
(`internal/tools/research/browser_read.go:94`). Copying the new section is
outside B3 ownership and remains required for operator overrides.

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CONFIG-BROWSER-REAPER-01 | Cleanup request settings | OPEN: section and checks authored at `internal/config/browser_reaper.go:6` | All native entry paths consume the section | High | B3 integration | CLI/factory/research adapter owners | `go test -count=1 ./internal/config/...` passes; adapter tests prove explicit reaper settings reach both operator and headless research managers. |

## Constraints

Absent values use the config defaults. Invalid explicit values are errors,
not fallback candidates. Do not add a model process-killing tool or an agent-run
timeout while wiring these settings.
