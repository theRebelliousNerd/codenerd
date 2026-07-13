# logging — Architecture Corpus (`internal/logging`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/logging/`  
> Scale: **4** non-test Go sources ≈ **2,034** lines; **5** test files ≈ **141** tests; **0** `.mg`

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
  logger.go                 # Initialize, Category, Logger, Timer, RequestLogger
  logger_convenience.go     # Boot/Kernel/API/... Info/Debug/Warn/Error wrappers
  audit.go                  # AuditEvent, AuditLogger, Mangle fact generation
  llm_io_logger.go          # Full LLM request/response dump
  *_test.go                 # Unit + coverage-oriented tests
```

## Verify

```powershell
go test ./internal/logging/...
go test -race ./internal/logging/...
```

Enable diagnostics in a workspace:

```json
// .nerd/config.json
{
  "logging": {
    "debug_mode": true,
    "level": "debug",
    "trace_llm_io": false,
    "json_format": false,
    "categories": { "kernel": true, "session": true },
    "performance_sampling": 0.1,
    "performance_thresholds_ms": { "default": 100, "kernel": 50 }
  }
}
```

Artifacts land under:

```
.nerd/logs/
  2026-07-13_boot.log
  2026-07-13_kernel.log
  2026-07-13_audit.log
  2026-07-13_llm_io.log      # only if trace_llm_io
  2026-07-13_performance.log
```

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring evidence, honest gaps — **not** auto-inventory stubs.
