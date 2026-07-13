# 06 — Public API and Types (`internal/logging`)

> Last verified: **2026-07-13**  
> File references are primary sources under `internal/logging/`.

## 1. Package identity

```go
package logging // import "codenerd/internal/logging"
```

Package comment (`logger.go`): config-driven categorized file-based logging; controlled by `debug_mode`; writes under `.nerd/logs/`.

---

## 2. Types

### Category

```go
type Category string
```

Constants listed in [02-CURRENT-STATE.md](02-CURRENT-STATE.md) / `logger.go` (~lines 20–58).

### StructuredLogEntry

| Field | Type | JSON tag |
|-------|------|----------|
| Timestamp | int64 | `ts` |
| Category | string | `cat` |
| Level | string | `lvl` |
| Message | string | `msg` |
| File | string | `file` |
| Line | int | `line` |
| RequestID | string | `req,omitempty` |
| Fields | map[string]any | `fields,omitempty` |

**File:** `logger.go`

### Logger

| Field | Visibility | Role |
|-------|------------|------|
| category | unexported | Category |
| logger | unexported | `*log.Logger` or nil |
| file | unexported | `*os.File` or nil |

**Methods:** `Debug`, `Info`, `Warn`, `Error`, `StructuredLog`, `WithContext`

### ContextLogger / RequestLogger / Timer

| Type | Construction | Methods |
|------|--------------|---------|
| `ContextLogger` | `Logger.WithContext(map)` | Debug/Info/Warn/Error |
| `RequestLogger` | `WithRequestID(cat, id)` | WithField, Debug/Info/Warn/Error |
| `Timer` | `StartTimer(cat, op)` | Stop, StopWithInfo, StopWithThreshold |

### AuditEventType / AuditEvent / AuditLogger

**File:** `audit.go`

`AuditEventType` is a string enum of event names (`shard_spawn`, `action_route`, `kernel_assert`, `llm_request`, `file_read`, `session_start`, `intent_parsed`, `memory_store`, `tool_invoke`, `safety_check`, `perf_metric`, `error_generic`, `campaign_start`, `learning_start`, …).

`AuditEvent` is the full JSON-serializable record including `MangleFact`.

`AuditLogger` holds optional `sessionID`, `category`, `shardID` scopes.

### LLMMessage / llmIOLogger

| Type | Exported? | File |
|------|-----------|------|
| `LLMMessage` | yes | `llm_io_logger.go` |
| `llmIOLogger` | no | `llm_io_logger.go` |

---

## 3. Lifecycle functions

| Func | Signature | Notes |
|------|-----------|-------|
| `Initialize` | `(ws string) error` | Once; empty ws errors |
| `ReloadConfig` | `() error` | Re-read config.json |
| `IsDebugMode` | `() bool` | |
| `IsCategoryEnabled` | `(Category) bool` | |
| `IsJSONFormat` | `() bool` | |
| `Get` | `(Category) *Logger` | May be no-op |
| `CloseAll` | `()` | Category files only |
| `InitAudit` | `() error` | Called from init |
| `CloseAudit` | `()` | |
| `Audit` | `() *AuditLogger` | Global singleton lazy |
| `AuditWithSession` | `(sessionID string) *AuditLogger` | |
| `AuditWithShard` | `(shardID string) *AuditLogger` | |
| `AuditWithContext` | `(session, shard string, cat Category) *AuditLogger` | |
| `WithRequestID` | `(Category, string) *RequestLogger` | |
| `StartTimer` | `(Category, string) *Timer` | |
| `IsLLMIOTracingEnabled` | `() bool` | Lazy init |
| `LogLLMRequest` | `(callsite, system, user string, history []LLMMessage, model string, temp float64)` | |
| `LogLLMResponse` | `(callsite, response string, duration time.Duration, tokenEstimate int)` | |
| `LogLLMError` | `(callsite string, err error, duration time.Duration)` | |
| `CloseLLMIOLogger` | `()` | |

---

## 4. AuditLogger methods (typed)

| Method | Purpose |
|--------|---------|
| `Log(AuditEvent)` | Core write |
| `ShardSpawn` / `ShardExecute` / `ShardComplete` | Shard lifecycle |
| `ActionRoute` / `ActionComplete` | Action routing |
| `KernelAssert` / `KernelQuery` | Kernel ops |
| `LLMCall` | LLM response-oriented audit |
| `FileOp` | File R/W/D |
| `IntentParsed` | Perception outcome |
| `SafetyCheck` | Allow/block with reason |
| `PerfMetric` | Duration vs threshold |
| `Error` | Generic/critical error |
| `SessionStart` / `SessionEnd` | Session |
| `TurnStart` / `TurnEnd` | Turn |
| `ToolExec` | Tool complete/error |
| `CampaignEvent` | Campaign lifecycle |
| `LearningEvent` | Autopoiesis |

---

## 5. Convenience package functions

Pattern: `X` → Info, `XDebug` → Debug; many also have `XWarn` / `XError`.

Covered families: Boot, Session, Kernel, API, Perception, Articulation, Routing, Tools, VirtualStore, Shards, Coder, Tester, Reviewer, Researcher, SystemShards, Dream, Autopoiesis, Campaign, Context, World, Embedding, Store, Browser, Tactile, JIT, Build.

**Missing convenience:** dedicated Northstar Info/Debug/Warn/Error (category constant exists — use `Get(CategoryNorthstar)`).

**File:** `logger_convenience.go`

---

## 6. Level constants

```go
const (
  LevelDebug = 0
  LevelInfo  = 1
  LevelWarn  = 2
  LevelError = 3
)
```

---

## 7. Unexported but behaviorally important

| Symbol | Why it matters |
|--------|----------------|
| `loggingConfig` | Exact JSON keys this package honors |
| `generateMangleFact` | Audit predicate shapes |
| `escapeString` | Mangle string safety |
| `logPerformance` | Timer side effects |
| `performanceSamplingRate` / `performanceThresholdMs` | Sampling policy |

---

## 8. Usage snippets (canonical)

### Minimal category log

```go
logging.Get(logging.CategoryKernel).Info("facts loaded: %d", n)
// or
logging.Kernel("facts loaded: %d", n)
```

### Timer

```go
t := logging.StartTimer(logging.CategoryWorld, "Scan")
defer t.StopWithThreshold(2 * time.Second)
```

### Audit

```go
a := logging.AuditWithContext(sessionID, shardID, logging.CategoryShards)
a.ShardSpawn(shardID, "coder")
```

### LLM I/O

```go
if logging.IsLLMIOTracingEnabled() {
    logging.LogLLMRequest("articulation-emitter", sys, user, hist, model, temp)
}
```

---

## 9. Compatibility notes for callers

1. Always safe to call convenience functions before init — they no-op.  
2. After `Initialize` with debug off, still safe — no-op.  
3. Do not rely on `CloseAll` to flush audit/LLM I/O.  
4. Do not pass secrets into Info strings when debug is on in shared workspaces.  
5. Prefer typed categories over inventing string filenames.
