# 11 — Observability

## Signals that exist

| Signal | Current contract |
|---|---|
| YAML load | **VERIFIED CURRENT** — boot log records load/missing/error and provider/model, not API key, in `internal/config/config.go#Load`. |
| feature activation | **VERIFIED CURRENT** — successful present JSON logs `features.Summary` in `internal/config/user_config.go#LoadUserConfig`. |
| logging projection | **VERIFIED CURRENT** — `internal/logging/logger.go#loadConfig` independently decodes only `logging` and `Initialize` applies it once. |
| full LLM trace | **PARTIAL** — defaults-off is tested by `internal/config/config_security_test.go#TestSensitiveTracingDefaultsOff`, but `internal/logging/llm_io_logger.go#LogLLMRequest` and `#LogLLMResponse` persist raw prompts/history/responses without redaction or bounded response size when enabled. |
| JIT shard trace | **PARTIAL** — `internal/shards/system/legislator.go#llmClientAdapter.CompleteWithSystem` and `internal/shards/system/mangle_repair.go#MangleRepairShard.ValidateAndRepair` put raw prompts/responses in structured logs when effective JIT trace is true. |

## Diagnostic blind spots

- No immutable config snapshot ID or field-origin record.
- No distinction in downstream logs between absent, malformed, invalid, and
  soft-ignored config.
- No snapshot/projection receipt proving which execution values every surface
  accepted; campaigns still diverge from shared Cortex.
- No record correlating a wizard save with disk and active feature/JIT state.
- No redaction policy identifier, retention bound, rotation guarantee, or
  operator-facing list of active raw trace sinks.
- `logging.Initialize` is one-shot, so later saves do not change the active sink.

## Secret and privacy boundary

**PARTIAL.** Config logs do not deliberately dump provider keys, and raw trace
now defaults false with a `0600` file request. When explicitly enabled, prompts,
history, responses and error bodies can still contain secrets; no content
redaction, response bound, rotation/retention guarantee or cross-platform ACL
test is proven.

**PROPOSED UPLIFT.** Trace defaults false, opt-in is explicit and visible, values
pass structural and content redaction, previews and files are bounded, retention
is declared, modes are owner-only, and every event carries snapshot/projection
correlation without raw secret-bearing config.

## Operator diagnosis target

A safe boot receipt should answer: workspace identity digest, file state, schema
and migration, validation result, effective provider/engine names, projection
IDs, defaults/env origin classes, and which consumers accepted the snapshot. It
must not answer with keys, tokens, raw config, raw prompts, or full paths outside
the configured disclosure policy.
