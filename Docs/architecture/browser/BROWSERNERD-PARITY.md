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
| BP-14 | Stable/fact/condition waits | `browser_wait` is context-cancelable, fresh-only by default, action-watermark aware, and hard-bounded; live Cortex proof covers conditions and stability | done | Context-cancelable waits consume fresh live-kernel facts and enforce time/result bounds |
| BP-15 | Progressive observe/act/reason/audit | Observe/act/reason are native and JIT-first; audit remains BPAR-5 work | partial | Native `browser_observe`, `browser_act`, `browser_reason`, and `browser_audit` tools ship JIT-first |
| BP-16 | Bounded Mangle read/query/rule/temporal/watch | `browser_mangle` provides bounded live-kernel read/query/temporal/evaluate/wait operations; fact mutation and rule submission are intentionally absent pending a safe sandbox | partial | `browser_mangle` delegates to the live kernel with explicit query/rule/result/time ceilings |
| BP-17 | Default credential redaction | Central pre-sink redactor plus live progressive fill proof: secret absent from result and sink | done | URLs, headers, input events, results, logs, sessions, and evidence redact by default |
| BP-18 | Confined model-directed writes | Symlink-aware writable-root policy; current-user-only CLI, screenshot, and evidence artifacts on Windows and Unix | partial | Screenshot/evidence/spec outputs resolve under allowed roots and reject traversal/symlink escapes |
| BP-19 | Unsafe JavaScript gate | No arbitrary JS tool | missing | Disabled by default; enabling config alone is insufficient without constitutional approval |
| BP-20 | Bounded runtime evidence and flight recorder | Default-on redacted JSONL recorder with per-session reads, rotation/pruning, confined export, hard scan/result ceilings, and live Chrome/ACL proof | done | Bounded route/toast/console/request evidence and redacted owner-only JSONL traces |
| BP-21 | Configurable spec delivery and conformance | Native `browser_specs` lists/ranks named workspace corpora, parses native and BrowserNERD-compatible invariants, and checks bounded single atoms against the live session kernel; live Chrome proof passes | done | `browser_specs` discovers bounded workspace docs and runs declared invariants |
| BP-22 | Declarative browser tests and generation | Native `browser_test` create/inspect/generate/run; selector-free semantic targets; execution-only `value_env`; per-assertion fresh baselines; live Cortex replay and causal diagnosis proof | done | Create/inspect/run declarative tests plus bounded generation and live execution proof |
| BP-23 | Console, toast, request, and stability diagnosis | `browser_reason` returns bounded live-kernel failures, correlations, contradictions, blockers, changes, and recommendations; live session-isolation proof passes | done | `browser_reason` returns bounded correlated findings from fresh facts |
| BP-24 | Contract audit and repository trace | Bounded confined repository tracing and the passive discover phase with six-way finding classification ship with unit coverage; execute, report and resume phases, Mangle representation, tool surfacing and live proof remain. | partial | Bounded non-mutating discovery, explicit mutating approval, evidence, and source-path findings |
| BP-25 | Docker log correlation | Correlation core, Docker fetcher, SessionManager wiring, and `browser_reason` surfacing ship with unit coverage; live container proof remains. | partial | Optional configured containers correlate runtime errors without becoming a hard dependency |
| BP-26 | Delivery evaluation | Unit/race/live Chrome gates cover lifecycle, progressive actions, fresh waits, same-kernel queries, session isolation, diagnosis, evidence, spec conformance, and declarative generation/replay; competitor evaluation remains | partial | Unit, race, live Chrome, declarative, security, and token-budget gates are reproducible |

### BP-24 status detail

- `internal/browser/security/path_policy.go` — `ConfineToRoot(root, candidate)`, the read-side counterpart to `ResolveForWrite`. Resolves symlinks on BOTH sides before comparing via `pathWithin`; rejection error is `path escapes root` and deliberately omits the resolved outside path so a diagnosis cannot leak filesystem layout. Seven tests including a real symlink planted inside the root pointing out of it.
- `internal/browser/repo_trace.go` — `TraceRepository`, non-mutating and confined by `ConfineToRoot` on every walked entry. A zero limit means `DEFAULT` never unlimited, and a caller cannot raise a ceiling — over-ceiling values clamp down with a `Note`. Needles are literal substrings not regular expressions because they come from page facts where a stray metacharacter would error or match wildly. Binary files are detected by a `NUL` byte in the first 512 bytes rather than by extension. Paths are returned relative to the root, snippets are bounded, trimmed at rune boundaries and redacted at construction. Per-file problems are `Notes` not errors. Fourteen tests.
- `internal/browser/contract_audit.go` — `DiscoverContract`, the passive discover phase, plus `AuditFindingKind` classification (`observation`, `inference`, `skipped`, `approval_required`, `execution_failure`, `contract_mismatch`). Discovery may only emit `observation`, `inference`, `skipped` and `approval_required`; a test asserts it can never emit `contract_mismatch` or `execution_failure`. A textual match is recorded as `INFERENCE` not `observation`. No needles yields an explicit `skipped` finding rather than an empty result, because silence would read as "searched and found nothing". Query strings are discarded during needle derivation so a token in a query can never become a search term. Mutating controls produce `approval_required` findings and are never pressed — a test asserts the scanned tree is unmodified. Twelve tests.
- REMAINING for `done`: the execute phase (replay under navigation, mutation, risk and destructive-action gates), the report and resume phases, representation of audit state and discovered hazards in Mangle, surfacing through a `browser_audit` tool, and live proof against a real page and repository.


### BP-25 status detail

- `internal/browser/docker_correlation.go` - `CorrelateContainerLogs` pairs container log lines with browser runtime errors inside a time window. Bounded on every axis: 8 containers, 500 lines each, 50 correlations, each cap emitting a Note rather than truncating silently. `DeltaMs` is signed, so a container line that preceded the browser error is distinguishable from one that followed it. Output is sorted closest-in-time first and is deterministic.
- `internal/browser/docker_fetcher.go` - `LookupDockerBinary` checks `execution.allowed_binaries` BEFORE `exec.LookPath`: PATH presence is capability, the allowlist is authorization, and authorization is checked first. `NewDockerLogFetcher` returns nil when Docker is unauthorized or absent, and the core turns a nil fetcher into a Note rather than an error. That composition is what makes the gate's "without becoming a hard dependency" clause a tested property rather than a claim. No shell is invoked; the container name is passed as its own argv element and rejected when empty or "-" prefixed. Reads are capped at 4 MiB across stdout and stderr combined.
- `internal/browser/session_manager.go` - `CorrelateContainerErrors` is off unless `CorrelationContainers` is set, never returns an error (a correlation failure must not fail the diagnosis that asked for it), and redacts with the manager's own redactor so correlation follows the same policy as `sanitizeFacts`. Both constructors delegate to a shared `newSessionManager`, so the field cannot be wired in one and missed in the other.
- Redaction happens at correlation construction, not at a later print site, so no unredacted value can escape by a route that forgets. Container logs are exactly where a bearer token or connection string appears.
- `internal/tools/research/browser_reasoning.go` - the diagnosis adapts its failed-request and visible-error facts into `RuntimeErrorEvent`s and publishes `container_correlations` alongside the existing browser-to-browser `correlations`, which answer different questions and are both kept. The keys, the counts entry and the evidence handle are emitted only when there is something to report, so a diagnosis is unchanged for an operator who has not enabled correlation. Adaptation reuses `factTimestamp` with the same predicate specs `correlateBrowserFailures` uses, so the two correlations cannot disagree about when a fact happened; facts with an undeterminable timestamp are skipped rather than correlating against the epoch; adaptation is capped at 32 most-recent events so a failure storm cannot become an unbounded correlation pass.
- REMAINING for `done`: live proof against a real running container.

`browser.correlation_containers` in `.nerd/config.json` selects the containers and enabling correlation takes two deliberate operator acts — listing containers AND authorizing `docker` in `execution.allowed_binaries`.

## Delivery order

1. **BPAR-1 — runtime truth and lifecycle.** Live-kernel sink, one shared manager route, multi-browser/tab lifecycle, close/focus, limits, cancellation, redaction, path policy.
2. **BPAR-2 — progressive observation and action.** Stable refs, bounded state/nav/interactive/grid/hidden views, fill/key/history, `browser_observe` and `browser_act`, JIT atoms.
3. **BPAR-3 — reasoning and waits (complete).** Live-kernel read-only `browser_mangle`, bounded fresh waits, `browser_reason`, session-scoped diagnosis, temporal evidence.
4. **BPAR-4 — evidence, specs, and declarative tests (complete).** Flight recorder, spec delivery/conformance, and native browser test create/inspect/generate/run all have bounded unit and live production-route proof.
5. **BPAR-5 — audit and final proof.** Contract audit, repo trace, optional Docker correlation, security assault, live end-to-end parity matrix.

## Final parity gate

Feature parity may be claimed only when every `BP-*` row is `done`, the production modular-tool route has live Chrome evidence, the browser fact stream is queryable from the same kernel that authorizes actions, and no unredacted credential fixture survives the security suite.
