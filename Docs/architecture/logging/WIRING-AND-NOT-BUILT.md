# logging wiring — and what is NOT built

> Re-verified 2026-09-25 against `77a5027` (lane B build-out): the "Exists
> but nothing calls" section was wrong about the audit read/export API and is
> corrected; sampling and the stale pointer are fixed. First verified
> 2026-09-21 against commit `3463477`. Callers below are files that
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

## Exists but nothing calls — corrected 2026-09-25

- The audit export and read API **are** called; the 2026-09-21 claim was
  wrong. `ExportAuditFacts` backs `nerd audit facts`
  (`cmd/nerd/cmd_audit.go:74`); `LatestAuditLogPath` is called at
  `cmd/nerd/cmd_audit.go:45` and `cmd/nerd/cmd_transparency.go:83,204`;
  `CountAuditEventTypes` at `cmd/nerd/cmd_transparency.go:170,221`;
  `ReadRecentAuditEvents` at `cmd/nerd/cmd_transparency.go:265`.
- `ClearInjectedConfig` (`logger.go:299`) and `resetLLMIOLogger`
  (`llm_io_logger.go:96`) are test seams by design.
- `ContextLogger` (`WithContext`) and `RequestLogger` (`WithRequestID`) are
  exported scoped variants with no production callers. Decided, not a wiring
  gap: adopting a request ID for correlation is the maintainer's call
  (`Docs/architecture/logging/TODO.md`, "Decided, not built").

## Assumed by the design, not done by the code

- **No network or span export.** Every durable sink is a file; the
  keep-file-only decision is recorded in
  `Docs/architecture/transparency/TODO.md:31`. The single non-file egress is
  `ExportAuditFacts` writing to a caller-supplied `io.Writer`
  (`audit_facts.go:37`) — a hook nobody has implemented.
- **Sampling drops data — no longer silently (2026-09-25, `77a5027`).** Only
  non-slow timings are ever sampled; a slow one always logs. The dropped ones
  are now counted (`logger.go:1023`) and reported in one `performance.sampling`
  line as the sinks close (`reportPerformanceSampling`, `logger.go:953`);
  `PerformanceSamplingStats` exposes the counts.
- **Fresh-run cleanup destroys.** `clearOrdinaryLogs` (`fresh_run.go:277`)
  deletes other runs' top-level `*.log` files; only the symlink refusal
  (`fresh_run.go:116`) and the substantive-audit-log guard (`fresh_run.go:203`)
  hold it back. A crashed run's ordinary logs are gone on next boot by design.
- **The core write path's tests** — the 2026-09-21 claim that there is no
  `logger_test.go` was wrong: `internal/logging/logger_test.go` exists
  (`TestAllCategoriesLog` and siblings drive `Initialize`, `Get` and the
  category sinks directly).
- **Two spellings for every write.** The ~190 convenience wrappers duplicate
  `Get` exactly (3-line bodies, e.g. `Boot` at `logger_convenience.go:9-11`);
  nothing states which spelling new code should prefer.

## Stale pointer — fixed 2026-09-25 (`77a5027`)

- `cmd/nerd/cmd_audit.go:168` named `IMPLEMENTED_SPEC` and
  `09-SAFETY-AND-INVARIANTS.md`, which the corpus rewrite deleted. It now
  names `README`, `INTERNALS` and `WIRING-AND-NOT-BUILT`, and
  `TestAuditPlaybook_TheCorpusPagesItNamesExist` (`cmd/nerd/cmd_audit_test.go`)
  fails if a page it names disappears.
