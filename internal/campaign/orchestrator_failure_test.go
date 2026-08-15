package campaign

import (
	"codenerd/internal/core"
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"codenerd/internal/tactile"
)

func TestClassifyTaskError_DeterministicBuckets(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error defaults to logic",
			err:  nil,
			want: "/logic",
		},
		{
			name: "deadline exceeded is transient",
			err:  context.DeadlineExceeded,
			want: "/transient",
		},
		{
			name: "wrapped deadline exceeded is transient",
			err:  fmt.Errorf("executor timeout: %w", context.DeadlineExceeded),
			want: "/transient",
		},
		{
			name: "context canceled is transient",
			err:  context.Canceled,
			want: "/transient",
		},
		{
			name: "rate limit hint is transient",
			err:  errors.New("HTTP 429: too many requests"),
			want: "/transient",
		},
		{
			name: "network hint is transient",
			err:  errors.New("temporary network unavailable"),
			want: "/transient",
		},
		{
			name: "generic compile error is logic",
			err:  errors.New("compile failed: undefined symbol x"),
			want: "/logic",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyTaskError(tc.err)
			if got != tc.want {
				t.Fatalf("classifyTaskError(%v) = %s, want %s", tc.err, got, tc.want)
			}
		})
	}
}

func TestShouldEscalateLogicFailure_DeterministicPredicate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		attempts []TaskAttempt
		want     bool
	}{
		{
			name: "2 logic failures in last 3 attempts escalates",
			attempts: []TaskAttempt{
				{Outcome: "/failure", Timestamp: now.Add(-3 * time.Minute), Error: "compile failed: missing import"},
				{Outcome: "/failure", Timestamp: now.Add(-2 * time.Minute), Error: "timeout reaching service"},
				{Outcome: "/failure", Timestamp: now.Add(-1 * time.Minute), Error: "undefined variable x"},
			},
			want: true,
		},
		{
			name: "20 minute failing loop escalates even when last-3 logic count is below threshold",
			attempts: []TaskAttempt{
				{Outcome: "/failure", Timestamp: now.Add(-25 * time.Minute), Error: "compile failed: old issue"},
				{Outcome: "/failure", Timestamp: now.Add(-12 * time.Minute), Error: "network unavailable"},
				{Outcome: "/failure", Timestamp: now.Add(-6 * time.Minute), Error: "timeout reaching service"},
				{Outcome: "/failure", Timestamp: now.Add(-1 * time.Minute), Error: "compile failed: current issue"},
			},
			want: true,
		},
		{
			name: "transient-only failures do not escalate",
			attempts: []TaskAttempt{
				{Outcome: "/failure", Timestamp: now.Add(-5 * time.Minute), Error: "network unavailable"},
				{Outcome: "/failure", Timestamp: now.Add(-4 * time.Minute), Error: "rate limit exceeded"},
				{Outcome: "/failure", Timestamp: now.Add(-3 * time.Minute), Error: "connection refused"},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := shouldEscalateLogicFailure(tc.attempts, now)
			if got != tc.want {
				t.Fatalf("shouldEscalateLogicFailure() = %v (%s), want %v", got, reason, tc.want)
			}
			if got && strings.TrimSpace(reason) == "" {
				t.Fatalf("expected non-empty reason for escalation")
			}
		})
	}
}

func TestHandleTaskFailure_InsertsReproTaskAfterRepeatedLogicFailures(t *testing.T) {
	orch, kernel, events := newFailureTestOrchestrator(t, 5)

	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	orch.handleTaskFailure(context.Background(), phase, task, errors.New("compile failed: undefined symbol"))
	if got := len(orch.campaign.Phases[0].Tasks); got != 1 {
		t.Fatalf("expected no repro insertion on first failure, got %d tasks", got)
	}

	orch.handleTaskFailure(context.Background(), phase, task, errors.New("build failed: unresolved reference"))

	updatedPhase := &orch.campaign.Phases[0]
	if got := len(updatedPhase.Tasks); got != 2 {
		t.Fatalf("expected repro task insertion after deterministic escalation, got %d tasks", got)
	}

	reproTask := updatedPhase.Tasks[0]
	if !isReproDiagnosticTask(&reproTask) {
		t.Fatalf("expected first task to be repro diagnostic marker, got %#v", reproTask)
	}
	if reproTask.Type != TaskTypeTestRun {
		t.Fatalf("expected repro task type %s, got %s", TaskTypeTestRun, reproTask.Type)
	}
	if reproTask.Priority != PriorityCritical {
		t.Fatalf("expected repro task priority %s, got %s", PriorityCritical, reproTask.Priority)
	}
	if reproTask.InferredFrom != task.ID {
		t.Fatalf("expected repro inferred_from %s, got %s", task.ID, reproTask.InferredFrom)
	}
	if !strings.Contains(reproTask.Description, "run tests before next mutation") {
		t.Fatalf("expected repro description to include deterministic marker, got %q", reproTask.Description)
	}

	originalTask := updatedPhase.Tasks[1]
	if !containsString(originalTask.DependsOn, reproTask.ID) {
		t.Fatalf("expected failed task to depend on repro task %s, deps=%v", reproTask.ID, originalTask.DependsOn)
	}

	depFacts, _ := kernel.Query("task_dependency")
	foundDepFact := false
	for _, fact := range depFacts {
		if len(fact.Args) < 2 {
			continue
		}
		if fmt.Sprintf("%v", fact.Args[0]) == task.ID && fmt.Sprintf("%v", fact.Args[1]) == reproTask.ID {
			foundDepFact = true
			break
		}
	}
	if !foundDepFact {
		t.Fatalf("expected task_dependency fact for %s -> %s", task.ID, reproTask.ID)
	}

	taskErrFacts, _ := kernel.Query("task_error")
	foundReproMarker := false
	for _, fact := range taskErrFacts {
		if len(fact.Args) < 3 {
			continue
		}
		if fmt.Sprintf("%v", fact.Args[0]) == task.ID &&
			fmt.Sprintf("%v", fact.Args[1]) == "/repro_test_first_required" &&
			fmt.Sprintf("%v", fact.Args[2]) == reproTask.ID {
			foundReproMarker = true
			break
		}
	}
	if !foundReproMarker {
		t.Fatalf("expected /repro_test_first_required marker for task %s and repro %s", task.ID, reproTask.ID)
	}

	// Third failure should NOT insert another repro task while one is still active.
	orch.handleTaskFailure(context.Background(), phase, task, errors.New("compile failed: still broken"))
	if got := len(orch.campaign.Phases[0].Tasks); got != 2 {
		t.Fatalf("expected no duplicate repro insertion, got %d tasks", got)
	}

	// Auditability signal: escalation event should be emitted.
	foundEscalationEvent := false
	for {
		select {
		case evt := <-events:
			if evt.Type == "logic_failure_escalated" {
				foundEscalationEvent = true
			}
		default:
			if !foundEscalationEvent {
				t.Fatalf("expected logic_failure_escalated event")
			}
			return
		}
	}
}

func newFailureTestOrchestrator(t *testing.T, maxRetries int) (*Orchestrator, *MockKernel, chan OrchestratorEvent) {
	t.Helper()

	kernel := &MockKernel{}
	eventCh := make(chan OrchestratorEvent, 32)

	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:        t.TempDir(),
		Kernel:           kernel,
		LLMClient:        &MockLLMClient{},
		Executor:         tactile.NewDirectExecutor(),
		VirtualStore:     &core.VirtualStore{},
		ShardManager:     nil,
		TaskExecutor:     &MockTaskExecutor{},
		EventChan:        eventCh,
		MaxRetries:       maxRetries,
		DisableTimeouts:  true,
		CheckpointOnFail: false,
		AutoReplan:       false,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator() error = %v", err)
	}

	now := time.Now()
	orch.campaign = &Campaign{
		ID:        "campaign_failure_lane",
		Type:      CampaignTypeCustom,
		Title:     "Failure Lane",
		Goal:      "Test deterministic escalation",
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Phases: []Phase{
			{
				ID:         "phase_failure_lane",
				CampaignID: "campaign_failure_lane",
				Name:       "Failure Phase",
				Order:      0,
				Status:     PhaseInProgress,
				Tasks: []Task{
					{
						ID:          "task_mutate_1",
						PhaseID:     "phase_failure_lane",
						Description: "Modify source file",
						Status:      TaskPending,
						Type:        TaskTypeFileModify,
						Priority:    PriorityNormal,
						Order:       0,
					},
				},
			},
		},
		TotalPhases: 1,
		TotalTasks:  1,
	}

	return orch, kernel, eventCh
}

func containsString(values []string, target string) bool {
	return slices.Contains(values, target)
}

// -----------------------------------------------------------------------------
// Gap Implementations
// -----------------------------------------------------------------------------

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify classifyTaskError(err error) with completely empty or whitespace-only error strings.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify shouldEscalateLogicFailure(attempts []TaskAttempt, now time.Time) with an empty attempts slice.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify shouldEscalateLogicFailure with zero-value Timestamp in TaskAttempts.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify insertReproDiagnosticTaskLocked with empty or nil slices (e.g. phase.Tasks == nil).
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify findActiveReproTaskID with nil tasks slice.
// TODO: TEST_GAP: [Type Coercion] Verify Mangle Fact Type Dissonance in task_error assertions (ensuring ast.Name is used, not string "/logic").
// TODO: TEST_GAP: [Type Coercion] Verify task_retry_at Timestamp Coercion correctly handles int64 vs float64/int limits in Mangle layer.
// TODO: TEST_GAP: [User Request Extremes] Verify Massive Error Strings (50MB) in classifyTaskError and Kernel Assertions don't cause OOM or store limits.
// TODO: TEST_GAP: [User Request Extremes] Verify Unbounded Retries and Integer Overflow in computeRetryBackoff (passing math.MaxInt32).
// TODO: TEST_GAP: [User Request Extremes] Verify Repro Task Cascade (Infinite Insertion Loop) - a repro task failing should not spawn another repro task.
// TODO: TEST_GAP: [User Request Extremes] Verify Extreme Number of Task Attempts (1,000,000) passed to shouldEscalateLogicFailure does not cause CPU spike.
// TODO: TEST_GAP: [State Conflicts] Verify Race Condition during Phase/Task Mutation (e.g. AbortCampaign called while handleTaskFailure holds mu lock).
// TODO: TEST_GAP: [State Conflicts] Verify Kernel State vs In-Memory State Desynchronization if kernel.Assert throws an error halfway through handler.
// TODO: TEST_GAP: [State Conflicts] Verify TOC/TOU (Time of Check / Time of Use) in Repro Task Dependency Assertion (Go struct mutated before Kernel fact).
// TODO: TEST_GAP: [State Conflicts] Verify Concurrent Failure Handling for the Same Task (ensuring duplicate Repro tasks are not spawned).

// TODO: Gap - Null/Undefined/Empty: Test handleTaskFailure when task.ID is an empty string. Validate kernel assertion safety.
// TODO: Gap - User Request Extremes: Test computeRetryBackoff with RetryBackoffBase/Max set to time.Duration(math.MaxInt64) to check for integer overflow causing negative wait times.
// TODO: Gap - User Request Extremes: Test behavior when config.MaxRetries is explicitly 0 (fail-fast vs defaulting to 3).
// TODO: Gap - State Conflicts: Test handleTaskFailure when the kernel.Assert returns an error (e.g. read-only mode). Ensure orchestrator state doesn't desync or hang.
// TODO: Gap - Type Coercion / Adversarial: Test handleTaskFailure where err contains unescaped Mangle syntax or adversarial payload strings to ensure they don't break kernel fact parsing.
// TODO: Gap - State Conflicts: Pass an already canceled context.Context to handleTaskFailure and verify if o.saveCampaign() blocks or correctly handles the cancellation.
// TODO: Gap - User Request Extremes (Performance): Test shouldEscalateLogicFailure and handleTaskFailure with a task that has 100,000 previous attempts to ensure lock contention and memory pressure are manageable.

func TestOrchestratorFailure_NullEmptyPointers(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)

	// Should not panic on nil phase/task
	orch.handleTaskFailure(context.Background(), nil, nil, errors.New("error"))

	// Should not panic on task not in phase
	taskNotInPhase := &Task{ID: "not_in_phase"}
	phase := &orch.campaign.Phases[0]
	orch.handleTaskFailure(context.Background(), phase, taskNotInPhase, errors.New("error"))
}

func TestOrchestratorFailure_NilError(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	// Should not panic with nil err
	orch.handleTaskFailure(context.Background(), phase, task, nil)
}

func TestOrchestratorFailure_NegativeBackoff(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	// Force negative config
	orch.config.RetryBackoffBase = -1
	orch.config.RetryBackoffMax = -1

	backoff := orch.computeRetryBackoff("test-task", 1)
	if backoff < 0 {
		t.Fatalf("computeRetryBackoff returned negative duration: %v", backoff)
	}
}

func TestOrchestratorFailure_MassiveErrorString(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	massiveStr := strings.Repeat("A", 50*1024*1024)
	err := errors.New(massiveStr)

	// Should not OOM or hang
	orch.handleTaskFailure(context.Background(), phase, task, err)
}

func TestOrchestratorFailure_InfiniteRecursionProtection(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	// Make the task a Repro Diagnostic task
	task.Type = TaskTypeTestRun
	task.Description = "repro diagnostic: run tests before next mutation"

	// Fail the repro task
	orch.handleTaskFailure(context.Background(), phase, task, errors.New("logic failure in repro"))

	// Should NOT spawn another repro task
	if len(phase.Tasks) > 1 {
		t.Fatalf("expected no repro task spawned from a repro task, got %d", len(phase.Tasks))
	}
}

func TestOrchestratorFailure_StateConflicts_Concurrency(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	err := errors.New("some error")

	errCh := make(chan error, 50)
	for range 50 {
		go func() {
			// Catch any panic
			defer func() {
				if r := recover(); r != nil {
					errCh <- fmt.Errorf("panic: %v", r)
				} else {
					errCh <- nil
				}
			}()
			orch.handleTaskFailure(context.Background(), phase, task, err)
		}()
	}

	for range 50 {
		if e := <-errCh; e != nil {
			t.Fatalf("concurrent handleTaskFailure failed: %v", e)
		}
	}
}

// TODO: Gap - Null/Undefined/Empty: Test handleTaskFailure when task.ID is an empty string. Validate kernel assertion safety.
// TODO: Gap - User Request Extremes: Test computeRetryBackoff with RetryBackoffBase/Max set to time.Duration(math.MaxInt64) to check for integer overflow causing negative wait times.
// TODO: Gap - User Request Extremes: Test behavior when config.MaxRetries is explicitly 0 (fail-fast vs defaulting to 3).
// TODO: Gap - State Conflicts: Test handleTaskFailure when the kernel.Assert returns an error (e.g. read-only mode). Ensure orchestrator state doesn't desync or hang.
// TODO: Gap - Type Coercion / Adversarial: Test handleTaskFailure where err contains unescaped Mangle syntax or adversarial payload strings to ensure they don't break kernel fact parsing.
// TODO: Gap - State Conflicts: Pass an already canceled context.Context to handleTaskFailure and verify if o.saveCampaign() blocks or correctly handles the cancellation.
// TODO: Gap - User Request Extremes (Performance): Test shouldEscalateLogicFailure and handleTaskFailure with a task that has 100,000 previous attempts to ensure lock contention and memory pressure are manageable.

func TestOrchestratorFailure_EmptyTaskID(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	// Intentionally set an empty task ID
	task.ID = ""

	err := errors.New("some error")

	// Verify that it doesn't panic when task ID is empty
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleTaskFailure panicked with empty task.ID: %v", r)
		}
	}()

	orch.handleTaskFailure(context.Background(), phase, task, err)
}

func TestOrchestratorFailure_RetryBackoff_Overflow(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)

	// Cause overflow
	orch.config.RetryBackoffBase = time.Duration(math.MaxInt64)
	orch.config.RetryBackoffMax = time.Duration(math.MaxInt64)

	backoff := orch.computeRetryBackoff("task_mutate_1", 10)

	if backoff < 0 {
		t.Fatalf("computeRetryBackoff caused integer overflow and returned negative time: %v", backoff)
	}
}

func TestOrchestratorFailure_MaxRetriesZero(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	orch.config.MaxRetries = 0 // Explicitly set to 0 to bypass default 3 in NewOrchestrator

	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	err := errors.New("fail fast")

	orch.handleTaskFailure(context.Background(), phase, task, err)

	// Task should be failed immediately
	if task.Status != TaskFailed {
		t.Fatalf("expected task to be TaskFailed when MaxRetries is 0, got %s", task.Status)
	}
}

func TestOrchestratorFailure_AdversarialErrorString(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	adversarialStr := "this is a test error \") :- fail(). p(\""
	err := errors.New(adversarialStr)

	// Should not panic, or break kernel assertions
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleTaskFailure panicked on adversarial error string: %v", r)
		}
	}()

	orch.handleTaskFailure(context.Background(), phase, task, err)
}

func TestOrchestratorFailure_CanceledContext(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Already canceled

	err := errors.New("some error")

	// Verify that saveCampaign and handleTaskFailure handle canceled context correctly.
	orch.handleTaskFailure(ctx, phase, task, err)
}

func TestOrchestratorFailure_MassiveAttempts(t *testing.T) {
	orch, _, _ := newFailureTestOrchestrator(t, 5)
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]

	// Populate 100,000 attempts
	for i := 0; i < 100000; i++ {
		task.Attempts = append(task.Attempts, TaskAttempt{
			Timestamp: time.Now(),
			Error:     "test failure",
		})
	}

	err := errors.New("some error")

	start := time.Now()
	// Should not hang
	orch.handleTaskFailure(context.Background(), phase, task, err)
	duration := time.Since(start)

	if duration > 5*time.Second {
		t.Fatalf("handleTaskFailure took too long with 100k attempts: %v", duration)
	}
}
