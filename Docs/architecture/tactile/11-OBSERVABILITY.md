# tactile — Observability

> Last verified: **2026-07-13**

## Logging category

| Item | Value |
|------|-------|
| Category constant | `logging.CategoryTactile` = `"tactile"` |
| Defined in | `internal/logging/logger.go` |
| Helpers | `Tactile`, `TactileDebug`, `TactileWarn`, `TactileError` in `logger_convenience.go` |

## What gets logged

| Level | Typical events |
|-------|----------------|
| Info (`Tactile`) | Executor init, command start/complete summary, container create/start/stop, file write summary, image pull |
| Debug | Routing mode, docker args, env details, pool borrow/return, retry attempts, line ranges |
| Warn | Timeout kills, docker unresponsive, truncated output, container health fail, retries exhausted |
| Error | Validation failures, spawn failures, pull failures, file IO failures |

## Timers

`logging.StartTimer(logging.CategoryTactile, name)` used for:

- Direct command execution  
- Docker command execution  
- Docker exec (persistent)  
- File read / write / edit  

Timers emit duration on stop (per logging package behavior).

## Metrics (in-process)

`ExecutionMetrics` / `ExecutionMetricsSnapshot` in `audit.go`:

| Field | Meaning |
|-------|---------|
| TotalExecutions | start events |
| SuccessfulExecutions | complete + exit 0 |
| FailedExecutions | infra failure / error events |
| KilledExecutions | killed events |
| BlockedExecutions | blocked events |
| TotalDurationMs / TotalCPUTimeMs / TotalMemoryBytes | aggregates |
| ExecutionsByBinary / BySession | histograms |
| SuccessRate / AvgDurationMs | derived |

Access:

- `AuditLogger.GetMetrics()`  
- VirtualStore `GetAuditMetrics()` when logger installed  

Not exported as Prometheus by default — snapshot is pull-based.

## Audit file sink

`AuditFileLogger`:

- JSON Lines of `AuditEvent`  
- `EnableFileLogging(path)` on AuditLogger  
- `Rotate()` renames with timestamp  

Useful for forensic offline review of motor activity.

## Fact stream as observability

When fact callback wired, kernel receives structured events. This is **logic telemetry**, not just logs:

- Correlation via RequestID / SessionID  
- Queryable history after Assert  
- Policy can react (if rules exist)

## Debug hooks

| Hook | Use |
|------|-----|
| `SetAuditCallback` on executors | Custom fan-out |
| `AuditLogger.AddCallback` | Multiple subscribers |
| `Command.Tags` | Extra `execution_tag` facts |
| `Capabilities()` | Feature discovery at runtime |
| `PooledExecutor.Stats()` | Pool pressure |

## Gaps

| Gap | Note |
|-----|------|
| No OpenTelemetry spans | Logging/timers only |
| Docker resource stats | SupportsResourceUsage false |
| Unified glass-box surface | Transparency package may not surface tactile metrics by default |
| Combined output not interleaved | Harder log reconstruction of true TTY order |

## Operator tips

1. Filter logs by category `tactile`.  
2. For kernel-visible history, ensure modern executor / fact callback is on.  
3. On timeout storms, check DefaultTimeout and campaign-specific configs.  
4. On Docker failures, check `IsAvailable` and daemon logs separately from agent logs.
