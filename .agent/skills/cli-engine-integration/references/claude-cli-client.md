# Claude Code CLI Client Implementation

## Overview

The `ClaudeCodeCLIClient` implements the `LLMClient` and `SchemaCapableLLMClient` interfaces by executing the Claude Code CLI as a subprocess. It uses print mode (`-p`) with JSON output for programmatic parsing.

**Key Insight:** Claude CLI is used as an **LLM backend**, not an agent. codeNERD IS the agent - Claude provides intelligence only.

## CLI Command Format

### Basic Completion

```bash
claude -p --output-format json --model <model> --max-turns 1 --tools "" [--system-prompt "<system>"] "<prompt>"
```

### With JSON Schema (Piggyback Protocol)

```bash
claude -p --output-format json --model <model> --max-turns 3 --tools "" --system-prompt "<system>" --json-schema '<schema>' "<prompt>"
```

## Correct Flags (CRITICAL)

| Flag | Value | Purpose |
|------|-------|---------|
| `-p` | (required) | Print mode (non-interactive, exit after response) |
| `--output-format` | `json` | Return structured JSON response |
| `--model` | `sonnet`/`opus`/`haiku` | Model selection (aliases, not full IDs) |
| `--max-turns` | `1` or `3` | Single turn (3 for JSON schema retries) |
| `--tools` | `""` | **DISABLE all Claude Code tools** |
| `--system-prompt` | codeNERD prompt | **REPLACE** Claude Code system prompt |
| `--json-schema` | Piggyback schema | Guarantee structured output |

### Flags We DON'T Use

| Wrong Flag | Reason |
|------------|--------|
| `--allowed-tools` | Doesn't exist - use `--tools ""` |
| `--disallowed-tools` | Doesn't exist |
| `--fallback-model` | Doesn't exist - handle in Go code |
| `--mcp-config` | codeNERD has its own tool system |
| `--agents` | codeNERD IS the agent |
| `--append-system-prompt` | We REPLACE, not append |

## JSON Response Structure

### Standard Response (without JSON Schema)

```json
{
  "session_id": "abc123",
  "messages": [
    {
      "role": "user",
      "content": "..."
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "text",
          "text": "The actual response content"
        }
      ]
    }
  ],
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Final response"
      }
    ]
  },
  "cost_usd": 0.0012,
  "duration_ms": 1234,
  "num_turns": 1
}
```

### JSON Schema Response (CRITICAL DIFFERENCE)

When using `--json-schema`, the validated response is in `structured_output`:

```json
{
  "session_id": "abc123",
  "messages": [...],
  "result": {...},
  "structured_output": {
    "control_packet": {
      "intent_classification": {...},
      "mangle_updates": [...],
      "memory_operations": [...],
      "reasoning_trace": "..."
    },
    "surface_response": "User-facing text"
  },
  "cost_usd": 0.0012,
  "duration_ms": 1234
}
```

**Parse `structured_output` first when JSON schema is used, NOT `result`.**

## Piggyback Protocol JSON Schema

```go
const PiggybackEnvelopeSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["control_packet", "surface_response"],
  "additionalProperties": false,
  "properties": {
    "control_packet": {
      "type": "object",
      "required": ["intent_classification", "mangle_updates"],
      "additionalProperties": false,
      "properties": {
        "intent_classification": {
          "type": "object",
          "required": ["category", "verb", "target", "constraint", "confidence"],
          "additionalProperties": false,
          "properties": {
            "category": {"type": "string"},
            "verb": {"type": "string"},
            "target": {"type": "string"},
            "constraint": {"type": "string"},
            "confidence": {"type": "number", "minimum": 0, "maximum": 1}
          }
        },
        "mangle_updates": {
          "type": "array",
          "items": {"type": "string"},
          "description": "Mangle logic atoms to assert into the kernel"
        },
        "memory_operations": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["op", "key", "value"],
            "properties": {
              "op": {"type": "string", "enum": ["promote_to_long_term", "forget", "store_vector", "note"]},
              "key": {"type": "string"},
              "value": {"type": "string"}
            }
          }
        },
        "self_correction": {
          "type": "object",
          "properties": {
            "triggered": {"type": "boolean"},
            "hypothesis": {"type": "string"}
          }
        },
        "reasoning_trace": {"type": "string"}
      }
    },
    "surface_response": {
      "type": "string",
      "minLength": 1,
      "description": "User-facing natural language response"
    }
  }
}`
```

## Go Implementation

```go
package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"codenerd/internal/config"
)

// ClaudeCodeCLIClient implements LLMClient and SchemaCapableLLMClient.
type ClaudeCodeCLIClient struct {
	model         string
	fallbackModel string
	timeout       time.Duration
	maxTurns      int
}

// NewClaudeCodeCLIClient creates a new Claude CLI client.
func NewClaudeCodeCLIClient(cfg *config.ClaudeCLIConfig) *ClaudeCodeCLIClient {
	model := "sonnet"
	timeout := 300 * time.Second
	maxTurns := 1

	if cfg != nil {
		if cfg.Model != "" {
			model = cfg.Model
		}
		if cfg.Timeout > 0 {
			timeout = time.Duration(cfg.Timeout) * time.Second
		}
		if cfg.MaxTurns > 0 {
			maxTurns = cfg.MaxTurns
		}
	}

	return &ClaudeCodeCLIClient{
		model:         model,
		fallbackModel: cfg.FallbackModel,
		timeout:       timeout,
		maxTurns:      maxTurns,
	}
}

// Complete sends a prompt to Claude CLI and returns the response.
func (c *ClaudeCodeCLIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with system context to Claude CLI.
func (c *ClaudeCodeCLIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.execute(ctx, systemPrompt, userPrompt, "", c.model)
}

// CompleteWithSchema sends a prompt with JSON schema validation.
// This implements SchemaCapableLLMClient interface.
func (c *ClaudeCodeCLIClient) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	return c.execute(ctx, systemPrompt, userPrompt, jsonSchema, c.model)
}

// execute runs the Claude CLI with specified options.
func (c *ClaudeCodeCLIClient) execute(ctx context.Context, systemPrompt, userPrompt, jsonSchema, model string) (string, error) {
	// Build command args
	args := []string{
		"-p",
		"--output-format", "json",
		"--model", model,
		"--tools", "",  // DISABLE Claude Code tools - codeNERD has its own
	}

	// Set max-turns based on whether we're using JSON schema
	maxTurns := 1
	if jsonSchema != "" {
		maxTurns = 3  // Allow retries for schema validation
	}
	args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))

	// Add system prompt (REPLACES Claude Code instructions)
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	// Add JSON schema for structured output
	if jsonSchema != "" {
		args = append(args, "--json-schema", jsonSchema)
	}

	// Add the user prompt
	args = append(args, userPrompt)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Execute claude CLI
	cmd := exec.CommandContext(ctx, "claude", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Check for rate limit indicators
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "rate limit") || strings.Contains(stderrStr, "quota") {
			// Try fallback model if configured
			if c.fallbackModel != "" && model != c.fallbackModel {
				return c.execute(ctx, systemPrompt, userPrompt, jsonSchema, c.fallbackModel)
			}
			return "", &RateLimitError{
				Engine:  "claude-cli",
				Message: stderrStr,
			}
		}

		// Check for context cancellation
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("claude CLI timeout after %v", c.timeout)
		}

		return "", fmt.Errorf("claude CLI error: %v | stderr: %s", err, stderrStr)
	}

	// Parse JSON response
	return c.parseResponse(stdout.Bytes(), jsonSchema != "")
}

// claudeCLIResponse represents the JSON output structure.
type claudeCLIResponse struct {
	SessionID string `json:"session_id"`
	Messages  []struct {
		Role    string `json:"role"`
		Content any    `json:"content"` // Can be string or []ContentBlock
	} `json:"messages"`
	Result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	// CRITICAL: JSON Schema responses go here, NOT in Result
	StructuredOutput json.RawMessage `json:"structured_output,omitempty"`
	CostUSD          float64         `json:"cost_usd"`
	DurationMS       int             `json:"duration_ms"`
	NumTurns         int             `json:"num_turns"`
}

// parseResponse extracts the assistant's text from Claude CLI JSON output.
func (c *ClaudeCodeCLIClient) parseResponse(data []byte, hasSchema bool) (string, error) {
	var resp claudeCLIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("failed to parse claude CLI response: %w", err)
	}

	// CRITICAL: When using JSON schema, response is in structured_output
	if hasSchema && len(resp.StructuredOutput) > 0 {
		return string(resp.StructuredOutput), nil
	}

	// Extract text from result.content for non-schema responses
	var texts []string
	for _, block := range resp.Result.Content {
		if block.Type == "text" && block.Text != "" {
			texts = append(texts, block.Text)
		}
	}

	if len(texts) == 0 {
		// Fallback: try to get from last assistant message
		for i := len(resp.Messages) - 1; i >= 0; i-- {
			msg := resp.Messages[i]
			if msg.Role == "assistant" {
				switch content := msg.Content.(type) {
				case string:
					return content, nil
				case []interface{}:
					for _, block := range content {
						if blockMap, ok := block.(map[string]interface{}); ok {
							if blockMap["type"] == "text" {
								if text, ok := blockMap["text"].(string); ok {
									return text, nil
								}
							}
						}
					}
				}
			}
		}
		return "", fmt.Errorf("no text content in claude CLI response")
	}

	return strings.Join(texts, "\n"), nil
}
```

## Error Detection

### Rate Limit Patterns

Detect rate limits by checking stderr for:

- `"rate limit"`
- `"quota exceeded"`
- `"too many requests"`
- HTTP 429 status references

### Authentication Errors

Detect auth issues by checking for:

- `"not authenticated"`
- `"login required"`
- `"ANTHROPIC_API_KEY"`

## Testing

```go
func TestClaudeCodeCLIClient_Complete(t *testing.T) {
	// Skip if claude CLI not installed
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not installed")
	}

	client := NewClaudeCodeCLIClient(&config.ClaudeCLIConfig{
		Model:   "haiku", // Use fastest model for tests
		Timeout: 60,
	})

	ctx := context.Background()
	resp, err := client.Complete(ctx, "What is 2+2? Reply with just the number.")

	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if !strings.Contains(resp, "4") {
		t.Errorf("Expected response containing '4', got: %s", resp)
	}
}

func TestClaudeCodeCLIClient_CompleteWithSchema(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not installed")
	}

	client := NewClaudeCodeCLIClient(&config.ClaudeCLIConfig{
		Model:   "haiku",
		Timeout: 60,
	})

	schema := `{"type":"object","required":["answer"],"properties":{"answer":{"type":"integer"}}}`
	ctx := context.Background()
	resp, err := client.CompleteWithSchema(ctx, "You are a math assistant.", "What is 2+2?", schema)

	if err != nil {
		t.Fatalf("CompleteWithSchema failed: %v", err)
	}

	// Response should be valid JSON matching schema
	var result struct {
		Answer int `json:"answer"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		t.Fatalf("Response not valid JSON: %v", err)
	}

	if result.Answer != 4 {
		t.Errorf("Expected answer 4, got: %d", result.Answer)
	}
}
```

## Model Aliases

| Alias | Full Model ID | Subscription |
|-------|---------------|--------------|
| `sonnet` | `claude-sonnet-4-5-*` | Pro, Max |
| `opus` | `claude-opus-4-5-*` | Max only |
| `haiku` | `claude-haiku-4-5-*` | Pro, Max |

## Latency Characteristics

| Operation | Typical Latency |
|-----------|-----------------|
| CLI startup overhead | ~500ms-1s |
| Haiku response | ~1-2s total |
| Sonnet response | ~2-5s total |
| Opus response | ~5-15s total |
| With JSON schema | +1-2s (validation) |

## Limitations

1. **No streaming** - Print mode returns complete response only
2. **Single turn** - Use `--max-turns 1` (or 3 for schema) to prevent agentic behavior
3. **No tool use** - Tools disabled with `--tools ""`
4. **Subscription required** - CLI uses subscription quota, not API credits
5. **No `--fallback-model` flag** - Handle fallback in Go code
