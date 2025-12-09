package perception

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"codenerd/internal/config"
)

// CodexCLIClient implements LLMClient using the Codex CLI subprocess.
// It executes `codex exec - --model <model> --sandbox <sandbox> --json --color never`
// and parses the NDJSON response stream.
type CodexCLIClient struct {
	model   string
	sandbox string
	timeout time.Duration
}

// codexNDJSONEvent represents an event in the Codex NDJSON stream.
// The stream contains events with types: "message_start", "content_block_delta", "message_stop".
type codexNDJSONEvent struct {
	Type    string `json:"type"`
	Message *struct {
		ID      string `json:"id,omitempty"`
		Role    string `json:"role,omitempty"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content,omitempty"`
	} `json:"message,omitempty"`
	Index int `json:"index,omitempty"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewCodexCLIClient creates a new Codex CLI client.
// If cfg is nil, defaults are applied (model: "gpt-5", sandbox: "read-only", timeout: 300s).
func NewCodexCLIClient(cfg *config.CodexCLIConfig) *CodexCLIClient {
	// Apply defaults
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

// Complete sends a prompt to Codex CLI and returns the completion.
func (c *CodexCLIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with an optional system message to Codex CLI.
// Since `codex exec` does not have a --system flag, the system prompt is wrapped
// in <system_instructions> tags and prepended to the user prompt.
func (c *CodexCLIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Build the combined prompt with system instructions wrapped in XML tags
	var combinedPrompt string
	if strings.TrimSpace(systemPrompt) != "" {
		combinedPrompt = fmt.Sprintf("<system_instructions>\n%s\n</system_instructions>\n\n%s", systemPrompt, userPrompt)
	} else {
		combinedPrompt = userPrompt
	}

	// Execute the CLI command
	response, err := c.executeCLI(ctx, combinedPrompt)
	if err != nil {
		return "", err
	}

	return response, nil
}

// executeCLI runs the codex CLI command with prompt piped to stdin and parses the NDJSON response.
func (c *CodexCLIClient) executeCLI(ctx context.Context, prompt string) (string, error) {
	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Build command arguments:
	// codex exec - --model <model> --sandbox <sandbox> --json --color never
	// The "-" flag tells codex to read the prompt from stdin
	args := []string{
		"exec", "-",
		"--model", c.model,
		"--sandbox", c.sandbox,
		"--json",
		"--color", "never",
	}

	// Create command with context for cancellation support
	cmd := exec.CommandContext(ctx, "codex", args...)

	// Set up stdin pipe for prompt input
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe for codex CLI: %w", err)
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start codex CLI: %w", err)
	}

	// Write prompt to stdin and close
	if _, err := io.WriteString(stdin, prompt); err != nil {
		return "", fmt.Errorf("failed to write prompt to codex CLI stdin: %w", err)
	}
	if err := stdin.Close(); err != nil {
		return "", fmt.Errorf("failed to close codex CLI stdin: %w", err)
	}

	// Wait for command to complete
	err = cmd.Wait()
	if err != nil {
		// Check for context cancellation
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("codex CLI timed out after %v: %w", c.timeout, ctx.Err())
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", fmt.Errorf("codex CLI execution canceled: %w", ctx.Err())
		}

		// Check if stderr contains rate limit information
		stderrStr := stderr.String()
		if isRateLimitError(stderrStr) {
			return "", &RateLimitError{
				Provider:    "codex-cli",
				RawResponse: stderrStr,
			}
		}

		// Return detailed error with stderr content
		return "", fmt.Errorf("codex CLI execution failed: %w (stderr: %s)", err, stderrStr)
	}

	// Parse the NDJSON response
	response, err := c.parseNDJSONResponse(stdout.Bytes())
	if err != nil {
		return "", fmt.Errorf("failed to parse codex CLI response: %w", err)
	}

	return response, nil
}

// parseNDJSONResponse extracts the assistant message text from the NDJSON event stream.
// It processes events line-by-line and extracts text from message_stop event's message.content[].text.
func (c *CodexCLIClient) parseNDJSONResponse(data []byte) (string, error) {
	if len(data) == 0 {
		return "", errors.New("empty response from codex CLI")
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))

	// Track accumulated text from content_block_delta events
	var deltaText strings.Builder

	// Track final message content from message_stop event
	var finalText string

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event codexNDJSONEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Skip malformed lines, continue processing
			continue
		}

		// Check for error events
		if event.Error != nil {
			// Check if error indicates rate limiting
			if strings.Contains(strings.ToLower(event.Error.Message), "rate limit") ||
				strings.Contains(strings.ToLower(event.Error.Type), "rate_limit") {
				return "", &RateLimitError{
					Provider:    "codex-cli",
					RawResponse: event.Error.Message,
				}
			}
			return "", fmt.Errorf("codex CLI error: %s (type: %s)", event.Error.Message, event.Error.Type)
		}

		switch event.Type {
		case "content_block_delta":
			// Accumulate text from delta events
			if event.Delta != nil && event.Delta.Type == "text_delta" {
				deltaText.WriteString(event.Delta.Text)
			}

		case "message_stop":
			// Extract final message content
			if event.Message != nil {
				var result strings.Builder
				for _, content := range event.Message.Content {
					if content.Type == "text" {
						result.WriteString(content.Text)
					}
				}
				finalText = result.String()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading NDJSON stream: %w", err)
	}

	// Prefer final message from message_stop, fallback to accumulated deltas
	result := strings.TrimSpace(finalText)
	if result == "" {
		result = strings.TrimSpace(deltaText.String())
	}

	if result == "" {
		return "", errors.New("no text content in codex CLI response")
	}

	return result, nil
}

// SetModel changes the model used for completions.
func (c *CodexCLIClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *CodexCLIClient) GetModel() string {
	return c.model
}

// SetSandbox changes the sandbox mode.
func (c *CodexCLIClient) SetSandbox(sandbox string) {
	c.sandbox = sandbox
}

// GetSandbox returns the current sandbox mode.
func (c *CodexCLIClient) GetSandbox() string {
	return c.sandbox
}

// SetTimeout changes the timeout for CLI execution.
func (c *CodexCLIClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// GetTimeout returns the current timeout.
func (c *CodexCLIClient) GetTimeout() time.Duration {
	return c.timeout
}
