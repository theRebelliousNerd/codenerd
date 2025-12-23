// Package core implements Dream Plan execution for Dream State consultations.
// This file defines the data structures for actionable execution plans extracted
// from Dream State multi-agent consultations.
//
// Architecture:
//
//	Dream Consultation → Extract Subtasks → Store Plan → User Approval → Execute
//
// The DreamPlan bridges the gap between hypothetical exploration (Dream State)
// and actual execution. When a user says "do it" after a dream consultation,
// the stored plan is converted to Subtasks and executed using the existing
// multi-step task infrastructure.
package core

import (
	"time"
)

// DreamPlanStatus represents the lifecycle state of a dream plan.
type DreamPlanStatus string

const (
	// DreamPlanStatusPending indicates the plan is awaiting user decision.
	DreamPlanStatusPending DreamPlanStatus = "pending"
	// DreamPlanStatusApproved indicates the user said "do it".
	DreamPlanStatusApproved DreamPlanStatus = "approved"
	// DreamPlanStatusExecuting indicates the plan is currently running.
	DreamPlanStatusExecuting DreamPlanStatus = "executing"
	// DreamPlanStatusCompleted indicates all steps finished successfully.
	DreamPlanStatusCompleted DreamPlanStatus = "completed"
	// DreamPlanStatusFailed indicates execution stopped due to an error.
	DreamPlanStatusFailed DreamPlanStatus = "failed"
	// DreamPlanStatusCancelled indicates the user cancelled (Ctrl+X).
	DreamPlanStatusCancelled DreamPlanStatus = "cancelled"
)

// DreamSubtaskStatus represents the state of an individual subtask.
type DreamSubtaskStatus string

const (
	// SubtaskStatusPending indicates the step hasn't started yet.
	SubtaskStatusPending DreamSubtaskStatus = "pending"
	// SubtaskStatusRunning indicates the step is currently executing.
	SubtaskStatusRunning DreamSubtaskStatus = "running"
	// SubtaskStatusCompleted indicates the step finished successfully.
	SubtaskStatusCompleted DreamSubtaskStatus = "completed"
	// SubtaskStatusFailed indicates the step encountered an error.
	SubtaskStatusFailed DreamSubtaskStatus = "failed"
	// SubtaskStatusSkipped indicates the step was skipped (e.g., dependency failed).
	SubtaskStatusSkipped DreamSubtaskStatus = "skipped"
)

// DreamSubtask represents a single executable step extracted from shard consultations.
// Each subtask maps to a shard invocation when the plan is executed.
type DreamSubtask struct {
	// ID is a unique identifier for this subtask (e.g., "dream-0", "dream-1").
	ID string `json:"id"`

	// Order is the execution order (0-indexed).
	Order int `json:"order"`

	// ShardName is the name of the shard that proposed this step.
	ShardName string `json:"shard_name"`

	// ShardType is the type of shard to use for execution (e.g., "coder", "tester").
	ShardType string `json:"shard_type"`

	// Description is the human-readable step description.
	Description string `json:"description"`

	// Action is the normalized action verb (e.g., "create", "modify", "test").
	Action string `json:"action"`

	// Target is the file or symbol target for this step.
	Target string `json:"target"`

	// IsMutation indicates whether this step modifies files (affects breakpoint mode).
	IsMutation bool `json:"is_mutation"`

	// DependsOn lists indices of prerequisite steps that must complete first.
	DependsOn []int `json:"depends_on,omitempty"`

	// Status tracks the current state of this subtask.
	Status DreamSubtaskStatus `json:"status"`

	// Result holds the execution result (populated after completion).
	Result string `json:"result,omitempty"`

	// Error holds the error message if the step failed.
	Error string `json:"error,omitempty"`

	// StartedAt records when execution began.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt records when execution finished.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// DreamPlan represents an actionable execution plan extracted from Dream State consultations.
// This structure persists the full plan for potential execution after user approval.
type DreamPlan struct {
	// ID is a unique identifier for this plan.
	ID string `json:"id"`

	// Hypothetical is the original dream query (e.g., "what if I added caching").
	Hypothetical string `json:"hypothetical"`

	// Subtasks are the ordered execution steps.
	Subtasks []DreamSubtask `json:"subtasks"`

	// RiskLevel is the assessed risk (low/medium/high) based on shard concerns.
	RiskLevel string `json:"risk_level"`

	// Status tracks the overall plan state.
	Status DreamPlanStatus `json:"status"`

	// ConsultedShards lists which shards were consulted for this plan.
	ConsultedShards []string `json:"consulted_shards"`

	// RequiredTools lists existing tools needed for execution.
	RequiredTools []string `json:"required_tools,omitempty"`

	// MissingTools lists tools that would need to be generated (Ouroboros candidates).
	MissingTools []string `json:"missing_tools,omitempty"`

	// PendingQuestions lists clarifications that may be needed.
	PendingQuestions []string `json:"pending_questions,omitempty"`

	// CreatedAt records when the plan was extracted.
	CreatedAt time.Time `json:"created_at"`

	// ApprovedAt records when the user approved execution.
	ApprovedAt *time.Time `json:"approved_at,omitempty"`

	// CompletedAt records when execution finished.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// CurrentStep tracks which step is currently executing (0-indexed).
	CurrentStep int `json:"current_step"`

	// CompletedSteps counts how many steps have finished.
	CompletedSteps int `json:"completed_steps"`

	// FailedSteps counts how many steps have failed.
	FailedSteps int `json:"failed_steps"`
}

// NewDreamPlan creates a new pending dream plan.
func NewDreamPlan(id, hypothetical string) *DreamPlan {
	return &DreamPlan{
		ID:           id,
		Hypothetical: hypothetical,
		Subtasks:     make([]DreamSubtask, 0),
		Status:       DreamPlanStatusPending,
		CreatedAt:    time.Now(),
	}
}

// AddSubtask appends a subtask to the plan.
func (p *DreamPlan) AddSubtask(subtask DreamSubtask) {
	subtask.Order = len(p.Subtasks)
	subtask.Status = SubtaskStatusPending
	p.Subtasks = append(p.Subtasks, subtask)
}

// GetNextPendingSubtask returns the next subtask ready for execution.
// Returns nil if no subtasks are pending or dependencies are unmet.
func (p *DreamPlan) GetNextPendingSubtask() *DreamSubtask {
	for i := range p.Subtasks {
		if p.Subtasks[i].Status == SubtaskStatusPending {
			// Check dependencies
			if p.areDependenciesMet(i) {
				return &p.Subtasks[i]
			}
		}
	}
	return nil
}

// areDependenciesMet checks if all dependencies for a subtask are completed.
func (p *DreamPlan) areDependenciesMet(index int) bool {
	subtask := p.Subtasks[index]
	for _, depIdx := range subtask.DependsOn {
		if depIdx >= 0 && depIdx < len(p.Subtasks) {
			if p.Subtasks[depIdx].Status != SubtaskStatusCompleted {
				return false
			}
		}
	}
	return true
}

// MarkSubtaskRunning updates a subtask to running status.
func (p *DreamPlan) MarkSubtaskRunning(id string) {
	for i := range p.Subtasks {
		if p.Subtasks[i].ID == id {
			p.Subtasks[i].Status = SubtaskStatusRunning
			now := time.Now()
			p.Subtasks[i].StartedAt = &now
			p.CurrentStep = i
			break
		}
	}
}

// MarkSubtaskCompleted updates a subtask to completed status.
func (p *DreamPlan) MarkSubtaskCompleted(id, result string) {
	for i := range p.Subtasks {
		if p.Subtasks[i].ID == id {
			p.Subtasks[i].Status = SubtaskStatusCompleted
			p.Subtasks[i].Result = result
			now := time.Now()
			p.Subtasks[i].CompletedAt = &now
			p.CompletedSteps++
			break
		}
	}
}

// MarkSubtaskFailed updates a subtask to failed status.
func (p *DreamPlan) MarkSubtaskFailed(id, errMsg string) {
	for i := range p.Subtasks {
		if p.Subtasks[i].ID == id {
			p.Subtasks[i].Status = SubtaskStatusFailed
			p.Subtasks[i].Error = errMsg
			now := time.Now()
			p.Subtasks[i].CompletedAt = &now
			p.FailedSteps++
			break
		}
	}
}

// IsComplete returns true if all subtasks are done (completed or failed).
func (p *DreamPlan) IsComplete() bool {
	for _, s := range p.Subtasks {
		if s.Status == SubtaskStatusPending || s.Status == SubtaskStatusRunning {
			return false
		}
	}
	return true
}

// AllSucceeded returns true if all subtasks completed successfully.
func (p *DreamPlan) AllSucceeded() bool {
	for _, s := range p.Subtasks {
		if s.Status != SubtaskStatusCompleted {
			return false
		}
	}
	return len(p.Subtasks) > 0
}

// Progress returns the completion percentage (0.0 to 1.0).
func (p *DreamPlan) Progress() float64 {
	if len(p.Subtasks) == 0 {
		return 0.0
	}
	return float64(p.CompletedSteps+p.FailedSteps) / float64(len(p.Subtasks))
}
