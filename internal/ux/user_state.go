package ux

import (
	"codenerd/internal/config"
)

// UserJourneyState represents a user's position in the onboarding journey.
type UserJourneyState string

const (
	// StateNew indicates a first-time user (no .nerd/ directory).
	StateNew UserJourneyState = "new"

	// StateOnboarding indicates the user is actively in the welcome wizard.
	StateOnboarding UserJourneyState = "onboarding"

	// StateLearning indicates the user is in their first 10-20 sessions.
	StateLearning UserJourneyState = "learning"

	// StateProductive indicates the user is comfortable with basics.
	StateProductive UserJourneyState = "productive"

	// StatePower indicates an advanced user who needs minimal guidance.
	StatePower UserJourneyState = "power"
)

// UserMetrics tracks interaction statistics for journey transitions.
type UserMetrics struct {
	SessionsCount         int     `json:"sessions_count"`
	CommandsExecuted      int     `json:"commands_executed"`
	ClarificationsNeeded  int     `json:"clarifications_needed"`
	HelpRequests          int     `json:"help_requests"`
	SuccessfulTasks       int     `json:"successful_tasks"`
	ErrorsEncountered     int     `json:"errors_encountered"`
	LastSession           string  `json:"last_session,omitempty"`
	ClarificationRate     float64 `json:"clarification_rate,omitempty"` // Computed
}

// ShouldTransition checks if user metrics warrant a state transition.
func (m *UserMetrics) ShouldTransition(currentState UserJourneyState) (UserJourneyState, bool) {
	switch currentState {
	case StateNew:
		// Transition to onboarding after first command
		if m.CommandsExecuted >= 1 {
			return StateOnboarding, true
		}

	case StateOnboarding:
		// Transition to learning after completing onboarding (handled elsewhere)
		// This is controlled by OnboardingState.SetupComplete

	case StateLearning:
		// Transition to productive after stable metrics
		if m.SessionsCount >= 15 && m.SuccessfulTasks >= 20 {
			if m.SuccessfulTasks > 0 {
				rate := float64(m.ClarificationsNeeded) / float64(m.SuccessfulTasks)
				if rate < 0.15 {
					return StateProductive, true
				}
			}
		}

	case StateProductive:
		// Transition to power after mastery indicators
		if m.SessionsCount >= 50 && m.CommandsExecuted >= 200 && m.HelpRequests < 5 {
			return StatePower, true
		}
	}

	return currentState, false
}

// GetDisclosureLevel returns the appropriate command disclosure level.
func GetDisclosureLevel(state UserJourneyState, guidanceLevel config.GuidanceLevel) DisclosureLevel {
	// User can override with guidance level
	if guidanceLevel == config.GuidanceNone {
		return DisclosureMinimal
	}

	switch state {
	case StatePower:
		return DisclosureMinimal
	case StateProductive:
		return DisclosureStandard
	case StateLearning:
		return DisclosureVerbose
	default:
		return DisclosureTutorial
	}
}

// DisclosureLevel controls how much information is shown.
type DisclosureLevel int

const (
	// DisclosureMinimal shows minimal info for power users.
	DisclosureMinimal DisclosureLevel = iota

	// DisclosureStandard shows standard info for productive users.
	DisclosureStandard

	// DisclosureVerbose shows verbose info for learning users.
	DisclosureVerbose

	// DisclosureTutorial shows maximum info for new/onboarding users.
	DisclosureTutorial
)

// String returns a human-readable name for the disclosure level.
func (d DisclosureLevel) String() string {
	switch d {
	case DisclosureMinimal:
		return "minimal"
	case DisclosureStandard:
		return "standard"
	case DisclosureVerbose:
		return "verbose"
	case DisclosureTutorial:
		return "tutorial"
	default:
		return "unknown"
	}
}
