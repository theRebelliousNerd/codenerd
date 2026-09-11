package config

// ShardProfile defines per-shard configuration.
// Each shard type (coder, tester, reviewer, researcher) can have custom settings.
type ShardProfile struct {
	// Model Configuration
	// Model is the model tag used by the worker/main LLM client for this shard
	// (e.g. gemma4:12b on Ollama, or grok-4.5 on xAI). Prefer config.worker for
	// provider selection; this field is the model name within that client.
	// Model, when set, overrides the client's model for this shard's calls
	// (types.WithModelName). Empty means the client's configured model; it
	// used to default to "glm-4.7" regardless of provider.
	Model string `yaml:"model" json:"model"`

	// Sampling. Zero means "not configured": the client keeps its own
	// default for that parameter. A configured value rides on the request
	// context (types.WithSampling) and every client's request builder
	// honours it. These were collected and persisted for months while no
	// client read them; they went live on 2026-09-11.
	Temperature float64 `yaml:"temperature" json:"temperature"` // 0.0-1.0
	TopP        float64 `yaml:"top_p" json:"top_p"`             // 0.0-1.0

	// There is no per-shard context or output ceiling. Every shard runs on
	// the worker (or main) client, whose completion ceiling is
	// max_output_tokens at the top level or on the worker/planner slot, and
	// its prompt budget is jit.token_budget / context_window.max_tokens. A
	// per-profile max_output_tokens (deleted 2026-09-10), max_context_tokens
	// and max_facts_in_shard_kernel (deleted 2026-09-11) each validated,
	// persisted, were echoed by the config wizard, and were read by nothing;
	// the last two were caps with no substrate to cap (there is no
	// per-persona kernel; kernel shards partition predicates by domain).

	// Execution Limits
	MaxExecutionTimeSec int `yaml:"max_execution_time_sec" json:"max_execution_time_sec"` // Timeout per task
	MaxRetries          int `yaml:"max_retries" json:"max_retries"`                       // Verification attempts per delegated task (VerifyWithRetry)

	// EnableLearning gates whether this shard's runs are recorded for prompt
	// evolution (chat delegation and the session executor both honour it).
	EnableLearning bool `yaml:"enable_learning" json:"enable_learning"`
}

// applyShardDefaults fills in zero values with defaults.
func applyShardDefaults(p ShardProfile) ShardProfile {
	// Temperature and TopP are deliberately not defaulted: zero leaves the
	// client's own default in force, so only a profile that chose a value
	// changes sampling.
	if p.MaxExecutionTimeSec == 0 {
		p.MaxExecutionTimeSec = 300
	}
	if p.MaxRetries == 0 {
		p.MaxRetries = 3
	}
	return p
}

// DefaultShardProfile returns a ShardProfile with sensible defaults.
func DefaultShardProfile() *ShardProfile {
	return &ShardProfile{
		Temperature:         0.7,
		TopP:                0.9,
		MaxExecutionTimeSec: 300,
		MaxRetries:          3,
		EnableLearning:      true,
	}
}

// DefaultShardProfiles returns a map of default ShardProfiles.
func DefaultShardProfiles() map[string]ShardProfile {
	return map[string]ShardProfile{
		"coder": {
			Temperature:         0.7,
			TopP:                0.9,
			MaxExecutionTimeSec: 600,
			MaxRetries:          3,
			EnableLearning:      true,
		},
		"tester": {
			Temperature:         0.5,
			TopP:                0.9,
			MaxExecutionTimeSec: 300,
			MaxRetries:          3,
			EnableLearning:      true,
		},
		"reviewer": {
			Temperature:         0.3,
			TopP:                0.9,
			MaxExecutionTimeSec: 900,
			MaxRetries:          2,
			EnableLearning:      false,
		},
		"researcher": {
			Temperature:         0.6,
			TopP:                0.95,
			MaxExecutionTimeSec: 600,
			MaxRetries:          3,
			EnableLearning:      true,
		},
	}
}
