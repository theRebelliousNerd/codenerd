# 11 — Observability: articulation

> Last verified: 2026-07-13

## Log category

| Symbol | Value | Meaning |
|--------|-------|---------|
| `logging.CategoryArticulation` | `"articulation"` | Atoms↔NL / Piggyback |

Convenience helpers (`internal/logging/logger_convenience.go`):

- `logging.Articulation` → Info  
- `logging.ArticulationDebug` → Debug  
- `logging.ArticulationWarn` / `ArticulationError`  

## Timed operations

`logging.StartTimer(CategoryArticulation, name)` wraps:

| Timer name | Location |
|------------|----------|
| `Process` | ResponseProcessor.Process |
| `parseJSON` | parseJSON |
| `parseMarkdownWrappedJSON` | markdown path |
| `extractEmbeddedJSON` | embedded path |
| `Emit` / `MarshalEnvelope` | Emitter |
| `ExtractSurfaceOnly` | helper |
| `AssembleSystemPrompt` | PromptAssembler |

## Operational log patterns

| Event | Level | Example intent |
|-------|-------|----------------|
| Process start | Info | length, attempt counter |
| Direct JSON success | Info | surface length, control counts |
| Self-correction | Info | hypothesis |
| Markdown / embedded success | Info / Warn | mixed content warn |
| Fallback parse | **Error** (default) | errors list + 300-char preview |
| Fallback empty / quiet | Warn / Debug | AllowPlain mode |
| Strict failure | Error | all attempts failed |
| JIT success | Info | bytes, atoms, budget % |
| JIT failure | Warn | fall back to legacy |
| Constitutional safety | Warn | reason string |
| Mangle filter skips | via Warnings on result | not always Error |

## In-process metrics

`ProcessorStats` on each `ResponseProcessor`:

- `TotalProcessed`  
- `SuccessfulParses`  
- `FallbackParses`  
- `ValidationFailures`  
- `SelfCorrections`  

Access: `GetStats()` / `ResetStats()`. **Not** automatically exported to Prometheus/glass-box as of verification date — operators rely on logs unless a caller scrapes stats.

## Result-carried diagnostics

`ArticulationResult.Warnings` strings for:

- Self-correction  
- Mixed content extraction  
- Truncations (surface, mangle list, memory, tools, knowledge, reasoning)  
- Invalid mangle skips  
- Partial envelope salvage / truncation messages  

Chat `ArticulationOutput.Warnings` propagates these.

## JIT telemetry fact

On JIT compile failure, assembler may assert:

```text
jit_fallback(/shardType, "reason...").
```

via `compiler.AssertFacts` (best-effort).

## Debugging playbook

1. Reproduce with articulation category at Debug.  
2. Inspect fallback Error previews (first 300 chars).  
3. Check `ParseMethod` and `Confidence` on the result object.  
4. For stream issues, dump `StreamParser.GetFullBuffer()` after completion.  
5. For prompt issues, log assembled prompt length and whether JIT or legacy path ran.  
6. If mangle asserts fail downstream, inspect applyCaps warnings and session blocked list.

## Gaps

- No package-level global counters.  
- No standard glass-box event type for “piggyback_fallback”.  
- Processor stats are per-instance; chat creates new processors frequently → stats rarely aggregated.
