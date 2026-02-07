package config

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
}

// CodexCLIConfig holds configuration for Codex CLI backend.
// Used when Engine="codex-cli" to execute Codex via subprocess.
//
// IMPORTANT: Codex CLI is used as a SUBPROCESS LLM API, not as an agent.
// - Sandbox is always "read-only" (codeNERD has its own Tactile Layer)
// - Single completion per call, no agentic loops
type CodexCLIConfig struct {
	// Model: "gpt-5.1-codex-max" (recommended), "gpt-5.1-codex-mini", "gpt-5.1",
	// "gpt-5.3-codex", "gpt-5.2-codex", "gpt-5-codex", "gpt-5", "o4-mini", "codex-mini-latest"
	Model string `json:"model,omitempty"`

	// Sandbox mode: "read-only" (default), "workspace-write"
	// Always use "read-only" with codeNERD since file ops go through Tactile Layer
	Sandbox string `json:"sandbox,omitempty"`

	// Timeout in seconds for CLI execution (default: 300)
	Timeout int `json:"timeout,omitempty"`

	// FallbackModel is used when the primary model is rate-limited or overloaded
	// Example: "o4-mini" as fallback for "gpt-5"
	// NOTE: This is handled in Go code, not via CLI flag
	FallbackModel string `json:"fallback_model,omitempty"`

	// Streaming enables real-time streaming output
	// When true, responses are streamed as they arrive
	Streaming bool `json:"streaming,omitempty"`

	// ReasoningEffortDefault overrides Codex CLI `model_reasoning_effort` when no
	// per-capability override applies. Example values seen in the wild: "low",
	// "medium", "high", "xhigh".
	ReasoningEffortDefault string `json:"reasoning_effort_default,omitempty"`

	// Per-capability reasoning effort overrides. These are selected based on the
	// shard's ModelCapability hint in the request context.
	ReasoningEffortHighReasoning string `json:"reasoning_effort_high_reasoning,omitempty"`
	ReasoningEffortBalanced      string `json:"reasoning_effort_balanced,omitempty"`
	ReasoningEffortHighSpeed     string `json:"reasoning_effort_high_speed,omitempty"`

	// DisableShellTool disables Codex CLI's shell tool execution. Default should
	// be true for codeNERD, since execution is handled by the Tactile layer.
	DisableShellTool *bool `json:"disable_shell_tool,omitempty"`

	// EnableOutputSchema enables Codex CLI `--output-schema` for Piggyback
	// structured outputs when we detect a Piggyback prompt.
	EnableOutputSchema *bool `json:"enable_output_schema,omitempty"`

	// ConfigOverrides allows passing additional `codex exec -c key=value` overrides.
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
type GeminiProviderConfig struct {
	// EnableThinking enables thinking/reasoning mode
	EnableThinking bool `json:"enable_thinking,omitempty"`

	// ThinkingLevel for Gemini 3: "minimal", "low", "medium", "high" (MUST be lowercase)
	// Default: "high" when thinking is enabled
	ThinkingLevel string `json:"thinking_level,omitempty"`

	// EnableGoogleSearch enables Google Search grounding
	// Responses will be grounded with real-time search results
	EnableGoogleSearch bool `json:"enable_google_search,omitempty"`

	// EnableURLContext enables the URL context tool
	// Allows including URLs for grounding (max 20 URLs)
	EnableURLContext bool `json:"enable_url_context,omitempty"`
}

// AntigravityProviderConfig holds Antigravity-specific configuration.
// Supports Google's internal Cloud Code IDE features.
type AntigravityProviderConfig struct {
	// EnableThinking enables thinking/reasoning mode
	EnableThinking bool `json:"enable_thinking,omitempty"`

	// ThinkingLevel for Gemini 3: "minimal", "low", "medium", "high" (MUST be lowercase)
	ThinkingLevel string `json:"thinking_level,omitempty"`

	// ProjectID overrides the Google Cloud Project ID (default: auto-detect)
	ProjectID string `json:"project_id,omitempty"`
}

// DefaultGeminiProviderConfig returns sensible defaults for Gemini 3 Flash Preview.
// Uses "high" thinking level for dynamic reasoning (Gemini 3 default).
// Available levels: "minimal", "low", "medium", "high"
func DefaultGeminiProviderConfig() *GeminiProviderConfig {
	return &GeminiProviderConfig{
		EnableThinking:     true,
		ThinkingLevel:      "high", // Dynamic reasoning - maximizes reasoning depth
		EnableGoogleSearch: true,
		EnableURLContext:   true,
	}
}
