package perception

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// CodexCLIClient implements LLMClient using the Codex CLI subprocess.
//
// IMPORTANT: codeNERD uses Codex CLI as a SUBPROCESS LLM API, NOT as an agent.
// - Shell tool execution is disabled by default (codeNERD uses its own Tactile Layer)
// - Single completion per call (no agentic loops)
// - System prompt is embedded in user prompt via tags (Codex CLI has no --system flag)
//
// Implementation notes:
//   - Uses `codex exec -` and relies on `--output-last-message` for the final message
//     to avoid brittle JSONL event parsing across Codex CLI versions.
//   - Optionally uses `--output-schema` to enforce Piggyback structured output when detected.
type CodexCLIClient struct {
	model         string
	fallbackModel string
	sandbox       string
	timeout       time.Duration
	streaming     bool

	disableShellTool   bool
	enableOutputSchema bool

	reasoningEffortDefault       string
	reasoningEffortHighReasoning string
	reasoningEffortBalanced      string
	reasoningEffortHighSpeed     string

	configOverrides map[string]string
}

// NewCodexCLIClient creates a new Codex CLI client.
// If cfg is nil, defaults are applied (model: "gpt-5.3-codex", sandbox: "read-only", timeout: 300s).
func NewCodexCLIClient(cfg *config.CodexCLIConfig) *CodexCLIClient {
	client := &CodexCLIClient{
		model:              "gpt-5.3-codex",
		sandbox:            "read-only",
		timeout:            300 * time.Second,
		disableShellTool:   true,
		enableOutputSchema: true,
	}

	if cfg != nil {
		if cfg.Model != "" {
			client.model = cfg.Model
		}
		if cfg.Sandbox != "" {
			client.sandbox = cfg.Sandbox
		}
		if cfg.Timeout > 0 {
			client.timeout = time.Duration(cfg.Timeout) * time.Second
		}
		client.fallbackModel = cfg.FallbackModel
		client.streaming = cfg.Streaming

		if cfg.DisableShellTool != nil {
			client.disableShellTool = *cfg.DisableShellTool
		}
		if cfg.EnableOutputSchema != nil {
			client.enableOutputSchema = *cfg.EnableOutputSchema
		}

		client.reasoningEffortDefault = strings.TrimSpace(cfg.ReasoningEffortDefault)
		client.reasoningEffortHighReasoning = strings.TrimSpace(cfg.ReasoningEffortHighReasoning)
		client.reasoningEffortBalanced = strings.TrimSpace(cfg.ReasoningEffortBalanced)
		client.reasoningEffortHighSpeed = strings.TrimSpace(cfg.ReasoningEffortHighSpeed)

		if len(cfg.ConfigOverrides) > 0 {
			client.configOverrides = make(map[string]string, len(cfg.ConfigOverrides))
			for k, v := range cfg.ConfigOverrides {
				client.configOverrides[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}

	return client
}

// Complete sends a prompt to Codex CLI and returns the completion.
func (c *CodexCLIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with an optional system message to Codex CLI.
func (c *CodexCLIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	combinedPrompt := c.buildPrompt(systemPrompt, userPrompt)

	// Schema enforcement is only applied when we detect Piggyback protocol.
	var schema map[string]interface{}
	if c.enableOutputSchema && isPiggybackPrompt(systemPrompt, userPrompt) {
		schema = piggybackEnvelopeRawSchema()
	}

	return c.executeWithFallback(ctx, combinedPrompt, schema)
}

// CompleteStreaming sends a prompt and streams the response.
//
// Best-effort: Codex CLI emits JSONL events, but event schemas can change. We rely on
// --output-last-message for correctness and emit the final message as a single chunk.
func (c *CodexCLIClient) CompleteStreaming(ctx context.Context, systemPrompt, userPrompt string, callback StreamCallback) error {
	text, err := c.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) != "" {
		if err := callback(StreamChunk{Type: "text", Text: text}); err != nil {
			return err
		}
	}
	return callback(StreamChunk{Done: true})
}

// CompleteWithStreaming adapts callback-based streaming into channel-based streaming.
func (c *CodexCLIClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	go func() {
		defer close(contentChan)
		defer close(errorChan)

		err := c.CompleteStreaming(ctx, systemPrompt, userPrompt, func(chunk StreamChunk) error {
			if chunk.Error != "" {
				return fmt.Errorf("stream error: %s", chunk.Error)
			}
			delta := chunk.Text
			if delta == "" {
				delta = chunk.Content
			}
			if delta == "" || chunk.Done {
				return nil
			}
			select {
			case contentChan <- delta:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		if err != nil {
			errorChan <- err
		}
	}()

	return contentChan, errorChan
}

func (c *CodexCLIClient) executeWithFallback(ctx context.Context, prompt string, schema map[string]interface{}) (string, error) {
	response, err := c.executeCLI(ctx, prompt, c.model, schema)
	if err != nil {
		var rateLimitErr *RateLimitError
		if errors.As(err, &rateLimitErr) && c.fallbackModel != "" {
			response, err = c.executeCLI(ctx, prompt, c.fallbackModel, schema)
			if err != nil {
				return "", fmt.Errorf("fallback model also failed: %w", err)
			}
			return response, nil
		}
		return "", err
	}
	return response, nil
}

// buildPrompt combines system and user prompts.
func (c *CodexCLIClient) buildPrompt(systemPrompt, userPrompt string) string {
	if strings.TrimSpace(systemPrompt) != "" {
		return fmt.Sprintf("<system_instructions>\n%s\n</system_instructions>\n\n%s", systemPrompt, userPrompt)
	}
	return userPrompt
}

func isPiggybackPrompt(systemPrompt, userPrompt string) bool {
	// Keep this heuristic consistent with API clients. If we detect Piggyback, we can safely
	// enforce the Piggyback JSON schema via Codex CLI --output-schema.
	return strings.Contains(systemPrompt, "control_packet") ||
		strings.Contains(systemPrompt, "surface_response") ||
		strings.Contains(userPrompt, "PiggybackEnvelope") ||
		strings.Contains(userPrompt, "control_packet")
}

func (c *CodexCLIClient) reasoningEffortForContext(ctx context.Context) string {
	var capHint types.ModelCapability
	if v := ctx.Value(types.CtxKeyModelCapability); v != nil {
		switch vv := v.(type) {
		case types.ModelCapability:
			capHint = vv
		case string:
			capHint = types.ModelCapability(strings.TrimSpace(vv))
		}
	}

	switch capHint {
	case types.CapabilityHighReasoning:
		if c.reasoningEffortHighReasoning != "" {
			return c.reasoningEffortHighReasoning
		}
	case types.CapabilityHighSpeed:
		if c.reasoningEffortHighSpeed != "" {
			return c.reasoningEffortHighSpeed
		}
	case types.CapabilityBalanced:
		if c.reasoningEffortBalanced != "" {
			return c.reasoningEffortBalanced
		}
	}

	if c.reasoningEffortDefault != "" {
		return c.reasoningEffortDefault
	}
	return ""
}

func (c *CodexCLIClient) buildCLIArgs(ctx context.Context, model, outPath, schemaPath string) []string {
	args := []string{
		"exec", "-",
		"--model", model,
		"--sandbox", c.sandbox,
		"--color", "never",
		"--output-last-message", outPath,
		"--json",
	}
	if schemaPath != "" {
		args = append(args, "--output-schema", schemaPath)
	}
	if c.disableShellTool {
		// Defense-in-depth: prevent Codex from running shell commands at all.
		args = append(args, "--disable", "shell_tool")
	}

	// Build -c overrides (deterministic ordering for testability).
	overrides := make(map[string]string, len(c.configOverrides)+1)
	for k, v := range c.configOverrides {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		overrides[k] = v
	}

	// Per-shard reasoning multiplexing via context hint. Skip if user explicitly overrides.
	if _, ok := overrides["model_reasoning_effort"]; !ok {
		if effort := strings.TrimSpace(c.reasoningEffortForContext(ctx)); effort != "" {
			overrides["model_reasoning_effort"] = strconv.Quote(effort)
		}
	}

	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-c", fmt.Sprintf("%s=%s", k, overrides[k]))
	}

	return args
}

// executeCLI runs the codex CLI command with prompt piped to stdin and returns the last message.
func (c *CodexCLIClient) executeCLI(ctx context.Context, prompt, model string, schema map[string]interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	outFile, err := os.CreateTemp("", "codex-last-message-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp output file: %w", err)
	}
	outPath := outFile.Name()
	_ = outFile.Close()
	defer os.Remove(outPath)

	schemaPath := ""
	if schema != nil {
		f, err := os.CreateTemp("", "codex-output-schema-*.json")
		if err != nil {
			return "", fmt.Errorf("failed to create temp schema file: %w", err)
		}
		schemaPath = f.Name()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(schema); err != nil {
			f.Close()
			os.Remove(schemaPath)
			return "", fmt.Errorf("failed to write schema file: %w", err)
		}
		if err := f.Close(); err != nil {
			os.Remove(schemaPath)
			return "", fmt.Errorf("failed to close schema file: %w", err)
		}
		defer os.Remove(schemaPath)
	}

	args := c.buildCLIArgs(ctx, model, outPath, schemaPath)
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = io.Discard

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("codex CLI timed out after %v: %w", c.timeout, ctx.Err())
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", fmt.Errorf("codex CLI execution canceled: %w", ctx.Err())
		}

		stderrStr := strings.TrimSpace(stderr.String())
		if isRateLimitError(stderrStr) {
			return "", &RateLimitError{
				Provider:    "codex-cli",
				RawResponse: stderrStr,
			}
		}

		return "", fmt.Errorf("codex CLI execution failed: %w (stderr: %s)", err, truncateString(stderrStr, 2000))
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to read codex last-message file: %w", err)
	}
	result := strings.TrimSpace(string(data))
	if result == "" {
		return "", fmt.Errorf("codex CLI produced empty last-message output (model=%s)", model)
	}

	return result, nil
}

// SetModel changes the model used for completions.
func (c *CodexCLIClient) SetModel(model string) { c.model = model }

// GetModel returns the current model.
func (c *CodexCLIClient) GetModel() string { return c.model }

// SetFallbackModel sets the fallback model for rate limit resilience.
func (c *CodexCLIClient) SetFallbackModel(model string) { c.fallbackModel = model }

// GetFallbackModel returns the fallback model.
func (c *CodexCLIClient) GetFallbackModel() string { return c.fallbackModel }

// SetSandbox changes the sandbox mode.
func (c *CodexCLIClient) SetSandbox(sandbox string) { c.sandbox = sandbox }

// GetSandbox returns the current sandbox mode.
func (c *CodexCLIClient) GetSandbox() string { return c.sandbox }

// SetTimeout changes the timeout for CLI execution.
func (c *CodexCLIClient) SetTimeout(timeout time.Duration) { c.timeout = timeout }

// GetTimeout returns the current timeout.
func (c *CodexCLIClient) GetTimeout() time.Duration { return c.timeout }

// SetStreaming enables or disables streaming mode (best-effort).
func (c *CodexCLIClient) SetStreaming(streaming bool) { c.streaming = streaming }

// GetStreaming returns whether streaming is enabled.
func (c *CodexCLIClient) GetStreaming() bool { return c.streaming }

// CompleteWithTools sends a prompt with tool definitions.
// Codex CLI is used as a backend here; tools are requested via Piggyback control_packet.tool_requests.
func (c *CodexCLIClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	text, err := c.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	return &LLMToolResponse{
		Text:       text,
		StopReason: "end_turn",
	}, nil
}

// ShouldUsePiggybackTools returns true to instruct the system to use Piggyback Protocol
// for tool invocation. This is required for Codex CLI since codeNERD owns tool execution.
func (c *CodexCLIClient) ShouldUsePiggybackTools() bool { return true }
