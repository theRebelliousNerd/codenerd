# TODO — browser architecture backlog

> Last verified: 2026-08-09 · BrowserNERD parity is tracked in [BROWSERNERD-PARITY.md](BROWSERNERD-PARITY.md)

## P0

- [ ] Complete BPAR-1 runtime truth, lifecycle, cancellation, and privacy gates
- [x] Single Cortex-owned `SessionManager` with live kernel `EngineSink`
- [ ] On-demand construct + inject into `TactileRouterShard` / chat model on first browser action
- [x] Point research tools at shared manager
- [ ] Document operator risk: CLI engines do not feed chat kernel (already true — keep explicit in UX)

## P1

- [ ] Complete BPAR-2 progressive observe/act surface with JIT atoms
- [ ] Complete BPAR-3 live-kernel reasoning and waits
- [ ] Implement Go `honeypot_suspicious_url` assertion or drop from reason table/policy
- [ ] Align Go reason checklist with `browser_honeypot.mg` (clip/overflow/no_keyboard coverage)
- [ ] Optional gate: Click/Type refuse when `is_honeypot` for resolved element (caller-level or manager-level flag)
- [ ] Cobra: `list`, `screenshot`, `click`, `type`, `fork`, `honeypot`

## P2

- [ ] Complete BPAR-4 evidence/spec/declarative-test surface
- [ ] TUI browser status / session list slash command
- [ ] VS `handleBrowse` thin delegate to shared manager (if design accepts)
- [x] Session close API that stops event-stream ctx per session
- [ ] Contract tests: SnapshotDOM predicates ⊆ Decl in schemas_browser.mg

## P3

- [ ] Complete BPAR-5 contract audit, repo trace, Docker correlation, and final live parity gate
- [x] Redact secrets from browser logs, fact sink, results, and session store
- [ ] Fact GC / epoch for long event streams
- [ ] Header ingestion default policy for research vs operator modes
- [ ] CI job: integration tag with headless Chrome

## Done (baseline — do not re-open without evidence)

- [x] SessionManager + Rod lifecycle  
- [x] DOM/React/event reification  
- [x] HoneypotDetector + policy files  
- [x] CLI launch/session/snapshot  
- [x] Dense unit/coverage tests + optional lifecycle/integration  
