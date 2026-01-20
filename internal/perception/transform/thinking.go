package transform

// ThinkingConfig represents the thinking configuration for API requests.
// Different models use different field names and formats.
type ThinkingConfig struct {
	// Gemini 3 format (camelCase)
	IncludeThoughts bool   `json:"includeThoughts,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`  // Gemini 3: "low", "medium", "high"
	ThinkingBudget  int    `json:"thinkingBudget,omitempty"` // Gemini 2.5: numeric budget

	// Claude format (snake_case) - used when sending to Claude models
	IncludeThoughtsClaude bool `json:"include_thoughts,omitempty"`
	ThinkingBudgetClaude  int  `json:"thinking_budget,omitempty"`
}

// GeminiThinkingConfig is the Gemini-specific thinking config format
type GeminiThinkingConfig struct {
	IncludeThoughts bool   `json:"includeThoughts"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`  // Gemini 3 only
	ThinkingBudget  int    `json:"thinkingBudget,omitempty"` // Gemini 2.5 only
}

// ClaudeThinkingConfig is the Claude-specific thinking config format (snake_case)
type ClaudeThinkingConfig struct {
	IncludeThoughts bool `json:"include_thoughts"`
	ThinkingBudget  int  `json:"thinking_budget,omitempty"`
}

// BuildThinkingConfigForModel builds the appropriate thinking config for a model
func BuildThinkingConfigForModel(model string, includeThoughts bool, tier ThinkingTier, budget int) interface{} {
	if IsClaudeThinkingModel(model) {
		return buildClaudeThinkingConfig(includeThoughts, tier, budget)
	}
	if IsGemini3Model(model) {
		return buildGemini3ThinkingConfig(includeThoughts, tier)
	}
	if IsGemini25Model(model) {
		return buildGemini25ThinkingConfig(includeThoughts, budget)
	}
	// Default to Gemini format
	return buildGemini25ThinkingConfig(includeThoughts, budget)
}

// buildClaudeThinkingConfig builds Claude-specific thinking config with snake_case keys
func buildClaudeThinkingConfig(includeThoughts bool, tier ThinkingTier, budget int) map[string]interface{} {
	config := map[string]interface{}{
		"include_thoughts": includeThoughts,
	}

	// Determine budget from tier or use provided budget
	effectiveBudget := budget
	if tier != "" && budget == 0 {
		effectiveBudget = GetThinkingBudgetForTier("claude", tier)
	}
	if effectiveBudget == 0 {
		// Default to high for Claude
		effectiveBudget = thinkingTierBudgets["claude"][ThinkingTierHigh]
	}

	if effectiveBudget > 0 {
		config["thinking_budget"] = effectiveBudget
	}

	return config
}

// buildGemini3ThinkingConfig builds Gemini 3-specific thinking config with thinkingLevel
func buildGemini3ThinkingConfig(includeThoughts bool, tier ThinkingTier) map[string]interface{} {
	config := map[string]interface{}{
		"includeThoughts": includeThoughts,
	}

	// Gemini 3 uses thinkingLevel string
	level := tier
	if level == "" {
		level = ThinkingTierLow // Default to low
	}

	config["thinkingLevel"] = string(level)

	return config
}

// buildGemini25ThinkingConfig builds Gemini 2.5-specific thinking config with numeric budget
func buildGemini25ThinkingConfig(includeThoughts bool, budget int) map[string]interface{} {
	config := map[string]interface{}{
		"includeThoughts": includeThoughts,
	}

	if budget > 0 {
		config["thinkingBudget"] = budget
	}

	return config
}

// GenerationConfig represents the generation configuration with thinking support
type GenerationConfig struct {
	Temperature     float64     `json:"temperature,omitempty"`
	MaxOutputTokens int         `json:"maxOutputTokens,omitempty"`
	ThinkingConfig  interface{} `json:"thinkingConfig,omitempty"`
}

// BuildGenerationConfigForModel builds generation config with appropriate thinking config
func BuildGenerationConfigForModel(model string, temperature float64, maxTokens int, enableThinking bool, tier ThinkingTier, budget int) *GenerationConfig {
	config := &GenerationConfig{
		Temperature:     temperature,
		MaxOutputTokens: maxTokens,
	}

	if enableThinking && IsThinkingCapableModel(model) {
		config.ThinkingConfig = BuildThinkingConfigForModel(model, true, tier, budget)

		// Claude thinking models need larger max output tokens
		if IsClaudeThinkingModel(model) {
			effectiveBudget := budget
			if tier != "" && budget == 0 {
				effectiveBudget = GetThinkingBudgetForTier("claude", tier)
			}
			if config.MaxOutputTokens <= effectiveBudget || config.MaxOutputTokens == 0 {
				config.MaxOutputTokens = ClaudeThinkingMaxOutputTokens
			}
		}
	}

	return config
}

// GetThinkingLevelForBudget converts a numeric budget to Gemini 3 thinking level
func GetThinkingLevelForBudget(budget int) ThinkingTier {
	return BudgetToGemini3Level(budget)
}

// ApplyInterleavedThinkingHint appends the interleaved thinking hint to system instruction
// for Claude thinking models with tools (via Piggyback protocol)
func ApplyInterleavedThinkingHint(systemPrompt string) string {
	if systemPrompt == "" {
		return ClaudeInterleavedThinkingHint
	}
	return systemPrompt + "\n\n" + ClaudeInterleavedThinkingHint
}
