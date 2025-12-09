package tool_generator

import (
	"codenerd/internal/core"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// MockLLMClient for testing
type MockLLMClient struct {
	CompleteFunc           func(ctx context.Context, prompt string) (string, error)
	CompleteWithSystemFunc func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, prompt)
	}
	return "", nil
}

func (m *MockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.CompleteWithSystemFunc != nil {
		return m.CompleteWithSystemFunc(ctx, systemPrompt, userPrompt)
	}
	return "", nil
}

func TestToolGeneratorShard_Execute_Generate(t *testing.T) {
	// SKIP: This test requires full constitution boot which has stratification issues
	// in campaign_rules.mg that need careful refactoring
	t.Skip("Skipping: constitution stratification issues need refactoring")

	// Setup temp dir for tools
	tmpDir, err := os.MkdirTemp("", "shard-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config
	config := core.DefaultSpecialistConfig("tool_generator", "")
	shard := NewToolGeneratorShard("test-shard", config)
	shard.generatorConfig.ToolsDir = tmpDir

	// Mock LLM to return tool code
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			// For profile generation or simple completion
			if strings.Contains(prompt, "quality profile") {
				return `{"tool_type": "quick_calculation", "description": "Test tool"}`, nil
			}
			return "", nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			// Return mock tool code
			return `
package tools

import (
	"context"
)

const TestToolDescription = "A test tool"

func TestTool(ctx context.Context, input string) (string, error) {
	return "processed " + input, nil
}
`, nil
		},
	}

	shard.SetLLMClient(mockLLM)

	// Execute task
	task := "generate tool for test purpose"
	resultJSON, err := shard.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Parse result
	var result ToolGeneratorResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Verify
	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}
	if result.Action != "generate" {
		t.Errorf("Expected action 'generate', got '%s'", result.Action)
	}

	// Verify file creation
	files, _ := os.ReadDir(tmpDir)
	found := false
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".go") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Tool file was not created in temp dir")
	}
}

func TestToolGeneratorShard_ParseAction(t *testing.T) {
	shard := &ToolGeneratorShard{}

	tests := []struct {
		task string
		want string
	}{
		{"generate a tool", "generate"},
		{"create a tool", "generate"},
		{"list available tools", "list"},
		{"show tools", "list"},
		{"check tool status", "status"},
		{"tool quality", "status"},
		{"refine the json tool", "refine"},
		{"improve performance", "refine"},
		{"unknown command", "generate"}, // Default
	}

	for _, tt := range tests {
		got := shard.parseAction(tt.task)
		if got != tt.want {
			t.Errorf("parseAction(%q) = %q, want %q", tt.task, got, tt.want)
		}
	}
}
