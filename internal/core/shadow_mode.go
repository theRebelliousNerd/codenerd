// Package core provides the Shadow Mode implementation for counterfactual reasoning.
// Shadow Mode allows the system to simulate "what if?" scenarios without committing changes.
package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codenerd/internal/types"
)

// ShadowMode represents a counterfactual simulation environment.
// It maintains a separate kernel instance to test hypothetical changes.
type ShadowMode struct {
	mu           sync.RWMutex
	parentKernel *RealKernel
	shadowKernel *RealKernel
	simulations  map[string]*Simulation
	activeSimID  string
	maxSimTime   time.Duration
}

// Simulation represents a single counterfactual scenario.
type Simulation struct {
	ID           string
	Description  string
	StartTime    time.Time
	EndTime      time.Time
	Status       SimulationStatus
	Actions      []SimulatedAction
	Effects      []SimulatedEffect
	Violations   []ProjectionViolation
	IsSafe       bool
	ErrorMessage string
}

// SimulationStatus represents the state of a simulation.
type SimulationStatus string

const (
	SimStatusPending   SimulationStatus = "pending"
	SimStatusRunning   SimulationStatus = "running"
	SimStatusCompleted SimulationStatus = "completed"
	SimStatusFailed    SimulationStatus = "failed"
)

// SimulatedAction represents an action being tested in shadow mode.
type SimulatedAction struct {
	ID          string
	Type        SimActionType
	Target      string
	Description string
	Timestamp   time.Time
}

// SimActionType categorizes the type of mutation being simulated.
type SimActionType string

const (
	ActionTypeFileWrite  SimActionType = "file_write"
	ActionTypeFileDelete SimActionType = "file_delete"
	ActionTypeExec       SimActionType = "exec"
	ActionTypeRefactor   SimActionType = "refactor"
	ActionTypeGitCommit  SimActionType = "git_commit"
)

// SimulatedEffect represents the projected effect of an action.
type SimulatedEffect struct {
	ActionID   string
	Predicate  string
	Args       []interface{}
	IsPositive bool // true for assertion, false for retraction
}

// ProjectionViolation represents a safety rule violation in the simulation.
type ProjectionViolation struct {
	ActionID      string
	ViolationType string
	Description   string
	Severity      string // "error", "warning", "info"
	Blocking      bool
}

// NewShadowMode creates a new Shadow Mode engine attached to a parent kernel.
func NewShadowMode(parent *RealKernel) *ShadowMode {
	return &ShadowMode{
		parentKernel: parent,
		simulations:  make(map[string]*Simulation),
		maxSimTime:   30 * time.Second,
	}
}

// StartSimulation begins a new counterfactual scenario.
func (sm *ShadowMode) StartSimulation(ctx context.Context, description string) (*Simulation, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Create unique simulation ID
	simID := fmt.Sprintf("sim_%d", time.Now().UnixNano())

	// Create shadow kernel as a copy of parent state
	shadowKernel, err := NewRealKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to create shadow kernel: %w", err)
	}
	shadowKernel.SetSchemas(sm.parentKernel.GetSchemas())
	shadowKernel.SetPolicy(sm.parentKernel.GetPolicy())

	// Copy parent facts to shadow
	parentFacts := sm.parentKernel.facts
	shadowKernel.LoadFacts(parentFacts)

	sm.shadowKernel = shadowKernel

	// Create simulation record
	sim := &Simulation{
		ID:          simID,
		Description: description,
		StartTime:   time.Now(),
		Status:      SimStatusRunning,
		Actions:     make([]SimulatedAction, 0),
		Effects:     make([]SimulatedEffect, 0),
		Violations:  make([]ProjectionViolation, 0),
		IsSafe:      true,
	}

	sm.simulations[simID] = sim
	sm.activeSimID = simID

	// Add shadow_state fact
	shadowStateFact := Fact{
		Predicate: "shadow_state",
		Args:      []interface{}{simID, simID, "/valid"},
	}
	shadowKernel.Assert(shadowStateFact)

	return sim, nil
}

// SimulateAction tests an action in the shadow environment.
func (sm *ShadowMode) SimulateAction(ctx context.Context, action SimulatedAction) (*SimulationResult, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.activeSimID == "" {
		return nil, fmt.Errorf("no active simulation")
	}

	sim, exists := sm.simulations[sm.activeSimID]
	if !exists {
		return nil, fmt.Errorf("simulation not found: %s", sm.activeSimID)
	}

	// Check timeout
	select {
	case <-ctx.Done():
		sim.Status = SimStatusFailed
		sim.ErrorMessage = "simulation timed out"
		return nil, ctx.Err()
	default:
	}

	action.Timestamp = time.Now()
	sim.Actions = append(sim.Actions, action)

	// Project the effects of the action
	effects := sm.projectEffects(action)
	sim.Effects = append(sim.Effects, effects...)

	// Assert simulated effects
	for _, effect := range effects {
		effectFact := Fact{
			Predicate: "simulated_effect",
			Args:      []interface{}{action.ID, effect.Predicate, fmt.Sprintf("%v", effect.Args)},
		}
		sm.shadowKernel.Assert(effectFact)
	}

	// Check for violations
	violations := sm.checkViolations(action.ID)
	sim.Violations = append(sim.Violations, violations...)

	// Determine if simulation is still safe
	for _, v := range violations {
		if v.Blocking {
			sim.IsSafe = false
		}
	}

	return &SimulationResult{
		ActionID:   action.ID,
		Effects:    effects,
		Violations: violations,
		IsSafe:     len(violations) == 0 || !hasBlockingViolation(violations),
	}, nil
}

// SimulationResult holds the outcome of a simulated action.
type SimulationResult struct {
	ActionID   string
	Effects    []SimulatedEffect
	Violations []ProjectionViolation
	IsSafe     bool
}

// projectEffects determines what facts would change if the action were executed.
func (sm *ShadowMode) projectEffects(action SimulatedAction) []SimulatedEffect {
	effects := make([]SimulatedEffect, 0)

	switch action.Type {
	case ActionTypeFileWrite:
		// Simulate file modification
		effects = append(effects, SimulatedEffect{
			ActionID:   action.ID,
			Predicate:  "modified",
			Args:       []interface{}{action.Target},
			IsPositive: true,
		})

		// Check if this triggers impacted dependencies
		deps, _ := sm.shadowKernel.Query("dependency_link")
		for _, dep := range deps {
			if len(dep.Args) >= 2 && types.ExtractString(dep.Args[1]) == action.Target {
				effects = append(effects, SimulatedEffect{
					ActionID:   action.ID,
					Predicate:  "impacted",
					Args:       []interface{}{dep.Args[0]},
					IsPositive: true,
				})
			}
		}

	case ActionTypeFileDelete:
		// Simulate file deletion
		effects = append(effects, SimulatedEffect{
			ActionID:   action.ID,
			Predicate:  "deleted_file",
			Args:       []interface{}{action.Target},
			IsPositive: true,
		})

	case ActionTypeRefactor:
		// Simulate refactoring
		effects = append(effects, SimulatedEffect{
			ActionID:   action.ID,
			Predicate:  "modified",
			Args:       []interface{}{action.Target},
			IsPositive: true,
		})
		effects = append(effects, SimulatedEffect{
			ActionID:   action.ID,
			Predicate:  "refactored",
			Args:       []interface{}{action.Target},
			IsPositive: true,
		})

	case ActionTypeExec:
		// Simulate command execution
		effects = append(effects, SimulatedEffect{
			ActionID:   action.ID,
			Predicate:  "exec_result",
			Args:       []interface{}{action.Target, "/pending"},
			IsPositive: true,
		})

	case ActionTypeGitCommit:
		// Simulate git commit
		effects = append(effects, SimulatedEffect{
			ActionID:   action.ID,
			Predicate:  "commit_pending",
			Args:       []interface{}{action.Target},
			IsPositive: true,
		})
	}

	return effects
}

// checkViolations queries the shadow kernel for safety violations.
func (sm *ShadowMode) checkViolations(actionID string) []ProjectionViolation {
	violations := make([]ProjectionViolation, 0)

	// Check for block_commit
	blockCommits, _ := sm.shadowKernel.Query("block_commit")
	for _, bc := range blockCommits {
		reason := "unknown"
		if len(bc.Args) > 0 {
			reason = types.ExtractString(bc.Args[0])
		}
		violations = append(violations, ProjectionViolation{
			ActionID:      actionID,
			ViolationType: "block_commit",
			Description:   fmt.Sprintf("Commit blocked: %s", reason),
			Severity:      "error",
			Blocking:      true,
		})
	}

	// Check for unsafe_to_refactor
	unsafeRefactors, _ := sm.shadowKernel.Query("unsafe_to_refactor")
	for _, ur := range unsafeRefactors {
		target := "unknown"
		if len(ur.Args) > 0 {
			target = types.ExtractString(ur.Args[0])
		}
		violations = append(violations, ProjectionViolation{
			ActionID:      actionID,
			ViolationType: "unsafe_refactor",
			Description:   fmt.Sprintf("Unsafe to refactor: %s lacks test coverage", target),
			Severity:      "warning",
			Blocking:      false,
		})
	}

	// Check for chesterton_fence_warning
	fenceWarnings, _ := sm.shadowKernel.Query("chesterton_fence_warning")
	for _, fw := range fenceWarnings {
		file := "unknown"
		reason := ""
		if len(fw.Args) > 0 {
			file = types.ExtractString(fw.Args[0])
		}
		if len(fw.Args) > 1 {
			reason = types.ExtractString(fw.Args[1])
		}
		violations = append(violations, ProjectionViolation{
			ActionID:      actionID,
			ViolationType: "chesterton_fence",
			Description:   fmt.Sprintf("Chesterton's Fence warning for %s: %s", file, reason),
			Severity:      "warning",
			Blocking:      false,
		})
	}

	// Check for projection_violation
	projViolations, _ := sm.shadowKernel.Query("projection_violation")
	for _, pv := range projViolations {
		violationType := "unknown"
		if len(pv.Args) > 1 {
			violationType = types.ExtractString(pv.Args[1])
		}
		violations = append(violations, ProjectionViolation{
			ActionID:      actionID,
			ViolationType: violationType,
			Description:   fmt.Sprintf("Projection violation: %s", violationType),
			Severity:      "error",
			Blocking:      true,
		})
	}

	return violations
}

// CommitSimulation finalizes a successful simulation.
func (sm *ShadowMode) CommitSimulation(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.activeSimID == "" {
		return fmt.Errorf("no active simulation")
	}

	sim, exists := sm.simulations[sm.activeSimID]
	if !exists {
		return fmt.Errorf("simulation not found")
	}

	if !sim.IsSafe {
		return fmt.Errorf("cannot commit unsafe simulation")
	}

	sim.Status = SimStatusCompleted
	sim.EndTime = time.Now()

	// Apply effects to the parent kernel
	for _, effect := range sim.Effects {
		if effect.IsPositive {
			fact := Fact{
				Predicate: effect.Predicate,
				Args:      effect.Args,
			}
			sm.parentKernel.Assert(fact)
		} else {
			sm.parentKernel.Retract(effect.Predicate)
		}
	}

	sm.activeSimID = ""
	sm.shadowKernel = nil

	return nil
}

// AbortSimulation cancels the current simulation without applying changes.
func (sm *ShadowMode) AbortSimulation(reason string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.activeSimID == "" {
		return
	}

	if sim, exists := sm.simulations[sm.activeSimID]; exists {
		sim.Status = SimStatusFailed
		sim.EndTime = time.Now()
		sim.ErrorMessage = reason
	}

	sm.activeSimID = ""
	sm.shadowKernel = nil
}

// GetSimulation retrieves a simulation by ID.
func (sm *ShadowMode) GetSimulation(simID string) (*Simulation, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sim, exists := sm.simulations[simID]
	return sim, exists
}

// GetActiveSimulation returns the currently active simulation.
func (sm *ShadowMode) GetActiveSimulation() (*Simulation, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.activeSimID == "" {
		return nil, false
	}
	sim, exists := sm.simulations[sm.activeSimID]
	return sim, exists
}

// IsShadowModeActive returns true if a simulation is currently running.
func (sm *ShadowMode) IsShadowModeActive() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.activeSimID != ""
}

// GetShadowKernel returns the shadow kernel for direct querying.
func (sm *ShadowMode) GetShadowKernel() *RealKernel {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.shadowKernel
}

// hasBlockingViolation checks if any violation is blocking.
func hasBlockingViolation(violations []ProjectionViolation) bool {
	for _, v := range violations {
		if v.Blocking {
			return true
		}
	}
	return false
}

// WhatIf runs a quick counterfactual query without full simulation.
// Returns the projected effects and violations for a hypothetical action.
func (sm *ShadowMode) WhatIf(ctx context.Context, action SimulatedAction) (*SimulationResult, error) {
	// Start a temporary simulation
	sim, err := sm.StartSimulation(ctx, fmt.Sprintf("What-If: %s", action.Description))
	if err != nil {
		return nil, err
	}

	// Run the simulation
	result, err := sm.SimulateAction(ctx, action)
	if err != nil {
		sm.AbortSimulation(err.Error())
		return nil, err
	}

	// Abort (don't commit) - this is just a query
	sm.AbortSimulation("what-if query completed")

	// Keep the simulation record
	sim.Status = SimStatusCompleted

	return result, nil
}

// ToFacts converts the active simulation state to Mangle facts.
func (sm *ShadowMode) ToFacts() []Fact {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	facts := make([]Fact, 0)

	if sm.activeSimID == "" {
		return facts
	}

	sim, exists := sm.simulations[sm.activeSimID]
	if !exists {
		return facts
	}

	// Add shadow_state fact
	isValid := "/valid"
	if !sim.IsSafe {
		isValid = "/invalid"
	}
	facts = append(facts, Fact{
		Predicate: "shadow_state",
		Args:      []interface{}{sim.ID, sim.ID, isValid},
	})

	// Add simulated_effect facts
	for _, effect := range sim.Effects {
		facts = append(facts, Fact{
			Predicate: "simulated_effect",
			Args:      []interface{}{effect.ActionID, effect.Predicate, fmt.Sprintf("%v", effect.Args)},
		})
	}

	return facts
}
