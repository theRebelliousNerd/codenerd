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

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `client.go` | ~1700 | Multi-provider LLM client system (ZAI, Anthropic, OpenAI, Gemini, xAI, OpenRouter) |
| `transducer.go` | ~1200 | Main Transducer, VerbCorpus, intent parsing |
| `taxonomy.go` | ~400 | Intent taxonomy and classification |
| `taxonomy_persistence.go` | ~200 | Taxonomy persistence to disk |
| `learning.go` | ~300 | Autopoiesis learning from rejections |
| `tracing_client.go` | ~150 | Debug wrapper for LLM calls |
| `debug.go` | ~100 | Debug utilities |

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

| Provider | Config Key | Environment Variable | Default Model |
|----------|------------|---------------------|---------------|
| Z.AI | `zai_api_key` | `ZAI_API_KEY` | `glm-4.6` |
| Anthropic | `anthropic_api_key` | `ANTHROPIC_API_KEY` | `claude-sonnet-4-20250514` |
| OpenAI | `openai_api_key` | `OPENAI_API_KEY` | `gpt-4o` |
| Gemini | `gemini_api_key` | `GEMINI_API_KEY` | `gemini-3-pro-preview` |
| xAI | `xai_api_key` | `XAI_API_KEY` | `grok-3-beta` |
| OpenRouter | `openrouter_api_key` | `OPENROUTER_API_KEY` | `anthropic/claude-sonnet-4` |

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

## Testing

```bash
go test ./internal/perception/...
```
