# BrowserNERD parity contract

> Source baseline: `C:\CodeProjects\SybioGenv3\crossthread\dev_tools\BrowserNERD` at CrossThread commit `0610dacd022dfc3736f6c4f756a6625ce0bfd21b` (2026-08-03)
> codeNERD baseline: `9170ad07` (2026-08-09)
> Status: active implementation contract

## Meaning of parity

Parity means the BrowserNERD capability is reachable through codeNERD's production runtime, bounded and tested, with equivalent observable behavior. It does not mean embedding BrowserNERD's standalone MCP server or its second Mangle engine.

Binding adaptation rules:

- Browser facts enter codeNERD's live `RealKernel`; no parallel reasoning reality.
- Agent-facing behavior is exposed as native modular tools and JIT prompt atoms. Names follow codeNERD's underscore convention.
- Mutating browser actions remain subject to codeNERD constitutional permission and default deny.
- Workspace settings live under `.nerd/`; no independent `.browsernerd` authority root.
- Source-derived implementation retains BrowserNERD attribution and Apache-2.0 notice requirements.
- A row is `done` only after unit coverage and live Chrome proof reach the production-facing route.

## Capability ledger

| ID | BrowserNERD capability | codeNERD baseline evidence | Baseline | Acceptance gate |
|---|---|---|---|---|
| BP-01 | Launch, attach, reconnect, shutdown | Manager-owned browser contexts; stale reconnect; stream-first shutdown; CLI recovery | partial | Managed and attached browsers survive request contexts and shut down without orphaned streams |
| BP-02 | Multiple browser instances | `LaunchAdditional`, `ListBrowsers`, selection, promotion, and `CloseBrowser` exist with bounds | partial | List, launch, select, and close bounded browser instances |
| BP-03 | Shared tabs plus explicit isolation | `CreateTab` shares by default; explicit incognito; forks always isolate; live storage differential | partial | Shared tabs are the default; callers may request isolated contexts |
| BP-04 | Session list/create/attach/focus/fork/close | Package and progressive agent lifecycle are complete; CLI exposure remains incomplete | partial | Every lifecycle operation is available from package, progressive tool, and CLI where operator-relevant |
| BP-05 | Tab/browser limits and idle reaping | Native config, allocation guards, idle reaper, shutdown cancel, focused tests | partial | Configured limits fail closed; idle tabs are reaped; reaper cancels on shutdown |
| BP-06 | Per-session ingestion lifetime | Background stream context stored per session; close/browser/shutdown cancel it; live request-cancel proof | done | Each session owns a cancelable background stream closed by session/browser shutdown |
| BP-07 | Persistent, privacy-safe metadata | Redacted URLs and owner-only session/control/snapshot files; security tests | partial | Credentials and sensitive query values are redacted; files are owner-only |
| BP-08 | Compact page state and navigation map | `browser_observe` summary/compact/full state and nav slices; live registry proof | done | Bounded state and navigation observations return summaries and stable evidence handles |
| BP-09 | Stable interactive element refs | Per-session generation registry, opaque refs, fingerprint re-identification, navigation invalidation | done | Observe returns session-scoped refs with generation-aware re-identification |
| BP-10 | Interactive, grid, and hidden-content discovery | Bounded native views plus live composite fixture | done | Token-bounded discovery modes match BrowserNERD's observable slices |
| BP-11 | Screenshot, DOM, and React observations | Progressive modes; private confined screenshot evidence; bounded DOM/React summaries; live route proof | done | They are reachable through progressive observe with bounded outputs and safe file policy |
| BP-12 | Navigate/interact/fill/key/history | Ref resolution, batch fill, closed keyboard vocabulary, back/forward/reload; live route proof | done | Ref-based interaction, form batches, keyboard, and history are production-routed |
| BP-13 | Sequential action plans | `browser_act`, 25-op ceiling, stop-on-error default, bounded result views | done | `browser_act` executes typed operations with stop-on-error and bounded result views |
| BP-14 | Stable/fact/condition waits | No browser wait surface | missing | Context-cancelable waits consume fresh live-kernel facts and enforce time/result bounds |
| BP-15 | Progressive observe/act/reason/audit | Native observe/act tools and JIT atoms ship; reason/audit remain BPAR-3/BPAR-5 work | partial | Native `browser_observe`, `browser_act`, `browser_reason`, and `browser_audit` tools ship JIT-first |
| BP-16 | Bounded Mangle read/query/rule/temporal/watch | Cortex manager now asserts into live `SystemKernel`; bounded `browser_mangle` surface remains absent | partial | `browser_mangle` delegates to the live kernel with explicit query/rule/result/time ceilings |
| BP-17 | Default credential redaction | Central pre-sink redactor plus live progressive fill proof: secret absent from result and sink | done | URLs, headers, input events, results, logs, sessions, and evidence redact by default |
| BP-18 | Confined model-directed writes | Symlink-aware writable-root policy; private CLI snapshots/session/control files | partial | Screenshot/evidence/spec outputs resolve under allowed roots and reject traversal/symlink escapes |
| BP-19 | Unsafe JavaScript gate | No arbitrary JS tool | missing | Disabled by default; enabling config alone is insufficient without constitutional approval |
| BP-20 | Bounded runtime evidence and flight recorder | General logging exists; no session step evidence or browser trace export | partial | Bounded route/toast/console/request evidence and redacted owner-only JSONL traces |
| BP-21 | Configurable spec delivery and conformance | Architecture corpus exists; no browser spec tools | missing | `browser_specs` discovers bounded workspace docs and runs declared invariants |
| BP-22 | Declarative browser tests and generation | Go lifecycle tests only | partial | Create/inspect/run declarative tests plus bounded generation and live execution proof |
| BP-23 | Console, toast, request, and stability diagnosis | Console/network facts are emitted; no progressive diagnosis | partial | `browser_reason` returns bounded correlated findings from fresh facts |
| BP-24 | Contract audit and repository trace | No browser contract/repo tracing | missing | Bounded non-mutating discovery, explicit mutating approval, evidence, and source-path findings |
| BP-25 | Docker log correlation | No browser-facing correlation | missing | Optional configured containers correlate runtime errors without becoming a hard dependency |
| BP-26 | Delivery evaluation | One live lifecycle test; no competitor/eval harness | partial | Unit, race, live Chrome, declarative, security, and token-budget gates are reproducible |

## Delivery order

1. **BPAR-1 — runtime truth and lifecycle.** Live-kernel sink, one shared manager route, multi-browser/tab lifecycle, close/focus, limits, cancellation, redaction, path policy.
2. **BPAR-2 — progressive observation and action.** Stable refs, bounded state/nav/interactive/grid/hidden views, fill/key/history, `browser_observe` and `browser_act`, JIT atoms.
3. **BPAR-3 — reasoning and waits.** Live-kernel `browser_mangle`, bounded waits, `browser_reason`, diagnosis, temporal evidence.
4. **BPAR-4 — evidence, specs, and declarative tests.** Flight recorder, spec delivery/conformance, browser test create/inspect/run/generate.
5. **BPAR-5 — audit and final proof.** Contract audit, repo trace, optional Docker correlation, security assault, live end-to-end parity matrix.

## Final parity gate

Feature parity may be claimed only when every `BP-*` row is `done`, the production modular-tool route has live Chrome evidence, the browser fact stream is queryable from the same kernel that authorizes actions, and no unredacted credential fixture survives the security suite.
