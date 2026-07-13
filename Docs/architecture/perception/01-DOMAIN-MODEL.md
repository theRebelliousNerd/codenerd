# perception — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/perception/` (49 non-test .go, 47 tests, 0 .mg)**


## Source package

`internal/perception/`

## Exported / primary types (sampled)

| Type | Location |
|------|----------|
| `RateLimitError` | `internal/perception/claude_cli_client.go:20` |
| `StreamChunk` | `internal/perception/claude_cli_client.go:35` |
| `StreamCallback` | `internal/perception/claude_cli_client.go:45` |
| `ClaudeCodeCLIClient` | `internal/perception/claude_cli_client.go:59` |
| `ExecutionOptions` | `internal/perception/claude_cli_client.go:205` |
| `AnthropicClient` | `internal/perception/client_anthropic.go:18` |
| `ProviderConfig` | `internal/perception/client_factory.go:34` |
| `GeminiClient` | `internal/perception/client_gemini.go:29` |
| `OpenAIClient` | `internal/perception/client_openai.go:18` |
| `OpenRouterClient` | `internal/perception/client_openrouter.go:19` |
| `LLMClient` | `internal/perception/client_types.go:13` |
| `ToolDefinition` | `internal/perception/client_types.go:17` |
| `ToolCall` | `internal/perception/client_types.go:21` |
| `ToolResult` | `internal/perception/client_types.go:24` |
| `LLMToolResponse` | `internal/perception/client_types.go:32` |
| `Provider` | `internal/perception/client_types.go:35` |
| `ZAIConfig` | `internal/perception/client_types.go:47` |
| `AnthropicConfig` | `internal/perception/client_types.go:62` |
| `OpenAIConfig` | `internal/perception/client_types.go:70` |
| `GeminiConfig` | `internal/perception/client_types.go:78` |
| `XAIConfig` | `internal/perception/client_types.go:96` |
| `OpenRouterConfig` | `internal/perception/client_types.go:104` |
| `ZAIStreamOptions` | `internal/perception/client_types.go:114` |
| `ZAIResponseFormat` | `internal/perception/client_types.go:119` |
| `ZAIJSONSchema` | `internal/perception/client_types.go:125` |

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 0 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| — | 0 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **NL→atoms transduction, semantic classifier, LLM clients**

## Data & control concepts

- Primary language surface: Go under `internal/perception/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
