# TODO — browser architecture backlog

> Last verified: 2026-07-13 · Prioritized for implementers (docs-only corpus; no code changes here)

## P0

- [ ] Single Cortex-owned `SessionManager` with live kernel `EngineSink`
- [ ] On-demand construct + inject into `TactileRouterShard` / chat model on first browser action
- [ ] Point research tools at shared manager (or inject sink into singleton)
- [ ] Document operator risk: CLI engines do not feed chat kernel (already true — keep explicit in UX)

## P1

- [ ] Implement Go `honeypot_suspicious_url` assertion or drop from reason table/policy
- [ ] Align Go reason checklist with `browser_honeypot.mg` (clip/overflow/no_keyboard coverage)
- [ ] Optional gate: Click/Type refuse when `is_honeypot` for resolved element (caller-level or manager-level flag)
- [ ] Cobra: `list`, `screenshot`, `click`, `type`, `fork`, `honeypot`

## P2

- [ ] TUI browser status / session list slash command
- [ ] VS `handleBrowse` thin delegate to shared manager (if design accepts)
- [ ] Session close API that stops event-stream ctx per session
- [ ] Contract tests: SnapshotDOM predicates ⊆ Decl in schemas_browser.mg

## P3

- [ ] Redact secrets from URL logging / session store
- [ ] Fact GC / epoch for long event streams
- [ ] Header ingestion default policy for research vs operator modes
- [ ] CI job: integration tag with headless Chrome

## Done (baseline — do not re-open without evidence)

- [x] SessionManager + Rod lifecycle  
- [x] DOM/React/event reification  
- [x] HoneypotDetector + policy files  
- [x] CLI launch/session/snapshot  
- [x] Dense unit/coverage tests + optional lifecycle/integration  
