// Package core provides fact categorization for ephemeral vs persistent predicates.
//
// This file implements the separation of ephemeral facts (session-specific, volatile)
// from persistent facts (survive across sessions). This is critical for quiescent boot:
// the system should start clean without stale user_intent or pending_action facts
// from previous sessions.
package core

// FactCategory represents the persistence category of a predicate.
type FactCategory int

const (
	// FactCategoryPersistent facts survive across sessions and are loaded from disk.
	FactCategoryPersistent FactCategory = iota

	// FactCategoryEphemeral facts are session-specific and NOT loaded from disk.
	// They are created during the session and discarded on exit.
	FactCategoryEphemeral

	// FactCategoryDerived facts are computed by Mangle rules, not stored.
	FactCategoryDerived
)

// String returns the string representation of a FactCategory.
func (c FactCategory) String() string {
	switch c {
	case FactCategoryPersistent:
		return "persistent"
	case FactCategoryEphemeral:
		return "ephemeral"
	case FactCategoryDerived:
		return "derived"
	default:
		return "unknown"
	}
}

// EphemeralPredicates lists predicates that should NOT be loaded from disk.
// These are session-specific and must start fresh each session.
var EphemeralPredicates = map[string]bool{
	// User intent from current session
	"user_intent": true,

	// Action state (what's pending, what's next)
	"pending_action":  true,
	"next_action":     true,
	"action_blocked":  true,
	"action_executed": true,

	// Session-specific state
	"session_active":    true,
	"current_turn":      true,
	"turn_context":      true,
	"active_shard":      true,
	"shard_executing":   true,
	"subagent_active":   true,
	"subagent_task":     true,
	"subagent_result":   true,

	// Transient reasoning state
	"hypothesis_active":  true,
	"verification_state": true,
	"clarification_need": true,
	"intent_unknown":     true,
	"intent_unmapped":    true,
	"no_action_reason":   true,
	"learning_candidate": true,
	"learning_candidate_count": true,
	"clarification_question":   true,
	"clarification_option":     true,
	"ooda_timeout":             true,

	// Tool execution state
	"tool_invoked":   true,
	"tool_result":    true,
	"tool_pending":   true,

	// Dream mode ephemeral state
	"dream_hypothesis":  true,
	"dream_simulation":  true,
	"dream_exploration": true,

	// Campaign ephemeral state (phase tracking is persistent, but current execution is not)
	"campaign_active_phase": true,
	"campaign_turn":         true,

	// Activation state (spreading activation is recomputed each session)
	"activation_score":     true,
	"context_priority":     true,
	"selected_context":     true,
}

// DerivedPredicates lists predicates that are computed by Mangle rules.
// These should never be stored, only derived at runtime.
var DerivedPredicates = map[string]bool{
	// Safety derivations
	"permitted":       true,
	"blocked":         true,
	"safe_action":     true,
	"unsafe_action":   true,

	// Context derivations
	"context_atom":        true,
	"relevant_fact":       true,
	"activation_spread":   true,

	// Action routing
	"route_to_tool":      true,
	"route_to_shard":     true,
	"action_type":        true,

	// Test framework detection
	"test_framework":     true,
	"detected_framework": true,

	// Policy decisions
	"policy_allows":      true,
	"policy_denies":      true,
}

// IsEphemeral returns true if the predicate should not be loaded from disk.
func IsEphemeral(predicate string) bool {
	return EphemeralPredicates[predicate]
}

// IsDerived returns true if the predicate is computed by Mangle rules.
func IsDerived(predicate string) bool {
	return DerivedPredicates[predicate]
}

// IsPersistent returns true if the predicate should be loaded from disk.
func IsPersistent(predicate string) bool {
	return !IsEphemeral(predicate) && !IsDerived(predicate)
}

// GetCategory returns the category of a predicate.
func GetCategory(predicate string) FactCategory {
	if IsDerived(predicate) {
		return FactCategoryDerived
	}
	if IsEphemeral(predicate) {
		return FactCategoryEphemeral
	}
	return FactCategoryPersistent
}

// FilterPersistent filters a slice of predicates to only those that are persistent.
func FilterPersistent(predicates []string) []string {
	result := make([]string, 0, len(predicates))
	for _, p := range predicates {
		if IsPersistent(p) {
			result = append(result, p)
		}
	}
	return result
}

// FilterEphemeral filters a slice of predicates to only those that are ephemeral.
func FilterEphemeral(predicates []string) []string {
	result := make([]string, 0, len(predicates))
	for _, p := range predicates {
		if IsEphemeral(p) {
			result = append(result, p)
		}
	}
	return result
}

// ShouldLoadFromDisk returns true if facts for this predicate should be loaded
// from disk during kernel initialization.
func ShouldLoadFromDisk(predicate string) bool {
	return IsPersistent(predicate)
}

// ShouldPersistToDisk returns true if facts for this predicate should be
// saved to disk on kernel shutdown.
func ShouldPersistToDisk(predicate string) bool {
	return IsPersistent(predicate)
}
