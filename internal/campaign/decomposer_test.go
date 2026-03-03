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

// TODO: TEST_GAP: Null/Undefined/Empty Input Vectors
// 1. TestDecompose_EmptyGoal: Call Decompose with req.Goal = "" and verify it returns a specific error or falls back gracefully without wasting LLM calls.
// 2. TestDecompose_NilKernel: Verify instantiating NewDecomposer with a nil kernel (or passing one via DI) does not panic when `d.kernel.LoadFacts(facts)` is called.
// 3. TestClassifyDocuments_EmptyFiles: Pass empty slices or files with 0 bytes to classifyDocuments and ensure it doesn't cause panics or index out-of-bounds errors.

// TODO: TEST_GAP: Type Coercion & Malformed Data
// 1. TestLLMProposePlan_InvalidEnums: Mock the LLM to return invalid strings for enums like Category (e.g. "/magic") or Type, and ensure they are sanitized before Mangle assertion.
// 2. TestCleanJSONResponse_EdgeCases: Test cleanJSONResponse with deeply malformed JSON, markdown preamble/postamble, and unescaped quotes to ensure it robustly extracts valid JSON.
// 3. TestBuildCampaign_InvalidDependencies: Mock LLM output returning strings instead of integers for dependencies, ensuring unmarshaling fails cleanly instead of panicking.

// TODO: TEST_GAP: User Request Extremes & System Stress
// 1. TestIngestSourceDocuments_ExtremeFileCount: Test ingesting a directory with thousands of files to ensure bounds/rate limits apply so the LLM isn't called infinitely.
// 2. TestExtractRequirements_MassiveString: Test extractRequirementsSmart with extremely large files to ensure chunking logic does not run out of memory.
// 3. TestDecompose_UnexecutableLanguage: Send a goal requiring a non-existent programming language and ensure the system identifies the impossibility.

// TODO: TEST_GAP: State Conflicts & Race Conditions
// 1. TestDecompose_KernelCommitFailure: Mock the kernel transaction to fail on Commit() and verify Decompose returns an error rather than silently proceeding with desynchronized state.
// 2. TestExtractRequirementsSmart_MissingVectorDB: Ensure that if knowledge.db fails to create/open, the system gracefully falls back or returns an error instead of panicking.
// 3. TestConcurrentDecompose_IDCollision: Call Decompose concurrently to simulate ID generation collisions to ensure database locks or UUID generation doesn't corrupt state.
