# perception — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/perception/` (49 non-test .go, 47 tests, 0 .mg)**


## 1. Purpose

NL→atoms transduction, semantic classifier, LLM clients

## 2. Source paths

| Path | Role |
|------|------|
| `internal/perception/` | Primary implementation |
| `Docs/architecture/perception/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | N/A or global | **n/a** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 85% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

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
| `internal/perception/client_types.go` | 623 | source |
| `internal/perception/transducer.go` | 616 | source |
| `internal/perception/codex_cli_client.go` | 596 | source |
| `internal/perception/claude_cli_client.go` | 576 | source |

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
