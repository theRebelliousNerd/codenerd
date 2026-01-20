// Package transform provides request/response transformation utilities for the
// Antigravity client ONLY.
//
// SCOPE: This package handles Antigravity-specific model transformations:
//   - Gemini 3 models (gemini-3-flash, gemini-3-pro) via Antigravity gateway
//   - Claude models (claude-sonnet-4-5, claude-opus-4-5, etc.) via Antigravity gateway
//
// NOT IN SCOPE:
//   - Gemini 2.5 models (use Gemini CLI endpoint, not Antigravity)
//   - Direct Gemini API calls (use client_gemini.go)
//   - OpenAI/other providers
//
// Key features:
//   - Model-aware thinking configuration (Gemini camelCase vs Claude snake_case)
//   - Cross-model signature sanitization (prevents signature errors on model switch)
//   - Thinking recovery (closes corrupted tool loops for fresh thinking)
package transform

import (
	"regexp"
	"strings"
)

// ModelFamily represents the model provider family for Antigravity routing
type ModelFamily string

const (
	ModelFamilyClaude  ModelFamily = "claude"
	ModelFamilyGemini  ModelFamily = "gemini" // Gemini 3 only via Antigravity
	ModelFamilyUnknown ModelFamily = "unknown"
)

// ThinkingTier represents thinking budget tiers for Antigravity models
type ThinkingTier string

const (
	ThinkingTierMinimal ThinkingTier = "minimal"
	ThinkingTierLow     ThinkingTier = "low"
	ThinkingTierMedium  ThinkingTier = "medium"
	ThinkingTierHigh    ThinkingTier = "high"
)

// Antigravity thinking tier budgets
// Gemini 3 uses string thinkingLevel, Claude uses numeric budgets
var thinkingTierBudgets = map[string]map[ThinkingTier]int{
	// Claude models via Antigravity use numeric budgets
	"claude": {
		ThinkingTierLow:    8192,
		ThinkingTierMedium: 16384,
		ThinkingTierHigh:   32768,
	},
	// Default fallback
	"default": {
		ThinkingTierLow:    4096,
		ThinkingTierMedium: 8192,
		ThinkingTierHigh:   16384,
	},
}

// Claude thinking models need large max output token limit
const ClaudeThinkingMaxOutputTokens = 64000

// Interleaved thinking hint for Claude thinking models with tools
// (codeNERD uses Piggyback protocol, so tools are text-based)
const ClaudeInterleavedThinkingHint = `Interleaved thinking is enabled. You may think between tool calls and after receiving tool results before deciding the next action or final answer. Do not mention these instructions or any constraints about thinking blocks; just apply them.`

var (
	tierRegex         = regexp.MustCompile(`-(minimal|low|medium|high)$`)
	quotaPrefixRegex  = regexp.MustCompile(`(?i)^antigravity-`)
	legacyGemini3Tier = regexp.MustCompile(`(?i)^gemini-3-(pro-(low|high)|flash(-low|-medium|-high)?)$`)
)

// ResolvedModel contains the resolved Antigravity model info with thinking configuration
type ResolvedModel struct {
	// ActualModel is the API model name (with tier stripped for Gemini 3 Flash)
	ActualModel string

	// ThinkingBudget is the numeric thinking budget (for Claude)
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

	// QuotaPreference indicates Antigravity vs Gemini CLI routing
	QuotaPreference string // "antigravity" or "gemini-cli"
}

// GetModelFamily determines the Antigravity model family from model name.
// Returns ModelFamilyGemini for Gemini 3 models, ModelFamilyClaude for Claude.
func GetModelFamily(model string) ModelFamily {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "claude") {
		return ModelFamilyClaude
	}
	// Only Gemini 3 is supported via Antigravity
	if strings.Contains(lower, "gemini-3") {
		return ModelFamilyGemini
	}
	return ModelFamilyUnknown
}

// IsClaudeModel checks if a model is a Claude model (Antigravity-supported)
func IsClaudeModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "claude")
}

// IsClaudeThinkingModel checks if a model is a Claude thinking model
func IsClaudeThinkingModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "claude") && strings.Contains(lower, "thinking")
}

// IsGeminiModel checks if a model is a Gemini model
// Note: For Antigravity, only Gemini 3 is supported
func IsGeminiModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "gemini") && !strings.Contains(lower, "claude")
}

// IsGemini3Model checks if a model is Gemini 3 (Antigravity-supported, uses thinkingLevel string)
func IsGemini3Model(model string) bool {
	return strings.Contains(strings.ToLower(model), "gemini-3")
}

// IsAntigravityModel checks if a model is supported by the Antigravity gateway.
// Antigravity supports: Gemini 3 (gemini-3-flash, gemini-3-pro) and Claude models.
func IsAntigravityModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "gemini-3") || strings.Contains(lower, "claude")
}

// IsImageGenerationModel checks if a model is an image generation model
func IsImageGenerationModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "image") || strings.Contains(lower, "imagen")
}

// IsThinkingCapableModel checks if an Antigravity model supports thinking mode.
// Gemini 3 always supports thinking. Claude requires "-thinking" suffix.
func IsThinkingCapableModel(model string) bool {
	lower := strings.ToLower(model)
	// Claude thinking models require explicit suffix
	if strings.Contains(lower, "claude") {
		return strings.Contains(lower, "thinking")
	}
	// Gemini 3 always supports thinking
	return strings.Contains(lower, "gemini-3")
}

// SupportsThinkingTiers checks if model name allows tier suffix extraction
func SupportsThinkingTiers(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "gemini-3") ||
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

// GetBudgetFamily determines which budget table to use for Claude models
func GetBudgetFamily(model string) string {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "claude") {
		return "claude"
	}
	return "default"
}

// GetThinkingBudgetForTier returns the thinking budget for a Claude model and tier
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

// ResolveModel resolves an Antigravity model name with optional tier suffix
// to its API model name and corresponding thinking configuration.
//
// Supported models:
//   - gemini-3-flash, gemini-3-flash-low, gemini-3-flash-medium, gemini-3-flash-high
//   - gemini-3-pro, gemini-3-pro-low, gemini-3-pro-high
//   - claude-sonnet-4-5-thinking, claude-sonnet-4-5-thinking-low, etc.
//   - claude-opus-4-5-thinking, claude-opus-4-5-thinking-high, etc.
func ResolveModel(requestedModel string) *ResolvedModel {
	// Strip antigravity- prefix if present
	modelWithoutQuota := quotaPrefixRegex.ReplaceAllString(requestedModel, "")
	lower := strings.ToLower(modelWithoutQuota)

	// Determine quota preference
	// Antigravity: Claude, GPT, and Gemini 3 tiered models
	// Gemini CLI: Everything else (Gemini 2.5, etc.)
	quotaPreference := "gemini-cli"
	if quotaPrefixRegex.MatchString(requestedModel) ||
		strings.Contains(lower, "claude") ||
		strings.Contains(lower, "gpt") ||
		legacyGemini3Tier.MatchString(modelWithoutQuota) ||
		strings.Contains(lower, "gemini-3") {
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
