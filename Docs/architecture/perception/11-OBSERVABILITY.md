# 11 — Observability (perception)

> Last verified: **2026-07-13**

## Logging category

Primary category: `logging.CategoryPerception`.

Helpers used throughout:

- `logging.Perception(...)`  
- `logging.PerceptionDebug(...)`  
- `logging.PerceptionError(...)` / `logging.Get(...).Warn`  
- `logging.StartTimer(CategoryPerception, name)` for spans  

### High-signal log events

| Event | Source | Fields of interest |
|-------|--------|--------------------|
| `[ParseIntentWithContext] START/COMPLETE` | `understanding_adapter.go` | input preview, timings, verb, confidence |
| Semantic match top-k | adapter | sim, verb, text preview |
| `[Understand] START` | `transducer_llm.go` | client type, thinking, hasKernel |
| Prompt composition | `transducer_llm.go` | token estimates, breakdown lengths |
| LLM failed / responded | `transducer_llm.go` | duration, response len, thinking tokens |
| Routing derived | `transducer_llm.go` | mode, shards, override vs suggestion |
| SemanticClassifier create/classify | `semantic_classifier.go` | TopK, store presence |
| DetectProvider / factory | `client_factory.go` | engine, provider |
| ConsolidationWorker | `consolidation.go` | enqueue/drop/shutdown |
| Client retries | provider files | 503 / backoff |

## Metrics

`metrics.go`:

```go
type LLMMetrics struct {
  Calls, TokensUsed, DurationMs, Errors int64
}
// keyed by shardCategory + ":" + shardType
RecordLLMCall(...)
GetLLMMetrics() map[string]LLMMetrics
```

Updated by tracing client path after LLM calls. Process-local; not automatically exported to Prometheus.

## Traces

`ReasoningTrace` captures full system/user prompts, response, model, tokens, duration, shard attribution, success/error, quality score hooks.

Storage is pluggable via `TraceStore`. Downstream learning uses these traces.

## Debug hooks

| Hook | Use |
|------|-----|
| `lastUnderstanding` cache on transducer | GAP-018 debugging |
| `GetCurrentContext` on TracingLLMClient | shard attribution checks |
| `PerceptionDebug` raw LLM response | format diagnosis (may be large) |
| Codex/Claude probes | auth health for `nerd auth` |
| `debug.go` | package debug utilities |

## Bottleneck labels

`identifyBottleneck` emits human labels:

- `LLM_API(Nms)`  
- `PROMPT_BUILD(Nms)`  
- `PARSE_ROUTE(Nms)`  

Use these when classifying “why is chat slow?” — classification LLM vs prompt bloat vs parse.

## What is not instrumented (honest)

- No OpenTelemetry spans in-package by default.  
- Provider HTTP status histograms not centralized.  
- Semantic search latency is timer-logged but not in `LLMMetrics`.  
- Queue drop counts for ConsolidationWorker are log-only.

## Operator playbook

| Symptom | Where to look |
|---------|----------------|
| Every turn slow | Understand COMPLETE bottleneck; classification model |
| Always /explain | LLM FAILED logs; TransientFailure; parse FAILED previews |
| No semantic matches | SharedSemanticClassifier=nil; embed engine warn |
| Auth fails | Claude/Codex/xai probe logs; engine config |
| Learning never sticks | Consolidation queue full warns; learned_taxonomy path |
| Campaign serialization | Confirm clients use `NewSharedHTTPClient` |

## Related

CLI transparency / glass box docs: `Docs/architecture/cli/12-TELEMETRY-OBSERVABILITY.md`.  
Logging package: `Docs/architecture/logging/`.
