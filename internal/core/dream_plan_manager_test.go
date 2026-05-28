package core

import (
	"errors"
	"testing"
	"time"

	"github.com/google/mangle/analysis"
)

// mockDreamPlanKernel implements Kernel interface for testing
type mockDreamPlanKernel struct {
	assertedFacts []Fact
	assertErr     error
}

func (m *mockDreamPlanKernel) LoadFacts(facts []Fact) error           { return nil }
func (m *mockDreamPlanKernel) Query(predicate string) ([]Fact, error) { return nil, nil }
func (m *mockDreamPlanKernel) QueryAll() (map[string][]Fact, error)   { return nil, nil }
func (m *mockDreamPlanKernel) Assert(fact Fact) error {
	m.assertedFacts = append(m.assertedFacts, fact)
	return m.assertErr
}
func (m *mockDreamPlanKernel) AssertBatch(facts []Fact) error            { return nil }
func (m *mockDreamPlanKernel) Retract(predicate string) error            { return nil }
func (m *mockDreamPlanKernel) RetractFact(fact Fact) error               { return nil }
func (m *mockDreamPlanKernel) UpdateSystemFacts() error                  { return nil }
func (m *mockDreamPlanKernel) GetProgramInfo() *analysis.ProgramInfo     { return nil }
func (m *mockDreamPlanKernel) Reset()                                    {}
func (m *mockDreamPlanKernel) AppendPolicy(policy string)                {}
func (m *mockDreamPlanKernel) RetractExactFactsBatch(facts []Fact) error { return nil }
func (m *mockDreamPlanKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	return nil
}

func TestDreamPlanManager_New(t *testing.T) {
	k := &mockDreamPlanKernel{}
	mgr := NewDreamPlanManager(k)
	if mgr == nil {
		t.Fatal("NewDreamPlanManager returned nil")
	}
	if mgr.kernel != k {
		t.Error("Expected kernel to be set")
	}
}

func TestDreamPlanManager_StorePlan(t *testing.T) {
	k := &mockDreamPlanKernel{}
	mgr := NewDreamPlanManager(k)

	// Store first plan (nil kernel check is implicit if we pass it, but let's also test nil kernel)
	plan1 := NewDreamPlan("plan-1", "Query 1")
	plan1.AddSubtask(DreamSubtask{ID: "task-1", Status: SubtaskStatusPending})
	mgr.StorePlan(plan1)

	if mgr.GetCurrentPlan() != plan1 {
		t.Errorf("Expected current plan to be plan1, got %v", mgr.GetCurrentPlan())
	}
	if len(k.assertedFacts) != 2 {
		t.Errorf("Expected 2 asserted facts (dream_plan + dream_plan_subtask), got %d", len(k.assertedFacts))
	}
	if k.assertedFacts[0].Predicate != "dream_plan" {
		t.Errorf("Expected dream_plan predicate, got %s", k.assertedFacts[0].Predicate)
	}
	if len(k.assertedFacts) > 1 && k.assertedFacts[1].Predicate != "dream_plan_subtask" {
		t.Errorf("Expected dream_plan_subtask predicate, got %s", k.assertedFacts[1].Predicate)
	}

	// Store second plan - triggers archive of plan1
	plan2 := NewDreamPlan("plan-2", "Query 2")
	mgr.StorePlan(plan2)

	history := mgr.GetHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 plan in history, got %d", len(history))
	}
	if history[0] != plan1 {
		t.Errorf("Expected archived plan to be plan1")
	}

	// Test kernel assert error (should not panic)
	k.assertErr = errors.New("assert error")
	plan3 := NewDreamPlan("plan-3", "Query 3")
	mgr.StorePlan(plan3) // should not panic

	// Test nil kernel (should not panic)
	mgrNil := NewDreamPlanManager(nil)
	mgrNil.StorePlan(plan3) // should not panic
}

func TestDreamPlanManager_HasPendingPlan(t *testing.T) {
	mgr := NewDreamPlanManager(nil)
	if mgr.HasPendingPlan() {
		t.Error("Expected false when currentPlan is nil")
	}

	plan := NewDreamPlan("plan-1", "Query 1")
	mgr.StorePlan(plan)
	if !mgr.HasPendingPlan() {
		t.Error("Expected true when plan status is pending")
	}

	plan.Status = DreamPlanStatusApproved
	if mgr.HasPendingPlan() {
		t.Error("Expected false when plan status is approved")
	}
}

func TestDreamPlanManager_ApprovePlan(t *testing.T) {
	k := &mockDreamPlanKernel{}
	mgr := NewDreamPlanManager(k)

	// No plan
	err := mgr.ApprovePlan()
	if err == nil {
		t.Error("Expected error when approving nil plan")
	}

	// Plan not pending
	plan := NewDreamPlan("plan-1", "Query 1")
	plan.Status = DreamPlanStatusApproved
	mgr.StorePlan(plan)
	err = mgr.ApprovePlan()
	if err == nil {
		t.Error("Expected error when plan is already approved")
	}

	// Successful approval
	plan.Status = DreamPlanStatusPending
	k.assertedFacts = nil
	err = mgr.ApprovePlan()
	if err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}
	if plan.Status != DreamPlanStatusApproved {
		t.Errorf("Expected approved status, got %s", plan.Status)
	}
	if plan.ApprovedAt == nil {
		t.Error("Expected ApprovedAt to be set")
	}
	if len(k.assertedFacts) != 1 || k.assertedFacts[0].Predicate != "dream_plan_approved" {
		t.Errorf("Expected dream_plan_approved fact, got %v", k.assertedFacts)
	}

	// Verify error on Assert does not panic
	plan.Status = DreamPlanStatusPending
	k.assertErr = errors.New("assert error")
	err = mgr.ApprovePlan()
	if err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}
}

func TestDreamPlanManager_StartExecution(t *testing.T) {
	mgr := NewDreamPlanManager(nil)

	// No plan
	err := mgr.StartExecution()
	if err == nil {
		t.Error("Expected error when executing nil plan")
	}

	// Plan not approved
	plan := NewDreamPlan("plan-1", "Query 1")
	mgr.StorePlan(plan) // status is pending
	err = mgr.StartExecution()
	if err == nil {
		t.Error("Expected error when plan is pending")
	}

	// Success
	plan.Status = DreamPlanStatusApproved
	err = mgr.StartExecution()
	if err != nil {
		t.Fatalf("StartExecution failed: %v", err)
	}
	if plan.Status != DreamPlanStatusExecuting {
		t.Errorf("Expected executing status, got %s", plan.Status)
	}
}

func TestDreamPlanManager_CancelPlan(t *testing.T) {
	mgr := NewDreamPlanManager(nil)

	// Cancel on nil should not panic/fail
	mgr.CancelPlan()

	plan := NewDreamPlan("plan-1", "Query 1")
	plan.AddSubtask(DreamSubtask{ID: "task-1", Status: SubtaskStatusPending})
	plan.AddSubtask(DreamSubtask{ID: "task-2", Status: SubtaskStatusRunning})
	mgr.StorePlan(plan)

	mgr.CancelPlan()

	if plan.Status != DreamPlanStatusCancelled {
		t.Errorf("Expected cancelled status, got %s", plan.Status)
	}
	if plan.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}
	if plan.Subtasks[0].Status != SubtaskStatusSkipped {
		t.Errorf("Expected pending task to be skipped, got %s", plan.Subtasks[0].Status)
	}
	if plan.Subtasks[1].Status != SubtaskStatusRunning {
		t.Errorf("Expected running task to remain running, got %s", plan.Subtasks[1].Status)
	}
	if mgr.GetCurrentPlan() != nil {
		t.Error("Expected currentPlan to be nil after cancel")
	}
	if len(mgr.GetHistory()) != 1 {
		t.Errorf("Expected 1 plan in history, got %d", len(mgr.GetHistory()))
	}
}

func TestDreamPlanManager_GetNextSubtask(t *testing.T) {
	mgr := NewDreamPlanManager(nil)
	if mgr.GetNextSubtask() != nil {
		t.Error("Expected nil when no plan")
	}

	plan := NewDreamPlan("plan-1", "Query 1")
	plan.AddSubtask(DreamSubtask{ID: "task-1", Status: SubtaskStatusPending})
	mgr.StorePlan(plan)

	subtask := mgr.GetNextSubtask()
	if subtask == nil || subtask.ID != "task-1" {
		t.Errorf("Expected task-1, got %v", subtask)
	}
}

func TestDreamPlanManager_MarkSubtaskRunning(t *testing.T) {
	mgr := NewDreamPlanManager(nil)
	mgr.MarkSubtaskRunning("task-1") // should not panic

	plan := NewDreamPlan("plan-1", "Query 1")
	plan.AddSubtask(DreamSubtask{ID: "task-1", Status: SubtaskStatusPending})
	mgr.StorePlan(plan)

	mgr.MarkSubtaskRunning("task-1")
	if plan.Subtasks[0].Status != SubtaskStatusRunning {
		t.Errorf("Expected running, got %s", plan.Subtasks[0].Status)
	}
}

func TestDreamPlanManager_MarkSubtaskComplete(t *testing.T) {
	k := &mockDreamPlanKernel{}
	mgr := NewDreamPlanManager(k)

	// Nil plan
	if !mgr.MarkSubtaskComplete("task-1", "done") {
		t.Error("Expected true when no plan")
	}

	// Standard step completion (not yet fully complete)
	plan := NewDreamPlan("plan-1", "Query 1")
	plan.AddSubtask(DreamSubtask{ID: "task-1", Status: SubtaskStatusRunning, Order: 1})
	plan.AddSubtask(DreamSubtask{ID: "task-2", Status: SubtaskStatusPending, Order: 2})
	mgr.StorePlan(plan)

	k.assertedFacts = nil
	done := mgr.MarkSubtaskComplete("task-1", "result-1")
	if done {
		t.Error("Expected false since task-2 is still pending")
	}

	// Verify asserted fact
	if len(k.assertedFacts) != 1 || k.assertedFacts[0].Predicate != "dream_plan_step_completed" {
		t.Errorf("Expected dream_plan_step_completed fact, got %v", k.assertedFacts)
	}
	if k.assertedFacts[0].Args[2] != "result-1" {
		t.Errorf("Expected result-1, got %v", k.assertedFacts[0].Args[2])
	}

	// Test truncation of result
	longResult := ""
	for range 200 {
		longResult += "a"
	}
	plan.Subtasks[1].Status = SubtaskStatusRunning
	k.assertedFacts = nil
	done = mgr.MarkSubtaskComplete("task-2", longResult)
	if !done {
		t.Error("Expected true since all tasks are completed")
	}
	if plan.Status != DreamPlanStatusCompleted {
		t.Errorf("Expected plan status completed, got %s", plan.Status)
	}

	// Verify truncated result in fact
	expectedTrunc := longResult[:97] + "..."
	if k.assertedFacts[0].Args[2] != expectedTrunc {
		t.Errorf("Expected truncated result, got %v", k.assertedFacts[0].Args[2])
	}

	// Test kernel assert error (should not panic)
	plan2 := NewDreamPlan("plan-2", "Query 2")
	plan2.AddSubtask(DreamSubtask{ID: "task-1", Status: SubtaskStatusRunning, Order: 1})
	mgr.StorePlan(plan2)
	k.assertErr = errors.New("assert error")
	mgr.MarkSubtaskComplete("task-1", "done") // should not panic

	// Test plan complete with failure
	plan3 := NewDreamPlan("plan-3", "Query 3")
	plan3.AddSubtask(DreamSubtask{ID: "task-1", Status: SubtaskStatusFailed, Order: 1})
	mgr.StorePlan(plan3)
	mgr.completePlan()
	if plan3.Status != DreamPlanStatusFailed {
		t.Errorf("Expected failed plan status, got %s", plan3.Status)
	}
}

func TestDreamPlanManager_MarkSubtaskFailed(t *testing.T) {
	mgr := NewDreamPlanManager(nil)
	mgr.MarkSubtaskFailed("task-1", nil) // should not panic

	plan := NewDreamPlan("plan-1", "Query 1")
	plan.AddSubtask(DreamSubtask{ID: "task-1", Status: SubtaskStatusRunning})
	mgr.StorePlan(plan)

	// Failed with nil error
	mgr.MarkSubtaskFailed("task-1", nil)
	if plan.Subtasks[0].Status != SubtaskStatusFailed || plan.Subtasks[0].Error != "" {
		t.Errorf("Expected failed with empty error, got status=%s, err=%q", plan.Subtasks[0].Status, plan.Subtasks[0].Error)
	}

	// Failed with real error
	plan.Subtasks[0].Status = SubtaskStatusRunning
	mgr.MarkSubtaskFailed("task-1", errors.New("crash"))
	if plan.Subtasks[0].Status != SubtaskStatusFailed || plan.Subtasks[0].Error != "crash" {
		t.Errorf("Expected failed with crash error, got status=%s, err=%q", plan.Subtasks[0].Status, plan.Subtasks[0].Error)
	}
}

func TestDreamPlanManager_GetProgress(t *testing.T) {
	mgr := NewDreamPlanManager(nil)
	comp, tot, prog := mgr.GetProgress()
	if comp != 0 || tot != 0 || prog != 0.0 {
		t.Errorf("Expected 0s, got %d, %d, %f", comp, tot, prog)
	}

	plan := NewDreamPlan("plan-1", "Query 1")
	plan.AddSubtask(DreamSubtask{ID: "task-1", Status: SubtaskStatusCompleted})
	plan.AddSubtask(DreamSubtask{ID: "task-2", Status: SubtaskStatusPending})
	plan.CompletedSteps = 1
	mgr.StorePlan(plan)

	comp, tot, prog = mgr.GetProgress()
	if comp != 1 || tot != 2 || prog != 0.5 {
		t.Errorf("Expected 1, 2, 0.5, got %d, %d, %f", comp, tot, prog)
	}
}

func TestDreamPlanManager_GetStatus(t *testing.T) {
	mgr := NewDreamPlanManager(nil)
	if mgr.GetStatus() != "" {
		t.Errorf("Expected empty status, got %s", mgr.GetStatus())
	}

	plan := NewDreamPlan("plan-1", "Query 1")
	mgr.StorePlan(plan)
	if mgr.GetStatus() != DreamPlanStatusPending {
		t.Errorf("Expected pending status, got %s", mgr.GetStatus())
	}
}

func TestDreamPlanManager_GetPlanSummary(t *testing.T) {
	mgr := NewDreamPlanManager(nil)
	if mgr.GetPlanSummary() != "No pending plan" {
		t.Errorf("Expected 'No pending plan', got %q", mgr.GetPlanSummary())
	}

	plan := NewDreamPlan("plan-1", "Query 1")
	plan.RiskLevel = "high"
	plan.AddSubtask(DreamSubtask{ID: "task-1", IsMutation: true})
	plan.AddSubtask(DreamSubtask{ID: "task-2", IsMutation: false})
	mgr.StorePlan(plan)

	summary := mgr.GetPlanSummary()
	expected := "2 steps (1 mutations), risk: high"
	if summary != expected {
		t.Errorf("Expected %q, got %q", expected, summary)
	}
}

func TestDreamPlanManager_ClearExpiredPlan(t *testing.T) {
	mgr := NewDreamPlanManager(nil)
	if mgr.ClearExpiredPlan(time.Second) {
		t.Error("Expected false when no current plan")
	}

	plan := NewDreamPlan("plan-1", "Query 1")
	plan.CreatedAt = time.Now().Add(-10 * time.Minute)
	mgr.StorePlan(plan)

	// Not expired
	if mgr.ClearExpiredPlan(20 * time.Minute) {
		t.Error("Expected false when plan is not older than timeout")
	}
	if mgr.GetCurrentPlan() != plan {
		t.Error("Plan should not be cleared")
	}

	// Expired
	if !mgr.ClearExpiredPlan(5 * time.Minute) {
		t.Error("Expected true when plan is older than timeout")
	}
	if mgr.GetCurrentPlan() != nil {
		t.Error("Plan should be cleared")
	}
	if len(mgr.GetHistory()) != 1 {
		t.Errorf("Expected 1 plan in history, got %d", len(mgr.GetHistory()))
	}
}

func TestDreamPlanManager_HistoryTrimming(t *testing.T) {
	mgr := NewDreamPlanManager(nil)
	mgr.maxHistory = 2

	plan1 := NewDreamPlan("plan-1", "Query 1")
	plan1.AddSubtask(DreamSubtask{ID: "task-1"})
	mgr.StorePlan(plan1)

	plan2 := NewDreamPlan("plan-2", "Query 2")
	plan2.AddSubtask(DreamSubtask{ID: "task-2"})
	mgr.StorePlan(plan2) // archives plan1

	plan3 := NewDreamPlan("plan-3", "Query 3")
	plan3.AddSubtask(DreamSubtask{ID: "task-3"})
	mgr.StorePlan(plan3) // archives plan2

	plan4 := NewDreamPlan("plan-4", "Query 4")
	plan4.AddSubtask(DreamSubtask{ID: "task-4"})
	mgr.StorePlan(plan4) // archives plan3

	history := mgr.GetHistory()
	if len(history) != 2 {
		t.Fatalf("Expected history length 2, got %d", len(history))
	}
	// history should contain plan2 and plan3 (plan1 trimmed)
	if history[0].ID != "plan-2" || history[1].ID != "plan-3" {
		t.Errorf("Expected history to contain plan-2 and plan-3, got %s and %s", history[0].ID, history[1].ID)
	}
}
