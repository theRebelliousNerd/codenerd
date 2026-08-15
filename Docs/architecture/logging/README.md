# logging — Architecture Corpus (`internal/logging`)

> Last verified against codebase: **2026-08-15**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/logging/`  
> Scale: **9** non-test Go sources; **15** test files; **0** `.mg`

## Scope

This corpus documents codeNERD’s **config-driven, categorized, file-based diagnostic logging** subsystem:

1. **Category loggers** — per-system append files under `.nerd/logs/YYYY-MM-DD_<category>.log`
2. **Audit trail** — JSON lines + preformatted Mangle facts in `YYYY-MM-DD_audit.log`
3. **LLM I/O tracing** — full prompt packages / responses when `trace_llm_io` is on
4. **Timers / performance** — `Timer` helpers + sampled `performance` category metrics

It is **not** the Uber zap CLI console logger (`cmd/nerd/main.go`), **not** glass-box / transparency UX, and **not** `internal/observability` metrics. Those are adjacent surfaces that operators often confuse with this package.

## North-star placement

| Concern | Owner |
|---------|--------|
| LLM creative center | model providers / perception / articulation |
| Executive control | Mangle kernel (`permitted`, `next_action`) |
| Diagnostic evidence of what happened | **`internal/logging`** (this package) |
| Live operator glass box | CLI transparency / observability |

Logging is **substrate telemetry**: it must never become the executive, never invent policy, and must default to **silent production** when `debug_mode` is false.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product / architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and functions |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream / downstream imports |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, CLI, chat, consumers |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, privacy |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Categories, audit, LLM I/O, performance |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Source layout

```
internal/logging/
  logger.go                 # Initialize/rebind, Category, Logger, Timer, RequestLogger
  logger_convenience.go     # Boot/Kernel/API/... Info/Debug/Warn/Error wrappers
  audit.go                  # AuditEvent, AuditLogger, Mangle fact generation
  audit_reader.go           # read the audit JSONL back
  audit_facts.go            # audit JSONL -> loadable .mg (offline forensics)
  llm_io_logger.go          # Full LLM request/response dump (redacted)
  redact.go                 # secret redaction at the log boundary
  rotate.go                 # size/age rotation for every sink
  fresh_run.go              # run prefixes + cross-run retention
  *_test.go                 # Unit + coverage-oriented tests
```

## Verify

```powershell
go test ./internal/logging/...
go test -race ./internal/logging/...
```

Enable diagnostics in a workspace:

```json
// .nerd/config.json  — the same file config.LoadUserConfig reads
{
  "logging": {
    "debug_mode": true,
    "level": "debug",
    "format": "text",             // "json" for structured lines; "json_format": true is a legacy alias
    "trace_llm_io": false,        // full prompt/response dump
    "trace_llm_io_raw": false,    // disable secret redaction in that dump (unsafe)
    "categories": { "kernel": true, "session": true },
    "performance_sampling": 0.1,
    "performance_thresholds_ms": { "default": 100, "kernel": 50 },
    "max_log_file_mb": 32,        // rotate a segment past this size (-1 = never)
    "max_log_file_minutes": 0,    // rotate a segment older than this (0 = off)
    "max_rotated_files": 3        // archived segments kept per file
  }
}
```

Boot can inject an already-parsed config with `logging.ApplyConfig(logging.Config{...})`
instead of letting this package re-read the file.

Artifacts land under `.nerd/logs/`, one set per run (`<runPrefix>_` = sortable UTC
timestamp + pid + counter; the newest runs are kept, older ones swept at startup):

```
.nerd/logs/
  <run>_boot.log
  <run>_problems.log         # every WARN and ERROR from every category — start here
  <run>_kernel.log
  <run>_audit.log            # JSONL, one Mangle fact per line
  <run>_llm_io.log           # only if trace_llm_io
  <run>_performance.log
  <run>_kernel.<stamp>.log   # rotated segment, expires with its run
```

## Operator playbook

`nerd audit playbook` prints this from the CLI.

| Want | Do |
|------|----|
| See what failed | `<run>_problems.log`, or `nerd logs` for a grouped view |
| Query what happened | `nerd audit facts --out run.mg` (offline `.mg`: Decls + deduped facts) |
| Only safety verdicts | `nerd audit facts --event safety_allow --event safety_block` |
| Debug a prompt | `trace_llm_io: true`, read `<run>_llm_io.log` (secrets redacted) |
| See a redacted value | `trace_llm_io_raw: true` for one run, then delete the file |
| Machine-readable lines | `"format": "json"` — every line becomes a `StructuredLogEntry` |

Audit logging is gated on `debug_mode`: an empty `.nerd/logs` almost always means
diagnostics are off, not that nothing happened.

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring evidence, honest gaps — **not** auto-inventory stubs.
