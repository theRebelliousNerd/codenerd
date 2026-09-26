// Package core provides fact categorization for ephemeral vs persistent predicates.
//
// This file implements the separation of ephemeral facts (session-specific, volatile)
// from persistent facts (survive across sessions). This is critical for quiescent boot:
// the system should start clean without stale user_intent or pending_action facts
// from previous sessions.
package core

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
	"session_active":  true,
	"current_turn":    true,
	"turn_context":    true,
	"active_shard":    true,
	"shard_executing": true,
	"subagent_active": true,
	"subagent_task":   true,
	"subagent_result": true,

	// Transient reasoning state
	"hypothesis_active":        true,
	"verification_state":       true,
	"clarification_need":       true,
	"user_input_string":        true,
	"intent_unknown":           true,
	"intent_unmapped":          true,
	"no_action_reason":         true,
	"learning_candidate":       true,
	"learning_candidate_fact":  true,
	"learning_candidate_count": true,
	"clarification_question":   true,
	"clarification_option":     true,
	"ooda_timeout":             true,

	// Tool execution state
	"tool_invoked": true,
	"tool_result":  true,
	"tool_pending": true,

	// Dream mode ephemeral state
	"dream_hypothesis":  true,
	"dream_simulation":  true,
	"dream_exploration": true,

	// Campaign ephemeral state (phase tracking is persistent, but current execution is not)
	"campaign_active_phase": true,
	"campaign_turn":         true,

	// Activation state (spreading activation is recomputed each session)
	"activation_score": true,
	"context_priority": true,
	"selected_context": true,
}

// IsEphemeral returns true if the predicate should not be loaded from disk.
func IsEphemeral(predicate string) bool {
	return EphemeralPredicates[predicate]
}
