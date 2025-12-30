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
└── codex_cli_client.go     # Codex CLI subprocess
```

## LLM Providers

| Provider | Config Key | Default Model |
|----------|------------|---------------|
| Z.AI | `zai_api_key` | `glm-4.7` |
| Anthropic | `anthropic_api_key` | `claude-sonnet-4` |
| OpenAI | `openai_api_key` | `gpt-5.1-codex-max` |
| Gemini | `gemini_api_key` | `gemini-3-pro-preview` |
| xAI | `xai_api_key` | `grok-3-beta` |
| OpenRouter | `openrouter_api_key` | (various) |
| Claude CLI | `engine: claude-cli` | (CLI default) |
| Codex CLI | `engine: codex-cli` | (CLI default) |

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
