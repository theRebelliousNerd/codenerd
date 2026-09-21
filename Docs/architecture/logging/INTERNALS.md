# logging internals

> Verified 2026-09-21 against commit `3463477`. Every symbol below names the
> file it lives in; line numbers are 1-indexed into that commit.

## Boot order

`Initialize(ws)` (`logger.go:174`) delegates to `initializeInternal`
(`logger.go:307`), which mints the run prefix (`runPrefix =
generateRunPrefix()`, `logger.go:320`) and opens the category sinks.
Configuration is injected `Config` first, file second: `ApplyConfig`
(`logger.go:275`) overrides whatever `loadConfig` (`logger.go:370`) read from
disk, `ReloadConfig` (`logger.go:421`) re-reads the file, and
`ClearInjectedConfig` (`logger.go:299`) drops the injection — a test seam, not
a runtime path.

## Category sinks

`Get(category)` (`logger.go:454`) returns the `*Logger` (`logger.go:119`) for a
`Category` (`logger.go:33`). `Debug`/`Info`/`Warn`/`Error` (`logger.go:557-608`)
format and emit; JSON mode goes through `logJSON` (`logger.go:505`), gated by
`IsJSONFormat` (`logger.go:697`). Call-site attribution comes from `callerSite`
(`logger.go:529`), which skips the package's own frames via
`isLoggingInternalFile` (`logger.go:551`). Entries are also mirrored to
`problems.log` by `mirrorToProblems` (`logger.go:630`, closed by
`closeProblemsLog` at `logger.go:660`).

`logger_convenience.go` holds ~190 thin wrappers (typically 3-line bodies, e.g.
`Boot` at `logger_convenience.go:9-11`) spelling `Get(CategoryX).Level(...)` as
`XLevel(...)` — `Session`, `Kernel`, `API`, `Tools`, `Shards`, and the rest.

Slow-operation logging is sampled, not complete: `logPerformance`
(`logger.go:963`) consults `performanceSamplingRate` (`logger.go:933`, drawn
from `secureFloat64` at `logger.go:1054`) and a per-category
`performanceThresholdMs` (`logger.go:948`). The `Timer` type (`logger.go:912`)
is the public handle: `StartTimer` (`logger.go:1016`),
`Stop`/`StopWithInfo`/`StopWithThreshold` (`logger.go:1025-1051`).

Two scoped variants avoid threading context through every call:
`ContextLogger` via `WithContext` (`logger.go:718-726`) and `RequestLogger`
via `WithRequestID` (`logger.go:831-844`) with chained `WithField`
(`logger.go:852`).

## Audit trail

`AuditEvent` (`audit.go:103`) is the row type; `AuditLogger` (`audit.go:131`)
is the writer. `Log` (`audit.go:217`) appends one JSONL line to the audit file
and derives the Mangle fact for the same event via `generateMangleFact`
(`audit.go:278`), with `mangleBool` (`audit.go:262`), `mangleString`
(`audit.go:273`), and `escapeString` (`audit.go:362`) doing the quoting. ~20
typed helpers cover the event vocabulary — `ShardSpawn` (`audit.go:394`),
`ToolExec` (`audit.go:617`), `IntentParsed` (`audit.go:501`), `SafetyCheck`
(`audit.go:516`), `SessionStart`/`SessionEnd` (`audit.go:572-591`), through
`LearningEvent` (`audit.go:645`).

Reading back: `AuditLogsDir` (`audit_reader.go:36`) locates the directory,
`LatestAuditLogPath` (`audit_reader.go:45`) picks the newest file,
`ReadRecentAuditEvents` (`audit_reader.go:69`) filters by event type with a
limit, and `CountAuditEventTypes` (`audit_reader.go:140`) histograms a file.
`ExportAuditFacts` (`audit_facts.go:37`) replays a file as Mangle source:
`Decl` headers via `declArgs` (`audit_facts.go:129`), duplicate suppression
via `dedupKey` (`audit_facts.go:142`), shape checks via `parseFactShape`
(`audit_facts.go:192`) and `isDigits` (`audit_facts.go:175`).

## Fresh-run hygiene

Each process mints a unique, lexically sortable prefix in `generateRunPrefix`
(`fresh_run.go:85`; the 46-char `timestamp_pid_seq_rand` shape is pinned by
`fresh_run_substantive_test.go:27`), readable via `currentRunPrefix`
(`fresh_run.go:98`) and parsed back out of filenames by
`runPrefixFromLogName` (`fresh_run.go:137`). On a fresh start,
`clearOrdinaryLogs` (`fresh_run.go:277`) removes or truncates previous
top-level `*.log` files — but never substantive audit logs, which
`isSubstantiveAuditLog` (`fresh_run.go:203`) protects, and never through a
symlinked directory, which `logsDirSymlinkRejected` (`fresh_run.go:116`, via
`isSymlink` at `fresh_run.go:105`) refuses. Partial writes are retried as
truncate-then-remove in `truncateOrRemove` (`fresh_run.go:425`).

## Size rotation

All three durable writers share one mechanism: `rotatingFile` (`rotate.go:41`).
`shouldRotateLocked` (`rotate.go:100`) trips on size, `rotateLocked`
(`rotate.go:115`) renames the full segment to a timestamped name from
`rotatedSegmentName` (`rotate.go:174`), `rotatedSegments` (`rotate.go:183`)
lists segments and `pruneRotatedSegments` (`rotate.go:196`) enforces the keep
count. Limits come from `rotationPolicy` (`rotate.go:214`). The three owners
are the audit file (`auditFile *rotatingFile`, `audit.go:125`, opened at
`audit.go:159`), each category sink (`sink *rotatingFile`, `logger.go:122`),
and the LLM trace file (`file *rotatingFile`, `llm_io_logger.go:35`, opened
at `llm_io_logger.go:73`).

## LLM trace + redaction

`initLLMIOLogger` (`llm_io_logger.go:52`) opens the trace file only when the
`trace_llm_io` option is set; every `Log*` entry point self-ensures init
(`llm_io_logger.go:104,116,189,218`), so callers never init explicitly.
`LogLLMRequest` (`llm_io_logger.go:115`) records callsite, prompts, history,
model, and temperature; `LogLLMResponse` (`llm_io_logger.go:188`) and
`LogLLMError` (`llm_io_logger.go:217`) close the loop with duration and token
estimates. `IsLLMIOTracingEnabled` (`llm_io_logger.go:103`) is the gate,
`CloseLLMIOLogger` (`llm_io_logger.go:239`) the drain, and
`resetLLMIOLogger` (`llm_io_logger.go:96`) exists for tests.

Secrets never reach the trace unredacted: `RedactSecrets` (`redact.go:52`),
length-capped by `RedactForLog` (`redact.go:76`), with `redactTrace`
(`redact.go:90`) for stack-trace text.
