# system — Observability

> Last verified: **2026-07-13**

## 1. Logging approach

Package uses `codenerd/internal/logging` category loggers. Boot also prints **stderr warnings** for some soft failures (`fmt.Fprintf(os.Stderr, "Warning: ...")`) when logging may not yet be ideal for operators.

Logging is initialized in `initCoreComponents` via `logging.Initialize(workspace)`.

## 2. Categories used

| Category | Example events |
|----------|----------------|
| `CategorySession` | Maintenance start/stop/skip; modular tools hydrate fail; JITExecutor wired |
| `CategoryStore` | Maintenance cycle fail / summary (archived/purged/logs) |
| `CategoryPerception` | LLM from config/engine; legacy Z.AI key; worker LLM; image LLM |
| `CategoryContext` | Missing LLM error; hybrid ingest; corpus materialize/open; agent JIT register; AssertBatch unknown constant |
| `CategoryEmbedding` | Health check failed |
| `CategoryTools` | MCP bridge init/wire/connect |
| `CategoryWorld` | Deep scan failed (holographic) |

## 3. Boot signal map (operator view)

Useful greps after a failed/successful boot:

```
LLM client from config
Worker LLM enabled
Image LLM enabled
no LLM client configured
Failed to init JIT compiler
failed to boot cortex kernel
failed to start system shards
Registered N user-defined agents
JITExecutor wired in BootCortex
Maintenance schedule started
```

TUI adds its own `[boot]` step lines in `session_shared_boot.go` (outside this package) with elapsed seconds.

## 4. Metrics

**No formal metrics/Prometheus surface** in `internal/system`. Quantitative signals today:

- Maintenance stats via log line (`FactsArchived`, `FactsPurged`, `ActivationLogsDeleted`)  
- Hybrid prompt count ingested  
- User agent count registered  

Usage accounting is delegated to `usage.Tracker` on Cortex (package initializes tracker; does not define metrics schema).

## 5. Debug hooks

| Hook | Location | Purpose |
|------|----------|---------|
| `BootConfig` overrides | factory | Isolate subsystems in tests |
| `DisableSystemShards` | factory | Normalized set controls cache identity and shard start |
| JIT `DebugMode` | from UserConfig JIT | Passed into compiler config |
| `debug_program_ERROR.mg` | package tree artifact | Kernel crash dump of combined .mg (not a deliberate hook) |
| Cortex field inspection | after boot | All major deps public fields |

## 6. Gaps

- No structured boot span / timing table inside system (TUI only)  
- Maintenance start/stop is logged, but it is not correlated to a Cortex identity or Close receipt
- Cache hit vs miss not logged (would help diagnose Bug #15 class issues)  
- No stage acquisition/degradation/rollback/close receipt
- MCP bridge/cancel/done are retained and closed, but no structured receipt
  reports the connection and cleanup outcome
- API key is never logged (required); any future identity fingerprint must remain one-way and bounded

## 7. Proposed bounded operator receipt

A receipt should contain a random boot correlation ID, canonical identity hash
prefix, code/config/policy/prompt fingerprints, ordered stage IDs and durations,
required/degraded/skipped outcomes, owned resource IDs, rollback/close outcomes,
and bounded error classes. It must never contain API keys, prompt bodies, user
file contents, arbitrary payload values, or an executable action.

Cache hit/miss, VirtualStore action ID, and Close should share correlation where
possible. Persistence must be size-capped with explicit retention. The receipt
proposal is `system-boot-receipt-registry-v1` in [TODO.md](TODO.md).
