# 02 — Current State (`internal/logging`)

> Last verified: **2026-07-13**  
> Inventory grounded in on-disk sources under `internal/logging/`

## 1. File inventory

### Non-test sources (4)

| File | ≈Lines | Responsibility |
|------|-------:|----------------|
| `logger.go` | 681 | Package docs, categories, init, config load, Logger, JSON, Context/Request loggers, Timer, performance |
| `audit.go` | 618 | AuditEvent(s), InitAudit, generateMangleFact, typed audit helpers, escapeString |
| `logger_convenience.go` | 531 | Package-level Info/Debug/Warn/Error per category |
| `llm_io_logger.go` | 207 | LLM I/O enablement and multi-line dumps |

**Total source ≈ 2,034 lines.**

### Tests (5)

| File | Focus |
|------|-------|
| `logger_test.go` | End-to-end category files, disable modes, timer |
| `logging_comprehensive_test.go` | Init, concurrency, mangle facts, structured entry |
| `audit_coverage_test.go` | escape + generateMangleFact branches |
| `coverage_boost_test.go` | reload, sampling, JSON, request logger, convenience, LLM disabled |
| `audit_benchmark_test.go` | escapeString bench |

**≈ 141 `Test*` functions** (including coverage variants).

### Mangle

**0** `.mg` files in package. Mangle appears only as **string generation** inside audit.

---

## 2. Runtime globals (process state)

| Symbol | Type | Role |
|--------|------|------|
| `loggers` | `map[Category]*Logger` | Open category writers |
| `loggersMu` | `sync.RWMutex` | Map protection |
| `logsDir` | `string` | `<ws>/.nerd/logs` |
| `workspace` | `string` | Init workspace |
| `config` | `loggingConfig` | Loaded settings |
| `configLoaded` | `bool` | Load happened |
| `configMu` | `sync.RWMutex` | Config protection |
| `logLevel` | `int` | 0–3 threshold |
| `initOnce` / `initErr` / `initialized` | Once + error + flag | Idempotent init |
| `auditFile` / `auditMu` / `auditLogger` | file + mutex + singleton | Audit stream |
| `llmIO` / `llmIOOnce` | pointer + Once | LLM I/O stream |

---

## 3. Category surface (29 constants)

Core: `boot`, `session`, `performance`, `kernel`, `api`  
Transduction: `perception`, `articulation`  
Execution: `routing`, `tools`, `virtual_store`  
Shards: `shards`, `coder`, `tester`, `reviewer`, `researcher`, `system_shards`  
Advanced: `dream`, `autopoiesis`, `campaign`, `context`, `world`, `embedding`, `store`, `browser`, `tactile`, `jit`, `build`, `northstar`

---

## 4. API surface summary

### Lifecycle

- `Initialize(ws string) error`
- `ReloadConfig() error`
- `CloseAll()`, `CloseAudit()`, `CloseLLMIOLogger()`
- `IsDebugMode() bool`, `IsCategoryEnabled(Category) bool`, `IsJSONFormat() bool`
- `IsLLMIOTracingEnabled() bool`

### Logging

- `Get(Category) *Logger`
- `Logger.{Debug,Info,Warn,Error,StructuredLog,WithContext}`
- `WithRequestID(Category, string) *RequestLogger`
- `StartTimer(Category, string) *Timer` + `Stop*`
- Large convenience set in `logger_convenience.go`

### Audit

- `InitAudit() error`, `Audit() *AuditLogger`
- `AuditWithSession`, `AuditWithShard`, `AuditWithContext`
- `AuditLogger.Log` + typed helpers (Shard*, Action*, Kernel*, LLMCall, FileOp, IntentParsed, SafetyCheck, PerfMetric, Error, Session*, Turn*, ToolExec, CampaignEvent, LearningEvent)

### LLM I/O

- `LogLLMRequest`, `LogLLMResponse`, `LogLLMError`
- Type `LLMMessage{Role, Content}`

---

## 5. On-disk artifacts (when debug on)

```
<workspace>/.nerd/logs/
  YYYY-MM-DD_<category>.log
  YYYY-MM-DD_audit.log
  YYYY-MM-DD_llm_io.log          # if trace_llm_io
```

Permissions: dirs `0755`, files `0644`. Append-only open flags.

---

## 6. Hotspots

| Hotspot | Why it matters |
|---------|----------------|
| `Get` double-checked locking | High call frequency from many packages |
| `logPerformance` | Extra work + audit on every timer stop |
| `AuditLogger.Log` | Serializes all audit through one mutex/file |
| `LogLLMRequest` | Can write multi-MB prompt packages |
| `sync.Once` init | First workspace wins for process lifetime |

---

## 7. Adjacent systems (not this package)

| System | Relationship |
|--------|--------------|
| Uber zap in `cmd/nerd` | Separate console logging for non-interactive CLI |
| `internal/observability` | Metrics / startup snapshot; may use logging as sink |
| Glass box / transparency | Operator UX; different event stream |
| `internal/config.LoggingConfig` | Schema mirror / defaults; not imported here |

---

## 8. Maturity assessment

| Area | Maturity |
|------|----------|
| Category file logging | **High** — production use across monorepo |
| Audit helpers | **High** API completeness; **medium** adoption consistency across callers |
| LLM I/O tracer | **High** implementation; **low** default enablement (by design) |
| Performance sampling | **Medium** — implemented + tested; operator docs sparse |
| Config dual schema | **Medium risk** |
