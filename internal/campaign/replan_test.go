package campaign

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TODO: TEST_GAP: Null/Undefined/Empty Input Vectors
// 1. TestReplan_NilCampaign: Call Replan(ctx, nil, "") and assert it doesn't panic but returns an error.
// 2. TestReplanForNewRequirement_EmptyRequirement: Call ReplanForNewRequirement(ctx, validCampaign, "") and ensure error handling rejects empty string.
// 3. TestRefineNextPhase_NilCampaignOrPhase: Ensure RefineNextPhase gracefully returns when campaign or phase is nil.
// 4. TestReplan_EmptyLLMResponse: Mock the LLM to return `""` or `{}` and verify json unmarshalling fails cleanly without corrupting state.
// 5. TestReplanner_NilKernel: Verify instantiating NewReplanner with a nil kernel is either rejected or gracefully handled during method calls.

// TODO: TEST_GAP: Type Coercion & Malformed Data
// 1. TestReplanForNewRequirement_InvalidEnumTypes: Mock LLM to return invalid strings for enums like `TaskType` (e.g., `"/magic_fix"`) or `TaskPriority` (e.g., `"/super_high"`), and ensure the Replanner falls back to defaults instead of blindly passing invalid values to the kernel.
// 2. TestRefineNextPhase_MalformedJSON: Pass deeply nested, invalid, or string-escaped JSON outputs from the LLM and assert that the unmarshaling does not cause a panic and the system returns a parsable error for a retry loop.
// 3. TestRefineNextPhase_InvalidBooleanAndIntegerCoercion: Ensure LLM outputs like `{"success": "true", "phase_order": "0"}` do not trigger fatal parsing failures or crash the campaign progression.

// TODO: TEST_GAP: User Request Extremes & System Stress
// 1. TestContextPager_ExtremeCampaignHistory: Create a mock campaign with 5,000 phases and 100 failed tasks with massive attempt error strings. Assert that `buildReplanContext` truncates or summarizes the text so it does not exceed typical LLM token window limits (e.g., throwing a `TokenLimitExceeded` from the provider).
// 2. TestReplanner_UnexecutableInvention: Pass an LLM requirement to write code in a non-existent DSL or uninstalled framework. Validate that the system rejects the plan or errors instead of entering an infinite Replan -> Fail loop.
// 3. TestReplan_PromptInjection: Inject a prompt evasion string (e.g., `"; DROP TABLE; --`) into a mock failed task attempt's error message and assert it is properly delimited and escaped when sent to the LLM via `buildReplanContext`.

// TODO: TEST_GAP: State Conflicts & Race Conditions
// 1. TestConcurrentReplanning_RaceCondition: Spin up 10 goroutines calling `Replan` and `ReplanForNewRequirement` simultaneously on the same `*Campaign` pointer. Run with `-race` to expose the torn write race condition during slice appends, validating the need for a `sync.RWMutex`.
// 2. TestStateDesync_KernelAssertFailure: Mock the `core.Kernel` to return an error on `Assert()` or `LoadFacts()`. Validate that if the Kernel rejects the update, the `*Campaign` Go struct state is rolled back so the Go engine and Mangle logic engine remain synchronized.
// 3. TestReplan_MangleFactDuplication: Ensure that when a task's description is updated by the replanner, the previous Mangle fact for that task is explicitly retracted or overwritten, instead of leaving duplicate conflicting facts in the SQLite store.

func TestReplanner_RecursionFix(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "Mock response", nil
		},
	}

	// Create replanner with nil kernel (not needed for this test)
	// We pass the mock as the LLMClient
	r := NewReplanner(nil, mockLLM)

	// Context
	ctx := context.Background()

	// Execution
	// This should NOT panic with stack overflow
	resp, err := r.completeWithGrounding(ctx, "Test prompt")

	// Verification
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if resp != "Mock response" {
		t.Errorf("Expected 'Mock response', got '%s'", resp)
	}
}

func TestReplanner_RecursionFix_ErrorPropagates(t *testing.T) {
	// Setup
	expectedErr := errors.New("LLM error")
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "", expectedErr
		},
	}

	r := NewReplanner(nil, mockLLM)
	ctx := context.Background()

	// Execution
	_, err := r.completeWithGrounding(ctx, "Test prompt")

	// Verification
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestReplan_NilCampaign(t *testing.T) {
	r := NewReplanner(&MockKernel{}, &MockLLMClient{})

	err := r.Replan(context.Background(), nil, "")
	if !errors.Is(err, ErrNilCampaign) {
		t.Fatalf("expected ErrNilCampaign, got %v", err)
	}
}

func TestReplanForNewRequirement_EmptyRequirement(t *testing.T) {
	r := NewReplanner(&MockKernel{}, &MockLLMClient{})

	err := r.ReplanForNewRequirement(context.Background(), &Campaign{ID: "/campaign_test"}, "   ")
	if !errors.Is(err, ErrEmptyRequirement) {
		t.Fatalf("expected ErrEmptyRequirement, got %v", err)
	}
}

func TestReplanForNewRequirement_InvalidEnumsNormalized(t *testing.T) {
	kernel := &MockKernel{}
	r := NewReplanner(kernel, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{
				"new_tasks": [
					{
						"phase_order": 0,
						"description": "Write regression coverage",
						"type": "/magic_fix",
						"priority": "/super_high"
					}
				],
				"modified_tasks": [],
				"summary": "Added safer test work"
			}`, nil
		},
	})

	campaign := &Campaign{
		ID:              "/campaign_test",
		Title:           "Planner Reliability",
		CompletedPhases: 0,
		TotalPhases:     1,
		CompletedTasks:  0,
		TotalTasks:      0,
		Phases: []Phase{{
			ID:       "/phase_test_0",
			Order:    0,
			Category: "/test",
			Tasks:    nil,
		}},
	}

	if err := r.ReplanForNewRequirement(context.Background(), campaign, "Add regression coverage"); err != nil {
		t.Fatalf("ReplanForNewRequirement failed: %v", err)
	}

	if got := len(campaign.Phases[0].Tasks); got != 1 {
		t.Fatalf("expected 1 new task, got %d", got)
	}
	task := campaign.Phases[0].Tasks[0]
	if task.Type != TaskTypeTestWrite {
		t.Fatalf("task type = %s, want %s", task.Type, TaskTypeTestWrite)
	}
	if task.Priority != PriorityNormal {
		t.Fatalf("task priority = %s, want %s", task.Priority, PriorityNormal)
	}
	if campaign.TotalTasks != 1 {
		t.Fatalf("campaign.TotalTasks = %d, want 1", campaign.TotalTasks)
	}
}

func TestReplan_RollsBackOnKernelLoadFailure(t *testing.T) {
	loadErr := errors.New("kernel load failed")
	kernel := &MockKernel{LoadFactsErr: loadErr}
	r := NewReplanner(kernel, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{
				"success": true,
				"change_summary": "Retry with safer approach",
				"retry_tasks": [
					{"task_id": "/task_test_0_0", "new_approach": "Retry by shrinking prompt scope"}
				],
				"skip_tasks": [],
				"add_tasks": [],
				"modify_dependencies": []
			}`, nil
		},
	})

	campaign := &Campaign{
		ID:             "/campaign_test",
		Title:          "Planner Reliability",
		Goal:           "Harden replanning",
		CompletedTasks: 0,
		TotalTasks:     1,
		Phases: []Phase{{
			ID:    "/phase_test_0",
			Order: 0,
			Tasks: []Task{{
				ID:          "/task_test_0_0",
				PhaseID:     "/phase_test_0",
				Description: "Original failed task",
				Status:      TaskFailed,
				Type:        TaskTypeFileModify,
				Priority:    PriorityNormal,
				Attempts: []TaskAttempt{{
					Number:    1,
					Outcome:   "/failure",
					Timestamp: time.Now(),
					Error:     "compile failed",
				}},
				LastError: "compile failed",
			}},
		}},
	}

	err := r.Replan(context.Background(), campaign, "")
	if err == nil || !strings.Contains(err.Error(), "failed to reload campaign") {
		t.Fatalf("expected reload failure, got %v", err)
	}
	if got := campaign.Phases[0].Tasks[0].Description; got != "Original failed task" {
		t.Fatalf("campaign mutated despite load failure, description=%q", got)
	}
	if campaign.RevisionNumber != 0 {
		t.Fatalf("revision number mutated despite load failure: %d", campaign.RevisionNumber)
	}
}

func TestBuildReplanContext_TruncatesLargeHistory(t *testing.T) {
	r := NewReplanner(&MockKernel{}, &MockLLMClient{})
	campaign := &Campaign{
		ID:              "/campaign_test",
		Title:           "Very Large Campaign",
		Status:          StatusActive,
		CompletedPhases: 1,
		TotalPhases:     9,
		CompletedTasks:  2,
		TotalTasks:      20,
	}

	failedTasks := []Task{{
		ID:          "/task_test_0_0",
		Description: strings.Repeat("desc ", 2000),
		LastError:   strings.Repeat("error ", 2000),
		Attempts: []TaskAttempt{
			{Number: 1, Outcome: "/failure", Error: strings.Repeat("attempt1 ", 1000)},
			{Number: 2, Outcome: "/failure", Error: strings.Repeat("attempt2 ", 1000)},
			{Number: 3, Outcome: "/failure", Error: strings.Repeat("attempt3 ", 1000)},
			{Number: 4, Outcome: "/failure", Error: strings.Repeat("attempt4 ", 1000)},
			{Number: 5, Outcome: "/failure", Error: strings.Repeat("attempt5 ", 1000)},
		},
	}}

	contextText := r.buildReplanContext(campaign, failedTasks, nil, nil)
	if len(contextText) > maxReplanContextChars {
		t.Fatalf("context length = %d, want <= %d", len(contextText), maxReplanContextChars)
	}
	if got := strings.Count(contextText, "Attempt "); got != maxReplanAttemptsPerTask {
		t.Fatalf("attempt count in context = %d, want %d", got, maxReplanAttemptsPerTask)
	}
	if !strings.Contains(contextText, "[truncated]") {
		t.Fatalf("expected truncated marker in context, got %q", contextText)
	}
}
