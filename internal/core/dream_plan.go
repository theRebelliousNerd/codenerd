package core

import (
	"fmt"
	"time"
)

// DreamPlanStatus tracks the state of a dream plan.
type DreamPlanStatus string

const (
	DreamPlanStatusPending   DreamPlanStatus = "pending"
	DreamPlanStatusApproved  DreamPlanStatus = "approved"
	DreamPlanStatusExecuting DreamPlanStatus = "executing"
	DreamPlanStatusCompleted DreamPlanStatus = "completed"
	DreamPlanStatusFailed    DreamPlanStatus = "failed"
	DreamPlanStatusCancelled DreamPlanStatus = "cancelled"
)

// DreamPlan represents a full execution plan extracted from dream consultations.
// It maps the high-level intent into concrete, executable subtasks.
type DreamPlan struct {
	ID               string            `json:"id"`
	Hypothetical     string            `json:"hypothetical"` // Original user "dream" query
	RiskLevel        string            `json:"risk_level"`   // low, medium, high, critical
	Subtasks         []DreamSubtask    `json:"subtasks"`
	Status           DreamPlanStatus   `json:"status"`
	CreatedAt        time.Time         `json:"created_at"`
	ApprovedAt       *time.Time        `json:"approved_at,omitempty"`
	CompletedAt      *time.Time        `json:"completed_at,omitempty"`
	CompletedSteps   int               `json:"completed_steps"`
	FailedSteps      int               `json:"failed_steps"`
	ConsultedShards  []string          `json:"consulted_shards"`
	RequiredTools    []string          `json:"required_tools"`
	PendingQuestions []string          `json:"pending_questions"`
}

// DreamSubtask is a single step in a dream plan.
type DreamSubtask struct {
	ID          string        `json:"id"`
	Order       int           `json:"order"`
	ShardName   string        `json:"shard_name"` // e.g. "coder", "my-specialist"
	ShardType   string        `json:"shard_type"` // e.g. "coder", "tester"
	Task        string        `json:"task"`       // Actionable task description
	Action      string        `json:"action"`     // Primary verb (create, fix, test)
	Target      string        `json:"target"`     // Target file/symbol
	Description string        `json:"description"`
	IsMutation  bool          `json:"is_mutation"` // True if it modifies state
	Status      SubtaskStatus `json:"status"`
	Result      string        `json:"result,omitempty"`
	Error       string        `json:"error,omitempty"`
	DependsOn   []int         `json:"depends_on"` // Indices of dependencies
}

// SubtaskStatus tracks individual step status.
type SubtaskStatus string

const (
	SubtaskStatusPending   SubtaskStatus = "pending"
	SubtaskStatusRunning   SubtaskStatus = "running"
	SubtaskStatusCompleted SubtaskStatus = "completed"
	SubtaskStatusFailed    SubtaskStatus = "failed"
	SubtaskStatusSkipped   SubtaskStatus = "skipped"
)

// NewDreamPlan creates a initialized plan.
func NewDreamPlan(id, hypothetical string) *DreamPlan {
	if id == "" {
		id = fmt.Sprintf("plan-%d", time.Now().UnixNano())
	}
	return &DreamPlan{
		ID:               id,
		Hypothetical:     hypothetical,
		Status:           DreamPlanStatusPending,
		CreatedAt:        time.Now(),
		Subtasks:         make([]DreamSubtask, 0),
		ConsultedShards:  make([]string, 0),
		RequiredTools:    make([]string, 0),
		PendingQuestions: make([]string, 0),
	}
}

// AddSubtask adds a step to the plan.
func (p *DreamPlan) AddSubtask(subtask DreamSubtask) {
	p.Subtasks = append(p.Subtasks, subtask)
}

// GetNextPendingSubtask returns the next pending subtask, or nil if none.
func (p *DreamPlan) GetNextPendingSubtask() *DreamSubtask {
	for i := range p.Subtasks {
		if p.Subtasks[i].Status == SubtaskStatusPending {
			// Check dependencies
			allDepsMet := true
			for _, depIdx := range p.Subtasks[i].DependsOn {
				if depIdx >= 0 && depIdx < len(p.Subtasks) {
					if p.Subtasks[depIdx].Status != SubtaskStatusCompleted && p.Subtasks[depIdx].Status != SubtaskStatusSkipped {
						allDepsMet = false
						break
					}
				}
			}
			if allDepsMet {
				return &p.Subtasks[i]
			}
		}
	}
	return nil
}

// MarkSubtaskRunning marks a subtask as running.
func (p *DreamPlan) MarkSubtaskRunning(id string) {
	for i := range p.Subtasks {
		if p.Subtasks[i].ID == id {
			p.Subtasks[i].Status = SubtaskStatusRunning
			return
		}
	}
}

// MarkSubtaskCompleted marks a subtask as completed.
func (p *DreamPlan) MarkSubtaskCompleted(id, result string) {
	for i := range p.Subtasks {
		if p.Subtasks[i].ID == id {
			p.Subtasks[i].Status = SubtaskStatusCompleted
			p.Subtasks[i].Result = result
			p.CompletedSteps++
			return
		}
	}
}

// MarkSubtaskFailed marks a subtask as failed.
func (p *DreamPlan) MarkSubtaskFailed(id, err string) {
	for i := range p.Subtasks {
		if p.Subtasks[i].ID == id {
			p.Subtasks[i].Status = SubtaskStatusFailed
			p.Subtasks[i].Error = err
			p.FailedSteps++
			return
		}
	}
}

// IsComplete returns true if all steps are final.
func (p *DreamPlan) IsComplete() bool {
	for _, s := range p.Subtasks {
		if s.Status == SubtaskStatusPending || s.Status == SubtaskStatusRunning {
			return false
		}
	}
	return true
}

// AllSucceeded returns true if all steps completed successfully.
func (p *DreamPlan) AllSucceeded() bool {
	if len(p.Subtasks) == 0 {
		return false
	}
	for _, s := range p.Subtasks {
		if s.Status != SubtaskStatusCompleted && s.Status != SubtaskStatusSkipped {
			return false
		}
	}
	return true
}

// Progress returns progress fraction 0.0-1.0.
func (p *DreamPlan) Progress() float64 {
	if len(p.Subtasks) == 0 {
		return 0
	}
	final := 0
	for _, s := range p.Subtasks {
		if s.Status != SubtaskStatusPending && s.Status != SubtaskStatusRunning {
			final++
		}
	}
	return float64(final) / float64(len(p.Subtasks))
}
