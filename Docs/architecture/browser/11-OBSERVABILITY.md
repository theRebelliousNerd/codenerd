# 11 — Observability: browser

> Last verified against codebase: 2026-07-13

## 1. Logging category

| Item | Value |
|------|-------|
| Category constant | `logging.CategoryBrowser` = `"browser"` (`internal/logging/logger.go`) |
| Description | Browser automation, DOM events |
| Convenience | `logging.Browser`, `BrowserDebug`, `BrowserWarn`, `BrowserError` (`logger_convenience.go`) |

## 2. Timed operations

`logging.StartTimer(CategoryBrowser, name)` used for:

| Timer name | Location |
|------------|----------|
| `Browser session start` | `SessionManager.Start` |
| `Create browser session` | `CreateSession` |
| `Page navigation` | `Navigate` |
| `Screenshot capture` | `Screenshot` |
| `Honeypot page analysis` | `HoneypotDetector.AnalyzePage` |

## 3. Log density by flow

### Start / connect

- Info: starting, launching binary, fallback, default launcher, success  
- Debug: health check, load sessions, connect URL  
- Warn: stale connection, launch primary fail  
- Error: load sessions fail, launch fail, connect fail  

### Session ops

- Info: create/attach URL, navigate, click selector, type length, screenshot sizes  
- Error: unknown session, element not found, ensureStarted failures  

### Event stream

- Debug: stream config (level, captureDOM, captureHeaders)  
- Error: per-predicate AddFacts failures with `[session:%s]` prefix  
- Debug: skip when no engine  

### Honeypot

- Info: analysis start/complete counts  
- Debug: per-element detection, reasons, safe link filter counts  
- Error: emit facts / element query failures  

## 4. Config-driven telemetry volume

| `EventLoggingLevel` | Effect |
|---------------------|--------|
| `minimal` | Console errors/warnings only; no DOM capture stream; headers off |
| `normal` (default) | DOM capture if enabled; full console; headers if enabled |
| `verbose` | Same gates as normal for DOM/headers flags (level is lower-cased; not a separate verbose branch beyond non-minimal) |

Additional knobs:

- `EnableDOMIngestion` (default true)  
- `EnableHeaderIngestion` (default false)  
- `EventThrottleMs` (default 100) — reduces fact/log pressure  

## 5. Operator artifacts

| Artifact | Observability role |
|----------|-------------------|
| `.nerd/browser/control.txt` | Live CDP endpoint for attach |
| `.nerd/browser/sessions.json` | Session inventory after crash |
| `.nerd/browser/snapshots/*.mg` | Post-mortem fact dumps from CLI |

## 6. Metrics / glass box

No dedicated Prometheus counters in package. Glass box / tool event bus may observe tool-level browser calls from shards/tools when those systems are wired — not emitted from SessionManager itself.

## 7. Debug practices

1. Raise EventLoggingLevel and enable header ingestion for network issues.  
2. Use `nerd browser snapshot` after hang to see last DOM facts.  
3. Filter logs by category `browser`.  
4. If no facts appear, check `engine == nil` and EventLoggingLevel `minimal`.  
5. Stale control.txt after crashed launch → connect errors; delete and relaunch.
