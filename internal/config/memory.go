package config

// MemoryConfig configures the memory shards.
type MemoryConfig struct {
	// Shard A: Working Memory (RAM)
	WorkingMemorySize int `yaml:"working_memory_size"`

	// Shard B/C/D: SQLite storage
	DatabasePath string `yaml:"database_path"`

	// Session management
	SessionTTL string `yaml:"session_ttl"`

	// Context Window Management (§8.2 Semantic Compression)
	ContextWindow ContextWindowConfig `yaml:"context_window"`
}

// EmbeddingConfig configures the vector embedding engine.
// Supports Ollama (local) and GenAI (cloud) backends.
type EmbeddingConfig struct {
	// Provider: "ollama" or "genai"
	Provider string `yaml:"provider" json:"provider"`

	// Ollama Configuration (local embedding server)
	OllamaEndpoint string `yaml:"ollama_endpoint" json:"ollama_endpoint"` // Default: "http://localhost:11434"
	OllamaModel    string `yaml:"ollama_model" json:"ollama_model"`       // Default: "embeddinggemma"

	// GenAI Configuration (Google cloud embedding)
	GenAIAPIKey string `yaml:"genai_api_key" json:"genai_api_key"`
	GenAIModel  string `yaml:"genai_model" json:"genai_model"` // Default: "gemini-embedding-001"

	// TaskType for GenAI embeddings:
	// SEMANTIC_SIMILARITY, CLASSIFICATION, CLUSTERING,
	// RETRIEVAL_DOCUMENT, RETRIEVAL_QUERY, CODE_RETRIEVAL_QUERY,
	// QUESTION_ANSWERING, FACT_VERIFICATION
	TaskType string `yaml:"task_type" json:"task_type"` // Default: "SEMANTIC_SIMILARITY"
}

// ContextWindowConfig configures the semantic compression context window.
//
// Token Budget Architecture:
//   Total Model Context = InputBudget + OutputReserve + ThinkingReserve + ToolUseBuffer
//
// Where InputBudget is further divided by reserve percentages:
//   InputBudget = CoreReserve + AtomReserve + HistoryReserve + WorkingReserve
//
// Example for 200k context window:
//   MaxTokens: 200000 (input budget for context/prompt)
//   OutputReserve: 8000 (max response tokens)
//   ThinkingReserve: 0 (disabled by default, enable for extended thinking models)
//   ToolUseBuffer: 4000 (for tool call/response cycles)
type ContextWindowConfig struct {
	// Maximum tokens for input/context (prompt + context atoms + history)
	// This is the budget for what we SEND to the model, not the model's total context window.
	// Default: 200000
	MaxTokens int `yaml:"max_tokens" json:"max_tokens"`

	// Token budget allocation percentages (applied to MaxTokens)
	CoreReservePercent    int `yaml:"core_reserve_percent" json:"core_reserve_percent"`       // % for constitutional facts (default: 5)
	AtomReservePercent    int `yaml:"atom_reserve_percent" json:"atom_reserve_percent"`       // % for high-activation atoms (default: 30)
	HistoryReservePercent int `yaml:"history_reserve_percent" json:"history_reserve_percent"` // % for compressed history (default: 15)
	WorkingReservePercent int `yaml:"working_reserve_percent" json:"working_reserve_percent"` // % for working memory (default: 50)

	// Output token reserve - max tokens for model response (default: 8000)
	// This is passed as max_tokens to the LLM API
	OutputReserve int `yaml:"output_reserve" json:"output_reserve"`

	// Thinking token reserve - for extended thinking models (default: 0 = disabled)
	// Set to positive value for Claude models with extended thinking enabled
	// Recommended: 16000-32000 for complex reasoning tasks
	ThinkingReserve int `yaml:"thinking_reserve" json:"thinking_reserve"`

	// Tool use buffer - reserved for multi-turn tool call/response cycles (default: 4000)
	// Each tool call consumes tokens for: tool schema, parameters, and result
	ToolUseBuffer int `yaml:"tool_use_buffer" json:"tool_use_buffer"`

	// Recent turn window (how many turns to keep with full metadata)
	RecentTurnWindow int `yaml:"recent_turn_window" json:"recent_turn_window"`

	// Compression settings
	CompressionThreshold   float64 `yaml:"compression_threshold" json:"compression_threshold"`       // Trigger at this % usage (default: 0.60)
	TargetCompressionRatio float64 `yaml:"target_compression_ratio" json:"target_compression_ratio"` // Target ratio (default: 100.0)
	ActivationThreshold    float64 `yaml:"activation_threshold" json:"activation_threshold"`         // Min score to include (default: 30.0)
}

// TotalContextWindow returns the total tokens needed (input + output + thinking + tool buffer).
// Use this to validate against the model's actual context window limit.
func (c ContextWindowConfig) TotalContextWindow() int {
	total := c.MaxTokens
	if c.OutputReserve > 0 {
		total += c.OutputReserve
	} else {
		total += 8000 // Default output reserve
	}
	if c.ThinkingReserve > 0 {
		total += c.ThinkingReserve
	}
	if c.ToolUseBuffer > 0 {
		total += c.ToolUseBuffer
	} else {
		total += 4000 // Default tool buffer
	}
	return total
}

// EffectiveInputBudget returns the actual tokens available for input after reserves.
func (c ContextWindowConfig) EffectiveInputBudget() int {
	return c.MaxTokens
}

// DefaultContextWindowConfig returns sensible defaults for context window management.
func DefaultContextWindowConfig() ContextWindowConfig {
	return ContextWindowConfig{
		MaxTokens:              200000, // 200k tokens input budget
		CoreReservePercent:     5,
		AtomReservePercent:     30,
		HistoryReservePercent:  15,
		WorkingReservePercent:  50,
		OutputReserve:          8000,  // 8k output tokens
		ThinkingReserve:        0,     // Disabled by default
		ToolUseBuffer:          4000,  // 4k for tool cycles
		RecentTurnWindow:       5,
		CompressionThreshold:   0.60,
		TargetCompressionRatio: 100.0,
		ActivationThreshold:    30.0,
	}
}
