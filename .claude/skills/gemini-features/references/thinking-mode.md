# Thinking Mode Reference

## Overview

Gemini's thinking mode enables explicit reasoning before generating responses. The model "thinks" through the problem step-by-step, improving accuracy for complex tasks.

## Gemini 3 vs Gemini 2.5

| Feature | Gemini 3 | Gemini 2.5 |
|---------|----------|------------|
| Control | `thinkingLevel` | `thinkingBudget` |
| Values | minimal/low/medium/high | 128-32768 tokens |
| Disable | Flash only (minimal) | budget=0 |
| Default | Dynamic (high) | Dynamic |

## Thinking Level (Gemini 3)

```go
type GeminiThinkingConfig struct {
    IncludeThoughts bool   `json:"includeThoughts,omitempty"`
    ThinkingLevel   string `json:"thinkingLevel,omitempty"`
}
```

**Levels (MUST be lowercase):**

| Level | Description | Availability |
|-------|-------------|--------------|
| `minimal` | Minimal reasoning overhead | Flash only |
| `low` | Light reasoning | Pro, Flash |
| `medium` | Moderate reasoning | Flash only |
| `high` | Maximum reasoning depth | Pro, Flash (default) |

**codeNERD default:** `"high"` for maximum reasoning quality.

## Thinking Budget (Gemini 2.5)

```go
type GeminiThinkingConfig struct {
    IncludeThoughts bool `json:"includeThoughts,omitempty"`
    ThinkingBudget  int  `json:"thinkingBudget,omitempty"`
}
```

| Value | Meaning |
|-------|---------|
| 0 | Disable thinking |
| -1 | Dynamic (model decides) |
| 128-32768 | Token budget for reasoning |

## Implementation

```go
func (c *GeminiClient) buildThinkingConfig() *GeminiThinkingConfig {
    if !c.enableThinking {
        return nil
    }

    cfg := &GeminiThinkingConfig{
        IncludeThoughts: true,
    }

    // Gemini 3: use thinkingLevel
    if c.thinkingLevel != "" {
        cfg.ThinkingLevel = c.thinkingLevel
    }

    // Gemini 2.5: use thinkingBudget (if specified)
    if c.thinkingBudget > 0 {
        cfg.ThinkingBudget = c.thinkingBudget
    }

    return cfg
}
```

## REST API Format

```bash
curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent" \
  -H "x-goog-api-key: $GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"parts": [{"text": "Explain quantum computing"}]}],
    "generationConfig": {
      "thinkingConfig": {
        "includeThoughts": true,
        "thinkingLevel": "high"
      }
    }
  }'
```

## Response with Thoughts

When `includeThoughts: true`, the response includes:

```go
type GeminiResponse struct {
    ThoughtSummary string `json:"thoughtSummary,omitempty"` // Summary of reasoning
    UsageMetadata  struct {
        ThoughtsTokenCount int `json:"thoughtsTokenCount,omitempty"` // Tokens used
    }
}
```

**Note:** The `thoughtSummary` is a compressed summary, not the full reasoning. Token billing includes full thinking tokens.

## Token Billing

```
Total cost = Output tokens + Thinking tokens
```

Access via `response.UsageMetadata.ThoughtsTokenCount`.

## Best Practices

1. **Use high for complex reasoning** - Mathematical proofs, code analysis, multi-step logic
2. **Use low for simple tasks** - Summarization, translation, formatting
3. **Monitor token usage** - Thinking tokens add to cost
4. **Capture thought summaries** - Store in ExecutionRecord for learning

## codeNERD Integration

```go
// Config: internal/config/llm.go
type GeminiProviderConfig struct {
    EnableThinking bool   `json:"enable_thinking,omitempty"`
    ThinkingLevel  string `json:"thinking_level,omitempty"` // lowercase!
    ThinkingBudget int    `json:"thinking_budget,omitempty"`
}

// Default: internal/config/llm.go
func DefaultGeminiProviderConfig() *GeminiProviderConfig {
    return &GeminiProviderConfig{
        EnableThinking: true,
        ThinkingLevel:  "high", // Must be lowercase
    }
}
```

## Streaming with Thoughts

For streaming responses, thoughts appear in parts:

```go
for chunk := range streamChan {
    for _, part := range chunk.Candidates[0].Content.Parts {
        if part.Thought {
            // This is a thought summary part
            log.Printf("Thinking: %s", part.Text)
        } else {
            // This is the actual response
            output.WriteString(part.Text)
        }
    }
}
```
