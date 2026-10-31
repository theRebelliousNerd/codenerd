# 14 — OpenAI on the Responses API (as built, 2026-10-29)

## Transport uncertainty contract

Shared non-streaming Responses and grounded-search helpers must notify enclosing
incomplete-usage observers after an HTTP transport error, before retry or typed
cancellation returns. A later counted success preserves its counts and cannot
resolve the failed exchange's unknown cost. GAP-PERCEPTION-CROSS-LOSS owns this
bounded strengthening across the shared helpers. Keep OpenAI request, answer,
retry and sanitized grounded-error behavior intact; no billing amount is
invented and non-200 billing classification remains separate.

This boundary is closed locally in
`33bf57d2731bde876270dfe635b4efa4177be869` at
`internal/perception/responses_transport.go:57` and
`internal/perception/client_openai_compat_grounding.go:239`. Shared OpenAI
controls, including grounded transport error sanitization and cancellation,
pass under race. The new lost-response broker/mission matrix directly exercises
DashScope and Meta; this is offline accounting evidence, not live acceptance.

## What changed

`OpenAIClient` (`internal/perception/client_openai*.go`) sends every request to
`POST /v1/responses`. It used to speak Chat Completions with a 4096-token
default ceiling, no reasoning effort, no multi-turn tool path and no caching
controls. OpenAI's GPT-6 family calls tools only through Responses, and only
Responses carries reasoning across turns.

The Responses protocol code that the Meta client already had is now shared:

| File | Holds |
|---|---|
| `responses_wire.go` | request/reply/usage/item types |
| `responses_input.go` | history → input items, reasoning replay in position, `promptCacheKey` |
| `responses_reply.go` | answer text, tool response with ordered blocks, grounding capture, `responsesTerminal` |
| `responses_stream.go` | the Responses SSE reader (`readResponsesStream`) |
| `responses_transport.go` | `postResponses`: bounded retry, Retry-After, classified status errors |

Each client keeps its own options and tools: Meta's search switch and reserved
names stay in `client_meta_responses.go`; OpenAI's in `client_openai*.go`.
Ollama now owns the Chat Completions transport (`client_ollama_transport.go`).

## Request contract

| Field | Source |
|---|---|
| `model` | slot model, or a shard profile's (`types.WithModelName`) |
| `reasoning.effort` | slot `reasoning_effort`, or a profile's (`types.WithReasoningEffort`) |
| `reasoning.mode/context/summary` | `openai.reasoning_mode/context/summary` (summary only on streams that asked for thinking) |
| `store` | always `false`; reasoning arrives as `encrypted_content` and is replayed |
| `include` | `reasoning.encrypted_content` on reasoning models; `web_search_call.action.sources` with search |
| `max_output_tokens` | slot `max_output_tokens`, else the model's maximum (`config.OpenAIMaxOutputTokens`), else 32768 |
| `temperature` | non-reasoning models only (a reasoning model rejects it unless effort is none) |
| `text.verbosity` | `openai.verbosity` |
| `text.format` | strict `PiggybackEnvelope` json_schema for piggyback prompts; dropped once on a schema rejection |
| `prompt_cache_key` | `codenerd:<model>:<tool-catalog hash>` |
| `prompt_cache_options.ttl` | `openai.prompt_cache_ttl`, GPT-5.6+ only |
| `prompt_cache_retention` | `24h` on GPT-5 models before 5.6 |
| `service_tier` | `openai.service_tier` |
| `tools` | flat function tools, plus `web_search` when `openai.enable_web_search` is on |

## Refusals before the wire

- `config.ValidateOpenAIEffort`: an effort outside the API vocabulary, or one
  the model rejects (GPT-6 Astra and GPT-6.1 Sol: low, medium, high, xhigh, max;
  GPT-6.1 Terra, GPT-6 Luna, GPT-6 Sol: none plus those). Checked at config load,
  at client build, and per request for a profile's effort.
- `OpenAIProviderConfig.Check`: any `openai.*` value outside its vocabulary.
- `admitInputWindow`: input over the slot's `max_input_tokens`.

## Outcomes

- `incomplete` with `max_output_tokens` → `*types.OutputTruncated` with the
  partial text; any other incomplete reason, `failed` or `cancelled` → error.
- Streams end only on `response.completed|incomplete|failed`; a clean EOF
  before one is `StreamIncomplete`. Commentary-phase items are not forwarded.
  A completed run that ends in a function call is `StreamUnsupported` on a
  text stream.
- HTTP failures read `request failed with status N`, so
  `ClassifyProviderFailure` classes them; 429 and 5xx retry.
- Grounded-search errors carry the status and sanitized code/type only.

## Usage

`input_tokens_details.cache_write_tokens` → `UsageMetadata.CacheWriteTokens`
(known only when reported). Prices for the GPT-6, GPT-5.6, 5.5 and 5.4 rows
are in `internal/usage/pricing.go`.

## Verified

Live against `gpt-6.1-sol` on 2026-10-29: plain completion (`phase:
final_answer`, cache-write usage fields), streamed event names, a tool call
whose follow-up replayed the encrypted reasoning, and a 400 for effort `none`.
Unit tests: `client_openai_test.go`, `stream_conformance_test.go` (Responses
fixture), `provider_failure_conformance_test.go`, `internal/config/openai_test.go`.
