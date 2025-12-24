package ux

import (
	"testing"

	"codenerd/internal/config"
)

func TestDefaultUserPreferences(t *testing.T) {
	prefs := DefaultUserPreferences()
	if prefs.Version != PreferencesVersion {
		t.Fatalf("unexpected preferences version: %s", prefs.Version)
	}
	if prefs.Guidance.Level != config.GuidanceNormal {
		t.Fatalf("unexpected guidance level: %s", prefs.Guidance.Level)
	}
	if !prefs.AgentSelection.AutoAcceptRecommended {
		t.Fatalf("expected auto accept recommended to be true")
	}
}

func TestPreferencesManagerLoadSave(t *testing.T) {
	workspace := t.TempDir()
	pm := NewPreferencesManager(workspace)
	if err := pm.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if err := pm.SetGuidanceLevel(config.GuidanceMinimal); err != nil {
		t.Fatalf("set guidance failed: %v", err)
	}
	if err := pm.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	pm2 := NewPreferencesManager(workspace)
	if err := pm2.Load(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if pm2.GetGuidanceLevel() != config.GuidanceMinimal {
		t.Fatalf("expected guidance level persisted")
	}
}

func TestIncrementMetric(t *testing.T) {
	pm := NewPreferencesManager(t.TempDir())
	if err := pm.IncrementMetric("sessions_count"); err != nil {
		t.Fatalf("increment metric failed: %v", err)
	}
	if pm.Get().Metrics.SessionsCount != 1 {
		t.Fatalf("expected sessions count to increment")
	}
	if err := pm.IncrementMetric("unknown"); err == nil {
		t.Fatalf("expected error for unknown metric")
	}
}

func TestRecordCorrection(t *testing.T) {
	pm := NewPreferencesManager(t.TempDir())
	if err := pm.RecordCorrection("parse1", "parse2"); err != nil {
		t.Fatalf("record correction failed: %v", err)
	}
	if err := pm.RecordCorrection("parse1", "parse2"); err != nil {
		t.Fatalf("record correction failed: %v", err)
	}
	corrections := pm.Get().LearnedPatterns.IntentCorrections
	if len(corrections) != 1 {
		t.Fatalf("expected single correction entry")
	}
	if corrections[0].ReinforcementCount != 2 {
		t.Fatalf("expected reinforcement count 2, got %d", corrections[0].ReinforcementCount)
	}
}

func TestOnboardingFlow(t *testing.T) {
	pm := NewPreferencesManager(t.TempDir())

	if err := pm.CompleteOnboardingStep("step1"); err != nil {
		t.Fatalf("complete step failed: %v", err)
	}
	if err := pm.CompleteOnboardingStep("step1"); err != nil {
		t.Fatalf("complete step failed: %v", err)
	}
	if len(pm.Get().UserJourney.CompletedSteps) != 1 {
		t.Fatalf("expected single completed step")
	}

	if err := pm.SkipOnboarding(); err != nil {
		t.Fatalf("skip onboarding failed: %v", err)
	}
	if !pm.IsOnboardingComplete() {
		t.Fatalf("expected onboarding complete after skip")
	}

	pm = NewPreferencesManager(t.TempDir())
	if err := pm.MarkOnboardingComplete(); err != nil {
		t.Fatalf("mark onboarding complete failed: %v", err)
	}
	if !pm.IsOnboardingComplete() {
		t.Fatalf("expected onboarding complete")
	}
}
