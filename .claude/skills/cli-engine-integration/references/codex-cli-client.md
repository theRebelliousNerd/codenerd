# Codex CLI Client Implementation

## Overview

The `CodexCLIClient` implements the `LLMClient` interface by executing the OpenAI Codex CLI as a subprocess. It uses exec mode with JSON output and stdin piping for prompts.

**Key Insight:** Like Claude CLI, Codex CLI is used as an **LLM backend**, not an agent. codeNERD IS the agent - Codex provides intelligence only.

## CLI Command Format

```bash
codex exec - --model <model> --sandbox <mode> --json --color never
```

The prompt is piped to stdin (indicated by `-`).

### Flags Used

| Flag | Purpose |
|------|---------|
| `exec` | Non-interactive execution mode |
| `-` | Read prompt from stdin |
| `--model <model>` | Model selection: `gpt-5`, `o4-mini`, etc. |
| `--sandbox <mode>` | Execution sandbox: `read-only`, `workspace-write` |
| `--json` | Output newline-delimited JSON events |
| `--color never` | Disable ANSI color codes |

**Important:** Always use `read-only` sandbox with codeNERD since file operations are handled by the Tactile Layer, not the LLM.

## JSON Event Stream (NDJSON)

The `--json` flag returns newline-delimited JSON events:

```jsonl
{"type":"message_start","message":{"role":"assistant"}}
{"type":"content_block_start","content_block":{"type":"text"}}
{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}
{"type":"content_block_delta","delta":{"type":"text_delta","text":" World"}}
{"type":"content_block_stop"}
{"type":"message_stop","message":{"role":"assistant","content":[{"type":"text","text":"Hello World"}]}}
```

### Key Event Types

| Event Type | Description |
|------------|-------------|
| `message_start` | Assistant message begins |
| `content_block_start` | Content block (text/tool_use) begins |
| `content_block_delta` | Incremental content update |
| `content_block_stop` | Content block ends |
| `message_stop` | Final message with complete content |

## System Prompt Handling

Codex CLI doesn't have a distinct `--system-prompt` flag in exec mode. System context is wrapped in XML tags:

```go
if systemPrompt != "" {
    combinedPrompt = fmt.Sprintf("<system_instructions>\n%s\n</system_instructions>\n\n%s", systemPrompt, userPrompt)
} else {
    combinedPrompt = userPrompt
}
```

This works because GPT models understand XML-like delimiters for instruction separation.

## Go Implementation

```go
package perception

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"codenerd/internal/config"
)

// CodexCLIClient implements LLMClient using Codex CLI subprocess.
type CodexCLIClient struct {
	model   string
	sandbox string
	timeout time.Duration
}

// NewCodexCLIClient creates a new Codex CLI client.
func NewCodexCLIClient(cfg *config.CodexCLIConfig) *CodexCLIClient {
	model := "gpt-5"
	sandbox := "read-only"
	timeout := 300 * time.Second

	if cfg != nil {
		if cfg.Model != "" {
			model = cfg.Model
		}
		if cfg.Sandbox != "" {
			sandbox = cfg.Sandbox
		}
		if cfg.Timeout > 0 {
			timeout = time.Duration(cfg.Timeout) * time.Second
		}
	}

	return &CodexCLIClient{
		model:   model,
		sandbox: sandbox,
		timeout: timeout,
	}
}

// Complete sends a prompt to Codex CLI and returns the response.
func (c *CodexCLIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with system context to Codex CLI.
func (c *CodexCLIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Codex CLI doesn't have a distinct --system flag in exec mode,
	// so wrap system prompt in XML-like tags that models understand.
	var combinedPrompt string
	if systemPrompt != "" {
		combinedPrompt = fmt.Sprintf("<system_instructions>\n%s\n</system_instructions>\n\n%s", systemPrompt, userPrompt)
	} else {
		combinedPrompt = userPrompt
	}

	// Build command args
	args := []string{
		"exec", "-",
		"--model", c.model,
		"--sandbox", c.sandbox,
		"--json",
		"--color", "never",
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Execute codex CLI
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Stdin = strings.NewReader(combinedPrompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Check for rate limit indicators
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "rate limit") ||
			strings.Contains(stderrStr, "quota") ||
			strings.Contains(stderrStr, "429") {
			return "", &RateLimitError{
				Engine:  "codex-cli",
				Message: stderrStr,
			}
		}

		// Check for context cancellation
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("codex CLI timeout after %v", c.timeout)
		}

		return "", fmt.Errorf("codex CLI error: %v | stderr: %s", err, stderrStr)
	}

	// Parse NDJSON response
	return c.parseNDJSON(stdout.Bytes())
}

// codexEvent represents a single event in the NDJSON stream.
type codexEvent struct {
	Type    string `json:"type"`
	Message *struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message,omitempty"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}

// parseNDJSON extracts the final response from Codex NDJSON output.
func (c *CodexCLIClient) parseNDJSON(data []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var finalText string
	var deltaTexts []string

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event codexEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue // Skip malformed lines
		}

		switch event.Type {
		case "message_stop":
			// Prefer complete message from message_stop
			if event.Message != nil {
				for _, block := range event.Message.Content {
					if block.Type == "text" {
						finalText = block.Text
					}
				}
			}

		case "content_block_delta":
			// Accumulate deltas as fallback
			if event.Delta != nil && event.Delta.Type == "text_delta" {
				deltaTexts = append(deltaTexts, event.Delta.Text)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to parse codex NDJSON: %w", err)
	}

	// Prefer message_stop content, fall back to accumulated deltas
	if finalText != "" {
		return finalText, nil
	}

	if len(deltaTexts) > 0 {
		return strings.Join(deltaTexts, ""), nil
	}

	return "", fmt.Errorf("no text content in codex CLI response")
}
```

## Comparison with Claude CLI

| Feature | Claude CLI | Codex CLI |
|---------|------------|-----------|
| System prompt | `--system-prompt` flag | XML tags in prompt |
| Output format | Single JSON object | NDJSON stream |
| Tool disable | `--tools ""` | `--sandbox read-only` |
| Turn limit | `--max-turns 1` | N/A (exec mode is single turn) |
| JSON Schema | `--json-schema` | Not available |

## SchemaCapableLLMClient Support

Unlike Claude CLI, Codex CLI **does not support JSON Schema validation**. For Piggyback Protocol compliance:

1. Include schema instructions in the system prompt
2. Use the emitter's fallback parsing (JSON → Markdown-wrapped → Embedded extraction)

```go
// Codex CLI does NOT implement SchemaCapableLLMClient
// Use regular CompleteWithSystem with schema instructions in prompt

systemPrompt := `You MUST respond in valid JSON matching this schema:
{
  "control_packet": {...},
  "surface_response": "..."
}
Do not include any text outside the JSON object.`
```

## Error Detection

### Rate Limit Patterns

Detect rate limits by checking stderr for:

- `"rate limit"`
- `"quota exceeded"`
- `"429"`
- `"too many requests"`

### Authentication Errors

Detect auth issues by checking for:

- `"not authenticated"`
- `"login required"`
- `"OPENAI_API_KEY"`

## Reasoning Trace Capture

Codex CLI with o-series models outputs reasoning tokens to stderr:

```go
// Optional: Capture reasoning trace for logging
if strings.Contains(c.model, "o") { // o4-mini, o3, etc.
	reasoningTrace := stderr.String()
	if reasoningTrace != "" {
		// Log reasoning trace for audit/debugging
		log.Debug().Str("reasoning", reasoningTrace).Msg("Codex reasoning trace")
	}
}
```

## Testing

```go
func TestCodexCLIClient_Complete(t *testing.T) {
	// Skip if codex CLI not installed
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex CLI not installed")
	}

	client := NewCodexCLIClient(&config.CodexCLIConfig{
		Model:   "codex-mini-latest", // Use fastest model for tests
		Sandbox: "read-only",
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
```

## Model Options

| Model | Description | Best For |
|-------|-------------|----------|
| `gpt-5` | Default, most capable | Complex coding tasks |
| `o4-mini` | Fast reasoning model | Quick iteration |
| `o3` | Advanced reasoning | Complex problem solving |
| `o3-mini` | Fast reasoning | Simple reasoning tasks |
| `codex-mini-latest` | Optimized for Codex | Codex-specific workflows |

## Sandbox Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `read-only` | Can only read files | Safe for codeNERD (recommended) |
| `workspace-write` | Can write to workspace | When Codex needs to edit |
| `danger-full-access` | Full system access | Never use with codeNERD |

**Important:** Always use `read-only` sandbox with codeNERD since file operations are handled by the Tactile Layer, not the LLM.

## Latency Characteristics

| Operation | Typical Latency |
|-----------|-----------------|
| CLI startup overhead | ~500ms-1s |
| codex-mini response | ~1-3s total |
| gpt-5 response | ~3-8s total |
| o4-mini response | ~2-5s total |

## Limitations

1. **No native system prompt** - System context wrapped in XML tags
2. **Sandbox required** - Must specify sandbox mode
3. **NDJSON parsing** - More complex than single JSON response
4. **No JSON Schema** - Can't guarantee structured output like Claude CLI
5. **Subscription limits** - ChatGPT Plus/Pro have 5-hour usage caps
