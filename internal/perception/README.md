# internal/perception/

Natural Language Understanding and Multi-Provider LLM Client System.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Overview

The Perception layer sits at the entry point of the system, converting natural language into structured `Intent` objects via the Piggyback Protocol, plus providing multi-provider LLM client support.

## Architecture

```
User Input → Transducer → Intent{Verb, Target, Category}
                ↓
         Piggyback Protocol
                ↓
    ┌──────────┴──────────┐
    ↓                     ↓
surface_response    control_packet
(for user)          (for kernel)
```

## Structure

```
perception/
├── transducer.go           # NL → Intent conversion
├── taxonomy.go             # Verb taxonomy engine
├── client.go               # LLM client modularization marker
├── client_types.go         # LLMClient interface
├── client_schema.go        # Provider-specific JSON schemas
├── client_zai.go           # Z.AI client
├── client_anthropic.go     # Anthropic/Claude client
├── client_openai.go        # OpenAI/Codex client
├── client_gemini.go        # Google Gemini client
├── client_xai.go           # xAI/Grok client
├── client_openrouter.go    # OpenRouter multi-model
├── claude_cli_client.go    # Claude CLI subprocess
├── codex_cli_client.go     # Codex exec subprocess backend
└── codex_exec_client.go    # Explicit codex exec client alias
```

## LLM Providers

| Provider | Config Key | Default Model |
|----------|------------|---------------|
| Z.AI | `zai_api_key` | `glm-4.7` |
| Anthropic | `anthropic_api_key` | Explicit `model` required |
| OpenAI | `openai_api_key` | `gpt-5.1-codex-max` |
| Gemini | `gemini_api_key` | `gemini-3-pro-preview` |
| xAI | `xai_api_key` | `grok-3-beta` |
| OpenRouter | `openrouter_api_key` | (various) |
| Claude CLI | `engine: claude-cli` | (CLI default) |
| Codex CLI | `engine: codex-cli` | (CLI default) |

## API key pools

Configure `api_key_pools` in `.nerd/config.json` to balance multiple credentials
for the same API provider. Existing single-key configurations continue to work.
An explicitly configured pool replaces that provider's scalar key.

```json
{
  "engine": "api",
  "provider": "anthropic",
  "model": "claude-opus-5-5",
  "classification_model": "claude-haiku-5-5",
  "api_key_pools": {
    "anthropic": [
      { "name": "primary", "env": "ANTHROPIC_KEY_A" },
      { "name": "secondary", "env": "ANTHROPIC_KEY_B" }
    ]
  }
}
```

Export both referenced environment variables before starting codeNERD. Each
entry needs a unique `name` and exactly one of `env` or `api_key`. Environment
references keep credentials out of the workspace file. Missing references,
empty pools, duplicate credentials, and invalid entries fail explicitly.
Anthropic entries may also specify `workspace_id`; otherwise they inherit the
slot's Anthropic workspace setting. Pools accept 1-64 credentials.

Supported providers are `anthropic`, `openai`, `gemini`, `xai`, `zai`,
`openrouter`, `dashscope`, `meta`, and `moonshot`. Pools apply to the LLM
completion interfaces: text, schema output, native tool calls/results, and
streaming (including separate thought streams when supported). CLI/OAuth
engines and keyless Ollama keep their existing authentication paths. Provider
file uploads and image-generation helpers are separate surfaces, outside this
completion pool.

Main, worker, planner, and classification clients use the shared pool for their
provider. Selection prefers the least busy healthy credential and rotates ties.
Each credential has its own client and broker, so admission/token counting and
inference use the same key and workspace, including after failover. Counts may
be memoized within that credential's counter. Changing a referenced environment
key changes the Cortex configuration identity instead of retaining stale
clients. Restart the process when changing its environment.

A failed call can try each credential at most once. An authentication failure
(401) disables a credential for this process/pool configuration. A 403 is a
denial of that request (a guardrail or moderation block, a model, resource or
region the key may not use): the call fails, no other credential is tried, and
the credential stays healthy for later requests. Rate limits
and transient server failures apply shared cooldowns, honoring `Retry-After`.
A canceled caller stops waiting; when all remaining credentials are cooling,
selection waits within the caller's deadline (or a ten-minute call ceiling).
Ordinary request errors such as 400 do not rotate credentials. A streaming
failure can fail over only before any text or thoughts have been emitted.
Provider-side cached-content handles pin that client to its current credential
until the handle is cleared, because those resources can belong to one account.

Credential health/load are shared in-process across clients with the same
provider and credential set. They are not a distributed rate limiter between
separate codeNERD processes. The existing global API scheduler remains the
concurrency ceiling; adding keys does not automatically raise it. Multiple keys
can also share one provider quota: Anthropic enforces organization-level limits
and optional workspace limits, so additional keys in the same organization do
not multiply its quota. See [Anthropic rate limits](https://platform.claude.com/docs/en/api/rate-limits).

Broker receipts contain `credential_alias`, and usage JSON contains
`by_credential` totals such as `anthropic/primary`. These records use aliases,
not raw keys. Retained usage events also carry the alias when event logging is
on. Existing provider/model/session totals remain intact.

## Direct Anthropic API

`provider: "anthropic"` uses the Messages REST API with an API key, with API
version `2026-10-21`. Set `ANTHROPIC_API_KEY` or `anthropic_api_key` and explicitly
choose a model. The factory preserves your model; it never selects a newer one
on your behalf. For example:

```json
{
  "engine": "api",
  "provider": "anthropic",
  "model": "claude-opus-5-5",
  "classification_model": "claude-haiku-5-5",
  "reasoning_effort": "high",
  "max_output_tokens": 16384,
  "anthropic": {
    "thinking": "adaptive"
  }
}
```

`reasoning_effort` accepts `low`, `medium`, `high`, `xhigh`, and `max`; availability
varies by model. Without an override, supported models map codeNERD's speed,
balanced and reasoning capability hints to `low`, `medium` and `high`. Unhinted
calls preserve the model default. Classification uses `low` when supported and
retains its small completion budget and system-prompt cache.

Optional `anthropic` settings are `workspace_id` (for a multi-workspace key),
`thinking` (`adaptive`, `enabled`, `disabled`, or `between_tools`),
`thinking_budget_tokens` (legacy `enabled` mode only, at least 1024 and below
`max_output_tokens`), `thinking_display` (`summarized` or `omitted`, alongside
`adaptive` or `enabled`), and `strict_tools`. Empty thinking settings use the
model's defaults. Current models differ in whether thinking can be disabled;
the endpoint rejects unsupported model/settings combinations. The client never
silently changes those settings. `temperature`, `top_p`, and `top_k` are omitted
because this API version removes them.

Workers and planners support their own `reasoning_effort`, `max_output_tokens`,
`endpoint` and `anthropic` block. An omitted slot `anthropic` block inherits the
shared block; a supplied block replaces it. `base_url` overrides the main API
base (including `/v1`), and inference and broker token counting use the same
workspace and API version.

Native tools execute through codeNERD's governed executor. Tool history keeps
ordered text, tool IDs/results, and signed or redacted thinking. Server tools,
provider compaction, and provider-managed agent execution require separate
harness integration and are not enabled. Unknown response blocks and unsupported
stop reasons fail explicitly.

`CompleteWithSchema` uses `output_config.format` and validates the returned JSON
against the original schema locally. Numeric/length constraints unsupported by
the provider grammar become prompt guidance on the wire and remain mandatory
in local validation. Objects with declared properties are closed for the API;
open-ended maps without declared properties are rejected instead of being
silently emptied. Schema references cannot read files or fetch URLs. Optional
strict tools use the same schema preparation and validate tool arguments before
returning calls to the executor. Existing open-ended Piggyback tool argument maps
should use the native tool path; the full Piggyback envelope is not representable
as a closed Anthropic output schema without further specialization.

Streaming emits text only. It requires `message_stop` and a final stop reason,
reports token truncation and provider errors, and meters cache reads/writes and
partial responses. HTTP rate limits/transient errors retry with cancellation and
`Retry-After`; a stream is never replayed after it starts. API error diagnostics
retain status, error type and request ID without exposing the provider's body.
Credentials are never forwarded to redirect destinations.

To run the opt-in live smoke test, set `CODENERD_LIVE_ANTHROPIC=1`,
`ANTHROPIC_API_KEY`, and an explicit `ANTHROPIC_MODEL`, then run
`go test ./internal/perception -run '^TestLiveAnthropicDirectAPI$' -v`.
`ANTHROPIC_WORKSPACE_ID` is optional. This test makes billed requests through
the production factory and covers text, streaming, JSON output, and a native
tool round trip. Ordinary tests use local mock servers.

API references: [versioning](https://platform.claude.com/docs/en/api/versioning),
[thinking](https://platform.claude.com/docs/en/build-with-claude/thinking),
[structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs),
and [streaming](https://platform.claude.com/docs/en/build-with-claude/streaming).

## LLMClient Interface

```go
type LLMClient interface {
    Complete(ctx context.Context, prompt string) (string, error)
    CompleteWithSystem(ctx context.Context, system, user string) (string, error)
}

// Auto-detect provider
client, err := perception.NewClientFromEnv()
```

## Intent Structure

```go
type Intent struct {
    Category   string   // /query, /mutation, /instruction
    Verb       string   // /review, /fix, /explain
    Target     string   // File, function, or "codebase"
    Constraint string   // Additional constraints
    Confidence float64  // 0.0-1.0
}
```

## Verb Categories

| Category | Verbs |
|----------|-------|
| `/query` | explain, search, read, explore, analyze |
| `/mutation` | fix, refactor, create, delete, test, assault |
| `/instruction` | configure, always, prefer |

## Gemini Features

The GeminiClient supports advanced features:

- **Thinking Mode**: `minimal`, `low`, `medium`, `high` reasoning levels
- **Google Search**: Real-time search grounding
- **URL Context**: Ground responses with up to 20 URLs
- **Thought Signatures**: Multi-turn function calling support

```go
cfg := GeminiConfig{
    EnableThinking:     true,
    ThinkingLevel:      "high",
    EnableGoogleSearch: true,
}
```

## Testing

```bash
go test ./internal/perception/...
```

---

**Last Updated:** December 2024
