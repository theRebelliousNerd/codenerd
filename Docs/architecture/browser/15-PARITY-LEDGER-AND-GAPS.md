---
doc-class: shipped-with-future
subsystem: browser
implementation-status: target-state
last-verified: 2026-09-29
verified-against: 6597099c
supersedes: []
---

# BrowserNERD Parity Ledger and Gap Analysis

> Shipped vs. future capability audit grounded in commit 6597099c.
> Evaluates baseline implementation against BrowserNERD 1.2.0 (CrossThread commit 0806144c6, commit 1396e5309, and uncommitted process reaper).

## 1. Parity Audit Ledger (BP-01 through BP-35)

In accordance with the Parity Contract (`13-BROWSERNERD-PARITY.md`), a capability is scored as `done` only after passing unit coverage and live Chrome execution on codeNERD's production routes. Because this audit phase is specification-first and does not execute live browser binaries, rows with working code and existing test suites are classified as `Implemented (unverified against live Chrome)`. Rows with partial functionality are marked `Partial`, and unimplemented capabilities are marked `Missing`.

| ID | Capability | Status | Current Code Evidence | Target State Reference |
|---|---|---|---|---|
| BP-01 | Browser launch, attach, reconnect, and clean shutdown | Partial | Launch and manager lifecycle in `internal/browser/session_lifecycle.go:71`, `149-180`, `387-425`, `534-587`. Stream starts after navigation (`339-374`). No process-tree termination. | `14-BROWSERNERD-1.2-PORT-SPEC.md#3-capability-group-2-process-lifecycle-rod-hardening-and-process-tree-reaper` |
| BP-02 | Multiple browser instances and instance switching | Implemented (unverified against live Chrome) | `LaunchAdditional` at `internal/browser/session_lifecycle.go:184`, `ListBrowsers` at line 227, `CloseBrowser` at line 472. Test suite in `internal/browser/session_lifecycle_test.go:59`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#3-capability-group-2-process-lifecycle-rod-hardening-and-process-tree-reaper` |
| BP-03 | Shared tabs by default with explicit incognito isolation | Partial | Default shared tabs and isolated mode in `internal/browser/session_lifecycle.go:292-324`. Storage snapshot/restore in `internal/browser/session_manager.go:801-868` and `internal/browser/session_manager_dom.go:731-770`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#3-capability-group-2-process-lifecycle-rod-hardening-and-process-tree-reaper` |
| BP-04 | Session lifecycle management (list, create, attach, focus, fork, close) | Partial | Session operations in `internal/browser/progressive_action.go:195-262`. CLI commands in `cmd/nerd/cmd_browser.go:44-150` and TUI slash command `/browser` in `cmd/nerd/chat/commands_handlers_misc.go:374-395`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#5-capability-group-4-structured-action-execution-locators-and-verification` |
| BP-05 | Tab and browser allocation limits with idle tab reaping | Implemented (unverified against live Chrome) | Live tab counting in `internal/browser/session_lifecycle.go:264-271`, tab reservation at lines 274-281, reaper cancellation at lines 539-542, `reapIdleTabs` at lines 615-630. Test in `internal/browser/session_lifecycle_test.go:105`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#3-capability-group-2-process-lifecycle-rod-hardening-and-process-tree-reaper` |
| BP-06 | Per-session cancelable background event stream | Implemented (unverified against live Chrome) | Stream context lifecycle in `internal/browser/session_lifecycle.go:369-374`, cancellation at lines 450-451. Test in `internal/browser/session_lifecycle_test.go:33`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#2-capability-group-1-first-byte-capture-stream-and-page-hooks` |
| BP-07 | Persistent session metadata with owner-only privacy protection | Implemented (unverified against live Chrome) | URL redaction in `internal/browser/session_manager_dom.go:39`, `442`. Private file permissions enforced in `session_manager_dom.go:795-798` via `internal/browser/security/path_policy.go:139-165`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#3-capability-group-2-process-lifecycle-rod-hardening-and-process-tree-reaper` |
| BP-08 | Compact page state and navigation discovery | Partial | Summaries and handles in `internal/browser/progressive_observe.go:173-355`. Lacks raw text mode, storage mode, and control ranking. | `14-BROWSERNERD-1.2-PORT-SPEC.md#4-capability-group-3-progressive-observation-storage-and-control-ranking` |
| BP-09 | Stable interactive element references and fingerprinting | Implemented (unverified against live Chrome) | Element registry in `internal/browser/element_registry.go:52-125`. Placeholder bug remains in `internal/browser/progressive_observe.go:628`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#4-capability-group-3-progressive-observation-storage-and-control-ranking` |
| BP-10 | Interactive, grid, and hidden-content observation slices | Partial | Modes implemented in `internal/browser/progressive_observe.go:370-685`. Lacks React hydration classification (`dead_control`) and candidate ranking rules. | `14-BROWSERNERD-1.2-PORT-SPEC.md#4-capability-group-3-progressive-observation-storage-and-control-ranking` |
| BP-11 | Screenshot, DOM snapshot, and React inspection | Partial | Core observation modes in `internal/browser/progressive_observe.go:209-243`. Lacks visual element annotations and single-ref crop. | `14-BROWSERNERD-1.2-PORT-SPEC.md#4-capability-group-3-progressive-observation-storage-and-control-ranking` |
| BP-12 | Basic interactions: navigate, click, type, fill, key, and history | Partial | Action execution in `internal/browser/progressive_action.go:109-183`, `300-386`. Lacks text/label/role/point locators, hover, read-back verification, and file upload. | `14-BROWSERNERD-1.2-PORT-SPEC.md#5-capability-group-4-structured-action-execution-locators-and-verification` |
| BP-13 | Sequential action plans with stop-on-error behavior | Partial | Batch runner in `internal/browser/progressive_action.go:80-295` and tool in `internal/tools/research/browser_progressive.go:73-95`. Lacks pre-batch validation and effects digest. | `14-BROWSERNERD-1.2-PORT-SPEC.md#5-capability-group-4-structured-action-execution-locators-and-verification` |
| BP-14 | Condition and fact waits in the live kernel | Implemented (unverified against live Chrome) | Bounded waits in `internal/tools/research/browser_reasoning.go:277-318`. Waits are kept out of `browser_act` by design (`internal/browser/progressive_action.go:4-5`). | `14-BROWSERNERD-1.2-PORT-SPEC.md#6-capability-group-5-diagnostic-reasoning-bucketed-causality-and-backend-correlation` |
| BP-15 | Modular progressive tools: observe, act, reason, and audit | Implemented (unverified against live Chrome) | Tool implementations in `internal/tools/research/browser_progressive.go:14-95` and `internal/tools/research/browser_reasoning.go:99-178`. JIT atoms in `internal/prompt/atoms/capability/browser_progressive.yaml:1-48` omit 1.2.0 modes. | `14-BROWSERNERD-1.2-PORT-SPEC.md#8-capability-group-7-tool-surface-jit-prompt-atoms-and-token-budget-discipline` |
| BP-16 | Bounded live Mangle query and temporal inspection | Partial | Live kernel query in `internal/tools/research/browser_reasoning.go:97-103`, `480-496`. Restricts queries to single atoms. Deliberately refuses rule mutation. | `14-BROWSERNERD-1.2-PORT-SPEC.md#6-capability-group-5-diagnostic-reasoning-bucketed-causality-and-backend-correlation` |
| BP-17 | Centralized credential and sensitive input redaction | Partial | Redaction of URLs and inputs in `internal/browser/session_manager_dom.go:39`, `442` and `internal/browser/security/redactor.go`. Lacks failure-body, dialog, and download sinks. | `14-BROWSERNERD-1.2-PORT-SPEC.md#2-capability-group-1-first-byte-capture-stream-and-page-hooks` |
| BP-18 | Writable path confinement for browser artifacts | Partial | Path policy in `internal/browser/security/path_policy.go:139-165`. Lacks workspace read-confinement for file input uploads. | `14-BROWSERNERD-1.2-PORT-SPEC.md#5-capability-group-4-structured-action-execution-locators-and-verification` |
| BP-19 | Arbitrary JavaScript execution gate | Deliberately Excluded | Rejected by design. Unconstrained JavaScript evaluation is intentionally absent from codeNERD tools (`internal/browser/progressive_action.go:4-5`). | `13-BROWSERNERD-PARITY.md#3-scope-boundaries-what-is-and-is-not-ported` |
| BP-20 | Flight recorder trace export and session event retention | Partial | Flight recorder in `internal/browser/flight_recorder.go:60-250`. Fact allowlist lacks dialog, download, WebSocket, and page-load-failed events. | `14-BROWSERNERD-1.2-PORT-SPEC.md#2-capability-group-1-first-byte-capture-stream-and-page-hooks` |
| BP-21 | Bounded specification delivery and conformance checking | Implemented (unverified against live Chrome) | Spec catalog in `internal/tools/research/browser_specs.go:22-132` and `internal/browser/specs/`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#7-capability-group-6-declarative-fixtures-and-test-execution` |
| BP-22 | Declarative browser test fixtures and execution | Partial | Spec parser in `internal/browser/testspec/types.go:26-32`, empty assertion check in `parser.go:109`, empty env check at lines 295-296. Lacks `operations` alias, upload steps, and polymorphic locators. | `14-BROWSERNERD-1.2-PORT-SPEC.md#7-capability-group-6-declarative-fixtures-and-test-execution` |
| BP-23 | Diagnostic reasoning across console, toast, and network | Partial | Diagnostic topics in `internal/tools/research/browser_reasoning.go:344-477`. Lacks `network` topic, JSON body shapes, resource-typed 4xx rules, and per-request backend lines. | `14-BROWSERNERD-1.2-PORT-SPEC.md#6-capability-group-5-diagnostic-reasoning-bucketed-causality-and-backend-correlation` |
| BP-24 | Contract audit discovery and repository tracing | Implemented (unverified against live Chrome) | Contract audit in `internal/browser/contract_audit.go` and `internal/core/defaults/policy/browser.mg:100-103`. Maintained under an independent lane. | `13-BROWSERNERD-PARITY.md#3-scope-boundaries-what-is-and-is-not-ported` |
| BP-25 | Docker container log correlation | Partial | Non-fatal log correlation in `internal/browser/docker_correlation.go:81-109`. Lacks per-request path matching; all-container failure does not return an error. | `14-BROWSERNERD-1.2-PORT-SPEC.md#6-capability-group-5-diagnostic-reasoning-bucketed-causality-and-backend-correlation` |
| BP-26 | Delivery evaluation and test harness gates | Partial | Integration tests exist (`browser_integration_test.go`, `browser_progressive_live_test.go`). Lacks probe harness suite and tool schema token budget gate. | `14-BROWSERNERD-1.2-PORT-SPEC.md#8-capability-group-7-tool-surface-jit-prompt-atoms-and-token-budget-discipline` |
| BP-27 | First-byte capture stream and pre-document page hooks | Missing | Need pre-navigation subscriptions, pre-document hooks, dialogs, downloads, WebSockets, exceptions, browser logs, same-document nav, and `page_load_failed`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#2-capability-group-1-first-byte-capture-stream-and-page-hooks` |
| BP-28 | Observation modes: text, storage, hydration ranking, and locators | Missing | Need `text` and `storage` modes, `action_candidate` rules, `dead_control` classification, route-announcer toast filtering, and visible-first locators. | `14-BROWSERNERD-1.2-PORT-SPEC.md#4-capability-group-3-progressive-observation-storage-and-control-ranking` |
| BP-29 | Action pre-validation, locators, read-back, upload, and effects | Missing | Need batch pre-validation, polymorphic locators, input read-back, upload support, document-parsed wait, and derived effects digest. | `14-BROWSERNERD-1.2-PORT-SPEC.md#5-capability-group-4-structured-action-execution-locators-and-verification` |
| BP-30 | Diagnostic network topic, body shapes, and bucketed causality | Missing | Need `network` topic, JSON schema shapes, asset 4xx discrimination in Mangle, bucketed `caused_by` rules, console grouping, and per-request Docker matching. | `14-BROWSERNERD-1.2-PORT-SPEC.md#6-capability-group-5-diagnostic-reasoning-bucketed-causality-and-backend-correlation` |
| BP-31 | Signal-aware epoch retention preserving failure facts | Missing | Current epoch logic (`internal/browser/fact_epoch.go:164-168`) drops all stream facts on saturation. Need signal-aware eviction prioritizing failures. | `14-BROWSERNERD-1.2-PORT-SPEC.md#8-capability-group-7-tool-surface-jit-prompt-atoms-and-token-budget-discipline` |
| BP-32 | Rod launcher hardening (`Leakless(false)`) and process reaper | Authored; unverified | `launchControlURL` at `internal/browser/session_lifecycle.go:166`, `ReapOrphans` at `internal/browser/reaper.go:237`, and `shutdown` at `internal/browser/session_lifecycle.go:623`. B3 tests are unrun; config adapters remain pending. | `16-PROCESS-LIFECYCLE.md` |
| BP-33 | Internal system health diagnostics and per-call deadlines | Missing | Need internal health inspection (Chrome, sessions, reaped orphans) and enforced per-call tool timeouts without an external MCP server. | `14-BROWSERNERD-1.2-PORT-SPEC.md#8-capability-group-7-tool-surface-jit-prompt-atoms-and-token-budget-discipline` |
| BP-34 | Unattended navigation reporting on subsequent calls | Missing | Need detection and reporting of navigation events occurring outside intentional model actions. | `14-BROWSERNERD-1.2-PORT-SPEC.md#3-capability-group-2-process-lifecycle-rod-hardening-and-process-tree-reaper` |
| BP-35 | Same-URL navigation forced reload | Authored; unverified | `Navigate` at `internal/browser/session_manager.go:881` explicitly reloads matching live URLs; `TestNavigate_SameURL_Reloads` at `internal/browser/reaper_live_test.go:127` is unrun. | `16-PROCESS-LIFECYCLE.md` |

---

## 2. Master Gap Matrix (GAP-BROWSER-01 through GAP-BROWSER-19)

| Gap ID | Capability | Current State | Target State | Severity | Phase | Blocking Dependencies | Exit Criteria |
|---|---|---|---|---|---|---|---|
| GAP-BROWSER-01 | Browser Mangle Schema and Diagnostic Policy | Generic status >= 400 failure rule at `internal/core/defaults/policy/browser.mg:56-64`. Missing asset types, bucketed causality, and candidate ranking. | `14-BROWSERNERD-1.2-PORT-SPEC.md#22-facts-and-predicate-schema` and `#62-facts-and-predicate-schema` | Critical | Lane 1 (Wave 1) | None | `go test ./internal/core/defaults/policy/...` passes with tests proving favicon 4xx does NOT derive `failed_request` while script 4xx does, and proving bucketed `caused_by` derives nearest failure. |
| GAP-BROWSER-02 | First-Byte Capture Stream Lifecycle | `CreateTab` navigates before starting stream at `internal/browser/session_lifecycle.go:339-374`. | `14-BROWSERNERD-1.2-PORT-SPEC.md#21-implementation-details` | High | Lane 2 (Wave 2) | GAP-BROWSER-01 | Live Chrome test loading a page with early script error confirms `console_event` and `page_load_failed` are asserted into kernel before navigation completes. |
| GAP-BROWSER-03 | Pre-Document Hooks and Toast Filtering | Toast detection in `session_manager_dom.go:114-129` treats all live regions as alerts, polluting logs with Next.js route announcements. | `14-BROWSERNERD-1.2-PORT-SPEC.md#21-implementation-details` | Medium | Lane 2 (Wave 2) | GAP-BROWSER-01 | Unit test verifies route announcer DOM mutations (`data-route-announcer` or matching `document.title`) do NOT emit `toast_notification`. |
| GAP-BROWSER-04 | Signal-Aware Epoch Fact Retention | `fact_epoch.go:164-168` drops all incoming facts indiscriminately when saturation limit is reached. | `14-BROWSERNERD-1.2-PORT-SPEC.md#82-the-gomangle-split-and-even-better-design` | High | Lane 2 (Wave 2) | GAP-BROWSER-01 | Unit test verifies that when event budget is saturated, polling facts (`dom_updated`) are evicted while `net_http_error` and `console_event` are retained. |
| GAP-BROWSER-05 | Rod Launcher Hardening (`Leakless(false)`) | OPEN: authored in `launchControlURL` (`internal/browser/session_lifecycle.go:166`); no live proof. | `16-PROCESS-LIFECYCLE.md` | High | Lane 3 verification | Chrome host | Source shows leakless disabled on every launch; opt-in `TestBrowserProcessLifecycle_LiveShutdown` passes. |
| GAP-BROWSER-06 | Process-Tree Reaper and Safe Cleanup | OPEN: `ReapOrphans` (`internal/browser/reaper.go:237`) and tracked cleanup authored; config adapter propagation and verification pending. | `16-PROCESS-LIFECYCLE.md` | Critical | Lane 3 verification | GAP-BROWSER-05; CLI/factory/research adapter owners | `go test -count=1 ./internal/browser/...` and `go test -count=1 ./internal/core/defaults/...` pass; live shutdown/startup-orphan tests pass with `CODENERD_BROWSER_LIVE_CHROME`; Windows/Linux build checks pass. |
| GAP-BROWSER-07 | Same-URL Reload and Unattended Navigation | OPEN: explicit reload in `Navigate` (`internal/browser/session_manager.go:881`), sensor in `markAttendedNavigation` (`internal/browser/session_lifecycle.go:456`); live derivation/output proof pending. | `16-PROCESS-LIFECYCLE.md` | Medium | Lane 3/B2/B1 verification | GAP-BROWSER-02 | `TestNavigate_SameURL_Reloads` passes, and a live kernel test asserts `unattended_navigation` for an autonomous redirect and reports it through normal observations. |
| GAP-BROWSER-08 | Observe Modes: Text and Storage with JWT Expiry | `progressive_observe.go:208-370` lacks `text` and `storage` modes. Storage values risk exfiltration. | `14-BROWSERNERD-1.2-PORT-SPEC.md#41-implementation-details` | High | Lane 4 (Wave 3) | GAP-BROWSER-01 | `TestObserveStorage_ExtractsJWTExpiryWithoutValues` passes, asserting that JWT `exp` is parsed and raw secret tokens are withheld. |
| GAP-BROWSER-09 | Element Label Correction and Hydration Ranking | `progressive_observe.go:628` falls back to placeholder for element labels; controls are not ranked or filtered for hydration. | `14-BROWSERNERD-1.2-PORT-SPEC.md#41-implementation-details` | High | Lane 4 (Wave 3) | GAP-BROWSER-01 | `TestElementLabel_NeverUsesPlaceholder` passes, and live React test verifies that unhydrated controls are classified as `dead_control` and omitted from top actions. |
| GAP-BROWSER-10 | Upload Workspace Path Confinement | `internal/browser/security/path_policy.go:139-165` lacks read-confinement validation for file inputs. | `14-BROWSERNERD-1.2-PORT-SPEC.md#51-implementation-details` | Critical | Lane 5 (Wave 3) | None | `path_policy_test.go` passes assertions refusing file paths outside workspace root, directory traversal (`../`), and symlink escapes for upload operations. |
| GAP-BROWSER-11 | Action Batch Pre-Validation and Locators | `progressive_action.go:108` validates steps sequentially during execution; lacks text/label/role/point locators. | `14-BROWSERNERD-1.2-PORT-SPEC.md#51-implementation-details` | High | Lane 6 (Wave 4) | GAP-BROWSER-09, GAP-BROWSER-10 | `TestExecuteActions_FailsEntireBatchBeforeStepZero` passes when step N is invalid, ensuring step 0 never executes. |
| GAP-BROWSER-12 | Form Field Read-Back and Effects Digest | `progressive_action.go:339-360` returns without verifying value change; no causal effects digest. | `14-BROWSERNERD-1.2-PORT-SPEC.md#51-implementation-details` | Medium | Lane 6 (Wave 4) | GAP-BROWSER-11 | Live Chrome test confirms `type` and `select` verify actual element value via read-back, and response contains `effects` summary with network/console deltas. |
| GAP-BROWSER-13 | File Upload and Hover Operations in Act | `progressive_action.go:108-266` lacks `upload` and `hover` operation types. | `14-BROWSERNERD-1.2-PORT-SPEC.md#51-implementation-details` | High | Lane 6 (Wave 4) | GAP-BROWSER-10, GAP-BROWSER-11 | Live test `TestAct_UploadFile_Confined` successfully attaches a workspace file to `<input type="file">`, and refuses non-workspace file. |
| GAP-BROWSER-14 | Reason Network Topic and Error Body Shapes | `browser_reasoning.go:344-477` lacks `network` topic, JSON body shape summaries, and console repeat aggregation. | `14-BROWSERNERD-1.2-PORT-SPEC.md#61-implementation-details` | High | Lane 7 (Wave 4) | GAP-BROWSER-01 | `TestReasonNetwork_GroupsAndIdentifiesStorms` passes, and `TestReasonWhyFailed_ReturnsJSONSchemaShape` confirms error bodies are rendered as structural schemas. |
| GAP-BROWSER-15 | Docker Per-Request Path Matching and Failure Refusal | `docker_correlation.go:81-109` returns empty success when all containers fail and lacks per-request path matching. | `14-BROWSERNERD-1.2-PORT-SPEC.md#61-implementation-details` | High | Lane 7 (Wave 4) | GAP-BROWSER-01 | `TestCorrelateContainerLogs_AllContainersFail_ReturnsError` passes, and path-matched container logs correlate directly to the failed request ID. |
| GAP-BROWSER-16 | Declarative Fixture Vocabulary and Environment Validation | `internal/browser/testspec/parser.go:105-115` requires `actions`, rejects `operations`, and does not validate `value_env` before execution. | `14-BROWSERNERD-1.2-PORT-SPEC.md#71-implementation-details` | Medium | Lane 8 (Wave 5) | GAP-BROWSER-11 | `TestParseSpec_AcceptsOperationsAlias` passes, and missing/empty `value_env` halts fixture before any action is executed. |
| GAP-BROWSER-17 | Constitutional Action Partitioning and Dialog Default-Deny | `constitution.mg:161-162` defines blanket `safe_action(/browser_act)`. Dialogs lack default-deny policy. | `14-BROWSERNERD-1.2-PORT-SPEC.md#53-the-gomangle-split-and-even-better-design` | Critical | Lane 9 (Wave 5) | GAP-BROWSER-13 | `go test ./internal/core/defaults/policy/...` verifies that `upload` and dialog accepts are denied without explicit permission, and unhandled dialogs auto-dismiss. |
| GAP-BROWSER-18 | JIT Prompt Atoms and Tool Definition Token Budget Gate | `browser_progressive.yaml:13-25` is missing 1.2.0 modes. No automated test limits tool definition schema length. | `14-BROWSERNERD-1.2-PORT-SPEC.md#81-implementation-details` | Medium | Lane 9 (Wave 5) | GAP-BROWSER-08, GAP-BROWSER-14 | `TestBrowserToolDefinitions_TokenBudget` passes, asserting that marshaled tool schemas do not exceed token limits (`len(json)/4`). |
| GAP-BROWSER-19 | Live Chrome End-to-End Proof and Acceptance Gate | Missing live Chrome integration test covering 1.2.0 probe suite on production routes. | `14-BROWSERNERD-1.2-PORT-SPEC.md#10-test-verification-strategy-and-upstream-traceability` | Critical | Lane 10 (Wave 5) | GAP-BROWSER-01 through GAP-BROWSER-18 | `internal/browser/browser_probe_live_test.go` passes against Chrome using upstream probe pages, verifying full end-to-end parity. |

## Accepted browser configuration-to-policy contract

The browser manager and its Mangle policy now share validated operator
configuration. `BrowserAutomationConfig.Policy` carries the typed settings
(`internal/config/user_config.go:1475-1520`), and the policy requires seventeen
browser parameters and derives `slow_api` and `action_candidate` from them
(`internal/core/defaults/policy/browser.mg:58-62`, `:122`, `:263-291`). The
factory asserts the configured values on the live kernel before constructing
the manager (`internal/system/factory.go:1885-1889`, `:1950-1964`, then
`:1933`). `TestBootPublishesConfiguredBrowserPolicyBeforeManagerUse` verifies
all parameters and configured derivations through normal boot
(`internal/system/factory_browser_policy_test.go:16`); invalid configuration
is rejected before browser binding at `:99`, and rejected kernel publication
is surfaced before either runtime binds at `:116`. The factory and config
focused tests passed; aggregate verification remains pending.
[`../config/05-BROWSER-REAPER-CONFIG.md`](../config/05-BROWSER-REAPER-CONFIG.md)
owns the accepted contract and `GAP-BROWSER-PARAM-BRIDGE` exit criteria. This
bridge is implemented and focused-tested. The separate candidate-observation
gap below remains open: this configuration test does not prove that live
browser snapshots publish the policy's control facts.

## Required contract: Fresh policy-ranked interactive observations

`GAP-BROWSER-09` remains open for hydration detection and broad candidate
ranking. The narrower `GAP-BROWSER-CANDIDATE-OBSERVATION` contract makes each
current interactive observation usable by policy without inventing readiness
signals. The live JS snapshot now reads native/ARIA disabled state, computed
style and bounding-box floats (`internal/browser/progressive_observe.go:831-865`).
`materializeInteractive` builds the policy facts, exact decimal geometry and
integer projection (`:567-640`), while `Observe` replaces the bounded control
snapshot and queries/filters `action_candidate` rows by current session and refs
(`internal/browser/progressive_observe.go:394-440`). The headless real-kernel
acceptance test `TestProgressiveObservePolicyRoundTrip`
(`internal/browser/progressive_observe_policy_live_test.go:30`) passes.

For one successful bounded interactive/composite observation, publish a
measured `browser_observed_at(SessionID, UnixMillis)` and, for each current
registry ref, `interactive(SessionID, Ref, Type, SanitizedLabel, Action)` plus
`element_enabled(SessionID, Ref, "true"|"false")` based on the actual native
disabled property or `aria-disabled`. Publish actual computed display,
visibility and opacity plus exact x/y/width/height geometry under the
session-qualified opaque ref (`SessionID:Ref`), the same key used by the
policy's honeypot guards (`internal/core/defaults/policy/browser.mg:24-42`).
Preserve the browser's fractional CSS-pixel measurement in
`browser_geometry_css_px(SessionID, Ref, XCss, YCss, WidthCss, HeightCss)`;
all four coordinates are canonical decimal strings measured from
`getBoundingClientRect()`, so decoding them recovers the measured float64
values. The existing `geometry/5` remains an integer CSS-pixel projection for
policy guards and ordering: round x/y to the nearest integer, preserve an exact
zero width or height as zero, and ensure every positive measured dimension
projects to at least one. The raw decimal predicate is ground truth; do not
change `geometry/5` to 1/64-pixel units or mix integer and fixed-point
producers. Tests must cover fractional positive dimensions versus exact zero
and exact decimal query round trips. Emit `browser_control_attribute` only for a finite safe
allowlist of explicitly observed control metadata (including `type` for the
submit rule); do not copy arbitrary attributes, input values, or secret
content. Respect normalized `max_items` (`progressive_observe.go:358`,
`normalizeObserveOptions`). Query the live kernel's `action_candidate` result
and expose `data.action_candidates` only for the current session and current
registry generation. Preserve Mangle's `Priority` and `Reason`; Go does not
rank or authorize actions.

The production `FactRetractor` must replace/retract the previous bounded
control-fact snapshot before publishing the next one so removed or newly
disabled elements cannot stay eligible. If no retractor is available, result
filtering must still exclude refs absent from the current snapshot and report
that the kernel may retain stale control facts; it must not return those old
candidates as current. Existing retractor support is optional today
(`internal/browser/fact_epoch.go:34-53`, `FactRetractor` and
`SetFactRetractor`), so this limitation must remain explicit. Do not derive
hydration or JavaScript readiness from the snapshot: those remain separate
unfinished signals and `GAP-BROWSER-09` stays open.

| Gap ID | Capability | Current state / deficiency | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-BROWSER-CANDIDATE-OBSERVATION | Fresh policy-ranked interactive observations | `materializeInteractive` publishes the measured policy facts, canonical `browser_geometry_css_px/6` decimals and integer projection (`internal/browser/progressive_observe.go:567-640`); `Observe` replaces the snapshot and returns only live-kernel candidates for current refs (`:394-440`). | Required contract above; `GAP-BROWSER-09` hydration and broader ranking remain separate. | P1 | Implemented; focused tests passed; aggregate verification pending | Configured browser params; live policy query and snapshot retraction boundary | The local HTTP + headless-browser + real-kernel `Observe` round trip derives the enabled submit candidate with configured Mangle rank, excludes disabled and measured honeypot controls, preserves exact fractional CSS coordinates, keeps exact-zero geometry at zero and positive subpixel dimensions nonzero, and retracts the first snapshot after a second observation disables/removes controls. Timestamp and bounded-output behavior remain under the normalized `max_items` contract; no hydration/readiness is fabricated. |

**Verified current:** `TestProgressiveObservePolicyRoundTrip`
(`internal/browser/progressive_observe_policy_live_test.go:30`) passes with the
headless HTTP fixture and real kernel. It checks the configured rank and Mangle
reason, explicit measured style/disabled facts, exact `0.25` CSS-pixel geometry
alongside its positive integer projection, the exact-zero control, disabled and
CSS trap exclusion, and old-fact/candidate removal after the second observation
(`:34-40`, `:85-107`, `:113-136`, `:220-239`). The full feature remains partial:
hydration evidence is not emitted, and aggregate verification remains pending.

## Required contract: Bounded CDP failure and interaction telemetry

The schemas and policy declare `net_http_error/6`, `net_loading_failed/5`, `page_load_failed/4`, `ws_event/6`, `browser_log/6`, `js_dialog/6`, and `dialog_answer/2` (`internal/core/defaults/schemas_browser.mg:55-62`, `:129`; rules in `internal/core/defaults/policy/browser.mg:76-119`, `:188-193`, `:330-345`). The live event callbacks now publish typed HTTP/loading/page failures and WebSocket, log, and dialog facts while retaining the legacy network facts (`internal/browser/session_manager_dom.go:383-398`, `:414-455`, `:671-715`). `cdpEventClock` converts network monotonic timestamps only after pairing them with request wall time and uses documented runtime epoch milliseconds or callback time as appropriate (`internal/browser/event_telemetry.go:14-73`). Event listener readiness is probed before first navigation (`:102-142`, called from `session_manager_dom.go:660-666`). `browser_reason` reads the typed facts through its finite predicate map (`internal/tools/research/browser_reasoning.go:60-90`, `:391-407`, `:438-447`). The real-CDP provenance, clock-unit, privacy, cleanup, and consumer gates are authored, but focused pass evidence is still pending.

Publish `net_http_error(Session, Request, URL, Status, ResourceType, Timestamp)` only for a real response with status at least 400. Publish `net_loading_failed(Session, Request, Error, Canceled, Timestamp)` from actual CDP loading-failure fields, preserving canceled as an explicit string. Publish `page_load_failed` only for a tracked main-frame `Document` request with a real non-canceled loading error; do not infer page failure from an asset or canceled request. Record WebSocket lifecycle/error metadata in `ws_event` without frame payloads. Record actual sanitized `Log.EntryAdded` data in `browser_log`. Record only a genuine JavaScript dialog event in `js_dialog`; use the existing kernel-derived `dialog_answer` and default-deny policy, and never accept without separate exact typed authority.

Subscribe and finish event-listener setup before first navigation. Retire the exact stream-fact set on navigation and session close through the epoch `FactRetractor` where available, and keep the existing per-epoch fact bound (`internal/browser/fact_epoch.go:34-53`, `:95-145`; `internal/browser/session_lifecycle.go:397-430`, `:521-538`). Preserve failure evidence needed for policy within that bound. For event time, use console/Log epoch milliseconds directly. For network events, pair the request's epoch-seconds `WallTime` with its monotonic `Timestamp` to establish the measured offset, then convert later response, loading-failure, and WebSocket monotonic timestamps through that offset. A dialog or WebSocket-created event without a CDP timestamp, or an event without a valid offset, uses host callback observation time; never reinterpret monotonic seconds as Unix milliseconds. Tests must assert units and provenance.

Extend the real `browser_reason`/`browser_mangle` finite surface with the typed timestamp slots and compact batch, error, correlation, and unattended diagnostics needed to inspect these facts. Keep summaries useful for unattended reasoning without proof-clutter or invented/fake query roots. Storage changes and `act_batch` temporal attribution remain separate accepted work; hydration remains explicitly unimplemented. A real local HTTP/CDP fixture must cover first response, HTTP error, canceled request, asset failure exclusion from page failure, WebSocket event/error without payload, actual dialog and default dismissal, and close/navigation cleanup, including audit privacy.

| Gap ID | Capability | Current state / deficiency | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-BROWSER-TELEMETRY-OBSERVATIONS | Bounded CDP failure and interaction telemetry | Typed producer, clock, listener-readiness, and finite `browser_reason` read paths are present (`internal/browser/session_manager_dom.go:383-455`, `:660-715`; `internal/browser/event_telemetry.go:14-142`; `internal/tools/research/browser_reasoning.go:391-447`). The focused live CDP verification has not yet passed. | Required contract above; storage, `act_batch` attribution, and hydration remain separate. | P1 | Accepted; implementation pending | CDP stream lifecycle and epoch retraction; finite browser reasoning read surface | Local HTTP/CDP tests prove real event provenance, typed timestamps, main-frame/document-only page failures, no WebSocket frames, default-denied dialogs, bounded fact retention, and exact cleanup on navigation/session close. Audit output excludes payload secrets. |

## Accepted implementation refinement for GAP-BROWSER-08: Bounded storage metadata

The broad storage/JWT capability remains part of `GAP-BROWSER-08`; this
refinement pins a safe, bounded metadata-only slice for the existing `Observe`
route. `Observe` currently handles screenshot, React, DOM snapshot, and the
bounded DOM modes but has no storage mode (`internal/browser/progressive_observe.go:245`,
`Observe`; `:278-314`). The kernel already declares `storage_entry/5` and
`auth_expired/3` (`internal/core/defaults/schemas_browser.mg:101-105`), and the
policy derives expiry from `storage_entry` and the observed session clock
(`internal/core/defaults/policy/browser.mg:216-237`). The public typed tool
schema and JIT capability atom are the normal model-facing route
(`internal/tools/research/browser_progressive.go:13-36`, `BrowserObserveTool`;
`internal/prompt/atoms/capability/browser_progressive.yaml:13-22`).

Add `mode: "storage"` to the existing `browser_observe` tool schema,
registration, mode validator, and existing JIT capability atom; do not add an
ad-hoc Go prompt. The mode inspects only page-visible `localStorage`,
`sessionStorage`, and the non-HttpOnly `document.cookie` view. Never use CDP to
fetch cookie values, never return raw storage/cookie values to Go, Mangle,
tool output, flight-recorder evidence, or logs, and never infer native cookie
expiry. The page-side probe returns only projected metadata.

Normalize `max_items` to the existing 1..100 range and enforce all three
per-call bounds: at most `max_items` rows, at most 64 KiB inspected per value,
and at most 256 KiB total inspected UTF-8 bytes. Bound JSON parsing and JWT
claim decoding by those byte limits. Return each key, store, kind, measured
UTF-8 byte count, and only the unverified JWT `iat`, `exp`, and `iss` claims in
a clearly unverified metadata object. Convert `exp` to milliseconds only when
the claim is actually decoded as a valid numeric epoch-seconds value; do not
infer it from key names, token age, or host time. Use `ExpMs = 0` in
`storage_entry(SessionID, Store, Key, Kind, ExpMs)` when no expiry was
measured, including opaque values and native cookies. When a value exceeds its
limit or the total scan reaches its byte/item cap, return explicit per-row or
aggregate skipped/truncated metadata without returning the value.

Treat storage facts as a replaceable session snapshot: retract the prior exact
`storage_entry` set before asserting the next snapshot, and retire it on
navigation and session close. Use the production `FactRetractor`. If exact
retraction is unavailable or fails, surface the error/limitation and do not
claim that `auth_expired` reflects only the current snapshot. Mangle remains
the sole owner of expiry/authentication derivation from measured session time.

**Exit evidence required:** a headless local page and real kernel exercise
`Registry.Execute` for `browser_observe` with `mode: "storage"`, proving the
public schema, JIT atom, real Observe route, and facts are connected. Cover
decoded JWT metadata, opaque values, per-value and total byte limits, `max_items`,
changed and removed keys, HttpOnly absence from the page view, explicit
skipped/truncated results, exact replacement/retraction, and absence of
canary secrets from facts, returned JSON, evidence, and logs.

## Accepted implementation refinement for GAP-BROWSER-12: Closed action windows

The current batch policy attributes every later event to every historical
`act_batch` start because it has no end fact (`internal/core/defaults/policy/browser.mg:298-328`).
`ExecuteActions` records one wall-clock start/finish around the request
(`internal/browser/progressive_action.go:69-75`, `ActionExecution`; `:80-100`,
`ExecuteActions`), while the declared kernel surface contains only
`act_batch/3` and `act_effect/6` (`internal/core/defaults/schemas_browser.mg:127-128`).
This refinement supplies bounded temporal association; it does not claim
causality and does not close the rest of `GAP-BROWSER-12`'s read-back/effects
digest contract.

For every session touched by an action call, assert a unique
`act_batch(SessionID, BatchID, StartedMs)` at the measured start and add
`act_batch_end(SessionID, BatchID, FinishedMs, Outcome)`, where `Outcome` is
`/settled` only after a bounded network-settle phase observes its quiet
condition, or `/interrupted` after cancellation, operation failure, session
closure, or the finite settle deadline. The settle phase must have a finite
quiet interval and hard maximum; tests must exercise both and the source must
name the bounds. Add `act_batch_window(SessionID, BatchID, StartedMs, FinishedMs,
Outcome)` as a Mangle-derived closed interval only when both timestamps are
measured and `FinishedMs >= StartedMs`.

Mangle may derive the existing `act_effect/6` only for an event timestamp in
exactly one closed `[StartedMs, FinishedMs]` window and only when no other
same-session batch started by that event is still open. If multiple closed
windows overlap an event, or a started batch remains open and could overlap it,
derive an explicit `act_effect_ambiguous(SessionID, Kind, Subject, Timestamp)`
and emit no `act_effect` for that event. Events outside a closed window have no
batch attribution. A failed end-fact assertion, missing timestamp, or missing
end measurement must produce a visible unverified-effects result; it must not
invent `/settled` or a window. `/interrupted` is truthful partial execution
metadata and is not a completion or causal verdict.

The host emits measured starts, ends, and status only. Mangle selects windows,
marks overlap ambiguity, and derives effects; Go does not pick an event's
winning batch or emit a causal claim. Do not impose global or cross-session
serialization in Go to hide ambiguity. Expose bounded compact effects and
ambiguity through the action result and the existing finite `browser_reason`
reader only after the window closes; `browser_mangle` adds the exact
timestamped `act_effect` consumer.

**Exit evidence required:** deterministic tests cover one settled batch,
interrupted and missing-end cases, event-before/start/inside/after boundaries,
same-session overlapping closed windows, a second still-open batch, concurrent
different sessions, session switches, failed measurement/assertion, and no
cross-session attribution. The production action result reports unverified
effects when it cannot publish a valid end; the real kernel returns no effect
for ambiguous events and preserves ordinary action completion policy.
