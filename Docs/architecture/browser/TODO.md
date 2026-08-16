# TODO — browser architecture backlog

> Last verified: 2026-08-16 · BrowserNERD parity is tracked in [BROWSERNERD-PARITY.md](BROWSERNERD-PARITY.md)

## P0

- [x] Complete BPAR-1 runtime truth, lifecycle, cancellation, and privacy gates
- [x] Single Cortex-owned `SessionManager` with live kernel `EngineSink`
- [x] On-demand construct + inject into `TactileRouterShard` / chat model on first browser action — construction is inert and Chrome starts lazily via `ensureStarted`; pinned by `TestSessionManager_WhenConstructed_ShouldNotStartBrowser` and its live counterpart
- [x] Point research tools at shared manager
- [x] Document operator risk: CLI engines do not feed chat kernel — stated in `nerd browser --help` and printed after every fact-producing subcommand

## P1

- [x] Complete BPAR-2 progressive observe/act surface with JIT atoms
- [x] Complete BPAR-3 live-kernel reasoning and waits
- [x] Implement Go `honeypot_suspicious_url` assertion — Go classifies href shape into `link_url_pattern`; `browser_honeypot.mg` decides which shapes are traps
- [x] Align Go reason checklist with `browser_honeypot.mg` — reasons now come from `honeypot_reason/2`; clip/overflow rules added, `no_keyboard` is evidence only; contract test enforces the two agree
- [x] Optional gate: Click/Type/InteractRef refuse when `is_honeypot` (`Config.HoneypotGuard` off|warn|block, default block; fails open with no query path)
- [x] Cobra: `list`, `screenshot`, `click`, `type`, `fork`, `honeypot`

## P2

- [x] Complete BPAR-4 evidence/spec/declarative-test surface
- [x] Ship bounded redacted flight evidence with current-user-only JSONL export
- [x] Ship bounded workspace browser spec delivery and conformance
- [x] Ship browser test create/inspect/run/generate with live execution proof
- [x] TUI browser status / session list slash command
  - Closed 2026-08-16. internal/browser gained ListSessions() (a newest-first snapshot of session metadata, tie-broken by ID, returning value copies so a render path cannot mutate live session state) and DefaultSessionID(). cmd/nerd/chat gained a /browser command (commands.go dispatch plus handleCmdBrowser in commands_handlers_misc.go) that reports the session count and, per session, its ID, status, URL, title, relative last-active age, the isolated marker, and which session is current. It only reports and never starts a browser. browserMgr stays nil until browser automation is first used, so that is treated as the ordinary case rather than an error, and it is the path the test pins.
- Decided NO - VS `handleBrowse` thin delegate to shared manager (if design accepts) — internal/core/virtual_store_actions.go:657 handleBrowse deliberately refuses and returns "browser operations must be executed via TactileRouterShard", asserting browser_routing(Operation, /requires_shard). The comment at :672-674 states why: TactileRouterShard is the component with the SessionManager wired, and routing through it is what preserves session management, sandboxing and audit trails. VirtualStore holds no SessionManager. Making handleBrowse a thin delegate would require plumbing one into VirtualStore and would bypass the very boundary the current refusal exists to enforce. The refusal is the design, not a gap in it.
- [x] Session close API that stops event-stream ctx per session
- [x] Contract tests: SnapshotDOM predicates ⊆ Decl in schemas_browser.mg (plus bound-type checking; caught `position/5` coordinates asserted as strings)

## P3

- [ ] Complete BPAR-5 contract audit, repo trace, Docker correlation, and final live parity gate
  - this is a live-environment verification task, not code to write. It requires a running Docker environment and live Chromium to correlate container behaviour against the contract and run the parity gate, so it cannot be closed from a static pass. CI now exists (.github/workflows/ci.yml — build, vet and full test suite on windows-latest, plus a -race job), so the CI half of this is no longer blocked by the absence of a pipeline.
- [x] Redact secrets from browser logs, fact sink, results, and session store
- [x] Fact GC / epoch for long event streams — per-epoch budget, `browser_epoch` watermark, navigation rollover, and real retraction when a `FactRetractor` is wired
- [x] Header ingestion default policy for research vs operator modes — `HeaderIngestionMode` redacted for research, off for the CLI
- [~] CI job: integration tag with headless Chrome — live tests now discover Chromium under `PLAYWRIGHT_BROWSERS_PATH`/`NERD_TEST_CHROME_BIN` and run by default; .github/workflows/ci.yml now exists (build, vet, full default test suite and a -race job on windows-latest) but the integration-tagged job is not yet wired into it
  - .github/workflows/ci.yml exists and covers build, vet, the full default test suite and a -race job.
  - An integration-tagged job is not in it yet because the integration build was broken until 2026-08-16 and has only just been repaired: tests/e2e carried a mockPoisonKernel whose GetProgramInfo returned interface{} instead of *analysis.ProgramInfo, and session_spawner_config_integration_test.go set a Persona field that session.TaskRequest does not have (persona is carried by the intent verb). Both are fixed.
  - What remains before the job can be added: confirming the integration suite actually passes end to end on a runner, and provisioning Chromium there. The suite is slow to compile (it pulls the browser dependency graph), so the job needs its own timeout and probably its own cache.
  - Live tests discover Chromium under `PLAYWRIGHT_BROWSERS_PATH` / `NERD_TEST_CHROME_BIN`, which is what a runner would need to set.

## Done (baseline — do not re-open without evidence)

- [x] SessionManager + Rod lifecycle  
- [x] DOM/React/event reification  
- [x] HoneypotDetector + policy files  
- [x] CLI launch/session/snapshot  
- [x] Dense unit/coverage tests + optional lifecycle/integration  
