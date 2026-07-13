# perception — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/perception/` (complete internal coverage)
> **Implementation: `internal/perception/` — 50 non-test .go, 48 tests, 0 .mg**


## Package

`internal/perception/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `RateLimitError` | `internal/perception/claude_cli_client.go:20` |
| `StreamChunk` | `internal/perception/claude_cli_client.go:35` |
| `StreamCallback` | `internal/perception/claude_cli_client.go:45` |
| `ClaudeCodeCLIClient` | `internal/perception/claude_cli_client.go:59` |
| `ExecutionOptions` | `internal/perception/claude_cli_client.go:205` |
| `AnthropicClient` | `internal/perception/client_anthropic.go:18` |
| `ProviderConfig` | `internal/perception/client_factory.go:37` |
| `GeminiClient` | `internal/perception/client_gemini.go:29` |
| `OllamaClient` | `internal/perception/client_ollama.go:16` |
| `OllamaLLMConfig` | `internal/perception/client_ollama.go:32` |
| `OpenAIClient` | `internal/perception/client_openai.go:18` |
| `OpenRouterClient` | `internal/perception/client_openrouter.go:19` |
| `LLMClient` | `internal/perception/client_types.go:13` |
| `ToolDefinition` | `internal/perception/client_types.go:17` |
| `ToolCall` | `internal/perception/client_types.go:21` |
| `ToolResult` | `internal/perception/client_types.go:24` |
| `LLMToolResponse` | `internal/perception/client_types.go:32` |
| `Provider` | `internal/perception/client_types.go:35` |
| `ZAIConfig` | `internal/perception/client_types.go:48` |
| `AnthropicConfig` | `internal/perception/client_types.go:63` |
| `OpenAIConfig` | `internal/perception/client_types.go:71` |
| `GeminiConfig` | `internal/perception/client_types.go:79` |
| `XAIConfig` | `internal/perception/client_types.go:97` |
| `OpenRouterConfig` | `internal/perception/client_types.go:105` |
| `ZAIStreamOptions` | `internal/perception/client_types.go:115` |
| `ZAIResponseFormat` | `internal/perception/client_types.go:120` |
| `ZAIJSONSchema` | `internal/perception/client_types.go:126` |
| `ZAIThinking` | `internal/perception/client_types.go:133` |
| `ZAIMessage` | `internal/perception/client_types.go:139` |
| `ZAIRequest` | `internal/perception/client_types.go:145` |
| `ZAIResponse` | `internal/perception/client_types.go:160` |
| `AnthropicMessage` | `internal/perception/client_types.go:194` |
| `AnthropicContentBlock` | `internal/perception/client_types.go:200` |
| `AnthropicTool` | `internal/perception/client_types.go:212` |
| `AnthropicRequest` | `internal/perception/client_types.go:219` |
| `AnthropicCacheControl` | `internal/perception/client_types.go:232` |
| `AnthropicSystemCacheBlock` | `internal/perception/client_types.go:240` |
| `AnthropicResponse` | `internal/perception/client_types.go:262` |
| `OpenAITool` | `internal/perception/client_types.go:282` |
| `OpenAIFunction` | `internal/perception/client_types.go:288` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `Error` | `internal/perception/claude_cli_client.go:27` |
| `NewClaudeCodeCLIClient` | `internal/perception/claude_cli_client.go:97` |
| `Complete` | `internal/perception/claude_cli_client.go:123` |
| `CompleteWithSystem` | `internal/perception/claude_cli_client.go:129` |
| `CompleteWithSchema` | `internal/perception/claude_cli_client.go:149` |
| `CompleteStreaming` | `internal/perception/claude_cli_client.go:159` |
| `CompleteWithStreaming` | `internal/perception/claude_cli_client.go:169` |
| `SetModel` | `internal/perception/claude_cli_client.go:491` |
| `GetModel` | `internal/perception/claude_cli_client.go:496` |
| `SetFallbackModel` | `internal/perception/claude_cli_client.go:501` |
| `GetFallbackModel` | `internal/perception/claude_cli_client.go:506` |
| `SetTimeout` | `internal/perception/claude_cli_client.go:511` |
| `GetTimeout` | `internal/perception/claude_cli_client.go:516` |
| `SetStreaming` | `internal/perception/claude_cli_client.go:521` |
| `IsStreaming` | `internal/perception/claude_cli_client.go:526` |
| `SetMaxTurns` | `internal/perception/claude_cli_client.go:532` |
| `GetMaxTurns` | `internal/perception/claude_cli_client.go:537` |
| `CompleteWithTools` | `internal/perception/claude_cli_client.go:543` |
| `ShouldUsePiggybackTools` | `internal/perception/claude_cli_client.go:574` |
| `DefaultAnthropicConfig` | `internal/perception/client_anthropic.go:34` |
| `NewAnthropicClient` | `internal/perception/client_anthropic.go:44` |
| `NewAnthropicClientWithConfig` | `internal/perception/client_anthropic.go:50` |
| `EnableSystemCaching` | `internal/perception/client_anthropic.go:65` |
| `Complete` | `internal/perception/client_anthropic.go:98` |
| `CompleteWithSystem` | `internal/perception/client_anthropic.go:103` |
| `CompleteWithStreaming` | `internal/perception/client_anthropic.go:251` |
| `CompleteWithTools` | `internal/perception/client_anthropic.go:390` |
| `CompleteWithToolResults` | `internal/perception/client_anthropic.go:500` |
| `SetModel` | `internal/perception/client_anthropic.go:665` |
| `GetModel` | `internal/perception/client_anthropic.go:670` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **NL→atoms transduction, semantic classification, LLM clients**
