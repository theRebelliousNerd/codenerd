package campaign

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TODO: TEST_GAP: Null/Undefined/Empty Input Vectors
// - TestReplanner_NilDependencies: Verify that all public methods (Replan, ReplanForNewRequirement, RefineNextPhase) gracefully return ErrNilKernel when the Replanner is instantiated with a nil core.Kernel or nil LLMClient, preventing panics during rollback operations.
// - TestReplan_EmptyCampaignID: Call Replan with `Campaign{ID: ""}` and verify it returns ErrInvalidCampaignID to prevent asserting empty strings into Mangle atoms.
// - TestReplan_WhitespaceFailedTaskID: Pass `"   "` as failedTaskID to Replan and verify it is trimmed and treated correctly (either returning an error or defaulting to phase replan) without causing nil pointer dereferences.
// - TestReplan_EmptyLLMResponse: Mock the LLM to return `{}`, `[]`, `null`, and `""` to verify that `json.Unmarshal` failures or default boolean states (`success: false`) do not corrupt the Go struct state or Mangle DB.
// - TestReplan_EmptyTaskAttemptError: Feed a Campaign with empty `TaskAttempt.Error` strings to `buildReplanContext` and verify the output formatting remains clean without appending ambiguous context lines.

// TODO: TEST_GAP: Type Coercion & Malformed Data
// - TestReplan_InvalidEnumCoercionToMangle: Mock LLM output with invalid enum strings (e.g., `priority: "URGENT"`). Assert that `normalizeReplanResponse` intercepts these and forces valid defaults before asserting them as Mangle atoms, preventing silent logic failures (0 tuples) in `intent_routing.mg`.
// - TestReplan_BooleanAndFloatCoercion: Provide `{"success": "true"}` (string) and `{"phase_order": 1.5}` (float) in the LLM payload to ensure `json.Unmarshal` failures prompt a quick retry rather than corrupting execution state.
// - TestReplan_TruncatedJSONResponse: Mock a token-limit truncation mid-stream (e.g., `{"new_tasks": [{"description": "Write a func`). Assert that the Replanner aborts cleanly and the Kernel transaction rollback is invoked.
// - TestReplan_HallucinatedTaskID: Return a non-existent task ID (e.g., `/task_test_999`) in the `retry_tasks` array. Verify the system safely skips it or returns a targeted error without panicking on slice bounds out of range.
// - TestReplan_MangleAtomVsStringDissonance: In Go tests, construct an LLM response with valid string values. After processing, use `store.Read()` to retrieve the raw Mangle facts. Assert that `arg.Type()` explicitly returns `ast.NameType` for Priority, Status, and Type, NOT `ast.StringType`.

// TODO: TEST_GAP: User Request Extremes & System Stress
// - TestBuildReplanContext_ExtremeTokenExhaustion: Create a mock Campaign with 50 phases, 200 tasks, and massive error dumps. Assert `buildReplanContext` strictly bounds output length (e.g., `<= maxReplanContextChars`) to prevent HTTP 400 TokenLimitExceeded from the LLM provider.
// - TestReplan_InfiniteLoopPrevention: Simulate an LLM generating plans for an impossible task. Assert that the Replanner refuses to retry a task whose attempt count exceeds `MaxRetries`, breaking the infinite Replan -> Fail loop.
// - TestReplan_DeeplyRecursiveDependencies: Mock LLM returning cyclic dependencies (A->B, B->C, C->A). Assert that the Replanner catches this graph cycle before asserting it, or that Mangle's `analysis.Analyze` rejects the transaction.
// - TestReplan_PromptInjectionInErrors: Inject instruction overrides (e.g., `Ignore previous instructions...`) into `TaskAttempt.Error`. Validate that `buildReplanContext` properly delimits variables (e.g., using `<error>` XML tags) to mitigate injection crossover.
// - TestReplan_MassiveTaskGeneration: Mock LLM returning 50,000 new tasks. Ensure the Replanner enforces a hard maximum (e.g., 100), rejecting the output to prevent massive Go slice reallocations and SQLite "too many variables" locks.

// TODO: TEST_GAP: State Conflicts & Race Conditions
// - TestReplan_TornWriteRaceCondition: Spin up 10 goroutines reading `campaign.Phases` and 10 calling `ReplanForNewRequirement`. Run `go test -race` to prove the `sync.Mutex` prevents torn writes during slice appends.
// - TestReplan_KernelTransactionFailureStateReversibility: Mock `kernel.Transaction()` to return an error midway through asserting tasks. Assert that the Go `*Campaign` struct is completely untouched and reverted to its pre-Replan state.
// - TestReplan_StaleContextReplanning: Simulate a 15-second LLM delay where `campaign.CompletedTasks` increments in the background. Assert the Replanner detects the state change (via RevisionNumber or counters) and merges safely or aborts.
// - TestReplan_GhostFactsInKernel: Update a task's priority via the Replanner. Use `core.Kernel` to verify that an explicit retraction of the old fact occurred before the new fact was asserted, preventing Cartesian explosion in Mangle joins.
// - TestReplan_DuplicateTaskIDsAcrossPhases: Mock the LLM to return `{"task_id": "/task_existing"}` for a new task. Assert that the Replanner forces the generation of a fresh UUID, strictly ignoring hallucinated duplicate IDs.
// - TestReplan_ConcurrentTaskExecutorCallbacks: Simulate an in-flight task execution while `Replan` skips that same task. Verify that late-arriving results from the skipped task are gracefully dropped and do not resurrect it.

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
