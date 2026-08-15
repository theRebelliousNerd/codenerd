# TODO — `internal/logging`

> Last verified: **2026-08-15**
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

- [ ] `internal/config` should call `logging.ApplyConfig` at boot so the config
      file is parsed once. Needs an edit outside this package.
- [ ] `internal/shards/system/constitution.go` (`ConstitutionGateShard.CheckAction`)
      decides allow/deny without an audit event. Listed as a known gap in
      `safety_callsite_audit_test.go`; the fix belongs to `internal/shards`.
- [ ] `RequestLogger.WithField` still mutates a shared map (OPEN-QUESTIONS Q5).
- [ ] Bridge to `internal/observability` spans (Q6) and category taxonomy cap (Q7).

## Done (already in code before this pass)

- [x] Categorized file logging with debug master switch
- [x] Audit JSON + Mangle fact strings
- [x] LLM I/O full dump opt-in
- [x] Timers + performance sampling/thresholds
- [x] Dense unit test suite including concurrency
- [x] Idempotent Initialize via sync.Once
- [x] Run-prefix isolation + cross-run retention (`fresh_run.go`)
- [x] Aggregated `<run>_problems.log`
