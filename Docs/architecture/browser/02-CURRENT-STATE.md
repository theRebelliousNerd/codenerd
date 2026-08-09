# 02 — Current State: `internal/browser`

> Precise inventory refreshed 2026-08-09. Paths relative to repo root.

## 1. Package identity

| Property | Value |
|----------|-------|
| Import path | `codenerd/internal/browser` |
| Package comment | Browser automation with DOM/React reification into Mangle facts; adapted from BrowserNERD for Cortex 1.5.0 Browser Physics (§9.0) |
| Non-test Go files | **19 checked in** (18 per platform; Windows/Unix permission implementations are build-tagged) |
| Test Go files | **17** (12 root package + 2 `security` + 2 `specs` + 1 `testspec`) |
| Package-local `.mg` | **0** (schemas/policy live under `internal/core/defaults/`) |
| Primary third-party | `github.com/go-rod/rod` (+ launcher, proto) |
| UUID | `github.com/google/uuid` for session IDs |

## 2. File inventory (source)

| Path | ≈Lines | Role |
|------|-------:|------|
| `internal/browser/session_manager.go` | ~817 | Config/types, core operations, Navigate/Click/Type/Screenshot, ReifyReact |
| `internal/browser/element_registry.go` | ~171 | Generation-bound opaque refs and private element fingerprints |
| `internal/browser/progressive_observe.go` | ~689 | Bounded state/nav/interactive/grid/hidden/screenshot/DOM/React observation |
| `internal/browser/progressive_action.go` | ~766 | Ref/semantic-target resolution plus closed sequential action/lifecycle plans and portable action-intent evidence |
| `internal/browser/declarative_matcher.go` | ~146 | Selector-free semantic fixture matchers and unique live-page resolution |
| `internal/browser/session_lifecycle.go` | ~618 | Browser/tab lifecycle, shared/isolation semantics, limits, reaper, cancellation |
| `internal/browser/session_manager_dom.go` | ~825 | Event stream, redacted DOM capture, storage snapshot/restore, private session persistence |
| `internal/browser/fact_redaction.go` | ~66 | Copy-on-write pre-sink browser fact redaction |
| `internal/browser/flight_recorder.go` | ~461 | Bounded redacted per-session JSONL evidence, rotation, read, and export |
| `internal/browser/honeypot.go` | ~412 | HoneypotDetector: emit page facts, EvaluateRule, safe links |
| `internal/browser/security/redactor.go` | ~250 | URL/header/input/recursive evidence redaction |
| `internal/browser/security/path_policy.go` | ~187 | Writable-root and symlink confinement; private files |
| `internal/browser/security/private_permissions_*.go` | ~121 | Unix modes or protected current-user-only Windows DACLs |
| `internal/browser/specs/types.go` | ~180 | Bounded native catalog config, spec/binding/invariant and match contracts |
| `internal/browser/specs/parser.go` | ~329 | Capped Markdown/YAML parser with native and BrowserNERD-compatible invariants |
| `internal/browser/specs/catalog.go` | ~588 | Workspace-confined discovery, index hints, ranking, excerpts, and invariant selection |
| `internal/browser/testspec/parser.go` | ~349 | Strict bounded YAML/JSON fixtures, portable action validation, and execution-only environment resolution |
| `internal/browser/testspec/types.go` | ~32 | Declarative fixture/assertion contracts and hard ceilings |

**≈7,007 checked-in non-test LOC** across lifecycle, progressive tools, reification, evidence, security, specs/tests, and detector modules.

## 3. File inventory (tests)

| Path | Build tag | Role |
|------|-----------|------|
| `session_manager_coverage_test.go` | (default) | Config, throttler, constructors, session map, stringify helpers, JSON round-trips, concurrent access |
| `start_coverage_test.go` | (default) | Start error paths, session store load, concurrent R/W, throttler timing |
| `honeypot_coverage_test.go` | (default) | Confidence math, reason table, error paths without Chrome, fail sinks |
| `honeypot_test.go` | (default) | Table-driven honeypot rules against loaded `schemas_browser.mg` + `browser_honeypot.mg` |
| `lifecycle_coverage_test.go` | (default) | Single shared Chrome lifecycle when launcher finds Chrome; else Skip |
| `browser_integration_test.go` | `integration` | httptest + headless Chrome: navigation facts, click/input events |
| `session_lifecycle_test.go` | (default) | Lifecycle defaults, close cancellation, browser promotion, idle reaping |
| `fact_redaction_test.go` | (default) | Pre-sink redaction and manager output policy |
| `element_registry_test.go` | (default) | Stable refs, navigation invalidation, copy isolation, stale snapshot rejection |
| `progressive_action_test.go` | (default) | Observe option and action batch bounds/stop-on-error |
| `flight_recorder_test.go` | (default) | Pre-persistence redaction, bounded reads, rotation/pruning, confined private export, session privacy |
| `security/*_test.go` | (default) | Credential fixtures, traversal/symlink escape, private artifacts |
| `specs/*_test.go` | (default) | Native/compatible parsing, aliases/depth/size/count bounds, ranking, indexes, file ranges, traversal and symlink escape |
| `testspec/parser_test.go` | (default) | Strict portable fixture parsing, selector/ref/secret rejection, YAML bounds, and environment-copy isolation |
| `declarative_matcher_test.go` | (default) | Semantic matcher privacy/validation and portable action-intent evidence |

## 4. Companion files (outside package, load-bearing)

| Path | Role |
|------|------|
| `internal/core/defaults/schemas_browser.mg` | Decl for DOM, honeypot, React, network, events, interactable |
| `internal/core/defaults/policy/browser.mg` | Spatial/safety rules plus session-scoped failed request, slow API, root-cause, visible-error, and interaction-blocked derivations |
| `internal/core/defaults/policy/browser_reasoning_test.go` | Real-Mangle typed/session-isolation checks for browser reasoning rules |
| `internal/core/defaults/policy/browser_honeypot.mg` | Intermediate honeypot predicates + `is_honeypot` / `high_confidence_honeypot` |
| `internal/core/defaults/policy/browser_honeypot_test.go` | Policy-only unit tests |
| `internal/core/defaults/policy/constitution.mg` | Exact permission derivation for registered browser tool spellings |
| `internal/core/defaults/policy/delegation.mg` | `tool_capabilities(/browser, …)` |
| `internal/core/defaults/policy/system_routing.mg` | `routing_table(/browser, /browser_tool, /high)` |
| `internal/core/kernel_init.go` | Loads `schemas_browser.mg` into kernel program |
| `cmd/nerd/cmd_browser.go` | Cobra: launch / session / snapshot |
| `cmd/nerd/main.go` | Registers `browserCmd` |
| `internal/system/factory.go` | Constructs the Cortex manager with a live-kernel sink and injects it into tactile and research routes |
| `cmd/nerd/chat/session_boot.go` | Legacy chat boot path still declares `browserMgr` **nil until needed** |
| `cmd/nerd/chat/model_types.go` | `browserMgr` / `BrowserManager` fields |
| `internal/shards/system/router.go` | `SetBrowserManager`, tool routes for browser_* |
| `internal/tools/research/browser.go`, `browser_progressive.go`, `browser_reasoning.go`, `browser_evidence.go`, `browser_specs.go`, `browser_declarative.go` | Legacy, progressive, wait/query/diagnosis, private evidence, workspace spec, and declarative-test tools bound to the Cortex-owned manager and live kernel |
| `internal/prompt/atoms/capability/browser_progressive.yaml` | JIT ref-first observe/act method and safety boundary |
| `internal/prompt/atoms/capability/browser_reasoning.yaml` | JIT action-watermark, fresh-wait, read-only-query, and diagnosis method |
| `internal/prompt/atoms/capability/browser_evidence.yaml` | JIT bounded-read/export and privacy method |
| `internal/prompt/atoms/capability/browser_specs.yaml` | JIT bounded spec discovery, relevance, and live-kernel conformance method |
| `internal/prompt/atoms/capability/browser_tests.yaml` | JIT portable semantic-fixture generation/replay and credential boundary |
| `internal/core/virtual_store_types.go` | ActionBrowser* ActionType constants |
| `internal/core/virtual_store_actions.go` | `handleBrowse` refuse + modular tool arg mapping |
| `internal/logging/logger.go` | `CategoryBrowser` |
| `internal/logging/logger_convenience.go` | `Browser` / `BrowserDebug` / `BrowserWarn` / `BrowserError` |

## 5. Runtime artifacts (workspace)

| Path pattern | Written by | Content |
|--------------|------------|---------|
| `.nerd/browser/sessions.json` | SessionManager | Redacted JSON metadata, owner-only |
| `.nerd/browser/control.txt` | `nerd browser launch` | Owner-only DevTools WebSocket control URL |
| `.nerd/browser/snapshots/<session>_<unix>.mg` | `nerd browser snapshot` | Redacted owner-only fact dump under path policy |
| `.nerd/browser/traces/flight_<session>*.jsonl` | FlightRecorder | Redacted, rotated, current-user-only per-session event and tool evidence |
| `.nerd/browser/traces/exports/*.jsonl` | `browser_evidence export` | Explicit bounded selection under writable-root policy |
| `.nerd/browser/specs/*.md` | Workspace author/operator | Read-only native spec input; discovered only through configured confined roots/indexes |

## 6. Hotspots

1. **`startEventStream`** — goroutine fan-out (nav wait, multi-event wait, 500ms poll of in-page buffer).  
2. **`captureDOMFacts`** — dense fact emission (multiple typed predicates per session-qualified node).
3. **`ReifyReact`** — large inline JS fiber walker; type coercion for hook indices.  
4. **`HoneypotDetector.emitPageFacts`** — per-element style/position/attribute PushFact.  
5. **Route completeness** — system and modular tools share the Cortex manager/live kernel; observe/act/mangle/wait/reason/evidence/specs/test are live, while audit parity and operator CLI expansion remain open.

## 7. What is intentionally not here

- No HTTP API server inside the package  
- No Mangle source files inside `internal/browser/`  
- No VirtualStore interface implementation  
- Prompt behavior lives outside the package in the JIT atom corpus
- System boot browser facts enter the live kernel through `browserKernelSink`; standalone CLI managers intentionally use a schema-loaded export engine
