# 11 — Observability: session

> Last verified: 2026-07-13

## 1. Logging category

| Symbol | Value | File |
|--------|-------|------|
| `logging.CategorySession` | `"session"` | `internal/logging/logger.go` |

Convenience helpers:

| Helper | Level |
|--------|-------|
| `logging.Session` | Info |
| `logging.SessionDebug` | Debug |
| CategorySession `.Warn` / `.Error` | Warn / Error |

## 2. High-value log points

| Event | Approx location | Level |
|-------|-----------------|-------|
| New Executor / Spawner / SubAgent | constructors | Info/Debug |
| Process start (char count, preset flag) | ProcessWithIntent | Info |
| JIT / config failures | compile paths | Warn |
| Tool defs count / Piggyback catalog size | generateResponse* | Info/Debug |
| No-tool retry triggered | runToolLoop | Warn |
| Base/hard tool budget and extension policy | factory boot | Info |
| Extension granted/refused with reason | runToolLoop | Warn |
| Final effective tool iteration budget | runToolLoop | Warn |
| Tool failure | execute paths | Error |
| Safety deny / fail closed | checkSafety | Warn/Error |
| safe_action fallback allow | checkSafety | Warn |
| Executive gate block | executeToolCall | Warn |
| Piggyback control detected | processPiggybackControlPacket | Info |
| Self-correction / memory ops | same | Info/Debug |
| Blocked mangle_updates | processMangleUpdates | Debug |
| SubAgent start/complete/fail | Run | Info/Error |
| Memory compression | CompressMemory | Debug/Info |
| Persist success/fail | persistTurn | Debug/Warn |
| WaitForResult stop on cancel | JITExecutor | Info/Debug |
| JITExecutor wired | system factory | Info |

## 3. Metrics surface

`SubAgentMetrics` / `Spawner.GetMetrics()` expose ID, name, type, state, turn count, duration.

There is **no** dedicated Prometheus/OpenTelemetry export inside session; higher layers may scrape logs or call GetMetrics.

`ExecutionResult.Duration` and `ToolCallsExecuted` are per-call metrics for callers.

## 4. Flight recorder / glass box

Session does not own CLI glass-box events. Chat/CLI layers may wrap TaskExecutor/Process to emit operator-visible stages. When adding long-running session behavior, prefer:

1. CategorySession structured logs  
2. Optional hooks in chat glass box (outside this package)

## 5. Debug techniques

| Symptom | Look for |
|---------|----------|
| Tools never run | no-tool retry warnings; AllowedTools empty; intent_requires_tool_call |
| Tools always denied | Safety check denied; nil kernel Error; payload too large |
| Wrong persona | preset_intent vs observe; IntentVerb maps |
| History contamination | missing CloneForTask (caller bug) |
| Hang | ToolTimeout; Wait polling; max iterations |
| Spawn refused | max active subagents |

## 6. Gaps

- No span/trace IDs threading Process → tool → gate.  
- Persist is fire-and-forget; failures only logged.  
- Piggyback confidence/parse method logged but not aggregated.

## 7. Guidance for new code

Log start/end of Process with duration (already partially done). Never log full API keys or entire multi-MB payloads; log lengths instead (already pattern for input chars / payload_len).
