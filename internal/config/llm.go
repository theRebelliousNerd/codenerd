package config

import "strings"

const (
	// DefaultCodexExecSkillName is the repo-local Codex skill injected into
	// codex exec prompts when skill support is enabled.
	DefaultCodexExecSkillName = "codenerd-codex-exec"

	// DefaultCodexMaxConcurrentCalls is the conservative default concurrency for
	// subscription-backed codex exec usage.
	DefaultCodexMaxConcurrentCalls = 2
)

// LLMConfig configures the LLM transducer.
type LLMConfig struct {
	Provider string `yaml:"provider"` // zai, anthropic, openai
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
	Timeout  string `yaml:"timeout"`
}

// ClaudeCLIConfig holds configuration for Claude Code CLI backend.
// Used when Engine="claude-cli" to execute Claude via subprocess.
//
// IMPORTANT: Claude CLI is used as a SUBPROCESS LLM API, not as an agent.
// - Claude Code tools are always DISABLED (codeNERD has its own tools)
// - System prompts REPLACE Claude Code instructions (not append)
// - MaxTurns defaults to 1 (single completion, no agentic loops)
type ClaudeCLIConfig struct {
	// Model alias: "sonnet", "opus", "haiku"
	Model string `json:"model,omitempty"`

	// Timeout in seconds for CLI execution (default: 300)
	Timeout int `json:"timeout,omitempty"`

	// FallbackModel is used when the primary model is rate-limited or overloaded
	// Example: "haiku" as fallback for "sonnet"
	// NOTE: This is handled in Go code, not via CLI flag (--fallback-model doesn't exist)
	FallbackModel string `json:"fallback_model,omitempty"`

	// MaxTurns limits the number of agentic turns (default: 1)
	// For codeNERD, this should always be 1 (single completion, no agentic loops)
	MaxTurns int `json:"max_turns,omitempty"`

	// Streaming enables real-time streaming output (--output-format stream-json)
	// When true, responses are streamed as they arrive
	Streaming bool `json:"streaming,omitempty"`

	// MaxConcurrentCalls optionally caps LLM concurrency under core_limits
	// (same pattern as codex_cli). 0 = use core_limits only.
	MaxConcurrentCalls int `json:"max_concurrent_calls,omitempty"`
}

// CodexCLIConfig holds configuration for Codex CLI backend.
// Used when Engine="codex-cli" to execute Codex via subprocess.
//
// IMPORTANT: Codex CLI is used as a SUBPROCESS LLM API, not as an agent.
// - Sandbox is always "read-only" (codeNERD has its own Tactile Layer)
// - Single completion per call, no agentic loops
type CodexCLIConfig struct {
	// Model is the Codex model, as the Codex CLI names it. Required: none is
	// filled in.
	Model string `json:"model,omitempty"`

	// Sandbox is retained for config compatibility but is forced to "read-only".
	// File operations must go through codeNERD's constitutional Tactile layer.
	Sandbox string `json:"sandbox,omitempty"`

	// Timeout in seconds for CLI execution (default: 300)
	Timeout int `json:"timeout,omitempty"`

	// FallbackModel is used when the primary model is rate-limited or
	// overloaded; empty means no fallback. Handled in Go code, not via a CLI
	// flag.
	FallbackModel string `json:"fallback_model,omitempty"`

	// Streaming enables real-time streaming output
	// When true, responses are streamed as they arrive
	Streaming bool `json:"streaming,omitempty"`

	// SkillEnabled controls whether codeNERD injects the repo-local Codex skill
	// into each `codex exec` invocation. Defaults to true.
	SkillEnabled *bool `json:"skill_enabled,omitempty"`

	// SkillName is the explicit Codex skill name injected into prompts.
	// Defaults to DefaultCodexExecSkillName.
	SkillName string `json:"skill_name,omitempty"`

	// MaxConcurrentCalls limits codex-cli requests beneath the global scheduler
	// ceiling. Defaults to DefaultCodexMaxConcurrentCalls for subscription-backed
	// Codex usage.
	MaxConcurrentCalls int `json:"max_concurrent_calls,omitempty"`

	// ReasoningEffortDefault is Codex CLI's `model_reasoning_effort` when no
	// per-capability override applies; unset is the main slot's
	// reasoning_effort (GetCodexCLIConfig), so every Codex request carries one.
	// Values seen in the wild: "low", "medium", "high", "xhigh".
	ReasoningEffortDefault string `json:"reasoning_effort_default,omitempty"`

	// Per-capability reasoning effort overrides. These are selected based on the
	// shard's ModelCapability hint in the request context.
	ReasoningEffortHighReasoning string `json:"reasoning_effort_high_reasoning,omitempty"`
	ReasoningEffortBalanced      string `json:"reasoning_effort_balanced,omitempty"`
	ReasoningEffortHighSpeed     string `json:"reasoning_effort_high_speed,omitempty"`

	// DisableShellTool is retained for config compatibility and is forced true.
	// Execution is handled by the constitutional Tactile layer.
	DisableShellTool *bool `json:"disable_shell_tool,omitempty"`

	// EnableOutputSchema enables Codex CLI `--output-schema` for Piggyback
	// structured outputs when we detect a Piggyback prompt.
	EnableOutputSchema *bool `json:"enable_output_schema,omitempty"`

	// ConfigOverrides allows a narrow presentation/reasoning allowlist of
	// `codex exec -c key=value` overrides. Execution, sandbox, shell, network,
	// approval, MCP, and tool overrides are ignored by the subprocess client.
	// Values are passed as raw TOML fragments (or literals if TOML parsing fails).
	// Example:
	//   {"personality": "\"friendly\"", "shell_environment_policy.inherit": "all"}
	ConfigOverrides map[string]string `json:"config_overrides,omitempty"`
}

// GeminiProviderConfig holds Gemini-specific configuration.
// Supports Gemini 3 Flash/Pro features: thinking mode, grounding tools.
//
// Thinking Mode:
//   - Use ThinkingLevel ("minimal", "low", "medium", "high")
//
// Built-in Tools:
//   - GoogleSearch: Enables grounding responses with Google Search results
//   - URLContext: Allows including URLs for context (max 20 URLs, 34MB each)
//
// Output Budget:
//   - MaxOutputTokens caps the response budget. Counts toward the same pool
//     thinking tokens consume, so when ThinkingLevel="high" the model can
//     burn most of the budget on thinking and leave very little for the
//     visible answer — which manifests as truncated piggyback envelopes.
//     If you see truncation in the chat surface, raise MaxOutputTokens or
//     lower ThinkingLevel to "medium".
type GeminiProviderConfig struct {
	// EnableThinking enables thinking/reasoning mode
	EnableThinking bool `json:"enable_thinking,omitempty"`

	// ThinkingLevel for Gemini 3: "minimal", "low", "medium", "high" (MUST be lowercase)
	// Default: "high" when thinking is enabled
	ThinkingLevel string `json:"thinking_level,omitempty"`

	// MaxOutputTokens caps the response budget (visible output + thinking).
	// 0 / unset means use the model's default. Gemini 3 supports up to
	// 65536. Lower this when you want to constrain cost; raise it (within
	// the model cap) when high-thinking calls are getting truncated.
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`

	// EnableGoogleSearch enables Google Search grounding
	// Responses will be grounded with real-time search results
	EnableGoogleSearch bool `json:"enable_google_search,omitempty"`

	// EnableURLContext enables the URL context tool
	// Allows including URLs for grounding (max 20 URLs)
	EnableURLContext bool `json:"enable_url_context,omitempty"`
}

// DefaultGeminiProviderConfig returns sensible defaults for Gemini 3 Flash Preview.
// Uses "high" thinking level for dynamic reasoning (Gemini 3 default).
// Available levels: "minimal", "low", "medium", "high"
func DefaultGeminiProviderConfig() *GeminiProviderConfig {
	return &GeminiProviderConfig{
		EnableThinking:     true,
		ThinkingLevel:      "high", // Dynamic reasoning - maximizes reasoning depth
		MaxOutputTokens:    65536,  // Gemini 3 max; counts thinking + visible output together
		EnableGoogleSearch: true,
		EnableURLContext:   true,
	}
}

// DefaultClaudeCLIConfig returns a ClaudeCLIConfig with sensible defaults.
// No model: an empty model means the Claude CLI's own configured model is
// used, so codeNERD never picks one on the user's behalf.
func DefaultClaudeCLIConfig() *ClaudeCLIConfig {
	return &ClaudeCLIConfig{
		Timeout:  300,
		MaxTurns: 1,
	}
}

// CLIModelLabel is how an engine's model is shown when the config leaves it
// empty and the CLI decides.
func CLIModelLabel(model string) string {
	if strings.TrimSpace(model) == "" {
		return "(CLI default)"
	}
	return model
}

// DefaultCodexCLIConfig returns a CodexCLIConfig with sensible defaults. No
// model, for the same reason as DefaultClaudeCLIConfig.
func DefaultCodexCLIConfig() *CodexCLIConfig {
	t := true
	return &CodexCLIConfig{
		Sandbox:            "read-only",
		Timeout:            300,
		SkillEnabled:       &t,
		SkillName:          DefaultCodexExecSkillName,
		MaxConcurrentCalls: DefaultCodexMaxConcurrentCalls,
		DisableShellTool:   &t,
		EnableOutputSchema: &t,
		ConfigOverrides:    make(map[string]string),
	}
}
