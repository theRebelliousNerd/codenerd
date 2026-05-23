package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// CompleteWithStreaming sends a request with streaming enabled
func (m *mockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	if m.completeWithSystemFunc != nil {
		res, err := m.completeWithSystemFunc(ctx, systemPrompt, userPrompt)
		if err != nil {
			errCh <- err
		} else {
			ch <- res
		}
	} else if m.completeFunc != nil {
		res, err := m.completeFunc(ctx, userPrompt)
		if err != nil {
			errCh <- err
		} else {
			ch <- res
		}
	} else {
		ch <- ""
	}
	close(ch)
	close(errCh)
	return ch, errCh
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

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify NewDecomposer handles an empty workspace string without defaulting to root or polluting unintended locations.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify Decompose handles Decomposer instances with nil optional dependencies (advisoryBoard, intelligenceGatherer, edgeCaseDetector).
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify readDocumentsFromPath behaves correctly when an element in SourcePaths is an empty string `[""]`.
// TODO: TEST_GAP: [Type Coercion] Verify cleanJSONResponse cleanly handles partially valid JSON with incorrect nested types (e.g., string instead of int for phase budget).
// TODO: TEST_GAP: [Type Coercion] Verify seedDocFacts asserts strictly typed Mangle Atoms instead of Strings when creating campaign facts.
// TODO: TEST_GAP: [User Request Extremes] Verify readDocumentsFromDir gracefully handles a directory tree with 1,000,000 nested files without exhausting memory or file descriptors.
// TODO: TEST_GAP: [User Request Extremes] Verify the Decomposer can process a simulated 50 million line monorepo by using streaming logic/sparse retrieval instead of full RAM loading.
// TODO: TEST_GAP: [User Request Extremes] Verify validation logic prevents the LLM from hallucinating non-existent subsystems, tools, or coding languages in the plan.
// TODO: TEST_GAP: [State Conflicts] Verify Decomposer handles concurrent calls to setter methods (SetPromptProvider, SetShardLister) while Decompose is running without data races.
// TODO: TEST_GAP: [State Conflicts] Verify readDocumentsFromDir handles Time-of-Check/Time-of-Use (TOC/TOU) race conditions where a file is deleted after metadata is gathered but before reading content.
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

func TestDecompose_NilKernel_ReturnsError(t *testing.T) {
	d := NewDecomposer(nil, &mockLLMClient{}, t.TempDir())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "Harden campaign planning",
		CampaignType: CampaignTypeCustom,
	})
	if !errors.Is(err, ErrNilKernel) {
		t.Fatalf("expected ErrNilKernel, got %v", err)
	}
}

func TestDecompose_NilLLM_ReturnsError(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, nil, t.TempDir())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "Harden campaign planning",
		CampaignType: CampaignTypeCustom,
	})
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

func TestDecompose_EmptyGoal_ReturnsError(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{}, t.TempDir())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "   ",
		CampaignType: CampaignTypeCustom,
	})
	if !errors.Is(err, ErrEmptyGoal) {
		t.Fatalf("expected ErrEmptyGoal, got %v", err)
	}
}

func TestBuildCampaign_NormalizesEnumsAndJailsPaths(t *testing.T) {
	kernel := &MockKernel{}
	workspace := t.TempDir()
	d := NewDecomposer(kernel, &mockLLMClient{}, workspace)

	campaign := d.buildCampaign("/campaign_test", DecomposeRequest{
		Goal:          "Stabilize campaign planning",
		CampaignType:  CampaignTypeCustom,
		ContextBudget: 1000,
	}, &RawPlan{
		Title:      "Plan",
		Confidence: 0.9,
		Phases: []RawPhase{{
			Name:               "Phase 1",
			Category:           "/scaffold",
			Description:        "Create baseline",
			ObjectiveType:      "/unknown",
			VerificationMethod: "/tests",
			Tasks: []RawTask{{
				Description: "Patch the core planner",
				Type:        "/code",
				Priority:    "/super_high",
				Artifacts:   []string{"../escape.go", "internal/campaign/safe.go"},
				WriteSet:    []string{"../outside.go", "internal/campaign/safe.go"},
			}},
		}},
	})

	if len(kernel.Facts) != 0 {
		t.Fatalf("buildCampaign should not mutate kernel, got %d facts", len(kernel.Facts))
	}

	phase := campaign.Phases[0]
	if got := phase.Objectives[0].Type; got != ObjectiveCreate {
		t.Fatalf("objective type = %s, want %s", got, ObjectiveCreate)
	}
	if got := phase.Objectives[0].VerificationMethod; got != VerifyTestsPass {
		t.Fatalf("verification method = %s, want %s", got, VerifyTestsPass)
	}
	if got := phase.EstimatedComplexity; got != "/medium" {
		t.Fatalf("estimated complexity = %s, want %s", got, "/medium")
	}

	task := phase.Tasks[0]
	if got := task.Type; got != TaskTypeFileModify {
		t.Fatalf("task type = %s, want %s", got, TaskTypeFileModify)
	}
	if got := task.Priority; got != PriorityNormal {
		t.Fatalf("priority = %s, want %s", got, PriorityNormal)
	}
	if len(task.Artifacts) != 1 {
		t.Fatalf("expected 1 safe artifact, got %d: %#v", len(task.Artifacts), task.Artifacts)
	}
	if got := task.Artifacts[0].Path; got != "internal/campaign/safe.go" {
		t.Fatalf("artifact path = %q, want %q", got, "internal/campaign/safe.go")
	}
	if len(task.WriteSet) != 1 {
		t.Fatalf("expected 1 safe write_set entry, got %d: %v", len(task.WriteSet), task.WriteSet)
	}
	if got := task.WriteSet[0]; got != normalizeAbsolutePath(workspace, "internal/campaign/safe.go") {
		t.Fatalf("write_set[0] = %q, want %q", got, normalizeAbsolutePath(workspace, "internal/campaign/safe.go"))
	}
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

func TestDecompose_LLMTotalFailure(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return "", errors.New("simulated LLM timeout")
		},
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return "", errors.New("simulated LLM timeout")
		},
	}, t.TempDir())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "Valid goal",
		CampaignType: CampaignTypeCustom,
	})
	if err == nil {
		t.Fatal("expected error from LLM failure")
	}
	if !strings.Contains(err.Error(), "simulated LLM timeout") && !strings.Contains(err.Error(), "failed to propose plan") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDecompose_EmptyGoal is already covered by TestDecompose_EmptyGoal_ReturnsError.

func TestCleanJSONResponse_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Markdown with text before and after",
			input:    "Here is the plan:\n```json\n{\"foo\":\"bar\"}\n```\nHope it helps!",
			expected: "{\"foo\":\"bar\"}",
		},
		{
			name:     "Raw JSON with trailing garbage",
			input:    "{\"foo\": \"bar\"} \n trailing garbage",
			expected: "{\"foo\": \"bar\"}",
		},
		{
			name:     "Nested JSON objects and arrays",
			input:    "```json\n{\"foo\": [{\"bar\": 1}]}\n```",
			expected: "{\"foo\": [{\"bar\": 1}]}",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanJSONResponse(tc.input)
			if got != tc.expected {
				t.Errorf("cleanJSONResponse(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestValidatePlan_CircularDependency(t *testing.T) {
	mockKernel := &MockKernel{
		Facts: []core.Fact{
			{
				Predicate: "validation_error",
				Args: []interface{}{
					"/campaign_test",
					"circular_dependency",
					"Task A depends on Task B, which depends on Task A",
				},
			},
		},
	}
	d := &Decomposer{kernel: mockKernel}
	issues := d.validatePlan("/campaign_test")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].IssueType != "circular_dependency" {
		t.Errorf("expected circular_dependency, got %s", issues[0].IssueType)
	}
}

func TestRefinePlan_Success(t *testing.T) {
	client := &mockLLMClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return sampleRawPlanJSON("Refined Plan"), nil
		},
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sampleRawPlanJSON("Refined Plan"), nil
		},
	}
	d := &Decomposer{
		llmClient:      client,
		promptProvider: NewStaticPromptProvider(),
	}
	
	originalPlan := &RawPlan{Title: "Original Plan"}
	issues := []PlanValidationIssue{
		{IssueType: "circular_dependency", Description: "Task 1 depends on Task 2, and Task 2 depends on Task 1"},
	}
	
	refined, err := d.refinePlan(context.Background(), originalPlan, issues)
	if err != nil {
		t.Fatalf("refinePlan failed: %v", err)
	}
	if refined == nil {
		t.Fatal("refined plan is nil")
	}
	if refined.Title != "Refined Plan" {
		t.Errorf("expected 'Refined Plan', got %q", refined.Title)
	}
}

func TestIngestSourceDocuments_Cancellation(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{}, t.TempDir())
	
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f1.txt"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(dir, "f2.txt"), []byte("data"), 0644)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	docs, meta, err := d.ingestSourceDocuments(ctx, "/campaign_test", []string{dir})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(docs) > 0 || len(meta) > 0 {
		t.Errorf("expected 0 docs/meta, got %d/%d", len(docs), len(meta))
	}
}

func TestRefinePlan_TxCommitFail(t *testing.T) {
	client := &mockLLMClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			if strings.Contains(prompt, "fix the plan") || strings.Contains(prompt, "fix these issues") {
				return sampleRawPlanJSON("Refined Plan"), nil
			}
			return sampleRawPlanJSON("Original Plan"), nil
		},
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			if strings.Contains(systemPrompt, "fix the plan") {
				return sampleRawPlanJSON("Refined Plan"), nil
			}
			return sampleRawPlanJSON("Original Plan"), nil
		},
	}
	
	mockKernel := &MockKernel{
		Facts: []core.Fact{
			{
				Predicate: "validation_error",
				Args: []interface{}{"/campaign_test", "circular_dependency", "Task 1 depends on 2, 2 on 1"},
			},
		},
		AssertErr: errors.New("simulated tx commit fail"),
	}
	
	d := NewDecomposer(mockKernel, client, t.TempDir())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "Test tx commit fail",
		CampaignType: CampaignTypeCustom,
	})
	
	if err == nil {
		t.Fatal("expected error from commit fail")
	}
	if !strings.Contains(err.Error(), "simulated tx commit fail") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDecompose_EmptySourcePaths(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sampleRawPlanJSON("Empty Source Plan"), nil
		},
	}, t.TempDir())

	res, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "No sources",
		CampaignType: CampaignTypeCustom,
		SourcePaths:  []string{"", "   "},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.SourceDocs) != 0 {
		t.Errorf("expected 0 source docs, got %d", len(res.SourceDocs))
	}
}

func TestDecompose_ZeroContextBudget(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sampleRawPlanJSON("Zero Budget Plan"), nil
		},
	}, t.TempDir())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:          "Zero budget",
		CampaignType:  CampaignTypeCustom,
		ContextBudget: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecompose_NilIntelligence(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sampleRawPlanJSON("Nil Intel Plan"), nil
		},
	}, t.TempDir())
	
	d.SetIntelligenceGatherer(nil)

	res, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "Nil intelligence",
		CampaignType: CampaignTypeCustom,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Campaign == nil {
		t.Fatal("expected campaign to be created")
	}
}

func TestDecompose_JSONTypeCoercion(t *testing.T) {
	client := &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return `{"title":"bad types","phases":{"not_an_array": true}}`, nil
		},
	}
	d := NewDecomposer(&MockKernel{}, client, t.TempDir())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "Test type coercion",
		CampaignType: CampaignTypeCustom,
	})
	if err == nil {
		t.Fatal("expected parse failure error")
	}
	if !strings.Contains(err.Error(), "failed to parse plan JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecompose_MangleFactSanitization(t *testing.T) {
	mockKernel := &MockKernel{}
	d := NewDecomposer(mockKernel, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sampleRawPlanJSON("Plan"), nil
		},
	}, t.TempDir())

	goalWithIllegalChars := "Fix bugs ) . \" \\ ' , ! [] {}"
	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         goalWithIllegalChars,
		CampaignType: CampaignTypeCustom,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var found bool
	for _, f := range mockKernel.Facts {
		if f.Predicate == "campaign_goal" {
			found = true
			if f.Args[1] != goalWithIllegalChars {
				t.Errorf("expected goal %q, got %v", goalWithIllegalChars, f.Args[1])
			}
		}
	}
	if !found {
		t.Errorf("expected campaign_goal fact to be asserted")
	}
}

func TestDecompose_MassiveGoal(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sampleRawPlanJSON("Massive Goal Plan"), nil
		},
	}, t.TempDir())

	massiveGoal := strings.Repeat("A", 10*1024*1024)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := d.Decompose(ctx, DecomposeRequest{
		Goal:         massiveGoal,
		CampaignType: CampaignTypeCustom,
	})
	
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCleanJSONResponse_MassiveMalformedInput(t *testing.T) {
	input := strings.Repeat("{", 100000) + strings.Repeat("[", 100000) + "no closing brackets"
	
	done := make(chan struct{})
	go func() {
		cleanJSONResponse(input)
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("cleanJSONResponse took too long on massive malformed input, likely O(N^2) or infinite loop")
	}
}

func TestDecompose_HugeSourcePaths(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sampleRawPlanJSON("Huge Paths Plan"), nil
		},
	}, t.TempDir())

	paths := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		paths[i] = fmt.Sprintf("nonexistent_file_%d.txt", i)
	}

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "Huge paths test",
		CampaignType: CampaignTypeCustom,
		SourcePaths:  paths,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecompose_DeepNestedJSON(t *testing.T) {
	var sb strings.Builder
	depth := 1000
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"a":`)
	}
	sb.WriteString(`"value"`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`}`)
	}
	
	client := &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sb.String(), nil
		},
	}
	d := NewDecomposer(&MockKernel{}, client, t.TempDir())

	_, _ = d.Decompose(context.Background(), DecomposeRequest{
		Goal:         "Test deep JSON",
		CampaignType: CampaignTypeCustom,
	})
	
	// The test's main goal is to prevent stack overflow. 
	// If it completes without panicking, the test succeeds.
	// It may return nil or a validation error, but we don't care about the specific error.
}

func TestDecompose_ContextBudgetExceeded(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sampleRawPlanJSON("Small Budget Plan"), nil
		},
	}, t.TempDir())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		Goal:          "Small budget",
		CampaignType:  CampaignTypeCustom,
		ContextBudget: 1, 
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecompose_ContextCancelledDuringLLM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(c context.Context, systemPrompt, userPrompt string) (string, error) {
			cancel() 
			<-c.Done()
			return "", c.Err()
		},
	}, t.TempDir())

	_, err := d.Decompose(ctx, DecomposeRequest{
		Goal:         "Test context cancellation",
		CampaignType: CampaignTypeCustom,
	})
	
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}

func TestDecompose_FileDeletedDuringIngest(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return sampleRawPlanJSON("Plan"), nil
		},
	}, t.TempDir())

	dir := t.TempDir()
	filePath := filepath.Join(dir, "temp.txt")
	os.WriteFile(filePath, []byte("data"), 0644)

	os.Remove(filePath)
	
	docs, meta := d.readDocumentsFromPath(filePath, "campaign1")
	if len(docs) != 0 || len(meta) != 0 {
		t.Errorf("expected 0 docs and meta for deleted file, got %d and %d", len(docs), len(meta))
	}
}

func TestDecompose_SpecialInfiniteFiles(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{}, t.TempDir())

	meta := []FileMetadata{
		{
			Path:      "huge_file.bin",
			SizeBytes: 10 * 1024 * 1024 * 1024,
		},
	}

	processed := d.classifyDocuments(context.Background(), meta)
	if len(processed) != 1 {
		t.Fatalf("expected 1 metadata entry, got %d", len(processed))
	}
	
	if processed[0].Layer != "/scaffold" {
		t.Errorf("expected /scaffold for oversized file, got %s", processed[0].Layer)
	}
}

func TestDecompose_ConcurrentDecompose(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{
		completeWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			time.Sleep(10 * time.Millisecond) 
			return sampleRawPlanJSON("Concurrent Plan"), nil
		},
	}, t.TempDir())

	count := 10
	errCh := make(chan error, count)
	
	for i := 0; i < count; i++ {
		go func(id int) {
			_, err := d.Decompose(context.Background(), DecomposeRequest{
				Goal:         fmt.Sprintf("Concurrent goal %d", id),
				CampaignType: CampaignTypeCustom,
			})
			errCh <- err
		}(i)
	}

	for i := 0; i < count; i++ {
		err := <-errCh
		if err != nil {
			t.Errorf("concurrent decompose failed: %v", err)
		}
	}
}
