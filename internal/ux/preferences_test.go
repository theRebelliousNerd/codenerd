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

func TestIncrementMetric_AllMetrics(t *testing.T) {
	pm := NewPreferencesManager(t.TempDir())
	for _, metric := range []string{"sessions_count", "commands_executed", "clarifications_needed", "help_requests", "successful_tasks", "errors_encountered"} {
		if err := pm.IncrementMetric(metric); err != nil {
			t.Fatalf("IncrementMetric(%q): %v", metric, err)
		}
	}
	got := pm.Get().Metrics
	if got.SessionsCount != 1 || got.CommandsExecuted != 1 || got.ClarificationsNeeded != 1 || got.HelpRequests != 1 || got.SuccessfulTasks != 1 || got.ErrorsEncountered != 1 {
		t.Fatalf("not every metric incremented exactly once: %+v", got)
	}
}

func TestRecordMetric_SavesAndRejectsUnknown(t *testing.T) {
	ws := t.TempDir()
	pm := NewPreferencesManager(ws)
	if err := pm.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := pm.RecordMetric("commands_executed"); err != nil {
		t.Fatalf("RecordMetric: %v", err)
	}
	reloaded := NewPreferencesManager(ws)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Get().Metrics.CommandsExecuted != 1 {
		t.Fatalf("RecordMetric did not persist commands_executed")
	}
	if err := pm.RecordMetric("bogus_metric"); err == nil {
		t.Fatalf("RecordMetric with unknown metric should fail")
	}
}

func TestLoad_WhenFileIsCorrupt_ShouldFail(t *testing.T) {
	ws := t.TempDir()
	writePrefsFile(t, ws, `{"truncated": `)
	pm := NewPreferencesManager(ws)
	if err := pm.Load(); err == nil {
		t.Fatalf("Load of corrupt preferences.json should fail")
	}
}

func TestGetJourneyState_WhenEmpty_ShouldDefaultToNew(t *testing.T) {
	pm := NewPreferencesManager(t.TempDir())
	if got := pm.GetJourneyState(); got != StateNew {
		t.Fatalf("fresh manager journey state = %q, want new", got)
	}
	// A persisted document without a journey state also reads as new.
	ws := t.TempDir()
	writePrefsFile(t, ws, `{"version": "2.0"}`)
	loaded := NewPreferencesManager(ws)
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.GetJourneyState(); got != StateNew {
		t.Fatalf("empty journey state = %q, want new", got)
	}
}

func TestGetGuidanceLevel_WhenEmpty_ShouldDefaultToNormal(t *testing.T) {
	ws := t.TempDir()
	writePrefsFile(t, ws, `{"version": "2.0"}`)
	pm := NewPreferencesManager(ws)
	if err := pm.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := pm.GetGuidanceLevel(); got != config.GuidanceNormal {
		t.Fatalf("empty guidance level = %q, want normal", got)
	}
}

func TestIsOnboardingComplete_WhenFresh_ShouldBeFalse(t *testing.T) {
	pm := NewPreferencesManager(t.TempDir())
	if err := pm.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if pm.IsOnboardingComplete() {
		t.Fatalf("fresh preferences should not report onboarding complete")
	}
}

func TestCheckJourneyTransition_WhenNoTransition_ShouldReportFalse(t *testing.T) {
	pm := NewPreferencesManager(t.TempDir())
	if err := pm.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	state, transitioned, err := pm.CheckJourneyTransition()
	if err != nil {
		t.Fatalf("CheckJourneyTransition: %v", err)
	}
	if transitioned {
		t.Fatalf("fresh user should not transition, got %q", state)
	}
	if state != StateNew {
		t.Fatalf("fresh user state = %q, want new", state)
	}
}

func TestGet_WhenNilPreferences_ShouldReturnDefaults(t *testing.T) {
	pm := NewPreferencesManager(t.TempDir())
	got := pm.Get()
	if got == nil || got.Version != PreferencesVersion || got.UserJourney.State != StateNew {
		t.Fatalf("Get on unloaded manager should return new-user defaults, got %+v", got)
	}
	got.Metrics.SessionsCount = 999
	if again := pm.Get(); again.Metrics.SessionsCount != 0 {
		t.Fatalf("Get should return a copy that callers cannot use to mutate the manager")
	}
}

func TestSetJourneyState_WhenNilPreferences_ShouldInitDefaults(t *testing.T) {
	pm := NewPreferencesManager(t.TempDir())
	if err := pm.SetJourneyState(StateLearning); err != nil {
		t.Fatalf("SetJourneyState: %v", err)
	}
	if got := pm.GetJourneyState(); got != StateLearning {
		t.Fatalf("journey state = %q, want learning", got)
	}
	if pm.Get().UserJourney.TransitionTimestamp == "" {
		t.Fatalf("SetJourneyState should stamp the transition time")
	}
}
