package ux

import (
	"testing"

	"codenerd/internal/config"
)

func TestUserMetricsShouldTransition(t *testing.T) {
	metrics := UserMetrics{CommandsExecuted: 1}
	if state, ok := metrics.ShouldTransition(StateNew); !ok || state != StateOnboarding {
		t.Fatalf("expected transition to onboarding")
	}

	metrics = UserMetrics{
		SessionsCount:        15,
		SuccessfulTasks:      20,
		ClarificationsNeeded: 2,
	}
	if state, ok := metrics.ShouldTransition(StateLearning); !ok || state != StateProductive {
		t.Fatalf("expected transition to productive")
	}

	metrics = UserMetrics{
		SessionsCount:    50,
		CommandsExecuted: 200,
		HelpRequests:     1,
	}
	if state, ok := metrics.ShouldTransition(StateProductive); !ok || state != StatePower {
		t.Fatalf("expected transition to power")
	}
}

func TestGetDisclosureLevel(t *testing.T) {
	if level := GetDisclosureLevel(StatePower, config.GuidanceNormal); level != DisclosureMinimal {
		t.Fatalf("expected minimal disclosure for power users")
	}
	if level := GetDisclosureLevel(StateLearning, config.GuidanceNormal); level != DisclosureVerbose {
		t.Fatalf("expected verbose disclosure for learning users")
	}
	if level := GetDisclosureLevel(StateNew, config.GuidanceNone); level != DisclosureMinimal {
		t.Fatalf("expected minimal disclosure when guidance disabled")
	}
}

func TestDisclosureLevelString(t *testing.T) {
	if DisclosureLevel(99).String() != "unknown" {
		t.Fatalf("expected unknown disclosure level")
	}
}

func TestUserMetricsShouldTransition_NoTransition(t *testing.T) {
	cases := []struct {
		name    string
		metrics UserMetrics
		state   UserJourneyState
	}{
		{"new_without_commands_stays", UserMetrics{}, StateNew},
		{"onboarding_never_auto_transitions", UserMetrics{CommandsExecuted: 100, SessionsCount: 100, SuccessfulTasks: 100}, StateOnboarding},
		{"learning_needs_sessions_and_tasks", UserMetrics{SessionsCount: 3, SuccessfulTasks: 2}, StateLearning},
		{"learning_high_clarification_rate_stays", UserMetrics{SessionsCount: 20, SuccessfulTasks: 20, ClarificationsNeeded: 10}, StateLearning},
		{"productive_needs_volume_and_few_help_requests", UserMetrics{SessionsCount: 5, CommandsExecuted: 10, HelpRequests: 0}, StateProductive},
		{"productive_too_many_help_requests_stays", UserMetrics{SessionsCount: 60, CommandsExecuted: 300, HelpRequests: 9}, StateProductive},
		{"unknown_state_stays", UserMetrics{SessionsCount: 1000, CommandsExecuted: 1000, SuccessfulTasks: 1000}, UserJourneyState("archived")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if next, ok := tc.metrics.ShouldTransition(tc.state); ok || next != tc.state {
				t.Fatalf("ShouldTransition(%q) = (%q, %v), want (%q, false)", tc.state, next, ok, tc.state)
			}
		})
	}
}

func TestGetDisclosureLevel_AllStates(t *testing.T) {
	cases := []struct {
		state UserJourneyState
		want  DisclosureLevel
	}{
		{StatePower, DisclosureMinimal},
		{StateProductive, DisclosureStandard},
		{StateLearning, DisclosureVerbose},
		{StateNew, DisclosureTutorial},
		{StateOnboarding, DisclosureTutorial},
		{UserJourneyState("archived"), DisclosureTutorial},
		{UserJourneyState(""), DisclosureTutorial},
	}
	for _, tc := range cases {
		if got := GetDisclosureLevel(tc.state, config.GuidanceNormal); got != tc.want {
			t.Errorf("GetDisclosureLevel(%q, normal) = %v, want %v", tc.state, got, tc.want)
		}
		// GuidanceNone collapses every journey state to minimal.
		if got := GetDisclosureLevel(tc.state, config.GuidanceNone); got != DisclosureMinimal {
			t.Errorf("GetDisclosureLevel(%q, none) = %v, want minimal", tc.state, got)
		}
	}
}

func TestDisclosureLevelString_AllLevels(t *testing.T) {
	cases := map[DisclosureLevel]string{
		DisclosureMinimal:  "minimal",
		DisclosureStandard: "standard",
		DisclosureVerbose:  "verbose",
		DisclosureTutorial: "tutorial",
	}
	for level, want := range cases {
		if got := level.String(); got != want {
			t.Errorf("DisclosureLevel(%d).String() = %q, want %q", int(level), got, want)
		}
	}
}
