# 10 — Testing Alignment: browser

> Last verified against codebase: 2026-08-09

## 1. Commands

```powershell
# Default package tests (unit + optional Chrome lifecycle)
go test ./internal/browser/ -count=1

# Verbose
go test ./internal/browser/ -count=1 -v

# Integration-tagged tests only (requires Chrome)
go test -tags=integration ./internal/browser/ -count=1 -timeout 120s

# Companion honeypot policy
go test ./internal/core/defaults/policy/ -run 'Honeypot|Browser' -count=1

# Research tool browser wrappers (other package)
go test ./internal/tools/research/ -run Browser -count=1

# Live progressive modular registry route
go test -tags=integration ./internal/tools/research/ -run TestBrowserProgressiveTools_Live -count=1

# Focused race gate
go test -race ./internal/browser ./internal/tools/research ./internal/prompt ./internal/prompt/sync
```

## 2. Test file map

| File | Tag | Focus |
|------|-----|-------|
| `session_manager_coverage_test.go` | — | Config defaults/getters, throttler, constructors, session map CRUD, JSON types, shutdown, concurrent access, startEventStream nil engine |
| `start_coverage_test.go` | — | Start failures (bad debugger, bad binary), session store load/invalid JSON, double Start healthy reuse, persist multi-session, concurrent R/W |
| `honeypot_coverage_test.go` | — | Confidence math, reasons from QueryFacts, ReifyReact/SnapshotDOM/Navigate/Click/Type/Screenshot/Fork/Attach error paths without healthy browser |
| `honeypot_test.go` | — | Full engine + schema + policy: display none, visibility, offscreen, zero size, suspicious URL fact, normal element |
| `lifecycle_coverage_test.go` | — | Shared Chrome: create, navigate, click, type, screenshot, snapshot, multi-page; Skip if no Chrome |
| `browser_integration_test.go` | `integration` | httptest Hello World + interaction facts (`dom_text`/`navigation_event`/`click_event`/`input_event`) |
| `element_registry_test.go` | — | Stable refs, copy isolation, navigation generation, stale snapshot refusal |
| `progressive_action_test.go` | — | Hard bounds, closed modes, stop-on-error |
| `internal/tools/research/browser_progressive_live_test.go` | `integration` | Live registry route: all BPAR-2 views/actions, secret sink/result check, confined screenshot |

## 3. Coverage strengths

- Pure logic: throttler, config zero-defaults, coalesce/isInternalScript, stringifyConsoleArgs.  
- Session map without Chrome via injected records / nil browser error paths.  
- Honeypot rule table against real `.mg` files (not duplicated rule strings alone).  
- Persist/load round-trip of SessionStore.  
- Explicit concurrency smoke on map + throttler.

## 4. Coverage gaps

| Area | Gap |
|------|-----|
| Full CDP event matrix | net_request/net_response/console/header paths lightly or unasserted in unit tests |
| ReifyReact happy path | Progressive route reaches the mode on a non-React page; a real React Fiber fixture remains absent |
| ForkSession happy path | Error paths covered; full cookie restore may be lifecycle-only |
| HoneypotDetector with live page | AnalyzePage/GetSafeLinks need Chrome + HTML fixtures (partial in lifecycle) |
| CLI cmd_browser | No package tests under cmd for browser subcommands (may live elsewhere) |
| Wiring | System factory binding is unit-tested and modular registry is live-tested; legacy chat injection remains unproved |
| Multi-manager contention | Not tested |
| Fact schema type rejections | Only react_state int coercion documented/tested indirectly |

## 5. Alignment with principles

| Principle | Test support |
|-----------|--------------|
| Budgeted DOM | Integration expects some facts, not node counts |
| Nil engine skips stream | Covered |
| Honeypot via Mangle | `honeypot_test.go` + policy tests |
| Chrome optional for unit | Default `go test` should not require Chrome (lifecycle Skips) |

## 6. Recommended test additions (backlog)

1. httptest page with `display:none` link → AnalyzePage flags honeypot when Chrome available.  
2. SnapshotDOM predicate inventory assertion vs Decl list.  
3. Event throttle under synthetic burst sink.  
4. Attach after CreateSession TargetID round-trip.  
5. Cortex query assertion after live modular-tool fact ingestion.

## 7. CI posture

- Default unit: always on.  
- Integration tag: opt-in job with Chrome installed.  
- Avoid marking Chrome-required tests without Skip or build tag.
