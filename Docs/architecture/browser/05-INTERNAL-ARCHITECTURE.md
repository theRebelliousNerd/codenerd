# 05 — Internal Architecture: browser

> Last verified against codebase: 2026-08-09
> Sources: `session_manager.go`, `session_lifecycle.go`, `session_manager_dom.go`, `fact_redaction.go`, `security/`, `honeypot.go`

## 1. Component diagram

```
                    ┌─────────────────────────────────────┐
                    │           SessionManager            │
                    │  cfg, engine EngineSink, mu,        │
                    │  browsers map, sessions map,        │
                    │  redactor, path policy, reaper      │
                    └──────────────┬──────────────────────┘
           Start/Connect           │
                                   ▼
                    ┌─────────────────────────────────────┐
                    │     Chrome (CDP WebSocket)          │
                    │     controlURL                      │
                    └──────────────┬──────────────────────┘
                                   │ Shared or isolated Page
              ┌────────────────────┼────────────────────┐
              ▼                    ▼                    ▼
      sessionRecord          sessionRecord         event stream
      meta + *rod.Page       ...                   goroutines
              │
              ├─ Navigate / Click / Type / Screenshot
              ├─ SnapshotDOM → captureDOMFacts
              ├─ ReifyReact → fiber JS → facts
              └─ ForkSession → cookies + storage

                    ┌─────────────────────────────────────┐
                    │        HoneypotDetector             │
                    │  engine *mangle.Engine              │
                    │  AnalyzePage / IsHoneypot / Links   │
                    └─────────────────────────────────────┘
```

## 2. Key types (internal)

| Type | Visibility | Role |
|------|------------|------|
| `Session` | exported | Public metadata JSON |
| `sessionRecord` | private | meta + page + isolated context + stream cancel |
| `BrowserInstance` | exported | Managed browser metadata and tab count |
| `Config` | exported | Launch, viewport, timeouts, ingestion flags |
| `eventThrottler` | private | Per-key rate limit for CDP fact spam |
| `EngineSink` | exported interface | `AddFacts([]mangle.Fact) error` |
| `engineAdapter` | private | Wraps `*mangle.Engine` |
| `SessionManager` | exported | Owns bounded browsers, sessions, streams, and evidence policy |
| `HoneypotDetector` | exported | Rule-based analysis |
| `DetectionResult`, `Link` | exported | Honeypot API results |

## 3. Lifecycle state machine (manager)

```
                  NewSessionManager*
                         │
                         ▼
                   [disconnected]
                    browser=nil
                         │ Start(ctx)
          ┌──────────────┼──────────────────┐
          │ loadSessionsLocked              │
          │ resolve controlURL:             │
          │  DebuggerURL | Launch | default │
          │ rod.Connect                     │
          ▼                                 │
   [connected] ◄──── Version() OK ─── reuse │
          │                                  │
          │ Version() fail → Close, reset map
          │
          │ CreateTab / AttachToBrowser / ensureStarted
          ▼
   [sessions: active | attached | forked | detached]
          │
          │ Shutdown
          ▼
   cancel streams, close pages/contexts, close browsers, clear maps
```

Session statuses observed in code:

| Status | When |
|--------|------|
| `active` | After `CreateSession` |
| `attached` | After `Attach` |
| `forked` | After `ForkSession` metadata update |
| `detached` | After load from SessionStore (page=nil) |

## 4. CreateSession flow

```
ensureStarted
  → selected browser or explicit Incognito()
  → Page(about:blank)
  → EmulationSetDeviceMetricsOverride (viewport)
  → Navigate with NavigationTimeout
  → uuid session ID, store sessionRecord
  → manager-owned startEventStream (if sink != nil)
  → redacted private persistSessions
```

## 5. Event stream architecture

`startEventStream` returns immediately after launching a goroutine that:

1. Optionally `DOMEnable` + initial `captureDOMFacts` when DOM ingestion enabled and level ≠ minimal.  
2. Injects page hooks (`window.__browsernerdEvents`) for click/input/change + MutationObserver on `data-state*`.  
3. `EachEvent` for `PageFrameNavigated` → `navigation_event`, `current_url` + metadata URL update.  
4. `EachEvent` for console, `NetworkRequestWillBeSent`, `NetworkResponseReceived`, `DOMDocumentUpdated`.  
5. Ticker 500ms drains `__browsernerdEvents` → `click_event` / `input_event` / `state_change`.  
6. WaitGroup of three: nav waiter, rest waiter, poller.

**Skip condition:** `m.engine == nil` → entire stream disabled (debug log only).

## 6. DOM capture pipeline

Per node (up to 200):

| Predicate family | Purpose |
|------------------|---------|
| `dom_node`, `dom_text`, `dom_attr`, `dom_layout` | Schema-aligned tree |
| `attribute` (and atomized true/-1) | Honeypot attribute rules |
| `element`, `position`, `geometry` | Spatial + honeypot geometry |
| `interactable` | button/a/input/textarea/select classification |
| `computed_style`, `css_property` (string + atom) | Style-based honeypot |

Visibility flag on layout uses display/visibility/opacity/rect heuristics in page JS.

## 7. React reification pipeline

1. Eval JS walks `__reactFiber*` from root / `#root` / body.  
2. Sanitize props/state to primitives.  
3. Emit `react_component`, `react_prop`, `react_state` (hook index coerced to int64), `dom_mapping`.  
4. Copy/redact facts, then `EngineSink.AddFacts`; requires non-nil sink.

## 8. Honeypot analysis pipeline

```
AnalyzePage(page)
  → emitPageFacts (element, css_property, position, attribute, link)
  → engine.EvaluateRule("is_honeypot")
  → for each hit: getHoneypotReasons + calculateConfidence
```

Confidence: base 0.5 + 0.15 × reason count, cap 1.0.

`GetSafeLinks` filters out links with any reason; `GetAllLinksWithAnalysis` returns both.

## 9. Persistence

- **SessionStore** path: JSON marshal of `[]Session` metadata only (no cookies in file).  
- Load marks sessions detached with `page=nil`.  
- CLI additionally writes **control.txt** for debugger URL sharing across process invocations.

## 10. ForkSession

Snapshot Network cookies + localStorage/sessionStorage JSON → CreateSession → SetCookies + restoreStorage → status `forked`.

## 11. Threading model

| Concern | Mechanism |
|---------|-----------|
| Session map | `sync.RWMutex` on manager |
| Event throttler | own `sync.Mutex` |
| Event stream | detached goroutines; uses page Context |
| Research tools | package-level `sync.Once` manager + mutex |

No global lock across managers.
