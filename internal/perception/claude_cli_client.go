package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"codenerd/internal/config"
)

// RateLimitError indicates the LLM provider returned a rate limit response.
// Callers can use errors.As to detect this error type and implement backoff.
type RateLimitError struct {
	Provider    string
	RetryAfter  time.Duration
	RawResponse string
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s rate limit exceeded, retry after %v", e.Provider, e.RetryAfter)
	}
	return fmt.Sprintf("%s rate limit exceeded", e.Provider)
}

// ClaudeCodeCLIClient implements LLMClient using the Claude Code CLI subprocess.
// It executes `claude -p --output-format json --model <model>` and parses the JSON response.
type ClaudeCodeCLIClient struct {
	model   string
	timeout time.Duration
}

// claudeCLIResponse represents the JSON output from `claude --output-format json`.
// The structure contains result.content[].text for assistant message text.
type claudeCLIResponse struct {
	Result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	// Rate limit indicator in error response
	IsRateLimited bool `json:"is_rate_limited,omitempty"`
}

// NewClaudeCodeCLIClient creates a new Claude Code CLI client.
// If cfg is nil, defaults are applied (model: "sonnet", timeout: 300s).
func NewClaudeCodeCLIClient(cfg *config.ClaudeCLIConfig) *ClaudeCodeCLIClient {
	// Apply defaults
	model := "sonnet"
	timeout := 300 * time.Second

	if cfg != nil {
		if cfg.Model != "" {
			model = cfg.Model
		}
		if cfg.Timeout > 0 {
			timeout = time.Duration(cfg.Timeout) * time.Second
		}
	}

	return &ClaudeCodeCLIClient{
		model:   model,
		timeout: timeout,
	}
}

// Complete sends a prompt to Claude Code CLI and returns the completion.
func (c *ClaudeCodeCLIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with an optional system message to Claude Code CLI.
// The system prompt is prepended to the user prompt since Claude CLI uses -p for prompt input.
func (c *ClaudeCodeCLIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Build the combined prompt
	var combinedPrompt string
	if strings.TrimSpace(systemPrompt) != "" {
		combinedPrompt = fmt.Sprintf("[System Instructions]\n%s\n\n[User Request]\n%s", systemPrompt, userPrompt)
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

// executeCLI runs the claude CLI command and parses the JSON response.
func (c *ClaudeCodeCLIClient) executeCLI(ctx context.Context, prompt string) (string, error) {
	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Build command arguments
	args := []string{
		"-p", prompt,
		"--output-format", "json",
		"--model", c.model,
	}

	// Create command with context for cancellation support
	cmd := exec.CommandContext(ctx, "claude", args...)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	err := cmd.Run()
	if err != nil {
		// Check for context cancellation
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("claude CLI timed out after %v: %w", c.timeout, ctx.Err())
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", fmt.Errorf("claude CLI execution canceled: %w", ctx.Err())
		}

		// Check if stderr contains rate limit information
		stderrStr := stderr.String()
		if isRateLimitError(stderrStr) {
			return "", &RateLimitError{
				Provider:    "claude-cli",
				RawResponse: stderrStr,
			}
		}

		// Return detailed error with stderr content
		return "", fmt.Errorf("claude CLI execution failed: %w (stderr: %s)", err, stderrStr)
	}

	// Parse the JSON response
	response, err := c.parseResponse(stdout.Bytes())
	if err != nil {
		return "", fmt.Errorf("failed to parse claude CLI response: %w", err)
	}

	return response, nil
}

// parseResponse extracts the assistant message text from the JSON response.
func (c *ClaudeCodeCLIClient) parseResponse(data []byte) (string, error) {
	if len(data) == 0 {
		return "", errors.New("empty response from claude CLI")
	}

	var resp claudeCLIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON response: %w (raw: %s)", err, truncateString(string(data), 500))
	}

	// Check for rate limit error in response
	if resp.IsRateLimited {
		return "", &RateLimitError{
			Provider:    "claude-cli",
			RawResponse: string(data),
		}
	}

	// Check for error in response
	if resp.Error != nil {
		// Check if error indicates rate limiting
		if strings.Contains(strings.ToLower(resp.Error.Message), "rate limit") ||
			strings.Contains(strings.ToLower(resp.Error.Type), "rate_limit") {
			return "", &RateLimitError{
				Provider:    "claude-cli",
				RawResponse: resp.Error.Message,
			}
		}
		return "", fmt.Errorf("claude CLI error: %s (type: %s)", resp.Error.Message, resp.Error.Type)
	}

	// Extract text from content blocks
	var result strings.Builder
	for _, content := range resp.Result.Content {
		if content.Type == "text" {
			result.WriteString(content.Text)
		}
	}

	text := strings.TrimSpace(result.String())
	if text == "" {
		return "", errors.New("no text content in claude CLI response")
	}

	return text, nil
}

// SetModel changes the model used for completions.
func (c *ClaudeCodeCLIClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *ClaudeCodeCLIClient) GetModel() string {
	return c.model
}

// SetTimeout changes the timeout for CLI execution.
func (c *ClaudeCodeCLIClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// GetTimeout returns the current timeout.
func (c *ClaudeCodeCLIClient) GetTimeout() time.Duration {
	return c.timeout
}

// isRateLimitError checks if the error message indicates a rate limit.
func isRateLimitError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	return strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "rate_limit") ||
		strings.Contains(lower, "too many requests") ||
		strings.Contains(lower, "429")
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
