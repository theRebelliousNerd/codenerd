package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// mockLLMClient implements perception.LLMClient for testing.
type mockLLMClient struct {
	completeFunc           func(ctx context.Context, prompt string) (string, error)
	completeWithSystemFunc func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	completeWithSchemaFunc func(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error)
	schemaCapable          bool
}

func (m *mockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "", nil
}

func (m *mockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.completeWithSystemFunc != nil {
		return m.completeWithSystemFunc(ctx, systemPrompt, userPrompt)
	}
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

func (m *mockLLMClient) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	if m.completeWithSchemaFunc != nil {
		return m.completeWithSchemaFunc(ctx, systemPrompt, userPrompt, jsonSchema)
	}
	return "", errors.New("schema not configured")
}

func (m *mockLLMClient) SchemaCapable() bool {
	return m.schemaCapable
}

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

func TestLLMProposePlan_UsesSchemaCapableClient(t *testing.T) {
	var schemaCalls int
	client := &mockLLMClient{
		schemaCapable: true,
		completeWithSchemaFunc: func(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
			schemaCalls++
			if jsonSchema == "" || !strings.Contains(jsonSchema, `"title"`) {
				t.Fatalf("expected non-empty plan schema")
			}
			return sampleRawPlanJSON("Schema Plan"), nil
		},
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			t.Fatalf("unexpected fallback CompleteWithSystem call")
			return "", nil
		},
	}

	d := &Decomposer{
		llmClient:      client,
		workspace:      t.TempDir(),
		promptProvider: NewStaticPromptProvider(),
	}

	plan, err := d.llmProposePlan(context.Background(), "/campaign_test", DecomposeRequest{
		Goal:         "Build campaign planning reliability",
		CampaignType: CampaignTypeCustom,
	}, "", nil, nil)
	if err != nil {
		t.Fatalf("llmProposePlan failed: %v", err)
	}
	if schemaCalls != 1 {
		t.Fatalf("expected 1 schema call, got %d", schemaCalls)
	}
	if plan.Title != "Schema Plan" {
		t.Fatalf("expected schema plan title, got %q", plan.Title)
	}
}

func TestLLMProposePlan_SchemaFailureFallsBack(t *testing.T) {
	var schemaCalls int
	var systemCalls int
	client := &mockLLMClient{
		schemaCapable: true,
		completeWithSchemaFunc: func(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
			schemaCalls++
			return "", errors.New("schema rejected")
		},
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			systemCalls++
			return sampleRawPlanJSON("Fallback Plan"), nil
		},
	}

	d := &Decomposer{
		llmClient:      client,
		workspace:      t.TempDir(),
		promptProvider: NewStaticPromptProvider(),
	}

	plan, err := d.llmProposePlan(context.Background(), "/campaign_test", DecomposeRequest{
		Goal:         "Fallback from schema failure",
		CampaignType: CampaignTypeCustom,
	}, "", nil, nil)
	if err != nil {
		t.Fatalf("llmProposePlan failed: %v", err)
	}
	if schemaCalls != 1 {
		t.Fatalf("expected 1 schema attempt, got %d", schemaCalls)
	}
	if systemCalls != 1 {
		t.Fatalf("expected 1 fallback system call, got %d", systemCalls)
	}
	if plan.Title != "Fallback Plan" {
		t.Fatalf("expected fallback plan title, got %q", plan.Title)
	}
}

func TestLLMProposePlan_MalformedThenRetrySucceeds(t *testing.T) {
	callCount := 0
	client := &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			callCount++
			if callCount == 1 {
				return `{"title":"broken","confidence":"not-a-number","phases":[}`, nil
			}
			return sampleRawPlanJSON("Recovered Plan"), nil
		},
	}

	d := &Decomposer{
		llmClient:      client,
		workspace:      t.TempDir(),
		promptProvider: NewStaticPromptProvider(),
	}

	plan, err := d.llmProposePlan(context.Background(), "/campaign_test", DecomposeRequest{
		Goal:         "Recover from malformed output",
		CampaignType: CampaignTypeCustom,
	}, "", nil, nil)
	if err != nil {
		t.Fatalf("llmProposePlan failed: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls (initial + retry), got %d", callCount)
	}
	if plan.Title != "Recovered Plan" {
		t.Fatalf("expected recovered plan title, got %q", plan.Title)
	}
}

func TestLLMProposePlan_MalformedAfterRetryFails(t *testing.T) {
	client := &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return `{"title":"bad","phases":[`, nil
		},
	}

	d := &Decomposer{
		llmClient:      client,
		workspace:      t.TempDir(),
		promptProvider: NewStaticPromptProvider(),
	}

	_, err := d.llmProposePlan(context.Background(), "/campaign_test", DecomposeRequest{
		Goal:         "Should fail after malformed retry",
		CampaignType: CampaignTypeCustom,
	}, "", nil, nil)
	if err == nil {
		t.Fatal("expected parse failure error")
	}
	if !strings.Contains(err.Error(), "failed to parse plan JSON after retry") {
		t.Fatalf("expected retry parse failure message, got %v", err)
	}
}

func TestLLMProposePlan_EmptyPhasesFallsBackToScaffold(t *testing.T) {
	client := &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return `{"title":"Empty Plan","confidence":0.9,"phases":[]}`, nil
		},
	}

	d := &Decomposer{
		llmClient:      client,
		workspace:      t.TempDir(),
		promptProvider: NewStaticPromptProvider(),
	}

	plan, err := d.llmProposePlan(context.Background(), "/campaign_test", DecomposeRequest{
		Goal:         "Generate fallback phases",
		CampaignType: CampaignTypeCustom,
	}, "", nil, nil)
	if err != nil {
		t.Fatalf("llmProposePlan failed: %v", err)
	}
	if len(plan.Phases) != 3 {
		t.Fatalf("expected fallback 3 phases, got %d", len(plan.Phases))
	}
	if plan.Confidence != 0.5 {
		t.Fatalf("expected fallback confidence 0.5, got %.2f", plan.Confidence)
	}
}

func sampleRawPlanJSON(title string) string {
	return fmt.Sprintf(`{
  "title": %q,
  "confidence": 0.92,
  "phases": [
    {
      "name": "Phase 1",
      "order": 0,
      "category": "/scaffold",
      "description": "Create baseline scaffolding",
      "objective_type": "/create",
      "verification_method": "/none",
      "complexity": "/low",
      "depends_on": [],
      "focus_patterns": ["internal/campaign/*"],
      "required_tools": ["fs_read", "fs_write"],
      "tasks": [
        {
          "description": "Create skeleton files",
          "type": "/file_create",
          "priority": "/normal",
          "order": 0,
          "depends_on": [],
          "artifacts": ["internal/campaign/new_file.go"],
          "write_set": ["internal/campaign/new_file.go"]
        }
      ]
    }
  ]
}`, title)
}

// TODO: TEST_GAP: TestDecompose_LLMTotalFailure
// Mock LLMClient.Complete to return an error or timeout.
// Verify Decompose returns a wrapped error and cleans up.

// TODO: TEST_GAP: TestDecompose_EmptyGoal
// Verify Decompose handles an empty goal gracefully (error or fallback plan).

// TODO: TEST_GAP: TestCleanJSONResponse_EdgeCases
// Test cleanJSONResponse with:
// - Markdown containing text before and after the ```json block.
// - Raw JSON without markdown fences but with trailing garbage.
// - Nested JSON objects and arrays.

// TODO: TEST_GAP: TestValidatePlan_CircularDependency
// Mock Kernel.Query("validation_error") to return a circular dependency issue.
// Verify validatePlan correctly parses the issue into the PlanValidationIssue slice.

// TODO: TEST_GAP: TestRefinePlan_Success
// Provide a RawPlan and a list of issues. Mock LLM to return a corrected plan.
// Verify the refined plan is returned.

// TODO: TEST_GAP: TestIngestSourceDocuments_Cancellation
// Create a directory with dummy files. Pass a context that cancels after a few files are processed.
// Verify the function returns early with context.Canceled.

// TODO: TEST_GAP: TestRefinePlan_TxCommitFail
// Mock tx.Commit() to return error during atomic rebuild failure.
// Reverts state, returns error, logs warning.
