package transform

// Tests for Antigravity-only transform package
// Scope: Gemini 3 (gemini-3-flash, gemini-3-pro) and Claude models only

import (
	"encoding/json"
	"testing"
)

func TestGetModelFamily(t *testing.T) {
	tests := []struct {
		model    string
		expected ModelFamily
	}{
		{"claude-sonnet-4-5-thinking", ModelFamilyClaude},
		{"claude-opus-4-5", ModelFamilyClaude},
		{"gemini-3-flash", ModelFamilyGemini},
		{"gemini-3-pro-high", ModelFamilyGemini},
		{"gemini-2.5-pro", ModelFamilyUnknown}, // Not Antigravity (Gemini CLI)
		{"gpt-4", ModelFamilyUnknown},
		{"", ModelFamilyUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := GetModelFamily(tc.model)
			if result != tc.expected {
				t.Errorf("GetModelFamily(%q) = %v, want %v", tc.model, result, tc.expected)
			}
		})
	}
}

func TestIsClaudeModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"claude-sonnet-4-5-thinking", true},
		{"Claude-Opus", true},
		{"gemini-3-flash", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := IsClaudeModel(tc.model)
			if result != tc.expected {
				t.Errorf("IsClaudeModel(%q) = %v, want %v", tc.model, result, tc.expected)
			}
		})
	}
}

func TestIsClaudeThinkingModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"claude-sonnet-4-5-thinking", true},
		{"claude-opus-4-5-thinking-high", true},
		{"claude-sonnet-4-5", false}, // Not thinking model
		{"gemini-3-flash", false},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := IsClaudeThinkingModel(tc.model)
			if result != tc.expected {
				t.Errorf("IsClaudeThinkingModel(%q) = %v, want %v", tc.model, result, tc.expected)
			}
		})
	}
}

func TestIsGemini3Model(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"gemini-3-flash", true},
		{"gemini-3-pro-high", true},
		{"gemini-2.5-pro", false}, // Gemini CLI, not Antigravity
		{"claude-sonnet-4-5", false},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := IsGemini3Model(tc.model)
			if result != tc.expected {
				t.Errorf("IsGemini3Model(%q) = %v, want %v", tc.model, result, tc.expected)
			}
		})
	}
}

func TestIsAntigravityModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"gemini-3-flash", true},
		{"gemini-3-pro-high", true},
		{"claude-sonnet-4-5-thinking", true},
		{"claude-opus-4-5", true},
		{"gemini-2.5-pro", false},   // Gemini CLI
		{"gemini-2.5-flash", false}, // Gemini CLI
		{"gpt-4", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := IsAntigravityModel(tc.model)
			if result != tc.expected {
				t.Errorf("IsAntigravityModel(%q) = %v, want %v", tc.model, result, tc.expected)
			}
		})
	}
}

func TestExtractThinkingTier(t *testing.T) {
	tests := []struct {
		model    string
		expected ThinkingTier
	}{
		{"gemini-3-pro-high", ThinkingTierHigh},
		{"gemini-3-flash-low", ThinkingTierLow},
		{"gemini-3-flash-medium", ThinkingTierMedium},
		{"claude-sonnet-4-5-thinking-high", ThinkingTierHigh},
		{"gemini-3-flash", ""}, // No tier
		{"gpt-4-medium", ""},   // Not a thinking-tier model
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := ExtractThinkingTier(tc.model)
			if result != tc.expected {
				t.Errorf("ExtractThinkingTier(%q) = %v, want %v", tc.model, result, tc.expected)
			}
		})
	}
}

func TestBuildThinkingConfigForModel_Gemini3(t *testing.T) {
	config := BuildThinkingConfigForModel("gemini-3-flash", true, ThinkingTierHigh, 0)

	configMap, ok := config.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", config)
	}

	// Should have camelCase keys
	if _, ok := configMap["includeThoughts"]; !ok {
		t.Error("Expected includeThoughts key for Gemini 3")
	}

	// Should have thinkingLevel for Gemini 3
	if level, ok := configMap["thinkingLevel"]; !ok || level != "high" {
		t.Errorf("Expected thinkingLevel=high, got %v", level)
	}

	// Should NOT have thinkingBudget
	if _, ok := configMap["thinkingBudget"]; ok {
		t.Error("Should not have thinkingBudget for Gemini 3")
	}
}

func TestBuildThinkingConfigForModel_Claude(t *testing.T) {
	config := BuildThinkingConfigForModel("claude-sonnet-4-5-thinking", true, ThinkingTierHigh, 0)

	configMap, ok := config.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", config)
	}

	// Should have snake_case keys
	if _, ok := configMap["include_thoughts"]; !ok {
		t.Error("Expected include_thoughts key for Claude")
	}

	// Should have thinking_budget for Claude
	if budget, ok := configMap["thinking_budget"]; !ok {
		t.Error("Expected thinking_budget for Claude")
	} else {
		budgetInt, ok := budget.(int)
		if !ok || budgetInt <= 0 {
			t.Errorf("Expected positive thinking_budget, got %v", budget)
		}
	}

	// Should NOT have thinkingLevel (that's Gemini 3)
	if _, ok := configMap["thinkingLevel"]; ok {
		t.Error("Should not have thinkingLevel for Claude")
	}
}

func TestBuildThinkingConfigForModel_JSONSerialization(t *testing.T) {
	// Test that configs serialize correctly with proper casing
	tests := []struct {
		model       string
		expectedKey string
	}{
		{"gemini-3-flash", "includeThoughts"},
		{"claude-sonnet-4-5-thinking", "include_thoughts"},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			config := BuildThinkingConfigForModel(tc.model, true, ThinkingTierHigh, 0)

			jsonBytes, err := json.Marshal(config)
			if err != nil {
				t.Fatalf("Failed to marshal config: %v", err)
			}

			jsonStr := string(jsonBytes)
			if !contains(jsonStr, tc.expectedKey) {
				t.Errorf("JSON should contain %q, got %s", tc.expectedKey, jsonStr)
			}
		})
	}
}

func TestResolveModel(t *testing.T) {
	tests := []struct {
		model        string
		wantTier     ThinkingTier
		wantThinking bool
		wantFamily   ModelFamily
		wantQuota    string
	}{
		{
			model:        "gemini-3-flash-high",
			wantTier:     ThinkingTierHigh,
			wantThinking: true,
			wantFamily:   ModelFamilyGemini,
			wantQuota:    "antigravity",
		},
		{
			model:        "claude-sonnet-4-5-thinking-low",
			wantTier:     ThinkingTierLow,
			wantThinking: true,
			wantFamily:   ModelFamilyClaude,
			wantQuota:    "antigravity",
		},
		{
			model:        "gemini-3-flash",
			wantTier:     "",
			wantThinking: true,
			wantFamily:   ModelFamilyGemini,
			wantQuota:    "antigravity",
		},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := ResolveModel(tc.model)

			if result.Tier != tc.wantTier {
				t.Errorf("Tier = %v, want %v", result.Tier, tc.wantTier)
			}
			if result.IsThinkingModel != tc.wantThinking {
				t.Errorf("IsThinkingModel = %v, want %v", result.IsThinkingModel, tc.wantThinking)
			}
			if result.Family != tc.wantFamily {
				t.Errorf("Family = %v, want %v", result.Family, tc.wantFamily)
			}
			if result.QuotaPreference != tc.wantQuota {
				t.Errorf("QuotaPreference = %v, want %v", result.QuotaPreference, tc.wantQuota)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
