# 04 — Architectural Principles: browser

> Last verified against codebase: 2026-07-13  
> Binding for changes under `internal/browser/` and its immediate callers.

## P1 — Facts are the product; pixels are evidence

Screenshots and raw HTML are secondary. Primary output is **structured Mangle facts** aligned with `schemas_browser.mg` (`dom_node`, `css_property`, `geometry`, `react_*`, event predicates). New observations should prefer new Decl + emitters over free-text-only returns.

## P2 — Logic judges safety; Go extracts evidence

Honeypot **judgment** is rule-derived (`is_honeypot`, `honeypot_detected`, `safe_interactable`). Go may emit CSS/geometry/attribute facts and intermediate facts (e.g. future `honeypot_suspicious_url`) but must not become a second, divergent safety policy with hard-coded if/else trees that ignore Mangle.

## P3 — EngineSink is the only write path into logic

`SessionManager` depends on `EngineSink` (not concrete kernel types beyond the constructor adapter). Tests use fake sinks. Callers that pass `nil` deliberately disable reification — that must be an explicit, documented choice (research tools today).

## P4 — Session identity is first-class

Every mutation (`Navigate`, `Click`, `Type`, `Screenshot`, `SnapshotDOM`, `ReifyReact`) is keyed by `sessionID`. Unknown session ⇒ error, not silent no-op (except `UpdateMetadata` which no-ops by design).

## P5 — Isolation by default (incognito)

`CreateSession` opens an **incognito** context then a page. Forking clones cookies + storage into a new session rather than sharing the same browsing context. Prefer this over global profile pollution.

## P6 — Budget the world model

- DOM capture capped at **200** nodes.  
- Event throttler (`EventThrottleMs`, default 100).  
- Event logging levels: `minimal` | `normal` | `verbose` control DOM/header/console verbosity.  

Unbounded MutationObserver → fact pipelines are forbidden without new budgets.

## P7 — On-demand Chrome, not boot-time Chrome

Chat boot must not launch Chromium for every interactive session. Start on first need; allow attach via `DebuggerURL` / CLI control file for long-lived operator browsers.

## P8 — Concurrency: one lock, many readers

`SessionManager.mu` protects browser pointer and session map. Event stream updates metadata under the same API. New fields touching shared maps must take the lock; long Rod calls should not hold write locks across network waits where avoidable (`ensureStarted` pattern).

## P9 — Schema dual-encoding is intentional

DOM capture emits both string-style and atom-style CSS (`"display"`/`"none"` and `/display`/`/none`) so string-bound runtime facts and atom fixtures both fire honeypot rules. Do not “simplify” by dropping one encoding without updating `browser_honeypot.mg` and tests.

## P10 — External Chrome is the dependency

go-rod launcher discovers/uses system Chrome. Package tests must Skip cleanly when Chrome is missing (`lifecycle_coverage_test.go` pattern). CI unit tests must not require Chrome; integration tests may.

## P11 — Constitutional default deny for high-risk gestures

Navigate / screenshot / read_dom are constitutionally `safe_action`. Click and type are higher risk and are **not** listed as safe in `constitution.mg`. Package APIs still allow them for operators/tools — callers that claim agent autonomy must layer policy checks.

## P12 — Wiring audit before deletion

Presence of multiple entry points (CLI, research tools, tactile field, VS action types) is partial integration, not dead code. Grep all of them before removing SessionManager APIs or honeypot surfaces.
