# TODO — `internal/logging`

> Last verified: **2026-08-16**
> Backlog. Items marked done carry the test that keeps them done.

## P1

- [x] Align config schema: `json_format` bool vs `config.LoggingConfig.Format` string
      → `format` is canonical, `json_format` is a legacy alias, both OR'd in
      `IsJSONFormat`. `config_schema_test.go` parses a `config.LoggingConfig`-shaped
      blob and fails if a key stops landing.
- [x] Document (and optionally implement) loading from the same file the rest of
      the app treats as source of truth → it is already the same file
      (`.nerd/config.json`); `ApplyConfig` / `ClearInjectedConfig` now let boot
      inject its parsed config instead of a second parse. **Open:**
      `internal/config` does not call it yet, so the file is still parsed twice
      (same file, so no divergence — only a wasted read).
- [x] LLM I/O redaction hooks for common secret patterns → `redact.go`, applied
      to prompts, history, responses and errors; `trace_llm_io_raw` opts out.

## P2

- [x] Make `CloseAll` close audit + LLM I/O → `closeAllSinks` closes categories,
      problems, audit and LLM I/O; all closers idempotent.
- [x] Resolve `sync.Once` + `--workspace` first-init race → `Initialize` rebinds
      when the absolute workspace changes (last-writer-wins), rearming the LLM
      I/O `sync.Once` and reloading config.
- [x] Add enabled-path tests for `trace_llm_io` writing expected markers →
      `llm_io_trace_test.go` asserts every block marker, redaction, raw opt-out,
      and that char counts describe the original text.

## P3

- [x] ContextLogger / RequestLogger respect `json_format` via structured entries
      → `emit(...)`; request ID lands in `req`, context in `fields`.
- [x] Operator playbook → `nerd audit playbook` (`cmd/nerd/cmd_audit.go`) and the
      playbook section in this corpus README.
- [x] Optional CLI offline: audit JSONL → `.mg` facts file → `nerd audit facts`
      (`ExportAuditFacts`), with Decls, dedup, and a real-parser test.
- [x] Size/time-based log rotation beyond daily name → `rotate.go`
      (`max_log_file_mb`, `max_log_file_minutes`, `max_rotated_files`).

## P4

- [x] Northstar convenience wrappers (Info/Debug/Warn/Error) — plus Regression
      and `PersistError`.
- [x] Optional `runtime.Caller` population of StructuredLogEntry file/line →
      `callerSite()`, JSON path only, walks past this package's own frames.
- [x] Expand call-site audit for `SafetyCheck` next to real `permitted` checks →
      `safety_callsite_audit_test.go` classifies every kernel-permission query
      site (gate / not-gate / known-gap) and fails on an unclassified site or on
      a gate package that stops recording its verdict.

## Still open

- [x] `internal/config` should call `logging.ApplyConfig` at boot so the config
      file is parsed once. Needs an edit outside this package.
      → Closed. `internal/config/logging.go` adds `LoggingConfig.ToLoggingConfig()`, `internal/config/user_config.go` adds `ApplyLoggingConfig()`, and `internal/system/factory.go` calls it once `appCfg` is resolved (covering the `UserConfigOverride` branch too). `logging.Initialize` no longer re-parses `.nerd/config.json`, and the injected config is pinned so a later `ReloadConfig` cannot revert to disk. Note that only fields present on both structs are mapped: `LoggingConfig` has no `TraceLLMIORaw`, `MaxLogFileMB`, `MaxLogFileMinutes` or `MaxRotatedFiles`.
- [x] `internal/shards/system/constitution.go` (`ConstitutionGateShard.CheckAction`)
      decides allow/deny without an audit event.
      → Closed. Both verdict branches now call `logging.Audit().SafetyCheck` in `internal/shards/system/constitution.go` (allow at line 331, deny at 339), in the style of `internal/core/virtual_store_routing.go`. The `safety_callsite_audit_test.go` inventory entry moved from `classKnownGap` to `classGate` with `auditedIn` set to that file, so removing the call is now a test failure rather than a silent regression.
- [x] `RequestLogger.WithField` still mutates a shared map (OPEN-QUESTIONS Q5).
      → Closed. `WithField` is copy-on-write: it returns a newly derived `RequestLogger` with a copied field map instead of mutating the receiver and returning it. That fixes both the aliasing (sibling derivations previously saw each other's fields) and the data race on a plain map. New tests in `request_logger_fields_test.go` cover non-mutation and 50 concurrent derivations, and pass under `-race`. `TestRequestLogger_WithField_ShouldChain` previously asserted pointer identity, which was asserting the defect; it now verifies chaining by accumulated fields. `WithField` had zero production callers, so nothing depended on the old side effect.
- [x] Category taxonomy cap (Q7) -> `internal/logging/category_inventory_test.go` declares a 30-row inventory with an owner per category and a numeric `categoryCap`, and its guard AST-parses `internal/logging/logger.go` and fails on drift in either direction — a new constant with no row, or a stale row whose constant is gone. This closes only the cap half of Q7; the flat-versus-hierarchical redesign stays open in OPEN-QUESTIONS.md.
- [x] Bridge to `internal/observability` spans (Q6).
      → resolved 2026-08-16 as keep-file-only. `internal/observability` has no OTel spans to bridge to - it is `runtime/trace` flight recording plus runtime metrics - so the item's premise had no referent. `otel/sdk` is not a dependency (the API is present only as `// indirect`, with zero imports repo-wide). Machine-readable export already ships as `EventSink` plus the env-attached NDJSON sink wired in `NewGlassBoxEventBus`. Full reasoning in OPEN-QUESTIONS.md Q6.

## Done (already in code before this pass)

- [x] Categorized file logging with debug master switch
- [x] Audit JSON + Mangle fact strings
- [x] LLM I/O full dump opt-in
- [x] Timers + performance sampling/thresholds
- [x] Dense unit test suite including concurrency
- [x] Idempotent Initialize via sync.Once
- [x] Run-prefix isolation + cross-run retention (`fresh_run.go`)
- [x] Aggregated `<run>_problems.log`
