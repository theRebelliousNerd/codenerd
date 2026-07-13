# 06 — Public API and Types (perception)

> Last verified: **2026-07-13**  
> Focus: symbols **callers outside the package** rely on. File refs are under `internal/perception/` unless noted.

## Core transduction

| Symbol | File | Role |
|--------|------|------|
| `Transducer` | `transducer.go` | Primary interface |
| `TransducerWithKernel` | `transducer.go` | Kernel-aware extension |
| `PromptAssembler` | `transducer.go` | JIT prompt bridge |
| `Intent` | `transducer.go` | Parsed intent + flags |
| `ConversationTurn` | `transducer.go` | History turn (+ thoughts) |
| `FocusResolution` | `transducer.go` | Reference resolution fact |
| `Intent.ToFact` | `transducer.go` | `user_intent` fact |
| `FocusResolution.ToFact` | `transducer.go` | `focus_resolution` fact |
| `NewUnderstandingTransducer` | `understanding_adapter.go` | Canonical constructor |
| `UnderstandingTransducer` | `understanding_adapter.go` | Implementation |
| `GeminiThinkingTransducer` | `transducer_gemini.go` | Thinking specialization |
| `LLMTransducer` | `transducer_llm.go` | Understand + route |
| `NewLLMTransducer` | `transducer_llm.go` | Constructor |
| `ExtractCleanJSON` | `transducer_llm.go` | JSON extraction utility |
| `RoutingKernel` | `transducer_llm.go` | Mode/shard queries |
| `KernelAsserter` | `transducer_llm.go` | Assert routing facts |
| `RoutingMatch` | `transducer_llm.go` | Query result |

## Understanding model

| Symbol | File |
|--------|------|
| `Understanding` | `understanding.go` |
| `Scope` | `understanding.go` |
| `Signals` | `understanding.go` |
| `SuggestedApproach` | `understanding.go` |
| `Routing` | `understanding.go` |
| `UnderstandingEnvelope` | `understanding.go` |

## Semantic classification

| Symbol | File |
|--------|------|
| `SemanticMatch` | `semantic_classifier.go` |
| `SemanticConfig` / `DefaultSemanticConfig` | `semantic_classifier.go` |
| `SemanticClassifier` | `semantic_classifier.go` |
| `NewSemanticClassifier` | `semantic_classifier.go` |
| `NewSemanticClassifierFromConfig` | `semantic_classifier.go` |
| `EmbeddedCorpusStore` / `LearnedCorpusStore` | `semantic_classifier.go` |
| `CorpusEntry` | `semantic_classifier.go` |
| `SharedSemanticClassifier` | `semantic_classifier.go` |
| `InitSharedSemanticClassifier` | `semantic_classifier.go` |
| `CloseSharedSemanticClassifier` | `semantic_classifier.go` |

## Taxonomy & learning

| Symbol | File |
|--------|------|
| `TaxonomyEngine` | `taxonomy.go` |
| `SharedTaxonomy` | `taxonomy.go` |
| `NewTaxonomyEngine` | `taxonomy.go` |
| `VerbEntry` / `GetVerbCorpus` / `SetVerbCorpus` | `transducer.go` |
| `ConsolidationWorker` | `consolidation.go` |
| `CriticSystemPrompt` / `ExtractFactFromResponse` | `learning.go` |
| `ReasoningTrace` | `tracing_client.go` |
| `TraceStore` | `tracing_client.go` |

## LLM clients & factory

| Symbol | File |
|--------|------|
| `LLMClient` | `client_types.go` (alias `types.LLMClient`) |
| `Provider` + constants | `client_types.go` |
| `*Config` structs (ZAI, Anthropic, …) | `client_types.go` |
| `ToolResult` | `client_types.go` |
| `ProviderConfig` | `client_factory.go` |
| `DetectProvider` / `LoadConfigJSON` | `client_factory.go` |
| `NewClientFromEnv` / `NewClientFromConfig` | `client_factory.go` |
| `NewClassificationClientFromConfig` | `client_factory.go` |
| `NewWorkerClientFromUserConfig` | `client_factory.go` |
| `NewImageClientFromUserConfig` | `client_factory.go` |
| `NewZAIClient*` | `client_zai.go` |
| `NewAnthropicClient*` | `client_anthropic.go` |
| `NewOpenAIClient*` | `client_openai.go` |
| `NewGeminiClient*` | `client_gemini.go` |
| `NewXAIClient*` | `client_xai.go` |
| `NewOpenRouterClient*` | `client_openrouter.go` |
| `NewOllamaClient*` | `client_ollama.go` |
| `ErrLLMUnavailable` | `client_gemini.go` |
| `NewClaudeCodeCLIClient` | `claude_cli_client.go` |
| `RateLimitError` | `claude_cli_client.go` |
| `NewCodexExecClient` / probe helpers | `codex_*.go` |
| `NewTracingLLMClient` | `tracing_client.go` |
| `RecordLLMCall` / `GetLLMMetrics` | `metrics.go` |
| `NewSharedHTTPClient` | `transport.go` |

## Schema builders

| Symbol | File |
|--------|------|
| `BuildZAIPiggybackEnvelopeSchema` | `client_schema.go` |
| `BuildOpenAIPiggybackEnvelopeSchema` | `client_schema.go` |
| `BuildGeminiPiggybackEnvelopeSchema` | `client_schema.go` |
| `BuildOpenRouterPiggybackEnvelopeSchema` | `client_schema.go` |

## Type aliases from articulation

Re-exported in `transducer.go` for compatibility:

- `PiggybackEnvelope`, `ControlPacket`, `IntentClassification`  
- `MemoryOperation`, `SelfCorrection`

## xaioauth public surface

| Symbol | Package path |
|--------|--------------|
| Client constructors / Complete* | `internal/perception/xaioauth` |
| Store / token / device auth | same |
| Probe helpers | same |

Exact exported names: see `xaioauth/client.go`, `store.go`, `auth_device.go`, `probe.go`.

## Important behavioral contracts

1. **`ParseIntentWithContext` error contract:** often returns `(degradedIntent, nil)` on LLM failure. Check `TransientFailure` and `Response`.  
2. **Classification client nil:** not an error — use main client.  
3. **Semantic Classify empty:** not fatal.  
4. **`LLMClient` minimal surface:** tools/streaming require concrete type or richer interface.

## Not public / internal helpers (selected)

- `matchVerbFromCorpus`, `getRegexCandidates`, `sanitizeFactArg` (package-level; used heavily in tests)  
- `newPooledScanner`  
- `understandingSystemPrompt` embedded constant (adapter)  
- Provider wire DTOs — exported types but rarely constructed outside clients
