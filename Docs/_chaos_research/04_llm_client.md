# 04 - LLM Client System

## 1. Interface & Provider Inventory

### LLMClient Interface (`internal/types/interfaces.go:30-37`)

The `LLMClient` interface defines three methods:

| Method | Signature | Purpose |
|--------|-----------|---------|
| `Complete` | `(ctx, prompt) -> (string, error)` | Simple single-prompt completion |
| `CompleteWithSystem` | `(ctx, systemPrompt, userPrompt) -> (string, error)` | Completion with system instructions |
| `CompleteWithTools` | `(ctx, systemPrompt, userPrompt, tools) -> (*LLMToolResponse, error)` | Agentic completion with tool calling |

Supporting types: `ToolDefinition` (name, description, input_schema), `ToolCall` (id, name, input), `LLMToolResponse` (text, tool_calls, stop_reason, usage, thought metadata, grounding sources).

### Provider Implementations (ALL found via glob)

| Provider | File | Constructor |
|----------|------|-------------|
| **Gemini** | `internal/perception/client_gemini.go` | `NewGeminiClient` / `NewGeminiClientWithConfig` |
| **Anthropic** | `internal/perception/client_anthropic.go` | `NewAnthropicClient` |
| **OpenAI** | `internal/perception/client_openai.go` | `NewOpenAIClient` |
| **xAI** | `internal/perception/client_xai.go` | `NewXAIClient` |
| **Z.AI** | `internal/perception/client_zai.go` | `NewZAIClient` |
| **OpenRouter** | `internal/perception/client_openrouter.go` | `NewOpenRouterClient` |
| **Antigravity** | `internal/perception/client_antigravity.go` | `NewAntigravityClient` |
| **Claude CLI** | (CLI engine, not API) | `NewClaudeCodeCLIClient` |
| **Codex CLI** | (CLI engine, not API) | `NewCodexCLIClient` |

Support files: `client_types.go` (configs/request/response structs), `client_schema.go` (schema helpers), `client_tool_helpers.go` (tool conversion utils), `client_gemini_files.go` (Gemini file upload).

### Provider Detection Priority (`client_factory.go:82-128`)

1. **Config file** (`.nerd/config.json`) checked first via `LoadConfigJSON`
2. CLI engines (`claude-cli`, `codex-cli`) take precedence -- no API key needed
3. API mode requires non-empty API key
4. **Environment variable fallback** in strict order:
   - `ANTHROPIC_API_KEY` > `OPENAI_API_KEY` > `GEMINI_API_KEY` > `XAI_API_KEY` > `ZAI_API_KEY` > `OPENROUTER_API_KEY`
5. If nothing found: hard error with list of valid env vars

## 2. Request Construction & Size Limits

### Message Construction (Gemini as representative: `client_gemini.go:370-386`)

Requests are built as Go structs, then `json.Marshal`'d:

```
GeminiRequest {
  Contents:          [ {Role: "user", Parts: [{Text: userPrompt}]} ]
  SystemInstruction: &GeminiContent{ Parts: [{Text: systemPrompt}] }
  GenerationConfig:  { Temperature: 1.0, MaxOutputTokens: 65536, ThinkingConfig, ResponseMimeType, ResponseSchema }
  Tools:             built-in tools (Google Search, URL Context) or function declarations
}
```

- **System prompt**: if empty, defaults to a hardcoded `defaultSystemPrompt` string (`client_types.go:9`)
- **User prompt**: passed through with NO size validation, NO truncation, NO token counting before send
- **Piggyback detection**: substring search for `"control_packet"` or `"PiggybackEnvelope"` in prompt text (`client_gemini.go:357-359`) -- triggers `application/json` response MIME and schema enforcement

### Input Size Limits

**There are NO input size limits enforced before sending to the LLM.** The system:
- Does NOT count input tokens before sending
- Does NOT truncate oversized prompts
- Does NOT validate payload size
- Relies entirely on the upstream API to reject oversized requests (returns HTTP 400)
- `MaxOutputTokens` is set (65536 for Gemini 3) but this controls OUTPUT only

### Token Counting

No pre-send token estimation exists in the client layer. Token usage is only captured AFTER the response arrives via `UsageMetadata` in the response struct (`interfaces.go:53-60`). The JIT compiler handles budget upstream, but the client itself is a dumb pipe.

## 3. Response Parsing & Error Handling

### Non-Streaming Response Parsing (`client_gemini.go:430-470`)

1. `io.ReadAll(resp.Body)` -- reads entire response body into memory with **no size limit**
2. `json.Unmarshal(body, &geminiResp)` -- standard JSON parse
3. Checks `geminiResp.Error` field for API-level errors
4. Checks `len(Candidates) == 0 || len(Parts) == 0` -- returns `"no completion returned"` error
5. Iterates parts, separating `Thought` parts from response parts
6. Extracts grounding metadata from `GroundingMetadata.GroundingChunks`

### Streaming SSE Parsing (`client_gemini.go:804-1013`)

- Uses `bufio.NewScanner` on response body
- **Scanner buffer**: initial 64KB, max **1MB** (`client_gemini.go:934`)
- Content channel: buffered with capacity **100** (`client_gemini.go:807`)
- Error channel: buffered with capacity **1** (`client_gemini.go:808`)
- SSE parsing: looks for `data:` prefix, trims whitespace, checks for `[DONE]` sentinel
- **Malformed JSON in SSE chunks is silently skipped** (`client_gemini.go:955-957` -- `continue` on unmarshal error)
- Scanner goroutine runs independently; parent `select` waits on `scanDone` or `ctx.Done()`
- On context cancellation: closes `resp.Body` then drains scanner goroutine (`client_gemini.go:1000-1001`)

### Rate Limiting

- **Gemini**: 100ms minimum between requests, enforced via `sync.Mutex` + `time.Sleep` (`client_gemini.go:362-368`)
- **Z.AI**: 600ms rate limit delay (configurable via `RateLimitDelay` in `ZAIConfig`)
- HTTP 429 responses trigger retry (up to `maxRetries`)

### Retry Logic (`client_gemini.go:402-536`)

- **Max retries**: hardcoded to **3** in Gemini client (not configurable per-call)
- **Backoff**: exponential `1<<(attempt-1)` seconds (1s, 2s, 4s)
- Retries on: HTTP errors, rate limits (429), read failures, and schema rejection (400 with specific body patterns)
- Non-429 non-retryable errors: return immediately with full error body
- After all retries exhausted: wraps `lastErr` with `"max retries exceeded"`

### Timeout Configuration (`internal/config/llm_timeouts.go:1-180`)

Three-tiered timeout architecture:

| Tier | Timeout | Default | Fast | Aggressive |
|------|---------|---------|------|------------|
| **HTTP Client** | `HTTPClientTimeout` | 10 min | 5 min | 5 min |
| **Per Call** | `PerCallTimeout` | 10 min | 5 min | 5 min |
| **Streaming** | `StreamingTimeout` | 15 min | 6 min | 5 min |
| **Slot Acquisition** | `SlotAcquisitionTimeout` | 10 min | 6 min | 6 min |
| **Shard Execution** | `ShardExecutionTimeout` | 30 min | 7 min | 5 min |
| **Retry Backoff** | Base / Max | 1s / 30s | 500ms / 10s | 250ms / 5s |
| **Max Retries** | `MaxRetries` | 3 | 2 | 1 |

Key insight from comments: "In Go, the SHORTEST timeout in the chain wins." Auto-timeout applied when context has no deadline (`client_gemini.go:330-334`).

Global singleton pattern: `globalLLMTimeouts` set at startup via `SetLLMTimeouts()` (`llm_timeouts.go:168-179`).

## 4. CHAOS FAILURE PREDICTIONS

### P1: 10MB garbage input sent as userPrompt -- **CRITICAL**
- **Location**: `client_gemini.go:374` (userPrompt placed directly into `GeminiPart.Text`)
- **What happens**: The entire 10MB string is `json.Marshal`'d into the request body (doubling memory with escaping). The API will likely reject with 400/413, but the full error body is read via `io.ReadAll` (`client_gemini.go:430`). If the API returns a large error response, that's also unbounded memory. The retry loop will attempt this 3 more times, each time marshaling 10MB+ again.
- **Root cause**: Zero input validation or size cap in the client layer.

### P2: LLM returns malformed JSON (non-streaming) -- **HIGH**
- **Location**: `client_gemini.go:457-458`
- **What happens**: `json.Unmarshal` fails, returns `"failed to parse response"` error. This is NOT retried -- the error returns immediately to the caller. No fallback to raw text extraction. If the Piggyback Protocol or schema enforcement mode was active, the entire turn is lost with no recovery.
- **Root cause**: No fallback parser for partially valid JSON.

### P3: LLM returns empty response (200 OK, zero candidates) -- **HIGH**
- **Location**: `client_gemini.go:469-471`
- **What happens**: Returns `"no completion returned"` error. This is NOT retried (only 429 and network errors trigger retry). Callers in the shard layer receive a generic error with no indication whether to retry or abort. The shard execution timeout (30 min default) continues burning while the caller may re-invoke.
- **Root cause**: Empty-response is not treated as a transient/retryable condition.

### P4: Streaming SSE interrupted mid-chunk -- **CRITICAL**
- **Location**: `client_gemini.go:954-957`
- **What happens**: The scanner reads a partial `data:` line containing truncated JSON. `json.Unmarshal` fails and the error is **silently ignored** (`continue`). The stream continues reading, potentially accumulating corrupt partial data. The content channel may receive partial text that downstream consumers interpret as complete. No error is surfaced to the caller.
- **Root cause**: Silent `continue` on unmarshal error with no logging, no error propagation, no corruption detection.

### P5: All retries exhausted -- **MEDIUM**
- **Location**: `client_gemini.go:535-536`
- **What happens**: Returns `"max retries exceeded"` wrapping the last error. The wrapped error may be a rate limit, network timeout, or read failure. Callers must unwrap to determine the root cause. The `CompleteWithTools` method has NO retry loop at all (`client_gemini.go:1106`) -- a single failure is terminal.
- **Root cause**: Inconsistent retry behavior across methods; `CompleteWithTools` has zero resilience.

### P6: Invalid API key -- **MEDIUM**
- **Location**: `client_gemini.go:339-341` (empty key check), `client_gemini.go:441-453` (non-200 status)
- **What happens**: Empty key returns immediately with `"API key not configured"`. Invalid (non-empty) key results in 401/403 from the API, which returns the full error body as a string. This is NOT retried (only 429 triggers retry). The API key is included in the URL as a query parameter (`client_gemini.go:400` -- `?key=%s`), meaning it appears in logs and error messages.
- **Root cause**: API key in URL (not header) leaks into error strings and logs.

### P7: Scanner buffer overflow (SSE event > 1MB) -- **CRITICAL**
- **Location**: `client_gemini.go:934`
- **What happens**: The scanner has a hard max buffer of 1MB (`1024*1024`). If a single SSE `data:` line exceeds 1MB (possible with very large thinking blocks or grounding metadata), `scanner.Scan()` returns false and `scanner.Err()` returns `bufio.ErrTooLong`. The error is caught (`client_gemini.go:985-987`) and sent to `scanErrChan`, but any content received before the overflow is already sent to `contentChan`. Partial results with a late error.
- **Root cause**: 1MB scanner limit is undersized for models with large thinking/grounding responses.

### P8: Race condition on `lastThoughtSignature` during streaming -- **HIGH**
- **Location**: `client_gemini.go:962-973`
- **What happens**: The streaming goroutine (`client_gemini.go:939`) writes to `c.lastThoughtSignature` without holding `c.mu`. The parent goroutine and any concurrent callers reading `GetLastThoughtSignature()` (`client_gemini.go:284`) also access this field without synchronization. This is a data race under Go's memory model.
- **Root cause**: Streaming goroutine bypasses the mutex used by rate limiting (`c.mu`) for thought signature writes.

### P9: `defer resp.Body.Close()` inside retry loop -- **HIGH**
- **Location**: `client_gemini.go:428`
- **What happens**: Each retry iteration defers `resp.Body.Close()`. If 4 iterations execute (1 initial + 3 retries), 4 deferred closures accumulate, but only the last response body matters. The earlier bodies are read via `io.ReadAll` but their Close is deferred until the function returns, holding connections open during retries.
- **Root cause**: `defer` in a loop; should use explicit `resp.Body.Close()` after each `io.ReadAll`.

### P10: Unbounded `io.ReadAll` on error response bodies -- **MEDIUM**
- **Location**: `client_gemini.go:430`, `client_gemini.go:903`, `client_gemini.go:910`, `client_gemini.go:1113`
- **What happens**: If a malicious or buggy API proxy returns a multi-GB error response body, `io.ReadAll` will attempt to read it all into memory, causing OOM. This applies to all code paths: non-streaming success, streaming 429 handling, streaming non-200 handling, and `CompleteWithTools`.
- **Root cause**: No `io.LimitReader` wrapper on response bodies.

### Summary Table

| # | Scenario | Severity | Retried? | Silent? |
|---|----------|----------|----------|---------|
| P1 | 10MB garbage input | CRITICAL | Yes (3x, worsens it) | No |
| P2 | Malformed JSON response | HIGH | No | No |
| P3 | Empty response (0 candidates) | HIGH | No | No |
| P4 | SSE interrupted mid-chunk | CRITICAL | No | **YES** |
| P5 | All retries exhausted | MEDIUM | N/A | No |
| P6 | Invalid API key | MEDIUM | No | No (but key leaks) |
| P7 | SSE event > 1MB | CRITICAL | No | Partial |
| P8 | Race on `lastThoughtSignature` | HIGH | N/A | **YES** |
| P9 | `defer` in retry loop | HIGH | N/A | **YES** |
| P10 | Unbounded error body read | MEDIUM | N/A | **YES** |
