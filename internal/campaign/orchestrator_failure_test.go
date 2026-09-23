package campaign

import (
	"codenerd/internal/config"
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

// newPolicyFailureOrchestrator is an orchestrator over the shipped kernel, for
// tests of what a failed task does next: the move is derived
// (task_next_move, policy/campaign_decisions.mg), so a mock kernel would fail
// every task for want of one.
func newPolicyFailureOrchestrator(t *testing.T, edit func(*config.CampaignConfig), task Task) (*Orchestrator, core.Kernel) {
	t.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       kernel,
		LLMClient:    &MockLLMClient{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		TaskExecutor: &MockTaskExecutor{},
		EventChan:    make(chan OrchestratorEvent, 64),
		Campaign:     testCampaignConfig(edit),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	task.PhaseID = "/phase_failure_policy"
	task.Status = TaskPending
	now := time.Now()
	orch.campaign = &Campaign{
		ID: "/campaign_failure_policy", Type: CampaignTypeCustom, Title: "failure policy", Goal: "g",
		Status: StatusActive, CreatedAt: now, UpdatedAt: now, TotalPhases: 1, TotalTasks: 1,
		Phases: []Phase{{
			ID: "/phase_failure_policy", CampaignID: "/campaign_failure_policy", Name: "p",
			Status: PhaseInProgress, Tasks: []Task{task},
		}},
	}
	if err := kernel.LoadFacts(orch.campaign.ToFacts()); err != nil {
		t.Fatalf("load campaign facts: %v", err)
	}
	return orch, kernel
}

// redSuite is a failure whose turn verdict found the tests not green.
func redSuite() error {
	return withSignals(fmt.Errorf("turn: %w", ErrTaskNotDone), "/unverified", "/tests_not_green")
}

func failTask(orch *Orchestrator, id string, err error) *Task {
	phase := &orch.campaign.Phases[0]
	live, _, _ := orch.liveTaskLocked(id)
	orch.handleTaskFailure(context.Background(), phase, live, err)
	live, _, _ = orch.liveTaskLocked(id)
	return live
}

func reproTasks(orch *Orchestrator) []Task {
	var out []Task
	for _, t := range orch.campaign.Phases[0].Tasks {
		if isReproDiagnosticTask(&t) {
			out = append(out, t)
		}
	}
	return out
}

// The calibration case (campaign 7b853890): a Markdown task failed twice, and a
// repro task ran `go test ./...` for 29 minutes before the Markdown task could
// retry. A repro is owed only by a task that writes code; a document task never
// owes one, whatever its failure said.
func TestHandleTaskFailure_ADocumentTaskNeverOwesARepro(t *testing.T) {
	orch, _ := newPolicyFailureOrchestrator(t, nil, Task{
		ID: "/task_doc", Description: "write the gap analysis", Type: TaskTypeFileCreate,
		WriteSet: []string{"Docs/architecture/features/03-GAP-ANALYSIS.md"},
	})
	for i, err := range []error{
		errors.New("task unresolved: the working policy derived working_stop(/repeated_cycle)"),
		redSuite(),
		redSuite(),
	} {
		live := failTask(orch, "/task_doc", err)
		if n := len(reproTasks(orch)); n != 0 {
			t.Fatalf("failure %d of a document task inserted %d repro task(s)", i+1, n)
		}
		if live.Status != TaskPending || live.NextRetryAt.IsZero() {
			t.Fatalf("failure %d: status %s, next retry %v; want a pending retry", i+1, live.Status, live.NextRetryAt)
		}
	}
}

// A task that writes Go and fails on a red suite repro_after_failures times gets
// one repro task first, depends on it, and further failures reuse it.
func TestHandleTaskFailure_ACodeTaskOnARedSuiteGetsOneRepro(t *testing.T) {
	orch, kernel := newPolicyFailureOrchestrator(t, func(c *config.CampaignConfig) {
		c.MaxTaskAttempts = 5
		c.ReproAfterFailures = 2
	}, Task{
		ID: "/task_code", Description: "fix the pool", Type: TaskTypeFileModify,
		WriteSet: []string{"internal/pool/pool.go"},
	})

	failTask(orch, "/task_code", redSuite())
	if n := len(reproTasks(orch)); n != 0 {
		t.Fatalf("the first red-suite failure inserted %d repro task(s); the policy asks for 2", n)
	}
	live := failTask(orch, "/task_code", redSuite())
	repros := reproTasks(orch)
	if len(repros) != 1 {
		t.Fatalf("the second red-suite failure inserted %d repro tasks, want 1", len(repros))
	}
	if repros[0].Type != TaskTypeTestRun || repros[0].InferredFrom != "/task_code" {
		t.Fatalf("repro task = %+v", repros[0])
	}
	if !slices.Contains(live.DependsOn, repros[0].ID) {
		t.Fatalf("the failed task does not depend on its repro (deps %v)", live.DependsOn)
	}
	deps, _ := kernel.Query("task_dependency")
	found := false
	for _, f := range deps {
		if len(f.Args) >= 2 && fmt.Sprint(f.Args[0]) == "/task_code" && fmt.Sprint(f.Args[1]) == repros[0].ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("no task_dependency(/task_code, %s) in the kernel", repros[0].ID)
	}

	failTask(orch, "/task_code", redSuite())
	if n := len(reproTasks(orch)); n != 1 {
		t.Fatalf("a third failure left %d repro tasks, want the one still active", n)
	}
}

// A failure that is not a red suite never owes a repro, on a code task either.
func TestHandleTaskFailure_ACodeTaskFailingOtherwiseOwesNoRepro(t *testing.T) {
	orch, _ := newPolicyFailureOrchestrator(t, func(c *config.CampaignConfig) { c.MaxTaskAttempts = 5 }, Task{
		ID: "/task_code", Description: "fix the pool", Type: TaskTypeFileModify,
		WriteSet: []string{"internal/pool/pool.go"},
	})
	for range 3 {
		failTask(orch, "/task_code", errors.New("compile failed: undefined: Pool"))
	}
	if n := len(reproTasks(orch)); n != 0 {
		t.Fatalf("a code task failing without a red suite got %d repro task(s)", n)
	}
}

// A repro task that fails does not get a repro of its own.
func TestHandleTaskFailure_AReproNeverOwesARepro(t *testing.T) {
	orch, _ := newPolicyFailureOrchestrator(t, func(c *config.CampaignConfig) { c.MaxTaskAttempts = 5 }, Task{
		ID: "/task_code/repro_002", Description: reproDiagnosticDescriptionPrefix + " reproduce", Type: TaskTypeTestRun,
		InferredFrom: "/task_code", InferenceConf: 1, InferenceReason: reproInferenceReason,
		// A Go write set, so everything but task_is_repro says it owes one.
		WriteSet: []string{"internal/pool/pool_test.go"},
	})
	for range 3 {
		failTask(orch, "/task_code/repro_002", redSuite())
	}
	if n := len(reproTasks(orch)); n != 1 {
		t.Fatalf("a failing repro task produced %d repro tasks; want only itself", n)
	}
}

// A refusal is waited out on the full backoff; any other failure retries on
// the shorter one.
func TestHandleTaskFailure_ARefusalWaitsLongerThanAFailure(t *testing.T) {
	edit := func(c *config.CampaignConfig) {
		c.RetryBackoffBase = "20s"
		c.RetryBackoffMax = "10m"
		c.RetryWithReasonBackoffMax = "30s"
	}
	// The second attempt's backoff is 20s << 1 = 40s: above the 30s cap.
	wait := func(err error) time.Duration {
		orch, _ := newPolicyFailureOrchestrator(t, edit, Task{ID: "/task_x", Description: "x", Type: TaskTypeFileModify})
		failTask(orch, "/task_x", err)
		live := failTask(orch, "/task_x", err)
		if live.Status != TaskPending {
			t.Fatalf("status %s, want a pending retry", live.Status)
		}
		return live.NextRetryAt.Sub(live.Attempts[len(live.Attempts)-1].Timestamp)
	}
	if got := wait(refusal()); got != 40*time.Second {
		t.Errorf("a refusal waits %v, want the full 40s", got)
	}
	if got := wait(errors.New("compile failed")); got != 30*time.Second {
		t.Errorf("a failure waits %v, want the 30s cap", got)
	}
}

// At the attempt cap a task fails; with replan_at_attempt_cap off it fails
// outright.
func TestHandleTaskFailure_TheCapFailsTheTask(t *testing.T) {
	no := false
	orch, _ := newPolicyFailureOrchestrator(t, func(c *config.CampaignConfig) {
		c.MaxTaskAttempts = 1
		c.ReplanAtAttemptCap = &no
	}, Task{ID: "/task_x", Description: "x", Type: TaskTypeFileModify})
	if live := failTask(orch, "/task_x", errors.New("fail fast")); live.Status != TaskFailed {
		t.Fatalf("status %s at the cap, want %s", live.Status, TaskFailed)
	}
}

func newFailureTestOrchestrator(t *testing.T, maxRetries int) (*Orchestrator, *MockKernel, chan OrchestratorEvent) {
	t.Helper()

	kernel := &MockKernel{}
	eventCh := make(chan OrchestratorEvent, 32)

	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       kernel,
		LLMClient:    &MockLLMClient{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		ShardManager: nil,
		TaskExecutor: &MockTaskExecutor{},
		EventChan:    eventCh,
		Campaign:     testCampaignConfig(func(c *config.CampaignConfig) { c.MaxTaskAttempts = maxRetries + 1 }),
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

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify insertReproDiagnosticTaskLocked with empty or nil slices (e.g. phase.Tasks == nil).
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify findActiveReproTaskID with nil tasks slice.
// TODO: TEST_GAP: [Type Coercion] Verify Mangle Fact Type Dissonance in task_error assertions (ensuring ast.Name is used, not string "/logic").
// TODO: TEST_GAP: [Type Coercion] Verify task_retry_at Timestamp Coercion correctly handles int64 vs float64/int limits in Mangle layer.
// TODO: TEST_GAP: [User Request Extremes] Verify Unbounded Retries and Integer Overflow in computeRetryBackoff (passing math.MaxInt32).
// TODO: TEST_GAP: [User Request Extremes] Verify Repro Task Cascade (Infinite Insertion Loop) - a repro task failing should not spawn another repro task.
// TODO: TEST_GAP: [State Conflicts] Verify Race Condition during Phase/Task Mutation (e.g. AbortCampaign called while handleTaskFailure holds mu lock).
// TODO: TEST_GAP: [State Conflicts] Verify Kernel State vs In-Memory State Desynchronization if kernel.Assert throws an error halfway through handler.
// TODO: TEST_GAP: [State Conflicts] Verify TOC/TOU (Time of Check / Time of Use) in Repro Task Dependency Assertion (Go struct mutated before Kernel fact).
// TODO: TEST_GAP: [State Conflicts] Verify Concurrent Failure Handling for the Same Task (ensuring duplicate Repro tasks are not spawned).

// TODO: Gap - Null/Undefined/Empty: Test handleTaskFailure when task.ID is an empty string. Validate kernel assertion safety.
// TODO: Gap - User Request Extremes: Test computeRetryBackoff with RetryBackoffBase/Max set to time.Duration(math.MaxInt64) to check for integer overflow causing negative wait times.
// TODO: Gap - State Conflicts: Test handleTaskFailure when the kernel.Assert returns an error (e.g. read-only mode). Ensure orchestrator state doesn't desync or hang.
// TODO: Gap - Type Coercion / Adversarial: Test handleTaskFailure where err contains unescaped Mangle syntax or adversarial payload strings to ensure they don't break kernel fact parsing.
// TODO: Gap - State Conflicts: Pass an already canceled context.Context to handleTaskFailure and verify if o.saveCampaign() blocks or correctly handles the cancellation.

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

// A backoff that is not positive is a contradiction in the config, and the
// orchestrator refuses it at construction rather than computing waits from it.
func TestOrchestratorFailure_NegativeBackoffIsRefused(t *testing.T) {
	_, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       &MockKernel{},
		LLMClient:    &MockLLMClient{},
		TaskExecutor: &MockTaskExecutor{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		Campaign:     testCampaignConfig(func(c *config.CampaignConfig) { c.RetryBackoffBase = "-1ns" }),
	})
	if err == nil || !strings.Contains(err.Error(), "retry_backoff_base") {
		t.Fatalf("NewOrchestrator error = %v, want a refusal naming campaign.retry_backoff_base", err)
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
// TODO: Gap - State Conflicts: Test handleTaskFailure when the kernel.Assert returns an error (e.g. read-only mode). Ensure orchestrator state doesn't desync or hang.
// TODO: Gap - Type Coercion / Adversarial: Test handleTaskFailure where err contains unescaped Mangle syntax or adversarial payload strings to ensure they don't break kernel fact parsing.
// TODO: Gap - State Conflicts: Pass an already canceled context.Context to handleTaskFailure and verify if o.saveCampaign() blocks or correctly handles the cancellation.

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
	orch.policy.RetryBackoffBase = time.Duration(math.MaxInt64)
	orch.policy.RetryBackoffMax = time.Duration(math.MaxInt64)

	backoff := orch.computeRetryBackoff(10, true)

	if backoff < 0 {
		t.Fatalf("computeRetryBackoff caused integer overflow and returned negative time: %v", backoff)
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
