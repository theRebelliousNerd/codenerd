---
doc-class: north-star
subsystem: browser
implementation-status: target-state
last-verified: 2026-09-29
verified-against: 6597099c
supersedes: []
---

# BrowserNERD 1.2.0 Port Specification

> Target-state engineering specification for porting BrowserNERD 1.2.0 capabilities into codeNERD.
> Grounded in the upstream CrossThread BrowserNERD codebase at commit 0806144c6, commit 1396e5309, and the uncommitted process-reaper subsystem.

## 1. Architectural Overview and North Star Alignment

codeNERD's core architecture requires a strict separation of roles: the large language model serves as the creative center, while the deterministic Mangle kernel acts as the executive engine. In browser automation, this means that visual, network, and DOM perceptions must be transduced into formal logic facts before any executive decisions or actions occur.

The BrowserNERD 1.2.0 port deepens this architecture. Rather than relying on fragile heuristics in imperative Go code or delegating control to a standalone MCP server, browser events are captured at the lowest protocol layer, filtered, and asserted as immutable facts into codeNERD's live kernel (`internal/core/defaults/schemas_browser.mg:40-110`). The kernel evaluates stratified rules over these facts to derive diagnostic causes, filter noise, rank candidate interactions, and enforce safety boundaries.

This specification details seven discrete capability groups, defining for each:
- The technical mechanics and implementable details.
- The exact Mangle predicate signatures and types.
- The precise division of responsibilities between Go and Mangle.
- The incorporation of the "even better" design decisions.
- Edge cases and failure modes.
- The automated unit and live Chrome tests required for verification.

---

## 2. Capability Group 1: First-Byte Capture Stream and Page Hooks

### 2.1 Implementation Details

The browser capture pipeline must capture all relevant activity from the very first byte of a page load, eliminating race conditions where initial script errors, rapid redirects, or early network failures are missed.

In the current codeNERD baseline (`internal/browser/session_lifecycle.go:339-374`), a new tab navigates and waits for page load before initializing the event stream. This creates an unmonitored window during document parsing. The target architecture realigns `CreateTab` with upstream `browser_instances.go:104-181` and `capture_page.go:137-154`:
1. The Chrome tab is instantiated at the clean URL `about:blank`.
2. All Chrome DevTools Protocol (CDP) event listeners are registered via `startEventStream` before any navigation occurs.
3. Pre-document page hooks are injected via CDP's `Page.addScriptToEvaluateOnNewDocument`.
4. Navigation to the target URL is initiated.
5. The runtime waits for `waitDocumentParsed` (the DOMContentLoaded state) rather than full window load. Waiting for window load blocks prematurely on slow background subresources, whereas waiting for parsed DOM ensures that interactive elements and scripts are bound while keeping the driver responsive.

The event stream captures seven categories of runtime evidence:
- **Network Errors**: Responses with HTTP status codes of 400 or greater are captured as `net_http_error`, recording the CDP resource type (Document, XHR, Fetch, Script, Image, Font, Stylesheet, Media, Manifest, TextTrack, Other) and the precise timestamp.
- **Network Loading Failures**: Transport-level failures (connection reset, DNS failure, CORS block, TLS abort) are captured as `net_loading_failed`, explicitly distinguishing user/navigation cancellations from true server errors.
- **Redacted Failure Bodies**: For failed text or JSON responses (status 400 or greater), the first 1,024 bytes of the response body are captured, redacted through the centralized security redactor, and stored as `net_failure_body`. Binary responses are ignored.
- **WebSocket Lifecycle**: Connection creation, handshakes, opens, closures, and socket errors are recorded as `ws_event`. WebSocket frame payloads are never captured, preventing unbounded memory growth and credential leakage.
- **Console and Uncaught Exceptions**: Console logs are captured as `console_event`. Uncaught JavaScript runtime exceptions and unhandled promise rejections are normalized into console error events.
- **Browser Internal Logs**: Security violations, Content Security Policy (CSP) infractions, deprecation notices, and browser intervention warnings are captured as `browser_log`.
- **Pre-Document Hooks and Toasts**: Early page hooks monitor DOM mutations for user-visible notifications. Route announcers (elements matching Next.js/Gatsby routing announcements, elements with `data-route-announcer`, or elements whose text equals `document.title`) are explicitly excluded from toast detection. Reused live regions update cleanly without creating duplicate alerts.
- **JavaScript Dialogs and Downloads**: JavaScript alerts, confirms, prompts, and beforeunload modals are intercepted immediately and recorded as `js_dialog`. File downloads triggered by the browser are monitored via CDP's browser download events and recorded as `download`.
- **Navigation Failures**: When the top-level document fails to commit, encounters a network refusal, or displays Chrome's error interstitial, a `page_load_failed` fact is asserted.

### 2.2 Facts and Predicate Schema

The following predicate declarations are added to `internal/core/defaults/schemas_browser.mg`:

| Predicate | Arity and Bound Types | Description |
|---|---|---|
| `net_http_error` | `(SessionID, ReqID, URL, Status, ResourceType, Timestamp)` bound `[/string, /string, /string, /number, /string, /number]` | Emitted when an HTTP response has status >= 400. Includes CDP resource type. |
| `net_loading_failed` | `(SessionID, ReqID, ErrorText, Canceled, Timestamp)` bound `[/string, /string, /string, /string, /number]` | Emitted on network transport drop or abort. Canceled is "true" or "false". |
| `net_failure_body` | `(SessionID, ReqID, Snippet)` bound `[/string, /string, /string]` | First 1 KB of redacted text/JSON body from a failed request. |
| `ws_event` | `(SessionID, WsID, URL, Event, Detail, Timestamp)` bound `[/string, /string, /string, /string, /string, /number]` | WebSocket lifecycle state transitions: created, handshake, open, closed, error. |
| `browser_log` | `(SessionID, Source, Level, Text, URL, Timestamp)` bound `[/string, /string, /string, /string, /string, /number]` | Browser-internal events including CSP violations, security errors, and deprecations. |
| `js_dialog` | `(SessionID, Type, Message, Accepted, HandledBy, Timestamp)` bound `[/string, /string, /string, /string, /string, /number]` | JavaScript dialog occurrence and resolution (alert, confirm, prompt, beforeunload). |
| `download` | `(SessionID, Guid, URL, SuggestedName, State, Timestamp)` bound `[/string, /string, /string, /string, /string, /number]` | CDP download state: started, completed, or canceled. |
| `page_load_failed` | `(SessionID, URL, ErrorText, Timestamp)` bound `[/string, /string, /string, /number]` | Top-level frame navigation refusal or error page commit. |

### 2.3 The Go/Mangle Split and "Even Better" Design

In BrowserNERD 1.2.0, several aspects of event triage were implemented imperatively in Go. In codeNERD, the boundary is strictly maintained:
- **Go Responsibilities**: Go manages CDP event subscriptions, attaches pre-document scripts via Rod, applies input and credential redaction to URL parameters and response snippets, normalizes timestamps into epoch milliseconds, and writes typed `mangle.Fact` structs directly to the kernel sink.
- **Mangle Responsibilities**: The kernel derives whether an HTTP 4xx error constitutes an actionable application failure or benign static asset noise, correlates console errors with preceding request failures, and tracks user-visible alerts.
- **Even Better Rule 1 (Asset 4xx)**: Go asserts raw `net_http_error` facts with their CDP resource types. Mangle rules filter out assets (Image, Font, Stylesheet, Media, Manifest, TextTrack, Other), preventing benign missing favicons or optional stylesheets from polluting diagnosis.
- **Even Better Rule 6 (Dialog Handling)**: JavaScript dialogs block the renderer if unanswered. Go listens for dialog events. If an armed accept rule exists in Mangle, it accepts; otherwise, Go immediately dismisses the dialog with default-deny semantics and asserts `js_dialog` with `Accepted: "false"`.

### 2.4 Failure Modes

- **Renderer Freeze**: An unhandled `alert()` or `beforeunload` dialog halts JavaScript execution. Mitigated by installing the CDP dialog handler before the first navigation; dialogs are answered within milliseconds.
- **Event Flood on Polling**: Rapid polling loops or live streams can generate thousands of network events per minute. Mitigated by signal-aware epoch retention and stream throttling.
- **Flaking Navigation Assertions**: If Chrome emits multiple failure events for a single navigation refusal (as identified in upstream commit `1aa1c4e65`), duplicate `page_load_failed` facts could be asserted. Mitigated by deduplicating failed navigations per URL in Go before asserting into the kernel.

### 2.5 Verification and Tests

- **Unit Tests**: Table-driven tests in `internal/browser/session_manager_dom_test.go` verifying CDP event mapping to `mangle.Fact` structures, payload truncation at 1,024 bytes, and route announcer filtering.
- **Live Chrome Tests**: Integration tests in `internal/browser/browser_integration_test.go` loading a live test harness containing a 404 image, a 404 API route, an uncaught error, an immediate `alert()`, and a simulated download, asserting that the corresponding kernel facts are emitted with correct arity and attributes.

---

## 3. Capability Group 2: Process Lifecycle, Rod Hardening, and Process-Tree Reaper

### 3.1 Implementation Details

Chrome process lifecycle management must be robust against antivirus interference, sudden agent termination, and orphaned background processes. The port incorporates the uncommitted BrowserNERD process reaper (`reaper.go`, `reaper_windows.go`, `reaper_unix.go`, `reaper_test.go`):

1. **Rod Hardening via Leakless Disabled**: Antivirus engines (notably Windows Defender) frequently quarantine Rod's default `leakless` companion binary as a false positive. Chrome is configured with `launcher.New().Leakless(false)` across all platforms (`internal/browser/session_lifecycle.go:149-180`).
2. **Process and Directory Tracking**: When a browser launches, codeNERD captures the root process ID (PID), the parent launcher PID, and the exact `--user-data-dir` directory path.
3. **Dedicated Temporary Profiles**: Browsers launched by codeNERD use unique temporary profiles prefixed with `rod` or `browsernerd` located under the system temporary directory.
4. **Safety-Gated Directory Deletion**: Profile deletion includes strict path validation (`isBrowserNERDUserDataDir`). The path must contain designated temporary markers (e.g. `rod/user-data/` or `browsernerd/user-data/`) and must never match personal user profile paths (such as `Default` or `Users/name/AppData/Local/Google/Chrome/User Data`). If the path check fails, the deletion is refused with an explicit error.
5. **Process-Tree Termination**: On `SessionManager.Shutdown()` or individual browser closure, codeNERD executes `KillProcessTree(pid)`:
   - On Windows, descendants are enumerated using the Win32 Toolhelp snapshot API (`CreateToolhelp32Snapshot`), terminated via `taskkill /F /T /PID <pid>`, with a fallback to direct `OpenProcess` and `TerminateProcess` handles.
   - On Unix, descendants are gathered by scanning the process table or sending signals to the process group.
   - The reaper waits up to two seconds for processes to exit, then cleans up the temporary profile with retry backoff.
6. **Startup Orphan Reaping**: When `SessionManager.Start()` runs, `ReapOrphans` scans the operating system for running Chrome processes whose command line contains a temporary `user-data-dir` and whose parent launcher process is dead. It terminates those orphaned process trees and reclaims disk space.
7. **Same-URL Reload**: Calling `Navigate` with the current URL reloads the page rather than no-opping, ensuring fresh DOM parsing.
8. **Unattended Navigation Detection**: When a page navigates due to client-side redirects, meta-refreshes, or background scripts outside of an explicit `browser_act` step, codeNERD reifies this as an unattended navigation and reports it in the next observation.

### 3.2 Facts and Predicate Schema

| Predicate | Arity and Bound Types | Description |
|---|---|---|
| `attended` | `(SessionID, Timestamp)` bound `[/string, /number]` | Records the timestamp of an intentional, model-directed navigation or action. |
| `unattended_navigation` | `(SessionID, URL, Timestamp)` bound `[/string, /string, /number]` | Derived or emitted when `navigation_event` occurs without a corresponding `attended` mark. |

### 3.3 The Go/Mangle Split and "Even Better" Design

- **Even Better Rule 9 (Reaper as Lifecycle Guard, Not Tool)**: Upstream considered exposing process cleanup commands. In codeNERD, the reaper runs strictly at lifecycle boundaries: startup (`ReapOrphans`) and shutdown (`KillProcessTree`). It is never exposed as an LLM tool. The model never receives shell-level process killing capabilities.
- **Even Better Rule 10 (Unattended Navigation Reification)**: Go records the timestamps of explicit action batches. When a navigation event occurs without an active action batch, Go asserts `unattended_navigation`, which Mangle surfaces in `browser_observe` and `browser_reason` outputs.

### 3.4 Failure Modes

- **Antivirus Interference**: Launcher binaries are blocked if leakless helpers are used. Mitigated by setting `Leakless(false)` unconditionally.
- **Catastrophic Profile Deletion**: If directory resolution logic resolves an empty string or the root Chrome directory, user data could be lost. Mitigated by `removeUserDataDir`'s multi-point safety validation, ensuring the directory explicitly contains the temporary prefix and refusing execution otherwise.
- **Zombie Process Leaves**: Windows `taskkill` may fail under access restrictions. Mitigated by following `taskkill` with direct Win32 process handles via `TerminateProcess`.

### 3.5 Verification and Tests

- **Unit Tests**: `internal/browser/reaper_test.go` porting upstream tests `TestReapOrphans_ReapsOnlyDeadParentTrees` and `TestRemoveUserDataDir_SafetyRefusal` using a mock `ProcessQuerier`.
- **Live Chrome Tests**: Verification in `internal/browser/session_lifecycle_test.go` ensuring that launching and shutting down a managed browser leaves zero running Chrome processes carrying that temporary user-data path.

---

## 4. Capability Group 3: Progressive Observation, Storage, and Control Ranking

### 4.1 Implementation Details

Progressive observation allows the agent to inspect the page at multiple levels of granularity without exhausting context budgets (`internal/browser/progressive_observe.go:1-690`).

The port adds two dedicated observation modes and refines control ranking:
1. **Text Observation Mode (`mode: text`)**: Extracts clean, readable text content from the active page, stripping script, style, and svg tags, collapsing excessive whitespace, and formatting headings and paragraphs. Useful for document extraction and article reading.
2. **Storage Observation Mode (`mode: storage`)**: Inspects `localStorage`, `sessionStorage`, and non-HttpOnly cookies. For each item, it reports key name, storage type, size in bytes, and decodes JSON structures to extract JSON Web Token (JWT) metadata (`iat`, `exp`, `iss`) without exposing the raw secret values.
3. **Elimination of Placeholder as Label**: Resolves the bug where `placeholder` was treated as an element's accessibility name (`progressive_observe.go:628`). Elements are named strictly by `aria-label`, visible inner text, `title`, or `alt`. If an element has only a placeholder, it is reported under an explicit `placeholder` field.
4. **Hydration Classification**: Elements are classified according to their React hydration status:
   - `owned`: Rendered by React with attached fibers and handlers.
   - `foreign`: Third-party widget library DOM nested inside a React root.
   - `dead`: Native HTML elements inside a React root that lack React fibers or props; clicking them performs no action.
   - `unhydrated`: A page containing React scripts where the root component failed to mount after three seconds.
5. **Visible-First Locator Ordering**: When resolving text or label queries, matches are sorted: visible exact matches take top priority, followed by visible case-insensitive matches, visible substring matches, and finally hidden elements. A hidden exact match never shadows a visible partial match.
6. **Smart Screenshot Annotations**: In screenshot mode, visible interactive elements can be highlighted with bounding box overlays and reference IDs, or a screenshot can be cropped to a single reference element.

### 4.2 Facts and Predicate Schema

| Predicate | Arity and Bound Types | Description |
|---|---|---|
| `storage_entry` | `(SessionID, Store, Key, Kind, Exp)` bound `[/string, /string, /string, /string, /number]` | Metadata for a cookie or web storage item. Kind is "jwt", "json", or "opaque". Exp is epoch ms or 0. |
| `auth_expired` | `(SessionID, Store, Key)` bound `[/string, /string, /string]` | Derived when `storage_entry` has an expiration timestamp prior to the current session time. |
| `dead_control` | `(SessionID, Ref, Label)` bound `[/string, /string, /string]` | Identifies an element inside a React root that possesses no attached event handlers. |
| `unhydrated_page` | `(SessionID, URL)` bound `[/string, /string]` | Flags a page that has failed to complete framework hydration. |
| `action_candidate` | `(SessionID, Ref, Label, Action, Priority, Reason)` bound `[/string, /string, /string, /string, /number, /string]` | Ranked interactive recommendations derived across buttons, links, inputs, tabs, and widgets. |

### 4.3 The Go/Mangle Split and "Even Better" Design

- **Even Better Rule 4 (Storage Expiry in Logic)**: In BrowserNERD, storage inspection parsed values in Go. In codeNERD, the page evaluation script extracts only structural metadata and JWT timestamps; raw tokens never leave the browser. Go asserts `storage_entry`. Mangle rules derive `auth_expired(Session, Store, Key)` and join it with HTTP 401/403 `failed_request` facts to deduce authentication failure root causes.
- **Even Better Rule 5 (Action Priority as Logic Rules)**: Rather than sorting elements using arbitrary heuristic numbers in Go, ranking rules are declared in `policy/browser.mg` (`action_candidate`). High-priority primary actions (e.g. submit buttons, open comboboxes, active tabs) are derived with distinct priority ranks. Go groups identical labels for display presentation and discards elements marked as `dead_control`.

### 4.4 Failure Modes

- **Token Exfiltration**: Storage dumps can inadvertently expose session tokens or passwords. Mitigated by extracting only key names, sizes, and decoded timestamp claims; raw storage values are withheld by the in-page JavaScript probe.
- **Stale Reference Resolution**: Navigations invalidate DOM nodes. Calling actions on outdated references produces errors. Mitigated by assigning generation-bound identifiers to references that are cleared on navigation.

### 4.5 Verification and Tests

- **Unit Tests**: Tests in `internal/browser/observe_units_test.go` verifying locator ranking order, storage JSON parsing, and label extraction without placeholder fallback.
- **Live Chrome Tests**: Integration tests in `internal/browser/observe_probe_live_test.go` validating hydration classification and storage inspection against a live page containing simulated JWT tokens and React widgets.

---

## 5. Capability Group 4: Structured Action Execution, Locators, and Verification

### 5.1 Implementation Details

`browser_act` provides sequential, bounded action execution (`internal/browser/progressive_action.go:1-772`). The port enhances execution rigor:

1. **Pre-Batch Validation**: The entire array of operations (capped at 25 steps) is validated before step zero begins (`act_ops.go:232-541`). If any step contains missing required fields, illegal values, or unresolved `value_env` placeholders, the entire batch is rejected immediately without executing partial actions.
2. **Polymorphic Locators**: Elements may be targeted via:
   - `ref`: Opaque reference from the latest observation.
   - `text`: Visible inner text.
   - `label`: Associated label or `aria-label`.
   - `role` and `name`: ARIA semantic role combined with accessible name.
   - `target`: Coordinate or semantic matcher.
3. **Read-Back Verification**: For mutating form actions (`type`, `fill`, `select`), the driver reads the element's value back immediately after the event dispatch to verify that the target accepted the input. For custom select dropdowns with hidden native `<select>` tags, the driver automatically targets the visible custom trigger.
4. **Post-Batch Effects Digest**: Following batch execution, the driver waits for network and DOM quietude and compiles an effects digest: new console errors, non-asset HTTP 4xx/5xx responses, pending requests, active dialogs, triggered downloads, and new toast notifications.
5. **Path-Confined File Uploads**: Implements the `upload` operation for `<input type="file">` elements using CDP's `DOM.setFileInputFiles`. Uploaded files are strictly validated against workspace boundaries via `internal/browser/security/path_policy.go:139-159`. Attempts to upload files outside the workspace root or via traversing symlinks fail closed.
6. **Hover and Focus Operations**: Exposes `hover` to trigger CSS `:hover` states and mouseover event listeners.
7. **Honeypot Gate Preservation**: The existing `guardElement` check (`progressive_action.go:324`) is applied across all locator types (text, label, role, coordinate) to ensure hidden traps cannot be triggered via alternative selectors.

### 5.2 Facts and Predicate Schema

| Predicate | Arity and Bound Types | Description |
|---|---|---|
| `act_batch` | `(SessionID, BatchID, StartedMs)` bound `[/string, /string, /number]` | Marks the initiation of a sequential action batch for causal correlation. |
| `click_event` | `(SessionID, NodeID, Timestamp)` bound `[/string, /string, /number]` | Confirms physical or simulated click dispatch on an element. |
| `input_event` | `(SessionID, NodeID, Value, Timestamp)` bound `[/string, /string, /string, /number]` | Confirms input value change, redacted where appropriate. |

### 5.3 The Go/Mangle Split and "Even Better" Design

- **Even Better Rule 3 (Derived Action Effects)**: In BrowserNERD, `act_effects.go` calculated side effects through imperative loops. In codeNERD, Go asserts `act_batch(Session, BatchID, StartedMs)` and waits for network settlement. Mangle rules join `act_batch` with subsequent event streams to derive the causal effects of the batch.
- **Even Better Rule 7 (Constitutional Action Partitioning)**: In `internal/core/defaults/policy/constitution.mg:161-162`, `browser_act` was formerly defined as a blanket `safe_action`. With the introduction of file uploads and dialog actions, this rule is partitioned: standard interactions (`navigate`, `click`, `type`, `fill`, `key`, `history`) remain permitted as safe actions, whereas `upload` and armed dialog accepts require dedicated constitutional permissions.

### 5.4 Failure Modes

- **Honeypot Evasion via Alternate Locators**: Using text or coordinate locators could potentially bypass reference-based honeypot detection. Mitigated by enforcing `guardElement` resolution across all locator branches prior to action dispatch.
- **Arbitrary File Exfiltration via Upload**: An unconstrained upload action could attach sensitive files (e.g. `.ssh/id_rsa` or credentials). Mitigated by strict containment validation in `internal/browser/security/path_policy.go`.
- **Silent Input Failures**: JavaScript frameworks often ignore value property assignments if change events are not dispatched. Mitigated by verifying field values via read-back after typing.

### 5.5 Verification and Tests

- **Unit Tests**: `internal/browser/security/path_policy_test.go` verifying upload path traversal prevention and symlink refusal; `internal/browser/act_ops_test.go` verifying pre-batch validation.
- **Live Chrome Tests**: `internal/browser/act_live_test.go` verifying form filling with read-back verification, custom select interaction, upload confinement, and hover effects on live test pages.

---

## 6. Capability Group 5: Diagnostic Reasoning, Bucketed Causality, and Backend Correlation

### 6.1 Implementation Details

`browser_reason` provides diagnostic reasoning over browser evidence (`internal/tools/research/browser_reasoning.go:1-1172`). Rather than dumping unstructured event logs, it computes causal chains and root-cause explanations.

The port introduces the following enhancements:
1. **Network Diagnostic Topic (`topic: network`)**: Groups HTTP calls by method and URL, highlights data API endpoints before static assets, calculates latencies, and detects request storms (rapid duplicate requests within milliseconds).
2. **Body Shape Inspection (`body: true`)**: When diagnosing failed requests, the tool presents the structural JSON schema (object keys and value types) of the error response rather than printing the raw payload, conserving context tokens while explaining API validation errors.
3. **Repeated Console Message Aggregation**: Identical console errors are grouped with occurrence counts and first/last timestamps, preventing repetitive logs from dominating the diagnostic summary.
4. **Structured Docker Backend Correlation**: Integrates with `internal/browser/docker_correlation.go:81-132`. Container logs are matched against failed browser requests by exact endpoint path and timestamp window.
5. **Defensive Error Handling for Containers**: If container correlation is enabled and all configured containers fail to return logs or Docker is unreachable, the tool returns an explicit error rather than silently reporting empty success.

### 6.2 Facts and Predicate Schema

The causal and diagnostic rules are ported into `internal/core/defaults/policy/browser.mg`:

| Predicate | Arity and Bound Types | Description |
|---|---|---|
| `asset_resource_type` | `(Type)` bound `[/string]` | Fact table declaring static asset categories: Image, Font, Stylesheet, Media, Manifest, TextTrack, Other. |
| `asset_http_error` | `(SessionID, ReqID)` bound `[/string, /string]` | Identifies that an HTTP error belongs to a static asset. |
| `failed_request_done` | `(SessionID, ReqID, URL, Status, Timestamp)` bound `[/string, /string, /string, /number, /number]` | True API failure (5xx or non-asset 4xx). |
| `network_failure` | `(SessionID, ReqID, URL, ErrorText, Timestamp)` bound `[/string, /string, /string, /string, /number]` | True transport failure (excluding client-initiated cancellations). |
| `console_error_bucket` | `(SessionID, Bucket, Message, Timestamp)` bound `[/string, /number, /string, /number]` | Console error assigned to a 2,000 ms time bucket (`div(Timestamp, 2000)`). |
| `request_failure_bucket`| `(SessionID, Bucket, ReqID, Timestamp)` bound `[/string, /number, /string, /number]` | Failed request assigned to a 2,000 ms time bucket. |
| `caused_by_candidate` | `(SessionID, ErrorTs, ConsoleErr, ReqID, FailTs)` bound `[/string, /number, /string, /string, /number]` | Joins errors and request failures within current or preceding time bucket where `FailTs <= ErrorTs` and difference < 2,000 ms. |
| `caused_by_nearest` | `(SessionID, ErrorTs, ConsoleErr, FailTs)` bound `[/string, /number, /string, /number]` | Aggregation rule selecting the maximum (most recent) `FailTs` for a console error. |
| `caused_by` | `(SessionID, ConsoleErr, ReqID)` bound `[/string, /string, /string]` | Conclusive derivation identifying the exact failed request that triggered the console crash. |

### 6.3 The Go/Mangle Split and "Even Better" Design

- **Even Better Rule 2 (Bucketed Causal Chain)**: BrowserNERD previously attempted pairwise quadratic time joins. In codeNERD, events are partitioned into two-second integer buckets (`div(T, 2000)`). The join is constrained to candidates in the same or immediately preceding bucket, ensuring linear evaluation performance even under high event volumes.
- **Even Better Rule 12 (Docker Backend Matching)**: In `docker_correlation.go`, all-container failure is transformed from a silent note into an explicit diagnostic error. Log searching prioritizes path matches corresponding to the failed request before falling back to temporal proximity.

### 6.4 Failure Modes

- **Evaluation Timeout on Event Surges**: Evaluating unindexed temporal joins across thousands of stream events can freeze the Mangle kernel. Mitigated by bucketing joins and enforcing epoch fact limits.
- **Misattributed Failures**: A background image 404 occurring immediately before an unrelated JavaScript error could be falsely identified as the root cause. Mitigated by excluding asset resource types from the `caused_by` rule.

### 6.5 Verification and Tests

- **Unit Tests**: Policy tests in `internal/core/defaults/policy/browser_test.go` verifying that 404 image errors do not derive `failed_request`, while 404 API calls do, and verifying nearest-failure causal selection.
- **Live Chrome Tests**: `internal/browser/browser_reasoning_live_test.go` loading a page that triggers a 500 API call followed by an uncaught script error, verifying that `browser_reason` reports the exact `caused_by` relation.

---

## 7. Capability Group 6: Declarative Fixtures and Test Execution

### 7.1 Implementation Details

Declarative fixtures allow regression tests to be defined in portable YAML/JSON files without writing Go code (`internal/browser/testspec/types.go:1-33`, `internal/browser/testspec/parser.go:1-350`).

The port adds the following capabilities:
1. **Vocabulary Compatibility**: Supports `operations` as a direct alias for `actions` in fixture specifications.
2. **Immediate Environment Variable Validation**: If a fixture references an environment variable via `value_env`, the variable is resolved and validated for non-emptiness before step zero begins.
3. **Empty Test Refusal**: Fixtures with zero assertions or zero operations are rejected during parsing (`testspec/parser.go:109-110`).
4. **Action Parity in Fixtures**: Fixtures support the full range of operations including `upload`, `hover`, and new locator types (`text`, `label`, `role`).

### 7.2 The Go/Mangle Split

- **Go Responsibilities**: Parses YAML/JSON fixtures, enforces byte and step ceilings (maximum 256 KB fixture size, 25 actions, 100 assertions), executes actions sequentially, and evaluates assertions against the kernel.
- **Mangle Responsibilities**: Validates assertions via read-only queries against live facts (`AssertFact`).

### 7.3 Failure Modes

- **Secret Leakage in Fixtures**: Hardcoded test passwords could be committed to git. Mitigated by enforcing `value_env` for sensitive inputs and redacting values before logging.

### 7.4 Verification and Tests

- **Unit Tests**: `internal/browser/testspec/parser_test.go` verifying validation of `operations`, empty assertion rejection, and missing `value_env` handling.
- **Live Chrome Tests**: `internal/tools/research/browser_test_test.go` running an end-to-end declarative fixture against a local test server.

---

## 8. Capability Group 7: Tool Surface, JIT Prompt Atoms, and Token Budget Discipline

### 8.1 Implementation Details

The model interacts with browser capabilities through modular tools and dynamic prompt atoms:

1. **Concise Tool Definitions**: Tool schemas in `internal/tools/research/browser_progressive.go` and `internal/tools/research/browser_reasoning.go` provide minimal, precise structural definitions without verbose instructional prose.
2. **JIT Capability Atoms**: Behavioral rules (e.g. prefer compact observations, re-observe after navigation, avoid clicking dead controls) are defined in YAML prompt atoms (`internal/prompt/atoms/capability/browser_progressive.yaml` and `browser_reasoning.yaml`). They are injected into context only when browser tools are active.
3. **Tool Definition Token Budget Gate**: A unit test enforces an upper token limit on tool definitions, measuring the JSON serialization length divided by 4 (`len(json)/4`), matching upstream BrowserNERD's smoke measurement in `cmd/smoke/main.go:106-116`. If tool descriptions expand excessively, the test fails.
4. **Per-Call Execution Deadlines**: Every browser tool invocation is bound by a context deadline (maximum 30 seconds for actions, 15 seconds for observations), preventing hung CDP calls from locking the agent loop.
5. **Internal Health Diagnostics**: codeNERD provides native Go health reporting covering browser binary availability, active sessions, and reaper metrics, without exposing an independent MCP server.

### 8.2 The Go/Mangle Split and "Even Better" Design

- **Even Better Rule 8 (Declared Signal-Aware Retention)**: In `internal/browser/fact_epoch.go:150-184`, saturation formerly resulted in dropping all subsequent facts indiscriminately. The new retention policy classifies facts: low-signal polling events (`dom_updated`, frequent mouse movements) are dropped first, while failure facts (`net_http_error`, `console_event`, `page_load_failed`) are preserved.
- **Even Better Rule 11 (Prompt Atoms over Schema Bloat)**: Operational instructions reside in JIT atoms rather than static tool description strings, keeping baseline context usage compact.

### 8.3 Failure Modes

- **Context Exhaustion via Schema Bloat**: Large tool definitions consume prompt tokens on every turn. Mitigated by the automated token budget test.

### 8.4 Verification and Tests

- **Unit Tests**: `internal/tools/research/browser_tool_budget_test.go` asserting that tool schema token counts remain below their specified ceilings.
- **Prompt Tests**: Verification that `browser_progressive.yaml` atoms compile and inject cleanly under researcher and tester shard intents.

---

## 9. Cross-Cutting Failure Modes and Recovery Safeguards

| Risk Category | Consequence | Mitigation Mechanism |
|---|---|---|
| Unhandled JavaScript Dialog | Browser renderer blocks indefinitely | Automatic dismiss via default-deny dialog handler installed prior to navigation |
| Antivirus False Positive | Rod launcher helper quarantined, breaking launch | Unconditional `Leakless(false)` configuration |
| Orphaned Browser Accumulation | Memory and process table exhaustion | Startup orphan reaper and shutdown process-tree termination |
| Accidental Data Loss on Cleanup | User profiles or workspace files deleted | Multi-stage safety validation ensuring directory path contains temporary markers |
| Unsafe File Exfiltration | Arbitrary system files uploaded via file inputs | Strict workspace root and symlink validation in `path_policy.go` |
| Trapping in Honeypot Controls | Agent triggers invisible administrative actions | Honeypot derivation gate evaluated across all locator types before execution |
| Context Flooding via Verbose Tools | Context window exhausted by tool descriptions | Strict token budget test on tool schemas; guidance moved to JIT atoms |

---

## 10. Test Verification Strategy and Upstream Traceability

To claim parity, every capability must satisfy the acceptance gate: passing unit tests and passing live Chrome tests executing codeNERD's production code path.

### Upstream Test Porting Map

| Upstream Test File | Target codeNERD Test | Verification Scope |
|---|---|---|
| `browser_instances_test.go` | `internal/browser/session_lifecycle_test.go` | Detached tabs, shared vs isolated sessions |
| `reaper_test.go` | `internal/browser/reaper_test.go` | Process tree killing, dead launcher detection, deletion safety |
| `event_stream_test.go` | `internal/browser/session_manager_dom_test.go` | Pre-nav event subscriptions, error parsing |
| `observe_units_test.go` | `internal/browser/observe_units_test.go` | Locator ranking, text mode, storage extraction |
| `act_ops_test.go` | `internal/browser/act_ops_test.go` | Pre-batch validation, input read-back, locators |
| `path_policy_test.go` | `internal/browser/security/path_policy_test.go` | Upload file path confinement |
| `reason_diagnosis_test.go` | `internal/core/defaults/policy/browser_test.go` | Bucketed causality, asset 4xx filtering |
| `match_test.go` | `internal/browser/docker_correlation_test.go` | Path-based container log matching |
| `cmd/smoke/main.go` (upstream) | `internal/tools/research/browser_tool_budget_test.go` | Tool definition token budget ceilings |
| `probe_harness_live_test.go` | `internal/browser/browser_probe_live_test.go` | End-to-end live Chrome test against probe suite |
