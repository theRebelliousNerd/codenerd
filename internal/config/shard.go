package config

// ShardProfile defines per-shard configuration.
// Each shard type (coder, tester, reviewer, researcher) can have custom settings.
type ShardProfile struct {
	// Model Configuration
	Model       string  `yaml:"model" json:"model"`             // "claude-sonnet-4", "claude-opus-4", etc.
	Temperature float64 `yaml:"temperature" json:"temperature"` // 0.0-1.0
	TopP        float64 `yaml:"top_p" json:"top_p"`             // 0.0-1.0

	// Context Limits (per-shard)
	MaxContextTokens int `yaml:"max_context_tokens" json:"max_context_tokens"` // Shard-specific context limit
	MaxOutputTokens  int `yaml:"max_output_tokens" json:"max_output_tokens"`   // Max generation length

	// Execution Limits
	MaxExecutionTimeSec int `yaml:"max_execution_time_sec" json:"max_execution_time_sec"` // Timeout per task
	MaxRetries          int `yaml:"max_retries" json:"max_retries"`                       // Retry limit on failure

	// Memory Limits (per-shard kernel)
	MaxFactsInShardKernel int `yaml:"max_facts_in_shard_kernel" json:"max_facts_in_shard_kernel"` // EDB limit

	// Autopoiesis (learning) enabled for this shard?
	EnableLearning bool `yaml:"enable_learning" json:"enable_learning"`
}

// applyShardDefaults fills in zero values with defaults.
func applyShardDefaults(p ShardProfile) ShardProfile {
	if p.Model == "" {
		p.Model = "glm-4.7"
	}
	if p.Temperature == 0 {
		p.Temperature = 0.7
	}
	if p.TopP == 0 {
		p.TopP = 0.9
	}
	if p.MaxContextTokens == 0 {
		p.MaxContextTokens = 20000
	}
	if p.MaxOutputTokens == 0 {
		p.MaxOutputTokens = 4000
	}
	if p.MaxExecutionTimeSec == 0 {
		p.MaxExecutionTimeSec = 300
	}
	if p.MaxRetries == 0 {
		p.MaxRetries = 3
	}
	if p.MaxFactsInShardKernel == 0 {
		p.MaxFactsInShardKernel = 20000
	}
	return p
}
