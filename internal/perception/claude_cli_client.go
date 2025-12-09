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

// StreamChunk represents a chunk of streaming output from Claude CLI.
type StreamChunk struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Text    string `json:"text,omitempty"`
	Done    bool   `json:"done,omitempty"`
	Error   string `json:"error,omitempty"`
}

// StreamCallback is called for each chunk of streaming output.
// Return an error to abort the stream.
type StreamCallback func(chunk StreamChunk) error

// ClaudeCodeCLIClient implements LLMClient using the Claude Code CLI subprocess.
// It executes `claude -p --output-format json --model <model>` and parses the JSON response.
//
// Enhanced features (Claude CLI exclusive):
// - JSON Schema validation for structured output
// - Streaming output for real-time responses
// - Fallback model for rate limit resilience
// - MCP server integration
// - Tool control
type ClaudeCodeCLIClient struct {
	model           string
	fallbackModel   string
	timeout         time.Duration
	streaming       bool
	allowedTools    string
	disallowedTools string
	mcpConfig       string
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
	client := &ClaudeCodeCLIClient{
		model:   "sonnet",
		timeout: 300 * time.Second,
	}

	if cfg != nil {
		if cfg.Model != "" {
			client.model = cfg.Model
		}
		if cfg.Timeout > 0 {
			client.timeout = time.Duration(cfg.Timeout) * time.Second
		}
		client.fallbackModel = cfg.FallbackModel
		client.streaming = cfg.Streaming
		client.allowedTools = cfg.AllowedTools
		client.disallowedTools = cfg.DisallowedTools
		client.mcpConfig = cfg.MCPConfig
	}

	return client
}

// Complete sends a prompt to Claude Code CLI and returns the completion.
func (c *ClaudeCodeCLIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with an optional system message to Claude Code CLI.
// The system prompt is passed via --system-prompt flag for proper handling.
func (c *ClaudeCodeCLIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	opts := &ExecutionOptions{
		SystemPrompt: systemPrompt,
	}
	return c.executeWithOptions(ctx, userPrompt, opts)
}

// CompleteWithSchema sends a prompt and validates the response against a JSON schema.
// This is useful for Piggyback Protocol to ensure structured output.
//
// Example schema for Piggyback Protocol:
//
//	{
//	  "type": "object",
//	  "properties": {
//	    "surface": {"type": "string"},
//	    "control": {"type": "array", "items": {"type": "string"}}
//	  },
//	  "required": ["surface", "control"]
//	}
func (c *ClaudeCodeCLIClient) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	opts := &ExecutionOptions{
		SystemPrompt: systemPrompt,
		JSONSchema:   jsonSchema,
	}
	return c.executeWithOptions(ctx, userPrompt, opts)
}

// CompleteStreaming sends a prompt and streams the response in real-time.
// The callback is called for each chunk of output.
func (c *ClaudeCodeCLIClient) CompleteStreaming(ctx context.Context, systemPrompt, userPrompt string, callback StreamCallback) error {
	opts := &ExecutionOptions{
		SystemPrompt: systemPrompt,
		Streaming:    true,
	}
	return c.executeStreaming(ctx, userPrompt, opts, callback)
}

// ExecutionOptions configures a single execution.
type ExecutionOptions struct {
	SystemPrompt string
	JSONSchema   string
	Streaming    bool
}

// executeWithOptions runs the CLI with the given options.
func (c *ClaudeCodeCLIClient) executeWithOptions(ctx context.Context, prompt string, opts *ExecutionOptions) (string, error) {
	// Try primary model first
	response, err := c.executeCLI(ctx, prompt, c.model, opts)
	if err != nil {
		// If rate limited and we have a fallback, try it
		var rateLimitErr *RateLimitError
		if errors.As(err, &rateLimitErr) && c.fallbackModel != "" {
			response, err = c.executeCLI(ctx, prompt, c.fallbackModel, opts)
			if err != nil {
				return "", fmt.Errorf("fallback model also failed: %w", err)
			}
			return response, nil
		}
		return "", err
	}
	return response, nil
}

// executeCLI runs the claude CLI command and parses the JSON response.
func (c *ClaudeCodeCLIClient) executeCLI(ctx context.Context, prompt, model string, opts *ExecutionOptions) (string, error) {
	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Build command arguments
	args := c.buildArgs(prompt, model, opts)

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

// executeStreaming runs the CLI in streaming mode.
func (c *ClaudeCodeCLIClient) executeStreaming(ctx context.Context, prompt string, opts *ExecutionOptions, callback StreamCallback) error {
	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Force streaming output format
	opts.Streaming = true
	args := c.buildArgs(prompt, c.model, opts)

	// Create command with context for cancellation support
	cmd := exec.CommandContext(ctx, "claude", args...)

	// Get stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude CLI: %w", err)
	}

	// Read streaming output line by line
	scanner := bufio.NewScanner(stdout)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			// Try to extract text content even if not valid JSON
			chunk = StreamChunk{Type: "text", Text: line}
		}

		// Accumulate content
		if chunk.Text != "" {
			fullContent.WriteString(chunk.Text)
		} else if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
		}

		// Call the callback
		if err := callback(chunk); err != nil {
			// User requested abort
			cmd.Process.Kill()
			return err
		}

		if chunk.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("error reading stream: %w", err)
	}

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		stderrStr := stderr.String()
		if isRateLimitError(stderrStr) {
			return &RateLimitError{
				Provider:    "claude-cli",
				RawResponse: stderrStr,
			}
		}
		return fmt.Errorf("claude CLI streaming failed: %w (stderr: %s)", err, stderrStr)
	}

	return nil
}

// buildArgs constructs the CLI arguments.
func (c *ClaudeCodeCLIClient) buildArgs(prompt, model string, opts *ExecutionOptions) []string {
	args := []string{
		"-p", prompt,
		"--model", model,
	}

	// Output format
	if opts != nil && opts.Streaming {
		args = append(args, "--output-format", "stream-json")
	} else {
		args = append(args, "--output-format", "json")
	}

	// System prompt
	if opts != nil && opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}

	// JSON Schema for structured output
	if opts != nil && opts.JSONSchema != "" {
		args = append(args, "--json-schema", opts.JSONSchema)
	}

	// Tool control
	if c.allowedTools != "" {
		args = append(args, "--allowed-tools", c.allowedTools)
	}
	if c.disallowedTools != "" {
		args = append(args, "--disallowed-tools", c.disallowedTools)
	}

	// MCP configuration
	if c.mcpConfig != "" {
		args = append(args, "--mcp-config", c.mcpConfig)
	}

	// Fallback model for resilience
	if c.fallbackModel != "" {
		args = append(args, "--fallback-model", c.fallbackModel)
	}

	return args
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

// SetFallbackModel sets the fallback model for rate limit resilience.
func (c *ClaudeCodeCLIClient) SetFallbackModel(model string) {
	c.fallbackModel = model
}

// GetFallbackModel returns the current fallback model.
func (c *ClaudeCodeCLIClient) GetFallbackModel() string {
	return c.fallbackModel
}

// SetTimeout changes the timeout for CLI execution.
func (c *ClaudeCodeCLIClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// GetTimeout returns the current timeout.
func (c *ClaudeCodeCLIClient) GetTimeout() time.Duration {
	return c.timeout
}

// SetStreaming enables or disables streaming mode.
func (c *ClaudeCodeCLIClient) SetStreaming(enabled bool) {
	c.streaming = enabled
}

// IsStreaming returns whether streaming is enabled.
func (c *ClaudeCodeCLIClient) IsStreaming() bool {
	return c.streaming
}

// SetAllowedTools sets the allowed tools constraint.
func (c *ClaudeCodeCLIClient) SetAllowedTools(tools string) {
	c.allowedTools = tools
}

// SetDisallowedTools sets the disallowed tools constraint.
func (c *ClaudeCodeCLIClient) SetDisallowedTools(tools string) {
	c.disallowedTools = tools
}

// SetMCPConfig sets the MCP configuration file path.
func (c *ClaudeCodeCLIClient) SetMCPConfig(path string) {
	c.mcpConfig = path
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
