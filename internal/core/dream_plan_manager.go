// Package core implements Dream Plan lifecycle management.
// This file manages the storage, approval, and execution state of dream plans.
package core

import (
	"codenerd/internal/logging"
	"fmt"
	"sync"
	"time"
)

// DreamPlanManager handles the lifecycle of dream plans.
// It stores the current pending plan and tracks execution state.
type DreamPlanManager struct {
	mu          sync.RWMutex
	currentPlan *DreamPlan
	kernel      Kernel
	history     []*DreamPlan // Past plans for reference
	maxHistory  int          // Maximum history entries to keep
}

// NewDreamPlanManager creates a new plan manager.
func NewDreamPlanManager(kernel Kernel) *DreamPlanManager {
	logging.Dream("Creating DreamPlanManager")
	return &DreamPlanManager{
		kernel:     kernel,
		history:    make([]*DreamPlan, 0),
		maxHistory: 10,
	}
}

// StorePlan stores a new pending plan, replacing any existing one.
// The old plan (if any) is moved to history.
func (m *DreamPlanManager) StorePlan(plan *DreamPlan) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Archive current plan if it exists and has subtasks
	if m.currentPlan != nil && len(m.currentPlan.Subtasks) > 0 {
		m.archivePlan(m.currentPlan)
	}

	m.currentPlan = plan
	logging.Dream("Stored new dream plan: %s with %d subtasks", plan.ID, len(plan.Subtasks))

	// Assert plan fact to kernel
	if m.kernel != nil {
		fact := Fact{
			Predicate: "dream_plan",
			Args:      []interface{}{plan.ID, plan.Hypothetical, string(plan.Status), plan.CreatedAt.Unix()},
		}
		if err := m.kernel.Assert(fact); err != nil {
			logging.Get(logging.CategoryDream).Error("Failed to assert dream_plan fact: %v", err)
		}
	}
}

// GetCurrentPlan returns the current pending plan, or nil if none.
func (m *DreamPlanManager) GetCurrentPlan() *DreamPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentPlan
}

// HasPendingPlan returns true if there's a plan awaiting execution.
func (m *DreamPlanManager) HasPendingPlan() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentPlan != nil && m.currentPlan.Status == DreamPlanStatusPending
}

// ApprovePlan marks the current plan as approved for execution.
func (m *DreamPlanManager) ApprovePlan() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentPlan == nil {
		return fmt.Errorf("no plan to approve")
	}

	if m.currentPlan.Status != DreamPlanStatusPending {
		return fmt.Errorf("plan is not pending (status: %s)", m.currentPlan.Status)
	}

	now := time.Now()
	m.currentPlan.Status = DreamPlanStatusApproved
	m.currentPlan.ApprovedAt = &now

	logging.Dream("Approved dream plan: %s", m.currentPlan.ID)

	// Assert approval fact
	if m.kernel != nil {
		fact := Fact{
			Predicate: "dream_plan_approved",
			Args:      []interface{}{m.currentPlan.ID, now.Unix()},
		}
		if err := m.kernel.Assert(fact); err != nil {
			logging.Get(logging.CategoryDream).Error("Failed to assert dream_plan_approved fact: %v", err)
		}
	}

	return nil
}

// StartExecution marks the plan as executing.
func (m *DreamPlanManager) StartExecution() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentPlan == nil {
		return fmt.Errorf("no plan to execute")
	}

	if m.currentPlan.Status != DreamPlanStatusApproved {
		return fmt.Errorf("plan is not approved (status: %s)", m.currentPlan.Status)
	}

	m.currentPlan.Status = DreamPlanStatusExecuting
	logging.Dream("Started execution of dream plan: %s", m.currentPlan.ID)

	return nil
}

// CancelPlan cancels the current plan.
func (m *DreamPlanManager) CancelPlan() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentPlan == nil {
		return
	}

	m.currentPlan.Status = DreamPlanStatusCancelled
	now := time.Now()
	m.currentPlan.CompletedAt = &now

	// Mark remaining pending subtasks as skipped
	for i := range m.currentPlan.Subtasks {
		if m.currentPlan.Subtasks[i].Status == SubtaskStatusPending {
			m.currentPlan.Subtasks[i].Status = SubtaskStatusSkipped
		}
	}

	logging.Dream("Cancelled dream plan: %s", m.currentPlan.ID)

	// Archive and clear
	m.archivePlan(m.currentPlan)
	m.currentPlan = nil
}

// GetNextSubtask returns the next subtask ready for execution.
func (m *DreamPlanManager) GetNextSubtask() *DreamSubtask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentPlan == nil {
		return nil
	}

	return m.currentPlan.GetNextPendingSubtask()
}

// MarkSubtaskRunning updates a subtask to running status.
func (m *DreamPlanManager) MarkSubtaskRunning(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentPlan != nil {
		m.currentPlan.MarkSubtaskRunning(id)
	}
}

// MarkSubtaskComplete marks a subtask as completed and returns if plan is done.
func (m *DreamPlanManager) MarkSubtaskComplete(id, result string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentPlan == nil {
		return true
	}

	m.currentPlan.MarkSubtaskCompleted(id, result)

	// Assert completion fact
	if m.kernel != nil {
		// Find the subtask to get its order
		for _, s := range m.currentPlan.Subtasks {
			if s.ID == id {
				fact := Fact{
					Predicate: "dream_plan_step_completed",
					Args:      []interface{}{m.currentPlan.ID, s.Order, truncateResult(result), time.Now().Unix()},
				}
				if err := m.kernel.Assert(fact); err != nil {
					logging.Get(logging.CategoryDream).Error("Failed to assert step completion fact: %v", err)
				}
				break
			}
		}
	}

	// Check if plan is complete
	if m.currentPlan.IsComplete() {
		m.completePlan()
		return true
	}

	return false
}

// MarkSubtaskFailed marks a subtask as failed.
func (m *DreamPlanManager) MarkSubtaskFailed(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentPlan == nil {
		return
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	m.currentPlan.MarkSubtaskFailed(id, errMsg)
	logging.Dream("Dream plan subtask %s failed: %s", id, errMsg)
}

// GetProgress returns the current execution progress.
func (m *DreamPlanManager) GetProgress() (completed, total int, progress float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentPlan == nil {
		return 0, 0, 0.0
	}

	return m.currentPlan.CompletedSteps,
		len(m.currentPlan.Subtasks),
		m.currentPlan.Progress()
}

// GetStatus returns the current plan status.
func (m *DreamPlanManager) GetStatus() DreamPlanStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentPlan == nil {
		return ""
	}

	return m.currentPlan.Status
}

// GetPlanSummary returns a human-readable summary of the current plan.
func (m *DreamPlanManager) GetPlanSummary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentPlan == nil {
		return "No pending plan"
	}

	p := m.currentPlan
	mutations := 0
	for _, s := range p.Subtasks {
		if s.IsMutation {
			mutations++
		}
	}

	return fmt.Sprintf("%d steps (%d mutations), risk: %s",
		len(p.Subtasks), mutations, p.RiskLevel)
}

// completePlan finalizes the plan after all subtasks are done.
func (m *DreamPlanManager) completePlan() {
	if m.currentPlan == nil {
		return
	}

	now := time.Now()
	m.currentPlan.CompletedAt = &now

	if m.currentPlan.AllSucceeded() {
		m.currentPlan.Status = DreamPlanStatusCompleted
		logging.Dream("Dream plan completed successfully: %s", m.currentPlan.ID)
	} else {
		m.currentPlan.Status = DreamPlanStatusFailed
		logging.Dream("Dream plan completed with failures: %s (%d failed)",
			m.currentPlan.ID, m.currentPlan.FailedSteps)
	}

	// Archive and clear
	m.archivePlan(m.currentPlan)
	m.currentPlan = nil
}

// archivePlan moves a plan to history.
func (m *DreamPlanManager) archivePlan(plan *DreamPlan) {
	m.history = append(m.history, plan)

	// Trim history if needed
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

// GetHistory returns past plans.
func (m *DreamPlanManager) GetHistory() []*DreamPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DreamPlan, len(m.history))
	copy(result, m.history)
	return result
}

// ClearExpiredPlan clears the current plan if it's stale (older than timeout).
func (m *DreamPlanManager) ClearExpiredPlan(timeout time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentPlan == nil {
		return false
	}

	if time.Since(m.currentPlan.CreatedAt) > timeout {
		logging.Dream("Clearing expired dream plan: %s (age: %v)",
			m.currentPlan.ID, time.Since(m.currentPlan.CreatedAt))
		m.archivePlan(m.currentPlan)
		m.currentPlan = nil
		return true
	}

	return false
}

// truncateResult shortens result text for fact storage.
func truncateResult(result string) string {
	if len(result) > 100 {
		return result[:97] + "..."
	}
	return result
}
