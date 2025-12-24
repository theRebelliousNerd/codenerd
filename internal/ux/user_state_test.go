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
