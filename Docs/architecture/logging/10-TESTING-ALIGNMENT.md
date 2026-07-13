# 10 — Testing Alignment (`internal/logging`)

> Last verified: **2026-07-13**

## 1. Commands

```powershell
go test ./internal/logging/...
go test -count=1 ./internal/logging/...
go test -race ./internal/logging/...
go test -bench=. ./internal/logging/
```

No CGO required for this package’s unit tests.

## 2. Test files vs behavior

| File | What it proves |
|------|----------------|
| `logger_test.go` | All categories create files when debug on; silent when off; category toggle; timer produces output |
| `logging_comprehensive_test.go` | Init idempotency, levels, concurrency multi-writer, mangle fact families, close safety, convenience no-panic |
| `audit_coverage_test.go` | `escapeString` matrix; each major `generateMangleFact` branch; audit scope helpers |
| `coverage_boost_test.go` | Reload levels, sampling/threshold helpers, JSON mode, Context/Request loggers, Warn/Error convenience, LLM disabled paths, audit helpers no-panic without file |
| `audit_benchmark_test.go` | `escapeString` performance regression guard |

## 3. Coverage strengths

- **Enablement matrix:** debug on/off, category on/off, nil categories map  
- **Level matrix:** debug/info/warn/error thresholds  
- **Fact generation:** shard, action, kernel, LLM tokens, file size, intent, safety, perf, error, session, tool, campaign, learning, default  
- **Concurrency:** multiple goroutines writing without race detector failures (when run with `-race`)  
- **Disabled LLM I/O:** no panic  

## 4. Test infrastructure patterns

Tests reset package globals because production uses `sync.Once`:

```go
CloseAll(); CloseAudit()
loggers = make(...)
logsDir = ""; workspace = ""
configLoaded = false; config = loggingConfig{}
initOnce = sync.Once{}; initErr = nil; initialized = false
auditLogger = nil
```

This reaches into **unexported** state — correct for same-package tests; production callers cannot rebind.

Temp workspaces write `.nerd/config.json` with `"logging": { "debug_mode": true, ... }`.

## 5. Gaps

| Gap | Severity | Notes |
|-----|----------|-------|
| LLM I/O **enabled** path (file contents assertions) | Medium | Mostly disabled-path tests |
| Cross-package integration (chat boot + logging) | Medium | Lives outside package tests |
| Midnight rollover / date filename | Low | Untested |
| Secret redaction | N/A | Feature absent |
| Audit file JSON round-trip load into Mangle engine | Low | Out of package |
| Workspace Once wrong-first-workspace | Medium | Process-level; hard unit test without exported reset |

## 6. Recommended additional tests (backlog)

1. Enable `trace_llm_io` in temp config; assert `*_llm_io.log` contains BEGIN SYSTEM PROMPT.  
2. Assert `CloseAll` does **not** currently close audit (documents bug or drive fix + invert test).  
3. Performance: slow path always logs even when `performance_sampling` is 0.01.  
4. `json_format:true` end-to-end line parse as `StructuredLogEntry`.  

## 7. Alignment with package principles

| Principle | Test support |
|-----------|--------------|
| Production silence | `TestDebugModeDisabled`, comprehensive disabled cases |
| No panic | Dozens of no-panic cases |
| Mangle escape | escape + error event tests |
| Sampling clamps | `performanceSamplingRate` table tests |

## 8. CI guidance

Package should be in the default `go test ./...` set. Prefer including `-race` for this package periodically due to shared globals and multi-consumer use.
