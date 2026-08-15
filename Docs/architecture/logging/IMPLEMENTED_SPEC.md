# logging — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/logging/`  
> Scale: **4** non-test Go files ≈ **2,034** lines; **5** test files; **0** Mangle sources  
> Config mirror: `internal/config/logging.go` (`LoggingConfig`) — **not imported** by this package

## 1. Overview

`internal/logging` is codeNERD’s **workspace-scoped diagnostic file logger**. It answers: *what did each subsystem do, when, and how long did it take?* — without participating in the OODA executive path.

Three independent streams share one config master-switch (`debug_mode`):

| Stream | Primary API | On-disk artifact |
|--------|-------------|------------------|
| **Category logs** | `Get(Category)`, convenience funcs, `Logger.{Debug,Info,Warn,Error}` | `.nerd/logs/<date>_<category>.log` |
| **Audit events** | `Audit()`, `AuditWithContext(...)`, typed helpers | `.nerd/logs/<date>_audit.log` (JSON lines + `mangle` field) |
| **LLM I/O dump** | `LogLLMRequest` / `LogLLMResponse` / `LogLLMError` | `.nerd/logs/<date>_llm_io.log` |

A fourth cross-cutting path, **performance telemetry**, is layered on timers: `StartTimer` → `Stop*` writes category debug/info/warn **and** sampled/thresholded entries into `CategoryPerformance` plus audit `PerfMetric`.

### Key characteristics

| Property | Value |
|----------|-------|
| Package | `codenerd/internal/logging` |
| Master switch | `logging.debug_mode` in `.nerd/config.json` |
| Default when no config | **Production silence** (`DebugMode=false`) |
| Init model | `Initialize(workspace)` once via `sync.Once` |
| Circular-import avoidance | Local `loggingConfig` mirror; does **not** import `internal/config` |
| Concurrency | `loggersMu`, `configMu`, `auditMu`, `llmIO.mu` |
| Mangle integration | **String facts only** in audit (`generateMangleFact`); no kernel assert from this package |
| Constitutional role | Observes safety checks when callers use `AuditLogger.SafetyCheck`; does not enforce `permitted` |

### Place in fact-flow

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools → articulation → user

         │              │                 │
         └──── logging.Get / Audit / LogLLM* ────┘
                    (side-channel evidence only)
```

Logging never produces `user_intent`, never derives `next_action`, and never routes actions. Callers instrument their own paths.

---

## 2. Implementation status

| Component | Status | Evidence |
|-----------|--------|----------|
| Idempotent `Initialize` + empty-workspace error | **Implemented** | `logger.go` `Initialize` / `initializeInternal` |
| Config load from `.nerd/config.json` | **Implemented** | `loadConfig` |
| Per-category file loggers with date prefix | **Implemented** | `Get` |
| Level filtering (debug/info/warn/error) | **Implemented** | `Logger` methods + `logLevel` |
| JSON structured lines (`json_format`) | **Implemented** | `logJSON`, `StructuredLog` |
| No-op when disabled | **Implemented** | `Get` returns `Logger` with nil `logger` |
| Convenience package funcs (all major categories) | **Implemented** | `logger_convenience.go` |
| `ContextLogger` / `RequestLogger` | **Implemented** | `logger.go` |
| `Timer` + performance sampling/thresholds | **Implemented** | `StartTimer`, `logPerformance` |
| Audit JSON + Mangle fact strings | **Implemented** | `audit.go` |
| Typed audit helpers (shard/action/kernel/LLM/…) | **Implemented** | `AuditLogger` methods |
| LLM I/O full dump (`trace_llm_io`) | **Implemented** | `llm_io_logger.go` |
| `CloseAll` / `CloseAudit` / `CloseLLMIOLogger` | **Implemented** | separate closers |
| Unified shutdown that closes all three streams | **Implemented** | `CloseAll` → `closeAllSinks` (categories + problems + audit + LLM I/O) |
| Config field parity with `config.LoggingConfig` | **Implemented** | `format` canonical, `json_format` legacy alias; `config_schema_test.go` pins both |
| Feed audit Mangle facts into kernel | **Not implemented (by design)** | offline only; `ExportAuditFacts` writes a standalone `.mg` |
| Audit facts parse as Mangle | **Implemented** | `mangleBool` / `mangleString`; parser-backed test in `cmd/nerd/cmd_audit_test.go` |
| Northstar convenience Info/Debug/Warn/Error | **Implemented** | `logger_convenience.go` (also Regression, PersistError) |
| Log rotation beyond date prefix | **Implemented** | `rotate.go` size + age rotation, bounded archive count |
| Redaction of secrets in LLM I/O | **Implemented** | `redact.go`, on by default, `trace_llm_io_raw` opts out |
| Workspace rebind after first `Initialize` | **Implemented** | last-writer-wins on absolute path; `init_rebind_test.go` |
| Injected config from boot (no double parse) | **Implemented** | `ApplyConfig` / `ClearInjectedConfig` (caller-side wiring still open) |
| Structured `ContextLogger` / `RequestLogger` | **Implemented** | `emit(...)` honours `json_format` |
| Offline audit → `.mg` CLI | **Implemented** | `nerd audit facts` (`cmd/nerd/cmd_audit.go`) |

**Overall:** production-ready diagnostic substrate with high test density — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/logging/
  logger.go                   # core runtime
  logger_convenience.go       # thin wrappers per category
  audit.go                    # structured audit + Mangle facts
  audit_reader.go             # read audit JSONL back (transparency/glassbox)
  audit_facts.go              # audit JSONL → loadable .mg facts (offline)
  llm_io_logger.go            # LLM prompt/response tracing
  redact.go                   # secret redaction at the log boundary
  rotate.go                   # size/age rotation for every sink
  fresh_run.go                # run prefixes + cross-run retention
  logger_test.go              # category/debug/timer integration-style tests
  logging_comprehensive_test.go
  audit_coverage_test.go
  audit_benchmark_test.go
  coverage_boost_test.go
```

### 3.2 Non-test sources (line counts ≈)

| Path | Lines | Role |
|------|------:|------|
| `internal/logging/logger.go` | ~681 | Init, categories, Logger, timers, request/context loggers |
| `internal/logging/audit.go` | ~618 | AuditEvent, fact generation, typed helpers |
| `internal/logging/logger_convenience.go` | ~531 | Package-level Boot/Kernel/… wrappers |
| `internal/logging/llm_io_logger.go` | ~207 | Full LLM I/O dump |

### 3.3 Test sources

| Path | Role |
|------|------|
| `logger_test.go` | All categories write files; debug off; category toggle; timer |
| `logging_comprehensive_test.go` | Init, levels, concurrency, audit facts, close safety |
| `audit_coverage_test.go` | `escapeString`, every `generateMangleFact` branch |
| `coverage_boost_test.go` | Reload, sampling, JSON, request logger, convenience, LLM I/O disabled paths |
| `audit_benchmark_test.go` | `escapeString` performance |

---

## 4. Categories

Defined as `type Category string` constants in `logger.go`:

### Core

| Constant | File key | Typical use |
|----------|----------|-------------|
| `CategoryBoot` | `boot` | Startup, config, init |
| `CategorySession` | `session` | Session lifecycle |
| `CategoryPerformance` | `performance` | Aggregated slow/sampled metrics |
| `CategoryKernel` | `kernel` | Mangle engine operations |
| `CategoryAPI` | `api` | LLM HTTP/API layer |

### Transduction

| Constant | File key |
|----------|----------|
| `CategoryPerception` | `perception` |
| `CategoryArticulation` | `articulation` |

### Execution

| Constant | File key |
|----------|----------|
| `CategoryRouting` | `routing` |
| `CategoryTools` | `tools` |
| `CategoryVirtualStore` | `virtual_store` |

### Shards

| Constant | File key |
|----------|----------|
| `CategoryShards` | `shards` |
| `CategoryCoder` | `coder` |
| `CategoryTester` | `tester` |
| `CategoryReviewer` | `reviewer` |
| `CategoryResearcher` | `researcher` |
| `CategorySystemShards` | `system_shards` |

### Advanced

| Constant | File key |
|----------|----------|
| `CategoryDream` | `dream` |
| `CategoryAutopoiesis` | `autopoiesis` |
| `CategoryCampaign` | `campaign` |
| `CategoryContext` | `context` |
| `CategoryWorld` | `world` |
| `CategoryEmbedding` | `embedding` |
| `CategoryStore` | `store` |
| `CategoryBrowser` | `browser` |
| `CategoryTactile` | `tactile` |
| `CategoryJIT` | `jit` |
| `CategoryBuild` | `build` |
| `CategoryNorthstar` | `northstar` |

**Enablement rule** (`IsCategoryEnabled`):

1. If `!DebugMode` → false  
2. If `Categories == nil` → true (all on in debug)  
3. If category key missing → true  
4. Else use map value  

Unknown category strings are not validated at compile time for map keys; only typed `Category` constants are first-class.

---

## 5. Initialization control flow

```
main() / PersistentPreRunE / chat session boot
        │
        ▼
logging.Initialize(workspace)
        │  sync.Once
        ▼
initializeInternal(ws)
        │
        ├─ workspace = ws
        ├─ logsDir = <ws>/.nerd/logs
        ├─ loadConfig()  ──► .nerd/config.json → loggingConfig
        │                      missing file ⇒ DebugMode=false
        │                      parse error  ⇒ stderr warn + DebugMode=false
        │
        ├─ if !DebugMode: return nil   (no directory, silent)
        │
        ├─ MkdirAll(logsDir)
        ├─ Get(CategoryBoot).Info(...)  boot banner
        └─ InitAudit()                  opens <date>_audit.log
```

**Idempotency and rebind:** `Initialize` normalizes the path with `filepath.Abs` and compares it to `boundWorkspace`.

- same workspace → no-op, returns the first `initErr` (the original Init-Spam guard)
- different workspace → **rebind**: close every sink, rearm the LLM I/O `sync.Once`, drop the loaded config, re-run `initializeInternal`, and note the move in the new workspace's boot log

The rebind exists because `main()` calls `Initialize(cwd)` before Cobra parses argv, so under a bare `sync.Once` the later `--workspace` init in `PersistentPreRunE` silently did nothing and the entire run — audit trail included — logged into the wrong tree. Binding is last-writer-wins, matching the flag's override semantics.

Empty workspace errors before the `Once` is consumed.

**Interactive vs CLI:**  
- `main()` eagerly `Initialize(cwd)` before metrics  
- Non-interactive Cobra: `PersistentPreRunE` initializes + `PersistentPostRun` `CloseAll`  
- Interactive root skips zap setup but chat boot calls `logging.Initialize(workspace)` again (Once)

---

## 6. Category logger deep dive

### 6.1 `Get(category)`

```
Get(cat)
  if !IsCategoryEnabled(cat) → no-op Logger{category}
  if logsDir == ""           → no-op Logger
  RLock map → hit? return
  Lock → double-check
  open <date>_<cat>.log APPEND
  log.New(file, "", Ldate|Ltime|Lmicroseconds)
  store in map
```

Open failure: stderr warning + no-op logger (never panics).

### 6.2 Levels

| Method | Gated by |
|--------|----------|
| `Debug` | `logLevel <= LevelDebug` |
| `Info` | `logLevel <= LevelInfo` |
| `Warn` | `logLevel <= LevelWarn` |
| `Error` | always if underlying `logger != nil` |

Config levels: `debug`, `info`, `warn`/`warning`, `error`; default `info`.

### 6.3 Text vs JSON

When `json_format: true`, each line is a `StructuredLogEntry`:

```json
{"ts":1710000000123,"cat":"kernel","lvl":"info","msg":"..."}
```

Optional fields: `file`, `line`, `req`, `fields`.  
`StructuredLog(level, msg, fields)` always prefers JSON when enabled; falls back to text with `| fields=...`.

### 6.4 Context and request loggers

- `Logger.WithContext(map)` → `ContextLogger`. Text mode appends `| ctx=%v`; JSON mode emits a `StructuredLogEntry` carrying the context as `fields`.
- `WithRequestID(cat, id)` → `RequestLogger`. Text mode prefixes `[req:<id>]`; JSON mode sets `req` and `fields`. `WithField` still mutates a shared map (not copy-on-write) — single-goroutine ownership assumed.
- Both mirror WARN/ERROR into `<run>_problems.log`.
- In JSON mode `file`/`line` are filled by `callerSite()`, which walks past this package's own frames so the site is the caller, not `logger.go`.

### 6.5 Convenience API

`logger_convenience.go` exposes `Boot`, `BootDebug`, `Kernel`, … plus Warn/Error variants for most categories. These always call `Get(...).Info/Debug/Warn/Error` — safe no-ops when disabled. Browser/Tactile/JIT/Build/Northstar have full Debug/Warn/Error convenience; Regression and Persist have Info/Debug/Warn(/Error).

---

## 7. Timer and performance path

```
t := StartTimer(CategoryKernel, "Query")
// ... work ...
t.Stop()            // Debug on category + logPerformance
t.StopWithInfo()    // Info on category + logPerformance
t.StopWithThreshold(d)  // Warn if slow else Debug + logPerformance
```

`logPerformance`:

1. Skip if source category is already `CategoryPerformance`  
2. Require `CategoryPerformance` enabled  
3. Resolve threshold: explicit `StopWithThreshold` arg, else config map for category, else `"default"`  
4. If not slow: sample with `performance_sampling` (0 or out of range ⇒ 1.0)  
5. Emit `StructuredLog` on performance logger with fields `system`, `operation`, `duration_ms`, optional `threshold_ms`  
6. Also `AuditWithContext("", "", category).PerfMetric(...)`

Uses package-level `math/rand` (seeded once in init).

---

## 8. Audit deep dive

### 8.1 Event model

`AuditEvent` carries correlation (`SessionID`, `RequestID`, `ShardID`), outcome (`Success`, `DurationMs`, `Error`), and `MangleFact` string.

`AuditEventType` constants map to conceptual predicates:

| Family | Examples | Generated predicate |
|--------|----------|---------------------|
| Shard lifecycle | `shard_spawn`… | `shard_lifecycle(...)` |
| Actions | `action_route`… | `action_event(...)` |
| Kernel | `kernel_assert`… | `kernel_op(...)` |
| LLM | `llm_request`… | `llm_call(...)` |
| Files | `file_read`… | `file_op(...)` |
| Intent | `intent_parsed` | `intent_parsed(...)` |
| Safety | `safety_check/block/allow` | `safety_check(...)` |
| Perf | `perf_metric` / `perf_slow` | `perf_metric(...)` |
| Errors | `error_*` | `error_event(...)` |
| Session | `session_start`… | `session_event(...)` |
| Tools | `tool_*` | `tool_exec(...)` |
| Campaign | `campaign_*` | `campaign_event(...)` |
| Learning | `learning_*` | `learning_event(...)` |
| Default | other | `audit_event(...)` |

### 8.2 Write path

```
AuditLogger.Log(event)
  if !IsDebugMode || auditFile == nil → return
  fill defaults from logger scope
  MangleFact = generateMangleFact(event)
  marshal JSON → append line under auditMu
```

`escapeString` escapes `" \ \n \r \t` for Mangle string safety (Builder-optimized).

### 8.3 Init

`InitAudit` no-ops if not debug mode; opens `<date>_audit.log`; writes comment header. Called from `initializeInternal`.

### 8.4 Important honesty note

Mangle facts are **embedded in JSON for offline tooling**. This package does **not** assert them into the live kernel. Any “audit-driven reasoning” requires a separate loader (not in this package).

---

## 9. LLM I/O tracing deep dive

Enabled by `trace_llm_io: true` in the **same** config blob loaded by `loadConfig`.

Lazy init via `sync.Once` on first `IsLLMIOTracingEnabled` / `LogLLM*`:

- Requires `TraceLLMIO && logsDir != ""`
- File: `<run>_llm_io.log`, custom formatted multi-line blocks (not JSON), rotating
- History messages truncated at **2000** chars in request dump; system/user prompts and responses are **full**
- Token estimate heuristic: `len(chars)/4`
- Thread-safe via `llmIO.mu`

Callsite strings (documented examples): `"perception-transducer"`, `"articulation-emitter"`, `"coder-shard"`.

**Privacy:** every system prompt, user prompt, history turn, response, and error string passes through `RedactSecrets` (`redact.go`) before it reaches disk. The patterns are shape-based — credential-ish key names, `Authorization`/bearer, `sk-`/`ghp_`/`AKIA`/`xox` prefixes, PEM blocks, URL credentials — and deliberately duplicate `internal/mcp`'s redactor, because `internal/mcp` imports this package and the dependency can only run one way.

Char/token counts are computed on the **original** text so redaction cannot misreport context usage.

`trace_llm_io_raw: true` disables redaction for the run (answering OPEN-QUESTIONS Q4: redaction is the default because a leaked key in a file that gets pasted into an issue is unrecoverable, while a masked value during JIT debugging is recoverable by one config flag). The boot log states which mode is active.

Enabling the trace at all still dumps code and user content: a deliberate debug tool, not a production default.

---

## 10. Config contract

### 10.1 What this package reads

Local struct `loggingConfig` in `logger.go`:

| JSON field | Type | Effect |
|------------|------|--------|
| `debug_mode` | bool | Master switch |
| `trace_llm_io` | bool | LLM I/O file |
| `categories` | map[string]bool | Per-category filter |
| `level` | string | Min level |
| `format` | string | `json` \| `text` — **canonical**, matches `config.LoggingConfig` |
| `json_format` | bool | Legacy alias for `format: "json"` (OR'd, never ranked) |
| `trace_llm_io_raw` | bool | Disable secret redaction in the LLM I/O trace |
| `performance_sampling` | float64 | Sample rate for non-slow perf |
| `performance_thresholds_ms` | map[string]int64 | Slow thresholds |
| `max_log_file_mb` | int64 | Rotate a segment past this size (0 = 32 MiB default, <0 = never) |
| `max_log_file_minutes` | int64 | Rotate a segment older than this (0 = off) |
| `max_rotated_files` | int | Archived segments kept per file (0 = 3 default, <0 = none) |

Path: **only** `<workspace>/.nerd/config.json` (not `config.yaml`).

### 10.2 Mirror in `internal/config`

`config.LoggingConfig` has `Level`, `Format`, `File`, `DebugMode`, `TraceLLMIO`, `Categories`, `PerformanceSampling`, `PerformanceThresholdsMs`.

**Resolved divergence:**

| Concern | `config.LoggingConfig` | `logging.loggingConfig` |
|---------|------------------------|-------------------------|
| Structure format | `format` string (`json`/`text`) | `format` (canonical) **and** `json_format` (legacy alias) |
| Legacy single file | `file` | unused — this package writes one file per category |
| Mutual import | n/a | intentionally none (`config` imports `logging`) |

Either spelling turns structured output on; `IsJSONFormat()` ORs them, so neither loader can silently disable what the other enabled. `config_schema_test.go` parses a literal `config.LoggingConfig`-shaped JSON blob and asserts every key lands, so a rename on that side fails here.

**Avoiding the double parse:** `ApplyConfig(logging.Config{...})` installs an already-parsed config and pins it against disk reads; `ClearInjectedConfig()` releases the pin. Boot (`internal/config`) can call it because the import direction allows it — the wiring is not yet in place, so today both sides still parse `.nerd/config.json` independently. That is the same file, so they cannot disagree about content, only about schema — which the alias above closes.

### 10.3 Reload

`ReloadConfig()` re-reads disk into process config. **Does not** recreate open loggers, re-open audit, or re-run LLM I/O once. Category enablement checks re-read config on each `Get`/`IsCategoryEnabled`.

---

## 11. Integration map (consumers)

High-fan-in package. Representative import sites (non-exhaustive):

| Area | Examples |
|------|----------|
| CLI | `cmd/nerd/main.go` (Initialize/CloseAll), `cmd_browser.go`, `cmd_advanced.go`, `cmd_mangle_lsp.go`, `cmd_init_scan.go` |
| Chat | `session_boot.go`, `session_shared_boot.go`, `process.go`, `delegation*.go`, `campaign.go`, wizards |
| Core runtime peers | `internal/mangle`, `internal/world`, `internal/init`, `internal/embedding` |
| Agent systems | `internal/autopoiesis`, `internal/campaign`, `internal/context`, `internal/articulation` |
| Browser / build | `internal/browser`, `internal/build` |
| Config | `internal/config/user_config.go` (boot feature summary log) |

**Does not import:** kernel types, VirtualStore, shards manager — one-way dependency (others → logging).

---

## 12. Concurrency model

| Mutex | Protects |
|-------|----------|
| `loggersMu` | `loggers` map and file open |
| `configMu` | `config`, `configLoaded` (RW) |
| `auditMu` | `auditFile` writes |
| `llmIO.mu` | LLM I/O file writes |
| `initOnce` / `llmIOOnce` | One-shot init (both rearmable on workspace rebind) |
| `initMu` | Serializes `Initialize`, including rebind |
| `rotatingFile.mu` | One sink's file handle, size, and rotation |

`Logger` methods assume single `*log.Logger` writer; standard library logger is safe for concurrent use. Category enablement races with `ReloadConfig` are bounded by RWMutex.

---

## 13. Shutdown

| Function | Closes |
|----------|--------|
| `CloseAll()` | **Everything**: category loggers, `<run>_problems.log`, audit, LLM I/O |
| `CloseAudit()` | Audit file only |
| `CloseLLMIOLogger()` | LLM I/O file only |

`CloseAll` previously closed only the category loggers, so the CLI's `PersistentPostRun` leaked two of three sinks — including the audit log, whose open handle also blocked the next run's fresh-run cleanup from reclaiming the name on Windows. All closers are idempotent and safe to call after shutdown; a `Logger` handle captured before close degrades to a no-op instead of panicking.

---

## 14. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) for prioritized matrix. Headline gaps:

1. ~~Split shutdown~~ — `CloseAll` now closes all four sinks  
2. ~~Config schema drift~~ — `format` canonical, `json_format` aliased, pinned by test  
3. Audit facts not kernel-ingested — **intentional**; `nerd audit facts` exports an offline `.mg` instead  
4. ~~`ContextLogger`/`RequestLogger` ignore JSON~~ — both emit structured entries  
5. ~~No secret redaction for LLM I/O~~ — redaction on by default, `trace_llm_io_raw` opts out  
6. ~~`sync.Once` blocks workspace rebind~~ — rebind implemented  
7. Remaining: `internal/config` does not yet call `ApplyConfig`, so the config file is parsed twice; `ConstitutionGateShard.CheckAction` (internal/shards/system) still decides allow/deny without an audit event (tracked by `safety_callsite_audit_test.go`)  

---

## 15. Verify commands

```powershell
go test ./internal/logging/...
go test -count=1 ./internal/logging/...
go test -race ./internal/logging/...
go test -bench=EscapeString ./internal/logging/
```

---

## 16. Non-goals of this package

- Replacing zap console logging for CLI operators  
- Live TUI glass-box event stream  
- Metrics counters / flight recorder (`internal/observability`)  
- Policy enforcement (`permitted`, default deny)  
- Long-term log shipping / SIEM integrations  

Those belong to other packages; this corpus must not claim them as logging features.
