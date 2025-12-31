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
	// "gpt-5-codex", "gpt-5", "o4-mini", "codex-mini-latest"
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
