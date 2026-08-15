# TODO — browser architecture backlog

> Last verified: 2026-08-15 · BrowserNERD parity is tracked in [BROWSERNERD-PARITY.md](BROWSERNERD-PARITY.md)

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
- [ ] TUI browser status / session list slash command
- [ ] VS `handleBrowse` thin delegate to shared manager (if design accepts)
- [x] Session close API that stops event-stream ctx per session
- [x] Contract tests: SnapshotDOM predicates ⊆ Decl in schemas_browser.mg (plus bound-type checking; caught `position/5` coordinates asserted as strings)

## P3

- [ ] Complete BPAR-5 contract audit, repo trace, Docker correlation, and final live parity gate
- [x] Redact secrets from browser logs, fact sink, results, and session store
- [x] Fact GC / epoch for long event streams — per-epoch budget, `browser_epoch` watermark, navigation rollover, and real retraction when a `FactRetractor` is wired
- [x] Header ingestion default policy for research vs operator modes — `HeaderIngestionMode` redacted for research, off for the CLI
- [~] CI job: integration tag with headless Chrome — live tests now discover Chromium under `PLAYWRIGHT_BROWSERS_PATH`/`NERD_TEST_CHROME_BIN` and run by default; the workflow file itself is still unwritten

## Done (baseline — do not re-open without evidence)

- [x] SessionManager + Rod lifecycle  
- [x] DOM/React/event reification  
- [x] HoneypotDetector + policy files  
- [x] CLI launch/session/snapshot  
- [x] Dense unit/coverage tests + optional lifecycle/integration  
