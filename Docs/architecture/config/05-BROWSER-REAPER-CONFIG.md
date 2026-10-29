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

As built 2026-10-28: all three adapters now carry the section. `initAutopoiesisAndBrowser`
copies `configured.Reaper` into the interactive manager and passes it to
`research.SetBrowserRuntime` (`BrowserRuntimeOptions.Reaper`), which `researchHeadlessConfig`
applies to the headless research manager; `getBrowserConfig` copies it for the CLI
(`TestGetBrowserConfigReadsWorkspaceNativeSettings`, written but not run: `cmd/nerd` did not
build while other lanes were mid-edit).

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CONFIG-BROWSER-REAPER-01 | Cleanup request settings | OPEN: section and checks authored at `internal/config/browser_reaper.go:6` | All native entry paths consume the section | High | B3 integration | CLI/factory/research adapter owners | `go test -count=1 ./internal/config/...` passes; adapter tests prove explicit reaper settings reach both operator and headless research managers. |

## Constraints

Absent values use the config defaults. Invalid explicit values are errors,
not fallback candidates. Do not add a model process-killing tool or an agent-run
timeout while wiring these settings.

## Accepted browser policy parameters contract

`BrowserAutomationConfig` must also own a typed, JSON-configurable browser
policy section. It supplies the four millisecond windows consumed by policy
(`browser_slow_api_ms`, `browser_causal_bucket_ms`,
`browser_unhydrated_age_ms`, `browser_attended_navigation_window_ms`) and all
thirteen explicit action priorities (`browser_priority_primary` plus the twelve
`browser_action_rank` keys). Defaults preserve the values already pinned by
`internal/core/defaults/policy/browser_port_test.go:19` and `:231`; positive
thresholds and priorities are validated as config input. The attended
navigation window preserves its pinned zero default; zero is allowed and
negative values are rejected. Existing policy-required keys must stay
complete: all seventeen
`config_param_required(/browser, Key)` keys in
`internal/core/defaults/policy/browser.mg:58-62` and `:275` receive a value.

Before browser policy is queried or `SessionManager` is constructed, the normal
factory path must turn the typed config into `config_param` rows and call the
real parameter assertion boundary (`config.EnsureParams` / `ParamFacts`) on the
live kernel. Missing/invalid values or assertion failures are surfaced and
stop browser manager initialization. `slow_api` thresholds and
`action_candidate` ranks are derived from those configured rows, so operator
values affect normal boot rather than only a policy test fixture.

### Verified current implementation

`BrowserAutomationConfig.Policy` now carries the typed policy settings and its
defaults (`internal/config/user_config.go:1475-1520`). The config conversion and
validation are in `internal/config/browser_policy.go`; the browser-specific
factory seam renders/asserts parameters before manager construction
(`internal/system/factory.go:1885-1889`, `:1950-1964`), with the manager created
afterward at `:1933`. `TestBootPublishesConfiguredBrowserPolicyBeforeManagerUse`
asserts that all seventeen values reach a real kernel before manager use and
that configured priorities and thresholds change `action_candidate` and
`slow_api` (`internal/system/factory_browser_policy_test.go:16`). Invalid
values are rejected before browser binding by
`TestInitAutopoiesisAndBrowserRejectsInvalidPolicyBeforeBinding`
(`factory_browser_policy_test.go:99`), and a rejected `config_param` assertion
is surfaced before either runtime binds by
`TestInitAutopoiesisAndBrowserPropagatesPolicyPublicationFailure`
(`factory_browser_policy_test.go:116`). Config-level tests cover defaults,
custom values, invalid thresholds/priorities, the zero-window case, and JSON
conversion (`internal/config/browser_policy_test.go:13-169`). Focused factory
and config tests passed; aggregate verification remains pending.

| Gap ID | Capability | Current state / deficiency | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-BROWSER-PARAM-BRIDGE | Browser policy configuration reaches Mangle before manager use | Typed `BrowserAutomationConfig.Policy` and the factory parameter assertion seam are implemented (`internal/config/user_config.go:1475-1520`; `internal/system/factory.go:1885-1889`, `:1950-1964`). The seventeen required policy keys remain declared in `internal/core/defaults/policy/browser.mg:58-62`, `:275`. | Required contract above. | P1 | Implemented; focused tests passed; aggregate verification pending | Aggregate verification | `TestBootPublishesConfiguredBrowserPolicyBeforeManagerUse` proves all 17 keys reach a real kernel before manager use and configured thresholds/priorities affect derivations; invalid-policy boot refusal and config default/custom/invalid-value tests pass. |
