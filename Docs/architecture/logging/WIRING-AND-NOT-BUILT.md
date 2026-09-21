# logging wiring — and what is NOT built

> Verified 2026-09-21 against commit `3463477`. Callers below are files that
> contain the call today, with one line cited each; the grep over `*.go` was
> capped at 50 hits, so the list is representative, not exhaustive.

## Wired and reachable

- `cmd/nerd/chat/*.go` is the dominant caller, through both spellings: bare
  wrappers (`logging.Session` at `agent_wizard.go:147`, `logging.Kernel` at
  `campaign.go:209`, `logging.Shards` at `delegation_modes.go:353`,
  `logging.Boot` at `ingest.go:73`, `logging.API` at `process.go:58`) and
  direct `Get` (`logging.Get(logging.CategoryAutopoiesis)` at
  `delegation.go:595`, `logging.Get(logging.CategoryPerformance)` at
  `model_update.go:30`). The category vocabulary it consumes is enumerated in
  `cmd/nerd/cmd_audit.go:20-46`.
- Tests drain the sinks: `logging.CloseAll()` (`logger.go:805`) is called
  repeatedly from `cmd/nerd/chat/live_integration_test.go` (e.g. lines 1246,
  1286, 1359).
- Inside the package: the audit file opens only inside `InitAudit`
  (`audit.go:159`), reached lazily through any `Audit*()` constructor
  (`agents.md:5-6`); the run prefix is minted during
  `initializeInternal` (`logger.go:320`); all three durable writers share
  `rotatingFile` (`audit.go:125`, `logger.go:122`, `llm_io_logger.go:35`).
- The 12 test files cover fresh-run behavior (3 files), rotation (2), audit
  write/read/roundtrip/perf (5), redaction (1), LLM trace (1), call-site
  attribution (1), and the convenience wrappers (1).

## Exists but nothing calls

- `ExportAuditFacts` (`audit_facts.go:37`) has no production caller: the
  audit→Mangle replay path is implemented, tested
  (`audit_facts_test.go`, `audit_roundtrip_test.go`), and unreachable outside
  tests. Same for the read API — `ReadRecentAuditEvents`
  (`audit_reader.go:69`), `CountAuditEventTypes` (`audit_reader.go:140`), and
  `LatestAuditLogPath` (`audit_reader.go:45`) are exercised by
  `audit_reader_test.go` but no `*.go` caller outside the package was found.
- `ClearInjectedConfig` (`logger.go:299`) and `resetLLMIOLogger`
  (`llm_io_logger.go:96`) are test seams with no production callers.
- `ContextLogger` (`WithContext`, `logger.go:718`) and `RequestLogger`
  (`WithRequestID`, `logger.go:838`) are exported scoped variants with no
  callers in the sampled `cmd/nerd/chat` traffic, which uses bare wrappers
  and direct `Get` instead.

## Assumed by the design, not done by the code

- **No network or span export.** Every durable sink is a file; the
  keep-file-only decision is recorded in
  `Docs/architecture/transparency/TODO.md:31`. The single non-file egress is
  `ExportAuditFacts` writing to a caller-supplied `io.Writer`
  (`audit_facts.go:37`) — a hook nobody has implemented.
- **Sampling drops data silently.** Performance events pass through
  `performanceSamplingRate` (`logger.go:933`); unsampled slow operations leave
  no trace, and no symbol in the package counts or reports dropped events.
- **Fresh-run cleanup destroys.** `clearOrdinaryLogs` (`fresh_run.go:277`)
  deletes other runs' top-level `*.log` files; only the symlink refusal
  (`fresh_run.go:116`) and the substantive-audit-log guard (`fresh_run.go:203`)
  hold it back. A crashed run's ordinary logs are gone on next boot by design.
- **The core write path has no dedicated unit test.** There is no
  `logger_test.go`: `Initialize`/`Get`/emit are covered only indirectly
  (integration drains via `CloseAll`, attribution via
  `logger_safety_callsite_test.go`).
- **Two spellings for every write.** The ~190 convenience wrappers duplicate
  `Get` exactly (3-line bodies, e.g. `Boot` at `logger_convenience.go:9-11`);
  nothing states which spelling new code should prefer.

## Known stale pointer (not fixable here — no Go changes in this rewrite)

- `cmd/nerd/cmd_audit.go:168` names `IMPLEMENTED_SPEC` and
  `09-SAFETY-AND-INVARIANTS.md`, which this rewrite deletes. Its owner should
  repoint it at `Docs/architecture/logging/` (`README.md`,
  `WIRING-AND-NOT-BUILT.md`).
