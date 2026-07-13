# 11 — Observability: config

> Last verified: 2026-07-13  

## 1. What config logs

| Event | API | Level/category |
|-------|-----|----------------|
| YAML Load start | `logging.BootDebug` | boot debug |
| Missing YAML | `logging.Boot` | boot |
| YAML read/parse errors | `logging.BootError` | boot error |
| YAML loaded provider/model | `logging.Boot` | boot |
| UserConfig features install | `logging.Get(CategoryBoot).Info` with `features.Summary()` | boot |

Config package itself is **not** chatty on Get* paths.

## 2. LoggingConfig (runtime for whole process)

Configured via UserConfig.Logging / YAML Logging:

| Field | Meaning |
|-------|---------|
| `Level` | debug/info/warn/error (semantic for logging subsystem) |
| `Format` | json/text |
| `File` | legacy single file path |
| `DebugMode` | master switch — false ⇒ `IsCategoryEnabled` always false |
| `TraceLLMIO` | dump full LLM I/O (also mirrored on JITConfig) |
| `Categories` | per-category toggles when debug on |
| `PerformanceSampling` | 0.0–1.0 for non-slow perf logs |
| `PerformanceThresholdsMs` | per-system slow thresholds |

`IsCategoryEnabled(category)`:

```
if !DebugMode → false
if Categories nil → true
if category missing → true
else → map value
```

## 3. JIT / LLM I/O traces

| Flag | Location | Effect (consumers) |
|------|----------|--------------------|
| `jit.trace_llm_io` | JITConfig | Full JIT prompts / LLM I/O when consumers honor it |
| `logging.trace_llm_io` | LoggingConfig | Same class of dump for logging subsystem |
| `jit.debug_mode` | JITConfig | Verbose JIT assembly logs |

Defaults in DefaultJITConfig / DefaultUserConfig seed often set TraceLLMIO **true** — operators on disk may override.

## 4. Transparency / glass box (config-owned knobs)

`TransparencyConfig` fields consumed by CLI/transparency packages:

- ShardPhases, StreamReasoning, SafetyExplanations  
- JITExplain, OperationSummaries, VerboseErrors  
- GlassBoxEnabled / GlassBoxDisabled / GlassBoxCategories / GlassBoxVerbose  

These do not log from config; they **gate** UI overlays.

## 5. Metrics

No Prometheus/metrics export in this package. Performance thresholds in LoggingConfig are **data** for observability systems elsewhere.

## 6. Debug tips

```powershell
# After loading config, boot log should include features summary
# Enable category logging in .nerd/config.json:
# "logging": { "debug_mode": true, "categories": { "boot": true, "kernel": true } }

# Trace JIT I/O
# "jit": { "debug_mode": true, "trace_llm_io": true }
```

## 7. What not to log

API keys, OAuth refresh tokens, full config.json dumps with secrets. Prefer provider name + model only (as YAML Load already does).
