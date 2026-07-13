# perception — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/perception/` (complete internal coverage)
> **Implementation: `internal/perception/` — 50 non-test .go, 48 tests, 0 .mg**


## 1. Purpose

NL→atoms transduction, semantic classification, LLM clients

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/perception/` | Primary implementation |
| `Docs/architecture/perception/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 85%** as living package (50 src / 48 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/perception/semantic_classifier.go` | 1254 | source |
| `internal/perception/client_zai.go` | 1041 | source |
| `internal/perception/transducer_llm.go` | 949 | source |
| `internal/perception/tracing_client.go` | 946 | source |
| `internal/perception/client_gemini.go` | 944 | source |
| `internal/perception/taxonomy.go` | 799 | source |
| `internal/perception/understanding_adapter.go` | 725 | source |
| `internal/perception/client_anthropic.go` | 686 | source |
| `internal/perception/client_types.go` | 624 | source |
| `internal/perception/transducer.go` | 616 | source |
| `internal/perception/codex_cli_client.go` | 596 | source |
| `internal/perception/claude_cli_client.go` | 576 | source |
| `internal/perception/client_openai.go` | 505 | source |
| `internal/perception/client_openrouter.go` | 441 | source |
| `internal/perception/client_factory.go` | 425 | source |
| `internal/perception/client_gemini_files.go` | 403 | source |
| `internal/perception/client_gemini_tools.go` | 400 | source |
| `internal/perception/learning.go` | 400 | source |
| `internal/perception/client_gemini_streaming.go` | 354 | source |
| `internal/perception/client_zai_streaming.go` | 308 | source |

### Types (sampled)

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

### Functions (sampled)

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

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
