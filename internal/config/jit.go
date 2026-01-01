package config

import "encoding/json"

// JITConfig configures the JIT Prompt Compiler.
// The JIT compiler dynamically assembles system prompts from YAML atoms
// based on the current context (operational mode, shard type, language, etc.).
type JITConfig struct {
	// Enabled controls whether JIT compilation is used (default: true)
	// When false, falls back to static prompts
	Enabled bool `yaml:"enabled" json:"enabled"`

	// FallbackEnabled allows fallback to static prompts on JIT failure (default: true)
	FallbackEnabled bool `yaml:"fallback_enabled" json:"fallback_enabled"`

	// TokenBudget is the maximum tokens for compiled prompts (default: 200000)
	// Can be overridden via ContextWindow.MaxTokens in config.json
	TokenBudget int `yaml:"token_budget" json:"token_budget"`

	// ReservedTokens is tokens reserved for response generation (default: 8000)
	ReservedTokens int `yaml:"reserved_tokens" json:"reserved_tokens"`

	// DebugMode enables verbose JIT logging (default: false)
	DebugMode bool `yaml:"debug_mode" json:"debug_mode"`

	// TraceLLMIO logs full JIT prompts and LLM I/O when enabled (default: false)
	TraceLLMIO bool `yaml:"trace_llm_io" json:"trace_llm_io"`

	// SemanticTopK is the number of semantic search results to consider (default: 20)
	SemanticTopK int `yaml:"semantic_top_k" json:"semantic_top_k"`

	enabledSet         bool
	fallbackEnabledSet bool
}

// UnmarshalJSON tracks which boolean fields were explicitly set so defaults can apply.
func (c *JITConfig) UnmarshalJSON(data []byte) error {
	type alias JITConfig
	aux := struct {
		Enabled         *bool `json:"enabled"`
		FallbackEnabled *bool `json:"fallback_enabled"`
		*alias
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Enabled != nil {
		c.Enabled = *aux.Enabled
		c.enabledSet = true
	}
	if aux.FallbackEnabled != nil {
		c.FallbackEnabled = *aux.FallbackEnabled
		c.fallbackEnabledSet = true
	}
	return nil
}

// DefaultJITConfig returns sensible defaults for JIT compilation.
// Note: TokenBudget should be overridden from config.ContextWindow.MaxTokens if available.
func DefaultJITConfig() JITConfig {
	return JITConfig{
		Enabled:         true,
		FallbackEnabled: true,
		TokenBudget:     200000, // 200k tokens default
		ReservedTokens:  8000,
		DebugMode:       false,
		TraceLLMIO:      false,
		SemanticTopK:    20,
	}
}
