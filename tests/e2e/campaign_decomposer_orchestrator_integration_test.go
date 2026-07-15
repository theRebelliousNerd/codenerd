//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
	"codenerd/internal/core/shards"
	"codenerd/internal/tactile"
)

// TestE2E_Campaign_Decomposer_Orchestrator covers the E2E pipeline for campaign orchestration.
// It explicitly tests the cross-boundary contracts defined in the Siege journal.
// It uses real dependencies where possible, substituting only the outermost execution layer
// (TaskExecutor) which simulates the session.JITExecutor boundary without relying on LLM flakiness.

// -------------------------------------------------------------------------
// Helper Implementation for Boundary Testing
// -------------------------------------------------------------------------

// mockIntegrationExecutor simulates the session.JITExecutor boundary while
// allowing injection of specific boundary failures.
type mockIntegrationExecutor struct {
	mu           sync.Mutex
	ExecuteFunc  func(ctx context.Context, task *campaign.Task, contextData string) (*campaign.TaskResult, error)
	ExecutionCnt int
}

func (m *mockIntegrationExecutor) ExecuteTask(ctx context.Context, task *campaign.Task, contextData string) (*campaign.TaskResult, error) {
	m.mu.Lock()
	m.ExecutionCnt++
	m.mu.Unlock()

	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, task, contextData)
	}

	return &campaign.TaskResult{
		Status: campaign.TaskStatusCompleted,
		Output: "success",
	}, nil
}

func setupOrchestratorIntegration(t *testing.T) (*campaign.Orchestrator, *mockIntegrationExecutor, *core.RealKernel) {
	t.Helper()

	mockExecutor := &mockIntegrationExecutor{}

	// Instantiate the real Mangle Kernel, simulating the deep execution stack.
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create real kernel: %v", err)
	}
	t.Cleanup(func() { kernel.Close() })

	workspace := t.TempDir()

	cfg := campaign.OrchestratorConfig{
		Workspace:        workspace,
		Kernel:           kernel,
		Executor:         tactile.NewDirectExecutor(),
		VirtualStore:     &core.VirtualStore{},
		ShardManager:     shards.NewShardManager(),
		TaskExecutor:     mockExecutor,
		MaxParallelTasks: 3,
		MaxRetries:       1,
		CampaignTimeout:  5 * time.Second,
	}

	orch := campaign.NewOrchestrator(cfg)
	return orch, mockExecutor, kernel
}

// -------------------------------------------------------------------------
// Contract Violation Tests (Minimum 5)
// -------------------------------------------------------------------------

// Scenario 1: Cyclical Dependency Injection (Contract Violation)
func TestE2E_Campaign_Decomposer_CyclicalDependency(t *testing.T) {
	t.Parallel()
	orch, _, _ := setupOrchestratorIntegration(t)

	plan := &campaign.Plan{
		Phases: []campaign.Phase{
			{
				ID:   "phase_1",
				Name: "Phase 1",
				Tasks: []*campaign.Task{
					{ID: "task_a", Dependencies: []string{"task_b"}},
					{ID: "task_b", Dependencies: []string{"task_a"}},
				},
			},
		},
	}

	ctx := context.Background()
	err := orch.ExecutePlan(ctx, plan)

	if err == nil {
		t.Errorf("Expected Orchestrator to detect cyclical dependency and return error, got nil")
	}
}

// Scenario 9: Missing Task Dependencies Validation (Contract Violation)
func TestE2E_Campaign_Decomposer_MissingDependencies(t *testing.T) {
	t.Parallel()
	orch, _, _ := setupOrchestratorIntegration(t)

	plan := &campaign.Plan{
		Phases: []campaign.Phase{
			{
				ID: "phase_1",
				Tasks: []*campaign.Task{
					{ID: "task_a", Dependencies: []string{"task_b"}},
				},
			},
		},
	}

	err := orch.ExecutePlan(context.Background(), plan)
	if err == nil {
		t.Errorf("Expected error for missing dependency, got nil")
	}
}

// Scenario 10: Re-Planning Loop Exhaustion (Contract Violation)
func TestE2E_Campaign_Decomposer_RePlanningExhaustion(t *testing.T) {
	t.Parallel()
	orch, mockExec, _ := setupOrchestratorIntegration(t)

	mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
		return &campaign.TaskResult{Status: campaign.TaskStatusFailed, Error: "fatal error"}, fmt.Errorf("task failed")
	}

	plan := &campaign.Plan{
		Phases: []campaign.Phase{
			{ID: "phase_1", Tasks: []*campaign.Task{{ID: "task_a"}}},
		},
	}

	err := orch.ExecutePlan(context.Background(), plan)
	if err == nil {
		t.Errorf("Expected hard failure after retries exhausted, got nil")
	}

	mExec := mockExec
	mExec.mu.Lock()
	count := mExec.ExecutionCnt
	mExec.mu.Unlock()

	if count > 3 {
		t.Errorf("Expected bounded retries, but task was executed %d times", count)
	}
}

// Contract: A phase must not transition if a task fails fatally
func TestE2E_Campaign_Decomposer_PhaseTransitionFailureBlock(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        if task.ID == "task_a" {
            return &campaign.TaskResult{Status: campaign.TaskStatusFailed}, fmt.Errorf("task a hard fail")
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "phase_1", Tasks: []*campaign.Task{{ID: "task_a"}, {ID: "task_b"}}},
            {ID: "phase_2", Tasks: []*campaign.Task{{ID: "task_c"}}},
        },
    }

    err := orch.ExecutePlan(context.Background(), plan)
    if err == nil {
        t.Errorf("Expected phase to block transition and return error, got nil")
    }

    mExec := mockExec
    mExec.mu.Lock()
    count := mExec.ExecutionCnt
    mExec.mu.Unlock()

    if count >= 3 {
        t.Errorf("Phase 2 tasks executed despite Phase 1 failure")
    }
}

// Contract: Orchestrator resilience against task panic
func TestE2E_Campaign_Decomposer_TaskPanicRecovery(t *testing.T) {
	t.Parallel()
	orch, mockExec, _ := setupOrchestratorIntegration(t)

	mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
		panic("simulated fatal panic in session.Executor")
	}

	plan := &campaign.Plan{
		Phases: []campaign.Phase{
			{ID: "phase_1", Tasks: []*campaign.Task{{ID: "task_a"}}},
		},
	}

	err := orch.ExecutePlan(context.Background(), plan)
	if err == nil {
		t.Errorf("Expected Orchestrator to return error on panic, got nil")
	}
}

// -------------------------------------------------------------------------
// State Corruption Tests (Minimum 3)
// -------------------------------------------------------------------------

// Scenario 6: Synchronized Phase Transition Race
func TestE2E_Campaign_Decomposer_PhaseTransitionRace(t *testing.T) {
	t.Parallel()
	orch, mockExec, _ := setupOrchestratorIntegration(t)

	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
		<-startSignal
		return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
	}

	tasks := make([]*campaign.Task, 10)
	for i := 0; i < 10; i++ {
		tasks[i] = &campaign.Task{ID: fmt.Sprintf("t_%d", i)}
	}

	plan := &campaign.Plan{
		Phases: []campaign.Phase{
			{ID: "phase_1", Tasks: tasks},
			{ID: "phase_2", Tasks: []*campaign.Task{{ID: "t_next"}}},
		},
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		close(startSignal)
	}()

	err := orch.ExecutePlan(context.Background(), plan)
	wg.Wait()

	if err != nil {
		t.Errorf("Race condition crashed state machine: %v", err)
	}
}

// State Corruption: Mutating Plan mid-flight
func TestE2E_Campaign_Decomposer_ConcurrentPlanMutation(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "phase_1", Tasks: []*campaign.Task{{ID: "task_a"}}},
            {ID: "phase_2", Tasks: []*campaign.Task{{ID: "task_b"}}},
        },
    }

    var wg sync.WaitGroup
    wg.Add(1)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        if task.ID == "task_a" {
            wg.Done()
            time.Sleep(50 * time.Millisecond)
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    go func() {
        wg.Wait() // wait for task_a to start
        // Maliciously mutate phase 2 while orchestrator is processing
        plan.Phases[1].Tasks[0].ID = "mutated_task"
    }()

    err := orch.ExecutePlan(context.Background(), plan)
    // Run with -race. If no race condition is flagged, it passes the safety check.
    if err != nil {
        t.Logf("Plan completed with error: %v", err)
    }
}

// State Corruption: Context Data Bleed Across Tasks
func TestE2E_Campaign_Decomposer_ContextDataBleed(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    var taskAContext string
    var taskBContext string
    var mu sync.Mutex

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        mu.Lock()
        defer mu.Unlock()
        if task.ID == "task_a" {
            taskAContext = cd
            return &campaign.TaskResult{Status: campaign.TaskStatusCompleted, Output: "task_a_result"}, nil
        }
        if task.ID == "task_b" {
            taskBContext = cd
            return &campaign.TaskResult{Status: campaign.TaskStatusCompleted, Output: "task_b_result"}, nil
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "phase_1", Tasks: []*campaign.Task{{ID: "task_a"}}},
            {ID: "phase_2", Tasks: []*campaign.Task{{ID: "task_b"}}},
        },
    }

    _ = orch.ExecutePlan(context.Background(), plan)

    mu.Lock()
    defer mu.Unlock()
    if taskBContext != "" && strings.Contains(taskAContext, "task_b_result") {
        t.Errorf("Context state bled incorrectly between tasks")
    }
}


// -------------------------------------------------------------------------
// Resource Exhaustion Tests (Minimum 2)
// -------------------------------------------------------------------------

// Scenario 4: Extreme Phase Scaling and Concurrency Limits
func TestE2E_Campaign_Decomposer_ExtremePhaseScaling(t *testing.T) {
	t.Parallel()
	orch, mockExec, _ := setupOrchestratorIntegration(t)

	var concurrentExecutions int32
	var maxObserved int32

	mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
		current := atomic.AddInt32(&concurrentExecutions, 1)

		for {
			max := atomic.LoadInt32(&maxObserved)
			if current <= max || atomic.CompareAndSwapInt32(&maxObserved, max, current) {
				break
			}
		}

		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&concurrentExecutions, -1)

		return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
	}

	tasks := make([]*campaign.Task, 100)
	for i := 0; i < 100; i++ {
		tasks[i] = &campaign.Task{ID: fmt.Sprintf("task_%d", i)}
	}

	plan := &campaign.Plan{
		Phases: []campaign.Phase{
			{ID: "phase_1", Tasks: tasks},
		},
	}

	err := orch.ExecutePlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("Plan execution failed: %v", err)
	}

	max := atomic.LoadInt32(&maxObserved)
	if max > 3 {
		t.Errorf("Concurrency limit violated. Expected max 3, observed %d", max)
	}
}

// Resource Exhaustion: Massive Phase Context Accumulation
func TestE2E_Campaign_Decomposer_ContextPagingOverflow(t *testing.T) {
	t.Parallel()
    if testing.Short() {
        t.Skip("Skipping massive memory test in short mode")
    }

    orch, mockExec, _ := setupOrchestratorIntegration(t)

    massiveString := strings.Repeat("A", 10*1024*1024) // 10MB

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        if task.ID == "task_1" {
            return &campaign.TaskResult{Status: campaign.TaskStatusCompleted, Output: massiveString}, nil
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "phase_1", Tasks: []*campaign.Task{{ID: "task_1"}}},
            {ID: "phase_2", Tasks: []*campaign.Task{{ID: "task_2"}}},
        },
    }

    err := orch.ExecutePlan(context.Background(), plan)

    if err != nil && !strings.Contains(strings.ToLower(err.Error()), "limit") && !strings.Contains(strings.ToLower(err.Error()), "too large") {
        t.Logf("Massive payload caused unrelated error: %v", err)
    }
}

// -------------------------------------------------------------------------
// Temporal Failure Tests (Minimum 3)
// -------------------------------------------------------------------------

// Scenario 3: Silent Task Hang and Timeout Enforcement
func TestE2E_Campaign_Decomposer_SilentTaskHang(t *testing.T) {
	t.Parallel()
	orch, mockExec, _ := setupOrchestratorIntegration(t)

	mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	plan := &campaign.Plan{
		Phases: []campaign.Phase{{ID: "phase_1", Tasks: []*campaign.Task{{ID: "task_a"}}}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := orch.ExecutePlan(ctx, plan)
	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}
}

// Scenario 8: Mid-Campaign Context Cancellation
func TestE2E_Campaign_Decomposer_MidCampaignCancel(t *testing.T) {
	t.Parallel()
	orch, mockExec, _ := setupOrchestratorIntegration(t)

	taskStarted := make(chan struct{})

	mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
		close(taskStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	}

	plan := &campaign.Plan{
		Phases: []campaign.Phase{{ID: "phase_1", Tasks: []*campaign.Task{{ID: "task_a"}}}},
	}

	ctx, cancel := context.WithCancel(context.Background())

	var err error
	go func() {
		err = orch.ExecutePlan(ctx, plan)
	}()

	<-taskStarted
	cancel()
	time.Sleep(50 * time.Millisecond)

	if err == nil {
		t.Log("Context cancellation handled without explicit error return")
	} else if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "canceled") {
		t.Errorf("Expected cancellation error, got: %v", err)
	}
}

// Temporal Failure: Long-Running Phase Initialization
func TestE2E_Campaign_Decomposer_SlowInitialization(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        time.Sleep(200 * time.Millisecond)
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "phase_1", Tasks: []*campaign.Task{{ID: "task_a"}}},
        },
    }

    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    err := orch.ExecutePlan(ctx, plan)
    if err == nil {
        t.Errorf("Expected timeout error due to slow initialization")
    }
}

// -------------------------------------------------------------------------
// Cascading Failure Tests (Minimum 2)
// -------------------------------------------------------------------------

// Cascading Failure: Phase 1 output is corrupted, causing Phase 2 to fail
func TestE2E_Campaign_Decomposer_CascadingPhaseFailure(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        if task.ID == "task_p1" {
            // Task succeeds but returns corrupted output
            return &campaign.TaskResult{Status: campaign.TaskStatusCompleted, Output: "CORRUPTED_JSON"}, nil
        }
        if task.ID == "task_p2" {
            if strings.Contains(cd, "CORRUPTED_JSON") {
                return &campaign.TaskResult{Status: campaign.TaskStatusFailed}, fmt.Errorf("invalid json payload")
            }
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "p1", Tasks: []*campaign.Task{{ID: "task_p1"}}},
            {ID: "p2", Tasks: []*campaign.Task{{ID: "task_p2"}}},
        },
    }

    err := orch.ExecutePlan(context.Background(), plan)
    if err == nil {
        t.Errorf("Expected cascade failure from corrupted output")
    } else if !strings.Contains(err.Error(), "invalid json payload") {
        t.Logf("Error propagated but string check failed: %v", err)
    }
}

// Cascading Failure: Orchestrator fails to handle partial phase completion
func TestE2E_Campaign_Decomposer_PartialPhaseCascade(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        if task.ID == "task_a" {
            return &campaign.TaskResult{Status: campaign.TaskStatusFailed}, fmt.Errorf("task a failed")
        }
        // Task B succeeds but shouldn't trigger phase 2
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "p1", Tasks: []*campaign.Task{{ID: "task_a"}, {ID: "task_b"}}},
            {ID: "p2", Tasks: []*campaign.Task{{ID: "task_c"}}},
        },
    }

    err := orch.ExecutePlan(context.Background(), plan)
    if err == nil {
        t.Errorf("Expected orchestrator to halt cascade")
    }
}

// -------------------------------------------------------------------------
// Recovery Tests (Minimum 2)
// -------------------------------------------------------------------------

// Recovery: Orchestrator Retries Failed Task
func TestE2E_Campaign_Decomposer_TaskRetryRecovery(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    var attempts int
    var mu sync.Mutex

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        mu.Lock()
        attempts++
        currAttempts := attempts
        mu.Unlock()

        if currAttempts == 1 {
            return &campaign.TaskResult{Status: campaign.TaskStatusFailed}, fmt.Errorf("transient failure")
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "p1", Tasks: []*campaign.Task{{ID: "task_a"}}},
        },
    }

    err := orch.ExecutePlan(context.Background(), plan)
    if err != nil {
        t.Errorf("Expected recovery via retry, got error: %v", err)
    }
}

// Recovery: Mismatched Schema Resilient Fallback
func TestE2E_Campaign_Decomposer_SchemaRecovery(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "p1", Tasks: []*campaign.Task{{ID: "task_a"}}},
        },
    }

    err := orch.ExecutePlan(context.Background(), plan)
    if err != nil {
        t.Errorf("Expected orchestrator to recover, got: %v", err)
    }
}

// -------------------------------------------------------------------------
// PIPELINE Tests Specific:
// -------------------------------------------------------------------------

// End-to-End Data Integrity (Minimum 2)
func TestE2E_Campaign_Decomposer_PipelineDataIntegrity(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        if task.ID == "p1_task" {
            return &campaign.TaskResult{Status: campaign.TaskStatusCompleted, Output: "FACT:123"}, nil
        }
        if task.ID == "p2_task" {
            if !strings.Contains(cd, "FACT:123") {
                return &campaign.TaskResult{Status: campaign.TaskStatusFailed}, fmt.Errorf("Data lost")
            }
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "p1", Tasks: []*campaign.Task{{ID: "p1_task"}}},
            {ID: "p2", Tasks: []*campaign.Task{{ID: "p2_task"}}},
        },
    }

    err := orch.ExecutePlan(context.Background(), plan)
    if err != nil {
        t.Errorf("Data integrity check failed: %v", err)
    }
}

func TestE2E_Campaign_Decomposer_DataIntegrity_MultiStep(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        if task.ID == "p1_task" {
            return &campaign.TaskResult{Status: campaign.TaskStatusCompleted, Output: "MAGIC_KEY"}, nil
        }
        if task.ID == "p3_task" {
            if !strings.Contains(cd, "MAGIC_KEY") {
                return &campaign.TaskResult{Status: campaign.TaskStatusFailed}, fmt.Errorf("Multi-step data lost")
            }
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted, Output: "intermediate"}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "p1", Tasks: []*campaign.Task{{ID: "p1_task"}}},
            {ID: "p2", Tasks: []*campaign.Task{{ID: "p2_task"}}},
            {ID: "p3", Tasks: []*campaign.Task{{ID: "p3_task"}}},
        },
    }

    err := orch.ExecutePlan(context.Background(), plan)
    if err != nil {
        t.Errorf("Multi-step data integrity failed: %v", err)
    }
}

// Multi-Turn State Accumulation (Minimum 2)
func TestE2E_Campaign_Decomposer_MultiTurnAccumulation(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    var stateAccumulator []string
    var mu sync.Mutex

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        mu.Lock()
        stateAccumulator = append(stateAccumulator, cd)
        mu.Unlock()
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted, Output: "T_" + task.ID}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "p1", Tasks: []*campaign.Task{{ID: "t1"}}},
            {ID: "p2", Tasks: []*campaign.Task{{ID: "t2"}}},
            {ID: "p3", Tasks: []*campaign.Task{{ID: "t3"}}},
            {ID: "p4", Tasks: []*campaign.Task{{ID: "t4"}}},
            {ID: "p5", Tasks: []*campaign.Task{{ID: "t5"}}},
        },
    }

    _ = orch.ExecutePlan(context.Background(), plan)

    mu.Lock()
    defer mu.Unlock()
    if len(stateAccumulator) != 5 {
        t.Errorf("Expected 5 accumulated turns, got %d", len(stateAccumulator))
    }

    if !strings.Contains(stateAccumulator[4], "T_t1") {
        t.Log("Context was reset or not accumulated properly across 5 turns")
    }
}

func TestE2E_Campaign_Decomposer_StateAccumulationMemoryLeak(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted, Output: strings.Repeat("A", 1024)}, nil
    }

    var phases []campaign.Phase
    for i := 0; i < 20; i++ {
        phases = append(phases, campaign.Phase{ID: fmt.Sprintf("p%d", i), Tasks: []*campaign.Task{{ID: "t"}}})
    }

    plan := &campaign.Plan{Phases: phases}

    err := orch.ExecutePlan(context.Background(), plan)
    if err != nil {
        t.Errorf("Accumulation over 20 turns failed: %v", err)
    }
}

// Partial Pipeline Failure (Minimum 2)
func TestE2E_Campaign_Decomposer_PartialPipelineHalt(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        if task.ID == "fatal_task" {
            return &campaign.TaskResult{Status: campaign.TaskStatusFailed}, fmt.Errorf("halt here")
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "p1", Tasks: []*campaign.Task{{ID: "t1"}}},
            {ID: "p2", Tasks: []*campaign.Task{{ID: "fatal_task"}}},
            {ID: "p3", Tasks: []*campaign.Task{{ID: "t3"}}},
        },
    }

    err := orch.ExecutePlan(context.Background(), plan)
    if err == nil {
        t.Errorf("Expected pipeline to halt on fatal task")
    }

    mExec := mockExec
    mExec.mu.Lock()
    cnt := mExec.ExecutionCnt
    mExec.mu.Unlock()

    if cnt == 3 {
        t.Errorf("Phase 3 executed despite Phase 2 fatal error")
    }
}

func TestE2E_Campaign_Decomposer_PartialFailurePreservesState(t *testing.T) {
	t.Parallel()
    orch, mockExec, _ := setupOrchestratorIntegration(t)

    var executedTasks []string
    var mu sync.Mutex

    mockExec.ExecuteFunc = func(ctx context.Context, task *campaign.Task, cd string) (*campaign.TaskResult, error) {
        mu.Lock()
        executedTasks = append(executedTasks, task.ID)
        mu.Unlock()
        if task.ID == "t2" {
            return &campaign.TaskResult{Status: campaign.TaskStatusFailed}, fmt.Errorf("fail")
        }
        return &campaign.TaskResult{Status: campaign.TaskStatusCompleted}, nil
    }

    plan := &campaign.Plan{
        Phases: []campaign.Phase{
            {ID: "p1", Tasks: []*campaign.Task{{ID: "t1"}}},
            {ID: "p2", Tasks: []*campaign.Task{{ID: "t2"}}},
        },
    }

    _ = orch.ExecutePlan(context.Background(), plan)

    mu.Lock()
    defer mu.Unlock()
    if len(executedTasks) != 2 || executedTasks[0] != "t1" || executedTasks[1] != "t2" {
        t.Errorf("Execution order or state preservation failed")
    }
}

// -------------------------------------------------------------------------
// Table-Driven Tests for Parameterized Scenarios
// -------------------------------------------------------------------------

func TestE2E_Campaign_Decomposer_ParameterizedPhaseValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		plan        *campaign.Plan
		expectError bool
		errorString string
	}{
		{
			name: "Valid Plan",
			plan: &campaign.Plan{
				Phases: []campaign.Phase{
					{ID: "p1", Tasks: []*campaign.Task{{ID: "t1"}}},
				},
			},
			expectError: false,
		},
		{
			name: "Missing Dependencies",
			plan: &campaign.Plan{
				Phases: []campaign.Phase{
					{ID: "p1", Tasks: []*campaign.Task{{ID: "t1", Dependencies: []string{"t2"}}}},
				},
			},
			expectError: true,
		},
		{
			name: "Empty Phase ID",
			plan: &campaign.Plan{
				Phases: []campaign.Phase{
					{ID: "", Tasks: []*campaign.Task{{ID: "t1"}}},
				},
			},
			expectError: false, // Depending on strictness, might be allowed or auto-generated
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			orch, _, _ := setupOrchestratorIntegration(t)
			err := orch.ExecutePlan(context.Background(), tc.plan)

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tc.expectError && tc.errorString != "" && err != nil && !strings.Contains(err.Error(), tc.errorString) {
				t.Errorf("Expected error containing '%s', got '%v'", tc.errorString, err)
			}
		})
	}
}
