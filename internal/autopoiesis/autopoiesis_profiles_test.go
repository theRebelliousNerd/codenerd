package autopoiesis

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOrchestrator_ToolProfiles(t *testing.T) {
	// Setup
	pm := NewProfileStore(filepath.Join(os.TempDir(), "profiles"))
	orch := &Orchestrator{
		profiles: pm,
	}

	toolName := "test-tool"

	// 1. Test GetDefaultToolProfile
	defProfile := orch.GetDefaultToolProfile(toolName, ToolTypeDataFetch)
	if defProfile == nil {
		t.Fatal("GetDefaultToolProfile returned nil")
	}
	if defProfile.ToolType != ToolTypeDataFetch {
		t.Errorf("Expected tool type %s, got %s", ToolTypeDataFetch, defProfile.ToolType)
	}

	// 2. Test SetToolProfile and GetToolProfile
	customProfile := *defProfile
	customProfile.Description = "Custom Description"

	orch.SetToolProfile(&customProfile)

	retrieved := orch.GetToolProfile(toolName)
	if retrieved == nil {
		t.Fatal("GetToolProfile returned nil after setting")
	}
	if retrieved.Description != "Custom Description" {
		t.Errorf("Expected description 'Custom Description', got '%s'", retrieved.Description)
	}
}

func TestParseProfileResponse(t *testing.T) {
	// Valid JSON response from LLM
	jsonResp := `{
		"tool_type": "data_fetch",
		"description": "Fetches data",
		"performance": {
			"expected_duration_min_ms": 100,
			"expected_duration_max_ms": 5000,
			"acceptable_duration_ms": 2000,
			"timeout_duration_ms": 10000,
			"max_retries": 3
		},
		"output": {
			"expected_format": "json"
		},
		"usage_pattern": {
			"frequency": "often"
		},
		"caching": {
			"cacheable": true
		}
	}`

	// Wrap it in markdown code block as LLM might output
	llmOutput := "Here is the profile:\n```json\n" + jsonResp + "\n```"

	profile, err := parseProfileResponse("test-tool", llmOutput)
	if err != nil {
		t.Fatalf("parseProfileResponse failed: %v", err)
	}

	if profile.ToolType != ToolTypeDataFetch {
		t.Errorf("Expected type data_fetch, got %s", profile.ToolType)
	}
	if profile.Performance.MaxRetries != 3 {
		t.Errorf("Expected 3 retries, got %d", profile.Performance.MaxRetries)
	}
	if profile.Caching.Cacheable != true {
		t.Error("Expected cacheable true")
	}
}

func TestParseProfileResponse_InvalidJSON(t *testing.T) {
	llmOutput := "Some text { invalid json content } end text"
	_, err := parseProfileResponse("test-tool", llmOutput)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// MockProfileLLMClient for testing generation
type MockProfileLLMClient struct{}

func (m *MockProfileLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	// Return a valid profile JSON
	return `{
		"tool_type": "data_fetch",
		"description": "Generated Description",
		"performance": {
			"expected_duration_min_ms": 100,
			"expected_duration_max_ms": 5000,
			"acceptable_duration_ms": 1000,
			"timeout_duration_ms": 10000,
			"max_retries": 3
		},
		"output": {
			"expected_format": "json"
		},
		"usage_pattern": {
			"frequency": "occasional"
		},
		"caching": {
			"cacheable": true
		}
	}`, nil
}

func (m *MockProfileLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.Complete(ctx, userPrompt)
}

func TestGenerateToolProfile(t *testing.T) {
	// Setup
	pm := NewProfileStore(filepath.Join(os.TempDir(), "profiles_gen"))
	mockClient := &MockProfileLLMClient{}

	// Create Orchestrator manually with dependencies needed for GenerateToolProfile
	orch := &Orchestrator{
		profiles: pm,
		client:   mockClient,
	}

	// Test
	ctx := context.Background()
	toolName := "generated-tool"
	desc := "A tool that does something"
	code := "package tools\nfunc DoIt() {}"

	profile, err := orch.GenerateToolProfile(ctx, toolName, desc, code)
	if err != nil {
		t.Fatalf("GenerateToolProfile failed: %v", err)
	}

	if profile == nil {
		t.Fatal("Returned profile is nil")
	}
	if profile.ToolName != toolName {
		t.Errorf("Expected tool name %s, got %s", toolName, profile.ToolName)
	}
	if profile.ToolType != ToolTypeDataFetch { // matched mock JSON
		t.Errorf("Expected type data_fetch, got %s", profile.ToolType)
	}

	// Verify it was stored
	stored := orch.GetToolProfile(toolName)
	if stored == nil {
		t.Error("Profile was not stored in ProfileStore")
	}
}
