package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

var codexExecCommandContext = exec.CommandContext

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

	skillEnabled   bool
	skillName      string
	skillAvailable bool
	skillPath      string
	workspaceRoot  string
	skillWarnOnce  sync.Once

	disableShellTool   bool
	enableOutputSchema bool

	reasoningEffortDefault       string
	reasoningEffortHighReasoning string
	reasoningEffortBalanced      string
	reasoningEffortHighSpeed     string

	configOverrides map[string]string
}

// NewCodexCLIClient creates a new Codex CLI client.
// If cfg is nil, defaults are applied (model: "gpt-5.4", sandbox: "read-only", timeout: 300s).
func NewCodexCLIClient(cfg *config.CodexCLIConfig) *CodexCLIClient {
	client := &CodexCLIClient{
		model:              "gpt-5.4",
		sandbox:            "read-only",
		timeout:            300 * time.Second,
		skillEnabled:       true,
		skillName:          config.DefaultCodexExecSkillName,
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
		if cfg.SkillEnabled != nil {
			client.skillEnabled = *cfg.SkillEnabled
		}
		if strings.TrimSpace(cfg.SkillName) != "" {
			client.skillName = strings.TrimSpace(cfg.SkillName)
		}

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

	client.workspaceRoot = codexExecWorkspaceRoot()
	client.skillPath, client.skillAvailable = codexExecSkillPath(client.workspaceRoot, client.skillName)
	if client.skillEnabled {
		if client.skillAvailable {
			logging.Perception("Codex CLI skill injection enabled: skill=%s path=%s", client.skillName, client.skillPath)
		} else {
			logging.PerceptionWarn("Codex CLI skill enabled but missing: skill=%s root=%s", client.skillName, client.workspaceRoot)
		}
	} else {
		logging.Perception("Codex CLI skill injection disabled: skill=%s", client.skillName)
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
	return c.executeWithFallback(ctx, combinedPrompt, nil)
}

// CompleteWithSchema sends a prompt with an explicit JSON schema for validation.
func (c *CodexCLIClient) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	combinedPrompt := c.buildPrompt(systemPrompt, userPrompt)

	var schema map[string]interface{}
	if strings.TrimSpace(jsonSchema) != "" {
		if err := json.Unmarshal([]byte(jsonSchema), &schema); err != nil {
			return "", fmt.Errorf("invalid JSON schema for codex-cli: %w", err)
		}
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
	model := c.ModelForContext(ctx)
	response, err := c.executeCLI(ctx, prompt, model, schema)
	if err != nil {
		var rateLimitErr *RateLimitError
		if errors.As(err, &rateLimitErr) && c.fallbackModel != "" {
			logging.PerceptionWarn("Codex CLI primary model rate-limited; trying fallback model=%s (primary=%s)", c.fallbackModel, model)
			response, err = c.executeCLI(ctx, prompt, c.fallbackModel, schema)
			if err != nil {
				logging.PerceptionError("Codex CLI fallback model exhausted: model=%s error=%v", c.fallbackModel, err)
				return "", fmt.Errorf("fallback model also failed: %w", err)
			}
			logging.Perception("Codex CLI fallback model succeeded: model=%s", c.fallbackModel)
			return response, nil
		}
		return "", err
	}
	return response, nil
}

// buildPrompt combines system and user prompts.
func (c *CodexCLIClient) buildPrompt(systemPrompt, userPrompt string) string {
	var prompt string
	if strings.TrimSpace(systemPrompt) != "" {
		prompt = fmt.Sprintf("<system_instructions>\n%s\n</system_instructions>\n\n%s", systemPrompt, userPrompt)
	} else {
		prompt = userPrompt
	}

	if !c.skillEnabled {
		return prompt
	}
	if !c.skillAvailable {
		c.skillWarnOnce.Do(func() {
			logging.PerceptionWarn("Codex CLI repo skill missing; falling back to legacy prompt path: skill=%s expected=%s", c.skillName, c.skillPath)
		})
		return prompt
	}

	return fmt.Sprintf("$%s\n\n%s", c.skillName, prompt)
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

// ModelForContext resolves the effective model for this request, preferring any
// per-shard override carried in the context over the client's default model.
func (c *CodexCLIClient) ModelForContext(ctx context.Context) string {
	if ctx != nil {
		if v := ctx.Value(types.CtxKeyModelName); v != nil {
			if model, ok := v.(string); ok && strings.TrimSpace(model) != "" {
				return strings.TrimSpace(model)
			}
		}
	}
	return c.model
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
	cmd := codexExecCommandContext(ctx, "codex", args...)
	if c.workspaceRoot != "" {
		cmd.Dir = c.workspaceRoot
	}
	cmd.Stdin = strings.NewReader(prompt)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

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
		stdoutStr := strings.TrimSpace(stdout.String())
		if isRateLimitError(stderrStr) {
			return "", &RateLimitError{
				Provider:    "codex-cli",
				RawResponse: stderrStr,
			}
		}
		if fallback := extractCodexExecAgentMessage(stdoutStr); strings.TrimSpace(fallback) != "" {
			logging.PerceptionWarn("Codex CLI returned non-zero exit but emitted a final agent message; using stdout JSONL fallback")
			return strings.TrimSpace(fallback), nil
		}

		diag := joinCodexExecDiagnostics(stderrStr, stdoutStr)
		return "", fmt.Errorf("codex CLI execution failed: %w (%s)", err, diag)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to read codex last-message file: %w", err)
	}
	result := strings.TrimSpace(string(data))
	if result == "" {
		if fallback := extractCodexExecAgentMessage(stdout.String()); strings.TrimSpace(fallback) != "" {
			logging.PerceptionWarn("Codex CLI wrote empty last-message output; recovered final agent message from stdout JSONL")
			return strings.TrimSpace(fallback), nil
		}
		diag := joinCodexExecDiagnostics(strings.TrimSpace(stderr.String()), strings.TrimSpace(stdout.String()))
		return "", fmt.Errorf("codex CLI produced empty last-message output (model=%s, %s)", model, diag)
	}

	return result, nil
}

// SchemaCapable reports whether the client can enforce response schemas.
func (c *CodexCLIClient) SchemaCapable() bool {
	return c.enableOutputSchema
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

// SkillEnabled reports whether repo-local Codex skill injection is enabled.
func (c *CodexCLIClient) SkillEnabled() bool { return c.skillEnabled }

// SkillName returns the configured Codex skill name.
func (c *CodexCLIClient) SkillName() string { return c.skillName }

// SkillPath returns the resolved repo-local skill path, if any.
func (c *CodexCLIClient) SkillPath() string { return c.skillPath }

// SkillAvailable reports whether the configured repo-local skill was found.
func (c *CodexCLIClient) SkillAvailable() bool { return c.skillAvailable }

// WorkspaceRoot returns the resolved workspace root for codex exec invocations.
func (c *CodexCLIClient) WorkspaceRoot() string { return c.workspaceRoot }

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

func codexExecWorkspaceRoot() string {
	root, err := config.FindWorkspaceRoot()
	if err == nil && root != "" {
		return root
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

func codexExecSkillPath(workspaceRoot, skillName string) (string, bool) {
	skillName = strings.TrimSpace(skillName)
	if workspaceRoot == "" || skillName == "" {
		return "", false
	}

	skillDir := filepath.Join(workspaceRoot, ".agents", "skills", skillName)
	skillDoc := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillDoc); err != nil {
		return skillDoc, false
	}
	return skillDoc, true
}

type codexExecJSONLEvent struct {
	Type string `json:"type"`
	Item *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func extractCodexExecAgentMessage(stdout string) string {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return ""
	}

	last := ""
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var event codexExecJSONLEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item != nil && event.Item.Type == "agent_message" && strings.TrimSpace(event.Item.Text) != "" {
			last = strings.TrimSpace(event.Item.Text)
		}
	}

	return last
}

func extractCodexExecFailureDetail(stdout string) string {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return ""
	}

	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var event codexExecJSONLEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Error != nil && strings.TrimSpace(event.Error.Message) != "" {
			return strings.TrimSpace(event.Error.Message)
		}
		if strings.TrimSpace(event.Message) != "" && (strings.Contains(strings.ToLower(event.Type), "fail") || strings.Contains(strings.ToLower(event.Type), "error")) {
			return strings.TrimSpace(event.Message)
		}
	}

	return ""
}

func joinCodexExecDiagnostics(stderrStr, stdoutStr string) string {
	parts := make([]string, 0, 2)
	if stderrStr != "" {
		parts = append(parts, fmt.Sprintf("stderr: %s", truncateString(stderrStr, 2000)))
	}
	if detail := extractCodexExecFailureDetail(stdoutStr); detail != "" {
		parts = append(parts, fmt.Sprintf("stdout: %s", truncateString(detail, 2000)))
	} else if stdoutStr != "" {
		parts = append(parts, fmt.Sprintf("stdout: %s", truncateString(stdoutStr, 2000)))
	}
	if len(parts) == 0 {
		return "no stderr/stdout diagnostics"
	}
	return strings.Join(parts, "; ")
}
