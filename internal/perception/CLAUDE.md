# internal/perception - Natural Language Understanding & LLM Providers

This package implements the Perception layer of codeNERD - the Transducer that converts natural language into structured intents using the Piggyback Protocol, plus the multi-provider LLM client system.

**Related Packages:**
- [cmd/nerd/chat](../../../cmd/nerd/chat/CLAUDE.md) - TUI that uses this package for LLM communication
- [internal/articulation](../articulation/CLAUDE.md) - Output transduction (Mangle → NL)

## Architecture

The Perception layer sits at the entry point of the system, parsing user input into structured `Intent` objects that the rest of the system can act upon.

### Piggyback Protocol

Every LLM response contains dual payloads:
```json
{
  "surface_response": "Natural language for the user",
  "control_packet": {
    "intent_classification": {...},
    "mangle_updates": [...],
    "memory_operations": [...]
  }
}
```

## File Index

| File | Description |
|------|-------------|
| `assault_verb_test.go` | Unit tests for assault verb parsing in the taxonomy system. Tests `/assault` intent detection and campaign trigger patterns. |
| `claude_cli_client.go` | LLMClient implementation using Claude Code CLI as subprocess. Exports ClaudeCodeCLIClient with streaming output and JSON schema validation for Piggyback Protocol. |
| `claude_cli_client_test.go` | Unit tests for ClaudeCodeCLIClient subprocess execution. Tests streaming output parsing and rate limit handling. |
| `client.go` | Package marker documenting client modularization across 10 files. Points to client_types, client_schema, and six provider implementations. |
| `client_anthropic.go` | LLMClient implementation for Anthropic API with streaming support. Exports AnthropicClient with Complete() and CompleteWithSystem() methods. |
| `client_factory.go` | Provider detection and client factory functions for multi-provider support. Exports DetectProvider(), LoadConfigJSON(), and NewClientFromEnv() with config priority. |
| `client_gemini.go` | LLMClient implementation for Google Gemini API with streaming, thinking mode, and grounding tools. Exports GeminiClient with CompleteWithTools(), CompleteWithToolResults(), thinking mode config (thinkingLevel: minimal/low/medium/high), Google Search grounding, URL context, and thought signature capture for multi-turn function calling. |
| `client_openai.go` | LLMClient implementation for OpenAI API including Codex models. Exports OpenAIClient with gpt-5.1-codex-max as default model. |
| `client_openrouter.go` | LLMClient implementation for OpenRouter multi-provider API. Exports OpenRouterClient with site attribution and model routing. |
| `client_schema.go` | Provider-specific JSON schema builders for structured output. Exports BuildZAIPiggybackEnvelopeSchema(), BuildOpenAIPiggybackEnvelopeSchema(), BuildGeminiPiggybackEnvelopeSchema(), BuildOpenRouterPiggybackEnvelopeSchema() for each provider's API format. |
| `client_types.go` | Core type definitions including LLMClient interface and config structs. Exports Provider constants and ZAIConfig, AnthropicConfig, OpenAIConfig, etc. |
| `client_xai.go` | LLMClient implementation for xAI (Grok) API. Exports XAIClient with Complete() and CompleteWithSystem() methods. |
| `client_zai.go` | LLMClient implementation for Z.AI API with concurrency semaphore. Exports ZAIClient with 5-concurrent-request limit and streaming support. |
| `codex_cli_client.go` | LLMClient implementation using Codex CLI as subprocess. Exports CodexCLIClient with NDJSON stream parsing and read-only sandbox mode. |
| `codex_cli_client_test.go` | Unit tests for CodexCLIClient subprocess execution. Tests NDJSON event parsing and sandbox mode enforcement. |
| `debug.go` | Debug utilities exposing internal classification logic for verification. Exports DebugTaxonomy() and DebugTaxonomyWithContext() for testing. |
| `learning.go` | Autopoiesis learning from user rejections via Meta-Cognitive Supervisor. Exports CriticSystemPrompt and ExtractFactFromResponse() for learned_exemplar generation. |
| `semantic_classifier.go` | Vector-based intent classification bridging embedding search and Mangle. Exports SemanticMatch and SemanticClassifier for neuro-symbolic pipeline. |
| `semantic_classifier_test.go` | Unit tests for SemanticClassifier vector search and Mangle integration. Tests similarity scoring and semantic_match fact assertion. |
| `taxonomy.go` | TaxonomyEngine managing verb taxonomy using Mangle for intent parsing. Exports SharedTaxonomy singleton with GetVerbs() and intent schema loading. |
| `taxonomy_persistence.go` | Persistence layer for taxonomy facts to SQLite database. Exports TaxonomyStore with StoreVerbDef(), StoreLearnedExemplar(), and HydrateEngine(). |
| `tracing_client.go` | Debug wrapper capturing all LLM interactions for learning analysis. Exports TracingLLMClient and ReasoningTrace for shard trace storage. |
| `transducer.go` | Main Transducer converting NL to structured intents via Piggyback Protocol. Exports VerbCorpus, VerbEntry, ParseIntent(), and PiggybackEnvelope aliases. |
| `transducer_gcd_stream_gate_test.go` | Unit tests for transducer GCD streaming gate logic. Tests rate limiting and concurrent request handling. |
| `transducer_json_test.go` | Unit tests for transducer JSON parsing and envelope extraction. Tests Piggyback Protocol compliance and fallback parsing. |
| `transducer_live_test.go` | Integration tests for live transducer execution. Tests end-to-end intent parsing with actual LLM calls. |

## Key Types

### Intent
```go
type Intent struct {
    Category   string   // /query, /mutation, /instruction
    Verb       string   // /review, /fix, /explain, etc.
    Target     string   // File, function, or "codebase"
    Constraint string   // Additional constraints
    Confidence float64  // 0.0-1.0
    Response   string   // Surface response for user
}
```

### VerbCorpus
Comprehensive mapping of natural language to intent verbs:
- Synonyms: Alternative phrasings
- Patterns: Regex patterns for matching
- Priority: Disambiguation weight
- ShardType: Recommended shard for execution

### LLMClient
Interface for LLM backends:
```go
type LLMClient interface {
    Complete(ctx context.Context, prompt string) (string, error)
    CompleteWithSystem(ctx context.Context, system, user string) (string, error)
}
```

## Verb Categories

### Query Verbs (/query)
Read-only operations: `/explain`, `/search`, `/read`, `/explore`, `/analyze`

### Mutation Verbs (/mutation)
Code-changing operations: `/fix`, `/refactor`, `/create`, `/delete`, `/test`, `/assault` (adversarial assault campaigns)

### Instruction Verbs (/instruction)
Preference setting: `/configure`, `/always`, `/prefer`

## Parsing Flow

1. Input → `ParseIntent()`
2. Try Piggyback Protocol (JSON extraction)
3. Fallback to simple pipe-delimited parsing
4. Ultimate fallback: heuristic corpus matching

## Adding New Verbs

1. Add a `VerbEntry` to `DefaultTaxonomyData` in `taxonomy.go` (synonyms, patterns, priority, shardType)
2. Ensure the intent prompt schema/atoms allow the verb (e.g., `internal/prompt/atoms/perception/transducer.yaml`)
3. If the verb should execute, wire routing (chat/CLI) and/or policy mappings as appropriate

## LLM Provider System

The package implements a multi-provider LLM client factory (`client.go`).

### Supported Providers

| Provider | Config Key | Environment Variable | Default Model | Notes |
|----------|------------|---------------------|---------------|-------|
| Z.AI | `zai_api_key` | `ZAI_API_KEY` | `glm-4.7` | 200K context, 128K output |
| Anthropic | `anthropic_api_key` | `ANTHROPIC_API_KEY` | `claude-sonnet-4` | Claude 4 series |
| OpenAI | `openai_api_key` | `OPENAI_API_KEY` | `gpt-5.1-codex-max` | Codex models |
| Gemini | `gemini_api_key` | `GEMINI_API_KEY` | `gemini-3-pro-preview` | Flash/Pro available |
| xAI | `xai_api_key` | `XAI_API_KEY` | `grok-3-beta` | Grok series |
| OpenRouter | `openrouter_api_key` | `OPENROUTER_API_KEY` | `anthropic/claude-sonnet-4` | Multi-model routing |
| Claude CLI | `engine: claude-cli` | - | (CLI default) | Subprocess backend |
| Codex CLI | `engine: codex-cli` | - | (CLI default) | Subprocess backend |

### Provider Detection

```go
// Auto-detect provider from config/environment
client, err := perception.NewClientFromEnv()

// Get provider configuration
providerCfg, err := perception.DetectProvider()
```

Configuration is read from `.nerd/config.json`:
```json
{
  "provider": "zai",
  "zai_api_key": "your-key-here",
  "model": "glm-4.6"
}
```

### Client Interface

All providers implement `LLMClient`:
```go
type LLMClient interface {
    Complete(ctx context.Context, prompt string) (string, error)
    CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}
```

Each concrete client also has `GetModel() string` for introspection.

## Gemini 3 Features

The GeminiClient supports advanced Gemini 3 Flash Preview features:

### Thinking Mode

Enables explicit reasoning before responding. Levels (lowercase):
- `minimal` - Minimal reasoning
- `low` - Light reasoning
- `medium` - Moderate reasoning
- `high` - Maximum reasoning depth (default)

```go
// Configure via GeminiConfig
cfg := GeminiConfig{
    EnableThinking: true,
    ThinkingLevel:  "high",
}
```

### Built-in Tools

| Tool | Description |
|------|-------------|
| Google Search | Real-time search grounding |
| URL Context | Ground responses with URLs (max 20) |

```go
cfg := GeminiConfig{
    EnableGoogleSearch: true,
    EnableURLContext:   true,
}
client.SetURLContextURLs([]string{"https://example.com/docs"})

// After a call, get grounding sources for transparency
sources := client.GetLastGroundingSources()
```

### Thought Signatures (Multi-Turn Function Calling)

For Gemini 3, thought signatures must be passed back in multi-turn conversations:

```go
// First call - get tool calls and thought signature
resp, _ := client.CompleteWithTools(ctx, system, user, tools)
sig := client.GetLastThoughtSignature()

// Continue conversation with tool results
resp2, _ := client.CompleteWithToolResults(ctx, system, contents, results, tools)
```

### LLMToolResponse Metadata

Gemini responses include thinking metadata for learning:

```go
type LLMToolResponse struct {
    ThoughtSummary   string   // Model's reasoning process
    ThoughtSignature string   // For multi-turn continuity
    ThinkingTokens   int      // Tokens used for reasoning
    GroundingSources []string // URLs from Google Search
}
```

### GroundingProvider Interface

For accessing grounding sources across the ecosystem, use the `types.GroundingProvider` interface:

```go
// Check if client supports grounding (GeminiClient does)
if gp, ok := client.(types.GroundingProvider); ok {
    sources := gp.GetLastGroundingSources()
    if len(sources) > 0 {
        // Display sources to user for transparency
    }
}
```

GeminiClient implements `GroundingProvider`:
- `GetLastGroundingSources()` - URLs used to ground the last response
- `IsGoogleSearchEnabled()` - Check if Google Search grounding is active
- `IsURLContextEnabled()` - Check if URL Context grounding is active

### Ecosystem Integration

When Gemini with grounding is used:
1. **Perception Layer**: Grounding sources captured during `CompleteWithSystem`
2. **Articulation Layer**: Sources extracted via `types.GroundingProvider` interface
3. **TUI Display**: Sources appended as "**Sources:**" section in response
4. **Logging**: Source counts logged in perception logs (`grounding_sources=N`)

## Testing

```bash
go test ./internal/perception/...
```
