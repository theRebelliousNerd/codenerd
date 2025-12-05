# internal/perception - Natural Language Understanding

This package implements the Perception layer of codeNERD - the Transducer that converts natural language into structured intents using the Piggyback Protocol.

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
| `transducer.go` | ~1200 | Main Transducer, VerbCorpus, intent parsing |
| `client.go` | ~1100 | LLMClient implementations (OpenAI, Anthropic) |

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
Code-changing operations: `/fix`, `/refactor`, `/create`, `/delete`, `/test`

### Instruction Verbs (/instruction)
Preference setting: `/configure`, `/always`, `/prefer`

## Parsing Flow

1. Input → `ParseIntent()`
2. Try Piggyback Protocol (JSON extraction)
3. Fallback to simple pipe-delimited parsing
4. Ultimate fallback: heuristic corpus matching

## Adding New Verbs

1. Add VerbEntry to VerbCorpus in `transducer.go`
2. Include synonyms, patterns, priority, and shardType
3. Update transducerSystemPrompt with new verb
4. Add action_mapping in `internal/mangle/policy.gl`

## Testing

```bash
go test ./internal/perception/...
```
