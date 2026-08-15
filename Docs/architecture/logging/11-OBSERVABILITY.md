# 11 — Observability (`internal/logging`)

> Last verified: **2026-07-13**  
> This package **is** an observability implementation. This doc maps its surfaces for operators and developers.

## 1. Operator enablement

### Minimal debug

```json
{
  "logging": {
    "debug_mode": true,
    "level": "info"
  }
}
```

Creates `.nerd/logs/`, enables all categories (map nil ⇒ all on), level info+.

### Focused investigation

```json
{
  "logging": {
    "debug_mode": true,
    "level": "debug",
    "categories": {
      "kernel": true,
      "session": true,
      "performance": true,
      "api": false,
      "world": false
    },
    "performance_sampling": 0.2,
    "performance_thresholds_ms": {
      "default": 100,
      "kernel": 25
    }
  }
}
```

### Prompt quality debugging

```json
{
  "logging": {
    "debug_mode": true,
    "trace_llm_io": true,
    "level": "debug",
    "categories": {
      "perception": true,
      "articulation": true,
      "api": true,
      "jit": true
    }
  }
}
```

**Warning:** disk may grow quickly (rotation bounds each segment, it does not make
the trace small), and prompts may contain secrets — they are redacted by default,
and `trace_llm_io_raw: true` removes that protection.

### Structured machine parse

```json
{
  "logging": {
    "debug_mode": true,
    "format": "json",
    "level": "info"
  }
}
```

## 2. Category catalog (observability map)

| Category | What to look for |
|----------|------------------|
| `boot` | Init order, config load, feature summary |
| `session` | Session/turn lifecycle |
| `kernel` | Assert/query/derive timing and errors |
| `api` | Provider HTTP failures, rate limits |
| `perception` / `articulation` | Transduction quality |
| `routing` / `virtual_store` / `tools` | Action execution path |
| `shards` + role categories | Parallel specialist work |
| `campaign` | Long-horizon assault phases |
| `context` | Compression / activation |
| `world` | AST / filesystem scan |
| `embedding` / `store` | Retrieval substrate |
| `browser` / `tactile` | External effectors |
| `jit` | Prompt compiler |
| `autopoiesis` / `dream` | Self-mod / simulation |
| `performance` | Cross-system duration metrics |
| `northstar` | Vision guardian |

## 3. Audit log as structured timeline

`YYYY-MM-DD_audit.log` is JSONL (after a comment header). Each line includes:

- `event`, `ts`, `session`, `shard`, `success`, `dur_ms`, `mangle`, …

**Offline analysis ideas:**

- Filter `event == "safety_block"`  
- Sort by `dur_ms` for slow actions  
- Group by `shard` for lifecycle completeness (spawn → complete)

The `mangle` field is a ready-made fact string for offline Mangle programs.

## 4. LLM I/O log format

Human-readable sections:

```
═══ LLM REQUEST [<callsite>] @ HH:MM:SS.mmm ═══
MODEL: ...
TEMPERATURE: ...
SYSTEM PROMPT (N chars, ~T tokens):
─── BEGIN SYSTEM PROMPT ───
...
─── END SYSTEM PROMPT ───
...
═══ LLM RESPONSE [<callsite>] @ ... (Nms) ═══
```

Use callsite strings to bind to subsystem.

## 5. Performance telemetry

| Mechanism | Output |
|-----------|--------|
| Category Debug/Info duration lines | Source category file |
| Sampled/threshold StructuredLog | `performance` file |
| Audit PerfMetric / PerfSlow | audit file |

Slow (above threshold) always logged (subject to level); non-slow sampled.

## 6. Correlation fields

| Mechanism | Field |
|-----------|-------|
| `WithRequestID` | `[req:...]` text prefix |
| Audit | `session`, `req`, `shard` JSON fields |
| StructuredLogEntry | `req` optional (not always filled by default paths) |

## 7. Relationship to other observability

| System | When to use |
|--------|-------------|
| **logging** (this) | Reconstruct subsystem history from files |
| **zap CLI** | Immediate stderr for scripts |
| **glass box / transparency** | Live TUI operator view |
| **observability metrics** | Counters/gauges/startup GC |

See also `Docs/architecture/cli/12-TELEMETRY-OBSERVABILITY.md`.

## 8. Debug playbooks

### Kernel slow

1. Enable `kernel` + `performance`, threshold 25ms  
2. Run failing command  
3. Grep `performance` log for `kernel.` ops; check audit `perf_slow`

### Bad articulation

1. Enable `articulation`, `api`, `trace_llm_io`  
2. Capture one turn  
3. Inspect `llm_io` system prompt vs expectation; cross-check JIT category

### Campaign abort

1. Enable `campaign`, `shards`, audit  
2. Find last `campaign_event` / `shard_complete` with `success:false`

## 9. Metrics this package does **not** emit

- Prometheus counters  
- OpenTelemetry spans  
- Remote shippers  

Those would be separate integrations.
