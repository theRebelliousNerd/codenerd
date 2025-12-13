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
type ContextWindowConfig struct {
	// Maximum tokens for the context window (default: 128000)
	MaxTokens int `yaml:"max_tokens" json:"max_tokens"`

	// Token budget allocation percentages
	CoreReservePercent    int `yaml:"core_reserve_percent" json:"core_reserve_percent"`       // % for constitutional facts (default: 5)
	AtomReservePercent    int `yaml:"atom_reserve_percent" json:"atom_reserve_percent"`       // % for high-activation atoms (default: 30)
	HistoryReservePercent int `yaml:"history_reserve_percent" json:"history_reserve_percent"` // % for compressed history (default: 15)
	WorkingReservePercent int `yaml:"working_reserve_percent" json:"working_reserve_percent"` // % for working memory (default: 50)

	// Recent turn window (how many turns to keep with full metadata)
	RecentTurnWindow int `yaml:"recent_turn_window" json:"recent_turn_window"`

	// Compression settings
	CompressionThreshold   float64 `yaml:"compression_threshold" json:"compression_threshold"`       // Trigger at this % usage (default: 0.60)
	TargetCompressionRatio float64 `yaml:"target_compression_ratio" json:"target_compression_ratio"` // Target ratio (default: 100.0)
	ActivationThreshold    float64 `yaml:"activation_threshold" json:"activation_threshold"`         // Min score to include (default: 30.0)
}
