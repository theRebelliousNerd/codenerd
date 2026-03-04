package campaign

import (
	"codenerd/internal/core"
	"context"
	"errors"
	"fmt"
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
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
