# 11 — Observability: JIT config

> Last verified against codebase: **2026-07-13**

## 1. Package-local telemetry

**None.** `internal/jit/config` does not log, metric, or trace. Observability is entirely **consumer-side**.

## 2. Related log categories

| Category | Where | JIT-config relevance |
|----------|-------|----------------------|
| `logging.CategoryJIT` | `internal/prompt/compiler.go` | Warn/debug on agent config generation failure/success during compile |
| `logging.CategorySession` | `internal/session/*` | Config compiled tool counts, specialist load, spawn, tool allowlist events |
| Boot logs | `cmd/nerd/chat/session_boot.go` | Factory wiring during Cortex boot |

Representative messages (paraphrased from code):

- `"Generated JIT Agent Config for intents: %v"` (JIT debug)  
- `"Failed to generate agent config: %v"` (JIT warn)  
- `"Config compiled: %d tools allowed"` (Session)  
- `"Loading specialist config for: %s from %s"` (Session debug)  
- `"Successfully loaded specialist config for %s"` (Session debug)  
- `"Specialist config not found… falling back to JIT generation"` (Session debug)  
- `"JIT compilation failed, retrying with baseline"` (Session warn)  
- `"Config compilation failed: %v"` then continue with empty config (Session warn)

## 3. Flight recorder / glass box

Prompt compilation may attach manifests/stats on `CompilationResult` (prompt package). Agent config attachment is optional and not a separate glass-box channel.

CLI UI may expose JIT status pages (`cmd/nerd/ui` / slash JIT status — see CLI corpus). Those surface compiler health more than the raw schema struct.

## 4. Metrics (current)

No dedicated Prometheus/OTel counters in `internal/jit`. Useful counters **if added** (consumers, not schema):

| Metric idea | Labels |
|-------------|--------|
| `jit_config_generate_total` | intent, result=ok/err |
| `jit_config_validate_fail_total` | reason=identity/empty_policy/noncanonical_policy/duplicate_policy |
| `jit_config_specialist_yaml_total` | result=ok/missing/invalid |
| `jit_config_empty_fallback_total` | path=executor/spawner |
| `jit_tools_allowed_count` | intent |

## 5. Debug practices

1. Log `IntentVerb`, tool/policy counts, and a bounded set/version identity when
   config is accepted; avoid dumping an arbitrary external policy list.
2. On empty fallback, log at **Warn** with reason (already partially done).  
3. Diff factory output vs YAML specialist for mis-tagged fields.  
4. When tools silently missing, check registry name mismatch vs `AllowedTools`.  
5. Use `go test` on factory Validate suite after atom edits.
6. Run core-inventory and prompt-provider parity tests after policy registry edits.

## 6. Gaps

| Gap | Impact |
|-----|--------|
| No structured event for Validate skip | Operators cannot count “unvalidated agents” |
| Empty config looks like success in higher layers | False confidence |
| ToolLoop ignored without log | YAML knobs appear to work |
