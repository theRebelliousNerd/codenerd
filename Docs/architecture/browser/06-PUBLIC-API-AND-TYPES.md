# 06 — Public API and Types: browser

> Last verified against codebase: 2026-08-09
> Package: `codenerd/internal/browser`

## 1. Config

### `Config` (`session_manager.go`)

| Field | JSON | Role |
|-------|------|------|
| `DebuggerURL` | `debugger_url` | Existing CDP WebSocket URL |
| `Launch` | `launch` | Binary path + optional flags (`name` or `name=value`) |
| `Headless` | `headless` | Headless mode (default **false**) |
| `ViewportWidth` / `ViewportHeight` | … | Default 1920×1080 if zero getters |
| `NavigationTimeoutMs` | `navigation_timeout_ms` | Default 30000 |
| `SessionStore` | `session_store` | JSON path for session metadata |
| `EventLoggingLevel` | `event_logging_level` | `minimal` \| `normal` \| `verbose` |
| `EnableDOMIngestion` | `enable_dom_ingestion` | Default true |
| `HeaderIngestionMode` | `header_ingestion_mode` | `off` \| `redacted`; `DefaultConfig` = `redacted` (research), CLI = `off` (operator) |
| `EnableHeaderIngestion` | `enable_header_ingestion` | Legacy bool; true is read as `redacted` |
| `HoneypotGuard` | `honeypot_guard` | `off` \| `warn` \| `block`; default `block` |
| `MaxEpochEventFacts` | `max_epoch_event_facts` | Per-epoch event-stream fact budget; 0 = default 20000, negative = unbounded |
| `EventThrottleMs` | `event_throttle_ms` | Default 100 |
| `MultiTabDefault` | `multi_tab_default` | Shared tabs by default; pointer preserves explicit false |
| `MaxTabs` / `MaxBrowsers` | … | Defaults 32 / 4 |
| `IdleTabTimeoutMs` | `idle_tab_timeout_ms` | Zero disables reaping |
| `ExtraSensitiveKeys` | `extra_sensitive_keys` | Additional fact/log/result redaction keys |
| `WorkspaceRoot` / `WritableRoots` | … | Browser artifact path policy |
| `Specs` | `specs` | Nested bounded workspace spec catalog: named sources, roots/indexes/globs, and delivery ceilings |

### Constructors / helpers

| Symbol | Behavior |
|--------|----------|
| `DefaultConfig()` | Sensible defaults above |
| `(Config).IsHeadless()` | Headless flag |
| `(Config).GetViewportWidth/Height()` | Zero → 1920/1080 |
| `(Config).NavigationTimeout()` | Zero → 30s |
| `(Config).IsMultiTabDefault()` | Nil → true |
| `(Config).GetMaxTabs/GetMaxBrowsers()` | Non-positive → 32/4 |
| `(Config).GetIdleTabTimeout()` | Non-positive → disabled |
| `(Config).Specs.Normalize()` | Defaults/clamps catalog sources and file/result/excerpt limits |

## 2. Session

```go
type Session struct {
    ID, BrowserID, TargetID, URL, Title, Status string
    Isolated                                  bool
    CreatedAt, LastActive                     time.Time
}
```

## 3. EngineSink

```go
type EngineSink interface {
    AddFacts(facts []mangle.Fact) error
}
```

Used by SessionManager for all reification paths. Tests implement this interface.

## 4. SessionManager

### Constructors

| Symbol | Notes |
|--------|-------|
| `NewSessionManager(cfg Config, engine *mangle.Engine)` | Wraps engine in `engineAdapter` if non-nil |
| `NewSessionManagerWithSink(cfg Config, sink EngineSink)` | Direct sink injection |

### Lifecycle

| Method | Signature (abbrev) | Notes |
|--------|-------------------|-------|
| `Start` | `(ctx) error` | Connect/launch; reuse healthy browser; load sessions |
| `Shutdown` | `(ctx) error` | Close pages + browser |
| `ControlURL` | `() string` | DevTools WS URL |
| `IsConnected` | `() bool` | `browser != nil` |
| `LaunchAdditional` | `(ctx) (*BrowserInstance, error)` | Bounded independent browser process |
| `ListBrowsers` | `() []BrowserInstance` | Deterministic inventory + tab counts |
| `CloseBrowser` | `(ctx, browserID) error` | Close its tabs; promote another default |

### Sessions

| Method | Notes |
|--------|-------|
| `List` | All metadata copies |
| `CreateSession(ctx, url)` | Uses configured shared/isolation default |
| `CreateTab(ctx, browserID, url, isolated)` | Explicit browser/profile semantics |
| `Attach(ctx, targetID)` | PageFromTarget |
| `AttachToBrowser(ctx, browserID, targetID)` | Attach on selected browser |
| `FocusSession(ctx, id)` | Activate tab |
| `CloseSession(ctx, id)` | Cancel stream + close tab/context; idempotent |
| `GetSession(id)` | (Session, bool) |
| `Page(id)` | (*rod.Page, bool) — escape hatch |
| `UpdateMetadata(id, func)` | In-place meta transform |
| `ForkSession(ctx, sessionID, url)` | Clone cookies/storage into isolation |
| `Registry(id)` | Session-scoped opaque-ref registry; created lazily for restored records |
| `LoadSpecs(ctx)` | Loads the bounded workspace catalog; disabled/workspace-less managers fail closed |
| `SpecsConfig()` / `SpecsEnabled()` | Normalized delivery limits and catalog availability |

### Progressive observations and actions

| Method | Notes |
|--------|-------|
| `Observe(ctx, sessionID, ObserveOptions)` | Bounded state/nav/interactive/grid/hidden/screenshot/DOM/React/session disclosure |
| `ExecuteActions(ctx, sessionID, []ActionOperation, stopOnError)` | Closed sequence; hard cap 25 |
| `InteractRef(ctx, sessionID, ref, action, value, submit)` | Fingerprint re-identification; stale refs fail closed |
| `FillRefs(ctx, sessionID, fields, submit, submitButton)` | Bounded ref/value form batch |
| `PressKey(ctx, sessionID, key)` | Closed named/printable key vocabulary and modifiers |
| `History(ctx, sessionID, action)` | Back, forward, reload; invalidates ref generation |

Public progressive types: `ObserveOptions`, `ProgressiveObservation`,
`PageState`, `InteractiveElement`, `NavigationElement`, `GridObservation`,
`HiddenObservation`, `ScreenshotEvidence`, `ElementFingerprint`,
`ActionOperation`, `FillField`, `ActionStepResult`, and `ActionExecution`.

`ObserveOptions` also accepts `IncludeSpecs` and bounded `SpecTerms`; the
research wrapper attaches ranked catalog scope to observe/act responses only
when explicitly requested.

### Effects

| Method | Notes |
|--------|-------|
| `Navigate(ctx, sessionID, url)` | Timeout-bound |
| `Click(ctx, sessionID, selector)` | Left click once |
| `Type(ctx, sessionID, selector, text)` | `el.Input` |
| `Screenshot(ctx, sessionID, fullPage)` | `[]byte` PNG |
| `ResolveOutputPath(...)` | Confine artifact under writable roots |
| `SanitizeForEvidence(value)` | Redact before return/log/persist |

### Reification

| Method | Notes |
|--------|-------|
| `SnapshotDOM(ctx, sessionID)` | One-shot `captureDOMFacts` |
| `ReifyReact(ctx, sessionID)` | Returns facts; requires engine |

## 5. Honeypot API

### Types

| Type | Fields |
|------|--------|
| `DetectionResult` | ElementID, Selector, Reasons, Confidence, TagName, Href |
| `Link` | Selector, Href, Text, IsHoneypot, HoneypotReasons |

### `HoneypotDetector`

| Method | Notes |
|--------|-------|
| `NewHoneypotDetector(engine *mangle.Engine)` | Requires engine with schemas/rules loaded |
| `AnalyzePage(page)` | Full scan → `[]DetectionResult` |
| `IsHoneypot(page, selector)` | (bool, reasons, err) for one element |
| `GetSafeLinks(page)` | Non-honeypot links only |
| `GetAllLinksWithAnalysis(page)` | All links + flags |

**Note:** Detector uses `*mangle.Engine` directly (`PushFact`, `EvaluateRule`, `QueryFacts`), not `EngineSink`.

## 6. Fact predicates emitted (SessionManager)

| Predicate | Typical args | Source |
|-----------|--------------|--------|
| `navigation_event` | session, url, ms | CDP nav |
| `current_url` | session, url | CDP nav |
| `console_event` | session, level, msg, ms | CDP console |
| `net_request` | session, id, method, url, initType, ms | CDP network |
| `request_initiator` | session, id, type, parentRef | CDP network |
| `net_response` | session, id, status, latency, duration | CDP network |
| `net_failure` | session, id, error, blockedReason, ms | CDP network failure |
| `net_header` | session, id, req\|res, key, value | optional headers |
| `click_event` / `input_event` / `state_change` | session, … | bounded JS hook poll |
| `dom_updated` / `toast_notification` | session, …, ms | MutationObserver/hook poll |
| `browser_page_state` | session, url, loading, hasDialog, ms | progressive observe refresh |
| `dom_node` / `dom_text` / `dom_attr` / `dom_layout` | … | DOM capture |
| `element` / `position` / `geometry` / `interactable` | … | DOM capture |
| `attribute` / `css_property` / `computed_style` | … | DOM capture |
| `react_component` / `react_prop` / `react_state` / `dom_mapping` | … | ReifyReact |
| `link` / `interactable` / `visible` | qualified session ref + bounded observation data | Progressive observe |

Session qualification is part of the event schema, not a caller-side filter.
DOM node IDs are also session-qualified before assertion to prevent cross-tab
identity collisions.

## 7. Fact predicates (Honeypot emit)

| Predicate | Source method |
|-----------|---------------|
| `element` | emitPageFacts |
| `css_property` | computed styles |
| `position` | element shape quads (string coords) |
| `attribute` | all attributes |
| `link` | href |

Derived (policy, not emitted by detector as base): `is_honeypot`, `honeypot_*`, `high_confidence_honeypot`.

## 8. Native reasoning, spec, and declarative-test tools (companion research package)

| Tool | Contract |
|------|----------|
| `browser_mangle` | Read-only, session-scoped query/read/temporal/evaluate/await operations over an explicit browser-predicate allowlist; bounded results and no fact/rule mutation |
| `browser_wait` | Context-cancelable stable/fact/condition waits; fresh-only by default; accepts the `browser_act.started_ms` action watermark; timeout capped at 30 seconds |
| `browser_reason` | Refreshes page state and returns bounded health/failure/change views from live-kernel derived and event facts, scoped to the current route by default |
| `browser_evidence` | Status/read/export for redacted per-session JSONL evidence; reads cap items and scanned bytes; exports remain under configured writable roots |
| `browser_specs` | List/get/check named workspace-confined Markdown corpora; ranks by source/file/line/component/route/selector/terms and checks only declared single-atom present/absent invariants against one live session |
| `browser_test` | Create/inspect/generate/run strict portable fixtures; generation reads bounded redacted action-intent evidence, replay resolves unique selector-free semantic targets, credentials expand from `value_env` only in an execution copy, and assertions query the live session kernel with per-assertion fresh baselines |

The companion `internal/browser/specs` package exports `Source`, `Config`,
`Binding`, `Invariant`, `Spec`, `LoadResult`, `MatchInput`, `Match`,
`SelectedInvariant`, and `Catalog`. `NewCatalog`, `Catalog.Load`, `MatchSpecs`,
`CountMatchingSpecs`, and `SelectInvariants` enforce workspace and resource
boundaries before any document reaches a model-facing result.

The companion `internal/browser/testspec` package exports `Spec`, `Assertion`,
strict parse/marshal/normalize helpers, and execution-only environment
resolution. `browser.ElementMatcher` is the portable selector-free target type.

## 9. Non-exported helpers (test-visible same package)

Coverage tests call: `stringifyConsoleArgs`, `coalesceNonEmpty`, `isInternalScript`, `persistSessions`, `loadSessionsLocked`, `newEventThrottler`, `startEventStream`. These are package-private; external callers should not depend on them.
