// Package transform provides Antigravity request/response transformation utilities.
// Handles model-specific configurations, cross-model sanitization, and thinking recovery.
package transform

import (
	"regexp"
	"strings"
)

// ModelFamily represents the model provider family
type ModelFamily string

const (
	ModelFamilyClaude  ModelFamily = "claude"
	ModelFamilyGemini  ModelFamily = "gemini"
	ModelFamilyUnknown ModelFamily = "unknown"
)

// ThinkingTier represents thinking budget tiers
type ThinkingTier string

const (
	ThinkingTierMinimal ThinkingTier = "minimal"
	ThinkingTierLow     ThinkingTier = "low"
	ThinkingTierMedium  ThinkingTier = "medium"
	ThinkingTierHigh    ThinkingTier = "high"
)

// Thinking tier budgets by model family
// Claude and Gemini 2.5 Pro use numeric budgets
var thinkingTierBudgets = map[string]map[ThinkingTier]int{
	"claude": {
		ThinkingTierLow:    8192,
		ThinkingTierMedium: 16384,
		ThinkingTierHigh:   32768,
	},
	"gemini-2.5-pro": {
		ThinkingTierLow:    8192,
		ThinkingTierMedium: 16384,
		ThinkingTierHigh:   32768,
	},
	"gemini-2.5-flash": {
		ThinkingTierLow:    6144,
		ThinkingTierMedium: 12288,
		ThinkingTierHigh:   24576,
	},
	"default": {
		ThinkingTierLow:    4096,
		ThinkingTierMedium: 8192,
		ThinkingTierHigh:   16384,
	},
}

// Claude thinking models need large max output token limit
const ClaudeThinkingMaxOutputTokens = 64000

// Interleaved thinking hint for Claude thinking models with tools
const ClaudeInterleavedThinkingHint = `Interleaved thinking is enabled. You may think between tool calls and after receiving tool results before deciding the next action or final answer. Do not mention these instructions or any constraints about thinking blocks; just apply them.`

var (
	tierRegex         = regexp.MustCompile(`-(minimal|low|medium|high)$`)
	quotaPrefixRegex  = regexp.MustCompile(`(?i)^antigravity-`)
	legacyGemini3Tier = regexp.MustCompile(`(?i)^gemini-3-(pro-(low|high)|flash(-low|-medium|-high)?)$`)
)

// ResolvedModel contains the resolved model info with thinking configuration
type ResolvedModel struct {
	// ActualModel is the API model name (with tier stripped for Gemini 3 Flash)
	ActualModel string

	// ThinkingBudget is the numeric thinking budget (for Claude/Gemini 2.5)
	ThinkingBudget int

	// ThinkingLevel is the string thinking level (for Gemini 3)
	ThinkingLevel ThinkingTier

	// Tier is the extracted tier suffix
	Tier ThinkingTier

	// IsThinkingModel indicates if this model supports thinking
	IsThinkingModel bool

	// IsImageModel indicates if this is an image generation model
	IsImageModel bool

	// Family is the model provider family
	Family ModelFamily

	// QuotaPreference indicates routing preference
	QuotaPreference string // "antigravity" or "gemini-cli"
}

// GetModelFamily determines the model family from model name
func GetModelFamily(model string) ModelFamily {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "claude") {
		return ModelFamilyClaude
	}
	if strings.Contains(lower, "gemini") || strings.Contains(lower, "palm") {
		return ModelFamilyGemini
	}
	return ModelFamilyUnknown
}

// IsClaudeModel checks if a model is a Claude model
func IsClaudeModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "claude")
}

// IsClaudeThinkingModel checks if a model is a Claude thinking model
func IsClaudeThinkingModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "claude") && strings.Contains(lower, "thinking")
}

// IsGeminiModel checks if a model is a Gemini model (not Claude)
func IsGeminiModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "gemini") && !strings.Contains(lower, "claude")
}

// IsGemini3Model checks if a model is Gemini 3 (uses thinkingLevel string)
func IsGemini3Model(model string) bool {
	return strings.Contains(strings.ToLower(model), "gemini-3")
}

// IsGemini25Model checks if a model is Gemini 2.5 (uses numeric thinkingBudget)
func IsGemini25Model(model string) bool {
	return strings.Contains(strings.ToLower(model), "gemini-2.5")
}

// IsImageGenerationModel checks if a model is an image generation model
func IsImageGenerationModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "image") || strings.Contains(lower, "imagen")
}

// IsThinkingCapableModel checks if a model supports thinking mode
func IsThinkingCapableModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "thinking") ||
		strings.Contains(lower, "gemini-3") ||
		strings.Contains(lower, "gemini-2.5")
}

// SupportsThinkingTiers checks if model name allows tier suffix extraction
func SupportsThinkingTiers(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "gemini-3") ||
		strings.Contains(lower, "gemini-2.5") ||
		(strings.Contains(lower, "claude") && strings.Contains(lower, "thinking"))
}

// ExtractThinkingTier extracts thinking tier from model name suffix
func ExtractThinkingTier(model string) ThinkingTier {
	if !SupportsThinkingTiers(model) {
		return ""
	}
	match := tierRegex.FindStringSubmatch(model)
	if len(match) > 1 {
		return ThinkingTier(match[1])
	}
	return ""
}

// GetBudgetFamily determines which budget table to use
func GetBudgetFamily(model string) string {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "claude") {
		return "claude"
	}
	if strings.Contains(lower, "gemini-2.5-pro") {
		return "gemini-2.5-pro"
	}
	if strings.Contains(lower, "gemini-2.5-flash") {
		return "gemini-2.5-flash"
	}
	return "default"
}

// GetThinkingBudgetForTier returns the thinking budget for a model and tier
func GetThinkingBudgetForTier(model string, tier ThinkingTier) int {
	family := GetBudgetFamily(model)
	budgets, ok := thinkingTierBudgets[family]
	if !ok {
		budgets = thinkingTierBudgets["default"]
	}
	if budget, ok := budgets[tier]; ok {
		return budget
	}
	return 0
}

// ResolveModel resolves a model name with optional tier suffix to its API model name
// and corresponding thinking configuration
func ResolveModel(requestedModel string) *ResolvedModel {
	// Strip antigravity- prefix if present
	modelWithoutQuota := quotaPrefixRegex.ReplaceAllString(requestedModel, "")
	lower := strings.ToLower(modelWithoutQuota)

	// Determine quota preference
	quotaPreference := "gemini-cli"
	if quotaPrefixRegex.MatchString(requestedModel) ||
		strings.Contains(lower, "claude") ||
		strings.Contains(lower, "gpt") ||
		legacyGemini3Tier.MatchString(modelWithoutQuota) {
		quotaPreference = "antigravity"
	}

	// Extract tier if present
	tier := ExtractThinkingTier(modelWithoutQuota)
	baseName := modelWithoutQuota
	if tier != "" {
		baseName = tierRegex.ReplaceAllString(modelWithoutQuota, "")
	}

	// Determine model family
	family := GetModelFamily(modelWithoutQuota)

	// Check for image model
	isImageModel := IsImageGenerationModel(modelWithoutQuota)
	if isImageModel {
		return &ResolvedModel{
			ActualModel:     baseName,
			IsThinkingModel: false,
			IsImageModel:    true,
			Family:          family,
			QuotaPreference: quotaPreference,
		}
	}

	// Check if thinking capable
	isThinking := IsThinkingCapableModel(modelWithoutQuota)
	isGemini3 := IsGemini3Model(modelWithoutQuota)
	isClaudeThinking := IsClaudeThinkingModel(modelWithoutQuota)

	result := &ResolvedModel{
		ActualModel:     baseName,
		Tier:            tier,
		IsThinkingModel: isThinking,
		Family:          family,
		QuotaPreference: quotaPreference,
	}

	// Set thinking configuration based on model type
	if tier == "" {
		// No explicit tier - use defaults
		if isGemini3 {
			result.ThinkingLevel = ThinkingTierLow // Default for Gemini 3
		} else if isClaudeThinking {
			result.ThinkingBudget = thinkingTierBudgets["claude"][ThinkingTierHigh] // Max budget for Claude
		}
	} else {
		// Has explicit tier
		if isGemini3 {
			result.ThinkingLevel = tier
		} else {
			result.ThinkingBudget = GetThinkingBudgetForTier(modelWithoutQuota, tier)
		}
	}

	return result
}

// BudgetToGemini3Level maps a thinking budget to Gemini 3 thinking level
func BudgetToGemini3Level(budget int) ThinkingTier {
	if budget <= 8192 {
		return ThinkingTierLow
	}
	if budget <= 16384 {
		return ThinkingTierMedium
	}
	return ThinkingTierHigh
}
