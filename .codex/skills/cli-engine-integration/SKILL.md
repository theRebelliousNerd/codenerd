---
name: cli-engine-integration
description: Integrate Claude Code CLI and OpenAI Codex CLI as subscription-based LLM backends for codeNERD. This skill should be used when the user asks to add or debug CLI-based LLM engines (Claude CLI, Codex CLI), configure engine authentication, implement subprocess clients or the LLMClient interface, or troubleshoot CLI engine selection, request/response handling, or streaming.
---

# CLI Engine Integration

## Overview

This skill enables codeNERD to use Claude Code CLI and OpenAI Codex CLI as LLM backends, allowing subscription-based execution (Claude Pro/Max, ChatGPT Plus/Pro) instead of pay-per-token API usage. The CLI engines implement the standard `LLMClient` interface via subprocess execution.

## Key Insight: Claude as LLM Backend, NOT Agent

**codeNERD IS the agent.** Claude CLI provides the intelligence (the model), while codeNERD provides the agentic framework (shards, kernel, tools).

We want:
- **Claude's intelligence** (the model)
- **NOT Claude Code's agentic capabilities** (tools, file editing)

This means:
- **DISABLE all Claude Code tools** - codeNERD has its own Tactile Layer
- **REPLACE system prompt** - Use Piggyback Protocol instructions, not Claude Code defaults
- **Single turn completion** - No agentic loops, codeNERD handles orchestration
- **JSON Schema validation** - Guarantee structured Piggyback output

## Architecture

```text
┌─────────────────────────────────────────────────────────────────┐
│                        codeNERD                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                   LLMClient Interface                     │  │
│  │    Complete(ctx, prompt) (string, error)                  │  │
│  │    CompleteWithSystem(ctx, sys, user) (string, error)     │  │
│  │    CompleteWithSchema(ctx, sys, user, schema) (string, error) │
│  └──────────────────────────────────────────────────────────┘  │
│                              │                                  │
│              ┌───────────────┼───────────────┐                 │
│              ▼               ▼               ▼                 │
│       ┌──────────┐    ┌──────────┐    ┌──────────┐            │
│       │ API Mode │    │Claude CLI│    │Codex CLI │            │
│       │(existing)│    │  Client  │    │  Client  │            │
│       └──────────┘    └──────────┘    └──────────┘            │
│              │               │               │                 │
└──────────────┼───────────────┼───────────────┼─────────────────┘
               │               │               │
               ▼               ▼               ▼
        ┌──────────┐    ┌──────────┐    ┌──────────┐
        │ HTTP API │    │ claude   │    │ codex    │
        │ Endpoint │    │   -p     │    │  exec    │
        └──────────┘    └──────────┘    └──────────┘
```

## Correct CLI Flags (CRITICAL)

### Claude Code CLI

| Flag | Value | Purpose |
|------|-------|---------|
| `-p` | (required) | Print mode - single completion, exit |
| `--tools` | `""` | **DISABLE all Claude Code tools** - codeNERD has its own |
| `--max-turns` | `1` or `3` | Single completion (3 for JSON schema retries) |
| `--system-prompt` | codeNERD prompt | **REPLACE** Claude Code instructions with Piggyback Protocol |
| `--json-schema` | Piggyback schema | Guarantee structured output |
| `--output-format` | `json` | Parse response programmatically |
| `--model` | `sonnet`/`opus`/`haiku` | Model selection |

**Flags we DON'T use:**
- `--mcp-config` - codeNERD doesn't need Claude's MCP
- `--agents` - codeNERD IS the agent framework
- `--append-system-prompt` - We REPLACE, not append
- `--allowed-tools` - Wrong flag name (use `--tools ""`)
- `--disallowed-tools` - Doesn't exist

### JSON Schema Mode Response

When using `--json-schema`, the response is in `structured_output` field:

```json
{
  "session_id": "abc123",
  "result": {...},
  "structured_output": {
    "control_packet": {...},
    "surface_response": "..."
  },
  "cost_usd": 0.0012
}
```

**NOT** in `result` field - this is a critical parsing detail.

## Configuration

The engine is configured in `.nerd/config.json`:

```json
{
  "engine": "codex-cli",
  "claude_cli": {
    "model": "sonnet",
    "timeout": 300,
    "max_turns": 3,
    "fallback_model": "haiku",
    "streaming": false
  },
  "codex_cli": {
    "model": "gpt-5.1-codex-max",
    "sandbox": "read-only",
    "timeout": 300,
    "fallback_model": "gpt-5.1-codex-mini",
    "streaming": false
  }
}
```

| Engine Value | Backend | Subscription Required |
|--------------|---------|----------------------|
| `"api"` (default) | HTTP API | API key |
| `"claude-cli"` | Claude Code CLI | Claude Pro/Max |
| `"codex-cli"` | Codex CLI | ChatGPT Plus/Pro |

## Quick Start

### Claude Code CLI Setup

```bash
# 1. Install Claude Code CLI
npm install -g @anthropic-ai/claude-code

# 2. Authenticate
claude login

# 3. Configure codeNERD
nerd auth claude
# Or manually: /config engine claude-cli
```

### Codex CLI Setup

```bash
# 1. Install Codex CLI
npm install -g @openai/codex

# 2. Authenticate
codex login

# 3. Configure codeNERD
nerd auth codex
# Or manually: /config engine codex-cli
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `nerd auth claude` | Authenticate and configure Claude CLI engine |
| `nerd auth codex` | Authenticate and configure Codex CLI engine |
| `/config engine` | Show current engine |
| `/config engine api` | Switch to API mode |
| `/config engine claude-cli` | Switch to Claude CLI |
| `/config engine codex-cli` | Switch to Codex CLI |

## Model Selection

### Claude Code CLI Models

| Model | Flag | Subscription |
|-------|------|--------------|
| Sonnet 4.5 | `--model sonnet` | Pro, Max |
| Opus 4.5 | `--model opus` | Max only |
| Haiku 4.5 | `--model haiku` | Pro, Max |

### Codex CLI Models

| Model | Flag | Description |
|-------|------|-------------|
| GPT-5.1-Codex-Max | `--model gpt-5.1-codex-max` | **Recommended** - Best for agentic coding |
| GPT-5.1-Codex-Mini | `--model gpt-5.1-codex-mini` | Cost-effective, faster |
| GPT-5.1 | `--model gpt-5.1` | General coding and reasoning |
| GPT-5-Codex | `--model gpt-5-codex` | Legacy agentic model |
| GPT-5 | `--model gpt-5` | Legacy general model |
| o4-mini | `--model o4-mini` | Fast reasoning (legacy) |
| codex-mini-latest | `--model codex-mini-latest` | Low-latency code Q&A |

## SchemaCapableLLMClient Interface

Extended interface for JSON Schema validation:

```go
// SchemaCapableLLMClient can validate responses against JSON schema
type SchemaCapableLLMClient interface {
    LLMClient
    CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error)
}

// Helper function
func AsSchemaCapable(client LLMClient) (SchemaCapableLLMClient, bool) {
    sc, ok := client.(SchemaCapableLLMClient)
    return sc, ok
}
```

## Rate Limit Handling

When CLI rate limits are hit, codeNERD displays a message prompting the user to switch engines:

```text
Claude CLI rate limit reached. Switch engine with /config engine codex-cli
```

Fallback model handling is done in Go code for both Claude CLI and Codex CLI:

```go
func (c *CodexCLIClient) executeWithOptions(ctx context.Context, prompt string, opts *CodexExecutionOptions) (string, error) {
    // Build combined prompt with system instructions
    combinedPrompt := c.buildPrompt(opts.SystemPrompt, prompt)

    // Try primary model first
    response, err := c.executeCLI(ctx, combinedPrompt, c.model)
    if err != nil {
        // If rate limited and we have a fallback, try it
        var rateLimitErr *RateLimitError
        if errors.As(err, &rateLimitErr) && c.fallbackModel != "" {
            return c.executeCLI(ctx, combinedPrompt, c.fallbackModel)
        }
        return "", err
    }
    return response, nil
}
```

## Streaming Support

Both CLI clients support streaming for real-time responses:

```go
// Codex CLI streaming
func (c *CodexCLIClient) CompleteStreaming(ctx context.Context, systemPrompt, userPrompt string, callback StreamCallback) error

// Claude CLI streaming
func (c *ClaudeCodeCLIClient) CompleteStreaming(ctx context.Context, systemPrompt, userPrompt string, callback StreamCallback) error

// Callback type
type StreamCallback func(chunk StreamChunk) error

type StreamChunk struct {
    Type    string `json:"type"`
    Content string `json:"content,omitempty"`
    Text    string `json:"text,omitempty"`
    Done    bool   `json:"done,omitempty"`
    Error   string `json:"error,omitempty"`
}
```

## Feature Comparison

| Feature | Claude CLI | Codex CLI |
|---------|-----------|-----------|
| Basic completion | Yes | Yes |
| System prompts | `--system-prompt` | XML wrapped |
| JSON Schema | `--json-schema` | No (prompt-based) |
| Streaming | Yes | Yes |
| Fallback model | Yes | Yes |
| Timeout handling | Yes | Yes |
| Rate limit detection | Yes | Yes |
| SchemaCapableLLMClient | Yes | No |

## Implementation Reference

For detailed Go implementation patterns, see:

- [references/claude-cli-client.md](references/claude-cli-client.md) - Claude CLI subprocess client with JSON schema
- [references/codex-cli-client.md](references/codex-cli-client.md) - Codex CLI subprocess client
- [references/config-schema.md](references/config-schema.md) - Extended config schema
- [references/prompt-management.md](references/prompt-management.md) - Kernel-driven prompt assembly

## Key Files

| File | Purpose |
|------|---------|
| `internal/perception/claude_cli_client.go` | Claude CLI LLMClient |
| `internal/perception/codex_cli_client.go` | Codex CLI LLMClient |
| `internal/config/config.go` | Config schema with engine fields |
| `internal/perception/client.go` | Client factory with engine routing |
| `internal/articulation/prompt_assembler.go` | Kernel-driven prompt composition |
| `internal/articulation/schema.go` | Piggyback JSON Schema |
| `cmd/nerd/main.go` | `nerd auth` CLI commands |
| `cmd/nerd/chat/commands.go` | `/config engine` command |

## Latency Considerations

CLI subprocess execution adds 1-3 seconds overhead per call compared to direct API. This is acceptable for:

- Code generation (heavy tasks)
- Complex reasoning
- Deep research

For latency-sensitive operations (intent classification, quick queries), consider using API mode with a cheaper model.
