# Extended Config Schema for CLI Engines

## Overview

This document describes the extensions to `.nerd/config.json` for CLI engine support, including the new fields for JSON schema validation, fallback models, and prompt management.

## New Fields

### Top-Level Fields

```go
type UserConfig struct {
    // Existing fields...
    Provider          string            `json:"provider,omitempty"`
    AnthropicAPIKey   string            `json:"anthropic_api_key,omitempty"`
    OpenAIAPIKey      string            `json:"openai_api_key,omitempty"`
    // ... other existing fields

    // CLI Engine Configuration
    Engine    string           `json:"engine,omitempty"`     // "api", "claude-cli", "codex-cli"
    ClaudeCLI *ClaudeCLIConfig `json:"claude_cli,omitempty"` // Claude CLI settings
    CodexCLI  *CodexCLIConfig  `json:"codex_cli,omitempty"`  // Codex CLI settings
}
```

### ClaudeCLIConfig

```go
// ClaudeCLIConfig holds configuration for Claude Code CLI backend.
type ClaudeCLIConfig struct {
    // Model alias: "sonnet", "opus", "haiku"
    Model string `json:"model,omitempty"`

    // Timeout in seconds (default: 300)
    Timeout int `json:"timeout,omitempty"`

    // FallbackModel to use on rate limit (Go-level retry)
    FallbackModel string `json:"fallback_model,omitempty"`

    // MaxTurns for CLI execution (default: 1, use 3 for JSON schema)
    MaxTurns int `json:"max_turns,omitempty"`

    // Streaming support (not currently used, for future)
    Streaming bool `json:"streaming,omitempty"`
}
```

### CodexCLIConfig

```go
// CodexCLIConfig holds configuration for Codex CLI backend.
type CodexCLIConfig struct {
    // Model: "gpt-5", "o4-mini", "o3", "codex-mini-latest"
    Model string `json:"model,omitempty"`

    // Sandbox mode: "read-only" (default), "workspace-write"
    Sandbox string `json:"sandbox,omitempty"`

    // Timeout in seconds (default: 300)
    Timeout int `json:"timeout,omitempty"`
}
```

## Complete JSON Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "engine": {
      "type": "string",
      "enum": ["api", "claude-cli", "codex-cli"],
      "default": "api",
      "description": "LLM backend engine"
    },
    "claude_cli": {
      "type": "object",
      "properties": {
        "model": {
          "type": "string",
          "enum": ["sonnet", "opus", "haiku"],
          "default": "sonnet",
          "description": "Claude model alias"
        },
        "timeout": {
          "type": "integer",
          "minimum": 30,
          "maximum": 600,
          "default": 300,
          "description": "Request timeout in seconds"
        },
        "fallback_model": {
          "type": "string",
          "enum": ["sonnet", "opus", "haiku"],
          "description": "Model to use on rate limit (Go-level fallback)"
        },
        "max_turns": {
          "type": "integer",
          "minimum": 1,
          "maximum": 10,
          "default": 1,
          "description": "Max turns for CLI (use 3 for JSON schema validation)"
        },
        "streaming": {
          "type": "boolean",
          "default": false,
          "description": "Enable streaming (future support)"
        }
      }
    },
    "codex_cli": {
      "type": "object",
      "properties": {
        "model": {
          "type": "string",
          "enum": ["gpt-5", "o4-mini", "o3", "o3-mini", "codex-mini-latest"],
          "default": "gpt-5",
          "description": "Codex model name"
        },
        "sandbox": {
          "type": "string",
          "enum": ["read-only", "workspace-write"],
          "default": "read-only",
          "description": "Sandbox execution mode"
        },
        "timeout": {
          "type": "integer",
          "minimum": 30,
          "maximum": 600,
          "default": 300,
          "description": "Request timeout in seconds"
        }
      }
    }
  }
}
```

## Example Configurations

### API Mode (Default)

```json
{
  "engine": "api",
  "provider": "anthropic",
  "anthropic_api_key": "sk-ant-...",
  "model": "claude-sonnet-4-5-20250929"
}
```

### Claude CLI Mode (Basic)

```json
{
  "engine": "claude-cli",
  "claude_cli": {
    "model": "sonnet",
    "timeout": 300
  }
}
```

### Claude CLI Mode (With Fallback and JSON Schema Support)

```json
{
  "engine": "claude-cli",
  "claude_cli": {
    "model": "opus",
    "timeout": 600,
    "fallback_model": "sonnet",
    "max_turns": 3
  }
}
```

### Codex CLI Mode

```json
{
  "engine": "codex-cli",
  "codex_cli": {
    "model": "gpt-5",
    "sandbox": "read-only",
    "timeout": 300
  }
}
```

### Full Config with All Options

```json
{
  "engine": "claude-cli",
  "claude_cli": {
    "model": "opus",
    "timeout": 600,
    "fallback_model": "sonnet",
    "max_turns": 3
  },
  "codex_cli": {
    "model": "o4-mini",
    "sandbox": "read-only",
    "timeout": 120
  },
  "provider": "anthropic",
  "anthropic_api_key": "sk-ant-...",
  "openai_api_key": "sk-...",
  "theme": "dark",
  "shard_profiles": {
    "coder": {
      "model": "claude-sonnet-4-5-20250929",
      "temperature": 0.7
    }
  }
}
```

## Go Implementation

### Config Struct Updates

Add to `internal/config/config.go`:

```go
// ClaudeCLIConfig holds configuration for Claude Code CLI backend.
type ClaudeCLIConfig struct {
    Model         string `json:"model,omitempty"`
    Timeout       int    `json:"timeout,omitempty"`
    FallbackModel string `json:"fallback_model,omitempty"`
    MaxTurns      int    `json:"max_turns,omitempty"`
    Streaming     bool   `json:"streaming,omitempty"`
}

// CodexCLIConfig holds configuration for Codex CLI backend.
type CodexCLIConfig struct {
    Model   string `json:"model,omitempty"`
    Sandbox string `json:"sandbox,omitempty"`
    Timeout int    `json:"timeout,omitempty"`
}

// Add to UserConfig struct
type UserConfig struct {
    // ... existing fields ...

    // CLI Engine Configuration
    Engine    string           `json:"engine,omitempty"`
    ClaudeCLI *ClaudeCLIConfig `json:"claude_cli,omitempty"`
    CodexCLI  *CodexCLIConfig  `json:"codex_cli,omitempty"`
}
```

### Helper Methods

```go
// GetEngine returns the configured engine, defaulting to "api".
func (c *UserConfig) GetEngine() string {
    if c.Engine == "" {
        return "api"
    }
    return c.Engine
}

// GetClaudeCLIConfig returns Claude CLI config with defaults.
func (c *UserConfig) GetClaudeCLIConfig() *ClaudeCLIConfig {
    if c.ClaudeCLI == nil {
        return &ClaudeCLIConfig{
            Model:    "sonnet",
            Timeout:  300,
            MaxTurns: 1,
        }
    }
    cfg := *c.ClaudeCLI
    if cfg.Model == "" {
        cfg.Model = "sonnet"
    }
    if cfg.Timeout == 0 {
        cfg.Timeout = 300
    }
    if cfg.MaxTurns == 0 {
        cfg.MaxTurns = 1
    }
    return &cfg
}

// GetCodexCLIConfig returns Codex CLI config with defaults.
func (c *UserConfig) GetCodexCLIConfig() *CodexCLIConfig {
    if c.CodexCLI == nil {
        return &CodexCLIConfig{
            Model:   "gpt-5",
            Sandbox: "read-only",
            Timeout: 300,
        }
    }
    cfg := *c.CodexCLI
    if cfg.Model == "" {
        cfg.Model = "gpt-5"
    }
    if cfg.Sandbox == "" {
        cfg.Sandbox = "read-only"
    }
    if cfg.Timeout == 0 {
        cfg.Timeout = 300
    }
    return &cfg
}

// SetEngine updates the engine and validates.
func (c *UserConfig) SetEngine(engine string) error {
    validEngines := map[string]bool{
        "api":        true,
        "claude-cli": true,
        "codex-cli":  true,
    }
    if !validEngines[engine] {
        return fmt.Errorf("invalid engine: %s (valid: api, claude-cli, codex-cli)", engine)
    }
    c.Engine = engine
    return nil
}
```

## Engine Selection Priority

When selecting the LLM client:

1. Check `engine` field in config
2. If `"claude-cli"` -> create `ClaudeCodeCLIClient`
3. If `"codex-cli"` -> create `CodexCLIClient`
4. If `"api"` or empty -> use existing provider-based selection

```go
func NewClientFromConfig(config *ProviderConfig) (LLMClient, error) {
    // CLI engines take priority
    switch config.Engine {
    case "claude-cli":
        return NewClaudeCodeCLIClient(config.ClaudeCLI), nil
    case "codex-cli":
        return NewCodexCLIClient(config.CodexCLI), nil
    }

    // Fall back to API-based providers
    switch config.Provider {
    case ProviderAnthropic:
        return NewAnthropicClient(config.APIKey), nil
    // ... other providers
    }
}
```

## SchemaCapableLLMClient Detection

When a shard needs JSON Schema validation:

```go
func executeWithPiggyback(client LLMClient, systemPrompt, userPrompt string) (string, error) {
    // Check if client supports JSON Schema
    if schemaClient, ok := core.AsSchemaCapable(client); ok {
        // Use schema validation for guaranteed Piggyback output
        return schemaClient.CompleteWithSchema(ctx, systemPrompt, userPrompt, articulation.PiggybackEnvelopeSchema)
    }

    // Fall back to prompt-based instructions
    enhancedSystem := systemPrompt + "\n\n" + piggybackInstructions
    return client.CompleteWithSystem(ctx, enhancedSystem, userPrompt)
}
```

## Migration Notes

Existing configs without `engine` field will continue to work - they default to `"api"` mode which uses the existing provider-based selection logic.

### Breaking Changes

None - all new fields are optional with sensible defaults.

### Deprecation Notes

The following fields are NOT used with CLI engines:

- `anthropic_api_key` - Not needed for Claude CLI (uses subscription)
- `openai_api_key` - Not needed for Codex CLI (uses subscription)

However, keeping these fields allows easy switching between API and CLI modes.
