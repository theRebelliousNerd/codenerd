# 05 — Internal Architecture: browser

> Last verified against codebase: 2026-08-09
> Sources: `session_manager.go`, `session_lifecycle.go`, `session_manager_dom.go`, `element_registry.go`, `progressive_observe.go`, `progressive_action.go`, `fact_redaction.go`, `security/`, `honeypot.go`

## 1. Component diagram

```
                    ┌─────────────────────────────────────┐
                    │           SessionManager            │
                    │  cfg, engine EngineSink, mu,        │
                    │  browsers map, sessions map,        │
                    │  registries, redactor, path policy, │
                    │  reaper                             │
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
      + ElementRegistry
              │
              ├─ Navigate / Click / Type / Screenshot
              ├─ Observe → bounded views + opaque refs
              ├─ ExecuteActions → closed sequential plan
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
| `ElementRegistry` | exported | Owns generation-bound opaque refs and private selector resolution |
| `ObserveOptions`, `ProgressiveObservation` | exported | Bounded progressive observation request/result |
| `ActionOperation`, `ActionExecution` | exported | Closed sequential action request/result |
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
  → manager-owned startEventStream
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

With a nil sink, the stream still consumes main-frame navigation events so
session metadata and ref-generation invalidation remain correct. DOM, console,
network, and in-page event reification are enabled only when a sink is present.

## 6. Progressive observe/ref/action pipeline

```
Observe(session, mode/detail/bounds)
  → capture bounded page state
  → verify URL + registry generation did not change during capture
  → atomically register fingerprints for that generation
  → return redacted observations with opaque e<generation>_<n> refs
  → optional confined screenshot evidence (path, media type, bytes, SHA-256)

ExecuteActions(session, operations, stopOnError)
  → validate closed operation vocabulary and batch bounds
  → resolve refs privately through ElementRegistry
  → try stored selector candidates, then bounded fingerprint re-identification
  → execute in order and return redacted per-step results
```

Navigation clears the registry and advances its generation before explicit
navigation and when a real CDP main-frame navigation is observed. A captured
snapshot from an older generation cannot repopulate the registry. Selectors and
typed values remain private implementation details.

## 7. DOM capture pipeline

Per node (up to 200):

| Predicate family | Purpose |
|------------------|---------|
| `dom_node`, `dom_text`, `dom_attr`, `dom_layout` | Schema-aligned tree |
| `attribute` (and atomized true/-1) | Honeypot attribute rules |
| `element`, `position`, `geometry` | Spatial + honeypot geometry |
| `interactable` | button/a/input/textarea/select classification |
| `computed_style`, `css_property` (string + atom) | Style-based honeypot |

Visibility flag on layout uses display/visibility/opacity/rect heuristics in page JS.

## 8. React reification pipeline

1. Eval JS walks `__reactFiber*` from root / `#root` / body.  
2. Sanitize props/state to primitives.  
3. Emit `react_component`, `react_prop`, `react_state` (hook index coerced to int64), `dom_mapping`.  
4. Copy/redact facts, then `EngineSink.AddFacts`; requires non-nil sink.

## 9. Honeypot analysis pipeline

```
AnalyzePage(page)
  → emitPageFacts (element, css_property, position, attribute, link)
  → engine.EvaluateRule("is_honeypot")
  → for each hit: getHoneypotReasons + calculateConfidence
```

Confidence: base 0.5 + 0.15 × reason count, cap 1.0.

`GetSafeLinks` filters out links with any reason; `GetAllLinksWithAnalysis` returns both.

## 10. Persistence

- **SessionStore** path: JSON marshal of `[]Session` metadata only (no cookies in file).  
- Load marks sessions detached with `page=nil`.  
- CLI additionally writes **control.txt** for debugger URL sharing across process invocations.

## 11. ForkSession

Snapshot Network cookies + localStorage/sessionStorage JSON → CreateSession → SetCookies + restoreStorage → status `forked`.

## 12. Threading model

| Concern | Mechanism |
|---------|-----------|
| Session map | `sync.RWMutex` on manager |
| Event throttler | own `sync.Mutex` |
| Event stream | detached goroutines; uses page Context |
| Ref registry | own mutex; generation-checked batch registration |
| Research tools | package-level `sync.Once` manager + mutex |

No global lock across managers.
