package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// HealingType represents different self-healing strategies.
type HealingType string

const (
	// HealingRetry attempts the action again.
	HealingRetry HealingType = "/retry"
	// HealingRollback reverts to the previous state.
	HealingRollback HealingType = "/rollback"
	// HealingEscalate escalates to the user for manual intervention.
	HealingEscalate HealingType = "/escalate"
	// HealingAlternative tries an alternative approach.
	HealingAlternative HealingType = "/alternative_approach"
)

// HealingResult captures the outcome of a self-healing attempt.
type HealingResult struct {
	ActionID    string
	Strategy    HealingType
	Success     bool
	Attempts    int
	Error       string
	RecoveredAt time.Time
}

// SelfHealer handles automatic recovery from validation failures.
type SelfHealer struct {
	kernel     *RealKernel
	executor   ActionExecutor
	validators *ValidatorRegistry

	maxRetries   int
	retryBackoff time.Duration

	// Track healing attempts to prevent infinite loops
	healingAttempts map[string]int
	mu              sync.Mutex
}

// ActionExecutor is an interface for executing actions.
// This allows the SelfHealer to retry actions without import cycles.
type ActionExecutor interface {
	Execute(ctx context.Context, req ActionRequest) (ActionResult, error)
}

// SelfHealerConfig configures the self-healing behavior.
type SelfHealerConfig struct {
	MaxRetries   int
	RetryBackoff time.Duration
}

// DefaultSelfHealerConfig returns default configuration.
func DefaultSelfHealerConfig() SelfHealerConfig {
	return SelfHealerConfig{
		MaxRetries:   3,
		RetryBackoff: 500 * time.Millisecond,
	}
}

// NewSelfHealer creates a new self-healer.
func NewSelfHealer(kernel *RealKernel, validators *ValidatorRegistry, config SelfHealerConfig) *SelfHealer {
	return &SelfHealer{
		kernel:          kernel,
		validators:      validators,
		maxRetries:      config.MaxRetries,
		retryBackoff:    config.RetryBackoff,
		healingAttempts: make(map[string]int),
	}
}

// SetExecutor sets the action executor (set after construction to avoid cycles).
func (h *SelfHealer) SetExecutor(executor ActionExecutor) {
	h.executor = executor
}

// HandleValidationFailure handles a validation failure and attempts recovery.
func (h *SelfHealer) HandleValidationFailure(ctx context.Context, req ActionRequest, vr ValidationResult) (*HealingResult, error) {
	if h.executor == nil {
		return nil, fmt.Errorf("no executor set for self-healing")
	}

	// Determine the healing strategy
	healingType := h.determineHealingType(req.ActionID, vr)

	logging.VirtualStoreDebug("Self-healing triggered for action %s: strategy=%s, error=%s",
		req.ActionID, healingType, vr.Error)

	switch healingType {
	case HealingRetry:
		return h.retryAction(ctx, req, vr)
	case HealingRollback:
		return h.rollbackAction(ctx, req, vr)
	case HealingEscalate:
		return h.escalateToUser(ctx, req, vr)
	case HealingAlternative:
		return h.tryAlternative(ctx, req, vr)
	default:
		return h.escalateToUser(ctx, req, vr)
	}
}

// determineHealingType queries the Mangle kernel for the appropriate healing strategy.
func (h *SelfHealer) determineHealingType(actionID string, vr ValidationResult) HealingType {
	if h.kernel == nil {
		// Default to escalate if no kernel
		return HealingEscalate
	}

	// Query for specific healing strategy based on validation error
	// The Mangle rules in validation.mg derive needs_self_healing/2

	// Check if we've exceeded max retries
	h.mu.Lock()
	attempts := h.healingAttempts[actionID]
	h.mu.Unlock()

	if attempts >= h.maxRetries {
		return HealingEscalate
	}

	// Determine strategy based on error type
	switch {
	case vr.Error == "content hash mismatch":
		return HealingRetry
	case vr.Error == "cannot read back file":
		return HealingRetry
	case vr.Error == "syntax validation failed":
		return HealingRollback
	case vr.Error == "Go syntax error after CodeDOM edit":
		return HealingRollback
	case vr.Error == "target element no longer exists after edit":
		return HealingRollback
	case vr.Error == "file hash unchanged after edit - edit may not have been applied":
		return HealingRetry
	default:
		// For unknown errors, escalate
		return HealingEscalate
	}
}

// retryAction attempts the action again with exponential backoff.
func (h *SelfHealer) retryAction(ctx context.Context, req ActionRequest, vr ValidationResult) (*HealingResult, error) {
	h.mu.Lock()
	h.healingAttempts[req.ActionID]++
	attempt := h.healingAttempts[req.ActionID]
	h.mu.Unlock()

	if attempt > h.maxRetries {
		// Emit max retries reached fact
		h.emitMaxRetriesFact(req.ActionID)

		return &HealingResult{
			ActionID: req.ActionID,
			Strategy: HealingRetry,
			Success:  false,
			Attempts: attempt,
			Error:    fmt.Sprintf("max retries (%d) exceeded", h.maxRetries),
		}, nil
	}

	// Apply backoff
	backoff := h.retryBackoff * time.Duration(attempt)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(backoff):
	}

	logging.VirtualStoreDebug("Self-healing retry attempt %d/%d for action %s",
		attempt, h.maxRetries, req.ActionID)

	// Re-execute the action
	result, err := h.executor.Execute(ctx, req)
	if err != nil {
		return &HealingResult{
			ActionID: req.ActionID,
			Strategy: HealingRetry,
			Success:  false,
			Attempts: attempt,
			Error:    err.Error(),
		}, nil
	}

	// Re-validate
	if h.validators != nil {
		validations := h.validators.Validate(ctx, req, result)
		if ValidateAll(validations) {
			// Emit success
			h.emitValidationAttemptFact(req.ActionID, attempt, true)

			return &HealingResult{
				ActionID:    req.ActionID,
				Strategy:    HealingRetry,
				Success:     true,
				Attempts:    attempt,
				RecoveredAt: time.Now(),
			}, nil
		}
	}

	// Emit failure
	h.emitValidationAttemptFact(req.ActionID, attempt, false)

	// Still failing, try again
	return h.retryAction(ctx, req, vr)
}

// rollbackAction reverts to the previous state.
func (h *SelfHealer) rollbackAction(ctx context.Context, req ActionRequest, vr ValidationResult) (*HealingResult, error) {
	logging.VirtualStoreDebug("Self-healing rollback for action %s", req.ActionID)

	// Rollback is context-dependent - for file operations, we'd restore from backup
	// For now, we escalate since we don't have automatic backup restoration
	// In a full implementation, this would integrate with the transaction manager

	h.emitHealingAttemptFact(req.ActionID, HealingRollback, false, "rollback not implemented")

	return &HealingResult{
		ActionID: req.ActionID,
		Strategy: HealingRollback,
		Success:  false,
		Attempts: 1,
		Error:    "rollback requires transaction manager - escalating to user",
	}, nil
}

// escalateToUser escalates the issue to the user for manual intervention.
func (h *SelfHealer) escalateToUser(ctx context.Context, req ActionRequest, vr ValidationResult) (*HealingResult, error) {
	logging.VirtualStoreDebug("Self-healing escalating action %s to user: %s", req.ActionID, vr.Error)

	// Emit escalation fact for the kernel
	h.emitEscalationFact(req.ActionID, vr.Error)

	return &HealingResult{
		ActionID:    req.ActionID,
		Strategy:    HealingEscalate,
		Success:     true, // Escalation "succeeds" in that we've handled it
		Attempts:    0,
		RecoveredAt: time.Now(),
	}, nil
}

// tryAlternative attempts an alternative approach.
func (h *SelfHealer) tryAlternative(ctx context.Context, req ActionRequest, vr ValidationResult) (*HealingResult, error) {
	logging.VirtualStoreDebug("Self-healing trying alternative for action %s", req.ActionID)

	// Alternative approaches are action-type specific and complex
	// For now, escalate to user
	h.emitHealingAttemptFact(req.ActionID, HealingAlternative, false, "alternative not implemented")

	return &HealingResult{
		ActionID: req.ActionID,
		Strategy: HealingAlternative,
		Success:  false,
		Attempts: 1,
		Error:    "alternative approach not implemented - escalating to user",
	}, nil
}

// emitMaxRetriesFact emits a fact that max retries have been reached.
func (h *SelfHealer) emitMaxRetriesFact(actionID string) {
	if h.kernel == nil {
		return
	}

	fact := Fact{
		Predicate: "validation_max_retries_reached",
		Args:      []interface{}{actionID},
	}
	if err := h.kernel.Assert(fact); err != nil {
		logging.VirtualStoreDebug("Failed to emit max_retries_reached fact: %v", err)
	}
}

// boolToAtom converts a Go boolean to a Mangle atom string.
func boolToAtom(b bool) string {
	if b {
		return "/true"
	}
	return "/false"
}

// emitValidationAttemptFact emits a fact about a validation retry attempt.
func (h *SelfHealer) emitValidationAttemptFact(actionID string, attempt int, success bool) {
	if h.kernel == nil {
		return
	}

	fact := Fact{
		Predicate: "validation_attempt",
		Args:      []interface{}{actionID, attempt, boolToAtom(success), time.Now().Unix()},
	}
	if err := h.kernel.Assert(fact); err != nil {
		logging.VirtualStoreDebug("Failed to emit validation_attempt fact: %v", err)
	}
}

// emitHealingAttemptFact emits a fact about a healing attempt.
func (h *SelfHealer) emitHealingAttemptFact(actionID string, healingType HealingType, success bool, errorMsg string) {
	if h.kernel == nil {
		return
	}

	fact := Fact{
		Predicate: "healing_attempt",
		Args:      []interface{}{actionID, string(healingType), boolToAtom(success), errorMsg, time.Now().Unix()},
	}
	if err := h.kernel.Assert(fact); err != nil {
		logging.VirtualStoreDebug("Failed to emit healing_attempt fact: %v", err)
	}
}

// emitEscalationFact emits a fact that an action has been escalated.
func (h *SelfHealer) emitEscalationFact(actionID string, reason string) {
	if h.kernel == nil {
		return
	}

	fact := Fact{
		Predicate: "action_escalated",
		Args:      []interface{}{actionID, reason, time.Now().Unix()},
	}
	if err := h.kernel.Assert(fact); err != nil {
		logging.VirtualStoreDebug("Failed to emit action_escalated fact: %v", err)
	}
}

// ClearHealingAttempts resets the healing attempt counter for an action.
// Call this when an action succeeds on first try.
func (h *SelfHealer) ClearHealingAttempts(actionID string) {
	h.mu.Lock()
	delete(h.healingAttempts, actionID)
	h.mu.Unlock()
}

// GetHealingAttempts returns the number of healing attempts for an action.
func (h *SelfHealer) GetHealingAttempts(actionID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.healingAttempts[actionID]
}
