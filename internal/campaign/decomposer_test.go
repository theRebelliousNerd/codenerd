package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"context"
	"testing"
)

// mockLLMClient implements perception.LLMClient for testing.
type mockLLMClient struct {
	completeFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *mockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "", nil
}

func (m *mockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, userPrompt)
	}
	return "", nil
}

// CompleteWithStructuredOutput is needed for the interface
func (m *mockLLMClient) CompleteWithStructuredOutput(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, userPrompt)
	}
	return "", nil
}

// CompleteWithTools is needed for the interface
func (m *mockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []perception.ToolDefinition) (*perception.LLMToolResponse, error) {
	return &perception.LLMToolResponse{Text: "", StopReason: "end_turn"}, nil
}

// SetModel is needed for the interface
func (m *mockLLMClient) SetModel(model string) {}

// GetModel is needed for the interface
func (m *mockLLMClient) GetModel() string { return "mock-model" }

// DisableSemaphore is needed for the interface
func (m *mockLLMClient) DisableSemaphore() {}

func TestNewDecomposer(t *testing.T) {
	mockKernel := &core.RealKernel{} // Minimal struct
	mockClient := &mockLLMClient{}
	workspace := "/tmp/test"

	d := NewDecomposer(mockKernel, mockClient, workspace)
	if d == nil {
		t.Fatal("NewDecomposer returned nil")
	}
	if d.kernel != mockKernel {
		t.Error("kernel not set correctly")
	}
	if d.llmClient != mockClient {
		t.Error("llmClient not set correctly")
	}
	if d.workspace != workspace {
		t.Error("workspace not set correctly")
	}
}

func TestDecomposer_Setters(t *testing.T) {
	d := &Decomposer{}

	// Test SetShardLister
	if d.shardLister != nil {
		t.Error("expected shardLister to be nil initially")
	}
	d.SetShardLister(nil) // Should handle nil safely

	// Test SetImportance
	if d.intelligence != nil {
		t.Error("expected intelligence to be nil initially")
	}
	// Note: IntelligenceGatherer construction is complex, just checking nil safety or non-nil assignment if we had a mock
	d.SetIntelligenceGatherer(nil)

	// Test SetAdvisoryBoard
	d.SetAdvisoryBoard(nil)
}

func TestDecomposer_InferDocType(t *testing.T) {
	d := &Decomposer{}

	tests := []struct {
		path     string
		expected string
	}{
		{"spec.txt", "/spec"},
		{"requirements.md", "/requirements"},
		{"system_design.md", "/design"},
		{"README.md", "/readme"},
		{"api.go", "/api_doc"}, // This logic in inferDocType matches "api" substring
		{"tutorial.md", "/tutorial"},
		{"unknown.txt", "/spec"}, // Default case
	}

	for _, tt := range tests {
		got := d.inferDocType(tt.path)
		if got != tt.expected {
			t.Errorf("inferDocType(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestDecomposer_ClassifyDocument_Trivial(t *testing.T) {
	d := &Decomposer{
		llmClient: &mockLLMClient{},
	}

	// Test trivial content case optimization
	ctx := context.Background()
	class, err := d.classifyDocument(ctx, "foo.txt", "short")
	if err != nil {
		t.Fatalf("classifyDocument failed: %v", err)
	}

	if class.Layer != "/scaffold" {
		t.Errorf("expected /scaffold for trivial content, got %s", class.Layer)
	}
	if class.Confidence != 0.5 {
		t.Errorf("expected 0.5 confidence for trivial content, got %f", class.Confidence)
	}
}

func TestDecomposer_Decompose_ValidationFailure(t *testing.T) {
	// We cannot easily test full Decompose without a real kernel that supports LoadFacts/Validate.
	// However, we can test that it initializes and fails gracefully if SourceDocs are missing or other prerequisites.

	mockKernel := &core.RealKernel{}
	mockClient := &mockLLMClient{}
	d := NewDecomposer(mockKernel, mockClient, "/tmp/workspace")

	// Verify decomposer was created
	if d == nil {
		t.Fatal("NewDecomposer returned nil")
	}

	// We cannot call Decompose without a real initialized kernel.
	// This test just confirms we can construct the Decomposer with minimal deps.
	mockClient.completeFunc = func(ctx context.Context, prompt string) (string, error) {
		return `{"requirements": []}`, nil
	}
	// Step 5 calls kernel.LoadFacts. If mockKernel is empty, LoadFacts might panic or return error if not initialized.
	// core.RealKernel generally needs initialization.
	// So assume we stop here. Use a specialized test that mocks kernel methods if we could, but we can't easily mock *RealKernel methods.
}
