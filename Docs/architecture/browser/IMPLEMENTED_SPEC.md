# browser — Implemented Spec (Deep-Dive)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go  
> Package: `codenerd/internal/browser`  
> Primary sources: `internal/browser/*.go`  
> Companion logic: `internal/core/defaults/schemas_browser.mg`, `policy/browser.mg`, `policy/browser_honeypot.mg`  
> Scale: **3** non-test Go files ≈ **1,900** lines; **6** test files; **0** package-local `.mg`

## 1. Overview

`internal/browser` is codeNERD’s **Browser Physics** implementation: a go-rod control plane over Chrome that turns live pages into **Mangle facts** the executive kernel can reason over. It is adapted from BrowserNERD and oriented to Cortex §9.0.

The package has two public centers of gravity:

1. **`SessionManager`** — owns Chrome connection, multi-session pages, CDP/JS event streams, DOM snapshot, React Fiber reification, navigation and interaction primitives.  
2. **`HoneypotDetector`** — extracts element/CSS/geometry facts into a `*mangle.Engine` and evaluates `is_honeypot` / reason predicates defined in policy.

It does **not** own constitutional permission, VirtualStore routing, or chat boot assembly. Those wire *to* this package (partially).

### Key characteristics

| Property | Value |
|----------|-------|
| Automation engine | go-rod + system Chrome (CDP) |
| Default headless | **false** (`DefaultConfig`) |
| Default viewport | 1920×1080 |
| Default nav timeout | 30s |
| Default event throttle | 100ms |
| DOM capture budget | 200 nodes per snapshot |
| Session isolation | Incognito context per CreateSession |
| Fact interface | `EngineSink.AddFacts` |
| Logging | `logging.CategoryBrowser` |
| Workspace store (CLI) | `.nerd/browser/sessions.json` |
| Control URL file (CLI) | `.nerd/browser/control.txt` |

### High-level control flow

```
Caller (CLI | research tool | future chat/tactile)
   │
   ├─ NewSessionManager / WithSink
   ├─ Start(ctx) ──► launch or attach Chrome
   ├─ CreateSession / Attach / Fork
   │       │
   │       ├─ effect: Navigate / Click / Type / Screenshot
   │       └─ observe: startEventStream + SnapshotDOM + ReifyReact
   │                         │
   │                         ▼
   │                  EngineSink → mangle.Fact[]
   │
   └─ HoneypotDetector.AnalyzePage(page)
            │
            ▼
     PushFact(element/css/position/…)
     EvaluateRule("is_honeypot")
```

Fact-flow in the wider system (when fully wired):

```
user_intent → kernel next_action → tool/shard
  → SessionManager → facts → schemas_browser predicates
  → browser*.mg policy (is_honeypot / safe_interactable)
  → permitted actions / articulation
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Config + defaults | **Implemented** | `DefaultConfig`, getters |
| Start / connect / launch fallback | **Implemented** | DebuggerURL, Launch[], default launcher |
| Session CRUD + list | **Implemented** | Create, Attach, Get, List, Page |
| Navigate / Click / Type / Screenshot | **Implemented** | Selector-based |
| Fork (cookies + storage) | **Implemented** | Best-effort restore |
| Session metadata persist | **Implemented** | Optional SessionStore JSON |
| Event stream (nav/console/net/DOM/hooks) | **Implemented** | Requires non-nil sink |
| DOM capture / SnapshotDOM | **Implemented** | Max 200 nodes; multi-predicate |
| React Fiber reify | **Implemented** | Best-effort; needs fiber keys |
| Honeypot detector | **Implemented** | Depends on engine+policy load |
| CLI launch/session/snapshot | **Implemented** | `cmd/nerd/cmd_browser.go` |
| Research modular tools | **Implemented** | Nil engine — no reify |
| Chat boot live manager | **Partial** | Field exists; nil until needed; no construct path found |
| Tactile BrowserManager inject | **Partial** | Setter + routes; boot skips if nil |
| VS handleBrowse | **Stub** | Explicit refuse → shard |
| Package-local Mangle | **N/A** | Lives in core defaults |
| Unit/coverage tests | **Strong** | Large table coverage |
| Integration tests | **Present** | `//go:build integration` |

**Overall:** living production package — **not** pre-implementation. Integration into the full OODA loop is **partial**.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/browser/
  session_manager.go           # lifecycle, effects, React reify
  session_manager_dom.go       # event stream, DOM capture, persist helpers
  honeypot.go                  # honeypot detector
  session_manager_coverage_test.go
  start_coverage_test.go
  honeypot_coverage_test.go
  honeypot_test.go
  lifecycle_coverage_test.go
  browser_integration_test.go  # //go:build integration
```

### 3.2 Top non-test sources

| Path | ≈Lines | Purpose |
|------|-------:|---------|
| `internal/browser/session_manager.go` | 810 | SessionManager API surface |
| `internal/browser/session_manager_dom.go` | 678 | Observation pipeline |
| `internal/browser/honeypot.go` | 413 | Safety analysis API |

### 3.3 Companion Mangle (core)

| Path | Role |
|------|------|
| `internal/core/defaults/schemas_browser.mg` | All Decl for browser world |
| `internal/core/defaults/policy/browser.mg` | Spatial + safe_interactable |
| `internal/core/defaults/policy/browser_honeypot.mg` | is_honeypot derivations |
| `internal/core/kernel_init.go` | Loads schemas_browser.mg |

---

## 4. SessionManager deep dive

### 4.1 Construction

```go
NewSessionManager(cfg, *mangle.Engine)      // engineAdapter if non-nil
NewSessionManagerWithSink(cfg, EngineSink)  // tests / custom sinks
```

Nil engine/sink is allowed: manager still automates Chrome; observation is disabled.

### 4.2 Start algorithm

1. If `browser != nil` and `Version()` OK → return (reuse).  
2. Else close stale browser, clear sessions map.  
3. `loadSessionsLocked()` from SessionStore (status → `detached`).  
4. Resolve control URL:  
   - `cfg.DebuggerURL` if set  
   - else Launch binary with optional flags (Cut on `=`) + headless  
   - primary Launch fail → bare fallback Launch  
   - else default `launcher.New().Headless(...).Launch()`  
5. `rod.New().ControlURL(controlURL).Context(ctx).Connect()`  
6. Store browser + controlURL.

`ensureStarted` is the lazy entry used by session ops: RLock check, then Start.

### 4.3 CreateSession

- Incognito browser context  
- New page with initial URL  
- Device metrics override for viewport  
- Navigate (timeout) — navigate error currently ignored with `_ =` on create  
- UUID session ID; status `active`  
- `startEventStream`  
- `persistSessions`  

### 4.4 Attach / Fork

- **Attach:** `PageFromTarget`, status `attached`, stream, persist.  
- **Fork:** NetworkGetCookies + local/session storage JSON → CreateSession → SetCookies + restoreStorage → status `forked`.

### 4.5 Effects

| API | Rod primitive | Errors |
|-----|---------------|--------|
| Navigate | `page.Timeout(...).Navigate` | unknown session, timeout |
| Click | `Element(sel).Click(Left, 1)` | not found |
| Type | `Element(sel).Input(text)` | not found |
| Screenshot | `Screenshot(fullPage, nil)` | unknown session |

All call `ensureStarted` first.

### 4.6 ReifyReact

Inline JS walks React fiber tree from `[data-reactroot]` / `#root` / `body`. Emits:

- `react_component(id, name, parent)`  
- `react_prop(id, key, valueString)`  
- `react_state(id, hookIndexInt64, valueString)`  
- `dom_mapping(fiberId, domNodeId)` when stateNode has id  

Requires engine; returns facts after AddFacts.

---

## 5. Observation pipeline deep dive

### 5.1 startEventStream

Disabled if `engine == nil`.

Config derived:

- `captureDOM` = EnableDOMIngestion && level != minimal  
- `captureHeaders` = EnableHeaderIngestion && level != minimal  
- `consoleErrorsOnly` = level == minimal  
- throttler from EventThrottleMs  

Subsystems:

1. Optional initial DOM snapshot  
2. Inject `__browsernerdHooked` listeners (click/input/change + data-state MutationObserver)  
3. CDP PageFrameNavigated → navigation_event + current_url + meta URL  
4. CDP console / network request / response / DOMDocumentUpdated  
5. 500ms poll of `__browsernerdEvents` buffer  

Network enrichment: `request_initiator` with best external stack frame (skips chrome:// etc. via `isInternalScript`).

### 5.2 captureDOMFacts

Page script selects up to 200 elements; returns id/tag/text/parent/attrs/layout/styles. Go asserts dense multi-predicate facts (see [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md)).

Interactable classification:

| Tag | ElemType atoms |
|-----|----------------|
| button, a | `/click` |
| input checkbox/radio/submit/button/other | `/checkbox`, `/radio`, `/click`, `/input` |
| textarea, select | `/input` |

### 5.3 Persistence helpers

- `persistSessions` / `loadSessionsLocked` — metadata only  
- `snapshotStorage` / `restoreStorage` — JSON stringify of storage maps  

---

## 6. HoneypotDetector deep dive

### 6.1 AnalyzePage

1. `emitPageFacts` for interactive selectors:  
   `a, button, input, [onclick], [role='button'], [role='link']`  
2. Per element: tag → `element`; styles → `css_property`; shape → `position` (string coords); attributes; href → `link`  
3. `EvaluateRule("is_honeypot")`  
4. Reasons via `QueryFacts` on intermediate predicates  
5. Confidence = min(1.0, 0.5 + 0.15×N)  

### 6.2 Reason table (Go checklist)

Matches policy names for: css_hidden, css_invisible, opacity, offscreen, zero_size, aria_hidden, no_keyboard, suspicious_url, pointer_events_none.  
Also lists `honeypot_clip_hidden` and `honeypot_overflow_hidden` — **policy file does not fully define those intermediates** (alignment gap).

### 6.3 Link APIs

- `GetSafeLinks` — honeypots excluded  
- `GetAllLinksWithAnalysis` — both  
- Selectors synthesized as `a[href='...']` (fragile if quotes in URL)

### 6.4 Policy contract

`browser_honeypot.mg` comments note Decl live in schemas; string `fn:contains` unavailable — suspicious URL facts must come from Go (not implemented as classifier in package).

---

## 7. Mangle schema surface (summary)

Categories in `schemas_browser.mg`:

| Group | Example Decl |
|-------|----------------|
| Base element | element, css_property, computed_style, position, attribute, link, visible |
| Spatial / safety | left_of, above, honeypot_detected, safe_interactable, target_checkbox |
| Honeypot intermediates | honeypot_*, is_honeypot, high_confidence_honeypot |
| DOM extended | dom_node, dom_text, dom_attr, dom_layout |
| React | react_component, react_prop, react_state, dom_mapping |
| Network | net_request, net_response, net_header, request_initiator |
| Events | navigation_event, current_url, console_event, click_event, input_event, state_change |
| Interaction | interactable, geometry |

`browser.mg` spatial rules constrained to **interactable** pairs to avoid O(N²) on full DOM.

---

## 8. Integration map

### 8.1 Downstream

| Consumer | Path | Behavior |
|----------|------|----------|
| CLI | `cmd/nerd/cmd_browser.go` | Operator lifecycle + snapshot export |
| Research tools | `internal/tools/research/browser.go` | Shared manager, nil engine |
| Tactile router | `internal/shards/system/router.go` | Optional BrowserManager field |
| Chat types/boot | `cmd/nerd/chat/*` | Holds pointer; constructs nil |

### 8.2 Action / policy names

| Layer | Names |
|-------|-------|
| VS ActionType | browser_navigate, browser_extract, browser_screenshot, browser_click, browser_type, browser_close |
| Constitution safe_action | browser_navigate, browser_screenshot, browser_read_dom |
| Router patterns | browse, browser_navigate, browser_screenshot, browser_read_dom → browser_tool |
| Tool registry | browser_navigate/extract/screenshot/click/type/close |
| Intent routing | modular_tool_allowed browser_* under research/verify |

### 8.3 VirtualStore stance

`handleBrowse` returns failure with fact `browser_routing(operation, /requires_shard)`. Modular tool handler maps browser ActionTypes to tool args. Browser package is **not** imported by `internal/core`.

### 8.4 Wiring diagram (as of verification)

```
┌──────── CLI ────────┐   ┌──── research tools ────┐   ┌──── chat boot ────┐
│ throwaway Engine    │   │ nil Engine             │   │ browserMgr = nil  │
│ SessionManager      │   │ package singleton mgr  │   │ tactile if non-nil│
│ .nerd/browser/*     │   │ no fact stream         │   │                  │
└─────────────────────┘   └────────────────────────┘   └──────────────────┘
         │                          │                          │
         └──────────────┬───────────┴──────────────────────────┘
                        ▼
              internal/browser.SessionManager
                        │
                        ▼
                   Chrome (Rod)
```

---

## 9. Observability (package-local)

- Category: `browser`  
- Timers: start, create session, navigation, screenshot, honeypot analysis  
- Stream errors logged per session with predicate context  
- See [11-OBSERVABILITY.md](11-OBSERVABILITY.md)

---

## 10. Testing summary

| Layer | Mechanism |
|-------|-----------|
| Unit | Fake sinks, nil browser, table honeypot with real .mg load |
| Lifecycle | Single Chrome shared when LookPath finds binary |
| Integration tag | httptest + headless Chrome fact assertions |

Commands: see [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) and README.

---

## 11. Failure modes (index)

| ID | One-liner |
|----|-----------|
| FM-01 | Chrome missing / launch fail |
| FM-02 | Stale control URL |
| FM-03 | Dead TargetID after restart |
| FM-04 | Nil engine → no facts |
| FM-05 | Fact type reject |
| FM-06 | Selector miss |
| FM-07 | Nav timeout |
| FM-08 | Fact storm |
| FM-09/10 | Honeypot false neg/pos |
| FM-11 | Orphan Chrome |
| FM-12 | Dual managers |
| FM-13 | Empty React reify |
| FM-14 | Lifecycle races |
| FM-15 | VS browse refuse |

Details: [12-FAILURE-MODES.md](12-FAILURE-MODES.md).

---

## 12. Gaps pointer

Full matrix: [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

Top three:

1. No single Cortex-owned manager with live kernel sink.  
2. Research tools reify nothing.  
3. Chat/tactile BrowserManager remains nil at boot without completed on-demand construct.

---

## 13. Architectural principles (package)

See [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md). Short form:

1. Facts over pixels  
2. Mangle judges; Go extracts  
3. EngineSink isolation  
4. Session ID first-class  
5. Incognito isolation  
6. Budgeted capture  
7. On-demand Chrome  
8. RWMutex map safety  
9. Dual CSS encodings intentional  
10. System Chrome dependency  
11. Constitutional default deny for click/type  
12. Wiring audit before deletion  

---

## 14. Public API index

Constructors: `DefaultConfig`, `NewSessionManager`, `NewSessionManagerWithSink`, `NewHoneypotDetector`.  

SessionManager methods: Start, Shutdown, ControlURL, IsConnected, List, CreateSession, Attach, Page, UpdateMetadata, GetSession, ReifyReact, ForkSession, Navigate, Click, Type, Screenshot, SnapshotDOM.  

Honeypot methods: AnalyzePage, IsHoneypot, GetSafeLinks, GetAllLinksWithAnalysis.  

Types: Session, Config, EngineSink, SessionManager, HoneypotDetector, DetectionResult, Link.  

Full tables: [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md).

---

## 15. North-star fit

| Principle | Realization |
|-----------|-------------|
| LLM creative / kernel executive | Reification enables kernel; LLM does not invent honeypot status |
| permitted / default deny | Constitution lists safe browser subset; click/type not auto-safe |
| JIT prompt atoms | N/A inside package; tool names listed in prompt configs |
| Wiring before deletion | Multiple partial consumers — do not delete SessionManager APIs lightly |

---

## 16. Document set cross-links

| Doc | Content |
|-----|---------|
| [README.md](README.md) | Map + verify commands |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | Scores |
| [01-VISION.md](01-VISION.md) | Target vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Internals |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | API |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Deps |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logs |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Failures |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Backlog |

---

## 17. Verify (developer)

```powershell
go test ./internal/browser/ -count=1
go test -tags=integration ./internal/browser/ -count=1 -timeout 120s
go test ./internal/core/defaults/policy/ -run Honeypot -count=1
```

Manual:

```powershell
.\nerd.exe browser launch
.\nerd.exe browser session https://example.com
.\nerd.exe browser snapshot <session-id>
```

---

*End of living implemented spec for `internal/browser` — 2026-07-13.*
