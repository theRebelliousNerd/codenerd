# internal/config - Configuration Management

This package provides centralized configuration for all codeNERD components. It handles YAML/JSON configuration loading, per-shard profiles, resource limits, and integration settings.

**Related Packages:**
- [cmd/nerd/config](../../cmd/nerd/config/) - CLI configuration loading
- [internal/core](../core/CLAUDE.md) - Kernel consuming limits
- [internal/prompt](../prompt/CLAUDE.md) - JIT compiler using JITConfig

## Architecture

Configuration is split into domain-specific files:
- **config.go** - Root Config struct aggregating all sections
- **user_config.go** - User-facing .nerd/config.json settings
- **shard.go** - Per-shard profiles (model, temperature, limits)
- **limits.go** - System-wide resource constraints

## File Index

| File | Description |
|------|-------------|
| `config.go` | Root Config struct aggregating all configuration sections with YAML loading. Exports Config, DefaultConfig(), LoadFromYAML() with validation and workspace path resolution. |
| `user_config.go` | User-facing configuration from .nerd/config.json for LLM providers and UI. Exports UserConfig with multi-provider API keys, engine selection, and transparency settings. |
| `llm.go` | LLM provider configuration for API and CLI backends. Exports LLMConfig, ClaudeCLIConfig, CodexCLIConfig with model selection, timeouts, and fallback settings. |
| `shard.go` | Per-shard profile configuration for model and resource allocation. Exports ShardProfile with temperature, context limits, execution timeout, and learning toggle per shard type. |
| `jit.go` | JIT Prompt Compiler configuration for dynamic prompt assembly. Exports JITConfig with token budget, reserved tokens, semantic search top-k, and debug mode settings. |
| `memory.go` | Memory shard and embedding engine configuration. Exports MemoryConfig, EmbeddingConfig, ContextWindowConfig with compression thresholds and budget allocation percentages. |
| `mangle.go` | Mangle kernel configuration for schema and policy paths. Exports MangleConfig with fact limit, query timeout, and path overrides for embedded defaults. |
| `limits.go` | System-wide resource constraints and enforcement. Exports CoreLimits with max memory, concurrent shards, API calls, session duration, and derived facts gas limit. |
| `execution.go` | Tactile interface execution configuration. Exports ExecutionConfig with allowed binaries, default timeout, working directory, and environment variable whitelist. |
| `integrations.go` | External service integration configuration. Exports IntegrationsConfig, CodeGraphIntegration, BrowserIntegration, ScraperIntegration with enable toggles and endpoints. |
| `tool_generation.go` | Ouroboros tool generation target platform configuration. Exports ToolGenerationConfig with target OS and architecture for cross-compilation. |
| `world.go` | World model scanning configuration for AST parsing concurrency. Exports WorldConfig with fast/deep worker counts, ignore patterns, and max file size for parsing. |
| `logging.go` | Logging configuration with per-category toggles. Exports LoggingConfig with debug mode master toggle, level, format, and IsCategoryEnabled() for selective logging. |
| `build.go` | Build environment configuration for go build/test commands. Exports BuildConfig with CGO environment variables, Go flags, and CGO package detection. |
| `ux.go` | User experience and onboarding configuration. Exports ExperienceLevel constants, OnboardingState, TransparencyConfig for progressive disclosure and visibility settings. |
| `llm_timeouts.go` | Centralized LLM timeout configuration with 3-tier timeout hierarchy. Exports LLMTimeouts struct, DefaultLLMTimeouts(), FastLLMTimeouts(), AggressiveLLMTimeouts(), and global GetLLMTimeouts()/SetLLMTimeouts(). |
| `config_test.go` | Unit tests for configuration loading and validation. Tests DefaultConfig(), YAML parsing, and limit validation. |

## Key Types

### Config
```go
type Config struct {
    Name           string
    Version        string
    LLM            LLMConfig
    Mangle         MangleConfig
    Memory         MemoryConfig
    Embedding      EmbeddingConfig
    Integrations   IntegrationsConfig
    Execution      ExecutionConfig
    ToolGeneration ToolGenerationConfig
    Logging        LoggingConfig
    ShardProfiles  map[string]ShardProfile
    DefaultShard   ShardProfile
    CoreLimits     CoreLimits
}
```

### UserConfig
```go
type UserConfig struct {
    Provider         string  // anthropic, openai, gemini, xai, zai, openrouter
    AnthropicAPIKey  string
    OpenAIAPIKey     string
    GeminiAPIKey     string
    Model            string
    Engine           string  // api, claude-cli, codex-cli
    Theme            string
    Onboarding       OnboardingState
    Transparency     TransparencyConfig
}
```

### ShardProfile
```go
type ShardProfile struct {
    Model               string   // claude-sonnet-4, glm-4.6, etc.
    Temperature         float64  // 0.0-1.0
    MaxContextTokens    int
    MaxOutputTokens     int
    MaxExecutionTimeSec int
    MaxRetries          int
    EnableLearning      bool
}
```

### CoreLimits
```go
type CoreLimits struct {
    MaxTotalMemoryMB      int  // Total RAM limit
    MaxConcurrentShards   int  // Max parallel shards
    MaxConcurrentAPICalls int  // Max simultaneous LLM calls
    MaxFactsInKernel      int  // EDB size limit
    MaxDerivedFactsLimit  int  // Mangle gas limit
}
```

## Configuration Sources

| Source | Priority | Purpose |
|--------|----------|---------|
| `.nerd/config.json` | Highest | User overrides |
| Environment variables | High | API keys, secrets |
| `.nerd/config.yaml` | Medium | Project config |
| DefaultConfig() | Lowest | Built-in defaults |

## Supported LLM Providers

| Provider | Default Model | Config Key |
|----------|---------------|------------|
| Z.AI | glm-4.6 | zai_api_key |
| Anthropic | claude-sonnet-4 | anthropic_api_key |
| OpenAI | gpt-5.1-codex-max | openai_api_key |
| Gemini | gemini-3-pro-preview | gemini_api_key |
| xAI | grok-2-latest | xai_api_key |
| OpenRouter | (various) | openrouter_api_key |

## Testing

```bash
go test ./internal/config/...
```

---

**Remember: Push to GitHub regularly!**
