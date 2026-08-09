# 11 — Observability: browser

> Last verified against codebase: 2026-08-09

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
- Nil sink: navigation-only tracking remains active; fact-producing streams are omitted

### Progressive tools

- Every observation returns a stable evidence handle shaped as
  `browser:<session>:g<generation>:<mode>`.
- Summary/compact/full detail and item caps bound agent-visible output.
- Screenshots report confined path, media type, byte count, and SHA-256 rather
  than returning image bytes in tool JSON.
- Action results expose operation outcome and safe metadata, never selectors,
  input values, or credential material.

### Live reasoning and waits

- `browser_act` exposes `started_ms` and `finished_ms`; the start value is the
  freshness watermark for follow-up waits.
- `browser_wait` reports bounded matched conditions or network/DOM stability,
  never a silent success from stale facts when fresh-only is enabled.
- `browser_reason` returns bounded derived failures, blockers, correlations,
  contradictions, changes, and recommendations from the live kernel.
- `browser_mangle` exposes only allowlisted, session-scoped, re-redacted facts;
  it is not a policy mutation or arbitrary kernel-query interface.

### Flight evidence

- The default-on recorder writes selected already-redacted fact batches and
  bounded observe/act/wait/reason summaries as per-session JSONL.
- One record is capped at 64 KiB; oversized records become digest/size summaries.
- Reads cap at 100 items and 8 MiB scanned; exports cap at 1,000 items.
- Files rotate by `max_evidence_file_bytes` and prune to
  `max_evidence_files`; responses expose files read, bytes scanned, and
  truncation.
- `browser_evidence` performs explicit bounded status/read/export operations;
  exports are confined and current-user-only.

### Spec delivery

- `browser_specs` reports corpus scope as specs/files/directory entries/bytes
  scanned, warnings, and catalog/result truncation.
- List/get return ranked compact metadata and bounded redacted excerpts;
  observe/act spec context carries the same scope fields.
- Check reports checked/skipped/violations plus `passed`, `failed`, `no_checks`,
  or `incomplete`; warnings or truncation can never become a clean pass.

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
- `EvidenceEnabled` (default true)
- `EvidenceDir` (default `.nerd/browser/traces`)
- `MaxEvidenceFiles` (default 16)
- `MaxEvidenceFileBytes` (default 4 MiB)
- `Specs` (default enabled under `.nerd/browser/specs`, with normalized source,
  file, byte, result, excerpt, catalog, warning, and traversal ceilings)

## 5. Operator artifacts

| Artifact | Observability role |
|----------|-------------------|
| `.nerd/browser/control.txt` | Live CDP endpoint for attach |
| `.nerd/browser/sessions.json` | Session inventory after crash |
| `.nerd/browser/snapshots/*.mg` | Post-mortem fact dumps from CLI |
| Allowed writable-root screenshot path | Progressive screenshot evidence; owner-only file with digest metadata |
| `.nerd/browser/traces/flight_<session>*.jsonl` | Rotated redacted per-session flight evidence |
| `.nerd/browser/traces/exports/*.jsonl` | Explicit bounded owner-only evidence selection |
| `.nerd/browser/specs/*.md` | Read-only configured spec input; not written by the runtime |

## 6. Metrics / glass box

No dedicated Prometheus counters in package. Glass box / tool event bus may observe tool-level browser calls from shards/tools when those systems are wired — not emitted from SessionManager itself.

## 7. Debug practices

1. Raise EventLoggingLevel and enable header ingestion for network issues.  
2. Use `nerd browser snapshot` after hang to see last DOM facts.  
3. Filter logs by category `browser`.  
4. If no facts appear, check sink binding and EventLoggingLevel `minimal`; nil-sink navigation metadata may still advance.
5. Stale control.txt after crashed launch → connect errors; delete and relaunch.
6. If an action ref is stale, observe again and use the new generation's ref.
7. If a wait times out, compare its `since_ms` watermark with recent session-scoped event timestamps; do not disable freshness to mask a missing event.
8. Use `browser_evidence status/read` before export; a disabled recorder or truncated scan is explicit in the tool result/error.
9. Inspect `browser_specs` warnings and catalog scope before trusting a check; `incomplete` means the requested corpus was not fully proven.
