package config

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"codenerd/internal/embedding"
)

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
	OllamaModel    string `yaml:"ollama_model" json:"ollama_model"`       // Required for provider=ollama; no default
	// Dimensions is the length of the vectors the configured model returns
	// (embeddinggemma and nomic-embed-text: 768). It sizes the sqlite-vec index,
	// so it is the user's to state, like the model: there is no default, and the
	// first vector that comes back a different length is an error that names
	// both numbers. Changing it means `nerd embedding reembed`.
	Dimensions int `yaml:"dimensions" json:"dimensions"`

	// GenAI Configuration (Google cloud embedding)
	GenAIAPIKey string `yaml:"genai_api_key" json:"genai_api_key"`
	GenAIModel  string `yaml:"genai_model" json:"genai_model"` // Required for provider=genai; no default

	// TaskType for GenAI embeddings:
	// SEMANTIC_SIMILARITY, CLASSIFICATION, CLUSTERING,
	// RETRIEVAL_DOCUMENT, RETRIEVAL_QUERY, CODE_RETRIEVAL_QUERY,
	// QUESTION_ANSWERING, FACT_VERIFICATION
	TaskType string `yaml:"task_type" json:"task_type"` // Default: "SEMANTIC_SIMILARITY"

	// RequestTimeout bounds one embedding HTTP call (a single Embed or
	// EmbedWithTask). It is a request bound, not a run clock. Default 60s,
	// the bound pattern-learning embeds carried as a literal. The Ollama
	// engine cannot import this package (config imports embedding), so
	// SetEmbeddingRequestTimeout also publishes the bound into the embedding
	// package, and the Ollama client's Timeout is that published value.
	RequestTimeout string `yaml:"request_timeout" json:"request_timeout,omitempty"`

	// PullTimeout bounds one Ollama model download (POST /api/pull,
	// stream:false). The response body is the download, so this is a request
	// bound, not a run clock. Default 30m, the bound that download carried
	// as a literal. It is separate from RequestTimeout: EnsureModel runs
	// under embed and boot contexts that are shorter than a model download,
	// and a cancelled caller still aborts the request through its context.
	// SetEmbeddingPullTimeout publishes the bound into the embedding package,
	// and the pull client's Timeout is that published value.
	PullTimeout string `yaml:"pull_timeout" json:"pull_timeout,omitempty"`
}

// ContextWindowConfig configures the semantic compression context window.
//
// Token Budget Architecture:
//
//	Total Model Context = InputBudget + OutputReserve + ThinkingReserve + ToolUseBuffer
//
// Where InputBudget is further divided by reserve percentages:
//
//	InputBudget = CoreReserve + AtomReserve + HistoryReserve + WorkingReserve
//
// Example for 200k context window:
//
//	MaxTokens: 200000 (input budget for context/prompt)
//	OutputReserve: 8000 (max response tokens)
//	ThinkingReserve: 0 (disabled by default, enable for extended thinking models)
//	ToolUseBuffer: 4000 (for tool call/response cycles)
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
		OutputReserve:          8000, // 8k output tokens
		ThinkingReserve:        0,    // Disabled by default
		ToolUseBuffer:          4000, // 4k for tool cycles
		RecentTurnWindow:       5,
		CompressionThreshold:   0.60,
		TargetCompressionRatio: 100.0,
		ActivationThreshold:    30.0,
	}
}

// MissingModel says what the embedding config still needs before an engine can
// be built, or "" when the configured provider has its model. No model is ever
// assumed: an embedding model that differs from the one the stored vectors were
// built with returns wrong neighbours without an error.
func (c *EmbeddingConfig) MissingModel() string {
	if c == nil {
		return "no embedding configuration: set embedding.provider and its model in .nerd/config.json"
	}
	switch c.Provider {
	case "ollama":
		if strings.TrimSpace(c.OllamaModel) == "" {
			return "no Ollama embedding model configured: name one (embedding.ollama_model), e.g. `nerd embedding set ollama <model>`"
		}
	case "genai":
		if strings.TrimSpace(c.GenAIModel) == "" {
			return "no GenAI embedding model configured: name one (embedding.genai_model), e.g. `nerd embedding set genai <api-key> <model>`"
		}
	}
	return ""
}

// DefaultEmbeddingConfig returns an EmbeddingConfig with sensible defaults.
//
// The defaults are the embedding package's own (embedding.DefaultConfig). This
// used to be a second hand-written copy of them, the mirror image of
// EngineConfig below: two literals for one set of values, where a default
// changed in the package that owns the engines would silently not reach the
// config every workspace starts from.
func DefaultEmbeddingConfig() *EmbeddingConfig {
	d := embedding.DefaultConfig()
	return &EmbeddingConfig{
		Provider:       d.Provider,
		OllamaEndpoint: d.OllamaEndpoint,
		OllamaModel:    d.OllamaModel,
		Dimensions:     d.Dimensions,
		GenAIAPIKey:    d.GenAIAPIKey,
		GenAIModel:     d.GenAIModel,
		TaskType:       d.TaskType,
		RequestTimeout: "60s",
		PullTimeout:    "30m",
	}
}

// ResolvedRequestTimeout parses RequestTimeout, filling the default when the
// field is absent. Check refuses a value that does not parse; this is the
// same parse for the process-wide install.
func (c EmbeddingConfig) ResolvedRequestTimeout() (time.Duration, error) {
	raw := strings.TrimSpace(c.RequestTimeout)
	if raw == "" {
		raw = DefaultEmbeddingConfig().RequestTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("embedding.request_timeout: %q is not a duration (want e.g. \"60s\", \"2m\"): %w", raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("embedding.request_timeout: %q must be positive", raw)
	}
	return d, nil
}

// activeEmbeddingRequestTimeout is the process-wide bound one embedding call
// may take, installed by LoadUserConfig the same way LLM timeouts are.
// Stored as nanoseconds.
var activeEmbeddingRequestTimeout atomic.Int64

func init() {
	installDefaultEmbeddingRequestTimeout()
	installDefaultEmbeddingPullTimeout()
}

func installDefaultEmbeddingRequestTimeout() {
	d, err := DefaultEmbeddingConfig().ResolvedRequestTimeout()
	if err != nil {
		panic("config: default embedding.request_timeout: " + err.Error())
	}
	SetEmbeddingRequestTimeout(d)
}

// SetEmbeddingRequestTimeout installs the process-wide embedding request bound.
// LoadUserConfig is the production caller; tests install and restore.
func SetEmbeddingRequestTimeout(d time.Duration) {
	if d <= 0 {
		panic("config: embedding request timeout must be positive")
	}
	activeEmbeddingRequestTimeout.Store(int64(d))
	// The Ollama client lives in a package that cannot import config.
	embedding.SetEmbedRequestTimeout(d)
}

// EmbeddingRequestTimeout is the installed embedding request bound. Without a
// load it is the default (60s).
func EmbeddingRequestTimeout() time.Duration {
	if n := activeEmbeddingRequestTimeout.Load(); n > 0 {
		return time.Duration(n)
	}
	d, err := DefaultEmbeddingConfig().ResolvedRequestTimeout()
	if err != nil {
		panic("config: default embedding.request_timeout: " + err.Error())
	}
	return d
}

// ResolvedPullTimeout parses PullTimeout, filling the default when the field
// is absent. Check refuses a value that does not parse; this is the same
// parse for the process-wide install.
func (c EmbeddingConfig) ResolvedPullTimeout() (time.Duration, error) {
	raw := strings.TrimSpace(c.PullTimeout)
	if raw == "" {
		raw = DefaultEmbeddingConfig().PullTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("embedding.pull_timeout: %q is not a duration (want e.g. \"30m\", \"45s\"): %w", raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("embedding.pull_timeout: %q must be positive", raw)
	}
	return d, nil
}

// activeEmbeddingPullTimeout is the process-wide bound one Ollama model pull
// may take, installed by LoadUserConfig. Stored as nanoseconds.
var activeEmbeddingPullTimeout atomic.Int64

func installDefaultEmbeddingPullTimeout() {
	d, err := DefaultEmbeddingConfig().ResolvedPullTimeout()
	if err != nil {
		panic("config: default embedding.pull_timeout: " + err.Error())
	}
	SetEmbeddingPullTimeout(d)
}

// SetEmbeddingPullTimeout installs the process-wide Ollama model-pull bound.
// LoadUserConfig is the production caller; tests install and restore.
func SetEmbeddingPullTimeout(d time.Duration) {
	if d <= 0 {
		panic("config: embedding pull timeout must be positive")
	}
	activeEmbeddingPullTimeout.Store(int64(d))
	// The Ollama pull client lives in a package that cannot import config.
	embedding.SetPullTimeout(d)
}

// EmbeddingPullTimeout is the installed Ollama model-pull bound. Without a
// load it is the default (30m).
func EmbeddingPullTimeout() time.Duration {
	if n := activeEmbeddingPullTimeout.Load(); n > 0 {
		return time.Duration(n)
	}
	d, err := DefaultEmbeddingConfig().ResolvedPullTimeout()
	if err != nil {
		panic("config: default embedding.pull_timeout: " + err.Error())
	}
	return d
}

// EngineConfig is the embedding engine configuration for these settings. It is
// the only conversion: twelve call sites used to copy the fields by hand, and
// the two that skipped the copy (the campaign document ingestor, a factory
// fallback) built engines from embedding.DefaultConfig and never read the
// user's file -- invisible while a default model existed.
func (c EmbeddingConfig) EngineConfig() embedding.Config {
	return embedding.Config{
		Provider:       c.Provider,
		OllamaEndpoint: c.OllamaEndpoint,
		OllamaModel:    c.OllamaModel,
		Dimensions:     c.Dimensions,
		GenAIAPIKey:    c.GenAIAPIKey,
		GenAIModel:     c.GenAIModel,
		TaskType:       c.TaskType,
	}
}
