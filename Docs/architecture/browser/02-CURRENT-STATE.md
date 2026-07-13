# 02 — Current State: `internal/browser`

> Precise inventory as of 2026-07-13. Paths relative to repo root.

## 1. Package identity

| Property | Value |
|----------|-------|
| Import path | `codenerd/internal/browser` |
| Package comment | Browser automation with DOM/React reification into Mangle facts; adapted from BrowserNERD for Cortex 1.5.0 Browser Physics (§9.0) |
| Non-test Go files | **3** |
| Test Go files | **6** |
| Package-local `.mg` | **0** (schemas/policy live under `internal/core/defaults/`) |
| Primary third-party | `github.com/go-rod/rod` (+ launcher, proto) |
| UUID | `github.com/google/uuid` for session IDs |

## 2. File inventory (source)

| Path | ≈Lines | Role |
|------|-------:|------|
| `internal/browser/session_manager.go` | ~810 | Config, SessionManager lifecycle, Create/Attach/Fork, Navigate/Click/Type/Screenshot, ReifyReact |
| `internal/browser/session_manager_dom.go` | ~678 | Event stream, DOM capture, storage snapshot/restore, session persist/load helpers |
| `internal/browser/honeypot.go` | ~413 | HoneypotDetector: emit page facts, EvaluateRule, safe links |

**≈1,900 non-test LOC** concentrated in two SessionManager files + one detector.

## 3. File inventory (tests)

| Path | Build tag | Role |
|------|-----------|------|
| `session_manager_coverage_test.go` | (default) | Config, throttler, constructors, session map, stringify helpers, JSON round-trips, concurrent access |
| `start_coverage_test.go` | (default) | Start error paths, session store load, concurrent R/W, throttler timing |
| `honeypot_coverage_test.go` | (default) | Confidence math, reason table, error paths without Chrome, fail sinks |
| `honeypot_test.go` | (default) | Table-driven honeypot rules against loaded `schemas_browser.mg` + `browser_honeypot.mg` |
| `lifecycle_coverage_test.go` | (default) | Single shared Chrome lifecycle when launcher finds Chrome; else Skip |
| `browser_integration_test.go` | `integration` | httptest + headless Chrome: navigation facts, click/input events |

## 4. Companion files (outside package, load-bearing)

| Path | Role |
|------|------|
| `internal/core/defaults/schemas_browser.mg` | Decl for DOM, honeypot, React, network, events, interactable |
| `internal/core/defaults/policy/browser.mg` | Spatial `left_of`/`above`, `honeypot_detected`, `safe_interactable`, checkbox targeting |
| `internal/core/defaults/policy/browser_honeypot.mg` | Intermediate honeypot predicates + `is_honeypot` / `high_confidence_honeypot` |
| `internal/core/defaults/policy/browser_honeypot_test.go` | Policy-only unit tests |
| `internal/core/defaults/policy/constitution.mg` | `safe_action` for navigate/screenshot/read_dom |
| `internal/core/defaults/policy/delegation.mg` | `tool_capabilities(/browser, …)` |
| `internal/core/defaults/policy/system_routing.mg` | `routing_table(/browser, /browser_tool, /high)` |
| `internal/core/kernel_init.go` | Loads `schemas_browser.mg` into kernel program |
| `cmd/nerd/cmd_browser.go` | Cobra: launch / session / snapshot |
| `cmd/nerd/main.go` | Registers `browserCmd` |
| `cmd/nerd/chat/session_boot.go` | Declares `browserMgr` **nil until needed** |
| `cmd/nerd/chat/model_types.go` | `browserMgr` / `BrowserManager` fields |
| `internal/shards/system/router.go` | `SetBrowserManager`, tool routes for browser_* |
| `internal/tools/research/browser.go` | Modular tools wrapping SessionManager singleton |
| `internal/core/virtual_store_types.go` | ActionBrowser* ActionType constants |
| `internal/core/virtual_store_actions.go` | `handleBrowse` refuse + modular tool arg mapping |
| `internal/logging/logger.go` | `CategoryBrowser` |
| `internal/logging/logger_convenience.go` | `Browser` / `BrowserDebug` / `BrowserWarn` / `BrowserError` |

## 5. Runtime artifacts (workspace)

| Path pattern | Written by | Content |
|--------------|------------|---------|
| `.nerd/browser/sessions.json` | CLI `getBrowserConfig` SessionStore | JSON array of `Session` metadata |
| `.nerd/browser/control.txt` | `nerd browser launch` | DevTools WebSocket control URL |
| `.nerd/browser/snapshots/<session>_<unix>.mg` | `nerd browser snapshot` | Exported fact dump |

## 6. Hotspots

1. **`startEventStream`** — goroutine fan-out (nav wait, multi-event wait, 500ms poll of in-page buffer).  
2. **`captureDOMFacts`** — dense fact emission (multiple predicates per node; dual string/atom CSS encodings).  
3. **`ReifyReact`** — large inline JS fiber walker; type coercion for hook indices.  
4. **`HoneypotDetector.emitPageFacts`** — per-element style/position/attribute PushFact.  
5. **Dual ownership** — research package singleton vs chat `BrowserManager` field vs CLI ephemeral managers.

## 7. What is intentionally not here

- No HTTP API server inside the package  
- No Mangle source files inside `internal/browser/`  
- No VirtualStore interface implementation  
- No prompt atoms  
- BrowserMgr not constructed in default chat boot path (lazy/on-demand design comment)
