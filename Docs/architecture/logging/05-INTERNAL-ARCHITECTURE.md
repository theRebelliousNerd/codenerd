# 05 — Internal Architecture (`internal/logging`)

> Last verified: **2026-07-13**

## 1. Component diagram

```
                    ┌─────────────────────────────┐
                    │  .nerd/config.json          │
                    │  logging: { debug_mode, … } │
                    └──────────────┬──────────────┘
                                   │ loadConfig
                                   ▼
┌──────────────────────────────────────────────────────────────┐
│                     package logging                          │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────┐ │
│  │ Category     │  │ AuditLogger  │  │ llmIOLogger       │ │
│  │ Logger map   │  │ + auditFile  │  │ (lazy Once)       │ │
│  │ + loggersMu  │  │ + auditMu    │  │                   │ │
│  └──────┬───────┘  └──────┬───────┘  └─────────┬─────────┘ │
│         │                 │                    │           │
│         ▼                 ▼                    ▼           │
│  <date>_<cat>.log   <date>_audit.log   <date>_llm_io.log   │
└──────────────────────────────────────────────────────────────┘
         ▲                 ▲                    ▲
         │                 │                    │
   Get / convenience   Audit / Timer.perf   LogLLM*
         │                 │                    │
    ┌────┴─────────────────┴────────────────────┴────┐
    │  cmd/nerd, chat, core peers, shards, tools…    │
    └────────────────────────────────────────────────┘
```

## 2. Subsystems

### 2.1 Config & init (`logger.go`)

- **Inputs:** workspace path; disk JSON  
- **State:** `workspace`, `logsDir`, `config`, `logLevel`, `initialized`  
- **Outputs:** boot banner on `CategoryBoot`; `InitAudit` side effect  
- **Guards:** empty workspace error; Once; early return if not debug  

### 2.2 Category logger factory (`Get`)

- Lazy open per category  
- Double-checked locking  
- Date-stamped filename binds category to calendar day of **first open** (midnight rollover requires process restart or new Get after CloseAll)

### 2.3 Logger methods

```
Logger
  category Category
  logger   *log.Logger   // nil ⇒ no-op
  file     *os.File

methods:
  Debug/Info/Warn/Error → level gate → text or logJSON
  StructuredLog         → StructuredLogEntry (+ fields)
  WithContext           → ContextLogger
```

### 2.4 Correlation helpers

```
RequestLogger { logger, requestID, fields }
  WithField → mutate fields map
  formatMsg → [req:id] msg | fields

ContextLogger { logger, context map }
  appends | ctx=%v (always text path today)
```

### 2.5 Timer pipeline

```
StartTimer(cat, op)
   └─ Timer{category, op, start}

Stop / StopWithInfo / StopWithThreshold
   ├─ Get(cat).Debug|Info|Warn
   └─ logPerformance(cat, op, elapsed, threshold?)
          ├─ sample or slow?
          ├─ Get(CategoryPerformance).StructuredLog
          └─ AuditWithContext(...).PerfMetric
```

### 2.6 Audit pipeline

```
InitAudit → open audit file (debug only)

AuditLogger.Log(event)
  defaults → generateMangleFact → json.Marshal → WriteString

generateMangleFact: switch on EventType
  → predicate(...) strings with /atom name style for event type
```

### 2.7 LLM I/O pipeline

```
initLLMIOLogger (Once)
  if !TraceLLMIO or logsDir=="" → disabled

LogLLMRequest/Response/Error
  mutex → build multi-line string → Print
```

## 3. State machine: process logging lifecycle

```
        [uninitialized]
               │ Initialize(ws) OK + debug
               ▼
        [active: dir exists, audit open?]
               │ Get(cat)
               ▼
        [category files open on demand]
               │ optional ReloadConfig
               ▼
        [config mutated; open files unchanged]
               │ CloseAll / CloseAudit / CloseLLMIO
               ▼
        [closed; maps cleared; Once still spent]
               │ Initialize again
               ▼
        [no-op Once — cannot re-init successfully]
```

## 4. Data shapes

### StructuredLogEntry

| Field | JSON | Purpose |
|-------|------|---------|
| Timestamp | `ts` | Unix ms |
| Category | `cat` | string category |
| Level | `lvl` | debug/info/warn/error |
| Message | `msg` | formatted text |
| File/Line | `file`/`line` | optional source |
| RequestID | `req` | correlation |
| Fields | `fields` | free map |

### AuditEvent

| Field | Purpose |
|-------|---------|
| EventType | maps to Mangle predicate family |
| SessionID / RequestID / ShardID | correlation |
| Target / Action | operation identity |
| Success / DurationMs / Error | outcome |
| Fields | extra structured args (tokens, size, phase, …) |
| MangleFact | preformatted fact string |

## 5. Control-flow example: timed kernel query (caller pattern)

```
timer := logging.StartTimer(logging.CategoryKernel, "Query")
results, err := engine.Query(...)
if err != nil {
    logging.KernelError("query failed: %v", err)
    logging.Audit().Error("kernel", err, false)
    timer.StopWithThreshold(50 * time.Millisecond)
    return err
}
logging.Audit().KernelQuery(pred, len(results), timer.Stop().Milliseconds())
```

(Exact call patterns vary by package; this is the intended composition.)

## 6. Threading model

- **Readers** of config use `RLock`  
- **Logger map** uses RWMutex with upgrade pattern  
- **Audit / LLM** serial writers (single file each)  
- Standard `log.Logger` concurrent-safe for category streams  

No worker pool, no async batching, no backpressure — writes are synchronous.

## 7. Extension points (safe)

| Extension | How |
|-----------|-----|
| New category | Add `CategoryX` const + convenience funcs |
| New audit event family | Add `AuditEventType` + `generateMangleFact` case + helper |
| New performance system threshold | Config map key matching category string |
| LLM callsite | Pass distinctive `callsite` string into `LogLLM*` |

## 8. Anti-extension (unsafe)

- Adding network export inside hot path  
- Importing kernel to auto-assert  
- Writing secrets unconditionally when `debug_mode` alone is true  
- Sharing one log file across categories without category field (breaks isolation)
